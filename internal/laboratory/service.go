package laboratory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================
// 1. SERVICE STRUCTURE
// =============================================

type Service struct {
	db        *gorm.DB
	xpHandler XPHandler
}

// XPHandler interface pre dependency injection
type XPHandler interface {
	AwardLaboratoryXP(userID uuid.UUID, activity string, amount int) (interface{}, error)
	GetLaboratoryXPAmount(activity string) int
	GetUserLaboratoryLevel(playerLevel int) int
	CanUnlockLaboratoryFeature(playerLevel int, feature string) bool
}

// XPResult reprezentuje výsledok XP operácie
type XPResult struct {
	XPGained     int          `json:"xp_gained"`
	TotalXP      int          `json:"total_xp"`
	CurrentLevel int          `json:"current_level"`
	LevelUp      bool         `json:"level_up"`
	LevelUpInfo  *LevelUpInfo `json:"level_up_info,omitempty"`
}

// LevelUpInfo reprezentuje informácie o level up
type LevelUpInfo struct {
	OldLevel    int      `json:"old_level"`
	NewLevel    int      `json:"new_level"`
	Rewards     []string `json:"rewards"`
	LevelUpTime int64    `json:"level_up_time"`
}

func NewService(db *gorm.DB, xpHandler XPHandler) *Service {
	return &Service{
		db:        db,
		xpHandler: xpHandler,
	}
}

// =============================================
// 2. LABORATORY MANAGEMENT
// =============================================

// GetLaboratoryStatus returns complete laboratory status for a user
func (s *Service) GetLaboratoryStatus(userID uuid.UUID) (*LaboratoryStatusResponse, error) {
	// Get user info to determine player level
	var user User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get or create laboratory
	var lab Laboratory
	if err := s.db.Where("user_id = ?", userID).First(&lab).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default laboratory for new user
			lab = Laboratory{
				UserID:             userID,
				Level:              1,
				BaseChargingSlots:  1,
				ExtraChargingSlots: 0,
				ResearchUnlocked:   false,
				CraftingUnlocked:   false,
			}
			if err := s.db.Create(&lab).Error; err != nil {
				return nil, fmt.Errorf("failed to create laboratory: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get laboratory: %w", err)
		}
	}

	// Update laboratory features based on player level
	if err := s.updateLaboratoryFeaturesFromPlayerLevel(&lab, user.Level); err != nil {
		return nil, fmt.Errorf("failed to update laboratory features: %w", err)
	}

	// Get user XP info (now using player XP system)
	userXP := &LaboratoryXP{
		UserID:           userID,
		TotalXP:          user.XP,
		ResearchXP:       0, // Will be calculated from activities
		CraftingXP:       0, // Will be calculated from activities
		TaskXP:           0, // Will be calculated from activities
		Level:            user.Level,
		UnlockedFeatures: JSONB{},
	}

	// Get upgrade requirements (now based on player level)
	upgradeRequirements := []LaboratoryUpgradeRequirement{}
	if user.Level < 2 {
		// Show requirements for level 2 (research unlock)
		upgradeRequirements = append(upgradeRequirements, LaboratoryUpgradeRequirement{
			Level:            2,
			CreditsRequired:  0, // No credits needed, just player level
			ArtifactRequired: nil,
			ArtifactRarity:   nil,
		})
	}
	if user.Level < 3 {
		// Show requirements for level 3 (crafting unlock)
		upgradeRequirements = append(upgradeRequirements, LaboratoryUpgradeRequirement{
			Level:            3,
			CreditsRequired:  0, // No credits needed, just player level
			ArtifactRequired: nil,
			ArtifactRarity:   nil,
		})
	}

	// Check if laboratory needs to be placed
	if !lab.IsPlaced {
		return &LaboratoryStatusResponse{
			Laboratory:          &lab,
			ActiveCharging:      []BatteryChargingSession{},
			ActiveResearch:      []ResearchProject{},
			ActiveCrafting:      []CraftingSession{},
			AvailableTasks:      []LaboratoryTask{},
			XP:                  userXP,
			UpgradeRequirements: upgradeRequirements,
			NeedsPlacement:      true,
		}, nil
	}

	// Get active charging sessions
	var activeCharging []BatteryChargingSession
	if err := s.db.Where("user_id = ? AND status = 'active'", userID).Find(&activeCharging).Error; err != nil {
		return nil, fmt.Errorf("failed to get active charging: %w", err)
	}

	// Calculate progress for each charging session
	now := time.Now()
	for i := range activeCharging {
		session := &activeCharging[i]
		if session.Status == "active" {
			// Calculate progress based on time elapsed
			totalDuration := session.EndTime.Sub(session.StartTime)
			elapsed := now.Sub(session.StartTime)

			if totalDuration > 0 {
				progress := float64(elapsed) / float64(totalDuration) * 100.0
				if progress > 100.0 {
					progress = 100.0
				}
				session.Progress = progress
			} else {
				session.Progress = 0.0
			}
		}
	}

	// Get active research projects (Level 2+)
	var activeResearch []ResearchProject
	if lab.ResearchUnlocked {
		if err := s.db.Where("user_id = ? AND status = 'active'", userID).Find(&activeResearch).Error; err != nil {
			return nil, fmt.Errorf("failed to get active research: %w", err)
		}
	}

	// Get active crafting sessions (Level 3+)
	var activeCrafting []CraftingSession
	if lab.CraftingUnlocked {
		if err := s.db.Where("user_id = ? AND status = 'active'", userID).Find(&activeCrafting).Error; err != nil {
			return nil, fmt.Errorf("failed to get active crafting: %w", err)
		}
	}

	// Get available tasks
	var availableTasks []LaboratoryTask
	if err := s.db.Where("user_id = ? AND status = 'active' AND expires_at > ?", userID, time.Now()).Find(&availableTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get available tasks: %w", err)
	}

	return &LaboratoryStatusResponse{
		Laboratory:          &lab,
		ActiveCharging:      activeCharging,
		ActiveResearch:      activeResearch,
		ActiveCrafting:      activeCrafting,
		AvailableTasks:      availableTasks,
		XP:                  userXP,
		UpgradeRequirements: upgradeRequirements,
		NeedsPlacement:      false,
	}, nil
}

