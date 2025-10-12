package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"geoanomaly/internal/auth"
	"geoanomaly/internal/gameplay"
	"geoanomaly/internal/xp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================
// MAIN GAME ENDPOINTS
// ============================================

// ScanArea - hlavný endpoint pre hľadanie zón
func (h *Handler) ScanArea(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req ScanAreaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !IsValidGPSCoordinate(req.Latitude, req.Longitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GPS coordinates"})
		return
	}

	// Get user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Rate limiting check
	rateLimitKey := fmt.Sprintf("scan_area:%s", userID)
	if !h.checkAreaScanRateLimit(rateLimitKey) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":               "Rate limit exceeded",
			"next_scan_in_sec":    AreaScanCooldown * 60,
			"next_scan_available": time.Now().Add(AreaScanCooldown * time.Minute).Unix(),
		})
		return
	}

	// Get existing zones in area (7km visibility)
	existingZones := h.getExistingZonesInArea(req.Latitude, req.Longitude, AreaScanRadius)

	// ====== GARANCIA ZÓN PRE NÍZKE TIERY ======
	newZones := []gameplay.Zone{}
	zonesInSpawnRadius := h.getExistingZonesInArea(req.Latitude, req.Longitude, MaxSpawnRadius)

	if user.Tier == 0 {
		// Garantuj 2x tier 0
		tier0Count := 0
		for _, z := range zonesInSpawnRadius {
			if z.TierRequired == 0 && z.IsActive {
				tier0Count++
			}
		}
		if tier0Count < 2 {
			toSpawn := 2 - tier0Count
			log.Printf("✅ Guaranteeing %d tier 0 zone(s) for tier 0 player (currently %d in area)", toSpawn, tier0Count)
			tier0Zones := h.spawnDynamicZones(req.Latitude, req.Longitude, 0, toSpawn, user.ID)
			newZones = append(newZones, tier0Zones...)
		}
	} else if user.Tier == 1 {
		// Garantuj aspoň 1x tier 0 a 1x tier 1 zónu
		tier0 := false
		tier1 := false
		for _, z := range zonesInSpawnRadius {
			if z.TierRequired == 0 && z.IsActive {
				tier0 = true
			}
			if z.TierRequired == 1 && z.IsActive {
				tier1 = true
			}
		}
		if !tier0 {
			log.Printf("✅ Guaranteeing 1 tier 0 zone for tier 1 player")
			tier0Zones := h.spawnDynamicZones(req.Latitude, req.Longitude, 0, 1, user.ID)
			newZones = append(newZones, tier0Zones...)
		}
		if !tier1 {
			log.Printf("✅ Guaranteeing 1 tier 1 zone for tier 1 player")
			tier1Zones := h.spawnDynamicZones(req.Latitude, req.Longitude, 1, 1, user.ID)
			newZones = append(newZones, tier1Zones...)
		}
	} else if user.Tier == 2 {
		// Ak nie je v okolí žiadna tier 0/1/2, spawn 2 náhodné zóny z {0,1,2}
		tier0, tier1, tier2 := false, false, false
		for _, z := range zonesInSpawnRadius {
			if z.TierRequired == 0 && z.IsActive {
				tier0 = true
			}
			if z.TierRequired == 1 && z.IsActive {
				tier1 = true
			}
			if z.TierRequired == 2 && z.IsActive {
				tier2 = true
			}
		}
		if !(tier0 || tier1 || tier2) {
			log.Printf("✅ Guaranteeing 2 zones (randomly picked from tier 0,1,2) for tier 2 player")
			for i := 0; i < 2; i++ {
				randomTier := rand.Intn(3) // 0, 1 alebo 2
				zones := h.spawnDynamicZones(req.Latitude, req.Longitude, randomTier, 1, user.ID)
				newZones = append(newZones, zones...)
			}
		}
	}

	// Calculate how many new zones can be created (only count zones in spawn radius - 2km)
	maxZones := h.calculateMaxZones(user.Tier)
	currentDynamicZones := h.countDynamicZonesInArea(req.Latitude, req.Longitude, MaxSpawnRadius)
	newZonesNeeded := maxZones - currentDynamicZones

	if newZonesNeeded > 0 {
		log.Printf("🏗️ Creating %d new zones for tier %d player", newZonesNeeded, user.Tier)
		additionalZones := h.spawnDynamicZones(req.Latitude, req.Longitude, user.Tier, newZonesNeeded, user.ID)
		newZones = append(newZones, additionalZones...)
	}

	// Combine all zones
	allZones := append(existingZones, newZones...)

	// Filter zones by tier
	visibleZones := h.filterZonesByTier(allZones, user.Tier)

	// Build detailed zone info
	var zoneDetails []ZoneWithDetails
	for _, zone := range visibleZones {
		details := h.buildZoneDetails(zone, req.Latitude, req.Longitude, user.Tier)
		zoneDetails = append(zoneDetails, details)
	}

	response := ScanAreaResponse{
		ZonesCreated:      len(newZones),
		Zones:             zoneDetails,
		ScanAreaCenter:    LocationPoint(req),
		NextScanAvailable: time.Now().Add(AreaScanCooldown * time.Minute).Unix(),
		MaxZones:          maxZones,
		CurrentZoneCount:  len(visibleZones),
		PlayerTier:        user.Tier,
	}

	c.JSON(http.StatusOK, response)
}

