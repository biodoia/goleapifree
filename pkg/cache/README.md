# GoLeapAI Free - Advanced Caching System

Sistema di caching multi-layer avanzato per ridurre chiamate API, costi e latenza nelle richieste LLM.

## Architettura

Il sistema è composto da 4 layer principali:

### 1. Multi-Layer Cache (`cache.go`)
Cache ibrido con supporto per:
- **In-Memory Cache**: BigCache-like con LRU eviction
- **Redis Cache**: Cache distribuito per deployment multi-istanza
- **TTL Configurabile**: Scadenza automatica delle entry
- **Statistiche**: Hit rate, eviction rate, dimensioni

### 2. Semantic Cache (`semantic.go`)
Cache intelligente basato su similarity:
- **Embedding-based**: Usa vector embeddings per matching semantico
- **Similarity Threshold**: Cache hit se similarity > threshold (default 95%)
- **Vector Database**: Supporto per Qdrant/Weaviate (opzionale)
- **Fallback Hash**: Mode semplice senza embeddings

### 3. Response Cache (`response.go`)
Cache ottimizzato per chat completions:
- **Request Hashing**: Hash di model + messages + params
- **Compression**: Compressione gzip per large responses (>1KB)
- **Hit Rate Tracking**: Metriche dettagliate su performance
- **Deterministic Only**: Cache solo request non-stocastiche (temperature < 0.7)

### 4. Cache Middleware (`middleware.go`)
Middleware Fiber per caching trasparente:
- **Auto-caching**: Intercepta e cacha richieste automaticamente
- **Cache Headers**: Aggiunge `X-Cache-Hit: true/false`
- **Skip Paths**: Configurabile quali endpoint cachare
- **Admin Endpoints**: API per stats, clear, invalidate

## Quick Start

### Setup Base

```go
import "github.com/biodoia/goleapifree/pkg/cache"

// 1. Configurazione
config := &cache.Config{
    MemoryEnabled:    true,
    MemoryMaxSize:    100 * 1024 * 1024, // 100MB
    MemoryMaxEntries: 10000,
    MemoryTTL:        5 * time.Minute,

    RedisEnabled:  false,
    RedisHost:     "localhost:6379",

    LRUEnabled:    true,
}

// 2. Crea cache manager
manager, err := cache.NewCacheManager(&cache.CacheManagerConfig{
    MemoryConfig: config,
})
if err != nil {
    log.Fatal(err)
}
defer manager.Close()
```

### Integrazione Fiber

```go
// 1. Setup middleware
cacheMiddleware := cache.CacheMiddleware(&cache.CacheMiddlewareConfig{
    ResponseCache: manager.GetResponseCache(),
    Enabled:       true,
    CacheTTL:      30 * time.Minute,
    AddHeaders:    true,
})

// 2. Applica alle route
app.Group("/v1").Use(cacheMiddleware)

// 3. Admin endpoints
admin := app.Group("/admin/cache")
admin.Get("/stats", cache.CacheStatsMiddleware(manager.GetResponseCache()))
admin.Post("/clear", cache.CacheClearMiddleware(manager.GetResponseCache()))
```

### Uso Manuale

```go
ctx := context.Background()
responseCache := manager.GetResponseCache()

// Request
req := &cache.ChatCompletionRequest{
    Model: "gpt-4",
    Messages: []cache.Message{
        {Role: "user", Content: "What is Go?"},
    },
    Temperature: 0.0,
}

// Check cache
cached, err := responseCache.Get(ctx, req)
if err == nil {
    // Cache hit!
    return cached.Response
}

// Cache miss - chiama LLM
response := callLLM(req)

// Salva in cache
responseCache.Set(ctx, req, response, 30*time.Minute)
```

## Semantic Caching

Per abilitare il semantic caching:

```go
// 1. Configura semantic cache
semanticConfig := &cache.SemanticConfig{
    SimilarityThreshold: 0.95, // 95% similarity
    UseSimpleHash:       true, // o false se hai embedding provider
    EmbeddingProvider:   &cache.SimpleEmbeddingProvider{},
}

// 2. Crea cache manager con semantic
manager, err := cache.NewCacheManager(&cache.CacheManagerConfig{
    MemoryConfig:   baseConfig,
    SemanticConfig: semanticConfig,
    ResponseConfig: &cache.ResponseCacheConfig{
        UseSemanticCache: true,
    },
})

// 3. Ora le query simili generano cache hit
// "What is the capital of France?"
// "What's France's capital city?" <- CACHE HIT!
```

## Compression

Compression automatica per large responses:

```go
responseConfig := &cache.ResponseCacheConfig{
    UseCompression:     true,
    CompressionMinSize: 1024, // Comprimi se > 1KB
    DefaultTTL:         30 * time.Minute,
}
```

