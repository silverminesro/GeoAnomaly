package auth

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

// User model - migrovaný do auth.users
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
}

// PlayerSession model - migrovaný do auth.player_sessions
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
}

// TableName methods for GORM schema qualification
func (User) TableName() string {
	return "auth.users"
}

func (PlayerSession) TableName() string {
	return "auth.player_sessions"
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

// LocationWithAccuracy pre user tracking kde potrebujeme accuracy
type LocationWithAccuracy struct {
	Latitude  float64   `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude float64   `json:"longitude" gorm:"type:decimal(11,8)"`
	Accuracy  float64   `json:"accuracy,omitempty"`
	Timestamp time.Time `json:"timestamp" gorm:"autoUpdateTime"`
}
