package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Google Play Billing structures
type GooglePlayBillingService struct {
	db *gorm.DB
}

type PurchaseVerificationRequest struct {
	PurchaseToken    string `json:"purchase_token" binding:"required"`
	ProductID        string `json:"product_id" binding:"required"`
	OrderID          string `json:"order_id" binding:"required"`
	PurchaseTime     int64  `json:"purchase_time"`
	PurchaseState    int    `json:"purchase_state"`
	DeveloperPayload string `json:"developer_payload,omitempty"`
}

type GooglePlayVerificationResponse struct {
	OrderId                       string `json:"orderId"`
	PackageName                   string `json:"packageName"`
	ProductId                     string `json:"productId"`
	PurchaseTime                  int64  `json:"purchaseTime"`
	PurchaseState                 int    `json:"purchaseState"`
	PurchaseToken                 string `json:"purchaseToken"`
	DeveloperPayload              string `json:"developerPayload"`
	PurchaseTimeMillis            string `json:"purchaseTimeMillis"`
	PurchaseStateChangeTimeMillis string `json:"purchaseStateChangeTimeMillis"`
	Acknowledged                  bool   `json:"acknowledged"`
	Kind                          string `json:"kind"`
}

type GooglePlaySubscriptionResponse struct {
	OrderId                       string `json:"orderId"`
	PackageName                   string `json:"packageName"`
	ProductId                     string `json:"productId"`
	PurchaseTime                  int64  `json:"purchaseTime"`
	PurchaseState                 int    `json:"purchaseState"`
	PurchaseToken                 string `json:"purchaseToken"`
	DeveloperPayload              string `json:"developerPayload"`
	PurchaseTimeMillis            string `json:"purchaseTimeMillis"`
	PurchaseStateChangeTimeMillis string `json:"purchaseStateChangeTimeMillis"`
	Acknowledged                  bool   `json:"acknowledged"`
	Kind                          string `json:"kind"`
	ExpiryTimeMillis              string `json:"expiryTimeMillis"`
	AutoRenewing                  bool   `json:"autoRenewing"`
	PriceAmountMicros             string `json:"priceAmountMicros"`
	PriceCurrencyCode             string `json:"priceCurrencyCode"`
	IntroductoryPriceInfo         *struct {
		IntroductoryPriceAmountMicros string `json:"introductoryPriceAmountMicros"`
		IntroductoryPriceCurrencyCode string `json:"introductoryPriceCurrencyCode"`
	} `json:"introductoryPriceInfo,omitempty"`
}

// Product mapping from Google Play to our tiers
var ProductToTierMapping = map[string]int{
	"tier_1_monthly": 1,
	"tier_1_yearly":  1,
	"tier_2_monthly": 2,
	"tier_2_yearly":  2,
	"tier_3_monthly": 3,
	"tier_3_yearly":  3,
}

// Duration mapping
var ProductToDurationMapping = map[string]int{
	"tier_1_monthly": 1,
	"tier_1_yearly":  12,
	"tier_2_monthly": 1,
	"tier_2_yearly":  12,
	"tier_3_monthly": 1,
	"tier_3_yearly":  12,
}

func NewGooglePlayBillingService(db *gorm.DB) *GooglePlayBillingService {
	return &GooglePlayBillingService{db: db}
}

// Verify Google Play purchase
func (g *GooglePlayBillingService) VerifyPurchase(purchaseToken, productID string) (*GooglePlayVerificationResponse, error) {
	// Google Play Developer API endpoint
	url := fmt.Sprintf("https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s",
		getGooglePlayPackageName(), productID, purchaseToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	accessToken, err := g.getGooglePlayAccessToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google Play API error: %d - %s", resp.StatusCode, string(body))
	}

	var verificationResponse GooglePlayVerificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&verificationResponse); err != nil {
		return nil, err
	}

	return &verificationResponse, nil
}

