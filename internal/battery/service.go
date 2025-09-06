package battery

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service handles battery management business logic
type Service struct {
	db  *gorm.DB
	rng *rand.Rand
}

// NewService creates a new battery service
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:  db,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetBatteryTypes returns all available battery types
func (s *Service) GetBatteryTypes() (*GetBatteryTypesResponse, error) {
	var batteryTypes []BatteryType
	if err := s.db.Where("is_active = ?", true).Find(&batteryTypes).Error; err != nil {
		return nil, fmt.Errorf("failed to get battery types: %w", err)
	}

	var batteryTypeInfos []BatteryTypeInfo
	for _, bt := range batteryTypes {
		insuranceCost := int(math.Round(float64(bt.BasePriceCredits) * (bt.InsuranceRatePercent / 100.0)))
		batteryTypeInfos = append(batteryTypeInfos, BatteryTypeInfo{
			ID:                    bt.ID,
			Name:                  bt.Name,
			DurationHours:         bt.DurationHours,
			BasePriceCredits:      bt.BasePriceCredits,
			InsuranceRatePercent:  bt.InsuranceRatePercent,
			DurabilityLossPercent: bt.DurabilityLossPercent,
			InsuranceCostCredits:  insuranceCost,
		})
	}

	return &GetBatteryTypesResponse{
		BatteryTypes: batteryTypeInfos,
	}, nil
}

// GetBatteryInstances returns all battery instances for a user
func (s *Service) GetBatteryInstances(userID uuid.UUID) (*GetBatteryInstancesResponse, error) {
	var batteries []BatteryInstance
	if err := s.db.Preload("BatteryType").Where("user_id = ?", userID).Find(&batteries).Error; err != nil {
		return nil, fmt.Errorf("failed to get battery instances: %w", err)
	}

	var batteryInfos []BatteryInstanceInfo
	for _, battery := range batteries {
		info := s.convertToBatteryInstanceInfo(battery)
		batteryInfos = append(batteryInfos, info)
	}

	return &GetBatteryInstancesResponse{
		Batteries: batteryInfos,
	}, nil
}

// PurchaseBattery allows a user to purchase a new battery
func (s *Service) PurchaseBattery(userID uuid.UUID, req *PurchaseBatteryRequest) (*PurchaseBatteryResponse, error) {
	var batteryType BatteryType
	if err := s.db.Where("id = ? AND is_active = ?", req.BatteryTypeID, true).First(&batteryType).Error; err != nil {
		return nil, fmt.Errorf("battery type not found: %w", err)
	}

	// TODO: Check if user has enough credits and deduct them
	// For now, we'll assume the user has enough credits

	batteryInstance := BatteryInstance{
		UserID:                   userID,
		BatteryTypeID:            batteryType.ID,
		CurrentDurabilityPercent: 100.0,
		ChargingCount:            0,
		IsInsured:                false,
		PurchasePriceCredits:     batteryType.BasePriceCredits,
		PurchasedAt:              time.Now(),
		IsDestroyed:              false,
	}

	if err := s.db.Create(&batteryInstance).Error; err != nil {
		return nil, fmt.Errorf("failed to create battery instance: %w", err)
	}

	// Reload with relations
	if err := s.db.Preload("BatteryType").First(&batteryInstance, batteryInstance.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload battery instance: %w", err)
	}

	info := s.convertToBatteryInstanceInfo(batteryInstance)

	return &PurchaseBatteryResponse{
		Success:         true,
		BatteryInstance: info,
		Message:         "Battery purchased successfully",
	}, nil
}

// PurchaseInsurance allows a user to purchase insurance for a battery
func (s *Service) PurchaseInsurance(userID uuid.UUID, req *PurchaseInsuranceRequest) (*PurchaseInsuranceResponse, error) {
	var battery BatteryInstance
	if err := s.db.Preload("BatteryType").Where("id = ? AND user_id = ?", req.BatteryInstanceID, userID).First(&battery).Error; err != nil {
		return nil, fmt.Errorf("battery not found: %w", err)
	}

	// Check if battery can be insured
	if !s.canInsureBattery(battery) {
		return nil, fmt.Errorf("battery cannot be insured (durability must be > 50%% and not already insured)")
	}

	// Calculate insurance cost
	insuranceCost := s.calculateInsuranceCost(battery.BatteryType, battery.PurchasePriceCredits)

	// TODO: Check if user has enough credits and deduct them
	// For now, we'll assume the user has enough credits

	now := time.Now()
	battery.IsInsured = true
	battery.InsurancePurchasedAt = &now
	battery.InsuranceCostCredits = &insuranceCost

	if err := s.db.Save(&battery).Error; err != nil {
		return nil, fmt.Errorf("failed to update battery insurance: %w", err)
	}

	return &PurchaseInsuranceResponse{
		Success: true,
		Message: fmt.Sprintf("Insurance purchased for %d credits", insuranceCost),
	}, nil
}

