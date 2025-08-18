package user

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"geoanomaly/internal/common"
	"geoanomaly/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	redis_client "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	db    *gorm.DB
	redis *redis_client.Client
}

type UpdateProfileRequest struct {
	Username string `json:"username,omitempty" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email,omitempty" binding:"omitempty,email"`
}

type UpdateLocationRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
	Accuracy  float64 `json:"accuracy,omitempty"`
}

// ✅ OPRAVENÉ: Pridané TierExpires pole
type UserProfileResponse struct {
	ID          uuid.UUID              `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	Tier        int                    `json:"tier"`
	TierExpires *time.Time             `json:"tier_expires"`
	XP          int                    `json:"xp"`
	Level       int                    `json:"level"`
	IsActive    bool                   `json:"is_active"`
	CreatedAt   time.Time              `json:"created_at"`
	Stats       UserStats              `json:"stats"`
	Inventory   []common.InventoryItem `json:"inventory,omitempty"`
}

type UserStats struct {
	TotalArtifacts int `json:"total_artifacts"`
	TotalGear      int `json:"total_gear"`
	ZonesVisited   int `json:"zones_visited"`
	Level          int `json:"level"`
}

func NewHandler(db *gorm.DB, redisClient *redis_client.Client) *Handler {
	return &Handler{
		db:    db,
		redis: redisClient,
	}
}

// GetProfile - získanie profilu používateľa
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// Pokús sa najskôr z cache
	cacheKey := "user_profile:" + userID.(uuid.UUID).String()

	var user common.User
	if err := h.db.Preload("Inventory").First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	// Vypočítaj štatistiky
	stats := h.calculateUserStats(user.ID)

	// ✅ OPRAVENÉ: Pridané TierExpires do response
	response := UserProfileResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Tier:        user.Tier,
		TierExpires: user.TierExpires,
		XP:          user.XP,
		Level:       user.Level,
		IsActive:    user.IsActive,
		CreatedAt:   user.CreatedAt,
		Stats:       stats,
		Inventory:   user.Inventory,
	}

	// Cachuj na 5 minút (ak je Redis dostupný)
	if h.redis != nil {
		redis.SetWithExpiration(h.redis, cacheKey, response, 5*time.Minute)
	}

	c.JSON(http.StatusOK, response)
}

// UpdateProfile - aktualizácia profilu
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nájdi používateľa
	var user common.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Aktualizuj len poskytnuté polia
	updates := make(map[string]interface{})

	if req.Username != "" {
		// Skontroluj či username už neexistuje
		var existingUser common.User
		if err := h.db.Where("username = ? AND id != ?", req.Username, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		updates["username"] = req.Username
	}

	if req.Email != "" {
		// Skontroluj či email už neexistuje
		var existingUser common.User
		if err := h.db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		updates["email"] = req.Email
	}

	// Aktualizuj v databáze
	if len(updates) > 0 {
		if err := h.db.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}

		// Vymaž cache (ak je Redis dostupný)
		if h.redis != nil {
			cacheKey := "user_profile:" + userID.(uuid.UUID).String()
			redis.Delete(h.redis, cacheKey)
		}
	}

	// Vráť aktualizovaný profil
	h.GetProfile(c)
}

// ✅ OPRAVENÉ: GetInventory - s lepším error handling pre prázdny inventár
func (h *Handler) GetInventory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// Parametre pre pagináciu
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	itemType := c.Query("type") // artifact, gear

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	// ✅ OPRAVENÉ: Lepšie query building pre inventár
	query := h.db.Model(&common.InventoryItem{}).Where("user_id = ?", userID)

	if itemType != "" {
		query = query.Where("item_type = ?", itemType)
	}

	var inventory []common.InventoryItem
	var totalCount int64

	// ✅ OPRAVENÉ: Spočítaj celkový počet s error handling
	if err := query.Count(&totalCount).Error; err != nil {
		log.Printf("❌ Failed to count inventory items: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to count inventory items",
			"details": err.Error(),
		})
		return
	}

	// ✅ OPRAVENÉ: Získaj items s error handling pre prázdny result
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&inventory).Error; err != nil {
		log.Printf("❌ Failed to fetch inventory items: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch inventory items",
			"details": err.Error(),
		})
		return
	}

	// ✅ OPRAVENÉ: Inicializuj prázdny slice ak je nil
	if inventory == nil {
		inventory = []common.InventoryItem{}
	}

	// Vypočítaj total pages
	totalPages := int64(0)
	if totalCount > 0 {
		totalPages = (totalCount + int64(limit) - 1) / int64(limit)
	}

	// ✅ OPRAVENÉ: Odpoveď s metadátami (aj pre prázdny inventár)
	response := gin.H{
		"success": true,
		"message": "Inventory retrieved successfully",
		"items":   inventory,
		"pagination": gin.H{
			"current_page":   page,
			"total_pages":    totalPages,
			"total_items":    totalCount,
			"items_per_page": limit,
		},
		"filter": gin.H{
			"item_type": itemType,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}

// ✅ OPRAVENÉ: UpdateLocation - používa LocationWithAccuracy
func (h *Handler) UpdateLocation(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validácia GPS súradníc
	if !isValidGPSCoordinate(req.Latitude, req.Longitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GPS coordinates"})
		return
	}

	// ✅ OPRAVENÉ: Vytvor LocationWithAccuracy object
	location := common.LocationWithAccuracy{
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Accuracy:  req.Accuracy,
		Timestamp: time.Now(),
	}

	// ✅ OPRAVENÉ: Aktualizuj player session s error handling
	if err := h.updatePlayerSession(userID.(uuid.UUID), location); err != nil {
		log.Printf("❌ Failed to update player session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update location in player session",
			"details": err.Error(),
		})
		return
	}

	// Vymaž cache (ak je Redis dostupný)
	if h.redis != nil {
		cacheKey := "user_profile:" + userID.(uuid.UUID).String()
		redis.Delete(h.redis, cacheKey)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Location updated successfully in player session",
		"location":  location,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ✅ OPRAVENÉ: Pomocné funkcie s lepším error handling
func (h *Handler) calculateUserStats(userID uuid.UUID) UserStats {
	var stats UserStats

	// ✅ OPRAVENÉ: Use int64 variables then convert
	var totalArtifacts int64
	var totalGear int64

	// Spočítaj artefakty s error handling
	if err := h.db.Model(&common.InventoryItem{}).Where("user_id = ? AND item_type = ?", userID, "artifact").Count(&totalArtifacts).Error; err != nil {
		log.Printf("⚠️ Failed to count artifacts for user %s: %v", userID, err)
		totalArtifacts = 0
	}

	// Spočítaj gear s error handling
	if err := h.db.Model(&common.InventoryItem{}).Where("user_id = ? AND item_type = ?", userID, "gear").Count(&totalGear).Error; err != nil {
		log.Printf("⚠️ Failed to count gear for user %s: %v", userID, err)
		totalGear = 0
	}

	// Convert to int
	stats.TotalArtifacts = int(totalArtifacts)
	stats.TotalGear = int(totalGear)

	// Zóny navštívené (zatiaľ 0, implementujeme neskôr)
	stats.ZonesVisited = 0

	// Level na základe celkového počtu items
	totalItems := stats.TotalArtifacts + stats.TotalGear
	stats.Level = calculateLevel(totalItems)

	return stats
}

// ✅ OPRAVENÉ: updatePlayerSession používa nový PlayerSession model s individual fields
func (h *Handler) updatePlayerSession(userID uuid.UUID, location common.LocationWithAccuracy) error {
	session := common.PlayerSession{
		UserID:   userID,
		LastSeen: time.Now(),
		IsOnline: true,
		// ✅ OPRAVENÉ: Použiť individual fields namiesto embedded struct
		LastLocationLatitude:  location.Latitude,
		LastLocationLongitude: location.Longitude,
		LastLocationAccuracy:  location.Accuracy,
		LastLocationTimestamp: location.Timestamp,
	}

	// ✅ OPRAVENÉ: Upsert player session s error return
	if err := h.db.Where("user_id = ?", userID).Assign(session).FirstOrCreate(&session).Error; err != nil {
		log.Printf("❌ Failed to upsert player session for user %s: %v", userID, err)
		return err
	}

	log.Printf("📍 Player session updated for user %s: [%.6f, %.6f] (accuracy: %.1fm)", userID, location.Latitude, location.Longitude, location.Accuracy)
	return nil
}

func isValidGPSCoordinate(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func calculateLevel(totalItems int) int {
	// Jednoduchá formula pre level
	// Level 1: 0-9 items
	// Level 2: 10-24 items
	// Level 3: 25-49 items
	// atď.
	if totalItems < 10 {
		return 1
	} else if totalItems < 25 {
		return 2
	} else if totalItems < 50 {
		return 3
	} else if totalItems < 100 {
		return 4
	} else {
		return 5 + (totalItems-100)/50
	}
}

// Missing methods - stub implementation
func (h *Handler) GetInventoryByType(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Get inventory by type not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetLocationHistory(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Location history not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetUserStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "User stats not implemented yet",
		"status": "planned",
	})
}

// Add this method to user/handler.go

func (h *Handler) GetAllUsers(c *gin.Context) {
	// Super Admin only - get all users
	var users []common.User
	if err := h.db.Select("id, username, email, tier, is_active, is_banned, created_at, updated_at, xp, level, total_artifacts, total_gear").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch users",
			"details": err.Error(),
		})
		return
	}

	// Count by tiers
	tierCounts := make(map[int]int)
	for _, user := range users {
		tierCounts[user.Tier]++
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"users":        users,
		"total_users":  len(users),
		"tier_counts":  tierCounts,
		"message":      "All users retrieved successfully",
		"timestamp":    time.Now().Format(time.RFC3339),
		"requested_by": c.GetString("username"),
		"admin_level":  c.GetString("admin_level"),
	})
}

func (h *Handler) UpdateUserTier(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Update user tier not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) BanUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Ban user not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) UnbanUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Unban user not implemented yet",
		"status": "planned",
	})
}

func (h *Handler) GetPlayerAnalytics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":  "Player analytics not implemented yet",
		"status": "planned",
	})
}
