package discovery

// Example: Integration with backend server
//
// This file demonstrates how to integrate the discovery engine
// with your backend application.

/*

// In your main.go or server initialization:

package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/biodoia/goleapifree/internal/discovery"
    "github.com/biodoia/goleapifree/pkg/config"
    "github.com/biodoia/goleapifree/pkg/database"
    "github.com/rs/zerolog"
)

func main() {
    // Setup logger
    logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

    // Load configuration
    cfg, err := config.Load("config.yaml")
    if err != nil {
        logger.Fatal().Err(err).Msg("Failed to load config")
    }

    // Connect to database
    db, err := database.New(&cfg.Database)
    if err != nil {
        logger.Fatal().Err(err).Msg("Failed to connect to database")
    }
    defer db.Close()

    // Create context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // === START DISCOVERY SERVICE ===
    if cfg.Providers.AutoDiscovery {
        discoveryConfig := &discovery.DiscoveryConfig{
            Enabled:           true,
            Interval:          24 * time.Hour,
            GitHubToken:       os.Getenv("GITHUB_TOKEN"),
            GitHubEnabled:     true,
            ScraperEnabled:    true,
            MaxConcurrent:     5,
            ValidationTimeout: 30 * time.Second,
            MinHealthScore:    0.6,
        }

        discoveryEngine := discovery.NewEngine(discoveryConfig, db, logger)

        if err := discoveryEngine.Start(ctx); err != nil {
            logger.Error().Err(err).Msg("Failed to start discovery engine")
        } else {
            logger.Info().Msg("Discovery engine started")
            defer discoveryEngine.Stop()
        }

        // Start periodic verification of existing providers
        go discovery.ScheduleVerification(
            ctx,
            db,
            discovery.NewValidator(30*time.Second, logger),
            6*time.Hour,
            logger,
        )
    }

    // === START YOUR SERVER ===
    // ... your server initialization code ...

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    logger.Info().Msg("Shutting down...")
    cancel()
}

// === MANUAL DISCOVERY ===
// Run discovery manually from your code:

func runManualDiscovery(db *database.DB, logger zerolog.Logger) error {
    config := &discovery.DiscoveryConfig{
        Enabled:           true,
        GitHubToken:       os.Getenv("GITHUB_TOKEN"),
        GitHubEnabled:     true,
        ScraperEnabled:    true,
        MaxConcurrent:     5,
        ValidationTimeout: 30 * time.Second,
        MinHealthScore:    0.6,
    }

    engine := discovery.NewEngine(config, db, logger)
    ctx := context.Background()

    return engine.RunDiscovery(ctx)
}

// === VALIDATE SINGLE ENDPOINT ===
// Validate a single endpoint:

func validateSingleEndpoint(url string, logger zerolog.Logger) (*discovery.ValidationResult, error) {
    validator := discovery.NewValidator(30*time.Second, logger)
    ctx := context.Background()

    return validator.ValidateEndpoint(ctx, url, "api_key")
}

// === GET DISCOVERY STATS ===
// Retrieve discovery statistics:

func getStats(db *database.DB) (map[string]interface{}, error) {
    return discovery.GetDiscoveryStats(db)
}

// === API ENDPOINT EXAMPLE ===
// Example API endpoint to trigger discovery:

import "github.com/gofiber/fiber/v3"

func setupDiscoveryRoutes(app *fiber.App, engine *discovery.Engine, db *database.DB) {
    api := app.Group("/api/v1/discovery")

    // Get discovery stats
    api.Get("/stats", func(c *fiber.Ctx) error {
        stats, err := discovery.GetDiscoveryStats(db)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(stats)
    })

    // Trigger manual discovery
    api.Post("/run", func(c *fiber.Ctx) error {
        if !engine.IsRunning() {
            return c.Status(503).JSON(fiber.Map{"error": "discovery engine not running"})
        }

        go func() {
            if err := engine.RunDiscovery(context.Background()); err != nil {
                // Log error
            }
        }()

        return c.JSON(fiber.Map{"message": "discovery started"})
    })

    // Get engine status
    api.Get("/status", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{
            "running": engine.IsRunning(),
        })
    })

    // Validate endpoint
    api.Post("/validate", func(c *fiber.Ctx) error {
        var req struct {
            URL      string `json:"url"`
            AuthType string `json:"auth_type"`
        }

        if err := c.BodyParser(&req); err != nil {
            return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
        }

        validator := discovery.NewValidator(30*time.Second, zerolog.Nop())
        result, err := validator.ValidateEndpoint(context.Background(), req.URL, req.AuthType)

        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(result)
    })
}

*/
