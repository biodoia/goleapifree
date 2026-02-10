package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Validator valida la configurazione e le connessioni
type Validator struct {
	configPath string
	config     *config.Config
}

// NewValidator crea un nuovo validator
func NewValidator(configPath string) *Validator {
	return &Validator{
		configPath: configPath,
	}
}

// ValidateYAML valida la sintassi del file YAML
func (v *Validator) ValidateYAML() error {
	log.Debug().Str("path", v.configPath).Msg("Validating YAML configuration")

	// Load config
	cfg, err := config.Load(v.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	v.config = cfg

	// Validate config file syntax
	if v.configPath != "" {
		data, err := os.ReadFile(v.configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		var rawConfig map[string]interface{}
		if err := yaml.Unmarshal(data, &rawConfig); err != nil {
			return fmt.Errorf("invalid YAML syntax: %w", err)
		}
	}

	// Validate configuration values
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Additional validations
	if err := v.validateServerConfig(&cfg.Server); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if err := v.validateRoutingConfig(&cfg.Routing); err != nil {
		return fmt.Errorf("invalid routing config: %w", err)
	}

	log.Info().Msg("YAML configuration is valid")
	return nil
}

// ValidateDatabase testa la connessione al database
func (v *Validator) ValidateDatabase() error {
	log.Debug().Msg("Validating database connection")

	cfg, err := config.Load(v.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check tables
	requiredTables := []interface{}{
		&models.Provider{},
		&models.Model{},
		&models.Account{},
	}

	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("missing required table: %T", table)
		}
	}

	// Get connection stats
	stats := sqlDB.Stats()
	log.Info().
		Int("max_open", stats.MaxOpenConnections).
		Int("open", stats.OpenConnections).
		Int("in_use", stats.InUse).
		Msg("Database connection validated")

	return nil
}

// ValidateRedis testa la connessione a Redis
func (v *Validator) ValidateRedis() error {
	log.Debug().Msg("Validating Redis connection")

	cfg, err := config.Load(v.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Skip if Redis is not configured
	if cfg.Redis.Host == "" {
		log.Info().Msg("Redis not configured, skipping validation")
		return nil
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer client.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	// Get Redis info
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get Redis info")
	} else {
		log.Debug().Str("info", info).Msg("Redis info retrieved")
	}

	// Test set/get
	testKey := "goleapai:validator:test"
	testValue := fmt.Sprintf("test_%d", time.Now().Unix())

	if err := client.Set(ctx, testKey, testValue, time.Minute).Err(); err != nil {
		return fmt.Errorf("Redis SET failed: %w", err)
	}

	val, err := client.Get(ctx, testKey).Result()
	if err != nil {
		return fmt.Errorf("Redis GET failed: %w", err)
	}

	if val != testValue {
		return fmt.Errorf("Redis value mismatch: expected %s, got %s", testValue, val)
	}

	// Cleanup
	client.Del(ctx, testKey)

	log.Info().Str("host", cfg.Redis.Host).Msg("Redis connection validated")
	return nil
}

// ValidateProviders verifica la connettività dei provider
func (v *Validator) ValidateProviders() error {
	log.Debug().Msg("Validating provider connectivity")

	cfg, err := config.Load(v.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Get active providers
	var providers []models.Provider
	if err := db.Where("status = ?", models.ProviderStatusActive).Find(&providers).Error; err != nil {
		return fmt.Errorf("failed to fetch providers: %w", err)
	}

	if len(providers) == 0 {
		log.Warn().Msg("No active providers found")
		return nil
	}

	// Test each provider
	ctx := context.Background()
	healthyCount := 0
	totalCount := len(providers)

	for _, provider := range providers {
		if v.testProviderConnectivity(ctx, &provider) {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		return fmt.Errorf("all providers are unreachable")
	}

	if healthyCount < totalCount {
		log.Warn().
			Int("healthy", healthyCount).
			Int("total", totalCount).
			Msg("Some providers are unhealthy")
	} else {
		log.Info().
			Int("count", healthyCount).
			Msg("All providers are healthy")
	}

	return nil
}

// ValidateCache verifica il funzionamento della cache
func (v *Validator) ValidateCache() error {
	log.Debug().Msg("Validating cache functionality")

	cfg, err := config.Load(v.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize cache
	cacheConfig := &cache.Config{
		MemoryEnabled:    true,
		MemoryMaxEntries: 100,
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

	// Test cache operations
	ctx := context.Background()
	testKey := "validator:test"
	testValue := []byte("test_value")

	// Test SET
	if err := c.Set(ctx, testKey, testValue, time.Minute); err != nil {
		return fmt.Errorf("cache SET failed: %w", err)
	}

	// Test GET
	val, err := c.Get(ctx, testKey)
	if err != nil {
		return fmt.Errorf("cache GET failed: %w", err)
	}

	if string(val) != string(testValue) {
		return fmt.Errorf("cache value mismatch")
	}

	// Test DELETE
	if err := c.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("cache DELETE failed: %w", err)
	}

	// Verify deletion
	_, err = c.Get(ctx, testKey)
	if err == nil {
		return fmt.Errorf("cache entry not deleted")
	}

	log.Info().Msg("Cache functionality validated")
	return nil
}

// ValidateAll esegue tutte le validazioni
func (v *Validator) ValidateAll() error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"YAML Configuration", v.ValidateYAML},
		{"Database Connection", v.ValidateDatabase},
		{"Redis Connection", v.ValidateRedis},
		{"Provider Connectivity", v.ValidateProviders},
		{"Cache Functionality", v.ValidateCache},
	}

	results := make(map[string]error)

	for _, check := range checks {
		log.Info().Str("check", check.name).Msg("Running validation")
		err := check.fn()
		results[check.name] = err

		if err != nil {
			log.Error().Err(err).Str("check", check.name).Msg("Validation failed")
		} else {
			log.Info().Str("check", check.name).Msg("Validation passed")
		}
	}

	// Check if any validation failed
	failed := false
	for _, err := range results {
		if err != nil {
			failed = true
			break
		}
	}

	if failed {
		return fmt.Errorf("some validations failed")
	}

	return nil
}

// Helper methods

// validateServerConfig valida la configurazione del server
func (v *Validator) validateServerConfig(cfg *config.ServerConfig) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}

	if cfg.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if cfg.TLS.Enabled {
		if _, err := os.Stat(cfg.TLS.Cert); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate not found: %s", cfg.TLS.Cert)
		}

		if _, err := os.Stat(cfg.TLS.Key); os.IsNotExist(err) {
			return fmt.Errorf("TLS key not found: %s", cfg.TLS.Key)
		}
	}

	return nil
}

