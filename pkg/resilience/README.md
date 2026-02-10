# Resilience Package

Sistema enterprise-grade di resilience per proteggere il gateway da failures, cascading failures e sovraccarico.

## Componenti

### 1. Circuit Breaker

Previene chiamate a servizi non disponibili, aprendo il circuito dopo un numero configurabile di failures.

**Stati:**
- **Closed**: Circuito chiuso, richieste passano normalmente
- **Open**: Circuito aperto, richieste vengono rifiutate immediatamente
- **Half-Open**: Circuito in test, permette un numero limitato di richieste

**Features:**
- Failure threshold configurabile
- Auto-recovery con timeout
- Half-open state per gradual recovery
- Per-provider circuit breakers
- Statistiche dettagliate

**Esempio:**
```go
config := resilience.CircuitBreakerConfig{
    FailureThreshold:    5,      // Apri dopo 5 failures
    SuccessThreshold:    2,      // Chiudi dopo 2 successi in half-open
    Timeout:            60 * time.Second,
    HalfOpenMaxRequests: 3,
}

cb := resilience.NewCircuitBreaker(config)

err := cb.Execute(ctx, func() error {
    return provider.Call()
})
```

### 2. Bulkhead

Isola le risorse per provider, limitando il numero di richieste concorrenti.

**Features:**
- Limit concorrenza per provider
- Coda di attesa configurabile
- Timeout per richieste in coda
- Prevenzione cascading failures
- Isolamento risorse

**Esempio:**
```go
config := resilience.BulkheadConfig{
    MaxConcurrent: 10,           // Max 10 richieste concorrenti
    MaxQueue:      20,           // Max 20 richieste in coda
    QueueTimeout:  5 * time.Second,
}

bulkhead := resilience.NewBulkhead(config)

err := bulkhead.Execute(ctx, func() error {
    return provider.Call()
})
```

### 3. Retry

Riprova automaticamente richieste fallite con exponential backoff e jitter.

**Features:**
- Exponential backoff
- Jitter per evitare thundering herd
- Max retries configurabile
- Solo errori retryable
- Backoff configurabile

**Esempio:**
```go
config := resilience.RetryConfig{
    MaxRetries:         3,
    InitialBackoff:     100 * time.Millisecond,
    MaxBackoff:         10 * time.Second,
    BackoffMultiplier:  2.0,
    Jitter:             true,
    JitterFraction:     0.1,
}

retry := resilience.NewRetry(config)

err := retry.Execute(ctx, func() error {
    return provider.Call()
})
```

### 4. Fallback

Fornisce risposte alternative quando il servizio primario fallisce.

**Strategie:**
- **FallbackToCache**: Usa risposta dalla cache
- **FallbackToStale**: Usa dati stale dalla cache
- **FallbackToProvider**: Prova provider alternativo
- **FallbackToDegraded**: Restituisce risposta degradata
- **FallbackToError**: Restituisce errore strutturato

**Features:**
- Multiple strategie in sequenza
- Cache con TTL configurabile
- Stale data support
- Automatic cleanup
- Per-provider fallback

**Esempio:**
```go
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
}

fallback := resilience.NewFallback(config)

result, err := fallback.Execute(ctx, "cache-key", func() (interface{}, error) {
    return provider.Call()
})
```

## Manager Integrato

Il `Manager` combina tutti i pattern in un sistema unificato.

**Ordine di applicazione:**
1. Circuit Breaker (più esterno) - Previene chiamate a servizi down
2. Bulkhead - Limita concorrenza
3. Retry (più interno) - Riprova failures transitori
4. Fallback - Fornisce risposte alternative

**Esempio base:**
```go
config := resilience.DefaultConfig()
manager := resilience.NewManager(config)
defer manager.Close()

// Esegui richiesta con tutti i pattern
err := manager.Execute(ctx, "openai", func() error {
    return provider.Call()
})
```

**Esempio con fallback:**
```go
result, err := manager.ExecuteWithFallback(ctx, "openai", "cache-key", func() (interface{}, error) {
    return provider.ChatCompletion(req)
})
```

## Configurazione

### Default Config
```go
config := resilience.DefaultConfig()
// Modifica solo ciò che serve
config.CircuitBreaker.FailureThreshold = 10
config.Bulkhead.MaxConcurrent = 20
config.Retry.MaxRetries = 5
```

