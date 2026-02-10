package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/spf13/cobra"
)

// DoctorCmd rappresenta il comando doctor
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health diagnostics",
	Long: `Run comprehensive health checks on the GoLeapAI system.

This command checks database connectivity, Redis connection, provider health,
and overall system status to identify any issues.`,
	Example: `  # Run full diagnostic
  goleapai doctor

  # Check only database
  goleapai doctor --check database

  # Check specific provider
  goleapai doctor --provider groq

  # Verbose output
  goleapai doctor --verbose`,
	RunE: runDoctor,
}

var (
	doctorCheck    string
	doctorProvider string
	doctorVerbose  bool
)

func init() {
	DoctorCmd.Flags().StringVar(&doctorCheck, "check", "", "Run specific check (database, redis, providers)")
	DoctorCmd.Flags().StringVar(&doctorProvider, "provider", "", "Check specific provider")
	DoctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Verbose output")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("GoLeapAI System Health Check")
	fmt.Println("============================")
	fmt.Println()

	checks := map[string]func(*cobra.Command) error{
		"database":  checkDatabase,
		"redis":     checkRedis,
		"providers": checkProviders,
	}

	// Run specific check or all checks
	if doctorCheck != "" {
		if checkFunc, ok := checks[doctorCheck]; ok {
			return checkFunc(cmd)
		}
		return fmt.Errorf("unknown check: %s", doctorCheck)
	}

	// Run all checks
	results := make(map[string]bool)
	for name, checkFunc := range checks {
		err := checkFunc(cmd)
		results[name] = err == nil
		fmt.Println()
	}

	// Print summary
	fmt.Println("Summary")
	fmt.Println("-------")
	allPassed := true
	for name, passed := range results {
		status := "✓ PASS"
		if !passed {
			status = "✗ FAIL"
			allPassed = false
		}
		fmt.Printf("%-15s %s\n", name+":", status)
	}

	fmt.Println()
	if allPassed {
		fmt.Println("✓ All checks passed - system is healthy")
		return nil
	} else {
		fmt.Println("✗ Some checks failed - please review errors above")
		return fmt.Errorf("health check failed")
	}
}

func checkDatabase(cmd *cobra.Command) error {
	fmt.Println("[1/3] Database Health Check")
	fmt.Println("---------------------------")

	db, err := initDB(cmd)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		return err
	}
	defer db.Close()

	fmt.Println("✓ Database connection established")

	// Ping database
	sqlDB, err := db.DB.DB()
	if err != nil {
		fmt.Printf("✗ Failed to get database instance: %v\n", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		fmt.Printf("✗ Ping failed: %v\n", err)
		return err
	}

	fmt.Println("✓ Database ping successful")

	// Check connection stats
	stats := sqlDB.Stats()
	if doctorVerbose {
		fmt.Printf("  Open connections: %d\n", stats.OpenConnections)
		fmt.Printf("  In use: %d\n", stats.InUse)
		fmt.Printf("  Idle: %d\n", stats.Idle)
		fmt.Printf("  Wait count: %d\n", stats.WaitCount)
		fmt.Printf("  Max idle closed: %d\n", stats.MaxIdleClosed)
	}

	// Check if tables exist
	requiredTables := []interface{}{
		&models.Provider{},
		&models.Model{},
		&models.Account{},
	}

	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			fmt.Printf("✗ Missing table: %T\n", table)
			return fmt.Errorf("database schema incomplete")
		}
	}

	fmt.Println("✓ All required tables present")

	// Count providers
	var providerCount int64
	db.Model(&models.Provider{}).Count(&providerCount)
	fmt.Printf("✓ Found %d providers in database\n", providerCount)

	if providerCount == 0 {
		fmt.Println("⚠️  Warning: No providers found - run 'goleapai migrate seed'")
	}

	return nil
}

func checkRedis(cmd *cobra.Command) error {
	fmt.Println("[2/3] Redis Health Check")
	fmt.Println("------------------------")

	// TODO: Implement actual Redis check when Redis client is added
	fmt.Println("⚠️  Redis check not implemented yet")
	fmt.Println("   (Redis is optional for basic operation)")

	return nil
}

func checkProviders(cmd *cobra.Command) error {
	fmt.Println("[3/3] Provider Health Check")
	fmt.Println("---------------------------")

	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	var providers []models.Provider

	if doctorProvider != "" {
		// Check specific provider
		if err := db.Where("name = ?", doctorProvider).Find(&providers).Error; err != nil {
			return fmt.Errorf("failed to fetch provider: %w", err)
		}
		if len(providers) == 0 {
			return fmt.Errorf("provider not found: %s", doctorProvider)
		}
	} else {
		// Check all active providers
		if err := db.Where("status = ?", models.ProviderStatusActive).Find(&providers).Error; err != nil {
			return fmt.Errorf("failed to fetch providers: %w", err)
		}
	}

	if len(providers) == 0 {
		fmt.Println("⚠️  No active providers found")
		return nil
	}

	fmt.Printf("Checking %d provider(s)...\n\n", len(providers))

	healthyCount := 0
	for _, provider := range providers {
		fmt.Printf("Provider: %s\n", provider.Name)
		fmt.Printf("  URL: %s\n", provider.BaseURL)
		fmt.Printf("  Status: %s\n", provider.Status)
		fmt.Printf("  Type: %s\n", provider.Type)

		// Perform HTTP health check
		healthy := performProviderHealthCheck(&provider)
		if healthy {
			fmt.Println("  Health: ✓ OK")
			healthyCount++
		} else {
			fmt.Println("  Health: ✗ UNREACHABLE")
		}

		if doctorVerbose {
			fmt.Printf("  Last check: %s\n", formatTimeSince(provider.LastHealthCheck))
			fmt.Printf("  Health score: %.2f\n", provider.HealthScore)
			fmt.Printf("  Avg latency: %dms\n", provider.AvgLatencyMs)
		}

		fmt.Println()
	}

	fmt.Printf("Summary: %d/%d providers healthy\n", healthyCount, len(providers))

	if healthyCount < len(providers) {
		return fmt.Errorf("some providers are unhealthy")
	}

	return nil
}

func performProviderHealthCheck(provider *models.Provider) bool {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try to connect to provider
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", provider.BaseURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider any response (even errors) as "reachable"
	// A 404 or 401 means the server is responding
	return resp.StatusCode > 0
}
