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
	"io"
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

// GetBasicScanner - vráti základný scanner pre hráča
func (s *Service) GetBasicScanner() (*ScannerCatalog, error) {
	var scanner ScannerCatalog

	// Načítaj scanner z databázy
	if err := s.db.Where("code = ? AND is_basic = true", "echovane_mk0").First(&scanner).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Fallback na hardcoded hodnoty ak scanner neexistuje v DB
			return &ScannerCatalog{
				Code:        "echovane_mk0",
				Name:        "EchoVane Mk.0",
				Tagline:     "Základný sektorový skener",
				Description: "Minimalistický ručný pinger s 30° zorným klinom. Vždy ťa vedie k najbližšiemu nálezu.",
				BaseRangeM:  50,
				BaseFovDeg:  30,
				CapsJSON: ScannerCaps{
					RangePctMax:     40,
					FovPctMax:       50,
					ServerPollHzMax: 2.0,
				},
				DrainMult:       1.0,
				AllowedModules:  StringArray{"mod_range_i", "mod_fov_i", "mod_response_i"},
				SlotCount:       3,
				SlotTypes:       StringArray{"power", "range", "fov"},
				IsBasic:         true,
				MaxRarity:       "rare", // Základný scanner môže detekovať len common a rare
				DetectArtifacts: true,   // Môže detekovať artefakty
				DetectGear:      true,   // Môže detekovať gear
				Version:         1,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to load scanner from database: %w", err)
	}

	return &scanner, nil
}

// GetOrCreateScannerInstance - vráti alebo vytvorí scanner inštanciu pre hráča
func (s *Service) GetOrCreateScannerInstance(userID uuid.UUID) (*ScannerInstance, error) {
	// TODO: Implement with GORM when scanner tables are migrated
	// For now return mock instance
	basicScanner, err := s.GetBasicScanner()
	if err != nil {
		return nil, err
	}

	instance := &ScannerInstance{
		ID:          uuid.New(),
		OwnerID:     userID,
		ScannerCode: basicScanner.Code,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Scanner:     basicScanner,
		Modules:     []ScannerModule{}, // prázdne moduly
	}

	return instance, nil
}

// loadScannerDetails - načíta scanner catalog a moduly
func (s *Service) loadScannerDetails(instance *ScannerInstance) error {
	// TODO: Implement with GORM when scanner tables are migrated
	// For now just return as is
	return nil
}

// CalculateScannerStats - vypočíta efektívne stats scanner
func (s *Service) CalculateScannerStats(instance *ScannerInstance) (*ScannerStats, error) {
	if instance.Scanner == nil {
		return nil, fmt.Errorf("scanner catalog not loaded")
	}

	// Základné stats
	stats := &ScannerStats{
		RangeM:          instance.Scanner.BaseRangeM,
		FovDeg:          instance.Scanner.BaseFovDeg,
		ServerPollHz:    1.0,  // základná hodnota
		LockOnThreshold: 0.85, // základná hodnota
		EnergyCap:       100,  // basic energy cap
	}

	// Puls Scanner specific stats
	if s.isPulsScanner(instance.Scanner.Code) {
		stats = s.calculatePulsScannerStats(instance, stats)
	}

	// TODO: Implement module calculation with GORM when scanner tables are migrated
	// For now return basic stats

	return stats, nil
}

// isPulsScanner - kontroluje či je scanner puls typ
func (s *Service) isPulsScanner(scannerCode string) bool {
	return scannerCode == "puls_mk0" || scannerCode == "puls_mk1" || scannerCode == "puls_mk2"
}

