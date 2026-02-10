package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/notifications"
	"github.com/rs/zerolog/log"
)

// LogConfig configurazione per il canale log
type LogConfig struct {
	FilePath       string
	MaxFileSize    int64 // bytes
	MaxBackups     int
	Compress       bool
	Format         string // "json" o "text"
	IncludeConsole bool   // Scrivi anche su console
}

// LogChannel implementa il canale di notifica log
type LogChannel struct {
	config  *LogConfig
	file    *os.File
	mu      sync.Mutex
	size    int64
	backups int
}

// NewLogChannel crea un nuovo canale log
func NewLogChannel(config *LogConfig) *LogChannel {
	if config.FilePath == "" {
		config.FilePath = "./logs/notifications.log"
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 10 * 1024 * 1024 // 10MB default
	}
	if config.MaxBackups <= 0 {
		config.MaxBackups = 5
	}
	if config.Format == "" {
		config.Format = "json"
	}

	return &LogChannel{
		config: config,
	}
}

// Start avvia il canale log
func (lc *LogChannel) Start() error {
	// Crea directory se non esiste
	dir := filepath.Dir(lc.config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Apri file
	if err := lc.openFile(); err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	log.Info().
		Str("channel", "log").
		Str("file", lc.config.FilePath).
		Msg("Log channel started")

	return nil
}

// Stop ferma il canale log
func (lc *LogChannel) Stop() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.file != nil {
		if err := lc.file.Close(); err != nil {
			return err
		}
		lc.file = nil
	}

	log.Info().Msg("Log channel stopped")
	return nil
}

// Send invia una notifica al log
func (lc *LogChannel) Send(ctx context.Context, event notifications.Event) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Formatta messaggio
	var message string
	var err error

	if lc.config.Format == "json" {
		message, err = lc.formatJSON(event)
	} else {
		message, err = lc.formatText(event)
	}

	if err != nil {
		return fmt.Errorf("failed to format log message: %w", err)
	}

	// Scrivi su console se richiesto
	if lc.config.IncludeConsole {
		lc.writeToConsole(event)
	}

	// Scrivi su file
	n, err := lc.file.WriteString(message + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	lc.size += int64(n)

	// Check se serve rotazione
	if lc.size >= lc.config.MaxFileSize {
		if err := lc.rotate(); err != nil {
			log.Error().Err(err).Msg("Failed to rotate log file")
		}
	}

	return nil
}

// openFile apre il file di log
func (lc *LogChannel) openFile() error {
	file, err := os.OpenFile(lc.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Ottieni dimensione file
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	lc.file = file
	lc.size = info.Size()

	return nil
}

// rotate ruota i file di log
func (lc *LogChannel) rotate() error {
	// Chiudi file corrente
	if lc.file != nil {
		lc.file.Close()
		lc.file = nil
	}

	// Rinomina file esistenti
	for i := lc.config.MaxBackups - 1; i > 0; i-- {
		oldPath := lc.getBackupPath(i)
		newPath := lc.getBackupPath(i + 1)

		if _, err := os.Stat(oldPath); err == nil {
			if i == lc.config.MaxBackups-1 {
				// Rimuovi il più vecchio
				os.Remove(newPath)
			}
			os.Rename(oldPath, newPath)

			// Comprimi se richiesto
			if lc.config.Compress && i == lc.config.MaxBackups-1 {
				go lc.compressFile(newPath)
			}
		}
	}

	// Sposta il file corrente a backup.1
	backupPath := lc.getBackupPath(1)
	if err := os.Rename(lc.config.FilePath, backupPath); err != nil {
		return err
	}

	// Apri nuovo file
	if err := lc.openFile(); err != nil {
		return err
	}

	log.Info().
		Str("file", lc.config.FilePath).
		Msg("Log file rotated")

	return nil
}

// getBackupPath ottiene il path del backup
func (lc *LogChannel) getBackupPath(index int) string {
	ext := filepath.Ext(lc.config.FilePath)
	name := lc.config.FilePath[:len(lc.config.FilePath)-len(ext)]
	return fmt.Sprintf("%s.%d%s", name, index, ext)
}

// compressFile comprimi un file di log
func (lc *LogChannel) compressFile(path string) {
	// TODO: Implementa compressione gzip
	// Per ora è un placeholder
	log.Debug().Str("file", path).Msg("Compressing log file (not implemented)")
}

// formatJSON formatta l'evento come JSON
func (lc *LogChannel) formatJSON(event notifications.Event) (string, error) {
	entry := map[string]interface{}{
		"timestamp":  event.Timestamp().Format(time.RFC3339Nano),
		"event_type": string(event.Type()),
		"severity":   string(event.Severity()),
		"message":    event.Message(),
		"metadata":   event.Metadata(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// formatText formatta l'evento come testo
func (lc *LogChannel) formatText(event notifications.Event) (string, error) {
	timestamp := event.Timestamp().Format(time.RFC3339)
	severity := event.Severity()
	eventType := event.Type()
	message := event.Message()

	text := fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, severity, eventType, message)

	// Aggiungi metadata importanti
	meta := event.Metadata()
	if providerName, ok := meta["provider_name"].(string); ok {
		text += fmt.Sprintf(" | provider=%s", providerName)
	}
	if providerID, ok := meta["provider_id"].(string); ok {
		text += fmt.Sprintf(" | provider_id=%s", providerID)
	}

	return text, nil
}

// writeToConsole scrive l'evento anche su console usando zerolog
func (lc *LogChannel) writeToConsole(event notifications.Event) {
	logEvent := log.With().
		Str("event_type", string(event.Type())).
		Str("severity", string(event.Severity())).
		Fields(event.Metadata()).
		Logger()

	switch event.Severity() {
	case notifications.SeverityCritical:
		logEvent.Error().Msg(event.Message())
	case notifications.SeverityError:
		logEvent.Error().Msg(event.Message())
	case notifications.SeverityWarning:
		logEvent.Warn().Msg(event.Message())
	default:
		logEvent.Info().Msg(event.Message())
	}
}

// GetStats ottiene statistiche sul file di log
func (lc *LogChannel) GetStats() map[string]interface{} {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	stats := make(map[string]interface{})
	stats["file_path"] = lc.config.FilePath
	stats["current_size"] = lc.size
	stats["max_size"] = lc.config.MaxFileSize

	// Conta backup esistenti
	backupCount := 0
	for i := 1; i <= lc.config.MaxBackups; i++ {
		if _, err := os.Stat(lc.getBackupPath(i)); err == nil {
			backupCount++
		}
	}
	stats["backup_count"] = backupCount

	return stats
}

// Tail legge le ultime N righe del log
func (lc *LogChannel) Tail(n int) ([]string, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.file == nil {
		return nil, fmt.Errorf("log file not open")
	}

	// Leggi tutto il file (per semplicità)
	// In produzione, usare un approccio più efficiente
	data, err := os.ReadFile(lc.config.FilePath)
	if err != nil {
		return nil, err
	}

	// Split in righe
	lines := splitLines(string(data))

	// Prendi ultime N righe
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	return lines[start:], nil
}

// splitLines split string in righe
func splitLines(s string) []string {
	lines := make([]string, 0)
	current := ""

	for _, ch := range s {
		if ch == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}
