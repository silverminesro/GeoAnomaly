package loadout

import (
	"errors"
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetLoadoutSlots vráti všetky dostupné sloty
func (s *Service) GetLoadoutSlots() ([]common.LoadoutSlot, error) {
	var slots []common.LoadoutSlot
	err := s.db.Order("\"order\" ASC").Find(&slots).Error
	return slots, err
}

// GetUserLoadout vráti aktuálny loadout používateľa
func (s *Service) GetUserLoadout(userID uuid.UUID) (map[string]*common.LoadoutItem, error) {
	var loadoutItems []common.LoadoutItem
	err := s.db.Where("user_id = ?", userID).Find(&loadoutItems).Error
	if err != nil {
		return nil, err
	}

	loadout := make(map[string]*common.LoadoutItem)
	for i := range loadoutItems {
		loadout[loadoutItems[i].SlotID] = &loadoutItems[i]
	}

	return loadout, nil
}

// EquipItem vybaví gear na daný slot
func (s *Service) EquipItem(userID uuid.UUID, itemID uuid.UUID, slotID string) error {
	// Skontroluj, či slot existuje
	var slot common.LoadoutSlot
	if err := s.db.Where("id = ?", slotID).First(&slot).Error; err != nil {
		return errors.New("invalid slot")
	}

	// Skontroluj, či item existuje v inventári používateľa
	var inventoryItem common.InventoryItem
	if err := s.db.Where("id = ? AND user_id = ? AND item_type = ?", itemID, userID, "gear").First(&inventoryItem).Error; err != nil {
		return errors.New("item not found in inventory")
	}

	// Skontroluj, či item má správny slot
	itemSlot, ok := inventoryItem.Properties["slot"].(string)
	if !ok || itemSlot != slotID {
		return errors.New("item cannot be equipped in this slot")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Odvybav existujúci gear v tomto slote
		if err := tx.Where("user_id = ? AND slot_id = ?", userID, slotID).Delete(&common.LoadoutItem{}).Error; err != nil {
			return err
		}

		// Vytvor nový loadout item
		loadoutItem := common.LoadoutItem{
			UserID:     userID,
			SlotID:     slotID,
			ItemID:     itemID,
			ItemType:   "gear",
			EquippedAt: time.Now(),
		}

		// Nastav durability a odolnosť z properties
		if durability, ok := inventoryItem.Properties["durability"].(float64); ok {
			loadoutItem.Durability = int(durability)
			loadoutItem.MaxDurability = int(durability)
		} else {
			loadoutItem.Durability = 100
			loadoutItem.MaxDurability = 100
		}

		// Nastav odolnosť proti nepriateľom
		if zombieRes, ok := inventoryItem.Properties["zombie_resistance"].(float64); ok {
			loadoutItem.ZombieResistance = int(zombieRes)
		}
		if banditRes, ok := inventoryItem.Properties["bandit_resistance"].(float64); ok {
			loadoutItem.BanditResistance = int(banditRes)
		}
		if soldierRes, ok := inventoryItem.Properties["soldier_resistance"].(float64); ok {
			loadoutItem.SoldierResistance = int(soldierRes)
		}
		if monsterRes, ok := inventoryItem.Properties["monster_resistance"].(float64); ok {
			loadoutItem.MonsterResistance = int(monsterRes)
		}

		// Skopíruj properties
		loadoutItem.Properties = inventoryItem.Properties

		// Ulož loadout item
		if err := tx.Create(&loadoutItem).Error; err != nil {
			return err
		}

		// Označ item ako vybavený v inventári
		inventoryItem.Properties["equipped"] = true
		inventoryItem.Properties["equipped_at"] = time.Now().Format(time.RFC3339)
		return tx.Model(&inventoryItem).Update("properties", inventoryItem.Properties).Error
	})
}

// UnequipItem odvybaví gear zo slotu
func (s *Service) UnequipItem(userID uuid.UUID, slotID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Nájdi loadout item
		var loadoutItem common.LoadoutItem
		if err := tx.Where("user_id = ? AND slot_id = ?", userID, slotID).First(&loadoutItem).Error; err != nil {
			return errors.New("no item equipped in this slot")
		}

		// Odvybav z loadout
		if err := tx.Delete(&loadoutItem).Error; err != nil {
			return err
		}

		// Označ item ako nevybavený v inventári
		var inventoryItem common.InventoryItem
		if err := tx.Where("id = ? AND user_id = ?", loadoutItem.ItemID, userID).First(&inventoryItem).Error; err != nil {
			return err
		}

		inventoryItem.Properties["equipped"] = false
		delete(inventoryItem.Properties, "equipped_at")
		return tx.Model(&inventoryItem).Update("properties", inventoryItem.Properties).Error
	})
}

