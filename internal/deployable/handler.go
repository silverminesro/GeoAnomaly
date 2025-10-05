package deployable

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// DeployDevice - umiestni zariadenie na mapu
func (h *Handler) DeployDevice(c *gin.Context) {
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

	var req DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	response, err := h.service.DeployDevice(userUUID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetMyDevices - získa zariadenia hráča
func (h *Handler) GetMyDevices(c *gin.Context) {
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

	devices, err := h.service.GetMyDevices(userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"devices": devices,
	})
}

// GetDeviceDetails - získa detaily zariadenia
func (h *Handler) GetDeviceDetails(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	device, err := h.service.GetDeviceDetails(deviceID, userUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"device":  device,
	})
}

// ScanDevice - skenuje zariadenie
func (h *Handler) ScanDevice(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	var req DeployableScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	response, err := h.service.ScanDeployableDevice(userUUID, deviceID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCooldownStatus - získa status cooldownu
func (h *Handler) GetCooldownStatus(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	cooldown, err := h.service.GetCooldownStatus(userUUID, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cooldown)
}

// HackDevice - hackuje zariadenie
func (h *Handler) HackDevice(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	var req HackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	response, err := h.service.HackDevice(userUUID, deviceID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ClaimDevice - claimne opustené zariadenie
func (h *Handler) ClaimDevice(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	var req ClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	response, err := h.service.ClaimAbandonedDevice(userUUID, deviceID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetNearbyDevices - získa zariadenia v okolí
func (h *Handler) GetNearbyDevices(c *gin.Context) {
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

	// Get query parameters - lat/lng sú povinné
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	radiusStr := c.DefaultQuery("radius_m", "50")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat/lng sú povinné parametre"})
		return
	}

	// Parse coordinates
	latitude, err1 := strconv.ParseFloat(latStr, 64)
	longitude, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "neplatné GPS súradnice"})
		return
	}

	// Validate coordinate ranges
	if latitude < -90 || latitude > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latitude musí byť medzi -90 a 90"})
		return
	}
	if longitude < -180 || longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "longitude musí byť medzi -180 a 180"})
		return
	}

	// Parse and validate radius
	radiusM := 50
	if radiusInt, err := strconv.Atoi(radiusStr); err == nil && radiusInt > 0 && radiusInt <= 1000 {
		radiusM = radiusInt
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "radius_m musí byť medzi 1 a 1000 metrov"})
		return
	}

	devices, err := h.service.GetNearbyDevices(userUUID, latitude, longitude, radiusM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"devices": devices,
	})
}

// GetAbandonedDevices - získa opustené zariadenia v okolí
func (h *Handler) GetAbandonedDevices(c *gin.Context) {
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

	// Get query parameters - lat/lng sú povinné
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	radiusStr := c.DefaultQuery("radius_m", "5000") // Default 5km

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat/lng sú povinné parametre"})
		return
	}

	// Parse coordinates
	latitude, err1 := strconv.ParseFloat(latStr, 64)
	longitude, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "neplatné GPS súradnice"})
		return
	}

	// Validate coordinate ranges
	if latitude < -90 || latitude > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latitude musí byť medzi -90 a 90"})
		return
	}
	if longitude < -180 || longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "longitude musí byť medzi -180 a 180"})
		return
	}

	// Parse and validate radius
	radiusM := 5000 // 5km
	if radiusInt, err := strconv.Atoi(radiusStr); err == nil && radiusInt > 0 && radiusInt <= 10000 {
		radiusM = radiusInt
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "radius_m musí byť medzi 1 a 10000 metrov"})
		return
	}

	// Získať iba opustené zariadenia v okolí
	abandonedDevices, err := h.service.GetAbandonedDevicesInRadius(userUUID, latitude, longitude, radiusM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"abandoned_devices": abandonedDevices,
	})
}

// DeleteDevice - odstráni zariadenie
func (h *Handler) DeleteDevice(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}

	if err := h.service.DeleteDevice(userUUID, deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device deleted successfully",
	})
}

// GetHackTools - získa hackovacie nástroje hráča
func (h *Handler) GetHackTools(c *gin.Context) {
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

	// TODO: Implement hack tools retrieval
	// For now, return empty list
	_ = userUUID // Suppress unused variable warning
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"hack_tools": []HackTool{},
	})
}

// UseHackTool - použije hackovací nástroj
func (h *Handler) UseHackTool(c *gin.Context) {
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

	toolIDStr := c.Param("tool_id")
	_, err := uuid.Parse(toolIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tool ID"})
		return
	}

	var req struct {
		TargetDeviceID uuid.UUID `json:"target_device_id" binding:"required"`
		Latitude       float64   `json:"latitude" binding:"required"`
		Longitude      float64   `json:"longitude" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// TODO: Implement hack tool usage
	// For now, just return success
	_ = userUUID // Suppress unused variable warning
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Hack tool used successfully",
	})
}

// GetMapMarkers - získa mapové markery pre danú pozíciu
func (h *Handler) GetMapMarkers(c *gin.Context) {
	// 1) user_id môže byť uložený ako uuid.UUID alebo string – ošetri oba prípady
	rawID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID not found"})
		return
	}
	var userID uuid.UUID
	switch v := rawID.(type) {
	case uuid.UUID:
		userID = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "neplatné user ID"})
			return
		}
		userID = parsed
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "nepodporovaný typ user ID"})
		return
	}

	// 2. Parsovať GPS súradnice (povinné)
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat a lng sú povinné query parametre"})
		return
	}

	latitude, err1 := strconv.ParseFloat(latStr, 64)
	longitude, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "neplatné GPS súradnice"})
		return
	}

	// 3. Validovať rozsah súradníc
	if latitude < -90 || latitude > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latitude musí byť medzi -90 a 90"})
		return
	}
	if longitude < -180 || longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "longitude musí byť medzi -180 a 180"})
		return
	}

	// 4. Získať markery zo služby
	markers, err := h.service.GetMapMarkers(userID, latitude, longitude)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 5. Vrátiť markery
	c.JSON(http.StatusOK, markers)
}

// RemoveBattery - vyberie vybitú batériu z zariadenia
func (h *Handler) RemoveBattery(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID format"})
		return
	}

	response, err := h.service.RemoveBattery(deviceID, userUUID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vrátiť správny HTTP kód podľa úspechu
	if response.Success {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, response)
	}
}

// AttachBattery - pripojí batériu k zariadeniu
func (h *Handler) AttachBattery(c *gin.Context) {
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

	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID format"})
		return
	}

	var req AttachBatteryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	response, err := h.service.AttachBattery(deviceID, userUUID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vrátiť správny HTTP kód podľa úspechu
	if response.Success {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, response)
	}
}
