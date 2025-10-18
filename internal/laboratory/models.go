package laboratory

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// =============================================
// 1. LABORATORY CORE MODELS
// =============================================

// Laboratory represents a player's laboratory
type Laboratory struct {
	ID                 uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID             uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex"`
	Level              int       `json:"level" gorm:"not null;default:1;check:level >= 1 AND level <= 3"`
	BaseChargingSlots  int       `json:"base_charging_slots" gorm:"not null;default:1"`
	ExtraChargingSlots int       `json:"extra_charging_slots" gorm:"not null;default:0"`
	ResearchUnlocked   bool      `json:"research_unlocked" gorm:"not null;default:false"`
	CraftingUnlocked   bool      `json:"crafting_unlocked" gorm:"not null;default:false"`
	// Location fields for map placement
	// Generated column in DB – only read it, never write
	Location          *string    `json:"location,omitempty" gorm:"->;type:geography(POINT,4326)"` // PostGIS geography point
	LocationLatitude  *float64   `json:"location_latitude,omitempty" gorm:"type:decimal(10,8)"`
	LocationLongitude *float64   `json:"location_longitude,omitempty" gorm:"type:decimal(11,8)"`
	IsPlaced          bool       `json:"is_placed" gorm:"not null;default:false"`
	PlacedAt          *time.Time `json:"placed_at,omitempty"`
	// Relocation fields
	RelocationCount int        `json:"relocation_count" gorm:"not null;default:0"`
	LastRelocatedAt *time.Time `json:"last_relocated_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	User             *User                    `json:"user,omitempty" gorm:"foreignKey:UserID"`
	ChargingSessions []BatteryChargingSession `json:"charging_sessions,omitempty" gorm:"foreignKey:LaboratoryID"`
	ResearchProjects []ResearchProject        `json:"research_projects,omitempty" gorm:"foreignKey:LaboratoryID"`
	CraftingSessions []CraftingSession        `json:"crafting_sessions,omitempty" gorm:"foreignKey:LaboratoryID"`
	SlotPurchases    []ChargingSlotPurchase   `json:"slot_purchases,omitempty" gorm:"foreignKey:LaboratoryID"`
}

// LaboratoryUpgradeRequirement defines requirements for upgrading laboratory
type LaboratoryUpgradeRequirement struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Level               int       `json:"level" gorm:"not null;check:level >= 2 AND level <= 3"`
	CreditsRequired     int       `json:"credits_required" gorm:"not null"`
	ArtifactRequired    *string   `json:"artifact_required,omitempty" gorm:"type:varchar(100)"`
	ArtifactRarity      *string   `json:"artifact_rarity,omitempty" gorm:"type:varchar(20);check:artifact_rarity IN ('common', 'rare', 'epic', 'legendary')"`
	ArtifactQuantity    int       `json:"artifact_quantity" gorm:"default:1"`
	PlayerLevelRequired int       `json:"player_level_required" gorm:"default:1"`
	TierRequired        int       `json:"tier_required" gorm:"default:1"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// LaboratoryUpgradeHistory tracks laboratory upgrades for audit trail
type LaboratoryUpgradeHistory struct {
	ID            uuid.UUID     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	LaboratoryID  uuid.UUID     `json:"laboratory_id" gorm:"not null;index"`
	UserID        uuid.UUID     `json:"user_id" gorm:"not null;index"`
	FromLevel     int           `json:"from_level" gorm:"not null"`
	ToLevel       int           `json:"to_level" gorm:"not null"`
	CreditsSpent  int           `json:"credits_spent" gorm:"default:0"`
	ArtifactsUsed ArtifactsUsed `json:"artifacts_used" gorm:"type:jsonb;default:'[]'::jsonb"`
	UpgradedAt    time.Time     `json:"upgraded_at" gorm:"autoCreateTime"`

	// Relationships
	Laboratory *Laboratory `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
	User       *User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for LaboratoryUpgradeHistory
func (LaboratoryUpgradeHistory) TableName() string {
	return "laboratory.laboratory_upgrade_history"
}

// ArtifactUsed represents an artifact consumed during upgrade
type ArtifactUsed struct {
	ArtifactID   uuid.UUID `json:"artifact_id"`
	ArtifactName string    `json:"artifact_name"`
	Rarity       string    `json:"rarity"`
	Quantity     int       `json:"quantity"`
}

// ArtifactsUsed is a custom type for JSONB handling
type ArtifactsUsed []ArtifactUsed

// Scan implements the sql.Scanner interface for ArtifactsUsed
func (a *ArtifactsUsed) Scan(value interface{}) error {
	if value == nil {
		*a = ArtifactsUsed{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ArtifactsUsed", value)
	}

	return json.Unmarshal(bytes, a)
}

// Value implements the driver.Valuer interface for ArtifactsUsed
func (a ArtifactsUsed) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "[]", nil
	}

	bytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}

	return string(bytes), nil
}

// ChargingSlotPurchase represents purchase of extra charging slot
type ChargingSlotPurchase struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	LaboratoryID uuid.UUID `json:"laboratory_id" gorm:"type:uuid;not null"`
	EssenceCost  int       `json:"essence_cost" gorm:"not null"`
	PurchasedAt  time.Time `json:"purchased_at" gorm:"autoCreateTime"`

	// Relations
	User       *User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Laboratory *Laboratory `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
}

// =============================================
// 2. RESEARCH SYSTEM MODELS (Level 2+)
// =============================================

// ResearchProject represents an artifact research project
type ResearchProject struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	LaboratoryID uuid.UUID `json:"laboratory_id" gorm:"type:uuid;not null"`
	ArtifactID   uuid.UUID `json:"artifact_id" gorm:"type:uuid;not null"`
	ResearchType string    `json:"research_type" gorm:"type:varchar(50);not null;check:research_type IN ('basic', 'advanced', 'expert')"`

	// Active mode fields (minigame)
	Mode            string  `json:"mode" gorm:"type:varchar(20);not null;default:'active'"`
	SetupAccuracy   int     `json:"setup_accuracy" gorm:"type:integer;not null;default:0"`
	BonusMultiplier float64 `json:"bonus_multiplier" gorm:"type:numeric(4,2);not null;default:1.00"`

	StartTime     time.Time `json:"start_time" gorm:"not null"`
	EndTime       time.Time `json:"end_time" gorm:"not null"`
	Status        string    `json:"status" gorm:"type:varchar(20);not null;default:'active';check:status IN ('active', 'completed', 'failed', 'cancelled')"`
	ResearchLevel int       `json:"research_level" gorm:"not null;default:1"`
	Results       *JSONB    `json:"results,omitempty" gorm:"type:jsonb"`
	Cost          int       `json:"cost" gorm:"not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	User       *User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Laboratory *Laboratory `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
	Artifact   *Artifact   `json:"artifact,omitempty" gorm:"foreignKey:ArtifactID"`
}

