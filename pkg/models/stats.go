package models

import (
	"time"

	"github.com/google/uuid"
)

// ProviderStats rappresenta statistiche aggregate per un provider
type ProviderStats struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	ProviderID uuid.UUID `json:"provider_id" gorm:"type:uuid;not null;index"`
	Timestamp  time.Time `json:"timestamp" gorm:"index;not null"`

	// Metrics
	SuccessRate   float64 `json:"success_rate"`   // 0.0-1.0
	AvgLatencyMs  int     `json:"avg_latency_ms"`
	TotalRequests int64   `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	CostSaved     float64 `json:"cost_saved"` // vs official API pricing

	// Error tracking
	ErrorCount    int64  `json:"error_count"`
	TimeoutCount  int64  `json:"timeout_count"`
	QuotaExhausted int64 `json:"quota_exhausted"`

	// Relations
	Provider Provider `json:"provider" gorm:"foreignKey:ProviderID"`

	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate hook
func (s *ProviderStats) BeforeCreate() error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}
	return nil
}

// TableName specifica il nome della tabella
func (ProviderStats) TableName() string {
	return "provider_stats"
}

// RequestLog rappresenta un singolo log di richiesta
type RequestLog struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	ProviderID uuid.UUID `json:"provider_id" gorm:"type:uuid;not null;index"`
	ModelID    uuid.UUID `json:"model_id" gorm:"type:uuid;index"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;index"`

	// Request details
	Method       string    `json:"method"`
	Endpoint     string    `json:"endpoint"`
	StatusCode   int       `json:"status_code"`
	LatencyMs    int       `json:"latency_ms"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`

	// Success tracking
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message"`

	// Cost
	EstimatedCost float64  `json:"estimated_cost"`

	Timestamp time.Time `json:"timestamp" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate hook
func (r *RequestLog) BeforeCreate() error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}
	return nil
}

// TableName specifica il nome della tabella
func (RequestLog) TableName() string {
	return "request_logs"
}
