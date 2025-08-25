package database

import (
	"geoanomaly/internal/common"
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
		&common.User{},
		&common.Zone{},
		&common.InventoryItem{},
		&common.Artifact{},
		&common.Gear{},
		&common.PlayerSession{},
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
