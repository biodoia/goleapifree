package stats

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TimeWindow rappresenta una finestra temporale per l'aggregazione
type TimeWindow string

const (
	WindowMinute TimeWindow = "minute"
	WindowHour   TimeWindow = "hour"
	WindowDay    TimeWindow = "day"
)

// Aggregator aggrega statistiche in finestre temporali
type Aggregator struct {
	db        *database.DB
	collector *Collector

	// Configuration
	aggregationInterval time.Duration
	retentionDays       int

	// Control
	ticker *time.Ticker
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAggregator crea un nuovo aggregator
func NewAggregator(db *database.DB, collector *Collector, aggregationInterval time.Duration, retentionDays int) *Aggregator {
	if aggregationInterval <= 0 {
		aggregationInterval = 1 * time.Minute
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}

	return &Aggregator{
		db:                  db,
		collector:           collector,
		aggregationInterval: aggregationInterval,
		retentionDays:       retentionDays,
		stopCh:              make(chan struct{}),
	}
}

// Start avvia l'aggregator
func (a *Aggregator) Start() {
	a.ticker = time.NewTicker(a.aggregationInterval)
	a.wg.Add(1)

	go a.aggregationLoop()
	log.Info().
		Dur("interval", a.aggregationInterval).
		Int("retention_days", a.retentionDays).
		Msg("Stats aggregator started")
}

// Stop ferma l'aggregator
func (a *Aggregator) Stop() {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopCh)
	a.wg.Wait()

	log.Info().Msg("Stats aggregator stopped")
}

// aggregationLoop esegue l'aggregazione periodica
func (a *Aggregator) aggregationLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ticker.C:
			a.aggregate()
			a.cleanup()
		case <-a.stopCh:
			return
		}
	}
}

// aggregate aggrega le statistiche correnti nel database
func (a *Aggregator) aggregate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metrics := a.collector.GetAllMetrics()
	if len(metrics) == 0 {
		return
	}

	timestamp := time.Now().Truncate(time.Minute)
	stats := make([]*models.ProviderStats, 0, len(metrics))

	for providerID, metric := range metrics {
		if metric.TotalRequests == 0 {
			continue
		}

		successRate := float64(metric.SuccessCount) / float64(metric.TotalRequests)
		avgLatency := int(metric.TotalLatencyMs / metric.TotalRequests)

		stat := &models.ProviderStats{
			ProviderID:     providerID,
			Timestamp:      timestamp,
			SuccessRate:    successRate,
			AvgLatencyMs:   avgLatency,
			TotalRequests:  metric.TotalRequests,
			TotalTokens:    metric.TotalTokens,
			CostSaved:      metric.TotalCost,
			ErrorCount:     metric.ErrorCount,
			TimeoutCount:   metric.TimeoutCount,
			QuotaExhausted: metric.QuotaExhausted,
		}

		stats = append(stats, stat)
	}

	if len(stats) > 0 {
		if err := a.db.WithContext(ctx).Create(&stats).Error; err != nil {
			log.Error().
				Err(err).
				Int("count", len(stats)).
				Msg("Failed to save aggregated stats")
		} else {
			log.Debug().
				Int("count", len(stats)).
				Msg("Saved aggregated stats")
		}
	}

	// Reset metriche in memoria dopo aggregazione
	a.collector.ResetMetrics()
}

// cleanup rimuove dati vecchi dal database
func (a *Aggregator) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().AddDate(0, 0, -a.retentionDays)

	// Cleanup provider stats
	result := a.db.WithContext(ctx).
		Where("timestamp < ?", cutoff).
		Delete(&models.ProviderStats{})

	if result.Error != nil {
		log.Error().
			Err(result.Error).
			Msg("Failed to cleanup old provider stats")
	} else if result.RowsAffected > 0 {
		log.Info().
			Int64("rows_deleted", result.RowsAffected).
			Msg("Cleaned up old provider stats")
	}

	// Cleanup request logs (keep less logs, e.g., 7 days)
	logCutoff := time.Now().AddDate(0, 0, -7)
	result = a.db.WithContext(ctx).
		Where("timestamp < ?", logCutoff).
		Delete(&models.RequestLog{})

	if result.Error != nil {
		log.Error().
			Err(result.Error).
			Msg("Failed to cleanup old request logs")
	} else if result.RowsAffected > 0 {
		log.Info().
			Int64("rows_deleted", result.RowsAffected).
			Msg("Cleaned up old request logs")
	}
}

