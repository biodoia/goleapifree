package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidAPIKey indica che la chiave API non è valida
	ErrInvalidAPIKey = errors.New("invalid api key")
	// ErrAPIKeyRevoked indica che la chiave è stata revocata
	ErrAPIKeyRevoked = errors.New("api key revoked")
	// ErrAPIKeyExpired indica che la chiave è scaduta
	ErrAPIKeyExpired = errors.New("api key expired")
)

const (
	// Prefisso per le API keys
	apiKeyPrefix = "gla" // GoLeapAI
	// Lunghezza della chiave (in bytes, diventa più lunga in base64)
	apiKeyLength = 32
)

// APIKey rappresenta una chiave API
type APIKey struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	KeyHash     string // Bcrypt hash della chiave
	KeyPreview  string // Primi 8 caratteri per identificazione
	Permissions []string
	RateLimit   int       // Requests per minuto
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// APIKeyManager gestisce le chiavi API
type APIKeyManager struct {
	bcryptCost int
}

// NewAPIKeyManager crea un nuovo API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		bcryptCost: bcrypt.DefaultCost,
	}
}

// GenerateAPIKey genera una nuova chiave API
func (m *APIKeyManager) GenerateAPIKey(userID uuid.UUID, name string, permissions []string, rateLimit int, expiresIn time.Duration) (*APIKey, string, error) {
	// Genera bytes random
	keyBytes := make([]byte, apiKeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Codifica in base64
	keySecret := base64.RawURLEncoding.EncodeToString(keyBytes)

	// Crea la chiave formattata (prefix.secret)
	fullKey := fmt.Sprintf("%s_%s", apiKeyPrefix, keySecret)

	// Hash della chiave per storage
	keyHash, err := bcrypt.GenerateFromPassword([]byte(fullKey), m.bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash key: %w", err)
	}

	// Preview per identificazione (primi 8 caratteri + ...)
	preview := fullKey
	if len(fullKey) > 12 {
		preview = fullKey[:12] + "..."
	}

	now := time.Now()
	expiresAt := now.Add(expiresIn)
	if expiresIn == 0 {
		expiresAt = now.AddDate(10, 0, 0) // 10 anni se non specificato
	}

	apiKey := &APIKey{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		KeyHash:     string(keyHash),
		KeyPreview:  preview,
		Permissions: permissions,
		RateLimit:   rateLimit,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return apiKey, fullKey, nil
}

// ValidateAPIKey valida una chiave API
func (m *APIKeyManager) ValidateAPIKey(key string, storedKey *APIKey) error {
	// Verifica formato
	if !strings.HasPrefix(key, apiKeyPrefix+"_") {
		return ErrInvalidAPIKey
	}

	// Verifica che non sia revocata
	if storedKey.RevokedAt != nil {
		return ErrAPIKeyRevoked
	}

	// Verifica scadenza
	if time.Now().After(storedKey.ExpiresAt) {
		return ErrAPIKeyExpired
	}

	// Verifica hash
	if err := bcrypt.CompareHashAndPassword([]byte(storedKey.KeyHash), []byte(key)); err != nil {
		return ErrInvalidAPIKey
	}

	return nil
}

// HashAPIKey crea un hash della chiave per lookup veloce
// Usa SHA256 invece di bcrypt perché serve solo per trovare la chiave nel DB
func (m *APIKeyManager) HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// RevokeAPIKey revoca una chiave API
func (m *APIKeyManager) RevokeAPIKey(apiKey *APIKey) {
	now := time.Now()
	apiKey.RevokedAt = &now
	apiKey.UpdatedAt = now
}

// IsExpired verifica se una chiave è scaduta
func (k *APIKey) IsExpired() bool {
	return time.Now().After(k.ExpiresAt)
}

// IsRevoked verifica se una chiave è stata revocata
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsValid verifica se una chiave è valida (non scaduta e non revocata)
func (k *APIKey) IsValid() bool {
	return !k.IsExpired() && !k.IsRevoked()
}

// HasPermission verifica se la chiave ha un determinato permesso
func (k *APIKey) HasPermission(permission string) bool {
	for _, p := range k.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// UpdateLastUsed aggiorna il timestamp dell'ultimo utilizzo
func (k *APIKey) UpdateLastUsed() {
	now := time.Now()
	k.LastUsedAt = &now
	k.UpdatedAt = now
}

// ParseAPIKey estrae le informazioni da una chiave API
func ParseAPIKey(key string) (prefix, secret string, err error) {
	parts := strings.SplitN(key, "_", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidAPIKey
	}

	if parts[0] != apiKeyPrefix {
		return "", "", ErrInvalidAPIKey
	}

	return parts[0], parts[1], nil
}
