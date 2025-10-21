package menu

import (
	"errors"
	"fmt"
	"log"
	"time"

	"geoanomaly/internal/common"
	"geoanomaly/internal/gameplay"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrItemNotAvailable  = errors.New("item not available")
	ErrItemNotFound      = errors.New("item not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrAlreadyReserved   = errors.New("item already reserved")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInvalidCurrency   = errors.New("invalid currency type")
	ErrPackageNotFound   = errors.New("essence package not found")
	ErrPaymentFailed     = errors.New("payment failed")
	ErrItemEquipped      = errors.New("cannot sell equipped item - unequip it first")
	ErrPurchaseLimit     = errors.New("purchase limit exceeded")
	ErrOutOfStock        = errors.New("not enough stock")
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
				Amount: 0, // žiadne implicitné kredity
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
func (s *Service) GetMarketItems(userID uuid.UUID, category string, rarity string, includeLocked bool) ([]MarketItem, error) {
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
		// If includeLocked is true, show all items with lock status
		if includeLocked {
			// Add lock information to the item
			item = s.addLockInformation(user, &item)
			filteredItems = append(filteredItems, item)
		} else {
			// Original behavior - only show accessible items
			if item.IsAvailable() && s.canUserAccessItem(user, &item) {
				filteredItems = append(filteredItems, item)
			}
		}
	}

	return filteredItems, nil
}

// mapFlutterTypeToBackendType mapuje Flutter typy na backend typy a kategórie
func mapFlutterTypeToBackendType(flutterType string) (string, string) {
	switch flutterType {
	case "deployable_scanner":
		return "gear", "deployable_scanners"
	case "scanner_battery":
		return "consumable", "scanner_batteries"
	default:
		return flutterType, ""
	}
}

// Idempotentný nákup s vynútením limitov a bezpečným stock decrementom
func (s *Service) PurchaseMarketItemIdempotent(userID uuid.UUID, itemID uuid.UUID, quantity int, currencyType string, idempotencyKey *uuid.UUID) (*UserPurchase, error) {
	if quantity <= 0 {
		return nil, ErrInvalidAmount
	}

	// B1) Idempotencia – používaj iba ak idempotency_key != nil
	if idempotencyKey != nil {
		// Rýchla cesta: existujúci záznam?
		var existing UserPurchase
		err := s.db.Where("user_id = ? AND idempotency_key = ?", userID, *idempotencyKey).
			First(&existing).Error
		if err == nil {
			// Vráť existujúci výsledok
			return &existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	var result *UserPurchase
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Načítaj položku - najprv skús priamo podľa ID
		var item MarketItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}). // lock proti súbehu stocku
										Where("id = ? AND is_active = ?", itemID, true).
										First(&item).Error; err != nil {

			// Ak sa nenašiel podľa ID, skús nájsť podľa Flutter type
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Skús nájsť item podľa Flutter type (deployable_scanner, scanner_battery)
				// Najprv skús nájsť podľa type = 'gear' a category = 'deployable_scanners'
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("type = ? AND category = ? AND is_active = ?", "gear", "deployable_scanners", true).
					First(&item).Error; err != nil {

					// Skús nájsť podľa type = 'consumable' a category = 'scanner_batteries'
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("type = ? AND category = ? AND is_active = ?", "consumable", "scanner_batteries", true).
						First(&item).Error; err != nil {
						return ErrItemNotFound
					}
				}
			} else {
				return err
			}
		}
		if !item.IsAvailable() { // existujúca metóda
			return ErrItemNotAvailable
		}

		// Načítaj usera a skontroluj prístup (tier/level)
		user, err := s.getUser(userID)
		if err != nil {
			return err
		}
		if !s.canUserAccessItem(user, &item) {
			return errors.New("user does not meet requirements")
		}

		// 2) Limity (počítaj jednotky/quantity)
		now := time.Now()
		var usedDaily, usedWeekly int64
		if item.DailyLimit != nil {
			if err := tx.Model(&UserPurchase{}).
				Where("user_id = ? AND market_item_id = ? AND state = ? AND created_at >= ?",
					userID, itemID, PurchaseStateCompleted, now.Add(-24*time.Hour)).
				Select("COALESCE(SUM(quantity),0)").
				Scan(&usedDaily).Error; err != nil {
				return err
			}
			if int(usedDaily)+quantity > *item.DailyLimit {
				return ErrPurchaseLimit
			}
		}
		if item.WeeklyLimit != nil {
			if err := tx.Model(&UserPurchase{}).
				Where("user_id = ? AND market_item_id = ? AND state = ? AND created_at >= ?",
					userID, itemID, PurchaseStateCompleted, now.Add(-7*24*time.Hour)).
				Select("COALESCE(SUM(quantity),0)").
				Scan(&usedWeekly).Error; err != nil {
				return err
			}
			if int(usedWeekly)+quantity > *item.WeeklyLimit {
				return ErrPurchaseLimit
			}
		}
		if item.MaxPerUser > 0 {
			var lifetimeUnits int64
			if err := tx.Model(&UserPurchase{}).
				Where("user_id = ? AND market_item_id = ? AND state = ?", userID, itemID, PurchaseStateCompleted).
				Select("COALESCE(SUM(quantity),0)").Scan(&lifetimeUnits).Error; err != nil {
				return err
			}
			if int(lifetimeUnits)+quantity > item.MaxPerUser {
				return ErrPurchaseLimit
			}
		}

		// 3) Cena + zostatok
		var price int
		switch currencyType {
		case CurrencyCredits:
			price = item.CreditsPrice * quantity
		case CurrencyEssence:
			price = item.EssencePrice * quantity
		default:
			return ErrInvalidCurrency
		}
		currency, err := s.GetUserCurrency(userID, currencyType)
		if err != nil {
			return err
		}
		if !currency.HasEnough(price) {
			return ErrInsufficientFunds
		}

		// 4) Bezpečný stock decrement (atómovo)
		if item.IsLimited {
			if item.Stock < quantity {
				return ErrOutOfStock
			}
			// UPDATE stock = stock - quantity WHERE id = ? AND stock >= quantity
			res := tx.Model(&MarketItem{}).
				Where("id = ? AND stock >= ?", item.ID, quantity).
				UpdateColumn("stock", gorm.Expr("stock - ?", quantity))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrOutOfStock
			}
		}

		// B2) Rezervácia skladu – isti si lock a kolízie rieš graciózne
		if item.IsLimited {
			// Vytvor stock rezerváciu
			reserve := StockLedger{
				MarketItemID: item.ID,
				Delta:        -quantity, // záporné pri reserve
				Reason:       StockReasonReserve,
				RefID:        &itemID, // použijeme itemID ako ref_id
			}

			if err := tx.Create(&reserve).Error; err != nil {
				// Ak padlo na unique constraint, vráť „already reserved"
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return ErrAlreadyReserved
				}
				return err
			}
		}

		// B3) Vytvor transakciu (credits/essence log) → musí vzniknúť skôr, než user_purchases
		before := currency.Amount
		currency.Subtract(price)
		txn := Transaction{
			UserID:        userID,
			Type:          TransactionTypePurchase,
			CurrencyType:  currencyType,
			Amount:        -price,
			BalanceBefore: before,
			BalanceAfter:  currency.Amount,
			Description:   fmt.Sprintf("Purchased %d x %s", quantity, item.Name),
			ItemID:        &itemID,
			ItemType:      item.Type,
		}
		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		// B1) Až teraz user_purchases – POZOR na ON CONFLICT
		if idempotencyKey != nil {
			// INSERT s ON CONFLICT len ak máme non-null idemKey a index existuje
			p := UserPurchase{
				UserID:         userID,
				MarketItemID:   itemID,
				Quantity:       quantity,
				State:          PurchaseStateCompleted,
				TransactionID:  txn.ID,
				IdempotencyKey: idempotencyKey,
			}
			if currencyType == CurrencyCredits {
				p.PaidCredits = price
			} else {
				p.PaidEssence = price
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}},
				DoNothing: true,
			}).Create(&p).Error; err != nil {
				return err
			}

			// Znovu prečítaj pre istotu (ak DO NOTHING nastal)
			var up UserPurchase
			if err := tx.Where("user_id = ? AND idempotency_key = ?", userID, *idempotencyKey).
				Last(&up).Error; err != nil {
				return err
			}
			result = &up

			// Mint items do inventára len ak sa vytvoril nový záznam (nie duplicitný)
			// Skontroluj, či je to nový záznam porovnaním s p.ID
			if p.ID != uuid.Nil {
				for i := 0; i < quantity; i++ {
					if err := s.mintItemToInventory(tx, userID, &item); err != nil {
						return fmt.Errorf("chyba pri mintovaní itemu do inventára: %w", err)
					}
				}
			}
		} else {
			// Bez idempotencie – obyčajný insert
			p := UserPurchase{
				UserID:        userID,
				MarketItemID:  itemID,
				Quantity:      quantity,
				State:         PurchaseStateCompleted,
				TransactionID: txn.ID,
			}
			if currencyType == CurrencyCredits {
				p.PaidCredits = price
			} else {
				p.PaidEssence = price
			}

			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			result = &p
			// Mint items do inventára
			for i := 0; i < quantity; i++ {
				if err := s.mintItemToInventory(tx, userID, &item); err != nil {
					return fmt.Errorf("chyba pri mintovaní itemu do inventára: %w", err)
				}
			}
		}

		// 7) Update currency (po úspešnom zápise)
		if err := tx.Save(currency).Error; err != nil {
			return err
		}
		return nil
	})
	return result, err
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
	var inventoryItem gameplay.InventoryItem
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
	var loadoutItem gameplay.LoadoutItem
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
func (s *Service) getUser(userID uuid.UUID) (*User, error) {
	var user User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) canUserAccessItem(user *User, item *MarketItem) bool {
	return user.Tier >= item.TierRequired && user.Level >= item.LevelRequired
}

