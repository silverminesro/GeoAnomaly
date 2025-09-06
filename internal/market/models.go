package market

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BaseModel - spoločný základný model
type BaseModel struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// JSONB type for PostgreSQL
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}

	bytes, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}

	return string(bytes), nil
}

// MarketItem model - migrovaný do market.market_items
type MarketItem struct {
	BaseModel
	SellerID    uuid.UUID  `json:"seller_id" gorm:"not null;index"`
	ItemType    string     `json:"item_type" gorm:"not null;size:50"` // artifact, gear, battery
	ItemID      uuid.UUID  `json:"item_id" gorm:"not null"`
	Price       int        `json:"price" gorm:"not null"` // v centoch
	Currency    string     `json:"currency" gorm:"not null;size:10;default:'USD'"`
	Quantity    int        `json:"quantity" gorm:"default:1"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Properties  JSONB      `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
	Description string     `json:"description,omitempty" gorm:"type:text"`

	// Relationships
	Seller *User `json:"seller,omitempty" gorm:"foreignKey:SellerID"`
}

// Transaction model - migrovaný do market.transactions
type Transaction struct {
	BaseModel
	BuyerID      uuid.UUID  `json:"buyer_id" gorm:"not null;index"`
	SellerID     uuid.UUID  `json:"seller_id" gorm:"not null;index"`
	MarketItemID uuid.UUID  `json:"market_item_id" gorm:"not null;index"`
	Amount       int        `json:"amount" gorm:"not null"` // v centoch
	Currency     string     `json:"currency" gorm:"not null;size:10"`
	Quantity     int        `json:"quantity" gorm:"default:1"`
	Status       string     `json:"status" gorm:"not null;size:20;default:'pending'"` // pending, completed, cancelled
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Properties   JSONB      `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	Buyer      *User       `json:"buyer,omitempty" gorm:"foreignKey:BuyerID"`
	Seller     *User       `json:"seller,omitempty" gorm:"foreignKey:SellerID"`
	MarketItem *MarketItem `json:"market_item,omitempty" gorm:"foreignKey:MarketItemID"`
}

// UserPurchase model - migrovaný do market.user_purchases
type UserPurchase struct {
	BaseModel
	UserID       uuid.UUID  `json:"user_id" gorm:"not null;index"`
	PurchaseType string     `json:"purchase_type" gorm:"not null;size:50"` // tier_upgrade, currency, item
	ItemID       *uuid.UUID `json:"item_id,omitempty"`
	Amount       int        `json:"amount" gorm:"not null"` // v centoch
	Currency     string     `json:"currency" gorm:"not null;size:10"`
	Status       string     `json:"status" gorm:"not null;size:20;default:'pending'"` // pending, completed, refunded
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Properties   JSONB      `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// UserTierPurchase model - migrovaný do market.user_tier_purchases
type UserTierPurchase struct {
	BaseModel
	UserID     uuid.UUID  `json:"user_id" gorm:"not null;index"`
	Tier       int        `json:"tier" gorm:"not null"`
	Duration   int        `json:"duration" gorm:"not null"` // v dňoch
	Amount     int        `json:"amount" gorm:"not null"`   // v centoch
	Currency   string     `json:"currency" gorm:"not null;size:10"`
	Status     string     `json:"status" gorm:"not null;size:20;default:'pending'"` // pending, active, expired, cancelled
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	AutoRenew  bool       `json:"auto_renew" gorm:"default:false"`
	Properties JSONB      `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// Currency model - migrovaný do market.currencies
type Currency struct {
	ID           string  `json:"id" gorm:"primaryKey;size:10"`
	Name         string  `json:"name" gorm:"not null;size:50"`
	Symbol       string  `json:"symbol" gorm:"not null;size:10"`
	ExchangeRate float64 `json:"exchange_rate" gorm:"not null;default:1.0"` // relatívne k USD
	IsActive     bool    `json:"is_active" gorm:"default:true"`
	Properties   JSONB   `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
}

// EssencePackage model - migrovaný do market.essence_packages
type EssencePackage struct {
	BaseModel
	Name          string `json:"name" gorm:"not null;size:100"`
	Description   string `json:"description,omitempty" gorm:"type:text"`
	EssenceAmount int    `json:"essence_amount" gorm:"not null"`
	Price         int    `json:"price" gorm:"not null"` // v centoch
	Currency      string `json:"currency" gorm:"not null;size:10;default:'USD'"`
	IsActive      bool   `json:"is_active" gorm:"default:true"`
	Properties    JSONB  `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`
}

// TableName methods for GORM schema qualification
func (MarketItem) TableName() string {
	return "market.market_items"
}

func (Transaction) TableName() string {
	return "market.transactions"
}

func (UserPurchase) TableName() string {
	return "market.user_purchases"
}

func (UserTierPurchase) TableName() string {
	return "market.user_tier_purchases"
}

func (Currency) TableName() string {
	return "market.currencies"
}

func (EssencePackage) TableName() string {
	return "market.essence_packages"
}

// User model reference (from auth module)
type User struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Tier      int       `json:"tier"`
	XP        int       `json:"xp"`
	Level     int       `json:"level"`
	IsActive  bool      `json:"is_active"`
	IsBanned  bool      `json:"is_banned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "auth.users"
}
