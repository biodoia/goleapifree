package quota

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// SelectionStrategy definisce la strategia di selezione degli account
type SelectionStrategy string

const (
	StrategyRoundRobin SelectionStrategy = "round_robin"
	StrategyLeastUsed  SelectionStrategy = "least_used"
	StrategyRandom     SelectionStrategy = "random"
)

// PoolManager gestisce il pool di account per provider
type PoolManager struct {
	db            *gorm.DB
	cache         *cache.RedisClient
	quotaManager  *Manager
	rateLimiter   *RateLimiter
	strategy      SelectionStrategy
	mu            sync.RWMutex
	roundRobinIdx map[uuid.UUID]int // Indice per round-robin per provider
}

// NewPoolManager crea un nuovo pool manager
func NewPoolManager(db *gorm.DB, cache *cache.RedisClient, quotaManager *Manager, rateLimiter *RateLimiter) *PoolManager {
	return &PoolManager{
		db:            db,
		cache:         cache,
		quotaManager:  quotaManager,
		rateLimiter:   rateLimiter,
		strategy:      StrategyLeastUsed,
		roundRobinIdx: make(map[uuid.UUID]int),
	}
}

// SetStrategy imposta la strategia di selezione
func (pm *PoolManager) SetStrategy(strategy SelectionStrategy) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.strategy = strategy
}

// GetAccount ottiene un account disponibile per un provider
func (pm *PoolManager) GetAccount(ctx context.Context, providerID uuid.UUID, tokensNeeded int64) (*models.Account, error) {
	// Carica tutti gli account attivi per il provider
	accounts, err := pm.loadActiveAccounts(ctx, providerID)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active accounts for provider %s", providerID)
	}

	// Carica rate limits per il provider
	var rateLimits []models.RateLimit
	if err := pm.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Find(&rateLimits).Error; err != nil {
		return nil, fmt.Errorf("failed to load rate limits: %w", err)
	}

	// Seleziona account basato sulla strategia
	pm.mu.RLock()
	strategy := pm.strategy
	pm.mu.RUnlock()

	var selectedAccount *models.Account

	switch strategy {
	case StrategyRoundRobin:
		selectedAccount, err = pm.selectRoundRobin(ctx, accounts, tokensNeeded, rateLimits)
	case StrategyLeastUsed:
		selectedAccount, err = pm.selectLeastUsed(ctx, accounts, tokensNeeded, rateLimits)
	case StrategyRandom:
		selectedAccount, err = pm.selectRandom(ctx, accounts, tokensNeeded, rateLimits)
	default:
		selectedAccount, err = pm.selectLeastUsed(ctx, accounts, tokensNeeded, rateLimits)
	}

	if err != nil {
		return nil, err
	}

	if selectedAccount == nil {
		return nil, fmt.Errorf("no suitable account found (all exhausted or rate limited)")
	}

	log.Debug().
		Str("provider_id", providerID.String()).
		Str("account_id", selectedAccount.ID.String()).
		Str("strategy", string(strategy)).
		Msg("Account selected from pool")

	return selectedAccount, nil
}

