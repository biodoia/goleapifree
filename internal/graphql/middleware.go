package graphql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// ================================================================================
// Authentication Middleware
// ================================================================================

// JWTAuthenticator validates JWT tokens
type JWTAuthenticator struct {
	secret []byte
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(secret string) *JWTAuthenticator {
	return &JWTAuthenticator{
		secret: []byte(secret),
	}
}

// Authenticate validates JWT token and adds user info to context
func (j *JWTAuthenticator) Authenticate(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return fmt.Errorf("missing authorization header")
	}

	// Strip "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Validate token (simplified - production should use proper JWT library)
	// For now, just check if token is non-empty
	if token == "" {
		return fmt.Errorf("invalid token")
	}

	// Add user info to context
	c.Locals("user_id", "authenticated-user")
	c.Locals("authenticated", true)

	return nil
}

// OptionalAuthenticator allows requests with or without authentication
type OptionalAuthenticator struct {
	authenticator AuthenticatorFunc
}

// NewOptionalAuthenticator creates an authenticator that doesn't fail on missing auth
func NewOptionalAuthenticator(auth AuthenticatorFunc) *OptionalAuthenticator {
	return &OptionalAuthenticator{
		authenticator: auth,
	}
}

// Authenticate attempts authentication but doesn't fail if missing
func (o *OptionalAuthenticator) Authenticate(c *fiber.Ctx) error {
	if err := o.authenticator(c); err != nil {
		// Set as unauthenticated but don't fail
		c.Locals("authenticated", false)
		return nil
	}

	c.Locals("authenticated", true)
	return nil
}

// ================================================================================
// Rate Limiting Middleware
// ================================================================================

// TokenBucketRateLimiter implements token bucket algorithm
type TokenBucketRateLimiter struct {
	capacity       int
	refillRate     int
	refillInterval time.Duration

	mu      sync.RWMutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens     int
	lastRefill time.Time
}

// NewTokenBucketRateLimiter creates a new rate limiter
func NewTokenBucketRateLimiter(capacity, refillRate int, refillInterval time.Duration) *TokenBucketRateLimiter {
	limiter := &TokenBucketRateLimiter{
		capacity:       capacity,
		refillRate:     refillRate,
		refillInterval: refillInterval,
		buckets:        make(map[string]*bucket),
	}

	// Start background cleanup
	go limiter.cleanup()

	return limiter
}

// Limit checks if request should be rate limited
func (r *TokenBucketRateLimiter) Limit(c *fiber.Ctx) error {
	// Get identifier (IP or user ID)
	identifier := c.IP()
	if userID := c.Locals("user_id"); userID != nil {
		identifier = fmt.Sprintf("user:%v", userID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create bucket
	b, exists := r.buckets[identifier]
	if !exists {
		b = &bucket{
			tokens:     r.capacity,
			lastRefill: time.Now(),
		}
		r.buckets[identifier] = b
	}

	// Refill tokens
	now := time.Now()
	timeSinceRefill := now.Sub(b.lastRefill)
	tokensToAdd := int(timeSinceRefill / r.refillInterval * time.Duration(r.refillRate))

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > r.capacity {
			b.tokens = r.capacity
		}
		b.lastRefill = now
	}

	// Check if request can proceed
	if b.tokens <= 0 {
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", r.capacity))
		c.Set("X-RateLimit-Remaining", "0")
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", b.lastRefill.Add(r.refillInterval).Unix()))

		return fmt.Errorf("rate limit exceeded")
	}

	// Consume token
	b.tokens--

	// Add rate limit headers
	c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", r.capacity))
	c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", b.tokens))

	return nil
}

func (r *TokenBucketRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		now := time.Now()

		for id, b := range r.buckets {
			if now.Sub(b.lastRefill) > 10*time.Minute {
				delete(r.buckets, id)
			}
		}

		r.mu.Unlock()
	}
}

// ================================================================================
// Complexity Limiting Middleware
// ================================================================================

// ComplexityLimiter limits query complexity to prevent DoS
type ComplexityLimiter struct {
	maxComplexity int
}

// NewComplexityLimiter creates a new complexity limiter
func NewComplexityLimiter(maxComplexity int) *ComplexityLimiter {
	return &ComplexityLimiter{
		maxComplexity: maxComplexity,
	}
}

// Limit checks query complexity
func (cl *ComplexityLimiter) Limit(c *fiber.Ctx, query string) error {
	// Simplified complexity calculation
	// Production should use proper GraphQL complexity analysis
	complexity := estimateComplexity(query)

	if complexity > cl.maxComplexity {
		return fmt.Errorf("query complexity %d exceeds maximum %d", complexity, cl.maxComplexity)
	}

	c.Locals("query_complexity", complexity)
	return nil
}

func estimateComplexity(query string) int {
	// Very basic estimation based on query length and nesting
	// Production should use proper AST analysis
	complexity := len(query) / 10

	// Penalize nested queries
	nestingLevel := 0
	maxNesting := 0

	for _, char := range query {
		if char == '{' {
			nestingLevel++
			if nestingLevel > maxNesting {
				maxNesting = nestingLevel
			}
		} else if char == '}' {
			nestingLevel--
		}
	}

	complexity += maxNesting * 10

	return complexity
}

// ================================================================================
// Logging Middleware
// ================================================================================

// QueryLogger logs GraphQL queries and their performance
type QueryLogger struct{}

// NewQueryLogger creates a new query logger
func NewQueryLogger() *QueryLogger {
	return &QueryLogger{}
}

// Log logs query execution
func (ql *QueryLogger) Log(c *fiber.Ctx, query string, duration time.Duration, err error) {
	event := log.Info()

	if err != nil {
		event = log.Error().Err(err)
	}

	event.
		Str("query", query).
		Dur("duration", duration).
		Str("ip", c.IP()).
		Int("complexity", c.Locals("query_complexity").(int)).
		Bool("authenticated", c.Locals("authenticated") != nil && c.Locals("authenticated").(bool)).
		Msg("GraphQL query executed")
}

// ================================================================================
// CORS Middleware
// ================================================================================

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
		},
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

// CORS middleware for GraphQL endpoint
func CORS(config *CORSConfig) fiber.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}

	return func(c *fiber.Ctx) error {
		// Set CORS headers
		origin := c.Get("Origin")
		if origin != "" {
			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range config.AllowOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				c.Set("Access-Control-Allow-Origin", origin)
			}
		}

		if config.AllowCredentials {
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Method() == "OPTIONS" {
			c.Set("Access-Control-Allow-Methods", join(config.AllowMethods, ", "))
			c.Set("Access-Control-Allow-Headers", join(config.AllowHeaders, ", "))
			c.Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// ================================================================================
// Context Enrichment Middleware
// ================================================================================

// ContextEnricher adds useful data to request context
type ContextEnricher struct{}

// NewContextEnricher creates a new context enricher
func NewContextEnricher() *ContextEnricher {
	return &ContextEnricher{}
}

// Enrich adds metadata to context
func (ce *ContextEnricher) Enrich(c *fiber.Ctx) error {
	// Create enriched context
	ctx := c.Context()

	// Add request metadata
	ctx = context.WithValue(ctx, "request_id", c.Get("X-Request-ID"))
	ctx = context.WithValue(ctx, "start_time", time.Now())
	ctx = context.WithValue(ctx, "ip", c.IP())
	ctx = context.WithValue(ctx, "user_agent", c.Get("User-Agent"))

	// Update context in fiber
	c.SetUserContext(ctx)

	return nil
}

// ================================================================================
// Utilities
// ================================================================================

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}

	return result
}
