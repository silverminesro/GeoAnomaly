package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
)

// Zone management functions
func (h *Handler) getExistingZonesInArea(lat, lng, radiusMeters float64) []common.Zone {
	var zones []common.Zone

	if err := h.db.Where("is_active = true").Find(&zones).Error; err != nil {
		log.Printf("❌ Failed to query zones: %v", err)
		return []common.Zone{}
	}

	// Manual distance filtering
	var filteredZones []common.Zone
	for _, zone := range zones {
		distance := CalculateDistance(lat, lng, zone.Location.Latitude, zone.Location.Longitude)
		if distance <= radiusMeters {
			filteredZones = append(filteredZones, zone)
		}
	}

	log.Printf("📍 Found %d zones in area (radius: %.0fm)", len(filteredZones), radiusMeters)
	return filteredZones
}

// Filter zones by tier based on user tier
// This function ensures that users only see zones they are allowed to enter based on their tier
func (h *Handler) filterZonesByTier(zones []common.Zone, userTier int) []common.Zone {
	var maxVisibleTier int
	switch userTier {
	case 0:
		maxVisibleTier = 2
	case 1, 2:
		maxVisibleTier = 3
	case 3, 4:
		maxVisibleTier = 4
	default:
		maxVisibleTier = userTier
	}
	var visibleZones []common.Zone
	for _, zone := range zones {
		if zone.TierRequired <= maxVisibleTier {
			visibleZones = append(visibleZones, zone)
		}
	}
	log.Printf("🔍 Filtered zones: %d visible out of %d total (user tier: %d, max visible: %d)", len(visibleZones), len(zones), userTier, maxVisibleTier)
	return visibleZones
}

func (h *Handler) countDynamicZonesInArea(lat, lng, radiusMeters float64) int {
	zones := h.getExistingZonesInArea(lat, lng, radiusMeters)
	count := 0
	for _, zone := range zones {
		if zone.ZoneType == "dynamic" {
			count++
		}
	}
	return count
}

func (h *Handler) calculateMaxZones(playerTier int) int {
	switch playerTier {
	case 0:
		return 1 // Free - 1 zóna len
	case 1:
		return 2 // Basic - 2 zóny
	case 2:
		return 3 // Premium - 3 zóny
	case 3:
		return 5 // Pro - 5 zón
	case 4:
		return 7 // Elite - 7 zón
	default:
		return 1
	}
}

func (h *Handler) buildZoneDetails(zone common.Zone, playerLat, playerLng float64, playerTier int) ZoneWithDetails {
	distance := CalculateDistance(playerLat, playerLng, zone.Location.Latitude, zone.Location.Longitude)

	// Počet aktívnych items
	var artifactCount, gearCount int64
	h.db.Model(&common.Artifact{}).Where("zone_id = ? AND is_active = true", zone.ID).Count(&artifactCount)
	h.db.Model(&common.Gear{}).Where("zone_id = ? AND is_active = true", zone.ID).Count(&gearCount)

	// Počet aktívnych hráčov
	var playerCount int64
	h.db.Model(&common.PlayerSession{}).Where("current_zone = ? AND is_online = true AND last_seen > ?", zone.ID, time.Now().Add(-5*time.Minute)).Count(&playerCount)

	details := ZoneWithDetails{
		Zone:            zone,
		DistanceMeters:  distance,
		CanEnter:        playerTier >= zone.TierRequired,
		ActiveArtifacts: int(artifactCount),
		ActiveGear:      int(gearCount),
		ActivePlayers:   int(playerCount),
		Biome:           zone.Biome,
		DangerLevel:     zone.DangerLevel,
	}

	// ✅ NEW: TTL info for zones with ExpiresAt
	if zone.ExpiresAt != nil {
		expiry := zone.ExpiresAt.Unix()
		details.ExpiresAt = &expiry

		timeLeft := zone.TimeUntilExpiry()
		if timeLeft > 0 {
			timeLeftStr := FormatDuration(timeLeft)
			details.TimeToExpiry = &timeLeftStr
		}
	}

	return details
}

// ✅ NEW: Tier-based zone radius calculation with random ranges
func (h *Handler) getTierZoneRadius(zoneTier int) (float64, float64) {
	switch zoneTier {
	case 0:
		return Tier0MinRadius, Tier0MaxRadius
	case 1:
		return Tier1MinRadius, Tier1MaxRadius
	case 2:
		return Tier2MinRadius, Tier2MaxRadius
	case 3:
		return Tier3MinRadius, Tier3MaxRadius
	case 4:
		return Tier4MinRadius, Tier4MaxRadius
	default:
		return Tier0MinRadius, Tier0MaxRadius
	}
}