// RepairItem opraví durability gearu
func (s *Service) RepairItem(userID uuid.UUID, slotID string, repairAmount int) error {
	var loadoutItem common.LoadoutItem
	if err := s.db.Where("user_id = ? AND slot_id = ?", userID, slotID).First(&loadoutItem).Error; err != nil {
		return errors.New("no item equipped in this slot")
	}

	if repairAmount <= 0 {
		// Full repair
		loadoutItem.Durability = loadoutItem.MaxDurability
	} else {
		// Partial repair
		loadoutItem.Durability = min(loadoutItem.Durability+repairAmount, loadoutItem.MaxDurability)
	}

	loadoutItem.LastRepaired = time.Now()
	return s.db.Model(&loadoutItem).Updates(map[string]interface{}{
		"durability":    loadoutItem.Durability,
		"last_repaired": loadoutItem.LastRepaired,
	}).Error
}

// GetLoadoutStats vráti štatistiky loadoutu
func (s *Service) GetLoadoutStats(userID uuid.UUID) (map[string]interface{}, error) {
	var loadoutItems []common.LoadoutItem
	if err := s.db.Where("user_id = ?", userID).Find(&loadoutItems).Error; err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_items":        len(loadoutItems),
		"total_durability":   0,
		"average_durability": 0,
		"zombie_resistance":  0,
		"bandit_resistance":  0,
		"soldier_resistance": 0,
		"monster_resistance": 0,
		"equipped_slots":     make([]string, 0),
		"damaged_items":      make([]string, 0),
		"critical_items":     make([]string, 0),
	}

	if len(loadoutItems) == 0 {
		return stats, nil
	}

	totalDurability := 0
	for _, item := range loadoutItems {
		totalDurability += item.Durability
		stats["total_durability"] = totalDurability
		stats["zombie_resistance"] = stats["zombie_resistance"].(int) + item.ZombieResistance
		stats["bandit_resistance"] = stats["bandit_resistance"].(int) + item.BanditResistance
		stats["soldier_resistance"] = stats["soldier_resistance"].(int) + item.SoldierResistance
		stats["monster_resistance"] = stats["monster_resistance"].(int) + item.MonsterResistance

		equippedSlots := stats["equipped_slots"].([]string)
		equippedSlots = append(equippedSlots, item.SlotID)
		stats["equipped_slots"] = equippedSlots

		// Skontroluj poškodenie
		if item.Durability < 50 {
			damagedItems := stats["damaged_items"].([]string)
			damagedItems = append(damagedItems, item.SlotID)
			stats["damaged_items"] = damagedItems
		}

		if item.Durability < 20 {
			criticalItems := stats["critical_items"].([]string)
			criticalItems = append(criticalItems, item.SlotID)
			stats["critical_items"] = criticalItems
		}
	}

	stats["average_durability"] = totalDurability / len(loadoutItems)
	return stats, nil
}

// GetGearCategories vráti všetky kategórie gearu
func (s *Service) GetGearCategories() ([]common.GearCategory, error) {
	var categories []common.GearCategory
	err := s.db.Where("is_active = ?", true).Order("level ASC, name ASC").Find(&categories).Error
	return categories, err
}

// ApplyDurabilityDamage aplikuje poškodenie na gear pri návšteve zóny
func (s *Service) ApplyDurabilityDamage(userID uuid.UUID, zoneDangerLevel int, zoneBiome string) error {
	var loadoutItems []common.LoadoutItem
	if err := s.db.Where("user_id = ?", userID).Find(&loadoutItems).Error; err != nil {
		return err
	}

	for _, item := range loadoutItems {
		// Vypočítaj poškodenie na základe danger level a biome
		damage := calculateDurabilityDamage(zoneDangerLevel, zoneBiome, item)

		if damage > 0 {
			item.Durability = max(0, item.Durability-damage)
			s.db.Model(&item).Update("durability", item.Durability)
		}
	}

	return nil
}

// calculateDurabilityDamage vypočítá poškodenie durability
func calculateDurabilityDamage(dangerLevel int, biome string, item common.LoadoutItem) int {
	baseDamage := dangerLevel * 2 // Základné poškodenie podľa danger level

	// Biome modifiers
	biomeModifier := 1.0
	switch biome {
	case "radioactive":
		biomeModifier = 1.5
	case "chemical":
		biomeModifier = 1.3
	case "urban":
		biomeModifier = 1.1
	case "forest":
		biomeModifier = 0.8
	case "mountain":
		biomeModifier = 0.9
	case "water":
		biomeModifier = 1.2
	case "industrial":
		biomeModifier = 1.0
	}

	// Odolnosť proti nepriateľom môže znížiť poškodenie
	resistanceBonus := 0
	if biome == "urban" || biome == "industrial" {
		resistanceBonus = item.BanditResistance / 10
	} else if biome == "radioactive" {
		resistanceBonus = item.MonsterResistance / 10
	}

	finalDamage := int(float64(baseDamage)*biomeModifier) - resistanceBonus
	return max(0, finalDamage)
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
