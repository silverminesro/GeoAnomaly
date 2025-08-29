package scanner

import (
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"geoanomaly/internal/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
	db      *gorm.DB
}

func NewHandler(service *Service, db *gorm.DB) *Handler {
	return &Handler{service: service, db: db}
}

// GetScannerInstance - vráti scanner inštanciu hráča
func (h *Handler) GetScannerInstance(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	instance, _, err := h.service.GetOrCreateScannerInstance(userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"scanner": instance,
	})
}

// Scan - vykoná skenovanie
func (h *Handler) Scan(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// ✅ NOVÉ: Aktualizuj polohu v player_sessions
	h.updatePlayerLocation(userUUID, req.Latitude, req.Longitude)

	// Validácia heading hodnoty - ak je NaN alebo Infinity, použij 0.0
	if math.IsNaN(req.Heading) || math.IsInf(req.Heading, 0) {
		req.Heading = 0.0
	}

	response, err := h.service.Scan(userUUID, &req)
	if err != nil {
		// Ak je chyba "must enter zone first", vráť 400 Bad Request
		if err.Error() == "must enter zone first to use scanner" ||
			strings.Contains(err.Error(), "must enter zone first") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Must enter zone first",
				"message": "You must enter a zone before using the scanner",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ✅ NOVÉ: updatePlayerLocation - aktualizuje polohu v player_sessions
func (h *Handler) updatePlayerLocation(userID uuid.UUID, latitude, longitude float64) {
	// Aktualizuj player session s novou polohou
	session := common.PlayerSession{
		UserID:                userID,
		LastSeen:              time.Now(),
		IsOnline:              true,
		LastLocationLatitude:  latitude,
		LastLocationLongitude: longitude,
		LastLocationTimestamp: time.Now(),
	}

	// Upsert player session
	if err := h.db.Where("user_id = ?", userID).Assign(session).FirstOrCreate(&session).Error; err != nil {
		log.Printf("❌ Failed to update player location for user %s: %v", userID, err)
	} else {
		log.Printf("📍 Player location updated for user %s: [%.6f, %.6f]", userID, latitude, longitude)
	}
}

// GetScannerStats - vráti stats scanner
func (h *Handler) GetScannerStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	instance, _, err := h.service.GetOrCreateScannerInstance(userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stats, err := h.service.CalculateScannerStats(instance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// GetScannerCatalog - vráti katalóg scannerov
func (h *Handler) GetScannerCatalog(c *gin.Context) {
	var scanners []ScannerCatalog

	// Explicitne vyber všetky polia okrem computed fields
	if err := h.db.Select("id, code, name, tagline, description, base_range_m, base_fov_deg, caps_json, drain_mult, allowed_modules, slot_count, slot_types, is_basic, max_rarity, detect_artifacts, detect_gear, version, effective_from, created_at, updated_at").
		Find(&scanners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load scanner catalog"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"scanners": scanners,
	})
}

// GetScannerByCode - vráti scanner podľa kódu
func (h *Handler) GetScannerByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Scanner code is required"})
		return
	}

	scanner, err := h.service.GetScannerByCode(code)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scanner not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"scanner": scanner,
	})
}

// GetModuleCatalog - vráti katalóg modulov
func (h *Handler) GetModuleCatalog(c *gin.Context) {
	var modules []ModuleCatalog

	// Explicitne vyber všetky polia okrem computed fields
	if err := h.db.Select("id, code, name, type, effects_json, energy_cost, drain_mult, compatible_scanners, craft_json, store_price, version, created_at, updated_at").
		Find(&modules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load module catalog"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"modules": modules,
	})
}

// GetSecureZoneData returns encrypted zone data for client-side processing
func (h *Handler) GetSecureZoneData(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userUUID, ok := userIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}
	zoneID := c.Param("zone_id")

	if zoneID == "" {
		c.JSON(400, gin.H{"error": "zone_id is required"})
		return
	}

	log.Printf("🔐 [SCANNER] secure-data for user=%s zone=%s", userUUID, zoneID)

	secureData, err := h.service.GetSecureZoneData(zoneID, userUUID.String())
	if err != nil {
		log.Printf("Failed to get secure zone data: %v", err)
		c.JSON(500, gin.H{"error": "Failed to get zone data"})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data":    secureData,
	})
}

// ✅ REMOVED: ValidateClaim handler - now using CollectItem system
// Scanner now integrates with /game/zones/{zone_id}/collect endpoint
