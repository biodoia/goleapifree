package admin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// BackupManager gestisce backup e restore
type BackupManager struct {
	db         *database.DB
	config     *config.Config
	backupDir  string
}

// BackupMetadata contiene metadati del backup
type BackupMetadata struct {
	ID              uuid.UUID `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	DatabaseType    string    `json:"database_type"`
	ProviderCount   int       `json:"provider_count"`
	UserCount       int       `json:"user_count"`
	AccountCount    int       `json:"account_count"`
	ModelCount      int       `json:"model_count"`
	Size            int64     `json:"size"`
	Compressed      bool      `json:"compressed"`
	FilePath        string    `json:"file_path"`
}

// BackupData rappresenta tutti i dati da salvare
type BackupData struct {
	Metadata    BackupMetadata      `json:"metadata"`
	Providers   []models.Provider   `json:"providers"`
	Models      []models.Model      `json:"models"`
	Accounts    []models.Account    `json:"accounts"`
	Users       []models.User       `json:"users"`
	APIKeys     []models.APIKey     `json:"api_keys"`
	RateLimits  []models.RateLimit  `json:"rate_limits"`
	Config      *config.Config      `json:"config"`
}

// NewBackupManager crea un nuovo backup manager
func NewBackupManager(db *database.DB, cfg *config.Config) *BackupManager {
	backupDir := "./backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create backup directory")
	}

	return &BackupManager{
		db:        db,
		config:    cfg,
		backupDir: backupDir,
	}
}

// CreateBackup crea un backup completo del database
func (h *AdminHandlers) CreateBackup(c fiber.Ctx) error {
	var req struct {
		IncludeUsers    bool `json:"include_users"`
		IncludeLogs     bool `json:"include_logs"`
		Compress        bool `json:"compress"`
		Description     string `json:"description"`
	}

	// Defaults
	req.IncludeUsers = true
	req.Compress = true

	if err := c.Bind().JSON(&req); err != nil {
		// Use defaults if no body provided
	}

	backup, err := h.backupManager.CreateBackup(req.IncludeUsers, req.IncludeLogs, req.Compress)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create backup")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create backup",
			"details": err.Error(),
		})
	}

	log.Info().Str("backup_id", backup.ID.String()).Msg("Backup created successfully")

	return c.Status(201).JSON(fiber.Map{
		"message": "Backup created successfully",
		"backup":  backup,
	})
}

// CreateBackup esegue il backup
func (bm *BackupManager) CreateBackup(includeUsers, includeLogs, compress bool) (*BackupMetadata, error) {
	log.Info().Msg("Starting database backup")

	backupData := BackupData{
		Metadata: BackupMetadata{
			ID:           uuid.New(),
			Timestamp:    time.Now(),
			Version:      "1.0.0",
			DatabaseType: bm.config.Database.Type,
			Compressed:   compress,
		},
		Config: bm.config,
	}

	// Load providers
	if err := bm.db.Preload("Models").Preload("RateLimits").Find(&backupData.Providers).Error; err != nil {
		return nil, fmt.Errorf("failed to load providers: %w", err)
	}
	backupData.Metadata.ProviderCount = len(backupData.Providers)

	// Load models
	if err := bm.db.Find(&backupData.Models).Error; err != nil {
		return nil, fmt.Errorf("failed to load models: %w", err)
	}
	backupData.Metadata.ModelCount = len(backupData.Models)

	// Load rate limits
	if err := bm.db.Find(&backupData.RateLimits).Error; err != nil {
		return nil, fmt.Errorf("failed to load rate limits: %w", err)
	}

	// Load accounts
	if err := bm.db.Find(&backupData.Accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}
	backupData.Metadata.AccountCount = len(backupData.Accounts)

	// Load users if requested
	if includeUsers {
		if err := bm.db.Find(&backupData.Users).Error; err != nil {
			return nil, fmt.Errorf("failed to load users: %w", err)
		}
		backupData.Metadata.UserCount = len(backupData.Users)

		// Load API keys
		if err := bm.db.Find(&backupData.APIKeys).Error; err != nil {
			return nil, fmt.Errorf("failed to load API keys: %w", err)
		}
	}

	// Generate filename
	filename := fmt.Sprintf("backup_%s_%s.json",
		backupData.Metadata.Timestamp.Format("20060102_150405"),
		backupData.Metadata.ID.String()[:8])

	if compress {
		filename += ".gz"
	}

	filePath := filepath.Join(bm.backupDir, filename)
	backupData.Metadata.FilePath = filePath

	// Write backup file
	if compress {
		if err := bm.writeCompressedBackup(filePath, &backupData); err != nil {
			return nil, fmt.Errorf("failed to write compressed backup: %w", err)
		}
	} else {
		if err := bm.writeBackup(filePath, &backupData); err != nil {
			return nil, fmt.Errorf("failed to write backup: %w", err)
		}
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		backupData.Metadata.Size = fileInfo.Size()
	}

	log.Info().
		Str("file", filename).
		Int64("size", backupData.Metadata.Size).
		Int("providers", backupData.Metadata.ProviderCount).
		Int("users", backupData.Metadata.UserCount).
		Msg("Backup completed successfully")

	return &backupData.Metadata, nil
}

// writeBackup scrive il backup non compresso
func (bm *BackupManager) writeBackup(filePath string, data *BackupData) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeCompressedBackup scrive il backup compresso con gzip
func (bm *BackupManager) writeCompressedBackup(filePath string, data *BackupData) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	encoder := json.NewEncoder(gzipWriter)
	return encoder.Encode(data)
}

// ListBackups restituisce tutti i backup disponibili
func (h *AdminHandlers) ListBackups(c fiber.Ctx) error {
	backups, err := h.backupManager.ListBackups()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to list backups",
		})
	}

	return c.JSON(fiber.Map{
		"backups": backups,
		"count":   len(backups),
	})
}

// ListBackups elenca tutti i backup
func (bm *BackupManager) ListBackups() ([]BackupMetadata, error) {
	var backups []BackupMetadata

	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(bm.backupDir, file.Name())
		metadata, err := bm.readBackupMetadata(filePath)
		if err != nil {
			log.Warn().Err(err).Str("file", file.Name()).Msg("Failed to read backup metadata")
			continue
		}

		backups = append(backups, *metadata)
	}

	return backups, nil
}

// readBackupMetadata legge solo i metadati di un backup
func (bm *BackupManager) readBackupMetadata(filePath string) (*BackupMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if compressed
	if filepath.Ext(filePath) == ".gz" {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	var data BackupData
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	// Update file path
	data.Metadata.FilePath = filePath

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		data.Metadata.Size = fileInfo.Size()
	}

	return &data.Metadata, nil
}

// RestoreBackup ripristina da un backup
func (h *AdminHandlers) RestoreBackup(c fiber.Ctx) error {
	var req struct {
		BackupID       string `json:"backup_id"`
		FilePath       string `json:"file_path"`
		RestoreUsers   bool   `json:"restore_users"`
		RestoreConfig  bool   `json:"restore_config"`
		ClearExisting  bool   `json:"clear_existing"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.FilePath == "" && req.BackupID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "backup_id or file_path is required",
		})
	}

	// Find backup file
	var filePath string
	if req.FilePath != "" {
		filePath = req.FilePath
	} else {
		backups, err := h.backupManager.ListBackups()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to list backups",
			})
		}

		for _, backup := range backups {
			if backup.ID.String() == req.BackupID || backup.ID.String()[:8] == req.BackupID {
				filePath = backup.FilePath
				break
			}
		}

		if filePath == "" {
			return c.Status(404).JSON(fiber.Map{
				"error": "Backup not found",
			})
		}
	}

	// Restore backup
	if err := h.backupManager.RestoreBackup(filePath, req.RestoreUsers, req.RestoreConfig, req.ClearExisting); err != nil {
		log.Error().Err(err).Msg("Failed to restore backup")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to restore backup",
			"details": err.Error(),
		})
	}

	log.Info().Str("file", filePath).Msg("Backup restored successfully")

	return c.JSON(fiber.Map{
		"message": "Backup restored successfully",
	})
}

