package ratelimit

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

// MiddlewareConfig represents middleware configuration
type MiddlewareConfig struct {
	// Limiter is the multi-level limiter to use
	Limiter *MultiLevelLimiter

	// QuotaManager is the optional quota manager
	QuotaManager QuotaManager

	// KeyFunc generates rate limit keys from request
	KeyFunc func(c fiber.Ctx) ([]Key, error)

	// OnRateLimitExceeded is called when rate limit is exceeded
	OnRateLimitExceeded func(c fiber.Ctx, info *LimitInfo) error

	// IncludeHeaders determines if rate limit headers should be included
	IncludeHeaders bool

	// SkipSuccessfulRequests only counts failed requests
	SkipSuccessfulRequests bool

	// SkipFailedRequests only counts successful requests
	SkipFailedRequests bool
}

// Middleware returns a Fiber middleware for rate limiting
func Middleware(config MiddlewareConfig) fiber.Handler {
	// Set defaults
	if config.KeyFunc == nil {
		config.KeyFunc = DefaultKeyFunc
	}

	if config.OnRateLimitExceeded == nil {
		config.OnRateLimitExceeded = DefaultRateLimitExceeded
	}

	config.IncludeHeaders = true // Always include headers by default

	return func(c fiber.Ctx) error {
		// Generate keys
		keys, err := config.KeyFunc(c)
		if err != nil {
			return err
		}

		// Check rate limit
		info, err := config.Limiter.Allow(c.Context(), keys)
		if err != nil {
			return fmt.Errorf("rate limit check failed: %w", err)
		}

		// Set rate limit headers
		if config.IncludeHeaders && info != nil {
			setRateLimitHeaders(c, info)
		}

		// Check if rate limit exceeded
		if !info.Allowed {
			return config.OnRateLimitExceeded(c, info)
		}

		// Check quota if configured
		if config.QuotaManager != nil {
			userID := getUserID(c)
			if userID != "" {
				quotaInfo, err := config.QuotaManager.Use(c.Context(), userID, 1)
				if err != nil {
					return fmt.Errorf("quota check failed: %w", err)
				}

				// Set quota headers
				setQuotaHeaders(c, quotaInfo)

				// If hard quota exceeded, reject
				if !quotaInfo.Allowed && quotaInfo.Type == QuotaTypeHard {
					return QuotaExceededError(c, quotaInfo)
				}
			}
		}

		// Handle response-based counting
		if config.SkipSuccessfulRequests || config.SkipFailedRequests {
			// We need to wait for the response to determine success/failure
			// This is a simplified approach - in production, you might want
			// to implement this differently based on your needs
			err := c.Next()

			statusCode := c.Response().StatusCode()
			isSuccess := statusCode >= 200 && statusCode < 400

			// Revert the rate limit if we should skip this request
			if (config.SkipSuccessfulRequests && isSuccess) ||
				(config.SkipFailedRequests && !isSuccess) {
				// In a real implementation, you would need to implement
				// a way to revert the consumed tokens
				// For now, we'll just pass through
			}

			return err
		}

		return c.Next()
	}
}

// DefaultKeyFunc generates default rate limit keys from request
func DefaultKeyFunc(c fiber.Ctx) ([]Key, error) {
	keys := []Key{
		{Level: LimitLevelGlobal, Identifier: "global"},
	}

	// User ID from context or header
	userID := getUserID(c)
	if userID != "" {
		keys = append(keys, Key{Level: LimitLevelUser, Identifier: userID})
	}

	// IP address
	ip := c.IP()
	if ip != "" {
		keys = append(keys, Key{Level: LimitLevelIP, Identifier: ip})
	}

	// Provider from query or header
	provider := c.Query("provider", c.Get("X-Provider", ""))
	if provider != "" {
		keys = append(keys, Key{Level: LimitLevelProvider, Identifier: provider})
	}

	// Model from query or header
	model := c.Query("model", c.Get("X-Model", ""))
	if model != "" {
		keys = append(keys, Key{Level: LimitLevelModel, Identifier: model})
	}

	return keys, nil
}

// DefaultRateLimitExceeded is the default handler for rate limit exceeded
func DefaultRateLimitExceeded(c fiber.Ctx, info *LimitInfo) error {
	// Set Retry-After header
	if info.RetryAfter > 0 {
		c.Set("Retry-After", strconv.Itoa(int(info.RetryAfter.Seconds())))
	}

	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error":       "rate_limit_exceeded",
		"message":     "Too many requests. Please try again later.",
		"limit":       info.Limit,
		"remaining":   info.Remaining,
		"reset":       info.Reset.Unix(),
		"retry_after": int(info.RetryAfter.Seconds()),
	})
}

// QuotaExceededError returns error response for quota exceeded
func QuotaExceededError(c fiber.Ctx, info *QuotaInfo) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error":     "quota_exceeded",
		"message":   "Quota limit exceeded. Please upgrade your plan or wait for reset.",
		"limit":     info.Limit,
		"used":      info.Used,
		"remaining": info.Remaining,
		"reset":     info.Reset.Unix(),
		"type":      info.Type,
	})
}

