package scanner

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// StringArray - custom typ pre JSONB string arrays
type StringArray []string

// Value a Scan pre JSONB string arrays
func (sa StringArray) Value() (driver.Value, error) {
	return json.Marshal(sa)
}

func (sa *StringArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sa)
}

// ScannerCatalog - definícia scanner typu
type ScannerCatalog struct {
	ID             uuid.UUID   `json:"id" db:"id" gorm:"primaryKey"`
	Code           string      `json:"code" db:"code" gorm:"uniqueIndex"`
	Name           string      `json:"name" db:"name"`
	Tagline        string      `json:"tagline" db:"tagline"`
	Description    string      `json:"description" db:"description"`
	BaseRangeM     int         `json:"base_range_m" db:"base_range_m"`
	BaseFovDeg     int         `json:"base_fov_deg" db:"base_fov_deg"`
	CapsJSON       ScannerCaps `json:"caps_json" db:"caps_json" gorm:"type:jsonb"`
	DrainMult      float64     `json:"drain_mult" db:"drain_mult"`
	AllowedModules StringArray `json:"allowed_modules" db:"allowed_modules" gorm:"type:jsonb"`
	SlotCount      int         `json:"slot_count" db:"slot_count"`
	SlotTypes      StringArray `json:"slot_types" db:"slot_types" gorm:"type:jsonb"`
	IsBasic        bool        `json:"is_basic" db:"is_basic"`
	// Scanner detection capabilities
	MaxRarity       string    `json:"max_rarity" db:"max_rarity"`             // Najvyššia rarity ktorú môže detekovať
	DetectArtifacts bool      `json:"detect_artifacts" db:"detect_artifacts"` // Môže detekovať artefakty
	DetectGear      bool      `json:"detect_gear" db:"detect_gear"`           // Môže detekovať gear
	Version         int       `json:"version" db:"version"`
	EffectiveFrom   time.Time `json:"effective_from" db:"effective_from"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (ScannerCatalog) TableName() string {
	return "scanner_catalog"
}

// ScannerCaps - limity scanner
type ScannerCaps struct {
	RangePctMax     int     `json:"range_pct_max"`
	FovPctMax       int     `json:"fov_pct_max"`
	ServerPollHzMax float64 `json:"server_poll_hz_max"`
}

// Value a Scan pre JSONB
func (sc ScannerCaps) Value() (driver.Value, error) {
	return json.Marshal(sc)
}

func (sc *ScannerCaps) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sc)
}

// ModuleCatalog - definícia modulu
type ModuleCatalog struct {
	ID                 uuid.UUID     `json:"id" db:"id" gorm:"primaryKey"`
	Code               string        `json:"code" db:"code" gorm:"uniqueIndex"`
	Name               string        `json:"name" db:"name"`
	Type               string        `json:"type" db:"type"`
	EffectsJSON        ModuleEffects `json:"effects_json" db:"effects_json" gorm:"type:jsonb"`
	EnergyCost         int           `json:"energy_cost" db:"energy_cost"`
	DrainMult          float64       `json:"drain_mult" db:"drain_mult"`
	CompatibleScanners StringArray   `json:"compatible_scanners" db:"compatible_scanners" gorm:"type:jsonb"`
	CraftJSON          *CraftRecipe  `json:"craft_json" db:"craft_json" gorm:"type:jsonb"`
	StorePrice         int           `json:"store_price" db:"store_price"`
	Version            int           `json:"version" db:"version"`
	CreatedAt          time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at" db:"updated_at"`
}

// TableName - explicitne špecifikuje názov tabuľky pre GORM
func (ModuleCatalog) TableName() string {
	return "module_catalog"
}

// ModuleEffects - účinky modulu
type ModuleEffects struct {
	RangePct             *int     `json:"range_pct,omitempty"`
	FovPct               *int     `json:"fov_pct,omitempty"`
	ServerPollHzAdd      *float64 `json:"server_poll_hz_add,omitempty"`
	LockOnThresholdDelta *float64 `json:"lock_on_threshold_delta,omitempty"`
	OffSectorHint        *bool    `json:"off_sector_hint,omitempty"`
	HapticsBoost         *bool    `json:"haptics_boost,omitempty"`
	TurnHintDistanceM    *int     `json:"turn_hint_distance_m,omitempty"`
}

