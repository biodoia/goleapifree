package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	// ErrMaxRetriesExceeded viene restituito quando si supera il numero massimo di retry
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// RetryConfig contiene la configurazione del retry
type RetryConfig struct {
	// MaxRetries numero massimo di tentativi (0 = nessun retry)
	MaxRetries int

	// InitialBackoff backoff iniziale
	InitialBackoff time.Duration

	// MaxBackoff backoff massimo
	MaxBackoff time.Duration

	// BackoffMultiplier moltiplicatore per exponential backoff
	BackoffMultiplier float64

	// Jitter abilita jitter nel backoff
	Jitter bool

	// JitterFraction frazione di jitter (0.0-1.0)
	JitterFraction float64

	// RetryableErrors lista di errori che possono essere ritentati
	RetryableErrors []error

	// RetryableChecker funzione custom per verificare se un errore è retryable
	RetryableChecker func(error) bool

	// OnRetry callback chiamata prima di ogni retry
	OnRetry func(attempt int, err error, backoff time.Duration)
}

// DefaultRetryConfig restituisce una configurazione di default
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:         3,
		InitialBackoff:     100 * time.Millisecond,
		MaxBackoff:         10 * time.Second,
		BackoffMultiplier:  2.0,
		Jitter:             true,
		JitterFraction:     0.1,
		RetryableErrors:    nil,
		RetryableChecker:   nil,
		OnRetry:            nil,
	}
}

// Retry implementa retry logic con exponential backoff e jitter
type Retry struct {
	config RetryConfig
	rng    *rand.Rand
}

// NewRetry crea un nuovo retry handler
func NewRetry(config RetryConfig) *Retry {
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = DefaultRetryConfig().InitialBackoff
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = DefaultRetryConfig().MaxBackoff
	}
	if config.BackoffMultiplier <= 0 {
		config.BackoffMultiplier = DefaultRetryConfig().BackoffMultiplier
	}
	if config.JitterFraction < 0 || config.JitterFraction > 1 {
		config.JitterFraction = DefaultRetryConfig().JitterFraction
	}

	return &Retry{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute esegue una funzione con retry logic
func (r *Retry) Execute(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Esegui la funzione
		err := fn()

		// Se nessun errore, successo
		if err == nil {
			return nil
		}

		lastErr = err

		// Verifica se l'errore è retryable
		if !r.isRetryable(err) {
			log.Debug().
				Err(err).
				Msg("Error is not retryable, stopping retries")
			return err
		}

		// Se abbiamo esaurito i tentativi, restituisci l'errore
		if attempt >= r.config.MaxRetries {
			log.Warn().
				Err(err).
				Int("attempts", attempt+1).
				Msg("Max retries exceeded")
			return errors.Join(ErrMaxRetriesExceeded, err)
		}

		// Calcola il backoff
		backoff := r.calculateBackoff(attempt)

		// Notifica callback
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err, backoff)
		}

		log.Debug().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", r.config.MaxRetries).
			Dur("backoff", backoff).
			Msg("Retrying after error")

		// Attendi il backoff
		select {
		case <-time.After(backoff):
			// Continua con il prossimo tentativo
		case <-ctx.Done():
			// Context cancellato
			return ctx.Err()
		}
	}

	return lastErr
}

// calculateBackoff calcola il backoff per un tentativo
func (r *Retry) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: initial * multiplier^attempt
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffMultiplier, float64(attempt))

	// Limita al max backoff
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	// Aggiungi jitter se abilitato
	if r.config.Jitter {
		backoff = r.addJitter(backoff)
	}

	return time.Duration(backoff)
}

// addJitter aggiunge jitter al backoff
func (r *Retry) addJitter(backoff float64) float64 {
	// Jitter: backoff ± (backoff * jitterFraction * random(-1, 1))
	jitter := backoff * r.config.JitterFraction * (r.rng.Float64()*2 - 1)
	return backoff + jitter
}

