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
	force := c.Query("force") == "true"
	fmt.Printf("🔍 DeleteItem called path=%s force=%v\n", c.Request.URL.Path, force)

	userID, exists := c.Get("user_id")
	fmt.Printf("🔍 UserID from context exists=%v\n", exists)

	if !exists {
		fmt.Println("❌ User ID not found in context!")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	itemID := c.Param("id")
	fmt.Printf("🔍 Item ID=%s\n", itemID)

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

	// Transakcia – celé mazanie (a prípadné odpojenie) urobíme atomicky
	tx := h.db.Begin()
	if tx.Error != nil {
		fmt.Printf("❌ Failed to begin tx: %v\n", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			fmt.Printf("❌ Panic during delete tx: %v\n", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unexpected error"})
		}
	}()

	// Dotiahni položku FOR UPDATE (aj keď je už zmazaná, nech vieme odpovedať idempotentne)
	var item gameplay.InventoryItem
	if err := tx.
		Set("gorm:query_option", "FOR UPDATE").
		Where("id = ? AND user_id = ?", itemUUID, userUUID).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Idempotentné – nič nie je potrebné urobiť
			_ = tx.Rollback()
			c.JSON(http.StatusOK, gin.H{
				"success":      true,
				"message":      "Item not found or already deleted",
				"deleted_item": gin.H{"id": itemUUID},
			})
			return
		}
		_ = tx.Rollback()
		fmt.Printf("❌ Failed to find item: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find item"})
		return
	}

	// Ak už je zmazaný, vráť idempotentný OK
	if item.DeletedAt != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Item not found or already deleted",
			"deleted_item": gin.H{
				"id":        item.ID,
				"item_type": item.ItemType,
			},
		})
		return
	}

	// 🔒 Check if item is locked in any activity
	if item.LockedInActivity != nil && *item.LockedInActivity != "" {
		_ = tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Item is currently locked in %s. Cannot delete until activity completes.", *item.LockedInActivity),
		})
		return
	}

	// Zisti „použitie" itemu
	var inUseAsBattery int64
	if err := tx.
		Raw(`
			SELECT COUNT(*) 
			FROM gameplay.deployed_devices 
			WHERE is_active = TRUE AND battery_inventory_id = ?`, item.ID).
		Scan(&inUseAsBattery).Error; err != nil {
		_ = tx.Rollback()
		fmt.Printf("❌ Failed to check battery usage: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check usage"})
		return
	}

	var inUseAsDevice int64
	if err := tx.
		Raw(`
			SELECT COUNT(*) 
			FROM gameplay.deployed_devices 
			WHERE is_active = TRUE AND device_inventory_id = ?`, item.ID).
		Scan(&inUseAsDevice).Error; err != nil {
		_ = tx.Rollback()
		fmt.Printf("❌ Failed to check device usage: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check usage"})
		return
	}

	var inCharging int64
	if err := tx.
		Raw(`
			SELECT COUNT(*) 
			FROM laboratory.battery_charging_sessions 
			WHERE battery_instance_id = ? AND status IN ('pending','charging')`, item.ID).
		Scan(&inCharging).Error; err != nil {
		_ = tx.Rollback()
		fmt.Printf("❌ Failed to check charging sessions: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check usage"})
		return
	}

	// Ak je to scanner (device) a je nasadený – mazanie neumožniť (ani force)
	if item.ItemType == "deployable_scanner" && inUseAsDevice > 0 {
		_ = tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Device is deployed. Remove the deployment before deleting this scanner.",
		})
		return
	}

	// Ak je to batéria a je v používaní: buď blokuj, alebo force-odpoj
	if item.ItemType == "scanner_battery" && (inUseAsBattery > 0 || inCharging > 0) {
		if !force {
			_ = tx.Rollback()
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Battery is in use (deployed or charging). Use ?force=true to detach and delete.",
			})
			return
		}
		// FORCE: odpoj z device a zruš nabíjanie
		if inUseAsBattery > 0 {
			if err := tx.Exec(`
				UPDATE gameplay.deployed_devices
				SET battery_inventory_id = NULL,
					battery_status       = 'removed',
					battery_level        = 0,
					updated_at           = NOW()
				WHERE is_active = TRUE AND battery_inventory_id = ?`,
				item.ID).Error; err != nil {
				_ = tx.Rollback()
				fmt.Printf("❌ Failed to detach battery from device: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detach battery"})
				return
			}
		}
		if inCharging > 0 {
			if err := tx.Exec(`
				UPDATE laboratory.battery_charging_sessions
				SET status = 'cancelled',
					updated_at = NOW()
				WHERE battery_instance_id = ? AND status IN ('pending','charging')`,
				item.ID).Error; err != nil {
				_ = tx.Rollback()
				fmt.Printf("❌ Failed to cancel charging sessions: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel charging"})
				return
			}
		}
	}

	// Soft delete (idempotentne – necháme updated_at)
	now := time.Now()
	if err := tx.Model(&item).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		_ = tx.Rollback()
		fmt.Printf("❌ Failed to soft-delete item: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		fmt.Printf("❌ Commit failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Meno z properties – preferuj display_name, fallback name
	itemName := "Unknown Item"
	if item.Properties != nil {
		// keď ide o JSONB (map[string]any), null môže prísť ako nil
		if v, ok := item.Properties["display_name"]; ok && v != nil {
			if s, ok := v.(string); ok && s != "" {
				itemName = s
			}
		}
		if itemName == "Unknown Item" {
			if v, ok := item.Properties["name"]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
					itemName = s
				}
			}
		}
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
