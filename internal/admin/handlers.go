package admin

import (
	"time"

	"github.com/biodoia/goleapifree/internal/health"
	"github.com/biodoia/goleapifree/internal/providers/anthropic"
	"github.com/biodoia/goleapifree/internal/providers/openai"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/middleware"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AdminHandlers gestisce gli endpoint amministrativi
type AdminHandlers struct {
	db            *database.DB
	config        *config.Config
	healthMonitor *health.Monitor
	userManager   *UserManager
	backupManager *BackupManager
	maintenance   *MaintenanceManager
}

// NewAdminHandlers crea una nuova istanza degli handler admin
func NewAdminHandlers(db *database.DB, cfg *config.Config, healthMonitor *health.Monitor) *AdminHandlers {
	return &AdminHandlers{
		db:            db,
		config:        cfg,
		healthMonitor: healthMonitor,
		userManager:   NewUserManager(db),
		backupManager: NewBackupManager(db, cfg),
		maintenance:   NewMaintenanceManager(db, healthMonitor),
	}
}

// RegisterRoutes registra tutte le route admin
func (h *AdminHandlers) RegisterRoutes(app *fiber.App, authConfig middleware.AuthConfig) {
	admin := app.Group("/admin")

	// Require admin role for all admin endpoints
	admin.Use(middleware.Auth(authConfig))
	admin.Use(middleware.RequireRole("admin"))

	// Provider management
	admin.Get("/providers", h.ListProviders)
	admin.Post("/providers", h.AddProvider)
	admin.Put("/providers/:id", h.UpdateProvider)
	admin.Delete("/providers/:id", h.DeleteProvider)
	admin.Put("/providers/:id/test", h.TestProvider)
	admin.Post("/providers/:id/toggle", h.ToggleProvider)

	// Account management
	admin.Get("/accounts", h.ListAccounts)
	admin.Post("/accounts", h.CreateAccount)
	admin.Put("/accounts/:id", h.UpdateAccount)
	admin.Delete("/accounts/:id", h.DeleteAccount)

	// User management
	admin.Get("/users", h.ListUsers)
	admin.Post("/users", h.CreateUser)
	admin.Put("/users/:id", h.UpdateUser)
	admin.Delete("/users/:id", h.DeleteUser)
	admin.Post("/users/:id/reset-quota", h.ResetUserQuota)
	admin.Get("/users/:id/api-keys", h.ListUserAPIKeys)

	// Statistics
	admin.Get("/stats", h.GetDetailedStats)
	admin.Get("/stats/providers", h.GetProviderStats)
	admin.Get("/stats/users", h.GetUserStats)
	admin.Get("/stats/requests", h.GetRequestStats)

	// Configuration
	admin.Get("/config", h.GetConfig)
	admin.Post("/config", h.UpdateConfig)

	// Backup & Restore
	admin.Post("/backup", h.CreateBackup)
	admin.Get("/backup", h.ListBackups)
	admin.Post("/restore", h.RestoreBackup)
	admin.Post("/export", h.ExportConfiguration)
	admin.Post("/import", h.ImportProviders)

	// Maintenance
	admin.Post("/maintenance/clear-cache", h.ClearCache)
	admin.Post("/maintenance/prune-logs", h.PruneLogs)
	admin.Post("/maintenance/reindex", h.ReindexDatabase)
	admin.Post("/maintenance/health-check", h.ForceHealthCheckAll)
	admin.Get("/maintenance/status", h.GetMaintenanceStatus)
}

// ListProviders restituisce tutti i provider
func (h *AdminHandlers) ListProviders(c fiber.Ctx) error {
	var providers []models.Provider

	query := h.db.Preload("Models").Preload("RateLimits")

	// Filter by status
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Filter by type
	if providerType := c.Query("type"); providerType != "" {
		query = query.Where("type = ?", providerType)
	}

	if err := query.Find(&providers).Error; err != nil {
		log.Error().Err(err).Msg("Failed to list providers")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve providers",
		})
	}

	return c.JSON(fiber.Map{
		"providers": providers,
		"count":     len(providers),
	})
}

