package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// CacheMiddlewareConfig configurazione per il middleware di cache
type CacheMiddlewareConfig struct {
	ResponseCache *ResponseCache
	Enabled       bool
	SkipPaths     []string // Paths da non cachare
	CacheTTL      time.Duration
	AddHeaders    bool // Aggiungi X-Cache-Hit header
}

// CacheMiddleware crea un middleware Fiber per il caching
func CacheMiddleware(config *CacheMiddlewareConfig) fiber.Handler {
	if config == nil || !config.Enabled || config.ResponseCache == nil {
		// Return no-op middleware se disabilitato
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	if config.CacheTTL == 0 {
		config.CacheTTL = 30 * time.Minute
	}

	log.Info().
		Dur("ttl", config.CacheTTL).
		Bool("add_headers", config.AddHeaders).
		Int("skip_paths", len(config.SkipPaths)).
		Msg("Cache middleware initialized")

	return func(c fiber.Ctx) error {
		// Skip se non è POST
		if c.Method() != fiber.MethodPost {
			return c.Next()
		}

		// Skip se path è nella skip list
		path := c.Path()
		for _, skipPath := range config.SkipPaths {
			if path == skipPath {
				return c.Next()
			}
		}

		// Skip se non è chat completion endpoint
		if path != "/v1/chat/completions" && path != "/v1/messages" {
			return c.Next()
		}

		// Parse request
		var req ChatCompletionRequest
		if err := c.Bind().Body(&req); err != nil {
			log.Debug().Err(err).Msg("Failed to parse request for caching")
			return c.Next()
		}

		// Verifica se è cacheable
		if !config.ResponseCache.IsCacheable(&req) {
			log.Debug().Msg("Request not cacheable")
			return c.Next()
		}

		ctx := context.Background()

		// Prova a recuperare dal cache
		cached, err := config.ResponseCache.Get(ctx, &req)
		if err == nil {
			// Cache hit!
			if config.AddHeaders {
				c.Set("X-Cache-Hit", "true")
				c.Set("X-Cache-Key", cached.CacheKey)
				c.Set("X-Cached-At", cached.CachedAt.Format(time.RFC3339))
			}

			log.Info().
				Str("path", path).
				Str("model", req.Model).
				Str("cache_key", cached.CacheKey).
				Msg("Cache hit - serving cached response")

			// Restituisci response cachata
			c.Set("Content-Type", "application/json")
			return c.Send(cached.Response)
		}

		// Cache miss - procedi con la richiesta originale
		if config.AddHeaders {
			c.Set("X-Cache-Hit", "false")
		}

		// Cattura response per cacharla
		// Salva il body originale per re-parsing
		bodyBytes := c.Body()

		// Processa la richiesta
		if err := c.Next(); err != nil {
			return err
		}

		// Cattura response solo se 200 OK
		if c.Response().StatusCode() == 200 {
			// Leggi response body
			responseBody := c.Response().Body()

			// Verifica che sia JSON valido
			var responseData interface{}
			if err := json.Unmarshal(responseBody, &responseData); err != nil {
				log.Debug().Err(err).Msg("Response not valid JSON, skipping cache")
				return nil
			}

			// Re-parse request (body è stato consumato)
			var reqForCache ChatCompletionRequest
			if err := json.Unmarshal(bodyBytes, &reqForCache); err != nil {
				log.Debug().Err(err).Msg("Failed to re-parse request for caching")
				return nil
			}

			// Salva nel cache
			if err := config.ResponseCache.Set(ctx, &reqForCache, responseData, config.CacheTTL); err != nil {
				log.Warn().Err(err).Msg("Failed to cache response")
			} else {
				log.Info().
					Str("path", path).
					Str("model", reqForCache.Model).
					Msg("Response cached")
			}
		}

		return nil
	}
}

// CacheStatsMiddleware middleware per esporre statistiche del cache
func CacheStatsMiddleware(responseCache *ResponseCache) fiber.Handler {
	return func(c fiber.Ctx) error {
		stats := responseCache.ResponseStats()
		metrics := responseCache.GetMetrics()

		return c.JSON(fiber.Map{
			"cache_stats": fiber.Map{
				"hits":         stats.Hits,
				"misses":       stats.Misses,
				"sets":         stats.Sets,
				"deletes":      stats.Deletes,
				"hit_rate":     stats.HitRate(),
				"size":         stats.Size,
			},
			"response_stats": fiber.Map{
				"compressed_sets":    stats.CompressedSets,
				"avg_response_size":  stats.AvgResponseSize,
				"large_responses":    stats.LargeResponses,
				"compression_ratio":  stats.CompressionRatio,
			},
			"metrics": metrics,
		})
	}
}

// CacheClearMiddleware middleware per svuotare il cache
func CacheClearMiddleware(responseCache *ResponseCache) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()

		if err := responseCache.Clear(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to clear cache",
			})
		}

		log.Info().Msg("Cache cleared via middleware")

		return c.JSON(fiber.Map{
			"message": "Cache cleared successfully",
		})
	}
}

// CacheKeyMiddleware middleware per generare e esporre cache key
func CacheKeyMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Parse request
		var req ChatCompletionRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request",
			})
		}

		// Genera cache key
		rc := &ResponseCache{}
		cacheKey := rc.generateCacheKey(&req)

		return c.JSON(fiber.Map{
			"cache_key": cacheKey,
			"model":     req.Model,
			"messages":  len(req.Messages),
		})
	}
}

