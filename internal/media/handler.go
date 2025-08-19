package media

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	// Spusti cleanup cache každých 10 minút
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			service.CleanupCache()
		}
	}()

	return &Handler{service: service}
}

// GetArtifactImage streamuje obrázok pre daný typ artefaktu
func (h *Handler) GetArtifactImage(c *gin.Context) {
	artifactType := c.Param("type")

	// ✅ PRIDANÉ: Debug logging
	log.Printf("🖼️ GetArtifactImage called with type: %s", artifactType)
	log.Printf("🔍 Request URL: %s", c.Request.URL.Path)
	log.Printf("🔍 Request method: %s", c.Request.Method)
	log.Printf("🔑 Authorization header present: %v", c.GetHeader("Authorization") != "")

	// Získaj dáta obrázka
	imageData, contentType, err := h.service.GetArtifactImageData(c.Request.Context(), artifactType)
	if err != nil {
		log.Printf("❌ Failed to get image data: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Artifact image not found",
			"type":    artifactType,
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ Image data retrieved: %d bytes, type: %s", len(imageData), contentType)

	// Nastav cache headers pre browser
	c.Header("Cache-Control", "public, max-age=3600") // 1 hodina
	c.Header("ETag", fmt.Sprintf(`"%s"`, artifactType))

	// Skontroluj If-None-Match header
	if match := c.GetHeader("If-None-Match"); match == fmt.Sprintf(`"%s"`, artifactType) {
		log.Printf("📄 Returning 304 Not Modified")
		c.Status(http.StatusNotModified)
		return
	}

	log.Printf("✅ Sending image: %d bytes", len(imageData))
	// Pošli obrázok
	c.Data(http.StatusOK, contentType, imageData)
}

// GetImage streamuje konkrétny obrázok podľa názvu súboru
func (h *Handler) GetImage(c *gin.Context) {
	filename := c.Param("filename")

	// Bezpečnostná kontrola - zabráň path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}

	// Získaj dáta obrázka
	imageData, contentType, err := h.service.GetImageData(c.Request.Context(), filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Image not found",
		})
		return
	}

	// Nastav cache headers
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("ETag", fmt.Sprintf(`"%s"`, filename))

	// Skontroluj If-None-Match
	if match := c.GetHeader("If-None-Match"); match == fmt.Sprintf(`"%s"`, filename) {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, contentType, imageData)
}

// GetGearImage streamuje obrázok pre daný typ gear
func (h *Handler) GetGearImage(c *gin.Context) {
	gearType := c.Param("type")

	// ✅ PRIDANÉ: Debug logging
	log.Printf("🖼️ GetGearImage called with type: %s", gearType)
	log.Printf("🔍 Request URL: %s", c.Request.URL.Path)
	log.Printf("🔍 Request method: %s", c.Request.Method)
	log.Printf("🔑 Authorization header present: %v", c.GetHeader("Authorization") != "")

	// Získaj dáta obrázka
	imageData, contentType, err := h.service.GetGearImageData(c.Request.Context(), gearType)
	if err != nil {
		log.Printf("❌ Failed to get gear image data: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Gear image not found",
			"type":    gearType,
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ Gear image data retrieved: %d bytes, type: %s", len(imageData), contentType)

	// Nastav cache headers pre browser
	c.Header("Cache-Control", "public, max-age=3600") // 1 hodina
	c.Header("ETag", fmt.Sprintf(`"%s"`, gearType))

	// Skontroluj If-None-Match header
	if match := c.GetHeader("If-None-Match"); match == fmt.Sprintf(`"%s"`, gearType) {
		log.Printf("📄 Returning 304 Not Modified")
		c.Status(http.StatusNotModified)
		return
	}

	log.Printf("✅ Sending gear image: %d bytes", len(imageData))
	// Pošli obrázok
	c.Data(http.StatusOK, contentType, imageData)
}