// AddProvider aggiunge un nuovo provider
func (h *AdminHandlers) AddProvider(c fiber.Ctx) error {
	var req struct {
		Name              string              `json:"name"`
		Type              models.ProviderType `json:"type"`
		BaseURL           string              `json:"base_url"`
		AuthType          models.AuthType     `json:"auth_type"`
		Tier              int                 `json:"tier"`
		SupportsStreaming bool                `json:"supports_streaming"`
		SupportsTools     bool                `json:"supports_tools"`
		SupportsJSON      bool                `json:"supports_json"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Name == "" || req.BaseURL == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Name and base_url are required",
		})
	}

	provider := models.Provider{
		ID:                uuid.New(),
		Name:              req.Name,
		Type:              req.Type,
		BaseURL:           req.BaseURL,
		AuthType:          req.AuthType,
		Tier:              req.Tier,
		Status:            models.ProviderStatusActive,
		SupportsStreaming: req.SupportsStreaming,
		SupportsTools:     req.SupportsTools,
		SupportsJSON:      req.SupportsJSON,
		DiscoveredAt:      time.Now(),
		Source:            "manual",
		HealthScore:       1.0,
	}

	if err := h.db.Create(&provider).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create provider")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create provider",
		})
	}

	log.Info().Str("provider", provider.Name).Msg("Provider created successfully")

	return c.Status(201).JSON(fiber.Map{
		"message":  "Provider created successfully",
		"provider": provider,
	})
}

// UpdateProvider aggiorna un provider esistente
func (h *AdminHandlers) UpdateProvider(c fiber.Ctx) error {
	providerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid provider ID",
		})
	}

	var provider models.Provider
	if err := h.db.First(&provider, providerID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Provider not found",
		})
	}

	var req struct {
		Name              *string              `json:"name"`
		Type              *models.ProviderType `json:"type"`
		BaseURL           *string              `json:"base_url"`
		Status            *models.ProviderStatus `json:"status"`
		Tier              *int                 `json:"tier"`
		SupportsStreaming *bool                `json:"supports_streaming"`
		SupportsTools     *bool                `json:"supports_tools"`
		SupportsJSON      *bool                `json:"supports_json"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update only provided fields
	if req.Name != nil {
		provider.Name = *req.Name
	}
	if req.Type != nil {
		provider.Type = *req.Type
	}
	if req.BaseURL != nil {
		provider.BaseURL = *req.BaseURL
	}
	if req.Status != nil {
		provider.Status = *req.Status
	}
	if req.Tier != nil {
		provider.Tier = *req.Tier
	}
	if req.SupportsStreaming != nil {
		provider.SupportsStreaming = *req.SupportsStreaming
	}
	if req.SupportsTools != nil {
		provider.SupportsTools = *req.SupportsTools
	}
	if req.SupportsJSON != nil {
		provider.SupportsJSON = *req.SupportsJSON
	}

	if err := h.db.Save(&provider).Error; err != nil {
		log.Error().Err(err).Msg("Failed to update provider")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to update provider",
		})
	}

	log.Info().Str("provider", provider.Name).Msg("Provider updated successfully")

	return c.JSON(fiber.Map{
		"message":  "Provider updated successfully",
		"provider": provider,
	})
}

// DeleteProvider rimuove un provider
func (h *AdminHandlers) DeleteProvider(c fiber.Ctx) error {
	providerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid provider ID",
		})
	}

	var provider models.Provider
	if err := h.db.First(&provider, providerID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Provider not found",
		})
	}

	// Delete associated models and rate limits
	h.db.Where("provider_id = ?", providerID).Delete(&models.Model{})
	h.db.Where("provider_id = ?", providerID).Delete(&models.RateLimit{})

	// Delete provider
	if err := h.db.Delete(&provider).Error; err != nil {
		log.Error().Err(err).Msg("Failed to delete provider")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to delete provider",
		})
	}

	log.Info().Str("provider", provider.Name).Msg("Provider deleted successfully")

	return c.JSON(fiber.Map{
		"message": "Provider deleted successfully",
	})
}

