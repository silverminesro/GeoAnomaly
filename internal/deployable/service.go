package deployable

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"geoanomaly/internal/auth"
	"geoanomaly/internal/gameplay"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Deployment distance constants
const (
	// TESTOVACIA VZDIALENOST: 30km (30000m) - len pre testovanie!
	// OSTRA PREVADZKA: 50m (50) - produkčné nastavenie
	MaxDeploymentDistanceMeters = 30000 // TODO: Zmeniť na 50 pre produkciu

	// Security threshold - ak je vzdialenosť > 50km, pravdepodobne sa pokúša obísť
	SecurityDistanceThresholdMeters = 50000
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	rand.Seed(time.Now().UnixNano())
	return &Service{
		db: db,
	}
}

// DeployDevice - umiestni zariadenie na mapu
func (s *Service) DeployDevice(userID uuid.UUID, req *DeployRequest) (*DeployResponse, error) {
	log.Printf("🔧 DeployDevice: userID=%s, deviceID=%s, batteryID=%s, lat=%.6f, lng=%.6f",
		userID, req.DeviceInventoryID, req.BatteryInventoryID, req.Latitude, req.Longitude)

	// 1. Validovať tier obmedzenia
	if err := s.validateTierLimits(userID); err != nil {
		log.Printf("❌ DeployDevice: tier validation failed: %v", err)
		return nil, err
	}

	// 2. Validovať vzdialenosť od hráča
	if err := s.validateDeploymentDistance(userID, req.Latitude, req.Longitude); err != nil {
		log.Printf("❌ DeployDevice: distance validation failed: %v", err)
		return nil, err
	}

	// 3. Validácia inventára – hráč to musí vlastniť a kusy nesmú byť v použití
	iq := NewInventoryQueries(s.db)
	if err := iq.ValidateDeploymentInventory(userID, req.DeviceInventoryID, req.BatteryInventoryID); err != nil {
		log.Printf("❌ DeployDevice: inventory validation failed: %v", err)
		return nil, err
	}

	// 4. Vytvoriť zariadenie
	device := DeployedDevice{
		ID:                 uuid.New(),
		OwnerID:            userID,
		DeviceInventoryID:  req.DeviceInventoryID,
		BatteryInventoryID: &req.BatteryInventoryID,   // Konvertovať na pointer
		BatteryStatus:      &[]string{"installed"}[0], // Nový stĺpec
		Name:               req.Name,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		DeployedAt:         time.Now().UTC(),
		IsActive:           true,
		BatteryLevel:       &[]int{100}[0],
		Status:             DeviceStatusActive,
		HackResistance:     1,        // Default hack resistance
		ScanRadiusKm:       1.0,      // Default 1km scan radius
		MaxRarityDetected:  "common", // Default common rarity detection
		Properties:         make(map[string]any),
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	// 5. Uložiť do databázy
	if err := s.db.Create(&device).Error; err != nil {
		log.Printf("❌ DeployDevice: failed to create device: %v", err)
		return nil, fmt.Errorf("failed to create deployed device: %w", err)
	}

	// 6. Odstrániť scanner a batériu z inventára (soft-delete)
	if err := s.removeScannerFromInventory(req.DeviceInventoryID); err != nil {
		log.Printf("❌ DeployDevice: failed to remove scanner: %v", err)
		// Ak sa nepodarí odstrániť scanner z inventára, odstráň aj nasadené zariadenie
		s.db.Delete(&device)
		return nil, fmt.Errorf("failed to remove scanner from inventory: %w", err)
	}

	if err := s.removeBatteryFromInventory(req.BatteryInventoryID); err != nil {
		log.Printf("❌ DeployDevice: failed to remove battery: %v", err)
		// Ak sa nepodarí odstrániť batériu z inventára, rollback všetko
		s.db.Delete(&device)
		s.restoreScannerToInventory(req.DeviceInventoryID) // Vráť scanner späť
		return nil, fmt.Errorf("failed to remove battery from inventory: %w", err)
	}

	log.Printf("✅ Deployed device %s for user %s at [%.6f, %.6f] (scanner+battery removed from inventory)", device.Name, userID, req.Latitude, req.Longitude)

	return &DeployResponse{
		Success:    true,
		DeviceID:   device.ID,
		DeviceName: device.Name,
	}, nil
}

// removeScannerFromInventory odstráni scanner z inventára po nasadení (soft delete)
func (s *Service) removeScannerFromInventory(scannerInventoryID uuid.UUID) error {
	if err := s.db.Model(&InventoryItem{}).
		Where("id = ?", scannerInventoryID).
		Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to remove scanner from inventory: %w", err)
	}
	log.Printf("✅ Removed scanner %s from inventory", scannerInventoryID)
	return nil
}

// restoreScannerToInventory obnoví scanner do inventára (soft-undelete)
func (s *Service) restoreScannerToInventory(scannerInventoryID uuid.UUID) error {
	if err := s.db.Model(&InventoryItem{}).
		Where("id = ?", scannerInventoryID).
		Update("deleted_at", nil).Error; err != nil {
		return fmt.Errorf("failed to restore scanner to inventory: %w", err)
	}
	log.Printf("✅ Restored scanner %s to inventory", scannerInventoryID)
	return nil
}

// removeBatteryFromInventory odstráni batériu z inventára po nasadení (soft delete)
func (s *Service) removeBatteryFromInventory(batteryInventoryID uuid.UUID) error {
	if err := s.db.Model(&InventoryItem{}).
		Where("id = ?", batteryInventoryID).
		Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to remove battery from inventory: %w", err)
	}
	log.Printf("✅ Removed battery %s from inventory", batteryInventoryID)
	return nil
}

// removeItemsFromInventory - DEPRECATED: Používa sa removeBatteryFromInventory
func (s *Service) removeItemsFromInventory(deviceInventoryID, batteryInventoryID uuid.UUID) error {
	// DEPRECATED: Scanner sa už nemazá z inventára po deploy
	// Používa sa removeBatteryFromInventory
	return s.removeBatteryFromInventory(batteryInventoryID)
}

// GetMyDevices - získa všetky zariadenia hráča (aktívne aj neaktívne)
func (s *Service) GetMyDevices(userID uuid.UUID) ([]DeployedDevice, error) {
	var devices []DeployedDevice
	if err := s.db.Where("owner_id = ?", userID).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to get user devices: %w", err)
	}
	return devices, nil
}

// GetMyActiveDevices - získa len aktívne zariadenia hráča
func (s *Service) GetMyActiveDevices(userID uuid.UUID) ([]DeployedDevice, error) {
	var devices []DeployedDevice
	if err := s.db.Where("owner_id = ? AND is_active = true", userID).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to get user active devices: %w", err)
	}
	return devices, nil
}

// GetDeviceDetails - získa detaily zariadenia
func (s *Service) GetDeviceDetails(deviceID uuid.UUID, userID uuid.UUID) (*DeployedDevice, error) {
	var device DeployedDevice
	if err := s.db.Where("id = ? AND owner_id = ?", deviceID, userID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("device not found or not owned by user")
		}
		return nil, fmt.Errorf("failed to get device details: %w", err)
	}
	return &device, nil
}

