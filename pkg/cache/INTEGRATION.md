# Guida Integrazione Sistema di Caching

## Integrazione con Gateway

Per integrare il sistema di caching nel gateway esistente:

### 1. Modifica config.go

Aggiungi configurazione cache:

```go
// In pkg/config/config.go

type Config struct {
    Server     ServerConfig     `yaml:"server"`
    Database   database.Config  `yaml:"database"`
    Redis      RedisConfig      `yaml:"redis"`
    Cache      CacheConfig      `yaml:"cache"`      // <-- NUOVO
    Providers  ProvidersConfig  `yaml:"providers"`
    Routing    RoutingConfig    `yaml:"routing"`
    Monitoring MonitoringConfig `yaml:"monitoring"`
}

// CacheConfig configurazione del sistema di caching
type CacheConfig struct {
    Enabled              bool    `yaml:"enabled"`
    MemoryEnabled        bool    `yaml:"memory_enabled"`
    MemoryMaxSizeMB      int     `yaml:"memory_max_size_mb"`
    MemoryMaxEntries     int     `yaml:"memory_max_entries"`
    MemoryTTLMinutes     int     `yaml:"memory_ttl_minutes"`

    RedisEnabled         bool    `yaml:"redis_enabled"`
    RedisTTLMinutes      int     `yaml:"redis_ttl_minutes"`

    SemanticEnabled      bool    `yaml:"semantic_enabled"`
    SimilarityThreshold  float64 `yaml:"similarity_threshold"`

    CompressionEnabled   bool    `yaml:"compression_enabled"`
    CompressionMinSizeKB int     `yaml:"compression_min_size_kb"`
}
```

### 2. Aggiungi defaults in setDefaults

```go
// Cache defaults
v.SetDefault("cache.enabled", true)
v.SetDefault("cache.memory_enabled", true)
v.SetDefault("cache.memory_max_size_mb", 100)
v.SetDefault("cache.memory_max_entries", 10000)
v.SetDefault("cache.memory_ttl_minutes", 5)

v.SetDefault("cache.redis_enabled", false)
v.SetDefault("cache.redis_ttl_minutes", 30)

v.SetDefault("cache.semantic_enabled", false)
v.SetDefault("cache.similarity_threshold", 0.95)

v.SetDefault("cache.compression_enabled", true)
v.SetDefault("cache.compression_min_size_kb", 1)
```

### 3. Modifica gateway.go

```go
// In internal/gateway/gateway.go

import (
    "github.com/biodoia/goleapifree/pkg/cache"
)

type Gateway struct {
    config       *config.Config
    db           *database.DB
    app          *fiber.App
    router       *router.Router
    health       *health.Monitor
    cacheManager *cache.CacheManager  // <-- NUOVO
}

func New(cfg *config.Config, db *database.DB) (*Gateway, error) {
    // ... existing code ...

    // Initialize cache manager
    var cacheManager *cache.CacheManager
    if cfg.Cache.Enabled {
        cacheConfig := &cache.Config{
            MemoryEnabled:    cfg.Cache.MemoryEnabled,
            MemoryMaxSize:    int64(cfg.Cache.MemoryMaxSizeMB) * 1024 * 1024,
            MemoryMaxEntries: cfg.Cache.MemoryMaxEntries,
            MemoryTTL:        time.Duration(cfg.Cache.MemoryTTLMinutes) * time.Minute,

            RedisEnabled:  cfg.Cache.RedisEnabled && cfg.Redis.Host != "",
            RedisHost:     cfg.Redis.Host,
            RedisPassword: cfg.Redis.Password,
            RedisDB:       cfg.Redis.DB,
            RedisTTL:      time.Duration(cfg.Cache.RedisTTLMinutes) * time.Minute,

            LRUEnabled:   true,
        }

        semanticConfig := &cache.SemanticConfig{
            SimilarityThreshold: cfg.Cache.SimilarityThreshold,
            UseSimpleHash:       !cfg.Cache.SemanticEnabled,
            EmbeddingProvider:   &cache.SimpleEmbeddingProvider{},
        }

        responseConfig := &cache.ResponseCacheConfig{
            DefaultTTL:         time.Duration(cfg.Cache.MemoryTTLMinutes) * time.Minute,
            CompressionMinSize: cfg.Cache.CompressionMinSizeKB * 1024,
            UseCompression:     cfg.Cache.CompressionEnabled,
            UseSemanticCache:   cfg.Cache.SemanticEnabled,
        }

        var err error
        cacheManager, err = cache.NewCacheManager(&cache.CacheManagerConfig{
            MemoryConfig:   cacheConfig,
            SemanticConfig: semanticConfig,
            ResponseConfig: responseConfig,
        })

        if err != nil {
            log.Warn().Err(err).Msg("Failed to initialize cache manager, continuing without cache")
        } else {
            log.Info().Msg("Cache manager initialized successfully")
        }
    }

    gw := &Gateway{
        config:       cfg,
        db:           db,
        app:          app,
        router:       r,
        health:       healthMonitor,
        cacheManager: cacheManager,  // <-- NUOVO
    }

    gw.setupRoutes()

    return gw, nil
}
```