// TestProvider testa la connessione a un provider
func (h *AdminHandlers) TestProvider(c fiber.Ctx) error {
	providerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid provider ID",
		})
	}

	var provider models.Provider
	if err := h.db.First(&provider, providerID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Provider not found",
		})
	}

	// Test connection based on provider type
	start := time.Now()
	var testErr error

	switch {
	case provider.Name == "anthropic" || provider.BaseURL == "https://api.anthropic.com":
		client := anthropic.NewClient(provider.BaseURL, "test-key", 30*time.Second)
		testErr = client.HealthCheck()
	case provider.Name == "openai" || provider.BaseURL == "https://api.openai.com":
		client := openai.NewClient(provider.BaseURL, "test-key", 30*time.Second)
		testErr = client.HealthCheck()
	default:
		// Generic HTTP health check
		testErr = nil // TODO: Implement generic health check
	}

	latency := time.Since(start).Milliseconds()

	if testErr != nil {
		return c.Status(503).JSON(fiber.Map{
			"status":     "unhealthy",
			"error":      testErr.Error(),
			"latency_ms": latency,
		})
	}

	// Update health status
	provider.LastHealthCheck = time.Now()
	provider.HealthScore = 1.0
	provider.AvgLatencyMs = int(latency)
	h.db.Save(&provider)

	return c.JSON(fiber.Map{
		"status":     "healthy",
		"latency_ms": latency,
		"provider":   provider.Name,
	})
}

// ToggleProvider attiva/disattiva un provider
func (h *AdminHandlers) ToggleProvider(c fiber.Ctx) error {
	providerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid provider ID",
		})
	}

	var provider models.Provider
	if err := h.db.First(&provider, providerID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Provider not found",
		})
	}

	// Toggle status
	if provider.Status == models.ProviderStatusActive {
		provider.Status = models.ProviderStatusDown
	} else {
		provider.Status = models.ProviderStatusActive
	}

	if err := h.db.Save(&provider).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to toggle provider status",
		})
	}

	return c.JSON(fiber.Map{
		"message":  "Provider status toggled",
		"provider": provider,
	})
}

// ListAccounts restituisce tutti gli account
func (h *AdminHandlers) ListAccounts(c fiber.Ctx) error {
	var accounts []models.Account

	query := h.db.Preload("Provider")

	// Filter by provider
	if providerID := c.Query("provider_id"); providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	// Filter by user
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Find(&accounts).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve accounts",
		})
	}

	return c.JSON(fiber.Map{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// CreateAccount crea un nuovo account
func (h *AdminHandlers) CreateAccount(c fiber.Ctx) error {
	var req struct {
		UserID      uuid.UUID `json:"user_id"`
		ProviderID  uuid.UUID `json:"provider_id"`
		Credentials string    `json:"credentials"`
		QuotaLimit  int64     `json:"quota_limit"`
		ExpiresAt   time.Time `json:"expires_at"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// TODO: Encrypt credentials before storing
	account := models.Account{
		ID:         uuid.New(),
		UserID:     req.UserID,
		ProviderID: req.ProviderID,
		QuotaLimit: req.QuotaLimit,
		ExpiresAt:  req.ExpiresAt,
		Active:     true,
		LastReset:  time.Now(),
	}

	if err := h.db.Create(&account).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create account",
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Account created successfully",
		"account": account,
	})
}

// UpdateAccount aggiorna un account
func (h *AdminHandlers) UpdateAccount(c fiber.Ctx) error {
	accountID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}

	var account models.Account
	if err := h.db.First(&account, accountID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Account not found",
		})
	}

	var req struct {
		Active     *bool      `json:"active"`
		QuotaLimit *int64     `json:"quota_limit"`
		QuotaUsed  *int64     `json:"quota_used"`
		ExpiresAt  *time.Time `json:"expires_at"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Active != nil {
		account.Active = *req.Active
	}
	if req.QuotaLimit != nil {
		account.QuotaLimit = *req.QuotaLimit
	}
	if req.QuotaUsed != nil {
		account.QuotaUsed = *req.QuotaUsed
	}
	if req.ExpiresAt != nil {
		account.ExpiresAt = *req.ExpiresAt
	}

	if err := h.db.Save(&account).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to update account",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Account updated successfully",
		"account": account,
	})
}

// DeleteAccount elimina un account
func (h *AdminHandlers) DeleteAccount(c fiber.Ctx) error {
	accountID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid account ID",
		})
	}

	if err := h.db.Delete(&models.Account{}, accountID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to delete account",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Account deleted successfully",
	})
}

