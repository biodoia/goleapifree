package embeddings

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// IntegrationExample mostra come integrare semantic cache nel sistema
type IntegrationExample struct {
	middleware *CacheMiddleware
	generator  EmbeddingGenerator
	cache      *SemanticCache
}

// NewIntegrationExample crea un esempio di integrazione
func NewIntegrationExample() (*IntegrationExample, error) {
	// 1. Setup embedding generator
	generatorConfig := &GeneratorConfig{
		Provider:   "cohere",
		APIKey:     os.Getenv("COHERE_API_KEY"),
		Model:      "embed-english-light-v3.0",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		BatchSize:  96,
	}

	generator, err := NewGenerator(generatorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	// 2. Setup semantic cache
	cacheConfig := &SemanticCacheConfig{
		SimilarityThreshold: 0.9,
		DefaultTTL:          10 * time.Minute,
		MaxCacheSize:        1000,
		CleanupInterval:     5 * time.Minute,
		EnableAutoCleanup:   true,
	}

	cache, err := NewSemanticCache(generator, cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// 3. Setup middleware
	middleware := NewCacheMiddleware(cache, &CacheMiddlewareConfig{
		Enabled: true,
	})

	return &IntegrationExample{
		middleware: middleware,
		generator:  generator,
		cache:      cache,
	}, nil
}

// HandleChatCompletion esempio di handler per chat completions
func (ie *IntegrationExample) HandleChatCompletion(ctx context.Context, prompt string, model string) (string, error) {
	// Usa middleware per caching automatico
	result, err := ie.middleware.GetOrCompute(
		ctx,
		prompt,
		10*time.Minute,
		func(ctx context.Context) (interface{}, error) {
			// Qui andresti a chiamare l'API reale
			// Per esempio: OpenAI, Anthropic, etc.
			log.Info().
				Str("prompt", prompt[:min(50, len(prompt))]).
				Str("model", model).
				Msg("Cache miss - calling API")

			// Simula chiamata API
			response := fmt.Sprintf("Response to: %s", prompt)
			time.Sleep(100 * time.Millisecond) // Simula latenza API

			return response, nil
		},
	)

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// BatchProcessRequests processa batch di richieste con deduplication
func (ie *IntegrationExample) BatchProcessRequests(ctx context.Context, prompts []string, model string) ([]string, error) {
	// Converti prompts in requests
	requests := make([]*CompletionRequest, len(prompts))
	for i, prompt := range prompts {
		requests[i] = &CompletionRequest{
			Prompt:      prompt,
			Model:       model,
			Temperature: 0.7,
			MaxTokens:   1000,
		}
	}

	// Deduplica richieste semanticamente simili
	groups, err := ie.middleware.DeduplicateRequests(ctx, requests, 0.9)
	if err != nil {
		return nil, fmt.Errorf("failed to deduplicate: %w", err)
	}

	log.Info().
		Int("original", len(requests)).
		Int("deduplicated", len(groups)).
		Msg("Deduplication complete")

	// Processa solo richieste uniche
	responses := make(map[string]string)
	for _, group := range groups {
		// Cerca in cache
		cached, found, err := ie.cache.Get(ctx, group.Representative.Prompt)
		if err == nil && found {
			responses[group.Representative.Prompt] = cached.(string)
			log.Debug().Msg("Using cached response for group")
			continue
		}

		// Cache miss - chiama API
		response, err := ie.callAPI(ctx, group.Representative)
		if err != nil {
			return nil, err
		}

		responses[group.Representative.Prompt] = response

		// Cache la risposta
		ie.cache.Set(ctx, group.Representative.Prompt, response, 10*time.Minute)
	}

	// Mappa risposte a prompts originali
	results := make([]string, len(prompts))
	for i, req := range requests {
		results[i] = responses[req.Prompt]
	}

	return results, nil
}

// callAPI simula chiamata API
func (ie *IntegrationExample) callAPI(ctx context.Context, req *CompletionRequest) (string, error) {
	log.Info().
		Str("prompt", req.Prompt[:min(50, len(req.Prompt))]).
		Str("model", req.Model).
		Msg("Calling API")

	// Simula latenza API
	time.Sleep(100 * time.Millisecond)

	return fmt.Sprintf("API response to: %s", req.Prompt), nil
}

// WarmupCache pre-carica cache con risposte comuni
func (ie *IntegrationExample) WarmupCache(ctx context.Context) error {
	commonPrompts := []struct {
		Prompt   string
		Response string
		TTL      time.Duration
	}{
		{
			Prompt:   "What is artificial intelligence?",
			Response: "Artificial intelligence (AI) is the simulation of human intelligence by machines...",
			TTL:      1 * time.Hour,
		},
		{
			Prompt:   "Explain machine learning",
			Response: "Machine learning is a subset of AI that enables systems to learn from data...",
			TTL:      1 * time.Hour,
		},
		{
			Prompt:   "What is deep learning?",
			Response: "Deep learning is a subset of machine learning that uses neural networks...",
			TTL:      1 * time.Hour,
		},
	}

	for _, item := range commonPrompts {
		if err := ie.cache.Set(ctx, item.Prompt, item.Response, item.TTL); err != nil {
			log.Warn().
				Err(err).
				Str("prompt", item.Prompt).
				Msg("Failed to warmup cache entry")
		}
	}

	log.Info().
		Int("count", len(commonPrompts)).
		Msg("Cache warmup complete")

	return nil
}

// GetStats restituisce statistiche complete
func (ie *IntegrationExample) GetStats() map[string]interface{} {
	cacheStats := ie.cache.Stats()

	stats := map[string]interface{}{
		"cache": map[string]interface{}{
			"hit_rate":         cacheStats.HitRate() * 100,
			"total_queries":    cacheStats.TotalQueries,
			"hits":             cacheStats.Hits,
			"misses":           cacheStats.Misses,
			"avg_lookup_time":  cacheStats.AvgLookupTime.String(),
			"size":             ie.cache.Size(),
			"max_size":         ie.cache.config.MaxCacheSize,
			"fill_percentage":  float64(ie.cache.Size()) / float64(ie.cache.config.MaxCacheSize) * 100,
		},
	}

	// Generator stats (se cached generator)
	if cached, ok := ie.generator.(*CachedGenerator); ok {
		hits, misses, size := cached.CacheStats()
		total := hits + misses
		hitRate := 0.0
		if total > 0 {
			hitRate = float64(hits) / float64(total) * 100
		}

		stats["generator"] = map[string]interface{}{
			"cache_hits":     hits,
			"cache_misses":   misses,
			"cache_size":     size,
			"cache_hit_rate": hitRate,
		}
	}

	return stats
}

// FindSimilarQueries trova query simili a quella data
func (ie *IntegrationExample) FindSimilarQueries(ctx context.Context, query string, limit int) ([]SimilarPrompt, error) {
	return ie.cache.FindSimilar(ctx, query, limit)
}

// GetTopQueries restituisce le query più popolari
func (ie *IntegrationExample) GetTopQueries(n int) []CachedResponse {
	return ie.cache.GetTopHits(n)
}

// InvalidateSimilarToQuery invalida query simili
func (ie *IntegrationExample) InvalidateSimilarToQuery(ctx context.Context, query string, threshold float64) (int, error) {
	return ie.middleware.InvalidateSimilar(ctx, query, threshold)
}

// ExportCache esporta il cache in formato JSON
func (ie *IntegrationExample) ExportCache() ([]byte, error) {
	return ie.cache.Export()
}

// ImportCache importa cache da JSON
func (ie *IntegrationExample) ImportCache(ctx context.Context, data []byte) error {
	return ie.cache.Import(ctx, data)
}

// Close chiude le risorse
func (ie *IntegrationExample) Close() error {
	return ie.cache.Close()
}

// DemoUsage dimostra l'uso completo del sistema
func DemoUsage() {
	ctx := context.Background()

	// Setup
	example, err := NewIntegrationExample()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup")
	}
	defer example.Close()

	// Warmup cache
	if err := example.WarmupCache(ctx); err != nil {
		log.Warn().Err(err).Msg("Warmup failed")
	}

	// Test 1: Single query con caching
	log.Info().Msg("=== Test 1: Single query ===")
	response1, _ := example.HandleChatCompletion(ctx, "What is AI?", "gpt-3.5")
	fmt.Printf("Response: %s\n", response1)

	// Query simile dovrebbe usare cache
	response2, _ := example.HandleChatCompletion(ctx, "What is artificial intelligence?", "gpt-3.5")
	fmt.Printf("Response (cached): %s\n", response2)

	// Test 2: Batch processing con deduplication
	log.Info().Msg("\n=== Test 2: Batch processing ===")
	prompts := []string{
		"What is machine learning?",
		"Explain machine learning",
		"What is deep learning?",
		"Tell me about deep learning",
	}

	responses, _ := example.BatchProcessRequests(ctx, prompts, "gpt-3.5")
	for i, resp := range responses {
		fmt.Printf("Prompt %d: %s\n", i+1, resp)
	}

	// Test 3: Analytics
	log.Info().Msg("\n=== Test 3: Analytics ===")
	stats := example.GetStats()
	fmt.Printf("Cache hit rate: %.2f%%\n", stats["cache"].(map[string]interface{})["hit_rate"])
	fmt.Printf("Total queries: %v\n", stats["cache"].(map[string]interface{})["total_queries"])

	// Top queries
	topQueries := example.GetTopQueries(5)
	fmt.Println("\nTop queries:")
	for i, q := range topQueries {
		fmt.Printf("%d. %s (hits: %d)\n", i+1, q.Prompt, q.Hits)
	}

	// Test 4: Similar queries
	log.Info().Msg("\n=== Test 4: Similar queries ===")
	similar, _ := example.FindSimilarQueries(ctx, "AI explanation", 3)
	fmt.Println("Similar queries to 'AI explanation':")
	for _, s := range similar {
		fmt.Printf("- %s (similarity: %.2f)\n", s.Prompt, s.Similarity)
	}

	// Test 5: Cache invalidation
	log.Info().Msg("\n=== Test 5: Cache invalidation ===")
	count, _ := example.InvalidateSimilarToQuery(ctx, "machine learning", 0.85)
	fmt.Printf("Invalidated %d similar entries\n", count)

	// Final stats
	log.Info().Msg("\n=== Final Stats ===")
	finalStats := example.GetStats()
	cacheStats := finalStats["cache"].(map[string]interface{})
	fmt.Printf("Hit rate: %.2f%%\n", cacheStats["hit_rate"])
	fmt.Printf("Cache size: %v/%v (%.1f%% full)\n",
		cacheStats["size"],
		cacheStats["max_size"],
		cacheStats["fill_percentage"])
	fmt.Printf("Avg lookup time: %v\n", cacheStats["avg_lookup_time"])
}

// HTTPHandlerExample esempio di integrazione in HTTP handler
type HTTPHandlerExample struct {
	integration *IntegrationExample
}

// NewHTTPHandlerExample crea un nuovo handler
func NewHTTPHandlerExample(integration *IntegrationExample) *HTTPHandlerExample {
	return &HTTPHandlerExample{
		integration: integration,
	}
}

// HandleCompletionRequest simula un HTTP handler per completions
func (h *HTTPHandlerExample) HandleCompletionRequest(ctx context.Context, prompt string) (map[string]interface{}, error) {
	start := time.Now()

	// Processa con semantic cache
	response, err := h.integration.HandleChatCompletion(ctx, prompt, "gpt-3.5-turbo")
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)

	// Controlla se era cachato
	_, cached, _ := h.integration.cache.Get(ctx, prompt)

	return map[string]interface{}{
		"response":      response,
		"cached":        cached,
		"latency_ms":    elapsed.Milliseconds(),
		"model":         "gpt-3.5-turbo",
		"timestamp":     time.Now().Unix(),
	}, nil
}

// HandleStatsRequest handler per statistiche
func (h *HTTPHandlerExample) HandleStatsRequest() map[string]interface{} {
	return h.integration.GetStats()
}

// HandleTopQueriesRequest handler per top queries
func (h *HTTPHandlerExample) HandleTopQueriesRequest(limit int) []map[string]interface{} {
	topQueries := h.integration.GetTopQueries(limit)
	result := make([]map[string]interface{}, len(topQueries))

	for i, q := range topQueries {
		result[i] = map[string]interface{}{
			"prompt":     q.Prompt,
			"hits":       q.Hits,
			"created_at": q.CreatedAt.Unix(),
			"last_hit":   q.LastHitAt.Unix(),
			"ttl_sec":    q.TTL().Seconds(),
		}
	}

	return result
}

// HandleSimilarQueriesRequest handler per query simili
func (h *HTTPHandlerExample) HandleSimilarQueriesRequest(ctx context.Context, query string, limit int) ([]map[string]interface{}, error) {
	similar, err := h.integration.FindSimilarQueries(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(similar))
	for i, s := range similar {
		result[i] = map[string]interface{}{
			"prompt":     s.Prompt,
			"similarity": s.Similarity,
			"hits":       s.Hits,
			"created_at": s.CreatedAt.Unix(),
		}
	}

	return result, nil
}
