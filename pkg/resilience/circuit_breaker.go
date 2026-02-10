package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	// ErrCircuitOpen viene restituito quando il circuit breaker è aperto
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests viene restituito quando ci sono troppe richieste in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// State rappresenta lo stato del circuit breaker
type State int

const (
	// StateClosed il circuito è chiuso, le richieste passano normalmente
	StateClosed State = iota

	// StateOpen il circuito è aperto, le richieste vengono rifiutate
	StateOpen

	// StateHalfOpen il circuito sta testando se tornare chiuso
	StateHalfOpen
)

// String restituisce la rappresentazione string dello stato
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig contiene la configurazione del circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold numero di errori consecutivi prima di aprire il circuito
	FailureThreshold int

	// SuccessThreshold numero di successi consecutivi in half-open prima di chiudere
	SuccessThreshold int

	// Timeout durata prima di passare da open a half-open
	Timeout time.Duration

	// HalfOpenMaxRequests numero massimo di richieste in half-open
	HalfOpenMaxRequests int

	// OnStateChange callback chiamata quando lo stato cambia
	OnStateChange func(from, to State)
}

// DefaultCircuitBreakerConfig restituisce una configurazione di default
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:            60 * time.Second,
		HalfOpenMaxRequests: 3,
		OnStateChange:      nil,
	}
}

// CircuitBreaker implementa il pattern circuit breaker per prevenire cascading failures
type CircuitBreaker struct {
	config CircuitBreakerConfig

	mu                 sync.RWMutex
	state              State
	failures           int
	successes          int
	lastFailureTime    time.Time
	nextRetryTime      time.Time
	halfOpenRequests   int

	// Statistiche
	totalRequests      int64
	totalFailures      int64
	totalSuccesses     int64
	totalRejected      int64
}

// NewCircuitBreaker crea un nuovo circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = DefaultCircuitBreakerConfig().FailureThreshold
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = DefaultCircuitBreakerConfig().SuccessThreshold
	}
	if config.Timeout <= 0 {
		config.Timeout = DefaultCircuitBreakerConfig().Timeout
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = DefaultCircuitBreakerConfig().HalfOpenMaxRequests
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute esegue una funzione protetta dal circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Verifica se possiamo procedere
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Esegui la funzione
	err := fn()

	// Gestisci il risultato
	cb.afterRequest(err)

	return err
}

// beforeRequest verifica se la richiesta può procedere
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case StateClosed:
		// Le richieste passano normalmente
		return nil

	case StateOpen:
		// Verifica se è il momento di passare in half-open
		if time.Now().After(cb.nextRetryTime) {
			cb.setState(StateHalfOpen)
			cb.halfOpenRequests = 0
			return nil
		}

		// Rigetta la richiesta
		cb.totalRejected++
		return ErrCircuitOpen

	case StateHalfOpen:
		// Limita il numero di richieste in half-open
		if cb.halfOpenRequests >= cb.config.HalfOpenMaxRequests {
			cb.totalRejected++
			return ErrTooManyRequests
		}

		cb.halfOpenRequests++
		return nil

	default:
		return nil
	}
}

// afterRequest gestisce il risultato della richiesta
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure gestisce un fallimento
func (cb *CircuitBreaker) onFailure() {
	cb.totalFailures++
	cb.failures++
	cb.successes = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Se superiamo la soglia, apriamo il circuito
		if cb.failures >= cb.config.FailureThreshold {
			cb.open()
		}

	case StateHalfOpen:
		// In half-open, qualsiasi errore riapre il circuito
		cb.open()
	}
}

// onSuccess gestisce un successo
func (cb *CircuitBreaker) onSuccess() {
	cb.totalSuccesses++
	cb.successes++
	cb.failures = 0

	switch cb.state {
	case StateHalfOpen:
		// Se superiamo la soglia di successi, chiudiamo il circuito
		if cb.successes >= cb.config.SuccessThreshold {
			cb.close()
		}
	}
}

// open apre il circuito
func (cb *CircuitBreaker) open() {
	cb.setState(StateOpen)
	cb.nextRetryTime = time.Now().Add(cb.config.Timeout)
	cb.failures = 0
	cb.successes = 0

	log.Warn().
		Str("next_retry", cb.nextRetryTime.Format(time.RFC3339)).
		Msg("Circuit breaker opened")
}

// close chiude il circuito
func (cb *CircuitBreaker) close() {
	cb.setState(StateClosed)
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0

	log.Info().Msg("Circuit breaker closed")
}