// validateRoutingConfig valida la configurazione del routing
func (v *Validator) validateRoutingConfig(cfg *config.RoutingConfig) error {
	validStrategies := map[string]bool{
		"cost_optimized": true,
		"latency_first":  true,
		"quality_first":  true,
	}

	if !validStrategies[cfg.Strategy] {
		return fmt.Errorf("invalid routing strategy: %s", cfg.Strategy)
	}

	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}

	if cfg.MaxRetries > 10 {
		log.Warn().Int("max_retries", cfg.MaxRetries).Msg("High max_retries value")
	}

	return nil
}

// testProviderConnectivity testa la connettività di un provider
func (v *Validator) testProviderConnectivity(ctx context.Context, provider *models.Provider) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", provider.BaseURL, nil)
	if err != nil {
		log.Debug().Err(err).Str("provider", provider.Name).Msg("Failed to create request")
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("provider", provider.Name).Msg("Provider unreachable")
		return false
	}
	defer resp.Body.Close()

	// Any response means the server is reachable
	log.Debug().
		Str("provider", provider.Name).
		Int("status", resp.StatusCode).
		Msg("Provider reachable")

	return resp.StatusCode > 0
}

// GenerateReport genera un report di validazione
func (v *Validator) GenerateReport() (*ValidationReport, error) {
	report := &ValidationReport{
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}

	// Run all checks and collect results
	checks := map[string]func() error{
		"yaml":      v.ValidateYAML,
		"database":  v.ValidateDatabase,
		"redis":     v.ValidateRedis,
		"providers": v.ValidateProviders,
		"cache":     v.ValidateCache,
	}

	for name, checkFn := range checks {
		start := time.Now()
		err := checkFn()
		duration := time.Since(start)

		result := CheckResult{
			Name:     name,
			Passed:   err == nil,
			Duration: duration,
		}

		if err != nil {
			result.Error = err.Error()
		}

		report.Checks[name] = result
	}

	// Calculate overall status
	report.OverallStatus = "PASS"
	for _, result := range report.Checks {
		if !result.Passed {
			report.OverallStatus = "FAIL"
			break
		}
	}

	return report, nil
}

// Data structures

// ValidationReport rappresenta un report di validazione
type ValidationReport struct {
	Timestamp     time.Time              `json:"timestamp"`
	OverallStatus string                 `json:"overall_status"`
	Checks        map[string]CheckResult `json:"checks"`
}

// CheckResult rappresenta il risultato di un check
type CheckResult struct {
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// ConfigIssue rappresenta un problema di configurazione
type ConfigIssue struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	Field    string `json:"field"`
	Message  string `json:"message"`
}

// FindConfigIssues cerca problemi nella configurazione
func (v *Validator) FindConfigIssues() ([]ConfigIssue, error) {
	issues := make([]ConfigIssue, 0)

	if v.config == nil {
		cfg, err := config.Load(v.configPath)
		if err != nil {
			return nil, err
		}
		v.config = cfg
	}

	cfg := v.config

	// Check server configuration
	if cfg.Server.Port < 1024 && os.Geteuid() != 0 {
		issues = append(issues, ConfigIssue{
			Severity: "warning",
			Field:    "server.port",
			Message:  "Port < 1024 requires root privileges",
		})
	}

	// Check database configuration
	if cfg.Database.MaxConns > 100 {
		issues = append(issues, ConfigIssue{
			Severity: "warning",
			Field:    "database.max_conns",
			Message:  "High max_conns value may consume excessive resources",
		})
	}

	// Check routing configuration
	if cfg.Routing.MaxRetries > 5 {
		issues = append(issues, ConfigIssue{
			Severity: "info",
			Field:    "routing.max_retries",
			Message:  "High retry count may increase latency",
		})
	}

	// Check provider auto-discovery
	if !cfg.Providers.AutoDiscovery {
		issues = append(issues, ConfigIssue{
			Severity: "info",
			Field:    "providers.auto_discovery",
			Message:  "Auto-discovery disabled - providers must be added manually",
		})
	}

	return issues, nil
}
