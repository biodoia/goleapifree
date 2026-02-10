package admin

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/health"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// MaintenanceManager gestisce operazioni di manutenzione
type MaintenanceManager struct {
	db            *database.DB
	healthMonitor *health.Monitor
	cache         sync.Map // In-memory cache for maintenance status
}

// MaintenanceStatus rappresenta lo stato delle operazioni di manutenzione
type MaintenanceStatus struct {
	LastCacheClear      time.Time `json:"last_cache_clear"`
	LastLogPrune        time.Time `json:"last_log_prune"`
	LastReindex         time.Time `json:"last_reindex"`
	LastHealthCheck     time.Time `json:"last_health_check"`
	ProviderCount       int64     `json:"provider_count"`
	ActiveProviders     int64     `json:"active_providers"`
	UserCount           int64     `json:"user_count"`
	RequestLogCount     int64     `json:"request_log_count"`
	DatabaseSize        int64     `json:"database_size_bytes"`
	MemoryUsage         uint64    `json:"memory_usage_bytes"`
	GoroutineCount      int       `json:"goroutine_count"`
	Uptime              string    `json:"uptime"`
}

var startTime = time.Now()

// NewMaintenanceManager crea un nuovo maintenance manager
func NewMaintenanceManager(db *database.DB, healthMonitor *health.Monitor) *MaintenanceManager {
	return &MaintenanceManager{
		db:            db,
		healthMonitor: healthMonitor,
	}
}

// GetMaintenanceStatus restituisce lo stato del sistema
func (h *AdminHandlers) GetMaintenanceStatus(c fiber.Ctx) error {
	status := h.maintenance.GetStatus()

	return c.JSON(status)
}

// GetStatus raccoglie le informazioni di stato
func (mm *MaintenanceManager) GetStatus() MaintenanceStatus {
	status := MaintenanceStatus{
		GoroutineCount: runtime.NumGoroutine(),
		Uptime:         time.Since(startTime).String(),
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	status.MemoryUsage = memStats.Alloc

	// Load cached maintenance times
	if val, ok := mm.cache.Load("last_cache_clear"); ok {
		status.LastCacheClear = val.(time.Time)
	}
	if val, ok := mm.cache.Load("last_log_prune"); ok {
		status.LastLogPrune = val.(time.Time)
	}
	if val, ok := mm.cache.Load("last_reindex"); ok {
		status.LastReindex = val.(time.Time)
	}
	if val, ok := mm.cache.Load("last_health_check"); ok {
		status.LastHealthCheck = val.(time.Time)
	}

	// Get database stats
	mm.db.Model(&models.Provider{}).Count(&status.ProviderCount)
	mm.db.Model(&models.Provider{}).Where("status = ?", models.ProviderStatusActive).Count(&status.ActiveProviders)
	mm.db.Model(&models.User{}).Count(&status.UserCount)
	mm.db.Model(&models.RequestLog{}).Count(&status.RequestLogCount)

	// Get database size (works for SQLite)
	sqlDB, err := mm.db.DB.DB()
	if err == nil {
		var size int64
		sqlDB.QueryRow("SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
		status.DatabaseSize = size
	}

	return status
}

// ClearCache pulisce la cache
func (h *AdminHandlers) ClearCache(c fiber.Ctx) error {
	cleared, err := h.maintenance.ClearCache()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to clear cache",
			"details": err.Error(),
		})
	}

	log.Info().Msg("Cache cleared successfully")

	return c.JSON(fiber.Map{
		"message": "Cache cleared successfully",
		"cleared": cleared,
	})
}

// ClearCache esegue la pulizia della cache
func (mm *MaintenanceManager) ClearCache() (int, error) {
	// Force garbage collection
	runtime.GC()

	// Clear internal cache
	cleared := 0
	mm.cache.Range(func(key, value interface{}) bool {
		if key != "last_cache_clear" && key != "last_log_prune" &&
		   key != "last_reindex" && key != "last_health_check" {
			mm.cache.Delete(key)
			cleared++
		}
		return true
	})

	mm.cache.Store("last_cache_clear", time.Now())

	log.Info().Int("entries", cleared).Msg("Cache cleared")

	return cleared, nil
}

// PruneLogs elimina i log vecchi
func (h *AdminHandlers) PruneLogs(c fiber.Ctx) error {
	var req struct {
		OlderThanDays int  `json:"older_than_days"`
		KeepCount     int  `json:"keep_count"`
		DryRun        bool `json:"dry_run"`
	}

	// Defaults
	req.OlderThanDays = 30
	req.KeepCount = 10000

	if err := c.Bind().JSON(&req); err != nil {
		// Use defaults
	}

	deleted, err := h.maintenance.PruneLogs(req.OlderThanDays, req.KeepCount, req.DryRun)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to prune logs",
			"details": err.Error(),
		})
	}

	message := "Logs pruned successfully"
	if req.DryRun {
		message = "Dry run completed (no logs deleted)"
	}

	log.Info().Int64("deleted", deleted).Msg("Log pruning completed")

	return c.JSON(fiber.Map{
		"message": message,
		"deleted": deleted,
		"dry_run": req.DryRun,
	})
}

