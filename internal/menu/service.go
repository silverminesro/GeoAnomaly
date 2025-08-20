package menu

import (
	"errors"
	"fmt"
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrItemNotAvailable  = errors.New("item not available")
	ErrItemNotFound      = errors.New("item not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInvalidCurrency   = errors.New("invalid currency type")
	ErrPackageNotFound   = errors.New("essence package not found")
	ErrPaymentFailed     = errors.New("payment failed")
	ErrItemEquipped      = errors.New("cannot sell equipped item - unequip it first")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Currency management
func (s *Service) GetUserCurrency(userID uuid.UUID, currencyType string) (*Currency, error) {
	var currency Currency
	err := s.db.Where("user_id = ? AND type = ?", userID, currencyType).First(&currency).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create default currency if not exists
			currency = Currency{
				UserID: userID,
				Type:   currencyType,
				Amount: 0,
			}
			if currencyType == CurrencyCredits {
				currency.Amount = 5000 // Default credits for new users
			}
			err = s.db.Create(&currency).Error
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return &currency, nil
}

func (s *Service) AddCurrency(userID uuid.UUID, currencyType string, amount int, description string) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	currency, err := s.GetUserCurrency(userID, currencyType)
	if err != nil {
		return err
	}

	balanceBefore := currency.Amount
	currency.Add(amount)

	// Create transaction record
	transaction := Transaction{
		UserID:        userID,
		Type:          TransactionTypeReward,
		CurrencyType:  currencyType,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  currency.Amount,
		Description:   description,
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(currency).Error; err != nil {
			return err
		}
		return tx.Create(&transaction).Error
	})
}

func (s *Service) SubtractCurrency(userID uuid.UUID, currencyType string, amount int, description string) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	currency, err := s.GetUserCurrency(userID, currencyType)
	if err != nil {
		return err
	}

	if !currency.HasEnough(amount) {
		return ErrInsufficientFunds
	}

	balanceBefore := currency.Amount
	currency.Subtract(amount)

	// Create transaction record
	transaction := Transaction{
		UserID:        userID,
		Type:          TransactionTypePurchase,
		CurrencyType:  currencyType,
		Amount:        -amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  currency.Amount,
		Description:   description,
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(currency).Error; err != nil {
			return err
		}
		return tx.Create(&transaction).Error
	})
}