// Phase 1: Helper method to add lock information to items
func (s *Service) addLockInformation(user *User, item *MarketItem) MarketItem {
	// Create a copy of the item to avoid modifying the original
	itemCopy := *item

	// Check if item is locked and why
	var lockedReasons []string
	isLocked := false

	// Check tier requirement
	if user.Tier < item.TierRequired {
		lockedReasons = append(lockedReasons, "tier")
		isLocked = true
	}

	// Check level requirement
	if user.Level < item.LevelRequired {
		lockedReasons = append(lockedReasons, "level")
		isLocked = true
	}

	// Check time availability
	now := time.Now()
	if item.AvailableFrom != nil && now.Before(*item.AvailableFrom) {
		lockedReasons = append(lockedReasons, "time")
		isLocked = true
	}
	if item.AvailableUntil != nil && now.After(*item.AvailableUntil) {
		lockedReasons = append(lockedReasons, "time")
		isLocked = true
	}

	// Check stock availability
	if item.IsLimited && item.Stock <= 0 {
		lockedReasons = append(lockedReasons, "stock")
		isLocked = true
	}

	// Check daily/weekly/max_per_user limits (len indikácia, bez ťahania veľkých datasetov)
	if item.DailyLimit != nil {
		var used int64
		_ = s.db.Model(&UserPurchase{}).
			Where("user_id = ? AND market_item_id = ? AND state = ? AND created_at >= ?",
				user.ID, item.ID, PurchaseStateCompleted, now.Add(-24*time.Hour)).
			Select("COALESCE(SUM(quantity),0)").Scan(&used).Error
		if int(used) >= *item.DailyLimit {
			lockedReasons = append(lockedReasons, "limit")
			isLocked = true
		}
	}
	if item.WeeklyLimit != nil {
		var used int64
		_ = s.db.Model(&UserPurchase{}).
			Where("user_id = ? AND market_item_id = ? AND state = ? AND created_at >= ?",
				user.ID, item.ID, PurchaseStateCompleted, now.Add(-7*24*time.Hour)).
			Select("COALESCE(SUM(quantity),0)").Scan(&used).Error
		if int(used) >= *item.WeeklyLimit {
			lockedReasons = append(lockedReasons, "limit")
			isLocked = true
		}
	}
	if item.MaxPerUser > 0 {
		var total int64
		_ = s.db.Model(&UserPurchase{}).
			Where("user_id = ? AND market_item_id = ? AND state = ?", user.ID, item.ID, PurchaseStateCompleted).
			Select("COALESCE(SUM(quantity),0)").Scan(&total).Error
		if int(total) >= item.MaxPerUser {
			lockedReasons = append(lockedReasons, "limit")
			isLocked = true
		}
	}

	// Add lock information to properties
	if itemCopy.Properties == nil {
		itemCopy.Properties = make(common.JSONB)
	}

	itemCopy.Properties["locked"] = isLocked
	if isLocked {
		itemCopy.Properties["locked_reasons"] = lockedReasons
	}

	return itemCopy
}

