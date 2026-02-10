# Quota Management and Rate Limiting System

Sistema completo per la gestione delle quote e rate limiting distribuito con Redis.

## Componenti

### 1. Manager (`manager.go`)
Gestisce le quote degli account con tracking e notifiche.

**Funzionalità:**
- Track usage per account/provider
- Check quota availability prima delle richieste
- Auto-reset giornaliero
- Notifiche quando quota raggiunge 80%
- Notifiche quando quota è esaurita

**Esempio:**
```go
manager := quota.NewManager(db, redisClient)

// Verifica disponibilità
status, err := manager.CheckAvailability(ctx, accountID, tokensNeeded)
if !status.Available {
    log.Warn().Str("reason", status.Reason).Msg("Quota not available")
    return
}

// Consuma quota
err = manager.ConsumeQuota(ctx, accountID, tokensUsed)
```

### 2. RateLimiter (`rate_limiter.go`)
Implementa rate limiting distribuito con Redis.

**Algoritmi:**
- **Sliding Window**: Per RPM, RPH, RPD limits
- **Token Bucket**: Per TPM, TPD limits
- **Concurrent**: Per limite di richieste parallele

**Tipi di limiti supportati:**
- `rpm`: Requests per minute
- `rph`: Requests per hour
- `rpd`: Requests per day
- `tpm`: Tokens per minute
- `tpd`: Tokens per day
- `concurrent`: Richieste concorrenti

**Esempio:**
```go
rateLimiter := quota.NewRateLimiter(redisClient)

// Verifica rate limits
result, err := rateLimiter.CheckLimit(ctx, providerID, accountID, rateLimits)
if !result.Allowed {
    log.Warn().
        Str("limit_type", string(result.LimitType)).
        Dur("retry_after", result.RetryAfter).
        Msg("Rate limit exceeded")
    return
}

// Registra richiesta per sliding window
rateLimiter.RecordRequest(ctx, providerID, accountID, models.LimitTypeRPM)

// Gestione concorrenza
concurrent, _ := rateLimiter.IncrementConcurrent(ctx, providerID, accountID)
defer rateLimiter.DecrementConcurrent(ctx, providerID, accountID)
```

### 3. Tracker (`tracker.go`)
Traccia l'utilizzo delle API e calcola costi.

**Funzionalità:**
- Count requests con dettagli completi
- Track tokens (input/output)
- Calculate costs
- Store in database (RequestLog, ProviderStats)
- Statistiche aggregate

**Esempio:**
```go
tracker := quota.NewTracker(db)

// Traccia richiesta
req := &quota.TrackingRequest{
    ProviderID:    providerID,
    ModelID:       modelID,
    UserID:        userID,
    Method:        "POST",
    Endpoint:      "/v1/chat/completions",
    StatusCode:    200,
    LatencyMs:     150,
    InputTokens:   500,
    OutputTokens:  500,
    Success:       true,
    EstimatedCost: 0.01,
}
tracker.TrackRequest(ctx, req)

// Ottieni statistiche
stats, _ := tracker.GetUsageStats(ctx, accountID, from, to)
fmt.Printf("Success rate: %.2f%%\n", stats.SuccessRate*100)
fmt.Printf("Avg latency: %.2fms\n", stats.AvgLatencyMs)
```

### 4. PoolManager (`pool.go`)
Gestisce pool di account multipli per provider.

**Strategie di selezione:**
- `round_robin`: Rotazione circolare
- `least_used`: Minimo utilizzo
- `random`: Selezione casuale

**Funzionalità:**
- Auto-switch su quota exhausted
- Health check degli account
- Rebalancing del carico
- Selezione intelligente basata su metriche

