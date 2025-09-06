package analytics

import (
	"time"

	"github.com/google/uuid"
)

// BaseModel - spoločný základný model
type BaseModel struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// JSONB type for PostgreSQL
type JSONB map[string]interface{}

// ModuleCatalog model - migrovaný do analytics.module_catalog
type ModuleCatalog struct {
	BaseModel
	Name        string `json:"name" gorm:"not null;size:100"`
	Type        string `json:"type" gorm:"not null;size:50"` // core, power, sensor, utility
	Description string `json:"description,omitempty" gorm:"type:text"`
	Rarity      string `json:"rarity" gorm:"not null;size:20"` // common, rare, epic, legendary
	Level       int    `json:"level" gorm:"default:1"`

	// Module effects
	RangeBonus      float64 `json:"range_bonus" gorm:"default:0"`       // +20% max range
	FlightTimeBonus float64 `json:"flight_time_bonus" gorm:"default:0"` // +15% flight time
	SpeedBonus      float64 `json:"speed_bonus" gorm:"default:0"`       // +10% speed
	AccuracyBonus   float64 `json:"accuracy_bonus" gorm:"default:0"`    // +10% accuracy
	StealthBonus    float64 `json:"stealth_bonus" gorm:"default:0"`     // +20% stealth
	DurabilityBonus float64 `json:"durability_bonus" gorm:"default:0"`  // +15% durability

	// Trade-offs
	BatteryDrainPenalty float64 `json:"battery_drain_penalty" gorm:"default:0"` // +8% battery drain
	SpeedPenalty        float64 `json:"speed_penalty" gorm:"default:0"`         // -5% speed
	WeightPenalty       float64 `json:"weight_penalty" gorm:"default:0"`        // +10% weight

	// Compatibility
	CompatibleDroneTypes JSONB `json:"compatible_drone_types" gorm:"type:jsonb;default:'[]'::jsonb"`
	MinBatteryCapacity   int   `json:"min_battery_capacity" gorm:"default:0"`
	MaxBatteryCapacity   int   `json:"max_battery_capacity" gorm:"default:999999"`

	// Market info
	BasePrice   int    `json:"base_price" gorm:"default:0"` // v centoch
	Currency    string `json:"currency" gorm:"not null;size:10;default:'USD'"`
	IsAvailable bool   `json:"is_available" gorm:"default:true"`
	IsExclusive bool   `json:"is_exclusive" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
}

// PowerCellsCatalog model - migrovaný do analytics.power_cells_catalog
type PowerCellsCatalog struct {
	BaseModel
	Name        string `json:"name" gorm:"not null;size:100"`
	Type        string `json:"type" gorm:"not null;size:50"` // standard, high_capacity, fast_charge
	Description string `json:"description,omitempty" gorm:"type:text"`
	Rarity      string `json:"rarity" gorm:"not null;size:20"` // common, rare, epic, legendary
	Level       int    `json:"level" gorm:"default:1"`

	// Battery specs
	Capacity      int     `json:"capacity" gorm:"not null"`                         // mAh
	Voltage       float64 `json:"voltage" gorm:"not null"`                          // V
	DischargeRate int     `json:"discharge_rate" gorm:"not null"`                   // C rating
	ChargeRate    int     `json:"charge_rate" gorm:"not null"`                      // C rating
	Weight        float64 `json:"weight" gorm:"not null"`                           // grams
	Dimensions    JSONB   `json:"dimensions" gorm:"type:jsonb;default:'{}'::jsonb"` // length, width, height

	// Performance
	Efficiency     float64 `json:"efficiency" gorm:"default:0.95"`     // 95% efficiency
	CycleLife      int     `json:"cycle_life" gorm:"default:500"`      // charge cycles
	TemperatureMin int     `json:"temperature_min" gorm:"default:-20"` // °C
	TemperatureMax int     `json:"temperature_max" gorm:"default:60"`  // °C

	// Compatibility
	CompatibleDroneTypes JSONB `json:"compatible_drone_types" gorm:"type:jsonb;default:'[]'::jsonb"`
	CompatibleModules    JSONB `json:"compatible_modules" gorm:"type:jsonb;default:'[]'::jsonb"`

	// Market info
	BasePrice   int    `json:"base_price" gorm:"default:0"` // v centoch
	Currency    string `json:"currency" gorm:"not null;size:10;default:'USD'"`
	IsAvailable bool   `json:"is_available" gorm:"default:true"`
	IsExclusive bool   `json:"is_exclusive" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
}

// TableName methods for GORM schema qualification
func (ModuleCatalog) TableName() string {
	return "analytics.module_catalog"
}

func (PowerCellsCatalog) TableName() string {
	return "analytics.power_cells_catalog"
}
