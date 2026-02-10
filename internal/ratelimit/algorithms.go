package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TokenBucketLimiter implements the token bucket algorithm
type TokenBucketLimiter struct {
	config  Config
	buckets sync.Map // map[string]*bucket
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket limiter
func NewTokenBucketLimiter(config Config) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		config: config,
	}
}

// Allow checks if a request is allowed
func (t *TokenBucketLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return t.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (t *TokenBucketLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := key.String()

	bucketInterface, _ := t.buckets.LoadOrStore(keyStr, &bucket{
		tokens:     float64(t.config.Limit + t.config.Burst),
		lastRefill: time.Now(),
	})

	b := bucketInterface.(*bucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)

	// Calculate refill rate (tokens per second)
	refillRate := float64(t.config.Limit) / t.config.Window.Seconds()
	tokensToAdd := refillRate * elapsed.Seconds()

	maxTokens := float64(t.config.Limit + t.config.Burst)
	b.tokens = min(b.tokens+tokensToAdd, maxTokens)
	b.lastRefill = now

	// Check if we have enough tokens
	allowed := b.tokens >= float64(n)

	if allowed {
		b.tokens -= float64(n)
	}

	remaining := int64(b.tokens)
	if remaining < 0 {
		remaining = 0
	}

	// Calculate retry after if not allowed
	var retryAfter time.Duration
	if !allowed {
		tokensNeeded := float64(n) - b.tokens
		retryAfter = time.Duration(tokensNeeded/refillRate) * time.Second
	}

	// Calculate reset time (when bucket will be full)
	tokensToFill := maxTokens - b.tokens
	resetDuration := time.Duration(tokensToFill/refillRate) * time.Second

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      t.config.Limit + t.config.Burst,
		Remaining:  remaining,
		Reset:      now.Add(resetDuration),
		RetryAfter: retryAfter,
	}, nil
}

// GetInfo returns current limit info without consuming tokens
func (t *TokenBucketLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := key.String()

	bucketInterface, ok := t.buckets.Load(keyStr)
	if !ok {
		return &LimitInfo{
			Allowed:   true,
			Limit:     t.config.Limit + t.config.Burst,
			Remaining: t.config.Limit + t.config.Burst,
			Reset:     time.Now().Add(t.config.Window),
		}, nil
	}

	b := bucketInterface.(*bucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens (without modifying)
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	refillRate := float64(t.config.Limit) / t.config.Window.Seconds()
	tokensToAdd := refillRate * elapsed.Seconds()

	maxTokens := float64(t.config.Limit + t.config.Burst)
	currentTokens := min(b.tokens+tokensToAdd, maxTokens)

	remaining := int64(currentTokens)
	if remaining < 0 {
		remaining = 0
	}

	tokensToFill := maxTokens - currentTokens
	resetDuration := time.Duration(tokensToFill/refillRate) * time.Second

	return &LimitInfo{
		Allowed:   currentTokens >= 1,
		Limit:     t.config.Limit + t.config.Burst,
		Remaining: remaining,
		Reset:     now.Add(resetDuration),
	}, nil
}

// Reset resets the bucket for a key
func (t *TokenBucketLimiter) Reset(ctx context.Context, key Key) error {
	t.buckets.Delete(key.String())
	return nil
}

// LeakyBucketLimiter implements the leaky bucket algorithm
type LeakyBucketLimiter struct {
	config  Config
	buckets sync.Map // map[string]*leakyBucket
}

type leakyBucket struct {
	queue    []time.Time
	lastLeak time.Time
	mu       sync.Mutex
}

// NewLeakyBucketLimiter creates a new leaky bucket limiter
func NewLeakyBucketLimiter(config Config) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
		config: config,
	}
}