// ResearchResult represents the result of artifact research
type ResearchResult struct {
	TrueRarity       string   `json:"true_rarity"`
	HiddenProperties []string `json:"hidden_properties"`
	CraftingValue    int      `json:"crafting_value"`
	MarketValue      int      `json:"market_value"`
	SpecialEffects   []string `json:"special_effects"`
	ResearchXP       int      `json:"research_xp"`
}

// =============================================
// 3. CRAFTING SYSTEM MODELS (Level 3+)
// =============================================

// CraftingRecipe represents a crafting recipe
type CraftingRecipe struct {
	ID                      uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                    string    `json:"name" gorm:"type:varchar(200);not null"`
	Description             *string   `json:"description,omitempty" gorm:"type:text"`
	Category                string    `json:"category" gorm:"type:varchar(100);not null"`
	Level                   int       `json:"level" gorm:"not null;default:1"`
	LaboratoryLevelRequired int       `json:"laboratory_level_required" gorm:"not null;default:3;check:laboratory_level_required >= 3"`
	Materials               JSONB     `json:"materials" gorm:"type:jsonb;not null"`
	Result                  JSONB     `json:"result" gorm:"type:jsonb;not null"`
	CraftTimeSeconds        int       `json:"craft_time_seconds" gorm:"not null"`
	XPReward                int       `json:"xp_reward" gorm:"not null;default:0"`
	Unlocked                bool      `json:"unlocked" gorm:"not null;default:false"`
	CreatedAt               time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// CraftingSession represents an active crafting session
type CraftingSession struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID        uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	LaboratoryID  uuid.UUID `json:"laboratory_id" gorm:"type:uuid;not null"`
	RecipeID      uuid.UUID `json:"recipe_id" gorm:"type:uuid;not null"`
	StartTime     time.Time `json:"start_time" gorm:"not null"`
	EndTime       time.Time `json:"end_time" gorm:"not null"`
	Status        string    `json:"status" gorm:"type:varchar(20);not null;default:'active';check:status IN ('active', 'completed', 'failed', 'cancelled')"`
	Progress      float64   `json:"progress" gorm:"not null;default:0.0;check:progress >= 0.0 AND progress <= 1.0"`
	MaterialsUsed *JSONB    `json:"materials_used,omitempty" gorm:"type:jsonb"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	User       *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Laboratory *Laboratory     `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
	Recipe     *CraftingRecipe `json:"recipe,omitempty" gorm:"foreignKey:RecipeID"`
}

