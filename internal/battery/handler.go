package battery

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler handles HTTP requests for battery management
type Handler struct {
	service *Service
}

// NewHandler creates a new battery handler
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// GetBatteryTypes returns all available battery types
// GET /api/v1/batteries/types
func (h *Handler) GetBatteryTypes(c *gin.Context) {
	response, err := h.service.GetBatteryTypes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBatteryInstances returns all battery instances for a user
// GET /api/v1/batteries/instances
func (h *Handler) GetBatteryInstances(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.GetBatteryInstances(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// PurchaseBattery allows a user to purchase a new battery
// POST /api/v1/batteries/purchase
func (h *Handler) PurchaseBattery(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req PurchaseBatteryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	response, err := h.service.PurchaseBattery(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// PurchaseInsurance allows a user to purchase insurance for a battery
// POST /api/v1/batteries/insurance/purchase
func (h *Handler) PurchaseInsurance(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req PurchaseInsuranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	response, err := h.service.PurchaseInsurance(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// SellBattery allows a user to sell a used battery
// POST /api/v1/batteries/sell
func (h *Handler) SellBattery(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req SellBatteryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	response, err := h.service.SellBattery(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRiskAssessment returns risk assessment for a battery
// GET /api/v1/batteries/:id/risk-assessment
func (h *Handler) GetRiskAssessment(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	batteryInstanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid battery ID"})
		return
	}

	response, err := h.service.GetRiskAssessment(userID, batteryInstanceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetInsuranceClaims returns insurance claims for a user
// GET /api/v1/batteries/insurance/claims
func (h *Handler) GetInsuranceClaims(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.GetInsuranceClaims(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBatteryStats returns battery statistics for a user
// GET /api/v1/batteries/stats
func (h *Handler) GetBatteryStats(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.GetBatteryStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ProcessChargingResult processes the result of a battery charging attempt
// POST /api/v1/batteries/:id/charging-result
func (h *Handler) ProcessChargingResult(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	batteryInstanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid battery ID"})
		return
	}

	response, err := h.service.ProcessChargingResult(batteryInstanceID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"risk_info": response,
		"message":   "Charging result processed",
	})
}

// Helper methods

// getUserID extracts user ID from the request context
func (h *Handler) getUserID(c *gin.Context) (uuid.UUID, error) {
	// SECURITY: Always get user_id from auth middleware context first (secure)
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case string:
			return uuid.Parse(v)
		case uuid.UUID:
			return v, nil
		}
	}

	// FALLBACK: Only for development - query parameter (can be spoofed!)
	userIDStr := c.Query("user_id")
	if userIDStr != "" {
		return uuid.Parse(userIDStr)
	}

	// FALLBACK: Only for development - header (can be spoofed!)
	userIDStr = c.GetHeader("X-User-ID")
	if userIDStr != "" {
		return uuid.Parse(userIDStr)
	}

	return uuid.Nil, fmt.Errorf("user ID required - must be set by auth middleware")
}

// Middleware to require user ID
func (h *Handler) RequireUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := h.getUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID required"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

// Middleware to validate battery instance ID
func (h *Handler) ValidateBatteryInstanceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		batteryInstanceIDStr := c.Param("id")
		if batteryInstanceIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Battery instance ID required"})
			c.Abort()
			return
		}

		batteryInstanceID, err := uuid.Parse(batteryInstanceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid battery instance ID"})
			c.Abort()
			return
		}

		c.Set("battery_instance_id", batteryInstanceID)
		c.Next()
	}
}

// Health check endpoint
// GET /api/v1/batteries/health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "battery-management",
		"version": "1.0.0",
	})
}