// UpgradeLaboratory upgrades laboratory to next level (now automatic based on player level)
func (s *Service) UpgradeLaboratory(userID uuid.UUID, targetLevel int) error {
	// Get current user
	var user User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check if player level is sufficient
	if user.Level < targetLevel {
		return fmt.Errorf("player level %d is insufficient for laboratory level %d (requires player level %d)",
			user.Level, targetLevel, targetLevel)
	}

	// Get current laboratory
	var lab Laboratory
	if err := s.db.Where("user_id = ?", userID).First(&lab).Error; err != nil {
		return fmt.Errorf("failed to get laboratory: %w", err)
	}

	// Update laboratory features based on player level
	if err := s.updateLaboratoryFeaturesFromPlayerLevel(&lab, user.Level); err != nil {
		return fmt.Errorf("failed to update laboratory features: %w", err)
	}

	return nil
}

// PurchaseExtraChargingSlot allows user to buy extra charging slot with essence
func (s *Service) PurchaseExtraChargingSlot(userID uuid.UUID, essenceCost int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory: %w", err)
		}

		// TODO: Check if user has enough essence
		// This would require integration with user/inventory systems

		// Create purchase record
		purchase := ChargingSlotPurchase{
			UserID:       userID,
			LaboratoryID: lab.ID,
			EssenceCost:  essenceCost,
		}

		if err := tx.Create(&purchase).Error; err != nil {
			return fmt.Errorf("failed to create slot purchase: %w", err)
		}

		// Update laboratory
		if err := tx.Model(&lab).Update("extra_charging_slots", lab.ExtraChargingSlots+1).Error; err != nil {
			return fmt.Errorf("failed to update extra slots: %w", err)
		}

		return nil
	})
}

// =============================================
// 3. RESEARCH SYSTEM (Level 2+)
// =============================================

// StartResearch starts a research project for an artifact
func (s *Service) StartResearch(userID uuid.UUID, req *StartResearchRequest) (*ResearchProject, error) {
	var project *ResearchProject
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory: %w", err)
		}

		if !lab.ResearchUnlocked {
			return fmt.Errorf("research system requires laboratory level 2 or higher")
		}
		// defense-in-depth: lab musí byť umiestnené
		if !lab.IsPlaced {
			return fmt.Errorf("laboratory must be placed on map before starting research")
		}

		// Check if user already has active research
		var activeCount int64
		if err := tx.Model(&ResearchProject{}).Where("user_id = ? AND status = 'active'", userID).Count(&activeCount).Error; err != nil {
			return fmt.Errorf("failed to check active research: %w", err)
		}

		maxResearchSlots := lab.Level // Level 2 = 2 slots, Level 3 = 3 slots
		if activeCount >= int64(maxResearchSlots) {
			return fmt.Errorf("maximum research slots reached (%d)", maxResearchSlots)
		}

		// TODO: Check if user owns the artifact
		// This would require integration with inventory system

		// Calculate research time and cost
		var duration time.Duration
		var cost int
		switch req.ResearchType {
		case "basic":
			duration = 1 * time.Hour
			cost = 100
		case "advanced":
			duration = 4 * time.Hour
			cost = 500
		case "expert":
			duration = 12 * time.Hour
			cost = 2000
		default:
			return fmt.Errorf("invalid research type: %s", req.ResearchType)
		}

		// TODO: Deduct credits from user
		// This would require integration with user system

		// Create research project
		newProject := ResearchProject{
			UserID:        userID,
			LaboratoryID:  lab.ID,
			ArtifactID:    req.ArtifactID,
			ResearchType:  req.ResearchType,
			StartTime:     time.Now(),
			EndTime:       time.Now().Add(duration),
			Status:        "active",
			ResearchLevel: lab.Level,
			Cost:          cost,
		}

		if err := tx.Create(&newProject).Error; err != nil {
			return fmt.Errorf("failed to create research project: %w", err)
		}

		project = &newProject
		return nil
	})

	if err != nil {
		return nil, err
	}
	return project, nil
}