// Market management
func (s *Service) GetMarketItems(userID uuid.UUID, category string, rarity string) ([]MarketItem, error) {
	var items []MarketItem
	query := s.db.Where("is_active = ?", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if rarity != "" {
		query = query.Where("rarity = ?", rarity)
	}

	err := query.Find(&items).Error
	if err != nil {
		return nil, err
	}

	// Filter by user requirements
	var filteredItems []MarketItem
	user, err := s.getUser(userID)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.IsAvailable() && s.canUserAccessItem(user, &item) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

func (s *Service) PurchaseMarketItem(userID uuid.UUID, itemID uuid.UUID, quantity int, currencyType string) error {
	if quantity <= 0 {
		return ErrInvalidAmount
	}

	var item MarketItem
	err := s.db.First(&item, itemID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrItemNotFound
		}
		return err
	}

	if !item.IsAvailable() {
		return ErrItemNotAvailable
	}

	user, err := s.getUser(userID)
	if err != nil {
		return err
	}

	if !s.canUserAccessItem(user, &item) {
		return errors.New("user does not meet requirements")
	}

	// Check if user can afford the item
	var price int
	if currencyType == CurrencyCredits {
		price = item.CreditsPrice * quantity
	} else if currencyType == CurrencyEssence {
		price = item.EssencePrice * quantity
	} else {
		return ErrInvalidCurrency
	}

	currency, err := s.GetUserCurrency(userID, currencyType)
	if err != nil {
		return err
	}

	if !currency.HasEnough(price) {
		return ErrInsufficientFunds
	}

	// Check purchase limits
	if item.MaxPerUser > 0 {
		var purchaseCount int64
		err = s.db.Model(&UserPurchase{}).Where("user_id = ? AND market_item_id = ?", userID, itemID).Count(&purchaseCount).Error
		if err != nil {
			return err
		}
		if int(purchaseCount)+quantity > item.MaxPerUser {
			return errors.New("purchase limit exceeded")
		}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Subtract currency
		balanceBefore := currency.Amount
		currency.Subtract(price)

		// Create transaction
		transaction := Transaction{
			UserID:        userID,
			Type:          TransactionTypePurchase,
			CurrencyType:  currencyType,
			Amount:        -price,
			BalanceBefore: balanceBefore,
			BalanceAfter:  currency.Amount,
			Description:   fmt.Sprintf("Purchased %d x %s", quantity, item.Name),
			ItemID:        &itemID,
			ItemType:      item.Type,
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		// Create purchase record
		purchase := UserPurchase{
			UserID:        userID,
			MarketItemID:  itemID,
			Quantity:      quantity,
			TransactionID: transaction.ID,
		}

		if currencyType == CurrencyCredits {
			purchase.PaidCredits = price
		} else {
			purchase.PaidEssence = price
		}

		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}

		// ✅ PRIDANÉ: Pridaj predmet do inventára
		inventoryItem := common.InventoryItem{
			UserID:   userID,
			ItemType: item.Type,
			ItemID:   item.ID,
			Quantity: quantity,
			Properties: common.JSONB{
				"name":           item.Name,
				"type":           item.Type,
				"category":       item.Category,
				"rarity":         item.Rarity,
				"level":          item.Level,
				"purchased_at":   time.Now().Unix(),
				"purchased_from": "market",
				"market_item_id": itemID.String(),
			},
		}

		if err := tx.Create(&inventoryItem).Error; err != nil {
			return err
		}

		// Update currency
		if err := tx.Save(currency).Error; err != nil {
			return err
		}

		// Update stock if limited
		if item.IsLimited {
			if err := tx.Model(&item).Update("stock", item.Stock-quantity).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// Essence package management
func (s *Service) GetEssencePackages() ([]EssencePackage, error) {
	var packages []EssencePackage
	err := s.db.Where("is_active = ?", true).Find(&packages).Error
	return packages, err
}

func (s *Service) PurchaseEssencePackage(userID uuid.UUID, packageID uuid.UUID, paymentMethod string, paymentCurrency string, paymentAmount int) error {
	var pkg EssencePackage
	err := s.db.First(&pkg, packageID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPackageNotFound
		}
		return err
	}

	if !pkg.IsActive {
		return ErrItemNotAvailable
	}

	// Validate payment amount
	var expectedAmount int
	switch paymentCurrency {
	case "USD":
		expectedAmount = pkg.PriceUSD
	case "EUR":
		expectedAmount = pkg.PriceEUR
	case "GBP":
		expectedAmount = pkg.PriceGBP
	default:
		return errors.New("unsupported payment currency")
	}

	if paymentAmount != expectedAmount {
		return errors.New("invalid payment amount")
	}

	// Calculate essence to receive
	totalEssence := pkg.EssenceAmount + pkg.BonusEssence

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Add essence to user
		if err := s.AddCurrency(userID, CurrencyEssence, totalEssence, fmt.Sprintf("Essence package: %s", pkg.Name)); err != nil {
			return err
		}

		// Create essence purchase record
		purchase := UserEssencePurchase{
			UserID:           userID,
			EssencePackageID: packageID,
			PaymentMethod:    paymentMethod,
			PaymentCurrency:  paymentCurrency,
			PaymentAmount:    paymentAmount,
			PaymentStatus:    "completed",
			EssenceReceived:  pkg.EssenceAmount,
			BonusEssence:     pkg.BonusEssence,
		}

		// Get the transaction that was created by AddCurrency
		var transaction Transaction
		err := tx.Where("user_id = ? AND type = ? AND description LIKE ?", userID, TransactionTypeReward, fmt.Sprintf("%%%s%%", pkg.Name)).Order("created_at DESC").First(&transaction).Error
		if err != nil {
			return err
		}

		purchase.TransactionID = transaction.ID

		return tx.Create(&purchase).Error
	})
}

// Item selling (converting inventory items to credits)
func (s *Service) SellInventoryItem(userID uuid.UUID, inventoryItemID uuid.UUID) error {
	var inventoryItem common.InventoryItem
	err := s.db.First(&inventoryItem, inventoryItemID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrItemNotFound
		}
		return err
	}

	if inventoryItem.UserID != userID {
		return errors.New("item does not belong to user")
	}

	// ✅ OPRAVENÉ: Skontroluj či je item vybavený v loadoute
	var loadoutItem common.LoadoutItem
	err = s.db.Where("user_id = ? AND item_id = ?", userID, inventoryItem.ID).First(&loadoutItem).Error
	if err == nil {
		// Item je vybavený v loadoute
		return ErrItemEquipped
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Iná chyba
		return err
	}

	// Calculate sell price based on item type and rarity
	sellPrice := s.calculateSellPrice(&inventoryItem)

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Add credits to user
		if err := s.AddCurrency(userID, CurrencyCredits, sellPrice, fmt.Sprintf("Sold %s", inventoryItem.ItemType)); err != nil {
			return err
		}

		// Delete inventory item
		if err := tx.Delete(&inventoryItem).Error; err != nil {
			return err
		}

		return nil
	})
}

