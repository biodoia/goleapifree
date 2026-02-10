package ratelimit_test

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/ratelimit"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// Example demonstrates basic usage of the rate limiter
func Example_basic() {
	// Create a multi-level limiter
	limiter := ratelimit.NewMultiLevelLimiter()

	// Add global rate limit: 1000 requests per minute
	globalLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:      ratelimit.LimitLevelGlobal,
		Algorithm:  ratelimit.AlgorithmTokenBucket,
		Limit:      1000,
		Window:     time.Minute,
		Burst:      100,
		KeyPrefix:  "global",
	})
	limiter.AddLimiter(ratelimit.LimitLevelGlobal, globalLimiter)

	// Add per-user rate limit: 100 requests per minute
	userLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:             ratelimit.LimitLevelUser,
		Algorithm:         ratelimit.AlgorithmSlidingWindowCounter,
		Limit:             100,
		Window:            time.Minute,
		Burst:             10,
		PremiumMultiplier: 2.0, // Premium users get 2x limits
		KeyPrefix:         "user",
	})
	limiter.AddLimiter(ratelimit.LimitLevelUser, userLimiter)

	// Mark a user as premium
	limiter.AddPremiumUser("premium-user-123")

	// Check rate limit
	ctx := context.Background()
	keys := []ratelimit.Key{
		{Level: ratelimit.LimitLevelGlobal, Identifier: "global"},
		{Level: ratelimit.LimitLevelUser, Identifier: "user-123"},
	}

	info, err := limiter.Allow(ctx, keys)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if info.Allowed {
		fmt.Println("Request allowed")
		fmt.Printf("Remaining: %d/%d\n", info.Remaining, info.Limit)
	} else {
		fmt.Println("Rate limit exceeded")
		fmt.Printf("Retry after: %v\n", info.RetryAfter)
	}
}

// Example demonstrates HTTP middleware usage
func Example_httpMiddleware() {
	app := fiber.New()

	// Create limiter
	limiter := ratelimit.NewMultiLevelLimiter()

	// Add limiters
	globalLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelGlobal,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     1000,
		Window:    time.Minute,
		Burst:     100,
	})
	limiter.AddLimiter(ratelimit.LimitLevelGlobal, globalLimiter)

	// Apply middleware
	app.Use(ratelimit.Middleware(ratelimit.MiddlewareConfig{
		Limiter:        limiter,
		IncludeHeaders: true,
	}))

	// Routes
	app.Get("/api/v1/chat", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello World"})
	})

	// Start server
	app.Listen(":3000")
}

// Example demonstrates distributed rate limiting with Redis
func Example_distributed() {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	// Create distributed limiter
	limiter := ratelimit.NewDistributedLimiter(ratelimit.Config{
		Level:       ratelimit.LimitLevelUser,
		Algorithm:   ratelimit.AlgorithmSlidingWindowCounter,
		Limit:       100,
		Window:      time.Minute,
		Burst:       10,
		Distributed: true,
		KeyPrefix:   "ratelimit",
	}, redisClient)

	// Use limiter
	ctx := context.Background()
	key := ratelimit.Key{
		Level:      ratelimit.LimitLevelUser,
		Identifier: "user-123",
	}

	info, err := limiter.Allow(ctx, key)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Allowed: %v, Remaining: %d\n", info.Allowed, info.Remaining)
}

// Example demonstrates quota management
func Example_quota() {
	// Create quota manager
	quotaManager := ratelimit.NewLocalQuotaManager(ratelimit.QuotaConfig{
		Limit:            10000,
		Period:           ratelimit.QuotaPeriodMonthly,
		Type:             ratelimit.QuotaTypeHard,
		WarningThreshold: 0.8,
	})

	// Use quota
	ctx := context.Background()
	userID := "user-123"

	info, err := quotaManager.Use(ctx, userID, 10)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !info.Allowed {
		fmt.Println("Quota exceeded")
		fmt.Printf("Used: %d/%d\n", info.Used, info.Limit)
		fmt.Printf("Reset: %v\n", info.Reset)
		return
	}

	if info.Warning {
		fmt.Printf("Warning: %d%% of quota used\n", info.Used*100/info.Limit)
	}

	fmt.Printf("Quota remaining: %d/%d\n", info.Remaining, info.Limit)
}