// ScanDeployableDevice - skenuje deployable zariadenie
func (s *Service) ScanDeployableDevice(userID uuid.UUID, deviceID uuid.UUID, req *DeployableScanRequest) (*DeployableScanResponse, error) {
	// 1. Získať polohu hráča z player_sessions
	var session auth.PlayerSession
	if err := s.db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("hráč nemá aktívnu session")
	}

	// 2. Získať zariadenie (bez owner filtra, iba aktívne)
	var device DeployedDevice
	if err := s.db.Where("id = ? AND is_active = true", deviceID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("zariadenie nebolo nájdené alebo nie je aktívne")
	}

	// 2b. Overiť oprávnenie na sken: owner alebo platný záznam v gameplay.device_access
	if device.OwnerID != userID {
		var count int64
		err := s.db.
			Table("gameplay.device_access").
			Where("device_id = ? AND user_id = ?", deviceID, userID).
			Where("(access_level IN ('owner','permanent') OR (access_level = 'temporary' AND expires_at > NOW()))").
			Where("(is_device_disabled = FALSE OR (is_device_disabled = TRUE AND disabled_until <= NOW()))").
			Limit(1).
			Count(&count).Error
		if err != nil {
			return nil, fmt.Errorf("chyba pri kontrole prístupu: %w", err)
		}
		if count == 0 {
			return nil, fmt.Errorf("nemáš prístup na skenovanie tohto zariadenia")
		}
	}

	// 3. Vypočítať vzdialenosť medzi hráčom a zariadením
	distance := s.calculateDistance(
		session.LastLocationLatitude,
		session.LastLocationLongitude,
		device.Latitude,
		device.Longitude,
	)

	// 4. Validovať vzdialenosť (20km limit)
	if distance > 20000 {
		return &DeployableScanResponse{
			Success: false,
			Message: fmt.Sprintf("Žiadny signál - príliš ďaleko od zariadenia (%dkm)", distance/1000),
		}, nil
	}

	// 5. Skontrolovať cooldown
	if err := s.validateScanCooldown(userID, deviceID); err != nil {
		return nil, err
	}

	// 5b. Skontrolovať stav batérie
	if device.BatteryInventoryID == nil || device.BatteryLevel == nil || *device.BatteryLevel <= 0 {
		return &DeployableScanResponse{
			Success: false,
			Message: "Scanner nemá batériu alebo je vybitá - skenovanie nie je možné",
		}, nil
	}

	// 5c. Skontrolovať battery_status (ak je definovaný)
	if device.BatteryStatus != nil && (*device.BatteryStatus == "removed" || *device.BatteryStatus == "depleted") {
		return &DeployableScanResponse{
			Success: false,
			Message: "Scanner má vybratú alebo vybitú batériu - skenovanie nie je možné",
		}, nil
	}

	// 6. Nájsť najbližšie zóny v okolí zariadenia – použijeme skutočný dosah zariadenia
	// Ensure default values for scan radius
	scanRadiusKm := device.ScanRadiusKm
	if scanRadiusKm <= 0 {
		scanRadiusKm = 1.0 // Default 1km
	}

	zones, err := s.findNearbyZones(device.Latitude, device.Longitude, scanRadiusKm)
	if err != nil {
		return nil, fmt.Errorf("chyba pri hľadaní zón: %w", err)
	}

	// 7. Načítať artefakty zo všetkých nájdených zón
	var allArtifacts []gameplay.Artifact
	for _, zone := range zones {
		artifacts, err := s.getZoneArtifacts(zone.ID.String())
		if err != nil {
			continue // Preskočiť zónu ak je chyba
		}
		allArtifacts = append(allArtifacts, artifacts...)
	}

	// 8. Filtrovať artefakty podľa scanner schopností
	scanResults := s.filterArtifactsByScanner(allArtifacts, req, &device)

	// 8b. BATTERY CONSUMPTION DURING SCANNING (CURRENTLY DISABLED)
	// NOTE: We have implemented 5% battery consumption per scan, but it's currently disabled
	// because we now use passive battery drain via scheduler instead.
	// If you want to re-enable this, uncomment the following code:
	/*
		// 8b. Znížiť battery level (5% za scan)
		batteryConsumption := 5
		newBatteryLevel := *device.BatteryLevel - batteryConsumption
		if newBatteryLevel < 0 {
			newBatteryLevel = 0
		}

		// Aktualizovať battery level v databáze
		if err := s.db.Model(&device).Update("battery_level", newBatteryLevel).Error; err != nil {
			log.Printf("⚠️ Failed to update battery level for device %s: %v", deviceID, err)
		} else {
			log.Printf("🔋 Battery consumed: device %s (%s) %d%% → %d%% (-%d%%)",
				deviceID, device.Name, *device.BatteryLevel, newBatteryLevel, batteryConsumption)
		}
	*/

	// 9. Aktualizovať cooldown (5 minút) a vypočítať koniec cooldownu
	cooldownSeconds := 300
	if err := s.updateScanCooldown(userID, deviceID, cooldownSeconds); err != nil {
		return nil, fmt.Errorf("chyba pri uložení cooldownu: %w", err)
	}
	cooldownUntil := time.Now().UTC().Add(time.Duration(cooldownSeconds) * time.Second)

	// 10. Vrátiť výsledok so správnym CooldownUntil
	return &DeployableScanResponse{
		Success:       true,
		ItemsFound:    len(scanResults),
		ScanResults:   scanResults,
		CooldownUntil: &cooldownUntil,
	}, nil
}

// HackDevice - hackuje zariadenie
func (s *Service) HackDevice(hackerID uuid.UUID, deviceID uuid.UUID, req *HackRequest) (*HackResponse, error) {
	// 1. Validovať vzdialenosť a získať session pre výpočet vzdialenosti
	var session auth.PlayerSession
	if err := s.db.Where("user_id = ?", hackerID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("hacker nemá aktívnu session")
	}

	// 2. Získať zariadenie
	var device DeployedDevice
	if err := s.db.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("zariadenie nebolo nájdené")
	}

	// 3. Vypočítať skutočnú vzdialenosť
	distance := s.calculateDistance(
		session.LastLocationLatitude,
		session.LastLocationLongitude,
		device.Latitude,
		device.Longitude,
	)

	// 4. Validovať vzdialenosť (50m pre hack)
	if distance > 50 {
		return nil, fmt.Errorf("príliš ďaleko od zariadenia (%dm)", distance)
	}

	// 5. Získať a validovať hackovací nástroj z inventory_items
	var inventoryItem gameplay.InventoryItem
	if err := s.db.Where("id = ? AND user_id = ? AND item_type = ? AND deleted_at IS NULL",
		req.HackToolID, hackerID, "hack_tool").First(&inventoryItem).Error; err != nil {
		return nil, fmt.Errorf("hackovací nástroj nebol nájdený v inventári")
	}

	// Extrahuj uses_left z properties
	usesLeft := 0
	if ul, ok := inventoryItem.Properties["uses_left"].(float64); ok {
		usesLeft = int(ul)
	}

	// Validovať uses_left
	if usesLeft <= 0 {
		return nil, fmt.Errorf("hackovací nástroj nemá žiadne zostávajúce použitia")
	}

	// Extrahuj tool_type z properties
	toolType := "circuit_breaker" // default
	if tt, ok := inventoryItem.Properties["tool_type"].(string); ok {
		switch tt {
		case "basic_hack":
			toolType = "circuit_breaker"
		case "advanced_hack":
			toolType = "code_cracker"
		case "device_claimer":
			toolType = "stealth_infiltration"
		case "stealth_hack":
			toolType = "stealth_infiltration"
		default:
			toolType = tt
		}
	}

	// Vytvor HackTool štruktúru pre kompatibilitu
	hackTool := HackTool{
		ID:         inventoryItem.ID,
		UserID:     inventoryItem.UserID,
		ToolType:   toolType,
		UsesLeft:   usesLeft,
		Properties: datatypes.JSONMap(inventoryItem.Properties),
	}

	// 6. Výsledok minihry z frontendu
	// Frontend spustil minihru a poslal výsledok (success/fail)
	success := req.MinigameSuccess
	minigameType := req.MinigameType
	if minigameType == "" {
		minigameType = hackTool.ToolType // Fallback na tool type
	}

	// 7. Validácia minigame duration (anti-cheat - minihra musí trvať aspoň 3 sekundy)
	if req.MinigameDuration < 3 {
		log.Printf("⚠️ Suspicious minigame duration: %d seconds for user %s", req.MinigameDuration, hackerID)
		// Môžeš to odmietnuť alebo len logovaťpre monitoring
		// return nil, fmt.Errorf("minihra musí trvať minimálne 3 sekundy")
	}

	// 8. Zaznamenať hack s minigame dátami
	hack := DeviceHack{
		ID:               uuid.New(),
		DeviceID:         deviceID,
		HackerID:         hackerID,
		HackTime:         time.Now().UTC(),
		Success:          success,
		HackToolUsed:     hackTool.ToolType,
		DistanceM:        float64(distance),
		HackDurationSec:  req.MinigameDuration, // Trvanie minihry
		MinigameType:     minigameType,
		MinigameScore:    req.MinigameScore,
		MinigameDuration: req.MinigameDuration,
		Properties:       make(map[string]any),
		CreatedAt:        time.Now().UTC(),
	}

	s.db.Create(&hack)

	log.Printf("🎮 Hack attempt: user=%s, device=%s, minigame=%s, success=%v, score=%d, duration=%ds",
		hackerID, deviceID, minigameType, success, req.MinigameScore, req.MinigameDuration)

	// Spotrebovať hack tool (znížiť uses_left alebo vymazať ak 0)
	newUsesLeft := usesLeft - 1
	if newUsesLeft < 0 {
		newUsesLeft = 0
	}

	var result *gorm.DB
	if newUsesLeft == 0 {
		// Ak uses_left dosiahne 0, vymaž item z inventára (soft delete)
		result = s.db.Exec(`
			UPDATE gameplay.inventory_items
			SET deleted_at = NOW(),
			    updated_at = NOW()
			WHERE id = ?
		`, hackTool.ID)
		log.Printf("🗑️ Hack tool deleted (uses depleted): %s, uses: %d → 0 (deleted)", hackTool.ID, usesLeft)
	} else {
		// Inak len zníž uses_left
		result = s.db.Exec(`
			UPDATE gameplay.inventory_items
			SET properties = jsonb_set(properties, '{uses_left}', to_jsonb(?::int), true),
			    updated_at = NOW()
			WHERE id = ?
		`, newUsesLeft, hackTool.ID)
		log.Printf("🔧 Hack tool consumed: %s, uses: %d → %d", hackTool.ID, usesLeft, newUsesLeft)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("chyba pri spotrebovaní nástroja: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("nástroj už neexistuje v inventári")
	}

	if !success {
		return &HackResponse{Success: false}, nil
	}

	// 6. Spracovať výsledok hacku
	if device.Status == DeviceStatusAbandoned {
		// Opustené zariadenie - claim s minigame (už prebehla vo Flutteri)
		hackResponse, err := s.claimAbandonedDevice(hackerID, deviceID, &hackTool)
		if err != nil {
			return nil, err
		}

		log.Printf("🎯 Abandoned device claimed via hack minigame: device=%s, new_owner=%s", deviceID, hackerID)

		return hackResponse, nil
	} else {
		// Funkčné zariadenie - prístup na 24h
		return s.grantDeviceAccess(hackerID, deviceID, &hackTool)
	}
}

