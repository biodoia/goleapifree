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

// RequestMetrics rappresenta le metriche di una singola richiesta
type RequestMetrics struct {
	ProviderID    uuid.UUID
	ModelID       uuid.UUID
	UserID        uuid.UUID
	Method        string
	Endpoint      string
	StatusCode    int
	LatencyMs     int
	InputTokens   int
	OutputTokens  int
	Success       bool
	ErrorMessage  string
	EstimatedCost float64
	Timestamp     time.Time
}

// AggregatedMetrics rappresenta metriche aggregate in memoria
type AggregatedMetrics struct {
	ProviderID     uuid.UUID
	TotalRequests  int64
	SuccessCount   int64
	ErrorCount     int64
	TimeoutCount   int64
	QuotaExhausted int64
	TotalLatencyMs int64
	TotalTokens    int64
	TotalCost      float64
	LastUpdated    time.Time
}

// Collector raccoglie e aggrega metriche dalle richieste
type Collector struct {
	db *database.DB

	// In-memory aggregation per provider
	metrics map[uuid.UUID]*AggregatedMetrics
	mu      sync.RWMutex

	// Buffer per request logs
	logBuffer    []*models.RequestLog
	logBufferMu  sync.Mutex
	bufferSize   int
	flushTicker  *time.Ticker
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewCollector crea un nuovo collector
func NewCollector(db *database.DB, bufferSize int) *Collector {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	c := &Collector{
		db:         db,
		metrics:    make(map[uuid.UUID]*AggregatedMetrics),
		logBuffer:  make([]*models.RequestLog, 0, bufferSize),
		bufferSize: bufferSize,
		stopCh:     make(chan struct{}),
	}

	return c
}

// Start avvia il collector
func (c *Collector) Start(flushInterval time.Duration) {
	c.flushTicker = time.NewTicker(flushInterval)
	c.wg.Add(1)

	go c.flushLoop()
	log.Info().
		Dur("flush_interval", flushInterval).
		Msg("Stats collector started")
}

// Stop ferma il collector
func (c *Collector) Stop() {
	if c.flushTicker != nil {
		c.flushTicker.Stop()
	}
	close(c.stopCh)
	c.wg.Wait()

	// Final flush
	c.flush()
	log.Info().Msg("Stats collector stopped")
}

// Record registra le metriche di una richiesta
func (c *Collector) Record(metrics *RequestMetrics) {
	// Crea request log
	reqLog := &models.RequestLog{
		ProviderID:    metrics.ProviderID,
		ModelID:       metrics.ModelID,
		UserID:        metrics.UserID,
		Method:        metrics.Method,
		Endpoint:      metrics.Endpoint,
		StatusCode:    metrics.StatusCode,
		LatencyMs:     metrics.LatencyMs,
		InputTokens:   metrics.InputTokens,
		OutputTokens:  metrics.OutputTokens,
		Success:       metrics.Success,
		ErrorMessage:  metrics.ErrorMessage,
		EstimatedCost: metrics.EstimatedCost,
		Timestamp:     metrics.Timestamp,
	}

	if reqLog.Timestamp.IsZero() {
		reqLog.Timestamp = time.Now()
	}

	// Aggiungi al buffer
	c.logBufferMu.Lock()
	c.logBuffer = append(c.logBuffer, reqLog)
	shouldFlush := len(c.logBuffer) >= c.bufferSize
	c.logBufferMu.Unlock()

	// Aggiorna metriche in memoria
	c.updateAggregatedMetrics(metrics)

	// Flush se buffer pieno
	if shouldFlush {
		go c.flush()
	}
}

// updateAggregatedMetrics aggiorna le metriche aggregate in memoria
func (c *Collector) updateAggregatedMetrics(metrics *RequestMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	agg, exists := c.metrics[metrics.ProviderID]
	if !exists {
		agg = &AggregatedMetrics{
			ProviderID: metrics.ProviderID,
		}
		c.metrics[metrics.ProviderID] = agg
	}

	agg.TotalRequests++
	if metrics.Success {
		agg.SuccessCount++
	} else {
		agg.ErrorCount++
		if metrics.StatusCode == 429 {
			agg.QuotaExhausted++
		}
		if metrics.StatusCode == 504 || metrics.StatusCode == 408 {
			agg.TimeoutCount++
		}
	}

	agg.TotalLatencyMs += int64(metrics.LatencyMs)
	agg.TotalTokens += int64(metrics.InputTokens + metrics.OutputTokens)
	agg.TotalCost += metrics.EstimatedCost
	agg.LastUpdated = time.Now()
}

// GetProviderMetrics restituisce le metriche aggregate per un provider
func (c *Collector) GetProviderMetrics(providerID uuid.UUID) *AggregatedMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if metrics, exists := c.metrics[providerID]; exists {
		// Return copy
		copy := *metrics
		return &copy
	}

	return nil
}

