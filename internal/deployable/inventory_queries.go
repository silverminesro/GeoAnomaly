package deployable

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InventoryQueries obsahuje praktické SELECT-y pre inventárny systém
type InventoryQueries struct {
	db *gorm.DB
}

// NewInventoryQueries vytvorí novú inštanciu InventoryQueries
func NewInventoryQueries(db *gorm.DB) *InventoryQueries {
	return &InventoryQueries{db: db}
}

// GetPlayerScanners vráti všetky scannery, ktoré hráč vlastní v inventári
func (iq *InventoryQueries) GetPlayerScanners(userID uuid.UUID) ([]PlayerScanner, error) {
	var scanners []PlayerScanner

	query := `
		SELECT 
			ii.id AS inventory_id,
			ii.item_type,
			ii.properties,
			ii.quantity,
			ii.acquired_at,
			-- Skontroluj či je scanner aktívne nasadený
			dd.id IS NOT NULL as is_deployed,
			dd.name as deployed_name,
			dd.status as deployed_status,
			dd.battery_level,
			dd.latitude,
			dd.longitude
		FROM gameplay.inventory_items ii
		LEFT JOIN gameplay.deployed_devices dd ON dd.device_inventory_id = ii.id AND dd.is_active = TRUE
		WHERE ii.user_id = ? 
			AND ii.item_type = 'deployable_scanner' 
			AND ii.deleted_at IS NULL
		ORDER BY ii.acquired_at DESC
	`

	err := iq.db.Raw(query, userID).Scan(&scanners).Error
	return scanners, err
}

// GetPlayerBatteries vráti všetky batérie, ktoré hráč vlastní v inventári
func (iq *InventoryQueries) GetPlayerBatteries(userID uuid.UUID) ([]PlayerBattery, error) {
	var batteries []PlayerBattery

	query := `
		SELECT 
			ii.id AS inventory_id,
			ii.item_type,
			ii.properties,
			ii.quantity,
			ii.acquired_at,
			-- Skontroluj či je batéria aktívne použitá
			dd.id IS NOT NULL as is_in_use,
			dd.name as device_name,
			dd.battery_level
		FROM gameplay.inventory_items ii
		LEFT JOIN gameplay.deployed_devices dd ON dd.battery_inventory_id = ii.id AND dd.is_active = TRUE
		WHERE ii.user_id = ? 
			AND ii.item_type = 'scanner_battery' 
			AND ii.deleted_at IS NULL
		ORDER BY ii.acquired_at DESC
	`

	err := iq.db.Raw(query, userID).Scan(&batteries).Error
	return batteries, err
}

// GetPlayerHackTools vráti všetky hackovacie nástroje, ktoré hráč vlastní v inventári
func (iq *InventoryQueries) GetPlayerHackTools(userID uuid.UUID) ([]PlayerHackTool, error) {
	var tools []PlayerHackTool

	query := `
		SELECT 
			ii.id AS inventory_id,
			ii.item_type,
			ii.properties,
			ii.quantity,
			ii.acquired_at
		FROM gameplay.inventory_items ii
		WHERE ii.user_id = ? 
			AND ii.item_type = 'hack_tool' 
			AND ii.deleted_at IS NULL
		ORDER BY ii.acquired_at DESC
	`

	err := iq.db.Raw(query, userID).Scan(&tools).Error
	return tools, err
}