// ClaimAbandonedDevice - claimne opustené zariadenie
func (s *Service) ClaimAbandonedDevice(hackerID uuid.UUID, deviceID uuid.UUID, req *ClaimRequest) (*ClaimResponse, error) {
	// 1. Získať session pre výpočet vzdialenosti
	var session auth.PlayerSession
	if err := s.db.Where("user_id = ?", hackerID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("hacker nemá aktívnu session")
	}

	// 2. Získať zariadenie
	var device DeployedDevice
	if err := s.db.Where("id = ? AND status = ?", deviceID, DeviceStatusAbandoned).First(&device).Error; err != nil {
		return nil, fmt.Errorf("opustené zariadenie nebolo nájdené")
	}

	// 3. Vypočítať skutočnú vzdialenosť
	distance := s.calculateDistance(
		session.LastLocationLatitude,
		session.LastLocationLongitude,
		device.Latitude,
		device.Longitude,
	)

	// 4. Validovať vzdialenosť (50m pre claim)
	if distance > 50 {
		return nil, fmt.Errorf("príliš ďaleko od zariadenia (%dm)", distance)
	}

	// 5. Získať a validovať hackovací nástroj z inventory_items
	var inventoryItem gameplay.InventoryItem
	if err := s.db.Where("id = ? AND user_id = ? AND item_type = ? AND deleted_at IS NULL",
		req.HackToolID, hackerID, "hack_tool").First(&inventoryItem).Error; err != nil {
		return nil, fmt.Errorf("hackovací nástroj nebol nájdený v inventári")
	}

	// Extrahuj uses_left z properties
	usesLeft := 0
	if ul, ok := inventoryItem.Properties["uses_left"].(float64); ok {
		usesLeft = int(ul)
	}

	// Validovať uses_left
	if usesLeft <= 0 {
		return nil, fmt.Errorf("hackovací nástroj nemá žiadne zostávajúce použitia")
	}

	// Extrahuj tool_type z properties
	toolType := "stealth_infiltration" // default pre claim
	if tt, ok := inventoryItem.Properties["tool_type"].(string); ok {
		switch tt {
		case "basic_hack":
			toolType = "circuit_breaker"
		case "advanced_hack":
			toolType = "code_cracker"
		case "device_claimer":
			toolType = "stealth_infiltration"
		case "stealth_hack":
			toolType = "stealth_infiltration"
		default:
			toolType = tt
		}
	}

	// Vytvor HackTool štruktúru pre kompatibilitu
	hackTool := HackTool{
		ID:         inventoryItem.ID,
		UserID:     inventoryItem.UserID,
		ToolType:   toolType,
		UsesLeft:   usesLeft,
		Properties: datatypes.JSONMap(inventoryItem.Properties),
	}

	// 6. Výsledok minihry z frontendu (rovnako ako pri hacku)
	success := req.MinigameSuccess
	minigameType := req.MinigameType
	if minigameType == "" {
		minigameType = "ip_hacker" // Default pre claim
	}

	// 7. Validácia minigame duration (anti-cheat - claim minihra max 30 sekúnd)
	if req.MinigameDuration > 30 {
		log.Printf("⚠️ Suspicious claim minigame duration: %d seconds (max 30s) for user %s", req.MinigameDuration, hackerID)
		return nil, fmt.Errorf("claim minihra trvala príliš dlho (max 30s)")
	}
	if req.MinigameDuration < 3 {
		log.Printf("⚠️ Suspicious claim minigame duration: %d seconds (min 3s) for user %s", req.MinigameDuration, hackerID)
	}

	// 8. Ak minihra nebola úspešná, neclaimuj
	if !success {
		log.Printf("❌ Claim failed - minihra neúspešná pre user %s, device %s", hackerID, deviceID)
		
		// ✨ Odpocítaj uses_left aj pri FAILED claim
		usesLeft := 0
		if ul, ok := hackTool.Properties["uses_left"].(float64); ok {
			usesLeft = int(ul)
		}
		
		newUsesLeft := usesLeft - 1
		if newUsesLeft < 0 {
			newUsesLeft = 0
		}
		
		var result *gorm.DB
		if newUsesLeft == 0 {
			// Ak uses_left dosiahne 0, vymaž item z inventára (soft delete)
			result = s.db.Exec(`
				UPDATE gameplay.inventory_items
				SET deleted_at = NOW(),
					updated_at = NOW()
				WHERE id = ?
			`, hackTool.ID)
			log.Printf("🗑️ Claim tool deleted (uses depleted, failed): %s, uses: %d → 0 (deleted)", hackTool.ID, usesLeft)
		} else {
			// Inak len zníž uses_left
			result = s.db.Exec(`
				UPDATE gameplay.inventory_items
				SET properties = jsonb_set(properties, '{uses_left}', to_jsonb(?::int), true),
					updated_at = NOW()
				WHERE id = ?
			`, newUsesLeft, hackTool.ID)
			log.Printf("🔧 Claim tool consumed (failed): %s, uses: %d → %d", hackTool.ID, usesLeft, newUsesLeft)
		}
		
		if result.Error != nil {
			log.Printf("❌ Chyba pri spotrebovaní claim tool: %v", result.Error)
		}
		
		return &ClaimResponse{Success: false}, nil
	}

	// 9. Claim zariadenie (minihra bola úspešná)
	hackResponse, err := s.claimAbandonedDevice(hackerID, deviceID, &hackTool)
	if err != nil {
		return nil, err
	}

	// 10. Zaznamenať claim pokus s minigame dátami
	hack := DeviceHack{
		ID:               uuid.New(),
		DeviceID:         deviceID,
		HackerID:         hackerID,
		HackTime:         time.Now().UTC(),
		Success:          hackResponse.Success,
		HackToolUsed:     hackTool.ToolType,
		DistanceM:        float64(distance),
		HackDurationSec:  req.MinigameDuration,
		MinigameType:     minigameType,
		MinigameScore:    req.MinigameScore,
		MinigameDuration: req.MinigameDuration,
		Properties:       make(map[string]any),
		CreatedAt:        time.Now().UTC(),
	}
	s.db.Create(&hack)

	log.Printf("🎮 Claim attempt: user=%s, device=%s, minigame=%s, success=%v, score=%d, duration=%ds",
		hackerID, deviceID, minigameType, success, req.MinigameScore, req.MinigameDuration)

	// Spotrebovať hack tool (znížiť uses_left alebo vymazať ak 0)
	newUsesLeft := usesLeft - 1
	if newUsesLeft < 0 {
		newUsesLeft = 0
	}

	var result *gorm.DB
	if newUsesLeft == 0 {
		// Ak uses_left dosiahne 0, vymaž item z inventára (soft delete)
		result = s.db.Exec(`
			UPDATE gameplay.inventory_items
			SET deleted_at = NOW(),
			    updated_at = NOW()
			WHERE id = ?
		`, hackTool.ID)
		log.Printf("🗑️ Hack tool deleted (uses depleted, claim): %s, uses: %d → 0 (deleted)", hackTool.ID, usesLeft)
	} else {
		// Inak len zníž uses_left
		result = s.db.Exec(`
			UPDATE gameplay.inventory_items
			SET properties = jsonb_set(properties, '{uses_left}', to_jsonb(?::int), true),
			    updated_at = NOW()
			WHERE id = ?
		`, newUsesLeft, hackTool.ID)
		log.Printf("🔧 Hack tool consumed (claim): %s, uses: %d → %d", hackTool.ID, usesLeft, newUsesLeft)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("chyba pri spotrebovaní nástroja: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("nástroj už neexistuje v inventári")
	}

	// Convert HackResponse to ClaimResponse
	return &ClaimResponse{
		Success:         hackResponse.Success,
		NewOwnerID:      *hackResponse.NewOwnerID,
		BatteryReplaced: hackResponse.BatteryReplaced,
		DeviceStatus:    hackResponse.DeviceStatus,
	}, nil
}

// GetCooldownStatus - získa status cooldownu
func (s *Service) GetCooldownStatus(userID uuid.UUID, deviceID uuid.UUID) (*CooldownStatus, error) {
	// Najprv skontroluj stav zariadenia a batérie
	var device DeployedDevice
	if err := s.db.Where("id = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &CooldownStatus{CanScan: false, Reason: "device_not_found"}, nil
		}
		return nil, fmt.Errorf("chyba pri načítaní zariadenia")
	}

	// Ak zariadenie nie je v stave na skenovanie, vráť dôvod
	if device.Status == DeviceStatusAbandoned {
		return &CooldownStatus{CanScan: false, Reason: "abandoned"}, nil
	}
	if device.BatteryInventoryID == nil || device.BatteryLevel == nil || *device.BatteryLevel <= 0 {
		return &CooldownStatus{CanScan: false, Reason: "battery_depleted"}, nil
	}
	if device.BatteryStatus != nil && (*device.BatteryStatus == "removed" || *device.BatteryStatus == "depleted") {
		return &CooldownStatus{CanScan: false, Reason: "battery_removed"}, nil
	}

	// Potom kontrola cooldownu – chýbajúci záznam znamená, že je možné skenovať
	var cooldown ScanCooldown
	err := s.db.Where("device_id = ? AND user_id = ?", deviceID, userID).First(&cooldown).Error
	if err == gorm.ErrRecordNotFound {
		return &CooldownStatus{CanScan: true}, nil
	}
	if err != nil {
		// potlačiť detailnú chybu do generickej, aby nezahlcovalo logy
		return nil, fmt.Errorf("chyba pri kontrole cooldownu")
	}

	now := time.Now().UTC()
	canScan := now.After(cooldown.CooldownUntil)
	remainingSeconds := 0
	if !canScan {
		remainingSeconds = int(time.Until(cooldown.CooldownUntil).Seconds())
	}
	var until *time.Time
	if !canScan {
		until = &cooldown.CooldownUntil
	}
	return &CooldownStatus{
		CanScan:             canScan,
		CooldownUntil:       until,
		RemainingSeconds:    remainingSeconds,
		CooldownDurationSec: cooldown.CooldownDurationSec,
	}, nil
}

