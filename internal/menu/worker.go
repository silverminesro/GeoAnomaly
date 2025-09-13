package menu

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OrderWorker - worker pre automatické stavové prechody objednávok
type OrderWorker struct {
	db     *gorm.DB
	stopCh chan bool
}

// NewOrderWorker - vytvorenie nového worker-a
func NewOrderWorker(db *gorm.DB) *OrderWorker {
	return &OrderWorker{
		db:     db,
		stopCh: make(chan bool),
	}
}

// Start - spustenie worker-a
func (w *OrderWorker) Start() {
	log.Println("Order Worker: Starting...")

	ticker := time.NewTicker(1 * time.Minute) // Spúšťaj každú minútu
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processOrders()
		case <-w.stopCh:
			log.Println("Order Worker: Stopping...")
			return
		}
	}
}

// Stop - zastavenie worker-a
func (w *OrderWorker) Stop() {
	close(w.stopCh)
}

// processOrders - hlavná logika worker-a
func (w *OrderWorker) processOrders() {
	// Pridaj náhodný jitter (±10 sekúnd) pre distribúciu loadu
	jitter := time.Duration(rand.Intn(20)-10) * time.Second
	time.Sleep(jitter)

	// Získaj distributed lock
	lockID := w.getLockID()
	if !w.acquireLock(lockID) {
		log.Println("Order Worker: Could not acquire lock, skipping this run")
		return
	}
	defer w.releaseLock(lockID)

	log.Println("Order Worker: Processing orders...")

	// Spracuj SCHEDULED → READY_FOR_PICKUP
	if err := w.processScheduledOrders(); err != nil {
		log.Printf("Order Worker: Error processing scheduled orders: %v", err)
	}

	// Spracuj READY_FOR_PICKUP → CANCELLED_FORFEIT (expired)
	if err := w.processExpiredPickups(); err != nil {
		log.Printf("Order Worker: Error processing expired pickups: %v", err)
	}

	log.Println("Order Worker: Processing completed")
}

// processScheduledOrders - spracovanie SCHEDULED objednávok
func (w *OrderWorker) processScheduledOrders() error {
	now := time.Now()

	// Získaj objednávky ktoré majú ETA <= now s row-lockom a SKIP LOCKED
	var orders []Order
	err := w.db.
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("state = ? AND eta_at <= ?", OrderStateScheduled, now).
		Find(&orders).Error
	if err != nil {
		return fmt.Errorf("chyba pri získavaní scheduled objednávok: %w", err)
	}

	log.Printf("Order Worker: Found %d scheduled orders ready for pickup", len(orders))

	for _, order := range orders {
		if err := w.transitionToReadyForPickup(&order); err != nil {
			log.Printf("Order Worker: Error transitioning order %s: %v", order.ID, err)
			continue
		}
		log.Printf("Order Worker: Order %s transitioned to READY_FOR_PICKUP", order.ID)
	}

	return nil
}

// processExpiredPickups - spracovanie expirovaných pickup-ov
func (w *OrderWorker) processExpiredPickups() error {
	now := time.Now()

	// Získaj objednávky ktoré majú pickup_expires_at < now
	var orders []Order
	err := w.db.Where("state = ? AND pickup_expires_at < ?", OrderStateReadyForPickup, now).
		Find(&orders).Error
	if err != nil {
		return fmt.Errorf("chyba pri získavaní expirovaných pickup-ov: %w", err)
	}

	log.Printf("Order Worker: Found %d expired pickup orders", len(orders))

	for _, order := range orders {
		if err := w.transitionToCancelledForfeit(&order); err != nil {
			log.Printf("Order Worker: Error transitioning order %s to forfeit: %v", order.ID, err)
			continue
		}
		log.Printf("Order Worker: Order %s transitioned to CANCELLED_FORFEIT", order.ID)
	}

	return nil
}