// CompleteResearch completes a research project and returns results
func (s *Service) CompleteResearch(userID uuid.UUID, projectID uuid.UUID) (*ResearchResult, error) {
	var result *ResearchResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get research project
		var project ResearchProject
		if err := tx.Where("id = ? AND user_id = ?", projectID, userID).First(&project).Error; err != nil {
			return fmt.Errorf("failed to get research project: %w", err)
		}

		if project.Status != "active" {
			return fmt.Errorf("research project is not active")
		}

		if time.Now().Before(project.EndTime) {
			return fmt.Errorf("research is not yet complete")
		}

		// TODO: Generate research results based on artifact properties
		// This would require integration with artifact system
		researchResult := &ResearchResult{
			TrueRarity:       "rare", // Placeholder
			HiddenProperties: []string{"durability_boost", "efficiency_bonus"},
			CraftingValue:    150,
			MarketValue:      200,
			SpecialEffects:   []string{"battery_life_extension"},
			ResearchXP:       25,
		}

		// Update project
		project.Status = "completed"
		project.Results = &JSONB{
			"true_rarity":       researchResult.TrueRarity,
			"hidden_properties": researchResult.HiddenProperties,
			"crafting_value":    researchResult.CraftingValue,
			"market_value":      researchResult.MarketValue,
			"special_effects":   researchResult.SpecialEffects,
			"research_xp":       researchResult.ResearchXP,
		}

		if err := tx.Save(&project).Error; err != nil {
			return fmt.Errorf("failed to update research project: %w", err)
		}

		// Update XP
		xpType := fmt.Sprintf("research_%s", project.ResearchType)
		if err := s.updateLaboratoryXP(tx, userID, xpType, researchResult.ResearchXP); err != nil {
			return fmt.Errorf("failed to update XP: %w", err)
		}

		result = researchResult
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// =============================================
// 4. CRAFTING SYSTEM (Level 3+)
// =============================================

// StartCrafting starts a crafting session
func (s *Service) StartCrafting(userID uuid.UUID, req *StartCraftingRequest) (*CraftingSession, error) {
	var session *CraftingSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory: %w", err)
		}

		if !lab.CraftingUnlocked {
			return fmt.Errorf("crafting system requires laboratory level 3 or higher")
		}
		// defense-in-depth: lab musí byť umiestnené
		if !lab.IsPlaced {
			return fmt.Errorf("laboratory must be placed on map before starting crafting")
		}

		// Get recipe
		var recipe CraftingRecipe
		if err := tx.Where("id = ? AND laboratory_level_required <= ?", req.RecipeID, lab.Level).First(&recipe).Error; err != nil {
			return fmt.Errorf("failed to get recipe: %w", err)
		}

		// Check if user already has active crafting
		var activeCount int64
		if err := tx.Model(&CraftingSession{}).Where("user_id = ? AND status = 'active'", userID).Count(&activeCount).Error; err != nil {
			return fmt.Errorf("failed to check active crafting: %w", err)
		}

		maxCraftingSlots := lab.Level // Level 3 = 3 slots
		if activeCount >= int64(maxCraftingSlots) {
			return fmt.Errorf("maximum crafting slots reached (%d)", maxCraftingSlots)
		}

		// TODO: Check if user has required materials
		// This would require integration with inventory system

		// Create crafting session
		newSession := CraftingSession{
			UserID:       userID,
			LaboratoryID: lab.ID,
			RecipeID:     req.RecipeID,
			StartTime:    time.Now(),
			EndTime:      time.Now().Add(time.Duration(recipe.CraftTimeSeconds) * time.Second),
			Status:       "active",
			Progress:     0.0,
		}

		if err := tx.Create(&newSession).Error; err != nil {
			return fmt.Errorf("failed to create crafting session: %w", err)
		}

		session = &newSession
		return nil
	})

	if err != nil {
		return nil, err
	}
	return session, nil
}

