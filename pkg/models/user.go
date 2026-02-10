package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserRole rappresenta il ruolo di un utente
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleUser   UserRole = "user"
	RoleViewer UserRole = "viewer"
)

// User rappresenta un utente del sistema
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"not null"` // Never expose in JSON
	Name         string    `json:"name"`
	Role         UserRole  `json:"role" gorm:"not null;default:'user'"`

	// Status
	Active       bool      `json:"active" gorm:"default:true"`
	EmailVerified bool     `json:"email_verified" gorm:"default:false"`
	LastLoginAt  *time.Time `json:"last_login_at"`

	// Quota management
	QuotaTokens      int64 `json:"quota_tokens" gorm:"default:0"`       // 0 = unlimited
	QuotaTokensUsed  int64 `json:"quota_tokens_used" gorm:"default:0"`
	QuotaRequests    int64 `json:"quota_requests" gorm:"default:0"`     // 0 = unlimited
	QuotaRequestsUsed int64 `json:"quota_requests_used" gorm:"default:0"`
	QuotaResetAt     time.Time `json:"quota_reset_at"`

	// API Keys
	APIKeys []APIKey `json:"api_keys,omitempty" gorm:"foreignKey:UserID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey rappresenta una chiave API associata a un utente (model per DB)
type APIKey struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	UserID      uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	KeyHash     string    `json:"-" gorm:"not null;uniqueIndex"` // SHA256 hash per lookup veloce
	KeyBcrypt   string    `json:"-" gorm:"not null"`             // Bcrypt hash per validazione
	KeyPreview  string    `json:"key_preview" gorm:"not null"`   // First chars for display

	// Permissions
	Permissions string `json:"permissions" gorm:"default:'read,write'"` // Comma separated

	// Rate limiting
	RateLimit int `json:"rate_limit" gorm:"default:60"` // Requests per minute

	// Usage tracking
	UsageCount int64 `json:"usage_count" gorm:"default:0"`

	// Status
	Active      bool       `json:"active" gorm:"default:true"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`

	// Relations
	User User `json:"-" gorm:"foreignKey:UserID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook per User
func (u *User) BeforeCreate() error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.QuotaResetAt.IsZero() {
		u.QuotaResetAt = time.Now().AddDate(0, 1, 0) // Reset monthly
	}
	return nil
}

// BeforeCreate hook per APIKey
func (k *APIKey) BeforeCreate() error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

// SetPassword imposta la password hashata
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifica la password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// IsAdmin verifica se l'utente è admin
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// HasQuotaTokens verifica se l'utente ha token disponibili
func (u *User) HasQuotaTokens(tokens int64) bool {
	if u.QuotaTokens == 0 {
		return true // Unlimited
	}
	return u.QuotaTokensUsed+tokens <= u.QuotaTokens
}

// HasQuotaRequests verifica se l'utente ha richieste disponibili
func (u *User) HasQuotaRequests() bool {
	if u.QuotaRequests == 0 {
		return true // Unlimited
	}
	return u.QuotaRequestsUsed < u.QuotaRequests
}

// ResetQuota resetta le quote mensili
func (u *User) ResetQuota() {
	u.QuotaTokensUsed = 0
	u.QuotaRequestsUsed = 0
	u.QuotaResetAt = time.Now().AddDate(0, 1, 0)
}

// TableName specifica il nome della tabella
func (User) TableName() string {
	return "users"
}

// IsValid verifica se la chiave API è valida
func (k *APIKey) IsValid() bool {
	now := time.Now()
	return k.Active &&
		k.RevokedAt == nil &&
		(k.ExpiresAt.IsZero() || k.ExpiresAt.After(now))
}

// IsExpired verifica se la chiave è scaduta
func (k *APIKey) IsExpired() bool {
	return !k.ExpiresAt.IsZero() && time.Now().After(k.ExpiresAt)
}

// IsRevoked verifica se la chiave è stata revocata
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// UpdateUsage aggiorna il contatore di utilizzo
func (k *APIKey) UpdateUsage() {
	now := time.Now()
	k.LastUsedAt = &now
	k.UsageCount++
	k.UpdatedAt = now
}

// TableName specifica il nome della tabella
func (APIKey) TableName() string {
	return "api_keys"
}