// transitionToReadyForPickup - prechod SCHEDULED → READY_FOR_PICKUP
func (w *OrderWorker) transitionToReadyForPickup(order *Order) error {
	return w.db.Transaction(func(tx *gorm.DB) error {
		// Znovu načítaj objednávku s lock-om
		var lockedOrder Order
		if err := tx.Where("id = ? AND state = ?", order.ID, OrderStateScheduled).
			First(&lockedOrder).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Objednávka už bola spracovaná alebo zmenená
				return nil
			}
			return fmt.Errorf("chyba pri načítaní objednávky: %w", err)
		}

		// 1) Rezervuj sklad (atómovo; ak zlyhá, nepreklápaj stav)
		if err := w.reserveStockIfNeeded(tx, &lockedOrder); err != nil {
			return fmt.Errorf("chyba pri rezervácii skladu: %w", err)
		}

		// 2) Nastav stav a pickup window
		pickupWindowHours, err := w.getSettingInt("pickup_window_hours")
		if err != nil {
			pickupWindowHours = 6 // 6 hodín default
		}
		pickupExpiresAt := time.Now().Add(time.Duration(pickupWindowHours) * time.Hour)
		lockedOrder.State = OrderStateReadyForPickup
		lockedOrder.PickupExpiresAt = &pickupExpiresAt
		if err := tx.Save(&lockedOrder).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii objednávky: %w", err)
		}

		return nil
	})
}

// transitionToCancelledForfeit - prechod READY_FOR_PICKUP → CANCELLED_FORFEIT
func (w *OrderWorker) transitionToCancelledForfeit(order *Order) error {
	return w.db.Transaction(func(tx *gorm.DB) error {
		// Znovu načítaj objednávku s lock-om
		var lockedOrder Order
		if err := tx.Where("id = ? AND state = ?", order.ID, OrderStateReadyForPickup).
			First(&lockedOrder).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Objednávka už bola spracovaná alebo zmenená
				return nil
			}
			return fmt.Errorf("chyba pri načítaní objednávky: %w", err)
		}

		// Vypočítaj forfeit percento
		forfeitPct, err := w.getSettingInt("forfeit_pct_post_ready")
		if err != nil {
			forfeitPct = 20 // 20% default
		}

		// Vypočítaj refund (s forfeit fee)
		refundCredits := lockedOrder.DepositAmountCredits * (100 - forfeitPct) / 100
		refundEssence := lockedOrder.DepositAmountEssence * (100 - forfeitPct) / 100

		// Vráť prostriedky (s forfeit fee)
		if refundCredits > 0 {
			if err := w.addCurrency(tx, lockedOrder.UserID, CurrencyCredits, refundCredits, "Order forfeit refund"); err != nil {
				return fmt.Errorf("chyba pri vracaní credits: %w", err)
			}
		}

		if refundEssence > 0 {
			if err := w.addCurrency(tx, lockedOrder.UserID, CurrencyEssence, refundEssence, "Order forfeit refund"); err != nil {
				return fmt.Errorf("chyba pri vracaní essence: %w", err)
			}
		}

		// Uvoľni rezerváciu skladu
		if err := w.releaseStock(tx, lockedOrder.MarketItemID, lockedOrder.Quantity, lockedOrder.ID); err != nil {
			return fmt.Errorf("chyba pri uvoľňovaní rezervácie skladu: %w", err)
		}

		// Aktualizuj stav objednávky
		lockedOrder.State = OrderStateCancelledForfeit
		if err := tx.Save(&lockedOrder).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii stavu objednávky: %w", err)
		}

		return nil
	})
}

// reserveStockIfNeeded - rezervácia skladu ak je potrebná
func (w *OrderWorker) reserveStockIfNeeded(tx *gorm.DB, order *Order) error {
	// Skontroluj či už nie je rezervovaný
	var existingReserve StockLedger
	err := tx.Where("market_item_id = ? AND reason = ? AND ref_id = ?",
		order.MarketItemID, StockReasonReserve, order.ID).
		First(&existingReserve).Error

	if err == nil {
		// Už je rezervovaný
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("chyba pri kontrole rezervácie: %w", err)
	}

	// Rezervuj sklad
	ledger := StockLedger{
		MarketItemID: order.MarketItemID,
		Delta:        -order.Quantity, // Negatívne pre rezerváciu
		Reason:       StockReasonReserve,
		RefID:        &order.ID,
	}
	return tx.Create(&ledger).Error
}