// GetNearbyDevices - získa zariadenia v okolí s geografickým filtrom
func (s *Service) GetNearbyDevices(userID uuid.UUID, lat, lng float64, radiusM int) (*DeviceListResponse, error) {
	// Získať vlastné zariadenia
	myDevices, err := s.GetMyDevices(userID)
	if err != nil {
		return nil, err
	}

	// PostGIS query pre aktívne zariadenia v okolí
	q := `
		SELECT *
		FROM gameplay.deployed_devices
		WHERE is_active = true
		  AND status = ?
		  AND ST_DWithin(
		        location,
		        ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		        ?
		  )
		  AND owner_id != ?
		ORDER BY location <-> ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		LIMIT 200
	`
	var nearbyDevices []DeployedDevice
	if err := s.db.Raw(q, DeviceStatusActive, lng, lat, radiusM, userID, lng, lat).Scan(&nearbyDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to get nearby devices: %w", err)
	}

	// PostGIS query pre opustené zariadenia v okolí
	var abandonedDevices []DeployedDevice
	if err := s.db.Raw(q, DeviceStatusAbandoned, lng, lat, radiusM, userID, lng, lat).Scan(&abandonedDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to get abandoned devices: %w", err)
	}

	return &DeviceListResponse{
		MyDevices:        myDevices,
		NearbyDevices:    nearbyDevices,
		AbandonedDevices: abandonedDevices,
	}, nil
}

// GetAbandonedDevicesInRadius - získa iba opustené zariadenia v okolí
func (s *Service) GetAbandonedDevicesInRadius(userID uuid.UUID, lat, lng float64, radiusM int) ([]DeployedDevice, error) {
	// PostGIS query pre opustené zariadenia v okolí
	q := `
		SELECT *
		FROM gameplay.deployed_devices
		WHERE is_active = true
		  AND status = ?
		  AND ST_DWithin(
		        location,
		        ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		        ?
		  )
		  AND owner_id != ?
		ORDER BY location <-> ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		LIMIT 200
	`
	var abandonedDevices []DeployedDevice
	if err := s.db.Raw(q, DeviceStatusAbandoned, lng, lat, radiusM, userID, lng, lat).Scan(&abandonedDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to get abandoned devices: %w", err)
	}

	return abandonedDevices, nil
}

// DeleteDevice - odstráni zariadenie (iba vlastník). Obnoví batériu späť do inventára (soft-delete -> NULL) a zmaže zariadenie.
func (s *Service) DeleteDevice(userID uuid.UUID, deviceID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Načítať zariadenie a overiť vlastníctvo
		var device DeployedDevice
		if err := tx.Where("id = ? AND owner_id = ?", deviceID, userID).First(&device).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("device not found or not owned by user")
			}
			return fmt.Errorf("failed to load device: %w", err)
		}

		// Obnov scanner do inventára (soft-undelete)
		if err := tx.Exec(`
			UPDATE gameplay.inventory_items
			SET deleted_at = NULL,
				updated_at = NOW()
			WHERE id = ?
		`, device.DeviceInventoryID).Error; err != nil {
			return fmt.Errorf("failed to restore scanner in inventory: %w", err)
		}
		log.Printf("🔧 Scanner %s restored to inventory for user %s", device.DeviceInventoryID, userID)

		// Ak má pripojenú batériu, obnov ju do inventára (zruš soft delete) a nastav charge_pct podľa battery_level (alebo 0%)
		if device.BatteryInventoryID != nil {
			level := 0
			if device.BatteryLevel != nil {
				level = *device.BatteryLevel
				if level < 0 {
					level = 0
				}
				if level > 100 {
					level = 100
				}
			}
			if err := tx.Exec(`
                UPDATE gameplay.inventory_items
                SET deleted_at = NULL,
                    properties = jsonb_set(COALESCE(properties,'{}'::jsonb), '{charge_pct}', to_jsonb(?)::jsonb, true),
                    updated_at = NOW()
                WHERE id = ?
            `, level, *device.BatteryInventoryID).Error; err != nil {
				return fmt.Errorf("failed to restore battery in inventory: %w", err)
			}
			log.Printf("🔋 Battery %s restored to inventory for user %s with %d%% charge", *device.BatteryInventoryID, userID, level)
		}

		// Zmazať záznam zariadenia (kaskádne zmaže prístupy, cooldowny, históriu)
		if err := tx.Delete(&DeployedDevice{}, "id = ?", deviceID).Error; err != nil {
			return fmt.Errorf("failed to delete device: %w", err)
		}
		log.Printf("🗑️ Deployed device %s (%s) deleted by user %s", deviceID, device.Name, userID)

		return nil
	})
}

// Helper functions

func (s *Service) validateTierLimits(userID uuid.UUID) error {
	// TODO: Implement tier validation
	// Get user tier and check device limits
	_ = userID // Suppress unused parameter warning
	return nil
}

