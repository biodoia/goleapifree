package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// ProvidersCmd rappresenta il comando providers
var ProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage LLM providers",
	Long: `Manage LLM provider configurations.

This command allows you to list, add, remove, test, and sync providers
with the GoLeapAI gateway.`,
	Example: `  # List all providers
  goleapai providers list

  # Add a new provider
  goleapai providers add --name "MyAPI" --url "https://api.example.com"

  # Test provider connection
  goleapai providers test groq

  # Force sync from discovery
  goleapai providers sync`,
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all providers",
	Long:  `Display a list of all configured LLM providers with their status and metrics.`,
	Example: `  # List all providers
  goleapai providers list

  # List only active providers
  goleapai providers list --status active

  # List with JSON output
  goleapai providers list --json`,
	RunE: runProvidersList,
}

var providersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new provider",
	Long:  `Manually add a new LLM provider to the gateway.`,
	Example: `  # Add a new provider
  goleapai providers add --name "MyAPI" --url "https://api.example.com" --type free

  # Add with authentication
  goleapai providers add --name "MyAPI" --url "https://api.example.com" --auth-type api_key`,
	RunE: runProvidersAdd,
}

var providersRemoveCmd = &cobra.Command{
	Use:   "remove [provider-name]",
	Short: "Remove a provider",
	Long:  `Remove a provider from the gateway by name or ID.`,
	Example: `  # Remove by name
  goleapai providers remove groq

  # Remove by ID
  goleapai providers remove 550e8400-e29b-41d4-a716-446655440000`,
	Args: cobra.ExactArgs(1),
	RunE: runProvidersRemove,
}

