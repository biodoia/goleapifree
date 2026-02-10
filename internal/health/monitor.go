package health

import (
	"context"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog/log"
)

// Monitor gestisce il monitoraggio della salute dei provider
type Monitor struct {
	db       *database.DB
	interval string
	ticker   *time.Ticker
	done     chan bool
}

// NewMonitor crea un nuovo monitor
func NewMonitor(db *database.DB, interval string) *Monitor {
	return &Monitor{
		db:       db,
		interval: interval,
		done:     make(chan bool),
	}
}

// Start avvia il monitoraggio
func (m *Monitor) Start() {
	duration, err := time.ParseDuration(m.interval)
	if err != nil {
		log.Error().Err(err).Msg("Invalid health check interval, using default 5m")
		duration = 5 * time.Minute
	}

	m.ticker = time.NewTicker(duration)

	go func() {
		// Run initial check
		m.checkAllProviders()

		for {
			select {
			case <-m.ticker.C:
				m.checkAllProviders()
			case <-m.done:
				return
			}
		}
	}()

	log.Info().Str("interval", m.interval).Msg("Health monitoring started")
}

// Stop ferma il monitoraggio
func (m *Monitor) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.done <- true
	log.Info().Msg("Health monitoring stopped")
}

// checkAllProviders esegue health check su tutti i provider attivi
func (m *Monitor) checkAllProviders() {
	var providers []models.Provider

	result := m.db.Where("status = ?", models.ProviderStatusActive).Find(&providers)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to fetch providers for health check")
		return
	}

	log.Debug().Int("count", len(providers)).Msg("Starting health check")

	for _, provider := range providers {
		go m.checkProvider(&provider)
	}
}

// checkProvider esegue health check su un singolo provider
func (m *Monitor) checkProvider(provider *models.Provider) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()

	// TODO: Implement actual HTTP health check
	_ = ctx

	latency := time.Since(start).Milliseconds()

	// Update provider health metrics
	provider.LastHealthCheck = time.Now()
	provider.AvgLatencyMs = int(latency)
	provider.HealthScore = 1.0 // TODO: Calculate based on success rate

	if err := m.db.Save(provider).Error; err != nil {
		log.Error().
			Err(err).
			Str("provider", provider.Name).
			Msg("Failed to update provider health metrics")
	}

	log.Debug().
		Str("provider", provider.Name).
		Int64("latency_ms", latency).
		Float64("health_score", provider.HealthScore).
		Msg("Health check completed")
}