// SellBattery allows a user to sell a used battery
func (s *Service) SellBattery(userID uuid.UUID, req *SellBatteryRequest) (*SellBatteryResponse, error) {
	var battery BatteryInstance
	if err := s.db.Where("id = ? AND user_id = ? AND is_destroyed = ?", req.BatteryInstanceID, userID, false).First(&battery).Error; err != nil {
		return nil, fmt.Errorf("battery not found: %w", err)
	}

	// Calculate sell price
	sellPrice := s.calculateSellPrice(battery)

	// TODO: Add credits to user account
	// For now, we'll just mark the battery as sold (we could delete it or mark it differently)

	// Delete the battery instance (it's been sold)
	if err := s.db.Delete(&battery).Error; err != nil {
		return nil, fmt.Errorf("failed to sell battery: %w", err)
	}

	return &SellBatteryResponse{
		Success:          true,
		SellPriceCredits: sellPrice,
		Message:          fmt.Sprintf("Battery sold for %d credits", sellPrice),
	}, nil
}

// GetRiskAssessment returns risk assessment for a battery
func (s *Service) GetRiskAssessment(userID uuid.UUID, batteryInstanceID uuid.UUID) (*GetRiskAssessmentResponse, error) {
	var battery BatteryInstance
	if err := s.db.Preload("BatteryType").Where("id = ? AND user_id = ?", batteryInstanceID, userID).First(&battery).Error; err != nil {
		return nil, fmt.Errorf("battery not found: %w", err)
	}

	riskInfo := s.calculateChargingRisk(battery)

	return &GetRiskAssessmentResponse{
		BatteryInstanceID:        battery.ID,
		CurrentDurabilityPercent: battery.CurrentDurabilityPercent,
		ChargingCount:            battery.ChargingCount,
		ChargingRiskPercent:      riskInfo.ChargingRiskPercent,
		IsGuaranteed:             riskInfo.IsGuaranteed,
		GuaranteeChargesLeft:     riskInfo.GuaranteeChargesLeft,
		CanInsure:                riskInfo.CanInsure,
		InsuranceCostCredits:     riskInfo.InsuranceCostCredits,
	}, nil
}

// GetInsuranceClaims returns insurance claims for a user
func (s *Service) GetInsuranceClaims(userID uuid.UUID) (*GetInsuranceClaimsResponse, error) {
	var claims []BatteryInsuranceClaim
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&claims).Error; err != nil {
		return nil, fmt.Errorf("failed to get insurance claims: %w", err)
	}

	var claimInfos []InsuranceClaimInfo
	for _, claim := range claims {
		claimInfos = append(claimInfos, InsuranceClaimInfo{
			ID:                 claim.ID,
			BatteryInstanceID:  claim.BatteryInstanceID,
			ClaimAmountCredits: claim.ClaimAmountCredits,
			ClaimReason:        claim.ClaimReason,
			ClaimStatus:        claim.ClaimStatus,
			ProcessedAt:        claim.ProcessedAt,
			CreatedAt:          claim.CreatedAt,
		})
	}

	return &GetInsuranceClaimsResponse{
		Claims: claimInfos,
	}, nil
}

// GetBatteryStats returns battery statistics for a user
func (s *Service) GetBatteryStats(userID uuid.UUID) (*GetBatteryStatsResponse, error) {
	var stats BatteryStats

	// Count total batteries
	var totalBatteries int64
	s.db.Model(&BatteryInstance{}).Where("user_id = ?", userID).Count(&totalBatteries)
	stats.TotalBatteries = int(totalBatteries)

	// Count active batteries
	var activeBatteries int64
	s.db.Model(&BatteryInstance{}).Where("user_id = ? AND is_destroyed = ?", userID, false).Count(&activeBatteries)
	stats.ActiveBatteries = int(activeBatteries)

	// Count destroyed batteries
	var destroyedBatteries int64
	s.db.Model(&BatteryInstance{}).Where("user_id = ? AND is_destroyed = ?", userID, true).Count(&destroyedBatteries)
	stats.DestroyedBatteries = int(destroyedBatteries)

	// Count insured batteries
	var insuredBatteries int64
	s.db.Model(&BatteryInstance{}).Where("user_id = ? AND is_insured = ?", userID, true).Count(&insuredBatteries)
	stats.InsuredBatteries = int(insuredBatteries)

	// Sum total charging count
	var totalCharging int
	s.db.Model(&BatteryInstance{}).Where("user_id = ?", userID).Select("COALESCE(SUM(charging_count), 0)").Scan(&totalCharging)
	stats.TotalChargingCount = totalCharging

	// Calculate average durability
	var avgDurability float64
	s.db.Model(&BatteryInstance{}).Where("user_id = ? AND is_destroyed = ?", userID, false).Select("COALESCE(AVG(current_durability_percent), 0)").Scan(&avgDurability)
	stats.AverageDurability = avgDurability

	// Sum total insurance cost
	var totalInsuranceCost int
	s.db.Model(&BatteryInstance{}).Where("user_id = ? AND is_insured = ?", userID, true).Select("COALESCE(SUM(insurance_cost_credits), 0)").Scan(&totalInsuranceCost)
	stats.TotalInsuranceCost = totalInsuranceCost

	// Sum total claim amount
	var totalClaimAmount int
	s.db.Model(&BatteryInsuranceClaim{}).Where("user_id = ?", userID).Select("COALESCE(SUM(claim_amount_credits), 0)").Scan(&totalClaimAmount)
	stats.TotalClaimAmount = totalClaimAmount

	return &GetBatteryStatsResponse{
		Stats: stats,
	}, nil
}