func (s *Service) validateDeploymentDistance(userID uuid.UUID, deviceLat, deviceLng float64) error {
	// Získať aktuálnu polohu hráča z existujúcej player_sessions tabuľky
	var session auth.PlayerSession
	if err := s.db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		return fmt.Errorf("hráč nemá aktívnu session")
	}

	// Vypočítať vzdialenosť medzi hráčom a miestom deployu
	distance := s.calculateDistance(
		session.LastLocationLatitude,
		session.LastLocationLongitude,
		deviceLat,
		deviceLng,
	)

	// Validovať vzdialenosť pre deployment
	if distance > MaxDeploymentDistanceMeters {
		// Detekcia pokusu o obchádzanie limitu
		if distance > SecurityDistanceThresholdMeters {
			log.Printf("🚨 SECURITY: User %s attempted to deploy device at suspicious distance: %dm (current location: [%.6f, %.6f], deploy location: [%.6f, %.6f])",
				userID, distance, session.LastLocationLatitude, session.LastLocationLongitude, deviceLat, deviceLng)
			return fmt.Errorf("bezpečnostné obmedzenie: príliš veľká vzdialenosť (%dm). Kontaktujte administrátora", distance)
		}
		return fmt.Errorf("príliš ďaleko od aktuálnej polohy (%dm). Maximálna povolená vzdialenosť: %dm", distance, MaxDeploymentDistanceMeters)
	}

	return nil
}

// validateInventoryItems - DEPRECATED: Používa sa ValidateDeploymentInventory z inventory_queries.go
func (s *Service) validateInventoryItems(userID uuid.UUID, deviceID, batteryID uuid.UUID) error {
	// DEPRECATED: Táto funkcia je nahradená ValidateDeploymentInventory z inventory_queries.go
	// Zachováva sa pre kompatibilitu, ale nemala by sa používať
	iq := NewInventoryQueries(s.db)
	return iq.ValidateDeploymentInventory(userID, deviceID, batteryID)
}

// removeFromInventory - DEPRECATED: Items sa už nemazajú z inventára po deploy
func (s *Service) removeFromInventory(userID, itemID uuid.UUID) error {
	// DEPRECATED: Items sa už nemazajú z inventára po deploy
	// "V používaní" je dané referenciou v deployed_devices a vynútené UNIQUE indexom
	log.Printf("⚠️ removeFromInventory called but items are no longer removed from inventory after deploy")
	return nil
}

func (s *Service) validateScanCooldown(userID uuid.UUID, deviceID uuid.UUID) error {
	var cooldown ScanCooldown
	err := s.db.Where("device_id = ? AND user_id = ?", deviceID, userID).First(&cooldown).Error

	if err == gorm.ErrRecordNotFound {
		// Prvý sken - žiadny cooldown
		return nil
	}

	if err != nil {
		return fmt.Errorf("chyba pri kontrole cooldownu")
	}

	// Skontrolovať či cooldown vypršal
	if time.Now().UTC().Before(cooldown.CooldownUntil) {
		remainingTime := time.Until(cooldown.CooldownUntil)
		return fmt.Errorf("musíš počkať %v pred ďalším skenovaním", remainingTime.Round(time.Second))
	}

	return nil
}

func (s *Service) updateScanCooldown(userID uuid.UUID, deviceID uuid.UUID, cooldownSeconds int) error {
	cooldownUntil := time.Now().UTC().Add(time.Duration(cooldownSeconds) * time.Second)

	// Upsert cooldown record
	result := s.db.Exec(`
		INSERT INTO gameplay.scan_cooldowns (device_id, user_id, last_scan_at, cooldown_until, cooldown_duration_seconds, updated_at)
		VALUES (?, ?, NOW(), ?, ?, NOW())
		ON CONFLICT (device_id, user_id) 
		DO UPDATE SET 
			last_scan_at = NOW(),
			cooldown_until = EXCLUDED.cooldown_until,
			cooldown_duration_seconds = EXCLUDED.cooldown_duration_seconds,
			updated_at = NOW()
	`, deviceID, userID, cooldownUntil, cooldownSeconds)

	return result.Error
}

func (s *Service) findNearbyZones(deviceLat, deviceLng float64, scanRadiusKm float64) ([]gameplay.Zone, error) {
	var zones []gameplay.Zone

	// SQL query s geografickým výpočtom vzdialenosti
	query := `
		SELECT *, 
		       ST_Distance(
		           ST_SetSRID(ST_MakePoint(location_longitude, location_latitude), 4326)::geography,
		           ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		       ) as distance_m
		FROM gameplay.zones
		WHERE is_active = true 
		  AND ST_DWithin(
		      ST_SetSRID(ST_MakePoint(location_longitude, location_latitude), 4326)::geography,
		      ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		      ?
		  )
		ORDER BY distance_m ASC
	`

	radiusMeters := scanRadiusKm * 1000
	err := s.db.Raw(query, deviceLng, deviceLat, deviceLng, deviceLat, radiusMeters).Scan(&zones).Error

	return zones, err
}

func (s *Service) getZoneArtifacts(zoneID string) ([]gameplay.Artifact, error) {
	var artifacts []gameplay.Artifact
	err := s.db.Where("zone_id = ? AND is_active = true AND is_claimed = false", zoneID).Find(&artifacts).Error
	return artifacts, err
}

func (s *Service) filterArtifactsByScanner(artifacts []gameplay.Artifact, req *DeployableScanRequest, device *DeployedDevice) []DeployableScanResult {
	_ = req // Suppress unused parameter warning
	var results []DeployableScanResult

	// Ensure default values for scan radius and max rarity
	scanRadiusKm := device.ScanRadiusKm
	if scanRadiusKm <= 0 {
		scanRadiusKm = 1.0 // Default 1km
	}

	maxRarityDetected := device.MaxRarityDetected
	if maxRarityDetected == "" {
		maxRarityDetected = "common" // Default common rarity
	}

	// Konvertovať scan radius z km na metre (podľa capability zariadenia)
	maxRangeM := int(scanRadiusKm * 1000)

	for _, artifact := range artifacts {
		// Vypočítať vzdialenosť od zariadenia k artefaktu
		distance := s.calculateDistance(
			device.Latitude,
			device.Longitude,
			artifact.Location.Latitude,
			artifact.Location.Longitude,
		)

		// Skontrolovať či je v dosahu scanneru
		if distance > maxRangeM {
			continue
		}

		// Skontrolovať či scanner dokáže detekovať túto raritu
		if !s.canDetectRarity(artifact.Rarity, maxRarityDetected) {
			continue
		}

		// Vytvoriť scan result
		result := DeployableScanResult{
			Type:           "artifact",
			Name:           artifact.Name,
			Rarity:         artifact.Rarity,
			DistanceM:      distance,
			BearingDeg:     s.calculateBearing(device.Latitude, device.Longitude, artifact.Location.Latitude, artifact.Location.Longitude),
			SignalStrength: s.calculateSignalStrength(distance, maxRangeM),
		}

		results = append(results, result)
	}

	return results
}

// canDetectRarity - skontroluje či scanner dokáže detekovať danú raritu
func (s *Service) canDetectRarity(artifactRarity, maxRarity string) bool {
	// Rarity hierarchy (higher number = higher rarity)
	rarityLevels := map[string]int{
		"common":    1,
		"rare":      2,
		"epic":      3,
		"legendary": 4,
	}

	artifactLevel, artifactExists := rarityLevels[artifactRarity]
	maxLevel, maxExists := rarityLevels[maxRarity]

	if !artifactExists || !maxExists {
		return false // Unknown rarity
	}

	// Scanner can detect artifacts with rarity level <= max detected level
	return artifactLevel <= maxLevel
}

// performHack - DEPRECATED: Táto funkcia už nie je používaná
// Hack úspešnosť je teraz určená minihrami vo Flutteri, nie RNG výpočtom
// Zachované pre historické účely a možné budúce použitie (napr. AI hackery)
func (s *Service) performHack(device *DeployedDevice, hackTool *HackTool) bool {
	// Base success rate
	base := 0.5 // 50% default

	// Allow tools to override base via "success_rate"
	if sr, ok := hackTool.Properties["success_rate"].(float64); ok {
		base = sr
	}

	// Hack resistance penalty (-5% per resistance level)
	resPenalty := float64(device.HackResistance) * 0.05

	// Optional additive bonus from "success_bonus"
	toolBonus := 0.0
	if bonus, ok := hackTool.Properties["success_bonus"].(float64); ok {
		toolBonus = bonus
	}

	// Calculate final success probability
	successRate := base - resPenalty + toolBonus

	// Ensure minimum 5% and maximum 95% success rate
	successRate = math.Max(0.05, math.Min(0.95, successRate))

	return rand.Float64() < successRate
}