// InvalidateCacheMiddleware middleware per invalidare specifiche cache entries
func InvalidateCacheMiddleware(responseCache *ResponseCache) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Parse request
		var req ChatCompletionRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request",
			})
		}

		ctx := context.Background()

		if err := responseCache.Delete(ctx, &req); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to invalidate cache",
			})
		}

		log.Info().
			Str("model", req.Model).
			Msg("Cache entry invalidated")

		return c.JSON(fiber.Map{
			"message": "Cache entry invalidated successfully",
		})
	}
}

// RequestBodyCapture helper per catturare request body
type RequestBodyCapture struct {
	body []byte
}

// NewRequestBodyCapture crea un nuovo body capture
func NewRequestBodyCapture(c fiber.Ctx) (*RequestBodyCapture, error) {
	body := c.Body()
	capture := &RequestBodyCapture{
		body: make([]byte, len(body)),
	}
	copy(capture.body, body)
	return capture, nil
}

// Body restituisce il body catturato
func (r *RequestBodyCapture) Body() []byte {
	return r.body
}

// Reader restituisce un reader per il body
func (r *RequestBodyCapture) Reader() io.Reader {
	return bytes.NewReader(r.body)
}

// ResponseWriter wrapper per catturare response
type ResponseWriter struct {
	fiber.Ctx
	body       *bytes.Buffer
	statusCode int
}

// NewResponseWriter crea un nuovo response writer
func NewResponseWriter(c fiber.Ctx) *ResponseWriter {
	return &ResponseWriter{
		Ctx:        c,
		body:       &bytes.Buffer{},
		statusCode: 200,
	}
}

// Write implementa io.Writer
func (w *ResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

// Body restituisce il body catturato
func (w *ResponseWriter) Body() []byte {
	return w.body.Bytes()
}

// StatusCode restituisce lo status code
func (w *ResponseWriter) StatusCode() int {
	return w.statusCode
}

// CacheWarmupConfig configurazione per cache warmup
type CacheWarmupConfig struct {
	Requests      []*ChatCompletionRequest
	ResponseCache *ResponseCache
	TTL           time.Duration
}

// WarmupCache pre-popola il cache con richieste comuni
func WarmupCache(ctx context.Context, config *CacheWarmupConfig) error {
	if config == nil || config.ResponseCache == nil {
		return ErrInvalidConfig
	}

	log.Info().
		Int("requests", len(config.Requests)).
		Msg("Starting cache warmup")

	for i, req := range config.Requests {
		// Qui dovresti fare la richiesta reale all'LLM e cachare il risultato
		// Per ora loggiamo solo
		log.Debug().
			Int("index", i).
			Str("model", req.Model).
			Int("messages", len(req.Messages)).
			Msg("Warmup request (placeholder)")

		// TODO: Implementa chiamata reale e caching
		// response, err := callLLM(ctx, req)
		// if err != nil {
		// 	log.Warn().Err(err).Int("index", i).Msg("Warmup request failed")
		// 	continue
		// }
		//
		// if err := config.ResponseCache.Set(ctx, req, response, config.TTL); err != nil {
		// 	log.Warn().Err(err).Int("index", i).Msg("Failed to cache warmup response")
		// }
	}

	log.Info().Msg("Cache warmup completed")
	return nil
}

// CacheHealthCheck verifica lo stato del cache
func CacheHealthCheck(cache Cache) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Prova a scrivere e leggere
	testKey := "health_check_" + time.Now().Format("20060102150405")
	testValue := []byte("test")

	if err := cache.Set(ctx, testKey, testValue, 10*time.Second); err != nil {
		return fmt.Errorf("cache write failed: %w", err)
	}

	if _, err := cache.Get(ctx, testKey); err != nil {
		return fmt.Errorf("cache read failed: %w", err)
	}

	if err := cache.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("cache delete failed: %w", err)
	}

	return nil
}

// CacheMetricsCollector raccoglie metriche dal cache
type CacheMetricsCollector struct {
	cache         Cache
	responseCache *ResponseCache
	interval      time.Duration
	metrics       map[string]interface{}
	mu            sync.RWMutex
}

// NewCacheMetricsCollector crea un nuovo collector
func NewCacheMetricsCollector(cache Cache, responseCache *ResponseCache, interval time.Duration) *CacheMetricsCollector {
	return &CacheMetricsCollector{
		cache:         cache,
		responseCache: responseCache,
		interval:      interval,
		metrics:       make(map[string]interface{}),
	}
}

// Start inizia la raccolta metriche
func (c *CacheMetricsCollector) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for range ticker.C {
			c.collect()
		}
	}()
}

// collect raccoglie le metriche
func (c *CacheMetricsCollector) collect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := c.cache.Stats()

	c.metrics["timestamp"] = time.Now().Unix()
	c.metrics["hits"] = stats.Hits
	c.metrics["misses"] = stats.Misses
	c.metrics["hit_rate"] = stats.HitRate()
	c.metrics["sets"] = stats.Sets
	c.metrics["deletes"] = stats.Deletes
	c.metrics["size"] = stats.Size

	if c.responseCache != nil {
		c.metrics["response"] = c.responseCache.GetMetrics()
	}
}

// GetMetrics restituisce le metriche correnti
func (c *CacheMetricsCollector) GetMetrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Copia per evitare race conditions
	metrics := make(map[string]interface{})
	for k, v := range c.metrics {
		metrics[k] = v
	}

	return metrics
}