// calculatePulsScannerStats - vypočíta stats pre puls scanner
func (s *Service) calculatePulsScannerStats(instance *ScannerInstance, baseStats *ScannerStats) *ScannerStats {
	stats := *baseStats // Copy base stats

	// Pridaj puls-specific capabilities
	if instance.Scanner.CapsJSON.WaveDurationMs != nil {
		stats.WaveDurationMs = instance.Scanner.CapsJSON.WaveDurationMs
	}
	if instance.Scanner.CapsJSON.EchoDelayMs != nil {
		stats.EchoDelayMs = instance.Scanner.CapsJSON.EchoDelayMs
	}
	if instance.Scanner.CapsJSON.MaxWaves != nil {
		stats.MaxWaves = instance.Scanner.CapsJSON.MaxWaves
	}
	if instance.Scanner.CapsJSON.WaveSpeedMs != nil {
		stats.WaveSpeedMs = instance.Scanner.CapsJSON.WaveSpeedMs
	}
	if instance.Scanner.CapsJSON.NoiseLevel != nil {
		stats.NoiseLevel = instance.Scanner.CapsJSON.NoiseLevel
	}
	if instance.Scanner.CapsJSON.RealTimeCapable != nil {
		stats.RealTimeCapable = instance.Scanner.CapsJSON.RealTimeCapable
	}
	if instance.Scanner.CapsJSON.AdvancedEcho != nil {
		stats.AdvancedEcho = instance.Scanner.CapsJSON.AdvancedEcho
	}
	if instance.Scanner.CapsJSON.NoiseFilter != nil {
		stats.NoiseFilter = instance.Scanner.CapsJSON.NoiseFilter
	}

	return &stats
}

// Scan - vykoná skenovanie
func (s *Service) Scan(userID uuid.UUID, req *ScanRequest) (*ScanResponse, error) {
	// Získaj scanner inštanciu
	instance, err := s.GetOrCreateScannerInstance(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get scanner instance: %w", err)
	}

	// Vypočítať stats
	stats, err := s.CalculateScannerStats(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate scanner stats: %w", err)
	}

	// Hľadaj items v dosahu
	scanResults, err := s.findItemsInRange(userID, req.Latitude, req.Longitude, req.Heading, stats)
	if err != nil {
		return nil, fmt.Errorf("failed to find items: %w", err)
	}

	response := &ScanResponse{
		Success:      true,
		ScanResults:  scanResults,
		ScannerStats: stats,
	}

	log.Printf("🔍 [SCANNER] User %s scanned at (%.6f, %.6f) heading %.1f° - found %d items",
		userID, req.Latitude, req.Longitude, req.Heading, len(scanResults))

	return response, nil
}

// findItemsInRange - nájde items v dosahu scanner
func (s *Service) findItemsInRange(userID uuid.UUID, lat, lon, heading float64, stats *ScannerStats) ([]ScanResult, error) {
	// 1. Skontroluj či je hráč v aktívnej zóne
	activeZone, err := s.getActiveZoneForPlayer(userID)
	if err != nil || activeZone == nil {
		// Hráč nie je v zóne - scanner vyžaduje enter zone
		log.Printf("🔍 [SCANNER] User %s scanned outside of active zone - must enter zone first", userID)
		return nil, fmt.Errorf("must enter zone first to use scanner")
	}

	// 2. Hráč je v zóne - hľadaj items v zóne
	log.Printf("🔍 [SCANNER] User %s scanning in zone %s", userID, activeZone.ID)

	// Získaj scanner inštanciu pre detaily
	scannerInstance, err := s.GetOrCreateScannerInstance(userID)
	if err != nil {
		log.Printf("🔍 [SCANNER] Failed to get scanner instance: %v", err)
		return s.findItemsInZone(activeZone.ID, lat, lon, heading, stats, nil)
	}

	return s.findItemsInZone(activeZone.ID, lat, lon, heading, stats, scannerInstance)
}

