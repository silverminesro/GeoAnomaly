package media

import (
	"geoanomaly/internal/game"
	"log"
)

func (s *Service) GetGearImage(gearType string) (string, bool) {
	// ✅ POUŽÍVAME CENTRALIZOVANÚ FUNKCIU Z /game/gear.go
	filename := game.GetGearImageFilename(gearType)
	
	// ✅ PRIDANÉ: Debug logging
	log.Printf("🔍 GetGearImage: type='%s', filename='%s'",
		gearType, filename)

	// ✅ Kontrola či filename nie je fallback
	if filename == "default_gear.jpg" {
		log.Printf("⚠️  Gear type '%s' not found in mapping", gearType)
		return "", false
	}

	return filename, true
}