func (s *Service) calculateSellPrice(item *gameplay.InventoryItem) int {
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
	User        *User        `json:"user,omitempty" gorm:"foreignKey:UserID"`
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
		if err := tx.Model(&User{}).
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
	var user User
	err := s.db.First(&user, userID).Error
	if err != nil {
		log.Printf("❌ [TIER SERVICE] Failed to find user %s: %v", userID.String(), err)
		return err
	}

	log.Printf("📊 [TIER SERVICE] User %s - Tier: %d, Expires: %v", userID.String(), user.Tier, user.TierExpires)

	// Ak má tier a je expirovaný
	if user.Tier > 0 && user.TierExpires != nil && user.TierExpires.Before(time.Now()) {
		log.Printf("🔄 [TIER SERVICE] Resetting expired tier for user %s (tier %d expired at %v)", userID.String(), user.Tier, user.TierExpires)
		// Reset na tier 0
		err := s.db.Model(&User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"tier":         0,
				"tier_expires": nil,
			}).Error
		if err != nil {
			log.Printf("❌ [TIER SERVICE] Failed to reset tier for user %s: %v", userID.String(), err)
			return err
		}
		log.Printf("✅ [TIER SERVICE] Successfully reset tier to 0 for user %s", userID.String())
		return nil
	}

	log.Printf("✓ [TIER SERVICE] No action needed for user %s", userID.String())
	return nil
}