// ProcessChargingResult processes one charging attempt atomicky (TX + FOR UPDATE)
func (s *Service) ProcessChargingResult(batteryInstanceID uuid.UUID, userID uuid.UUID) (*ChargingRiskInfo, error) {
	var out *ChargingRiskInfo
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1) Načítaj batériu s lockom
		var battery BatteryInstance
		if err := tx.Preload("BatteryType").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", batteryInstanceID, userID).
			First(&battery).Error; err != nil {
			return fmt.Errorf("battery not found: %w", err)
		}

		// 2) Vstupné guardy
		if battery.IsDestroyed {
			return fmt.Errorf("battery is already destroyed")
		}
		if battery.CurrentDurabilityPercent <= 0 {
			return fmt.Errorf("battery has 0%% durability and cannot be charged")
		}

		// 3) Riziko pred pokusom
		riskInfo := s.calculateChargingRisk(battery)

		// 4) Náhodný výsledok
		destroyed := s.isBatteryDestroyed(riskInfo.ChargingRiskPercent)
		now := time.Now()

		if destroyed {
			battery.IsDestroyed = true
			battery.DestroyedAt = &now

			if battery.IsInsured {
				claimAmount := int(math.Round(float64(battery.PurchasePriceCredits) * 0.5))
				claim := BatteryInsuranceClaim{
					BatteryInstanceID:  battery.ID,
					UserID:             userID,
					ClaimAmountCredits: claimAmount,
					ClaimReason:        ClaimReasonChargingDestruction,
					ClaimStatus:        ClaimStatusApproved, // auto-approve MVP
					ProcessedAt:        &now,
				}
				if err := tx.Create(&claim).Error; err != nil {
					return fmt.Errorf("failed to create insurance claim: %w", err)
				}
				// TODO: kreditovať hráča o claimAmount
			}

			if err := tx.Save(&battery).Error; err != nil {
				return fmt.Errorf("failed to update destroyed battery: %w", err)
			}

			out = &riskInfo
			return nil
		}

		// 5) Prežilo – zníž opotrebenie, zvýš charging_count, nastav správny LastChargedAt
		battery.ChargingCount++
		battery.CurrentDurabilityPercent -= battery.BatteryType.DurabilityLossPercent
		if battery.CurrentDurabilityPercent < 0 {
			battery.CurrentDurabilityPercent = 0
		}
		battery.LastChargedAt = &now

		if err := tx.Save(&battery).Error; err != nil {
			return fmt.Errorf("failed to update battery after charging: %w", err)
		}

		// Prepočítať risk po updatoch
		updated := s.calculateChargingRisk(battery)
		out = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Helper methods

