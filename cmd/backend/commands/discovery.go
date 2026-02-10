package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/biodoia/goleapifree/internal/discovery"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// DiscoveryCmd gestisce il sistema di auto-discovery
var DiscoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: "Manage API auto-discovery system",
	Long: `Auto-discovery system for finding new free LLM APIs.

The discovery system searches GitHub repositories, awesome lists, and other
sources to automatically find and validate new API providers.`,
}

var discoveryRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run discovery once",
	Long: `Execute a single discovery run to find and validate new providers.

This will:
1. Search GitHub for relevant repositories
2. Scrape awesome lists and documentation
3. Validate discovered endpoints
4. Save valid providers to the database`,
	RunE: runDiscoveryOnce,
}

var discoveryStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start discovery service",
	Long: `Start the discovery service with periodic execution.

The service will run discovery at configured intervals and keep
running until interrupted.`,
	RunE: runDiscoveryService,
}

var discoveryStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show discovery statistics",
	Long:  "Display statistics about discovered providers",
	RunE:  showDiscoveryStats,
}

var discoveryValidateCmd = &cobra.Command{
	Use:   "validate [url]",
	Short: "Validate a specific endpoint",
	Long:  "Validate a single API endpoint and show detailed results",
	Args:  cobra.ExactArgs(1),
	RunE:  validateEndpoint,
}

var discoveryVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify existing providers",
	Long:  "Run validation on all existing providers in the database",
	RunE:  verifyExistingProviders,
}

