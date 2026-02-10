package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	// ErrAllFallbacksFailed viene restituito quando tutti i fallback falliscono
	ErrAllFallbacksFailed = errors.New("all fallback strategies failed")

	// ErrNoFallbackAvailable viene restituito quando non ci sono fallback disponibili
	ErrNoFallbackAvailable = errors.New("no fallback available")
)

// FallbackStrategy rappresenta una strategia di fallback
type FallbackStrategy int

const (
	// FallbackToCache usa una risposta dalla cache
	FallbackToCache FallbackStrategy = iota

	// FallbackToProvider prova un provider alternativo
	FallbackToProvider

	// FallbackToDegraded restituisce una risposta degradata
	FallbackToDegraded

	// FallbackToError restituisce un errore strutturato
	FallbackToError

	// FallbackToStale usa dati stale dalla cache
	FallbackToStale
)

// String restituisce la rappresentazione string della strategia
func (s FallbackStrategy) String() string {
	switch s {
	case FallbackToCache:
		return "cache"
	case FallbackToProvider:
		return "provider"
	case FallbackToDegraded:
		return "degraded"
	case FallbackToError:
		return "error"
	case FallbackToStale:
		return "stale"
	default:
		return "unknown"
	}
}

// FallbackConfig contiene la configurazione del fallback
type FallbackConfig struct {
	// Strategies lista di strategie da provare in ordine
	Strategies []FallbackStrategy

	// EnableCache abilita fallback a cache
	EnableCache bool

	// CacheTTL TTL della cache per fallback
	CacheTTL time.Duration

	// EnableStale abilita l'uso di dati stale
	EnableStale bool

	// StaleTTL TTL per dati stale
	StaleTTL time.Duration

	// AlternativeProviders lista di provider alternativi
	AlternativeProviders []string

	// DegradedResponse risposta degradata di default
	DegradedResponse interface{}

	// OnFallback callback chiamata quando si usa un fallback
	OnFallback func(strategy FallbackStrategy, reason error)
}

// DefaultFallbackConfig restituisce una configurazione di default
func DefaultFallbackConfig() FallbackConfig {
	return FallbackConfig{
		Strategies: []FallbackStrategy{
			FallbackToCache,
			FallbackToProvider,
			FallbackToDegraded,
		},
		EnableCache:          true,
		CacheTTL:             5 * time.Minute,
		EnableStale:          true,
		StaleTTL:             30 * time.Minute,
		AlternativeProviders: nil,
		DegradedResponse:     nil,
		OnFallback:           nil,
	}
}

// CacheEntry rappresenta una entry nella cache di fallback
type CacheEntry struct {
	Data      interface{}
	Timestamp time.Time
	TTL       time.Duration
}

// IsValid verifica se la cache entry è valida
func (e *CacheEntry) IsValid() bool {
	if e == nil {
		return false
	}
	return time.Since(e.Timestamp) < e.TTL
}

// IsStale verifica se la cache entry è stale ma utilizzabile
func (e *CacheEntry) IsStale(staleTTL time.Duration) bool {
	if e == nil {
		return false
	}
	age := time.Since(e.Timestamp)
	return age >= e.TTL && age < staleTTL
}

// Fallback implementa strategie di fallback
type Fallback struct {
	config FallbackConfig

	mu    sync.RWMutex
	cache map[string]*CacheEntry

	// Statistiche
	totalRequests      int64
	totalFallbacks     int64
	fallbacksByStrategy map[FallbackStrategy]int64
}

// NewFallback crea un nuovo fallback handler
func NewFallback(config FallbackConfig) *Fallback {
	if config.CacheTTL <= 0 {
		config.CacheTTL = DefaultFallbackConfig().CacheTTL
	}
	if config.StaleTTL <= 0 {
		config.StaleTTL = DefaultFallbackConfig().StaleTTL
	}
	if len(config.Strategies) == 0 {
		config.Strategies = DefaultFallbackConfig().Strategies
	}

	return &Fallback{
		config:              config,
		cache:               make(map[string]*CacheEntry),
		fallbacksByStrategy: make(map[FallbackStrategy]int64),
	}
}

