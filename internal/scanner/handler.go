package scanner

import (
	"math"
	"net/http"

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
