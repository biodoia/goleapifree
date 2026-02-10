package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Modality rappresenta la modalità del modello
type Modality string

const (
	ModalityChat       Modality = "chat"
	ModalityCompletion Modality = "completion"
	ModalityEmbedding  Modality = "embedding"
	ModalityAudio      Modality = "audio"
	ModalityVideo      Modality = "video"
	ModalityImage      Modality = "image"
	ModalityRealtime   Modality = "realtime"
	ModalityMultimodal Modality = "multimodal"
)

// Model rappresenta un modello LLM
type Model struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	ProviderID uuid.UUID `json:"provider_id" gorm:"type:uuid;not null;index"`

	Name          string   `json:"name" gorm:"not null;index"`
	DisplayName   string   `json:"display_name"`
	Modality      Modality `json:"modality" gorm:"not null;index"`

	// Specifications
	ContextLength    int     `json:"context_length"`
	MaxOutputTokens  int     `json:"max_output_tokens"`
	InputPricePer1k  float64 `json:"input_price_per_1k" gorm:"default:0.0"`
	OutputPricePer1k float64 `json:"output_price_per_1k" gorm:"default:0.0"`

	// Capabilities (JSON)
	Capabilities datatypes.JSON `json:"capabilities" gorm:"type:jsonb"`
	// Example: {"streaming": true, "tools": true, "vision": true, "json_mode": true}

	// Quality metrics
	QualityScore float64 `json:"quality_score" gorm:"default:0.5"` // 0.0-1.0
	SpeedScore   float64 `json:"speed_score" gorm:"default:0.5"`   // 0.0-1.0

	// Metadata
	Description string         `json:"description"`
	Tags        datatypes.JSON `json:"tags" gorm:"type:jsonb"` // ["coding", "creative", "fast"]

	// Relations
	Provider Provider `json:"provider" gorm:"foreignKey:ProviderID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook
func (m *Model) BeforeCreate() error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// IsFree verifica se il modello è gratuito
func (m *Model) IsFree() bool {
	return m.InputPricePer1k == 0.0 && m.OutputPricePer1k == 0.0
}

// EstimateCost stima il costo per una richiesta
func (m *Model) EstimateCost(inputTokens, outputTokens int) float64 {
	inputCost := (float64(inputTokens) / 1000.0) * m.InputPricePer1k
	outputCost := (float64(outputTokens) / 1000.0) * m.OutputPricePer1k
	return inputCost + outputCost
}

// TableName specifica il nome della tabella
func (Model) TableName() string {
	return "models"
}
