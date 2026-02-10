package cache

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// ExampleSetup mostra come configurare il sistema di caching
func ExampleSetup() (*CacheManager, error) {
	// 1. Configurazione base cache
	baseConfig := &Config{
		MemoryEnabled:    true,
		MemoryMaxSize:    100 * 1024 * 1024, // 100MB
		MemoryMaxEntries: 10000,
		MemoryTTL:        5 * time.Minute,

		RedisEnabled:  false, // Abilita se hai Redis
		RedisHost:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		RedisTTL:      30 * time.Minute,

		LRUEnabled:   true,
		LRUMaxSize:   1000,
		EvictionRate: 0.1,
	}

	// 2. Configurazione semantic cache
	semanticConfig := &SemanticConfig{
		SimilarityThreshold: 0.95,
		UseVectorDB:         false,
		UseSimpleHash:       true, // Usa hash finché non hai embedding provider
		EmbeddingProvider:   &SimpleEmbeddingProvider{},
	}

	// 3. Configurazione response cache
	responseConfig := &ResponseCacheConfig{
		DefaultTTL:         30 * time.Minute,
		CompressionMinSize: 1024,
		UseCompression:     true,
		UseSemanticCache:   true,
	}

	// 4. Crea cache manager
	manager, err := NewCacheManager(&CacheManagerConfig{
		MemoryConfig:   baseConfig,
		SemanticConfig: semanticConfig,
		ResponseConfig: responseConfig,
	})

	if err != nil {
		return nil, err
	}

	log.Info().Msg("Cache system initialized successfully")

	return manager, nil
}

// ExampleFiberIntegration mostra come integrare con Fiber
func ExampleFiberIntegration(app *fiber.App, manager *CacheManager) {
	// 1. Setup cache middleware
	cacheMiddleware := CacheMiddleware(&CacheMiddlewareConfig{
		ResponseCache: manager.GetResponseCache(),
		Enabled:       true,
		SkipPaths:     []string{"/health", "/metrics"},
		CacheTTL:      30 * time.Minute,
		AddHeaders:    true,
	})

	// 2. Applica middleware alle route che vuoi cachare
	api := app.Group("/v1")
	api.Use(cacheMiddleware)

	// 3. Setup admin endpoints per gestire il cache
	admin := app.Group("/admin/cache")

	admin.Get("/stats", CacheStatsMiddleware(manager.GetResponseCache()))
	admin.Post("/clear", CacheClearMiddleware(manager.GetResponseCache()))
	admin.Post("/invalidate", InvalidateCacheMiddleware(manager.GetResponseCache()))

	log.Info().Msg("Cache middleware integrated with Fiber")
}

// ExampleUsage mostra l'uso base del cache
func ExampleUsage() {
	ctx := context.Background()

	// 1. Crea cache manager
	manager, err := ExampleSetup()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup cache")
		return
	}
	defer manager.Close()

	// 2. Esempio: usa response cache
	responseCache := manager.GetResponseCache()

	// Crea una richiesta
	req := &ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	// Prova a recuperare dal cache
	cached, err := responseCache.Get(ctx, req)
	if err == nil {
		log.Info().Msg("Cache hit! Using cached response")
		log.Info().Interface("response", cached).Msg("Cached response")
		return
	}

	// Cache miss - simula chiamata all'LLM
	log.Info().Msg("Cache miss - calling LLM")

	response := map[string]interface{}{
		"id":      "chatcmpl-123",
		"model":   req.Model,
		"choices": []map[string]interface{}{
			{
				"message": map[string]string{
					"role":    "assistant",
					"content": "The capital of France is Paris.",
				},
			},
		},
	}

	// Cacha la response
	if err := responseCache.Set(ctx, req, response, 30*time.Minute); err != nil {
		log.Error().Err(err).Msg("Failed to cache response")
		return
	}

	log.Info().Msg("Response cached successfully")
}

