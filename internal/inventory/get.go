package inventory

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"geoanomaly/internal/gameplay"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GET /api/v1/inventory/items
func (h *Handler) GetInventory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Get query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	itemType := c.Query("type") // "artifact" or "gear"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	// Build query
	query := h.db.Model(&gameplay.InventoryItem{}).Where("user_id = ? AND deleted_at IS NULL", userID)

	if itemType != "" {
		query = query.Where("item_type = ?", itemType)
	}

	// Get total count
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to count items",
			"details": err.Error(),
		})
		return
	}

	// Get items as maps first (to handle JSONB properly)
	var rawItems []map[string]interface{}
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&rawItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch inventory",
			"details": err.Error(),
		})
		return
	}

	// Calculate pagination
	totalPages := int64(0)
	if totalCount > 0 {
		totalPages = (totalCount + int64(limit) - 1) / int64(limit)
	}

	// Format items with enricher
	var formattedItems []gin.H
	for _, rawItem := range rawItems {
		// ✅ FIX: Handle properties - they might be string in DB
		var properties map[string]interface{}

		switch props := rawItem["properties"].(type) {
		case string:
			// Properties are stored as JSON string, parse them
			if err := json.Unmarshal([]byte(props), &properties); err != nil {
				log.Printf("Failed to parse properties string: %v", err)
				properties = make(map[string]interface{})
			}
		case map[string]interface{}:
			// Properties are already a map
			properties = props
		default:
			log.Printf("Unexpected properties type: %T", rawItem["properties"])
			properties = make(map[string]interface{})
		}

		// Create DTO for enricher
		itemID, _ := rawItem["item_id"].(string)
		itemUUID, _ := uuid.Parse(itemID)

		dto := &InventoryItemDTO{
			ID:         rawItem["id"].(uuid.UUID),
			ItemID:     itemUUID,
			ItemType:   rawItem["item_type"].(string),
			Properties: properties,
			CreatedAt:  rawItem["created_at"].(time.Time).Format(time.RFC3339),
			UpdatedAt:  rawItem["updated_at"].(time.Time).Format(time.RFC3339),
		}

		// Enrich item with display_name and image_url
		h.enricher.EnrichItem(dto)

		// Build response
		itemData := gin.H{
			"id":           dto.ID,
			"user_id":      rawItem["user_id"],
			"item_type":    dto.ItemType,
			"item_id":      dto.ItemID,
			"quantity":     rawItem["quantity"],
			"display_name": dto.DisplayName,
			"properties":   dto.Properties,
			"created_at":   dto.CreatedAt,
			"updated_at":   dto.UpdatedAt,
		}

		// Add image URL if available
		if dto.ImageURL != nil {
			itemData["image_url"] = *dto.ImageURL
		}

		// Extract common properties for backward compatibility
		if name, ok := properties["name"].(string); ok {
			itemData["name"] = name
		}
		if desc, ok := properties["description"].(string); ok {
			itemData["description"] = desc
		}
		if rarity, ok := properties["rarity"].(string); ok {
			itemData["rarity"] = rarity
		}
		if biome, ok := properties["biome"].(string); ok {
			itemData["biome"] = biome
		}

		// Handle gear-specific properties
		itemTypeStr := dto.ItemType
		if itemTypeStr == "gear" {
			// ✅ OPRAVENÉ: Skontroluj či je item skutočne vybavený v loadoute
			var isEquipped bool
			var loadoutCount int64
			h.db.Model(&gameplay.LoadoutItem{}).Where("user_id = ? AND item_id = ?", userID, rawItem["id"]).Count(&loadoutCount)
			isEquipped = loadoutCount > 0
			itemData["is_equipped"] = isEquipped

			// Keep existing equipped field from properties for backward compatibility
			if equipped, ok := properties["equipped"].(bool); ok {
				itemData["equipped"] = equipped
			}
			if slot, ok := properties["slot"].(string); ok {
				itemData["slot"] = slot
			}
		}

		// Add favorite status
		if favorite, ok := properties["favorite"].(bool); ok {
			itemData["favorite"] = favorite
		}

		formattedItems = append(formattedItems, itemData)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"items":   formattedItems,
		"pagination": gin.H{
			"current_page": page,
			"total_pages":  totalPages,
			"total_items":  totalCount,
			"limit":        limit,
		},
		"filter": gin.H{
			"item_type": itemType,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GET /api/v1/inventory/items/:id
func (h *Handler) GetItemDetail(c *gin.Context) {
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

	// Parse UUID
	itemUUID, err := uuid.Parse(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item UUID"})
		return
	}

	// ✅ FIX: Use raw SQL query to avoid GORM model issues
	var id, userIDResult, itemType, itemIDResult string
	var quantity int
	var propertiesJSON string
	var createdAt, updatedAt time.Time
	var deletedAt sql.NullTime

	query := `
		SELECT 
			id, user_id, item_type, item_id, quantity, 
			properties::text, created_at, updated_at, deleted_at
		FROM gameplay.inventory_items 
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	err = h.db.Raw(query, itemUUID, userID).Row().Scan(
		&id, &userIDResult, &itemType, &itemIDResult,
		&quantity, &propertiesJSON, &createdAt, &updatedAt, &deletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		}
		log.Printf("Database error in GetItemDetail: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database error",
			"details": err.Error(),
		})
		return
	}

	// Parse properties JSON
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		log.Printf("Failed to parse properties JSON: %v", err)
		log.Printf("Raw properties: %s", propertiesJSON)
		properties = make(map[string]interface{})
	}

	// Create DTO for enricher
	itemUUID, parseErr := uuid.Parse(itemIDResult)
	if parseErr != nil {
		log.Printf("Failed to parse item_id UUID: %v", parseErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Invalid item ID format",
			"details": parseErr.Error(),
		})
		return
	}

	dto := &InventoryItemDTO{
		ID:         itemUUID,
		ItemID:     itemUUID,
		ItemType:   itemType,
		Properties: properties,
		CreatedAt:  createdAt.Format(time.RFC3339),
		UpdatedAt:  updatedAt.Format(time.RFC3339),
	}

	// Enrich item with display_name and image_url
	h.enricher.EnrichItem(dto)

	// Build response
	response := gin.H{
		"id":           dto.ID,
		"user_id":      userIDResult,
		"item_type":    dto.ItemType,
		"item_id":      dto.ItemID,
		"quantity":     quantity,
		"display_name": dto.DisplayName,
		"properties":   dto.Properties,
		"created_at":   dto.CreatedAt,
		"updated_at":   dto.UpdatedAt,
	}

	// Add image URL if available
	if dto.ImageURL != nil {
		response["image_url"] = *dto.ImageURL
	}

	// Extract common fields from properties for backward compatibility
	if name, ok := properties["name"].(string); ok {
		response["name"] = name
	}
	if desc, ok := properties["description"].(string); ok {
		response["description"] = desc
	}
	if rarity, ok := properties["rarity"].(string); ok {
		response["rarity"] = rarity
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"item":      response,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
