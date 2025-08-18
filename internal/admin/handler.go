package admin

import (
	"geoanomaly/internal/common"
	"net/http"

	"geoanomaly/internal/game"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	db    *gorm.DB
	redis *redis.Client
}

type CreateEventZoneRequest struct {
	Name           string                  `json:"name" binding:"required"`
	Description    string                  `json:"description"`
	Location       common.Location         `json:"location" binding:"required"`
	RadiusMeters   int                     `json:"radius_meters" binding:"required"`
	TierRequired   int                     `json:"tier_required" binding:"required"`
	EventType      string                  `json:"event_type"`
	Permanent      bool                    `json:"permanent"`
	EventArtifacts []CreateArtifactRequest `json:"event_artifacts"`
}

type CreateArtifactRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Rarity string `json:"rarity"`
}

func NewHandler(db *gorm.DB, redisClient *redis.Client) *Handler {
	return &Handler{
		db:    db,
		redis: redisClient,
	}
}

func (h *Handler) CreateEventZone(c *gin.Context) {
	var req CreateEventZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zone := common.Zone{
		BaseModel:    common.BaseModel{ID: uuid.New()},
		Name:         req.Name,
		Description:  req.Description,
		Location:     req.Location,
		RadiusMeters: req.RadiusMeters,
		TierRequired: req.TierRequired,
		ZoneType:     "event",
		Properties: common.JSONB{
			"event_type": req.EventType,
			"created_by": "admin",
			"permanent":  req.Permanent,
		},
		IsActive: true,
	}

	if err := h.db.Create(&zone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zone"})
		return
	}

	// Spawn event artifacts
	for _, artifact := range req.EventArtifacts {
		h.spawnEventArtifact(zone.ID, artifact)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Event zone created successfully",
		"zone":    zone,
	})
}

func (h *Handler) spawnEventArtifact(zoneID uuid.UUID, artifact CreateArtifactRequest) {
	// Implementation for spawning artifacts
	// TODO: Add artifact creation logic
}

// Add other admin methods as stubs
func (h *Handler) UpdateZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update zone not implemented yet"})
}

func (h *Handler) DeleteZone(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Delete zone not implemented yet"})
}

func (h *Handler) SpawnArtifact(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Spawn artifact not implemented yet"})
}

func (h *Handler) SpawnGear(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Spawn gear not implemented yet"})
}

// AddInventoryItem - pridá predmet do inventára používateľa
func (h *Handler) AddInventoryItem(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter required"})
		return
	}

	var req struct {
		CategoryID string `json:"category_id" binding:"required"`
		ItemType   string `json:"item_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse user ID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Check if user exists
	var user common.User
	if err := h.db.First(&user, "id = ?", userUUID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Handle different item types
	switch req.ItemType {
	case "gear":
		// Create gear item using GearService
		gearService := game.NewGearService(h.db)
		inventoryItem, err := gearService.CreateGearItem(req.CategoryID, userUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gear item"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Gear item added to inventory",
			"item":    inventoryItem,
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported item type"})
	}
}

func (h *Handler) CleanupExpiredZones(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Cleanup expired zones not implemented yet"})
}

func (h *Handler) GetExpiredZones(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get expired zones not implemented yet"})
}

func (h *Handler) GetZoneAnalytics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Zone analytics not implemented yet"})
}

func (h *Handler) GetItemAnalytics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Item analytics not implemented yet"})
}