// Verify Google Play subscription
func (g *GooglePlayBillingService) VerifySubscription(purchaseToken, productID string) (*GooglePlaySubscriptionResponse, error) {
	// Google Play Developer API endpoint for subscriptions
	url := fmt.Sprintf("https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/subscriptions/%s/tokens/%s",
		getGooglePlayPackageName(), productID, purchaseToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	accessToken, err := g.getGooglePlayAccessToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google Play API error: %d - %s", resp.StatusCode, string(body))
	}

	var subscriptionResponse GooglePlaySubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptionResponse); err != nil {
		return nil, err
	}

	return &subscriptionResponse, nil
}

// Process Google Play purchase
func (g *GooglePlayBillingService) ProcessGooglePlayPurchase(userID uuid.UUID, req PurchaseVerificationRequest) error {
	// Verify the purchase with Google Play
	verification, err := g.VerifyPurchase(req.PurchaseToken, req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to verify purchase: %w", err)
	}

	// Check if purchase is valid
	if verification.PurchaseState != 0 { // 0 = purchased
		return errors.New("purchase is not in valid state")
	}

	// Check if already processed
	if g.isPurchaseAlreadyProcessed(req.PurchaseToken) {
		return errors.New("purchase already processed")
	}

	// Get tier and duration from product ID
	tierLevel, exists := ProductToTierMapping[req.ProductID]
	if !exists {
		return fmt.Errorf("unknown product ID: %s", req.ProductID)
	}

	durationMonths := ProductToDurationMapping[req.ProductID]

	// Calculate expiration date
	expiresAt := time.Now().AddDate(0, durationMonths, 0)

	// Process the purchase in database
	return g.db.Transaction(func(tx *gorm.DB) error {
		// Create transaction record
		transaction := Transaction{
			UserID:        userID,
			Type:          TransactionTypePurchase,
			CurrencyType:  "USD", // Google Play uses USD
			Amount:        0,     // No internal currency change for Google Play purchases
			BalanceBefore: 0,
			BalanceAfter:  0,
			Description:   fmt.Sprintf("Google Play Tier %d purchase for %d months", tierLevel, durationMonths),
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		// Create tier purchase record
		purchase := UserTierPurchase{
			UserID:          userID,
			TierLevel:       tierLevel,
			DurationMonths:  durationMonths,
			ExpiresAt:       expiresAt,
			PaymentMethod:   "google_play",
			PaymentCurrency: "USD",
			PaymentAmount:   0, // Will be set from Google Play data
			PaymentStatus:   "completed",
			TransactionID:   transaction.ID,
			// Store Google Play specific data
			Properties: common.JSONB{
				"google_play_purchase_token": req.PurchaseToken,
				"google_play_order_id":       req.OrderID,
				"google_play_product_id":     req.ProductID,
				"google_play_purchase_time":  req.PurchaseTime,
				"google_play_purchase_state": req.PurchaseState,
				"developer_payload":          req.DeveloperPayload,
			},
		}

		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}

		// Update user tier and expiration
		if err := tx.Model(&common.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"tier":         tierLevel,
				"tier_expires": expiresAt,
			}).Error; err != nil {
			return err
		}

		// Mark purchase as processed
		if err := g.markPurchaseAsProcessed(req.PurchaseToken); err != nil {
			return err
		}

		return nil
	})
}