**Esempio:**
```go
poolManager := quota.NewPoolManager(db, redisClient, manager, rateLimiter)

// Imposta strategia
poolManager.SetStrategy(quota.StrategyLeastUsed)

// Ottieni account disponibile
account, err := poolManager.GetAccount(ctx, providerID, tokensNeeded)
if err != nil {
    log.Error().Err(err).Msg("No account available")
    return
}

// Ottieni il miglior account
bestAccount, _ := poolManager.GetBestAccount(ctx, providerID, tokensNeeded)

// Status del pool
poolStatus, _ := poolManager.GetPoolStatus(ctx, providerID)
fmt.Printf("Available: %d/%d\n",
    poolStatus.AvailableAccounts,
    poolStatus.TotalAccounts)
```

## Setup

### 1. Inizializzazione

```go
import (
    "github.com/biodoia/goleapifree/internal/quota"
    "github.com/biodoia/goleapifree/pkg/cache"
)

// Crea Redis client
redisClient, err := cache.NewRedisClient("localhost:6379", "", 0)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to connect to Redis")
}

// Crea sistema completo
qs, err := quota.NewQuotaSystem(db, "localhost:6379", "", 0)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to create quota system")
}
```

### 2. Configurazione Callbacks

```go
qs.Manager.SetWarningCallback(func(account *models.Account, usagePercent float64) {
    // Invia email/webhook quando quota al 80%
    sendWarningNotification(account, usagePercent)
})

qs.Manager.SetExhaustedCallback(func(account *models.Account) {
    // Invia alert quando quota esaurita
    sendExhaustedAlert(account)

    // Disabilita account temporaneamente
    disableAccount(account.ID)
})
```

## Workflow Completo

```go
func handleAPIRequest(ctx context.Context, qs *quota.QuotaSystem,
                      providerID uuid.UUID, tokensNeeded int64) error {

    // 1. Ottieni account dal pool
    account, err := qs.PoolManager.GetAccount(ctx, providerID, tokensNeeded)
    if err != nil {
        return fmt.Errorf("no account available: %w", err)
    }

    // 2. Verifica quota
    quotaStatus, err := qs.Manager.CheckAvailability(ctx, account.ID, tokensNeeded)
    if err != nil || !quotaStatus.Available {
        return fmt.Errorf("quota not available: %s", quotaStatus.Reason)
    }

    // 3. Verifica rate limits
    var rateLimits []models.RateLimit
    db.Where("provider_id = ?", account.ProviderID).Find(&rateLimits)

    rateLimitResult, err := qs.RateLimiter.CheckLimit(
        ctx, account.ProviderID, account.ID, rateLimits)
    if err != nil || !rateLimitResult.Allowed {
        return fmt.Errorf("rate limit exceeded, retry after: %v",
            rateLimitResult.RetryAfter)
    }

    // 4. Gestione concorrenza
    concurrent, _ := qs.RateLimiter.IncrementConcurrent(
        ctx, account.ProviderID, account.ID)
    defer qs.RateLimiter.DecrementConcurrent(
        ctx, account.ProviderID, account.ID)

    // 5. Esegui richiesta API
    startTime := time.Now()
    response, err := executeAPIRequest(account, request)
    latencyMs := int(time.Since(startTime).Milliseconds())

    // 6. Consuma quota
    tokensUsed := response.InputTokens + response.OutputTokens
    qs.Manager.ConsumeQuota(ctx, account.ID, int64(tokensUsed))

    // 7. Registra per rate limiting
    for _, limit := range rateLimits {
        if isRequestLimit(limit.LimitType) {
            qs.RateLimiter.RecordRequest(
                ctx, account.ProviderID, account.ID, limit.LimitType)
        }
    }

    // 8. Traccia richiesta
    trackingReq := &quota.TrackingRequest{
        ProviderID:    account.ProviderID,
        ModelID:       modelID,
        UserID:        account.UserID,
        Method:        "POST",
        Endpoint:      "/v1/chat/completions",
        StatusCode:    response.StatusCode,
        LatencyMs:     latencyMs,
        InputTokens:   response.InputTokens,
        OutputTokens:  response.OutputTokens,
        Success:       err == nil,
        ErrorMessage:  getErrorMessage(err),
        EstimatedCost: calculateCost(response),
    }
    qs.Tracker.TrackRequest(ctx, trackingReq)

    return err
}
```

