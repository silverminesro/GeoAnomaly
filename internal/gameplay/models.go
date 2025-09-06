package gameplay

import (
	"time"

	"github.com/google/uuid"
)

// BaseModel - spoločný základný model (import z common)
type BaseModel struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// JSONB type for PostgreSQL
type JSONB map[string]interface{}

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

// Zone model - migrovaný do gameplay.zones
type Zone struct {
	BaseModel
	Name         string   `json:"name" gorm:"not null;size:100"`
	Description  string   `json:"description,omitempty" gorm:"type:text"`
	TierRequired int      `json:"tier_required" gorm:"not null"`
	Location     Location `json:"location" gorm:"embedded;embeddedPrefix:location_"`
	RadiusMeters int      `json:"radius_meters" gorm:"not null"`
	IsActive     bool     `json:"is_active" gorm:"default:true"`
	ZoneType     string   `json:"zone_type" gorm:"not null;default:'static'"`

	// Biome system fields
	Biome       string `json:"biome" gorm:"size:50;default:'forest'"`
	DangerLevel string `json:"danger_level" gorm:"size:20;default:'low'"`

	// TTL & Cleanup fields
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	LastActivity time.Time  `json:"last_activity" gorm:"default:CURRENT_TIMESTAMP"`
	AutoCleanup  bool       `json:"auto_cleanup" gorm:"default:true"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	Artifacts []Artifact `json:"artifacts,omitempty" gorm:"foreignKey:ZoneID"`
	Gear      []Gear     `json:"gear,omitempty" gorm:"foreignKey:ZoneID"`
}

// Helper methods for Zone TTL
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

// Artifact model - migrovaný do gameplay.artifacts
type Artifact struct {
	BaseModel
	ZoneID   uuid.UUID `json:"zone_id" gorm:"not null;index"`
	Name     string    `json:"name" gorm:"not null;size:100"`
	Type     string    `json:"type" gorm:"not null;size:50"`
	Rarity   string    `json:"rarity" gorm:"not null;size:20"` // common, rare, epic, legendary
	Location Location  `json:"location" gorm:"embedded;embeddedPrefix:location_"`

	// Biome system fields
	Biome            string `json:"biome" gorm:"size:50;default:'forest'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`
	IsClaimed  bool  `json:"is_claimed" gorm:"default:false"`

	// Relationships
	Zone *Zone `json:"zone,omitempty" gorm:"foreignKey:ZoneID"`
}

// Gear model - migrovaný do gameplay.gear
type Gear struct {
	BaseModel
	ZoneID   uuid.UUID `json:"zone_id" gorm:"not null;index"`
	Name     string    `json:"name" gorm:"not null;size:100"`
	Type     string    `json:"type" gorm:"not null;size:50"` // weapon, armor, tool
	Level    int       `json:"level" gorm:"default:1"`
	Location Location  `json:"location" gorm:"embedded;embeddedPrefix:location_"`

	// Biome system fields
	Biome            string `json:"biome" gorm:"size:50;default:'forest'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`
	IsClaimed  bool  `json:"is_claimed" gorm:"default:false"`

	// Relationships
	Zone *Zone `json:"zone,omitempty" gorm:"foreignKey:ZoneID"`
}

// InventoryItem model - migrovaný do gameplay.inventory_items
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

// LoadoutSlot model - migrovaný do gameplay.loadout_slots
type LoadoutSlot struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	MaxItems    int    `json:"max_items" gorm:"default:1"`
	IsRequired  bool   `json:"is_required" gorm:"default:false"`
	Order       int    `json:"order" gorm:"default:0"`
}

// LoadoutItem model - migrovaný do gameplay.loadout_items
type LoadoutItem struct {
	BaseModel
	UserID     uuid.UUID `json:"user_id" gorm:"not null;index"`
	SlotID     string    `json:"slot_id" gorm:"not null;size:50"`
	ItemID     uuid.UUID `json:"item_id" gorm:"not null"`
	ItemType   string    `json:"item_type" gorm:"not null;size:50"`
	EquippedAt time.Time `json:"equipped_at" gorm:"not null"`

	// Durability systém
	Durability    int        `json:"durability" gorm:"default:100"` // 0-100
	MaxDurability int        `json:"max_durability" gorm:"default:100"`
	LastRepaired  *time.Time `json:"last_repaired"`

	// Odolnosť proti nepriateľom
	ZombieResistance  int `json:"zombie_resistance" gorm:"default:0"`
	BanditResistance  int `json:"bandit_resistance" gorm:"default:0"`
	SoldierResistance int `json:"soldier_resistance" gorm:"default:0"`
	MonsterResistance int `json:"monster_resistance" gorm:"default:0"`

	// Properties pre flexibilitu
	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// GearCategory model - migrovaný do gameplay.gear_categories
type GearCategory struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	SlotID      string `json:"slot_id" gorm:"not null;size:50"`
	Rarity      string `json:"rarity" gorm:"default:'common'"`
	Level       int    `json:"level" gorm:"default:1"`

	// Base stats
	BaseDurability        int `json:"base_durability" gorm:"default:100"`
	BaseZombieResistance  int `json:"base_zombie_resistance" gorm:"default:0"`
	BaseBanditResistance  int `json:"base_bandit_resistance" gorm:"default:0"`
	BaseSoldierResistance int `json:"base_soldier_resistance" gorm:"default:0"`
	BaseMonsterResistance int `json:"base_monster_resistance" gorm:"default:0"`

	// Biome specific
	Biome            string `json:"biome" gorm:"size:50;default:'all'"`
	ExclusiveToBiome bool   `json:"exclusive_to_biome" gorm:"default:false"`

	Properties JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	IsActive   bool  `json:"is_active" gorm:"default:true"`
}

// TableName methods for GORM schema qualification
func (Zone) TableName() string {
	return "gameplay.zones"
}

func (Artifact) TableName() string {
	return "gameplay.artifacts"
}

func (Gear) TableName() string {
	return "gameplay.gear"
}

func (InventoryItem) TableName() string {
	return "gameplay.inventory_items"
}

func (LoadoutSlot) TableName() string {
	return "gameplay.loadout_slots"
}

func (LoadoutItem) TableName() string {
	return "gameplay.loadout_items"
}

func (GearCategory) TableName() string {
	return "gameplay.gear_categories"
}

// User model reference (from auth module)
type User struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Tier      int       `json:"tier"`
	XP        int       `json:"xp"`
	Level     int       `json:"level"`
	IsActive  bool      `json:"is_active"`
	IsBanned  bool      `json:"is_banned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "auth.users"
}