// CompleteCrafting completes a crafting session
func (s *Service) CompleteCrafting(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get crafting session
		var session CraftingSession
		if err := tx.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
			return fmt.Errorf("failed to get crafting session: %w", err)
		}

		if session.Status != "active" {
			return fmt.Errorf("crafting session is not active")
		}

		if time.Now().Before(session.EndTime) {
			return fmt.Errorf("crafting is not yet complete")
		}

		// TODO: Add crafted item to user inventory
		// This would require integration with inventory system

		// Update session
		session.Status = "completed"
		session.Progress = 1.0

		if err := tx.Save(&session).Error; err != nil {
			return fmt.Errorf("failed to update crafting session: %w", err)
		}

		// Get recipe for XP
		var recipe CraftingRecipe
		if err := tx.Where("id = ?", session.RecipeID).First(&recipe).Error; err != nil {
			return fmt.Errorf("failed to get recipe: %w", err)
		}

		// Update XP
		xpType := fmt.Sprintf("crafting_%s", getCraftingLevel(recipe.Level))
		if err := s.updateLaboratoryXP(tx, userID, xpType, recipe.XPReward); err != nil {
			return fmt.Errorf("failed to update XP: %w", err)
		}

		return nil
	})
}

// =============================================
// 5. BATTERY CHARGING SYSTEM (Level 1+)
// =============================================

// GetAvailableBatteries returns batteries available for charging from user inventory
func (s *Service) GetAvailableBatteries(userID uuid.UUID) ([]AvailableBattery, error) {
	var batteries []AvailableBattery

	// Query to get all scanner batteries from user inventory (only 0% batérie)
	query := `
		SELECT 
			ii.id AS inventory_id,
			ii.properties,
			ii.acquired_at,
			-- Check if battery is currently in use
			dd.id IS NOT NULL as is_in_use,
			dd.name as device_name,
			-- Battery label & type (safe defaults)
			COALESCE(ii.properties->>'display_name','Battery') as battery_name,
			COALESCE(ii.properties->>'level','1') as battery_type,
			-- Current charge parsed leniently (non-numeric → 0)
			COALESCE(
			  CASE 
			    WHEN (ii.properties->>'charge_pct') ~ '^[0-9]+$' 
			      THEN (ii.properties->>'charge_pct')::int
			    ELSE 0
			  END, 0
			) as current_charge
		FROM gameplay.inventory_items ii
		LEFT JOIN gameplay.deployed_devices dd 
		  ON dd.battery_inventory_id = ii.id AND dd.is_active = TRUE
		WHERE ii.user_id = ? 
			AND ii.item_type = 'scanner_battery' 
			AND ii.deleted_at IS NULL
			AND (
			  CASE 
			    WHEN (ii.properties->>'charge_pct') ~ '^[0-9]+$' 
			      THEN (ii.properties->>'charge_pct')::int
			    ELSE 0
			  END
			) = 0
		ORDER BY ii.acquired_at DESC
	`

	rows, err := s.db.Raw(query, userID).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query available batteries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var battery AvailableBattery
		var propertiesStr string
		var batteryTypeStr string
		var deviceName sql.NullString

		if err := rows.Scan(
			&battery.InventoryID,
			&propertiesStr,
			&battery.AcquiredAt,
			&battery.IsInUse,
			&deviceName,
			&battery.BatteryName,
			&batteryTypeStr,
			&battery.CurrentCharge,
		); err != nil {
			return nil, fmt.Errorf("failed to scan battery row: %w", err)
		}

		// Parse properties JSON
		if err := json.Unmarshal([]byte(propertiesStr), &battery.Properties); err != nil {
			// If parsing fails, use empty JSONB
			battery.Properties = make(JSONB)
		}

		// Determine battery type from level string
		switch batteryTypeStr {
		case "1": // Basic (Level 1)
			battery.BatteryType = "basic"
		case "3": // Enhanced (Level 3)
			battery.BatteryType = "enhanced"
		case "5": // Advanced (Level 5)
			battery.BatteryType = "advanced"
		default:
			battery.BatteryType = "unknown"
		}

		if deviceName.Valid {
			name := deviceName.String
			battery.DeviceName = &name
		} else {
			battery.DeviceName = nil
		}
		batteries = append(batteries, battery)
	}

	return batteries, nil
}

