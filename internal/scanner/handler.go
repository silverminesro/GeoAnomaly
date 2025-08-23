package scanner

import (
	"log"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

	instance, err := h.service.GetOrCreateScannerInstance(userUUID)
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

	instance, err := h.service.GetOrCreateScannerInstance(userUUID)
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

// GetScannerCatalog - vráti katalóg scannerov (admin endpoint)
func (h *Handler) GetScannerCatalog(c *gin.Context) {
	// TODO: Implement catalog endpoint
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Scanner catalog endpoint - to be implemented",
	})
}

// GetModuleCatalog - vráti katalóg modulov (admin endpoint)
func (h *Handler) GetModuleCatalog(c *gin.Context) {
	// TODO: Implement module catalog endpoint
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Module catalog endpoint - to be implemented",
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

// ValidateClaim validates a claim request and processes the claim
func (h *Handler) ValidateClaim(c *gin.Context) {
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

	var req ClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	success, err := h.service.ValidateClaimRequest(req, userUUID.String())
	if err != nil {
		log.Printf("Claim validation failed: %v", err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if success {
		c.JSON(200, gin.H{
			"success": true,
			"message": "Item claimed successfully",
		})
	} else {
		c.JSON(400, gin.H{"error": "Failed to claim item"})
	}
}