// loadActiveAccounts carica tutti gli account attivi per un provider
func (pm *PoolManager) loadActiveAccounts(ctx context.Context, providerID uuid.UUID) ([]models.Account, error) {
	var accounts []models.Account

	err := pm.db.WithContext(ctx).
		Where("provider_id = ? AND active = true", providerID).
		Order("quota_used ASC"). // Ordina per utilizzo
		Find(&accounts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Filtra account scaduti
	activeAccounts := make([]models.Account, 0, len(accounts))
	for _, account := range accounts {
		if !account.IsExpired() {
			activeAccounts = append(activeAccounts, account)
		}
	}

	return activeAccounts, nil
}

// selectRoundRobin seleziona un account con strategia round-robin
func (pm *PoolManager) selectRoundRobin(ctx context.Context, accounts []models.Account, tokensNeeded int64, rateLimits []models.RateLimit) (*models.Account, error) {
	if len(accounts) == 0 {
		return nil, nil
	}

	providerID := accounts[0].ProviderID

	pm.mu.Lock()
	startIdx := pm.roundRobinIdx[providerID]
	pm.mu.Unlock()

	// Prova ogni account partendo dall'indice corrente
	for i := 0; i < len(accounts); i++ {
		idx := (startIdx + i) % len(accounts)
		account := &accounts[idx]

		if pm.isAccountSuitable(ctx, account, tokensNeeded, rateLimits) {
			// Aggiorna indice per prossima volta
			pm.mu.Lock()
			pm.roundRobinIdx[providerID] = (idx + 1) % len(accounts)
			pm.mu.Unlock()

			return account, nil
		}
	}

	return nil, nil
}

// selectLeastUsed seleziona l'account meno utilizzato
func (pm *PoolManager) selectLeastUsed(ctx context.Context, accounts []models.Account, tokensNeeded int64, rateLimits []models.RateLimit) (*models.Account, error) {
	// Gli account sono già ordinati per quota_used ASC dalla query
	for i := range accounts {
		account := &accounts[i]

		if pm.isAccountSuitable(ctx, account, tokensNeeded, rateLimits) {
			return account, nil
		}
	}

	return nil, nil
}

// selectRandom seleziona un account casuale disponibile
func (pm *PoolManager) selectRandom(ctx context.Context, accounts []models.Account, tokensNeeded int64, rateLimits []models.RateLimit) (*models.Account, error) {
	// Filtra account disponibili
	available := make([]*models.Account, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if pm.isAccountSuitable(ctx, account, tokensNeeded, rateLimits) {
			available = append(available, account)
		}
	}

	if len(available) == 0 {
		return nil, nil
	}

	// Seleziona casualmente
	idx := time.Now().UnixNano() % int64(len(available))
	return available[idx], nil
}

// isAccountSuitable verifica se un account è adatto per una richiesta
func (pm *PoolManager) isAccountSuitable(ctx context.Context, account *models.Account, tokensNeeded int64, rateLimits []models.RateLimit) bool {
	// Verifica quota
	quotaStatus, err := pm.quotaManager.CheckAvailability(ctx, account.ID, tokensNeeded)
	if err != nil {
		log.Error().Err(err).Str("account_id", account.ID.String()).Msg("Failed to check quota")
		return false
	}

	if !quotaStatus.Available {
		return false
	}

	// Verifica rate limits
	rateLimitResult, err := pm.rateLimiter.CheckLimit(ctx, account.ProviderID, account.ID, rateLimits)
	if err != nil {
		log.Error().Err(err).Str("account_id", account.ID.String()).Msg("Failed to check rate limit")
		return false
	}

	return rateLimitResult.Allowed
}

// GetPoolStatus ottiene lo status del pool per un provider
func (pm *PoolManager) GetPoolStatus(ctx context.Context, providerID uuid.UUID) (*PoolStatus, error) {
	accounts, err := pm.loadActiveAccounts(ctx, providerID)
	if err != nil {
		return nil, err
	}

	status := &PoolStatus{
		ProviderID:     providerID,
		TotalAccounts:  len(accounts),
		AccountsStatus: make([]AccountStatus, 0, len(accounts)),
	}

	for _, account := range accounts {
		quotaStatus, _ := pm.quotaManager.GetStatus(ctx, account.ID)

		accountStatus := AccountStatus{
			AccountID:    account.ID,
			Active:       account.Active,
			QuotaUsed:    account.QuotaUsed,
			QuotaLimit:   account.QuotaLimit,
			UsagePercent: 0,
			Available:    false,
		}

		if quotaStatus != nil {
			accountStatus.UsagePercent = quotaStatus.UsagePercent
			accountStatus.Available = quotaStatus.Available
		}

		status.AccountsStatus = append(status.AccountsStatus, accountStatus)

		if accountStatus.Available {
			status.AvailableAccounts++
		}
	}

	return status, nil
}

// GetBestAccount ottiene l'account migliore basato su metriche multiple
func (pm *PoolManager) GetBestAccount(ctx context.Context, providerID uuid.UUID, tokensNeeded int64) (*models.Account, error) {
	accounts, err := pm.loadActiveAccounts(ctx, providerID)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active accounts")
	}

	// Carica rate limits
	var rateLimits []models.RateLimit
	if err := pm.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Find(&rateLimits).Error; err != nil {
		return nil, fmt.Errorf("failed to load rate limits: %w", err)
	}

	// Calcola score per ogni account
	type accountScore struct {
		account *models.Account
		score   float64
	}

	scores := make([]accountScore, 0, len(accounts))

	for i := range accounts {
		account := &accounts[i]

		// Verifica disponibilità
		if !pm.isAccountSuitable(ctx, account, tokensNeeded, rateLimits) {
			continue
		}

		// Calcola score (più basso è meglio)
		score := pm.calculateAccountScore(ctx, account)
		scores = append(scores, accountScore{
			account: account,
			score:   score,
		})
	}

	if len(scores) == 0 {
		return nil, fmt.Errorf("no suitable account found")
	}

	// Ordina per score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	return scores[0].account, nil
}

