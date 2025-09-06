package battery

import (
	"time"

	"github.com/google/uuid"
)

// BatteryType represents a type of battery available in the game
type BatteryType struct {
	ID                    uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name                  string    `json:"name" gorm:"size:50;not null"`
	DurationHours         int       `json:"duration_hours" gorm:"not null"`
	BasePriceCredits      int       `json:"base_price_credits" gorm:"not null"`
	InsuranceRatePercent  float64   `json:"insurance_rate_percent" gorm:"type:decimal(5,2);not null"`
	DurabilityLossPercent float64   `json:"durability_loss_percent" gorm:"type:decimal(5,2);not null"`
	IsActive              bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (BatteryType) TableName() string {
	return "laboratory.battery_types"
}

// BatteryInstance represents an individual battery owned by a player
type BatteryInstance struct {
	ID                       uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	UserID                   uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	BatteryTypeID            uuid.UUID  `json:"battery_type_id" gorm:"type:uuid;not null"`
	CurrentDurabilityPercent float64    `json:"current_durability_percent" gorm:"type:decimal(5,2);not null;default:100.00"`
	ChargingCount            int        `json:"charging_count" gorm:"not null;default:0"`
	IsInsured                bool       `json:"is_insured" gorm:"not null;default:false"`
	InsurancePurchasedAt     *time.Time `json:"insurance_purchased_at,omitempty"`
	InsuranceCostCredits     *int       `json:"insurance_cost_credits,omitempty"`
	PurchasePriceCredits     int        `json:"purchase_price_credits" gorm:"not null"`
	PurchasedAt              time.Time  `json:"purchased_at"`
	LastChargedAt            *time.Time `json:"last_charged_at,omitempty"`
	IsDestroyed              bool       `json:"is_destroyed" gorm:"not null;default:false"`
	DestroyedAt              *time.Time `json:"destroyed_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`

	// Relations
	BatteryType BatteryType `json:"battery_type,omitempty" gorm:"foreignKey:BatteryTypeID"`
}

// TableName specifies the table name for GORM
func (BatteryInstance) TableName() string {
	return "laboratory.battery_instances"
}

// BatteryInsuranceClaim represents an insurance claim for a destroyed battery
type BatteryInsuranceClaim struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	BatteryInstanceID  uuid.UUID  `json:"battery_instance_id" gorm:"type:uuid;not null"`
	UserID             uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	ClaimAmountCredits int        `json:"claim_amount_credits" gorm:"not null"`
	ClaimReason        string     `json:"claim_reason" gorm:"size:100;not null;default:'charging_destruction'"`
	ClaimStatus        string     `json:"claim_status" gorm:"size:20;not null;default:'pending'"`
	ProcessedAt        *time.Time `json:"processed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`

	// Relations
	BatteryInstance BatteryInstance `json:"battery_instance,omitempty" gorm:"foreignKey:BatteryInstanceID"`
}

// TableName specifies the table name for GORM
func (BatteryInsuranceClaim) TableName() string {
	return "laboratory.battery_insurance_claims"
}

// BatteryMarketPrice represents market prices for selling used batteries
type BatteryMarketPrice struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	BatteryTypeID     uuid.UUID `json:"battery_type_id" gorm:"type:uuid;not null"`
	DurabilityPercent float64   `json:"durability_percent" gorm:"type:decimal(5,2);not null"`
	SellPriceCredits  int       `json:"sell_price_credits" gorm:"not null"`
	IsActive          bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Relations
	BatteryType BatteryType `json:"battery_type,omitempty" gorm:"foreignKey:BatteryTypeID"`
}

// TableName specifies the table name for GORM
func (BatteryMarketPrice) TableName() string {
	return "laboratory.battery_market_prices"
}

// Request/Response Models

// GetBatteryTypesResponse represents the response for getting available battery types
type GetBatteryTypesResponse struct {
	BatteryTypes []BatteryTypeInfo `json:"battery_types"`
}

// BatteryTypeInfo represents battery type information for API responses
type BatteryTypeInfo struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	DurationHours         int       `json:"duration_hours"`
	BasePriceCredits      int       `json:"base_price_credits"`
	InsuranceRatePercent  float64   `json:"insurance_rate_percent"`
	DurabilityLossPercent float64   `json:"durability_loss_percent"`
	InsuranceCostCredits  int       `json:"insurance_cost_credits"`
}

// GetBatteryInstancesResponse represents the response for getting player's batteries
type GetBatteryInstancesResponse struct {
	Batteries []BatteryInstanceInfo `json:"batteries"`
}

// BatteryInstanceInfo represents battery instance information for API responses
type BatteryInstanceInfo struct {
	ID                       uuid.UUID       `json:"id"`
	BatteryType              BatteryTypeInfo `json:"battery_type"`
	CurrentDurabilityPercent float64         `json:"current_durability_percent"`
	ChargingCount            int             `json:"charging_count"`
	IsInsured                bool            `json:"is_insured"`
	InsuranceCostCredits     *int            `json:"insurance_cost_credits,omitempty"`
	PurchasePriceCredits     int             `json:"purchase_price_credits"`
	PurchasedAt              time.Time       `json:"purchased_at"`
	LastChargedAt            *time.Time      `json:"last_charged_at,omitempty"`
	IsDestroyed              bool            `json:"is_destroyed"`
	DestroyedAt              *time.Time      `json:"destroyed_at,omitempty"`
	CanInsure                bool            `json:"can_insure"`
	ChargingRiskPercent      float64         `json:"charging_risk_percent"`
	SellPriceCredits         int             `json:"sell_price_credits"`
}

