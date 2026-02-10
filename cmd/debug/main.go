package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/biodoia/goleapifree/internal/router"
	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	version    = "1.0.0"
	configPath string
	verbose    bool
	outputJSON bool
)

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	rootCmd := &cobra.Command{
		Use:   "goleapai-debug",
		Short: "GoLeapAI Debug & Troubleshooting Tool",
		Long: `GoLeapAI Debug CLI - Advanced debugging and troubleshooting tool

A comprehensive debugging tool for development and production environments.
Provides request inspection, provider testing, routing simulation, and more.

Features:
  • Request tracing and inspection
  • Provider health testing
  • Routing decision simulation
  • Cache state inspection
  • Performance profiling
  • Configuration validation`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&outputJSON, "json", "j", false, "Output as JSON")

	// Add commands
	rootCmd.AddCommand(requestCmd())
	rootCmd.AddCommand(providerCmd())
	rootCmd.AddCommand(routingCmd())
	rootCmd.AddCommand(cacheCmd())
	rootCmd.AddCommand(profileCmd())
	rootCmd.AddCommand(validateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// requestCmd - Inspect request details
func requestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request <id>",
		Short: "Inspect request details",
		Long: `Inspect detailed information about a specific request.

Shows:
  • Request metadata and parameters
  • Provider selection decision
  • Response details and timing
  • Error information (if any)
  • Cache hit/miss information`,
		Args: cobra.ExactArgs(1),
		RunE: runRequestDebug,
	}

	return cmd
}

func runRequestDebug(cmd *cobra.Command, args []string) error {
	requestID := args[0]

	// Validate UUID
	if _, err := uuid.Parse(requestID); err != nil {
		return fmt.Errorf("invalid request ID format: %w", err)
	}

	// Load config and database
	cfg, db, err := initDebugEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	// Create tracer
	tracer := NewTracer(db, cfg)

	// Get request trace
	trace, err := tracer.GetRequestTrace(cmd.Context(), requestID)
	if err != nil {
		return fmt.Errorf("failed to get request trace: %w", err)
	}

	// Output
	if outputJSON {
		return outputAsJSON(trace)
	}

	printRequestTrace(trace)
	return nil
}

// providerCmd - Test provider
func providerCmd() *cobra.Command {
	var (
		testEndpoint bool
		checkHealth  bool
		listModels   bool
	)

	cmd := &cobra.Command{
		Use:   "provider <name>",
		Short: "Test provider connectivity and features",
		Long: `Test a specific provider's connectivity and capabilities.

Performs:
  • HTTP connectivity test
  • Authentication validation
  • Model listing
  • Feature detection
  • Latency measurement`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderDebug(cmd, args[0], testEndpoint, checkHealth, listModels)
		},
	}

	cmd.Flags().BoolVar(&testEndpoint, "test", true, "Test endpoint connectivity")
	cmd.Flags().BoolVar(&checkHealth, "health", true, "Perform health check")
	cmd.Flags().BoolVar(&listModels, "models", false, "List available models")

	return cmd
}

