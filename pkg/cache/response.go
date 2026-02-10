package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ResponseCache implementa caching per chat completion responses
type ResponseCache struct {
	cache             Cache
	semanticCache     *SemanticCache
	compressionMinSize int // Comprimi se > questo size (bytes)
	defaultTTL        time.Duration
	mu                sync.RWMutex
	stats             ResponseCacheStats
	useCompression    bool
	useSemanticCache  bool
}

// ResponseCacheStats statistiche per response cache
type ResponseCacheStats struct {
	CacheStats
	CompressedSets     int64
	DecompressionCount int64
	CompressionRatio   float64
	AvgResponseSize    int64
	LargeResponses     int64 // Responses > 10KB
}

// ResponseCacheConfig configurazione per response cache
type ResponseCacheConfig struct {
	BaseCache          Cache
	SemanticCache      *SemanticCache
	DefaultTTL         time.Duration
	CompressionMinSize int  // Default: 1024 (1KB)
	UseCompression     bool // Default: true
	UseSemanticCache   bool // Default: false
}

// CachedResponse rappresenta una response cachata
type CachedResponse struct {
	Model        string                 `json:"model"`
	Messages     []Message              `json:"messages"`
	Parameters   map[string]interface{} `json:"parameters"`
	Response     json.RawMessage        `json:"response"`
	Compressed   bool                   `json:"compressed"`
	CachedAt     time.Time              `json:"cached_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	ResponseSize int64                  `json:"response_size"`
	CacheKey     string                 `json:"cache_key"`
}

// Message rappresenta un messaggio nella conversazione
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest rappresenta una richiesta di chat completion
type ChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []Message              `json:"messages"`
	Temperature float64                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"-"` // Altri parametri
}

// NewResponseCache crea un nuovo response cache
func NewResponseCache(config *ResponseCacheConfig) (*ResponseCache, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	if config.DefaultTTL == 0 {
		config.DefaultTTL = 30 * time.Minute
	}

	if config.CompressionMinSize == 0 {
		config.CompressionMinSize = 1024 // 1KB
	}

	rc := &ResponseCache{
		cache:              config.BaseCache,
		semanticCache:      config.SemanticCache,
		compressionMinSize: config.CompressionMinSize,
		defaultTTL:         config.DefaultTTL,
		useCompression:     config.UseCompression,
		useSemanticCache:   config.UseSemanticCache && config.SemanticCache != nil,
		stats:              ResponseCacheStats{},
	}

	log.Info().
		Dur("ttl", config.DefaultTTL).
		Int("compression_min_size", config.CompressionMinSize).
		Bool("use_compression", config.UseCompression).
		Bool("use_semantic", rc.useSemanticCache).
		Msg("Response cache initialized")

	return rc, nil
}

// Get recupera una response dal cache
func (r *ResponseCache) Get(ctx context.Context, req *ChatCompletionRequest) (*CachedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Genera cache key
	cacheKey := r.generateCacheKey(req)

	// Prova semantic cache se abilitato
	if r.useSemanticCache {
		// Usa solo l'ultimo messaggio user per semantic match
		lastUserMsg := r.getLastUserMessage(req.Messages)
		if lastUserMsg != "" {
			data, err := r.semanticCache.Get(ctx, lastUserMsg)
			if err == nil {
				r.stats.Hits++
				log.Debug().
					Str("method", "semantic").
					Str("key", cacheKey).
					Msg("Response cache hit (semantic)")
				return r.deserializeResponse(data)
			}
		}
	}

	// Prova exact match cache
	data, err := r.cache.Get(ctx, cacheKey)
	if err != nil {
		r.stats.Misses++
		return nil, ErrCacheMiss
	}

	r.stats.Hits++
	log.Debug().
		Str("method", "exact").
		Str("key", cacheKey).
		Msg("Response cache hit")

	return r.deserializeResponse(data)
}

// Set salva una response nel cache
func (r *ResponseCache) Set(ctx context.Context, req *ChatCompletionRequest, response interface{}, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ttl == 0 {
		ttl = r.defaultTTL
	}

	// Serializza response
	responseData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	responseSize := int64(len(responseData))
	r.stats.AvgResponseSize = (r.stats.AvgResponseSize + responseSize) / 2
	if responseSize > 10*1024 {
		r.stats.LargeResponses++
	}

	// Crea cached response
	cached := &CachedResponse{
		Model:        req.Model,
		Messages:     req.Messages,
		Parameters:   r.extractParameters(req),
		Response:     responseData,
		Compressed:   false,
		CachedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(ttl),
		ResponseSize: responseSize,
		CacheKey:     r.generateCacheKey(req),
	}

	// Serializza
	data, err := r.serializeResponse(cached)
	if err != nil {
		return err
	}

	// Salva in cache principale
	cacheKey := r.generateCacheKey(req)
	if err := r.cache.Set(ctx, cacheKey, data, ttl); err != nil {
		return err
	}

	// Salva in semantic cache se abilitato
	if r.useSemanticCache {
		lastUserMsg := r.getLastUserMessage(req.Messages)
		if lastUserMsg != "" {
			_ = r.semanticCache.Set(ctx, lastUserMsg, data, ttl)
		}
	}

	r.stats.Sets++

	log.Debug().
		Str("key", cacheKey).
		Int64("size", responseSize).
		Bool("compressed", cached.Compressed).
		Dur("ttl", ttl).
		Msg("Response cached")

	return nil
}