// Helper methods
func (s *Service) getUser(userID uuid.UUID) (*common.User, error) {
	var user common.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) canUserAccessItem(user *common.User, item *MarketItem) bool {
	return user.Tier >= item.TierRequired && user.Level >= item.LevelRequired
}

func (s *Service) calculateSellPrice(item *common.InventoryItem) int {
	// Base prices for different item types
	basePrices := map[string]int{
		"artifact": 100,
		"gear":     150,
	}

	basePrice := basePrices[item.ItemType]
	if basePrice == 0 {
		basePrice = 50 // Default price
	}

	// Apply rarity multiplier
	rarityMultipliers := map[string]float64{
		"common":    1.0,
		"rare":      2.0,
		"epic":      5.0,
		"legendary": 10.0,
	}

	rarity, ok := item.Properties["rarity"].(string)
	if !ok {
		rarity = "common"
	}
	multiplier := rarityMultipliers[rarity]
	if multiplier == 0 {
		multiplier = 1.0
	}

	return int(float64(basePrice) * multiplier)
}

// Get user transaction history
func (s *Service) GetUserTransactions(userID uuid.UUID, limit int) ([]Transaction, error) {
	var transactions []Transaction
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&transactions).Error
	return transactions, err
}

// Get user purchase history
func (s *Service) GetUserPurchases(userID uuid.UUID, limit int) ([]UserPurchase, error) {
	var purchases []UserPurchase
	err := s.db.Preload("MarketItem").Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&purchases).Error
	return purchases, err
}

// Get user essence purchase history
func (s *Service) GetUserEssencePurchases(userID uuid.UUID, limit int) ([]UserEssencePurchase, error) {
	var purchases []UserEssencePurchase
	err := s.db.Preload("EssencePackage").Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&purchases).Error
	return purchases, err
}

// Tier purchase model (zjednodušený)
type UserTierPurchase struct {
	ID              uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	DeletedAt       *time.Time   `json:"deleted_at,omitempty" gorm:"index"`
	UserID          uuid.UUID    `json:"user_id" gorm:"not null"`
	TierLevel       int          `json:"tier_level" gorm:"not null"`
	DurationMonths  int          `json:"duration_months" gorm:"not null"`
	ExpiresAt       time.Time    `json:"expires_at" gorm:"not null"` // Pridané
	PaymentMethod   string       `json:"payment_method" gorm:"not null"`
	PaymentCurrency string       `json:"payment_currency" gorm:"not null"`
	PaymentAmount   int          `json:"payment_amount" gorm:"not null"` // v centoch
	PaymentStatus   string       `json:"payment_status" gorm:"not null;default:'pending'"`
	TransactionID   uuid.UUID    `json:"transaction_id" gorm:"not null"`
	Properties      common.JSONB `json:"properties,omitempty" gorm:"type:jsonb;default:'{}'::jsonb"`

	// Relationships
	User        *common.User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Transaction *Transaction `json:"transaction,omitempty" gorm:"foreignKey:TransactionID"`
}