// GetDetailedStats restituisce statistiche dettagliate
func (h *AdminHandlers) GetDetailedStats(c fiber.Ctx) error {
	timeRange := c.Query("range", "24h") // 1h, 24h, 7d, 30d

	stats := fiber.Map{
		"time_range": timeRange,
		"timestamp":  time.Now(),
	}

	// Provider count
	var providerCount int64
	h.db.Model(&models.Provider{}).Count(&providerCount)
	stats["total_providers"] = providerCount

	// Active providers
	var activeProviders int64
	h.db.Model(&models.Provider{}).Where("status = ?", models.ProviderStatusActive).Count(&activeProviders)
	stats["active_providers"] = activeProviders

	// User count
	var userCount int64
	h.db.Model(&models.User{}).Count(&userCount)
	stats["total_users"] = userCount

	// Request stats
	var totalRequests int64
	h.db.Model(&models.RequestLog{}).Count(&totalRequests)
	stats["total_requests"] = totalRequests

	// Success rate
	var successfulRequests int64
	h.db.Model(&models.RequestLog{}).Where("success = ?", true).Count(&successfulRequests)
	if totalRequests > 0 {
		stats["success_rate"] = float64(successfulRequests) / float64(totalRequests)
	} else {
		stats["success_rate"] = 1.0
	}

	// Average latency
	var avgLatency float64
	h.db.Model(&models.RequestLog{}).Select("AVG(latency_ms)").Row().Scan(&avgLatency)
	stats["avg_latency_ms"] = avgLatency

	// Total tokens
	var totalTokens int64
	h.db.Model(&models.RequestLog{}).Select("SUM(input_tokens + output_tokens)").Row().Scan(&totalTokens)
	stats["total_tokens"] = totalTokens

	// Cost saved
	var costSaved float64
	h.db.Model(&models.ProviderStats{}).Select("SUM(cost_saved)").Row().Scan(&costSaved)
	stats["cost_saved"] = costSaved

	return c.JSON(stats)
}

// GetProviderStats restituisce statistiche per provider
func (h *AdminHandlers) GetProviderStats(c fiber.Ctx) error {
	var stats []models.ProviderStats

	query := h.db.Preload("Provider")

	// Time range filter
	if timeRange := c.Query("range"); timeRange != "" {
		// TODO: Parse time range and filter
	}

	if err := query.Order("timestamp DESC").Limit(100).Find(&stats).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve provider stats",
		})
	}

	return c.JSON(fiber.Map{
		"stats": stats,
		"count": len(stats),
	})
}

// GetUserStats restituisce statistiche per utente
func (h *AdminHandlers) GetUserStats(c fiber.Ctx) error {
	type UserStat struct {
		UserID         uuid.UUID `json:"user_id"`
		Email          string    `json:"email"`
		TotalRequests  int64     `json:"total_requests"`
		TotalTokens    int64     `json:"total_tokens"`
		QuotaUsed      int64     `json:"quota_used"`
		QuotaLimit     int64     `json:"quota_limit"`
		LastRequestAt  time.Time `json:"last_request_at"`
	}

	var stats []UserStat

	err := h.db.Model(&models.User{}).
		Select(`users.id as user_id,
				users.email,
				users.quota_tokens_used as quota_used,
				users.quota_tokens as quota_limit,
				COUNT(request_logs.id) as total_requests,
				SUM(request_logs.input_tokens + request_logs.output_tokens) as total_tokens,
				MAX(request_logs.timestamp) as last_request_at`).
		Joins("LEFT JOIN request_logs ON request_logs.user_id = users.id").
		Group("users.id").
		Scan(&stats).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve user stats",
		})
	}

	return c.JSON(fiber.Map{
		"stats": stats,
		"count": len(stats),
	})
}