func (s *Service) grantDeviceAccess(hackerID uuid.UUID, deviceID uuid.UUID, hackTool *HackTool) (*HackResponse, error) {
	_ = hackTool // Suppress unused parameter warning
	// Získať zariadenie pre kontrolu cooldownu
	var device DeployedDevice
	if err := s.db.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("zariadenie nebolo nájdené")
	}

	// Kontrola cooldownu pre disable (max raz za 24h)
	isDisabled := false
	var disabledUntil *time.Time

	if device.LastDisabledAt == nil || time.Since(*device.LastDisabledAt) > 24*time.Hour {
		// Môže byť disabled - 25% šanca
		isDisabled = rand.Float64() < 0.25
		if isDisabled {
			dt := time.Now().UTC().Add(24 * time.Hour)
			disabledUntil = &dt

			// Aktualizovať last_disabled_at
			s.db.Model(&device).Update("last_disabled_at", time.Now().UTC())
		}
	}

	accessUntil := time.Now().UTC().Add(24 * time.Hour)

	// Vytvoriť prístupový záznam
	access := DeviceAccess{
		DeviceID:         deviceID,
		UserID:           hackerID,
		GrantedAt:        time.Now().UTC(),
		ExpiresAt:        accessUntil,
		AccessLevel:      "temporary",
		IsDeviceDisabled: isDisabled,
		DisabledUntil:    disabledUntil,
		CreatedAt:        time.Now().UTC(),
	}

	if err := s.db.Create(&access).Error; err != nil {
		return nil, fmt.Errorf("chyba pri vytváraní prístupu")
	}

	return &HackResponse{
		Success:              true,
		AccessGrantedUntil:   &accessUntil,
		DeviceDisabled:       isDisabled,
		DeviceDisabledUntil:  disabledUntil, // nil, ak nie je disabled
		OwnershipTransferred: false,
	}, nil
}

func (s *Service) claimAbandonedDevice(hackerID uuid.UUID, deviceID uuid.UUID, hackTool *HackTool) (*HackResponse, error) {
	_ = hackTool // Suppress unused parameter warning
	var result *HackResponse

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Advisory lock pre zabránenie race conditions
		// PG advisory lock potrebuje BIGINT: z UUID spravíme stabilný 64-bit hash
		if err := tx.Exec(
			"SELECT pg_advisory_xact_lock(hashtextextended(?::text, 0))",
			deviceID.String(),
		).Error; err != nil {
			return fmt.Errorf("chyba pri získaní locku: %w", err)
		}

		// 2. Získať zariadenie s row lock
		var device DeployedDevice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", deviceID, DeviceStatusAbandoned).
			First(&device).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("opustené zariadenie nebolo nájdené alebo už bolo claimnuté")
			}
			return fmt.Errorf("chyba pri načítaní zariadenia: %w", err)
		}

		// 3. TODO: Validovať a odpočítať claim kit/batériu z inventára hackera
		// if err := s.validateAndConsumeClaimKit(tx, hackerID); err != nil {
		//     return fmt.Errorf("chyba pri validácii claim kitu: %w", err)
		// }

		// 4. TODO: Vytvoriť novú batériu v inventári hackera
		// newBatteryID, err := s.createBatteryInInventory(tx, hackerID, "replacement_battery")
		// if err != nil {
		//     return fmt.Errorf("chyba pri vytvorení batérie: %w", err)
		// }

		// 5. Aktualizovať zariadenie s novým vlastníctvom
		// Logika: Ak má batériu → zostane vybitá (0%), ak nemá → bez batérie
		var batteryStatus string
		var batteryLevel int

		if device.BatteryInventoryID != nil {
			// Scanner má batériu → claimni ju a nastav na 0%
			batteryStatus = "depleted"
			batteryLevel = 0
			
			// ✨ Claimni batériu - zmeň owner na hackera a nastav charge na 0%
			if err := tx.Model(&gameplay.InventoryItem{}).
				Where("id = ?", device.BatteryInventoryID).
				Updates(map[string]interface{}{
					"user_id":    hackerID,
					"properties": `{"charge_pct": 0}`,
					"updated_at": time.Now().UTC(),
				}).Error; err != nil {
				return fmt.Errorf("chyba pri claimnutí batérie: %w", err)
			}
			
			log.Printf("🔋 Battery claimed: %s → user %s, charge: 0%%", device.BatteryInventoryID, hackerID)
		} else {
			// Scanner nemá batériu → bez batérie
			batteryStatus = "removed"
			batteryLevel = 0
		}

		updates := map[string]interface{}{
			"owner_id":            hackerID,
			"status":              DeviceStatusActive,
			"is_active":           true,
			"battery_level":       batteryLevel,
			"battery_status":      batteryStatus,
			"battery_depleted_at": nil,
			"abandoned_at":        nil,
			"updated_at":          time.Now().UTC(),
		}

		if err := tx.Model(&DeployedDevice{}).Where("id = ?", deviceID).Updates(updates).Error; err != nil {
			return fmt.Errorf("chyba pri aktualizácii zariadenia: %w", err)
		}

		// 6. Zmazať staré prístupové práva pre toto zariadenie
		if err := tx.Where("device_id = ?", deviceID).Delete(&DeviceAccess{}).Error; err != nil {
			return fmt.Errorf("chyba pri mazaní starých prístupov: %w", err)
		}

		// 7. Vytvoriť response
		result = &HackResponse{
			Success:              true,
			OwnershipTransferred: true,
			NewOwnerID:           &hackerID,
			// Kým nebudeš reálne manipulovať inventár batérií, nech je to false
			BatteryReplaced: false,
			DeviceStatus:    string(DeviceStatusActive),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Utility functions

func (s *Service) calculateDistance(lat1, lng1, lat2, lng2 float64) int {
	const earthRadius = 6371000 // meters

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return int(earthRadius * c)
}

func (s *Service) calculateBearing(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	y := math.Sin(deltaLng) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) -
		math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLng)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

func (s *Service) calculateSignalStrength(distanceM int, maxRangeM int) float64 {
	// Základná sila signálu (1.0 na 0m, 0.0 na max dosahu)
	baseStrength := 1.0 - float64(distanceM)/float64(maxRangeM)

	// Pridaj náhodný šum
	noise := (rand.Float64() - 0.5) * 0.1

	// Obmedz na rozsah 0.0 - 1.0
	strength := math.Max(0.0, math.Min(1.0, baseStrength+noise))

	return strength
}

// GetMapMarkers - získa mapové markery pre danú pozíciu
func (s *Service) GetMapMarkers(userID uuid.UUID, lat, lng float64) (*MapMarkersResponse, error) {
	var markers []MapMarker

	log.Printf("🗺️ [MAP MARKERS] Loading markers for user %s at [%.6f, %.6f]", userID, lat, lng)

	// 1. Vlastné scannery (do 50km) - vždy viditeľné pre majiteľa
	ownMarkers, err := s.getOwnScanners(userID, lat, lng, 50.0)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní vlastných scannerov: %w", err)
	}
	markers = append(markers, ownMarkers...)

	// 2. Opustené scannery (do 50m) - viditeľné pre všetkých
	abandonedMarkers, err := s.getAbandonedScanners(lat, lng, 0.05)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní opustených scannerov: %w", err)
	}
	markers = append(markers, abandonedMarkers...)

	// 3. Hacknuté scannery (do 20km) - viditeľné pre majiteľa aj hackera
	hackedMarkers, err := s.getHackedScanners(userID, lat, lng, 20.0)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní hacknutých scannerov: %w", err)
	}
	markers = append(markers, hackedMarkers...)

	// 4. Cudzie scannery pre scan data (do 200m) - viditeľné len z blízka
	// Radius zmenený z 20km na 0.2km (200m) pre bezpečnosť a gameplay
	scanDataMarkers, err := s.getScanDataScanners(userID, lat, lng, 0.2)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní scannerov pre scan data: %w", err)
	}
	log.Printf("🗺️ [MAP MARKERS] Found %d foreign scanners (scan_data) within 200m", len(scanDataMarkers))
	markers = append(markers, scanDataMarkers...)

	// 5) Doplň per-user cooldown do markerov (CanScan + CooldownUntil)
	for i := range markers {
		if !markers[i].CanScan {
			// nemá zmysel riešiť cooldown ak sa aj tak nedá skenovať (napr. abandoned)
			continue
		}
		if cs, err := s.GetCooldownStatus(userID, markers[i].ID); err == nil && cs != nil {
			// CanScan nech je *prienik* (stav zariadenia AND per-user cooldown)
			markers[i].CanScan = markers[i].CanScan && cs.CanScan
			markers[i].CooldownUntil = cs.CooldownUntil
		}
	}

	// Dedup podľa ID (ponechaj prvý výskyt, aby ostal „visibilityType" z primárneho zdroja)
	seen := make(map[uuid.UUID]bool)
	uniq := make([]MapMarker, 0, len(markers))
	for _, m := range markers {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		uniq = append(uniq, m)
	}
	return &MapMarkersResponse{Markers: uniq}, nil
}

