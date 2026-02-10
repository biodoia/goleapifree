package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// CacheMiddleware integra semantic caching con il sistema di cache esistente
type CacheMiddleware struct {
	semanticCache *SemanticCache
	fallbackCache Cache
	enabled       bool
}

// Cache interfaccia per il fallback cache (compatibile con pkg/cache)
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CacheMiddlewareConfig configurazione per il middleware
type CacheMiddlewareConfig struct {
	Enabled       bool
	FallbackCache Cache
}

// NewCacheMiddleware crea un nuovo middleware con semantic cache
func NewCacheMiddleware(semanticCache *SemanticCache, config *CacheMiddlewareConfig) *CacheMiddleware {
	if config == nil {
		config = &CacheMiddlewareConfig{
			Enabled: true,
		}
	}

	return &CacheMiddleware{
		semanticCache: semanticCache,
		fallbackCache: config.FallbackCache,
		enabled:       config.Enabled,
	}
}

// GetOrCompute cerca nel semantic cache, altrimenti esegue compute function
func (m *CacheMiddleware) GetOrCompute(
	ctx context.Context,
	prompt string,
	ttl time.Duration,
	compute func(context.Context) (interface{}, error),
) (interface{}, error) {
	if !m.enabled {
		return compute(ctx)
	}

	// Cerca nel semantic cache
	if cached, found, err := m.semanticCache.Get(ctx, prompt); err == nil && found {
		log.Debug().
			Str("prompt", prompt[:min(50, len(prompt))]).
			Msg("Semantic cache hit in middleware")
		return cached, nil
	}

	// Cache miss - esegui compute
	result, err := compute(ctx)
	if err != nil {
		return nil, err
	}

	// Salva nel semantic cache
	if err := m.semanticCache.Set(ctx, prompt, result, ttl); err != nil {
		log.Warn().Err(err).Msg("Failed to cache result in semantic cache")
	}

	return result, nil
}

// GetPromptResponse cerca una risposta cachata per un prompt
func (m *CacheMiddleware) GetPromptResponse(ctx context.Context, prompt string) (interface{}, bool, error) {
	if !m.enabled {
		return nil, false, nil
	}

	return m.semanticCache.Get(ctx, prompt)
}

// CachePromptResponse salva una risposta per un prompt
func (m *CacheMiddleware) CachePromptResponse(ctx context.Context, prompt string, response interface{}, ttl time.Duration) error {
	if !m.enabled {
		return nil
	}

	return m.semanticCache.Set(ctx, prompt, response, ttl)
}

// InvalidateSimilar invalida tutte le risposte simili a un prompt
func (m *CacheMiddleware) InvalidateSimilar(ctx context.Context, prompt string, threshold float64) (int, error) {
	if !m.enabled {
		return 0, nil
	}

	// Trova prompts simili
	embedding, err := m.semanticCache.generator.Generate(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("failed to generate embedding: %w", err)
	}

	results, err := m.semanticCache.vectorStore.Search(ctx, embedding, 100, threshold)
	if err != nil {
		return 0, fmt.Errorf("failed to search vectors: %w", err)
	}

	// Rimuovi tutti i risultati simili
	count := 0
	for _, result := range results {
		if err := m.semanticCache.Delete(ctx, result.ID); err == nil {
			count++
		}
	}

	log.Info().
		Str("prompt", prompt[:min(50, len(prompt))]).
		Int("invalidated", count).
		Msg("Invalidated similar cache entries")

	return count, nil
}

// Stats restituisce statistiche combinate
func (m *CacheMiddleware) Stats() MiddlewareStats {
	semanticStats := m.semanticCache.Stats()

	return MiddlewareStats{
		SemanticCache: semanticStats,
		Enabled:       m.enabled,
		Size:          m.semanticCache.Size(),
	}
}

// MiddlewareStats statistiche del middleware
type MiddlewareStats struct {
	SemanticCache SemanticCacheStats
	Enabled       bool
	Size          int
}

