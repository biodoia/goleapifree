package models

import (
	"time"

	"github.com/google/uuid"
)

// ProviderType categorizza i provider
type ProviderType string

const (
	ProviderTypeFree     ProviderType = "free"
	ProviderTypeFreemium ProviderType = "freemium"
	ProviderTypePaid     ProviderType = "paid"
	ProviderTypeLocal    ProviderType = "local"
)

// ProviderStatus indica lo stato operativo
type ProviderStatus string

const (
	ProviderStatusActive     ProviderStatus = "active"
	ProviderStatusDeprecated ProviderStatus = "deprecated"
	ProviderStatusDown       ProviderStatus = "down"
	ProviderStatusMaintenance ProviderStatus = "maintenance"
)

// AuthType specifica il tipo di autenticazione
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeOAuth2 AuthType = "oauth2"
)

// Provider rappresenta un provider LLM
type Provider struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key"`
	Name        string         `json:"name" gorm:"uniqueIndex;not null"`
	Type        ProviderType   `json:"type" gorm:"not null"`
	Status      ProviderStatus `json:"status" gorm:"not null;default:'active'"`
	BaseURL     string         `json:"base_url" gorm:"not null"`
	AuthType    AuthType       `json:"auth_type" gorm:"not null;default:'api_key'"`
	Tier        int            `json:"tier" gorm:"not null;default:3"` // 1=premium, 2=standard, 3=experimental

	// Discovery metadata
	DiscoveredAt  time.Time  `json:"discovered_at" gorm:"not null"`
	LastVerified  time.Time  `json:"last_verified"`
	Source        string     `json:"source"` // 'manual', 'github', 'scraper'

	// Capabilities
	SupportsStreaming bool   `json:"supports_streaming" gorm:"default:true"`
	SupportsTools     bool   `json:"supports_tools" gorm:"default:false"`
	SupportsJSON      bool   `json:"supports_json" gorm:"default:true"`

	// Health metrics
	LastHealthCheck time.Time `json:"last_health_check"`
	HealthScore     float64   `json:"health_score" gorm:"default:1.0"` // 0.0-1.0
	AvgLatencyMs    int       `json:"avg_latency_ms"`

	// Relations
	Models      []Model      `json:"models" gorm:"foreignKey:ProviderID"`
	RateLimits  []RateLimit  `json:"rate_limits" gorm:"foreignKey:ProviderID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook per generare UUID
func (p *Provider) BeforeCreate() error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.DiscoveredAt.IsZero() {
		p.DiscoveredAt = time.Now()
	}
	return nil
}

// IsAvailable verifica se il provider è utilizzabile
func (p *Provider) IsAvailable() bool {
	return p.Status == ProviderStatusActive &&
	       p.HealthScore > 0.5 &&
	       time.Since(p.LastHealthCheck) < 10*time.Minute
}

// TableName specifica il nome della tabella
func (Provider) TableName() string {
	return "providers"
}