// RestoreBackup ripristina da un file di backup
func (bm *BackupManager) RestoreBackup(filePath string, restoreUsers, restoreConfig, clearExisting bool) error {
	log.Info().Str("file", filePath).Msg("Starting backup restore")

	// Read backup file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if compressed
	if filepath.Ext(filePath) == ".gz" {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to open gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	var data BackupData
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode backup: %w", err)
	}

	// Clear existing data if requested
	if clearExisting {
		if err := bm.clearDatabase(restoreUsers); err != nil {
			return fmt.Errorf("failed to clear database: %w", err)
		}
	}

	// Restore providers
	for _, provider := range data.Providers {
		if err := bm.db.Create(&provider).Error; err != nil {
			log.Warn().Err(err).Str("provider", provider.Name).Msg("Failed to restore provider")
		}
	}

	// Restore models
	for _, model := range data.Models {
		if err := bm.db.Create(&model).Error; err != nil {
			log.Warn().Err(err).Str("model", model.Name).Msg("Failed to restore model")
		}
	}

	// Restore rate limits
	for _, rateLimit := range data.RateLimits {
		if err := bm.db.Create(&rateLimit).Error; err != nil {
			log.Warn().Err(err).Msg("Failed to restore rate limit")
		}
	}

	// Restore accounts
	for _, account := range data.Accounts {
		if err := bm.db.Create(&account).Error; err != nil {
			log.Warn().Err(err).Msg("Failed to restore account")
		}
	}

	// Restore users if requested
	if restoreUsers {
		for _, user := range data.Users {
			if err := bm.db.Create(&user).Error; err != nil {
				log.Warn().Err(err).Str("email", user.Email).Msg("Failed to restore user")
			}
		}

		for _, apiKey := range data.APIKeys {
			if err := bm.db.Create(&apiKey).Error; err != nil {
				log.Warn().Err(err).Msg("Failed to restore API key")
			}
		}
	}

	log.Info().
		Int("providers", len(data.Providers)).
		Int("models", len(data.Models)).
		Int("users", len(data.Users)).
		Msg("Backup restore completed")

	return nil
}

