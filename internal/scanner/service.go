package scanner

import (
	"fmt"
	"log"
	"math"
	"math/rand"
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

// GetBasicScanner - vráti základný scanner pre hráča
func (s *Service) GetBasicScanner() (*ScannerCatalog, error) {
	// TODO: Implement with GORM when scanner tables are migrated
	return &ScannerCatalog{
		Code:        "echovane_mk0",
		Name:        "EchoVane Mk.0",
		Tagline:     "Základný sektorový skener",
		Description: "Minimalistický ručný pinger s 30° zorným klinom. Vždy ťa vedie k najbližšiemu nálezu.",
		BaseRangeM:  100,
		BaseFovDeg:  30,
		CapsJSON: ScannerCaps{
			RangePctMax:     40,
			FovPctMax:       50,
			ServerPollHzMax: 2.0,
		},
		DrainMult: 1.0,
		IsBasic:   true,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
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

	// TODO: Implement module calculation with GORM when scanner tables are migrated
	// For now return basic stats
	stats := &ScannerStats{
		RangeM:          instance.Scanner.BaseRangeM,
		FovDeg:          instance.Scanner.BaseFovDeg,
		ServerPollHz:    1.0,  // základná hodnota
		LockOnThreshold: 0.85, // základná hodnota
		EnergyCap:       100,  // basic energy cap
	}

	return stats, nil
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
	return s.findItemsInZone(activeZone.ID, lat, lon, heading, stats)
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
func (s *Service) findItemsInZone(zoneID uuid.UUID, lat, lon, heading float64, stats *ScannerStats) ([]ScanResult, error) {
	var results []ScanResult

	// 1. Nájdi artefakty v zóne
	var artifacts []common.Artifact
	if err := s.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&artifacts).Error; err != nil {
		log.Printf("🔍 [SCANNER] Failed to load artifacts for zone %s: %v", zoneID, err)
		return results, nil
	}

	// 2. Nájdi gear items v zóne
	var gear []common.Gear
	if err := s.db.Where("zone_id = ? AND is_active = true", zoneID).Find(&gear).Error; err != nil {
		log.Printf("🔍 [SCANNER] Failed to load gear for zone %s: %v", zoneID, err)
		return results, nil
	}

	log.Printf("🔍 [SCANNER] Zone %s has %d artifacts and %d gear items", zoneID, len(artifacts), len(gear))

	// 3. Spracuj artefakty
	for _, artifact := range artifacts {
		distance := s.calculateDistance(lat, lon, artifact.Location.Latitude, artifact.Location.Longitude)

		// Len items do 50m
		if distance > 50 {
			continue
		}

		bearing := s.calculateBearing(lat, lon, artifact.Location.Latitude, artifact.Location.Longitude)

		// Základný signal strength
		signalStrength := s.calculateSignalStrength(distance, stats.RangeM, bearing, heading, float64(stats.FovDeg))

		// Pridaj rušenie - čím ďalej, tým väčšie rušenie
		signalStrength = s.addSignalNoise(signalStrength, distance)

		results = append(results, ScanResult{
			Type:           "artifact",
			DistanceM:      distance,
			BearingDeg:     bearing,
			SignalStrength: signalStrength,
			Name:           artifact.Name,
			Rarity:         artifact.Rarity,
			ItemID:         &artifact.ID,
		})
	}

	// 4. Spracuj gear items
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
		signalStrength = s.addSignalNoise(signalStrength, distance)

		results = append(results, ScanResult{
			Type:           "gear",
			DistanceM:      distance,
			BearingDeg:     bearing,
			SignalStrength: signalStrength,
			Name:           gearItem.Name,
			Rarity:         "common", // Gear nemá rarity v databáze, použijeme common
			ItemID:         &gearItem.ID,
		})
	}

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
