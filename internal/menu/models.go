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

// Purchase states
const (
	PurchaseStatePending   = "pending"
	PurchaseStateCompleted = "completed"
	PurchaseStateFailed    = "failed"
	PurchaseStateRefunded  = "refunded"
)

// Order states
const (
	OrderStatePlaced           = "PLACED"
	OrderStateScheduled        = "SCHEDULED"
	OrderStateReadyForPickup   = "READY_FOR_PICKUP"
	OrderStateCompleted        = "COMPLETED"
	OrderStateCancelledRefund  = "CANCELLED_REFUND"
	OrderStateCancelledForfeit = "CANCELLED_FORFEIT"
)

// Stock ledger reasons
const (
	StockReasonRestock = "restock"
	StockReasonReserve = "reserve"
	StockReasonRelease = "release"
	StockReasonSale    = "sale"
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

	// Phase 1: New fields for MVP
	DailyLimit         *int       `json:"daily_limit,omitempty"`           // Daily purchase limit per user
	WeeklyLimit        *int       `json:"weekly_limit,omitempty"`          // Weekly purchase limit per user
	ScannerCatalogID   *uuid.UUID `json:"scanner_catalog_id,omitempty"`    // FK to gameplay.scanner_catalog
	PowerCellCatalogID *uuid.UUID `json:"power_cell_catalog_id,omitempty"` // FK to analytics.power_cells_catalog

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

	// Phase 1: New fields for MVP
	IdempotencyKey *uuid.UUID `json:"idempotency_key,omitempty"`                         // For safe retry of purchases
	State          string     `json:"state" gorm:"not null;default:'completed';size:20"` // pending, completed, failed, refunded

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

// Phase 1: New helper methods for MVP
func (m *MarketItem) HasDailyLimit() bool {
	return m.DailyLimit != nil && *m.DailyLimit > 0
}

func (m *MarketItem) HasWeeklyLimit() bool {
	return m.WeeklyLimit != nil && *m.WeeklyLimit > 0
}

func (m *MarketItem) IsScannerItem() bool {
	return m.ScannerCatalogID != nil
}

func (m *MarketItem) IsPowerCellItem() bool {
	return m.PowerCellCatalogID != nil
}

// Helper methods for UserPurchase
func (p *UserPurchase) IsCompleted() bool {
	return p.State == PurchaseStateCompleted
}

func (p *UserPurchase) IsPending() bool {
	return p.State == PurchaseStatePending
}

func (p *UserPurchase) IsFailed() bool {
	return p.State == PurchaseStateFailed
}

func (p *UserPurchase) IsRefunded() bool {
	return p.State == PurchaseStateRefunded
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

// =====================================================
// PHASE 2 - ORDER SYSTEM MODELS
// =====================================================

// Order model - objednávky s back-order/pre-order
type Order struct {
	common.BaseModel
	UserID       uuid.UUID `json:"user_id" gorm:"not null;index"`
	MarketItemID uuid.UUID `json:"market_item_id" gorm:"not null;index"`
	Quantity     int       `json:"quantity" gorm:"not null;check:quantity > 0"`

	// Záloha a ceny
	DepositPct           int `json:"deposit_pct" gorm:"not null;check:deposit_pct >= 10 AND deposit_pct <= 90"`
	DepositAmountCredits int `json:"deposit_amount_credits" gorm:"not null;default:0"`
	DepositAmountEssence int `json:"deposit_amount_essence" gorm:"not null;default:0"`
	ExpediteEssence      int `json:"expedite_essence" gorm:"not null;default:0;check:expedite_essence >= 0"`

	// Časové informácie
	ETAAt           *time.Time `json:"eta_at" gorm:"not null"`
	PickupExpiresAt *time.Time `json:"pickup_expires_at"`

	// Zamknuté ceny (v čase objednávky)
	PriceLockedCredits int `json:"price_locked_credits" gorm:"not null;check:price_locked_credits >= 0"`
	PriceLockedEssence int `json:"price_locked_essence" gorm:"not null;default:0;check:price_locked_essence >= 0"`

	// Stav a idempotencia
	State          string     `json:"state" gorm:"not null;default:'PLACED'"`
	IdempotencyKey *uuid.UUID `json:"idempotency_key"`

	// Relations
	User       User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	MarketItem MarketItem `json:"market_item,omitempty" gorm:"foreignKey:MarketItemID"`
}

// StockLedger model - sledovanie skladu a rezervácií
type StockLedger struct {
	common.BaseModel
	MarketItemID uuid.UUID  `json:"market_item_id" gorm:"not null;index"`
	Delta        int        `json:"delta" gorm:"not null;check:delta != 0"`
	Reason       string     `json:"reason" gorm:"not null;check:reason IN ('restock','reserve','release','sale')"`
	RefID        *uuid.UUID `json:"ref_id" gorm:"index"`

	// Relations
	MarketItem MarketItem `json:"market_item,omitempty" gorm:"foreignKey:MarketItemID"`
}

// MarketSettings model - konfigurovateľné policy
type MarketSettings struct {
	common.BaseModel
	Key         string       `json:"key" gorm:"not null;uniqueIndex"`
	Value       common.JSONB `json:"value" gorm:"type:jsonb;not null"`
	Description string       `json:"description"`
}

// Order helper methods
func (o *Order) IsPlaced() bool {
	return o.State == OrderStatePlaced
}

func (o *Order) IsScheduled() bool {
	return o.State == OrderStateScheduled
}

func (o *Order) IsReadyForPickup() bool {
	return o.State == OrderStateReadyForPickup
}

func (o *Order) IsCompleted() bool {
	return o.State == OrderStateCompleted
}

func (o *Order) IsCancelled() bool {
	return o.State == OrderStateCancelledRefund || o.State == OrderStateCancelledForfeit
}

func (o *Order) IsRefunded() bool {
	return o.State == OrderStateCancelledRefund
}

func (o *Order) IsForfeited() bool {
	return o.State == OrderStateCancelledForfeit
}

func (o *Order) CanBeCancelled() bool {
	return o.State == OrderStatePlaced || o.State == OrderStateScheduled
}

func (o *Order) CanBeCompleted() bool {
	return o.State == OrderStateReadyForPickup
}

func (o *Order) CanBeExpedited() bool {
	return o.State == OrderStatePlaced || o.State == OrderStateScheduled
}

func (o *Order) GetRemainingPrice() (int, int) {
	// Vypočítaj zvyšok ceny po zálohách
	remainingCredits := (o.PriceLockedCredits * o.Quantity) - o.DepositAmountCredits
	remainingEssence := (o.PriceLockedEssence * o.Quantity) - o.DepositAmountEssence

	if remainingCredits < 0 {
		remainingCredits = 0
	}
	if remainingEssence < 0 {
		remainingEssence = 0
	}

	return remainingCredits, remainingEssence
}

// StockLedger helper methods
func (sl *StockLedger) IsRestock() bool {
	return sl.Reason == StockReasonRestock
}

func (sl *StockLedger) IsReserve() bool {
	return sl.Reason == StockReasonReserve
}

func (sl *StockLedger) IsRelease() bool {
	return sl.Reason == StockReasonRelease
}

func (sl *StockLedger) IsSale() bool {
	return sl.Reason == StockReasonSale
}

// Table name methods for Phase 2 models
func (Order) TableName() string {
	return "market.orders"
}

func (StockLedger) TableName() string {
	return "market.stock_ledger"
}

func (MarketSettings) TableName() string {
	return "market.settings"
}