// isRetryable verifica se un errore è retryable
func (r *Retry) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Usa checker custom se presente
	if r.config.RetryableChecker != nil {
		return r.config.RetryableChecker(err)
	}

	// Se non ci sono errori retryable configurati, ritenta tutto
	if len(r.config.RetryableErrors) == 0 {
		return true
	}

	// Verifica se l'errore è nella lista
	for _, retryableErr := range r.config.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// IsRetryableError restituisce true se l'errore è retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Errori sempre retryable
	retryableErrors := []error{
		context.DeadlineExceeded,
		ErrCircuitOpen,
		ErrBulkheadFull,
		ErrBulkheadTimeout,
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	// Verifica errori HTTP 5xx, timeout, ecc.
	// Questo può essere esteso con logica più complessa
	return false
}

// RetryStats contiene le statistiche dei retry
type RetryStats struct {
	TotalAttempts    int64
	TotalRetries     int64
	TotalSuccesses   int64
	TotalFailures    int64
	TotalBackoffTime time.Duration
}

// PerProviderRetry gestisce retry per ogni provider
type PerProviderRetry struct {
	config  RetryConfig
	mu      sync.RWMutex
	retries map[string]*Retry
	stats   map[string]*RetryStats
}

// NewPerProviderRetry crea un nuovo manager di retry per provider
func NewPerProviderRetry(config RetryConfig) *PerProviderRetry {
	return &PerProviderRetry{
		config:  config,
		retries: make(map[string]*Retry),
		stats:   make(map[string]*RetryStats),
	}
}

// Execute esegue una funzione con retry per uno specifico provider
func (ppr *PerProviderRetry) Execute(ctx context.Context, provider string, fn func() error) error {
	retry := ppr.getOrCreate(provider)

	startTime := time.Now()
	stats := ppr.getStats(provider)
	stats.TotalAttempts++

	// Wrapper per tracciare i retry
	attempt := 0
	wrappedFn := func() error {
		if attempt > 0 {
			stats.TotalRetries++
		}
		attempt++
		return fn()
	}

	err := retry.Execute(ctx, wrappedFn)

	// Aggiorna statistiche
	backoffTime := time.Since(startTime)
	stats.TotalBackoffTime += backoffTime

	if err != nil {
		stats.TotalFailures++
	} else {
		stats.TotalSuccesses++
	}

	return err
}

// getOrCreate ottiene o crea un retry handler per un provider
func (ppr *PerProviderRetry) getOrCreate(provider string) *Retry {
	ppr.mu.RLock()
	retry, exists := ppr.retries[provider]
	ppr.mu.RUnlock()

	if exists {
		return retry
	}

	ppr.mu.Lock()
	defer ppr.mu.Unlock()

	// Double-check dopo aver acquisito il write lock
	if retry, exists := ppr.retries[provider]; exists {
		return retry
	}

	retry = NewRetry(ppr.config)
	ppr.retries[provider] = retry
	ppr.stats[provider] = &RetryStats{}

	log.Debug().
		Str("provider", provider).
		Int("max_retries", ppr.config.MaxRetries).
		Msg("Created retry handler for provider")

	return retry
}

// getStats ottiene le statistiche per un provider
func (ppr *PerProviderRetry) getStats(provider string) *RetryStats {
	ppr.mu.RLock()
	stats, exists := ppr.stats[provider]
	ppr.mu.RUnlock()

	if exists {
		return stats
	}

	ppr.mu.Lock()
	defer ppr.mu.Unlock()

	stats = &RetryStats{}
	ppr.stats[provider] = stats
	return stats
}

// GetAllStats restituisce le statistiche di tutti i retry
func (ppr *PerProviderRetry) GetAllStats() map[string]RetryStats {
	ppr.mu.RLock()
	defer ppr.mu.RUnlock()

	result := make(map[string]RetryStats, len(ppr.stats))
	for provider, stats := range ppr.stats {
		result[provider] = *stats
	}

	return result
}

// ResetStats resetta le statistiche
func (ppr *PerProviderRetry) ResetStats() {
	ppr.mu.Lock()
	defer ppr.mu.Unlock()

	for provider := range ppr.stats {
		ppr.stats[provider] = &RetryStats{}
	}

	log.Info().Msg("Retry stats reset")
}