## Redis Keys Structure

```
# Quota tracking
quota:{account_id} -> current usage (int64)

# Rate limiting - Sliding Window
slidingwindow:{provider_id}:{account_id}:{limit_type} -> sorted set of timestamps

# Rate limiting - Token Bucket
tokenbucket:{provider_id}:{account_id}:{limit_type} -> remaining tokens (int64)
tokenbucket:{provider_id}:{account_id}:{limit_type}:last_refill -> timestamp (int64)

# Concurrent requests
ratelimit:concurrent:{provider_id}:{account_id} -> count (int64)
```

## Database Schema

Utilizza i seguenti modelli:
- `accounts`: Account con quota tracking
- `rate_limits`: Configurazione rate limits per provider
- `request_logs`: Log dettagliato delle richieste
- `provider_stats`: Statistiche aggregate giornaliere

## Monitoring

### Metriche disponibili

```go
// Usage statistics
stats, _ := qs.Tracker.GetUsageStats(ctx, accountID, from, to)
fmt.Printf("Total requests: %d\n", stats.TotalRequests)
fmt.Printf("Success rate: %.2f%%\n", stats.SuccessRate*100)
fmt.Printf("Avg latency: %.2fms\n", stats.AvgLatencyMs)

// Provider statistics
providerStats, _ := qs.Tracker.GetProviderStats(ctx, providerID, from, to)
fmt.Printf("Unique users: %d\n", providerStats.UniqueUsers)
fmt.Printf("Total tokens: %d\n", providerStats.TotalTokens)

// Pool status
poolStatus, _ := qs.PoolManager.GetPoolStatus(ctx, providerID)
fmt.Printf("Available accounts: %d/%d\n",
    poolStatus.AvailableAccounts, poolStatus.TotalAccounts)

// Rate limit stats
rateLimitStats, _ := qs.RateLimiter.GetStats(
    ctx, providerID, accountID, models.LimitTypeRPM)
fmt.Printf("Current RPM: %d\n", rateLimitStats.CurrentCount)

// Top models
topModels, _ := qs.Tracker.GetTopModels(ctx, 10, from, to)
for _, model := range topModels {
    fmt.Printf("Model %s: %d requests\n",
        model.ModelID, model.RequestCount)
}

// Error statistics
errorStats, _ := qs.Tracker.GetErrorStats(ctx, providerID, from, to)
fmt.Printf("Total errors: %d\n", errorStats.TotalErrors)
for errMsg, count := range errorStats.ErrorsByType {
    fmt.Printf("  %s: %d\n", errMsg, count)
}
```

## Performance

- **Redis caching**: Operazioni di quota in <1ms
- **Sliding window**: O(log N) dove N = numero di richieste nella finestra
- **Token bucket**: O(1) operations
- **Database batch updates**: Statistiche aggregate in background
- **Connection pooling**: Configurabile per ottimizzare throughput

## Best Practices

1. **Usa pool strategy appropriata**:
   - `least_used` per bilanciamento uniforme
   - `round_robin` per fairness
   - `random` per distribuire carico

2. **Configura rate limits appropriati**:
   - RPM per burst protection
   - RPH/RPD per limiti giornalieri
   - Concurrent per proteggere risorse

3. **Monitor quota usage**:
   - Configura alert al 80%
   - Auto-scaling degli account
   - Rebalancing periodico

4. **Gestisci gracefully i limiti**:
   - Usa `RetryAfter` per backoff
   - Fallback su altri provider
   - Queue requests se necessario

5. **Ottimizza performance**:
   - Cache quota status in Redis
   - Batch database updates
   - Usa pipeline Redis per operazioni multiple

## Testing

Vedere `example_integration.go` per esempi completi di utilizzo.
