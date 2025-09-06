package inventory

import (
	"fmt"
	"net/http"
	"time"

	"geoanomaly/internal/gameplay"

	"github.com/gin-gonic/gin"
)

// GET /api/v1/inventory/summary
func (h *Handler) GetInventorySummary(c *gin.Context) {
	fmt.Println("🔍 GetInventorySummary method called!")
	fmt.Printf("🔍 Request path: %s\n", c.Request.URL.Path)

	userID, exists := c.Get("user_id")
	fmt.Printf("🔍 UserID from context: exists=%v, value=%v\n", exists, userID)

	if !exists {
		fmt.Println("❌ User ID not found in context!")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	fmt.Printf("✅ User ID found: %v\n", userID)

	// Count by item type - ✅ Use table name explicitly
	var artifactCount, gearCount int64
	h.db.Model(&gameplay.InventoryItem{}).Where("user_id = ? AND item_type = ? AND deleted_at IS NULL", userID, "artifact").Count(&artifactCount)
	h.db.Model(&gameplay.InventoryItem{}).Where("user_id = ? AND item_type = ? AND deleted_at IS NULL", userID, "gear").Count(&gearCount)

	fmt.Printf("✅ Summary counts: artifacts=%d, gear=%d\n", artifactCount, gearCount)

	// ✅ Count by rarity (from properties JSON)
	var rarityStats map[string]int64 = make(map[string]int64)

	rows, err := h.db.Raw(`
		SELECT 
			properties->>'rarity' as rarity,
			COUNT(*) as count
		FROM gameplay.inventory_items 
		WHERE user_id = ? AND deleted_at IS NULL 
		AND properties->>'rarity' IS NOT NULL
		GROUP BY properties->>'rarity'
	`, userID).Rows()

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rarity string
			var count int64
			if err := rows.Scan(&rarity, &count); err == nil {
				rarityStats[rarity] = count
			}
		}
	}

	fmt.Printf("✅ Rarity stats: %v\n", rarityStats)

	// ✅ Count by biome (from properties JSON)
	var biomeStats map[string]int64 = make(map[string]int64)

	biomeRows, err := h.db.Raw(`
		SELECT 
			properties->>'biome' as biome,
			COUNT(*) as count
		FROM gameplay.inventory_items 
		WHERE user_id = ? AND deleted_at IS NULL 
		AND properties->>'biome' IS NOT NULL
		GROUP BY properties->>'biome'
	`, userID).Rows()

	if err == nil {
		defer biomeRows.Close()
		for biomeRows.Next() {
			var biome string
			var count int64
			if err := biomeRows.Scan(&biome, &count); err == nil {
				biomeStats[biome] = count
			}
		}
	}

	fmt.Printf("✅ Biome stats: %v\n", biomeStats)

	// ✅ CRITICAL FIX: Return JSON object, NOT string!
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"summary": gin.H{ // ✅ THIS IS A JSON OBJECT!
			"total_items":     artifactCount + gearCount,
			"total_artifacts": artifactCount,
			"total_gear":      gearCount,
			"by_rarity":       rarityStats,
			"by_biome":        biomeStats,
		},
		"message":   "Inventory summary retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
