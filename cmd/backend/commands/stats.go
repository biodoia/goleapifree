package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/spf13/cobra"
)

// StatsCmd rappresenta il comando stats
var StatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View and manage statistics",
	Long: `View aggregated statistics for providers, models, and requests.

This command provides insights into usage patterns, performance metrics,
and cost savings across all providers.`,
	Example: `  # Show current statistics
  goleapai stats show

  # Export stats to CSV
  goleapai stats export --format csv -o stats.csv

  # Reset all statistics
  goleapai stats reset --confirm`,
}

var statsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show aggregated statistics",
	Long:  `Display aggregated statistics for all providers and models.`,
	Example: `  # Show all stats
  goleapai stats show

  # Show stats for specific provider
  goleapai stats show --provider groq

  # Show stats for date range
  goleapai stats show --from 2024-01-01 --to 2024-01-31

  # Show in JSON format
  goleapai stats show --json`,
	RunE: runStatsShow,
}

var statsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export statistics to file",
	Long:  `Export statistics to CSV or JSON format for analysis.`,
	Example: `  # Export to CSV
  goleapai stats export --format csv -o stats.csv

  # Export to JSON
  goleapai stats export --format json -o stats.json

  # Export specific provider
  goleapai stats export --provider groq -o groq-stats.csv`,
	RunE: runStatsExport,
}

var statsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset statistics",
	Long:  `Reset all statistics data. This action cannot be undone.`,
	Example: `  # Reset all stats (requires confirmation)
  goleapai stats reset --confirm

  # Reset stats for specific provider
  goleapai stats reset --provider groq --confirm`,
	RunE: runStatsReset,
}

var (
	statsProvider string
	statsFrom     string
	statsTo       string
	statsFormat   string
	statsOutput   string
	statsConfirm  bool
)

func init() {
	// Show flags
	statsShowCmd.Flags().StringVar(&statsProvider, "provider", "", "Filter by provider name")
	statsShowCmd.Flags().StringVar(&statsFrom, "from", "", "Start date (YYYY-MM-DD)")
	statsShowCmd.Flags().StringVar(&statsTo, "to", "", "End date (YYYY-MM-DD)")
	statsShowCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Export flags
	statsExportCmd.Flags().StringVar(&statsProvider, "provider", "", "Filter by provider name")
	statsExportCmd.Flags().StringVar(&statsFrom, "from", "", "Start date (YYYY-MM-DD)")
	statsExportCmd.Flags().StringVar(&statsTo, "to", "", "End date (YYYY-MM-DD)")
	statsExportCmd.Flags().StringVar(&statsFormat, "format", "csv", "Export format (csv, json)")
	statsExportCmd.Flags().StringVarP(&statsOutput, "output", "o", "", "Output file path (required)")
	statsExportCmd.MarkFlagRequired("output")

	// Reset flags
	statsResetCmd.Flags().StringVar(&statsProvider, "provider", "", "Reset stats for specific provider")
	statsResetCmd.Flags().BoolVar(&statsConfirm, "confirm", false, "Confirm reset action")

	// Add subcommands
	StatsCmd.AddCommand(statsShowCmd)
	StatsCmd.AddCommand(statsExportCmd)
	StatsCmd.AddCommand(statsResetCmd)
}

func runStatsShow(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	var stats []models.ProviderStats
	query := db.DB.Preload("Provider")

	// Apply filters
	if statsProvider != "" {
		query = query.Joins("JOIN providers ON providers.id = provider_stats.provider_id").
			Where("providers.name = ?", statsProvider)
	}

	if statsFrom != "" {
		fromDate, err := time.Parse("2006-01-02", statsFrom)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
		query = query.Where("timestamp >= ?", fromDate)
	}

	if statsTo != "" {
		toDate, err := time.Parse("2006-01-02", statsTo)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}
		query = query.Where("timestamp <= ?", toDate)
	}

	if err := query.Order("timestamp DESC").Limit(100).Find(&stats).Error; err != nil {
		return fmt.Errorf("failed to fetch stats: %w", err)
	}

	if jsonOutput {
		return printJSON(stats)
	}

	return printStatsTable(stats)
}