// convertToBatteryInstanceInfo converts a BatteryInstance to BatteryInstanceInfo
func (s *Service) convertToBatteryInstanceInfo(battery BatteryInstance) BatteryInstanceInfo {
	// Calculate insurance cost (type-level) a priprav instance-level hodnotu
	insuranceCost := 0
	if !battery.IsInsured {
		insuranceCost = s.calculateInsuranceCost(battery.BatteryType, battery.PurchasePriceCredits)
	}
	var instanceInsuranceCost *int
	if battery.IsInsured {
		instanceInsuranceCost = battery.InsuranceCostCredits
	} else {
		// zobraz výpočtovú cenu poistenia aj priamo v instancii (komfort pre klienta)
		c := insuranceCost
		instanceInsuranceCost = &c
	}

	// Calculate risk info
	riskInfo := s.calculateChargingRisk(battery)

	// Calculate sell price
	sellPrice := s.calculateSellPrice(battery)

	// Convert battery type
	batteryTypeInfo := BatteryTypeInfo{
		ID:                    battery.BatteryType.ID,
		Name:                  battery.BatteryType.Name,
		DurationHours:         battery.BatteryType.DurationHours,
		BasePriceCredits:      battery.BatteryType.BasePriceCredits,
		InsuranceRatePercent:  battery.BatteryType.InsuranceRatePercent,
		DurabilityLossPercent: battery.BatteryType.DurabilityLossPercent,
		InsuranceCostCredits:  insuranceCost,
	}

	return BatteryInstanceInfo{
		ID:                       battery.ID,
		BatteryType:              batteryTypeInfo,
		CurrentDurabilityPercent: battery.CurrentDurabilityPercent,
		ChargingCount:            battery.ChargingCount,
		IsInsured:                battery.IsInsured,
		InsuranceCostCredits:     instanceInsuranceCost,
		PurchasePriceCredits:     battery.PurchasePriceCredits,
		PurchasedAt:              battery.PurchasedAt,
		LastChargedAt:            battery.LastChargedAt,
		IsDestroyed:              battery.IsDestroyed,
		DestroyedAt:              battery.DestroyedAt,
		CanInsure:                riskInfo.CanInsure,
		ChargingRiskPercent:      riskInfo.ChargingRiskPercent,
		SellPriceCredits:         sellPrice,
	}
}

// calculateChargingRisk calculates the charging risk for a battery
func (s *Service) calculateChargingRisk(battery BatteryInstance) ChargingRiskInfo {
	// First 2 charges are guaranteed safe
	isGuaranteed := battery.ChargingCount < GuaranteeCharges
	guaranteeChargesLeft := GuaranteeCharges - battery.ChargingCount
	if guaranteeChargesLeft < 0 {
		guaranteeChargesLeft = 0
	}

	// Calculate risk percentage
	riskPercent := 0.0
	if !isGuaranteed {
		// Base risk + durability factor
		durabilityFactor := (100.0 - battery.CurrentDurabilityPercent) / 100.0
		riskPercent = BaseChargingRiskPercent * (1.0 + durabilityFactor)

		// Cap at maximum risk
		if riskPercent > MaxChargingRiskPercent {
			riskPercent = MaxChargingRiskPercent
		}
	}

	// Check if can insure
	canInsure := s.canInsureBattery(battery)
	insuranceCost := 0
	if canInsure {
		insuranceCost = s.calculateInsuranceCost(battery.BatteryType, battery.PurchasePriceCredits)
	}

	return ChargingRiskInfo{
		BatteryInstanceID:        battery.ID,
		CurrentDurabilityPercent: battery.CurrentDurabilityPercent,
		ChargingCount:            battery.ChargingCount,
		ChargingRiskPercent:      riskPercent,
		IsGuaranteed:             isGuaranteed,
		GuaranteeChargesLeft:     guaranteeChargesLeft,
		IsInsured:                battery.IsInsured,
		CanInsure:                canInsure,
		InsuranceCostCredits:     insuranceCost,
	}
}

// canInsureBattery checks if a battery can be insured
func (s *Service) canInsureBattery(battery BatteryInstance) bool {
	return battery.CurrentDurabilityPercent > InsuranceThresholdPercent && !battery.IsInsured && !battery.IsDestroyed
}

// calculateInsuranceCost calculates the insurance cost for a battery
func (s *Service) calculateInsuranceCost(batteryType BatteryType, purchasePrice int) int {
	return int(math.Round(float64(purchasePrice) * (batteryType.InsuranceRatePercent / 100.0)))
}

// calculateSellPrice calculates the sell price for a used battery
func (s *Service) calculateSellPrice(battery BatteryInstance) int {
	// Preferuj DB funkciu a tabuľku battery_market_prices; ak zlyhá, fallback 20 %
	var price int
	if err := s.db.
		Raw("SELECT laboratory.calculate_sell_price(?, ?)", battery.BatteryTypeID, battery.CurrentDurabilityPercent).
		Scan(&price).Error; err == nil && price >= 0 {
		return price
	}
	return int(math.Round(float64(battery.PurchasePriceCredits) * (SellPricePercent / 100.0)))
}

// isBatteryDestroyed determines if a battery is destroyed based on risk percentage
func (s *Service) isBatteryDestroyed(riskPercent float64) bool {
	// Generate random number between 0 and 100 (service-scoped RNG)
	randomValue := s.rng.Float64() * 100.0
	return randomValue < riskPercent
}
