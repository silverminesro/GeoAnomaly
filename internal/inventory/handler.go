package inventory

import "gorm.io/gorm"

type Handler struct {
	db       *gorm.DB
	enricher *Enricher
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		db:       db,
		enricher: NewEnricher(db),
	}
}