// pomocné štruktúry na skenovanie s distance/hacked_by
type deviceWithDistance struct {
	DeployedDevice
	DistanceKm float64 `gorm:"column:distance_km"`
}
type hackedDeviceRow struct {
	DeployedDevice
	DistanceKm float64   `gorm:"column:distance_km"`
	HackedBy   uuid.UUID `gorm:"column:hacked_by"`
}

// getOwnScanners - vlastné scannery do 50km
func (s *Service) getOwnScanners(userID uuid.UUID, lat, lng float64, radiusKm float64) ([]MapMarker, error) {
	query := `
		SELECT *, 
		       ST_Distance(
		           location,
		           ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		       ) / 1000.0 as distance_km
		FROM gameplay.deployed_devices
		WHERE owner_id = ? 
		  AND is_active = true
		  AND ST_DWithin(
		      location,
		      ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		      ?
		  )
		ORDER BY distance_km ASC
	`

	radiusMeters := radiusKm * 1000
	var rows []deviceWithDistance
	err := s.db.Raw(query, lng, lat, userID, lng, lat, radiusMeters).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var markers []MapMarker
	for _, r := range rows {
		marker := s.createMarkerFromDevice(r.DeployedDevice, "owner")
		marker.DistanceKm = r.DistanceKm
		markers = append(markers, marker)
	}

	return markers, nil
}

// getAbandonedScanners - opustené scannery do 10km
func (s *Service) getAbandonedScanners(lat, lng float64, radiusKm float64) ([]MapMarker, error) {
	query := `
		SELECT *, 
		       ST_Distance(
		           location,
		           ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		       ) / 1000.0 as distance_km
		FROM gameplay.deployed_devices
		WHERE status = ?
		  AND ST_DWithin(
		      location,
		      ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		      ?
		  )
		ORDER BY distance_km ASC
	`

	radiusMeters := radiusKm * 1000
	var rows []deviceWithDistance
	err := s.db.Raw(query, lng, lat, DeviceStatusAbandoned, lng, lat, radiusMeters).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var markers []MapMarker
	for _, r := range rows {
		marker := s.createMarkerFromDevice(r.DeployedDevice, "public")
		marker.DistanceKm = r.DistanceKm
		markers = append(markers, marker)
	}

	return markers, nil
}

// getHackedScanners - hacknuté scannery do 20km
func (s *Service) getHackedScanners(userID uuid.UUID, lat, lng float64, radiusKm float64) ([]MapMarker, error) {
	query := `
		SELECT dd.*,
		       ST_Distance(
		           dd.location,
		           ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		       ) / 1000.0 AS distance_km,
		       da.user_id AS hacked_by
		FROM gameplay.deployed_devices dd
		LEFT JOIN gameplay.device_access da
		       ON dd.id = da.device_id AND da.user_id = ?
		WHERE dd.is_active = TRUE
		  AND da.expires_at > NOW()
		  AND (dd.owner_id = ? OR da.user_id IS NOT NULL)
		  AND ST_DWithin(
		        dd.location,
		        ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		        ?
		  )
		ORDER BY distance_km ASC
	`

	radiusMeters := radiusKm * 1000
	var rows []hackedDeviceRow
	if err := s.db.Raw(query, lng, lat, userID, userID, lng, lat, radiusMeters).Scan(&rows).Error; err != nil {
		return nil, err
	}
	var markers []MapMarker
	for _, r := range rows {
		marker := s.createMarkerFromDevice(r.DeployedDevice, "hacker")
		marker.HackedBy = &r.HackedBy
		marker.DistanceKm = r.DistanceKm
		markers = append(markers, marker)
	}
	return markers, nil
}

// getScanDataScanners - cudzie scannery pre scan data (viditeľné len z veľmi blízka)
// Hráč musí byť veľmi blízko cudzieho scanneru aby ho videl (typicky 100m)
func (s *Service) getScanDataScanners(userID uuid.UUID, lat, lng float64, radiusKm float64) ([]MapMarker, error) {
	query := `
		SELECT *, 
		       ST_Distance(
		           location,
		           ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
		       ) / 1000.0 as distance_km
		FROM gameplay.deployed_devices
		WHERE owner_id != ? 
		  AND status = ?
		  AND is_active = true
		  AND ST_DWithin(
		      location,
		      ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
		      ?
		  )
		ORDER BY distance_km ASC
	`

	radiusMeters := radiusKm * 1000
	var rows []deviceWithDistance
	err := s.db.Raw(query, lng, lat, userID, DeviceStatusActive, lng, lat, radiusMeters).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var markers []MapMarker
	for _, r := range rows {
		// Skontrolovať cooldown pre scan
		cooldownStatus, _ := s.GetCooldownStatus(userID, r.ID)
		if cooldownStatus != nil && cooldownStatus.CanScan {
			marker := s.createMarkerFromDevice(r.DeployedDevice, "scan_data")
			marker.DistanceKm = r.DistanceKm
			markers = append(markers, marker)
		}
	}

	return markers, nil
}

// createMarkerFromDevice - vytvorí marker z DeployedDevice
func (s *Service) createMarkerFromDevice(device DeployedDevice, visibilityType string) MapMarker {
	// Určiť status a ikonu
	status, icon := s.determineDeviceStatusAndIcon(device)

	// Cudzie scannery (scan_data) majú tmavo šedú ikonu namiesto zelenej
	if visibilityType == "scan_data" && icon == "scanner_green" {
		icon = "scanner_dark_gray"
		log.Printf("🎨 [MARKER] Cudzí scanner %s zmenený na tmavo šedý (visibility: %s)", device.ID, visibilityType)
	}

	// Určiť interakcie
	canHack := device.Status == DeviceStatusActive && device.IsActive
	canScan := device.Status == DeviceStatusActive && device.IsActive &&
		device.BatteryInventoryID != nil &&
		device.BatteryLevel != nil &&
		*device.BatteryLevel > 0 &&
		(device.BatteryStatus == nil || (*device.BatteryStatus != "removed" && *device.BatteryStatus != "depleted"))
	canClaim := device.Status == DeviceStatusAbandoned

	// Ensure default values for scan radius
	scanRadiusKm := device.ScanRadiusKm
	if scanRadiusKm <= 0 {
		scanRadiusKm = 1.0 // Default 1km
	}

	return MapMarker{
		ID:        device.ID,
		Type:      "deployed_scanner",
		Status:    status,
		Latitude:  device.Latitude,
		Longitude: device.Longitude,
		Icon:      icon,
		BatteryLevel: func() int {
			if device.BatteryLevel != nil {
				return *device.BatteryLevel
			}
			return 0
		}(),
		ScanRadiusKm:   scanRadiusKm,
		CanHack:        canHack,
		CanScan:        canScan,
		CanClaim:       canClaim,
		OwnerID:        &device.OwnerID,
		VisibilityType: visibilityType,
	}
}

// determineDeviceStatusAndIcon - určí status a ikonu zariadenia
func (s *Service) determineDeviceStatusAndIcon(device DeployedDevice) (status, icon string) {
	// Červená - vybitá batéria (len majiteľ vidí)
	if device.BatteryLevel != nil && *device.BatteryLevel <= 0 {
		return "battery_dead", "scanner_red"
	}

	// Modrá - hacknuté zariadenie
	if device.LastDisabledAt != nil && time.Since(*device.LastDisabledAt) < 24*time.Hour {
		return "hacked", "scanner_blue"
	}

	// Šedá - opustené zariadenie
	if device.Status == DeviceStatusAbandoned {
		return "abandoned", "scanner_gray"
	}

	// Zelená - aktívne zariadenie
	return "active", "scanner_green"
}

// RemoveBatteryResponse - odpoveď pre vybratie batérie
type RemoveBatteryResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Device  *DeployedDevice `json:"device,omitempty"`
}