// Batch check and reset all expired tiers (pre admin endpoint)
func (s *Service) CheckAndResetAllExpiredTiers() (int, error) {
	var count int64

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Nájdeme všetkých userov s expirovaným tier
		var users []User
		err := tx.Where("tier > 0 AND tier_expires < ?", time.Now()).Find(&users).Error
		if err != nil {
			return err
		}

		count = int64(len(users))

		// Reset na tier 0
		if count > 0 {
			if err := tx.Model(&User{}).
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

// =====================================================
// PHASE 2 - ORDER SYSTEM SERVICE METHODS
// =====================================================

// Order result types
type CreateOrderResult struct {
	Order *Order
}

type CompleteOrderResult struct {
	OrderID           uuid.UUID
	ItemsMinted       int
	FinalPriceCredits int
	FinalPriceEssence int
}

type CancelOrderResult struct {
	OrderID       uuid.UUID
	RefundCredits int
	RefundEssence int
}

// CreateOrder - vytvorenie novej objednávky
func (s *Service) CreateOrder(userID uuid.UUID, itemID uuid.UUID, quantity int, expediteEssence int, idempotencyKey *uuid.UUID) (*Order, error) {
	var order *Order

	// Idempotencia - skontroluj či už existuje objednávka s týmto idempotency_key
	if idempotencyKey != nil {
		var existingOrder Order
		err := s.db.Where("user_id = ? AND idempotency_key = ?", userID, *idempotencyKey).First(&existingOrder).Error
		if err == nil {
			// Objednávka už existuje, vráť ju
			return &existingOrder, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("chyba pri kontrole idempotencie: %w", err)
		}
	}

	// Získaj market item
	var marketItem MarketItem
	if err := s.db.Where("id = ? AND is_active = true", itemID).First(&marketItem).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("chyba pri získavaní market itemu: %w", err)
	}

	// Validácia tier a level požiadaviek
	user, err := s.getUser(userID)
	if err != nil {
		return nil, fmt.Errorf("chyba pri získavaní informácií o hráčovi: %w", err)
	}

	if user.Tier < marketItem.TierRequired {
		return nil, fmt.Errorf("nedostatočný tier: požadovaný %d, máte %d", marketItem.TierRequired, user.Tier)
	}

	if user.Level < marketItem.LevelRequired {
		return nil, fmt.Errorf("nedostatočný level: požadovaný %d, máte %d", marketItem.LevelRequired, user.Level)
	}

	// Kontrola časových obmedzení
	now := time.Now()
	if marketItem.AvailableFrom != nil && now.Before(*marketItem.AvailableFrom) {
		return nil, fmt.Errorf("item nie je ešte dostupný")
	}
	if marketItem.AvailableUntil != nil && now.After(*marketItem.AvailableUntil) {
		return nil, fmt.Errorf("item už nie je dostupný")
	}

	// Kontrola denných/týždenných limitov
	if err := s.checkPurchaseLimits(userID, itemID, quantity); err != nil {
		return nil, err
	}

	// Kontrola počtu otvorených objednávok
	if err := s.checkOrderLimits(userID, itemID); err != nil {
		return nil, err
	}

	// Vypočítaj ETA
	eta, err := s.calculateETA(marketItem.Rarity, expediteEssence)
	if err != nil {
		return nil, fmt.Errorf("chyba pri výpočte ETA: %w", err)
	}

	// Vypočítaj zálohu
	depositPct, err := s.getDepositPercentage(marketItem.Rarity)
	if err != nil {
		return nil, fmt.Errorf("chyba pri získavaní percenta zálohy: %w", err)
	}

	depositCredits := (marketItem.CreditsPrice * quantity * depositPct) / 100
	depositEssence := (marketItem.EssencePrice * quantity * depositPct) / 100

	// Kontrola dostatočných prostriedkov na zálohu
	if depositCredits > 0 {
		credits, err := s.GetUserCurrency(userID, CurrencyCredits)
		if err != nil {
			return nil, fmt.Errorf("chyba pri získavaní credits: %w", err)
		}
		if credits.Amount < depositCredits {
			return nil, ErrInsufficientFunds
		}
	}

	if depositEssence > 0 {
		essence, err := s.GetUserCurrency(userID, CurrencyEssence)
		if err != nil {
			return nil, fmt.Errorf("chyba pri získavaní essence: %w", err)
		}
		if essence.Amount < depositEssence {
			return nil, ErrInsufficientFunds
		}
	}

	// Vytvor objednávku v transakcii
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Vytvor objednávku
		order = &Order{
			UserID:               userID,
			MarketItemID:         itemID,
			Quantity:             quantity,
			DepositPct:           depositPct,
			DepositAmountCredits: depositCredits,
			DepositAmountEssence: depositEssence,
			ExpediteEssence:      expediteEssence,
			ETAAt:                &eta,
			PriceLockedCredits:   marketItem.CreditsPrice,
			PriceLockedEssence:   marketItem.EssencePrice,
			State:                OrderStatePlaced,
			IdempotencyKey:       idempotencyKey,
		}

		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("chyba pri vytváraní objednávky: %w", err)
		}

		// Zaúčtuj zálohu
		if depositCredits > 0 {
			credits, err := s.GetUserCurrency(userID, CurrencyCredits)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní credits: %w", err)
			}
			credits.Subtract(depositCredits)
			if err := tx.Save(credits).Error; err != nil {
				return fmt.Errorf("chyba pri zaúčtovaní credits zálohy: %w", err)
			}
		}

		if depositEssence > 0 {
			essence, err := s.GetUserCurrency(userID, CurrencyEssence)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní essence: %w", err)
			}
			essence.Subtract(depositEssence)
			if err := tx.Save(essence).Error; err != nil {
				return fmt.Errorf("chyba pri zaúčtovaní essence zálohy: %w", err)
			}
		}

		// Ak je sklad dostupný, rezervuj ho a preklop na SCHEDULED
		availableStock := s.getAvailableStock(itemID)
		if availableStock >= quantity {
			// Rezervuj sklad
			if err := s.reserveStock(tx, itemID, quantity, order.ID); err != nil {
				return fmt.Errorf("chyba pri rezervácii skladu: %w", err)
			}

			// Preklop na SCHEDULED
			order.State = OrderStateScheduled
			if err := tx.Save(order).Error; err != nil {
				return fmt.Errorf("chyba pri aktualizácii stavu objednávky: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return order, nil
}

// GetUserOrders - zoznam objednávok hráča
func (s *Service) GetUserOrders(userID uuid.UUID, state string, limit int, offset int) ([]Order, int, error) {
	var orders []Order
	var total int64

	query := s.db.Model(&Order{}).Where("user_id = ?", userID)

	if state != "" {
		query = query.Where("state = ?", state)
	}

	// Spočítaj celkový počet
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("chyba pri počítaní objednávok: %w", err)
	}

	// Získaj objednávky s market item informáciami
	if err := query.Preload("MarketItem").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("chyba pri získavaní objednávok: %w", err)
	}

	return orders, int(total), nil
}

