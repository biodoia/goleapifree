package testhelpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TestDB creates an in-memory test database
func TestDB(t *testing.T) *database.DB {
	t.Helper()

	cfg := &config.DatabaseConfig{
		Type:            "sqlite",
		Path:            ":memory:",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: "1h",
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(
		&models.Provider{},
		&models.Model{},
		&models.Account{},
		&models.RateLimit{},
	); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db
}

// CreateTestProvider creates a test provider
func CreateTestProvider(t *testing.T, db *gorm.DB, name string) *models.Provider {
	t.Helper()

	provider := &models.Provider{
		ID:                uuid.New(),
		Name:              name,
		Type:              models.ProviderTypeFree,
		Status:            models.ProviderStatusActive,
		BaseURL:           fmt.Sprintf("https://api.%s.com", name),
		AuthType:          models.AuthTypeAPIKey,
		Tier:              2,
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsJSON:      true,
		DiscoveredAt:      time.Now(),
		LastHealthCheck:   time.Now(),
		HealthScore:       1.0,
		AvgLatencyMs:      100,
	}

	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}

	return provider
}

// CreateTestModel creates a test model
func CreateTestModel(t *testing.T, db *gorm.DB, providerID uuid.UUID, name string) *models.Model {
	t.Helper()

	model := &models.Model{
		ID:               uuid.New(),
		ProviderID:       providerID,
		Name:             name,
		DisplayName:      name,
		Modality:         models.ModalityChat,
		ContextLength:    8192,
		MaxOutputTokens:  4096,
		InputPricePer1k:  0.0,
		OutputPricePer1k: 0.0,
		QualityScore:     0.8,
		SpeedScore:       0.9,
	}

	if err := db.Create(model).Error; err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}

	return model
}

// CreateTestAccount creates a test account
func CreateTestAccount(t *testing.T, db *gorm.DB, providerID uuid.UUID) *models.Account {
	t.Helper()

	account := &models.Account{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		ProviderID: providerID,
		QuotaLimit: 100000,
		QuotaUsed:  0,
		LastReset:  time.Now(),
		Active:     true,
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	if err := db.Create(account).Error; err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	return account
}

// CleanupDB cleans up the test database
func CleanupDB(t *testing.T, db *database.DB) {
	t.Helper()

	if db != nil {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for condition: %s", message)
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// AssertNoError fails the test if error is not nil
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertError fails the test if error is nil
func AssertError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", message)
	}
}

// AssertEqual fails if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotEqual fails if expected == actual
func AssertNotEqual(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if expected == actual {
		t.Fatalf("%s: expected values to be different, both are %v", message, expected)
	}
}

// AssertTrue fails if value is not true
func AssertTrue(t *testing.T, value bool, message string) {
	t.Helper()
	if !value {
		t.Fatalf("%s: expected true, got false", message)
	}
}

// AssertFalse fails if value is not false
func AssertFalse(t *testing.T, value bool, message string) {
	t.Helper()
	if value {
		t.Fatalf("%s: expected false, got true", message)
	}
}

// TestConfig returns a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:            "localhost",
			Port:            8080,
			ReadTimeout:     "30s",
			WriteTimeout:    "30s",
			ShutdownTimeout: "10s",
		},
		Database: config.DatabaseConfig{
			Type:            "sqlite",
			Path:            ":memory:",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: "1h",
		},
		Routing: config.RoutingConfig{
			Strategy:          "cost_optimized",
			EnableFailover:    true,
			MaxRetries:        3,
			RetryDelay:        "1s",
			LoadBalanceMethod: "round_robin",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval:  "5m",
			DiscoveryEnabled:     false,
			AutoVerifyInterval:   "24h",
			MaxConcurrentChecks:  10,
			RequestTimeout:       "30s",
		},
		Cache: config.CacheConfig{
			Enabled:         true,
			Type:            "memory",
			TTL:             "5m",
			MaxSize:         100 * 1024 * 1024,
			MaxEntries:      10000,
		},
		Monitoring: config.MonitoringConfig{
			Enabled: false,
			Prometheus: config.PrometheusConfig{
				Enabled: false,
				Path:    "/metrics",
			},
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// MockTime provides time mocking utilities
type MockTime struct {
	current time.Time
}

// NewMockTime creates a new mock time
func NewMockTime(t time.Time) *MockTime {
	return &MockTime{current: t}
}

// Now returns the current mock time
func (m *MockTime) Now() time.Time {
	return m.current
}

// Advance advances the mock time
func (m *MockTime) Advance(d time.Duration) {
	m.current = m.current.Add(d)
}

// Set sets the mock time
func (m *MockTime) Set(t time.Time) {
	m.current = t
}
