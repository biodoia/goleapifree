package resilience

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Config contiene la configurazione completa del sistema di resilience
type Config struct {
	// CircuitBreaker configurazione circuit breaker
	CircuitBreaker CircuitBreakerConfig

	// Bulkhead configurazione bulkhead
	Bulkhead BulkheadConfig

	// Retry configurazione retry
	Retry RetryConfig

	// Fallback configurazione fallback
	Fallback FallbackConfig

	// EnableCircuitBreaker abilita circuit breaker
	EnableCircuitBreaker bool

	// EnableBulkhead abilita bulkhead
	EnableBulkhead bool

	// EnableRetry abilita retry
	EnableRetry bool

	// EnableFallback abilita fallback
	EnableFallback bool
}

// DefaultConfig restituisce una configurazione di default
func DefaultConfig() Config {
	return Config{
		CircuitBreaker:       DefaultCircuitBreakerConfig(),
		Bulkhead:             DefaultBulkheadConfig(),
		Retry:                DefaultRetryConfig(),
		Fallback:             DefaultFallbackConfig(),
		EnableCircuitBreaker: true,
		EnableBulkhead:       true,
		EnableRetry:          true,
		EnableFallback:       true,
	}
}

// Manager gestisce tutti i pattern di resilience
type Manager struct {
	config Config

	circuitBreaker *PerProviderCircuitBreaker
	bulkhead       *PerProviderBulkhead
	retry          *PerProviderRetry
	fallback       *PerProviderFallback

	cleanupStopCh chan struct{}
}

// NewManager crea un nuovo manager di resilience
func NewManager(config Config) *Manager {
	m := &Manager{
		config: config,
	}

	// Inizializza i componenti abilitati
	if config.EnableCircuitBreaker {
		m.circuitBreaker = NewPerProviderCircuitBreaker(config.CircuitBreaker)
		log.Info().Msg("Circuit breaker enabled")
	}

	if config.EnableBulkhead {
		m.bulkhead = NewPerProviderBulkhead(config.Bulkhead)
		log.Info().
			Int("max_concurrent", config.Bulkhead.MaxConcurrent).
			Int("max_queue", config.Bulkhead.MaxQueue).
			Msg("Bulkhead enabled")
	}

	if config.EnableRetry {
		m.retry = NewPerProviderRetry(config.Retry)
		log.Info().
			Int("max_retries", config.Retry.MaxRetries).
			Msg("Retry enabled")
	}

	if config.EnableFallback {
		m.fallback = NewPerProviderFallback(config.Fallback)
		// Avvia cleanup scheduler per la cache
		m.cleanupStopCh = m.fallback.StartCleanupScheduler(10 * time.Minute)
		log.Info().Msg("Fallback enabled")
	}

	return m
}

// Execute esegue una funzione con tutti i pattern di resilience
func (m *Manager) Execute(ctx context.Context, provider string, fn func() error) error {
	// Wrapper che applica tutti i pattern in ordine
	resilientFn := m.wrapWithResilience(ctx, provider, fn)

	// Esegui
	return resilientFn()
}

// ExecuteWithFallback esegue una funzione con fallback che restituisce un valore
func (m *Manager) ExecuteWithFallback(ctx context.Context, provider, key string, fn func() (interface{}, error)) (interface{}, error) {
	// Se il fallback è disabilitato, esegui solo la funzione base
	if !m.config.EnableFallback {
		// Wrapper per adattare la funzione
		var result interface{}
		var err error

		wrappedFn := func() error {
			result, err = fn()
			return err
		}

		execErr := m.Execute(ctx, provider, wrappedFn)
		if execErr != nil {
			return nil, execErr
		}

		return result, err
	}

	// Wrapper che applica circuit breaker, bulkhead e retry
	resilientFn := func() (interface{}, error) {
		var result interface{}
		var err error

		wrappedFn := func() error {
			result, err = fn()
			return err
		}

		execErr := m.Execute(ctx, provider, wrappedFn)
		if execErr != nil {
			return nil, execErr
		}

		return result, err
	}

	// Esegui con fallback
	return m.fallback.Execute(ctx, provider, key, resilientFn)
}

// wrapWithResilience wrappa una funzione con tutti i pattern di resilience
func (m *Manager) wrapWithResilience(ctx context.Context, provider string, fn func() error) func() error {
	// Costruisci la catena di resilience dall'interno verso l'esterno:
	// 1. Funzione originale
	// 2. Retry (più interno)
	// 3. Bulkhead
	// 4. Circuit Breaker (più esterno)

	resilientFn := fn

	// 1. Applica Retry (più interno - riprova la funzione originale)
	if m.config.EnableRetry && m.retry != nil {
		originalFn := resilientFn
		resilientFn = func() error {
			return m.retry.Execute(ctx, provider, originalFn)
		}
	}

	// 2. Applica Bulkhead (limita concorrenza)
	if m.config.EnableBulkhead && m.bulkhead != nil {
		originalFn := resilientFn
		resilientFn = func() error {
			return m.bulkhead.Execute(ctx, provider, originalFn)
		}
	}

	// 3. Applica Circuit Breaker (più esterno - previene chiamate a servizi down)
	if m.config.EnableCircuitBreaker && m.circuitBreaker != nil {
		originalFn := resilientFn
		resilientFn = func() error {
			return m.circuitBreaker.Execute(ctx, provider, originalFn)
		}
	}

	return resilientFn
}

