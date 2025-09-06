package inventory

import (
	"net/http"
	"time"

	"geoanomaly/internal/gameplay"

	"github.com/gin-gonic/gin"
)

// EquipItem vybaví gear na daný slot, odvybaví predchádzajúci gear v slote
func (h *Handler) EquipItem(c *gin.Context) {
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

	// Nájdi item v inventári používateľa
	var item gameplay.InventoryItem
	if err := h.db.Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if item.ItemType != "gear" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only gear items can be equipped"})
		return
	}

	// Musí mať slot v properties (napr. "slot": "head")
	slotValue, ok := item.Properties["slot"].(string)
	if !ok || slotValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gear is missing slot information"})
		return
	}

	// Odvybav všetky geary tohto používateľa na tomto slote
	var itemsToUnequip []gameplay.InventoryItem
	if err := h.db.Where("user_id = ? AND item_type = ? AND properties->>'slot' = ?", userID, "gear", slotValue).Find(&itemsToUnequip).Error; err == nil {
		for i := range itemsToUnequip {
			if eq, _ := itemsToUnequip[i].Properties["equipped"].(bool); eq {
				itemsToUnequip[i].Properties["equipped"] = false
				h.db.Model(&itemsToUnequip[i]).Update("properties", itemsToUnequip[i].Properties)
			}
		}
	}

	// Vybav zvolený item (pridaj equipped: true + equipped_at)
	item.Properties["equipped"] = true
	item.Properties["equipped_at"] = time.Now().Format(time.RFC3339)
	if err := h.db.Model(&item).Update("properties", item.Properties).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to equip item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Gear equipped successfully",
		"item_id":   item.ID,
		"slot":      slotValue,
		"equipped":  true,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
