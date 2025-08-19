package media

import (
	"context"
	"fmt"
	"io"
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
