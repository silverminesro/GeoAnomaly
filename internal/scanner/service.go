package scanner

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"geoanomaly/internal/common"
	"geoanomaly/internal/loadout"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	db             *gorm.DB
	redis          *redis.Client
	loadoutService *loadout.Service
}

func NewService(db *gorm.DB, redisClient *redis.Client) *Service {
	loadoutService := loadout.NewService(db)
	return &Service{
		db:             db,
		redis:          redisClient,
		loadoutService: loadoutService,
	}
}

// GetScannerByCode - načíta scanner z katalógu podľa kódu
func (s *Service) GetScannerByCode(code string) (*ScannerCatalog, error) {
	var scanner ScannerCatalog

	// Explicitne vyber všetky polia okrem computed fields
	if err := s.db.Select("id, code, name, tagline, description, base_range_m, base_fov_deg, caps_json, drain_mult, allowed_modules, slot_count, slot_types, is_basic, max_rarity, detect_artifacts, detect_gear, version, effective_from, created_at, updated_at").
		Where("code = ?", code).First(&scanner).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("scanner with code '%s' not found", code)
		}
		return nil, fmt.Errorf("failed to load scanner from database: %w", err)
	}

	return &scanner, nil
}

// GetBasicScanner - vráti základný scanner pre hráča
func (s *Service) GetBasicScanner() (*ScannerCatalog, error) {
	return s.GetScannerByCode("echovane_mk0")
}

// resolveScannerCodeFromLoadout - helper: hráčov equipnutý scanner -> kód do scanner_catalog
func (s *Service) resolveScannerCodeFromLoadout(userID uuid.UUID) (string, error) {
	// načítame celý loadout
	lo, err := s.loadoutService.GetUserLoadout(userID)
	if err != nil {
		return "", err
	}

	// nič v slote 'scanner' => default
	slot, ok := lo["scanner"]
	if !ok || slot == nil || slot.ItemID == uuid.Nil {
		return "echovane_mk0", nil // bezpečný fallback
	}

	// properties skeneru z loadoutu – zvyknú obsahovať "name" a/alebo "model" a často aj "market_item_id"
	name := ""
	model := ""

	if slot.Properties != nil {
		if nameVal, exists := slot.Properties["name"]; exists {
			if nameStr, ok := nameVal.(string); ok {
				name = strings.ToLower(strings.TrimSpace(nameStr))
			}
		}
		if modelVal, exists := slot.Properties["model"]; exists {
			if modelStr, ok := modelVal.(string); ok {
				model = strings.ToLower(strings.TrimSpace(modelStr))
			}
		}
	}

	// Dočasná mapka názov→kód (kým nebude priamy market ID mapping)
	switch {
	case strings.Contains(name, "vesta scout") || strings.Contains(model, "vesta"):
		return "vesta_scout_50", nil
	case strings.Contains(name, "radian wide") || strings.Contains(model, "radian"):
		return "radian_wide_70", nil
	case strings.Contains(name, "aurora arc") || strings.Contains(model, "aurora"):
		return "aurora_arc_90", nil
	case strings.Contains(name, "omnisphere") || strings.Contains(model, "omnisphere"):
		return "omnisphere_360", nil
	case strings.Contains(name, "chemvisor") || strings.Contains(model, "chemvisor"):
		return "chemvisor_mk1", nil
	default:
		return "echovane_mk0", nil
	}
}

// GetOrCreateScannerInstance - vráti alebo vytvorí scanner inštanciu pre hráča
func (s *Service) GetOrCreateScannerInstance(userID uuid.UUID) (*ScannerInstance, *ScannerCatalog, error) {
	// 1) zisti pracujúci kod zo slotu scanner
	code, err := s.resolveScannerCodeFromLoadout(userID)
	if err != nil {
		return nil, nil, err
	}

	// 2) nájdi/ulož instanciu hráča s týmto kódom
	var instance ScannerInstance
	if err := s.db.Select("id, owner_id, scanner_code, energy_cap, power_cell_code, power_cell_started_at, power_cell_minutes_left, is_active, created_at, updated_at").
		Where("owner_id = ? AND is_active = true", userID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Vytvor novú inštanciu s loadout scannerom
			instance = ScannerInstance{
				ID:          uuid.New(),
				OwnerID:     userID,
				ScannerCode: code,
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			if err := s.db.Create(&instance).Error; err != nil {
				return nil, nil, fmt.Errorf("failed to create scanner instance: %w", err)
			}

			log.Printf("✅ Created scanner instance with loadout scanner: %s for user %s", code, userID)
		} else {
			return nil, nil, fmt.Errorf("failed to query scanner instance: %w", err)
		}
	} else {
		// Aktualizuj existujúcu inštanciu, ak sa scanner zmenil
		if instance.ScannerCode != code {
			instance.ScannerCode = code
			instance.UpdatedAt = time.Now()

			if err := s.db.Save(&instance).Error; err != nil {
				return nil, nil, fmt.Errorf("failed to update scanner instance: %w", err)
			}

			log.Printf("✅ Updated scanner instance from %s to %s for user %s", instance.ScannerCode, code, userID)
		}
	}

	// 3) načítaj katalógové schopnosti
	cat, err := s.GetScannerByCode(code)
	if err != nil {
		return nil, nil, err
	}

	// Načítaj scanner detaily
	if err := s.loadScannerDetails(&instance); err != nil {
		return nil, nil, fmt.Errorf("failed to load scanner details: %w", err)
	}

	return &instance, cat, nil
}

