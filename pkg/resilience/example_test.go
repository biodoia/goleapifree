package resilience_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/resilience"
)

// Simula un provider LLM flaky
type FlakyProvider struct {
	failureRate float64
	callCount   int
}

func (p *FlakyProvider) Call() error {
	p.callCount++

	// Simula failure rate
	if p.callCount%3 == 0 {
		return errors.New("provider temporarily unavailable")
	}

	// Simula latenza
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Example_circuitBreaker dimostra l'uso del circuit breaker
func Example_circuitBreaker() {
	// Configura circuit breaker
	config := resilience.CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             5 * time.Second,
		HalfOpenMaxRequests: 2,
	}

	cb := resilience.NewCircuitBreaker(config)

	provider := &FlakyProvider{}

	// Esegui richieste
	for i := 0; i < 10; i++ {
		ctx := context.Background()

		err := cb.Execute(ctx, func() error {
			return provider.Call()
		})

		if err != nil {
			fmt.Printf("Request %d failed: %v (circuit: %s)\n", i+1, err, cb.GetState())
		} else {
			fmt.Printf("Request %d succeeded (circuit: %s)\n", i+1, cb.GetState())
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Mostra statistiche
	stats := cb.GetStats()
	fmt.Printf("\nCircuit Breaker Stats:\n")
	fmt.Printf("- State: %s\n", stats.State)
	fmt.Printf("- Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("- Total Failures: %d\n", stats.TotalFailures)
	fmt.Printf("- Total Rejected: %d\n", stats.TotalRejected)
}

// Example_bulkhead dimostra l'uso del bulkhead
func Example_bulkhead() {
	// Configura bulkhead
	config := resilience.BulkheadConfig{
		MaxConcurrent: 3,
		MaxQueue:      5,
		QueueTimeout:  2 * time.Second,
	}

	bulkhead := resilience.NewBulkhead(config)
	defer bulkhead.Close()

	// Simula burst di richieste
	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := context.Background()

			err := bulkhead.Execute(ctx, func() error {
				fmt.Printf("Request %d executing...\n", id)
				time.Sleep(500 * time.Millisecond)
				return nil
			})

			if err != nil {
				fmt.Printf("Request %d rejected: %v\n", id, err)
			} else {
				fmt.Printf("Request %d completed\n", id)
			}
		}(i + 1)
	}

	// Attendi completamento
	time.Sleep(5 * time.Second)

	// Mostra statistiche
	stats := bulkhead.GetStats()
	fmt.Printf("\nBulkhead Stats:\n")
	fmt.Printf("- Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("- Total Completed: %d\n", stats.TotalCompleted)
	fmt.Printf("- Total Rejected: %d\n", stats.TotalRejected)
	fmt.Printf("- Total Timed Out: %d\n", stats.TotalTimedOut)
}

// Example_retry dimostra l'uso del retry
func Example_retry() {
	// Configura retry
	config := resilience.RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		JitterFraction:    0.1,
		OnRetry: func(attempt int, err error, backoff time.Duration) {
			fmt.Printf("Retry attempt %d after %v: %v\n", attempt, backoff, err)
		},
	}

	retry := resilience.NewRetry(config)

	provider := &FlakyProvider{}
	ctx := context.Background()

	// Esegui con retry
	err := retry.Execute(ctx, func() error {
		return provider.Call()
	})

	if err != nil {
		fmt.Printf("Failed after retries: %v\n", err)
	} else {
		fmt.Printf("Succeeded!\n")
	}
}

// Example_fallback dimostra l'uso del fallback
func Example_fallback() {
	// Configura fallback
	config := resilience.FallbackConfig{
		Strategies: []resilience.FallbackStrategy{
			resilience.FallbackToCache,
			resilience.FallbackToStale,
			resilience.FallbackToDegraded,
		},
		EnableCache:      true,
		CacheTTL:         5 * time.Minute,
		EnableStale:      true,
		StaleTTL:         30 * time.Minute,
		DegradedResponse: "Service temporarily unavailable",
		OnFallback: func(strategy resilience.FallbackStrategy, reason error) {
			fmt.Printf("Using fallback: %s (reason: %v)\n", strategy, reason)
		},
	}

	fallback := resilience.NewFallback(config)

	ctx := context.Background()

	// Prima richiesta - successo (popola cache)
	result1, err1 := fallback.Execute(ctx, "test-key", func() (interface{}, error) {
		return "Fresh response", nil
	})
	fmt.Printf("Request 1: %v (err: %v)\n", result1, err1)

	// Seconda richiesta - failure (usa cache)
	result2, err2 := fallback.Execute(ctx, "test-key", func() (interface{}, error) {
		return nil, errors.New("service down")
	})
	fmt.Printf("Request 2: %v (err: %v)\n", result2, err2)

	// Mostra statistiche
	stats := fallback.GetStats()
	fmt.Printf("\nFallback Stats:\n")
	fmt.Printf("- Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("- Total Fallbacks: %d\n", stats.TotalFallbacks)
	fmt.Printf("- Cache Entries: %d\n", stats.CacheEntries)
}

// Example_fullResilience dimostra l'uso completo del sistema
func Example_fullResilience() {
	// Configura resilience completa
	config := resilience.DefaultConfig()
	config.CircuitBreaker.FailureThreshold = 3
	config.Bulkhead.MaxConcurrent = 5
	config.Retry.MaxRetries = 2

	manager := resilience.NewManager(config)
	defer manager.Close()

	provider := &FlakyProvider{}

	// Esegui richieste
	for i := 0; i < 5; i++ {
		ctx := context.Background()

		err := manager.Execute(ctx, "openai", func() error {
			return provider.Call()
		})

		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("Request %d succeeded\n", i+1)
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Verifica disponibilità
	available := manager.IsProviderAvailable("openai")
	fmt.Printf("\nProvider available: %v\n", available)

	// Mostra statistiche complete
	stats := manager.GetStats()
	for provider, providerStats := range stats.Providers {
		fmt.Printf("\nProvider: %s\n", provider)
		fmt.Printf("%s\n", providerStats.String())
	}

	// Health check
	health := manager.HealthCheck()
	fmt.Printf("\nSystem healthy: %v\n", health.Healthy)
	for provider, providerHealth := range health.Providers {
		fmt.Printf("- %s: available=%v, issues=%v\n",
			provider, providerHealth.Available, providerHealth.Issues)
	}
}

// Example_perProviderResilience dimostra resilience per-provider
func Example_perProviderResilience() {
	config := resilience.DefaultConfig()
	manager := resilience.NewManager(config)
	defer manager.Close()

	providers := []string{"openai", "anthropic", "google"}

	// Esegui richieste su provider diversi
	for _, provider := range providers {
		for i := 0; i < 3; i++ {
			ctx := context.Background()

			err := manager.Execute(ctx, provider, func() error {
				// Simula comportamento diverso per provider
				if provider == "anthropic" && i == 1 {
					return errors.New("rate limit exceeded")
				}
				return nil
			})

			if err != nil {
				fmt.Printf("%s request %d failed: %v\n", provider, i+1, err)
			} else {
				fmt.Printf("%s request %d succeeded\n", provider, i+1)
			}
		}
	}

	// Mostra statistiche per provider
	stats := manager.GetStats()
	fmt.Printf("\nPer-Provider Statistics:\n")
	for provider, providerStats := range stats.Providers {
		fmt.Printf("\n%s:\n", provider)
		fmt.Printf("  Circuit: %s\n", providerStats.CircuitBreaker.State)
		fmt.Printf("  Active: %d/%d\n",
			providerStats.Bulkhead.ActiveRequests,
			providerStats.Bulkhead.MaxConcurrent)
		fmt.Printf("  Retries: %d\n", providerStats.Retry.TotalRetries)
	}
}

// Example_cascadingFailures dimostra protezione da cascading failures
func Example_cascadingFailures() {
	config := resilience.DefaultConfig()
	config.CircuitBreaker.FailureThreshold = 2
	config.CircuitBreaker.Timeout = 3 * time.Second

	manager := resilience.NewManager(config)
	defer manager.Close()

	ctx := context.Background()

	// Simula failures che aprono il circuit breaker
	fmt.Println("Simulating failures...")
	for i := 0; i < 5; i++ {
		err := manager.Execute(ctx, "unstable-provider", func() error {
			return errors.New("service unavailable")
		})

		if err != nil {
			if errors.Is(err, resilience.ErrCircuitOpen) {
				fmt.Printf("Request %d: Circuit breaker OPEN - fast fail\n", i+1)
			} else {
				fmt.Printf("Request %d: Failed - %v\n", i+1, err)
			}
		}
	}

	// Verifica stato
	stats := manager.GetStats()
	providerStats := stats.Providers["unstable-provider"]
	fmt.Printf("\nCircuit state: %s\n", providerStats.CircuitBreaker.State)
	fmt.Printf("Rejected requests: %d\n", providerStats.CircuitBreaker.TotalRejected)

	// Attendi timeout e riprova (half-open)
	fmt.Println("\nWaiting for circuit breaker timeout...")
	time.Sleep(4 * time.Second)

	// Prova con successo
	err := manager.Execute(ctx, "unstable-provider", func() error {
		return nil // Ora funziona
	})

	if err == nil {
		fmt.Println("Request succeeded - circuit recovering")
	}

	// Verifica nuovo stato
	stats = manager.GetStats()
	providerStats = stats.Providers["unstable-provider"]
	fmt.Printf("Circuit state: %s\n", providerStats.CircuitBreaker.State)
}