// CompleteOrder - dokončenie objednávky
func (s *Service) CompleteOrder(userID uuid.UUID, orderID uuid.UUID, idempotencyKey *uuid.UUID) (*CompleteOrderResult, error) {
	var result *CompleteOrderResult

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Získaj objednávku
		var order Order
		if err := tx.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("objednávka nebola nájdená")
			}
			return fmt.Errorf("chyba pri získavaní objednávky: %w", err)
		}

		// Kontrola stavu
		if !order.CanBeCompleted() {
			return fmt.Errorf("objednávka nemôže byť dokončená v stave %s", order.State)
		}

		// Idempotencia pre complete
		if idempotencyKey != nil {
			// Skontroluj či už nebola dokončená s týmto idempotency_key
			var existingPurchase UserPurchase
			err := tx.Where("user_id = ? AND idempotency_key = ?", userID, *idempotencyKey).First(&existingPurchase).Error
			if err == nil {
				// Už bola dokončená, vráť existujúci výsledok
				result = &CompleteOrderResult{
					OrderID:           order.ID,
					ItemsMinted:       order.Quantity,
					FinalPriceCredits: order.PriceLockedCredits * order.Quantity,
					FinalPriceEssence: order.PriceLockedEssence * order.Quantity,
				}
				return nil
			}
		}

		// Vypočítaj zvyšok ceny
		remainingCredits, remainingEssence := order.GetRemainingPrice()

		// Kontrola dostatočných prostriedkov
		if remainingCredits > 0 {
			credits, err := s.GetUserCurrency(userID, CurrencyCredits)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní credits: %w", err)
			}
			if credits.Amount < remainingCredits {
				return ErrInsufficientFunds
			}
		}

		if remainingEssence > 0 {
			essence, err := s.GetUserCurrency(userID, CurrencyEssence)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní essence: %w", err)
			}
			if essence.Amount < remainingEssence {
				return ErrInsufficientFunds
			}
		}

		// Zaúčtuj zvyšok ceny
		if remainingCredits > 0 {
			credits, err := s.GetUserCurrency(userID, CurrencyCredits)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní credits: %w", err)
			}
			credits.Subtract(remainingCredits)
			if err := tx.Save(credits).Error; err != nil {
				return fmt.Errorf("chyba pri zaúčtovaní credits: %w", err)
			}
		}

		if remainingEssence > 0 {
			essence, err := s.GetUserCurrency(userID, CurrencyEssence)
			if err != nil {
				return fmt.Errorf("chyba pri získavaní essence: %w", err)
			}
			essence.Subtract(remainingEssence)
			if err := tx.Save(essence).Error; err != nil {
				return fmt.Errorf("chyba pri zaúčtovaní essence: %w", err)
			}
		}

		// Mint items do inventára
		for i := 0; i < order.Quantity; i++ {
			if err := s.mintItemToInventory(tx, userID, &order.MarketItem); err != nil {
				return fmt.Errorf("chyba pri mintovaní itemu do inventára: %w", err)
			}
		}

		// Vytvor purchase record
		purchase := UserPurchase{
			UserID:         userID,
			MarketItemID:   order.MarketItemID,
			Quantity:       order.Quantity,
			PaidCredits:    order.PriceLockedCredits * order.Quantity,
			PaidEssence:    order.PriceLockedEssence * order.Quantity,
			State:          PurchaseStateCompleted,
			IdempotencyKey: idempotencyKey,
		}

		// Vytvor transaction record
		transactionID := uuid.New()
		purchase.TransactionID = transactionID

		if err := tx.Create(&purchase).Error; err != nil {
			return fmt.Errorf("chyba pri vytváraní purchase recordu: %w", err)
		}

		// Aktualizuj stav objednávky
		order.State = OrderStateCompleted
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii stavu objednávky: %w", err)
		}

		// Uvoľni rezerváciu skladu
		if err := s.releaseStock(tx, order.MarketItemID, order.Quantity, order.ID); err != nil {
			return fmt.Errorf("chyba pri uvoľňovaní rezervácie skladu: %w", err)
		}

		result = &CompleteOrderResult{
			OrderID:           order.ID,
			ItemsMinted:       order.Quantity,
			FinalPriceCredits: order.PriceLockedCredits * order.Quantity,
			FinalPriceEssence: order.PriceLockedEssence * order.Quantity,
		}

		return nil
	})

	return result, err
}

