package game

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

type Scheduler struct {
	db             *gorm.DB
	cleanupService *CleanupService
	ticker         *time.Ticker
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
}

type SchedulerStats struct {
	IsRunning       bool      `json:"is_running"`
	LastCleanup     time.Time `json:"last_cleanup"`
	NextCleanup     time.Time `json:"next_cleanup"`
	TotalCleanups   int       `json:"total_cleanups"`
	CleanupInterval string    `json:"cleanup_interval"`
	ZonesCleaned    int       `json:"zones_cleaned"`
	ItemsRemoved    int       `json:"items_removed"`
	PlayersAffected int       `json:"players_affected"`
}

func NewScheduler(db *gorm.DB) *Scheduler {
	cleanupService := NewCleanupService(db)
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		db:             db,
		cleanupService: cleanupService,
		ctx:            ctx,
		cancel:         cancel,
		isRunning:      false,
	}
}

// ✅ Start background cleanup scheduler
func (s *Scheduler) Start() {
	if s.isRunning {
		log.Printf("⚠️ Scheduler already running")
		return
	}

	s.isRunning = true
	s.ticker = time.NewTicker(5 * time.Minute) // Every 5 minutes

	log.Printf("🕐 Zone cleanup scheduler started (5min interval)")

	// Run initial cleanup
	go func() {
		log.Printf("🧹 Running initial cleanup...")
		result := s.cleanupService.CleanupExpiredZones()
		s.logCleanupResult(result)
	}()

	// Start scheduled cleanup
	go s.run()
}

// ✅ Stop scheduler
func (s *Scheduler) Stop() {
	if !s.isRunning {
		log.Printf("⚠️ Scheduler not running")
		return
	}

	s.cancel()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.isRunning = false

	log.Printf("🛑 Zone cleanup scheduler stopped")
}

// ✅ Main scheduler loop
func (s *Scheduler) run() {
	defer func() {
		s.isRunning = false
		if s.ticker != nil {
			s.ticker.Stop()
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🛑 Scheduler context cancelled")
			return

		case <-s.ticker.C:
			log.Printf("⏰ Scheduled cleanup triggered at %s", time.Now().Format("15:04:05"))

			// Run cleanup
			result := s.cleanupService.CleanupExpiredZones()
			s.logCleanupResult(result)

			// Check for zones about to expire (30min warning)
			s.checkExpiringZones()

			// Run battery drain for deployed scanners
			s.drainDeployedScannerBatteries()
		}
	}
}

// ✅ Log cleanup results
func (s *Scheduler) logCleanupResult(result CleanupResult) {
	if result.CleanedCount > 0 {
		log.Printf("🎯 CLEANUP COMPLETED: %d zones cleaned, %d items removed, %d players affected",
			result.CleanedCount, result.ItemsRemoved, result.PlayersAffected)

		// Log each cleaned zone
		for _, zone := range result.ExpiredZones {
			timeExpired := time.Since(*zone.ExpiresAt)
			log.Printf("   🗑️ Cleaned: %s (expired %s ago)", zone.Name, timeExpired.Round(time.Minute))
		}
	} else {
		log.Printf("✅ No expired zones found")
	}
}

// ✅ Check for zones about to expire
func (s *Scheduler) checkExpiringZones() {
	expiringZones := s.cleanupService.GetExpiringZones(30) // 30min warning

	if len(expiringZones) > 0 {
		log.Printf("⚠️ WARNING: %d zones expiring in next 30 minutes:", len(expiringZones))
		for _, zone := range expiringZones {
			timeLeft := time.Until(*zone.ExpiresAt)
			log.Printf("   ⏰ %s expires in %s", zone.Name, timeLeft.Round(time.Minute))
		}
	}
}

// ✅ Get scheduler status
func (s *Scheduler) GetStatus() SchedulerStats {
	stats := s.cleanupService.GetCleanupStats()

	return SchedulerStats{
		IsRunning:       s.isRunning,
		LastCleanup:     time.Now(), // TODO: Track actual last cleanup time
		NextCleanup:     time.Now().Add(5 * time.Minute),
		CleanupInterval: "5 minutes",
		ZonesCleaned:    int(stats["cleaned_zones"].(int64)),
		// TODO: Track cumulative stats
	}
}

