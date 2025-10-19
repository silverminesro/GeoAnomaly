package deployable

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DeployedDevice reprezentuje zariadenie umiestnené na mape
type DeployedDevice struct {
	ID                 uuid.UUID         `json:"id" db:"id" gorm:"primaryKey"`
	OwnerID            uuid.UUID         `json:"owner_id" db:"owner_id" gorm:"not null"`                       // FK na auth.users je v DB
	DeviceInventoryID  uuid.UUID         `json:"device_inventory_id" db:"device_inventory_id" gorm:"not null"` // FK na gameplay.inventory_items je v DB
	BatteryInventoryID *uuid.UUID        `json:"battery_inventory_id" db:"battery_inventory_id"`               // FK na gameplay.inventory_items je v DB (nullable pre vybraté batérie)
	BatteryStatus      *string           `json:"battery_status" db:"battery_status"`                           // installed, removed, depleted
	Name               string            `json:"name" db:"name"`
	Latitude           float64           `json:"latitude" db:"latitude"`
	Longitude          float64           `json:"longitude" db:"longitude"`
	DeployedAt         time.Time         `json:"deployed_at" db:"deployed_at"`
	LastScanAt         *time.Time        `json:"last_scan_at" db:"last_scan_at"`
	LastAccessedAt     *time.Time        `json:"last_accessed_at" db:"last_accessed_at"`
	IsActive           bool              `json:"is_active" db:"is_active"`
	BatteryLevel       *int              `json:"battery_level" db:"battery_level"` // 0-100%
	BatteryDepletedAt  *time.Time        `json:"battery_depleted_at" db:"battery_depleted_at"`
	AbandonedAt        *time.Time        `json:"abandoned_at" db:"abandoned_at"`
	LastDisabledAt     *time.Time        `json:"last_disabled_at" db:"last_disabled_at"`
	Status             DeviceStatus      `json:"status" db:"status"`
	HackResistance     int               `json:"hack_resistance" db:"hack_resistance"`         // 1-10
	ScanRadiusKm       float64           `json:"scan_radius_km" db:"scan_radius_km"`           // Scanning radius in kilometers
	MaxRarityDetected  string            `json:"max_rarity_detected" db:"max_rarity_detected"` // Maximum rarity level that can be detected
	Properties         datatypes.JSONMap `json:"properties" db:"properties" gorm:"type:jsonb"`
	CreatedAt          time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at" db:"updated_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (DeployedDevice) TableName() string {
	return "gameplay.deployed_devices"
}

// DeviceStatus - status zariadenia
type DeviceStatus string

const (
	DeviceStatusActive    DeviceStatus = "active"
	DeviceStatusDepleted  DeviceStatus = "depleted"
	DeviceStatusAbandoned DeviceStatus = "abandoned"
	DeviceStatusDestroyed DeviceStatus = "destroyed"
)

// DeviceHack reprezentuje pokus o hack
type DeviceHack struct {
	ID              uuid.UUID `json:"id" db:"id" gorm:"primaryKey"`
	DeviceID        uuid.UUID `json:"device_id" db:"device_id"`
	HackerID        uuid.UUID `json:"hacker_id" db:"hacker_id"`
	HackTime        time.Time `json:"hack_time" db:"hack_time"`
	Success         bool      `json:"success" db:"success"`
	HackToolUsed    string    `json:"hack_tool_used" db:"hack_tool_used"`
	DistanceM       float64   `json:"distance_m" db:"distance_m"`
	HackDurationSec int       `json:"hack_duration_seconds" db:"hack_duration_seconds"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (DeviceHack) TableName() string {
	return "gameplay.device_hacks"
}

// HackTool reprezentuje hackovací nástroj v inventári hráča
type HackTool struct {
	ID         uuid.UUID         `json:"id" db:"id" gorm:"primaryKey"`
	UserID     uuid.UUID         `json:"user_id" db:"user_id"`
	ToolType   string            `json:"tool_type" db:"tool_type"`
	Name       string            `json:"name" db:"name"`
	UsesLeft   int               `json:"uses_left" db:"uses_left"`
	ExpiresAt  *time.Time        `json:"expires_at" db:"expires_at"`
	Properties datatypes.JSONMap `json:"properties" db:"properties" gorm:"type:jsonb"`
	CreatedAt  time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at" db:"updated_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (HackTool) TableName() string {
	return "gameplay.hack_tools"
}

// DeviceAccess reprezentuje prístupové práva k zariadeniam (po hackovaní)
type DeviceAccess struct {
	ID               uuid.UUID  `json:"id" db:"id" gorm:"primaryKey"`
	DeviceID         uuid.UUID  `json:"device_id" db:"device_id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	GrantedAt        time.Time  `json:"granted_at" db:"granted_at"`
	ExpiresAt        time.Time  `json:"expires_at" db:"expires_at"`
	AccessLevel      string     `json:"access_level" db:"access_level"`
	GrantedByHackID  *uuid.UUID `json:"granted_by_hack_id" db:"granted_by_hack_id"`
	IsDeviceDisabled bool       `json:"is_device_disabled" db:"is_device_disabled"`
	DisabledUntil    *time.Time `json:"disabled_until" db:"disabled_until"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (DeviceAccess) TableName() string {
	return "gameplay.device_access"
}

// DeviceScanHistory reprezentuje skenovacie histórie zariadení
type DeviceScanHistory struct {
	ID              uuid.UUID         `json:"id" db:"id" gorm:"primaryKey"`
	DeviceID        uuid.UUID         `json:"device_id" db:"device_id"`
	ScannedByUserID uuid.UUID         `json:"scanned_by_user_id" db:"scanned_by_user_id"`
	ScanTime        time.Time         `json:"scan_time" db:"scan_time"`
	ScanResults     datatypes.JSONMap `json:"scan_results" db:"scan_results" gorm:"type:jsonb"`
	ScanRadiusKm    float64           `json:"scan_radius_km" db:"scan_radius_km"`
	ItemsFound      int               `json:"items_found" db:"items_found"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (DeviceScanHistory) TableName() string {
	return "gameplay.device_scan_history"
}

// ScanCooldown reprezentuje cooldown pre skenovanie
type ScanCooldown struct {
	ID                  uuid.UUID `json:"id" db:"id" gorm:"primaryKey"`
	DeviceID            uuid.UUID `json:"device_id" db:"device_id" gorm:"uniqueIndex:idx_scan_cooldowns_device_user"`
	UserID              uuid.UUID `json:"user_id" db:"user_id" gorm:"uniqueIndex:idx_scan_cooldowns_device_user"`
	LastScanAt          time.Time `json:"last_scan_at" db:"last_scan_at"`
	CooldownUntil       time.Time `json:"cooldown_until" db:"cooldown_until"`
	CooldownDurationSec int       `json:"cooldown_duration_seconds" db:"cooldown_duration_seconds"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (ScanCooldown) TableName() string {
	return "gameplay.scan_cooldowns"
}

// DeployableScanRequest - request na skenovanie deployable zariadenia
type DeployableScanRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

// DeployableScanResponse - response zo skenovania deployable zariadenia
type DeployableScanResponse struct {
	Success       bool                   `json:"success"`
	Message       string                 `json:"message,omitempty"`
	ItemsFound    int                    `json:"items_found"`
	ScanResults   []DeployableScanResult `json:"scan_results,omitempty"`
	CooldownUntil *time.Time             `json:"cooldown_until,omitempty"`
}

// DeployableScanResult - výsledok skenovania deployable zariadenia
type DeployableScanResult struct {
	Type           string  `json:"type"`
	Name           string  `json:"name"`
	Rarity         string  `json:"rarity"`
	DistanceM      int     `json:"distance_m"`
	BearingDeg     float64 `json:"bearing_deg"`
	SignalStrength float64 `json:"signal_strength"`
}

// HackResponse - response z hackovania zariadenia
type HackResponse struct {
	Success              bool       `json:"hack_successful"`
	AccessGrantedUntil   *time.Time `json:"access_granted_until,omitempty"`
	DeviceDisabled       bool       `json:"device_disabled"`
	DeviceDisabledUntil  *time.Time `json:"device_disabled_until,omitempty"`
	OwnershipTransferred bool       `json:"ownership_transferred"`
	NewOwnerID           *uuid.UUID `json:"new_owner_id,omitempty"`
	BatteryReplaced      bool       `json:"battery_replaced,omitempty"`
	DeviceStatus         string     `json:"device_status,omitempty"`
}

// DeployRequest - request na deploy zariadenia
type DeployRequest struct {
	DeviceInventoryID  uuid.UUID `json:"device_inventory_id" binding:"required"`
	BatteryInventoryID uuid.UUID `json:"battery_inventory_id" binding:"required"`
	Name               string    `json:"name" binding:"required"`
	Latitude           float64   `json:"latitude" binding:"required"`
	Longitude          float64   `json:"longitude" binding:"required"`
}

// DeployResponse - response z deploy zariadenia
type DeployResponse struct {
	Success    bool      `json:"success"`
	Message    string    `json:"message,omitempty"`
	DeviceID   uuid.UUID `json:"device_id,omitempty"`
	DeviceName string    `json:"device_name,omitempty"`
}

// HackRequest - request na hack zariadenia
type HackRequest struct {
	HackToolID uuid.UUID `json:"hack_tool_id" binding:"required"`
	Latitude   float64   `json:"latitude" binding:"required"`
	Longitude  float64   `json:"longitude" binding:"required"`
}

// ClaimRequest - request na claim opusteného zariadenia
type ClaimRequest struct {
	HackToolID uuid.UUID `json:"hack_tool_id" binding:"required"`
	Latitude   float64   `json:"latitude" binding:"required"`
	Longitude  float64   `json:"longitude" binding:"required"`
}

// ClaimResponse - response z claim zariadenia
type ClaimResponse struct {
	Success         bool      `json:"claim_successful"`
	NewOwnerID      uuid.UUID `json:"new_owner_id,omitempty"`
	BatteryReplaced bool      `json:"battery_replaced,omitempty"`
	DeviceStatus    string    `json:"device_status,omitempty"`
}

// CooldownStatus - status cooldownu
type CooldownStatus struct {
	CanScan             bool       `json:"can_scan"`
	CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
	RemainingSeconds    int        `json:"remaining_seconds"`
	CooldownDurationSec int        `json:"cooldown_duration_seconds,omitempty"`
	Reason              string     `json:"reason,omitempty"`
}

// DeviceListResponse - response pre zoznam zariadení
type DeviceListResponse struct {
	MyDevices        []DeployedDevice `json:"my_devices"`
	NearbyDevices    []DeployedDevice `json:"nearby_devices"`
	AbandonedDevices []DeployedDevice `json:"abandoned_devices"`
}

// MapMarker - ikona na mape
type MapMarker struct {
	ID             uuid.UUID  `json:"id"`
	Type           string     `json:"type"`   // "deployed_scanner"
	Status         string     `json:"status"` // "active", "abandoned", "hacked", "battery_dead"
	Latitude       float64    `json:"latitude"`
	Longitude      float64    `json:"longitude"`
	Icon           string     `json:"icon"` // "scanner_green", "scanner_gray", "scanner_blue", "scanner_red", "scanner_dark_gray"
	BatteryLevel   int        `json:"battery_level"`
	ScanRadiusKm   float64    `json:"scan_radius_km"`
	CanHack        bool       `json:"can_hack"`
	CanScan        bool       `json:"can_scan"`
	CanClaim       bool       `json:"can_claim"`
	CooldownUntil  *time.Time `json:"cooldown_until,omitempty"`
	OwnerID        *uuid.UUID `json:"owner_id,omitempty"`
	HackedBy       *uuid.UUID `json:"hacked_by,omitempty"`
	DistanceKm     float64    `json:"distance_km"`
	VisibilityType string     `json:"visibility_type"` // "owner", "hacker", "public", "scan_data"
}

// MapMarkersResponse - response pre mapové markery
type MapMarkersResponse struct {
	Markers []MapMarker `json:"markers"`
}

// Value a Scan pre JSONB - removed as they are not needed for map[string]any