// loadScannerDetails - načíta scanner catalog a moduly
func (s *Service) loadScannerDetails(instance *ScannerInstance) error {
	// Načítaj scanner z katalógu
	scanner, err := s.GetScannerByCode(instance.ScannerCode)
	if err != nil {
		return err
	}
	instance.Scanner = scanner

	// Načítaj inštalované moduly - explicitne vyber len základné polia
	var modules []ScannerModule
	if err := s.db.Select("instance_id, slot_index, module_code, installed_at").
		Where("instance_id = ?", instance.ID).Find(&modules).Error; err != nil {
		return fmt.Errorf("failed to load scanner modules: %w", err)
	}

	// Načítaj detaily modulov
	for i := range modules {
		var moduleCatalog ModuleCatalog
		if err := s.db.Where("code = ?", modules[i].ModuleCode).First(&moduleCatalog).Error; err != nil {
			log.Printf("Warning: module %s not found in catalog", modules[i].ModuleCode)
			continue
		}
		modules[i].Module = &moduleCatalog
	}

	instance.Modules = modules
	return nil
}

// CalculateScannerStats - vypočíta efektívne stats scanner
func (s *Service) CalculateScannerStats(instance *ScannerInstance) (*ScannerStats, error) {
	if instance.Scanner == nil {
		return nil, fmt.Errorf("scanner details not loaded")
	}

	// Použi fillCapsDefaults pre bezpečné doplnenie hodnôt
	caps := s.fillCapsDefaults(instance.Scanner)

	stats := &ScannerStats{
		RangeM:           caps.Limits.RangeM,
		FovDeg:           caps.Limits.FovDeg,
		ServerPollHz:     caps.ScanConfig.ServerPollHz,
		LockOnThreshold:  float64(caps.ScanConfig.LockOn.AngleDeg), // Použi angle_deg ako threshold
		EnergyCap:        100,                                      // Základná energia
		VisualStyle:      caps.Visual.Style,
		ScanMode:         caps.ScanConfig.Mode,
		ClientTickHz:     caps.ScanConfig.ClientTickHz,
		SeeMaxRarity:     caps.Limits.SeeMaxRarity,
		CollectMaxRarity: caps.Limits.CollectMaxRarity,
	}

	// Aplikuj moduly
	for _, module := range instance.Modules {
		if module.Module != nil {
			s.applyModuleEffects(stats, module.Module)
		}
	}

	return stats, nil
}

// fillCapsDefaults - bezpečné doplnenie caps z base_* stĺpcov
func (s *Service) fillCapsDefaults(row *ScannerCatalog) ScannerCaps {
	caps := row.CapsJSON // už naparsované caps_json (ak prázdne, nuly)
	if caps.Visual.Style == "" {
		caps.Visual.Style = "v_hud"
	}
	if caps.ScanConfig.ClientTickHz == 0 {
		caps.ScanConfig.ClientTickHz = 20
	}
	if caps.ScanConfig.ServerPollHz == 0 {
		caps.ScanConfig.ServerPollHz = 1.0
	}
	if caps.ScanConfig.LockOn.AngleDeg == 0 {
		caps.ScanConfig.LockOn.AngleDeg = 5
	}
	if caps.ScanConfig.LockOn.RadiusM == 0 {
		caps.ScanConfig.LockOn.RadiusM = 6
	}
	if caps.Limits.RangeM == 0 {
		caps.Limits.RangeM = row.BaseRangeM
	}
	if caps.Limits.FovDeg == 0 {
		caps.Limits.FovDeg = row.BaseFovDeg
	}
	if caps.Limits.SeeMaxRarity == "" {
		caps.Limits.SeeMaxRarity = row.MaxRarity
	}
	if caps.Limits.CollectMaxRarity == "" {
		caps.Limits.CollectMaxRarity = row.MaxRarity
	}
	if !caps.Filters.Artifacts && !caps.Filters.Gear {
		caps.Filters.Artifacts = true
		caps.Filters.Gear = true
	}
	return caps
}