// getActiveZoneForPlayer - získa aktívnu zónu pre hráča
func (s *Service) getActiveZoneForPlayer(userID uuid.UUID) (*common.Zone, error) {
	// Skontroluj PlayerSession pre aktuálnu zónu
	var session common.PlayerSession
	if err := s.db.Where("user_id = ?", userID).First(&session).Error; err != nil {
		log.Printf("🔍 [SCANNER] User %s has no active session", userID)
		return nil, nil // Hráč nie je v zóne
	}

	if session.CurrentZone == nil {
		log.Printf("🔍 [SCANNER] User %s is not in any zone", userID)
		return nil, nil // Hráč nie je v zóne
	}

	// Skontroluj či zóna existuje a je aktívna
	var zone common.Zone
	if err := s.db.Where("id = ? AND is_active = true", session.CurrentZone).First(&zone).Error; err != nil {
		log.Printf("🔍 [SCANNER] User %s zone %s not found or inactive", userID, session.CurrentZone)
		return nil, nil // Zóna neexistuje alebo nie je aktívna
	}

	log.Printf("🔍 [SCANNER] User %s is in active zone %s (%s)", userID, zone.ID, zone.Name)
	return &zone, nil
}

// findItemsInZone - nájde items v zóne
func (s *Service) findItemsInZone(zoneID uuid.UUID, lat, lon, heading float64, stats *ScannerStats, scannerInstance *ScannerInstance) ([]ScanResult, error) {
	var results []ScanResult

	// Získaj scanner schopnosti
	scannerMaxRarity := "common" // Default
	detectArtifacts := true      // Default
	detectGear := true           // Default

	if scannerInstance != nil && scannerInstance.Scanner != nil {
		scannerMaxRarity = scannerInstance.Scanner.MaxRarity
		detectArtifacts = scannerInstance.Scanner.DetectArtifacts
		detectGear = scannerInstance.Scanner.DetectGear
	}

	log.Printf("🔍 [SCANNER] Scanner capabilities - MaxRarity: %s, DetectArtifacts: %v, DetectGear: %v",
		scannerMaxRarity, detectArtifacts, detectGear)

	// 1. Nájdi artefakty v zóne (ak scanner môže detekovať artefakty)
	if detectArtifacts {
		var artifacts []common.Artifact
		if err := s.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&artifacts).Error; err != nil {
			log.Printf("🔍 [SCANNER] Failed to load artifacts for zone %s: %v", zoneID, err)
		} else {
			log.Printf("🔍 [SCANNER] Found %d artifacts in zone", len(artifacts))

			// Spracuj artefakty
			for _, artifact := range artifacts {
				// Skontroluj či scanner môže detekovať túto rarity
				if !s.canDetectRarity(scannerMaxRarity, artifact.Rarity) {
					log.Printf("🔍 [SCANNER] Skipping %s artifact (rarity: %s, scanner max: %s)",
						artifact.Name, artifact.Rarity, scannerMaxRarity)
					continue
				}

				distance := s.calculateDistance(lat, lon, artifact.Location.Latitude, artifact.Location.Longitude)

				// Len items do 50m
				if distance > 50 {
					continue
				}

				bearing := s.calculateBearing(lat, lon, artifact.Location.Latitude, artifact.Location.Longitude)

				// Základný signal strength
				signalStrength := s.calculateSignalStrength(distance, stats.RangeM, bearing, heading, float64(stats.FovDeg))

				// Pridaj rušenie - čím ďalej, tým väčšie rušenie
				signalStrength = s.addSignalNoise(signalStrength, int(distance))

				results = append(results, ScanResult{
					Type:           "artifact",
					DistanceM:      distance,
					BearingDeg:     bearing,
					SignalStrength: signalStrength,
					Name:           artifact.Name,
					Rarity:         artifact.Rarity,
					ItemID:         &artifact.ID,
				})

				log.Printf("🔍 [SCANNER] Detected artifact: %s (rarity: %s, distance: %dm)",
					artifact.Name, artifact.Rarity, distance)
			}
		}
	}

	// 2. Nájdi gear items v zóne (ak scanner môže detekovať gear)
	if detectGear {
		var gear []common.Gear
		if err := s.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&gear).Error; err != nil {
			log.Printf("🔍 [SCANNER] Failed to load gear for zone %s: %v", zoneID, err)
		} else {
			log.Printf("🔍 [SCANNER] Found %d gear items in zone", len(gear))

			// Spracuj gear items
			for _, gearItem := range gear {
				distance := s.calculateDistance(lat, lon, gearItem.Location.Latitude, gearItem.Location.Longitude)

				// Len items do 50m
				if distance > 50 {
					continue
				}

				bearing := s.calculateBearing(lat, lon, gearItem.Location.Latitude, gearItem.Location.Longitude)

				// Základný signal strength
				signalStrength := s.calculateSignalStrength(distance, stats.RangeM, bearing, heading, float64(stats.FovDeg))

				// Pridaj rušenie - čím ďalej, tým väčšie rušenie
				signalStrength = s.addSignalNoise(signalStrength, int(distance))

				results = append(results, ScanResult{
					Type:           "gear",
					DistanceM:      distance,
					BearingDeg:     bearing,
					SignalStrength: signalStrength,
					Name:           gearItem.Name,
					Rarity:         "common", // Gear nemá rarity v databáze, použijeme common
					ItemID:         &gearItem.ID,
				})

				log.Printf("🔍 [SCANNER] Detected gear: %s (distance: %dm)", gearItem.Name, distance)
			}
		}
	}

	log.Printf("🔍 [SCANNER] Total items detected: %d", len(results))
	return results, nil
}

