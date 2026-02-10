package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Account rappresenta un account utente per un provider
type Account struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	ProviderID uuid.UUID `json:"provider_id" gorm:"type:uuid;not null;index"`

	// Credentials (encrypted in database)
	Credentials datatypes.JSON `json:"credentials" gorm:"type:jsonb;not null"`
	// Example: {"api_key": "encrypted_key", "oauth_token": "encrypted_token"}

	// Quota tracking
	QuotaUsed  int64 `json:"quota_used" gorm:"default:0"`
	QuotaLimit int64 `json:"quota_limit"`
	LastReset  time.Time `json:"last_reset"`

	// Status
	Active    bool      `json:"active" gorm:"default:true"`
	ExpiresAt time.Time `json:"expires_at"`

	// Relations
	Provider Provider `json:"provider" gorm:"foreignKey:ProviderID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook
func (a *Account) BeforeCreate() error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.LastReset.IsZero() {
		a.LastReset = time.Now()
	}
	return nil
}

// IsQuotaAvailable verifica se c'è quota disponibile
func (a *Account) IsQuotaAvailable() bool {
	if a.QuotaLimit == 0 {
		return true // Unlimited
	}
	return a.QuotaUsed < a.QuotaLimit
}

// IsExpired verifica se l'account è scaduto
func (a *Account) IsExpired() bool {
	if a.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(a.ExpiresAt)
}

// TableName specifica il nome della tabella
func (Account) TableName() string {
	return "accounts"
}