// ✅ UPDATED: calculateZoneRadius now uses random ranges instead of fixed values
func (h *Handler) calculateZoneRadius(tier int) int {
	minRadius, maxRadius := h.getTierZoneRadius(tier)

	// Random radius within tier range
	randomRadius := minRadius + rand.Float64()*(maxRadius-minRadius)

	log.Printf("🏗️ [TIER %d] Zone radius: %.0fm (range: %.0f-%.0fm)",
		tier, randomRadius, minRadius, maxRadius)

	return int(randomRadius)
}

func (h *Handler) generateRandomPosition(centerLat, centerLng, radiusMeters float64) (float64, float64) {
	angle := rand.Float64() * 2 * math.Pi
	distance := rand.Float64() * radiusMeters
	earthRadius := 6371000.0

	latOffset := (distance * math.Cos(angle)) / earthRadius * (180 / math.Pi)
	lngOffset := (distance * math.Sin(angle)) / earthRadius * (180 / math.Pi) / math.Cos(centerLat*math.Pi/180)

	return centerLat + latOffset, centerLng + lngOffset
}

// ✅ SIMPLIFIED: Keep only essential functions, remove duplicates
func (h *Handler) getAvailableBiomes(playerTier int) []string {
	biomes := []string{BiomeForest} // Forest always available

	if playerTier >= 1 {
		biomes = append(biomes, BiomeMountain, BiomeUrban, BiomeWater)
	}
	if playerTier >= 2 {
		biomes = append(biomes, BiomeIndustrial) //BiomeNight
	}
	if playerTier >= 3 {
		biomes = append(biomes, BiomeRadioactive)
	}
	if playerTier >= 4 {
		biomes = append(biomes, BiomeChemical)
	}

	return biomes
}

// Nahradíš pôvodnú calculateZoneTier touto verziou:
func (h *Handler) calculateZoneTier(playerTier, biomeMinTier int) int {
	r := rand.Float64()

	switch playerTier {
	case 0:
		// 55% na tier 0, 45% na tier 1
		if biomeMinTier > 1 {
			return biomeMinTier
		}
		if r < 0.55 {
			return max(biomeMinTier, 0)
		}
		return max(biomeMinTier, 1)
	case 1:
		// 50% na tier 0, 45% na tier 1, 5% na tier 2
		if r < 0.50 {
			return max(biomeMinTier, 0)
		} else if r < 0.95 {
			return max(biomeMinTier, 1)
		}
		return max(biomeMinTier, 2)
	case 2:
		// 30% na tier 0, 40% na tier 1, 15% na tier 2, 15% na tier 3
		if r < 0.30 {
			return max(biomeMinTier, 0)
		} else if r < 0.70 {
			return max(biomeMinTier, 1)
		} else if r < 0.85 {
			return max(biomeMinTier, 2)
		}
		return max(biomeMinTier, 3)
	case 3:
		// 20% na tier 0, 20% na tier 1, 25% na tier 2, 35% na tier 3
		if r < 0.20 {
			return max(biomeMinTier, 0)
		} else if r < 0.40 {
			return max(biomeMinTier, 1)
		} else if r < 0.65 {
			return max(biomeMinTier, 2)
		}
		return max(biomeMinTier, 3)
	case 4:
		// 15% na tier 0, 15% na tier 1, 20% na tier 2, 25% na tier 3, 25% na tier 4
		if r < 0.15 {
			return max(biomeMinTier, 0)
		} else if r < 0.30 {
			return max(biomeMinTier, 1)
		} else if r < 0.50 {
			return max(biomeMinTier, 2)
		} else if r < 0.75 {
			return max(biomeMinTier, 3)
		}
		return max(biomeMinTier, 4)
	default:
		return biomeMinTier
	}
}

// Helper, pridaj hore do zones.go:
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (h *Handler) getZoneCategory(tier int) string {
	switch tier {
	case 0, 1:
		return "basic"
	case 2, 3:
		return "premium"
	case 4:
		return "elite"
	default:
		return "basic"
	}
}