### 4. Setup Routes con Cache Middleware

```go
func (g *Gateway) setupRoutes() {
    api := g.app.Group("/v1")

    // Cache middleware (se abilitato)
    if g.cacheManager != nil {
        cacheMiddleware := cache.CacheMiddleware(&cache.CacheMiddlewareConfig{
            ResponseCache: g.cacheManager.GetResponseCache(),
            Enabled:       true,
            SkipPaths:     []string{"/health", "/metrics"},
            CacheTTL:      30 * time.Minute,
            AddHeaders:    true,
        })
        api.Use(cacheMiddleware)

        log.Info().Msg("Cache middleware enabled for /v1 routes")
    }

    // OpenAI compatible endpoints
    api.Post("/chat/completions", g.handleChatCompletion)
    api.Get("/models", g.handleListModels)

    // ... rest of routes ...

    // Admin cache endpoints
    if g.cacheManager != nil {
        adminCache := g.app.Group("/admin/cache")
        adminCache.Get("/stats", cache.CacheStatsMiddleware(g.cacheManager.GetResponseCache()))
        adminCache.Post("/clear", cache.CacheClearMiddleware(g.cacheManager.GetResponseCache()))
        adminCache.Post("/invalidate", cache.InvalidateCacheMiddleware(g.cacheManager.GetResponseCache()))

        log.Info().Msg("Cache admin endpoints enabled at /admin/cache")
    }
}
```

### 5. Shutdown Graceful

```go
func (g *Gateway) Shutdown(ctx context.Context) error {
    // Stop health monitoring
    g.health.Stop()

    // Close cache manager
    if g.cacheManager != nil {
        if err := g.cacheManager.Close(); err != nil {
            log.Error().Err(err).Msg("Failed to close cache manager")
        } else {
            log.Info().Msg("Cache manager closed")
        }
    }

    // Shutdown HTTP server
    if err := g.app.ShutdownWithContext(ctx); err != nil {
        return fmt.Errorf("failed to shutdown server: %w", err)
    }

    log.Info().Msg("Gateway shutdown completed")
    return nil
}
```

### 6. Uso Manuale nel Handler (Opzionale)

Se vuoi controllare manualmente il caching in handleChatCompletion:

```go
func (g *Gateway) handleChatCompletion(c fiber.Ctx) error {
    var req cache.ChatCompletionRequest
    if err := c.Bind().Body(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
    }

    ctx := context.Background()

    // Check cache (se middleware non è abilitato)
    if g.cacheManager != nil {
        responseCache := g.cacheManager.GetResponseCache()

        cached, err := responseCache.Get(ctx, &req)
        if err == nil {
            log.Info().Msg("Serving cached response")
            c.Set("X-Cache-Hit", "true")
            return c.JSON(cached.Response)
        }
    }

    // Route request to provider
    response, err := g.router.RouteRequest(ctx, &req)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    // Cache response
    if g.cacheManager != nil && responseCache.IsCacheable(&req) {
        responseCache := g.cacheManager.GetResponseCache()
        if err := responseCache.Set(ctx, &req, response, 30*time.Minute); err != nil {
            log.Warn().Err(err).Msg("Failed to cache response")
        }
    }

    return c.JSON(response)
}
```

