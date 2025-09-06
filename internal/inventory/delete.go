package inventory

import (
	"fmt"
	"net/http"
	"time"

	"geoanomaly/internal/gameplay"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DELETE /api/v1/inventory/:id
func (h *Handler) DeleteItem(c *gin.Context) {
	fmt.Println("🔍 DeleteItem method called!")
	fmt.Printf("🔍 Request path: %s\n", c.Request.URL.Path)

	userID, exists := c.Get("user_id")
	fmt.Printf("🔍 UserID from context: exists=%v, value=%v\n", exists, userID)

	if !exists {
		fmt.Println("❌ User ID not found in context!")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	itemID := c.Param("id")
	fmt.Printf("🔍 Item ID from URL: %s\n", itemID)

	if itemID == "" {
		fmt.Println("❌ Item ID is empty!")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	// Parse UUIDs (to be sure types match)
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		fmt.Printf("❌ Invalid item UUID: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}
	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		fmt.Printf("❌ Invalid user UUID in context: %v\n", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
		return
	}

	// Find item by model (not map)
	var item gameplay.InventoryItem
	if err := h.db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", itemUUID, userUUID).First(&item).Error; err != nil {
		fmt.Printf("❌ Failed to find item: %v\n", err)
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find item"})
		}
		return
	}

	// Soft delete
	now := time.Now()
	if err := h.db.Model(&item).Update("deleted_at", now).Error; err != nil {
		fmt.Printf("❌ Failed to delete item: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}

	itemName := "Unknown Item"
	if n, ok := item.Properties["name"].(string); ok {
		itemName = n
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item deleted successfully",
		"deleted_item": gin.H{
			"id":        item.ID,
			"item_type": item.ItemType,
			"name":      itemName,
		},
		"timestamp": now.Format(time.RFC3339),
	})
}