// Allow checks if a request is allowed
func (l *LeakyBucketLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (l *LeakyBucketLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := key.String()

	bucketInterface, _ := l.buckets.LoadOrStore(keyStr, &leakyBucket{
		queue:    []time.Time{},
		lastLeak: time.Now(),
	})

	b := bucketInterface.(*leakyBucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// Leak tokens based on elapsed time
	leakRate := float64(l.config.Limit) / l.config.Window.Seconds()
	elapsed := now.Sub(b.lastLeak).Seconds()
	tokensToLeak := int(leakRate * elapsed)

	if tokensToLeak > 0 {
		if tokensToLeak >= len(b.queue) {
			b.queue = []time.Time{}
		} else {
			b.queue = b.queue[tokensToLeak:]
		}
		b.lastLeak = now
	}

	// Check if we can add n tokens
	capacity := l.config.Limit + l.config.Burst
	allowed := int64(len(b.queue))+n <= capacity

	if allowed {
		for i := int64(0); i < n; i++ {
			b.queue = append(b.queue, now)
		}
	}

	remaining := capacity - int64(len(b.queue))
	if remaining < 0 {
		remaining = 0
	}

	// Calculate retry after
	var retryAfter time.Duration
	if !allowed {
		overflow := int64(len(b.queue)) + n - capacity
		retryAfter = time.Duration(float64(overflow)/leakRate) * time.Second
	}

	// Calculate reset time
	resetDuration := time.Duration(float64(len(b.queue))/leakRate) * time.Second

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      capacity,
		Remaining:  remaining,
		Reset:      now.Add(resetDuration),
		RetryAfter: retryAfter,
	}, nil
}

// GetInfo returns current limit info
func (l *LeakyBucketLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := key.String()

	bucketInterface, ok := l.buckets.Load(keyStr)
	if !ok {
		capacity := l.config.Limit + l.config.Burst
		return &LimitInfo{
			Allowed:   true,
			Limit:     capacity,
			Remaining: capacity,
			Reset:     time.Now().Add(l.config.Window),
		}, nil
	}

	b := bucketInterface.(*leakyBucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	leakRate := float64(l.config.Limit) / l.config.Window.Seconds()
	elapsed := now.Sub(b.lastLeak).Seconds()
	tokensToLeak := int(leakRate * elapsed)

	queueSize := len(b.queue) - tokensToLeak
	if queueSize < 0 {
		queueSize = 0
	}

	capacity := l.config.Limit + l.config.Burst
	remaining := capacity - int64(queueSize)

	resetDuration := time.Duration(float64(queueSize)/leakRate) * time.Second

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     capacity,
		Remaining: remaining,
		Reset:     now.Add(resetDuration),
	}, nil
}

// Reset resets the bucket
func (l *LeakyBucketLimiter) Reset(ctx context.Context, key Key) error {
	l.buckets.Delete(key.String())
	return nil
}

// FixedWindowLimiter implements the fixed window algorithm
type FixedWindowLimiter struct {
	config  Config
	windows sync.Map // map[string]*fixedWindow
}

type fixedWindow struct {
	count      int64
	windowStart time.Time
	mu         sync.Mutex
}

// NewFixedWindowLimiter creates a new fixed window limiter
func NewFixedWindowLimiter(config Config) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		config: config,
	}
}

// Allow checks if a request is allowed
func (f *FixedWindowLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return f.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (f *FixedWindowLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := key.String()

	now := time.Now()
	windowStart := now.Truncate(f.config.Window)

	windowInterface, _ := f.windows.LoadOrStore(keyStr, &fixedWindow{
		count:      0,
		windowStart: windowStart,
	})

	w := windowInterface.(*fixedWindow)
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reset window if needed
	if now.Sub(w.windowStart) >= f.config.Window {
		w.count = 0
		w.windowStart = windowStart
	}

	limit := f.config.Limit + f.config.Burst
	allowed := w.count+n <= limit

	if allowed {
		w.count += n
	}

	remaining := limit - w.count
	if remaining < 0 {
		remaining = 0
	}

	reset := w.windowStart.Add(f.config.Window)

	var retryAfter time.Duration
	if !allowed {
		retryAfter = time.Until(reset)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		Reset:      reset,
		RetryAfter: retryAfter,
	}, nil
}

// GetInfo returns current limit info
func (f *FixedWindowLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := key.String()

	windowInterface, ok := f.windows.Load(keyStr)
	if !ok {
		limit := f.config.Limit + f.config.Burst
		return &LimitInfo{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
			Reset:     time.Now().Truncate(f.config.Window).Add(f.config.Window),
		}, nil
	}

	w := windowInterface.(*fixedWindow)
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Truncate(f.config.Window)

	count := w.count
	if now.Sub(w.windowStart) >= f.config.Window {
		count = 0
	}

	limit := f.config.Limit + f.config.Burst
	remaining := limit - count

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     windowStart.Add(f.config.Window),
	}, nil
}

