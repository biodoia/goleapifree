package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LimitLevel represents different levels of rate limiting
type LimitLevel string

const (
	LimitLevelGlobal   LimitLevel = "global"
	LimitLevelUser     LimitLevel = "user"
	LimitLevelProvider LimitLevel = "provider"
	LimitLevelModel    LimitLevel = "model"
	LimitLevelIP       LimitLevel = "ip"
)

// Algorithm represents the rate limiting algorithm type
type Algorithm string

const (
	AlgorithmTokenBucket         Algorithm = "token_bucket"
	AlgorithmLeakyBucket         Algorithm = "leaky_bucket"
	AlgorithmFixedWindow         Algorithm = "fixed_window"
	AlgorithmSlidingWindowLog    Algorithm = "sliding_window_log"
	AlgorithmSlidingWindowCounter Algorithm = "sliding_window_counter"
)

// Config represents rate limit configuration
type Config struct {
	// Level of rate limiting
	Level LimitLevel

	// Algorithm to use
	Algorithm Algorithm

	// Rate limit (requests per window)
	Limit int64

	// Window duration
	Window time.Duration

	// Burst allowance (additional requests allowed in short bursts)
	Burst int64

	// Premium multiplier (multiplies limits for premium users)
	PremiumMultiplier float64

	// Enable distributed mode (using Redis)
	Distributed bool

	// Key prefix for Redis
	KeyPrefix string
}

// LimitInfo contains information about current limit state
type LimitInfo struct {
	// Allowed indicates if the request is allowed
	Allowed bool

	// Limit is the maximum number of requests allowed
	Limit int64

	// Remaining is the number of requests remaining
	Remaining int64

	// Reset is when the limit will reset
	Reset time.Time

	// RetryAfter is how long to wait before retrying (if not allowed)
	RetryAfter time.Duration
}

// Key represents a rate limit key
type Key struct {
	Level      LimitLevel
	Identifier string // user ID, provider name, model name, etc.
}

// String returns a string representation of the key
func (k Key) String() string {
	return fmt.Sprintf("%s:%s", k.Level, k.Identifier)
}

// Limiter is the main rate limiter interface
type Limiter interface {
	// Allow checks if a request is allowed and returns limit info
	Allow(ctx context.Context, key Key) (*LimitInfo, error)

	// AllowN checks if N requests are allowed
	AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error)

	// Reset resets the limit for a key
	Reset(ctx context.Context, key Key) error

	// GetInfo returns current limit info without consuming tokens
	GetInfo(ctx context.Context, key Key) (*LimitInfo, error)
}

// MultiLevelLimiter applies rate limiting at multiple levels
type MultiLevelLimiter struct {
	limiters map[LimitLevel]Limiter
	mu       sync.RWMutex

	// Override rules
	premiumUsers map[string]bool
	whitelisted  map[string]bool
}

// NewMultiLevelLimiter creates a new multi-level limiter
func NewMultiLevelLimiter() *MultiLevelLimiter {
	return &MultiLevelLimiter{
		limiters:     make(map[LimitLevel]Limiter),
		premiumUsers: make(map[string]bool),
		whitelisted:  make(map[string]bool),
	}
}

// AddLimiter adds a limiter for a specific level
func (m *MultiLevelLimiter) AddLimiter(level LimitLevel, limiter Limiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters[level] = limiter
}

// AddPremiumUser marks a user as premium
func (m *MultiLevelLimiter) AddPremiumUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.premiumUsers[userID] = true
}

// RemovePremiumUser removes premium status
func (m *MultiLevelLimiter) RemovePremiumUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.premiumUsers, userID)
}

// AddWhitelisted adds a whitelisted identifier
func (m *MultiLevelLimiter) AddWhitelisted(identifier string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.whitelisted[identifier] = true
}

// RemoveWhitelisted removes from whitelist
func (m *MultiLevelLimiter) RemoveWhitelisted(identifier string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.whitelisted, identifier)
}

