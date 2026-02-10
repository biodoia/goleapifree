package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	configAll    bool
	backupConfig bool
	testOnly     bool
	dryRun       bool
	verbose      bool
)

func main() {
	// Setup logger
	setupLogger()

	rootCmd := &cobra.Command{
		Use:   "configure",
		Short: "GoLeapAI Configuration Tool",
		Long:  `Auto-configure CLI coding tools to use GoLeapAI gateway`,
		RunE:  runConfigure,
	}

	rootCmd.Flags().BoolVarP(&configAll, "all", "a", false, "Configure all detected tools")
	rootCmd.Flags().BoolVarP(&backupConfig, "backup", "b", true, "Backup existing configurations")
	rootCmd.Flags().BoolVarP(&testOnly, "test", "t", false, "Test connectivity only, don't configure")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Show what would be done without making changes")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runConfigure(cmd *cobra.Command, args []string) error {
	log.Info().Msg("🔧 GoLeapAI Auto-Configuration Tool")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Detect installed tools
	log.Info().Msg("\n📡 Detecting installed CLI coding tools...")
	tools, err := DetectAllTools()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	if len(tools) == 0 {
		log.Warn().Msg("⚠️  No CLI coding tools detected")
		log.Info().Msg("\nSupported tools:")
		log.Info().Msg("  - Claude Code (~/.config/claude-code)")
		log.Info().Msg("  - Continue (~/.continue)")
		log.Info().Msg("  - Cursor (~/.cursor)")
		log.Info().Msg("  - Aider (~/.aider)")
		log.Info().Msg("  - Codex (~/.codex)")
		return nil
	}

	// Display detected tools
	log.Info().Msgf("\n✅ Found %d tool(s):", len(tools))
	for _, tool := range tools {
		status := "not configured"
		if tool.IsConfigured {
			status = "already configured"
		}
		log.Info().Msgf("  • %s (%s) - %s", tool.Name, tool.ConfigPath, status)
	}

	// Test mode
	if testOnly {
		log.Info().Msg("\n🧪 Testing connectivity...")
		return testConnectivity()
	}

	// Dry run mode
	if dryRun {
		log.Info().Msg("\n🔍 DRY RUN - No changes will be made")
		return previewConfiguration(tools)
	}

	// Configure tools
	log.Info().Msg("\n⚙️  Configuring tools...")

	var configured []string
	var failed []string

	for _, tool := range tools {
		if !configAll && tool.IsConfigured {
			log.Info().Msgf("⏭️  Skipping %s (already configured)", tool.Name)
			continue
		}

		log.Info().Msgf("🔧 Configuring %s...", tool.Name)

		// Backup existing config
		if backupConfig && tool.IsConfigured {
			if err := BackupConfig(tool); err != nil {
				log.Warn().Err(err).Msgf("Failed to backup %s config", tool.Name)
			} else {
				log.Info().Msgf("  ✓ Backup created")
			}
		}

		// Generate configuration
		if err := GenerateConfig(tool); err != nil {
			log.Error().Err(err).Msgf("  ✗ Failed to configure %s", tool.Name)
			failed = append(failed, tool.Name)
			continue
		}

		log.Info().Msgf("  ✓ %s configured successfully", tool.Name)
		configured = append(configured, tool.Name)
	}

	// Test connectivity
	log.Info().Msg("\n🧪 Testing connectivity...")
	if err := testConnectivity(); err != nil {
		log.Warn().Err(err).Msg("Connectivity test failed")
	}

	// Summary
	log.Info().Msg("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Info().Msg("📊 Configuration Summary")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(configured) > 0 {
		log.Info().Msgf("✅ Successfully configured: %v", configured)
	}

	if len(failed) > 0 {
		log.Warn().Msgf("❌ Failed to configure: %v", failed)
	}

	if len(configured) == 0 && len(failed) == 0 {
		log.Info().Msg("ℹ️  No tools were configured")
	}

	log.Info().Msg("\n💡 Next steps:")
	log.Info().Msg("  1. Start GoLeapAI gateway: goleapai")
	log.Info().Msg("  2. Use your CLI tool normally - it will use GoLeapAI!")
	log.Info().Msg("  3. Monitor traffic: goleapai stats")

	return nil
}

func previewConfiguration(tools []DetectedTool) error {
	for _, tool := range tools {
		log.Info().Msgf("\n%s:", tool.Name)
		log.Info().Msgf("  Config path: %s", tool.ConfigPath)

		config, err := PreviewConfig(tool)
		if err != nil {
			log.Error().Err(err).Msgf("  Failed to preview config")
			continue
		}

		log.Info().Msgf("  Changes to be made:")
		log.Info().Msgf("    %s", config)
	}

	return nil
}

func testConnectivity() error {
	tester := NewConnectionTester()

	results, err := tester.TestAll()
	if err != nil {
		return err
	}

	log.Info().Msg("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Info().Msg("🔌 Connectivity Test Results")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, result := range results {
		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
		}

		log.Info().Msgf("\n%s %s:", status, result.TestName)
		log.Info().Msgf("  Duration: %s", result.Duration)

		if result.Success {
			log.Info().Msgf("  Details: %s", result.Message)
		} else {
			log.Error().Msgf("  Error: %s", result.Error)
		}
	}

	return nil
}

func setupLogger() {
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
		NoColor:    false,
	})
}