// addSignalNoise - pridá rušenie do signal strength
func (s *Service) addSignalNoise(signalStrength float64, distanceM int) float64 {
	// Čím ďalej, tým väčšie rušenie (0% na 0m, 100% na 50m)
	noiseFactor := float64(distanceM) / 50.0

	// Náhodné rušenie ±20%
	noise := (rand.Float64() - 0.5) * 0.4 * noiseFactor

	// Aplikuj rušenie
	result := signalStrength + noise

	// Obmedz na 0-1
	return math.Max(0.0, math.Min(1.0, result))
}

// canDetectRarity - skontroluje či scanner môže detekovať danú rarity
func (s *Service) canDetectRarity(scannerMaxRarity, itemRarity string) bool {
	// Rarity hierarchy (od najnižšej po najvyššiu)
	rarityLevels := map[string]int{
		"common":    0,
		"rare":      1,
		"epic":      2,
		"legendary": 3,
	}

	scannerLevel, scannerExists := rarityLevels[scannerMaxRarity]
	itemLevel, itemExists := rarityLevels[itemRarity]

	if !scannerExists || !itemExists {
		return false // Neznáme rarity
	}

	// Scanner môže detekovať item ak je jeho max rarity >= item rarity
	return scannerLevel >= itemLevel
}

// Helper functions remain the same

// calculateDistance - vypočíta vzdialenosť v metroch
func (s *Service) calculateDistance(lat1, lon1, lat2, lon2 float64) int {
	const R = 6371000 // polomer Zeme v metroch

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return int(R * c)
}

// calculateBearing - vypočíta bearing v stupňoch
func (s *Service) calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	y := math.Sin(deltaLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) -
		math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLon)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

// isInFieldOfView - skontroluje či je item v zornom poli
func (s *Service) isInFieldOfView(bearingDeg, headingDeg, fovDeg float64) bool {
	diff := math.Abs(bearingDeg - headingDeg)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff <= float64(fovDeg)/2
}