// calculateAccountScore calcola uno score per un account
func (pm *PoolManager) calculateAccountScore(ctx context.Context, account *models.Account) float64 {
	score := float64(0)

	// Fattore 1: Utilizzo quota (peso: 0.5)
	if account.QuotaLimit > 0 {
		usagePercent := float64(account.QuotaUsed) / float64(account.QuotaLimit)
		score += usagePercent * 0.5
	}

	// Fattore 2: Tempo dall'ultimo reset (peso: 0.3)
	timeSinceReset := time.Since(account.LastReset)
	resetScore := timeSinceReset.Hours() / 24.0 // Normalizza a giorni
	if resetScore > 1.0 {
		resetScore = 1.0
	}
	score += resetScore * 0.3

	// Fattore 3: Attività recente (peso: 0.2)
	// TODO: potremmo tracciare l'ultima richiesta in cache
	// Per ora usiamo un valore neutro
	score += 0.5 * 0.2

	return score
}

// RebalancePool riequilibra il carico tra gli account
func (pm *PoolManager) RebalancePool(ctx context.Context, providerID uuid.UUID) error {
	accounts, err := pm.loadActiveAccounts(ctx, providerID)
	if err != nil {
		return err
	}

	if len(accounts) == 0 {
		return nil
	}

	// Calcola utilizzo medio
	totalUsage := int64(0)
	totalLimit := int64(0)

	for _, account := range accounts {
		totalUsage += account.QuotaUsed
		totalLimit += account.QuotaLimit
	}

	if totalLimit == 0 {
		return nil // Unlimited accounts
	}

	avgUsagePercent := float64(totalUsage) / float64(totalLimit)

	log.Info().
		Str("provider_id", providerID.String()).
		Int("accounts", len(accounts)).
		Float64("avg_usage_percent", avgUsagePercent*100).
		Msg("Pool rebalance check")

	// TODO: Implementare logica di rebalancing se necessario
	// Ad esempio, disabilitare temporaneamente account sovraccarichi

	return nil
}

// PoolStatus rappresenta lo status del pool
type PoolStatus struct {
	ProviderID        uuid.UUID       `json:"provider_id"`
	TotalAccounts     int             `json:"total_accounts"`
	AvailableAccounts int             `json:"available_accounts"`
	AccountsStatus    []AccountStatus `json:"accounts_status"`
}

// AccountStatus rappresenta lo status di un account nel pool
type AccountStatus struct {
	AccountID    uuid.UUID `json:"account_id"`
	Active       bool      `json:"active"`
	Available    bool      `json:"available"`
	QuotaUsed    int64     `json:"quota_used"`
	QuotaLimit   int64     `json:"quota_limit"`
	UsagePercent float64   `json:"usage_percent"`
}
