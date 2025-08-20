package menu

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	service           *Service
	googlePlayBilling *GooglePlayBillingService
}

func NewHandler(db *gorm.DB) *Handler {
	service := NewService(db)
	googlePlayBilling := NewGooglePlayBillingService(db)
	return &Handler{
		service:           service,
		googlePlayBilling: googlePlayBilling,
	}
}

// GetService returns the service instance (for middleware access)
func (h *Handler) GetService() *Service {
	return h.service
}

// Currency endpoints
func (h *Handler) GetUserCurrency(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	currencyType := c.Param("type")
	if currencyType != CurrencyCredits && currencyType != CurrencyEssence {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency type"})
		return
	}

	currency, err := h.service.GetUserCurrency(userID.(uuid.UUID), currencyType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"currency": currency,
		"message":  "Currency retrieved successfully",
	})
}

func (h *Handler) GetAllUserCurrencies(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	credits, err := h.service.GetUserCurrency(userID.(uuid.UUID), CurrencyCredits)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	essence, err := h.service.GetUserCurrency(userID.(uuid.UUID), CurrencyEssence)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"currencies": gin.H{
			"credits": credits,
			"essence": essence,
		},
		"message": "All currencies retrieved successfully",
	})
}

// Market endpoints
func (h *Handler) GetMarketItems(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	category := c.Query("category")
	rarity := c.Query("rarity")

	items, err := h.service.GetMarketItems(userID.(uuid.UUID), category, rarity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":   items,
		"count":   len(items),
		"message": "Market items retrieved successfully",
	})
}

func (h *Handler) PurchaseMarketItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		ItemID       uuid.UUID `json:"item_id" binding:"required"`
		Quantity     int       `json:"quantity" binding:"required,min=1"`
		CurrencyType string    `json:"currency_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.CurrencyType != CurrencyCredits && request.CurrencyType != CurrencyEssence {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency type"})
		return
	}

	err := h.service.PurchaseMarketItem(userID.(uuid.UUID), request.ItemID, request.Quantity, request.CurrencyType)
	if err != nil {
		switch err {
		case ErrInsufficientFunds:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		case ErrItemNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		case ErrItemNotAvailable:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Item not available"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Item purchased successfully",
	})
}

// Essence package endpoints
func (h *Handler) GetEssencePackages(c *gin.Context) {
	packages, err := h.service.GetEssencePackages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"packages": packages,
		"count":    len(packages),
		"message":  "Essence packages retrieved successfully",
	})
}

func (h *Handler) PurchaseEssencePackage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		PackageID       uuid.UUID `json:"package_id" binding:"required"`
		PaymentMethod   string    `json:"payment_method" binding:"required"`
		PaymentCurrency string    `json:"payment_currency" binding:"required"`
		PaymentAmount   int       `json:"payment_amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate payment currency
	validCurrencies := []string{"USD", "EUR", "GBP"}
	isValidCurrency := false
	for _, currency := range validCurrencies {
		if request.PaymentCurrency == currency {
			isValidCurrency = true
			break
		}
	}

	if !isValidCurrency {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment currency"})
		return
	}

	err := h.service.PurchaseEssencePackage(
		userID.(uuid.UUID),
		request.PackageID,
		request.PaymentMethod,
		request.PaymentCurrency,
		request.PaymentAmount,
	)

	if err != nil {
		switch err {
		case ErrPackageNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Essence package not found"})
		case ErrItemNotAvailable:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Package not available"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Essence package purchased successfully",
	})
}

// Inventory selling endpoints
func (h *Handler) SellInventoryItem(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	itemIDStr := c.Param("id")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	err = h.service.SellInventoryItem(userID.(uuid.UUID), itemID)
	if err != nil {
		switch err {
		case ErrItemNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		case ErrItemEquipped:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot sell equipped item - unequip it first"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Item sold successfully",
	})
}

// Transaction history endpoints
func (h *Handler) GetUserTransactions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	transactions, err := h.service.GetUserTransactions(userID.(uuid.UUID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"count":        len(transactions),
		"message":      "Transaction history retrieved successfully",
	})
}

func (h *Handler) GetUserPurchases(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	purchases, err := h.service.GetUserPurchases(userID.(uuid.UUID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"purchases": purchases,
		"count":     len(purchases),
		"message":   "Purchase history retrieved successfully",
	})
}

func (h *Handler) GetUserEssencePurchases(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	purchases, err := h.service.GetUserEssencePurchases(userID.(uuid.UUID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"purchases": purchases,
		"count":     len(purchases),
		"message":   "Essence purchase history retrieved successfully",
	})
}

// Admin endpoints for managing market items
func (h *Handler) CreateMarketItem(c *gin.Context) {
	var item MarketItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.db.Create(&item).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"item":    item,
		"message": "Market item created successfully",
	})
}

func (h *Handler) UpdateMarketItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	var updates MarketItem
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var item MarketItem
	err = h.service.db.First(&item, itemID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Market item not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	err = h.service.db.Model(&item).Updates(updates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item":    item,
		"message": "Market item updated successfully",
	})
}