// PurchaseBatteryRequest represents the request to purchase a battery
type PurchaseBatteryRequest struct {
	BatteryTypeID uuid.UUID `json:"battery_type_id" binding:"required"`
}

// PurchaseBatteryResponse represents the response for purchasing a battery
type PurchaseBatteryResponse struct {
	Success         bool                `json:"success"`
	BatteryInstance BatteryInstanceInfo `json:"battery_instance"`
	Message         string              `json:"message"`
}

// PurchaseInsuranceRequest represents the request to purchase insurance
type PurchaseInsuranceRequest struct {
	BatteryInstanceID uuid.UUID `json:"battery_instance_id" binding:"required"`
}

// PurchaseInsuranceResponse represents the response for purchasing insurance
type PurchaseInsuranceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SellBatteryRequest represents the request to sell a battery
type SellBatteryRequest struct {
	BatteryInstanceID uuid.UUID `json:"battery_instance_id" binding:"required"`
}

// SellBatteryResponse represents the response for selling a battery
type SellBatteryResponse struct {
	Success          bool   `json:"success"`
	SellPriceCredits int    `json:"sell_price_credits"`
	Message          string `json:"message"`
}

// GetRiskAssessmentResponse represents the response for risk assessment
type GetRiskAssessmentResponse struct {
	BatteryInstanceID        uuid.UUID `json:"battery_instance_id"`
	CurrentDurabilityPercent float64   `json:"current_durability_percent"`
	ChargingCount            int       `json:"charging_count"`
	ChargingRiskPercent      float64   `json:"charging_risk_percent"`
	IsGuaranteed             bool      `json:"is_guaranteed"`
	GuaranteeChargesLeft     int       `json:"guarantee_charges_left"`
	CanInsure                bool      `json:"can_insure"`
	InsuranceCostCredits     int       `json:"insurance_cost_credits"`
}

// GetInsuranceClaimsResponse represents the response for getting insurance claims
type GetInsuranceClaimsResponse struct {
	Claims []InsuranceClaimInfo `json:"claims"`
}

// InsuranceClaimInfo represents insurance claim information for API responses
type InsuranceClaimInfo struct {
	ID                 uuid.UUID  `json:"id"`
	BatteryInstanceID  uuid.UUID  `json:"battery_instance_id"`
	ClaimAmountCredits int        `json:"claim_amount_credits"`
	ClaimReason        string     `json:"claim_reason"`
	ClaimStatus        string     `json:"claim_status"`
	ProcessedAt        *time.Time `json:"processed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ChargingRiskInfo represents charging risk information
type ChargingRiskInfo struct {
	BatteryInstanceID        uuid.UUID `json:"battery_instance_id"`
	CurrentDurabilityPercent float64   `json:"current_durability_percent"`
	ChargingCount            int       `json:"charging_count"`
	ChargingRiskPercent      float64   `json:"charging_risk_percent"`
	IsGuaranteed             bool      `json:"is_guaranteed"`
	GuaranteeChargesLeft     int       `json:"guarantee_charges_left"`
	IsInsured                bool      `json:"is_insured"`
	CanInsure                bool      `json:"can_insure"`
	InsuranceCostCredits     int       `json:"insurance_cost_credits"`
}

// BatteryStats represents battery statistics for a player
type BatteryStats struct {
	TotalBatteries     int     `json:"total_batteries"`
	ActiveBatteries    int     `json:"active_batteries"`
	DestroyedBatteries int     `json:"destroyed_batteries"`
	InsuredBatteries   int     `json:"insured_batteries"`
	TotalChargingCount int     `json:"total_charging_count"`
	AverageDurability  float64 `json:"average_durability"`
	TotalInsuranceCost int     `json:"total_insurance_cost"`
	TotalClaimAmount   int     `json:"total_claim_amount"`
}

// GetBatteryStatsResponse represents the response for getting battery statistics
type GetBatteryStatsResponse struct {
	Stats BatteryStats `json:"stats"`
}

// (duplicitné TableName() bez schémy odstránené)

// Constants for claim status
const (
	ClaimStatusPending  = "pending"
	ClaimStatusApproved = "approved"
	ClaimStatusRejected = "rejected"
	ClaimStatusPaid     = "paid"
)

// Constants for claim reasons
const (
	ClaimReasonChargingDestruction = "charging_destruction"
	ClaimReasonAccident            = "accident"
	ClaimReasonDefect              = "defect"
)

// Constants for battery system
const (
	BaseChargingRiskPercent   = 3.0  // 3% base risk
	GuaranteeCharges          = 2    // First 2 charges are guaranteed safe
	MaxChargingRiskPercent    = 50.0 // Maximum risk cap
	InsuranceThresholdPercent = 50.0 // Can only insure above 50% durability
	SellPricePercent          = 20.0 // Used batteries sell for 20% of purchase price
)