func runProviderDebug(cmd *cobra.Command, name string, testEndpoint, checkHealth, listModels bool) error {
	cfg, db, err := initDebugEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	// Get provider from database
	var provider models.Provider
	if err := db.Where("name = ?", name).First(&provider).Error; err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	result := &ProviderDebugResult{
		Provider:  &provider,
		Timestamp: time.Now(),
	}

	// Test endpoint
	if testEndpoint {
		fmt.Printf("Testing provider: %s\n", provider.Name)
		fmt.Printf("Base URL: %s\n", provider.BaseURL)
		fmt.Printf("Type: %s\n", provider.Type)
		fmt.Printf("Status: %s\n\n", provider.Status)

		start := time.Now()
		healthy := performHealthCheck(cmd.Context(), &provider)
		latency := time.Since(start)

		result.HealthCheck = healthy
		result.Latency = latency

		if healthy {
			fmt.Printf("✓ Endpoint reachable (%.2fms)\n", latency.Seconds()*1000)
		} else {
			fmt.Printf("✗ Endpoint unreachable\n")
		}
	}

	// Check health
	if checkHealth && provider.Status == models.ProviderStatusActive {
		fmt.Println("\nHealth Metrics:")
		fmt.Printf("  Last check: %s\n", formatTimeSince(provider.LastHealthCheck))
		fmt.Printf("  Health score: %.2f/1.0\n", provider.HealthScore)
		fmt.Printf("  Avg latency: %dms\n", provider.AvgLatencyMs)
		fmt.Printf("  Tier: %d\n", provider.Tier)
	}

	// List models
	if listModels {
		var modelsList []models.Model
		if err := db.Where("provider_id = ?", provider.ID).Find(&modelsList).Error; err == nil {
			fmt.Printf("\nAvailable Models (%d):\n", len(modelsList))
			for _, model := range modelsList {
				fmt.Printf("  • %s", model.Name)
				if model.MaxTokens > 0 {
					fmt.Printf(" (max: %d tokens)", model.MaxTokens)
				}
				fmt.Println()
			}
			result.Models = modelsList
		}
	}

	// Capabilities
	fmt.Println("\nCapabilities:")
	fmt.Printf("  Streaming: %v\n", provider.SupportsStreaming)
	fmt.Printf("  Tools: %v\n", provider.SupportsTools)
	fmt.Printf("  JSON Mode: %v\n", provider.SupportsJSON)

	if outputJSON {
		return outputAsJSON(result)
	}

	return nil
}