func (h *Handler) DeleteMarketItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	err = h.service.db.Delete(&MarketItem{}, itemID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Market item deleted successfully",
	})
}

// Admin endpoints for managing essence packages
func (h *Handler) CreateEssencePackage(c *gin.Context) {
	var pkg EssencePackage
	if err := c.ShouldBindJSON(&pkg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.db.Create(&pkg).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"package": pkg,
		"message": "Essence package created successfully",
	})
}

func (h *Handler) UpdateEssencePackage(c *gin.Context) {
	packageIDStr := c.Param("id")
	packageID, err := uuid.Parse(packageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid package ID"})
		return
	}

	var updates EssencePackage
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pkg EssencePackage
	err = h.service.db.First(&pkg, packageID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Essence package not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	err = h.service.db.Model(&pkg).Updates(updates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"package": pkg,
		"message": "Essence package updated successfully",
	})
}

func (h *Handler) DeleteEssencePackage(c *gin.Context) {
	packageIDStr := c.Param("id")
	packageID, err := uuid.Parse(packageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid package ID"})
		return
	}

	err = h.service.db.Delete(&EssencePackage{}, packageID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Essence package deleted successfully",
	})
}

// Tier package endpoints
func (h *Handler) GetTierPackages(c *gin.Context) {
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var tiers []struct {
		TierLevel              int     `json:"tier_level"`
		TierName               string  `json:"tier_name"`
		PriceMonthly           float64 `json:"price_monthly"`
		MaxZonesPerScan        int     `json:"max_zones_per_scan"`
		CollectCooldownSeconds int     `json:"collect_cooldown_seconds"`
		ScanCooldownMinutes    int     `json:"scan_cooldown_minutes"`
		InventorySlots         int     `json:"inventory_slots"`
		Features               string  `json:"features"` // Zmenené na string pre JSONB
	}

	err := h.service.db.Table("tier_definitions").
		Where("tier_level > 0 AND tier_level < 4"). // Len user tiers, nie admin
		Find(&tiers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tier_packages": tiers,
		"count":         len(tiers),
		"message":       "Tier packages retrieved successfully",
	})
}

func (h *Handler) PurchaseTierPackage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		TierLevel       int    `json:"tier_level" binding:"required"`
		DurationMonths  int    `json:"duration_months" binding:"required,min=1,max=12"`
		PaymentMethod   string `json:"payment_method" binding:"required"`
		PaymentCurrency string `json:"payment_currency" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate tier level
	if request.TierLevel < 1 || request.TierLevel > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tier level"})
		return
	}

	// Validate payment currency
	if request.PaymentCurrency != "USD" && request.PaymentCurrency != "EUR" && request.PaymentCurrency != "GBP" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment currency"})
		return
	}

	err := h.service.PurchaseTierPackage(
		userID.(uuid.UUID),
		request.TierLevel,
		request.DurationMonths,
		request.PaymentMethod,
		request.PaymentCurrency,
	)

	if err != nil {
		// Log error details
		fmt.Printf("Tier purchase error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tier package purchased successfully",
	})
}

func (h *Handler) GetUserTierHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	purchases, err := h.service.GetUserTierPurchases(userID.(uuid.UUID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"purchases": purchases,
		"count":     len(purchases),
		"message":   "Tier purchase history retrieved successfully",
	})
}

// Admin endpoint to manually check and reset expired tiers
func (h *Handler) CheckExpiredTiers(c *gin.Context) {
	// Check if user is admin (you might want to add admin middleware)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// TODO: Add admin check here
	// For now, allow any authenticated user to run this

	count, err := h.service.CheckAndResetAllExpiredTiers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Checked and reset %d expired tiers", count),
		"count":   count,
	})
}

// Google Play Billing endpoints

// POST /api/v1/menu/google-play/verify-purchase
func (h *Handler) VerifyGooglePlayPurchase(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req PurchaseVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process the Google Play purchase
	err := h.googlePlayBilling.ProcessGooglePlayPurchase(userID.(uuid.UUID), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Acknowledge the purchase with Google Play
	if err := h.googlePlayBilling.AcknowledgePurchase(req.PurchaseToken, req.ProductID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to acknowledge purchase: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Google Play purchase verified and processed successfully",
		"purchase_token": req.PurchaseToken,
		"product_id":     req.ProductID,
	})
}

// POST /api/v1/menu/google-play/verify-subscription
func (h *Handler) VerifyGooglePlaySubscription(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req PurchaseVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process the Google Play subscription
	err := h.googlePlayBilling.ProcessGooglePlaySubscription(userID.(uuid.UUID), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Acknowledge the subscription with Google Play
	if err := h.googlePlayBilling.AcknowledgeSubscription(req.PurchaseToken, req.ProductID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to acknowledge subscription: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Google Play subscription verified and processed successfully",
		"purchase_token": req.PurchaseToken,
		"product_id":     req.ProductID,
	})
}
