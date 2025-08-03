package media

import (
	"geoanomaly/internal/game"
	"log"
)

func (s *Service) GetArtifactImage(artifactType string) (string, bool) {
	// ✅ POUŽÍVAME CENTRALIZOVANÚ FUNKCIU Z /game/artifacts.go
	filename := game.GetArtifactImageFilename(artifactType)
	
	// ✅ PRIDANÉ: Debug logging
	log.Printf("🔍 GetArtifactImage: type='%s', filename='%s'",
		artifactType, filename)

	// ✅ Kontrola či filename nie je fallback
	if filename == "default_artifact.jpg" {
		log.Printf("⚠️  Artifact type '%s' not found in mapping", artifactType)
		return "", false
	}

	return filename, true
}
