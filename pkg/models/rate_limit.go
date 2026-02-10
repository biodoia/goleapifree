package models

import (
	"time"

	"github.com/google/uuid"
)

// LimitType specifica il tipo di limite
type LimitType string

const (
	LimitTypeRPM        LimitType = "rpm"  // Requests per minute
	LimitTypeRPH        LimitType = "rph"  // Requests per hour
	LimitTypeRPD        LimitType = "rpd"  // Requests per day
	LimitTypeTPM        LimitType = "tpm"  // Tokens per minute
	LimitTypeTPD        LimitType = "tpd"  // Tokens per day
	LimitTypeConcurrent LimitType = "concurrent" // Concurrent requests
)

// RateLimit rappresenta un limite di rate per un provider
type RateLimit struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	ProviderID uuid.UUID `json:"provider_id" gorm:"type:uuid;not null;index"`

	LimitType     LimitType `json:"limit_type" gorm:"not null"`
	LimitValue    int       `json:"limit_value" gorm:"not null"`
	ResetInterval int       `json:"reset_interval"` // In seconds (0 for daily reset)

	// Relations
	Provider Provider `json:"provider" gorm:"foreignKey:ProviderID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook
func (r *RateLimit) BeforeCreate() error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName specifica il nome della tabella
func (RateLimit) TableName() string {
	return "rate_limits"
}
