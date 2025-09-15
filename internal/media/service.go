package media

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

type Service struct {
	r2Client *R2Client
	cache    map[string]cachedImage // Jednoduchý in-memory cache
}

type cachedImage struct {
	data        []byte
	contentType string
	cachedAt    time.Time
}

func NewService(r2Client *R2Client) *Service {
	return &Service{
		r2Client: r2Client,
		cache:    make(map[string]cachedImage),
	}
}

// GetArtifactImageData získa dáta obrázka pre daný typ artefaktu
func (s *Service) GetArtifactImageData(ctx context.Context, artifactType string) ([]byte, string, error) {
	filename, exists := s.GetArtifactImage(artifactType)
	if !exists {
		return nil, "", fmt.Errorf("artifact type not found: %s", artifactType)
	}

	return s.GetImageData(ctx, filename)
}

// GetArtifactImageDataWithFallback získa dáta obrázka pre artifact s fallback na default
func (s *Service) GetArtifactImageDataWithFallback(ctx context.Context, artifactType string) ([]byte, string, error) {
	filename, exists := s.GetArtifactImage(artifactType)
	if !exists {
		return nil, "", fmt.Errorf("artifact type not found: %s", artifactType)
	}

	// Skontroluj cache (30 minút)
	if cached, ok := s.cache[filename]; ok {
		if time.Since(cached.cachedAt) < 30*time.Minute {
			return cached.data, cached.contentType, nil
		}
		delete(s.cache, filename) // Vymaž expirovanú cache
	}

	// Stiahni z R2 - artifact obrázky sú v artifacts/ priečinku
	key := fmt.Sprintf("artifacts/%s", filename)
	body, contentType, err := s.r2Client.GetObject(ctx, key)
	if err != nil {
		// PATCH C: fallback na default_artifact.jpg
		log.Printf("⚠️ Artifact image %s not found in R2, trying fallback to default_artifact.jpg", filename)
		
		fallbackKey := "artifacts/default_artifact.jpg"
		body, contentType, err = s.r2Client.GetObject(ctx, fallbackKey)
		if err != nil {
			// Posledná poistka - vráť default placeholder
			log.Printf("❌ Even default_artifact.jpg not found, using placeholder")
			return s.getDefaultPlaceholder(), "image/jpeg", nil
		}
	}
	defer body.Close()

	// Prečítaj dáta
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read artifact image data: %w", err)
	}

	// Ulož do cache
	s.cache[filename] = cachedImage{
		data:        data,
		contentType: contentType,
		cachedAt:    time.Now(),
	}

	return data, contentType, nil
}

// GetGearImageData získa dáta obrázka pre daný typ gear
func (s *Service) GetGearImageData(ctx context.Context, gearType string) ([]byte, string, error) {
	filename, exists := s.GetGearImage(gearType)
	if !exists {
		return nil, "", fmt.Errorf("gear type not found: %s", gearType)
	}

	// Skontroluj cache (30 minút)
	if cached, ok := s.cache[filename]; ok {
		if time.Since(cached.cachedAt) < 30*time.Minute {
			return cached.data, cached.contentType, nil
		}
		delete(s.cache, filename) // Vymaž expirovanú cache
	}

	// Stiahni z R2 - gear obrázky sú v gear/ priečinku
	key := fmt.Sprintf("gear/%s", filename)
	body, contentType, err := s.r2Client.GetObject(ctx, key)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get gear image from R2: %w", err)
	}
	defer body.Close()

	// Prečítaj dáta
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read gear image data: %w", err)
	}

	// Ulož do cache
	s.cache[filename] = cachedImage{
		data:        data,
		contentType: contentType,
		cachedAt:    time.Now(),
	}

	return data, contentType, nil
}

// GetGearImageDataWithFallback získa dáta obrázka pre gear s fallback na default
func (s *Service) GetGearImageDataWithFallback(ctx context.Context, gearType string) ([]byte, string, error) {
	filename, exists := s.GetGearImage(gearType)
	if !exists {
		return nil, "", fmt.Errorf("gear type not found: %s", gearType)
	}

	// Skontroluj cache (30 minút)
	if cached, ok := s.cache[filename]; ok {
		if time.Since(cached.cachedAt) < 30*time.Minute {
			return cached.data, cached.contentType, nil
		}
		delete(s.cache, filename) // Vymaž expirovanú cache
	}

	// Stiahni z R2 - gear obrázky sú v gear/ priečinku
	key := fmt.Sprintf("gear/%s", filename)
	body, contentType, err := s.r2Client.GetObject(ctx, key)
	if err != nil {
		// PATCH C: fallback na default_gear.jpg
		log.Printf("⚠️ Gear image %s not found in R2, trying fallback to default_gear.jpg", filename)
		
		fallbackKey := "gear/default_gear.jpg"
		body, contentType, err = s.r2Client.GetObject(ctx, fallbackKey)
		if err != nil {
			// Posledná poistka - vráť default placeholder
			log.Printf("❌ Even default_gear.jpg not found, using placeholder")
			return s.getDefaultPlaceholder(), "image/jpeg", nil
		}
	}
	defer body.Close()

	// Prečítaj dáta
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read gear image data: %w", err)
	}

	// Ulož do cache
	s.cache[filename] = cachedImage{
		data:        data,
		contentType: contentType,
		cachedAt:    time.Now(),
	}

	return data, contentType, nil
}

// getDefaultPlaceholder vráti default placeholder obrázok
func (s *Service) getDefaultPlaceholder() []byte {
	// Jednoduchý 1x1 pixel JPEG placeholder
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x01, 0x00, 0x48,
		0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
		0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20,
		0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29, 0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27,
		0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x11, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0x02, 0x11, 0x01, 0x03, 0x11, 0x01, 0xFF, 0xC4, 0x00, 0x14,
		0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x08, 0xFF, 0xC4, 0x00, 0x14, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xDA, 0x00, 0x0C, 0x03, 0x01, 0x00, 0x02,
		0x11, 0x03, 0x11, 0x00, 0x3F, 0x00, 0x8A, 0xFF, 0xD9,
	}
}

// GetImageData získa dáta obrázka z R2 (s cache)
func (s *Service) GetImageData(ctx context.Context, filename string) ([]byte, string, error) {
	// Skontroluj cache (30 minút)
	if cached, ok := s.cache[filename]; ok {
		if time.Since(cached.cachedAt) < 30*time.Minute {
			return cached.data, cached.contentType, nil
		}
		delete(s.cache, filename) // Vymaž expirovanú cache
	}

	// Stiahni z R2
	key := fmt.Sprintf("artifacts/%s", filename)
	body, contentType, err := s.r2Client.GetObject(ctx, key)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image from R2: %w", err)
	}
	defer body.Close()

	// Prečítaj dáta
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Ulož do cache
	s.cache[filename] = cachedImage{
		data:        data,
		contentType: contentType,
		cachedAt:    time.Now(),
	}

	return data, contentType, nil
}

// CleanupCache vyčistí expirované položky z cache
func (s *Service) CleanupCache() {
	now := time.Now()
	for filename, cached := range s.cache {
		if now.Sub(cached.cachedAt) > 30*time.Minute {
			delete(s.cache, filename)
		}
	}
}