// PruneLogs elimina i log vecchi
func (mm *MaintenanceManager) PruneLogs(olderThanDays, keepCount int, dryRun bool) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -olderThanDays)

	// Count logs to delete
	var count int64
	mm.db.Model(&models.RequestLog{}).
		Where("timestamp < ?", cutoffDate).
		Count(&count)

	// Check if we should keep some logs
	var totalCount int64
	mm.db.Model(&models.RequestLog{}).Count(&totalCount)

	if totalCount-count < int64(keepCount) {
		// Adjust to keep minimum logs
		toDelete := totalCount - int64(keepCount)
		if toDelete <= 0 {
			return 0, nil
		}
		count = toDelete
	}

	if dryRun {
		log.Info().
			Int64("count", count).
			Time("cutoff", cutoffDate).
			Msg("Dry run: would delete logs")
		return count, nil
	}

	// Delete old logs
	result := mm.db.Where("timestamp < ?", cutoffDate).
		Limit(int(count)).
		Delete(&models.RequestLog{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete logs: %w", result.Error)
	}

	mm.cache.Store("last_log_prune", time.Now())

	log.Info().
		Int64("deleted", result.RowsAffected).
		Time("cutoff", cutoffDate).
		Msg("Logs pruned successfully")

	return result.RowsAffected, nil
}

// ReindexDatabase ricostruisce gli indici del database
func (h *AdminHandlers) ReindexDatabase(c fiber.Ctx) error {
	err := h.maintenance.ReindexDatabase()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to reindex database",
			"details": err.Error(),
		})
	}

	log.Info().Msg("Database reindexed successfully")

	return c.JSON(fiber.Map{
		"message": "Database reindexed successfully",
	})
}

// ReindexDatabase ricostruisce gli indici
func (mm *MaintenanceManager) ReindexDatabase() error {
	sqlDB, err := mm.db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// For SQLite
	if _, err := sqlDB.Exec("REINDEX"); err != nil {
		return fmt.Errorf("failed to reindex: %w", err)
	}

	// Analyze tables for query optimization
	tables := []string{"providers", "models", "accounts", "users", "api_keys", "request_logs", "provider_stats", "rate_limits"}
	for _, table := range tables {
		if _, err := sqlDB.Exec(fmt.Sprintf("ANALYZE %s", table)); err != nil {
			log.Warn().Err(err).Str("table", table).Msg("Failed to analyze table")
		}
	}

	mm.cache.Store("last_reindex", time.Now())

	log.Info().Msg("Database reindexed and analyzed")

	return nil
}

// ForceHealthCheckAll forza il controllo di salute di tutti i provider
func (h *AdminHandlers) ForceHealthCheckAll(c fiber.Ctx) error {
	var req struct {
		Parallel bool `json:"parallel"`
	}

	// Default to parallel
	req.Parallel = true

	if err := c.Bind().JSON(&req); err != nil {
		// Use defaults
	}

	results, err := h.maintenance.ForceHealthCheckAll(req.Parallel)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to run health checks",
			"details": err.Error(),
		})
	}

	log.Info().Int("checked", len(results)).Msg("Health checks completed")

	return c.JSON(fiber.Map{
		"message": "Health checks completed",
		"results": results,
		"total":   len(results),
	})
}

