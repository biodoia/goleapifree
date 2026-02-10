package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// Prefissi chiavi Redis per rate limiting
	rateLimitKeyPrefix = "ratelimit:"
	tokenBucketPrefix  = "tokenbucket:"
	slidingWindowPrefix = "slidingwindow:"
)

// RateLimiter implementa rate limiting con Redis
type RateLimiter struct {
	cache *cache.RedisClient
}

// NewRateLimiter crea un nuovo rate limiter
func NewRateLimiter(cache *cache.RedisClient) *RateLimiter {
	return &RateLimiter{
		cache: cache,
	}
}

// CheckLimit verifica se una richiesta può essere eseguita
func (rl *RateLimiter) CheckLimit(ctx context.Context, providerID uuid.UUID, accountID uuid.UUID, limits []models.RateLimit) (*RateLimitResult, error) {
	// Controlla ogni limite configurato
	for _, limit := range limits {
		allowed, err := rl.checkSingleLimit(ctx, providerID, accountID, limit)
		if err != nil {
			return nil, err
		}

		if !allowed {
			return &RateLimitResult{
				Allowed:   false,
				LimitType: limit.LimitType,
				RetryAfter: rl.getRetryAfter(ctx, providerID, accountID, limit),
			}, nil
		}
	}

	return &RateLimitResult{
		Allowed: true,
	}, nil
}

// checkSingleLimit verifica un singolo limite
func (rl *RateLimiter) checkSingleLimit(ctx context.Context, providerID, accountID uuid.UUID, limit models.RateLimit) (bool, error) {
	switch limit.LimitType {
	case models.LimitTypeRPM, models.LimitTypeRPH, models.LimitTypeRPD:
		return rl.checkSlidingWindow(ctx, providerID, accountID, limit)
	case models.LimitTypeTPM, models.LimitTypeTPD:
		return rl.checkTokenBucket(ctx, providerID, accountID, limit)
	case models.LimitTypeConcurrent:
		return rl.checkConcurrent(ctx, providerID, accountID, limit)
	default:
		return true, nil
	}
}

// checkSlidingWindow implementa sliding window rate limiting
func (rl *RateLimiter) checkSlidingWindow(ctx context.Context, providerID, accountID uuid.UUID, limit models.RateLimit) (bool, error) {
	key := fmt.Sprintf("%s%s:%s:%s", slidingWindowPrefix, providerID, accountID, limit.LimitType)

	now := time.Now()
	window := rl.getWindowDuration(limit.LimitType)
	windowStart := now.Add(-window)

	// Usa sorted set di Redis per sliding window
	pipe := rl.cache.Pipeline()

	// Rimuovi richieste vecchie fuori dalla finestra
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Conta richieste nella finestra
	countCmd := pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart.UnixNano()), "+inf")

	// Esegui pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("failed to check sliding window: %w", err)
	}

	count := countCmd.Val()

	// Verifica se sotto il limite
	if count >= int64(limit.LimitValue) {
		log.Debug().
			Str("provider_id", providerID.String()).
			Str("account_id", accountID.String()).
			Str("limit_type", string(limit.LimitType)).
			Int64("count", count).
			Int("limit", limit.LimitValue).
			Msg("Rate limit exceeded")
		return false, nil
	}

	return true, nil
}

// RecordRequest registra una richiesta per sliding window
func (rl *RateLimiter) RecordRequest(ctx context.Context, providerID, accountID uuid.UUID, limitType models.LimitType) error {
	key := fmt.Sprintf("%s%s:%s:%s", slidingWindowPrefix, providerID, accountID, limitType)

	now := time.Now()
	window := rl.getWindowDuration(limitType)

	// Aggiungi timestamp corrente al sorted set
	if err := rl.cache.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	}); err != nil {
		return fmt.Errorf("failed to record request: %w", err)
	}

	// Imposta TTL sulla chiave
	rl.cache.Expire(ctx, key, window*2)

	return nil
}

// checkTokenBucket implementa token bucket algorithm
func (rl *RateLimiter) checkTokenBucket(ctx context.Context, providerID, accountID uuid.UUID, limit models.RateLimit) (bool, error) {
	key := fmt.Sprintf("%s%s:%s:%s", tokenBucketPrefix, providerID, accountID, limit.LimitType)
	lastRefillKey := key + ":last_refill"

	// Ottieni stato corrente del bucket
	tokensStr, err := rl.cache.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get token bucket: %w", err)
	}

	tokens := int64(limit.LimitValue)
	if tokensStr != "" {
		fmt.Sscanf(tokensStr, "%d", &tokens)
	}

	// Ottieni ultimo refill
	lastRefillStr, err := rl.cache.Get(ctx, lastRefillKey)
	if err != nil {
		return false, fmt.Errorf("failed to get last refill: %w", err)
	}

	lastRefill := time.Now()
	if lastRefillStr != "" {
		var nanos int64
		fmt.Sscanf(lastRefillStr, "%d", &nanos)
		lastRefill = time.Unix(0, nanos)
	}

	// Calcola refill
	now := time.Now()
	refillInterval := rl.getWindowDuration(limit.LimitType)
	elapsed := now.Sub(lastRefill)

	if elapsed >= refillInterval {
		// Refill bucket
		tokens = int64(limit.LimitValue)
		rl.cache.Set(ctx, lastRefillKey, now.UnixNano(), refillInterval*2)
	}

	// Verifica se ci sono token disponibili
	if tokens <= 0 {
		return false, nil
	}

	return true, nil
}