// setState cambia lo stato e notifica
func (cb *CircuitBreaker) setState(newState State) {
	oldState := cb.state
	cb.state = newState

	if cb.config.OnStateChange != nil && oldState != newState {
		// Esegui la callback fuori dal lock
		go cb.config.OnStateChange(oldState, newState)
	}
}

// GetState restituisce lo stato corrente
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen verifica se il circuito è aperto
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// IsClosed verifica se il circuito è chiuso
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.GetState() == StateClosed
}

// IsHalfOpen verifica se il circuito è half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.GetState() == StateHalfOpen
}

// Reset resetta il circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.lastFailureTime = time.Time{}
	cb.nextRetryTime = time.Time{}

	log.Info().Msg("Circuit breaker reset")
}

// GetStats restituisce le statistiche del circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:             cb.state.String(),
		TotalRequests:     cb.totalRequests,
		TotalFailures:     cb.totalFailures,
		TotalSuccesses:    cb.totalSuccesses,
		TotalRejected:     cb.totalRejected,
		ConsecutiveFailures: cb.failures,
		ConsecutiveSuccesses: cb.successes,
		LastFailureTime:   cb.lastFailureTime,
		NextRetryTime:     cb.nextRetryTime,
	}
}

// CircuitBreakerStats contiene le statistiche del circuit breaker
type CircuitBreakerStats struct {
	State                string
	TotalRequests        int64
	TotalFailures        int64
	TotalSuccesses       int64
	TotalRejected        int64
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastFailureTime      time.Time
	NextRetryTime        time.Time
}

// PerProviderCircuitBreaker gestisce circuit breaker per ogni provider
type PerProviderCircuitBreaker struct {
	config   CircuitBreakerConfig
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewPerProviderCircuitBreaker crea un nuovo manager di circuit breaker per provider
func NewPerProviderCircuitBreaker(config CircuitBreakerConfig) *PerProviderCircuitBreaker {
	return &PerProviderCircuitBreaker{
		config:   config,
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Execute esegue una funzione con circuit breaker per uno specifico provider
func (ppcb *PerProviderCircuitBreaker) Execute(ctx context.Context, provider string, fn func() error) error {
	breaker := ppcb.getOrCreate(provider)
	return breaker.Execute(ctx, fn)
}

// getOrCreate ottiene o crea un circuit breaker per un provider
func (ppcb *PerProviderCircuitBreaker) getOrCreate(provider string) *CircuitBreaker {
	ppcb.mu.RLock()
	breaker, exists := ppcb.breakers[provider]
	ppcb.mu.RUnlock()

	if exists {
		return breaker
	}

	ppcb.mu.Lock()
	defer ppcb.mu.Unlock()

	// Double-check dopo aver acquisito il write lock
	if breaker, exists := ppcb.breakers[provider]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(ppcb.config)
	ppcb.breakers[provider] = breaker

	log.Debug().
		Str("provider", provider).
		Msg("Created circuit breaker for provider")

	return breaker
}

// GetBreaker restituisce il circuit breaker per un provider
func (ppcb *PerProviderCircuitBreaker) GetBreaker(provider string) (*CircuitBreaker, bool) {
	ppcb.mu.RLock()
	defer ppcb.mu.RUnlock()

	breaker, exists := ppcb.breakers[provider]
	return breaker, exists
}

// Reset resetta il circuit breaker per un provider
func (ppcb *PerProviderCircuitBreaker) Reset(provider string) {
	if breaker, exists := ppcb.GetBreaker(provider); exists {
		breaker.Reset()
	}
}

// ResetAll resetta tutti i circuit breaker
func (ppcb *PerProviderCircuitBreaker) ResetAll() {
	ppcb.mu.RLock()
	defer ppcb.mu.RUnlock()

	for _, breaker := range ppcb.breakers {
		breaker.Reset()
	}

	log.Info().Msg("All circuit breakers reset")
}

// GetAllStats restituisce le statistiche di tutti i circuit breaker
func (ppcb *PerProviderCircuitBreaker) GetAllStats() map[string]CircuitBreakerStats {
	ppcb.mu.RLock()
	defer ppcb.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats, len(ppcb.breakers))
	for provider, breaker := range ppcb.breakers {
		stats[provider] = breaker.GetStats()
	}

	return stats
}

// IsProviderAvailable verifica se un provider è disponibile (circuito non aperto)
func (ppcb *PerProviderCircuitBreaker) IsProviderAvailable(provider string) bool {
	breaker, exists := ppcb.GetBreaker(provider)
	if !exists {
		return true // Se non esiste ancora il breaker, consideriamo il provider disponibile
	}

	return !breaker.IsOpen()
}