// GetChargingSlots returns all charging slots with their current status
func (s *Service) GetChargingSlots(userID uuid.UUID) ([]ChargingSlot, error) {
	// Get laboratory to determine total slots
	var lab Laboratory
	if err := s.db.Where("user_id = ?", userID).First(&lab).Error; err != nil {
		return nil, fmt.Errorf("failed to get laboratory: %w", err)
	}

	totalSlots := lab.BaseChargingSlots + lab.ExtraChargingSlots
	slots := make([]ChargingSlot, totalSlots)

	// Get active charging sessions + recently completed (last 5 minutes)
	var activeSessions []BatteryChargingSession
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	if err := s.db.Where("user_id = ? AND (status = 'active' OR (status = 'completed' AND updated_at > ?))", userID, fiveMinutesAgo).Find(&activeSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get charging sessions: %w", err)
	}

	// Create a map of slot numbers to active sessions
	slotToSession := make(map[int]*BatteryChargingSession)
	for i := range activeSessions {
		slotToSession[activeSessions[i].SlotNumber] = &activeSessions[i]
	}

	// Initialize all slots
	for i := 0; i < totalSlots; i++ {
		slotNumber := i + 1
		activeSession := slotToSession[slotNumber]

		var batteryInstanceID *uuid.UUID
		if activeSession != nil {
			batteryInstanceID = activeSession.BatteryInstanceID
		}

		slots[i] = ChargingSlot{
			SlotNumber:        slotNumber,
			IsAvailable:       activeSession == nil || activeSession.Status == "completed",
			ActiveSession:     activeSession,
			BatteryInstanceID: batteryInstanceID,
		}
	}

	return slots, nil
}

// StartBatteryCharging starts charging a battery
func (s *Service) StartBatteryCharging(userID uuid.UUID, req *StartChargingRequest) (*BatteryChargingSession, error) {
	var session *BatteryChargingSession
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

		// Calculate charging time and cost
		var duration time.Duration
		var cost int
		switch req.BatteryType {
		case "basic":
			duration = 2 * time.Hour
			cost = 50
		case "enhanced":
			duration = 4 * time.Hour
			cost = 100
		case "advanced":
			duration = 8 * time.Hour
			cost = 200
		default:
			return fmt.Errorf("invalid battery type: %s", req.BatteryType)
		}

		// validate device type early (DB má CHECK, ale vrátime krajšiu chybu)
		if req.DeviceType != "scanner" && req.DeviceType != "drone" {
			return fmt.Errorf("invalid device type: %s", req.DeviceType)
		}

		// Apply laboratory level speed bonus
		speedMultiplier := 1.0 + float64(lab.Level-1)*0.5 // Level 2: 1.5x, Level 3: 2.0x
		duration = time.Duration(float64(duration) / speedMultiplier)

		// Validate battery instance if provided
		if req.BatteryInstanceID != nil {
			// Check if user owns the battery and it's not in use
			var batteryCount int64
			if err := tx.Model(&InventoryItem{}).
				Where("id = ? AND user_id = ? AND item_type = 'scanner_battery' AND deleted_at IS NULL",
					*req.BatteryInstanceID, userID).
				Count(&batteryCount).Error; err != nil {
				return fmt.Errorf("failed to validate battery ownership: %w", err)
			}
			if batteryCount == 0 {
				return fmt.Errorf("battery not found in your inventory")
			}

			// Check if battery is already in use
			var batteryInUseCount int64
			if err := tx.Model(&DeployedDevice{}).
				Where("battery_inventory_id = ? AND is_active = TRUE", *req.BatteryInstanceID).
				Count(&batteryInUseCount).Error; err != nil {
				return fmt.Errorf("failed to check battery usage: %w", err)
			}
			if batteryInUseCount > 0 {
				return fmt.Errorf("battery is currently in use in a deployed device")
			}

			// Check if battery is already being charged
			var chargingCount int64
			if err := tx.Model(&BatteryChargingSession{}).
				Where("battery_instance_id = ? AND status = 'active'", *req.BatteryInstanceID).
				Count(&chargingCount).Error; err != nil {
				return fmt.Errorf("failed to check battery charging status: %w", err)
			}
			if chargingCount > 0 {
				return fmt.Errorf("battery is already being charged")
			}
		}

		// Create charging session
		newSession := BatteryChargingSession{
			UserID:            userID,
			LaboratoryID:      lab.ID,
			SlotNumber:        slotNumber,
			BatteryType:       req.BatteryType,
			DeviceType:        req.DeviceType,
			DeviceID:          req.DeviceID,
			BatteryInstanceID: req.BatteryInstanceID,
			StartTime:         time.Now(),
			EndTime:           time.Now().Add(duration),
			Status:            "active",
			ChargingSpeed:     speedMultiplier,
			CostCredits:       cost,
			Progress:          0.0,
		}

		if err := tx.Create(&newSession).Error; err != nil {
			return fmt.Errorf("failed to create charging session: %w", err)
		}

		session = &newSession
		return nil
	})

	if err != nil {
		return nil, err
	}
	return session, nil
}

