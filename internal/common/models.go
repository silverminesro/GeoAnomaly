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

// Base model
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

// User model - match exact database structure
type User struct {
	BaseModel
	Username        string     `json:"username" gorm:"uniqueIndex;not null;size:50"`
	Email           string     `json:"email" gorm:"uniqueIndex;not null;size:100"`
	PasswordHash    string     `json:"-" gorm:"not null;size:255"`
	Tier            int        `json:"tier" gorm:"default:0"`
	TierExpires     *time.Time `json:"tier_expires,omitempty"`
	TierAutoRenew   bool       `json:"tier_auto_renew" gorm:"default:false"`
	XP              int        `json:"xp" gorm:"default:0"`
	Level           int        `json:"level" gorm:"default:1"`
	TotalArtifacts  int        `json:"total_artifacts" gorm:"default:0"`
	TotalGear       int        `json:"total_gear" gorm:"default:0"`
	ZonesDiscovered int        `json:"zones_discovered" gorm:"default:0"`
	IsActive        bool       `json:"is_active" gorm:"default:true"`
	IsBanned        bool       `json:"is_banned" gorm:"default:false"`
	LastLogin       *time.Time `json:"last_login,omitempty"`
	ProfileData     JSONB      `json:"profile_data,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	Inventory []InventoryItem `json:"inventory,omitempty" gorm:"foreignKey:UserID"`
}

// ✅ UPDATED: Zone model with TTL system + biome support
type Zone struct {
	BaseModel
	Name         string   `json:"name" gorm:"not null;size:100"`
	Description  string   `json:"description,omitempty" gorm:"type:text"`
	TierRequired int      `json:"tier_required" gorm:"not null"`
	Location     Location `json:"location" gorm:"embedded;embeddedPrefix:location_"`
	RadiusMeters int      `json:"radius_meters" gorm:"not null"`
	IsActive     bool     `json:"is_active" gorm:"default:true"`
	ZoneType     string   `json:"zone_type" gorm:"not null;default:'static'"`

	// ✅ Biome system fields
	Biome       string `json:"biome" gorm:"size:50;default:'forest'"`
	DangerLevel string `json:"danger_level" gorm:"size:20;default:'low'"`

	// ✅ NEW: TTL & Cleanup fields
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	LastActivity time.Time  `json:"last_activity" gorm:"default:CURRENT_TIMESTAMP"`
	AutoCleanup  bool       `json:"auto_cleanup" gorm:"default:true"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	Artifacts []Artifact `json:"artifacts,omitempty" gorm:"foreignKey:ZoneID"`
	Gear      []Gear     `json:"gear,omitempty" gorm:"foreignKey:ZoneID"`
}

// ✅ NEW: Helper methods for Zone TTL
func (z *Zone) IsExpired() bool {
	if z.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*z.ExpiresAt)
}

func (z *Zone) TimeUntilExpiry() time.Duration {
	if z.ExpiresAt == nil {
		return 0
	}
	return time.Until(*z.ExpiresAt)
}

func (z *Zone) TTLStatus() string {
	if z.ExpiresAt == nil {
		return "permanent"
	}

	timeLeft := z.TimeUntilExpiry()
	if timeLeft <= 0 {
		return "expired"
	} else if timeLeft <= 1*time.Hour {
		return "expiring"
	} else if timeLeft <= 6*time.Hour {
		return "aging"
	} else {
		return "fresh"
	}
}

func (z *Zone) UpdateActivity() {
	z.LastActivity = time.Now()
}

func (z *Zone) SetRandomTTL() {
	// Random TTL between 6-24 hours
	minTTL := 6 * time.Hour
	maxTTL := 24 * time.Hour
	ttlRange := maxTTL - minTTL
	randomTTL := minTTL + time.Duration(float64(ttlRange)*(0.5+0.5)) // Simple randomization

	expiresAt := time.Now().Add(randomTTL)
	z.ExpiresAt = &expiresAt
}

