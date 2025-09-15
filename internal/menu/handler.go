package menu

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	includeLocked := c.Query("include_locked") == "true"

	items, err := h.service.GetMarketItems(userID.(uuid.UUID), category, rarity, includeLocked)
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
		ItemID         string     `json:"item_id" binding:"required"` // Môže byť UUID alebo Flutter type
		Quantity       int        `json:"quantity" binding:"required,min=1"`
		CurrencyType   string     `json:"currency_type" binding:"required"`
		IdempotencyKey *uuid.UUID `json:"idempotency_key,omitempty"` // Phase 1: For safe retry
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.CurrencyType != CurrencyCredits && request.CurrencyType != CurrencyEssence {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency type"})
		return
	}

	// Konvertuj ItemID na UUID (ak je to UUID) alebo použij ako string
	var itemID uuid.UUID
	var err error
	
	// Skús parsovať ako UUID
	if itemID, err = uuid.Parse(request.ItemID); err != nil {
		// Ak to nie je UUID, vytvor fake UUID z stringu pre kompatibilitu
		itemID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.ItemID))
	}

	// Phase 1: Use idempotent purchase method
	purchase, err := h.service.PurchaseMarketItemIdempotent(userID.(uuid.UUID), itemID, request.Quantity, request.CurrencyType, request.IdempotencyKey)
	if err != nil {
		switch err {
		case ErrInsufficientFunds:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		case ErrPurchaseLimit:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Purchase limit exceeded"})
		case ErrOutOfStock:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Not enough stock"})
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
		"purchase": purchase,
		"message":  "Item purchased successfully",
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

// =====================================================
// PHASE 2 - ORDER SYSTEM ENDPOINTS
// =====================================================

// CreateOrderRequest - request pre vytvorenie objednávky
type CreateOrderRequest struct {
	ItemID          uuid.UUID  `json:"item_id" binding:"required"`
	Quantity        int        `json:"quantity" binding:"required,min=1"`
	ExpediteEssence int        `json:"expedite_essence,omitempty"`
	IdempotencyKey  *uuid.UUID `json:"idempotency_key,omitempty"`
}

// CreateOrderResponse - response pre vytvorenie objednávky
type CreateOrderResponse struct {
	OrderID         uuid.UUID `json:"order_id"`
	ETAAt           string    `json:"eta_at"`
	DepositCredits  int       `json:"deposit_credits"`
	DepositEssence  int       `json:"deposit_essence"`
	ExpediteApplied bool      `json:"expedite_applied"`
	Message         string    `json:"message"`
}

// GetOrdersResponse - response pre zoznam objednávok
type GetOrdersResponse struct {
	Orders []OrderWithDetails `json:"orders"`
	Total  int                `json:"total"`
}

// OrderWithDetails - objednávka s detailmi
type OrderWithDetails struct {
	Order
	MarketItemName string `json:"market_item_name"`
	MarketItemType string `json:"market_item_type"`
	TimeToReady    *int64 `json:"time_to_ready_ms,omitempty"`         // ms do READY_FOR_PICKUP
	TimeToExpiry   *int64 `json:"time_to_pickup_expiry_ms,omitempty"` // ms do expirácie pickup
}

// CompleteOrderRequest - request pre dokončenie objednávky
type CompleteOrderRequest struct {
	IdempotencyKey *uuid.UUID `json:"idempotency_key,omitempty"`
}

// CompleteOrderResponse - response pre dokončenie objednávky
type CompleteOrderResponse struct {
	OrderID           uuid.UUID `json:"order_id"`
	ItemsMinted       int       `json:"items_minted"`
	FinalPriceCredits int       `json:"final_price_credits"`
	FinalPriceEssence int       `json:"final_price_essence"`
	Message           string    `json:"message"`
}

// ExpediteOrderRequest - request pre zrýchlenie objednávky
type ExpediteOrderRequest struct {
	ExpediteEssence int `json:"expedite_essence" binding:"required,min=1"`
}

// ExpediteOrderResponse - response pre zrýchlenie objednávky
type ExpediteOrderResponse struct {
	OrderID         uuid.UUID `json:"order_id"`
	NewETAAt        string    `json:"new_eta_at"`
	ExpediteApplied int       `json:"expedite_applied"`
	Message         string    `json:"message"`
}

// CancelOrderResponse - response pre zrušenie objednávky
type CancelOrderResponse struct {
	OrderID       uuid.UUID `json:"order_id"`
	RefundCredits int       `json:"refund_credits"`
	RefundEssence int       `json:"refund_essence"`
	Message       string    `json:"message"`
}

// CreateOrder - vytvorenie novej objednávky
func (h *Handler) CreateOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vytvor objednávku
	order, err := h.service.CreateOrder(userID.(uuid.UUID), req.ItemID, req.Quantity, req.ExpediteEssence, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vytvor response
	response := CreateOrderResponse{
		OrderID:         order.ID,
		ETAAt:           order.ETAAt.Format(time.RFC3339),
		DepositCredits:  order.DepositAmountCredits,
		DepositEssence:  order.DepositAmountEssence,
		ExpediteApplied: order.ExpediteEssence > 0,
		Message:         "Order created successfully",
	}

	c.JSON(http.StatusCreated, response)
}

// GetOrders - zoznam objednávok hráča
func (h *Handler) GetOrders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Query parametre
	state := c.Query("state")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Získaj objednávky
	orders, total, err := h.service.GetUserOrders(userID.(uuid.UUID), state, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Priprav response s detailmi
	ordersWithDetails := make([]OrderWithDetails, len(orders))
	now := time.Now()

	for i, order := range orders {
		ordersWithDetails[i] = OrderWithDetails{
			Order:          order,
			MarketItemName: order.MarketItem.Name,
			MarketItemType: order.MarketItem.Type,
		}

		// Vypočítaj čas do READY_FOR_PICKUP
		if order.ETAAt != nil && order.State == OrderStateScheduled {
			if order.ETAAt.After(now) {
				timeToReady := order.ETAAt.Sub(now).Milliseconds()
				ordersWithDetails[i].TimeToReady = &timeToReady
			}
		}

		// Vypočítaj čas do expirácie pickup
		if order.PickupExpiresAt != nil && order.State == OrderStateReadyForPickup {
			if order.PickupExpiresAt.After(now) {
				timeToExpiry := order.PickupExpiresAt.Sub(now).Milliseconds()
				ordersWithDetails[i].TimeToExpiry = &timeToExpiry
			}
		}
	}

	response := GetOrdersResponse{
		Orders: ordersWithDetails,
		Total:  total,
	}

	c.JSON(http.StatusOK, response)
}

// CompleteOrder - dokončenie objednávky (doplatenie a mint do inventára)
func (h *Handler) CompleteOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req CompleteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Dokonči objednávku
	result, err := h.service.CompleteOrder(userID.(uuid.UUID), orderID, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := CompleteOrderResponse{
		OrderID:           result.OrderID,
		ItemsMinted:       result.ItemsMinted,
		FinalPriceCredits: result.FinalPriceCredits,
		FinalPriceEssence: result.FinalPriceEssence,
		Message:           "Order completed successfully",
	}

	c.JSON(http.StatusOK, response)
}

// ExpediteOrder - zrýchlenie objednávky za essence
func (h *Handler) ExpediteOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req ExpediteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Zrýchli objednávku
	order, err := h.service.ExpediteOrder(userID.(uuid.UUID), orderID, req.ExpediteEssence)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := ExpediteOrderResponse{
		OrderID:         order.ID,
		NewETAAt:        order.ETAAt.Format(time.RFC3339),
		ExpediteApplied: order.ExpediteEssence,
		Message:         "Order expedited successfully",
	}

	c.JSON(http.StatusOK, response)
}

// CancelOrder - zrušenie objednávky
func (h *Handler) CancelOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Zruš objednávku
	result, err := h.service.CancelOrder(userID.(uuid.UUID), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := CancelOrderResponse{
		OrderID:       result.OrderID,
		RefundCredits: result.RefundCredits,
		RefundEssence: result.RefundEssence,
		Message:       "Order cancelled successfully",
	}

	c.JSON(http.StatusOK, response)
}
