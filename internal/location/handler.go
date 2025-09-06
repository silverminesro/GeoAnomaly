package location

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"geoanomaly/internal/auth"
	"geoanomaly/internal/common"
	"geoanomaly/internal/gameplay"
	"geoanomaly/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	redis_client "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	db    *gorm.DB
	redis *redis_client.Client
}

type UpdateLocationRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Accuracy  float64 `json:"accuracy,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	Heading   float64 `json:"heading,omitempty"`
}

type PlayerInZone struct {
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	Tier         int       `json:"tier"`
	LastSeen     time.Time `json:"last_seen"`
	Distance     float64   `json:"distance_meters"`
	IsOnline     bool      `json:"is_online"`
	Avatar       string    `json:"avatar,omitempty"`
	CurrentZone  string    `json:"current_zone,omitempty"`
	ZonePosition struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"zone_position"`
}

type NearbyPlayersResponse struct {
	Players      []PlayerInZone              `json:"players"`
	TotalPlayers int                         `json:"total_players"`
	ZoneID       uuid.UUID                   `json:"zone_id"`
	ZoneName     string                      `json:"zone_name"`
	YourPosition common.LocationWithAccuracy `json:"your_position"` // ✅ OPRAVENÉ: používa LocationWithAccuracy
}

func NewHandler(db *gorm.DB, redisClient *redis_client.Client) *Handler {
	return &Handler{
		db:    db,
		redis: redisClient,
	}
}