// =============================================
// 4. BATTERY CHARGING SYSTEM MODELS (Level 1+)
// =============================================

// BatteryChargingSession represents a battery charging session
type BatteryChargingSession struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID            uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	LaboratoryID      uuid.UUID  `json:"laboratory_id" gorm:"type:uuid;not null"`
	SlotNumber        int        `json:"slot_number" gorm:"not null"`
	BatteryInstanceID *uuid.UUID `json:"battery_instance_id,omitempty" gorm:"type:uuid;index"`
	BatteryType       string     `json:"battery_type" gorm:"type:varchar(50);not null"`
	DeviceType        string     `json:"device_type" gorm:"type:varchar(50);not null;check:device_type IN ('scanner', 'drone')"`
	DeviceID          *uuid.UUID `json:"device_id,omitempty" gorm:"type:uuid"`
	StartTime         time.Time  `json:"start_time" gorm:"not null"`
	EndTime           time.Time  `json:"end_time" gorm:"not null"`
	Status            string     `json:"status" gorm:"type:varchar(20);not null;default:'active';check:status IN ('active', 'completed', 'failed', 'cancelled')"`
	ChargingSpeed     float64    `json:"charging_speed" gorm:"not null;default:1.0"`
	CostCredits       int        `json:"cost_credits" gorm:"not null;default:0"`
	Progress          float64    `json:"progress" gorm:"not null;default:0.0;check:progress >= 0.0 AND progress <= 100.0"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	User       *User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Laboratory *Laboratory `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
}

// =============================================
// 5. TASK SYSTEM MODELS (Level 1+)
// =============================================

// LaboratoryTask represents a laboratory task (daily/weekly/monthly)
type LaboratoryTask struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID          uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	TaskType        string     `json:"task_type" gorm:"type:varchar(20);not null;check:task_type IN ('daily', 'weekly', 'monthly')"`
	TaskCategory    string     `json:"task_category" gorm:"type:varchar(50);not null"`
	TaskName        string     `json:"task_name" gorm:"type:varchar(200);not null"`
	Description     *string    `json:"description,omitempty" gorm:"type:text"`
	TargetValue     int        `json:"target_value" gorm:"not null"`
	CurrentProgress int        `json:"current_progress" gorm:"not null;default:0"`
	RewardCredits   int        `json:"reward_credits" gorm:"not null;default:0"`
	RewardXP        int        `json:"reward_xp" gorm:"not null;default:0"`
	RewardMaterials *JSONB     `json:"reward_materials,omitempty" gorm:"type:jsonb"`
	Status          string     `json:"status" gorm:"type:varchar(20);not null;default:'active';check:status IN ('active', 'completed', 'claimed', 'expired')"`
	AssignedAt      time.Time  `json:"assigned_at" gorm:"not null"`
	ExpiresAt       time.Time  `json:"expires_at" gorm:"not null"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ClaimedAt       *time.Time `json:"claimed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// =============================================
// 6. LABORATORY XP SYSTEM MODELS
// =============================================

