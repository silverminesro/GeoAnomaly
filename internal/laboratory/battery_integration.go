package laboratory

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BatteryService interface for battery management integration
type BatteryService interface {
	// vyhodnotí 1 dokončené nabíjanie (durability loss / zničenie / poistka)
	ProcessChargingResult(batteryInstanceID uuid.UUID, userID uuid.UUID) (interface{}, error)
	// voliteľné: pre status obrazovku
	GetBatteryInstances(userID uuid.UUID) (interface{}, error)
}

// EnhancedBatteryChargingSession represents a charging session with battery instance integration
type EnhancedBatteryChargingSession struct {
	BatteryChargingSession
	BatteryInstanceID   *uuid.UUID `json:"battery_instance_id,omitempty"`
	ChargingRiskPercent float64    `json:"charging_risk_percent,omitempty"`
	IsGuaranteed        bool       `json:"is_guaranteed,omitempty"`
	IsInsured           bool       `json:"is_insured,omitempty"`
}

// StartEnhancedBatteryCharging starts charging with battery instance integration
func (s *Service) StartEnhancedBatteryCharging(userID uuid.UUID, batteryInstanceID *uuid.UUID, req *StartChargingRequest, batteryService BatteryService) (*EnhancedBatteryChargingSession, error) {
	var session *EnhancedBatteryChargingSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory: %w", err)
		}
		// defense-in-depth: lab musí byť umiestnené
		if !lab.IsPlaced {
			return fmt.Errorf("laboratory must be placed on map before starting battery charging")
		}

		// Check available slots
		totalSlots := lab.BaseChargingSlots + lab.ExtraChargingSlots
		var activeCount int64
		if err := tx.Model(&BatteryChargingSession{}).Where("user_id = ? AND status = 'active'", userID).Count(&activeCount).Error; err != nil {
			return fmt.Errorf("failed to check active charging: %w", err)
		}

		if activeCount >= int64(totalSlots) {
			return fmt.Errorf("no available charging slots (max: %d)", totalSlots)
		}

		// Find next available slot
		var usedSlots []int
		if err := tx.Model(&BatteryChargingSession{}).Where("user_id = ? AND status = 'active'", userID).Pluck("slot_number", &usedSlots).Error; err != nil {
			return fmt.Errorf("failed to get used slots: %w", err)
		}

		slotNumber := 1
		for i := 1; i <= totalSlots; i++ {
			found := false
			for _, used := range usedSlots {
				if used == i {
					found = true
					break
				}
			}
			if !found {
				slotNumber = i
				break
			}
		}

		// Calculate charging time and cost based on battery type
		var duration time.Duration
		var cost int
		switch req.BatteryType {
		case "24h":
			duration = 2 * time.Hour
			cost = 50
		case "48h":
			duration = 4 * time.Hour
			cost = 100
		case "120h":
			duration = 8 * time.Hour
			cost = 200
		default:
			return fmt.Errorf("invalid battery type: %s (supported: 24h, 48h, 120h)", req.BatteryType)
		}

		// validate device type early (DB má CHECK, ale vrátime krajšiu chybu)
		if req.DeviceType != "scanner" && req.DeviceType != "drone" {
			return fmt.Errorf("invalid device type: %s", req.DeviceType)
		}

		// Apply laboratory level speed bonus (min 1.0)
		speedMultiplier := 1.0 + float64(lab.Level-1)*0.5 // L1:1.0, L2:1.5, L3:2.0
		if speedMultiplier < 1.0 {
			speedMultiplier = 1.0
		}
		duration = time.Duration(float64(duration) / speedMultiplier)

		// TODO: Check if user has battery instance to charge
		// This would require integration with battery management system
		// For now, we'll assume the user has the battery

		// Create charging session
		now := time.Now()
		newSession := BatteryChargingSession{
			UserID:            userID,
			LaboratoryID:      lab.ID,
			SlotNumber:        slotNumber,
			BatteryInstanceID: batteryInstanceID,
			BatteryType:       req.BatteryType,
			DeviceType:        req.DeviceType,
			Status:            "active",
			StartTime:         now,
			EndTime:           now.Add(duration),
			CostCredits:       cost,
		}

		if err := tx.Create(&newSession).Error; err != nil {
			return fmt.Errorf("failed to create charging session: %w", err)
		}

		// Create enhanced session with battery integration info
		session = &EnhancedBatteryChargingSession{
			BatteryChargingSession: newSession,
			BatteryInstanceID:      batteryInstanceID,
			// ChargingRiskPercent will be calculated by battery service
			// IsGuaranteed and IsInsured will be determined by battery service
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return session, nil
}

// CompleteEnhancedBatteryCharging completes charging with battery instance integration
func (s *Service) CompleteEnhancedBatteryCharging(userID uuid.UUID, sessionID uuid.UUID, batteryService BatteryService) (*EnhancedBatteryChargingSession, error) {
	var result *EnhancedBatteryChargingSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get charging session WITH LOCK to prevent double-complete
		var session BatteryChargingSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", sessionID, userID).
			First(&session).Error; err != nil {
			return fmt.Errorf("failed to get charging session: %w", err)
		}

		if session.Status != "active" {
			return fmt.Errorf("charging session is not active")
		}

		if time.Now().Before(session.EndTime) {
			return fmt.Errorf("charging is not yet complete")
		}

		// Process charging result via battery service (durability/risk/destroy/insurance)
		if batteryService != nil {
			if session.BatteryInstanceID == nil {
				return fmt.Errorf("charging session has no battery_instance_id")
			}
			if _, err := batteryService.ProcessChargingResult(*session.BatteryInstanceID, userID); err != nil {
				return fmt.Errorf("failed to process charging result: %w", err)
			}
		}

		// TODO: Update battery in inventory/device
		// This would require integration with inventory/device systems

		// Update session
		session.Status = "completed"

		if err := tx.Save(&session).Error; err != nil {
			return fmt.Errorf("failed to update charging session: %w", err)
		}

		// Update XP
		if err := s.updateLaboratoryXP(tx, userID, "battery_charging", 10); err != nil {
			return fmt.Errorf("failed to update XP: %w", err)
		}

		// Create enhanced result
		result = &EnhancedBatteryChargingSession{
			BatteryChargingSession: session,
			BatteryInstanceID:      session.BatteryInstanceID,
			// Additional fields will be populated by battery service integration
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetEnhancedChargingStatus returns charging status with battery integration info
func (s *Service) GetEnhancedChargingStatus(userID uuid.UUID, batteryService BatteryService) ([]EnhancedBatteryChargingSession, error) {
	var sessions []BatteryChargingSession
	if err := s.db.Where("user_id = ? AND status = 'active'", userID).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get charging sessions: %w", err)
	}

	var enhancedSessions []EnhancedBatteryChargingSession
	for _, session := range sessions {
		enhanced := EnhancedBatteryChargingSession{
			BatteryChargingSession: session,
			// Additional fields will be populated by battery service integration
		}

		// TODO: Get battery instance info from battery service
		// if batteryService != nil {
		//     batteryInfo, err := batteryService.GetBatteryInstanceInfo(session.BatteryInstanceID, userID)
		//     if err == nil {
		//         enhanced.ChargingRiskPercent = batteryInfo.ChargingRiskPercent
		//         enhanced.IsGuaranteed = batteryInfo.IsGuaranteed
		//         enhanced.IsInsured = batteryInfo.IsInsured
		//     }
		// }

		enhancedSessions = append(enhancedSessions, enhanced)
	}

	return enhancedSessions, nil
}