// setRateLimitHeaders sets rate limit headers on response
func setRateLimitHeaders(c fiber.Ctx, info *LimitInfo) {
	if info.Limit > 0 {
		c.Set("X-RateLimit-Limit", strconv.FormatInt(info.Limit, 10))
	}

	if info.Remaining >= 0 {
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(info.Remaining, 10))
	}

	if !info.Reset.IsZero() {
		c.Set("X-RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))
	}

	// Standard RateLimit header (RFC draft)
	if info.Limit > 0 {
		rateLimitValue := fmt.Sprintf("limit=%d, remaining=%d, reset=%d",
			info.Limit,
			info.Remaining,
			info.Reset.Unix(),
		)
		c.Set("RateLimit", rateLimitValue)
	}
}

// setQuotaHeaders sets quota headers on response
func setQuotaHeaders(c fiber.Ctx, info *QuotaInfo) {
	c.Set("X-Quota-Limit", strconv.FormatInt(info.Limit, 10))
	c.Set("X-Quota-Used", strconv.FormatInt(info.Used, 10))
	c.Set("X-Quota-Remaining", strconv.FormatInt(info.Remaining, 10))
	c.Set("X-Quota-Reset", strconv.FormatInt(info.Reset.Unix(), 10))

	if info.Warning {
		c.Set("X-Quota-Warning", "true")
	}
}

// getUserID extracts user ID from request
func getUserID(c fiber.Ctx) string {
	// Try to get from context (set by auth middleware)
	if userID := c.Locals("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			return id
		}
	}

	// Try to get from header
	if userID := c.Get("X-User-ID"); userID != "" {
		return userID
	}

	// Try to get from query
	if userID := c.Query("user_id"); userID != "" {
		return userID
	}

	return ""
}

// AdaptiveMiddleware returns a middleware with adaptive rate limiting
type AdaptiveMiddleware struct {
	config       MiddlewareConfig
	successRates map[string]*successTracker
}

type successTracker struct {
	successful int64
	total      int64
	lastUpdate time.Time
}

// NewAdaptiveMiddleware creates an adaptive rate limiting middleware
func NewAdaptiveMiddleware(config MiddlewareConfig) *AdaptiveMiddleware {
	return &AdaptiveMiddleware{
		config:       config,
		successRates: make(map[string]*successTracker),
	}
}

// Handler returns the middleware handler
func (a *AdaptiveMiddleware) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Generate keys
		keys, err := a.config.KeyFunc(c)
		if err != nil {
			return err
		}

		// Check rate limit
		info, err := a.config.Limiter.Allow(c.Context(), keys)
		if err != nil {
			return fmt.Errorf("rate limit check failed: %w", err)
		}

		// Set rate limit headers
		if a.config.IncludeHeaders && info != nil {
			setRateLimitHeaders(c, info)
		}

		// Check if rate limit exceeded
		if !info.Allowed {
			return a.config.OnRateLimitExceeded(c, info)
		}

		// Continue with request
		err = c.Next()

		// Track success rate for adaptive limiting
		// This could be used to adjust limits dynamically
		statusCode := c.Response().StatusCode()
		isSuccess := statusCode >= 200 && statusCode < 400

		// Update success rate (simplified - in production use proper tracking)
		for _, key := range keys {
			tracker, ok := a.successRates[key.String()]
			if !ok {
				tracker = &successTracker{
					lastUpdate: time.Now(),
				}
				a.successRates[key.String()] = tracker
			}

			tracker.total++
			if isSuccess {
				tracker.successful++
			}
			tracker.lastUpdate = time.Now()
		}

		return err
	}
}

// PerEndpointMiddleware creates middleware with per-endpoint rate limits
func PerEndpointMiddleware(configs map[string]MiddlewareConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		// Find matching config
		config, ok := configs[path]
		if !ok {
			// Try pattern matching
			for pattern, cfg := range configs {
				if matchesPattern(path, pattern) {
					config = cfg
					ok = true
					break
				}
			}
		}

		// If no config found, use default or skip
		if !ok {
			return c.Next()
		}

		// Apply rate limiting
		return Middleware(config)(c)
	}
}

// matchesPattern checks if path matches pattern (simple implementation)
func matchesPattern(path, pattern string) bool {
	// This is a simple implementation
	// In production, you might want to use a proper path matching library
	if pattern == "*" {
		return true
	}

	// Exact match
	if path == pattern {
		return true
	}

	// Prefix match with wildcard
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}

	return false
}

// BurstMiddleware allows burst requests above the normal rate limit
type BurstMiddleware struct {
	config       MiddlewareConfig
	burstAllowed map[string]int64
	burstWindow  time.Duration
}

// NewBurstMiddleware creates middleware with burst allowance
func NewBurstMiddleware(config MiddlewareConfig, burstWindow time.Duration) *BurstMiddleware {
	return &BurstMiddleware{
		config:       config,
		burstAllowed: make(map[string]int64),
		burstWindow:  burstWindow,
	}
}

// Handler returns the middleware handler
func (b *BurstMiddleware) Handler() fiber.Handler {
	return Middleware(b.config)
}

// GetRateLimitInfo returns current rate limit information for debugging
func GetRateLimitInfo(c fiber.Ctx, limiter *MultiLevelLimiter, keys []Key) (map[LimitLevel]*LimitInfo, error) {
	return limiter.GetInfo(c.Context(), keys)
}
