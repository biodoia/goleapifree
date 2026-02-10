package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const (
	// Soglia per notifica di quota in esaurimento
	QuotaWarningThreshold = 0.8 // 80%

	// Chiave Redis per tracking quota
	quotaKeyPrefix = "quota:"
)

// Manager gestisce le quote degli account
type Manager struct {
	db    *gorm.DB
	cache *cache.RedisClient
	mu    sync.RWMutex

	// Callbacks per notifiche
	onQuotaWarning  func(account *models.Account, usagePercent float64)
	onQuotaExhausted func(account *models.Account)
}

// NewManager crea un nuovo quota manager
func NewManager(db *gorm.DB, cache *cache.RedisClient) *Manager {
	m := &Manager{
		db:    db,
		cache: cache,
	}

	// Avvia goroutine per reset periodico
	go m.periodicReset()

	return m
}

// SetWarningCallback imposta la callback per quota warning
func (m *Manager) SetWarningCallback(fn func(account *models.Account, usagePercent float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onQuotaWarning = fn
}

// SetExhaustedCallback imposta la callback per quota exhausted
func (m *Manager) SetExhaustedCallback(fn func(account *models.Account)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onQuotaExhausted = fn
}

// CheckAvailability verifica se c'è quota disponibile per un account
func (m *Manager) CheckAvailability(ctx context.Context, accountID uuid.UUID, tokensNeeded int64) (*QuotaStatus, error) {
	// Carica account
	var account models.Account
	if err := m.db.WithContext(ctx).First(&account, "id = ?", accountID).Error; err != nil {
		return nil, fmt.Errorf("failed to load account: %w", err)
	}

	// Verifica se account è attivo
	if !account.Active {
		return &QuotaStatus{
			Available:     false,
			Reason:        "account inactive",
			CurrentUsage:  account.QuotaUsed,
			Limit:         account.QuotaLimit,
			ResetAt:       account.LastReset.Add(24 * time.Hour),
		}, nil
	}

	// Verifica scadenza
	if account.IsExpired() {
		return &QuotaStatus{
			Available:     false,
			Reason:        "account expired",
			CurrentUsage:  account.QuotaUsed,
			Limit:         account.QuotaLimit,
			ResetAt:       account.LastReset.Add(24 * time.Hour),
		}, nil
	}

	// Check se necessita reset
	if m.needsReset(&account) {
		if err := m.resetQuota(ctx, &account); err != nil {
			log.Error().Err(err).Str("account_id", accountID.String()).Msg("Failed to reset quota")
		}
	}

	// Verifica quota disponibile
	if account.QuotaLimit > 0 && account.QuotaUsed+tokensNeeded > account.QuotaLimit {
		// Notifica quota exhausted
		m.notifyQuotaExhausted(&account)

		return &QuotaStatus{
			Available:     false,
			Reason:        "quota exceeded",
			CurrentUsage:  account.QuotaUsed,
			Limit:         account.QuotaLimit,
			TokensNeeded:  tokensNeeded,
			ResetAt:       account.LastReset.Add(24 * time.Hour),
		}, nil
	}

	// Calcola percentuale di utilizzo
	usagePercent := float64(0)
	if account.QuotaLimit > 0 {
		usagePercent = float64(account.QuotaUsed) / float64(account.QuotaLimit)
	}

	return &QuotaStatus{
		Available:     true,
		CurrentUsage:  account.QuotaUsed,
		Limit:         account.QuotaLimit,
		UsagePercent:  usagePercent,
		TokensNeeded:  tokensNeeded,
		ResetAt:       account.LastReset.Add(24 * time.Hour),
	}, nil
}

// ConsumeQuota consuma quota per un account
func (m *Manager) ConsumeQuota(ctx context.Context, accountID uuid.UUID, tokens int64) error {
	// Incrementa in cache per performance
	cacheKey := fmt.Sprintf("%s%s", quotaKeyPrefix, accountID.String())
	newUsage, err := m.cache.IncrBy(ctx, cacheKey, tokens)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to increment quota in cache, falling back to DB")
	}

	// Aggiorna database
	result := m.db.WithContext(ctx).
		Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("quota_used", gorm.Expr("quota_used + ?", tokens))

	if result.Error != nil {
		return fmt.Errorf("failed to consume quota: %w", result.Error)
	}

	// Carica account aggiornato per check warning
	var account models.Account
	if err := m.db.WithContext(ctx).First(&account, "id = ?", accountID).Error; err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	// Check se è necessario notificare warning
	if account.QuotaLimit > 0 {
		usagePercent := float64(account.QuotaUsed) / float64(account.QuotaLimit)
		if usagePercent >= QuotaWarningThreshold {
			m.notifyQuotaWarning(&account, usagePercent)
		}
	}

	// Sync cache con DB
	if err == nil && newUsage != account.QuotaUsed {
		m.cache.Set(ctx, cacheKey, account.QuotaUsed, 24*time.Hour)
	}

	return nil
}