// CompleteBatteryCharging completes a battery charging session
func (s *Service) CompleteBatteryCharging(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get charging session
		var session BatteryChargingSession
		if err := tx.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
			return fmt.Errorf("failed to get charging session: %w", err)
		}

		if session.Status != "active" {
			return fmt.Errorf("charging session is not active")
		}

		if time.Now().Before(session.EndTime) {
			return fmt.Errorf("charging is not yet complete")
		}

		// Update battery in inventory if battery_instance_id is provided
		if session.BatteryInstanceID != nil {
			// Update battery charge to 100% in inventory
			if err := tx.Exec(`
				UPDATE gameplay.inventory_items 
				SET properties = jsonb_set(COALESCE(properties,'{}'::jsonb), '{charge_pct}', '100'::jsonb, true),
				    updated_at = NOW()
				WHERE id = ? AND user_id = ?
			`, *session.BatteryInstanceID, userID).Error; err != nil {
				return fmt.Errorf("failed to update battery charge: %w", err)
			}
		}

		// Update session
		session.Status = "completed"
		session.Progress = 100.0

		if err := tx.Save(&session).Error; err != nil {
			return fmt.Errorf("failed to update charging session: %w", err)
		}

		// Update XP
		if err := s.updateLaboratoryXP(tx, userID, "battery_charging", 10); err != nil {
			return fmt.Errorf("failed to update XP: %w", err)
		}

		return nil
	})
}

// =============================================
// 6. TASK SYSTEM (Level 1+)
// =============================================

// GetAvailableTasks returns available tasks for user
func (s *Service) GetAvailableTasks(userID uuid.UUID) ([]LaboratoryTask, error) {
	var tasks []LaboratoryTask
	if err := s.db.Where("user_id = ? AND status = 'active' AND expires_at > ?", userID, time.Now()).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get available tasks: %w", err)
	}
	return tasks, nil
}

// UpdateTaskProgress updates progress for a specific task
func (s *Service) UpdateTaskProgress(userID uuid.UUID, taskID uuid.UUID, progress int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var task LaboratoryTask
		if err := tx.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.Status != "active" {
			return fmt.Errorf("task is not active")
		}

		// Update progress
		task.CurrentProgress = progress

		// Check if task is completed
		if task.CurrentProgress >= task.TargetValue {
			task.Status = "completed"
			task.CompletedAt = &[]time.Time{time.Now()}[0]
		}

		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}

		return nil
	})
}

// ClaimTaskReward claims reward for completed task
func (s *Service) ClaimTaskReward(userID uuid.UUID, taskID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var task LaboratoryTask
		if err := tx.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.Status != "completed" {
			return fmt.Errorf("task is not completed")
		}

		if task.ClaimedAt != nil {
			return fmt.Errorf("task reward already claimed")
		}

		// TODO: Add rewards to user (credits, XP, materials)
		// This would require integration with user/inventory systems

		// Update task
		task.Status = "claimed"
		task.ClaimedAt = &[]time.Time{time.Now()}[0]

		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}

		// Update XP
		xpType := fmt.Sprintf("task_%s", task.TaskType)
		if err := s.updateLaboratoryXP(tx, userID, xpType, task.RewardXP); err != nil {
			return fmt.Errorf("failed to update XP: %w", err)
		}

		return nil
	})
}

// =============================================
// 6. LABORATORY PLACEMENT & MAP FUNCTIONS - DISABLED
// =============================================

// PlaceLaboratory places laboratory on map at specified location - DISABLED
/*
func (s *Service) PlaceLaboratory(userID uuid.UUID, req *PlaceLaboratoryRequest) (*PlaceLaboratoryResponse, error) {
	var result *PlaceLaboratoryResponse

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get or create laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new laboratory
				lab = Laboratory{
					UserID:             userID,
					Level:              1,
					BaseChargingSlots:  1,
					ExtraChargingSlots: 0,
					ResearchUnlocked:   false,
					CraftingUnlocked:   false,
					IsPlaced:           false,
				}
				if err := tx.Create(&lab).Error; err != nil {
					return fmt.Errorf("failed to create laboratory: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get laboratory: %w", err)
			}
		}

		// Check if already placed
		if lab.IsPlaced {
			return fmt.Errorf("laboratory is already placed on map")
		}

		// Check for nearby laboratories (minimum 50m distance)
		var nearbyCount int64
		if err := tx.Model(&Laboratory{}).
			Where("is_placed = true AND ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
				req.Longitude, req.Latitude, 50). // 50 meters minimum distance
			Count(&nearbyCount).Error; err != nil {
			return fmt.Errorf("failed to check nearby laboratories: %w", err)
		}

		if nearbyCount > 0 {
			return fmt.Errorf("location is too close to another laboratory (minimum 50m distance required)")
		}

		// Update laboratory location
		now := time.Now()
		updates := map[string]interface{}{
			"location":           fmt.Sprintf("POINT(%f %f)", req.Longitude, req.Latitude),
			"location_latitude":  req.Latitude,
			"location_longitude": req.Longitude,
			"is_placed":          true,
			"placed_at":          &now,
			"relocation_count":   0, // First placement is free
		}

		if err := tx.Model(&lab).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to place laboratory: %w", err)
		}

		// Update local lab object
		lab.LocationLatitude = &req.Latitude
		lab.LocationLongitude = &req.Longitude
		lab.IsPlaced = true
		lab.PlacedAt = &now

		result = &PlaceLaboratoryResponse{
			Success:    true,
			Laboratory: &lab,
			Message:    "Laboratory placed successfully on map",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
*/