## Monitoring

### Statistiche

```go
// Statistiche aggregate
allStats := manager.GetAllStats()

// Statistiche specifiche
responseStats := manager.GetResponseCache().GetMetrics()
fmt.Printf("Hit Rate: %.2f%%\n", responseStats["hit_rate"].(float64) * 100)
fmt.Printf("Total Hits: %d\n", responseStats["total_hits"])
fmt.Printf("Avg Response Size: %d bytes\n", responseStats["avg_response_size"])
fmt.Printf("Compression Ratio: %.2f\n", responseStats["compression_ratio"])
```

### Health Check

```go
if err := cache.CacheHealthCheck(manager.GetMultiLayerCache()); err != nil {
    log.Error("Cache unhealthy:", err)
}
```

### Metrics Collector

```go
collector := cache.NewCacheMetricsCollector(
    manager.GetMultiLayerCache(),
    manager.GetResponseCache(),
    1 * time.Minute, // Intervallo raccolta
)
collector.Start()

// Ottieni metriche
metrics := collector.GetMetrics()
```

## Cache Warmup

Pre-popola il cache con query comuni:

```go
commonQueries := []*cache.ChatCompletionRequest{
    {
        Model: "gpt-4",
        Messages: []cache.Message{
            {Role: "user", Content: "Hello world in Python"},
        },
        Temperature: 0.0,
    },
    // ... altre query comuni
}

cache.WarmupCache(ctx, &cache.CacheWarmupConfig{
    Requests:      commonQueries,
    ResponseCache: manager.GetResponseCache(),
    TTL:           24 * time.Hour,
})
```

## Configurazione Avanzata

### Redis Distribuito

```go
config := &cache.Config{
    RedisEnabled:  true,
    RedisHost:     "redis.example.com:6379",
    RedisPassword: "secret",
    RedisDB:       0,
    RedisTTL:      1 * time.Hour,
}
```

### Vector Database (Future)

```go
semanticConfig := &cache.SemanticConfig{
    UseVectorDB:      true,
    VectorDBEndpoint: "http://qdrant:6333",
    // Richiede implementazione VectorDB interface
}
```

## Performance

### Metriche Attese

- **Memory Cache**: < 1ms latency
- **Redis Cache**: 1-5ms latency
- **Semantic Match**: 5-20ms latency
- **Compression Ratio**: 60-80% per JSON responses
- **Hit Rate Target**: 40-60% per production workload

### Ottimizzazioni

1. **TTL Tuning**: Bilancia freshness vs hit rate
2. **Compression Threshold**: Più alto = meno CPU, meno risparmio storage
3. **Similarity Threshold**: Più basso = più cache hit, meno accuracy
4. **Memory Limits**: Evita OOM con `MemoryMaxSize`

## API Reference

### CacheManager

- `NewCacheManager(config)`: Crea manager
- `GetMemoryCache()`: Ottieni memory cache
- `GetRedisCache()`: Ottieni redis cache
- `GetSemanticCache()`: Ottieni semantic cache
- `GetResponseCache()`: Ottieni response cache
- `GetAllStats()`: Statistiche aggregate
- `Close()`: Chiudi connessioni

### ResponseCache

- `Get(ctx, req)`: Recupera cached response
- `Set(ctx, req, response, ttl)`: Salva response
- `Delete(ctx, req)`: Invalida cache entry
- `Clear(ctx)`: Svuota cache
- `Stats()`: Statistiche base
- `GetMetrics()`: Metriche dettagliate
- `IsCacheable(req)`: Check se cacheable

### SemanticCache

- `Get(ctx, prompt)`: Cerca con similarity
- `Set(ctx, prompt, value, ttl)`: Salva con embedding
- `SemanticStats()`: Statistiche semantic
- `CosineSimilarity(a, b)`: Calcola similarity

## Troubleshooting

### Cache Hit Rate Basso

1. Verifica TTL non troppo corto
2. Check determinismo request (temperature)
3. Abilita semantic cache
4. Verifica prompt normalization

### Memory Issues

1. Riduci `MemoryMaxEntries`
2. Riduci `MemoryMaxSize`
3. Abilita compression
4. Usa Redis invece di memory

### Redis Connection Failed

1. Verifica Redis running
2. Check host/password
3. Fallback a memory-only: `RedisEnabled: false`

## Roadmap

- [ ] Vector database integration (Qdrant, Weaviate)
- [ ] Embedding provider (OpenAI, HuggingFace)
- [ ] Distributed cache invalidation
- [ ] Cache persistence
- [ ] Advanced eviction policies (LFU, ARC)
- [ ] Cache analytics dashboard
- [ ] Multi-region replication

## License

MIT License - Part of GoLeapAI Free