// AttachBatteryRequest - request na pripojenie batérie
type AttachBatteryRequest struct {
	BatteryInventoryID uuid.UUID `json:"battery_inventory_id" binding:"required"`
}

// AttachBatteryResponse - odpoveď pre pripojenie batérie
type AttachBatteryResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Device  *DeployedDevice `json:"device,omitempty"`
}

// RemoveBattery - vyberie vybitú batériu z zariadenia
func (s *Service) RemoveBattery(deviceID uuid.UUID, userID uuid.UUID) (*RemoveBatteryResponse, error) {
	// 1. Načítať zariadenie
	var device DeployedDevice
	if err := s.db.Where("id = ? AND owner_id = ? AND is_active = true", deviceID, userID).First(&device).Error; err != nil {
		return &RemoveBatteryResponse{
			Success: false,
			Message: "Zariadenie nebolo nájdené alebo nepatrí vám",
		}, nil
	}

	// 2. Skontrolovať, či je batéria vybitá
	if device.BatteryLevel != nil && *device.BatteryLevel > 0 {
		return &RemoveBatteryResponse{
			Success: false,
			Message: "Batéria nie je vybitá, nemôže sa vybrať",
		}, nil
	}

	// 3. Skontrolovať, či má zariadenie batériu
	if device.BatteryInventoryID == nil {
		return &RemoveBatteryResponse{
			Success: false,
			Message: "Zariadenie nemá inštalovanú batériu",
		}, nil
	}

	// 4. Vrátiť batériu do inventára s 0% batériou v transakcii
	batteryInventoryID := *device.BatteryInventoryID
	var updatedDevice DeployedDevice

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Najprv odstrániť batériu zo zariadenia
		if err := tx.Model(&device).Updates(map[string]interface{}{
			"battery_inventory_id": nil,
			"battery_status":       "removed",
			"battery_level":        0,
			"updated_at":           time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("failed to remove battery from device: %w", err)
		}

		// Potom obnoviť batériu v inventári (odstrániť soft delete) a nastaviť na 0%
		// Použiť charge_pct namiesto battery_level pre správnu validáciu
		if err := tx.Exec(`
			UPDATE gameplay.inventory_items
			SET deleted_at = NULL,
			    properties = jsonb_set(COALESCE(properties,'{}'::jsonb), '{charge_pct}', '0'::jsonb, true),
			    updated_at = NOW()
			WHERE id = ?
		`, batteryInventoryID).Error; err != nil {
			return fmt.Errorf("failed to restore battery in inventory: %w", err)
		}

		// Načítať aktualizované zariadenie
		if err := tx.Where("id = ?", deviceID).First(&updatedDevice).Error; err != nil {
			return fmt.Errorf("failed to load updated device: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("⚠️ Failed to remove battery: %v", err)
		return &RemoveBatteryResponse{
			Success: false,
			Message: "Chyba pri vybratí batérie: " + err.Error(),
		}, nil
	}

	log.Printf("🔋 Battery removed from device %s (%s) by user %s", deviceID, device.Name, userID)

	return &RemoveBatteryResponse{
		Success: true,
		Message: "Batéria bola úspešne vybratá a vrátená do inventára s 0% batériou",
		Device:  &updatedDevice,
	}, nil
}

// AttachBattery - pripojí batériu k zariadeniu
func (s *Service) AttachBattery(deviceID uuid.UUID, userID uuid.UUID, req *AttachBatteryRequest) (*AttachBatteryResponse, error) {
	// 1. Načítať zariadenie
	var device DeployedDevice
	if err := s.db.Where("id = ? AND owner_id = ? AND is_active = true", deviceID, userID).First(&device).Error; err != nil {
		return &AttachBatteryResponse{
			Success: false,
			Message: "Zariadenie nebolo nájdené alebo nepatrí vám",
		}, nil
	}

	// 2. Skontrolovať, či zariadenie nemá už batériu
	if device.BatteryInventoryID != nil {
		return &AttachBatteryResponse{
			Success: false,
			Message: "Zariadenie už má inštalovanú batériu",
		}, nil
	}

	// 3. Skontrolovať, či batéria existuje v inventári hráča a má 100% nabitie
	var batteryItem gameplay.InventoryItem
	if err := s.db.Where("id = ? AND user_id = ? AND item_type = 'scanner_battery' AND deleted_at IS NULL", req.BatteryInventoryID, userID).First(&batteryItem).Error; err != nil {
		return &AttachBatteryResponse{
			Success: false,
			Message: "Batéria nebola nájdená v inventári",
		}, nil
	}

	// 4. Skontrolovať nabitie batérie
	chargePct, ok := batteryItem.Properties["charge_pct"].(float64)
	if !ok {
		chargePct = 100.0 // Default ak nie je v properties
	}
	if chargePct < 100 {
		return &AttachBatteryResponse{
			Success: false,
			Message: "Batéria musí byť plne nabitá (100%)",
		}, nil
	}

	// 5. Pripojiť batériu v transakcii
	var updatedDevice DeployedDevice
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Pripojiť batériu k zariadeniu
		if err := tx.Model(&device).Updates(map[string]interface{}{
			"battery_inventory_id": req.BatteryInventoryID,
			"battery_status":       "installed",
			"battery_level":        100,
			"status":               "active",
			"updated_at":           time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("failed to attach battery to device: %w", err)
		}

		// Odstrániť batériu z inventára (soft delete)
		if err := tx.Model(&batteryItem).Update("deleted_at", time.Now()).Error; err != nil {
			return fmt.Errorf("failed to remove battery from inventory: %w", err)
		}

		// Načítať aktualizované zariadenie
		if err := tx.Where("id = ?", deviceID).First(&updatedDevice).Error; err != nil {
			return fmt.Errorf("failed to load updated device: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("⚠️ Failed to attach battery: %v", err)
		return &AttachBatteryResponse{
			Success: false,
			Message: "Chyba pri pripojení batérie: " + err.Error(),
		}, nil
	}

	log.Printf("🔋 Battery attached to device %s (%s) by user %s", deviceID, device.Name, userID)

	return &AttachBatteryResponse{
		Success: true,
		Message: "Batéria bola úspešne pripojená k zariadeniu",
		Device:  &updatedDevice,
	}, nil
}

// GetHackTools - získa hackovacie nástroje hráča z inventory_items
func (s *Service) GetHackTools(userID uuid.UUID) ([]HackTool, error) {
	var inventoryItems []gameplay.InventoryItem

	// Načítaj hack tools z inventory_items (nie z hack_tools tabuľky!)
	if err := s.db.Where("user_id = ? AND item_type = ? AND deleted_at IS NULL",
		userID, "hack_tool").Find(&inventoryItems).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve hack tools from inventory: %w", err)
	}

	// Konvertuj inventory items na HackTool štruktúru
	var hackTools []HackTool
	for _, item := range inventoryItems {
		// Extrahuj uses_left z properties
		usesLeft := 0
		if ul, ok := item.Properties["uses_left"].(float64); ok {
			usesLeft = int(ul)
		}

		// Extrahuj tool_type z properties (mapuj na povolené typy pre DB constraint)
		toolType := "circuit_breaker" // default
		if tt, ok := item.Properties["tool_type"].(string); ok {
			// Mapovanie: basic_hack → circuit_breaker, advanced_hack → code_cracker, atď.
			switch tt {
			case "basic_hack":
				toolType = "circuit_breaker"
			case "advanced_hack":
				toolType = "code_cracker"
			case "device_claimer":
				toolType = "stealth_infiltration"
			case "stealth_hack":
				toolType = "stealth_infiltration"
			default:
				toolType = tt // Použij ako je (ak je už mapované)
			}
		}

		// Extrahuj name z properties
		name := "Unknown Hack Tool"
		if n, ok := item.Properties["name"].(string); ok {
			name = n
		} else if dn, ok := item.Properties["display_name"].(string); ok {
			name = dn
		}

		hackTools = append(hackTools, HackTool{
			ID:         item.ID,
			UserID:     item.UserID,
			ToolType:   toolType,
			Name:       name,
			UsesLeft:   usesLeft,
			ExpiresAt:  nil, // Hack tools z marketu nevypršavajú
			Properties: datatypes.JSONMap(item.Properties),
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}

	log.Printf("🔧 Retrieved %d hack tools from inventory for user %s", len(hackTools), userID)

	return hackTools, nil
}