### Custom Config
```go
config := resilience.Config{
    CircuitBreaker: resilience.CircuitBreakerConfig{
        FailureThreshold:    5,
        SuccessThreshold:    2,
        Timeout:            60 * time.Second,
        HalfOpenMaxRequests: 3,
    },
    Bulkhead: resilience.BulkheadConfig{
        MaxConcurrent: 10,
        MaxQueue:      20,
        QueueTimeout:  5 * time.Second,
    },
    Retry: resilience.RetryConfig{
        MaxRetries:         3,
        InitialBackoff:     100 * time.Millisecond,
        MaxBackoff:         10 * time.Second,
        BackoffMultiplier:  2.0,
        Jitter:             true,
    },
    Fallback: resilience.FallbackConfig{
        Strategies: []resilience.FallbackStrategy{
            resilience.FallbackToCache,
            resilience.FallbackToProvider,
        },
        EnableCache: true,
        CacheTTL:    5 * time.Minute,
    },
    EnableCircuitBreaker: true,
    EnableBulkhead:       true,
    EnableRetry:          true,
    EnableFallback:       true,
}
```

## Monitoring

### Statistiche

```go
// Statistiche complete
stats := manager.GetStats()

for provider, providerStats := range stats.Providers {
    fmt.Printf("Provider: %s\n", provider)
    fmt.Printf("  Circuit: %s\n", providerStats.CircuitBreaker.State)
    fmt.Printf("  Active Requests: %d/%d\n",
        providerStats.Bulkhead.ActiveRequests,
        providerStats.Bulkhead.MaxConcurrent)
    fmt.Printf("  Total Retries: %d\n", providerStats.Retry.TotalRetries)
    fmt.Printf("  Total Fallbacks: %d\n", providerStats.Fallback.TotalFallbacks)
}
```

### Health Check

```go
health := manager.HealthCheck()

if !health.Healthy {
    for provider, providerHealth := range health.Providers {
        if !providerHealth.Available {
            fmt.Printf("Provider %s unavailable: %v\n",
                provider, providerHealth.Issues)
        }
    }
}
```

### Reset

```go
// Reset specifico provider
manager.ResetProvider("openai")

// Reset tutto
manager.ResetAll()
```

## Best Practices

### 1. Tuning Circuit Breaker

- **FailureThreshold**: 3-10 per API esterne, 2-5 per servizi interni
- **Timeout**: 30-60s per recovery rapido, 5-10min per problemi prolungati
- **SuccessThreshold**: 2-3 per gradual recovery

### 2. Tuning Bulkhead

- **MaxConcurrent**: Basato su rate limits del provider
- **MaxQueue**: 1-2x MaxConcurrent
- **QueueTimeout**: User experience target (2-5s)

### 3. Tuning Retry

- **MaxRetries**: 2-3 per operazioni critiche, 1-2 per operazioni normali
- **InitialBackoff**: 100-500ms
- **MaxBackoff**: 5-30s
- **Jitter**: Sempre abilitato per evitare thundering herd

### 4. Tuning Fallback

- **CacheTTL**: Basato su quanto sono accettabili dati "vecchi"
- **StaleTTL**: 5-10x CacheTTL
- **Strategies**: Ordine dal più preferibile al meno preferibile

### 5. Per-Provider Configuration

```go
// Configurazioni diverse per provider diversi
openAIConfig := resilience.DefaultConfig()
openAIConfig.Bulkhead.MaxConcurrent = 50  // OpenAI ha rate limits alti

anthropicConfig := resilience.DefaultConfig()
anthropicConfig.Bulkhead.MaxConcurrent = 10  // Anthropic ha rate limits bassi

// Usa manager separati o configura dinamicamente
```

## Integrazione con Gateway

```go
// In gateway.go
type Gateway struct {
    resilience *resilience.Manager
    // ...
}

func (g *Gateway) handleChatCompletion(c fiber.Ctx) error {
    // Parse request
    var req ChatRequest
    if err := c.BodyParser(&req); err != nil {
        return err
    }

    // Determina provider
    provider := g.selectProvider(req.Model)

    // Esegui con resilience
    result, err := g.resilience.ExecuteWithFallback(
        c.Context(),
        provider.Name(),
        req.CacheKey(),
        func() (interface{}, error) {
            return provider.ChatCompletion(c.Context(), &req)
        },
    )

    if err != nil {
        return g.handleError(c, err)
    }

    return c.JSON(result)
}
```

## Testing

Vedi `example_test.go` per esempi completi:

```bash
# Esegui esempi
go test -v -run Example

# Esegui test
go test -v ./pkg/resilience/...

# Benchmark
go test -bench=. -benchmem ./pkg/resilience/...
```

## Performance

- **Circuit Breaker**: Overhead < 1μs per richiesta (chiuso)
- **Bulkhead**: Overhead < 10μs per richiesta
- **Retry**: Overhead dipende da backoff
- **Fallback**: Overhead < 5μs (cache hit), < 100μs (cache miss)

## Thread Safety

Tutti i componenti sono thread-safe e possono essere usati concorrentemente.

## Metriche Prometheus

```go
// TODO: Integrare con pkg/stats/prometheus.go
// - circuit_breaker_state{provider}
// - bulkhead_active_requests{provider}
// - retry_total{provider}
// - fallback_total{provider, strategy}
```