// ConsumeTokens consuma token dal bucket
func (rl *RateLimiter) ConsumeTokens(ctx context.Context, providerID, accountID uuid.UUID, limitType models.LimitType, amount int64) error {
	key := fmt.Sprintf("%s%s:%s:%s", tokenBucketPrefix, providerID, accountID, limitType)

	// Decrementa token
	newTokens, err := rl.cache.IncrBy(ctx, key, -amount)
	if err != nil {
		return fmt.Errorf("failed to consume tokens: %w", err)
	}

	if newTokens < 0 {
		// Rollback
		rl.cache.IncrBy(ctx, key, amount)
		return fmt.Errorf("insufficient tokens")
	}

	return nil
}

// checkConcurrent verifica il numero di richieste concorrenti
func (rl *RateLimiter) checkConcurrent(ctx context.Context, providerID, accountID uuid.UUID, limit models.RateLimit) (bool, error) {
	key := fmt.Sprintf("%sconcurrent:%s:%s", rateLimitKeyPrefix, providerID, accountID)

	count, err := rl.cache.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get concurrent count: %w", err)
	}

	concurrent := int64(0)
	if count != "" {
		fmt.Sscanf(count, "%d", &concurrent)
	}

	return concurrent < int64(limit.LimitValue), nil
}

// IncrementConcurrent incrementa il contatore di richieste concorrenti
func (rl *RateLimiter) IncrementConcurrent(ctx context.Context, providerID, accountID uuid.UUID) (int64, error) {
	key := fmt.Sprintf("%sconcurrent:%s:%s", rateLimitKeyPrefix, providerID, accountID)
	count, err := rl.cache.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("failed to increment concurrent: %w", err)
	}

	// Set TTL di 5 minuti per safety
	rl.cache.Expire(ctx, key, 5*time.Minute)

	return count, nil
}

// DecrementConcurrent decrementa il contatore di richieste concorrenti
func (rl *RateLimiter) DecrementConcurrent(ctx context.Context, providerID, accountID uuid.UUID) error {
	key := fmt.Sprintf("%sconcurrent:%s:%s", rateLimitKeyPrefix, providerID, accountID)
	_, err := rl.cache.IncrBy(ctx, key, -1)
	if err != nil {
		return fmt.Errorf("failed to decrement concurrent: %w", err)
	}
	return nil
}

// getRetryAfter calcola quando riprovare
func (rl *RateLimiter) getRetryAfter(ctx context.Context, providerID, accountID uuid.UUID, limit models.RateLimit) time.Duration {
	window := rl.getWindowDuration(limit.LimitType)

	switch limit.LimitType {
	case models.LimitTypeRPM, models.LimitTypeRPH, models.LimitTypeRPD,
		 models.LimitTypeTPM, models.LimitTypeTPD:
		// Ottieni TTL della chiave più vecchia
		key := fmt.Sprintf("%s%s:%s:%s", slidingWindowPrefix, providerID, accountID, limit.LimitType)
		ttl, err := rl.cache.TTL(ctx, key)
		if err == nil && ttl > 0 {
			return ttl
		}
		return window
	case models.LimitTypeConcurrent:
		// Per concurrent, suggerisci retry breve
		return 1 * time.Second
	default:
		return window
	}
}

// getWindowDuration ottiene la durata della finestra per un tipo di limite
func (rl *RateLimiter) getWindowDuration(limitType models.LimitType) time.Duration {
	switch limitType {
	case models.LimitTypeRPM, models.LimitTypeTPM:
		return 1 * time.Minute
	case models.LimitTypeRPH:
		return 1 * time.Hour
	case models.LimitTypeRPD, models.LimitTypeTPD:
		return 24 * time.Hour
	default:
		return 1 * time.Minute
	}
}

// Reset resetta tutti i limiti per un account/provider
func (rl *RateLimiter) Reset(ctx context.Context, providerID, accountID uuid.UUID) error {
	// Nota: questo è un'operazione costosa, usare con cautela
	// In produzione, considerare una strategia più efficiente
	// pattern := fmt.Sprintf("%s*:%s:%s:*", rateLimitKeyPrefix, providerID, accountID)
	// TODO: Implementare scan e delete delle chiavi con pattern

	log.Info().
		Str("provider_id", providerID.String()).
		Str("account_id", accountID.String()).
		Msg("Rate limit reset requested")

	return nil
}

// GetStats ottiene statistiche di rate limiting
func (rl *RateLimiter) GetStats(ctx context.Context, providerID, accountID uuid.UUID, limitType models.LimitType) (*RateLimitStats, error) {
	key := fmt.Sprintf("%s%s:%s:%s", slidingWindowPrefix, providerID, accountID, limitType)
	window := rl.getWindowDuration(limitType)
	windowStart := time.Now().Add(-window)

	count, err := rl.cache.ZCount(ctx, key, fmt.Sprintf("%d", windowStart.UnixNano()), "+inf")
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return &RateLimitStats{
		LimitType:     limitType,
		CurrentCount:  count,
		WindowStart:   windowStart,
		WindowEnd:     time.Now(),
	}, nil
}

// RateLimitResult rappresenta il risultato di un check di rate limit
type RateLimitResult struct {
	Allowed    bool              `json:"allowed"`
	LimitType  models.LimitType  `json:"limit_type,omitempty"`
	RetryAfter time.Duration     `json:"retry_after,omitempty"`
}

// RateLimitStats rappresenta statistiche di rate limiting
type RateLimitStats struct {
	LimitType    models.LimitType `json:"limit_type"`
	CurrentCount int64            `json:"current_count"`
	WindowStart  time.Time        `json:"window_start"`
	WindowEnd    time.Time        `json:"window_end"`
}
