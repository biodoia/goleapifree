# GoLeapAI Provider System - Guida all'Uso

## Quick Start

### 1. Setup Semplice

```go
package main

import (
    "context"
    "fmt"

    "github.com/biodoia/goleapifree/internal/provider-manager"
    "github.com/biodoia/goleapifree/internal/providers"
)

func main() {
    // Crea il manager
    pm := manager.NewProviderManager()

    // Registra provider con API keys
    apiKeys := map[string]string{
        "openai": "sk-your-openai-key",
        "groq":   "gsk-your-groq-key",
        "ollama": "", // Nessuna key necessaria per Ollama locale
    }

    pm.LoadDefaultProviders(apiKeys)

    // Usa il provider
    req := &providers.ChatRequest{
        Model: "gpt-3.5-turbo",
        Messages: []providers.Message{
            {Role: "user", Content: "Ciao!"},
        },
    }

    resp, err := pm.ChatCompletion(context.Background(), "openai", req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### 2. Streaming

```go
req := &providers.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []providers.Message{
        {Role: "user", Content: "Scrivi una poesia"},
    },
    Stream: true,
}

err := pm.Stream(context.Background(), "openai", req, func(chunk *providers.StreamChunk) error {
    if chunk.Done {
        fmt.Println("\n[Completato]")
        return nil
    }
    fmt.Print(chunk.Delta)
    return nil
})
```

### 3. Load Balancing Automatico

```go
// Prova tutti i provider disponibili finché uno funziona
resp, err := pm.LoadBalancedRequest(context.Background(), req)
```

### 4. Health Monitoring

```go
// Start background health check worker
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go pm.StartHealthCheckWorker(ctx, 5*time.Minute)

// Get current stats
stats := pm.GetStats()
fmt.Printf("Providers: %d active, %d healthy\n",
    stats.ActiveProviders,
    stats.HealthyProviders)
```

## Integrazione con GoLeapAI Gateway

### Esempio: Gateway Handler

```go
package gateway

import (
    "github.com/biodoia/goleapifree/internal/provider-manager"
    "github.com/biodoia/goleapifree/internal/providers"
    "github.com/gofiber/fiber/v3"
)

type Gateway struct {
    pm *manager.ProviderManager
}

func NewGateway() *Gateway {
    pm := manager.NewProviderManager()

    // Load from config
    apiKeys := loadAPIKeysFromEnv()
    pm.LoadDefaultProviders(apiKeys)

    return &Gateway{pm: pm}
}

// Handler per /v1/chat/completions
func (g *Gateway) ChatCompletionHandler(c fiber.Ctx) error {
    var req providers.ChatRequest
    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }

    // Get provider from header or use first available
    providerName := c.Get("X-Provider", "")

    if req.Stream {
        return g.handleStream(c, providerName, &req)
    }

    resp, err := g.pm.ChatCompletion(c.Context(), providerName, &req)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(resp)
}

func (g *Gateway) handleStream(c fiber.Ctx, provider string, req *providers.ChatRequest) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")

    ctx := c.Context()

    return g.pm.Stream(ctx, provider, req, func(chunk *providers.StreamChunk) error {
        if chunk.Done {
            c.Write([]byte("data: [DONE]\n\n"))
            return nil
        }

        data := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", chunk.Delta)
        c.Write([]byte(data))

        return nil
    })
}

// Handler per provider status
func (g *Gateway) ProviderStatusHandler(c fiber.Ctx) error {
    providers := g.pm.ListProviders()
    return c.JSON(providers)
}

// Handler per health check
func (g *Gateway) HealthHandler(c fiber.Ctx) error {
    results := g.pm.HealthCheckAll(c.Context())

    healthy := 0
    total := len(results)
    for _, err := range results {
        if err == nil {
            healthy++
        }
    }

    return c.JSON(fiber.Map{
        "healthy": healthy,
        "total":   total,
        "status":  results,
    })
}
```

### Setup Routes

```go
func SetupRoutes(app *fiber.App) {
    gateway := NewGateway()

    // OpenAI-compatible endpoints
    v1 := app.Group("/v1")
    v1.Post("/chat/completions", gateway.ChatCompletionHandler)

    // Management endpoints
    admin := app.Group("/admin")
    admin.Get("/providers", gateway.ProviderStatusHandler)
    admin.Get("/health", gateway.HealthHandler)
}
```

## Provider Personalizzati

### Aggiungere un nuovo provider OpenAI-compatible

```go
// Registra un provider custom
pm.RegisterOpenAICompatible(
    "custom-api",
    "https://api.custom.com",
    "custom-api-key",
)

// O con configurazione avanzata
config := manager.ProviderConfig{
    Name:       "custom-api",
    Type:       "openai",
    BaseURL:    "https://api.custom.com",
    APIKey:     "custom-key",
    Timeout:    60 * time.Second,
    MaxRetries: 5,
    Features: map[providers.Feature]bool{
        providers.FeatureStreaming: true,
        providers.FeatureTools:     false,
        providers.FeatureJSONMode:  true,
        providers.FeatureVision:    false,
    },
}