// clearDatabase cancella i dati esistenti
func (bm *BackupManager) clearDatabase(includeUsers bool) error {
	log.Warn().Msg("Clearing database...")

	// Delete in correct order due to foreign keys
	bm.db.Exec("DELETE FROM rate_limits")
	bm.db.Exec("DELETE FROM models")
	bm.db.Exec("DELETE FROM accounts")
	bm.db.Exec("DELETE FROM provider_stats")
	bm.db.Exec("DELETE FROM request_logs")
	bm.db.Exec("DELETE FROM providers")

	if includeUsers {
		bm.db.Exec("DELETE FROM api_keys")
		bm.db.Exec("DELETE FROM users")
	}

	return nil
}

// ExportConfiguration esporta solo la configurazione dei provider
func (h *AdminHandlers) ExportConfiguration(c fiber.Ctx) error {
	var providers []models.Provider
	if err := h.db.Preload("Models").Preload("RateLimits").Find(&providers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to export configuration",
		})
	}

	exportData := fiber.Map{
		"exported_at": time.Now(),
		"version":     "1.0.0",
		"providers":   providers,
	}

	// Set download headers
	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=providers_export_%s.json",
		time.Now().Format("20060102_150405")))

	return c.JSON(exportData)
}

// ImportProviders importa provider da un file JSON
func (h *AdminHandlers) ImportProviders(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	// Open uploaded file
	uploadedFile, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to open uploaded file",
		})
	}
	defer uploadedFile.Close()

	// Parse JSON
	var importData struct {
		Providers []models.Provider `json:"providers"`
	}

	decoder := json.NewDecoder(uploadedFile)
	if err := decoder.Decode(&importData); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON format",
			"details": err.Error(),
		})
	}

	// Import providers
	imported := 0
	skipped := 0

	for _, provider := range importData.Providers {
		// Check if provider already exists
		var existing models.Provider
		if err := h.db.Where("name = ?", provider.Name).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		// Reset IDs
		provider.ID = uuid.New()
		provider.DiscoveredAt = time.Now()
		provider.Source = "import"

		if err := h.db.Create(&provider).Error; err != nil {
			log.Warn().Err(err).Str("provider", provider.Name).Msg("Failed to import provider")
			skipped++
			continue
		}

		imported++
	}

	log.Info().Int("imported", imported).Int("skipped", skipped).Msg("Provider import completed")

	return c.JSON(fiber.Map{
		"message":  "Providers imported successfully",
		"imported": imported,
		"skipped":  skipped,
		"total":    len(importData.Providers),
	})
}

// CreateTarGzBackup crea un backup completo in formato tar.gz
func (bm *BackupManager) CreateTarGzBackup() (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_full_%s.tar.gz", timestamp)
	filePath := filepath.Join(bm.backupDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add database file if SQLite
	if bm.config.Database.Type == "sqlite" {
		if err := addFileToTar(tarWriter, bm.config.Database.Connection, "database.db"); err != nil {
			log.Warn().Err(err).Msg("Failed to add database to tar")
		}
	}

	// Add JSON backup
	backup, err := bm.CreateBackup(true, false, false)
	if err != nil {
		return "", err
	}

	if err := addFileToTar(tarWriter, backup.FilePath, "backup.json"); err != nil {
		return "", err
	}

	return filePath, nil
}

// addFileToTar aggiunge un file a un archivio tar
func addFileToTar(tarWriter *tar.Writer, filePath, nameInTar string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    nameInTar,
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}