// Reset resets the window
func (f *FixedWindowLimiter) Reset(ctx context.Context, key Key) error {
	f.windows.Delete(key.String())
	return nil
}

// SlidingWindowLogLimiter implements the sliding window log algorithm
type SlidingWindowLogLimiter struct {
	config Config
	logs   sync.Map // map[string]*requestLog
}

type requestLog struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// NewSlidingWindowLogLimiter creates a new sliding window log limiter
func NewSlidingWindowLogLimiter(config Config) *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{
		config: config,
	}
}

// Allow checks if a request is allowed
func (s *SlidingWindowLogLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return s.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (s *SlidingWindowLogLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := key.String()

	logInterface, _ := s.logs.LoadOrStore(keyStr, &requestLog{
		timestamps: []time.Time{},
	})

	log := logInterface.(*requestLog)
	log.mu.Lock()
	defer log.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-s.config.Window)

	// Remove old timestamps
	validTimestamps := []time.Time{}
	for _, ts := range log.timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	log.timestamps = validTimestamps

	limit := s.config.Limit + s.config.Burst
	currentCount := int64(len(log.timestamps))
	allowed := currentCount+n <= limit

	if allowed {
		for i := int64(0); i < n; i++ {
			log.timestamps = append(log.timestamps, now)
		}
		currentCount += n
	}

	remaining := limit - currentCount
	if remaining < 0 {
		remaining = 0
	}

	// Reset is the time when the oldest request will expire
	var reset time.Time
	if len(log.timestamps) > 0 {
		reset = log.timestamps[0].Add(s.config.Window)
	} else {
		reset = now.Add(s.config.Window)
	}

	// Retry after: wait until oldest request expires
	var retryAfter time.Duration
	if !allowed && len(log.timestamps) > 0 {
		retryAfter = time.Until(log.timestamps[0].Add(s.config.Window))
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		Reset:      reset,
		RetryAfter: retryAfter,
	}, nil
}

// GetInfo returns current limit info
func (s *SlidingWindowLogLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := key.String()

	logInterface, ok := s.logs.Load(keyStr)
	if !ok {
		limit := s.config.Limit + s.config.Burst
		return &LimitInfo{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
			Reset:     time.Now().Add(s.config.Window),
		}, nil
	}

	log := logInterface.(*requestLog)
	log.mu.Lock()
	defer log.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-s.config.Window)

	count := int64(0)
	for _, ts := range log.timestamps {
		if ts.After(windowStart) {
			count++
		}
	}

	limit := s.config.Limit + s.config.Burst
	remaining := limit - count

	var reset time.Time
	if len(log.timestamps) > 0 {
		reset = log.timestamps[0].Add(s.config.Window)
	} else {
		reset = now.Add(s.config.Window)
	}

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
	}, nil
}

// Reset resets the log
func (s *SlidingWindowLogLimiter) Reset(ctx context.Context, key Key) error {
	s.logs.Delete(key.String())
	return nil
}

// SlidingWindowCounterLimiter implements the sliding window counter algorithm
type SlidingWindowCounterLimiter struct {
	config  Config
	windows sync.Map // map[string]*slidingWindow
}

