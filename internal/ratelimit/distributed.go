package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLimiter implements rate limiting using Redis for distributed systems
type DistributedLimiter struct {
	config Config
	redis  *redis.Client
}

// NewDistributedLimiter creates a new distributed limiter
func NewDistributedLimiter(config Config, redisClient *redis.Client) *DistributedLimiter {
	return &DistributedLimiter{
		config: config,
		redis:  redisClient,
	}
}

// Allow checks if a request is allowed
func (d *DistributedLimiter) Allow(ctx context.Context, key Key) (*LimitInfo, error) {
	return d.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (d *DistributedLimiter) AllowN(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	switch d.config.Algorithm {
	case AlgorithmTokenBucket:
		return d.allowTokenBucket(ctx, key, n)
	case AlgorithmFixedWindow:
		return d.allowFixedWindow(ctx, key, n)
	case AlgorithmSlidingWindowLog:
		return d.allowSlidingWindowLog(ctx, key, n)
	case AlgorithmSlidingWindowCounter:
		return d.allowSlidingWindowCounter(ctx, key, n)
	default:
		return nil, fmt.Errorf("algorithm %s not supported in distributed mode", d.config.Algorithm)
	}
}

// allowTokenBucket implements token bucket using Redis
func (d *DistributedLimiter) allowTokenBucket(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := d.getRedisKey(key)
	now := time.Now()

	// Lua script for atomic token bucket operations
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local rate = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])
		local now = tonumber(ARGV[5])

		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1])
		local last_refill = tonumber(bucket[2])

		if tokens == nil then
			tokens = limit + burst
			last_refill = now
		end

		-- Refill tokens
		local elapsed = now - last_refill
		local tokens_to_add = rate * elapsed
		tokens = math.min(tokens + tokens_to_add, limit + burst)

		-- Check and consume
		local allowed = tokens >= requested
		if allowed then
			tokens = tokens - requested
		end

		-- Update state
		redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
		redis.call('EXPIRE', key, math.ceil((limit + burst) / rate) + 60)

		return {allowed and 1 or 0, math.floor(tokens)}
	`)

	refillRate := float64(d.config.Limit) / d.config.Window.Seconds()

	result, err := script.Run(ctx, d.redis, []string{keyStr},
		d.config.Limit,
		d.config.Burst,
		refillRate,
		n,
		now.Unix(),
	).Result()

	if err != nil {
		return nil, fmt.Errorf("token bucket script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	remaining := resultSlice[1].(int64)

	maxTokens := d.config.Limit + d.config.Burst
	tokensToFill := float64(maxTokens - remaining)
	resetDuration := time.Duration(tokensToFill/refillRate) * time.Second

	var retryAfter time.Duration
	if !allowed {
		tokensNeeded := float64(n) - float64(remaining)
		retryAfter = time.Duration(tokensNeeded/refillRate) * time.Second
	}

	return &LimitInfo{
		Allowed:    allowed,
		Limit:      maxTokens,
		Remaining:  remaining,
		Reset:      now.Add(resetDuration),
		RetryAfter: retryAfter,
	}, nil
}

// allowFixedWindow implements fixed window using Redis
func (d *DistributedLimiter) allowFixedWindow(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	now := time.Now()
	windowStart := now.Truncate(d.config.Window)
	keyStr := d.getRedisKey(key) + ":" + strconv.FormatInt(windowStart.Unix(), 10)

	// Lua script for atomic fixed window operations
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local requested = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])

		local current = tonumber(redis.call('GET', key)) or 0
		local allowed = current + requested <= limit

		if allowed then
			current = redis.call('INCRBY', key, requested)
			redis.call('EXPIRE', key, ttl)
		end

		return {allowed and 1 or 0, current}
	`)

	ttl := int(d.config.Window.Seconds()) + 1

	result, err := script.Run(ctx, d.redis, []string{keyStr},
		d.config.Limit+d.config.Burst,
		n,
		ttl,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("fixed window script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	current := resultSlice[1].(int64)

	limit := d.config.Limit + d.config.Burst
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	reset := windowStart.Add(d.config.Window)

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

// allowSlidingWindowLog implements sliding window log using Redis
func (d *DistributedLimiter) allowSlidingWindowLog(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	keyStr := d.getRedisKey(key)
	now := time.Now()
	windowStart := now.Add(-d.config.Window)

	// Lua script for atomic sliding window log operations
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])
		local ttl = tonumber(ARGV[5])

		-- Remove old entries
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- Count current requests
		local current = redis.call('ZCARD', key)
		local allowed = current + requested <= limit

		if allowed then
			-- Add new requests
			for i = 1, requested do
				redis.call('ZADD', key, now, now .. ':' .. i)
			end
			current = current + requested
		end

		redis.call('EXPIRE', key, ttl)

		-- Get oldest timestamp for reset calculation
		local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
		local oldest_ts = 0
		if #oldest > 0 then
			oldest_ts = tonumber(oldest[2])
		end

		return {allowed and 1 or 0, current, oldest_ts}
	`)

	ttl := int(d.config.Window.Seconds()) + 60

	result, err := script.Run(ctx, d.redis, []string{keyStr},
		d.config.Limit+d.config.Burst,
		windowStart.UnixNano(),
		now.UnixNano(),
		n,
		ttl,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("sliding window log script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	current := resultSlice[1].(int64)
	oldestTS := resultSlice[2].(int64)

	limit := d.config.Limit + d.config.Burst
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	var reset time.Time
	if oldestTS > 0 {
		oldestTime := time.Unix(0, oldestTS)
		reset = oldestTime.Add(d.config.Window)
	} else {
		reset = now.Add(d.config.Window)
	}

	var retryAfter time.Duration
	if !allowed && oldestTS > 0 {
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

// allowSlidingWindowCounter implements sliding window counter using Redis
func (d *DistributedLimiter) allowSlidingWindowCounter(ctx context.Context, key Key, n int64) (*LimitInfo, error) {
	now := time.Now()
	currentWindowStart := now.Truncate(d.config.Window)
	previousWindowStart := currentWindowStart.Add(-d.config.Window)

	currentKey := d.getRedisKey(key) + ":current:" + strconv.FormatInt(currentWindowStart.Unix(), 10)
	previousKey := d.getRedisKey(key) + ":previous:" + strconv.FormatInt(previousWindowStart.Unix(), 10)

	// Lua script for atomic sliding window counter operations
	script := redis.NewScript(`
		local current_key = KEYS[1]
		local previous_key = KEYS[2]
		local limit = tonumber(ARGV[1])
		local requested = tonumber(ARGV[2])
		local weight = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])

		local current_count = tonumber(redis.call('GET', current_key)) or 0
		local previous_count = tonumber(redis.call('GET', previous_key)) or 0

		-- Calculate weighted count
		local estimated = previous_count * (1 - weight) + current_count
		local allowed = estimated + requested <= limit

		if allowed then
			current_count = redis.call('INCRBY', current_key, requested)
			redis.call('EXPIRE', current_key, ttl)
			estimated = estimated + requested
		end

		redis.call('EXPIRE', previous_key, ttl)

		return {allowed and 1 or 0, math.floor(estimated), current_count, previous_count}
	`)

	elapsed := now.Sub(currentWindowStart)
	weight := float64(elapsed) / float64(d.config.Window)
	ttl := int(d.config.Window.Seconds()) * 2

	result, err := script.Run(ctx, d.redis, []string{currentKey, previousKey},
		d.config.Limit+d.config.Burst,
		n,
		weight,
		ttl,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("sliding window counter script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	estimated := resultSlice[1].(int64)

	limit := d.config.Limit + d.config.Burst
	remaining := limit - estimated
	if remaining < 0 {
		remaining = 0
	}

	reset := currentWindowStart.Add(d.config.Window)

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

// GetInfo returns current limit info without consuming tokens
func (d *DistributedLimiter) GetInfo(ctx context.Context, key Key) (*LimitInfo, error) {
	switch d.config.Algorithm {
	case AlgorithmTokenBucket:
		return d.getInfoTokenBucket(ctx, key)
	case AlgorithmFixedWindow:
		return d.getInfoFixedWindow(ctx, key)
	case AlgorithmSlidingWindowLog:
		return d.getInfoSlidingWindowLog(ctx, key)
	case AlgorithmSlidingWindowCounter:
		return d.getInfoSlidingWindowCounter(ctx, key)
	default:
		return nil, fmt.Errorf("algorithm %s not supported in distributed mode", d.config.Algorithm)
	}
}

// getInfoTokenBucket gets token bucket info
func (d *DistributedLimiter) getInfoTokenBucket(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := d.getRedisKey(key)
	now := time.Now()

	values, err := d.redis.HMGet(ctx, keyStr, "tokens", "last_refill").Result()
	if err != nil {
		return nil, err
	}

	maxTokens := d.config.Limit + d.config.Burst
	refillRate := float64(d.config.Limit) / d.config.Window.Seconds()

	if values[0] == nil {
		return &LimitInfo{
			Allowed:   true,
			Limit:     maxTokens,
			Remaining: maxTokens,
			Reset:     now.Add(d.config.Window),
		}, nil
	}

	tokens, _ := strconv.ParseFloat(values[0].(string), 64)
	lastRefill, _ := strconv.ParseInt(values[1].(string), 10, 64)

	elapsed := now.Unix() - lastRefill
	tokensToAdd := refillRate * float64(elapsed)
	currentTokens := min(tokens+tokensToAdd, float64(maxTokens))

	remaining := int64(currentTokens)
	tokensToFill := float64(maxTokens) - currentTokens
	resetDuration := time.Duration(tokensToFill/refillRate) * time.Second

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     maxTokens,
		Remaining: remaining,
		Reset:     now.Add(resetDuration),
	}, nil
}

// getInfoFixedWindow gets fixed window info
func (d *DistributedLimiter) getInfoFixedWindow(ctx context.Context, key Key) (*LimitInfo, error) {
	now := time.Now()
	windowStart := now.Truncate(d.config.Window)
	keyStr := d.getRedisKey(key) + ":" + strconv.FormatInt(windowStart.Unix(), 10)

	val, err := d.redis.Get(ctx, keyStr).Result()
	if err == redis.Nil {
		limit := d.config.Limit + d.config.Burst
		return &LimitInfo{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
			Reset:     windowStart.Add(d.config.Window),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	current, _ := strconv.ParseInt(val, 10, 64)
	limit := d.config.Limit + d.config.Burst
	remaining := limit - current

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     windowStart.Add(d.config.Window),
	}, nil
}

// getInfoSlidingWindowLog gets sliding window log info
func (d *DistributedLimiter) getInfoSlidingWindowLog(ctx context.Context, key Key) (*LimitInfo, error) {
	keyStr := d.getRedisKey(key)
	now := time.Now()
	windowStart := now.Add(-d.config.Window)

	// Count requests in current window
	count, err := d.redis.ZCount(ctx, keyStr, strconv.FormatInt(windowStart.UnixNano(), 10), "+inf").Result()
	if err != nil {
		return nil, err
	}

	limit := d.config.Limit + d.config.Burst
	remaining := limit - count

	// Get oldest timestamp
	oldest, err := d.redis.ZRangeWithScores(ctx, keyStr, 0, 0).Result()
	if err != nil {
		return nil, err
	}

	var reset time.Time
	if len(oldest) > 0 {
		oldestTime := time.Unix(0, int64(oldest[0].Score))
		reset = oldestTime.Add(d.config.Window)
	} else {
		reset = now.Add(d.config.Window)
	}

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
	}, nil
}

// getInfoSlidingWindowCounter gets sliding window counter info
func (d *DistributedLimiter) getInfoSlidingWindowCounter(ctx context.Context, key Key) (*LimitInfo, error) {
	now := time.Now()
	currentWindowStart := now.Truncate(d.config.Window)
	previousWindowStart := currentWindowStart.Add(-d.config.Window)

	currentKey := d.getRedisKey(key) + ":current:" + strconv.FormatInt(currentWindowStart.Unix(), 10)
	previousKey := d.getRedisKey(key) + ":previous:" + strconv.FormatInt(previousWindowStart.Unix(), 10)

	pipe := d.redis.Pipeline()
	currentCmd := pipe.Get(ctx, currentKey)
	previousCmd := pipe.Get(ctx, previousKey)
	_, err := pipe.Exec(ctx)

	currentCount := int64(0)
	previousCount := int64(0)

	if err == nil || err == redis.Nil {
		if val, err := currentCmd.Result(); err == nil {
			currentCount, _ = strconv.ParseInt(val, 10, 64)
		}
		if val, err := previousCmd.Result(); err == nil {
			previousCount, _ = strconv.ParseInt(val, 10, 64)
		}
	}

	elapsed := now.Sub(currentWindowStart)
	weight := float64(elapsed) / float64(d.config.Window)
	estimatedCount := float64(previousCount)*(1-weight) + float64(currentCount)

	limit := d.config.Limit + d.config.Burst
	remaining := int64(float64(limit) - estimatedCount)

	return &LimitInfo{
		Allowed:   remaining > 0,
		Limit:     limit,
		Remaining: remaining,
		Reset:     currentWindowStart.Add(d.config.Window),
	}, nil
}

// Reset resets the limit for a key
func (d *DistributedLimiter) Reset(ctx context.Context, key Key) error {
	keyPattern := d.getRedisKey(key) + "*"

	iter := d.redis.Scan(ctx, 0, keyPattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := d.redis.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}

	return iter.Err()
}

// getRedisKey generates a Redis key for the given rate limit key
func (d *DistributedLimiter) getRedisKey(key Key) string {
	prefix := d.config.KeyPrefix
	if prefix == "" {
		prefix = "ratelimit"
	}
	return fmt.Sprintf("%s:%s:%s", prefix, key.Level, key.Identifier)
}

// CleanupExpired removes expired entries (should be called periodically)
func (d *DistributedLimiter) CleanupExpired(ctx context.Context) error {
	// This is mostly handled by Redis TTL/EXPIRE
	// But we can do additional cleanup if needed
	return nil
}
