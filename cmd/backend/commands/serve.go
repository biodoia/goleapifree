package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biodoia/goleapifree/internal/gateway"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	devMode    bool
	verbose    bool
	autoMigrate bool
)

// ServeCmd rappresenta il comando serve
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start GoLeapAI gateway server",
	Long: `Start the GoLeapAI gateway server with all features enabled.

This command starts the HTTP server that acts as a unified gateway
for all free LLM API providers with intelligent routing.`,
	Example: `  # Start server with default settings
  goleapai serve

  # Start in development mode with verbose logging
  goleapai serve --dev --verbose

  # Start with auto-migration enabled
  goleapai serve --migrate

  # Start with custom config
  goleapai serve -c /path/to/config.yaml`,
	RunE: runServe,
}

func init() {
	ServeCmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode (pretty logging, hot reload)")
	ServeCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging (debug level)")
	ServeCmd.Flags().BoolVar(&autoMigrate, "migrate", true, "Auto-run database migrations on startup")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Setup logger
	setupLogger(verbose, devMode)

	log.Info().Msg("🚀 Starting GoLeapAI Gateway")

	// Load configuration
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Bool("http3", cfg.Server.HTTP3).
		Bool("dev_mode", devMode).
		Msg("Configuration loaded")

	// Initialize database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	log.Info().
		Str("type", cfg.Database.Type).
		Str("connection", cfg.Database.Connection).
		Msg("Database connected")

	// Run migrations if enabled
	if autoMigrate {
		log.Info().Msg("Running database migrations...")
		if err := db.AutoMigrate(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Info().Msg("✓ Database migrations completed")

		// Seed database
		if err := db.Seed(); err != nil {
			log.Warn().Err(err).Msg("Failed to seed database (may already be seeded)")
		} else {
			log.Info().Msg("✓ Database seeded with free API providers")
		}
	}

	// Create gateway instance
	gw, err := gateway.New(cfg, db)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Start gateway in background
	go func() {
		if err := gw.Start(); err != nil {
			log.Fatal().Err(err).Msg("Gateway failed to start")
		}
	}()

	// Log startup information
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Info().Msgf("🌐 Gateway running on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().Msgf("📊 Health check: http://%s:%d/health", cfg.Server.Host, cfg.Server.Port)
	log.Info().Msgf("🔧 Admin API: http://%s:%d/admin", cfg.Server.Host, cfg.Server.Port)
	if cfg.Monitoring.Prometheus.Enabled {
		log.Info().Msgf("📈 Metrics: http://%s:%d/metrics", cfg.Server.Host, cfg.Server.Port)
	}
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Info().Msg("Press Ctrl+C to stop")

	// Setup graceful shutdown
	return waitForShutdown(gw)
}

func waitForShutdown(gw *gateway.Gateway) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("⏳ Shutting down gracefully...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown gateway
	if err := gw.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		return err
	}

	log.Info().Msg("✓ GoLeapAI Gateway stopped cleanly")
	return nil
}

func setupLogger(verbose, dev bool) {
	// Set log level
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Pretty console output in development
	if dev {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON output for production
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}
}