// ✅ UPDATED: spawnDynamicZones with tier-based distance spawning & minimal distance between zones
func (h *Handler) spawnDynamicZones(lat, lng float64, playerTier int, count int, userID uuid.UUID) []gameplay.Zone {
	var newZones []gameplay.Zone

	// Získaj všetky existujúce zóny v maximálnom okruhu spawnu (2km)
	existingZones := h.getExistingZonesInArea(lat, lng, MaxSpawnRadius)

	// Špeciálne správanie pre tier 0 hráča
	var forcedTiers []int
	if playerTier == 0 && count > 0 {
		forcedTiers = append(forcedTiers, 0)
		if count > 1 {
			forcedTiers = append(forcedTiers, 0)
		}
	}

	for i := 0; i < count; i++ {
		var zoneTier int
		var biome string
		var template ZoneTemplate

		if i < len(forcedTiers) {
			zoneTier = forcedTiers[i]
			biome = h.selectBiome(zoneTier)
			template = GetZoneTemplate(biome)
		} else {
			biome = h.selectBiome(playerTier)
			template = GetZoneTemplate(biome)
			zoneTier = h.calculateZoneTier(playerTier, template.MinTierRequired)
		}

		var zoneLat, zoneLng float64
		valid := false
		maxTries := 10

		for try := 0; try < maxTries; try++ {
			zoneLat, zoneLng = h.generateTierBasedPosition(lat, lng, zoneTier)
			tooClose := false

			// Kontrola vzdialenosti voči už existujúcim aj novo spawnutým zónam
			for _, z := range append(existingZones, newZones...) {
				distance := CalculateDistance(zoneLat, zoneLng, z.Location.Latitude, z.Location.Longitude)
				if distance < MinZoneDistance {
					tooClose = true
					break
				}
			}

			if !tooClose {
				valid = true
				break
			}
		}

		if !valid {
			log.Printf("⚠️ Could not find free position for zone after %d tries, skipping spawn.", maxTries)
			continue
		}

		// Over, či pozícia je v rámci max spawn radius
		spawnDistance := CalculateDistance(lat, lng, zoneLat, zoneLng)
		if spawnDistance > MaxSpawnRadius {
			log.Printf("⚠️ Zone would spawn too far (%.0fm > %.0fm), skipping", spawnDistance, MaxSpawnRadius)
			continue
		}

		// TTL podľa tieru / Tier 0 zony načítava hodnoty s constants.go a upravuju ich životnosť
		var randomTTL time.Duration
		if zoneTier == 0 {
			minTTL := time.Duration(Tier0MinExpiryMinutes) * time.Minute
			maxTTL := time.Duration(Tier0MaxExpiryMinutes) * time.Minute
			ttlRange := maxTTL - minTTL
			randomTTL = minTTL + time.Duration(rand.Float64()*float64(ttlRange))
		} else {
			minTTL := time.Duration(ZoneMinExpiryHours) * time.Hour
			maxTTL := time.Duration(ZoneMaxExpiryHours) * time.Hour
			ttlRange := maxTTL - minTTL
			randomTTL = minTTL + time.Duration(rand.Float64()*float64(ttlRange))
		}
		expiresAt := time.Now().Add(randomTTL)

		zone := gameplay.Zone{
			BaseModel: gameplay.BaseModel{ID: uuid.New()},
			Name:      h.generateZoneName(biome),
			Location: gameplay.Location{
				Latitude:  zoneLat,
				Longitude: zoneLng,
				Timestamp: time.Now(),
			},
			TierRequired: zoneTier,
			RadiusMeters: h.calculateZoneRadius(zoneTier),
			IsActive:     true,
			ZoneType:     "dynamic",
			Biome:        biome,
			DangerLevel:  template.DangerLevel,

			// TTL fields
			ExpiresAt:    &expiresAt,
			LastActivity: time.Now(),
			AutoCleanup:  true,

			Properties: gameplay.JSONB{
				"spawned_by":            "scan_area",
				"created_by_user_id":    userID.String(),
				"ttl_hours":             randomTTL.Hours(),
				"biome":                 biome,
				"danger_level":          template.DangerLevel,
				"environmental_effects": template.EnvironmentalEffects,
				"zone_template":         "biome_based",
				"spawn_distance":        spawnDistance,
				"zone_tier":             zoneTier,
				"player_tier":           playerTier,
			},
		}

		if err := h.db.Create(&zone).Error; err == nil {
			h.spawnItemsInZone(zone.ID, zoneTier, zone.Biome, zone.Location, zone.RadiusMeters)
			newZones = append(newZones, zone)

			log.Printf("🏰 Zone spawned: %s (Tier: %d, Biome: %s, Distance: %.0fm, Radius: %dm, TTL: %.1fh)",
				zone.Name, zoneTier, biome, spawnDistance, zone.RadiusMeters, randomTTL.Hours())
		} else {
			log.Printf("❌ Failed to create zone: %v", err)
		}
	}

	return newZones
}

// GetNearbyZones - získanie zón v okolí
func (h *Handler) GetNearbyZones(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latitude"})
		return
	}

	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid longitude"})
		return
	}

	radius, err := strconv.ParseFloat(c.DefaultQuery("radius", "5000"), 64)
	if err != nil || radius > 10000 {
		radius = 5000 // Default 5km
	}

	// Get user tier
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get nearby zones
	zones := h.getExistingZonesInArea(lat, lng, radius)
	visibleZones := h.filterZonesByTier(zones, user.Tier)

	// Build detailed response
	var zoneDetails []ZoneWithDetails
	for _, zone := range visibleZones {
		details := h.buildZoneDetails(zone, lat, lng, user.Tier)
		zoneDetails = append(zoneDetails, details)
	}

	c.JSON(http.StatusOK, gin.H{
		"zones":       zoneDetails,
		"total_zones": len(zoneDetails),
		"scan_center": LocationPoint{Latitude: lat, Longitude: lng},
		"radius":      radius,
		"player_tier": user.Tier,
	})
}