// Execute esegue una funzione con fallback strategies
func (f *Fallback) Execute(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	f.mu.Lock()
	f.totalRequests++
	f.mu.Unlock()

	// Prova ad eseguire la funzione principale
	result, err := fn()

	// Se successo, aggiorna la cache e restituisci
	if err == nil {
		f.setCache(key, result)
		return result, nil
	}

	log.Debug().
		Err(err).
		Str("key", key).
		Msg("Primary execution failed, trying fallback strategies")

	// Fallback primario fallito, prova le strategie
	return f.executeFallback(ctx, key, err)
}

// executeFallback esegue le strategie di fallback
func (f *Fallback) executeFallback(ctx context.Context, key string, originalErr error) (interface{}, error) {
	f.mu.Lock()
	f.totalFallbacks++
	f.mu.Unlock()

	var lastErr error

	for _, strategy := range f.config.Strategies {
		log.Debug().
			Str("strategy", strategy.String()).
			Str("key", key).
			Msg("Trying fallback strategy")

		result, err := f.tryStrategy(ctx, key, strategy, originalErr)

		if err == nil {
			// Strategia riuscita
			f.recordFallback(strategy, originalErr)
			return result, nil
		}

		lastErr = err
	}

	// Tutte le strategie fallite
	log.Warn().
		Err(lastErr).
		Str("key", key).
		Msg("All fallback strategies failed")

	return nil, errors.Join(ErrAllFallbacksFailed, originalErr, lastErr)
}

// tryStrategy prova una specifica strategia di fallback
func (f *Fallback) tryStrategy(ctx context.Context, key string, strategy FallbackStrategy, originalErr error) (interface{}, error) {
	switch strategy {
	case FallbackToCache:
		return f.tryCache(key)

	case FallbackToStale:
		return f.tryStale(key)

	case FallbackToDegraded:
		return f.tryDegraded(key)

	case FallbackToError:
		return nil, originalErr

	case FallbackToProvider:
		// Questa strategia richiede logica esterna
		return nil, ErrNoFallbackAvailable

	default:
		return nil, ErrNoFallbackAvailable
	}
}

// tryCache prova a usare la cache
func (f *Fallback) tryCache(key string) (interface{}, error) {
	if !f.config.EnableCache {
		return nil, ErrNoFallbackAvailable
	}

	f.mu.RLock()
	entry, exists := f.cache[key]
	f.mu.RUnlock()

	if !exists || !entry.IsValid() {
		return nil, errors.New("no valid cache entry")
	}

	log.Info().
		Str("key", key).
		Dur("age", time.Since(entry.Timestamp)).
		Msg("Using cached fallback response")

	return entry.Data, nil
}

// tryStale prova a usare dati stale
func (f *Fallback) tryStale(key string) (interface{}, error) {
	if !f.config.EnableStale {
		return nil, ErrNoFallbackAvailable
	}

	f.mu.RLock()
	entry, exists := f.cache[key]
	f.mu.RUnlock()

	if !exists || !entry.IsStale(f.config.StaleTTL) {
		return nil, errors.New("no stale cache entry")
	}

	log.Warn().
		Str("key", key).
		Dur("age", time.Since(entry.Timestamp)).
		Msg("Using stale fallback response")

	return entry.Data, nil
}

// tryDegraded prova a usare una risposta degradata
func (f *Fallback) tryDegraded(key string) (interface{}, error) {
	if f.config.DegradedResponse == nil {
		return nil, ErrNoFallbackAvailable
	}

	log.Warn().
		Str("key", key).
		Msg("Using degraded fallback response")

	return f.config.DegradedResponse, nil
}

// setCache imposta una entry nella cache
func (f *Fallback) setCache(key string, data interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       f.config.CacheTTL,
	}
}

// recordFallback registra l'uso di una strategia di fallback
func (f *Fallback) recordFallback(strategy FallbackStrategy, reason error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fallbacksByStrategy[strategy]++

	if f.config.OnFallback != nil {
		go f.config.OnFallback(strategy, reason)
	}

	log.Info().
		Str("strategy", strategy.String()).
		Err(reason).
		Msg("Fallback strategy succeeded")
}

// GetStats restituisce le statistiche del fallback
func (f *Fallback) GetStats() FallbackStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	strategies := make(map[string]int64)
	for strategy, count := range f.fallbacksByStrategy {
		strategies[strategy.String()] = count
	}

	return FallbackStats{
		TotalRequests:        f.totalRequests,
		TotalFallbacks:       f.totalFallbacks,
		FallbacksByStrategy:  strategies,
		CacheEntries:         len(f.cache),
	}
}

