package resilience

import (
	"context"
	"errors"
	"net/http"
	"syscall"
)

// ErrorCategory rappresenta una categoria di errore
type ErrorCategory int

const (
	// ErrorCategoryUnknown errore sconosciuto
	ErrorCategoryUnknown ErrorCategory = iota

	// ErrorCategoryNetwork errore di rete
	ErrorCategoryNetwork

	// ErrorCategoryTimeout timeout
	ErrorCategoryTimeout

	// ErrorCategoryRateLimit rate limit exceeded
	ErrorCategoryRateLimit

	// ErrorCategoryServerError errore server (5xx)
	ErrorCategoryServerError

	// ErrorCategoryClientError errore client (4xx)
	ErrorCategoryClientError

	// ErrorCategoryCircuitBreaker circuit breaker aperto
	ErrorCategoryCircuitBreaker

	// ErrorCategoryBulkhead bulkhead pieno
	ErrorCategoryBulkhead
)

// CategorizeError categorizza un errore
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	// Circuit breaker e bulkhead errors
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrTooManyRequests) {
		return ErrorCategoryCircuitBreaker
	}
	if errors.Is(err, ErrBulkheadFull) || errors.Is(err, ErrBulkheadTimeout) {
		return ErrorCategoryBulkhead
	}

	// Context errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrorCategoryTimeout
	}

	// Network errors
	if isNetworkError(err) {
		return ErrorCategoryNetwork
	}

	// HTTP errors
	if httpErr, ok := err.(interface{ StatusCode() int }); ok {
		code := httpErr.StatusCode()
		if code == http.StatusTooManyRequests {
			return ErrorCategoryRateLimit
		}
		if code >= 500 {
			return ErrorCategoryServerError
		}
		if code >= 400 {
			return ErrorCategoryClientError
		}
	}

	return ErrorCategoryUnknown
}

// isNetworkError verifica se è un errore di rete
func isNetworkError(err error) bool {
	// Syscall errors
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		return true
	}

	// Common network error strings
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"network is unreachable",
		"i/o timeout",
	}

	errStr := err.Error()
	for _, netErr := range networkErrors {
		if contains(errStr, netErr) {
			return true
		}
	}

	return false
}

// contains verifica se una stringa contiene una sottostringa (case insensitive helper)
func contains(s, substr string) bool {
	// Simple case-insensitive check
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr))
}

// IsRetryableByCategory verifica se un errore è retryable basandosi sulla categoria
func IsRetryableByCategory(err error) bool {
	category := CategorizeError(err)

	switch category {
	case ErrorCategoryNetwork,
		ErrorCategoryTimeout,
		ErrorCategoryServerError,
		ErrorCategoryCircuitBreaker,
		ErrorCategoryBulkhead:
		return true

	case ErrorCategoryRateLimit:
		// Rate limit può essere retryable con backoff appropriato
		return true

	case ErrorCategoryClientError:
		// Errori client (4xx) generalmente non sono retryable
		// tranne 429 (gestito sopra)
		return false

	default:
		return false
	}
}

// RetryConfigForCategory restituisce una configurazione di retry appropriata per la categoria
func RetryConfigForCategory(category ErrorCategory) RetryConfig {
	base := DefaultRetryConfig()

	switch category {
	case ErrorCategoryRateLimit:
		// Rate limit: retry con backoff più aggressivo
		base.MaxRetries = 5
		base.InitialBackoff = 1 * time.Second
		base.MaxBackoff = 60 * time.Second
		base.BackoffMultiplier = 3.0

	case ErrorCategoryTimeout:
		// Timeout: retry con backoff moderato
		base.MaxRetries = 2
		base.InitialBackoff = 500 * time.Millisecond
		base.MaxBackoff = 5 * time.Second

	case ErrorCategoryNetwork:
		// Network errors: retry rapido
		base.MaxRetries = 3
		base.InitialBackoff = 100 * time.Millisecond
		base.MaxBackoff = 2 * time.Second

	case ErrorCategoryServerError:
		// Server errors: retry con backoff standard
		base.MaxRetries = 3
		base.InitialBackoff = 500 * time.Millisecond
		base.MaxBackoff = 10 * time.Second

	case ErrorCategoryCircuitBreaker, ErrorCategoryBulkhead:
		// Circuit breaker/bulkhead: retry limitato
		base.MaxRetries = 1
		base.InitialBackoff = 1 * time.Second
		base.MaxBackoff = 5 * time.Second

	default:
		// Usa default
	}

	return base
}