// ✅ Force immediate cleanup
func (s *Scheduler) ForceCleanup() CleanupResult {
	log.Printf("🔧 Force cleanup triggered manually")
	return s.cleanupService.CleanupExpiredZones()
}

// ✅ Health check
func (s *Scheduler) IsHealthy() bool {
	return s.isRunning
}

// ✅ Drain deployed scanner batteries (passive battery consumption)
func (s *Scheduler) drainDeployedScannerBatteries() {
	log.Printf("🔋 Starting battery drain process for deployed scanners...")

	// Calculate drain per 5 minutes based on battery drain_rate_per_hour
	// Scheduler runs every 5 minutes, so we need to calculate drain for 5/60 = 1/12 hour
	// drain_per_5min = (drain_rate_per_hour / 12) * 100 (to get percentage)

	// Update battery levels for active deployed scanners using actual battery drain rates
	result := s.db.Exec(`
		UPDATE gameplay.deployed_devices 
		SET 
			battery_level = GREATEST(
				battery_level - (
					COALESCE(
						(ii.properties->>'drain_rate_per_hour')::decimal / 12.0 * 100.0,
						1.0 / 12.0 * 100.0  -- Default 1.0% per hour if not specified
					),
					0.083  -- Minimum 0.083% per 5min (1% per hour)
				),
				0
			),
			updated_at = NOW()
		FROM gameplay.inventory_items ii
		WHERE 
			gameplay.deployed_devices.battery_inventory_id = ii.id
			AND gameplay.deployed_devices.is_active = true 
			AND gameplay.deployed_devices.battery_level > 0
			AND gameplay.deployed_devices.status = 'active'
	`)

	if result.Error != nil {
		log.Printf("❌ Error draining batteries: %v", result.Error)
		return
	}

	affectedRows := result.RowsAffected
	if affectedRows > 0 {
		log.Printf("🔋 Battery drain completed: %d scanners affected (using actual drain rates)", affectedRows)
	}

	// Mark scanners as depleted if battery reached 0
	depletedResult := s.db.Exec(`
		UPDATE gameplay.deployed_devices 
		SET 
			status = 'depleted',
			battery_depleted_at = NOW(),
			updated_at = NOW()
		WHERE 
			is_active = true 
			AND battery_level = 0
			AND status = 'active'
	`)

	if depletedResult.Error != nil {
		log.Printf("❌ Error marking depleted scanners: %v", depletedResult.Error)
		return
	}

	depletedCount := depletedResult.RowsAffected
	if depletedCount > 0 {
		log.Printf("🔋 Marked %d scanners as depleted (battery = 0%%)", depletedCount)
	}

	// Mark scanners as abandoned after 14 days of being depleted
	abandonedResult := s.db.Exec(`
		UPDATE gameplay.deployed_devices 
		SET 
			status = 'abandoned',
			abandoned_at = NOW(),
			updated_at = NOW()
		WHERE 
			is_active = true 
			AND status = 'depleted'
			AND battery_depleted_at IS NOT NULL
			AND battery_depleted_at < NOW() - INTERVAL '14 days'
	`)

	if abandonedResult.Error != nil {
		log.Printf("❌ Error marking abandoned scanners: %v", abandonedResult.Error)
		return
	}

	abandonedCount := abandonedResult.RowsAffected
	if abandonedCount > 0 {
		log.Printf("🔋 Marked %d scanners as abandoned (depleted for 14+ days)", abandonedCount)
	}

	// Mark scanners as destroyed after 21 days total (7 days after being abandoned)
	destroyedResult := s.db.Exec(`
		UPDATE gameplay.deployed_devices 
		SET 
			status = 'destroyed',
			is_active = false,
			updated_at = NOW()
		WHERE 
			status = 'abandoned'
			AND abandoned_at IS NOT NULL
			AND abandoned_at < NOW() - INTERVAL '7 days'
	`)

	if destroyedResult.Error != nil {
		log.Printf("❌ Error marking destroyed scanners: %v", destroyedResult.Error)
		return
	}

	destroyedCount := destroyedResult.RowsAffected
	if destroyedCount > 0 {
		log.Printf("🔋 Marked %d scanners as destroyed (abandoned for 7+ days)", destroyedCount)
	}

	// Note: We don't actually delete destroyed scanners from the database
	// They remain as historical records but are marked as inactive
	// This allows for analytics and prevents data loss
}
