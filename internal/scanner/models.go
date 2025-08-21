package scanner

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/your_repo/internal/common"
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

	// Puls Scanner specific capabilities
	WaveDurationMs  *int     `json:"wave_duration_ms,omitempty"`  // Duration of wave animation in milliseconds
	EchoDelayMs     *int     `json:"echo_delay_ms,omitempty"`     // Delay before echo signals appear
	MaxWaves        *int     `json:"max_waves,omitempty"`         // Maximum number of concurrent waves
	WaveSpeedMs     *int     `json:"wave_speed_ms,omitempty"`     // Wave propagation speed in m/s
	NoiseLevel      *float64 `json:"noise_level,omitempty"`       // Noise level (0.0 - 1.0)
	RealTimeCapable *bool    `json:"real_time_capable,omitempty"` // Can operate in real-time mode
	AdvancedEcho    *bool    `json:"advanced_echo,omitempty"`     // Has advanced echo processing
	NoiseFilter     *bool    `json:"noise_filter,omitempty"`      // Has built-in noise filtering
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

	// Puls Scanner specific effects
	NoiseReduction   *float64 `json:"noise_reduction,omitempty"`   // Reduces noise level
	EchoClarity      *float64 `json:"echo_clarity,omitempty"`      // Improves echo signal clarity
	EchoStrength     *float64 `json:"echo_strength,omitempty"`     // Increases echo signal strength
	RangeBoost       *float64 `json:"range_boost,omitempty"`       // Additional range boost
	SignalClarity    *float64 `json:"signal_clarity,omitempty"`    // Improves overall signal clarity
	WavePower        *float64 `json:"wave_power,omitempty"`        // Increases wave power
	Penetration      *float64 `json:"penetration,omitempty"`       // Improves wave penetration
	EnergyEfficiency *float64 `json:"energy_efficiency,omitempty"` // Improves energy efficiency
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

	// Puls Scanner specific stats
	WaveDurationMs  *int     `json:"wave_duration_ms,omitempty"`  // Duration of wave animation in milliseconds
	EchoDelayMs     *int     `json:"echo_delay_ms,omitempty"`     // Delay before echo signals appear
	MaxWaves        *int     `json:"max_waves,omitempty"`         // Maximum number of concurrent waves
	WaveSpeedMs     *int     `json:"wave_speed_ms,omitempty"`     // Wave propagation speed in m/s
	NoiseLevel      *float64 `json:"noise_level,omitempty"`       // Noise level (0.0 - 1.0)
	RealTimeCapable *bool    `json:"real_time_capable,omitempty"` // Can operate in real-time mode
	AdvancedEcho    *bool    `json:"advanced_echo,omitempty"`     // Has advanced echo processing
	NoiseFilter     *bool    `json:"noise_filter,omitempty"`      // Has built-in noise filtering
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

// SecureZoneData represents encrypted zone data for client-side processing
type SecureZoneData struct {
	EncryptedArtifacts string    `json:"encrypted_artifacts"`
	ZoneHash           string    `json:"zone_hash"`
	SessionToken       string    `json:"session_token"`
	ExpiresAt          time.Time `json:"expires_at"`
	MaxScans           int       `json:"max_scans"`
	ScanCount          int       `json:"scan_count"`
}

// ScanSession represents a scanning session for a user in a zone
type ScanSession struct {
	UserID    string    `json:"user_id"`
	ZoneID    string    `json:"zone_id"`
	ScanCount int       `json:"scan_count"`
	MaxScans  int       `json:"max_scans"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ZoneArtifacts represents all items in a zone for encryption
type ZoneArtifacts struct {
	Artifacts []common.Artifact `json:"artifacts"`
	Gear      []common.Gear     `json:"gear"`
	ZoneID    string            `json:"zone_id"`
	Timestamp time.Time         `json:"timestamp"`
}

// ClaimRequest represents a claim request with position verification
type ClaimRequest struct {
	ItemID       string  `json:"item_id" binding:"required"`
	ItemType     string  `json:"item_type" binding:"required"` // "artifact" or "gear"
	Latitude     float64 `json:"latitude" binding:"required"`
	Longitude    float64 `json:"longitude" binding:"required"`
	ZoneID       string  `json:"zone_id" binding:"required"`
	SessionToken string  `json:"session_token" binding:"required"`
}