// RelocateLaboratory relocates laboratory to new location for essence cost - DISABLED
/*
func (s *Service) RelocateLaboratory(userID uuid.UUID, req *RelocateLaboratoryRequest) (*RelocateLaboratoryResponse, error) {
	var result *RelocateLaboratoryResponse

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get current laboratory
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory: %w", err)
		}

		if !lab.IsPlaced {
			return fmt.Errorf("laboratory must be placed before relocation")
		}

		// Check for nearby laboratories (minimum 50m distance)
		var nearbyCount int64
		if err := tx.Model(&Laboratory{}).
			Where("is_placed = true AND id != ? AND ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
				lab.ID, req.Longitude, req.Latitude, 50). // 50 meters minimum distance, exclude current lab
			Count(&nearbyCount).Error; err != nil {
			return fmt.Errorf("failed to check nearby laboratories: %w", err)
		}

		if nearbyCount > 0 {
			return fmt.Errorf("location is too close to another laboratory (minimum 50m distance required)")
		}

		// Calculate essence cost (progressive increase with cap at 5000)
		relocationCost := s.getRelocationCost(lab.RelocationCount)

		// TODO: Check if user has enough essence and deduct it
		// This would require integration with user/inventory systems

		// Store old location for history
		oldLatitude := lab.LocationLatitude
		oldLongitude := lab.LocationLongitude

		// Update laboratory location
		now := time.Now()
		newRelocationCount := lab.RelocationCount + 1
		updates := map[string]interface{}{
			"location":           fmt.Sprintf("POINT(%f %f)", req.Longitude, req.Latitude),
			"location_latitude":  req.Latitude,
			"location_longitude": req.Longitude,
			"relocation_count":   newRelocationCount,
			"last_relocated_at":  &now,
		}

		if err := tx.Model(&lab).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to relocate laboratory: %w", err)
		}

		// Create relocation history record
		relocation := LaboratoryRelocation{
			LaboratoryID:     lab.ID,
			UserID:           userID,
			OldLatitude:      oldLatitude,
			OldLongitude:     oldLongitude,
			NewLatitude:      req.Latitude,
			NewLongitude:     req.Longitude,
			EssenceCost:      relocationCost,
			RelocationReason: "manual",
		}

		if err := tx.Create(&relocation).Error; err != nil {
			return fmt.Errorf("failed to create relocation history: %w", err)
		}

		// Update local lab object
		lab.LocationLatitude = &req.Latitude
		lab.LocationLongitude = &req.Longitude
		lab.RelocationCount = newRelocationCount
		lab.LastRelocatedAt = &now

		result = &RelocateLaboratoryResponse{
			Success:         true,
			Laboratory:      &lab,
			EssenceCost:     relocationCost,
			RelocationCount: newRelocationCount,
			Message:         fmt.Sprintf("Laboratory relocated successfully for %d essence", relocationCost),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetNearbyLaboratories returns laboratories within specified radius - DISABLED
/*
func (s *Service) GetNearbyLaboratories(userID uuid.UUID, req *GetNearbyLaboratoriesRequest) (*GetNearbyLaboratoriesResponse, error) {
	// Default radius if not specified
	radiusM := req.RadiusM
	if radiusM == 0 {
		radiusM = 1000 // 1km default
	}

	// Query nearby laboratories using PostGIS
	query := `
		SELECT
			l.id,
			l.user_id,
			u.username,
			l.level,
			l.location_latitude,
			l.location_longitude,
			ST_Distance(l.location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography) / 1000.0 as distance_km,
			l.placed_at
		FROM laboratory.laboratories l
		JOIN auth.users u ON l.user_id = u.id
		WHERE l.is_placed = true
		AND ST_DWithin(l.location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)
		ORDER BY distance_km ASC
		LIMIT 50
	`

	rows, err := s.db.Raw(query, req.Longitude, req.Latitude, req.Longitude, req.Latitude, radiusM).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query nearby laboratories: %w", err)
	}
	defer rows.Close()

	var laboratories []LaboratoryMarker
	for rows.Next() {
		var marker LaboratoryMarker
		var distanceKm float64

		if err := rows.Scan(
			&marker.ID,
			&marker.UserID,
			&marker.Username,
			&marker.Level,
			&marker.Latitude,
			&marker.Longitude,
			&distanceKm,
			&marker.PlacedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan laboratory marker: %w", err)
		}

		marker.DistanceKm = distanceKm
		marker.IsOwn = marker.UserID == userID
		marker.CanInteract = marker.IsOwn // Only own laboratory can be interacted with
		marker.Icon = s.getLaboratoryIcon(marker.Level, marker.IsOwn)

		laboratories = append(laboratories, marker)
	}

	return &GetNearbyLaboratoriesResponse{
		Success:      true,
		Laboratories: laboratories,
		Total:        len(laboratories),
	}, nil
}
*/