// applyModuleEffects - aplikuje účinky modulu na stats
func (s *Service) applyModuleEffects(stats *ScannerStats, module *ModuleCatalog) {
	effects := module.EffectsJSON

	if effects.RangePct != nil {
		stats.RangeM = int(float64(stats.RangeM) * (1 + float64(*effects.RangePct)/100))
	}

	if effects.FovPct != nil {
		stats.FovDeg = int(float64(stats.FovDeg) * (1 + float64(*effects.FovPct)/100))
	}

	if effects.ServerPollHzAdd != nil {
		stats.ServerPollHz += *effects.ServerPollHzAdd
	}

	if effects.LockOnThresholdDelta != nil {
		stats.LockOnThreshold += *effects.LockOnThresholdDelta
	}
}

// Scan - vykoná skenovanie v zóne
func (s *Service) Scan(userID uuid.UUID, req *ScanRequest) (*ScanResponse, error) {
	// Skontroluj či hráč má aktívnu zónu
	zoneID, err := s.getActiveZoneID(userID)
	if err != nil {
		return nil, fmt.Errorf("must enter zone first to use scanner")
	}

	// Načítaj scanner inštanciu
	instance, _, err := s.GetOrCreateScannerInstance(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get scanner instance: %w", err)
	}

	// Vypočítať scanner stats
	stats, err := s.CalculateScannerStats(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate scanner stats: %w", err)
	}

	// Načítaj itemy v zóne
	artifacts, gear, err := s.getZoneItems(zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone items: %w", err)
	}

	// Filtruj itemy podľa scanner schopností
	scanResults := s.filterItemsByScanner(artifacts, gear, req, stats, instance.Scanner)

	return &ScanResponse{
		Success:      true,
		ScanResults:  scanResults,
		ScannerStats: stats,
	}, nil
}

// getActiveZoneID - získa ID aktívnej zóny hráča z databázy
func (s *Service) getActiveZoneID(userID uuid.UUID) (string, error) {
	var session common.PlayerSession
	if err := s.db.Where("user_id = ? AND current_zone IS NOT NULL", userID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("no active zone session")
		}
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	if session.CurrentZone == nil {
		return "", fmt.Errorf("no active zone session")
	}

	return session.CurrentZone.String(), nil
}

// getZoneItems - načíta artefakty a gear v zóne
func (s *Service) getZoneItems(zoneID string) ([]common.Artifact, []common.Gear, error) {
	// Validácia UUID formátu
	if _, err := uuid.Parse(zoneID); err != nil {
		return nil, nil, fmt.Errorf("invalid zone ID: %w", err)
	}

	var artifacts []common.Artifact
	if err := s.db.Where("zone_id = ? AND is_active = true AND is_claimed = false", zoneID).Find(&artifacts).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to load artifacts: %w", err)
	}

	var gear []common.Gear
	if err := s.db.Where("zone_id = ? AND is_active = true AND is_claimed = false", zoneID).Find(&gear).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to load gear: %w", err)
	}

	return artifacts, gear, nil
}

// filterItemsByScanner - filtruje itemy podľa scanner schopností
func (s *Service) filterItemsByScanner(artifacts []common.Artifact, gear []common.Gear, req *ScanRequest, stats *ScannerStats, scanner *ScannerCatalog) []ScanResult {
	var results []ScanResult

	// Použi fillCapsDefaults pre bezpečné doplnenie hodnôt
	caps := s.fillCapsDefaults(scanner)

	// Filtruj artefakty
	if caps.Filters.Artifacts {
		for _, artifact := range artifacts {
			if s.canDetectItem(artifact.Rarity, caps.Limits.SeeMaxRarity) {
				result := s.createScanResult(&artifact, req, stats)
				if result != nil {
					results = append(results, *result)
				}
			}
		}
	}

	// Filtruj gear
	if caps.Filters.Gear {
		for _, g := range gear {
			// Gear nemá rarity, použijeme "common" ako default
			if s.canDetectItem("common", caps.Limits.SeeMaxRarity) {
				result := s.createScanResult(&g, req, stats)
				if result != nil {
					results = append(results, *result)
				}
			}
		}
	}

	return results
}

// canDetectItem - skontroluje či scanner môže detekovať item danej rarity
func (s *Service) canDetectItem(itemRarity, maxRarity string) bool {
	rarityLevels := map[string]int{
		"common":    1,
		"uncommon":  2,
		"rare":      3,
		"epic":      4,
		"legendary": 5,
	}

	itemLevel, itemExists := rarityLevels[itemRarity]
	maxLevel, maxExists := rarityLevels[maxRarity]

	if !itemExists || !maxExists {
		return false
	}

	return itemLevel <= maxLevel
}

