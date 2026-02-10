package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// QuotaSystem rappresenta il sistema completo di quota management
type QuotaSystem struct {
	Manager     *Manager
	RateLimiter *RateLimiter
	Tracker     *Tracker
	PoolManager *PoolManager
}

// NewQuotaSystem crea un nuovo sistema di quota management
func NewQuotaSystem(db *gorm.DB, redisHost, redisPassword string, redisDB int) (*QuotaSystem, error) {
	// Inizializza Redis client
	redisClient, err := cache.NewRedisClient(redisHost, redisPassword, redisDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Crea componenti
	manager := NewManager(db, redisClient)
	rateLimiter := NewRateLimiter(redisClient)
	tracker := NewTracker(db)
	poolManager := NewPoolManager(db, redisClient, manager, rateLimiter)

	// Configura callbacks
	manager.SetWarningCallback(func(account *models.Account, usagePercent float64) {
		log.Warn().
			Str("account_id", account.ID.String()).
			Float64("usage_percent", usagePercent*100).
			Msg("Quota warning: 80% threshold reached")

		// TODO: Invia notifica email/webhook
	})

	manager.SetExhaustedCallback(func(account *models.Account) {
		log.Error().
			Str("account_id", account.ID.String()).
			Msg("Quota exhausted")

		// TODO: Invia notifica email/webhook
		// TODO: Disabilita account temporaneamente
	})

	return &QuotaSystem{
		Manager:     manager,
		RateLimiter: rateLimiter,
		Tracker:     tracker,
		PoolManager: poolManager,
	}, nil
}

// ExampleUsage mostra come utilizzare il sistema
func ExampleUsage(qs *QuotaSystem, db *gorm.DB) {
	ctx := context.Background()

	// 1. Ottieni un account disponibile dal pool
	providerID := uuid.New() // ID del provider
	tokensNeeded := int64(1000)

	account, err := qs.PoolManager.GetAccount(ctx, providerID, tokensNeeded)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get account from pool")
		return
	}

	log.Info().
		Str("account_id", account.ID.String()).
		Msg("Account selected from pool")

	// 2. Verifica quota availability
	quotaStatus, err := qs.Manager.CheckAvailability(ctx, account.ID, tokensNeeded)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check quota availability")
		return
	}

	if !quotaStatus.Available {
		log.Warn().
			Str("reason", quotaStatus.Reason).
			Msg("Quota not available")
		return
	}

	// 3. Verifica rate limits
	var rateLimits []models.RateLimit
	db.Where("provider_id = ?", account.ProviderID).Find(&rateLimits)

	rateLimitResult, err := qs.RateLimiter.CheckLimit(ctx, account.ProviderID, account.ID, rateLimits)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check rate limit")
		return
	}

	if !rateLimitResult.Allowed {
		log.Warn().
			Str("limit_type", string(rateLimitResult.LimitType)).
			Dur("retry_after", rateLimitResult.RetryAfter).
			Msg("Rate limit exceeded")
		return
	}

	// 4. Incrementa contatore concurrent (se necessario)
	concurrent, err := qs.RateLimiter.IncrementConcurrent(ctx, account.ProviderID, account.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to increment concurrent")
		return
	}

	defer func() {
		// Decrementa al termine
		qs.RateLimiter.DecrementConcurrent(ctx, account.ProviderID, account.ID)
	}()

	log.Debug().Int64("concurrent", concurrent).Msg("Concurrent requests")

	// 5. Esegui la richiesta API
	startTime := time.Now()

	// ... esegui richiesta API qui ...
	success := true
	inputTokens := 500
	outputTokens := 500
	statusCode := 200
	errorMessage := ""

	latencyMs := int(time.Since(startTime).Milliseconds())

	// 6. Consuma quota
	if err := qs.Manager.ConsumeQuota(ctx, account.ID, int64(inputTokens+outputTokens)); err != nil {
		log.Error().Err(err).Msg("Failed to consume quota")
	}

	// 7. Registra richiesta per sliding window
	for _, limit := range rateLimits {
		if limit.LimitType == models.LimitTypeRPM ||
		   limit.LimitType == models.LimitTypeRPH ||
		   limit.LimitType == models.LimitTypeRPD {
			qs.RateLimiter.RecordRequest(ctx, account.ProviderID, account.ID, limit.LimitType)
		}
	}

	// 8. Calcola costo (se disponibile)
	modelID := uuid.New() // ID del modello
	cost, _ := qs.Tracker.CalculateCost(ctx, modelID, inputTokens, outputTokens)

	// 9. Traccia la richiesta
	trackingReq := &TrackingRequest{
		ProviderID:    account.ProviderID,
		ModelID:       modelID,
		UserID:        account.UserID,
		Method:        "POST",
		Endpoint:      "/v1/chat/completions",
		StatusCode:    statusCode,
		LatencyMs:     latencyMs,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		Success:       success,
		ErrorMessage:  errorMessage,
		EstimatedCost: cost,
	}

	if err := qs.Tracker.TrackRequest(ctx, trackingReq); err != nil {
		log.Error().Err(err).Msg("Failed to track request")
	}

	log.Info().
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Int("latency_ms", latencyMs).
		Float64("cost", cost).
		Msg("Request completed and tracked")
}