// releaseStock - uvoľnenie rezervácie skladu
func (w *OrderWorker) releaseStock(tx *gorm.DB, marketItemID uuid.UUID, quantity int, orderID uuid.UUID) error {
	ledger := StockLedger{
		MarketItemID: marketItemID,
		Delta:        quantity, // Pozitívne pre uvoľnenie
		Reason:       StockReasonRelease,
		RefID:        &orderID,
	}
	return tx.Create(&ledger).Error
}

// addCurrency - pridanie meny (helper metóda)
func (w *OrderWorker) addCurrency(tx *gorm.DB, userID uuid.UUID, currencyType string, amount int, description string) error {
	// Získaj alebo vytvor currency
	var currency Currency
	err := tx.Where("user_id = ? AND type = ?", userID, currencyType).First(&currency).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Vytvor novú currency
			currency = Currency{
				UserID: userID,
				Type:   currencyType,
				Amount: 0,
			}
			if err := tx.Create(&currency).Error; err != nil {
				return fmt.Errorf("chyba pri vytváraní currency: %w", err)
			}
		} else {
			return fmt.Errorf("chyba pri získavaní currency: %w", err)
		}
	}

	// Aktualizuj amount
	oldAmount := currency.Amount
	currency.Amount += amount
	if err := tx.Save(&currency).Error; err != nil {
		return fmt.Errorf("chyba pri aktualizácii currency: %w", err)
	}

	// Vytvor transaction record
	transaction := Transaction{
		UserID:        userID,
		Type:          TransactionTypeReward, // alebo TransactionTypeRefund
		CurrencyType:  currencyType,
		Amount:        amount,
		BalanceBefore: oldAmount,
		BalanceAfter:  currency.Amount,
		Description:   description,
	}

	return tx.Create(&transaction).Error
}

// getSettingInt - získanie integer nastavenia
func (w *OrderWorker) getSettingInt(key string) (int, error) {
	var setting MarketSettings
	err := w.db.Where("key = ?", key).First(&setting).Error
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

// Distributed lock methods

// getLockID - získanie ID pre distributed lock
func (w *OrderWorker) getLockID() int64 {
	// Použij hash z názvu worker-a
	return int64(12345) // Fixed ID pre order worker
}

// acquireLock - získanie distributed lock
func (w *OrderWorker) acquireLock(lockID int64) bool {
	var result bool
	err := w.db.Raw("SELECT pg_try_advisory_lock(?)", lockID).Scan(&result).Error
	if err != nil {
		log.Printf("Order Worker: Error acquiring lock: %v", err)
		return false
	}
	return result
}

// releaseLock - uvoľnenie distributed lock
func (w *OrderWorker) releaseLock(lockID int64) {
	err := w.db.Exec("SELECT pg_advisory_unlock(?)", lockID).Error
	if err != nil {
		log.Printf("Order Worker: Error releasing lock: %v", err)
	}
}

// Manual trigger methods (pre admin/testovanie)

// ProcessOrdersNow - manuálne spustenie spracovania objednávok
func (w *OrderWorker) ProcessOrdersNow() error {
	log.Println("Order Worker: Manual processing triggered")
	w.processOrders()
	return nil
}

// GetWorkerStats - štatistiky worker-a
func (w *OrderWorker) GetWorkerStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Počet objednávok v jednotlivých stavoch
	var scheduledCount, readyCount, expiredCount int64

	err := w.db.Model(&Order{}).Where("state = ?", OrderStateScheduled).Count(&scheduledCount).Error
	if err != nil {
		return nil, fmt.Errorf("chyba pri počítaní scheduled objednávok: %w", err)
	}

	err = w.db.Model(&Order{}).Where("state = ?", OrderStateReadyForPickup).Count(&readyCount).Error
	if err != nil {
		return nil, fmt.Errorf("chyba pri počítaní ready objednávok: %w", err)
	}

	err = w.db.Model(&Order{}).
		Where("state = ? AND pickup_expires_at < ?", OrderStateReadyForPickup, time.Now()).
		Count(&expiredCount).Error
	if err != nil {
		return nil, fmt.Errorf("chyba pri počítaní expirovaných objednávok: %w", err)
	}

	stats["scheduled_orders"] = scheduledCount
	stats["ready_for_pickup_orders"] = readyCount
	stats["expired_pickup_orders"] = expiredCount
	stats["last_check"] = time.Now()

	return stats, nil
}