// GetZoneDetails - detaily konkrétnej zóny
func (h *Handler) GetZoneDetails(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	zoneID := c.Param("id")
	if zoneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zone ID required"})
		return
	}

	// Get user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get zone
	var zone gameplay.Zone
	if err := h.db.First(&zone, "id = ? AND is_active = true", zoneID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found"})
		return
	}

	// Tier check
	if zone.TierRequired > user.Tier {
		c.JSON(http.StatusForbidden, gin.H{
			"error":         "Zone not accessible",
			"message":       "Upgrade your tier to access this zone",
			"required_tier": zone.TierRequired,
			"your_tier":     user.Tier,
		})
		return
	}

	// Build detailed response
	details := h.buildZoneDetails(zone, 0, 0, user.Tier)

	// Get all items in zone (filtered by tier)
	var artifacts []gameplay.Artifact
	var gear []gameplay.Gear
	h.db.Where("zone_id = ? AND is_active = true", zone.ID).Find(&artifacts)
	h.db.Where("zone_id = ? AND is_active = true", zone.ID).Find(&gear)

	filteredArtifacts := h.filterArtifactsByTier(artifacts, user.Tier)
	filteredGear := h.gearService.FilterGearByTier(gear, user.Tier)

	c.JSON(http.StatusOK, gin.H{
		"zone":      details,
		"artifacts": filteredArtifacts,
		"gear":      filteredGear,
		"can_enter": user.Tier >= zone.TierRequired,
		"message":   "Zone details retrieved successfully",
	})
}

// ✅ ENHANCED: EnterZone with activity tracking
func (h *Handler) EnterZone(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	zoneID := c.Param("id")
	if zoneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zone ID required"})
		return
	}

	// Get user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get zone
	var zone gameplay.Zone
	if err := h.db.First(&zone, "id = ? AND is_active = true", zoneID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found"})
		return
	}

	// Tier check
	if zone.TierRequired > user.Tier {
		c.JSON(http.StatusForbidden, gin.H{
			"error":         "Insufficient tier level",
			"message":       "Upgrade your tier to enter this zone",
			"required_tier": zone.TierRequired,
			"your_tier":     user.Tier,
		})
		return
	}

	// Update zone activity
	h.updateZoneActivity(zone.ID)

	// Update player session
	var session auth.PlayerSession
	if err := h.db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		// Create new session
		session = auth.PlayerSession{
			UserID:   user.ID,
			IsOnline: true,
			LastSeen: time.Now(),
		}
	}

	// Parse zone UUID
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid zone ID format"})
		return
	}

	session.CurrentZone = &zoneUUID
	session.LastSeen = time.Now()
	session.IsOnline = true

	if err := h.db.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enter zone"})
		return
	}

	// Apply durability damage to equipped gear
	if h.loadoutService != nil {
		// Convert danger level string to int
		dangerLevel := 1 // default
		switch zone.DangerLevel {
		case "low":
			dangerLevel = 1
		case "medium":
			dangerLevel = 3
		case "high":
			dangerLevel = 5
		case "extreme":
			dangerLevel = 8
		case "deadly":
			dangerLevel = 10
		}

		if err := h.loadoutService.ApplyDurabilityDamage(user.ID, dangerLevel, zone.Biome); err != nil {
			log.Printf("Warning: Failed to apply durability damage: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Successfully entered zone",
		"zone_name":            zone.Name,
		"biome":                zone.Biome,
		"danger_level":         zone.DangerLevel,
		"zone":                 zone,
		"entered_at":           time.Now().Unix(),
		"can_collect":          true,
		"player_tier":          user.Tier,
		"distance_from_center": 0,
		"ttl_status":           zone.TTLStatus(),
		"expires_in_seconds":   int64(zone.TimeUntilExpiry().Seconds()),
	})
}

// ExitZone - jednoduchá implementácia
func (h *Handler) ExitZone(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Get player session
	var session auth.PlayerSession
	if err := h.db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player session not found"})
		return
	}

	if session.CurrentZone == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not currently in any zone"})
		return
	}

	// Get zone name before clearing
	var zoneName string = "Unknown Zone"
	if session.CurrentZone != nil {
		var zone gameplay.Zone
		if err := h.db.First(&zone, "id = ?", *session.CurrentZone).Error; err == nil {
			zoneName = zone.Name
		}
	}

	// Calculate basic time in zone
	timeInZone := time.Since(session.CreatedAt)

	// Clear current zone
	session.CurrentZone = nil
	session.LastSeen = time.Now()

	if err := h.db.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exit zone"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Successfully exited zone",
		"exited_at":       time.Now().Unix(),
		"zone_name":       zoneName,
		"time_in_zone":    fmt.Sprintf("%.0fm", timeInZone.Minutes()),
		"items_collected": 0, // TODO: Implement if needed
		"xp_gained":       0, // TODO: Implement if needed
		"total_xp_gained": 0, // TODO: Implement if needed
	})
}