// GetStatus ottiene lo status corrente della quota
func (m *Manager) GetStatus(ctx context.Context, accountID uuid.UUID) (*QuotaStatus, error) {
	var account models.Account
	if err := m.db.WithContext(ctx).First(&account, "id = ?", accountID).Error; err != nil {
		return nil, fmt.Errorf("failed to load account: %w", err)
	}

	usagePercent := float64(0)
	if account.QuotaLimit > 0 {
		usagePercent = float64(account.QuotaUsed) / float64(account.QuotaLimit)
	}

	return &QuotaStatus{
		Available:    account.IsQuotaAvailable() && account.Active && !account.IsExpired(),
		CurrentUsage: account.QuotaUsed,
		Limit:        account.QuotaLimit,
		UsagePercent: usagePercent,
		ResetAt:      account.LastReset.Add(24 * time.Hour),
	}, nil
}

// ResetQuota resetta manualmente la quota di un account
func (m *Manager) ResetQuota(ctx context.Context, accountID uuid.UUID) error {
	var account models.Account
	if err := m.db.WithContext(ctx).First(&account, "id = ?", accountID).Error; err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	return m.resetQuota(ctx, &account)
}

// resetQuota resetta la quota di un account
func (m *Manager) resetQuota(ctx context.Context, account *models.Account) error {
	now := time.Now()

	result := m.db.WithContext(ctx).
		Model(account).
		Updates(map[string]interface{}{
			"quota_used": 0,
			"last_reset": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to reset quota: %w", result.Error)
	}

	// Aggiorna cache
	cacheKey := fmt.Sprintf("%s%s", quotaKeyPrefix, account.ID.String())
	m.cache.Set(ctx, cacheKey, 0, 24*time.Hour)

	log.Info().
		Str("account_id", account.ID.String()).
		Str("provider_id", account.ProviderID.String()).
		Msg("Quota reset")

	return nil
}

// needsReset verifica se la quota deve essere resettata
func (m *Manager) needsReset(account *models.Account) bool {
	// Reset giornaliero
	now := time.Now()
	lastReset := account.LastReset

	// Se è passato più di un giorno
	return now.Sub(lastReset) >= 24*time.Hour
}

// periodicReset controlla periodicamente gli account che necessitano reset
func (m *Manager) periodicReset() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()

		// Trova account che necessitano reset
		var accounts []models.Account
		cutoff := time.Now().Add(-24 * time.Hour)

		if err := m.db.WithContext(ctx).
			Where("last_reset < ? AND active = ?", cutoff, true).
			Find(&accounts).Error; err != nil {
			log.Error().Err(err).Msg("Failed to find accounts for reset")
			continue
		}

		// Reset quota per ogni account
		for _, account := range accounts {
			if err := m.resetQuota(ctx, &account); err != nil {
				log.Error().
					Err(err).
					Str("account_id", account.ID.String()).
					Msg("Failed to reset quota")
			}
		}

		if len(accounts) > 0 {
			log.Info().Int("count", len(accounts)).Msg("Periodic quota reset completed")
		}
	}
}

// notifyQuotaWarning notifica quando la quota raggiunge la soglia
func (m *Manager) notifyQuotaWarning(account *models.Account, usagePercent float64) {
	m.mu.RLock()
	callback := m.onQuotaWarning
	m.mu.RUnlock()

	if callback != nil {
		go callback(account, usagePercent)
	}

	log.Warn().
		Str("account_id", account.ID.String()).
		Str("provider_id", account.ProviderID.String()).
		Float64("usage_percent", usagePercent*100).
		Msg("Quota warning threshold reached")
}

// notifyQuotaExhausted notifica quando la quota è esaurita
func (m *Manager) notifyQuotaExhausted(account *models.Account) {
	m.mu.RLock()
	callback := m.onQuotaExhausted
	m.mu.RUnlock()

	if callback != nil {
		go callback(account)
	}

	log.Error().
		Str("account_id", account.ID.String()).
		Str("provider_id", account.ProviderID.String()).
		Msg("Quota exhausted")
}

// QuotaStatus rappresenta lo status della quota
type QuotaStatus struct {
	Available     bool      `json:"available"`
	Reason        string    `json:"reason,omitempty"`
	CurrentUsage  int64     `json:"current_usage"`
	Limit         int64     `json:"limit"`
	UsagePercent  float64   `json:"usage_percent"`
	TokensNeeded  int64     `json:"tokens_needed,omitempty"`
	ResetAt       time.Time `json:"reset_at"`
}