// LaboratoryXP represents laboratory XP tracking
type LaboratoryXP struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID           uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex"`
	TotalXP          int       `json:"total_xp" gorm:"not null;default:0"`
	ResearchXP       int       `json:"research_xp" gorm:"not null;default:0"`
	CraftingXP       int       `json:"crafting_xp" gorm:"not null;default:0"`
	TaskXP           int       `json:"task_xp" gorm:"not null;default:0"`
	Level            int       `json:"level" gorm:"not null;default:1"`
	UnlockedFeatures JSONB     `json:"unlocked_features" gorm:"type:jsonb;default:'[]'"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// LaboratoryRelocation represents laboratory relocation history
type LaboratoryRelocation struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	LaboratoryID     uuid.UUID `json:"laboratory_id" gorm:"type:uuid;not null;index"`
	UserID           uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	OldLatitude      *float64  `json:"old_latitude,omitempty" gorm:"type:decimal(10,8)"`
	OldLongitude     *float64  `json:"old_longitude,omitempty" gorm:"type:decimal(11,8)"`
	NewLatitude      float64   `json:"new_latitude" gorm:"type:decimal(10,8);not null"`
	NewLongitude     float64   `json:"new_longitude" gorm:"type:decimal(11,8);not null"`
	EssenceCost      int       `json:"essence_cost" gorm:"not null"`
	RelocationReason string    `json:"relocation_reason" gorm:"size:100;default:'manual'"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	Laboratory *Laboratory `json:"laboratory,omitempty" gorm:"foreignKey:LaboratoryID"`
	User       *User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// =============================================
// 7. REQUEST/RESPONSE MODELS
// =============================================

// PlaceLaboratoryRequest represents request to place laboratory on map
type PlaceLaboratoryRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
}

// PlaceLaboratoryResponse represents response after placing laboratory
type PlaceLaboratoryResponse struct {
	Success    bool        `json:"success"`
	Laboratory *Laboratory `json:"laboratory"`
	Message    string      `json:"message"`
}

// GetNearbyLaboratoriesRequest represents request to get nearby laboratories
type GetNearbyLaboratoriesRequest struct {
	Latitude  float64 `form:"lat" binding:"required,min=-90,max=90"`
	Longitude float64 `form:"lng" binding:"required,min=-180,max=180"`
	RadiusM   int     `form:"radius_m" binding:"min=100,max=10000"` // 100m to 10km
}

