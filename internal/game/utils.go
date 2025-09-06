package game

import (
	"context"
	"fmt"
	"geoanomaly/internal/auth"
	"geoanomaly/internal/gameplay"
	"math"
	"time"

	"github.com/google/uuid"
)

// Rate limiting functions
func (h *Handler) checkAreaScanRateLimit(key string) bool {
	return h.checkRateLimit(key, 1, AreaScanCooldown*time.Minute)
}

func (h *Handler) checkRateLimit(key string, limit int, duration time.Duration) bool {
	if h.redis == nil {
		return true // Allow if Redis unavailable
	}

	count, err := h.redis.Get(context.Background(), key).Int()
	if err != nil {
		return true // Allow if Redis error
	}

	if count >= limit {
		return false
	}

	h.redis.Incr(context.Background(), key)
	h.redis.Expire(context.Background(), key, duration)
	return true
}

// Distance calculation
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	dlat := (lat2 - lat1) * (math.Pi / 180.0)
	dlon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c * 1000
}

func IsValidGPSCoordinate(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func FormatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours()/24), int(d.Hours())%24)
}

// ✅ NEW: Enhanced session tracking helper functions
func (h *Handler) formatDurationDetailed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm %ds", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

func (h *Handler) calculateItemsPerHour(items int, duration time.Duration) float64 {
	if duration.Hours() == 0 || duration.Hours() < 0.01 { // Avoid division by zero
		return 0
	}
	return float64(items) / duration.Hours()
}

// ✅ NEW: Session item tracking
func (h *Handler) getSessionItemsCollected(userID uuid.UUID, zoneID *uuid.UUID, enteredAt time.Time) int {
	if zoneID == nil {
		return 0
	}

	// Count items collected from this zone since entered
	var count int64

	h.db.Model(&gameplay.InventoryItem{}).
		Where("user_id = ? AND created_at > ? AND properties->>'collected_from' = ?",
			userID, enteredAt, zoneID.String()).
		Count(&count)

	return int(count)
}

// ✅ NEW: XP calculation based on items and tier
func (h *Handler) calculateXPGained(itemsCollected int, zoneTier int, biome string) int {
	baseXP := itemsCollected * 10 // 10 XP per item

	// Bonus for higher tier zones
	tierBonus := zoneTier * 5

	// Bonus for dangerous biomes
	biomeBonus := 0
	switch biome {
	case BiomeRadioactive, BiomeChemical:
		biomeBonus = itemsCollected * 15 // Extreme danger bonus
	case BiomeIndustrial:
		biomeBonus = itemsCollected * 10 // High danger bonus
	case BiomeMountain, BiomeUrban, BiomeWater:
		biomeBonus = itemsCollected * 5 // Medium danger bonus
	default:
		biomeBonus = 0 // Forest - no bonus
	}

	return baseXP + tierBonus + biomeBonus
}

// ✅ NEW: Zone information extraction
func (h *Handler) getZoneInfo(zoneID *uuid.UUID) (string, string, string, int) {
	if zoneID == nil {
		return "Unknown Zone", "forest", "low", 0
	}

	var zone gameplay.Zone
	if err := h.db.First(&zone, "id = ?", zoneID).Error; err != nil {
		return "Unknown Zone", "forest", "low", 0
	}

	return zone.Name, zone.Biome, zone.DangerLevel, zone.TierRequired
}

// ✅ NEW: Session data storage in Properties
func (h *Handler) storeSessionData(session *auth.PlayerSession, data SessionTracker) {
	// Store session tracking data in PlayerSession (we could extend the model later)
	// For now, we'll calculate it on-demand in ExitZone
}

// ✅ NEW: Format time for better readability
func (h *Handler) formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
}

// ✅ NEW: Validate session integrity
func (h *Handler) validateSession(userID uuid.UUID, zoneID string) (*auth.PlayerSession, error) {
	var session auth.PlayerSession
	err := h.db.Preload("Zone").Where("user_id = ? AND current_zone = ?", userID, zoneID).First(&session).Error
	return &session, err
}
