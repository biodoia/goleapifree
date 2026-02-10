package main

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/plugins"
	"github.com/gofiber/fiber/v3"
)

// RateLimiterPlugin implementa un middleware di rate limiting
type RateLimiterPlugin struct {
	name        string
	version     string
	description string
	logger      plugins.Logger
	config      map[string]interface{}

	// Rate limiter state
	requests map[string]*rateLimitEntry
	mu       sync.RWMutex
	ticker   *time.Ticker
	stopCh   chan struct{}
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewPlugin factory function
func NewPlugin() (plugins.Plugin, error) {
	return &RateLimiterPlugin{
		name:        "rate-limiter",
		version:     "1.0.0",
		description: "Rate limiting middleware plugin",
		requests:    make(map[string]*rateLimitEntry),
		stopCh:      make(chan struct{}),
	}, nil
}

func (p *RateLimiterPlugin) Name() string        { return p.name }
func (p *RateLimiterPlugin) Version() string     { return p.version }
func (p *RateLimiterPlugin) Description() string { return p.description }
func (p *RateLimiterPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeMiddleware
}

func (p *RateLimiterPlugin) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"author":     "GoLeapAI Team",
		"license":    "MIT",
		"category":   "security",
		"features":   []string{"rate-limiting", "ddos-protection"},
		"applies_to": []string{"/v1/*"},
	}
}

func (p *RateLimiterPlugin) Init(ctx context.Context, deps *plugins.Dependencies) error {
	p.logger = deps.Logger
	p.config = deps.Config

	p.logger.Info("Initializing rate limiter plugin", map[string]interface{}{
		"name":    p.name,
		"version": p.version,
	})

	// Avvia cleanup periodico
	p.ticker = time.NewTicker(1 * time.Minute)
	go p.cleanupLoop()

	return nil
}

func (p *RateLimiterPlugin) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down rate limiter plugin", nil)

	if p.ticker != nil {
		p.ticker.Stop()
	}
	close(p.stopCh)

	return nil
}

// Handler implementa MiddlewarePlugin
func (p *RateLimiterPlugin) Handler() fiber.Handler {
	// Configurazione
	maxRequests := plugins.GetInt(p.config, "max_requests", 100)
	windowDuration := plugins.GetDuration(p.config, "window", 1*time.Minute)
	identifier := plugins.GetString(p.config, "identifier", "ip") // ip, api_key, user_id

	return func(c fiber.Ctx) error {
		// Identifica il client
		var clientID string
		switch identifier {
		case "ip":
			clientID = c.IP()
		case "api_key":
			clientID = c.Get("X-API-Key")
		case "user_id":
			clientID = c.Get("X-User-ID")
		default:
			clientID = c.IP()
		}

		if clientID == "" {
			clientID = "anonymous"
		}

		// Verifica rate limit
		allowed, remaining, resetTime := p.checkRateLimit(clientID, maxRequests, windowDuration)

		// Imposta header di rate limit
		c.Set("X-RateLimit-Limit", fiber.IntToString(maxRequests))
		c.Set("X-RateLimit-Remaining", fiber.IntToString(remaining))
		c.Set("X-RateLimit-Reset", fiber.IntToString(int(resetTime.Unix())))

		if !allowed {
			p.logger.Warn("Rate limit exceeded", map[string]interface{}{
				"client_id": clientID,
				"limit":     maxRequests,
			})

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":   "rate_limit_exceeded",
				"message": "Too many requests, please try again later",
				"retry_after": int(time.Until(resetTime).Seconds()),
			})
		}

		// Log richiesta (opzionale)
		p.logger.Debug("Request allowed", map[string]interface{}{
			"client_id": clientID,
			"remaining": remaining,
		})

		return c.Next()
	}
}

// Priority implementa MiddlewarePlugin
func (p *RateLimiterPlugin) Priority() int {
	return plugins.GetInt(p.config, "priority", 80) // Alta priorità
}

// ApplyRoutes implementa MiddlewarePlugin
func (p *RateLimiterPlugin) ApplyRoutes() []string {
	// Se configurato, restituisce le route specifiche
	if routes, ok := p.config["routes"].([]string); ok {
		return routes
	}

	// Default: applica a tutte le route API
	return []string{"/v1/*", "/api/*"}
}

// checkRateLimit verifica se il client può fare una richiesta
func (p *RateLimiterPlugin) checkRateLimit(clientID string, maxRequests int, window time.Duration) (allowed bool, remaining int, resetTime time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	entry, exists := p.requests[clientID]
	if !exists || now.After(entry.resetTime) {
		// Nuova entry o finestra scaduta
		entry = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(window),
		}
		p.requests[clientID] = entry
		return true, maxRequests - 1, entry.resetTime
	}

	// Incrementa counter
	entry.count++

	if entry.count > maxRequests {
		return false, 0, entry.resetTime
	}

	return true, maxRequests - entry.count, entry.resetTime
}

// cleanupLoop rimuove entry scadute
func (p *RateLimiterPlugin) cleanupLoop() {
	for {
		select {
		case <-p.ticker.C:
			p.cleanup()
		case <-p.stopCh:
			return
		}
	}
}

// cleanup rimuove entry scadute
func (p *RateLimiterPlugin) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	deleted := 0

	for clientID, entry := range p.requests {
		if now.After(entry.resetTime) {
			delete(p.requests, clientID)
			deleted++
		}
	}

	if deleted > 0 {
		p.logger.Debug("Cleaned up expired rate limit entries", map[string]interface{}{
			"deleted": deleted,
			"active":  len(p.requests),
		})
	}
}

// GetStats restituisce statistiche del rate limiter
func (p *RateLimiterPlugin) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"active_clients": len(p.requests),
		"total_entries":  len(p.requests),
	}
}
