package game

import (
	"geoanomaly/internal/gameplay"
	"geoanomaly/internal/loadout"
	"time"

	redis_client "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Handler pre game package
type Handler struct {
	db             *gorm.DB
	redis          *redis_client.Client
	loadoutService *loadout.Service
	gearService    *GearService
}

// Request/Response struktury
type ScanAreaRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

type ScanAreaResponse struct {
	ZonesCreated      int               `json:"zones_created"`
	Zones             []ZoneWithDetails `json:"zones"`
	ScanAreaCenter    LocationPoint     `json:"scan_area_center"`
	NextScanAvailable int64             `json:"next_scan_available"`
	MaxZones          int               `json:"max_zones"`
	CurrentZoneCount  int               `json:"current_zone_count"`
	PlayerTier        int               `json:"player_tier"`
}

type ZoneWithDetails struct {
	Zone            gameplay.Zone `json:"zone"`
	DistanceMeters  float64       `json:"distance_meters"`
	CanEnter        bool          `json:"can_enter"`
	ActiveArtifacts int           `json:"active_artifacts"`
	ActiveGear      int           `json:"active_gear"`
	ActivePlayers   int           `json:"active_players"`
	ExpiresAt       *int64        `json:"expires_at,omitempty"`
	TimeToExpiry    *string       `json:"time_to_expiry,omitempty"`
	Biome           string        `json:"biome"`
	DangerLevel     string        `json:"danger_level"`
}

type LocationPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type PlayerInZone struct {
	Username string    `json:"username"`
	Tier     int       `json:"tier"`
	LastSeen time.Time `json:"last_seen"`
	Distance float64   `json:"distance_meters"`
}

// ✅ NEW: Session tracking structures
type SessionTracker struct {
	EnteredAt      time.Time `json:"entered_at"`
	ItemsCollected int       `json:"items_collected"`
	XPGained       int       `json:"xp_gained"`
	ZoneID         string    `json:"zone_id"`
	ZoneName       string    `json:"zone_name"`
	LastActivity   time.Time `json:"last_activity"`
}

type ExitZoneResponse struct {
	Message        string       `json:"message"`
	ExitedAt       int64        `json:"exited_at"`
	ZoneName       string       `json:"zone_name"`
	TimeInZone     string       `json:"time_in_zone"`
	ItemsCollected int          `json:"items_collected"`
	XPGained       int          `json:"xp_gained"`
	TotalXPGained  int          `json:"total_xp_gained"`
	SessionStats   SessionStats `json:"session_stats"`
}

type SessionStats struct {
	EnteredAt           int64   `json:"entered_at"`
	DurationSeconds     int     `json:"duration_seconds"`
	AverageItemsPerHour float64 `json:"average_items_per_hour"`
	BiomeExplored       string  `json:"biome_explored"`
	DangerLevelFaced    string  `json:"danger_level_faced"`
}

type ZoneTemplate struct {
	Names                []string               `json:"names"`
	Biome                string                 `json:"biome"`
	DangerLevel          string                 `json:"danger_level"`
	MinTierRequired      int                    `json:"min_tier_required"`
	AllowedArtifacts     []string               `json:"allowed_artifacts"`
	ExclusiveArtifacts   []string               `json:"exclusive_artifacts"`
	ArtifactSpawnRates   map[string]float64     `json:"artifact_spawn_rates"`
	GearSpawnRates       map[string]float64     `json:"gear_spawn_rates"`
	EnvironmentalEffects map[string]interface{} `json:"environmental_effects"`
}

type ArtifactTemplate struct {
	Type        string  `json:"type"`
	DisplayName string  `json:"display_name"`
	Rarity      string  `json:"rarity"`
	Biome       string  `json:"biome"`
	Exclusive   bool    `json:"exclusive"`
	SpawnRate   float64 `json:"spawn_rate"`
}

func NewHandler(db *gorm.DB, redisClient *redis_client.Client) *Handler {
	return &Handler{
		db:             db,
		redis:          redisClient,
		loadoutService: loadout.NewService(db),
		gearService:    NewGearService(db),
	}
}