// calculateSignalStrength - vypočíta silu signálu (0-1)
func (s *Service) calculateSignalStrength(distanceM, maxRangeM int, bearingDeg, headingDeg, fovDeg float64) float64 {
	// Vzdialenosť factor (1 na 0m, 0 na maxRangeM)
	distanceFactor := math.Max(0, 1-float64(distanceM)/float64(maxRangeM))

	// FOV factor (1 v strede, 0 na okrajoch)
	diff := math.Abs(bearingDeg - headingDeg)
	if diff > 180 {
		diff = 360 - diff
	}
	fovFactor := math.Max(0, 1-diff/(float64(fovDeg)/2))

	// Kombinovaný signal strength
	signalStrength := distanceFactor * fovFactor

	// Aplikuj nelineárnu krivku
	return math.Pow(signalStrength, 0.8)
}

// GetSecureZoneData returns encrypted zone data for client-side processing
func (s *Service) GetSecureZoneData(zoneID string, userID string) (*SecureZoneData, error) {
	// Get all artifacts and gear in the zone
	artifacts, err := s.getAllArtifactsInZone(zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifacts: %w", err)
	}

	gear, err := s.getAllGearInZone(zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gear: %w", err)
	}

	// Create zone artifacts data
	zoneData := ZoneArtifacts{
		Artifacts: artifacts,
		Gear:      gear,
		ZoneID:    zoneID,
		Timestamp: time.Now(),
	}

	// Encrypt the data
	encryptedData, err := s.encryptZoneData(zoneData, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt zone data: %w", err)
	}

	// Create session token
	sessionToken := s.createSessionToken(userID, zoneID)

	// Create scan session
	session := ScanSession{
		UserID:    userID,
		ZoneID:    zoneID,
		ScanCount: 0,
		MaxScans:  50, // 50 scans per session
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10 minute session
	}

	// Store session in Redis
	if s.redis != nil {
		sessionKey := fmt.Sprintf("scan_session:%s", sessionToken)
		sessionJSON, _ := json.Marshal(session)
		err = s.redis.Set(context.Background(), sessionKey, sessionJSON, 10*time.Minute).Err()
		if err != nil {
			log.Printf("Warning: Failed to store session in Redis: %v", err)
		}
	}

	// Generate zone hash for verification
	zoneHash := s.generateZoneHash(zoneID, userID)

	return &SecureZoneData{
		EncryptedArtifacts: encryptedData,
		ZoneHash:           zoneHash,
		SessionToken:       sessionToken,
		ExpiresAt:          session.ExpiresAt,
		MaxScans:           session.MaxScans,
		ScanCount:          0,
	}, nil
}

// encryptZoneData encrypts zone artifacts data with user-specific key
func (s *Service) encryptZoneData(data ZoneArtifacts, userID string) (string, error) {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Create user-specific encryption key
	key := s.generateUserKey(userID)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(cryptorand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, jsonData, nil)

	// Return base64 encoded
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// generateUserKey creates a deterministic key for user
func (s *Service) generateUserKey(userID string) []byte {
	// Use a combination of user ID and server secret
	secret := "GeoAnomalySecret2024" // In production, use environment variable
	data := userID + secret
	hash := sha256.Sum256([]byte(data))
	return hash[:32] // Use first 32 bytes for AES-256
}

// createSessionToken creates a unique session token
func (s *Service) createSessionToken(userID, zoneID string) string {
	data := fmt.Sprintf("%s:%s:%d", userID, zoneID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:16])
}

// generateZoneHash creates a hash for zone verification
func (s *Service) generateZoneHash(zoneID, userID string) string {
	data := fmt.Sprintf("%s:%s:%s", zoneID, userID, "GeoAnomalyZoneHash")
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:16])
}

// getAllArtifactsInZone retrieves all artifacts in a zone
func (s *Service) getAllArtifactsInZone(zoneID string) ([]common.Artifact, error) {
	var artifacts []common.Artifact
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		return nil, fmt.Errorf("invalid zone ID: %w", err)
	}
	err = s.db.Where("zone_id = ? AND is_active = true", zoneUUID).Find(&artifacts).Error
	return artifacts, err
}

