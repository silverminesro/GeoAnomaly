package game

import (
	"geoanomaly/internal/gameplay"
	"geoanomaly/internal/loadout"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GearService poskytuje funkcie pre správu gear predmetov
type GearService struct {
	db             *gorm.DB
	loadoutService *loadout.Service
}

// NewGearService vytvorí novú inštanciu GearService
func NewGearService(db *gorm.DB) *GearService {
	return &GearService{
		db:             db,
		loadoutService: loadout.NewService(db),
	}
}

// CreateGearItem vytvorí gear predmet na základe kategórie
func (gs *GearService) CreateGearItem(categoryID string, userID uuid.UUID) (*gameplay.InventoryItem, error) {
	// Najdi kategóriu gearu
	var category gameplay.GearCategory
	if err := gs.db.Where("id = ? AND is_active = ?", categoryID, true).First(&category).Error; err != nil {
		return nil, err
	}

	// Vytvor inventory item
	inventoryItem := gameplay.InventoryItem{
		UserID:   userID,
		ItemType: "gear",
		ItemID:   uuid.New(), // Unikátne ID pre tento konkrétny predmet
		Quantity: 1,
		Properties: gameplay.JSONB{
			"name":               category.Name,
			"description":        category.Description,
			"slot":               category.SlotID,
			"rarity":             category.Rarity,
			"level":              category.Level,
			"durability":         category.BaseDurability,
			"max_durability":     category.BaseDurability,
			"zombie_resistance":  category.BaseZombieResistance,
			"bandit_resistance":  category.BaseBanditResistance,
			"soldier_resistance": category.BaseSoldierResistance,
			"monster_resistance": category.BaseMonsterResistance,
			"biome":              category.Biome,
			"category_id":        categoryID,
			"equipped":           false,
			"acquired_at":        time.Now().Format(time.RFC3339),
		},
	}

	if err := gs.db.Create(&inventoryItem).Error; err != nil {
		return nil, err
	}

	return &inventoryItem, nil
}

// CollectGearFromZone zozbiera gear zo zóny a pridá ho do inventára
func (gs *GearService) CollectGearFromZone(zoneID uuid.UUID, userID uuid.UUID, userTier int) ([]gameplay.InventoryItem, error) {
	// Najdi gear v zóne
	var zoneGear []gameplay.Gear
	if err := gs.db.Where("zone_id = ? AND is_active = ?", zoneID, true).Find(&zoneGear).Error; err != nil {
		return nil, err
	}

	var collectedItems []gameplay.InventoryItem

	for _, gear := range zoneGear {
		// Skontroluj tier requirements
		if !gs.canCollectGear(gear, userTier) {
			continue
		}

		// Skontroluj biome access
		if gear.Biome != "" && !gs.canAccessBiome(gear.Biome, userTier) {
			continue
		}

		// Vytvor inventory item
		inventoryItem := gameplay.InventoryItem{
			UserID:   userID,
			ItemType: "gear",
			ItemID:   uuid.New(),
			Quantity: 1,
			Properties: gameplay.JSONB{
				"name":               gear.Name,
				"type":               gear.Type,
				"level":              gear.Level,
				"slot":               gs.getSlotForGearType(gear.Type),
				"rarity":             gs.getRarityForLevel(gear.Level),
				"durability":         100,
				"max_durability":     100,
				"zombie_resistance":  gs.calculateResistance(gear.Level, "zombie"),
				"bandit_resistance":  gs.calculateResistance(gear.Level, "bandit"),
				"soldier_resistance": gs.calculateResistance(gear.Level, "soldier"),
				"monster_resistance": gs.calculateResistance(gear.Level, "monster"),
				"biome":              gear.Biome,
				"equipped":           false,
				"acquired_at":        time.Now().Format(time.RFC3339),
				"zone_id":            zoneID.String(),
			},
		}

		if err := gs.db.Create(&inventoryItem).Error; err != nil {
			continue // Pokračuj s ďalšími predmetmi
		}

		collectedItems = append(collectedItems, inventoryItem)
	}

	return collectedItems, nil
}

// GetAvailableGear vráti dostupné gear kategórie pre používateľa
func (gs *GearService) GetAvailableGear(userTier int, biome string) ([]gameplay.GearCategory, error) {
	query := gs.db.Where("is_active = ?", true)

	// Filtruj podľa tier requirements
	query = query.Where("level <= ?", gs.getMaxGearLevelForTier(userTier))

	// Filtruj podľa biome ak je špecifikovaný
	if biome != "" {
		query = query.Where("(biome = ? OR biome = 'all')", biome)
	}

	var categories []gameplay.GearCategory
	err := query.Order("level ASC, name ASC").Find(&categories).Error
	return categories, err
}

// GetAvailableGearInZones vráti gear ktorý je dostupný v zónach pre daný tier a biome
func (gs *GearService) GetAvailableGearInZones(userTier int, biome string) ([]gameplay.Gear, error) {
	// Namiesto hľadania v gear tabuľke, vytvoríme gear objekty z kategórií
	// ktoré sa môžu spawnovať v zónach
	categories, err := gs.GetAvailableGear(userTier, biome)
	if err != nil {
		return nil, err
	}

	var gear []gameplay.Gear
	for _, category := range categories {
		// Vytvor gear objekt z kategórie
		gearObj := gameplay.Gear{
			BaseModel: gameplay.BaseModel{ID: uuid.New()},
			Name:      category.Name,
			Type:      category.ID, // Použijeme ID kategórie ako type
			Level:     category.Level,
			Biome:     category.Biome,
			IsActive:  true,
		}
		gear = append(gear, gearObj)
	}

	return gear, nil
}

// EquipGear vybaví gear predmet na hráča
func (gs *GearService) EquipGear(inventoryItemID uuid.UUID, userID uuid.UUID) error {
	// Najdi inventory item
	var inventoryItem gameplay.InventoryItem
	if err := gs.db.Where("id = ? AND user_id = ? AND item_type = ?", inventoryItemID, userID, "gear").First(&inventoryItem).Error; err != nil {
		return err
	}

	// Získaj slot z properties
	slot, ok := inventoryItem.Properties["slot"].(string)
	if !ok {
		return gs.loadoutService.EquipItem(userID, inventoryItem.ItemID, "body") // Default slot
	}

	// Vybav cez loadout service
	return gs.loadoutService.EquipItem(userID, inventoryItem.ItemID, slot)
}

// UnequipGear odvybaví gear zo slotu
func (gs *GearService) UnequipGear(slotID string, userID uuid.UUID) error {
	return gs.loadoutService.UnequipItem(userID, slotID)
}

// RepairGear opraví gear v danom slote
func (gs *GearService) RepairGear(slotID string, userID uuid.UUID, repairAmount int) error {
	return gs.loadoutService.RepairItem(userID, slotID, repairAmount)
}

// GetGearDisplayName vráti zobrazovací názov pre gear
func GetGearDisplayName(gearType string) string {
	displayNames := map[string]string{
		// Head gear
		"military_helmet": "Military Helmet",
		"hazmat_hood":     "Hazmat Hood",
		"tactical_cap":    "Tactical Cap",

		// Face gear
		"gas_mask":             "Gas Mask",
		"night_vision_goggles": "Night Vision Goggles",
		"tactical_sunglasses":  "Tactical Sunglasses",

		// Body gear
		"bulletproof_vest": "Bulletproof Vest",
		"hazmat_suit":      "Hazmat Suit",
		"leather_jacket":   "Leather Jacket",

		// Vest gear
		"tactical_vest":  "Tactical Vest",
		"explosive_vest": "Explosive Vest",

		// Hands gear
		"combat_gloves":   "Combat Gloves",
		"hazmat_gloves":   "Hazmat Gloves",
		"tactical_gloves": "Tactical Gloves",

		// Legs gear
		"combat_pants":   "Combat Pants",
		"hazmat_pants":   "Hazmat Pants",
		"tactical_pants": "Tactical Pants",

		// Feet gear
		"combat_boots":   "Combat Boots",
		"hazmat_boots":   "Hazmat Boots",
		"tactical_boots": "Tactical Boots",

		// Scanner gear
		"basic_scanner":    "Basic Scanner",
		"advanced_scanner": "Advanced Scanner",
		"quantum_scanner":  "Quantum Scanner",
		"artifact_scanner": "Artifact Scanner",
	}

	if name, exists := displayNames[gearType]; exists {
		return name
	}
	return gearType
}

// Helper funkcie
func (gs *GearService) canCollectGear(gear gameplay.Gear, userTier int) bool {
	maxLevel := gs.getMaxGearLevelForTier(userTier)
	return gear.Level <= maxLevel
}

func (gs *GearService) getMaxGearLevelForTier(userTier int) int {
	switch userTier {
	case 0:
		return 2 // Free tier: max level 2
	case 1:
		return 4 // Basic tier: max level 4
	case 2:
		return 6 // Premium tier: max level 6
	case 3:
		return 8 // Pro tier: max level 8
	case 4:
		return 10 // Elite tier: max level 10
	default:
		return 1
	}
}

func (gs *GearService) canAccessBiome(biome string, userTier int) bool {
	// Základné biomy sú dostupné pre všetkých
	basicBiomes := map[string]bool{
		"forest": true,
		"urban":  true,
	}

	if basicBiomes[biome] {
		return true
	}

	// Pokročilé biomy vyžadujú vyšší tier
	switch biome {
	case "mountain", "industrial":
		return userTier >= 1
	case "water", "chemical":
		return userTier >= 2
	case "radioactive":
		return userTier >= 3
	default:
		return true
	}
}

func (gs *GearService) getSlotForGearType(gearType string) string {
	slotMapping := map[string]string{
		// Head gear
		"helmet": "head",
		"cap":    "head",
		"hood":   "head",

		// Face gear
		"mask":       "face",
		"goggles":    "face",
		"sunglasses": "face",

		// Body gear
		"vest":   "body",
		"suit":   "body",
		"jacket": "body",

		// Vest gear
		"tactical_vest":  "vest",
		"explosive_vest": "vest",

		// Hands gear
		"gloves": "hands",

		// Legs gear
		"pants": "legs",

		// Feet gear
		"boots": "feet",

		// Scanner gear
		"scanner": "scanner",
	}

	if slot, exists := slotMapping[gearType]; exists {
		return slot
	}
	return "body" // Default slot
}

func (gs *GearService) getRarityForLevel(level int) string {
	switch {
	case level <= 2:
		return "common"
	case level <= 4:
		return "uncommon"
	case level <= 6:
		return "rare"
	case level <= 8:
		return "epic"
	default:
		return "legendary"
	}
}

func (gs *GearService) calculateResistance(level int, enemyType string) int {
	baseResistance := level * 2

	switch enemyType {
	case "zombie":
		return baseResistance
	case "bandit":
		return baseResistance + level
	case "soldier":
		return baseResistance + level*2
	case "monster":
		return baseResistance + level*3
	default:
		return baseResistance
	}
}

// FilterGearByTier filtruje gear podľa tier requirements
func (gs *GearService) FilterGearByTier(gear []gameplay.Gear, userTier int) []gameplay.Gear {
	var filtered []gameplay.Gear
	for _, g := range gear {
		// Check tier requirements
		if !gs.canCollectGear(g, userTier) {
			continue
		}

		// Check biome access
		if g.Biome != "" {
			if !gs.canAccessBiome(g.Biome, userTier) {
				continue
			}
		}

		filtered = append(filtered, g)
	}
	return filtered
}

// GetGearImageFilename vráti názov súboru pre gear obrázok
func GetGearImageFilename(gearType string) string {
	imageMap := map[string]string{
		// Head gear
		"tactical_cap":    "tactical_cap.jpg",
		"military_helmet": "military_helmet.jpg",
		"hazmat_hood":     "hazmat_hood.jpg",

		// Face gear
		"sunglasses": "sunglasses.jpg",
		"gas_mask":   "gas_mask.jpg",

		// Body gear
		"leather_jacket":   "leather_jacket.jpg",
		"tactical_vest":    "tactical_vest.jpg",
		"explosive_vest":   "explosive_vest.jpg",
		"bulletproof_vest": "bulletproof_vest.jpg",
		"hazmat_suit":      "hazmat_suit.jpg",

		// Hands gear
		"combat_gloves":   "combat_gloves.jpg",
		"tactical_gloves": "tactical_gloves.jpg",
		"hazmat_gloves":   "hazmat_gloves.jpg",

		// Legs gear
		"combat_pants":   "combat_pants.jpg",
		"tactical_pants": "tactical_pants.jpg",
		"hazmat_pants":   "hazmat_pants.jpg",

		// Feet gear
		"combat_boots":   "combat_boots.jpg",
		"tactical_boots": "tactical_boots.jpg",
		"hazmat_boots":   "hazmat_boots.jpg",

		// Scanner gear
		"basic_scanner":    "basic_scanner.jpg",
		"advanced_scanner": "advanced_scanner.jpg",
		"quantum_scanner":  "quantum_scanner.jpg",
		"artifact_scanner": "artifact_scanner.jpg",

		// Night vision
		"night_vision_goggles": "night_vision_goggles.jpg",
	}

	if filename, exists := imageMap[gearType]; exists {
		return filename
	}
	return "default_gear.jpg" // fallback
}
