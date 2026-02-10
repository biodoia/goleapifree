package stats

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
)

// PrometheusExporter espone metriche in formato Prometheus
type PrometheusExporter struct {
	db        *database.DB
	collector *Collector

	// Metrics
	requestsTotal      *prometheus.CounterVec
	requestDuration    *prometheus.HistogramVec
	requestErrors      *prometheus.CounterVec
	activeProviders    prometheus.Gauge
	providerHealth     *prometheus.GaugeVec
	tokensProcessed    *prometheus.CounterVec
	quotaUsage         *prometheus.GaugeVec
	costSaved          *prometheus.CounterVec
	requestsInFlight   *prometheus.GaugeVec

	// Custom metrics
	successRate        *prometheus.GaugeVec
	avgLatency         *prometheus.GaugeVec

	// Control
	updateInterval time.Duration
	ticker         *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewPrometheusExporter crea un nuovo exporter
func NewPrometheusExporter(db *database.DB, collector *Collector, namespace string) *PrometheusExporter {
	if namespace == "" {
		namespace = "goleapai"
	}

	e := &PrometheusExporter{
		db:             db,
		collector:      collector,
		updateInterval: 15 * time.Second,
		stopCh:         make(chan struct{}),
	}

	// Request counters
	e.requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of requests by provider, model and status",
		},
		[]string{"provider", "model", "status"},
	)

	e.requestErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "request_errors_total",
			Help:      "Total number of request errors by provider and error type",
		},
		[]string{"provider", "error_type"},
	)

	// Request duration histogram
	e.requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_milliseconds",
			Help:      "Request duration in milliseconds",
			Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		},
		[]string{"provider", "model"},
	)

	// Active providers gauge
	e.activeProviders = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_providers",
			Help:      "Number of active providers",
		},
	)

	// Provider health score
	e.providerHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "provider_health_score",
			Help:      "Health score of each provider (0.0-1.0)",
		},
		[]string{"provider"},
	)

	// Tokens processed
	e.tokensProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tokens_processed_total",
			Help:      "Total number of tokens processed",
		},
		[]string{"provider", "model", "type"},
	)

	// Quota usage
	e.quotaUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "quota_usage_ratio",
			Help:      "Quota usage ratio by provider (0.0-1.0)",
		},
		[]string{"provider"},
	)

	// Cost saved
	e.costSaved = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cost_saved_total",
			Help:      "Total cost saved compared to official API pricing",
		},
		[]string{"provider"},
	)

	// Requests in flight
	e.requestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "requests_in_flight",
			Help:      "Number of requests currently being processed",
		},
		[]string{"provider"},
	)

	// Success rate
	e.successRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "success_rate",
			Help:      "Success rate by provider (0.0-1.0)",
		},
		[]string{"provider"},
	)

	// Average latency
	e.avgLatency = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "avg_latency_milliseconds",
			Help:      "Average request latency in milliseconds",
		},
		[]string{"provider"},
	)

	return e
}

// Start avvia l'exporter
func (e *PrometheusExporter) Start() {
	e.ticker = time.NewTicker(e.updateInterval)
	e.wg.Add(1)

	go e.updateLoop()
	log.Info().
		Dur("update_interval", e.updateInterval).
		Msg("Prometheus exporter started")
}

// Stop ferma l'exporter
func (e *PrometheusExporter) Stop() {
	if e.ticker != nil {
		e.ticker.Stop()
	}
	close(e.stopCh)
	e.wg.Wait()

	log.Info().Msg("Prometheus exporter stopped")
}

// updateLoop aggiorna periodicamente le metriche gauge
func (e *PrometheusExporter) updateLoop() {
	defer e.wg.Done()

	for {
		select {
		case <-e.ticker.C:
			e.updateGauges()
		case <-e.stopCh:
			return
		}
	}
}