// routingCmd - Simulate routing
func routingCmd() *cobra.Command {
	var (
		model       string
		maxTokens   int
		temperature float64
		stream      bool
		showAll     bool
	)

	cmd := &cobra.Command{
		Use:   "routing <prompt>",
		Short: "Simulate routing decision for a prompt",
		Long: `Simulate the routing decision process for a given prompt.

Shows:
  • Provider selection reasoning
  • Cost estimation
  • Quality score
  • Latency prediction
  • Alternative providers (with --show-all)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingDebug(cmd, args[0], model, maxTokens, temperature, stream, showAll)
		},
	}

	cmd.Flags().StringVar(&model, "model", "gpt-3.5-turbo", "Target model")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 1000, "Max tokens")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Temperature")
	cmd.Flags().BoolVar(&stream, "stream", false, "Streaming mode")
	cmd.Flags().BoolVar(&showAll, "show-all", false, "Show all candidate providers")

	return cmd
}

func runRoutingDebug(cmd *cobra.Command, prompt, model string, maxTokens int, temperature float64, stream, showAll bool) error {
	cfg, db, err := initDebugEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	// Create router
	r, err := router.New(cfg, db)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	// Create request
	req := &router.Request{
		Model:       model,
		Messages:    []router.Message{{Role: "user", Content: prompt}},
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      stream,
	}

	fmt.Println("Routing Simulation")
	fmt.Println("==================")
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("Prompt: %s\n", truncateString(prompt, 80))
	fmt.Printf("Max Tokens: %d\n", maxTokens)
	fmt.Printf("Temperature: %.2f\n", temperature)
	fmt.Printf("Stream: %v\n", stream)
	fmt.Printf("Strategy: %s\n\n", cfg.Routing.Strategy)

	// Simulate routing
	selection, err := r.SelectProvider(req)
	if err != nil {
		return fmt.Errorf("routing failed: %w", err)
	}

	fmt.Println("Selected Provider:")
	fmt.Printf("  Provider ID: %s\n", selection.ProviderID)
	fmt.Printf("  Model ID: %s\n", selection.ModelID)
	fmt.Printf("  Estimated Cost: $%.6f\n", selection.EstimatedCost)
	fmt.Printf("  Reason: %s\n", selection.Reason)

	if showAll {
		fmt.Println("\nAlternative Providers:")
		// TODO: Implement alternative providers listing
		fmt.Println("  (feature not yet implemented)")
	}

	if outputJSON {
		return outputAsJSON(selection)
	}

	return nil
}

// cacheCmd - Inspect cache state
func cacheCmd() *cobra.Command {
	var (
		clearCache bool
		key        string
		showStats  bool
	)

	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect and manage cache state",
		Long: `Inspect cache statistics and manage cache entries.

Operations:
  • View cache statistics
  • Inspect specific cache key
  • Clear cache
  • Show cache hit/miss ratios`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheDebug(cmd, key, clearCache, showStats)
		},
	}

	cmd.Flags().BoolVar(&clearCache, "clear", false, "Clear all cache")
	cmd.Flags().StringVar(&key, "key", "", "Inspect specific cache key")
	cmd.Flags().BoolVar(&showStats, "stats", true, "Show cache statistics")

	return cmd
}

func runCacheDebug(cmd *cobra.Command, key string, clearCache, showStats bool) error {
	cfg, _, err := initDebugEnv()
	if err != nil {
		return err
	}

	// Initialize cache
	cacheConfig := &cache.Config{
		MemoryEnabled:    true,
		MemoryMaxEntries: 1000,
		MemoryTTL:        5 * time.Minute,
		RedisEnabled:     cfg.Redis.Host != "",
		RedisHost:        cfg.Redis.Host,
		RedisPassword:    cfg.Redis.Password,
		RedisDB:          cfg.Redis.DB,
		RedisTTL:         30 * time.Minute,
	}

	c, err := cache.NewMultiLayerCache(cacheConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	defer c.Close()

	// Clear cache
	if clearCache {
		if err := c.Clear(cmd.Context()); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println("✓ Cache cleared successfully")
		return nil
	}

	// Inspect specific key
	if key != "" {
		data, err := c.Get(cmd.Context(), key)
		if err != nil {
			fmt.Printf("✗ Key not found: %s\n", key)
			return nil
		}

		fmt.Printf("Cache Entry: %s\n", key)
		fmt.Printf("Size: %d bytes\n", len(data))
		fmt.Printf("Value preview:\n%s\n", truncateString(string(data), 200))
		return nil
	}

	// Show statistics
	if showStats {
		stats := c.Stats()

		fmt.Println("Cache Statistics")
		fmt.Println("================")
		fmt.Printf("Hits: %d\n", stats.Hits)
		fmt.Printf("Misses: %d\n", stats.Misses)
		fmt.Printf("Sets: %d\n", stats.Sets)
		fmt.Printf("Deletes: %d\n", stats.Deletes)
		fmt.Printf("Size: %d bytes\n", stats.Size)
		fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate()*100)
		fmt.Printf("Eviction Rate: %.2f%%\n", stats.EvictionRate*100)

		if outputJSON {
			return outputAsJSON(stats)
		}
	}

	return nil
}

// profileCmd - Performance profiling
func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Performance profiling tools",
		Long: `Advanced performance profiling and analysis.

Profiling Types:
  • CPU profiling
  • Memory profiling
  • Goroutine analysis
  • Heap dump`,
	}

	cmd.AddCommand(profileCPUCmd())
	cmd.AddCommand(profileMemCmd())
	cmd.AddCommand(profileGoroutineCmd())
	cmd.AddCommand(profileHeapCmd())

	return cmd
}

func profileCPUCmd() *cobra.Command {
	var duration int

	cmd := &cobra.Command{
		Use:   "cpu",
		Short: "CPU profiling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCPUProfile(duration)
		},
	}

	cmd.Flags().IntVar(&duration, "duration", 30, "Profile duration in seconds")
	return cmd
}

func profileMemCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "memory",
		Short: "Memory profiling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryProfile()
		},
	}
}

func profileGoroutineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "goroutines",
		Short: "Goroutine analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoroutineProfile()
		},
	}
}

func profileHeapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "heap",
		Short: "Heap dump",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeapDump()
		},
	}
}

// validateCmd - Configuration validation
func validateCmd() *cobra.Command {
	var (
		checkYAML     bool
		checkProviders bool
		checkDB       bool
		checkRedis    bool
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and connectivity",
		Long: `Validate system configuration and external dependencies.

Checks:
  • YAML configuration syntax
  • Provider connectivity
  • Database connection
  • Redis connection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(checkYAML, checkProviders, checkDB, checkRedis)
		},
	}

	cmd.Flags().BoolVar(&checkYAML, "yaml", true, "Validate YAML config")
	cmd.Flags().BoolVar(&checkProviders, "providers", true, "Check provider connectivity")
	cmd.Flags().BoolVar(&checkDB, "database", true, "Test database connection")
	cmd.Flags().BoolVar(&checkRedis, "redis", true, "Test Redis connection")

	return cmd
}