// ExpediteOrder - zrýchlenie objednávky
func (s *Service) ExpediteOrder(userID uuid.UUID, orderID uuid.UUID, expediteEssence int) (*Order, error) {
	var order *Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Získaj objednávku
		if err := tx.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("objednávka nebola nájdená")
			}
			return fmt.Errorf("chyba pri získavaní objednávky: %w", err)
		}

		// Kontrola stavu
		if !order.CanBeExpedited() {
			return fmt.Errorf("objednávka nemôže byť zrýchlená v stave %s", order.State)
		}

		// Kontrola dostatočných essence
		essence, err := s.GetUserCurrency(userID, CurrencyEssence)
		if err != nil {
			return fmt.Errorf("chyba pri získavaní essence: %w", err)
		}
		if essence.Amount < expediteEssence {
			return ErrInsufficientFunds
		}

		// Zaúčtuj essence
		essence.Subtract(expediteEssence)
		if err := tx.Save(essence).Error; err != nil {
			return fmt.Errorf("chyba pri zaúčtovaní essence: %w", err)
		}

		// Vypočítaj novú ETA
		newETA, err := s.calculateETA(order.MarketItem.Rarity, order.ExpediteEssence+expediteEssence)
		if err != nil {
			return fmt.Errorf("chyba pri výpočte novej ETA: %w", err)
		}

		// Aktualizuj objednávku
		order.ExpediteEssence += expediteEssence
		order.ETAAt = &newETA
		if err := tx.Save(order).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii objednávky: %w", err)
		}

		return nil
	})

	return order, err
}

// CancelOrder - zrušenie objednávky
func (s *Service) CancelOrder(userID uuid.UUID, orderID uuid.UUID) (*CancelOrderResult, error) {
	var result *CancelOrderResult

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Získaj objednávku
		var order Order
		if err := tx.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("objednávka nebola nájdená")
			}
			return fmt.Errorf("chyba pri získavaní objednávky: %w", err)
		}

		// Kontrola stavu
		if !order.CanBeCancelled() {
			return fmt.Errorf("objednávka nemôže byť zrušená v stave %s", order.State)
		}

		// Vypočítaj refund
		refundCredits := order.DepositAmountCredits
		refundEssence := order.DepositAmountEssence

		// Aplikuj cancel fee ak je potrebné
		if order.State == OrderStateScheduled {
			// Malý fee pre cancel pred READY
			cancelFeePct, _ := s.getSettingInt("cancel_fee_pct_pre_ready")
			if cancelFeePct > 0 {
				refundCredits = refundCredits * (100 - cancelFeePct) / 100
				refundEssence = refundEssence * (100 - cancelFeePct) / 100
			}
		}

		// Vráť prostriedky
		if refundCredits > 0 {
			if err := s.AddCurrency(userID, CurrencyCredits, refundCredits, "Order cancellation refund"); err != nil {
				return fmt.Errorf("chyba pri vracaní credits: %w", err)
			}
		}

		if refundEssence > 0 {
			if err := s.AddCurrency(userID, CurrencyEssence, refundEssence, "Order cancellation refund"); err != nil {
				return fmt.Errorf("chyba pri vracaní essence: %w", err)
			}
		}

		// Uvoľni rezerváciu skladu ak existuje
		if order.State == OrderStateScheduled {
			if err := s.releaseStock(tx, order.MarketItemID, order.Quantity, order.ID); err != nil {
				return fmt.Errorf("chyba pri uvoľňovaní rezervácie skladu: %w", err)
			}
		}

		// Aktualizuj stav objednávky
		order.State = OrderStateCancelledRefund
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii stavu objednávky: %w", err)
		}

		result = &CancelOrderResult{
			OrderID:       order.ID,
			RefundCredits: refundCredits,
			RefundEssence: refundEssence,
		}

		return nil
	})

	return result, err
}

// Helper methods for Order System

// calculateETA - výpočet ETA na základe rarity a expedite essence
func (s *Service) calculateETA(rarity string, expediteEssence int) (time.Time, error) {
	// Získaj základnú ETA z settings
	baseMinutes, err := s.getSettingInt("eta_base_" + rarity)
	if err != nil {
		// Fallback hodnoty
		baseMinutes = map[string]int{
			"common":    15,
			"rare":      120,
			"epic":      480,
			"legendary": 1440,
		}[rarity]
		if baseMinutes == 0 {
			baseMinutes = 60 // 1 hodina default
		}
	}

	// Ak nie je expedite, vráť základnú ETA
	if expediteEssence <= 0 {
		return time.Now().Add(time.Duration(baseMinutes) * time.Minute), nil
	}

	// Získaj expedite nastavenia
	expediteCapPct, _ := s.getSettingInt("expedite_cap_pct")
	expediteK, _ := s.getSettingFloat("expedite_formula_k")

	// Fallback hodnoty
	if expediteCapPct == 0 {
		expediteCapPct = 85
	}
	if expediteK == 0 {
		expediteK = 0.1
	}

	// Vypočítaj redukciu (klesajúci výnos, cap 85%)
	reductionPct := float64(expediteCapPct) / 100.0
	if reductionPct > (1 - 1/(1+expediteK*float64(expediteEssence))) {
		reductionPct = 1 - 1/(1+expediteK*float64(expediteEssence))
	}

	// Finálna ETA
	finalMinutes := float64(baseMinutes) * (1 - reductionPct)
	if finalMinutes < 1 {
		finalMinutes = 1 // Minimálne 1 minúta
	}

	return time.Now().Add(time.Duration(finalMinutes) * time.Minute), nil
}