// ExampleSemanticCache mostra l'uso del semantic cache
func ExampleSemanticCache() {
	ctx := context.Background()

	// Crea semantic cache
	baseCache := NewMemoryCache(1000, 10*time.Minute)
	semanticCache, err := NewSemanticCache(&SemanticConfig{
		BaseCache:           baseCache,
		SimilarityThreshold: 0.95,
		UseSimpleHash:       true,
		EmbeddingProvider:   &SimpleEmbeddingProvider{},
	})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create semantic cache")
		return
	}

	// Salva una risposta
	prompt1 := "What is the capital of France?"
	response1 := []byte(`{"answer": "Paris"}`)

	if err := semanticCache.Set(ctx, prompt1, response1, 10*time.Minute); err != nil {
		log.Error().Err(err).Msg("Failed to set cache")
		return
	}

	// Prova con un prompt simile
	prompt2 := "What is France's capital city?" // Simile semanticamente

	cached, err := semanticCache.Get(ctx, prompt2)
	if err == nil {
		log.Info().
			Str("original_prompt", prompt1).
			Str("query_prompt", prompt2).
			Msg("Semantic cache hit!")
		log.Info().Bytes("response", cached).Msg("Cached response")
	} else {
		log.Info().Msg("Semantic cache miss")
	}

	// Statistiche
	stats := semanticCache.SemanticStats()
	log.Info().
		Int64("hits", stats.Hits).
		Int64("misses", stats.Misses).
		Float64("hit_rate", stats.HitRate()).
		Msg("Semantic cache stats")
}

// ExampleMonitoring mostra come monitorare il cache
func ExampleMonitoring(manager *CacheManager) {
	// 1. Crea metrics collector
	collector := NewCacheMetricsCollector(
		manager.GetMultiLayerCache(),
		manager.GetResponseCache(),
		1*time.Minute,
	)

	// 2. Avvia raccolta metriche
	collector.Start()

	// 3. Crea monitor con observer
	monitor := NewCacheMonitor(
		manager.GetMultiLayerCache(),
		5*time.Minute,
		&LoggingObserver{},
	)

	// 4. Avvia monitoring
	monitor.Start()

	log.Info().Msg("Cache monitoring started")

	// Per fermare:
	// monitor.Stop()
}

// ExampleHealthCheck mostra come fare health check del cache
func ExampleHealthCheck(manager *CacheManager) error {
	log.Info().Msg("Running cache health check")

	// Check multi-layer cache
	if err := CacheHealthCheck(manager.GetMultiLayerCache()); err != nil {
		log.Error().Err(err).Msg("Multi-layer cache health check failed")
		return err
	}

	// Check memory cache
	if err := CacheHealthCheck(manager.GetMemoryCache()); err != nil {
		log.Error().Err(err).Msg("Memory cache health check failed")
		return err
	}

	log.Info().Msg("Cache health check passed")
	return nil
}

// ExampleCacheWarmup mostra come fare warmup del cache
func ExampleCacheWarmup(manager *CacheManager) {
	ctx := context.Background()

	// Definisci query comuni da pre-cachare
	commonQueries := []*ChatCompletionRequest{
		{
			Model: "gpt-4",
			Messages: []Message{
				{Role: "user", Content: "What is the capital of France?"},
			},
			Temperature: 0.0,
		},
		{
			Model: "gpt-4",
			Messages: []Message{
				{Role: "user", Content: "Explain quantum computing in simple terms"},
			},
			Temperature: 0.0,
		},
		{
			Model: "gpt-3.5-turbo",
			Messages: []Message{
				{Role: "user", Content: "Write a hello world in Python"},
			},
			Temperature: 0.0,
		},
	}

	// Warmup cache
	warmupConfig := &CacheWarmupConfig{
		Requests:      commonQueries,
		ResponseCache: manager.GetResponseCache(),
		TTL:           24 * time.Hour,
	}

	if err := WarmupCache(ctx, warmupConfig); err != nil {
		log.Error().Err(err).Msg("Cache warmup failed")
		return
	}

	log.Info().
		Int("queries", len(commonQueries)).
		Msg("Cache warmup completed")
}

// ExampleStatistics mostra come ottenere statistiche dettagliate
func ExampleStatistics(manager *CacheManager) {
	// Ottieni tutte le statistiche
	allStats := manager.GetAllStats()

	log.Info().Interface("stats", allStats).Msg("Cache statistics")

	// Statistiche specifiche per response cache
	if responseCache := manager.GetResponseCache(); responseCache != nil {
		metrics := responseCache.GetMetrics()

		log.Info().
			Float64("hit_rate", metrics["hit_rate"].(float64)).
			Int64("total_hits", metrics["total_hits"].(int64)).
			Int64("total_misses", metrics["total_misses"].(int64)).
			Int64("avg_response_size", metrics["avg_response_size"].(int64)).
			Float64("compression_ratio", metrics["compression_ratio"].(float64)).
			Msg("Response cache metrics")
	}

	// Statistiche semantic cache
	if semanticCache := manager.GetSemanticCache(); semanticCache != nil {
		semanticStats := semanticCache.SemanticStats()

		log.Info().
			Int64("semantic_hits", semanticStats.SemanticHits).
			Int64("semantic_misses", semanticStats.SemanticMisses).
			Float64("avg_similarity", semanticStats.AverageSimilarity).
			Msg("Semantic cache statistics")
	}
}