// Value a Scan pre JSONB
func (me ModuleEffects) Value() (driver.Value, error) {
	return json.Marshal(me)
}

func (me *ModuleEffects) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, me)
}

// PowerCellCatalog - definícia power cell
type PowerCellCatalog struct {
	Code         string       `json:"code" db:"code"`
	Name         string       `json:"name" db:"name"`
	BaseMinutes  int          `json:"base_minutes" db:"base_minutes"`
	CraftJSON    *CraftRecipe `json:"craft_json" db:"craft_json"`
	PriceCredits int          `json:"price_credits" db:"price_credits"`
	Version      int          `json:"version" db:"version"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// CraftRecipe - recept na craftovanie
type CraftRecipe struct {
	Lab       string         `json:"lab"`
	Credits   int            `json:"credits"`
	Materials map[string]int `json:"materials"`
}

// Value a Scan pre JSONB
func (cr CraftRecipe) Value() (driver.Value, error) {
	return json.Marshal(cr)
}

func (cr *CraftRecipe) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, cr)
}

// ScannerInstance - hráčova scanner inštancia
type ScannerInstance struct {
	ID                   uuid.UUID  `json:"id" db:"id"`
	OwnerID              uuid.UUID  `json:"owner_id" db:"owner_id"`
	ScannerCode          string     `json:"scanner_code" db:"scanner_code"`
	EnergyCap            *int       `json:"energy_cap" db:"energy_cap"`
	PowerCellCode        *string    `json:"power_cell_code" db:"power_cell_code"`
	PowerCellStartedAt   *time.Time `json:"power_cell_started_at" db:"power_cell_started_at"`
	PowerCellMinutesLeft *int       `json:"power_cell_minutes_left" db:"power_cell_minutes_left"`
	IsActive             bool       `json:"is_active" db:"is_active"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`

	// Computed fields
	Scanner *ScannerCatalog `json:"scanner,omitempty"`
	Modules []ScannerModule `json:"modules,omitempty"`
}

// ScannerModule - inštalovaný modul
type ScannerModule struct {
	InstanceID  uuid.UUID `json:"instance_id" db:"instance_id"`
	SlotIndex   int       `json:"slot_index" db:"slot_index"`
	ModuleCode  string    `json:"module_code" db:"module_code"`
	InstalledAt time.Time `json:"installed_at" db:"installed_at"`

	// Computed fields
	Module *ModuleCatalog `json:"module,omitempty"`
}

// ScannerStats - efektívne stats scanner
type ScannerStats struct {
	RangeM           int     `json:"range_m"`
	FovDeg           int     `json:"fov_deg"`
	ServerPollHz     float64 `json:"server_poll_hz"`
	LockOnThreshold  float64 `json:"lock_on_threshold"`
	EnergyCap        int     `json:"energy_cap"`
	PowerCellMinutes *int    `json:"power_cell_minutes,omitempty"`
}

// ScanRequest - request na skenovanie
type ScanRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Heading   float64 `json:"heading"` // Odstránené required, lebo 0.0 je platná hodnota
}

// ScanResponse - response zo skenovania
type ScanResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
	ScanResults  []ScanResult  `json:"scan_results,omitempty"`
	ScannerStats *ScannerStats `json:"scanner_stats,omitempty"`
}

// ScanResult - výsledok skenovania
type ScanResult struct {
	Type           string     `json:"type"` // artifact, gear, consumable, cache, beacon
	DistanceM      int        `json:"distance_m"`
	BearingDeg     float64    `json:"bearing_deg"`
	SignalStrength float64    `json:"signal_strength"`
	ItemID         *uuid.UUID `json:"item_id,omitempty"`
	Name           string     `json:"name,omitempty"`
	Rarity         string     `json:"rarity,omitempty"`
}