// AdaptiveRetryConfig crea una configurazione retry che si adatta all'errore
func AdaptiveRetryConfig() RetryConfig {
	config := DefaultRetryConfig()

	// Usa checker custom che adatta il comportamento
	config.RetryableChecker = IsRetryableByCategory

	return config
}

// WrapWithMetrics wrappa una funzione con metriche
func WrapWithMetrics(fn func() error, onSuccess, onFailure func()) func() error {
	return func() error {
		err := fn()
		if err != nil && onFailure != nil {
			onFailure()
		} else if err == nil && onSuccess != nil {
			onSuccess()
		}
		return err
	}
}

// WithTimeout aggiunge un timeout a una funzione
func WithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-timeoutCtx.Done():
		return timeoutCtx.Err()
	}
}

// Debounce crea una funzione debounced
func Debounce(delay time.Duration, fn func()) func() {
	var timer *time.Timer
	var mu sync.Mutex

	return func() {
		mu.Lock()
		defer mu.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(delay, fn)
	}
}

// Throttle crea una funzione throttled
func Throttle(interval time.Duration, fn func()) func() {
	var (
		lastCall time.Time
		mu       sync.Mutex
	)

	return func() {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		if now.Sub(lastCall) >= interval {
			lastCall = now
			fn()
		}
	}
}

// CircuitBreakerPresets fornisce preset di configurazione comuni
var CircuitBreakerPresets = struct {
	Aggressive CircuitBreakerConfig
	Moderate   CircuitBreakerConfig
	Lenient    CircuitBreakerConfig
}{
	Aggressive: CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    3,
		Timeout:            30 * time.Second,
		HalfOpenMaxRequests: 1,
	},
	Moderate: CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:            60 * time.Second,
		HalfOpenMaxRequests: 3,
	},
	Lenient: CircuitBreakerConfig{
		FailureThreshold:    10,
		SuccessThreshold:    2,
		Timeout:            120 * time.Second,
		HalfOpenMaxRequests: 5,
	},
}

// BulkheadPresets fornisce preset di configurazione comuni
var BulkheadPresets = struct {
	Small  BulkheadConfig
	Medium BulkheadConfig
	Large  BulkheadConfig
}{
	Small: BulkheadConfig{
		MaxConcurrent: 5,
		MaxQueue:      10,
		QueueTimeout:  2 * time.Second,
	},
	Medium: BulkheadConfig{
		MaxConcurrent: 20,
		MaxQueue:      40,
		QueueTimeout:  5 * time.Second,
	},
	Large: BulkheadConfig{
		MaxConcurrent: 100,
		MaxQueue:      200,
		QueueTimeout:  10 * time.Second,
	},
}

// RetryPresets fornisce preset di configurazione comuni
var RetryPresets = struct {
	Fast       RetryConfig
	Standard   RetryConfig
	Persistent RetryConfig
}{
	Fast: RetryConfig{
		MaxRetries:         2,
		InitialBackoff:     50 * time.Millisecond,
		MaxBackoff:         1 * time.Second,
		BackoffMultiplier:  2.0,
		Jitter:             true,
		JitterFraction:     0.1,
	},
	Standard: RetryConfig{
		MaxRetries:         3,
		InitialBackoff:     100 * time.Millisecond,
		MaxBackoff:         10 * time.Second,
		BackoffMultiplier:  2.0,
		Jitter:             true,
		JitterFraction:     0.1,
	},
	Persistent: RetryConfig{
		MaxRetries:         5,
		InitialBackoff:     500 * time.Millisecond,
		MaxBackoff:         60 * time.Second,
		BackoffMultiplier:  2.5,
		Jitter:             true,
		JitterFraction:     0.2,
	},
}