// IsProviderAvailable verifica se un provider è disponibile
func (m *Manager) IsProviderAvailable(provider string) bool {
	// Verifica circuit breaker
	if m.config.EnableCircuitBreaker && m.circuitBreaker != nil {
		if !m.circuitBreaker.IsProviderAvailable(provider) {
			return false
		}
	}

	// Verifica bulkhead
	if m.config.EnableBulkhead && m.bulkhead != nil {
		if !m.bulkhead.IsProviderAvailable(provider) {
			return false
		}
	}

	return true
}

// ResetProvider resetta tutti i pattern per un provider
func (m *Manager) ResetProvider(provider string) {
	if m.circuitBreaker != nil {
		m.circuitBreaker.Reset(provider)
	}

	log.Info().
		Str("provider", provider).
		Msg("Provider resilience reset")
}

// ResetAll resetta tutti i pattern per tutti i provider
func (m *Manager) ResetAll() {
	if m.circuitBreaker != nil {
		m.circuitBreaker.ResetAll()
	}

	if m.retry != nil {
		m.retry.ResetStats()
	}

	if m.fallback != nil {
		m.fallback.ClearAllCaches()
	}

	log.Info().Msg("All resilience patterns reset")
}

// GetStats restituisce le statistiche complete
func (m *Manager) GetStats() Stats {
	stats := Stats{
		Providers: make(map[string]ProviderStats),
	}

	// Raccogli tutte le statistiche per provider
	if m.circuitBreaker != nil {
		cbStats := m.circuitBreaker.GetAllStats()
		for provider, cbStat := range cbStats {
			providerStat := stats.Providers[provider]
			providerStat.CircuitBreaker = cbStat
			stats.Providers[provider] = providerStat
		}
	}

	if m.bulkhead != nil {
		bhStats := m.bulkhead.GetAllStats()
		for provider, bhStat := range bhStats {
			providerStat := stats.Providers[provider]
			providerStat.Bulkhead = bhStat
			stats.Providers[provider] = providerStat
		}
	}

	if m.retry != nil {
		retryStats := m.retry.GetAllStats()
		for provider, retryStat := range retryStats {
			providerStat := stats.Providers[provider]
			providerStat.Retry = retryStat
			stats.Providers[provider] = providerStat
		}
	}

	if m.fallback != nil {
		fbStats := m.fallback.GetAllStats()
		for provider, fbStat := range fbStats {
			providerStat := stats.Providers[provider]
			providerStat.Fallback = fbStat
			stats.Providers[provider] = providerStat
		}
	}

	return stats
}

// Stats contiene tutte le statistiche di resilience
type Stats struct {
	Providers map[string]ProviderStats
}

// ProviderStats contiene le statistiche per un singolo provider
type ProviderStats struct {
	CircuitBreaker CircuitBreakerStats
	Bulkhead       BulkheadStats
	Retry          RetryStats
	Fallback       FallbackStats
}

// String restituisce una rappresentazione string delle statistiche
func (s ProviderStats) String() string {
	return fmt.Sprintf(
		"CircuitBreaker: %s, Bulkhead: %d/%d active, Retry: %d retries, Fallback: %d fallbacks",
		s.CircuitBreaker.State,
		s.Bulkhead.ActiveRequests,
		s.Bulkhead.MaxConcurrent,
		s.Retry.TotalRetries,
		s.Fallback.TotalFallbacks,
	)
}

// Close chiude il manager e libera le risorse
func (m *Manager) Close() {
	if m.bulkhead != nil {
		m.bulkhead.Close()
	}

	if m.cleanupStopCh != nil {
		close(m.cleanupStopCh)
	}

	log.Info().Msg("Resilience manager closed")
}

// HealthCheck verifica lo stato di salute del sistema di resilience
func (m *Manager) HealthCheck() HealthStatus {
	status := HealthStatus{
		Healthy:   true,
		Providers: make(map[string]ProviderHealth),
	}

	stats := m.GetStats()

	for provider, providerStats := range stats.Providers {
		providerHealth := ProviderHealth{
			Available: m.IsProviderAvailable(provider),
		}

		// Circuit breaker aperto = problema
		if providerStats.CircuitBreaker.State == "open" {
			providerHealth.Available = false
			providerHealth.Issues = append(providerHealth.Issues, "circuit breaker open")
			status.Healthy = false
		}

		// Bulkhead pieno = problema
		if providerStats.Bulkhead.AvailableSlots == 0 {
			providerHealth.Issues = append(providerHealth.Issues, "bulkhead full")
		}

		// Troppi fallback = warning
		if providerStats.Fallback.TotalFallbacks > 0 {
			fallbackRate := float64(providerStats.Fallback.TotalFallbacks) / float64(providerStats.Fallback.TotalRequests)
			if fallbackRate > 0.5 {
				providerHealth.Issues = append(providerHealth.Issues, fmt.Sprintf("high fallback rate: %.2f%%", fallbackRate*100))
			}
		}

		status.Providers[provider] = providerHealth
	}

	return status
}

// HealthStatus rappresenta lo stato di salute del sistema
type HealthStatus struct {
	Healthy   bool
	Providers map[string]ProviderHealth
}

// ProviderHealth rappresenta lo stato di salute di un provider
type ProviderHealth struct {
	Available bool
	Issues    []string
}