// ✅ OPRAVENÉ: UpdateLocation - používa LocationWithAccuracy
func (h *Handler) UpdateLocation(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	username, _ := c.Get("username")

	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validácia GPS súradníc
	if !isValidGPSCoordinate(req.Latitude, req.Longitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GPS coordinates"})
		return
	}

	// ✅ OPRAVENÉ: LocationWithAccuracy pre user tracking
	location := common.LocationWithAccuracy{
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Accuracy:  req.Accuracy,
		Timestamp: time.Now(),
	}

	// Aktualizuj v databáze (User table nemá location columns, takže len log)
	if err := h.updateUserLocation(userID.(uuid.UUID), location); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	// Nájdi aktuálnu zónu
	currentZone := h.findCurrentZone(req.Latitude, req.Longitude)

	// Aktualizuj player session
	h.updatePlayerSession(userID.(uuid.UUID), username.(string), currentZone, location, req.Speed, req.Heading)

	// Real-time notifikácie pre ostatných hráčov v zóne
	if currentZone != nil {
		h.notifyPlayersInZone(*currentZone, userID.(uuid.UUID), username.(string), location)
	}

	response := gin.H{
		"message":      "Location updated successfully",
		"location":     location,
		"current_zone": currentZone,
		"timestamp":    time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// ✅ OPRAVENÉ: GetNearbyPlayers - používa LocationWithAccuracy
func (h *Handler) GetNearbyPlayers(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Získaj GPS súradnice
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

	// Nájdi aktuálnu zónu
	currentZone := h.findCurrentZone(lat, lng)
	if currentZone == nil {
		c.JSON(http.StatusOK, NearbyPlayersResponse{
			Players:      []PlayerInZone{},
			TotalPlayers: 0,
			YourPosition: common.LocationWithAccuracy{Latitude: lat, Longitude: lng, Timestamp: time.Now()}, // ✅ OPRAVENÉ
		})
		return
	}

	// Nájdi všetkých hráčov v tejto zóne
	var playerSessions []auth.PlayerSession
	if err := h.db.Preload("User").Where("current_zone = ? AND user_id != ? AND is_online = true", currentZone, userID).Find(&playerSessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}

	// Získaj informácie o zóne
	var zone gameplay.Zone
	h.db.First(&zone, "id = ?", currentZone)

	// Vytvor response
	var players []PlayerInZone
	for _, session := range playerSessions {
		// Načítaj User samostatne
		var user auth.User
		if err := h.db.First(&user, "id = ?", session.UserID).Error; err != nil {
			continue
		}

		// ✅ OPRAVENÉ: Vypočítaj vzdialenosť pomocou individual fields
		distance := calculateDistance(lat, lng, session.LastLocationLatitude, session.LastLocationLongitude)

		// Iba ak sú v rozumnej vzdialenosti v rámci zóny (napr. 500m)
		if distance <= 500 {
			player := PlayerInZone{
				UserID:      session.UserID,
				Username:    user.Username,
				Tier:        user.Tier,
				LastSeen:    session.LastSeen,
				Distance:    distance,
				IsOnline:    session.IsOnline && time.Since(session.LastSeen) < 5*time.Minute,
				CurrentZone: zone.Name,
			}

			// ✅ OPRAVENÉ: Použiť individual fields
			player.ZonePosition.Latitude = session.LastLocationLatitude
			player.ZonePosition.Longitude = session.LastLocationLongitude

			players = append(players, player)
		}
	}

	response := NearbyPlayersResponse{
		Players:      players,
		TotalPlayers: len(players),
		ZoneID:       *currentZone,
		ZoneName:     zone.Name,
		YourPosition: common.LocationWithAccuracy{Latitude: lat, Longitude: lng, Timestamp: time.Now()}, // ✅ OPRAVENÉ
	}

	c.JSON(http.StatusOK, response)
}

// GetZoneActivity - aktivita v konkrétnej zóne
func (h *Handler) GetZoneActivity(c *gin.Context) {
	zoneID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid zone ID"})
		return
	}

	// Počet hráčov v zóne
	var activePlayersCount int64
	h.db.Model(&auth.PlayerSession{}).Where("current_zone = ? AND is_online = true AND last_seen > ?", zoneID, time.Now().Add(-5*time.Minute)).Count(&activePlayersCount)

	// Posledné aktivity (collecting, spawning)
	var recentCollections []gin.H

	// Nedávno zbierané artefakty
	var recentArtifacts []gameplay.InventoryItem
	h.db.Where("item_type = ? AND created_at > ?", "artifact", time.Now().Add(-1*time.Hour)).
		Order("created_at DESC").
		Limit(10).
		Find(&recentArtifacts)

	for _, item := range recentArtifacts {
		if zoneIDFromProps, exists := item.Properties["zone_id"]; exists {
			if zoneIDFromProps == zoneID.String() {
				recentCollections = append(recentCollections, gin.H{
					"type":      "artifact_collected",
					"item_name": item.Properties["name"],
					"rarity":    item.Properties["rarity"],
					"timestamp": item.CreatedAt.Unix(),
				})
			}
		}
	}

	// Aktívne artefakty v zóne
	var activeArtifacts int64
	h.db.Model(&gameplay.Artifact{}).Where("zone_id = ? AND is_active = true", zoneID).Count(&activeArtifacts)

	// Aktívne gear v zóne
	var activeGear int64
	h.db.Model(&gameplay.Gear{}).Where("zone_id = ? AND is_active = true", zoneID).Count(&activeGear)

	response := gin.H{
		"zone_id":          zoneID,
		"active_players":   activePlayersCount,
		"active_artifacts": activeArtifacts,
		"active_gear":      activeGear,
		"recent_activity":  recentCollections,
		"last_updated":     time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// ✅ OPRAVENÉ: Pomocné funkcie
func (h *Handler) findCurrentZone(lat, lng float64) *uuid.UUID {
	var zones []gameplay.Zone

	// ✅ OPRAVENÉ: Simplified query bez PostGIS dependency (rovnako ako v game handler)
	if err := h.db.Where("is_active = true").Find(&zones).Error; err != nil {
		return nil
	}

	// Manual distance filtering
	for _, zone := range zones {
		distance := calculateDistance(lat, lng, zone.Location.Latitude, zone.Location.Longitude)
		if distance <= float64(zone.RadiusMeters) {
			return &zone.ID
		}
	}

	return nil
}

// ✅ OPRAVENÉ: updateUserLocation - log only (User table nemá location columns)
func (h *Handler) updateUserLocation(userID uuid.UUID, location common.LocationWithAccuracy) error {
	// User table nemá location columns, takže len log
	// fmt.Printf("📍 User %s location updated: [%.6f, %.6f] (accuracy: %.1fm)\n", userID, location.Latitude, location.Longitude, location.Accuracy)
	return nil
}

// ✅ OPRAVENÉ: updatePlayerSession s LocationWithAccuracy a individual fields
func (h *Handler) updatePlayerSession(userID uuid.UUID, username string, currentZone *uuid.UUID, location common.LocationWithAccuracy, speed, heading float64) {
	session := auth.PlayerSession{
		UserID:      userID,
		LastSeen:    time.Now(),
		IsOnline:    true,
		CurrentZone: currentZone,
		// ✅ OPRAVENÉ: Použiť individual fields namiesto embedded struct
		LastLocationLatitude:  location.Latitude,
		LastLocationLongitude: location.Longitude,
		LastLocationAccuracy:  location.Accuracy,
		LastLocationTimestamp: location.Timestamp,
	}

	// Upsert player session
	h.db.Where("user_id = ?", userID).Assign(session).FirstOrCreate(&session)

	// Aktualizuj aj v Redis pre real-time tracking
	h.updateRedisPlayerSession(userID, username, currentZone, location, speed, heading)
}

// ✅ OPRAVENÉ: updateRedisPlayerSession s LocationWithAccuracy
func (h *Handler) updateRedisPlayerSession(userID uuid.UUID, username string, currentZone *uuid.UUID, location common.LocationWithAccuracy, speed, heading float64) {
	if h.redis == nil {
		return // Skip ak Redis nie je dostupný
	}

	playerData := map[string]interface{}{
		"user_id":   userID.String(),
		"username":  username,
		"latitude":  location.Latitude,
		"longitude": location.Longitude,
		"accuracy":  location.Accuracy,
		"speed":     speed,
		"heading":   heading,
		"timestamp": time.Now().Unix(),
	}

	if currentZone != nil {
		playerData["current_zone"] = currentZone.String()
	}

	// Uloži do Redis s TTL 10 minút
	key := fmt.Sprintf("player_session:%s", userID.String())
	redis.SetWithExpiration(h.redis, key, playerData, 10*time.Minute)

	// Pridaj do zoznamov hráčov v zóne
	if currentZone != nil {
		zoneKey := fmt.Sprintf("zone_players:%s", currentZone.String())
		h.redis.SAdd(context.Background(), zoneKey, userID.String())
		h.redis.Expire(context.Background(), zoneKey, 10*time.Minute)
	}
}

// ✅ OPRAVENÉ: notifyPlayersInZone s LocationWithAccuracy
func (h *Handler) notifyPlayersInZone(zoneID uuid.UUID, userID uuid.UUID, username string, location common.LocationWithAccuracy) {
	if h.redis == nil {
		return // Skip ak Redis nie je dostupný
	}

	// Real-time WebSocket notifikácie (pre budúce použitie)
	notificationKey := fmt.Sprintf("zone_notification:%s", zoneID.String())

	notification := map[string]interface{}{
		"type":      "player_moved",
		"user_id":   userID.String(),
		"username":  username,
		"zone_id":   zoneID.String(),
		"latitude":  location.Latitude,
		"longitude": location.Longitude,
		"timestamp": time.Now().Unix(),
	}

	// Uloži do Redis pre real-time systém
	redis.SetWithExpiration(h.redis, notificationKey, notification, 1*time.Minute)
}

func isValidGPSCoordinate(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// ✅ OPRAVENÉ: calculateDistance - použiť math functions namiesto vlastných
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Haversine formula
	const earthRadius = 6371000 // metrov

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// Missing methods - stub implementation
func (h *Handler) GetPlayersInZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get players in zone not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetLocationHistory(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Location history not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetLocationHeatmap(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Location heatmap not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) ShareLocation(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Share location not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetNearbyFriends(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get nearby friends not implemented yet",
		"status": "planned",
	})
}