// ScanZone - scan items v zóne
func (h *Handler) ScanZone(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	zoneID := c.Param("id")
	if zoneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zone ID required"})
		return
	}

	// Get user
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get zone
	var zone gameplay.Zone
	if err := h.db.First(&zone, "id = ? AND is_active = true", zoneID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Zone not found"})
		return
	}

	// Tier check
	if zone.TierRequired > user.Tier {
		c.JSON(http.StatusForbidden, gin.H{
			"error":         "Insufficient tier level",
			"required_tier": zone.TierRequired,
			"your_tier":     user.Tier,
		})
		return
	}

	// Check if player is in zone
	var session auth.PlayerSession
	if err := h.db.Where("user_id = ? AND current_zone = ?", userID, zoneID).First(&session).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Not in zone",
			"message": "You must enter the zone first",
		})
		return
	}

	// Update zone activity on scan
	h.updateZoneActivity(zone.ID)

	// Get items in zone (filtered by tier)
	var artifacts []gameplay.Artifact
	var gear []gameplay.Gear

	h.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&artifacts)
	h.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&gear)

	filteredArtifacts := h.filterArtifactsByTier(artifacts, user.Tier)
	filteredGear := h.gearService.FilterGearByTier(gear, user.Tier)

	c.JSON(http.StatusOK, gin.H{
		"zone_name":       zone.Name,
		"zone":            zone,
		"artifacts":       h.addDistanceToItems(filteredArtifacts, session.LastLocationLatitude, session.LastLocationLongitude),
		"gear":            h.addDistanceToGear(filteredGear, session.LastLocationLatitude, session.LastLocationLongitude),
		"total_artifacts": len(filteredArtifacts),
		"total_gear":      len(filteredGear),
		"scan_timestamp":  time.Now().Unix(),
		"message":         "Zone scanned successfully",
		"ttl_status":      zone.TTLStatus(),
		"expires_in":      int64(zone.TimeUntilExpiry().Seconds()),
	})
}