// getAllGearInZone retrieves all gear items in a zone
func (s *Service) getAllGearInZone(zoneID string) ([]common.Gear, error) {
	var gear []common.Gear
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		return nil, fmt.Errorf("invalid zone ID: %w", err)
	}
	err = s.db.Where("zone_id = ? AND is_active = true", zoneUUID).Find(&gear).Error
	return gear, err
}

// ValidateClaimRequest validates a claim request and returns success
func (s *Service) ValidateClaimRequest(req ClaimRequest, userID string) (bool, error) {
	// Validate session if Redis is available
	if s.redis != nil {
		sessionKey := fmt.Sprintf("scan_session:%s", req.SessionToken)
		sessionData, err := s.redis.Get(context.Background(), sessionKey).Result()
		if err != nil {
			return false, fmt.Errorf("invalid session token")
		}

		var session ScanSession
		if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
			return false, fmt.Errorf("invalid session data")
		}

		// Check if session belongs to user
		if session.UserID != userID {
			return false, fmt.Errorf("session does not belong to user")
		}
		// Check if session zone matches claim zone
		if session.ZoneID != req.ZoneID {
			return false, fmt.Errorf("zone mismatch for session")
		}

		// Check if session is expired
		if time.Now().After(session.ExpiresAt) {
			return false, fmt.Errorf("session expired")
		}

		// Increment scan count
		session.ScanCount++
		if session.ScanCount > session.MaxScans {
			return false, fmt.Errorf("max scans exceeded for session")
		}

		// Update session in Redis
		sessionJSON, _ := json.Marshal(session)
		s.redis.Set(context.Background(), sessionKey, sessionJSON, 10*time.Minute)
	}

	// Get item position based on type
	var itemLat, itemLon float64
	var itemID string

	if req.ItemType == "artifact" {
		var artifact common.Artifact
		err := s.db.Where("id = ? AND zone_id = ? AND is_claimed = false", req.ItemID, req.ZoneID).First(&artifact).Error
		if err != nil {
			return false, fmt.Errorf("artifact not found or already claimed")
		}
		itemLat = artifact.Location.Latitude
		itemLon = artifact.Location.Longitude
		itemID = artifact.ID.String()
	} else if req.ItemType == "gear" {
		var gear common.Gear
		err := s.db.Where("id = ? AND zone_id = ? AND is_claimed = false", req.ItemID, req.ZoneID).First(&gear).Error
		if err != nil {
			return false, fmt.Errorf("gear not found or already claimed")
		}
		itemLat = gear.Location.Latitude
		itemLon = gear.Location.Longitude
		itemID = gear.ID.String()
	} else {
		return false, fmt.Errorf("invalid item type")
	}

	// Calculate distance between player and item
	distance := s.calculateDistance(req.Latitude, req.Longitude, itemLat, itemLon)

	// Allow claiming if within 25 meters
	if distance > 25 {
		return false, fmt.Errorf("too far from item (distance: %dm)", distance)
	}

	// Mark item as claimed
	if req.ItemType == "artifact" {
		err := s.db.Model(&common.Artifact{}).Where("id = ?", req.ItemID).Update("is_claimed", true).Error
		if err != nil {
			return false, fmt.Errorf("failed to mark artifact as claimed")
		}
	} else {
		err := s.db.Model(&common.Gear{}).Where("id = ?", req.ItemID).Update("is_claimed", true).Error
		if err != nil {
			return false, fmt.Errorf("failed to mark gear as claimed")
		}
	}

	// Add item to user's inventory
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user ID")
	}

	itemUUID, err2 := uuid.Parse(itemID)
	if err2 != nil {
		return false, fmt.Errorf("invalid item ID")
	}

	inventoryItem := common.InventoryItem{
		UserID:   userUUID,
		ItemType: req.ItemType,
		ItemID:   itemUUID,
		Quantity: 1,
	}

	err3 := s.db.Create(&inventoryItem).Error
	if err3 != nil {
		return false, fmt.Errorf("failed to add item to inventory")
	}

	return true, nil
}
