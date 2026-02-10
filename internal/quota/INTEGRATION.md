# Guida all'Integrazione - Quota Management System

## Setup Iniziale

### 1. Configurazione Redis

Assicurati che Redis sia in esecuzione:

```bash
# Docker
docker run -d -p 6379:6379 redis:alpine

# O installa localmente
brew install redis  # macOS
sudo apt install redis-server  # Linux
```

Aggiungi configurazione Redis al tuo `config.yaml`:

```yaml
redis:
  host: "localhost:6379"
  password: ""
  db: 0
```

### 2. Inizializzazione nel Backend

In `/cmd/backend/main.go`:

```go
package main

import (
    "github.com/biodoia/goleapifree/internal/quota"
    "github.com/biodoia/goleapifree/pkg/cache"
    "github.com/biodoia/goleapifree/pkg/config"
    "github.com/biodoia/goleapifree/pkg/database"
)

func main() {
    // Carica configurazione
    cfg, err := config.Load("")
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to load config")
    }

    // Inizializza database
    db, err := database.New(&cfg.Database)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect to database")
    }
    defer db.Close()

    // Inizializza sistema di quota
    quotaSystem, err := quota.NewQuotaSystem(
        db.DB,
        cfg.Redis.Host,
        cfg.Redis.Password,
        cfg.Redis.DB,
    )
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to initialize quota system")
    }

    // Configura callbacks per notifiche
    setupQuotaCallbacks(quotaSystem)

    // Usa quota system nella tua applicazione
    // ...
}

func setupQuotaCallbacks(qs *quota.QuotaSystem) {
    // Warning al 80%
    qs.Manager.SetWarningCallback(func(account *models.Account, usagePercent float64) {
        log.Warn().
            Str("account_id", account.ID.String()).
            Float64("usage_percent", usagePercent*100).
            Msg("Quota warning threshold reached")

        // Invia email/webhook
        // notifyAdmin(account, usagePercent)
    })

    // Quota esaurita
    qs.Manager.SetExhaustedCallback(func(account *models.Account) {
        log.Error().
            Str("account_id", account.ID.String()).
            Msg("Quota exhausted")

        // Invia alert critico
        // sendCriticalAlert(account)

        // Opzionalmente disabilita account
        // disableAccount(account.ID)
    })
}
```

## Integrazione con Gateway API

### Handler Middleware

Crea un middleware per gestire quota e rate limiting:

```go
package middleware

import (
    "github.com/biodoia/goleapifree/internal/quota"
    "github.com/biodoia/goleapifree/pkg/models"
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
)

func QuotaMiddleware(qs *quota.QuotaSystem) fiber.Handler {
    return func(c fiber.Ctx) error {
        // Estrai provider e user da context
        providerID := c.Locals("provider_id").(uuid.UUID)
        userID := c.Locals("user_id").(uuid.UUID)

        // Stima token necessari (puoi calcolare meglio in base al request)
        tokensNeeded := int64(1000) // Default estimate

        // 1. Ottieni account dal pool
        account, err := qs.PoolManager.GetAccount(c.Context(), providerID, tokensNeeded)
        if err != nil {
            return c.Status(503).JSON(fiber.Map{
                "error": "no_account_available",
                "message": "Service temporarily unavailable",
            })
        }

        // 2. Verifica quota
        quotaStatus, err := qs.Manager.CheckAvailability(
            c.Context(), account.ID, tokensNeeded)
        if err != nil || !quotaStatus.Available {
            return c.Status(429).JSON(fiber.Map{
                "error": "quota_exceeded",
                "message": "Quota limit reached",
                "reset_at": quotaStatus.ResetAt,
            })
        }

        // 3. Verifica rate limits
        var rateLimits []models.RateLimit
        // Carica da DB o cache
        // db.Where("provider_id = ?", providerID).Find(&rateLimits)

        rateLimitResult, err := qs.RateLimiter.CheckLimit(
            c.Context(), account.ProviderID, account.ID, rateLimits)
        if err != nil || !rateLimitResult.Allowed {
            return c.Status(429).JSON(fiber.Map{
                "error": "rate_limit_exceeded",
                "message": "Rate limit exceeded",
                "retry_after": rateLimitResult.RetryAfter.Seconds(),
            })
        }

        // 4. Incrementa concurrent counter
        concurrent, _ := qs.RateLimiter.IncrementConcurrent(
            c.Context(), account.ProviderID, account.ID)

        // Store in locals per uso successivo
        c.Locals("account", account)
        c.Locals("quota_system", qs)

        // Decrementa al termine
        defer func() {
            qs.RateLimiter.DecrementConcurrent(
                c.Context(), account.ProviderID, account.ID)
        }()

        return c.Next()
    }
}
```

