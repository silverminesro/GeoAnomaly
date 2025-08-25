package game

import (
	"fmt"
	"log"

	"geoanomaly/internal/common"
	"geoanomaly/internal/scanner"

	"github.com/google/uuid"
)

// Main filtering functions that combine artifact and gear filtering
func (h *Handler) addDistanceToItems(artifacts []common.Artifact, playerLat, playerLng float64) []map[string]interface{} {
	var result []map[string]interface{}

	for _, artifact := range artifacts {
		distance := CalculateDistance(playerLat, playerLng, artifact.Location.Latitude, artifact.Location.Longitude)

		item := map[string]interface{}{
			"id":              artifact.ID,
			"name":            artifact.Name,
			"type":            artifact.Type,
			"rarity":          artifact.Rarity,
			"biome":           artifact.Biome,
			"location":        artifact.Location,
			"properties":      artifact.Properties,
			"is_active":       artifact.IsActive,
			"distance_meters": distance,
			"created_at":      artifact.CreatedAt,
		}
		result = append(result, item)
	}

	return result
}

func (h *Handler) addDistanceToGear(gear []common.Gear, playerLat, playerLng float64) []map[string]interface{} {
	var result []map[string]interface{}

	for _, g := range gear {
		distance := CalculateDistance(playerLat, playerLng, g.Location.Latitude, g.Location.Longitude)

		item := map[string]interface{}{
			"id":              g.ID,
			"name":            g.Name,
			"type":            g.Type,
			"level":           g.Level,
			"biome":           g.Biome,
			"location":        g.Location,
			"properties":      g.Properties,
			"is_active":       g.IsActive,
			"distance_meters": distance,
			"created_at":      g.CreatedAt,
		}
		result = append(result, item)
	}

	return result
}

// ✅ Enhanced tier checking with detailed logging
func (h *Handler) CheckUserCanCollectItem(userTier int, itemType, itemID string) (bool, string) {
	switch itemType {
	case "artifact":
		var artifact common.Artifact
		if err := h.db.First(&artifact, "id = ?", itemID).Error; err != nil {
			return false, "Artifact not found"
		}

		if !h.canCollectArtifact(artifact, userTier) {
			requiredTier := h.getRequiredTierForRarity(artifact.Rarity)
			log.Printf("🚫 User tier %d cannot collect %s artifact (requires tier %d)", userTier, artifact.Rarity, requiredTier)
			return false, fmt.Sprintf("Requires tier %d to collect %s artifacts", requiredTier, artifact.Rarity)
		}

		if !h.canAccessBiome(artifact.Biome, userTier) {
			log.Printf("🚫 User tier %d cannot access %s biome", userTier, artifact.Biome)
			return false, fmt.Sprintf("Requires higher tier to access %s biome", artifact.Biome)
		}

		return true, "OK"

	case "gear":
		var gear common.Gear
		if err := h.db.First(&gear, "id = ?", itemID).Error; err != nil {
			return false, "Gear not found"
		}

		maxLevel := h.gearService.getMaxGearLevelForTier(userTier)
		if gear.Level > maxLevel {
			log.Printf("🚫 User tier %d cannot collect level %d gear (max level %d)", userTier, gear.Level, maxLevel)
			return false, fmt.Sprintf("Requires higher tier to collect level %d gear", gear.Level)
		}

		if !h.canAccessBiome(gear.Biome, userTier) {
			log.Printf("🚫 User tier %d cannot access %s biome", userTier, gear.Biome)
			return false, fmt.Sprintf("Requires higher tier to access %s biome", gear.Biome)
		}

		return true, "OK"

	default:
		return false, "Invalid item type"
	}
}

// ✅ PRIDANÉ: Scanner collection validation
func (h *Handler) CheckScannerCanCollectItem(userID uuid.UUID, itemType, itemID string) (bool, string) {
	// Get user's scanner instance
	var scannerInstance scanner.ScannerInstance
	if err := h.db.Where("owner_id = ? AND is_active = true", userID).First(&scannerInstance).Error; err != nil {
		return false, "No active scanner equipped"
	}

	// Get scanner catalog details
	var scannerCatalog scanner.ScannerCatalog
	if err := h.db.Where("code = ?", scannerInstance.ScannerCode).First(&scannerCatalog).Error; err != nil {
		return false, "Scanner configuration not found"
	}

	// Get item details
	var itemRarity string
	switch itemType {
	case "artifact":
		var artifact common.Artifact
		if err := h.db.First(&artifact, "id = ?", itemID).Error; err != nil {
			return false, "Item not found"
		}
		itemRarity = artifact.Rarity
	case "gear":
		itemRarity = "common" // Gear nemá rarity, použijeme default
	default:
		return false, "Invalid item type"
	}

	// Check rarity limits
	rarityLevels := map[string]int{
		"common":    1,
		"uncommon":  2,
		"rare":      3,
		"epic":      4,
		"legendary": 5,
	}

	itemLevel, itemExists := rarityLevels[itemRarity]
	collectLevel, collectExists := rarityLevels[scannerCatalog.CapsJSON.Limits.CollectMaxRarity]

	if !itemExists || !collectExists {
		return false, "Invalid rarity configuration"
	}

	if itemLevel > collectLevel {
		return false, fmt.Sprintf("Scanner can only collect up to %s rarity", scannerCatalog.CapsJSON.Limits.CollectMaxRarity)
	}

	return true, ""
}
