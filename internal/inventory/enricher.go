package inventory

import (
	"fmt"
	"strings"

	"geoanomaly/internal/game"
	"geoanomaly/internal/scanner"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InventoryItemDTO - DTO pre enriched inventory items
type InventoryItemDTO struct {
	ID          uuid.UUID              `json:"id"`
	ItemID      uuid.UUID              `json:"item_id"`
	ItemType    string                 `json:"item_type"`
	DisplayName string                 `json:"display_name"`
	ImageURL    *string                `json:"image_url,omitempty"`
	Properties  map[string]interface{} `json:"properties"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// Enricher - service pre obohacovanie inventory items
type Enricher struct {
	db *gorm.DB
}

// NewEnricher vytvorí novú inštanciu Enricher
func NewEnricher(db *gorm.DB) *Enricher {
	return &Enricher{db: db}
}

// EnrichItem obohatí inventory item o display_name a image_url
func (e *Enricher) EnrichItem(dto *InventoryItemDTO) {
	if dto.Properties == nil {
		dto.Properties = make(map[string]interface{})
	}

	// 1) display_name z properties
	if dto.DisplayName == "" {
		if v, ok := dto.Properties["display_name"].(string); ok && v != "" {
			dto.DisplayName = v
		}
	}

	// 2) podľa typu z katalógu
	if dto.DisplayName == "" {
		switch dto.ItemType {
		case "artifact":
			if name := e.lookupArtifactName(dto); name != "" {
				dto.DisplayName = name
			}
		case "gear":
			if name := e.lookupGearName(dto); name != "" {
				dto.DisplayName = name
			}
		}
	}

	// 3) posledný fallback: prettify z kódu
	if dto.DisplayName == "" {
		if code, _ := dto.Properties["scanner_code"].(string); code != "" {
			dto.DisplayName = e.prettifyCode(code)
		} else if t, _ := dto.Properties["type"].(string); t != "" {
			dto.DisplayName = e.prettifyCode(t)
		} else {
			dto.DisplayName = "Unknown Item"
		}
	}

	// 4) image_url fallback, ak stále chýba
	if dto.ImageURL == nil {
		if dto.ItemType == "artifact" {
			if artifactType, exists := dto.Properties["type"].(string); exists {
				url := fmt.Sprintf("/api/v1/media/artifact/%s", artifactType)
				dto.ImageURL = &url
			}
		} else if dto.ItemType == "gear" {
			if gearType, exists := dto.Properties["type"].(string); exists {
				url := fmt.Sprintf("/api/v1/media/gear/%s", gearType)
				dto.ImageURL = &url
			}
		}
	}
}

// lookupArtifactName vyhľadá názov artefaktu z game.artifacts.go
func (e *Enricher) lookupArtifactName(dto *InventoryItemDTO) string {
	if artifactType, exists := dto.Properties["type"].(string); exists {
		return game.GetArtifactDisplayName(artifactType)
	}
	return ""
}

// lookupGearName vyhľadá názov gear z game.gear.go
func (e *Enricher) lookupGearName(dto *InventoryItemDTO) string {
	if gearType, exists := dto.Properties["type"].(string); exists {
		return game.GetGearDisplayName(gearType)
	}
	return ""
}

// lookupScannerNameAndIcon vyhľadá názov a ikonu scanner z katalógu
func (e *Enricher) lookupScannerNameAndIcon(dto *InventoryItemDTO) (string, string) {
	var scanner scanner.ScannerCatalog

	// Skús nájsť podľa item_id
	if err := e.db.Where("id = ?", dto.ItemID).First(&scanner).Error; err == nil {
		iconURL := fmt.Sprintf("/api/v1/media/scanner/%s", scanner.Code)
		return scanner.Name, iconURL
	}

	// Skús nájsť podľa scanner_code v properties
	if code, exists := dto.Properties["scanner_code"].(string); exists {
		if err := e.db.Where("code = ?", code).First(&scanner).Error; err == nil {
			iconURL := fmt.Sprintf("/api/v1/media/scanner/%s", scanner.Code)
			return scanner.Name, iconURL
		}
	}

	return "", ""
}

// lookupPowerCellNameAndIcon vyhľadá názov a ikonu power cell z katalógu
func (e *Enricher) lookupPowerCellNameAndIcon(dto *InventoryItemDTO) (string, string) {
	var powerCell scanner.PowerCellCatalog

	// Skús nájsť podľa item_id (ak je to UUID)
	if dto.ItemID != uuid.Nil {
		if err := e.db.Where("code = ?", dto.ItemID.String()).First(&powerCell).Error; err == nil {
			iconURL := fmt.Sprintf("/api/v1/media/power_cell/%s", powerCell.Code)
			return powerCell.Name, iconURL
		}
	}

	// Skús nájsť podľa battery_code v properties
	if code, exists := dto.Properties["battery_code"].(string); exists {
		if err := e.db.Where("code = ?", code).First(&powerCell).Error; err == nil {
			iconURL := fmt.Sprintf("/api/v1/media/power_cell/%s", powerCell.Code)
			return powerCell.Name, iconURL
		}
	}

	return "", ""
}

// prettifyCode prettify kód na čitateľný názov
func (e *Enricher) prettifyCode(code string) string {
	// Odstráň podčiarkovník a nahraď medzerami
	prettified := strings.ReplaceAll(code, "_", " ")

	// Capitalize každé slovo
	words := strings.Fields(prettified)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}
