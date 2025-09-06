package database

import (
	"geoanomaly/internal/auth"
	"geoanomaly/internal/deployable"
	"geoanomaly/internal/gameplay"
	"geoanomaly/internal/menu"
	"geoanomaly/internal/scanner"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	// Enable PostGIS extension
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS postgis").Error; err != nil {
		return err
	}

	// Auto-migrate all models
	err := db.AutoMigrate(
		&auth.User{},
		&gameplay.Zone{},
		&gameplay.InventoryItem{},
		&gameplay.Artifact{},
		&gameplay.Gear{},
		&auth.PlayerSession{},
		// Menu models
		&menu.Currency{},
		&menu.Transaction{},
		&menu.MarketItem{},
		&menu.UserPurchase{},
		&menu.EssencePackage{},
		&menu.UserEssencePurchase{},
		// Scanner models
		&scanner.ScannerCatalog{},
		&scanner.ModuleCatalog{},
		&scanner.PowerCellCatalog{},
		&scanner.ScannerInstance{},
		&scanner.ScannerModule{},
		// Deployable Scanner models
		&deployable.DeployedDevice{},
		&deployable.DeviceHack{},
		&deployable.HackTool{},
		&deployable.DeviceAccess{},
		&deployable.DeviceScanHistory{},
		&deployable.ScanCooldown{},
	)

	if err != nil {
		return err
	}

	// ✅ UPDATED: Enhanced spatial indexes with biome support
	if err := createSpatialIndexes(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Create biome-specific indexes
	if err := createBiomeIndexes(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Create menu-specific indexes
	if err := createMenuIndexes(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Create scanner-specific indexes
	if err := createScannerIndexes(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Create deployable scanner-specific indexes
	if err := createDeployableScannerIndexes(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Add missing constraints and indexes
	if err := addMissingConstraints(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Add PostGIS GEOGRAPHY column for deployed_devices
	if err := addGeographyColumn(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Add last_disabled_at column for deployed_devices
	if err := addLastDisabledAtColumn(db); err != nil {
		return err
	}

	// ✅ PRIDANÉ: Add scan_radius_km and max_rarity_detected columns for deployed_devices
	if err := addScanPropertiesColumns(db); err != nil {
		return err
	}

	return nil
}

func createSpatialIndexes(db *gorm.DB) error {
	// Index for zones location
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_zones_location 
		ON zones USING GIST (ST_Point(location_longitude, location_latitude))
	`).Error; err != nil {
		return err
	}

	// Index for artifacts location
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_artifacts_location 
		ON artifacts USING GIST (ST_Point(location_longitude, location_latitude))
	`).Error; err != nil {
		return err
	}

	// Index for gear location
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_gear_location 
		ON gear USING GIST (ST_Point(location_longitude, location_latitude))
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ SIMPLIFIED: Biome-specific database indexes (bez environmental_effects)
func createBiomeIndexes(db *gorm.DB) error {
	// Index for zones by biome
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_zones_biome 
		ON zones (biome)
	`).Error; err != nil {
		return err
	}

	// Index for zones by danger level
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_zones_danger_level 
		ON zones (danger_level)
	`).Error; err != nil {
		return err
	}

	// Index for artifacts by biome
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_artifacts_biome 
		ON artifacts (biome)
	`).Error; err != nil {
		return err
	}

	// Index for gear by biome
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_gear_biome 
		ON gear (biome)
	`).Error; err != nil {
		return err
	}

	// Index for exclusive biome items
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_artifacts_exclusive_biome 
		ON artifacts (exclusive_to_biome, biome)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_gear_exclusive_biome 
		ON gear (exclusive_to_biome, biome)
	`).Error; err != nil {
		return err
	}

	// Combined index for zone filtering
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_zones_tier_biome 
		ON zones (tier_required, biome, is_active)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Menu-specific database indexes
func createMenuIndexes(db *gorm.DB) error {
	// Index for currencies by user and type
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_currencies_user_type 
		ON currencies (user_id, type)
	`).Error; err != nil {
		return err
	}

	// Index for transactions by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_transactions_user 
		ON transactions (user_id, created_at DESC)
	`).Error; err != nil {
		return err
	}

	// Index for market items by category and rarity
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_market_items_category_rarity 
		ON market_items (category, rarity, is_active)
	`).Error; err != nil {
		return err
	}

	// Index for user purchases by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_user_purchases_user 
		ON user_purchases (user_id, created_at DESC)
	`).Error; err != nil {
		return err
	}

	// Index for essence packages by active status
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_essence_packages_active 
		ON essence_packages (is_active, is_popular)
	`).Error; err != nil {
		return err
	}

	// Index for user essence purchases by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_user_essence_purchases_user 
		ON user_essence_purchases (user_id, created_at DESC)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Scanner-specific database indexes
func createScannerIndexes(db *gorm.DB) error {
	// Unique index for scanner_catalog.code
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_scanner_catalog_code 
		ON scanner_catalog (code)
	`).Error; err != nil {
		return err
	}

	// Unique index for module_catalog.code
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_module_catalog_code 
		ON module_catalog (code)
	`).Error; err != nil {
		return err
	}

	// Index for scanner_instances.owner_id
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scanner_instances_owner_id 
		ON scanner_instances (owner_id, is_active)
	`).Error; err != nil {
		return err
	}

	// GIN index for scanner_catalog.caps_json
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scanner_catalog_caps_gin 
		ON scanner_catalog USING GIN (caps_json jsonb_path_ops)
	`).Error; err != nil {
		return err
	}

	// Index for scanner_modules by instance
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scanner_modules_instance 
		ON scanner_modules (instance_id, slot_index)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Deployable Scanner-specific database indexes
func createDeployableScannerIndexes(db *gorm.DB) error {
	// Index for deployed_devices by owner
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_owner 
		ON deployed_devices (owner_id)
	`).Error; err != nil {
		return err
	}

	// Spatial index for deployed_devices location
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_location 
		ON deployed_devices USING GIST (location)
	`).Error; err != nil {
		return err
	}

	// Index for active deployed devices
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_active 
		ON deployed_devices (is_active) WHERE is_active = true
	`).Error; err != nil {
		return err
	}

	// Index for deployed devices by status
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_status 
		ON deployed_devices (status)
	`).Error; err != nil {
		return err
	}

	// Index for deployed devices with battery
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_battery 
		ON deployed_devices (battery_level) WHERE battery_level > 0
	`).Error; err != nil {
		return err
	}

	// Index for device hacks by device
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_hacks_device 
		ON device_hacks (device_id)
	`).Error; err != nil {
		return err
	}

	// Index for device hacks by hacker
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_hacks_hacker 
		ON device_hacks (hacker_id)
	`).Error; err != nil {
		return err
	}

	// Index for device access by device
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_access_device 
		ON device_access (device_id)
	`).Error; err != nil {
		return err
	}

	// Index for device access by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_access_user 
		ON device_access (user_id)
	`).Error; err != nil {
		return err
	}

	// Index for active device access
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_access_expires 
		ON device_access (expires_at) WHERE expires_at > NOW()
	`).Error; err != nil {
		return err
	}

	// Unique index for scan cooldowns
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_cooldowns_device_user 
		ON scan_cooldowns (device_id, user_id)
	`).Error; err != nil {
		return err
	}

	// Index for active scan cooldowns
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scan_cooldowns_until 
		ON scan_cooldowns (cooldown_until) WHERE cooldown_until > NOW()
	`).Error; err != nil {
		return err
	}

	// Index for hack tools by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_hack_tools_user 
		ON hack_tools (user_id)
	`).Error; err != nil {
		return err
	}

	// Index for device scan history by device
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_scan_history_device 
		ON device_scan_history (device_id)
	`).Error; err != nil {
		return err
	}

	// Index for device scan history by user
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_scan_history_user 
		ON device_scan_history (scanned_by_user_id)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Add missing constraints and indexes