## File di Configurazione YAML

```yaml
# config.yaml

cache:
  enabled: true

  # Memory cache
  memory_enabled: true
  memory_max_size_mb: 100
  memory_max_entries: 10000
  memory_ttl_minutes: 5

  # Redis cache (opzionale)
  redis_enabled: false
  redis_ttl_minutes: 30

  # Semantic cache (sperimentale)
  semantic_enabled: false
  similarity_threshold: 0.95

  # Compression
  compression_enabled: true
  compression_min_size_kb: 1

redis:
  host: "localhost:6379"
  password: ""
  db: 0
```

## Test dell'Integrazione

### 1. Avvia il server

```bash
go run cmd/backend/main.go
```

### 2. Fai una richiesta

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.0
  }'
```

### 3. Fai la stessa richiesta (dovrebbe essere cachata)

```bash
curl -v -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.0
  }'

# Dovresti vedere header: X-Cache-Hit: true
```

### 4. Check statistiche

```bash
curl http://localhost:8080/admin/cache/stats
```

Output esempio:
```json
{
  "cache_stats": {
    "hits": 5,
    "misses": 3,
    "hit_rate": 0.625,
    "sets": 3
  },
  "response_stats": {
    "compressed_sets": 2,
    "avg_response_size": 1024,
    "compression_ratio": 0.65
  }
}
```

### 5. Svuota cache

```bash
curl -X POST http://localhost:8080/admin/cache/clear
```

## Metriche Prometheus (Future)

```go
// In handleMetrics
func (g *Gateway) handleMetrics(c fiber.Ctx) error {
    metrics := []string{
        "# HELP goleapai_cache_hits_total Total cache hits",
        "# TYPE goleapai_cache_hits_total counter",
    }

    if g.cacheManager != nil {
        stats := g.cacheManager.GetAllStats()

        if mlStats, ok := stats["multi_layer"].(cache.CacheStats); ok {
            metrics = append(metrics,
                fmt.Sprintf("goleapai_cache_hits_total %d", mlStats.Hits),
                fmt.Sprintf("goleapai_cache_misses_total %d", mlStats.Misses),
                fmt.Sprintf("goleapai_cache_hit_rate %.2f", mlStats.HitRate()),
            )
        }
    }

    return c.SendString(strings.Join(metrics, "\n"))
}
```

## Troubleshooting

### Cache non funziona

1. Verifica che `cache.enabled: true` in config.yaml
2. Check logs per errori di inizializzazione
3. Verifica che temperature sia < 0.7 (altrimenti non cacheable)
4. Non usi stream requests (non cacheable)

### Memory issues

Riduci `memory_max_size_mb` e `memory_max_entries` in config.yaml

### Redis connection failed

Abilita fallback a memory-only:
```yaml
cache:
  redis_enabled: false
  memory_enabled: true
```

## Performance Tips

1. **TTL Tuning**: Bilancia freshness vs hit rate
   - TTL corto = più fresh, meno hits
   - TTL lungo = meno fresh, più hits

2. **Compression**: Abilita per large responses
   ```yaml
   compression_enabled: true
   compression_min_size_kb: 1
   ```

3. **Semantic Cache**: Abilita per aumentare hit rate
   ```yaml
   semantic_enabled: true
   similarity_threshold: 0.95
   ```

4. **Redis**: Usa per deployment multi-istanza
   ```yaml
   redis_enabled: true
   ```

5. **Monitoring**: Monitora hit rate target 40-60%
   ```bash
   watch -n 5 'curl -s localhost:8080/admin/cache/stats | jq .cache_stats.hit_rate'
   ```
