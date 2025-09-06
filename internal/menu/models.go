package menu

import (
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
)

// Local User type to avoid import cycles
type User struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Tier        int        `json:"tier"`
	Level       int        `json:"level"`
	TierExpires *time.Time `json:"tier_expires"`
}

func (User) TableName() string {
	return "auth.users"
}

// Currency types
const (
	CurrencyCredits = "credits"
	CurrencyEssence = "essence"
)

// Transaction types
const (
	TransactionTypePurchase = "purchase"
	TransactionTypeSale     = "sale"
	TransactionTypeReward   = "reward"
	TransactionTypeRefund   = "refund"
	TransactionTypeGift     = "gift"
)

// Currency model
type Currency struct {
	common.BaseModel
	UserID      uuid.UUID `json:"user_id" gorm:"not null;index"`
	Type        string    `json:"type" gorm:"not null;size:20"` // credits, essence
	Amount      int       `json:"amount" gorm:"not null;default:0"`
	LastUpdated time.Time `json:"last_updated" gorm:"autoUpdateTime"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// Transaction model
type Transaction struct {
	common.BaseModel
	UserID        uuid.UUID  `json:"user_id" gorm:"not null;index"`
	Type          string     `json:"type" gorm:"not null;size:20"`          // purchase, sale, reward, refund, gift
	CurrencyType  string     `json:"currency_type" gorm:"not null;size:20"` // credits, essence
	Amount        int        `json:"amount" gorm:"not null"`
	BalanceBefore int        `json:"balance_before" gorm:"not null"`
	BalanceAfter  int        `json:"balance_after" gorm:"not null"`
	Description   string     `json:"description" gorm:"type:text"`
	ItemID        *uuid.UUID `json:"item_id,omitempty" gorm:"index"`
	ItemType      string     `json:"item_type,omitempty" gorm:"size:20"`  // artifact, gear
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty" gorm:"index"` // For linking to other transactions

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// MarketItem model
type MarketItem struct {
	common.BaseModel
	Name        string `json:"name" gorm:"not null;size:100"`
	Description string `json:"description" gorm:"type:text"`
	Type        string `json:"type" gorm:"not null;size:20"` // artifact, gear, consumable, cosmetic
	Category    string `json:"category" gorm:"not null;size:50"`
	Rarity      string `json:"rarity" gorm:"size:20"` // common, rare, epic, legendary
	Level       int    `json:"level" gorm:"default:1"`

	// Pricing
	CreditsPrice int `json:"credits_price" gorm:"default:0"`
	EssencePrice int `json:"essence_price" gorm:"default:0"`

	// Availability
	IsActive   bool `json:"is_active" gorm:"default:true"`
	IsLimited  bool `json:"is_limited" gorm:"default:false"`
	Stock      int  `json:"stock" gorm:"default:-1"`        // -1 = unlimited
	MaxPerUser int  `json:"max_per_user" gorm:"default:-1"` // -1 = unlimited

	// Requirements
	TierRequired  int `json:"tier_required" gorm:"default:0"`
	LevelRequired int `json:"level_required" gorm:"default:1"`

	// Properties
	Properties common.JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Media
	ImageURL string `json:"image_url,omitempty"`
	ModelURL string `json:"model_url,omitempty"`

	// Timestamps
	AvailableFrom  *time.Time `json:"available_from,omitempty"`
	AvailableUntil *time.Time `json:"available_until,omitempty"`
}

// UserPurchase model - tracks what users have purchased
type UserPurchase struct {
	common.BaseModel
	UserID        uuid.UUID `json:"user_id" gorm:"not null;index"`
	MarketItemID  uuid.UUID `json:"market_item_id" gorm:"not null;index"`
	Quantity      int       `json:"quantity" gorm:"not null;default:1"`
	PaidCredits   int       `json:"paid_credits" gorm:"default:0"`
	PaidEssence   int       `json:"paid_essence" gorm:"default:0"`
	TransactionID uuid.UUID `json:"transaction_id" gorm:"not null;index"`

	// Relationships
	User        *User        `json:"user,omitempty" gorm:"foreignKey:UserID"`
	MarketItem  *MarketItem  `json:"market_item,omitempty" gorm:"foreignKey:MarketItemID"`
	Transaction *Transaction `json:"transaction,omitempty" gorm:"foreignKey:TransactionID"`
}

// EssencePackage model - for real money purchases
type EssencePackage struct {
	common.BaseModel
	Name          string `json:"name" gorm:"not null;size:100"`
	Description   string `json:"description" gorm:"type:text"`
	EssenceAmount int    `json:"essence_amount" gorm:"not null"`

	// Real money pricing (in cents to avoid floating point issues)
	PriceUSD int `json:"price_usd" gorm:"default:0"` // Price in cents
	PriceEUR int `json:"price_eur" gorm:"default:0"` // Price in cents
	PriceGBP int `json:"price_gbp" gorm:"default:0"` // Price in pence

	// Bonus essence for larger packages
	BonusEssence int `json:"bonus_essence" gorm:"default:0"`

	// Availability
	IsActive  bool `json:"is_active" gorm:"default:true"`
	IsPopular bool `json:"is_popular" gorm:"default:false"`
	IsLimited bool `json:"is_limited" gorm:"default:false"`

	// Timestamps
	AvailableFrom  *time.Time `json:"available_from,omitempty"`
	AvailableUntil *time.Time `json:"available_until,omitempty"`
}

// UserEssencePurchase model - tracks real money purchases
type UserEssencePurchase struct {
	common.BaseModel
	UserID           uuid.UUID `json:"user_id" gorm:"not null;index"`
	EssencePackageID uuid.UUID `json:"essence_package_id" gorm:"not null;index"`
	TransactionID    uuid.UUID `json:"transaction_id" gorm:"not null;index"`

	// Payment details
	PaymentMethod   string `json:"payment_method" gorm:"size:50"`
	PaymentCurrency string `json:"payment_currency" gorm:"size:3"`                  // USD, EUR, GBP
	PaymentAmount   int    `json:"payment_amount" gorm:"not null"`                  // Amount in cents/pence
	PaymentStatus   string `json:"payment_status" gorm:"size:20;default:'pending'"` // pending, completed, failed, refunded

	// Essence received
	EssenceReceived int `json:"essence_received" gorm:"not null"`
	BonusEssence    int `json:"bonus_essence" gorm:"default:0"`

	// External payment reference
	PaymentReference string `json:"payment_reference,omitempty"`

	// Relationships
	User           *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	EssencePackage *EssencePackage `json:"essence_package,omitempty" gorm:"foreignKey:EssencePackageID"`
	Transaction    *Transaction    `json:"transaction,omitempty" gorm:"foreignKey:TransactionID"`
}

// Helper methods for Currency
func (c *Currency) Add(amount int) {
	c.Amount += amount
	c.LastUpdated = time.Now()
}

func (c *Currency) Subtract(amount int) bool {
	if c.Amount >= amount {
		c.Amount -= amount
		c.LastUpdated = time.Now()
		return true
	}
	return false
}

func (c *Currency) HasEnough(amount int) bool {
	return c.Amount >= amount
}

// Helper methods for MarketItem
func (m *MarketItem) IsAvailable() bool {
	if !m.IsActive {
		return false
	}

	now := time.Now()

	if m.AvailableFrom != nil && now.Before(*m.AvailableFrom) {
		return false
	}

	if m.AvailableUntil != nil && now.After(*m.AvailableUntil) {
		return false
	}

	if m.IsLimited && m.Stock <= 0 {
		return false
	}

	return true
}

func (m *MarketItem) CanAffordWithCredits(userCredits int) bool {
	return userCredits >= m.CreditsPrice
}

func (m *MarketItem) CanAffordWithEssence(userEssence int) bool {
	return userEssence >= m.EssencePrice
}

// Table name methods
func (Currency) TableName() string {
	return "market.currencies"
}

func (Transaction) TableName() string {
	return "market.transactions"
}

func (MarketItem) TableName() string {
	return "market.market_items"
}

func (UserPurchase) TableName() string {
	return "market.user_purchases"
}

func (EssencePackage) TableName() string {
	return "market.essence_packages"
}

func (UserEssencePurchase) TableName() string {
	return "market.user_essence_purchases"
}