// getDepositPercentage - získanie percenta zálohy podľa rarity
func (s *Service) getDepositPercentage(rarity string) (int, error) {
	depositPct, err := s.getSettingInt("deposit_pct_" + rarity)
	if err != nil {
		// Fallback hodnoty
		depositPct = map[string]int{
			"common":    30,
			"rare":      40,
			"epic":      50,
			"legendary": 60,
		}[rarity]
		if depositPct == 0 {
			depositPct = 30 // 30% default
		}
	}
	return depositPct, nil
}

// getAvailableStock - získanie dostupného skladu
func (s *Service) getAvailableStock(marketItemID uuid.UUID) int {
	var available int
	err := s.db.Raw("SELECT market.get_available_stock(?)", marketItemID).Scan(&available).Error
	if err != nil {
		return 0
	}
	return available
}

// reserveStock - rezervácia skladu
func (s *Service) reserveStock(tx *gorm.DB, marketItemID uuid.UUID, quantity int, orderID uuid.UUID) error {
	ledger := StockLedger{
		MarketItemID: marketItemID,
		Delta:        -quantity, // Negatívne pre rezerváciu
		Reason:       StockReasonReserve,
		RefID:        &orderID,
	}
	return tx.Create(&ledger).Error
}

// releaseStock - uvoľnenie rezervácie skladu
func (s *Service) releaseStock(tx *gorm.DB, marketItemID uuid.UUID, quantity int, orderID uuid.UUID) error {
	ledger := StockLedger{
		MarketItemID: marketItemID,
		Delta:        quantity, // Pozitívne pre uvoľnenie
		Reason:       StockReasonRelease,
		RefID:        &orderID,
	}
	return tx.Create(&ledger).Error
}

// checkPurchaseLimits - kontrola denných/týždenných limitov
func (s *Service) checkPurchaseLimits(userID uuid.UUID, itemID uuid.UUID, quantity int) error {
	var marketItem MarketItem
	if err := s.db.Where("id = ?", itemID).First(&marketItem).Error; err != nil {
		return fmt.Errorf("chyba pri získavaní market itemu: %w", err)
	}

	now := time.Now()

	// Kontrola denného limitu
	if marketItem.DailyLimit != nil && *marketItem.DailyLimit > 0 {
		var dailyUsed int
		err := s.db.Model(&UserPurchase{}).
			Where("user_id = ? AND market_item_id = ? AND created_at >= ?",
				userID, itemID, now.Truncate(24*time.Hour)).
			Select("COALESCE(SUM(quantity), 0)").
			Scan(&dailyUsed).Error
		if err != nil {
			return fmt.Errorf("chyba pri kontrole denného limitu: %w", err)
		}

		if dailyUsed+quantity > *marketItem.DailyLimit {
			return fmt.Errorf("denný limit prekročený: %d/%d", dailyUsed+quantity, *marketItem.DailyLimit)
		}
	}

	// Kontrola týždenného limitu
	if marketItem.WeeklyLimit != nil && *marketItem.WeeklyLimit > 0 {
		var weeklyUsed int
		err := s.db.Model(&UserPurchase{}).
			Where("user_id = ? AND market_item_id = ? AND created_at >= ?",
				userID, itemID, now.AddDate(0, 0, -7)).
			Select("COALESCE(SUM(quantity), 0)").
			Scan(&weeklyUsed).Error
		if err != nil {
			return fmt.Errorf("chyba pri kontrole týždenného limitu: %w", err)
		}

		if weeklyUsed+quantity > *marketItem.WeeklyLimit {
			return fmt.Errorf("týždenný limit prekročený: %d/%d", weeklyUsed+quantity, *marketItem.WeeklyLimit)
		}
	}

	return nil
}

// checkOrderLimits - kontrola počtu otvorených objednávok
func (s *Service) checkOrderLimits(userID uuid.UUID, itemID uuid.UUID) error {
	// Kontrola celkového počtu otvorených objednávok
	maxOrdersPerUser, _ := s.getSettingInt("max_orders_per_user")
	if maxOrdersPerUser > 0 {
		var openOrders int64
		err := s.db.Model(&Order{}).
			Where("user_id = ? AND state IN (?, ?)", userID, OrderStatePlaced, OrderStateScheduled).
			Count(&openOrders).Error
		if err != nil {
			return fmt.Errorf("chyba pri kontrole počtu otvorených objednávok: %w", err)
		}

		if int(openOrders) >= maxOrdersPerUser {
			return fmt.Errorf("príliš veľa otvorených objednávok: %d/%d", openOrders, maxOrdersPerUser)
		}
	}

	// Kontrola počtu objednávok na konkrétny SKU
	maxOrdersPerSKU, _ := s.getSettingInt("max_orders_per_sku")
	if maxOrdersPerSKU > 0 {
		var openOrdersForSKU int64
		err := s.db.Model(&Order{}).
			Where("user_id = ? AND market_item_id = ? AND state IN (?, ?)",
				userID, itemID, OrderStatePlaced, OrderStateScheduled).
			Count(&openOrdersForSKU).Error
		if err != nil {
			return fmt.Errorf("chyba pri kontrole počtu objednávok na SKU: %w", err)
		}

		if int(openOrdersForSKU) >= maxOrdersPerSKU {
			return fmt.Errorf("príliš veľa objednávok na tento item: %d/%d", openOrdersForSKU, maxOrdersPerSKU)
		}
	}

	return nil
}