// Process Google Play subscription
func (g *GooglePlayBillingService) ProcessGooglePlaySubscription(userID uuid.UUID, req PurchaseVerificationRequest) error {
	// Verify the subscription with Google Play
	subscription, err := g.VerifySubscription(req.PurchaseToken, req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to verify subscription: %w", err)
	}

	// Check if subscription is active
	if subscription.PurchaseState != 0 { // 0 = purchased
		return errors.New("subscription is not in valid state")
	}

	// Check if already processed
	if g.isPurchaseAlreadyProcessed(req.PurchaseToken) {
		return errors.New("subscription already processed")
	}

	// Get tier and duration from product ID
	tierLevel, exists := ProductToTierMapping[req.ProductID]
	if !exists {
		return fmt.Errorf("unknown product ID: %s", req.ProductID)
	}

	durationMonths := ProductToDurationMapping[req.ProductID]

	// Calculate expiration date from Google Play data
	var expiresAt time.Time
	if subscription.ExpiryTimeMillis != "" {
		expiryTime, err := time.Parse("2006-01-02T15:04:05.999Z", subscription.ExpiryTimeMillis)
		if err == nil {
			expiresAt = expiryTime
		} else {
			expiresAt = time.Now().AddDate(0, durationMonths, 0)
		}
	} else {
		expiresAt = time.Now().AddDate(0, durationMonths, 0)
	}

	// Process the subscription in database
	return g.db.Transaction(func(tx *gorm.DB) error {
		// Create transaction record
		transaction := Transaction{
			UserID:        userID,
			Type:          TransactionTypePurchase,
			CurrencyType:  subscription.PriceCurrencyCode,
			Amount:        0, // No internal currency change for Google Play purchases
			BalanceBefore: 0,
			BalanceAfter:  0,
			Description:   fmt.Sprintf("Google Play Tier %d subscription for %d months", tierLevel, durationMonths),
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		// Create tier purchase record
		purchase := UserTierPurchase{
			UserID:          userID,
			TierLevel:       tierLevel,
			DurationMonths:  durationMonths,
			ExpiresAt:       expiresAt,
			PaymentMethod:   "google_play_subscription",
			PaymentCurrency: subscription.PriceCurrencyCode,
			PaymentAmount:   0, // Will be set from Google Play data
			PaymentStatus:   "completed",
			TransactionID:   transaction.ID,
			// Store Google Play specific data
			Properties: common.JSONB{
				"google_play_purchase_token": req.PurchaseToken,
				"google_play_order_id":       req.OrderID,
				"google_play_product_id":     req.ProductID,
				"google_play_purchase_time":  req.PurchaseTime,
				"google_play_purchase_state": req.PurchaseState,
				"google_play_auto_renewing":  subscription.AutoRenewing,
				"google_play_price_amount":   subscription.PriceAmountMicros,
				"google_play_price_currency": subscription.PriceCurrencyCode,
				"developer_payload":          req.DeveloperPayload,
			},
		}

		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}

		// Update user tier and expiration
		if err := tx.Model(&common.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"tier":         tierLevel,
				"tier_expires": expiresAt,
			}).Error; err != nil {
			return err
		}

		// Mark purchase as processed
		if err := g.markPurchaseAsProcessed(req.PurchaseToken); err != nil {
			return err
		}

		return nil
	})
}

// Check if purchase was already processed
func (g *GooglePlayBillingService) isPurchaseAlreadyProcessed(purchaseToken string) bool {
	var count int64
	g.db.Model(&UserTierPurchase{}).
		Where("properties->>'google_play_purchase_token' = ?", purchaseToken).
		Count(&count)
	return count > 0
}

// Mark purchase as processed
func (g *GooglePlayBillingService) markPurchaseAsProcessed(purchaseToken string) error {
	// This could be implemented as a separate table or using the existing purchase records
	// For now, we'll use the existing purchase records as proof of processing
	return nil
}

// Get Google Play access token (implement OAuth2 flow)
func (g *GooglePlayBillingService) getGooglePlayAccessToken() (string, error) {
	// This should implement OAuth2 flow to get access token from Google
	// For now, return a placeholder
	// TODO: Implement proper OAuth2 flow
	return "", errors.New("Google Play OAuth2 not implemented yet")
}

// Helper function to get package name
func getGooglePlayPackageName() string {
	// This should come from environment variables
	return "com.geoanomaly.app" // Replace with your actual package name
}

// Acknowledge purchase (required by Google Play)
func (g *GooglePlayBillingService) AcknowledgePurchase(purchaseToken, productID string) error {
	url := fmt.Sprintf("https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s:acknowledge",
		getGooglePlayPackageName(), productID, purchaseToken)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	// Add authorization header
	accessToken, err := g.getGooglePlayAccessToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Google Play acknowledge error: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// Acknowledge subscription (required by Google Play)
func (g *GooglePlayBillingService) AcknowledgeSubscription(purchaseToken, productID string) error {
	url := fmt.Sprintf("https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/subscriptions/%s/tokens/%s:acknowledge",
		getGooglePlayPackageName(), productID, purchaseToken)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	// Add authorization header
	accessToken, err := g.getGooglePlayAccessToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Google Play acknowledge error: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}
