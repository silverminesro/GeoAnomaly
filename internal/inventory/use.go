package inventory

import (
	"geoanomaly/internal/gameplay"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// POST /api/v1/inventory/:id/use
func (h *Handler) UseItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	itemID := c.Param("id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item UUID"})
		return
	}

	var item gameplay.InventoryItem
	if err := h.db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", itemUUID, userID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// 🔒 Check if item is locked in any activity
	if item.LockedInActivity != nil && *item.LockedInActivity != "" {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Item is currently locked in " + *item.LockedInActivity + ". Cannot use until activity completes.",
		})
		return
	}

	// Example: reduce quantity, delete if zero
	if item.Quantity > 1 {
		item.Quantity--
		h.db.Save(&item)
	} else {
		now := time.Now()
		item.DeletedAt = &now
		h.db.Save(&item)
	}

	// TODO: Add custom logic per item_type if needed

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Item used",
		"item_id":      item.ID,
		"new_quantity": item.Quantity,
		"timestamp":    time.Now().Format(time.RFC3339),
	})
}
