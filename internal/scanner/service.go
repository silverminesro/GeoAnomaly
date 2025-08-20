package scanner

import (
	"fmt"
	"log"
	"math"
	"time"

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
	scanResults, err := s.findItemsInRange(req.Latitude, req.Longitude, req.Heading, stats)
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
func (s *Service) findItemsInRange(lat, lon, heading float64, stats *ScannerStats) ([]ScanResult, error) {
	// TODO: Implement with GORM when scanner tables are migrated
	// For now return mock results
	var results []ScanResult

	// Mock some sample results for testing
	// Artifact result
	if len(results) < 3 {
		results = append(results, ScanResult{
			Type:           "artifact",
			DistanceM:      int(float64(stats.RangeM) * 0.3), // 30% of max range
			BearingDeg:     45.0,
			SignalStrength: 0.8,
			Name:           "Anomalous Crystal",
			Rarity:         "rare",
		})

		// Gear result
		results = append(results, ScanResult{
			Type:           "gear",
			DistanceM:      int(float64(stats.RangeM) * 0.7), // 70% of max range
			BearingDeg:     135.0,
			SignalStrength: 0.4,
			Name:           "Quantum Detector",
			Rarity:         "common",
		})
	}

	return results, nil
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