// getLaboratoryIcon returns appropriate icon for laboratory level and ownership
func (s *Service) getLaboratoryIcon(level int, isOwn bool) string {
	if isOwn {
		switch level {
		case 1:
			return "laboratory_own_level1"
		case 2:
			return "laboratory_own_level2"
		case 3:
			return "laboratory_own_level3"
		default:
			return "laboratory_own"
		}
	} else {
		switch level {
		case 1:
			return "laboratory_other_level1"
		case 2:
			return "laboratory_other_level2"
		case 3:
			return "laboratory_other_level3"
		default:
			return "laboratory_other"
		}
	}
}

// getRelocationCost calculates essence cost for laboratory relocation - DISABLED
// First placement is FREE, relocations cost: 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000 (max)
/*
func (s *Service) getRelocationCost(relocationCount int) int {
	// First placement is free (relocationCount = 0)
	if relocationCount == 0 {
		return 0
	}

	// Progressive cost structure for relocations (relocationCount 1+)
	costs := []int{500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000}

	// If relocation count is within the progressive range
	if relocationCount <= len(costs) {
		return costs[relocationCount-1] // -1 because array is 0-indexed but relocationCount starts at 1
	}

	// After 10 relocations, cost stays at 5000
	return 5000
}
*/

// =============================================
// 7. HELPER FUNCTIONS
// =============================================

// updateLaboratoryFeaturesFromPlayerLevel updates laboratory features based on player level
func (s *Service) updateLaboratoryFeaturesFromPlayerLevel(lab *Laboratory, playerLevel int) error {
	updates := map[string]interface{}{}

	// Level 2: Unlock research
	if playerLevel >= 2 && !lab.ResearchUnlocked {
		updates["research_unlocked"] = true
		updates["base_charging_slots"] = 2
	}

	// Level 3: Unlock crafting
	if playerLevel >= 3 && !lab.CraftingUnlocked {
		updates["crafting_unlocked"] = true
		updates["base_charging_slots"] = 3
	}

	// Update laboratory level to match player level
	labLevel := s.xpHandler.GetUserLaboratoryLevel(playerLevel)
	if labLevel != lab.Level {
		updates["level"] = labLevel
	}

	if len(updates) > 0 {
		if err := s.db.Model(lab).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update laboratory features: %w", err)
		}

		// Update local lab object
		for key, value := range updates {
			switch key {
			case "research_unlocked":
				lab.ResearchUnlocked = value.(bool)
			case "crafting_unlocked":
				lab.CraftingUnlocked = value.(bool)
			case "base_charging_slots":
				lab.BaseChargingSlots = value.(int)
			case "level":
				lab.Level = value.(int)
			}
		}
	}

	return nil
}

// updateLaboratoryXP updates player XP through integrated XP system
func (s *Service) updateLaboratoryXP(tx *gorm.DB, userID uuid.UUID, xpType string, amount int) error {
	// Use integrated XP system
	result, err := s.xpHandler.AwardLaboratoryXP(userID, xpType, amount)
	if err != nil {
		return fmt.Errorf("failed to award laboratory XP: %w", err)
	}

	// Type assert result to get level up info
	if xpResult, ok := result.(*XPResult); ok && xpResult.LevelUp {
		var lab Laboratory
		if err := tx.Where("user_id = ?", userID).First(&lab).Error; err != nil {
			return fmt.Errorf("failed to get laboratory for level up: %w", err)
		}

		if err := s.updateLaboratoryFeaturesFromPlayerLevel(&lab, xpResult.CurrentLevel); err != nil {
			return fmt.Errorf("failed to update laboratory features after level up: %w", err)
		}
	}

	return nil
}

// getCraftingLevel vráti crafting level string na základe recipe levelu
func getCraftingLevel(recipeLevel int) string {
	switch {
	case recipeLevel >= 3:
		return "expert"
	case recipeLevel >= 2:
		return "advanced"
	default:
		return "basic"
	}
}

// =============================================
// 8. JSONB IMPLEMENTATION (moved to models.go)
// =============================================
