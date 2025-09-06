package xp

import (
	"fmt"
	"geoanomaly/internal/auth"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// ✅ MAIN: Award XP for artifact collection
func (h *Handler) AwardArtifactXP(userID uuid.UUID, rarity, biome string, zoneTier int) (*XPResult, error) {
	// Get current user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	// Calculate XP
	breakdown := h.calculateArtifactXP(rarity, biome, zoneTier)
	xpGained := breakdown.BaseXP + breakdown.RarityBonus + breakdown.BiomeBonus + breakdown.TierBonus

	oldLevel := user.Level
	oldXP := user.XP
	newXP := oldXP + xpGained

	// Update user XP
	if err := h.db.Model(&user).Update("xp", newXP).Error; err != nil {
		return nil, err
	}

	// Check for level up
	newLevel := h.getLevelFromXP(newXP)
	levelUp := newLevel > oldLevel

	var levelUpInfo *LevelUpInfo
	if levelUp {
		// Update user level
		if err := h.db.Model(&user).Update("level", newLevel).Error; err != nil {
			return nil, err
		}

		levelUpInfo = &LevelUpInfo{
			OldLevel:    oldLevel,
			NewLevel:    newLevel,
			Rewards:     []string{"Level progression unlocked"},
			LevelUpTime: time.Now().Unix(),
		}

		log.Printf("🎉 LEVEL UP! User %s: %d → %d (XP: %d → %d)", userID, oldLevel, newLevel, oldXP, newXP)
	}

	result := &XPResult{
		XPGained:     xpGained,
		TotalXP:      newXP,
		CurrentLevel: newLevel,
		LevelUp:      levelUp,
		Breakdown:    breakdown,
		LevelUpInfo:  levelUpInfo,
	}

	return result, nil
}

// Calculate XP for artifact
func (h *Handler) calculateArtifactXP(rarity, biome string, zoneTier int) XPBreakdown {
	breakdown := XPBreakdown{
		BaseXP: 10, // Base 10 XP per artifact
	}

	// Rarity bonus
	switch rarity {
	case "common":
		breakdown.RarityBonus = 0
	case "rare":
		breakdown.RarityBonus = 5
	case "epic":
		breakdown.RarityBonus = 15
	case "legendary":
		breakdown.RarityBonus = 30
	}

	// Biome danger bonus
	switch biome {
	case "forest":
		breakdown.BiomeBonus = 0
	case "mountain", "urban", "water":
		breakdown.BiomeBonus = 5
	case "industrial":
		breakdown.BiomeBonus = 10
	case "radioactive", "chemical":
		breakdown.BiomeBonus = 15
	}

	// Zone tier bonus
	breakdown.TierBonus = zoneTier * 3

	return breakdown
}

// Get level from XP using level_definitions table
func (h *Handler) getLevelFromXP(totalXP int) int {
	var levelDefs []struct {
		Level      int `json:"level"`
		XPRequired int `json:"xp_required"`
	}

	if err := h.db.Table("level_definitions").
		Order("level DESC").
		Find(&levelDefs).Error; err != nil {
		return 1 // Fallback
	}

	// Find highest level player qualifies for
	for _, levelDef := range levelDefs {
		if totalXP >= levelDef.XPRequired {
			return levelDef.Level
		}
	}

	return 1 // Fallback to level 1
}

// =============================================
// LABORATORY XP SYSTEM INTEGRATION
// =============================================

// LaboratoryXPSources definuje XP hodnoty pre laboratory aktivity
var LaboratoryXPSources = map[string]int{
	"research_basic":    25,  // Basic research
	"research_advanced": 50,  // Advanced research
	"research_expert":   100, // Expert research
	"crafting_basic":    15,  // Basic crafting
	"crafting_advanced": 30,  // Advanced crafting
	"crafting_expert":   60,  // Expert crafting
	"task_daily":        20,  // Daily task completion
	"task_weekly":       50,  // Weekly task completion
	"task_monthly":      150, // Monthly task completion
	"battery_charging":  10,  // Battery charging
}

// AwardLaboratoryXP pridáva XP z laboratory aktivít do player XP systému
func (h *Handler) AwardLaboratoryXP(userID uuid.UUID, activity string, amount int) (interface{}, error) {
	// Get current user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	// Validate activity
	if amount <= 0 {
		return nil, fmt.Errorf("invalid XP amount: %d", amount)
	}

	oldLevel := user.Level
	oldXP := user.XP
	newXP := oldXP + amount

	// Update user XP
	if err := h.db.Model(&user).Update("xp", newXP).Error; err != nil {
		return nil, err
	}

	// Check for level up
	newLevel := h.getLevelFromXP(newXP)
	levelUp := newLevel > oldLevel

	var levelUpInfo *LevelUpInfo
	if levelUp {
		// Update user level
		if err := h.db.Model(&user).Update("level", newLevel).Error; err != nil {
			return nil, err
		}

		levelUpInfo = &LevelUpInfo{
			OldLevel:    oldLevel,
			NewLevel:    newLevel,
			Rewards:     h.getLaboratoryLevelUpRewards(newLevel),
			LevelUpTime: time.Now().Unix(),
		}

		log.Printf("🎉 LABORATORY LEVEL UP! User %s: %d → %d (XP: %d → %d) from %s",
			userID, oldLevel, newLevel, oldXP, newXP, activity)
	}

	// Create laboratory-specific breakdown
	breakdown := XPBreakdown{
		BaseXP:      amount,
		RarityBonus: 0,
		BiomeBonus:  0,
		TierBonus:   0,
	}

	result := &XPResult{
		XPGained:     amount,
		TotalXP:      newXP,
		CurrentLevel: newLevel,
		LevelUp:      levelUp,
		Breakdown:    breakdown,
		LevelUpInfo:  levelUpInfo,
	}

	return result, nil
}

// GetLaboratoryXPAmount vráti XP množstvo pre danú aktivitu
func (h *Handler) GetLaboratoryXPAmount(activity string) int {
	if amount, exists := LaboratoryXPSources[activity]; exists {
		return amount
	}
	return 0
}

// getLaboratoryLevelUpRewards vráti rewards pre laboratory level up
func (h *Handler) getLaboratoryLevelUpRewards(newLevel int) []string {
	rewards := []string{"Level progression unlocked"}

	switch newLevel {
	case 2:
		rewards = append(rewards, "Research system unlocked", "2 charging slots available")
	case 3:
		rewards = append(rewards, "Crafting system unlocked", "3 charging slots available")
	case 5:
		rewards = append(rewards, "Advanced laboratory features unlocked")
	case 10:
		rewards = append(rewards, "Master laboratory features unlocked")
	}

	return rewards
}

// GetUserLaboratoryLevel vráti laboratórny level na základe player levelu
func (h *Handler) GetUserLaboratoryLevel(playerLevel int) int {
	switch {
	case playerLevel >= 3:
		return 3 // Crafting unlocked
	case playerLevel >= 2:
		return 2 // Research unlocked
	default:
		return 1 // Basic laboratory
	}
}

// CanUnlockLaboratoryFeature kontroluje či môže hráč odomknúť laboratórnu funkciu
func (h *Handler) CanUnlockLaboratoryFeature(playerLevel int, feature string) bool {
	switch feature {
	case "research":
		return playerLevel >= 2
	case "crafting":
		return playerLevel >= 3
	default:
		return false
	}
}