type slidingWindow struct {
	currentCount  int64
	previousCount int64
	currentStart  time.Time
	mu            sync.Mutex
}

// NewSlidingWindowCounterLimiter creates a new sliding window counter limiter
func NewSlidingWindowCounterLimiter(config Config) *SlidingWindowCounterLimiter {
	return &SlidingWindowCounterLimiter{
		config: config,
	}
}

// Allow checks if a request is allowed
func (s *SlidingWindowCounterLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return s.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (s *SlidingWindowCounterLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := key.String()

	now := time.Now()
	windowStart := now.Truncate(s.config.Window)

	windowInterface, _ := s.windows.LoadOrStore(keyStr, &slidingWindow{
		currentCount:  0,
		previousCount: 0,
		currentStart:  windowStart,
	})

	w := windowInterface.(*slidingWindow)
	w.mu.Lock()
	defer w.mu.Unlock()

	// Slide window if needed
	if now.Sub(w.currentStart) >= s.config.Window {
		w.previousCount = w.currentCount
		w.currentCount = 0
		w.currentStart = windowStart
	}

	// Calculate weighted count based on position in window
	elapsed := now.Sub(w.currentStart)
	weight := float64(elapsed) / float64(s.config.Window)
	estimatedCount := float64(w.previousCount)*(1-weight) + float64(w.currentCount)

	limit := float64(s.config.Limit + s.config.Burst)
	allowed := estimatedCount+float64(n) <= limit

	if allowed {
		w.currentCount += n
		estimatedCount += float64(n)
	}

	remaining := int64(limit - estimatedCount)
	if remaining < 0 {
		remaining = 0
	}

	reset := w.currentStart.Add(s.config.Window)

	var retryAfter time.Duration
	if !allowed {
		// Estimate when enough capacity will be available
		retryAfter = time.Until(reset)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      s.config.Limit + s.config.Burst,
		Remaining:  remaining,
		Reset:      reset,
		RetryAfter: retryAfter,
	}, nil
}

// GetInfo returns current limit info
func (s *SlidingWindowCounterLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := key.String()

	windowInterface, ok := s.windows.Load(keyStr)
	if !ok {
		limit := s.config.Limit + s.config.Burst
		return &LimitInfo{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
			Reset:     time.Now().Truncate(s.config.Window).Add(s.config.Window),
		}, nil
	}

	w := windowInterface.(*slidingWindow)
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Truncate(s.config.Window)

	currentCount := w.currentCount
	previousCount := w.previousCount
	currentStart := w.currentStart

	if now.Sub(currentStart) >= s.config.Window {
		previousCount = currentCount
		currentCount = 0
		currentStart = windowStart
	}

	elapsed := now.Sub(currentStart)
	weight := float64(elapsed) / float64(s.config.Window)
	estimatedCount := float64(previousCount)*(1-weight) + float64(currentCount)

	limit := s.config.Limit + s.config.Burst
	remaining := int64(float64(limit) - estimatedCount)

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     currentStart.Add(s.config.Window),
	}, nil
}

// Reset resets the window
func (s *SlidingWindowCounterLimiter) Reset(ctx context.Context, key Key) error {
	s.windows.Delete(key.String())
	return nil
}

// NewLimiter creates a limiter based on the algorithm specified in config
func NewLimiter(config Config) (Limiter, error) {
	switch config.Algorithm {
	case AlgorithmTokenBucket:
		return NewTokenBucketLimiter(config), nil
	case AlgorithmLeakyBucket:
		return NewLeakyBucketLimiter(config), nil
	case AlgorithmFixedWindow:
		return NewFixedWindowLimiter(config), nil
	case AlgorithmSlidingWindowLog:
		return NewSlidingWindowLogLimiter(config), nil
	case AlgorithmSlidingWindowCounter:
		return NewSlidingWindowCounterLimiter(config), nil
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", config.Algorithm)
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