### Handler API Request

```go
func HandleChatCompletion(c fiber.Ctx) error {
    // Recupera da middleware
    account := c.Locals("account").(*models.Account)
    qs := c.Locals("quota_system").(*quota.QuotaSystem)

    // Parse request
    var req ChatCompletionRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid_request"})
    }

    // Esegui richiesta API
    startTime := time.Now()
    response, err := executeAPIRequest(account, &req)
    latencyMs := int(time.Since(startTime).Milliseconds())

    if err != nil {
        // Track errore
        trackError(qs, account, err, latencyMs)
        return c.Status(500).JSON(fiber.Map{"error": "api_error"})
    }

    // Consuma quota
    tokensUsed := int64(response.Usage.TotalTokens)
    if err := qs.Manager.ConsumeQuota(c.Context(), account.ID, tokensUsed); err != nil {
        log.Error().Err(err).Msg("Failed to consume quota")
    }

    // Registra per rate limiting
    qs.RateLimiter.RecordRequest(
        c.Context(), account.ProviderID, account.ID, models.LimitTypeRPM)

    // Track richiesta
    trackingReq := &quota.TrackingRequest{
        ProviderID:    account.ProviderID,
        ModelID:       req.ModelID,
        UserID:        account.UserID,
        Method:        c.Method(),
        Endpoint:      c.Path(),
        StatusCode:    200,
        LatencyMs:     latencyMs,
        InputTokens:   response.Usage.PromptTokens,
        OutputTokens:  response.Usage.CompletionTokens,
        Success:       true,
        EstimatedCost: calculateCost(response),
    }
    qs.Tracker.TrackRequest(c.Context(), trackingReq)

    return c.JSON(response)
}

func trackError(qs *quota.QuotaSystem, account *models.Account, err error, latencyMs int) {
    req := &quota.TrackingRequest{
        ProviderID:   account.ProviderID,
        UserID:       account.UserID,
        Method:       "POST",
        StatusCode:   500,
        LatencyMs:    latencyMs,
        Success:      false,
        ErrorMessage: err.Error(),
    }
    qs.Tracker.TrackRequest(context.Background(), req)
}
```

## API Endpoints per Monitoring

### Stats Endpoint

```go
// GET /api/v1/stats/usage?from=2024-01-01&to=2024-01-31
func GetUsageStats(c fiber.Ctx, qs *quota.QuotaSystem) error {
    userID := c.Locals("user_id").(uuid.UUID)

    from, _ := time.Parse(time.RFC3339, c.Query("from"))
    to, _ := time.Parse(time.RFC3339, c.Query("to"))

    stats, err := qs.Tracker.GetUsageStats(c.Context(), userID, from, to)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(stats)
}

// GET /api/v1/stats/pool/:provider_id
func GetPoolStatus(c fiber.Ctx, qs *quota.QuotaSystem) error {
    providerID, _ := uuid.Parse(c.Params("provider_id"))

    status, err := qs.PoolManager.GetPoolStatus(c.Context(), providerID)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(status)
}

// GET /api/v1/stats/quota/:account_id
func GetQuotaStatus(c fiber.Ctx, qs *quota.QuotaSystem) error {
    accountID, _ := uuid.Parse(c.Params("account_id"))

    status, err := qs.Manager.GetStatus(c.Context(), accountID)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(status)
}
```

## Gestione Account Pool

### Configurazione Rate Limits

```go
func setupRateLimits(db *gorm.DB, providerID uuid.UUID) {
    limits := []models.RateLimit{
        {
            ProviderID:  providerID,
            LimitType:   models.LimitTypeRPM,
            LimitValue:  60, // 60 requests per minute
        },
        {
            ProviderID:  providerID,
            LimitType:   models.LimitTypeRPH,
            LimitValue:  3000, // 3000 requests per hour
        },
        {
            ProviderID:  providerID,
            LimitType:   models.LimitTypeRPD,
            LimitValue:  50000, // 50k requests per day
        },
        {
            ProviderID:  providerID,
            LimitType:   models.LimitTypeConcurrent,
            LimitValue:  10, // Max 10 concurrent requests
        },
    }

    for _, limit := range limits {
        db.Create(&limit)
    }
}
```

### Aggiunta Account al Pool