// Purchase tier package (zjednodušený)
func (s *Service) PurchaseTierPackage(userID uuid.UUID, tierLevel int, durationMonths int, paymentMethod string, paymentCurrency string) error {
	// Get tier definition
	var tierDef struct {
		TierLevel    int     `json:"tier_level"`
		TierName     string  `json:"tier_name"`
		PriceMonthly float64 `json:"price_monthly"` // Zmenené späť na float64 pre decimal
	}

	err := s.db.Table("tier_definitions").
		Where("tier_level = ?", tierLevel).
		First(&tierDef).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("tier not found")
		}
		return err
	}

	// Calculate total price (price_monthly je v centoch, konvertujeme na int)
	totalPrice := int(tierDef.PriceMonthly * float64(durationMonths))

	// Calculate expiration date
	expiresAt := time.Now().AddDate(0, durationMonths, 0)

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Create transaction record
		transaction := Transaction{
			UserID:        userID,
			Type:          TransactionTypePurchase,
			CurrencyType:  paymentCurrency,
			Amount:        -totalPrice, // Negative because it's a purchase
			BalanceBefore: 0,
			BalanceAfter:  0,
			Description:   fmt.Sprintf("Tier upgrade: %s for %d months", tierDef.TierName, durationMonths),
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		// Create tier purchase record
		purchase := UserTierPurchase{
			UserID:          userID,
			TierLevel:       tierLevel,
			DurationMonths:  durationMonths,
			ExpiresAt:       expiresAt, // Pridané
			PaymentMethod:   paymentMethod,
			PaymentCurrency: paymentCurrency,
			PaymentAmount:   totalPrice,
			PaymentStatus:   "completed",
			TransactionID:   transaction.ID,
		}

		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}

		// Update user tier and expiration (používa existujúce polia!)
		if err := tx.Model(&common.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"tier":         tierLevel,
				"tier_expires": expiresAt,
			}).Error; err != nil {
			return err
		}

		return nil
	})
}

// Get user tier purchase history
func (s *Service) GetUserTierPurchases(userID uuid.UUID, limit int) ([]UserTierPurchase, error) {
	var purchases []UserTierPurchase
	err := s.db.Preload("Transaction").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&purchases).Error
	return purchases, err
}

// Check and reset expired tier for a user
func (s *Service) CheckAndResetExpiredTier(userID uuid.UUID) error {
	var user common.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return err
	}

	// Ak má tier a je expirovaný
	if user.Tier > 0 && user.TierExpires != nil && user.TierExpires.Before(time.Now()) {
		// Reset na tier 0
		return s.db.Model(&common.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"tier":         0,
				"tier_expires": nil,
			}).Error
	}

	return nil
}

// Batch check and reset all expired tiers (pre admin endpoint)
func (s *Service) CheckAndResetAllExpiredTiers() (int, error) {
	var count int64

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Nájdeme všetkých userov s expirovaným tier
		var users []common.User
		err := tx.Where("tier > 0 AND tier_expires < ?", time.Now()).Find(&users).Error
		if err != nil {
			return err
		}

		count = int64(len(users))

		// Reset na tier 0
		if count > 0 {
			if err := tx.Model(&common.User{}).
				Where("tier > 0 AND tier_expires < ?", time.Now()).
				Updates(map[string]interface{}{
					"tier":         0,
					"tier_expires": nil,
				}).Error; err != nil {
				return err
			}
		}

		return nil
	})

	return int(count), err
}
