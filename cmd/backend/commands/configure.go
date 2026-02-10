package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// ConfigureCmd rappresenta il comando configure
var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure CLI coding tools",
	Long: `Auto-configure CLI coding tools to use GoLeapAI gateway.

This command detects installed CLI coding tools like Claude Code, Continue,
Cursor, Aider, and Codex, then automatically configures them to use the
GoLeapAI gateway as their backend.`,
	Example: `  # Configure all detected tools
  goleapai configure --all

  # Test connectivity only
  goleapai configure --test

  # Preview changes without applying
  goleapai configure --dry-run

  # Configure without backup
  goleapai configure --all --no-backup

  # Verbose output
  goleapai configure --all -v`,
	RunE: runConfigureTool,
}

var (
	configureAll    bool
	configureBackup bool
	configureTest   bool
	configureDryRun bool
	configureVerbose bool
)

func init() {
	ConfigureCmd.Flags().BoolVarP(&configureAll, "all", "a", false, "Configure all detected tools")
	ConfigureCmd.Flags().BoolVarP(&configureBackup, "backup", "b", true, "Backup existing configurations")
	ConfigureCmd.Flags().BoolVarP(&configureTest, "test", "t", false, "Test connectivity only, don't configure")
	ConfigureCmd.Flags().BoolVarP(&configureDryRun, "dry-run", "d", false, "Show what would be done without making changes")
	ConfigureCmd.Flags().BoolVarP(&configureVerbose, "verbose", "v", false, "Verbose output")
}

func runConfigureTool(cmd *cobra.Command, args []string) error {
	// Since we have the logic in cmd/configure/main.go, we'll create a symlink
	// or just inform the user to use the dedicated binary

	fmt.Println("🔧 GoLeapAI Configuration Tool")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("For full configuration capabilities, please build and run:")
	fmt.Println()
	fmt.Println("  $ cd cmd/configure")
	fmt.Println("  $ go build -o goleapai-configure")
	fmt.Println("  $ ./goleapai-configure --all")
	fmt.Println()
	fmt.Println("Or add it to your PATH:")
	fmt.Println()
	fmt.Println("  $ go build -o $GOPATH/bin/goleapai-configure ./cmd/configure")
	fmt.Println("  $ goleapai-configure --all")
	fmt.Println()

	// For convenience, we can also provide basic functionality here
	return runBasicConfigure()
}

func runBasicConfigure() error {
	fmt.Println("📡 Detecting CLI coding tools...")
	fmt.Println()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Check for common tools
	tools := []struct {
		name       string
		paths      []string
		configFile string
	}{
		{
			name: "Claude Code",
			paths: []string{
				filepath.Join(homeDir, ".config", "claude-code"),
				filepath.Join(homeDir, ".claude-code"),
			},
			configFile: "config.json",
		},
		{
			name: "Continue",
			paths: []string{
				filepath.Join(homeDir, ".continue"),
				filepath.Join(homeDir, ".config", "continue"),
			},
			configFile: "config.json",
		},
		{
			name: "Cursor",
			paths: []string{
				filepath.Join(homeDir, ".cursor"),
				filepath.Join(homeDir, ".config", "cursor"),
			},
			configFile: "User/settings.json",
		},
		{
			name: "Aider",
			paths: []string{
				filepath.Join(homeDir, ".aider"),
				filepath.Join(homeDir, ".config", "aider"),
			},
			configFile: "config.yml",
		},
	}

	found := false

	for _, tool := range tools {
		for _, path := range tool.paths {
			if _, err := os.Stat(path); err == nil {
				found = true
				configPath := filepath.Join(path, tool.configFile)
				exists := ""
				if _, err := os.Stat(configPath); err == nil {
					exists = "(config exists)"
				} else {
					exists = "(no config)"
				}

				fmt.Printf("✅ Found %s: %s %s\n", tool.name, path, exists)
				break
			}
		}
	}

	if !found {
		fmt.Println("⚠️  No CLI coding tools detected")
		fmt.Println()
		fmt.Println("Supported tools:")
		fmt.Println("  - Claude Code (~/.config/claude-code)")
		fmt.Println("  - Continue (~/.continue)")
		fmt.Println("  - Cursor (~/.cursor)")
		fmt.Println("  - Aider (~/.aider)")
		return nil
	}

	fmt.Println()
	fmt.Println("💡 Configuration Instructions:")
	fmt.Println()
	fmt.Println("For each tool, add the following settings:")
	fmt.Println()
	fmt.Println("API Endpoint: http://localhost:8090/v1")
	fmt.Println("API Key:      goleapai-free-tier")
	fmt.Println()
	fmt.Println("Example configurations:")
	fmt.Println()
	fmt.Println("Claude Code (config.json):")
	fmt.Println(`{
  "apiEndpoint": "http://localhost:8090/v1",
  "apiKey": "goleapai-free-tier",
  "model": "gpt-4o"
}`)
	fmt.Println()
	fmt.Println("Continue (config.json):")
	fmt.Println(`{
  "models": [{
    "title": "GoLeapAI",
    "provider": "openai",
    "model": "gpt-4o",
    "apiKey": "goleapai-free-tier",
    "apiBase": "http://localhost:8090/v1"
  }]
}`)
	fmt.Println()
	fmt.Println("Cursor (User/settings.json):")
	fmt.Println(`{
  "cursor.openaiBaseUrl": "http://localhost:8090/v1",
  "cursor.openaiApiKey": "goleapai-free-tier"
}`)
	fmt.Println()

	return nil
}
