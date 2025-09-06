package game

import (
	"fmt"
	"log"
	"time"

	"geoanomaly/internal/auth"
	"geoanomaly/internal/gameplay"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CleanupService struct {
	db *gorm.DB
}

type CleanupResult struct {
	ExpiredZones    []gameplay.Zone `json:"expired_zones"`
	CleanedCount    int             `json:"cleaned_count"`
	ItemsRemoved    int             `json:"items_removed"`
	PlayersAffected int             `json:"players_affected"`
	CleanupDuration string          `json:"cleanup_duration"`
	NextCleanupIn   string          `json:"next_cleanup_in"`
}

func NewCleanupService(db *gorm.DB) *CleanupService {
	return &CleanupService{db: db}
}

// ✅ Main cleanup function
func (cs *CleanupService) CleanupExpiredZones() CleanupResult {
	startTime := time.Now()
	log.Printf("🧹 Starting zone cleanup at %s", startTime.Format("15:04:05"))

	result := CleanupResult{
		ExpiredZones: []gameplay.Zone{},
	}

	// Find expired zones
	var expiredZones []gameplay.Zone
	if err := cs.db.Where("is_active = true AND auto_cleanup = true AND expires_at < ?", time.Now()).Find(&expiredZones).Error; err != nil {
		log.Printf("❌ Failed to find expired zones: %v", err)
		return result
	}

	result.ExpiredZones = expiredZones
	result.CleanedCount = len(expiredZones)

	if len(expiredZones) == 0 {
		log.Printf("✅ No expired zones found")
		return result
	}

	log.Printf("🗑️ Found %d expired zones to cleanup", len(expiredZones))

	// Process each expired zone
	for _, zone := range expiredZones {
		itemsRemoved, playersAffected := cs.cleanupSingleZone(zone)
		result.ItemsRemoved += itemsRemoved
		result.PlayersAffected += playersAffected
	}

	result.CleanupDuration = time.Since(startTime).String()
	result.NextCleanupIn = "5 minutes"

	log.Printf("🎯 Cleanup completed: %d zones, %d items, %d players affected in %s",
		result.CleanedCount, result.ItemsRemoved, result.PlayersAffected, result.CleanupDuration)

	return result
}

// ✅ Cleanup single zone
func (cs *CleanupService) cleanupSingleZone(zone gameplay.Zone) (int, int) {
	log.Printf("🗑️ Cleaning zone: %s (expired %s)", zone.Name, time.Since(*zone.ExpiresAt).String())

	itemsRemoved := 0
	playersAffected := 0

	// 1. Remove/deactivate artifacts
	var artifacts []gameplay.Artifact
	cs.db.Where("zone_id = ? AND is_active = true", zone.ID).Find(&artifacts)
	if len(artifacts) > 0 {
		cs.db.Model(&gameplay.Artifact{}).Where("zone_id = ?", zone.ID).Update("is_active", false)
		itemsRemoved += len(artifacts)
		log.Printf("   📦 Deactivated %d artifacts", len(artifacts))
	}

	// 2. Remove/deactivate gear
	var gear []gameplay.Gear
	cs.db.Where("zone_id = ? AND is_active = true", zone.ID).Find(&gear)
	if len(gear) > 0 {
		cs.db.Model(&gameplay.Gear{}).Where("zone_id = ?", zone.ID).Update("is_active", false)
		itemsRemoved += len(gear)
		log.Printf("   ⚔️ Deactivated %d gear items", len(gear))
	}

	// 3. Remove players from zone
	var sessions []auth.PlayerSession
	cs.db.Where("current_zone = ?", zone.ID).Find(&sessions)
	if len(sessions) > 0 {
		cs.db.Model(&auth.PlayerSession{}).Where("current_zone = ?", zone.ID).Update("current_zone", nil)
		playersAffected = len(sessions)
		log.Printf("   👥 Removed %d players from zone", playersAffected)
	}

	// 4. Deactivate zone
	zone.IsActive = false
	zone.Properties["cleanup_reason"] = "expired"
	zone.Properties["cleanup_time"] = time.Now().Unix()
	cs.db.Save(&zone)

	log.Printf("   ✅ Zone %s cleaned successfully", zone.Name)
	return itemsRemoved, playersAffected
}

// ✅ Get zones about to expire
func (cs *CleanupService) GetExpiringZones(warningMinutes int) []gameplay.Zone {
	warningTime := time.Now().Add(time.Duration(warningMinutes) * time.Minute)

	var expiringZones []gameplay.Zone
	cs.db.Where("is_active = true AND expires_at BETWEEN ? AND ?", time.Now(), warningTime).Find(&expiringZones)

	return expiringZones
}

// ✅ Force cleanup specific zone
func (cs *CleanupService) ForceCleanupZone(zoneID uuid.UUID, reason string) error {
	var zone gameplay.Zone
	if err := cs.db.First(&zone, "id = ? AND is_active = true", zoneID).Error; err != nil {
		return fmt.Errorf("zone not found: %v", err)
	}

	itemsRemoved, playersAffected := cs.cleanupSingleZone(zone)

	// Update cleanup reason
	zone.Properties["cleanup_reason"] = reason
	zone.Properties["force_cleanup"] = true
	cs.db.Save(&zone)

	log.Printf("🔧 Force cleanup completed for %s: %d items, %d players", zone.Name, itemsRemoved, playersAffected)
	return nil
}

// ✅ Cleanup statistics
func (cs *CleanupService) GetCleanupStats() map[string]interface{} {
	var totalZones, activeZones, expiredZones, cleanedZones int64

	cs.db.Model(&gameplay.Zone{}).Count(&totalZones)
	cs.db.Model(&gameplay.Zone{}).Where("is_active = true").Count(&activeZones)
	cs.db.Model(&gameplay.Zone{}).Where("is_active = true AND expires_at < ?", time.Now()).Count(&expiredZones)
	cs.db.Model(&gameplay.Zone{}).Where("is_active = false").Count(&cleanedZones)

	return map[string]interface{}{
		"total_zones":   totalZones,
		"active_zones":  activeZones,
		"expired_zones": expiredZones,
		"cleaned_zones": cleanedZones,
		"cleanup_rate":  fmt.Sprintf("%.1f%%", float64(cleanedZones)/float64(totalZones)*100),
	}
}