// GetAllMetrics restituisce tutte le metriche aggregate
func (c *Collector) GetAllMetrics() map[uuid.UUID]*AggregatedMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uuid.UUID]*AggregatedMetrics, len(c.metrics))
	for providerID, metrics := range c.metrics {
		copy := *metrics
		result[providerID] = &copy
	}

	return result
}

// CalculateSuccessRate calcola il success rate per un provider
func (c *Collector) CalculateSuccessRate(providerID uuid.UUID) float64 {
	metrics := c.GetProviderMetrics(providerID)
	if metrics == nil || metrics.TotalRequests == 0 {
		return 0.0
	}

	return float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
}

// CalculateAvgLatency calcola la latenza media per un provider
func (c *Collector) CalculateAvgLatency(providerID uuid.UUID) int {
	metrics := c.GetProviderMetrics(providerID)
	if metrics == nil || metrics.TotalRequests == 0 {
		return 0
	}

	return int(metrics.TotalLatencyMs / metrics.TotalRequests)
}

// flushLoop esegue il flush periodico
func (c *Collector) flushLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.flushTicker.C:
			c.flush()
		case <-c.stopCh:
			return
		}
	}
}

// flush scrive i dati bufferizzati nel database
func (c *Collector) flush() {
	// Flush request logs
	c.logBufferMu.Lock()
	logsToFlush := c.logBuffer
	c.logBuffer = make([]*models.RequestLog, 0, c.bufferSize)
	c.logBufferMu.Unlock()

	if len(logsToFlush) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := c.db.WithContext(ctx).CreateInBatches(logsToFlush, 100).Error; err != nil {
			log.Error().
				Err(err).
				Int("count", len(logsToFlush)).
				Msg("Failed to flush request logs")
		} else {
			log.Debug().
				Int("count", len(logsToFlush)).
				Msg("Flushed request logs to database")
		}
	}
}

// ResetMetrics resetta le metriche aggregate in memoria
func (c *Collector) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = make(map[uuid.UUID]*AggregatedMetrics)
	log.Info().Msg("Reset aggregated metrics")
}

// GetStats restituisce statistiche complete per un provider
func (c *Collector) GetStats(providerID uuid.UUID) *ProviderStatsSnapshot {
	metrics := c.GetProviderMetrics(providerID)
	if metrics == nil {
		return nil
	}

	successRate := 0.0
	avgLatency := 0
	if metrics.TotalRequests > 0 {
		successRate = float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
		avgLatency = int(metrics.TotalLatencyMs / metrics.TotalRequests)
	}

	return &ProviderStatsSnapshot{
		ProviderID:     providerID,
		TotalRequests:  metrics.TotalRequests,
		SuccessRate:    successRate,
		AvgLatencyMs:   avgLatency,
		TotalTokens:    metrics.TotalTokens,
		TotalCost:      metrics.TotalCost,
		ErrorCount:     metrics.ErrorCount,
		TimeoutCount:   metrics.TimeoutCount,
		QuotaExhausted: metrics.QuotaExhausted,
		Timestamp:      metrics.LastUpdated,
	}
}

// ProviderStatsSnapshot rappresenta uno snapshot delle statistiche di un provider
type ProviderStatsSnapshot struct {
	ProviderID     uuid.UUID
	TotalRequests  int64
	SuccessRate    float64
	AvgLatencyMs   int
	TotalTokens    int64
	TotalCost      float64
	ErrorCount     int64
	TimeoutCount   int64
	QuotaExhausted int64
	Timestamp      time.Time
}
