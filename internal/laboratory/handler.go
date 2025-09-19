package laboratory

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================
// 1. HANDLER STRUCTURE
// =============================================

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// =============================================
// 2. LABORATORY MANAGEMENT ENDPOINTS
// =============================================

// GetLaboratoryStatus returns complete laboratory status
// GET /api/v1/laboratory/status
func (h *Handler) GetLaboratoryStatus(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get laboratory status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// UpgradeLaboratory upgrades laboratory to next level
// POST /api/v1/laboratory/upgrade
func (h *Handler) UpgradeLaboratory(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req UpgradeLaboratoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.service.UpgradeLaboratory(userID, req.TargetLevel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to upgrade laboratory: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Laboratory upgraded successfully",
		"level":   req.TargetLevel,
	})
}

// PurchaseExtraChargingSlot allows user to buy extra charging slot
// POST /api/v1/laboratory/battery/slots/purchase
func (h *Handler) PurchaseExtraChargingSlot(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req PurchaseSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.service.PurchaseExtraChargingSlot(userID, req.EssenceCost); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to purchase slot: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Extra charging slot purchased successfully",
		"essence_cost": req.EssenceCost,
	})
}

// =============================================
// 3. RESEARCH SYSTEM ENDPOINTS (Level 2+)
// =============================================

// StartResearch starts a research project
// POST /api/v1/laboratory/research/start
func (h *Handler) StartResearch(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req StartResearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	project, err := h.service.StartResearch(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to start research: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Research started successfully",
		"project": project,
	})
}

// GetResearchStatus returns research project status
// GET /api/v1/laboratory/research/status
func (h *Handler) GetResearchStatus(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get all active research projects
	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get research status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, status.ActiveResearch)
}

// CompleteResearch completes a research project
// POST /api/v1/laboratory/research/complete/:id
func (h *Handler) CompleteResearch(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	result, err := h.service.CompleteResearch(userID, projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to complete research: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Research completed successfully",
		"result":  result,
	})
}

// =============================================
// 4. CRAFTING SYSTEM ENDPOINTS (Level 3+)
// =============================================

// StartCrafting starts a crafting session
// POST /api/v1/laboratory/craft/start
func (h *Handler) StartCrafting(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req StartCraftingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	session, err := h.service.StartCrafting(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to start crafting: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Crafting started successfully",
		"session": session,
	})
}

// GetCraftingStatus returns crafting session status
// GET /api/v1/laboratory/craft/status
func (h *Handler) GetCraftingStatus(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get all active crafting sessions
	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get crafting status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, status.ActiveCrafting)
}

// CompleteCrafting completes a crafting session
// POST /api/v1/laboratory/craft/complete/:id
func (h *Handler) CompleteCrafting(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	if err := h.service.CompleteCrafting(userID, sessionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to complete crafting: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Crafting completed successfully",
	})
}

// =============================================
// 5. BATTERY CHARGING ENDPOINTS (Level 1+)
// =============================================

// GetAvailableBatteries returns available batteries from user inventory
// GET /api/v1/laboratory/battery/available
func (h *Handler) GetAvailableBatteries(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	batteries, err := h.service.GetAvailableBatteries(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get available batteries: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"batteries": batteries,
	})
}

// StartBatteryCharging starts charging a battery
// POST /api/v1/laboratory/battery/charge
func (h *Handler) StartBatteryCharging(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req StartChargingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	session, err := h.service.StartBatteryCharging(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to start charging: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Battery charging started successfully",
		"session": session,
	})
}

// GetChargingSlots returns charging slots with their status
// GET /api/v1/laboratory/battery/slots
func (h *Handler) GetChargingSlots(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slots, err := h.service.GetChargingSlots(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get charging slots: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"slots":   slots,
	})
}

// GetBatteryChargingStatus returns charging session status with real-time progress
// GET /api/v1/laboratory/battery/charging-status
func (h *Handler) GetBatteryChargingStatus(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get all active charging sessions with progress calculation
	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get charging status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, status.ActiveCharging)
}

// CompleteBatteryCharging completes a charging session
// POST /api/v1/laboratory/battery/complete/:id
func (h *Handler) CompleteBatteryCharging(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	if err := h.service.CompleteBatteryCharging(userID, sessionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to complete charging: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Battery charging completed successfully",
	})
}

// =============================================
// 6. TASK SYSTEM ENDPOINTS (Level 1+)
// =============================================

// GetAvailableTasks returns available tasks
// GET /api/v1/laboratory/tasks
func (h *Handler) GetAvailableTasks(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tasks, err := h.service.GetAvailableTasks(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tasks: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
	})
}

