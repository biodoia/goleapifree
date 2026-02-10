package main

import (
	"fmt"
	"os"

	"github.com/biodoia/goleapifree/cmd/backend/commands"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "goleapai",
		Short: "GoLeapAI - Unified LLM Gateway",
		Long: `GoLeapAI - The Ultimate Free LLM Gateway

A high-performance gateway that aggregates all free LLM API providers
with intelligent routing, failover, and cost optimization.

Features:
  • Unified OpenAI-compatible API
  • Auto-discovery of free providers
  • Intelligent routing and load balancing
  • Built-in failover and retry logic
  • Comprehensive statistics and monitoring
  • HTTP/3 support for maximum performance`,
		Version: fmt.Sprintf("%s (commit: %s)", version, commit),
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Log level (debug, info, warn, error)")

	// Add all commands
	rootCmd.AddCommand(commands.ServeCmd)
	rootCmd.AddCommand(commands.ProvidersCmd)
	rootCmd.AddCommand(commands.StatsCmd)
	rootCmd.AddCommand(commands.ConfigCmd)
	rootCmd.AddCommand(commands.ConfigureCmd)
	rootCmd.AddCommand(commands.MigrateCmd)
	rootCmd.AddCommand(commands.DoctorCmd)
	rootCmd.AddCommand(commands.DiscoveryCmd)

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("GoLeapAI version %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
		},
	})

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