// getSettingInt - získanie integer nastavenia
func (s *Service) getSettingInt(key string) (int, error) {
	var setting MarketSettings
	err := s.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return 0, err
	}

	// Pokús sa extrahovať int z JSONB
	if val, ok := setting.Value["value"].(float64); ok {
		return int(val), nil
	}
	if val, ok := setting.Value["value"].(int); ok {
		return val, nil
	}

	return 0, fmt.Errorf("nepodporovaný typ pre nastavenie %s", key)
}

// getSettingFloat - získanie float nastavenia
func (s *Service) getSettingFloat(key string) (float64, error) {
	var setting MarketSettings
	err := s.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return 0, err
	}

	// Pokús sa extrahovať float z JSONB
	if val, ok := setting.Value["value"].(float64); ok {
		return val, nil
	}
	if val, ok := setting.Value["value"].(int); ok {
		return float64(val), nil
	}

	return 0, fmt.Errorf("nepodporovaný typ pre nastavenie %s", key)
}

// mintItemToInventory - mint itemu do inventára
func (s *Service) mintItemToInventory(tx *gorm.DB, userID uuid.UUID, marketItem *MarketItem) error {
	// Určite item_type na základe market item kategórie
	itemType := "gear" // default (legacy)

	switch marketItem.Category {
	case "deployable_scanners":
		itemType = "deployable_scanner"
	case "scanner_batteries":
		itemType = "scanner_battery"
	case "hack_tools":
		itemType = "hack_tool"
	case "potions", "buffs", "consumables":
		itemType = "consumable"
	case "cosmetics", "skins":
		itemType = "cosmetic"
	case "artifacts":
		itemType = "artifact"
	default:
		// Pre ostatné kategórie použij market item type
		if marketItem.IsScannerItem() {
			itemType = "deployable_scanner"
		} else if marketItem.IsPowerCellItem() {
			itemType = "scanner_battery"
		} else {
			itemType = marketItem.Type // fallback na market item type
		}
	}

	// Vytvor správne properties na základe MarketItem dát
	properties := map[string]interface{}{
		"display_name":   marketItem.Name, // pre UI fallback
		"name":           marketItem.Name,
		"slot":           "scanner",
		"type":           itemType, // zosúladiť s ItemType
		"level":          marketItem.Level,
		"rarity":         marketItem.Rarity,
		"category":       marketItem.Category,
		"equipped":       false,
		"purchased_at":   time.Now().Unix(),
		"market_item_id": marketItem.ID,
		"purchased_from": "market",
	}

	// ✨ Pridaj uses_left a tool_type pre hack tools
	if itemType == "hack_tool" {
		// Extrahuj uses z market item properties
		if marketItem.Properties != nil {
			if uses, ok := marketItem.Properties["uses"]; ok {
				properties["uses_left"] = uses
			} else {
				properties["uses_left"] = 1 // default 1 použitie
			}

			if toolType, ok := marketItem.Properties["tool_type"]; ok {
				properties["tool_type"] = toolType
			} else {
				properties["tool_type"] = "basic_hack" // default
			}

			// Skopíruj ostatné relevantné properties z market item
			if successRate, ok := marketItem.Properties["success_rate"]; ok {
				properties["success_rate"] = successRate
			}
			if hackTime, ok := marketItem.Properties["hack_time_seconds"]; ok {
				properties["hack_time_seconds"] = hackTime
			}
		} else {
			// Fallback ak market item nemá properties
			properties["uses_left"] = 1
			properties["tool_type"] = "basic_hack"
		}

		log.Printf("🔧 [MARKET PURCHASE] Hack Tool Created:")
		log.Printf("  → User ID: %s", userID)
		log.Printf("  → Name: %s", marketItem.Name)
		log.Printf("  → Tool Type: %v", properties["tool_type"])
		log.Printf("  → Uses Left: %v", properties["uses_left"])
		log.Printf("  → Level: %v", properties["level"])
		log.Printf("  → Rarity: %v", properties["rarity"])
		log.Printf("  → Market Item ID: %s", marketItem.ID)
	}

	// Zvoliť správny ItemID pre klientské mapovanie (katalóg > market)
	itemID := marketItem.ID
	if marketItem.IsScannerItem() && marketItem.ScannerCatalogID != nil {
		itemID = *marketItem.ScannerCatalogID
		properties["catalog_ref"] = "scanner_catalog"
	} else if marketItem.IsPowerCellItem() && marketItem.PowerCellCatalogID != nil {
		itemID = *marketItem.PowerCellCatalogID
		properties["catalog_ref"] = "power_cells_catalog"
	}

	// Vytvor inventory item
	inventoryItem := gameplay.InventoryItem{
		UserID:     userID,
		ItemType:   itemType,
		ItemID:     itemID,
		Properties: gameplay.JSONB(properties),
		Quantity:   1,
	}

	return tx.Create(&inventoryItem).Error
}