// UpdateTaskProgress updates task progress
// POST /api/v1/laboratory/tasks/:id/progress
func (h *Handler) UpdateTaskProgress(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	var req struct {
		Progress int `json:"progress" binding:"required,min=0,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateTaskProgress(userID, taskID, req.Progress); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update task progress: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Task progress updated successfully",
	})
}

// ClaimTaskReward claims task reward
// POST /api/v1/laboratory/tasks/:id/claim
func (h *Handler) ClaimTaskReward(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	if err := h.service.ClaimTaskReward(userID, taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to claim task reward: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Task reward claimed successfully",
	})
}

// =============================================
// 7. UTILITY ENDPOINTS
// =============================================

// GetCraftingRecipes returns available crafting recipes
// GET /api/v1/laboratory/recipes
func (h *Handler) GetCraftingRecipes(c *gin.Context) {
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get laboratory level to filter recipes
	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get laboratory status: " + err.Error()})
		return
	}

	// TODO: Get recipes based on laboratory level
	// This would require a service method to get available recipes

	c.JSON(http.StatusOK, gin.H{
		"recipes": []CraftingRecipe{}, // Placeholder
		"level":   status.Laboratory.Level,
	})
}

// =============================================
// 8. LABORATORY PLACEMENT & MAP HANDLERS
// =============================================

// PlaceLaboratory places laboratory on map
func (h *Handler) PlaceLaboratory(c *gin.Context) {
	// Get user ID from context
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request
	var req PlaceLaboratoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("❌ PlaceLaboratory validation error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("✅ PlaceLaboratory request: lat=%.6f, lng=%.6f\n", req.Latitude, req.Longitude)

	// Place laboratory
	result, err := h.service.PlaceLaboratory(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetNearbyLaboratories returns laboratories within specified radius
func (h *Handler) GetNearbyLaboratories(c *gin.Context) {
	// Get user ID from context
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse query parameters
	var req GetNearbyLaboratoriesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get nearby laboratories
	result, err := h.service.GetNearbyLaboratories(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RelocateLaboratory relocates laboratory to new location
func (h *Handler) RelocateLaboratory(c *gin.Context) {
	// Get user ID from context
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request
	var req RelocateLaboratoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Relocate laboratory
	result, err := h.service.RelocateLaboratory(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetRelocationCost returns essence cost for laboratory relocation
func (h *Handler) GetRelocationCost(c *gin.Context) {
	// Get user ID from context
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get laboratory
	status, err := h.service.GetLaboratoryStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get laboratory status"})
		return
	}

	if !status.Laboratory.IsPlaced {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Laboratory must be placed before relocation"})
		return
	}

	// Calculate relocation cost
	relocationCost := h.service.getRelocationCost(status.Laboratory.RelocationCount)
	nextCost := h.service.getRelocationCost(status.Laboratory.RelocationCount + 1)

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"essence_cost":       relocationCost,
		"relocation_count":   status.Laboratory.RelocationCount,
		"next_cost":          nextCost,
		"is_max_cost":        relocationCost == 5000,
		"is_first_placement": status.Laboratory.RelocationCount == 0,
		"cost_progression":   []int{0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000},
	})
}

// =============================================
// 9. MIDDLEWARE FUNCTIONS
// =============================================

// RequireLaboratoryPlaced middleware checks if laboratory is placed on map
func (h *Handler) RequireLaboratoryPlaced() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := h.getUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Get laboratory status
		status, err := h.service.GetLaboratoryStatus(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get laboratory status"})
			c.Abort()
			return
		}

		if status.NeedsPlacement {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Laboratory must be placed on map before using this feature",
				"needs_placement": true,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// =============================================
// 10. HELPER FUNCTIONS
// =============================================

// getUserIDFromContext extracts user ID from Gin context
func (h *Handler) getUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	rawID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}

	var userID uuid.UUID
	switch v := rawID.(type) {
	case uuid.UUID:
		userID = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid user ID format")
		}
		userID = parsed
	default:
		return uuid.Nil, fmt.Errorf("unsupported user ID type")
	}

	return userID, nil
}

// RequireLaboratoryLevel middleware checks if user has required laboratory level
func (h *Handler) RequireLaboratoryLevel(requiredLevel int) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := h.getUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		status, err := h.service.GetLaboratoryStatus(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get laboratory status"})
			c.Abort()
			return
		}

		if status.Laboratory.Level < requiredLevel {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "insufficient laboratory level",
				"required": requiredLevel,
				"current":  status.Laboratory.Level,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireResearchUnlocked middleware checks if research is unlocked
func (h *Handler) RequireResearchUnlocked() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := h.getUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		status, err := h.service.GetLaboratoryStatus(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get laboratory status"})
			c.Abort()
			return
		}

		if !status.Laboratory.ResearchUnlocked {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "research system not unlocked (requires level 2+)",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireCraftingUnlocked middleware checks if crafting is unlocked
func (h *Handler) RequireCraftingUnlocked() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := h.getUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		status, err := h.service.GetLaboratoryStatus(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get laboratory status"})
			c.Abort()
			return
		}

		if !status.Laboratory.CraftingUnlocked {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "crafting system not unlocked (requires level 3+)",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