pm.RegisterWithConfig(config)
```

## Database Integration

### Caricamento provider dal database

```go
func LoadProvidersFromDB(db *gorm.DB, pm *manager.ProviderManager) error {
    var dbProviders []models.Provider

    if err := db.Where("status = ?", "active").Find(&dbProviders).Error; err != nil {
        return err
    }

    for _, p := range dbProviders {
        config := manager.ProviderConfig{
            Name:       p.Name,
            Type:       string(p.Type),
            BaseURL:    p.BaseURL,
            APIKey:     getAPIKeyForProvider(p.Name), // From env/vault
            Timeout:    30 * time.Second,
            MaxRetries: 3,
            Features: map[providers.Feature]bool{
                providers.FeatureStreaming: p.SupportsStreaming,
                providers.FeatureTools:     p.SupportsTools,
                providers.FeatureJSONMode:  p.SupportsJSON,
            },
        }

        if err := pm.RegisterWithConfig(config); err != nil {
            log.Warn().Err(err).Str("provider", p.Name).Msg("Failed to register")
        }
    }

    return nil
}
```

### Sync stato con database

```go
func SyncProviderStatus(db *gorm.DB, pm *manager.ProviderManager) {
    providers := pm.ListProviders()

    for _, p := range providers {
        var dbProvider models.Provider
        if err := db.Where("name = ?", p.Name).First(&dbProvider).Error; err != nil {
            continue
        }

        // Update metrics
        db.Model(&dbProvider).Updates(map[string]interface{}{
            "last_health_check": p.LastHealthCheck,
            "health_score":      calculateHealthScore(p),
            "avg_latency_ms":    p.AvgLatency.Milliseconds(),
        })
    }
}

func calculateHealthScore(p manager.ProviderInfo) float64 {
    if p.SuccessCount == 0 {
        return 0.5
    }

    successRate := float64(p.SuccessCount) / float64(p.SuccessCount + p.ErrorCount)

    // Factor in health status
    if p.HealthStatus == "unhealthy" {
        successRate *= 0.5
    }

    return successRate
}
```

## Environment Variables

```bash
# .env file
OPENAI_API_KEY=sk-...
GROQ_API_KEY=gsk-...
ANTHROPIC_API_KEY=sk-ant-...
TOGETHER_API_KEY=...

# Provider URLs (optional, uses defaults)
OPENAI_BASE_URL=https://api.openai.com
GROQ_BASE_URL=https://api.groq.com/openai
OLLAMA_BASE_URL=http://localhost:11434
```

```go
func loadAPIKeysFromEnv() map[string]string {
    return map[string]string{
        "openai":    os.Getenv("OPENAI_API_KEY"),
        "groq":      os.Getenv("GROQ_API_KEY"),
        "anthropic": os.Getenv("ANTHROPIC_API_KEY"),
        "together":  os.Getenv("TOGETHER_API_KEY"),
        "ollama":    "", // No key needed
    }
}
```

## Metriche e Monitoring

```go
// Prometheus metrics
import "github.com/prometheus/client_golang/prometheus"

var (
    providerRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "provider_requests_total",
            Help: "Total requests per provider",
        },
        []string{"provider", "status"},
    )

    providerLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "provider_latency_seconds",
            Help: "Request latency per provider",
        },
        []string{"provider"},
    )
)

// Track metrics
func trackMetrics(provider string, latency time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
    }

    providerRequests.WithLabelValues(provider, status).Inc()
    providerLatency.WithLabelValues(provider).Observe(latency.Seconds())
}
```

## Best Practices

1. **Usa sempre il context**: Passa context.Context per gestire timeout e cancellazioni
2. **Handle errors**: Implementa fallback e retry logic
3. **Monitor health**: Usa health check worker per monitoraggio continuo
4. **Rate limiting**: I provider hanno rate limits, gestiscili con retry e backoff
5. **Secure API keys**: Non committare API keys, usa env vars o secret managers
6. **Test con mock**: Usa MockProvider per unit testing
7. **Log appropriatamente**: Usa zerolog per logging strutturato
8. **Feature detection**: Verifica SupportsFeature prima di usare funzionalità avanzate

## Troubleshooting

### Provider non disponibile

```go
provider, err := pm.GetProvider("openai")
if errors.Is(err, providers.ErrProviderNotFound) {
    // Provider not registered
}
if errors.Is(err, providers.ErrNoProvidersAvailable) {
    // No providers available
}
```

### Health check fallisce

```go
results := pm.HealthCheckAll(ctx)
for name, err := range results {
    if err != nil {
        log.Error().Err(err).Str("provider", name).Msg("Health check failed")

        // Disable provider temporarily
        pm.SetProviderStatus(name, providers.ProviderStatusMaintenance)
    }
}
```

### Rate limit errors

```go
resp, err := pm.ChatCompletion(ctx, "openai", req)
if errors.Is(err, openai.ErrRateLimitExceeded) {
    // Wait and retry with different provider
    time.Sleep(time.Second)
    resp, err = pm.LoadBalancedRequest(ctx, req)
}
```

## Testing

```go
func TestGateway(t *testing.T) {
    pm := manager.NewProviderManager()

    // Register mock provider
    mock := providers.NewMockProvider("test")
    pm.registry.Register("test", mock, "mock")

    req := &providers.ChatRequest{
        Model: "test-model",
        Messages: []providers.Message{
            {Role: "user", Content: "test"},
        },
    }

    resp, err := pm.ChatCompletion(context.Background(), "test", req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
}
```
