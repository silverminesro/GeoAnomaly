package loadout

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db      *gorm.DB
	service *Service
}

func NewHandler(db *gorm.DB) *Handler {
	service := NewService(db)
	return &Handler{
		db:      db,
		service: service,
	}
}

// GetLoadoutSlots vráti všetky dostupné sloty
func (h *Handler) GetLoadoutSlots(c *gin.Context) {
	slots, err := h.service.GetLoadoutSlots()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get loadout slots"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"slots":   slots,
	})
}

// GetUserLoadout vráti aktuálny loadout používateľa
func (h *Handler) GetUserLoadout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	loadout, err := h.service.GetUserLoadout(userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user loadout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"loadout": loadout,
	})
}

// EquipItem vybaví gear na daný slot
func (h *Handler) EquipItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		ItemID string `json:"item_id" binding:"required"`
		SlotID string `json:"slot_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	itemUUID, err := uuid.Parse(req.ItemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	err = h.service.EquipItem(userUUID, itemUUID, req.SlotID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item equipped successfully",
	})
}

// UnequipItem odvybaví gear zo slotu
func (h *Handler) UnequipItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	slotID := c.Param("slot_id")
	if slotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Slot ID required"})
		return
	}

	err := h.service.UnequipItem(userUUID, slotID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item unequipped successfully",
	})
}

// RepairItem opraví durability gearu
func (h *Handler) RepairItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	slotID := c.Param("slot_id")
	if slotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Slot ID required"})
		return
	}

	var req struct {
		RepairAmount int `json:"repair_amount"` // 0 = full repair
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	err := h.service.RepairItem(userUUID, slotID, req.RepairAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item repaired successfully",
	})
}

// GetLoadoutStats vráti štatistiky loadoutu
func (h *Handler) GetLoadoutStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	stats, err := h.service.GetLoadoutStats(userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get loadout stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// GetGearCategories vráti všetky kategórie gearu
func (h *Handler) GetGearCategories(c *gin.Context) {
	categories, err := h.service.GetGearCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get gear categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"categories": categories,
	})
}