```go
func addAccountToPool(db *gorm.DB, providerID, userID uuid.UUID) error {
    account := models.Account{
        ProviderID: providerID,
        UserID:     userID,
        Credentials: datatypes.JSON(`{
            "api_key": "encrypted_key_here"
        }`),
        Active:     true,
        QuotaLimit: 1000000, // 1M tokens per day
        QuotaUsed:  0,
        LastReset:  time.Now(),
        ExpiresAt:  time.Now().AddDate(0, 1, 0), // 1 mese
    }

    return db.Create(&account).Error
}
```

## Background Jobs

### Periodic Quota Reset

Il manager già include un reset automatico, ma puoi aggiungere un cron job:

```go
import "github.com/robfig/cron/v3"

func setupCronJobs(qs *quota.QuotaSystem, db *gorm.DB) {
    c := cron.New()

    // Reset giornaliero alle 00:00 UTC
    c.AddFunc("0 0 * * *", func() {
        log.Info().Msg("Running daily quota reset")
        // Il manager lo fa automaticamente, ma puoi forzarlo qui
    })

    // Cleanup vecchi logs (keep 90 giorni)
    c.AddFunc("0 2 * * *", func() {
        cutoff := time.Now().AddDate(0, 0, -90)
        db.Where("timestamp < ?", cutoff).Delete(&models.RequestLog{})
        log.Info().Msg("Old request logs cleaned up")
    })

    // Aggregate stats giornaliere
    c.AddFunc("0 1 * * *", func() {
        aggregateDailyStats(qs, db)
    })

    c.Start()
}
```

## Prometheus Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    quotaUsage = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "quota_usage_percent",
            Help: "Current quota usage percentage",
        },
        []string{"account_id", "provider_id"},
    )

    rateLimitHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rate_limit_hits_total",
            Help: "Total number of rate limit hits",
        },
        []string{"provider_id", "limit_type"},
    )
)

func init() {
    prometheus.MustRegister(quotaUsage)
    prometheus.MustRegister(rateLimitHits)
}

func updateMetrics(qs *quota.QuotaSystem) {
    // Update periodicamente
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        // Update quota metrics
        var accounts []models.Account
        db.Find(&accounts)

        for _, acc := range accounts {
            status, _ := qs.Manager.GetStatus(context.Background(), acc.ID)
            quotaUsage.WithLabelValues(
                acc.ID.String(),
                acc.ProviderID.String(),
            ).Set(status.UsagePercent)
        }
    }
}
```

## Testing

### Test di Integrazione

```go
func TestQuotaIntegration(t *testing.T) {
    // Setup test environment
    db := setupTestDB(t)
    redisClient, _ := cache.NewRedisClient("localhost:6379", "", 1)

    qs, err := quota.NewQuotaSystem(db, "localhost:6379", "", 1)
    require.NoError(t, err)

    // Crea provider e account
    provider := createTestProvider(t, db)
    account := createTestAccount(t, db, provider.ID)

    // Test quota flow
    tokensNeeded := int64(100)

    // Check availability
    status, err := qs.Manager.CheckAvailability(
        context.Background(), account.ID, tokensNeeded)
    require.NoError(t, err)
    assert.True(t, status.Available)

    // Consume quota
    err = qs.Manager.ConsumeQuota(
        context.Background(), account.ID, tokensNeeded)
    require.NoError(t, err)

    // Verify consumption
    var updated models.Account
    db.First(&updated, account.ID)
    assert.Equal(t, tokensNeeded, updated.QuotaUsed)
}
```

## Troubleshooting

### Redis Connection Issues

```go
// Test Redis connection
func testRedisConnection(cfg *config.Config) error {
    client, err := cache.NewRedisClient(
        cfg.Redis.Host,
        cfg.Redis.Password,
        cfg.Redis.DB,
    )
    if err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    _, err = client.Get(ctx, "test_key")
    return err
}
```

### Debug Logging

```go
// Abilita debug logging per quota system
func enableQuotaDebug() {
    zerolog.SetGlobalLevel(zerolog.DebugLevel)

    // Logs includeranno:
    // - Account selection da pool
    // - Quota checks
    // - Rate limit checks
    // - Token consumption
}
```

## Best Practices

1. **Usa connection pooling per Redis**: Già configurato, ma verifica limiti
2. **Cache quota status**: Evita query DB frequenti
3. **Background tracking**: Track async per non bloccare requests
4. **Graceful degradation**: Fallback se Redis non disponibile
5. **Monitor metriche**: Usa Prometheus per alerting
6. **Cleanup periodico**: Rimuovi vecchi logs
7. **Test load**: Verifica performance sotto carico
8. **Security**: Encrypt credentials in DB
9. **Backup Redis**: Persist state importante
10. **Documentation**: Documenta limiti per ogni provider