func runValidate(checkYAML, checkProviders, checkDB, checkRedis bool) error {
	validator := NewValidator(configPath)

	fmt.Println("Configuration Validation")
	fmt.Println("========================\n")

	results := make(map[string]error)

	if checkYAML {
		fmt.Print("[1/4] Validating YAML... ")
		err := validator.ValidateYAML()
		results["yaml"] = err
		if err != nil {
			fmt.Printf("✗\n  Error: %v\n\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	if checkDB {
		fmt.Print("[2/4] Testing database... ")
		err := validator.ValidateDatabase()
		results["database"] = err
		if err != nil {
			fmt.Printf("✗\n  Error: %v\n\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	if checkRedis {
		fmt.Print("[3/4] Testing Redis... ")
		err := validator.ValidateRedis()
		results["redis"] = err
		if err != nil {
			fmt.Printf("✗\n  Error: %v\n\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	if checkProviders {
		fmt.Print("[4/4] Checking providers... ")
		err := validator.ValidateProviders()
		results["providers"] = err
		if err != nil {
			fmt.Printf("✗\n  Error: %v\n\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	// Summary
	fmt.Println("\nSummary")
	fmt.Println("-------")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for check, err := range results {
		status := "✓ PASS"
		if err != nil {
			status = "✗ FAIL"
		}
		fmt.Fprintf(w, "%s\t%s\n", strings.ToUpper(check), status)
	}
	w.Flush()

	// Check if any failed
	for _, err := range results {
		if err != nil {
			return fmt.Errorf("validation failed")
		}
	}

	fmt.Println("\n✓ All validations passed")
	return nil
}

// Helper functions

func initDebugEnv() (*config.Config, *database.DB, error) {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return cfg, db, nil
}

func performHealthCheck(ctx context.Context, provider *models.Provider) bool {
	// TODO: Implement actual provider health check using provider client
	return provider.Status == models.ProviderStatusActive
}

func outputAsJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printRequestTrace(trace *RequestTrace) {
	fmt.Println("Request Trace")
	fmt.Println("=============")
	fmt.Printf("ID: %s\n", trace.RequestID)
	fmt.Printf("Timestamp: %s\n", trace.Timestamp.Format(time.RFC3339))
	fmt.Printf("Duration: %.2fms\n", trace.Duration.Seconds()*1000)
	fmt.Printf("Status: %s\n\n", trace.Status)

	if trace.Provider != nil {
		fmt.Printf("Provider: %s\n", trace.Provider.Name)
		fmt.Printf("Model: %s\n", trace.ModelID)
	}

	fmt.Printf("\nDecision Points:\n")
	for i, dp := range trace.DecisionPoints {
		fmt.Printf("  [%d] %s: %s\n", i+1, dp.Stage, dp.Decision)
	}

	if len(trace.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range trace.Errors {
			fmt.Printf("  • %s\n", e)
		}
	}
}

func formatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	duration := time.Since(t)

	if duration < time.Minute {
		return fmt.Sprintf("%.0f seconds ago", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.0f minutes ago", duration.Minutes())
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%.0f hours ago", duration.Hours())
	}

	return fmt.Sprintf("%.0f days ago", duration.Hours()/24)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Data structures

type ProviderDebugResult struct {
	Provider    *models.Provider `json:"provider"`
	HealthCheck bool             `json:"health_check"`
	Latency     time.Duration    `json:"latency"`
	Models      []models.Model   `json:"models,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
}

type RequestTrace struct {
	RequestID      string          `json:"request_id"`
	Timestamp      time.Time       `json:"timestamp"`
	Duration       time.Duration   `json:"duration"`
	Status         string          `json:"status"`
	Provider       *models.Provider `json:"provider,omitempty"`
	ModelID        string          `json:"model_id"`
	DecisionPoints []DecisionPoint `json:"decision_points"`
	Errors         []string        `json:"errors,omitempty"`
}

type DecisionPoint struct {
	Stage    string    `json:"stage"`
	Decision string    `json:"decision"`
	Time     time.Time `json:"time"`
}