// HealthCheckResult rappresenta il risultato di un health check
type HealthCheckResult struct {
	ProviderID   string  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	Status       string  `json:"status"`
	LatencyMs    int64   `json:"latency_ms"`
	HealthScore  float64 `json:"health_score"`
	Error        string  `json:"error,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// ForceHealthCheckAll esegue health check su tutti i provider
func (mm *MaintenanceManager) ForceHealthCheckAll(parallel bool) ([]HealthCheckResult, error) {
	var providers []models.Provider
	if err := mm.db.Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to load providers: %w", err)
	}

	results := make([]HealthCheckResult, 0, len(providers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, provider := range providers {
		if parallel {
			wg.Add(1)
			go func(p models.Provider) {
				defer wg.Done()
				result := mm.checkProviderHealth(&p)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(provider)
		} else {
			result := mm.checkProviderHealth(&provider)
			results = append(results, result)
		}
	}

	if parallel {
		wg.Wait()
	}

	mm.cache.Store("last_health_check", time.Now())

	return results, nil
}

// checkProviderHealth esegue health check su un singolo provider
func (mm *MaintenanceManager) checkProviderHealth(provider *models.Provider) HealthCheckResult {
	result := HealthCheckResult{
		ProviderID:   provider.ID.String(),
		ProviderName: provider.Name,
		Timestamp:    time.Now(),
	}

	start := time.Now()

	// Usa il monitor di health esistente se disponibile
	if mm.healthMonitor != nil {
		// TODO: Implement actual health check using the monitor
		// For now, simulate a basic check
		result.Status = "healthy"
		result.HealthScore = provider.HealthScore
		result.LatencyMs = time.Since(start).Milliseconds()
	} else {
		result.Status = "unknown"
		result.Error = "health monitor not available"
	}

	// Update provider health in database
	provider.LastHealthCheck = time.Now()
	provider.AvgLatencyMs = int(result.LatencyMs)
	mm.db.Save(provider)

	return result
}

// VacuumDatabase esegue VACUUM sul database (SQLite)
func (mm *MaintenanceManager) VacuumDatabase() error {
	sqlDB, err := mm.db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	log.Info().Msg("Starting database VACUUM (this may take a while)...")

	start := time.Now()
	if _, err := sqlDB.Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	duration := time.Since(start)
	log.Info().Dur("duration", duration).Msg("Database VACUUM completed")

	return nil
}

// OptimizeDatabase esegue operazioni di ottimizzazione complete
func (mm *MaintenanceManager) OptimizeDatabase() error {
	log.Info().Msg("Starting full database optimization")

	// Step 1: Prune old logs (keep last 30 days or 10000 entries)
	if deleted, err := mm.PruneLogs(30, 10000, false); err != nil {
		log.Warn().Err(err).Msg("Failed to prune logs during optimization")
	} else {
		log.Info().Int64("deleted", deleted).Msg("Logs pruned")
	}

	// Step 2: Reindex
	if err := mm.ReindexDatabase(); err != nil {
		log.Warn().Err(err).Msg("Failed to reindex during optimization")
	}

	// Step 3: Vacuum (SQLite only)
	if err := mm.VacuumDatabase(); err != nil {
		log.Warn().Err(err).Msg("Failed to vacuum during optimization")
	}

	// Step 4: Clear cache
	if _, err := mm.ClearCache(); err != nil {
		log.Warn().Err(err).Msg("Failed to clear cache during optimization")
	}

	log.Info().Msg("Database optimization completed")

	return nil
}

// GetDatabaseStats restituisce statistiche dettagliate del database
func (mm *MaintenanceManager) GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Table counts
	tableCounts := make(map[string]int64)
	tables := map[string]interface{}{
		"providers":      &models.Provider{},
		"models":         &models.Model{},
		"accounts":       &models.Account{},
		"users":          &models.User{},
		"api_keys":       &models.APIKey{},
		"request_logs":   &models.RequestLog{},
		"provider_stats": &models.ProviderStats{},
		"rate_limits":    &models.RateLimit{},
	}

	for name, model := range tables {
		var count int64
		mm.db.Model(model).Count(&count)
		tableCounts[name] = count
	}
	stats["table_counts"] = tableCounts

	// Database size
	sqlDB, err := mm.db.DB.DB()
	if err == nil {
		var size int64
		sqlDB.QueryRow("SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
		stats["database_size_bytes"] = size
		stats["database_size_mb"] = float64(size) / 1024.0 / 1024.0
	}

	// Request log statistics
	var oldestLog, newestLog time.Time
	mm.db.Model(&models.RequestLog{}).Select("MIN(timestamp)").Scan(&oldestLog)
	mm.db.Model(&models.RequestLog{}).Select("MAX(timestamp)").Scan(&newestLog)

	if !oldestLog.IsZero() {
		stats["oldest_log"] = oldestLog
		stats["newest_log"] = newestLog
		stats["log_timespan_days"] = newestLog.Sub(oldestLog).Hours() / 24
	}

	// Success rate
	var totalRequests, successfulRequests int64
	mm.db.Model(&models.RequestLog{}).Count(&totalRequests)
	mm.db.Model(&models.RequestLog{}).Where("success = ?", true).Count(&successfulRequests)
	if totalRequests > 0 {
		stats["success_rate"] = float64(successfulRequests) / float64(totalRequests)
	}

	// Average latency
	var avgLatency float64
	mm.db.Model(&models.RequestLog{}).Select("AVG(latency_ms)").Scan(&avgLatency)
	stats["avg_latency_ms"] = avgLatency

	return stats, nil
}

// ScheduledMaintenance esegue manutenzione programmata
func (mm *MaintenanceManager) ScheduledMaintenance() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		log.Info().Msg("Running scheduled maintenance")

		// Prune logs older than 30 days
		if deleted, err := mm.PruneLogs(30, 10000, false); err != nil {
			log.Error().Err(err).Msg("Scheduled log pruning failed")
		} else {
			log.Info().Int64("deleted", deleted).Msg("Scheduled log pruning completed")
		}

		// Clear cache
		if cleared, err := mm.ClearCache(); err != nil {
			log.Error().Err(err).Msg("Scheduled cache clear failed")
		} else {
			log.Info().Int("cleared", cleared).Msg("Scheduled cache clear completed")
		}

		// Force garbage collection
		runtime.GC()
	}
}