// ValidateDeploymentInventory skontroluje, či hráč má potrebné items v inventári pre deploy
func (iq *InventoryQueries) ValidateDeploymentInventory(userID, deviceInventoryID, batteryInventoryID uuid.UUID) error {
	// Skontroluj či hráč vlastní scanner
	var deviceCount int64
	err := iq.db.Model(&InventoryItem{}).
		Where("id = ? AND user_id = ? AND item_type = 'deployable_scanner' AND deleted_at IS NULL",
			deviceInventoryID, userID).
		Count(&deviceCount).Error
	if err != nil {
		return fmt.Errorf("chyba pri kontrole scanneru v inventári: %w", err)
	}
	if deviceCount == 0 {
		return fmt.Errorf("scanner nie je v tvojom inventári")
	}

	// Skontroluj či hráč vlastní batériu
	var batteryCount int64
	err = iq.db.Model(&InventoryItem{}).
		Where("id = ? AND user_id = ? AND item_type = 'scanner_battery' AND deleted_at IS NULL",
			batteryInventoryID, userID).
		Count(&batteryCount).Error
	if err != nil {
		return fmt.Errorf("chyba pri kontrole batérie v inventári: %w", err)
	}
	if batteryCount == 0 {
		return fmt.Errorf("batéria nie je v tvojom inventári")
	}

	// Skontroluj či scanner nie je už nasadený (iba aktívne zariadenia, nie depleted/abandoned)
	var deployedCount int64
	err = iq.db.Model(&DeployedDevice{}).
		Where("device_inventory_id = ? AND is_active = TRUE AND status IN ('active')", deviceInventoryID).
		Count(&deployedCount).Error
	if err != nil {
		return fmt.Errorf("chyba pri kontrole nasadenia scanneru: %w", err)
	}
	if deployedCount > 0 {
		return fmt.Errorf("scanner je už aktívne nasadený")
	}

	// Skontroluj či batéria nie je už použitá
	var batteryInUseCount int64
	err = iq.db.Model(&DeployedDevice{}).
		Where("battery_inventory_id = ? AND is_active = TRUE", batteryInventoryID).
		Count(&batteryInUseCount).Error
	if err != nil {
		return fmt.Errorf("chyba pri kontrole použitia batérie: %w", err)
	}
	if batteryInUseCount > 0 {
		return fmt.Errorf("batéria je už použitá v inom zariadení")
	}

	return nil
}

// GetActiveDeployedDevices vráti všetky aktívne nasadené zariadenia hráča
func (iq *InventoryQueries) GetActiveDeployedDevices(userID uuid.UUID) ([]DeployedDevice, error) {
	var devices []DeployedDevice

	err := iq.db.Where("owner_id = ? AND is_active = TRUE", userID).Find(&devices).Error
	return devices, err
}

// PlayerScanner reprezentuje scanner v inventári hráča
type PlayerScanner struct {
	InventoryID    uuid.UUID       `json:"inventory_id" db:"inventory_id"`
	ItemType       string          `json:"item_type" db:"item_type"`
	Properties     sql.NullString  `json:"properties" db:"properties"`
	Quantity       int             `json:"quantity" db:"quantity"`
	AcquiredAt     sql.NullTime    `json:"acquired_at" db:"acquired_at"`
	IsDeployed     bool            `json:"is_deployed" db:"is_deployed"`
	DeployedName   sql.NullString  `json:"deployed_name" db:"deployed_name"`
	DeployedStatus sql.NullString  `json:"deployed_status" db:"deployed_status"`
	BatteryLevel   sql.NullInt32   `json:"battery_level" db:"battery_level"`
	Latitude       sql.NullFloat64 `json:"latitude" db:"latitude"`
	Longitude      sql.NullFloat64 `json:"longitude" db:"longitude"`
}

// PlayerBattery reprezentuje batériu v inventári hráča
type PlayerBattery struct {
	InventoryID  uuid.UUID      `json:"inventory_id" db:"inventory_id"`
	ItemType     string         `json:"item_type" db:"item_type"`
	Properties   sql.NullString `json:"properties" db:"properties"`
	Quantity     int            `json:"quantity" db:"quantity"`
	AcquiredAt   sql.NullTime   `json:"acquired_at" db:"acquired_at"`
	IsInUse      bool           `json:"is_in_use" db:"is_in_use"`
	DeviceName   sql.NullString `json:"device_name" db:"device_name"`
	BatteryLevel sql.NullInt32  `json:"battery_level" db:"battery_level"`
}

// PlayerHackTool reprezentuje hackovací nástroj v inventári hráča
type PlayerHackTool struct {
	InventoryID uuid.UUID      `json:"inventory_id" db:"inventory_id"`
	ItemType    string         `json:"item_type" db:"item_type"`
	Properties  sql.NullString `json:"properties" db:"properties"`
	Quantity    int            `json:"quantity" db:"quantity"`
	AcquiredAt  sql.NullTime   `json:"acquired_at" db:"acquired_at"`
}

// InventoryItem - alias pre gameplay.InventoryItem (import z gameplay/models.go)
type InventoryItem struct {
	ID         uuid.UUID    `gorm:"primaryKey"`
	UserID     uuid.UUID    `gorm:"not null;index"`
	ItemType   string       `gorm:"not null;size:50"`
	ItemID     uuid.UUID    `gorm:"not null"`
	Properties string       `gorm:"type:jsonb;default:'{}'::jsonb"`
	Quantity   int          `gorm:"default:1"`
	AcquiredAt sql.NullTime `gorm:"autoCreateTime"`
	DeletedAt  sql.NullTime `gorm:"index"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (InventoryItem) TableName() string {
	return "gameplay.inventory_items"
}