// updateGauges aggiorna le metriche gauge dal database
func (e *PrometheusExporter) updateGauges() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update active providers count
	var activeCount int64
	if err := e.db.WithContext(ctx).
		Model(&models.Provider{}).
		Where("status = ?", models.ProviderStatusActive).
		Count(&activeCount).Error; err != nil {
		log.Error().Err(err).Msg("Failed to count active providers")
	} else {
		e.activeProviders.Set(float64(activeCount))
	}

	// Update provider health scores
	var providers []models.Provider
	if err := e.db.WithContext(ctx).Find(&providers).Error; err != nil {
		log.Error().Err(err).Msg("Failed to fetch providers")
		return
	}

	for _, provider := range providers {
		e.providerHealth.WithLabelValues(provider.Name).Set(provider.HealthScore)

		// Update success rate and avg latency from collector
		if metrics := e.collector.GetProviderMetrics(provider.ID); metrics != nil {
			successRate := 0.0
			avgLatency := 0.0
			if metrics.TotalRequests > 0 {
				successRate = float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
				avgLatency = float64(metrics.TotalLatencyMs) / float64(metrics.TotalRequests)
			}

			e.successRate.WithLabelValues(provider.Name).Set(successRate)
			e.avgLatency.WithLabelValues(provider.Name).Set(avgLatency)
		}
	}
}

// RecordRequest registra una richiesta
func (e *PrometheusExporter) RecordRequest(providerName, modelName, status string) {
	e.requestsTotal.WithLabelValues(providerName, modelName, status).Inc()
}

// RecordDuration registra la durata di una richiesta
func (e *PrometheusExporter) RecordDuration(providerName, modelName string, durationMs float64) {
	e.requestDuration.WithLabelValues(providerName, modelName).Observe(durationMs)
}

// RecordError registra un errore
func (e *PrometheusExporter) RecordError(providerName, errorType string) {
	e.requestErrors.WithLabelValues(providerName, errorType).Inc()
}

// RecordTokens registra i token processati
func (e *PrometheusExporter) RecordTokens(providerName, modelName, tokenType string, count int) {
	e.tokensProcessed.WithLabelValues(providerName, modelName, tokenType).Add(float64(count))
}

// RecordCostSaved registra il costo risparmiato
func (e *PrometheusExporter) RecordCostSaved(providerName string, amount float64) {
	e.costSaved.WithLabelValues(providerName).Add(amount)
}

// SetQuotaUsage imposta l'utilizzo quota
func (e *PrometheusExporter) SetQuotaUsage(providerName string, ratio float64) {
	e.quotaUsage.WithLabelValues(providerName).Set(ratio)
}

// IncRequestsInFlight incrementa le richieste in corso
func (e *PrometheusExporter) IncRequestsInFlight(providerName string) {
	e.requestsInFlight.WithLabelValues(providerName).Inc()
}

// DecRequestsInFlight decrementa le richieste in corso
func (e *PrometheusExporter) DecRequestsInFlight(providerName string) {
	e.requestsInFlight.WithLabelValues(providerName).Dec()
}

// RecordRequestComplete registra una richiesta completa con tutti i dettagli
func (e *PrometheusExporter) RecordRequestComplete(
	providerName, modelName string,
	success bool,
	durationMs int,
	inputTokens, outputTokens int,
	costSaved float64,
	errorType string,
) {
	// Status
	status := "success"
	if !success {
		status = "error"
	}

	// Record metrics
	e.RecordRequest(providerName, modelName, status)
	e.RecordDuration(providerName, modelName, float64(durationMs))

	if !success && errorType != "" {
		e.RecordError(providerName, errorType)
	}

	if inputTokens > 0 {
		e.RecordTokens(providerName, modelName, "input", inputTokens)
	}
	if outputTokens > 0 {
		e.RecordTokens(providerName, modelName, "output", outputTokens)
	}

	if costSaved > 0 {
		e.RecordCostSaved(providerName, costSaved)
	}

	e.DecRequestsInFlight(providerName)
}

// GetProviderLabels ottiene le label per un provider
func (e *PrometheusExporter) GetProviderLabels(ctx context.Context, providerID uuid.UUID) (string, error) {
	var provider models.Provider
	if err := e.db.WithContext(ctx).First(&provider, "id = ?", providerID).Error; err != nil {
		return "", err
	}
	return provider.Name, nil
}

// GetModelLabels ottiene le label per un modello
func (e *PrometheusExporter) GetModelLabels(ctx context.Context, modelID uuid.UUID) (string, string, error) {
	var model models.Model
	if err := e.db.WithContext(ctx).Preload("Provider").First(&model, "id = ?", modelID).Error; err != nil {
		return "", "", err
	}
	return model.Provider.Name, model.Name, nil
}

// Registry restituisce il registry Prometheus
func (e *PrometheusExporter) Registry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