// GetRequestStats restituisce statistiche delle richieste
func (h *AdminHandlers) GetRequestStats(c fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)

	var logs []models.RequestLog
	query := h.db.Order("timestamp DESC").Limit(limit).Offset(offset)

	// Filter by user
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// Filter by provider
	if providerID := c.Query("provider_id"); providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	// Filter by success
	if success := c.Query("success"); success != "" {
		query = query.Where("success = ?", success == "true")
	}

	if err := query.Find(&logs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve request logs",
		})
	}

	return c.JSON(fiber.Map{
		"logs":   logs,
		"count":  len(logs),
		"limit":  limit,
		"offset": offset,
	})
}

// GetConfig restituisce la configurazione corrente
func (h *AdminHandlers) GetConfig(c fiber.Ctx) error {
	// Return sanitized config (without sensitive data)
	safeConfig := fiber.Map{
		"server": fiber.Map{
			"port":  h.config.Server.Port,
			"host":  h.config.Server.Host,
			"http3": h.config.Server.HTTP3,
			"tls": fiber.Map{
				"enabled": h.config.Server.TLS.Enabled,
			},
		},
		"providers": fiber.Map{
			"auto_discovery":        h.config.Providers.AutoDiscovery,
			"health_check_interval": h.config.Providers.HealthCheckInterval,
			"default_timeout":       h.config.Providers.DefaultTimeout,
		},
		"routing": fiber.Map{
			"strategy":         h.config.Routing.Strategy,
			"failover_enabled": h.config.Routing.FailoverEnabled,
			"max_retries":      h.config.Routing.MaxRetries,
		},
		"monitoring": fiber.Map{
			"prometheus": fiber.Map{
				"enabled": h.config.Monitoring.Prometheus.Enabled,
				"port":    h.config.Monitoring.Prometheus.Port,
			},
			"logging": fiber.Map{
				"level":  h.config.Monitoring.Logging.Level,
				"format": h.config.Monitoring.Logging.Format,
			},
		},
	}

	return c.JSON(safeConfig)
}

// UpdateConfig aggiorna la configurazione
func (h *AdminHandlers) UpdateConfig(c fiber.Ctx) error {
	var req struct {
		Routing struct {
			Strategy        *string `json:"strategy"`
			FailoverEnabled *bool   `json:"failover_enabled"`
			MaxRetries      *int    `json:"max_retries"`
		} `json:"routing"`
		Providers struct {
			AutoDiscovery        *bool   `json:"auto_discovery"`
			HealthCheckInterval  *string `json:"health_check_interval"`
		} `json:"providers"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update routing config
	if req.Routing.Strategy != nil {
		h.config.Routing.Strategy = *req.Routing.Strategy
	}
	if req.Routing.FailoverEnabled != nil {
		h.config.Routing.FailoverEnabled = *req.Routing.FailoverEnabled
	}
	if req.Routing.MaxRetries != nil {
		h.config.Routing.MaxRetries = *req.Routing.MaxRetries
	}

	// Update providers config
	if req.Providers.AutoDiscovery != nil {
		h.config.Providers.AutoDiscovery = *req.Providers.AutoDiscovery
	}
	if req.Providers.HealthCheckInterval != nil {
		h.config.Providers.HealthCheckInterval = *req.Providers.HealthCheckInterval
	}

	log.Info().Msg("Configuration updated successfully")

	return c.JSON(fiber.Map{
		"message": "Configuration updated successfully",
		"config":  h.config,
	})
}
