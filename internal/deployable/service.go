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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// 1. Validovať tier obmedzenia
	if err := s.validateTierLimits(userID); err != nil {
		return nil, err
	}

	// 2. Validovať vzdialenosť od hráča
	if err := s.validateDeploymentDistance(userID, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}

	// 3. Validácia inventára – hráč to musí vlastniť a kusy nesmú byť v použití
	iq := NewInventoryQueries(s.db)
	if err := iq.ValidateDeploymentInventory(userID, req.DeviceInventoryID, req.BatteryInventoryID); err != nil {
		return nil, err
	}

	// 4. Vytvoriť zariadenie
	device := DeployedDevice{
		ID:                 uuid.New(),
		OwnerID:            userID,
		DeviceInventoryID:  req.DeviceInventoryID,
		BatteryInventoryID: req.BatteryInventoryID,
		Name:               req.Name,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		DeployedAt:         time.Now().UTC(),
		IsActive:           true,
		BatteryLevel:       100,
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
		return nil, fmt.Errorf("failed to create deployed device: %w", err)
	}

	// 6. NEMAZAŤ z inventára – kus ostáva vlastníctvom hráča
	// „v používaní" je dané referenciou v deployed_devices a vynútené UNIQUE indexom

	log.Printf("✅ Deployed device %s for user %s at [%.6f, %.6f]", device.Name, userID, req.Latitude, req.Longitude)

	return &DeployResponse{
		Success:    true,
		DeviceID:   device.ID,
		DeviceName: device.Name,
	}, nil
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

	// 5. Získať a validovať hackovací nástroj
	var hackTool HackTool
	if err := s.db.Where("id = ? AND user_id = ?", req.HackToolID, hackerID).First(&hackTool).Error; err != nil {
		return nil, fmt.Errorf("hackovací nástroj nebol nájdený")
	}

	// Validovať hack tool
	if hackTool.UsesLeft <= 0 {
		return nil, fmt.Errorf("hackovací nástroj nemá žiadne zostávajúce použitia")
	}
	if hackTool.ExpiresAt != nil && time.Now().UTC().After(*hackTool.ExpiresAt) {
		return nil, fmt.Errorf("hackovací nástroj vypršal")
	}

	// 6. Vykonať hack s meraním času
	start := time.Now()
	success := s.performHack(&device, &hackTool)
	hackDuration := int(time.Since(start).Seconds())

	// 7. Zaznamenať hack s skutočnými hodnotami
	hack := DeviceHack{
		ID:              uuid.New(),
		DeviceID:        deviceID,
		HackerID:        hackerID,
		HackTime:        time.Now().UTC(),
		Success:         success,
		HackToolUsed:    hackTool.ToolType,
		DistanceM:       float64(distance),
		HackDurationSec: hackDuration,
		CreatedAt:       time.Now().UTC(),
	}

	s.db.Create(&hack)

	// Spotrebovať hack tool (atomicky znížiť UsesLeft)
	result := s.db.Model(&HackTool{}).Where("id = ? AND uses_left > 0", hackTool.ID).UpdateColumn("uses_left", gorm.Expr("uses_left - 1"))
	if result.Error != nil {
		return nil, fmt.Errorf("chyba pri spotrebovaní nástroja: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("nástroj už nemá žiadne použitia")
	}

	if !success {
		return &HackResponse{Success: false}, nil
	}

	// 6. Spracovať výsledok hacku
	if device.Status == DeviceStatusAbandoned {
		// Opustené zariadenie - automaticky claimnutie
		return s.claimAbandonedDevice(hackerID, deviceID, &hackTool)
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

	// 4. Validovať vzdialenosť (50m pre hack)
	if distance > 50 {
		return nil, fmt.Errorf("príliš ďaleko od zariadenia (%dm)", distance)
	}

	// 5. Získať a validovať hackovací nástroj
	var hackTool HackTool
	if err := s.db.Where("id = ? AND user_id = ?", req.HackToolID, hackerID).First(&hackTool).Error; err != nil {
		return nil, fmt.Errorf("hackovací nástroj nebol nájdený")
	}

	// Validovať hack tool
	if hackTool.UsesLeft <= 0 {
		return nil, fmt.Errorf("hackovací nástroj nemá žiadne zostávajúce použitia")
	}
	if hackTool.ExpiresAt != nil && time.Now().UTC().After(*hackTool.ExpiresAt) {
		return nil, fmt.Errorf("hackovací nástroj vypršal")
	}

	// 6. Claim zariadenie s meraním času
	start := time.Now()
	hackResponse, err := s.claimAbandonedDevice(hackerID, deviceID, &hackTool)
	hackDuration := int(time.Since(start).Seconds())

	if err != nil {
		return nil, err
	}

	// 7. Zaznamenať hack s skutočnými hodnotami
	hack := DeviceHack{
		ID:              uuid.New(),
		DeviceID:        deviceID,
		HackerID:        hackerID,
		HackTime:        time.Now().UTC(),
		Success:         hackResponse.Success,
		HackToolUsed:    hackTool.ToolType,
		DistanceM:       float64(distance),
		HackDurationSec: hackDuration,
		CreatedAt:       time.Now().UTC(),
	}
	s.db.Create(&hack)

	// Spotrebovať hack tool (atomicky znížiť UsesLeft)
	result := s.db.Model(&HackTool{}).Where("id = ? AND uses_left > 0", hackTool.ID).UpdateColumn("uses_left", gorm.Expr("uses_left - 1"))
	if result.Error != nil {
		return nil, fmt.Errorf("chyba pri spotrebovaní nástroja: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("nástroj už nemá žiadne použitia")
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
	var cooldown ScanCooldown
	err := s.db.Where("device_id = ? AND user_id = ?", deviceID, userID).First(&cooldown).Error

	if err == gorm.ErrRecordNotFound {
		// Prvý sken - žiadny cooldown
		return &CooldownStatus{
			CanScan: true,
		}, nil
	}

	if err != nil {
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

	// Validovať vzdialenosť (max 100m pre deploy)
	if distance > 100 {
		return fmt.Errorf("príliš ďaleko od aktuálnej polohy (%dm)", distance)
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
		updates := map[string]interface{}{
			"owner_id":            hackerID,
			"status":              DeviceStatusActive,
			"is_active":           true,
			"battery_level":       100,
			"battery_depleted_at": nil,
			"abandoned_at":        nil,
			"updated_at":          time.Now().UTC(),
			// TODO: "battery_inventory_id": newBatteryID,
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

	// 1. Vlastné scannery (do 50km) - vždy viditeľné pre majiteľa
	ownMarkers, err := s.getOwnScanners(userID, lat, lng, 50.0)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní vlastných scannerov: %w", err)
	}
	markers = append(markers, ownMarkers...)

	// 2. Opustené scannery (do 10km) - viditeľné pre všetkých
	abandonedMarkers, err := s.getAbandonedScanners(lat, lng, 10.0)
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

	// 4. Cudzie scannery pre scan data (do 20km) - len ak môže skenovať
	scanDataMarkers, err := s.getScanDataScanners(userID, lat, lng, 20.0)
	if err != nil {
		return nil, fmt.Errorf("chyba pri načítaní scannerov pre scan data: %w", err)
	}
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

// getScanDataScanners - cudzie scannery pre scan data do 20km
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

	// Určiť interakcie
	canHack := device.Status == DeviceStatusActive && device.IsActive
	canScan := device.Status == DeviceStatusActive && device.IsActive
	canClaim := device.Status == DeviceStatusAbandoned

	// Ensure default values for scan radius
	scanRadiusKm := device.ScanRadiusKm
	if scanRadiusKm <= 0 {
		scanRadiusKm = 1.0 // Default 1km
	}

	return MapMarker{
		ID:             device.ID,
		Type:           "deployed_scanner",
		Status:         status,
		Latitude:       device.Latitude,
		Longitude:      device.Longitude,
		Icon:           icon,
		BatteryLevel:   device.BatteryLevel,
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
	if device.BatteryLevel <= 0 {
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