var providersTestCmd = &cobra.Command{
	Use:   "test [provider-name]",
	Short: "Test provider connection",
	Long:  `Test connectivity and health of a specific provider.`,
	Example: `  # Test a single provider
  goleapai providers test groq

  # Test all providers
  goleapai providers test --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProvidersTest,
}

var providersSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync providers from discovery",
	Long:  `Force synchronization of providers from auto-discovery sources.`,
	Example: `  # Sync all providers
  goleapai providers sync

  # Sync from specific source
  goleapai providers sync --source github`,
	RunE: runProvidersSync,
}

var (
	providerName     string
	providerURL      string
	providerType     string
	providerAuthType string
	providerStatus   string
	jsonOutput       bool
	testAll          bool
	syncSource       string
)

func init() {
	// List flags
	providersListCmd.Flags().StringVar(&providerStatus, "status", "", "Filter by status (active, deprecated, down)")
	providersListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Add flags
	providersAddCmd.Flags().StringVar(&providerName, "name", "", "Provider name (required)")
	providersAddCmd.Flags().StringVar(&providerURL, "url", "", "Provider base URL (required)")
	providersAddCmd.Flags().StringVar(&providerType, "type", "free", "Provider type (free, freemium, paid)")
	providersAddCmd.Flags().StringVar(&providerAuthType, "auth-type", "api_key", "Authentication type (none, api_key, bearer, oauth2)")
	providersAddCmd.MarkFlagRequired("name")
	providersAddCmd.MarkFlagRequired("url")

	// Test flags
	providersTestCmd.Flags().BoolVar(&testAll, "all", false, "Test all providers")

	// Sync flags
	providersSyncCmd.Flags().StringVar(&syncSource, "source", "", "Sync from specific source (github, scraper)")

	// Add subcommands
	ProvidersCmd.AddCommand(providersListCmd)
	ProvidersCmd.AddCommand(providersAddCmd)
	ProvidersCmd.AddCommand(providersRemoveCmd)
	ProvidersCmd.AddCommand(providersTestCmd)
	ProvidersCmd.AddCommand(providersSyncCmd)
}

func runProvidersList(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	var providers []models.Provider
	query := db.DB

	if providerStatus != "" {
		query = query.Where("status = ?", providerStatus)
	}

	if err := query.Find(&providers).Error; err != nil {
		return fmt.Errorf("failed to fetch providers: %w", err)
	}

	if jsonOutput {
		return printJSON(providers)
	}

	return printProvidersTable(providers)
}

func runProvidersAdd(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	provider := models.Provider{
		Name:        providerName,
		BaseURL:     providerURL,
		Type:        models.ProviderType(providerType),
		Status:      models.ProviderStatusActive,
		AuthType:    models.AuthType(providerAuthType),
		Tier:        3,
		DiscoveredAt: time.Now(),
		Source:      "manual",
		HealthScore: 1.0,
		SupportsStreaming: true,
		SupportsJSON: true,
	}

	if err := db.Create(&provider).Error; err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	fmt.Printf("✓ Provider '%s' added successfully (ID: %s)\n", provider.Name, provider.ID)
	return nil
}

func runProvidersRemove(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	identifier := args[0]

	// Try to parse as UUID first
	var provider models.Provider
	if id, err := uuid.Parse(identifier); err == nil {
		if err := db.First(&provider, "id = ?", id).Error; err != nil {
			return fmt.Errorf("provider not found: %s", identifier)
		}
	} else {
		// Try by name
		if err := db.First(&provider, "name = ?", identifier).Error; err != nil {
			return fmt.Errorf("provider not found: %s", identifier)
		}
	}

	if err := db.Delete(&provider).Error; err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	fmt.Printf("✓ Provider '%s' removed successfully\n", provider.Name)
	return nil
}

func runProvidersTest(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	if testAll {
		var providers []models.Provider
		if err := db.Where("status = ?", models.ProviderStatusActive).Find(&providers).Error; err != nil {
			return fmt.Errorf("failed to fetch providers: %w", err)
		}

		fmt.Printf("Testing %d providers...\n\n", len(providers))
		for _, provider := range providers {
			testProvider(&provider)
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("provider name required (or use --all)")
	}

	var provider models.Provider
	if err := db.First(&provider, "name = ?", args[0]).Error; err != nil {
		return fmt.Errorf("provider not found: %s", args[0])
	}

	return testProvider(&provider)
}

func runProvidersSync(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("🔄 Syncing providers from discovery sources...")

	// TODO: Implement actual discovery sync
	// For now, just run seed
	if err := db.Seed(); err != nil {
		return fmt.Errorf("failed to sync providers: %w", err)
	}

	fmt.Println("✓ Providers synchronized successfully")
	return nil
}

func testProvider(provider *models.Provider) error {
	fmt.Printf("Testing provider: %s\n", provider.Name)
	fmt.Printf("  URL: %s\n", provider.BaseURL)
	fmt.Printf("  Status: %s\n", provider.Status)
	fmt.Printf("  Health Score: %.2f\n", provider.HealthScore)

	// TODO: Implement actual HTTP health check
	fmt.Println("  Result: ✓ OK (mock test)")
	fmt.Println()

	return nil
}

func printProvidersTable(providers []models.Provider) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTYPE\tHEALTH\tLATENCY\tLAST CHECK\tURL")
	fmt.Fprintln(w, "----\t------\t----\t------\t-------\t----------\t---")

	for _, p := range providers {
		lastCheck := "never"
		if !p.LastHealthCheck.IsZero() {
			lastCheck = formatTimeSince(p.LastHealthCheck)
		}

		latency := "-"
		if p.AvgLatencyMs > 0 {
			latency = fmt.Sprintf("%dms", p.AvgLatencyMs)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%s\t%s\t%s\n",
			p.Name,
			p.Status,
			p.Type,
			p.HealthScore,
			latency,
			lastCheck,
			p.BaseURL,
		)
	}

	return w.Flush()
}

func printJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func formatTimeSince(t time.Time) string {
	duration := time.Since(t)
	if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
}

func initDB(cmd *cobra.Command) (*database.DB, error) {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return database.New(&cfg.Database)
}