// Inventory model
type InventoryItem struct {
	BaseModel
	UserID     uuid.UUID `json:"user_id" gorm:"not null;index"`
	ItemType   string    `json:"item_type" gorm:"not null;size:50"` // artifact, gear
	ItemID     uuid.UUID `json:"item_id" gorm:"not null"`
	Properties JSONB     `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	Quantity   int       `json:"quantity" gorm:"default:1"`
	AcquiredAt time.Time `json:"acquired_at" gorm:"autoCreateTime"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// ✅ UPDATED: Artifact model with biome support
type Artifact struct {
	BaseModel
	ZoneID   uuid.UUID `json:"zone_id" gorm:"not null;index"`
	Name     string    `json:"name" gorm:"not null;size:100"`
	Type     string    `json:"type" gorm:"not null;size:50"`
	Rarity   string    `json:"rarity" gorm:"not null;size:20"` // common, rare, epic, legendary
	Location Location  `json:"location" gorm:"embedded;embeddedPrefix:location_"`

	// ✅ Biome system fields
	Biome            string `json:"biome" gorm:"size:50;default:'forest'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`

	// Relationships
	Zone *Zone `json:"zone,omitempty" gorm:"foreignKey:ZoneID"`
}

// ✅ UPDATED: Gear model with biome support
type Gear struct {
	BaseModel
	ZoneID   uuid.UUID `json:"zone_id" gorm:"not null;index"`
	Name     string    `json:"name" gorm:"not null;size:100"`
	Type     string    `json:"type" gorm:"not null;size:50"` // weapon, armor, tool
	Level    int       `json:"level" gorm:"default:1"`
	Location Location  `json:"location" gorm:"embedded;embeddedPrefix:location_"`

	// ✅ Biome system fields
	Biome            string `json:"biome" gorm:"size:50;default:'forest'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`

	// Relationships
	Zone *Zone `json:"zone,omitempty" gorm:"foreignKey:ZoneID"`
}

// Table name methods
func (Artifact) TableName() string {
	return "artifacts"
}

func (Gear) TableName() string {
	return "gear"
}

// PlayerSession - NO BaseModel embedding to avoid conflicts
type PlayerSession struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	UserID      uuid.UUID  `json:"user_id" gorm:"not null;index"`
	LastSeen    time.Time  `json:"last_seen" gorm:"autoUpdateTime"`
	IsOnline    bool       `json:"is_online" gorm:"default:true"`
	CurrentZone *uuid.UUID `json:"current_zone,omitempty" gorm:"index"`

	// Individual location fields instead of embedded struct
	LastLocationLatitude  float64   `json:"last_location_latitude" gorm:"type:decimal(10,8)"`
	LastLocationLongitude float64   `json:"last_location_longitude" gorm:"type:decimal(11,8)"`
	LastLocationAccuracy  float64   `json:"last_location_accuracy"`
	LastLocationTimestamp time.Time `json:"last_location_timestamp"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Zone *Zone `json:"zone,omitempty" gorm:"foreignKey:CurrentZone"`
}

// Helper methods for PlayerSession
func (ps *PlayerSession) GetLastLocation() LocationWithAccuracy {
	return LocationWithAccuracy{
		Latitude:  ps.LastLocationLatitude,
		Longitude: ps.LastLocationLongitude,
		Accuracy:  ps.LastLocationAccuracy,
		Timestamp: ps.LastLocationTimestamp,
	}
}

func (ps *PlayerSession) SetLastLocation(loc LocationWithAccuracy) {
	ps.LastLocationLatitude = loc.Latitude
	ps.LastLocationLongitude = loc.Longitude
	ps.LastLocationAccuracy = loc.Accuracy
	ps.LastLocationTimestamp = loc.Timestamp
}

// LoadoutSlot definuje dostupné sloty pre gear
type LoadoutSlot struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	MaxItems    int    `json:"max_items" gorm:"default:1"`
	IsRequired  bool   `json:"is_required" gorm:"default:false"`
	Order       int    `json:"order" gorm:"default:0"`
}

// LoadoutItem reprezentuje vybavený gear v loadoute
type LoadoutItem struct {
	BaseModel
	UserID     uuid.UUID `json:"user_id" gorm:"not null;index"`
	SlotID     string    `json:"slot_id" gorm:"not null;size:50"`
	ItemID     uuid.UUID `json:"item_id" gorm:"not null"`
	ItemType   string    `json:"item_type" gorm:"not null;size:50"`
	EquippedAt time.Time `json:"equipped_at" gorm:"not null"`
	
	// Durability systém
	Durability     int       `json:"durability" gorm:"default:100"` // 0-100
	MaxDurability  int       `json:"max_durability" gorm:"default:100"`
	LastRepaired   time.Time `json:"last_repaired"`
	
	// Odolnosť proti nepriateľom
	ZombieResistance     int `json:"zombie_resistance" gorm:"default:0"`
	BanditResistance     int `json:"bandit_resistance" gorm:"default:0"`
	SoldierResistance    int `json:"soldier_resistance" gorm:"default:0"`
	MonsterResistance    int `json:"monster_resistance" gorm:"default:0"`
	
	// Properties pre flexibilitu
	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	
	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// GearCategory definuje kategórie gearu
type GearCategory struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	SlotID      string `json:"slot_id" gorm:"not null;size:50"`
	Rarity      string `json:"rarity" gorm:"default:'common'"`
	Level       int    `json:"level" gorm:"default:1"`
	
	// Base stats
	BaseDurability      int `json:"base_durability" gorm:"default:100"`
	BaseZombieResistance int `json:"base_zombie_resistance" gorm:"default:0"`
	BaseBanditResistance int `json:"base_bandit_resistance" gorm:"default:0"`
	BaseSoldierResistance int `json:"base_soldier_resistance" gorm:"default:0"`
	BaseMonsterResistance int `json:"base_monster_resistance" gorm:"default:0"`
	
	// Biome specific
	Biome            string `json:"biome" gorm:"size:50;default:'all'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`
	
	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`
}