// FallbackStats contiene le statistiche del fallback
type FallbackStats struct {
	TotalRequests       int64
	TotalFallbacks      int64
	FallbacksByStrategy map[string]int64
	CacheEntries        int
}

// ClearCache pulisce la cache
func (f *Fallback) ClearCache() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache = make(map[string]*CacheEntry)
	log.Info().Msg("Fallback cache cleared")
}

// CleanupStaleEntries rimuove entry stale dalla cache
func (f *Fallback) CleanupStaleEntries() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, entry := range f.cache {
		if now.Sub(entry.Timestamp) > f.config.StaleTTL {
			delete(f.cache, key)
			removed++
		}
	}

	if removed > 0 {
		log.Debug().
			Int("removed", removed).
			Msg("Removed stale cache entries")
	}

	return removed
}

// PerProviderFallback gestisce fallback per ogni provider
type PerProviderFallback struct {
	config    FallbackConfig
	mu        sync.RWMutex
	fallbacks map[string]*Fallback
}

// NewPerProviderFallback crea un nuovo manager di fallback per provider
func NewPerProviderFallback(config FallbackConfig) *PerProviderFallback {
	return &PerProviderFallback{
		config:    config,
		fallbacks: make(map[string]*Fallback),
	}
}

// Execute esegue una funzione con fallback per uno specifico provider
func (ppf *PerProviderFallback) Execute(ctx context.Context, provider, key string, fn func() (interface{}, error)) (interface{}, error) {
	fallback := ppf.getOrCreate(provider)
	return fallback.Execute(ctx, key, fn)
}

// getOrCreate ottiene o crea un fallback handler per un provider
func (ppf *PerProviderFallback) getOrCreate(provider string) *Fallback {
	ppf.mu.RLock()
	fallback, exists := ppf.fallbacks[provider]
	ppf.mu.RUnlock()

	if exists {
		return fallback
	}

	ppf.mu.Lock()
	defer ppf.mu.Unlock()

	// Double-check dopo aver acquisito il write lock
	if fallback, exists := ppf.fallbacks[provider]; exists {
		return fallback
	}

	fallback = NewFallback(ppf.config)
	ppf.fallbacks[provider] = fallback

	log.Debug().
		Str("provider", provider).
		Msg("Created fallback handler for provider")

	return fallback
}

// GetFallback restituisce il fallback handler per un provider
func (ppf *PerProviderFallback) GetFallback(provider string) (*Fallback, bool) {
	ppf.mu.RLock()
	defer ppf.mu.RUnlock()

	fallback, exists := ppf.fallbacks[provider]
	return fallback, exists
}

// GetAllStats restituisce le statistiche di tutti i fallback
func (ppf *PerProviderFallback) GetAllStats() map[string]FallbackStats {
	ppf.mu.RLock()
	defer ppf.mu.RUnlock()

	stats := make(map[string]FallbackStats, len(ppf.fallbacks))
	for provider, fallback := range ppf.fallbacks {
		stats[provider] = fallback.GetStats()
	}

	return stats
}

// ClearAllCaches pulisce tutte le cache
func (ppf *PerProviderFallback) ClearAllCaches() {
	ppf.mu.RLock()
	defer ppf.mu.RUnlock()

	for _, fallback := range ppf.fallbacks {
		fallback.ClearCache()
	}

	log.Info().Msg("All fallback caches cleared")
}

// CleanupAllStaleEntries rimuove entry stale da tutte le cache
func (ppf *PerProviderFallback) CleanupAllStaleEntries() int {
	ppf.mu.RLock()
	defer ppf.mu.RUnlock()

	totalRemoved := 0
	for _, fallback := range ppf.fallbacks {
		removed := fallback.CleanupStaleEntries()
		totalRemoved += removed
	}

	return totalRemoved
}

// StartCleanupScheduler avvia un cleanup scheduler periodico
func (ppf *PerProviderFallback) StartCleanupScheduler(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				removed := ppf.CleanupAllStaleEntries()
				if removed > 0 {
					log.Debug().
						Int("removed", removed).
						Msg("Periodic cache cleanup completed")
				}

			case <-stopCh:
				log.Info().Msg("Fallback cleanup scheduler stopped")
				return
			}
		}
	}()

	log.Info().
		Dur("interval", interval).
		Msg("Fallback cleanup scheduler started")

	return stopCh
}