// Keep existing biome-specific spawning functions
func (h *Handler) spawnSpecificArtifact(zoneID uuid.UUID, artifactType, biome string, tier int) error {
	var zone common.Zone
	if err := h.db.First(&zone, "id = ?", zoneID).Error; err != nil {
		return err
	}

	displayName := GetArtifactDisplayName(artifactType)
	rarity := GetArtifactRarity(artifactType, tier)

	lat, lng := h.generateRandomPosition(zone.Location.Latitude, zone.Location.Longitude, float64(zone.RadiusMeters))

	artifact := common.Artifact{
		BaseModel: common.BaseModel{ID: uuid.New()},
		ZoneID:    zoneID,
		Name:      displayName,
		Type:      artifactType,
		Rarity:    rarity,
		Biome:     biome,
		Location: common.Location{
			Latitude:  lat,
			Longitude: lng,
			Timestamp: time.Now(),
		},
		Properties: common.JSONB{
			"spawn_time":   time.Now().Unix(),
			"spawner":      "biome_specific",
			"zone_tier":    tier,
			"biome":        biome,
			"spawn_reason": "zone_creation",
		},
		IsActive: true,
	}

	return h.db.Create(&artifact).Error
}

func (h *Handler) spawnSpecificGear(zoneID uuid.UUID, gearType, biome string, tier int) error {
	var zone common.Zone
	if err := h.db.First(&zone, "id = ?", zoneID).Error; err != nil {
		return err
	}

	// Najdi gear kategóriu v databáze
	var gearCategory common.GearCategory
	if err := h.db.Where("id = ? AND is_active = ?", gearType, true).First(&gearCategory).Error; err != nil {
		// Ak sa kategória nenájde, použij fallback
		log.Printf("⚠️ Gear category %s not found in database, using fallback", gearType)
		return h.spawnSpecificGearFallback(zoneID, gearType, biome, tier)
	}

	// Skontroluj tier requirements
	if gearCategory.Level > tier {
		log.Printf("⚠️ Gear category %s requires tier %d, but zone is tier %d", gearType, gearCategory.Level, tier)
		return fmt.Errorf("gear level too high for zone tier")
	}

	// Skontroluj biome requirements
	if gearCategory.Biome != "all" && gearCategory.Biome != biome {
		log.Printf("⚠️ Gear category %s is for biome %s, but zone is %s", gearType, gearCategory.Biome, biome)
		return fmt.Errorf("gear biome mismatch")
	}

	lat, lng := h.generateRandomPosition(zone.Location.Latitude, zone.Location.Longitude, float64(zone.RadiusMeters))

	gear := common.Gear{
		BaseModel: common.BaseModel{ID: uuid.New()},
		ZoneID:    zoneID,
		Name:      gearCategory.Name,
		Type:      gearCategory.ID, // Použijeme ID kategórie ako type
		Level:     gearCategory.Level,
		Biome:     biome,
		Location: common.Location{
			Latitude:  lat,
			Longitude: lng,
			Timestamp: time.Now(),
		},
		Properties: common.JSONB{
			"spawn_time":         time.Now().Unix(),
			"spawner":            "biome_specific",
			"zone_tier":          tier,
			"biome":              biome,
			"spawn_reason":       "zone_creation",
			"category_id":        gearCategory.ID,
			"slot":               gearCategory.SlotID,
			"rarity":             gearCategory.Rarity,
			"base_durability":    gearCategory.BaseDurability,
			"zombie_resistance":  gearCategory.BaseZombieResistance,
			"bandit_resistance":  gearCategory.BaseBanditResistance,
			"soldier_resistance": gearCategory.BaseSoldierResistance,
			"monster_resistance": gearCategory.BaseMonsterResistance,
		},
		IsActive: true,
	}

	return h.db.Create(&gear).Error
}

// Fallback funkcia pre staré gear types
func (h *Handler) spawnSpecificGearFallback(zoneID uuid.UUID, gearType, biome string, tier int) error {
	var zone common.Zone
	if err := h.db.First(&zone, "id = ?", zoneID).Error; err != nil {
		return err
	}

	displayName := GetGearDisplayName(gearType)
	level := tier + rand.Intn(2) + 1

	lat, lng := h.generateRandomPosition(zone.Location.Latitude, zone.Location.Longitude, float64(zone.RadiusMeters))

	gear := common.Gear{
		BaseModel: common.BaseModel{ID: uuid.New()},
		ZoneID:    zoneID,
		Name:      displayName,
		Type:      gearType,
		Level:     level,
		Biome:     biome,
		Location: common.Location{
			Latitude:  lat,
			Longitude: lng,
			Timestamp: time.Now(),
		},
		Properties: common.JSONB{
			"spawn_time":   time.Now().Unix(),
			"spawner":      "biome_specific",
			"zone_tier":    tier,
			"biome":        biome,
			"spawn_reason": "zone_creation",
		},
		IsActive: true,
	}

	return h.db.Create(&gear).Error
}
