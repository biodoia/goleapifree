package commands

import (
	"fmt"
	"os"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ConfigCmd rappresenta il comando config
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage GoLeapAI configuration files.

This command allows you to view, validate, and generate configuration files
for the GoLeapAI gateway.`,
	Example: `  # Show current configuration
  goleapai config show

  # Validate configuration file
  goleapai config validate -c config.yaml

  # Generate template configuration
  goleapai config generate -o config.yaml`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the currently loaded configuration with all values.`,
	Example: `  # Show default config
  goleapai config show

  # Show specific config file
  goleapai config show -c /path/to/config.yaml`,
	RunE: runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate a configuration file for syntax and semantic errors.`,
	Example: `  # Validate default config
  goleapai config validate

  # Validate specific config
  goleapai config validate -c config.yaml`,
	RunE: runConfigValidate,
}

var configGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate template configuration",
	Long:  `Generate a template configuration file with all available options.`,
	Example: `  # Generate to stdout
  goleapai config generate

  # Generate to file
  goleapai config generate -o config.yaml

  # Generate production config
  goleapai config generate --env production -o prod.yaml`,
	RunE: runConfigGenerate,
}

var (
	configOutput string
	configEnv    string
)

func init() {
	configGenerateCmd.Flags().StringVarP(&configOutput, "output", "o", "", "Output file path (stdout if not specified)")
	configGenerateCmd.Flags().StringVar(&configEnv, "env", "development", "Environment (development, production)")

	ConfigCmd.AddCommand(configShowCmd)
	ConfigCmd.AddCommand(configValidateCmd)
	ConfigCmd.AddCommand(configGenerateCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Marshal to YAML for pretty printing
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("# Current Configuration")
	fmt.Println("# =====================")
	fmt.Println()
	fmt.Print(string(data))

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	fmt.Printf("Validating configuration: %s\n\n", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Println("✗ Failed to load configuration")
		return err
	}

	fmt.Println("✓ Configuration loaded successfully")

	if err := cfg.Validate(); err != nil {
		fmt.Println("✗ Configuration validation failed")
		return err
	}

	fmt.Println("✓ Configuration is valid")
	fmt.Println()
	fmt.Println("Configuration summary:")
	fmt.Printf("  Server:     %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("  Database:   %s (%s)\n", cfg.Database.Type, cfg.Database.Connection)
	fmt.Printf("  HTTP/3:     %v\n", cfg.Server.HTTP3)
	fmt.Printf("  TLS:        %v\n", cfg.Server.TLS.Enabled)
	fmt.Printf("  Prometheus: %v\n", cfg.Monitoring.Prometheus.Enabled)

	return nil
}

func runConfigGenerate(cmd *cobra.Command, args []string) error {
	cfg := generateTemplateConfig(configEnv)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comments
	output := `# GoLeapAI Configuration File
# ============================
#
# This is a template configuration for GoLeapAI Gateway.
# Adjust the values according to your environment.
#
# Environment: ` + configEnv + `

`
	output += string(data)

	// Write to file or stdout
	if configOutput != "" {
		if err := os.WriteFile(configOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
		fmt.Printf("✓ Configuration template generated: %s\n", configOutput)
	} else {
		fmt.Print(output)
	}

	return nil
}

func generateTemplateConfig(env string) *config.Config {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:  8080,
			Host:  "0.0.0.0",
			HTTP3: true,
		},
		Database: struct {
			Type       string `yaml:"type"`
			Connection string `yaml:"connection"`
			MaxConns   int    `yaml:"max_conns"`
			LogLevel   string `yaml:"log_level"`
		}{
			Type:       "sqlite",
			Connection: "./data/goleapai.db",
			MaxConns:   25,
			LogLevel:   "warn",
		},
		Redis: config.RedisConfig{
			Host:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Providers: config.ProvidersConfig{
			AutoDiscovery:       true,
			HealthCheckInterval: "5m",
			DefaultTimeout:      "30s",
		},
		Routing: config.RoutingConfig{
			Strategy:        "cost_optimized",
			FailoverEnabled: true,
			MaxRetries:      3,
		},
		Monitoring: config.MonitoringConfig{},
	}

	// Environment-specific settings
	if env == "production" {
		cfg.Server.Port = 443
		cfg.Server.TLS.Enabled = true
		cfg.Server.TLS.Cert = "/etc/goleapai/tls/cert.pem"
		cfg.Server.TLS.Key = "/etc/goleapai/tls/key.pem"
		cfg.Database.Type = "postgres"
		cfg.Database.Connection = "host=localhost user=goleapai password=changeme dbname=goleapai sslmode=require"
		cfg.Database.MaxConns = 100
		cfg.Monitoring.Logging.Level = "info"
		cfg.Monitoring.Logging.Format = "json"
	} else {
		cfg.Monitoring.Logging.Level = "debug"
		cfg.Monitoring.Logging.Format = "console"
	}

	cfg.Monitoring.Prometheus.Enabled = true
	cfg.Monitoring.Prometheus.Port = 9090

	return cfg
}
