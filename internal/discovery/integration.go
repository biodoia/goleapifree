package discovery

import (
	"context"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

// IntegrationConfig contiene la configurazione per l'integrazione del discovery
type IntegrationConfig struct {
	Enabled              bool
	Interval             time.Duration
	GitHubToken          string
	GitHubEnabled        bool
	ScraperEnabled       bool
	MaxConcurrent        int
	ValidationTimeout    time.Duration
	MinHealthScore       float64
	DiscoverySearchTerms []string
}

// InitFromAppConfig crea una DiscoveryConfig dalla configurazione dell'app
func InitFromAppConfig(cfg *config.Config) *DiscoveryConfig {
	// Estrai configurazione discovery (se presente nel config)
	// Per ora usiamo valori di default sensati
	return &DiscoveryConfig{
		Enabled:           cfg.Providers.AutoDiscovery,
		Interval:          24 * time.Hour, // Discovery giornaliero
		GitHubToken:       "", // Da configurare via env var GITHUB_TOKEN
		GitHubEnabled:     true,
		ScraperEnabled:    true,
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
		DiscoverySearchTerms: []string{
			"free llm api",
			"free ai api",
			"free gpt api",
			"free claude api",
			"ai proxy free",
			"llm gateway",
			"openai compatible",
			"chatgpt free",
			"gpt4free",
		},
	}
}

// StartDiscoveryService avvia il servizio di discovery
func StartDiscoveryService(ctx context.Context, cfg *config.Config, db *database.DB, logger zerolog.Logger) (*Engine, error) {
	discoveryConfig := InitFromAppConfig(cfg)

	// Controlla se discovery è abilitato
	if !discoveryConfig.Enabled {
		logger.Info().Msg("Discovery service is disabled")
		return nil, nil
	}

	// Crea engine
	engine := NewEngine(discoveryConfig, db, logger)

	// Avvia engine
	if err := engine.Start(ctx); err != nil {
		return nil, err
	}

	logger.Info().
		Dur("interval", discoveryConfig.Interval).
		Bool("github", discoveryConfig.GitHubEnabled).
		Bool("scraper", discoveryConfig.ScraperEnabled).
		Msg("Discovery service started")

	return engine, nil
}

// ScheduleVerification schedula la verifica periodica dei provider esistenti
func ScheduleVerification(ctx context.Context, db *database.DB, validator *Validator, interval time.Duration, logger zerolog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info().
		Dur("interval", interval).
		Msg("Provider verification scheduler started")

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Provider verification scheduler stopped")
			return
		case <-ticker.C:
			if err := VerifyExistingProviders(ctx, db, validator, logger); err != nil {
				logger.Error().Err(err).Msg("Provider verification failed")
			}
		}
	}
}

// VerifyExistingProviders verifica tutti i provider esistenti nel database
func VerifyExistingProviders(ctx context.Context, db *database.DB, validator *Validator, logger zerolog.Logger) error {
	logger.Info().Msg("Starting provider verification")

	// Carica tutti i provider attivi
	var providers []struct {
		ID       string
		Name     string
		BaseURL  string
		AuthType string
	}

	err := db.Table("providers").
		Where("status = ?", "active").
		Select("id, name, base_url, auth_type").
		Scan(&providers).Error

	if err != nil {
		return err
	}

	logger.Info().Int("count", len(providers)).Msg("Verifying providers")

	verified := 0
	failed := 0

	for _, provider := range providers {
		// Valida il provider
		result, err := validator.ValidateEndpoint(ctx, provider.BaseURL, models.AuthType(provider.AuthType))
		if err != nil {
			logger.Warn().
				Str("provider", provider.Name).
				Err(err).
				Msg("Provider validation failed")
			failed++
			continue
		}

		// Aggiorna metriche nel database
		updates := map[string]interface{}{
			"last_health_check": time.Now(),
			"health_score":      result.HealthScore,
			"avg_latency_ms":    result.LatencyMs,
		}

		// Se health score troppo basso, marca come down
		if result.HealthScore < 0.3 {
			updates["status"] = "down"
			logger.Warn().
				Str("provider", provider.Name).
				Float64("health_score", result.HealthScore).
				Msg("Provider marked as down")
		} else {
			updates["status"] = "active"
			verified++
		}

		err = db.Table("providers").
			Where("id = ?", provider.ID).
			Updates(updates).Error

		if err != nil {
			logger.Error().
				Str("provider", provider.Name).
				Err(err).
				Msg("Failed to update provider")
		}
	}

	logger.Info().
		Int("verified", verified).
		Int("failed", failed).
		Int("total", len(providers)).
		Msg("Provider verification completed")

	return nil
}

// GetDiscoveryStats ritorna statistiche sul discovery
func GetDiscoveryStats(db *database.DB) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Conta provider per source
	var sourceCounts []struct {
		Source string
		Count  int64
	}

	err := db.Table("providers").
		Select("source, count(*) as count").
		Group("source").
		Scan(&sourceCounts).Error

	if err != nil {
		return nil, err
	}

	sourceMap := make(map[string]int64)
	for _, sc := range sourceCounts {
		sourceMap[sc.Source] = sc.Count
	}
	stats["by_source"] = sourceMap

	// Conta provider per status
	var statusCounts []struct {
		Status string
		Count  int64
	}

	err = db.Table("providers").
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error

	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]int64)
	for _, sc := range statusCounts {
		statusMap[sc.Status] = sc.Count
	}
	stats["by_status"] = statusMap

	// Provider scoperti di recente (ultimi 7 giorni)
	var recentCount int64
	err = db.Table("providers").
		Where("discovered_at > ?", time.Now().AddDate(0, 0, -7)).
		Count(&recentCount).Error

	if err != nil {
		return nil, err
	}
	stats["discovered_last_7_days"] = recentCount

	// Health score medio
	var avgHealthScore float64
	err = db.Table("providers").
		Where("status = ?", "active").
		Select("AVG(health_score)").
		Scan(&avgHealthScore).Error

	if err != nil {
		return nil, err
	}
	stats["avg_health_score"] = avgHealthScore

	return stats, nil
}