// Example demonstrates multi-level rate limiting with different algorithms
func Example_multiAlgorithm() {
	limiter := ratelimit.NewMultiLevelLimiter()

	// Global: Token bucket (smooth rate limiting)
	globalLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelGlobal,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     10000,
		Window:    time.Hour,
		Burst:     1000,
	})
	limiter.AddLimiter(ratelimit.LimitLevelGlobal, globalLimiter)

	// User: Sliding window (more accurate)
	userLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelUser,
		Algorithm: ratelimit.AlgorithmSlidingWindowCounter,
		Limit:     100,
		Window:    time.Minute,
		Burst:     10,
	})
	limiter.AddLimiter(ratelimit.LimitLevelUser, userLimiter)

	// Provider: Fixed window (simple and fast)
	providerLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelProvider,
		Algorithm: ratelimit.AlgorithmFixedWindow,
		Limit:     500,
		Window:    time.Minute,
		Burst:     50,
	})
	limiter.AddLimiter(ratelimit.LimitLevelProvider, providerLimiter)

	// Test rate limiting
	ctx := context.Background()
	keys := []ratelimit.Key{
		{Level: ratelimit.LimitLevelGlobal, Identifier: "global"},
		{Level: ratelimit.LimitLevelUser, Identifier: "user-123"},
		{Level: ratelimit.LimitLevelProvider, Identifier: "openai"},
	}

	info, _ := limiter.Allow(ctx, keys)
	fmt.Printf("Allowed: %v, Remaining: %d\n", info.Allowed, info.Remaining)
}

// Example demonstrates per-endpoint rate limiting
func Example_perEndpoint() {
	app := fiber.New()

	// Create limiters with different configs
	strictLimiter := ratelimit.NewMultiLevelLimiter()
	globalStrict, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelGlobal,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     10,
		Window:    time.Minute,
		Burst:     2,
	})
	strictLimiter.AddLimiter(ratelimit.LimitLevelGlobal, globalStrict)

	relaxedLimiter := ratelimit.NewMultiLevelLimiter()
	globalRelaxed, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelGlobal,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     100,
		Window:    time.Minute,
		Burst:     20,
	})
	relaxedLimiter.AddLimiter(ratelimit.LimitLevelGlobal, globalRelaxed)

	// Apply per-endpoint middleware
	app.Use(ratelimit.PerEndpointMiddleware(map[string]ratelimit.MiddlewareConfig{
		"/api/v1/chat/completions": {
			Limiter:        strictLimiter,
			IncludeHeaders: true,
		},
		"/api/v1/models": {
			Limiter:        relaxedLimiter,
			IncludeHeaders: true,
		},
	}))

	app.Listen(":3000")
}

// Example demonstrates combined rate limiting and quota management
func Example_combined() {
	app := fiber.New()

	// Create rate limiter
	limiter := ratelimit.NewMultiLevelLimiter()
	userLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelUser,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     60,
		Window:    time.Minute,
		Burst:     10,
	})
	limiter.AddLimiter(ratelimit.LimitLevelUser, userLimiter)

	// Create quota manager
	quotaManager := ratelimit.NewLocalQuotaManager(ratelimit.QuotaConfig{
		Limit:            100000,
		Period:           ratelimit.QuotaPeriodMonthly,
		Type:             ratelimit.QuotaTypeHard,
		WarningThreshold: 0.8,
	})

	// Apply middleware with both rate limiting and quota
	app.Use(ratelimit.Middleware(ratelimit.MiddlewareConfig{
		Limiter:        limiter,
		QuotaManager:   quotaManager,
		IncludeHeaders: true,
	}))

	app.Get("/api/v1/chat", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello World"})
	})

	app.Listen(":3000")
}

// Example demonstrates adaptive rate limiting
func Example_adaptive() {
	app := fiber.New()

	limiter := ratelimit.NewMultiLevelLimiter()
	userLimiter, _ := ratelimit.NewLimiter(ratelimit.Config{
		Level:     ratelimit.LimitLevelUser,
		Algorithm: ratelimit.AlgorithmTokenBucket,
		Limit:     100,
		Window:    time.Minute,
		Burst:     20,
	})
	limiter.AddLimiter(ratelimit.LimitLevelUser, userLimiter)

	// Create adaptive middleware
	adaptive := ratelimit.NewAdaptiveMiddleware(ratelimit.MiddlewareConfig{
		Limiter:        limiter,
		IncludeHeaders: true,
	})

	app.Use(adaptive.Handler())

	app.Get("/api/v1/chat", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello World"})
	})

	app.Listen(":3000")
}