func runStatsExport(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	var stats []models.ProviderStats
	query := db.DB.Preload("Provider")

	// Apply filters (same as show)
	if statsProvider != "" {
		query = query.Joins("JOIN providers ON providers.id = provider_stats.provider_id").
			Where("providers.name = ?", statsProvider)
	}

	if statsFrom != "" {
		fromDate, err := time.Parse("2006-01-02", statsFrom)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
		query = query.Where("timestamp >= ?", fromDate)
	}

	if statsTo != "" {
		toDate, err := time.Parse("2006-01-02", statsTo)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}
		query = query.Where("timestamp <= ?", toDate)
	}

	if err := query.Order("timestamp ASC").Find(&stats).Error; err != nil {
		return fmt.Errorf("failed to fetch stats: %w", err)
	}

	switch statsFormat {
	case "csv":
		return exportStatsCSV(stats, statsOutput)
	case "json":
		return exportStatsJSON(stats, statsOutput)
	default:
		return fmt.Errorf("unsupported format: %s (use csv or json)", statsFormat)
	}
}

func runStatsReset(cmd *cobra.Command, args []string) error {
	if !statsConfirm {
		return fmt.Errorf("reset requires --confirm flag to proceed")
	}

	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	query := db.DB

	if statsProvider != "" {
		// Reset for specific provider
		query = query.Where("provider_id IN (SELECT id FROM providers WHERE name = ?)", statsProvider)
	}

	// Delete provider stats
	if err := query.Delete(&models.ProviderStats{}).Error; err != nil {
		return fmt.Errorf("failed to reset provider stats: %w", err)
	}

	// Delete request logs
	if err := query.Delete(&models.RequestLog{}).Error; err != nil {
		return fmt.Errorf("failed to reset request logs: %w", err)
	}

	if statsProvider != "" {
		fmt.Printf("✓ Statistics reset for provider: %s\n", statsProvider)
	} else {
		fmt.Println("✓ All statistics reset successfully")
	}

	return nil
}

func printStatsTable(stats []models.ProviderStats) error {
	if len(stats) == 0 {
		fmt.Println("No statistics found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tTIMESTAMP\tREQUESTS\tTOKENS\tSUCCESS RATE\tAVG LATENCY\tCOST SAVED")
	fmt.Fprintln(w, "--------\t---------\t--------\t------\t------------\t-----------\t----------")

	for _, s := range stats {
		providerName := "Unknown"
		if s.Provider.Name != "" {
			providerName = s.Provider.Name
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%.1f%%\t%dms\t$%.2f\n",
			providerName,
			s.Timestamp.Format("2006-01-02 15:04"),
			s.TotalRequests,
			s.TotalTokens,
			s.SuccessRate*100,
			s.AvgLatencyMs,
			s.CostSaved,
		)
	}

	fmt.Fprintln(w)

	// Print summary
	var totalRequests, totalTokens int64
	var totalCost float64
	for _, s := range stats {
		totalRequests += s.TotalRequests
		totalTokens += s.TotalTokens
		totalCost += s.CostSaved
	}

	fmt.Fprintf(w, "TOTAL\t\t%d\t%d\t\t\t$%.2f\n", totalRequests, totalTokens, totalCost)

	return w.Flush()
}

func exportStatsCSV(stats []models.ProviderStats, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Provider", "Timestamp", "Total Requests", "Total Tokens",
		"Success Rate", "Avg Latency (ms)", "Cost Saved",
		"Error Count", "Timeout Count", "Quota Exhausted",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, s := range stats {
		providerName := "Unknown"
		if s.Provider.Name != "" {
			providerName = s.Provider.Name
		}

		row := []string{
			providerName,
			s.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%d", s.TotalRequests),
			fmt.Sprintf("%d", s.TotalTokens),
			fmt.Sprintf("%.4f", s.SuccessRate),
			fmt.Sprintf("%d", s.AvgLatencyMs),
			fmt.Sprintf("%.2f", s.CostSaved),
			fmt.Sprintf("%d", s.ErrorCount),
			fmt.Sprintf("%d", s.TimeoutCount),
			fmt.Sprintf("%d", s.QuotaExhausted),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Exported %d records to %s\n", len(stats), filename)
	return nil
}

func exportStatsJSON(stats []models.ProviderStats, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(stats); err != nil {
		return err
	}

	fmt.Printf("✓ Exported %d records to %s\n", len(stats), filename)
	return nil
}