func addMissingConstraints(db *gorm.DB) error {
	// Add unique constraint for scan_cooldowns
	if err := db.Exec(`
		ALTER TABLE scan_cooldowns 
		ADD CONSTRAINT IF NOT EXISTS uq_scan_cooldowns_device_user 
		UNIQUE (device_id, user_id)
	`).Error; err != nil {
		return err
	}

	// Add sanity check for device_access
	if err := db.Exec(`
		ALTER TABLE device_access 
		ADD CONSTRAINT IF NOT EXISTS device_access_valid_range 
		CHECK (expires_at > granted_at)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Add PostGIS GEOGRAPHY column for deployed_devices
func addGeographyColumn(db *gorm.DB) error {
	// Add GEOGRAPHY column if it doesn't exist
	if err := db.Exec(`
		ALTER TABLE deployed_devices 
		ADD COLUMN IF NOT EXISTS location GEOGRAPHY(POINT, 4326) GENERATED ALWAYS AS (
			ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
		) STORED
	`).Error; err != nil {
		return err
	}

	// Update the spatial index to use the new GEOGRAPHY column
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_deployed_devices_location
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deployed_devices_location 
		ON deployed_devices USING GIST (location)
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Add last_disabled_at column for deployed_devices
func addLastDisabledAtColumn(db *gorm.DB) error {
	// Add last_disabled_at column if it doesn't exist
	if err := db.Exec(`
		ALTER TABLE deployed_devices 
		ADD COLUMN IF NOT EXISTS last_disabled_at TIMESTAMP WITH TIME ZONE
	`).Error; err != nil {
		return err
	}

	return nil
}

// ✅ PRIDANÉ: Add scan_radius_km and max_rarity_detected columns for deployed_devices
func addScanPropertiesColumns(db *gorm.DB) error {
	// Add scan_radius_km column if it doesn't exist
	if err := db.Exec(`
		ALTER TABLE deployed_devices 
		ADD COLUMN IF NOT EXISTS scan_radius_km DECIMAL(5, 2) DEFAULT 1.0 
		CHECK (scan_radius_km > 0 AND scan_radius_km <= 10)
	`).Error; err != nil {
		return err
	}

	// Add max_rarity_detected column if it doesn't exist
	if err := db.Exec(`
		ALTER TABLE deployed_devices 
		ADD COLUMN IF NOT EXISTS max_rarity_detected VARCHAR(20) DEFAULT 'common' 
		CHECK (max_rarity_detected IN ('common', 'rare', 'epic', 'legendary'))
	`).Error; err != nil {
		return err
	}

	return nil
}
