package common

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONB type for PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

// BaseModel - spoločný základný model pre všetky moduly
type BaseModel struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// Location bez Accuracy (databáza ho nemá)
type Location struct {
	Latitude  float64   `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude float64   `json:"longitude" gorm:"type:decimal(11,8)"`
	Timestamp time.Time `json:"timestamp" gorm:"autoUpdateTime"`
}

// LocationWithAccuracy pre user tracking kde potrebujeme accuracy
type LocationWithAccuracy struct {
	Latitude  float64   `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude float64   `json:"longitude" gorm:"type:decimal(11,8)"`
	Accuracy  float64   `json:"accuracy,omitempty"`
	Timestamp time.Time `json:"timestamp" gorm:"autoUpdateTime"`
}

// ✅ PRESUNUTÉ DO internal/auth/models.go

// ✅ PRESUNUTÉ DO internal/gameplay/models.go

// ✅ PRESUNUTÉ DO internal/gameplay/models.go

// ✅ PRESUNUTÉ DO internal/auth/models.go

// ✅ PRESUNUTÉ DO internal/gameplay/models.go

// ========================================
// 📋 SÚHRN PRESUNU MODELOV
// ========================================
//
// ✅ internal/auth/models.go:
//    - User, PlayerSession, LocationWithAccuracy
//
// ✅ internal/gameplay/models.go:
//    - Zone, Artifact, Gear, InventoryItem, LoadoutSlot, LoadoutItem, GearCategory
//
// ✅ internal/market/models.go:
//    - MarketItem, Transaction, UserPurchase, UserTierPurchase, Currency, EssencePackage
//
// ✅ internal/analytics/models.go:
//    - ModuleCatalog, PowerCellsCatalog
//
// ✅ internal/common/models.go (zostáva):
//    - BaseModel, JSONB, Location, LocationWithAccuracy (spoločné typy)
//
// ========================================
