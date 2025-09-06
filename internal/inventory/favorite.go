package inventory

import (
	"geoanomaly/internal/gameplay"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PUT /api/v1/inventory/:id/favorite
func (h *Handler) SetFavorite(c *gin.Context) {
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

	// Toggle favorite
	isFavorite := false
	if fav, ok := item.Properties["favorite"].(bool); ok {
		isFavorite = !fav
	} else {
		isFavorite = true
	}
	item.Properties["favorite"] = isFavorite

	h.db.Save(&item)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"favorite":  isFavorite,
		"item_id":   item.ID,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