// ✅ ENHANCED CollectItem - zber artefakt/gear s XP systémom + zone activity tracking + distance validation
func (h *Handler) CollectItem(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	zoneID := c.Param("id")

	var req struct {
		ItemType string `json:"item_type" binding:"required"` // "artifact" | "gear"
		ItemID   string `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "bad_request", "error": "Invalid body"})
		return
	}

	// 1) Session + in-zone validácia
	var session auth.PlayerSession
	if err := h.db.Where("user_id = ? AND current_zone = ?", userID, zoneID).First(&session).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "not_in_zone", "error": "You must be inside this zone to collect"})
		return
	}

	// 2) Načítaj item + over zhodu zóny
	var (
		itemLat, itemLng float64
		itemZoneID       uuid.UUID
		itemRarity       string
		itemName         string
	)

	switch req.ItemType {
	case "artifact":
		var a gameplay.Artifact
		if err := h.db.First(&a, "id = ?", req.ItemID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "not_found", "error": "Artifact not found"})
			return
		}
		if a.DeletedAt != nil || !a.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "inactive", "error": "Artifact is no longer available"})
			return
		}
		itemZoneID = a.ZoneID
		itemLat = a.Location.Latitude
		itemLng = a.Location.Longitude
		itemRarity = a.Rarity
		itemName = a.Name

	case "gear":
		var g gameplay.Gear
		if err := h.db.First(&g, "id = ?", req.ItemID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "not_found", "error": "Gear not found"})
			return
		}
		if g.DeletedAt != nil || !g.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "inactive", "error": "Gear is no longer available"})
			return
		}
		itemZoneID = g.ZoneID
		itemLat = g.Location.Latitude
		itemLng = g.Location.Longitude
		// gear nemá rarity – nechaj prázdne; rarity limit sa bude hodnotiť interným pravidlom pre gear (common)
		itemRarity = "common"
		itemName = g.Name

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "bad_request", "error": "Invalid item_type"})
		return
	}

	if itemZoneID.String() != zoneID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "wrong_zone", "error": "Item does not belong to this zone"})
		return
	}

	// 3) Vzdialenosť hráča od itemu (z poslednej session polohy)
	playerLat := session.LastLocationLatitude
	playerLng := session.LastLocationLongitude
	dist := CalculateDistance(playerLat, playerLng, itemLat, itemLng)

	if dist > MaxCollectRadius {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":              false,
			"code":                 "too_far",
			"error":                "Move closer to collect",
			"distance_m":           math.Round(dist*10) / 10,
			"max_collect_radius_m": MaxCollectRadius,
		})
		return
	}

	// 4) Get user for tier check and stats update
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "user_not_found", "error": "User not found"})
		return
	}

	// 5) Tier + biome + level (existujúca logika)
	canByTier, tierMsg := h.CheckUserCanCollectItem(user.Tier, req.ItemType, req.ItemID)
	if !canByTier {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "tier_blocked", "error": tierMsg})
		return
	}

	// 6) Scanner rarity limit (existujúca logika – už ju máš)
	canByScanner, scannerMsg := h.CheckScannerCanCollectItem(userID, req.ItemType, req.ItemID)
	if !canByScanner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "rarity_blocked", "error": scannerMsg})
		return
	}

	// Get zone info for biome context
	var zone gameplay.Zone
	if err := h.db.First(&zone, "id = ? AND is_active = true", zoneID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "zone_not_found", "error": "Zone not found"})
		return
	}

	// Update zone activity on collection
	h.updateZoneActivity(zone.ID)

	// Process collection based on item type
	var collectedItem interface{}
	var biome string
	var xpResult *xp.XPResult

	switch req.ItemType {
	case "artifact":
		var artifact gameplay.Artifact
		if err := h.db.First(&artifact, "id = ? AND zone_id = ? AND is_active = true", req.ItemID, zoneID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "not_found", "error": "Artifact not found"})
			return
		}

		// Deactivate artifact
		artifact.IsActive = false
		h.db.Save(&artifact)

		// Add to inventory
		inventory := gameplay.InventoryItem{
			UserID:   user.ID,
			ItemType: "artifact",
			ItemID:   artifact.ID,
			Quantity: 1,
			Properties: gameplay.JSONB{
				"name":           artifact.Name,
				"type":           artifact.Type,
				"rarity":         artifact.Rarity,
				"biome":          artifact.Biome,
				"collected_at":   time.Now().Unix(),
				"collected_from": zoneID,
				"zone_name":      zone.Name,
				"zone_biome":     zone.Biome,
				"danger_level":   zone.DangerLevel,
			},
		}
		h.db.Create(&inventory)

		// Award XP for artifact
		xpHandler := xp.NewHandler(h.db)
		var err error
		xpResult, err = xpHandler.AwardArtifactXP(user.ID, artifact.Rarity, artifact.Biome, zone.TierRequired)
		if err != nil {
			log.Printf("❌ Failed to award XP: %v", err)
		}

		collectedItem = artifact
		biome = artifact.Biome

		// Update user stats
		h.db.Model(&user).Update("total_artifacts", gorm.Expr("total_artifacts + ?", 1))

	case "gear":
		var gear gameplay.Gear
		if err := h.db.First(&gear, "id = ? AND zone_id = ? AND is_active = true", req.ItemID, zoneID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "not_found", "error": "Gear not found"})
			return
		}

		// Deactivate gear
		gear.IsActive = false
		h.db.Save(&gear)

		// Vytvor inventory item s properties z gear objektu alebo fallback na GearService
		properties := gameplay.JSONB{
			"name":           gear.Name,
			"type":           gear.Type,
			"level":          gear.Level,
			"biome":          gear.Biome,
			"equipped":       false,
			"collected_at":   time.Now().Unix(),
			"collected_from": zoneID,
			"zone_name":      zone.Name,
			"zone_biome":     gear.Biome,
			"danger_level":   zone.DangerLevel,
			"acquired_at":    time.Now().Format(time.RFC3339),
		}

		// Skontroluj či gear má už properties z databázy
		if slot, exists := gear.Properties["slot"].(string); exists {
			properties["slot"] = slot
		} else {
			properties["slot"] = h.gearService.getSlotForGearType(gear.Type)
		}

		if rarity, exists := gear.Properties["rarity"].(string); exists {
			properties["rarity"] = rarity
		} else {
			properties["rarity"] = h.gearService.getRarityForLevel(gear.Level)
		}

		if durability, exists := gear.Properties["base_durability"].(float64); exists {
			properties["durability"] = int(durability)
			properties["max_durability"] = int(durability)
		} else {
			properties["durability"] = 100
			properties["max_durability"] = 100
		}

		// Resistance properties
		if zombieRes, exists := gear.Properties["zombie_resistance"].(float64); exists {
			properties["zombie_resistance"] = int(zombieRes)
		} else {
			properties["zombie_resistance"] = h.gearService.calculateResistance(gear.Level, "zombie")
		}

		if banditRes, exists := gear.Properties["bandit_resistance"].(float64); exists {
			properties["bandit_resistance"] = int(banditRes)
		} else {
			properties["bandit_resistance"] = h.gearService.calculateResistance(gear.Level, "bandit")
		}

		if soldierRes, exists := gear.Properties["soldier_resistance"].(float64); exists {
			properties["soldier_resistance"] = int(soldierRes)
		} else {
			properties["soldier_resistance"] = h.gearService.calculateResistance(gear.Level, "soldier")
		}

		if monsterRes, exists := gear.Properties["monster_resistance"].(float64); exists {
			properties["monster_resistance"] = int(monsterRes)
		} else {
			properties["monster_resistance"] = h.gearService.calculateResistance(gear.Level, "monster")
		}

		// Pridaj category_id ak existuje
		if categoryID, exists := gear.Properties["category_id"].(string); exists {
			properties["category_id"] = categoryID
		}

		inventoryItem := gameplay.InventoryItem{
			UserID:     user.ID,
			ItemType:   "gear",
			ItemID:     uuid.New(), // Unikátne ID pre tento konkrétny predmet
			Quantity:   1,
			Properties: properties,
		}
		h.db.Create(&inventoryItem)

		collectedItem = inventoryItem
		biome = gear.Biome

		// Update user stats
		h.db.Model(&user).Update("total_gear", gorm.Expr("total_gear + ?", 1))

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "bad_request", "error": "Invalid item type"})
		return
	}

	// Check if zone should be marked for empty cleanup
	zoneUUID, _ := uuid.Parse(zoneID)
	go h.checkAndCleanupEmptyZone(zoneUUID)

	// Enhanced response s XP systémom
	response := gin.H{
		"success": true,
		"message": "Collected successfully",
		"collected_item": gin.H{
			"id":     req.ItemID,
			"type":   req.ItemType,
			"name":   itemName,
			"rarity": itemRarity,
		},
		"distance_m":   math.Round(dist*10) / 10,
		"item":         collectedItem,
		"item_name":    itemName,
		"item_type":    req.ItemType,
		"biome":        biome,
		"zone_name":    zone.Name,
		"danger_level": zone.DangerLevel,
		"collected_at": time.Now().Unix(),
		"new_total":    user.TotalArtifacts + user.TotalGear + 1,
	}

	// Add XP data if successful (len pre artifacts)
	if req.ItemType == "artifact" && xpResult != nil {
		response["xp_gained"] = xpResult.XPGained
		response["total_xp"] = xpResult.TotalXP
		response["current_level"] = xpResult.CurrentLevel
		response["xp_breakdown"] = xpResult.Breakdown

		if xpResult.LevelUp {
			response["level_up"] = true
			response["level_up_info"] = xpResult.LevelUpInfo
			response["congratulations"] = fmt.Sprintf("🎉 Level Up! You are now level %d!", xpResult.CurrentLevel)
		}
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to update zone activity
func (h *Handler) updateZoneActivity(zoneID uuid.UUID) {
	h.db.Model(&gameplay.Zone{}).Where("id = ?", zoneID).Update("last_activity", time.Now())
}

// Check and cleanup empty zone
func (h *Handler) checkAndCleanupEmptyZone(zoneID uuid.UUID) {
	// Wait a bit to allow for multiple rapid collections
	time.Sleep(30 * time.Second)

	var activeArtifacts int64
	h.db.Model(&gameplay.Artifact{}).Where("zone_id = ? AND is_active = true", zoneID).Count(&activeArtifacts)

	if activeArtifacts == 0 {
		// Zone is empty, mark for cleanup soon
		h.db.Model(&gameplay.Zone{}).Where("id = ?", zoneID).Update("last_activity", time.Now().Add(-10*time.Minute))
		log.Printf("🏰 Zone %s marked for empty cleanup", zoneID)
	}
}

// ============================================
// ✅ HELPER FUNCTIONS - TIER-BASED SPAWNING
// ============================================

// ✅ selectBiome používa getAvailableBiomes z zones.go
func (h *Handler) selectBiome(tier int) string {
	availableBiomes := h.getAvailableBiomes(tier)
	if len(availableBiomes) == 0 {
		return BiomeForest // fallback
	}
	return availableBiomes[rand.Intn(len(availableBiomes))]
}

// ✅ generateZoneName používa GetZoneTemplate z biomes.go
func (h *Handler) generateZoneName(biome string) string {
	template := GetZoneTemplate(biome)
	if len(template.Names) == 0 {
		return fmt.Sprintf("Unknown %s Zone", biome)
	}
	return template.Names[rand.Intn(len(template.Names))]
}

// ✅ NEW: Generate position based on tier distance ranges
func (h *Handler) generateTierBasedPosition(centerLat, centerLng float64, zoneTier int) (float64, float64) {
	minDistance, maxDistance := h.getTierSpawnDistance(zoneTier)

	// Random angle (0-360 degrees)
	angle := rand.Float64() * 2 * math.Pi

	// Random distance within tier range
	distance := minDistance + rand.Float64()*(maxDistance-minDistance)

	// Convert to GPS coordinates using Haversine
	earthRadius := 6371000.0 // meters

	latOffset := (distance * math.Cos(angle)) / earthRadius * (180 / math.Pi)
	lngOffset := (distance * math.Sin(angle)) / earthRadius * (180 / math.Pi) / math.Cos(centerLat*math.Pi/180)

	newLat := centerLat + latOffset
	newLng := centerLng + lngOffset

	log.Printf("🎯 [TIER %d] Spawning zone at distance %.0fm (range: %.0f-%.0fm) angle: %.1f°",
		zoneTier, distance, minDistance, maxDistance, angle*180/math.Pi)

	return newLat, newLng
}

// ✅ NEW: Tier-based distance calculation for zone spawning
func (h *Handler) getTierSpawnDistance(zoneTier int) (float64, float64) {
	switch zoneTier {
	case 0:
		return Tier0MinDistance, Tier0MaxDistance
	case 1:
		return Tier1MinDistance, Tier1MaxDistance
	case 2:
		return Tier2MinDistance, Tier2MaxDistance
	case 3:
		return Tier3MinDistance, Tier3MaxDistance
	case 4:
		return Tier4MinDistance, Tier4MaxDistance
	default:
		return Tier0MinDistance, Tier0MaxDistance
	}
}

// ✅ ENHANCED: spawnItemsInZone s konfigurovateľnými spawn rates
func (h *Handler) spawnItemsInZone(zoneID uuid.UUID, tier int, biome string, zoneCenter gameplay.Location, zoneRadius int) {
	template := GetZoneTemplate(biome)

	// ✅ KONFIGUROVATEĽNÉ NASTAVENIA - zmeň tieto hodnoty podľa potreby
	const (
		// Základné spawn rates (0.0 = 0%, 1.0 = 100%)
		baseArtifactSpawnRate = 0.8  // 80% šanca na artifact spawn
		baseGearSpawnRate     = 0.7  // 70% šanca na gear spawn
		exclusiveSpawnRate    = 0.15 // 15% šanca na exclusive artifacts

		// Multiplikátory podľa tier (vyšší tier = viac items)
		tierMultiplier = 0.1 // +10% za každý tier

		// Minimum garantovaných items
		minArtifactsPerZone = 1
		minGearPerZone      = 1

		// Maximum items per zone
		maxArtifactsPerZone = 5
		maxGearPerZone      = 4
	)

	log.Printf("🏭 [DEBUG] Spawning items in %s zone (tier %d)", biome, tier)
	log.Printf("🔧 [DEBUG] Template: %d artifact types, %d gear types, %d exclusive",
		len(template.ArtifactSpawnRates), len(template.GearSpawnRates), len(template.ExclusiveArtifacts))

	// Výpočet tier bonus
	tierBonus := float64(tier) * tierMultiplier
	adjustedArtifactRate := baseArtifactSpawnRate + tierBonus
	adjustedGearRate := baseGearSpawnRate + tierBonus
	adjustedExclusiveRate := exclusiveSpawnRate + tierBonus

	log.Printf("📊 [DEBUG] Adjusted rates: artifacts=%.2f, gear=%.2f, exclusive=%.2f",
		adjustedArtifactRate, adjustedGearRate, adjustedExclusiveRate)

	// ✅ ARTIFACT SPAWNING s debug informáciami
	artifactsSpawned := 0
	artifactAttempts := 0

	for artifactType, templateRate := range template.ArtifactSpawnRates {
		// Kombinuj template rate s našou adjusted rate
		finalRate := templateRate * adjustedArtifactRate
		roll := rand.Float64()

		log.Printf("🎲 [DEBUG] %s: roll=%.3f vs rate=%.3f (template=%.2f * adjusted=%.2f)",
			artifactType, roll, finalRate, templateRate, adjustedArtifactRate)

		if roll < finalRate && artifactsSpawned < maxArtifactsPerZone {
			if err := h.spawnSpecificArtifact(zoneID, artifactType, biome, tier); err != nil {
				log.Printf("❌ [ERROR] Failed to spawn artifact %s: %v", artifactType, err)
			} else {
				artifactsSpawned++
				log.Printf("💎 [SUCCESS] Spawned artifact: %s (roll %.3f < %.3f)",
					GetArtifactDisplayName(artifactType), roll, finalRate)
			}
		} else if roll >= finalRate {
			log.Printf("⭕ [SKIP] %s - roll failed", artifactType)
		} else {
			log.Printf("🚫 [LIMIT] %s - max artifacts reached", artifactType)
		}
		artifactAttempts++
	}

	// ✅ GUARANTEED MINIMUM ARTIFACTS
	if artifactsSpawned < minArtifactsPerZone && len(template.ArtifactSpawnRates) > 0 {
		log.Printf("🔄 [GUARANTEE] Need %d more artifacts for minimum", minArtifactsPerZone-artifactsSpawned)

		// Vyber náhodné artifact types z template
		artifactTypes := make([]string, 0, len(template.ArtifactSpawnRates))
		for artifactType := range template.ArtifactSpawnRates {
			artifactTypes = append(artifactTypes, artifactType)
		}

		for artifactsSpawned < minArtifactsPerZone && len(artifactTypes) > 0 {
			randomIndex := rand.Intn(len(artifactTypes))
			artifactType := artifactTypes[randomIndex]

			if err := h.spawnSpecificArtifact(zoneID, artifactType, biome, tier); err != nil {
				log.Printf("❌ [GUARANTEE] Failed to spawn guaranteed %s: %v", artifactType, err)
			} else {
				artifactsSpawned++
				log.Printf("💎 [GUARANTEE] Spawned guaranteed: %s", GetArtifactDisplayName(artifactType))
			}

			// Odstráň z listu aby sa neopakoval
			artifactTypes = append(artifactTypes[:randomIndex], artifactTypes[randomIndex+1:]...)
		}
	}

	// ✅ EXCLUSIVE ARTIFACTS s debug
	exclusiveSpawned := 0
	for _, exclusiveType := range template.ExclusiveArtifacts {
		roll := rand.Float64()
		log.Printf("🌟 [DEBUG] Exclusive %s: roll=%.3f vs rate=%.3f",
			exclusiveType, roll, adjustedExclusiveRate)

		if roll < adjustedExclusiveRate && (artifactsSpawned+exclusiveSpawned) < maxArtifactsPerZone {
			if err := h.spawnSpecificArtifact(zoneID, exclusiveType, biome, tier); err != nil {
				log.Printf("❌ [ERROR] Failed to spawn exclusive %s: %v", exclusiveType, err)
			} else {
				exclusiveSpawned++
				log.Printf("🌟 [SUCCESS] Spawned EXCLUSIVE: %s", GetArtifactDisplayName(exclusiveType))
			}
		}
	}

	// ✅ GEAR SPAWNING s debug informáciami
	gearSpawned := 0
	for gearType, templateRate := range template.GearSpawnRates {
		finalRate := templateRate * adjustedGearRate
		roll := rand.Float64()

		log.Printf("⚔️ [DEBUG] %s: roll=%.3f vs rate=%.3f", gearType, roll, finalRate)

		if roll < finalRate && gearSpawned < maxGearPerZone {
			if err := h.spawnSpecificGear(zoneID, gearType, biome, tier); err != nil {
				log.Printf("❌ [ERROR] Failed to spawn gear %s: %v", gearType, err)
			} else {
				gearSpawned++
				log.Printf("⚔️ [SUCCESS] Spawned gear: %s", GetGearDisplayName(gearType))
			}
		}
	}

	// ✅ GUARANTEED MINIMUM GEAR
	if gearSpawned < minGearPerZone && len(template.GearSpawnRates) > 0 {
		log.Printf("🔄 [GUARANTEE] Need %d more gear for minimum", minGearPerZone-gearSpawned)

		gearTypes := make([]string, 0, len(template.GearSpawnRates))
		for gearType := range template.GearSpawnRates {
			gearTypes = append(gearTypes, gearType)
		}

		for gearSpawned < minGearPerZone && len(gearTypes) > 0 {
			randomIndex := rand.Intn(len(gearTypes))
			gearType := gearTypes[randomIndex]

			if err := h.spawnSpecificGear(zoneID, gearType, biome, tier); err != nil {
				log.Printf("❌ [GUARANTEE] Failed to spawn guaranteed %s: %v", gearType, err)
			} else {
				gearSpawned++
				log.Printf("⚔️ [GUARANTEE] Spawned guaranteed: %s", GetGearDisplayName(gearType))
			}

			gearTypes = append(gearTypes[:randomIndex], gearTypes[randomIndex+1:]...)
		}
	}

	totalArtifacts := artifactsSpawned + exclusiveSpawned
	log.Printf("✅ [FINAL] Zone spawning complete: %d artifacts (%d regular + %d exclusive), %d gear items",
		totalArtifacts, artifactsSpawned, exclusiveSpawned, gearSpawned)
	log.Printf("📊 [STATS] Success rate: artifacts=%d/%d, gear=%d/%d",
		totalArtifacts, artifactAttempts, gearSpawned, len(template.GearSpawnRates))
}

// Generate random GPS coordinates within zone radius
func (h *Handler) generateRandomLocationInZone(center gameplay.Location, radiusMeters int) gameplay.Location {
	// Random angle (0-360 degrees)
	angle := rand.Float64() * 2 * math.Pi

	// Random distance (0 to radiusMeters)
	distance := rand.Float64() * float64(radiusMeters)

	// Convert to GPS coordinates
	// 1 degree ≈ 111,000 meters at equator
	latOffset := (distance * math.Cos(angle)) / 111000
	lngOffset := (distance * math.Sin(angle)) / (111000 * math.Cos(center.Latitude*math.Pi/180))

	return gameplay.Location{
		Latitude:  center.Latitude + latOffset,
		Longitude: center.Longitude + lngOffset,
		Timestamp: time.Now(),
	}
}

// ============================================
// ZONE CLEANUP ENDPOINTS (REAL IMPLEMENTATIONS)
// ============================================

func (h *Handler) CleanupExpiredZones(c *gin.Context) {
	cleanupService := NewCleanupService(h.db)
	result := cleanupService.CleanupExpiredZones()

	c.JSON(http.StatusOK, gin.H{
		"message": "Zone cleanup completed",
		"result":  result,
		"status":  "success",
	})
}

func (h *Handler) GetExpiredZones(c *gin.Context) {
	var expiredZones []gameplay.Zone
	h.db.Where("is_active = true AND expires_at < ?", time.Now()).Find(&expiredZones)

	c.JSON(http.StatusOK, gin.H{
		"expired_zones": expiredZones,
		"count":         len(expiredZones),
		"current_time":  time.Now().Format(time.RFC3339),
		"status":        "success",
	})
}

func (h *Handler) GetZoneAnalytics(c *gin.Context) {
	cleanupService := NewCleanupService(h.db)
	stats := cleanupService.GetCleanupStats()

	c.JSON(http.StatusOK, gin.H{
		"zone_analytics": stats,
		"timestamp":      time.Now().Format(time.RFC3339),
		"status":         "success",
	})
}

// ============================================
// STUB ENDPOINTS (TO BE IMPLEMENTED)
// ============================================

func (h *Handler) GetAvailableArtifacts(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get available artifacts not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetAvailableGear(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Get user tier
	var user auth.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get biome from query parameter
	biome := c.DefaultQuery("biome", "")
	if biome == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Biome parameter required"})
		return
	}

	// Get available gear using GearService - vráti gear ktorý sa môže spawnovať v zónach
	availableGear, err := h.gearService.GetAvailableGearInZones(user.Tier, biome)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get available gear"})
		return
	}

	// Konvertuj na response format
	var gearResponse []gin.H
	for _, gear := range availableGear {
		gearResponse = append(gearResponse, gin.H{
			"id":           gear.ID,
			"name":         gear.Name,
			"type":         gear.Type,
			"level":        gear.Level,
			"biome":        gear.Biome,
			"slot":         h.gearService.getSlotForGearType(gear.Type),
			"rarity":       h.gearService.getRarityForLevel(gear.Level),
			"display_name": GetGearDisplayName(gear.Type),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"gear":        gearResponse,
		"total_gear":  len(gearResponse),
		"user_tier":   user.Tier,
		"biome":       biome,
		"message":     "Available gear retrieved successfully",
		"description": "Gear that can spawn in zones for this tier and biome",
	})
}

func (h *Handler) UseItem(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Use item not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetLeaderboard(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get leaderboard not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetGameStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get game stats not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetZoneStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get zone stats not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) CreateEventZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Create event zone not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) UpdateZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Update zone not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) DeleteZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Delete zone not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) SpawnArtifact(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Spawn artifact not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) SpawnGear(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Spawn gear not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetItemAnalytics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get item analytics not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetAllUsers(c *gin.Context) {
	// Get all users (Super Admin only)
	var users []auth.User
	if err := h.db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch users",
		})
		return
	}

	// Remove password hashes for security
	for i := range users {
		users[i].PasswordHash = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"users":        users,
		"total_users":  len(users),
		"message":      "Users retrieved successfully",
		"timestamp":    time.Now().Format(time.RFC3339),
		"requested_by": c.GetString("username"),
	})
}