// IsPremium checks if a user is premium
func (m *MultiLevelLimiter) IsPremium(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.premiumUsers[userID]
}

// IsWhitelisted checks if an identifier is whitelisted
func (m *MultiLevelLimiter) IsWhitelisted(identifier string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.whitelisted[identifier]
}

// Allow checks if a request is allowed at all levels
func (m *MultiLevelLimiter) Allow(ctx context.Context, keys []Key) (*LimitInfo, error) {
	return m.AllowN(ctx, keys, 1)
}

// AllowN checks if N requests are allowed at all levels
func (m *MultiLevelLimiter) AllowN(ctx context.Context, keys []Key, n int64) (*LimitInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if any key is whitelisted
	for _, key := range keys {
		if m.whitelisted[key.Identifier] {
			return &LimitInfo{
				Allowed:   true,
				Limit:     -1, // Unlimited
				Remaining: -1,
				Reset:     time.Time{},
			}, nil
		}
	}

	var mostRestrictive *LimitInfo

	// Check all levels
	for _, key := range keys {
		limiter, ok := m.limiters[key.Level]
		if !ok {
			continue
		}

		info, err := limiter.AllowN(ctx, key, n)
		if err != nil {
			return nil, fmt.Errorf("checking limit for %s: %w", key.String(), err)
		}

		// If not allowed, return immediately
		if !info.Allowed {
			return info, nil
		}

		// Track most restrictive limit
		if mostRestrictive == nil || info.Remaining < mostRestrictive.Remaining {
			mostRestrictive = info
		}
	}

	if mostRestrictive == nil {
		// No limiters configured, allow by default
		return &LimitInfo{
			Allowed:   true,
			Limit:     -1,
			Remaining: -1,
			Reset:     time.Time{},
		}, nil
	}

	return mostRestrictive, nil
}

// GetInfo returns limit info for all keys without consuming tokens
func (m *MultiLevelLimiter) GetInfo(ctx context.Context, keys []Key) (map[LimitLevel]*LimitInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[LimitLevel]*LimitInfo)

	for _, key := range keys {
		limiter, ok := m.limiters[key.Level]
		if !ok {
			continue
		}

		info, err := limiter.GetInfo(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("getting info for %s: %w", key.String(), err)
		}

		result[key.Level] = info
	}

	return result, nil
}

// Reset resets all limits for the given keys
func (m *MultiLevelLimiter) Reset(ctx context.Context, keys []Key) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range keys {
		limiter, ok := m.limiters[key.Level]
		if !ok {
			continue
		}

		if err := limiter.Reset(ctx, key); err != nil {
			return fmt.Errorf("resetting %s: %w", key.String(), err)
		}
	}

	return nil
}

// Request represents a rate limit request
type Request struct {
	UserID     string
	IP         string
	Provider   string
	Model      string
	IsPremium  bool
	TokenCount int64 // Number of tokens to consume
}

// Keys generates all rate limit keys for a request
func (r *Request) Keys() []Key {
	keys := []Key{
		{Level: LimitLevelGlobal, Identifier: "global"},
	}

	if r.UserID != "" {
		keys = append(keys, Key{Level: LimitLevelUser, Identifier: r.UserID})
	}

	if r.IP != "" {
		keys = append(keys, Key{Level: LimitLevelIP, Identifier: r.IP})
	}

	if r.Provider != "" {
		keys = append(keys, Key{Level: LimitLevelProvider, Identifier: r.Provider})
	}

	if r.Model != "" {
		keys = append(keys, Key{Level: LimitLevelModel, Identifier: r.Model})
	}

	return keys
}

// RateLimitError represents a rate limit exceeded error
type RateLimitError struct {
	Info *LimitInfo
	Key  Key
}

// Error implements the error interface
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for %s: retry after %v", e.Key.String(), e.Info.RetryAfter)
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}