// CompletionRequest rappresenta una richiesta di completion (per integrazione)
type CompletionRequest struct {
	Prompt      string                 `json:"prompt"`
	Model       string                 `json:"model"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"max_tokens"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// CompletionResponse rappresenta una risposta di completion
type CompletionResponse struct {
	Text      string                 `json:"text"`
	Model     string                 `json:"model"`
	TokensIn  int                    `json:"tokens_in"`
	TokensOut int                    `json:"tokens_out"`
	Cached    bool                   `json:"cached"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// HashRequest genera un hash per una richiesta (per deduplication)
func HashRequest(req *CompletionRequest) string {
	data, _ := json.Marshal(map[string]interface{}{
		"prompt":      req.Prompt,
		"model":       req.Model,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	})
	return fmt.Sprintf("%x", data)
}

// DeduplicateRequests deduplica richieste simili usando semantic similarity
func (m *CacheMiddleware) DeduplicateRequests(ctx context.Context, requests []*CompletionRequest, threshold float64) ([]RequestGroup, error) {
	if !m.enabled || len(requests) == 0 {
		// Nessuna deduplica, ogni richiesta è unica
		groups := make([]RequestGroup, len(requests))
		for i, req := range requests {
			groups[i] = RequestGroup{
				Representative: req,
				Duplicates:     []*CompletionRequest{req},
				Count:          1,
			}
		}
		return groups, nil
	}

	// Genera embeddings per tutti i prompts
	prompts := make([]string, len(requests))
	for i, req := range requests {
		prompts[i] = req.Prompt
	}

	embeddings, err := m.semanticCache.generator.GenerateBatch(ctx, prompts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Raggruppa richieste simili
	groups := make([]RequestGroup, 0)
	used := make(map[int]bool)

	for i := 0; i < len(requests); i++ {
		if used[i] {
			continue
		}

		group := RequestGroup{
			Representative: requests[i],
			Duplicates:     []*CompletionRequest{requests[i]},
			Count:          1,
		}
		used[i] = true

		// Trova richieste simili
		for j := i + 1; j < len(requests); j++ {
			if used[j] {
				continue
			}

			// Calcola similarità
			similarity := CosineSimilarity(embeddings[i], embeddings[j])
			if similarity >= threshold {
				group.Duplicates = append(group.Duplicates, requests[j])
				group.Count++
				used[j] = true
			}
		}

		groups = append(groups, group)
	}

	log.Debug().
		Int("original", len(requests)).
		Int("groups", len(groups)).
		Float64("threshold", threshold).
		Msg("Deduplicated requests")

	return groups, nil
}

// RequestGroup rappresenta un gruppo di richieste duplicate
type RequestGroup struct {
	Representative *CompletionRequest
	Duplicates     []*CompletionRequest
	Count          int
}

// SemanticDeduplication esegue deduplication semantica su batch di richieste
type SemanticDeduplication struct {
	middleware *CacheMiddleware
	threshold  float64
}

// NewSemanticDeduplication crea un nuovo deduplicator
func NewSemanticDeduplication(middleware *CacheMiddleware, threshold float64) *SemanticDeduplication {
	return &SemanticDeduplication{
		middleware: middleware,
		threshold:  threshold,
	}
}

// Process processa un batch di richieste con deduplication
func (sd *SemanticDeduplication) Process(ctx context.Context, requests []*CompletionRequest) ([]*CompletionResponse, error) {
	// Deduplica richieste
	groups, err := sd.middleware.DeduplicateRequests(ctx, requests, sd.threshold)
	if err != nil {
		return nil, err
	}

	// Mappa per risultati
	results := make(map[*CompletionRequest]*CompletionResponse)

	// Processa ogni gruppo
	for _, group := range groups {
		// Cerca nel cache
		cached, found, err := sd.middleware.GetPromptResponse(ctx, group.Representative.Prompt)
		if err == nil && found {
			// Cache hit - usa per tutte le duplicate
			if resp, ok := cached.(*CompletionResponse); ok {
				resp.Cached = true
				for _, req := range group.Duplicates {
					results[req] = resp
				}
				continue
			}
		}

		// Cache miss - qui dovrebbe essere chiamata la vera API
		// Per ora creiamo una risposta placeholder
		resp := &CompletionResponse{
			Text:      "Response for: " + group.Representative.Prompt,
			Model:     group.Representative.Model,
			TokensIn:  len(group.Representative.Prompt) / 4,
			TokensOut: 100,
			Cached:    false,
		}

		// Cache la risposta
		_ = sd.middleware.CachePromptResponse(ctx, group.Representative.Prompt, resp, 10*time.Minute)

		// Assegna a tutte le duplicate
		for _, req := range group.Duplicates {
			results[req] = resp
		}
	}

	// Costruisci array di risposte nell'ordine originale
	responses := make([]*CompletionResponse, len(requests))
	for i, req := range requests {
		responses[i] = results[req]
	}

	log.Info().
		Int("requests", len(requests)).
		Int("unique", len(groups)).
		Msg("Processed batch with semantic deduplication")

	return responses, nil
}

// Warmup pre-carica il cache con richieste comuni
func (m *CacheMiddleware) Warmup(ctx context.Context, prompts []struct {
	Prompt   string
	Response interface{}
	TTL      time.Duration
}) error {
	if !m.enabled {
		return nil
	}

	for _, item := range prompts {
		if err := m.semanticCache.Set(ctx, item.Prompt, item.Response, item.TTL); err != nil {
			log.Warn().
				Err(err).
				Str("prompt", item.Prompt[:min(50, len(item.Prompt))]).
				Msg("Failed to warmup cache")
		}
	}

	log.Info().
		Int("count", len(prompts)).
		Msg("Cache warmed up")

	return nil
}

// Enable abilita il middleware
func (m *CacheMiddleware) Enable() {
	m.enabled = true
	log.Info().Msg("Semantic cache middleware enabled")
}

// Disable disabilita il middleware
func (m *CacheMiddleware) Disable() {
	m.enabled = false
	log.Info().Msg("Semantic cache middleware disabled")
}

// IsEnabled verifica se il middleware è abilitato
func (m *CacheMiddleware) IsEnabled() bool {
	return m.enabled
}

// Clear svuota il cache
func (m *CacheMiddleware) Clear(ctx context.Context) error {
	return m.semanticCache.Clear(ctx)
}