// LaboratoryMarker represents laboratory marker for map
type LaboratoryMarker struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	Level       int       `json:"level"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	DistanceKm  float64   `json:"distance_km"`
	IsOwn       bool      `json:"is_own"`
	CanInteract bool      `json:"can_interact"`
	Icon        string    `json:"icon"`
	PlacedAt    time.Time `json:"placed_at"`
}

// GetNearbyLaboratoriesResponse represents response with nearby laboratories
type GetNearbyLaboratoriesResponse struct {
	Success      bool               `json:"success"`
	Laboratories []LaboratoryMarker `json:"laboratories"`
	Total        int                `json:"total"`
}

// RelocateLaboratoryRequest represents request to relocate laboratory
type RelocateLaboratoryRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
}

// RelocateLaboratoryResponse represents response after relocating laboratory
type RelocateLaboratoryResponse struct {
	Success         bool        `json:"success"`
	Laboratory      *Laboratory `json:"laboratory"`
	EssenceCost     int         `json:"essence_cost"`
	RelocationCount int         `json:"relocation_count"`
	Message         string      `json:"message"`
}

// LaboratoryStatusResponse represents laboratory status
type LaboratoryStatusResponse struct {
	Laboratory          *Laboratory                    `json:"laboratory"`
	ActiveCharging      []BatteryChargingSession       `json:"active_charging"`
	ActiveResearch      []ResearchProject              `json:"active_research"`
	ActiveCrafting      []CraftingSession              `json:"active_crafting"`
	AvailableTasks      []LaboratoryTask               `json:"available_tasks"`
	XP                  *LaboratoryXP                  `json:"xp"`
	UpgradeRequirements []LaboratoryUpgradeRequirement `json:"upgrade_requirements"`
	NeedsPlacement      bool                           `json:"needs_placement"`
}

// UpgradeLaboratoryRequest represents laboratory upgrade request
type UpgradeLaboratoryRequest struct {
	TargetLevel int `json:"target_level" binding:"required,min=2,max=3"`
}

// StartResearchRequest represents research start request
type StartResearchRequest struct {
	ArtifactID   uuid.UUID `json:"artifact_id" binding:"required"`
	ResearchType string    `json:"research_type" binding:"required,oneof=basic advanced expert"`
	Mode         string    `json:"mode" binding:"omitempty,oneof=active"`
	Accuracy     int       `json:"accuracy" binding:"required,min=0,max=100"`
}

// StartCraftingRequest represents crafting start request
type StartCraftingRequest struct {
	RecipeID uuid.UUID `json:"recipe_id" binding:"required"`
}

// StartChargingRequest represents battery charging start request
type StartChargingRequest struct {
	BatteryType       string     `json:"battery_type" binding:"required"`
	DeviceType        string     `json:"device_type" binding:"required,oneof=scanner drone"`
	DeviceID          *uuid.UUID `json:"device_id,omitempty"`
	BatteryInstanceID *uuid.UUID `json:"battery_instance_id,omitempty"` // Flutter posiela batteryInstanceId
}

// AvailableBattery represents a battery available for charging from user inventory
type AvailableBattery struct {
	InventoryID   uuid.UUID `json:"inventory_id"`
	BatteryType   string    `json:"battery_type"`
	BatteryName   string    `json:"battery_name"`
	CurrentCharge int       `json:"current_charge"`        // 0-100%
	IsInUse       bool      `json:"is_in_use"`             // true if currently used in deployed device
	DeviceName    *string   `json:"device_name,omitempty"` // name of device if in use
	AcquiredAt    time.Time `json:"acquired_at"`
	Properties    JSONB     `json:"properties"`
}

// ChargingSlot represents a charging slot with its current status
type ChargingSlot struct {
	SlotNumber        int                     `json:"slot_number"`
	IsAvailable       bool                    `json:"is_available"`
	ActiveSession     *BatteryChargingSession `json:"active_session,omitempty"`
	BatteryInstanceID *uuid.UUID              `json:"battery_instance_id,omitempty"`
}

// PurchaseSlotRequest represents extra slot purchase request
type PurchaseSlotRequest struct {
	EssenceCost int `json:"essence_cost" binding:"required,min=1"`
}

// =============================================
// 8. HELPER TYPES
// =============================================

// JSONB represents a JSONB field
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}

	bytes, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}

	return string(bytes), nil
}

// User represents a user (placeholder - should be imported from user package)
type User struct {
	ID    uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	XP    int       `json:"xp" gorm:"default:0"`
	Level int       `json:"level" gorm:"default:1"`
}

// Artifact represents an artifact (placeholder - should be imported from game package)
type Artifact struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
}

// InventoryItem represents an inventory item (placeholder - should be imported from gameplay package)
type InventoryItem struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	ItemType   string     `json:"item_type" gorm:"type:varchar(50);not null"`
	ItemID     uuid.UUID  `json:"item_id" gorm:"type:uuid;not null"`
	Properties string     `json:"properties" gorm:"type:jsonb;default:'{}'::jsonb"`
	Quantity   int        `json:"quantity" gorm:"default:1"`
	AcquiredAt *time.Time `json:"acquired_at,omitempty"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// DeployedDevice represents a deployed device (placeholder - should be imported from deployable package)
type DeployedDevice struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	OwnerID            uuid.UUID  `json:"owner_id" gorm:"type:uuid;not null"`
	DeviceInventoryID  uuid.UUID  `json:"device_inventory_id" gorm:"type:uuid;not null"`
	BatteryInventoryID *uuid.UUID `json:"battery_inventory_id,omitempty" gorm:"type:uuid"`
	IsActive           bool       `json:"is_active" gorm:"not null;default:true"`
}

// =============================================
// 9. TABLE NAMES
// =============================================

func (Laboratory) TableName() string { return "laboratory.laboratories" }
func (LaboratoryUpgradeRequirement) TableName() string {
	return "laboratory.laboratory_upgrade_requirements"
}
func (ChargingSlotPurchase) TableName() string {
	return "laboratory.laboratory_charging_slot_purchases"
}
func (ResearchProject) TableName() string        { return "laboratory.research_projects" }
func (CraftingRecipe) TableName() string         { return "laboratory.crafting_recipes" }
func (CraftingSession) TableName() string        { return "laboratory.crafting_sessions" }
func (BatteryChargingSession) TableName() string { return "laboratory.battery_charging_sessions" }
func (LaboratoryTask) TableName() string         { return "laboratory.laboratory_tasks" }
func (LaboratoryXP) TableName() string           { return "laboratory.laboratory_xp" }
func (LaboratoryRelocation) TableName() string   { return "laboratory.laboratory_relocations" }
func (User) TableName() string                   { return "auth.users" }
func (Artifact) TableName() string               { return "gameplay.artifacts" }
func (InventoryItem) TableName() string          { return "gameplay.inventory_items" }
func (DeployedDevice) TableName() string         { return "gameplay.deployed_devices" }