// Delete rimuove una response dal cache
func (r *ResponseCache) Delete(ctx context.Context, req *ChatCompletionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cacheKey := r.generateCacheKey(req)
	r.stats.Deletes++

	return r.cache.Delete(ctx, cacheKey)
}

// Clear svuota il cache
func (r *ResponseCache) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.useSemanticCache {
		_ = r.semanticCache.Clear(ctx)
	}

	return r.cache.Clear(ctx)
}

// Stats restituisce le statistiche
func (r *ResponseCache) Stats() CacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats.CacheStats
}

// ResponseStats restituisce statistiche estese
func (r *ResponseCache) ResponseStats() ResponseCacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// generateCacheKey genera una chiave di cache univoca per la request
func (r *ResponseCache) generateCacheKey(req *ChatCompletionRequest) string {
	return HashKey(
		req.Model,
		req.Messages,
		req.Temperature,
		req.MaxTokens,
		req.TopP,
		req.Parameters,
	)
}

// extractParameters estrae parametri rilevanti dalla request
func (r *ResponseCache) extractParameters(req *ChatCompletionRequest) map[string]interface{} {
	params := make(map[string]interface{})

	if req.Temperature != 0 {
		params["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		params["max_tokens"] = req.MaxTokens
	}
	if req.TopP != 0 {
		params["top_p"] = req.TopP
	}

	// Aggiungi altri parametri custom
	for k, v := range req.Parameters {
		params[k] = v
	}

	return params
}

// getLastUserMessage estrae l'ultimo messaggio dell'utente
func (r *ResponseCache) getLastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// serializeResponse serializza e comprime una cached response
func (r *ResponseCache) serializeResponse(cached *CachedResponse) ([]byte, error) {
	// Serializza in JSON
	data, err := json.Marshal(cached)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cached response: %w", err)
	}

	// Comprimi se necessario
	if r.useCompression && len(data) > r.compressionMinSize {
		compressed, err := r.compress(data)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to compress response, storing uncompressed")
			return data, nil
		}

		// Calcola compression ratio
		ratio := float64(len(compressed)) / float64(len(data))
		r.stats.CompressionRatio = (r.stats.CompressionRatio + ratio) / 2
		r.stats.CompressedSets++

		cached.Compressed = true

		log.Debug().
			Int("original", len(data)).
			Int("compressed", len(compressed)).
			Float64("ratio", ratio).
			Msg("Response compressed")

		return compressed, nil
	}

	return data, nil
}

// deserializeResponse deserializza e decomprime una cached response
func (r *ResponseCache) deserializeResponse(data []byte) (*CachedResponse, error) {
	// Prova a deserializzare direttamente
	var cached CachedResponse
	err := json.Unmarshal(data, &cached)

	// Se fallisce, prova decompressione
	if err != nil {
		decompressed, err := r.decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize response: %w", err)
		}

		r.stats.DecompressionCount++
		data = decompressed

		err = json.Unmarshal(data, &cached)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal decompressed response: %w", err)
		}
	}

	// Verifica scadenza
	if time.Now().After(cached.ExpiresAt) {
		return nil, ErrCacheMiss
	}

	return &cached, nil
}

// compress comprime i dati usando gzip
func (r *ResponseCache) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)

	_, err := w.Write(data)
	if err != nil {
		w.Close()
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decomprime i dati gzip
func (rc *ResponseCache) decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

// IsCacheable determina se una request è cacheable
func (r *ResponseCache) IsCacheable(req *ChatCompletionRequest) bool {
	// Non cachare stream requests
	if req.Stream {
		return false
	}

	// Non cachare se temperature è molto alta (risposte non deterministiche)
	if req.Temperature > 0.7 {
		return false
	}

	// Non cachare se non ci sono messaggi
	if len(req.Messages) == 0 {
		return false
	}

	return true
}

// PurgeExpired rimuove entry scadute (da chiamare periodicamente)
func (r *ResponseCache) PurgeExpired(ctx context.Context) error {
	// TODO: Implementare sweep delle entry scadute
	// Richiede supporto per iterazione delle chiavi nel base cache
	log.Debug().Msg("Purge expired responses (not yet implemented)")
	return nil
}

// GetMetrics restituisce metriche dettagliate
func (r *ResponseCache) GetMetrics() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"hit_rate":           r.stats.HitRate(),
		"total_hits":         r.stats.Hits,
		"total_misses":       r.stats.Misses,
		"total_sets":         r.stats.Sets,
		"compressed_sets":    r.stats.CompressedSets,
		"avg_response_size":  r.stats.AvgResponseSize,
		"large_responses":    r.stats.LargeResponses,
		"compression_ratio":  r.stats.CompressionRatio,
		"decompression_count": r.stats.DecompressionCount,
	}
}