// createScanResult - vytvorí scan result pre item
func (s *Service) createScanResult(item interface{}, req *ScanRequest, stats *ScannerStats) *ScanResult {
	var itemLat, itemLng float64
	var itemID uuid.UUID
	var itemName, itemRarity, itemType string

	// Extrahuj dáta z item
	switch v := item.(type) {
	case *common.Artifact:
		itemLat = v.Location.Latitude
		itemLng = v.Location.Longitude
		itemID = v.ID
		itemName = v.Name
		itemRarity = v.Rarity
		itemType = "artifact"
	case *common.Gear:
		itemLat = v.Location.Latitude
		itemLng = v.Location.Longitude
		itemID = v.ID
		itemName = v.Name
		itemRarity = "common" // Gear nemá rarity, použijeme default
		itemType = "gear"
	default:
		return nil
	}

	// Vypočítať vzdialenosť a bearing
	distanceM := s.calculateDistance(req.Latitude, req.Longitude, itemLat, itemLng)
	bearingDeg := s.calculateBearing(req.Latitude, req.Longitude, itemLat, itemLng)

	// Skontroluj či je v dosahu
	if distanceM > stats.RangeM {
		return nil
	}

	// Vypočítať signal strength
	signalStrength := s.calculateSignalStrength(distanceM, stats)

	return &ScanResult{
		Type:           itemType,
		DistanceM:      distanceM,
		BearingDeg:     bearingDeg,
		SignalStrength: signalStrength,
		ItemID:         &itemID,
		Name:           itemName,
		Rarity:         itemRarity,
	}
}

// calculateDistance - vypočíta vzdialenosť medzi dvoma bodmi
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

// calculateBearing - vypočíta bearing medzi dvoma bodmi
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

// calculateSignalStrength - vypočíta silu signálu
func (s *Service) calculateSignalStrength(distanceM int, stats *ScannerStats) float64 {
	// Základná sila signálu (1.0 na 0m, 0.0 na max dosahu)
	baseStrength := 1.0 - float64(distanceM)/float64(stats.RangeM)

	// Pridaj náhodný šum
	noise := (rand.Float64() - 0.5) * 0.1

	// Obmedz na rozsah 0.0 - 1.0
	strength := math.Max(0.0, math.Min(1.0, baseStrength+noise))

	return strength
}

// GetSecureZoneData returns encrypted zone data for client-side processing
func (s *Service) GetSecureZoneData(zoneID string, userID string) (*SecureZoneData, error) {
	// Validate zone ID format
	if _, err := uuid.Parse(zoneID); err != nil {
		return nil, fmt.Errorf("invalid zone ID: %w", err)
	}

	// Load zone artifacts
	artifacts, gear, err := s.getZoneItems(zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to load zone items: %w", err)
	}

	// Create zone artifacts structure
	zoneArtifacts := ZoneArtifacts{
		Artifacts: artifacts,
		Gear:      gear,
		ZoneID:    zoneID,
		Timestamp: time.Now(),
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(zoneArtifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize zone data: %w", err)
	}

	// Generate encryption key (in production, use proper key management)
	key := s.generateEncryptionKey(zoneID, userID)

	// Encrypt data
	encryptedData, err := s.encryptData(jsonData, key)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt zone data: %w", err)
	}

	// Generate session token
	sessionToken := s.generateSessionToken(zoneID, userID)

	// Create secure data
	secureData := &SecureZoneData{
		EncryptedArtifacts: base64.StdEncoding.EncodeToString(encryptedData),
		ZoneHash:           s.generateZoneHash(zoneID),
		SessionToken:       sessionToken,
		ExpiresAt:          time.Now().Add(30 * time.Minute),
		MaxScans:           100,
		ScanCount:          0,
	}

	return secureData, nil
}

// generateEncryptionKey - generuje kľúč pre šifrovanie
func (s *Service) generateEncryptionKey(zoneID, userID string) []byte {
	// V produkcii použite proper key management
	keyData := zoneID + ":" + userID + ":geoanomaly_secret"
	hash := sha256.Sum256([]byte(keyData))
	return hash[:32] // AES-256 potrebuje 32 bajty
}

// encryptData - zašifruje dáta pomocou AES
func (s *Service) encryptData(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := cryptorand.Read(iv); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := make([]byte, len(data))
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext, data)

	// Combine IV and ciphertext
	result := make([]byte, 0, len(iv)+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)

	return result, nil
}

// generateSessionToken - generuje session token
func (s *Service) generateSessionToken(zoneID, userID string) string {
	data := zoneID + ":" + userID + ":" + time.Now().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// generateZoneHash - generuje hash zóny
func (s *Service) generateZoneHash(zoneID string) string {
	hash := sha256.Sum256([]byte(zoneID))
	return base64.StdEncoding.EncodeToString(hash[:])
}