// ExampleGetStats mostra come ottenere statistiche
func ExampleGetStats(qs *QuotaSystem) {
	ctx := context.Background()

	// Ottieni statistiche per un account
	accountID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	usageStats, err := qs.Tracker.GetUsageStats(ctx, accountID, from, to)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get usage stats")
		return
	}

	log.Info().
		Int64("total_requests", usageStats.TotalRequests).
		Float64("success_rate", usageStats.SuccessRate*100).
		Int64("total_tokens", usageStats.TotalInputTokens+usageStats.TotalOutputTokens).
		Float64("avg_latency_ms", usageStats.AvgLatencyMs).
		Msg("Usage statistics")

	// Ottieni status del pool
	providerID := uuid.New()
	poolStatus, err := qs.PoolManager.GetPoolStatus(ctx, providerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pool status")
		return
	}

	log.Info().
		Int("total_accounts", poolStatus.TotalAccounts).
		Int("available_accounts", poolStatus.AvailableAccounts).
		Msg("Pool status")

	// Ottieni top models
	topModels, err := qs.Tracker.GetTopModels(ctx, 10, from, to)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get top models")
		return
	}

	log.Info().Int("count", len(topModels)).Msg("Top models retrieved")

	for _, model := range topModels {
		log.Info().
			Str("model_id", model.ModelID.String()).
			Int64("requests", model.RequestCount).
			Int64("tokens", model.TotalTokens).
			Msg("Model usage")
	}
}

// ExamplePoolManagement mostra gestione del pool
func ExamplePoolManagement(qs *QuotaSystem) {
	ctx := context.Background()
	providerID := uuid.New()

	// Cambia strategia di selezione
	qs.PoolManager.SetStrategy(StrategyLeastUsed)

	// Ottieni l'account migliore
	account, err := qs.PoolManager.GetBestAccount(ctx, providerID, 1000)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get best account")
		return
	}

	log.Info().
		Str("account_id", account.ID.String()).
		Int64("quota_used", account.QuotaUsed).
		Int64("quota_limit", account.QuotaLimit).
		Msg("Best account selected")

	// Riequilibra il pool
	if err := qs.PoolManager.RebalancePool(ctx, providerID); err != nil {
		log.Error().Err(err).Msg("Failed to rebalance pool")
		return
	}

	log.Info().Msg("Pool rebalanced")
}

// ExampleRateLimitStats mostra statistiche di rate limiting
func ExampleRateLimitStats(qs *QuotaSystem) {
	ctx := context.Background()
	providerID := uuid.New()
	accountID := uuid.New()

	// Ottieni stats per RPM
	stats, err := qs.RateLimiter.GetStats(ctx, providerID, accountID, models.LimitTypeRPM)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get rate limit stats")
		return
	}

	log.Info().
		Str("limit_type", string(stats.LimitType)).
		Int64("current_count", stats.CurrentCount).
		Time("window_start", stats.WindowStart).
		Time("window_end", stats.WindowEnd).
		Msg("Rate limit statistics")
}
