package database

import (
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config contiene la configurazione del database
type Config struct {
	Type       string `yaml:"type"`       // "postgres" or "sqlite"
	Connection string `yaml:"connection"` // Connection string
	MaxConns   int    `yaml:"max_conns"`
	LogLevel   string `yaml:"log_level"`
}

// DB wrappa la connessione GORM
type DB struct {
	*gorm.DB
}

// New crea una nuova connessione al database
func New(cfg *Config) (*DB, error) {
	var dialector gorm.Dialector

	switch cfg.Type {
	case "postgres":
		dialector = postgres.Open(cfg.Connection)
	case "sqlite":
		dialector = sqlite.Open(cfg.Connection)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	// Configure logger
	logLevel := logger.Silent
	switch cfg.LogLevel {
	case "info":
		logLevel = logger.Info
	case "warn":
		logLevel = logger.Warn
	case "error":
		logLevel = logger.Error
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if cfg.MaxConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxConns)
		sqlDB.SetMaxIdleConns(cfg.MaxConns / 2)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{DB: db}, nil
}

// AutoMigrate esegue le migrazioni del database
func (db *DB) AutoMigrate() error {
	return db.DB.AutoMigrate(
		&models.User{},
		&models.APIKey{},
		&models.Provider{},
		&models.Model{},
		&models.Account{},
		&models.RateLimit{},
		&models.ProviderStats{},
		&models.RequestLog{},
	)
}

// Seed popola il database con dati iniziali
func (db *DB) Seed() error {
	// Check if already seeded
	var count int64
	db.Model(&models.Provider{}).Count(&count)
	if count > 0 {
		return nil // Already seeded
	}

	// Insert seed data
	providers := getFreeAPIProviders()
	for _, provider := range providers {
		if err := db.Create(&provider).Error; err != nil {
			return fmt.Errorf("failed to seed provider %s: %w", provider.Name, err)
		}
	}

	return nil
}

// Close chiude la connessione al database
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetAllProviders restituisce tutti i provider
func (db *DB) GetAllProviders() ([]models.Provider, error) {
	var providers []models.Provider
	err := db.Order("name").Find(&providers).Error
	return providers, err
}

// GetProviderByID restituisce un provider per ID
func (db *DB) GetProviderByID(id string) (*models.Provider, error) {
	var provider models.Provider
	err := db.Where("id = ?", id).First(&provider).Error
	return &provider, err
}

// UpdateProvider aggiorna un provider
func (db *DB) UpdateProvider(provider *models.Provider) error {
	return db.Save(provider).Error
}

// GetLatestStats restituisce le statistiche più recenti
func (db *DB) GetLatestStats() ([]models.ProviderStats, error) {
	var stats []models.ProviderStats
	// Get latest stats for each provider (last 24 hours)
	err := db.Where("timestamp > ?", time.Now().Add(-24*time.Hour)).
		Preload("Provider").
		Order("timestamp DESC").
		Limit(100).
		Find(&stats).Error
	return stats, err
}

// GetRecentLogs restituisce i log più recenti
func (db *DB) GetRecentLogs(limit int) ([]models.RequestLog, error) {
	var logs []models.RequestLog
	err := db.Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetAPIKeyByHash ottiene una chiave API dal suo hash
func (db *DB) GetAPIKeyByHash(keyHash string) (*models.APIKey, error) {
	var apiKey models.APIKey
	err := db.Preload("User").Where("key_hash = ? AND active = ?", keyHash, true).First(&apiKey).Error
	return &apiKey, err
}

// GetUserByID ottiene un utente per ID
func (db *DB) GetUserByID(id string) (*models.User, error) {
	var user models.User
	err := db.Where("id = ?", id).First(&user).Error
	return &user, err
}

// GetUserByEmail ottiene un utente per email
func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := db.Where("email = ?", email).First(&user).Error
	return &user, err
}

// CreateUser crea un nuovo utente
func (db *DB) CreateUser(user *models.User) error {
	return db.Create(user).Error
}

// UpdateUser aggiorna un utente
func (db *DB) UpdateUser(user *models.User) error {
	return db.Save(user).Error
}

// CreateAPIKey crea una nuova API key
func (db *DB) CreateAPIKey(apiKey *models.APIKey) error {
	return db.Create(apiKey).Error
}

// UpdateAPIKey aggiorna una API key
func (db *DB) UpdateAPIKey(apiKey *models.APIKey) error {
	return db.Save(apiKey).Error
}

// GetUserAPIKeys ottiene tutte le chiavi API di un utente
func (db *DB) GetUserAPIKeys(userID string) ([]models.APIKey, error) {
	var apiKeys []models.APIKey
	err := db.Where("user_id = ?", userID).Order("created_at DESC").Find(&apiKeys).Error
	return apiKeys, err
}