func init() {
	DiscoveryCmd.AddCommand(discoveryRunCmd)
	DiscoveryCmd.AddCommand(discoveryStartCmd)
	DiscoveryCmd.AddCommand(discoveryStatsCmd)
	DiscoveryCmd.AddCommand(discoveryValidateCmd)
	DiscoveryCmd.AddCommand(discoveryVerifyCmd)

	// Flags for run command
	discoveryRunCmd.Flags().Bool("github", true, "Enable GitHub discovery")
	discoveryRunCmd.Flags().Bool("scraper", true, "Enable web scraper")
	discoveryRunCmd.Flags().String("github-token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	discoveryRunCmd.Flags().Int("max-concurrent", 5, "Maximum concurrent validations")
	discoveryRunCmd.Flags().Float64("min-score", 0.6, "Minimum health score to accept")

	// Flags for start command
	discoveryStartCmd.Flags().Duration("interval", 24*time.Hour, "Discovery interval")
	discoveryStartCmd.Flags().Bool("github", true, "Enable GitHub discovery")
	discoveryStartCmd.Flags().Bool("scraper", true, "Enable web scraper")
	discoveryStartCmd.Flags().String("github-token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")

	// Flags for validate command
	discoveryValidateCmd.Flags().String("auth", "api_key", "Auth type (none, api_key, bearer)")
	discoveryValidateCmd.Flags().Duration("timeout", 30*time.Second, "Validation timeout")
}

func runDiscoveryOnce(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logger
	logger := setupLogger(cmd)

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Get flags
	githubEnabled, _ := cmd.Flags().GetBool("github")
	scraperEnabled, _ := cmd.Flags().GetBool("scraper")
	githubToken, _ := cmd.Flags().GetString("github-token")
	maxConcurrent, _ := cmd.Flags().GetInt("max-concurrent")
	minScore, _ := cmd.Flags().GetFloat64("min-score")

	// Create discovery config
	discoveryConfig := &discovery.DiscoveryConfig{
		Enabled:           true,
		Interval:          0, // Not used for single run
		GitHubToken:       githubToken,
		GitHubEnabled:     githubEnabled,
		ScraperEnabled:    scraperEnabled,
		MaxConcurrent:     maxConcurrent,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    minScore,
	}

	// Create engine
	engine := discovery.NewEngine(discoveryConfig, db, logger)

	logger.Info().Msg("Starting discovery run...")

	// Run discovery
	ctx := context.Background()
	if err := engine.RunDiscovery(ctx); err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	logger.Info().Msg("Discovery run completed successfully")

	// Show stats
	return showDiscoveryStatsImpl(db, logger)
}

func runDiscoveryService(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logger
	logger := setupLogger(cmd)

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Get flags
	interval, _ := cmd.Flags().GetDuration("interval")
	githubEnabled, _ := cmd.Flags().GetBool("github")
	scraperEnabled, _ := cmd.Flags().GetBool("scraper")
	githubToken, _ := cmd.Flags().GetString("github-token")

	// Create discovery config
	discoveryConfig := &discovery.DiscoveryConfig{
		Enabled:           true,
		Interval:          interval,
		GitHubToken:       githubToken,
		GitHubEnabled:     githubEnabled,
		ScraperEnabled:    scraperEnabled,
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
	}

	// Create engine
	engine := discovery.NewEngine(discoveryConfig, db, logger)

	logger.Info().
		Dur("interval", interval).
		Bool("github", githubEnabled).
		Bool("scraper", scraperEnabled).
		Msg("Starting discovery service...")

	// Start engine
	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	// Wait for interrupt
	logger.Info().Msg("Discovery service running. Press Ctrl+C to stop.")
	<-ctx.Done()

	engine.Stop()
	logger.Info().Msg("Discovery service stopped")

	return nil
}

func showDiscoveryStats(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logger
	logger := setupLogger(cmd)

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	return showDiscoveryStatsImpl(db, logger)
}

func showDiscoveryStatsImpl(db *database.DB, logger zerolog.Logger) error {
	stats, err := discovery.GetDiscoveryStats(db)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Println("\n=== Discovery Statistics ===\n")

	// By source
	if bySource, ok := stats["by_source"].(map[string]int64); ok {
		fmt.Println("Providers by source:")
		for source, count := range bySource {
			fmt.Printf("  %-15s: %d\n", source, count)
		}
		fmt.Println()
	}

	// By status
	if byStatus, ok := stats["by_status"].(map[string]int64); ok {
		fmt.Println("Providers by status:")
		for status, count := range byStatus {
			fmt.Printf("  %-15s: %d\n", status, count)
		}
		fmt.Println()
	}

	// Recent discoveries
	if recent, ok := stats["discovered_last_7_days"].(int64); ok {
		fmt.Printf("Discovered in last 7 days: %d\n", recent)
	}

	// Average health score
	if avgScore, ok := stats["avg_health_score"].(float64); ok {
		fmt.Printf("Average health score: %.2f\n", avgScore)
	}

	fmt.Println()

	return nil
}

func validateEndpoint(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Setup logger
	logger := setupLogger(cmd)

	// Get flags
	authTypeStr, _ := cmd.Flags().GetString("auth")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Parse auth type
	var authType string
	switch authTypeStr {
	case "none":
		authType = "none"
	case "api_key":
		authType = "api_key"
	case "bearer":
		authType = "bearer"
	default:
		authType = "api_key"
	}

	// Create validator
	validator := discovery.NewValidator(timeout, logger)

	logger.Info().
		Str("url", url).
		Str("auth", authType).
		Msg("Validating endpoint...")

	// Validate
	ctx := context.Background()
	result, err := validator.ValidateEndpoint(ctx, url, authType)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Display results
	fmt.Println("\n=== Validation Results ===\n")
	fmt.Printf("URL:              %s\n", url)
	fmt.Printf("Valid:            %v\n", result.IsValid)
	fmt.Printf("Health Score:     %.2f\n", result.HealthScore)
	fmt.Printf("Latency:          %dms\n", result.LatencyMs)
	fmt.Printf("Compatibility:    %s\n", result.Compatibility)
	fmt.Printf("Supports Stream:  %v\n", result.SupportsStreaming)
	fmt.Printf("Supports JSON:    %v\n", result.SupportsJSON)
	fmt.Printf("Supports Tools:   %v\n", result.SupportsTools)

	if len(result.AvailableModels) > 0 {
		fmt.Printf("\nAvailable Models:\n")
		for _, model := range result.AvailableModels {
			fmt.Printf("  - %s\n", model)
		}
	}

	if result.ErrorMessage != "" {
		fmt.Printf("\nError: %s\n", result.ErrorMessage)
	}

	fmt.Println()

	return nil
}

func verifyExistingProviders(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logger
	logger := setupLogger(cmd)

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Create validator
	validator := discovery.NewValidator(30*time.Second, logger)

	logger.Info().Msg("Verifying existing providers...")

	// Run verification
	ctx := context.Background()
	if err := discovery.VerifyExistingProviders(ctx, db, validator, logger); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	logger.Info().Msg("Verification completed successfully")

	// Show updated stats
	return showDiscoveryStatsImpl(db, logger)
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	configPath, _ := cmd.Flags().GetString("config")
	return config.Load(configPath)
}

func setupLogger(cmd *cobra.Command) zerolog.Logger {
	logLevel, _ := cmd.Flags().GetString("log-level")

	level := zerolog.InfoLevel
	switch logLevel {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	return zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Logger()
}