// AggregateWindow aggrega statistiche per una specifica finestra temporale
func (a *Aggregator) AggregateWindow(ctx context.Context, providerID uuid.UUID, window TimeWindow, start, end time.Time) (*WindowStats, error) {
	var stats []models.ProviderStats

	query := a.db.WithContext(ctx).
		Where("provider_id = ? AND timestamp >= ? AND timestamp < ?", providerID, start, end).
		Order("timestamp ASC")

	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return &WindowStats{
			ProviderID: providerID,
			Window:     window,
			Start:      start,
			End:        end,
		}, nil
	}

	// Aggregate
	var (
		totalRequests  int64
		totalTokens    int64
		totalCost      float64
		totalErrors    int64
		totalTimeouts  int64
		totalQuota     int64
		totalLatencyMs int64
		successCount   int64
	)

	for _, stat := range stats {
		totalRequests += stat.TotalRequests
		totalTokens += stat.TotalTokens
		totalCost += stat.CostSaved
		totalErrors += stat.ErrorCount
		totalTimeouts += stat.TimeoutCount
		totalQuota += stat.QuotaExhausted
		totalLatencyMs += int64(stat.AvgLatencyMs) * stat.TotalRequests
		successCount += int64(float64(stat.TotalRequests) * stat.SuccessRate)
	}

	successRate := 0.0
	avgLatency := 0
	if totalRequests > 0 {
		successRate = float64(successCount) / float64(totalRequests)
		avgLatency = int(totalLatencyMs / totalRequests)
	}

	return &WindowStats{
		ProviderID:     providerID,
		Window:         window,
		Start:          start,
		End:            end,
		TotalRequests:  totalRequests,
		SuccessRate:    successRate,
		AvgLatencyMs:   avgLatency,
		TotalTokens:    totalTokens,
		CostSaved:      totalCost,
		ErrorCount:     totalErrors,
		TimeoutCount:   totalTimeouts,
		QuotaExhausted: totalQuota,
		DataPoints:     len(stats),
	}, nil
}

// GetHourlyStats restituisce statistiche aggregate per ora
func (a *Aggregator) GetHourlyStats(ctx context.Context, providerID uuid.UUID, hours int) ([]*WindowStats, error) {
	if hours <= 0 {
		hours = 24
	}

	results := make([]*WindowStats, 0, hours)
	now := time.Now()

	for i := 0; i < hours; i++ {
		end := now.Add(time.Duration(-i) * time.Hour).Truncate(time.Hour)
		start := end.Add(-time.Hour)

		stats, err := a.AggregateWindow(ctx, providerID, WindowHour, start, end)
		if err != nil {
			return nil, err
		}

		results = append(results, stats)
	}

	return results, nil
}

// GetDailyStats restituisce statistiche aggregate per giorno
func (a *Aggregator) GetDailyStats(ctx context.Context, providerID uuid.UUID, days int) ([]*WindowStats, error) {
	if days <= 0 {
		days = 7
	}

	results := make([]*WindowStats, 0, days)
	now := time.Now()

	for i := 0; i < days; i++ {
		end := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		start := end.AddDate(0, 0, -1)

		stats, err := a.AggregateWindow(ctx, providerID, WindowDay, start, end)
		if err != nil {
			return nil, err
		}

		results = append(results, stats)
	}

	return results, nil
}

// GetRollingWindow calcola metriche per una rolling window
func (a *Aggregator) GetRollingWindow(ctx context.Context, providerID uuid.UUID, duration time.Duration) (*WindowStats, error) {
	end := time.Now()
	start := end.Add(-duration)

	return a.AggregateWindow(ctx, providerID, WindowMinute, start, end)
}

// WindowStats rappresenta statistiche aggregate per una finestra temporale
type WindowStats struct {
	ProviderID     uuid.UUID
	Window         TimeWindow
	Start          time.Time
	End            time.Time
	TotalRequests  int64
	SuccessRate    float64
	AvgLatencyMs   int
	TotalTokens    int64
	CostSaved      float64
	ErrorCount     int64
	TimeoutCount   int64
	QuotaExhausted int64
	DataPoints     int
}

// GetAllProvidersStats restituisce statistiche aggregate per tutti i provider
func (a *Aggregator) GetAllProvidersStats(ctx context.Context, window TimeWindow, start, end time.Time) (map[uuid.UUID]*WindowStats, error) {
	var providers []models.Provider
	if err := a.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, err
	}

	results := make(map[uuid.UUID]*WindowStats)
	for _, provider := range providers {
		stats, err := a.AggregateWindow(ctx, provider.ID, window, start, end)
		if err != nil {
			log.Error().
				Err(err).
				Str("provider", provider.Name).
				Msg("Failed to aggregate provider stats")
			continue
		}
		results[provider.ID] = stats
	}

	return results, nil
}

// CompareProviders confronta le prestazioni di più provider
func (a *Aggregator) CompareProviders(ctx context.Context, providerIDs []uuid.UUID, duration time.Duration) (map[uuid.UUID]*WindowStats, error) {
	end := time.Now()
	start := end.Add(-duration)

	results := make(map[uuid.UUID]*WindowStats)
	for _, providerID := range providerIDs {
		stats, err := a.AggregateWindow(ctx, providerID, WindowMinute, start, end)
		if err != nil {
			return nil, err
		}
		results[providerID] = stats
	}

	return results, nil
}
