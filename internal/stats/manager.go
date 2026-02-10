package stats

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Manager gestisce tutti i componenti del sistema statistiche
type Manager struct {
	db *database.DB

	collector  *Collector
	aggregator *Aggregator
	prometheus *PrometheusExporter
	dashboard  *Dashboard

	mu     sync.RWMutex
	started bool
}

// Config configurazione del manager
type Config struct {
	// Collector
	BufferSize          int
	CollectorFlushInterval time.Duration

	// Aggregator
	AggregationInterval time.Duration
	RetentionDays       int

	// Prometheus
	PrometheusNamespace string
	PrometheusEnabled   bool
}

// DefaultConfig restituisce una configurazione di default
func DefaultConfig() *Config {
	return &Config{
		BufferSize:             100,
		CollectorFlushInterval: 10 * time.Second,
		AggregationInterval:    1 * time.Minute,
		RetentionDays:          30,
		PrometheusNamespace:    "goleapai",
		PrometheusEnabled:      true,
	}
}

// NewManager crea un nuovo manager
func NewManager(db *database.DB, cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Create collector
	collector := NewCollector(db, cfg.BufferSize)

	// Create aggregator
	aggregator := NewAggregator(db, collector, cfg.AggregationInterval, cfg.RetentionDays)

	// Create prometheus exporter
	var prometheus *PrometheusExporter
	if cfg.PrometheusEnabled {
		prometheus = NewPrometheusExporter(db, collector, cfg.PrometheusNamespace)
	}

	// Create dashboard
	dashboard := NewDashboard(db, collector, aggregator)

	return &Manager{
		db:         db,
		collector:  collector,
		aggregator: aggregator,
		prometheus: prometheus,
		dashboard:  dashboard,
	}
}

// Start avvia tutti i componenti
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}

	// Start collector
	m.collector.Start(m.aggregator.aggregationInterval)

	// Start aggregator
	m.aggregator.Start()

	// Start prometheus exporter
	if m.prometheus != nil {
		m.prometheus.Start()
	}

	m.started = true
	log.Info().Msg("Stats manager started")

	return nil
}

// Stop ferma tutti i componenti
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	// Stop in reverse order
	if m.prometheus != nil {
		m.prometheus.Stop()
	}

	m.aggregator.Stop()
	m.collector.Stop()

	m.started = false
	log.Info().Msg("Stats manager stopped")
}

// IsStarted verifica se il manager è avviato
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// Record registra una richiesta
func (m *Manager) Record(metrics *RequestMetrics) {
	if !m.IsStarted() {
		return
	}

	// Record in collector
	m.collector.Record(metrics)

	// Record in prometheus if enabled
	if m.prometheus != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		providerName, err := m.prometheus.GetProviderLabels(ctx, metrics.ProviderID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get provider labels")
			return
		}

		modelName := ""
		if metrics.ModelID != (uuid.Nil) {
			_, modelName, err = m.prometheus.GetModelLabels(ctx, metrics.ModelID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get model labels")
			}
		}

		errorType := ""
		if !metrics.Success {
			if metrics.StatusCode == 429 {
				errorType = "rate_limit"
			} else if metrics.StatusCode == 504 || metrics.StatusCode == 408 {
				errorType = "timeout"
			} else if metrics.StatusCode >= 500 {
				errorType = "server_error"
			} else if metrics.StatusCode >= 400 {
				errorType = "client_error"
			}
		}

		m.prometheus.RecordRequestComplete(
			providerName,
			modelName,
			metrics.Success,
			metrics.LatencyMs,
			metrics.InputTokens,
			metrics.OutputTokens,
			metrics.EstimatedCost,
			errorType,
		)
	}
}

// RecordStart registra l'inizio di una richiesta
func (m *Manager) RecordStart(ctx context.Context, providerID uuid.UUID) {
	if !m.IsStarted() || m.prometheus == nil {
		return
	}

	providerName, err := m.prometheus.GetProviderLabels(ctx, providerID)
	if err != nil {
		return
	}

	m.prometheus.IncRequestsInFlight(providerName)
}

// Collector restituisce il collector
func (m *Manager) Collector() *Collector {
	return m.collector
}

// Aggregator restituisce l'aggregator
func (m *Manager) Aggregator() *Aggregator {
	return m.aggregator
}

// Prometheus restituisce il prometheus exporter
func (m *Manager) Prometheus() *PrometheusExporter {
	return m.prometheus
}

// Dashboard restituisce il dashboard
func (m *Manager) Dashboard() *Dashboard {
	return m.dashboard
}

// GetDashboardData ottiene tutti i dati del dashboard
func (m *Manager) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return m.dashboard.GetDashboardData(ctx)
}

// GetProviderStats ottiene statistiche per un provider
func (m *Manager) GetProviderStats(ctx context.Context, providerID uuid.UUID) (*ProviderStatsSnapshot, error) {
	return m.collector.GetStats(providerID), nil
}

// GetSummary ottiene statistiche di riepilogo
func (m *Manager) GetSummary(ctx context.Context) (*SummaryStats, error) {
	return m.dashboard.GetSummary(ctx)
}

// GetHourlyTrends ottiene trend orari
func (m *Manager) GetHourlyTrends(ctx context.Context, hours int) ([]*TrendPoint, error) {
	return m.dashboard.GetHourlyTrends(ctx, hours)
}

// GetDailyTrends ottiene trend giornalieri
func (m *Manager) GetDailyTrends(ctx context.Context, days int) ([]*TrendPoint, error) {
	return m.dashboard.GetDailyTrends(ctx, days)
}

// GetCostSavings calcola i risparmi
func (m *Manager) GetCostSavings(ctx context.Context) (*CostSavingsData, error) {
	return m.dashboard.GetCostSavings(ctx)
}
