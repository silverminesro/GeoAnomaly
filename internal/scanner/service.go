package scanner

import (
	"context"
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
	"time"

	"geoanomaly/internal/common"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewService(db *gorm.DB, redisClient *redis.Client) *Service {
	return &Service{db: db, redis: redisClient}
}

// GetScannerByCode - načíta scanner z katalógu podľa kódu
func (s *Service) GetScannerByCode(code string) (*ScannerCatalog, error) {
	var scanner ScannerCatalog

	if err := s.db.Where("code = ?", code).First(&scanner).Error; err != nil {
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

// GetOrCreateScannerInstance - vráti alebo vytvorí scanner inštanciu pre hráča
func (s *Service) GetOrCreateScannerInstance(userID uuid.UUID) (*ScannerInstance, error) {
	var instance ScannerInstance

	// Skús nájsť existujúcu inštanciu
	if err := s.db.Where("owner_id = ? AND is_active = true", userID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Vytvor novú inštanciu s základným scannerom
			basicScanner, err := s.GetBasicScanner()
			if err != nil {
				return nil, fmt.Errorf("failed to get basic scanner: %w", err)
			}

			instance = ScannerInstance{
				ID:          uuid.New(),
				OwnerID:     userID,
				ScannerCode: basicScanner.Code,
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			if err := s.db.Create(&instance).Error; err != nil {
				return nil, fmt.Errorf("failed to create scanner instance: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query scanner instance: %w", err)
		}
	}

	// Načítaj scanner detaily
	if err := s.loadScannerDetails(&instance); err != nil {
		return nil, fmt.Errorf("failed to load scanner details: %w", err)
	}

	return &instance, nil
}

// loadScannerDetails - načíta scanner catalog a moduly
func (s *Service) loadScannerDetails(instance *ScannerInstance) error {
	// Načítaj scanner z katalógu
	scanner, err := s.GetScannerByCode(instance.ScannerCode)
	if err != nil {
		return err
	}
	instance.Scanner = scanner

	// Načítaj inštalované moduly
	var modules []ScannerModule
	if err := s.db.Where("instance_id = ?", instance.ID).Find(&modules).Error; err != nil {
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

	stats := &ScannerStats{
		RangeM:           instance.Scanner.BaseRangeM,
		FovDeg:           instance.Scanner.BaseFovDeg,
		ServerPollHz:     instance.Scanner.CapsJSON.ScanConfig.ServerPollHz,
		LockOnThreshold:  5.0, // Základný lock-on threshold
		EnergyCap:        100, // Základná energia
		VisualStyle:      instance.Scanner.CapsJSON.Visual.Style,
		ScanMode:         instance.Scanner.CapsJSON.ScanConfig.Mode,
		ClientTickHz:     instance.Scanner.CapsJSON.ScanConfig.ClientTickHz,
		SeeMaxRarity:     instance.Scanner.CapsJSON.Limits.SeeMaxRarity,
		CollectMaxRarity: instance.Scanner.CapsJSON.Limits.CollectMaxRarity,
	}

	// Aplikuj moduly
	for _, module := range instance.Modules {
		if module.Module != nil {
			s.applyModuleEffects(stats, module.Module)
		}
	}

	return stats, nil
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
	instance, err := s.GetOrCreateScannerInstance(userID)
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

// getActiveZoneID - získa ID aktívnej zóny hráča
func (s *Service) getActiveZoneID(userID uuid.UUID) (string, error) {
	// Skontroluj Redis session
	sessionKey := fmt.Sprintf("user_session:%s", userID.String())
	zoneID, err := s.redis.Get(context.Background(), sessionKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("no active zone session")
		}
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	return zoneID, nil
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

	// Filtruj artefakty
	if scanner.DetectArtifacts {
		for _, artifact := range artifacts {
			if s.canDetectItem(artifact.Rarity, scanner.CapsJSON.Limits.SeeMaxRarity) {
				result := s.createScanResult(&artifact, req, stats)
				if result != nil {
					results = append(results, *result)
				}
			}
		}
	}

	// Filtruj gear
	if scanner.DetectGear {
		for _, g := range gear {
			// Gear nemá rarity, použijeme "common" ako default
			if s.canDetectItem("common", scanner.CapsJSON.Limits.SeeMaxRarity) {
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
