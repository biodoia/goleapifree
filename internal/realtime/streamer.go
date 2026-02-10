package realtime

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/stats"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Streamer gestisce lo streaming di dati in real-time
type Streamer struct {
	hub       *SSEHub
	collector *stats.Collector
	db        *database.DB

	// Configuration
	statsInterval     time.Duration
	logsInterval      time.Duration
	providersInterval time.Duration

	// State
	running bool
	mu      sync.RWMutex

	// Channels
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewStreamer crea un nuovo streamer
func NewStreamer(hub *SSEHub, collector *stats.Collector, db *database.DB) *Streamer {
	return &Streamer{
		hub:               hub,
		collector:         collector,
		db:                db,
		statsInterval:     2 * time.Second,
		logsInterval:      1 * time.Second,
		providersInterval: 5 * time.Second,
		stopCh:            make(chan struct{}),
	}
}

// SetIntervals configura gli intervalli di streaming
func (s *Streamer) SetIntervals(stats, logs, providers time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stats > 0 {
		s.statsInterval = stats
	}
	if logs > 0 {
		s.logsInterval = logs
	}
	if providers > 0 {
		s.providersInterval = providers
	}
}

// Start avvia lo streaming
func (s *Streamer) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	// Start streaming goroutines
	s.wg.Add(3)
	go s.streamStats()
	go s.streamProviders()
	go s.streamLogs()

	log.Info().Msg("Real-time streamer started")
}

// Stop ferma lo streaming
func (s *Streamer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	log.Info().Msg("Real-time streamer stopped")
}

// streamStats invia statistiche live
func (s *Streamer) streamStats() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.statsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.publishStats()

		case <-s.stopCh:
			return
		}
	}
}

// streamProviders invia aggiornamenti sui provider
func (s *Streamer) streamProviders() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.providersInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.publishProviders()

		case <-s.stopCh:
			return
		}
	}
}

// streamLogs invia i log delle richieste in tempo reale
func (s *Streamer) streamLogs() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.logsInterval)
	defer ticker.Stop()

	var lastLogID uuid.UUID

	for {
		select {
		case <-ticker.C:
			logs := s.getRecentLogs(lastLogID, 10)
			if len(logs) > 0 {
				s.publishLogs(logs)
				lastLogID = logs[len(logs)-1].ID
			}

		case <-s.stopCh:
			return
		}
	}
}

// publishStats pubblica statistiche aggregate
func (s *Streamer) publishStats() {
	allMetrics := s.collector.GetAllMetrics()

	statsData := make(map[string]interface{})
	var totalRequests int64
	var totalTokens int64
	var totalCost float64

	providerStats := make([]map[string]interface{}, 0, len(allMetrics))

	for providerID, metrics := range allMetrics {
		successRate := 0.0
		avgLatency := 0
		if metrics.TotalRequests > 0 {
			successRate = float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
			avgLatency = int(metrics.TotalLatencyMs / metrics.TotalRequests)
		}

		providerStats = append(providerStats, map[string]interface{}{
			"provider_id":     providerID.String(),
			"total_requests":  metrics.TotalRequests,
			"success_rate":    successRate,
			"avg_latency_ms":  avgLatency,
			"total_tokens":    metrics.TotalTokens,
			"total_cost":      metrics.TotalCost,
			"error_count":     metrics.ErrorCount,
			"timeout_count":   metrics.TimeoutCount,
			"quota_exhausted": metrics.QuotaExhausted,
			"last_updated":    metrics.LastUpdated.Unix(),
		})

		totalRequests += metrics.TotalRequests
		totalTokens += metrics.TotalTokens
		totalCost += metrics.TotalCost
	}

	statsData["timestamp"] = time.Now().Unix()
	statsData["total_requests"] = totalRequests
	statsData["total_tokens"] = totalTokens
	statsData["total_cost"] = totalCost
	statsData["providers"] = providerStats

	s.hub.Broadcast(EventTypeStats, statsData)
}

// publishProviders pubblica lo stato dei provider
func (s *Streamer) publishProviders() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var providers []models.Provider
	if err := s.db.WithContext(ctx).
		Preload("Models").
		Where("status = ?", models.ProviderStatusActive).
		Find(&providers).Error; err != nil {
		log.Error().Err(err).Msg("Failed to fetch providers")
		return
	}

	providersData := make([]map[string]interface{}, 0, len(providers))

	for _, provider := range providers {
		// Get real-time metrics
		metrics := s.collector.GetProviderMetrics(provider.ID)

		providerData := map[string]interface{}{
			"id":                 provider.ID.String(),
			"name":               provider.Name,
			"type":               provider.Type,
			"status":             provider.Status,
			"health_score":       provider.HealthScore,
			"avg_latency_ms":     provider.AvgLatencyMs,
			"supports_streaming": provider.SupportsStreaming,
			"supports_tools":     provider.SupportsTools,
			"model_count":        len(provider.Models),
			"last_health_check":  provider.LastHealthCheck.Unix(),
		}

		if metrics != nil {
			successRate := 0.0
			if metrics.TotalRequests > 0 {
				successRate = float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
			}

			providerData["realtime_requests"] = metrics.TotalRequests
			providerData["realtime_success_rate"] = successRate
			providerData["realtime_errors"] = metrics.ErrorCount
		}

		providersData = append(providersData, providerData)
	}

	data := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"providers": providersData,
		"total":     len(providersData),
	}

	s.hub.Broadcast(EventTypeProviders, data)
}

// publishLogs pubblica i log delle richieste
func (s *Streamer) publishLogs(logs []models.RequestLog) {
	logsData := make([]map[string]interface{}, 0, len(logs))

	for _, reqLog := range logs {
		logData := map[string]interface{}{
			"id":             reqLog.ID.String(),
			"provider_id":    reqLog.ProviderID.String(),
			"model_id":       reqLog.ModelID.String(),
			"user_id":        reqLog.UserID.String(),
			"method":         reqLog.Method,
			"endpoint":       reqLog.Endpoint,
			"status_code":    reqLog.StatusCode,
			"latency_ms":     reqLog.LatencyMs,
			"input_tokens":   reqLog.InputTokens,
			"output_tokens":  reqLog.OutputTokens,
			"success":        reqLog.Success,
			"error_message":  reqLog.ErrorMessage,
			"estimated_cost": reqLog.EstimatedCost,
			"timestamp":      reqLog.Timestamp.Unix(),
		}

		logsData = append(logsData, logData)
	}

	data := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"logs":      logsData,
		"count":     len(logsData),
	}

	s.hub.Broadcast(EventTypeLogs, data)
}

// getRecentLogs recupera i log più recenti
func (s *Streamer) getRecentLogs(afterID uuid.UUID, limit int) []models.RequestLog {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var logs []models.RequestLog
	query := s.db.WithContext(ctx).
		Order("timestamp DESC").
		Limit(limit)

	if afterID != uuid.Nil {
		query = query.Where("id > ?", afterID)
	}

	if err := query.Find(&logs).Error; err != nil {
		log.Error().Err(err).Msg("Failed to fetch recent logs")
		return nil
	}

	return logs
}

// PublishRequest pubblica una singola richiesta in real-time
func (s *Streamer) PublishRequest(reqLog *models.RequestLog) {
	requestData := map[string]interface{}{
		"id":             reqLog.ID.String(),
		"provider_id":    reqLog.ProviderID.String(),
		"model_id":       reqLog.ModelID.String(),
		"user_id":        reqLog.UserID.String(),
		"method":         reqLog.Method,
		"endpoint":       reqLog.Endpoint,
		"status_code":    reqLog.StatusCode,
		"latency_ms":     reqLog.LatencyMs,
		"input_tokens":   reqLog.InputTokens,
		"output_tokens":  reqLog.OutputTokens,
		"success":        reqLog.Success,
		"error_message":  reqLog.ErrorMessage,
		"estimated_cost": reqLog.EstimatedCost,
		"timestamp":      reqLog.Timestamp.Unix(),
	}

	s.hub.Broadcast(EventTypeRequests, requestData)
}

// PublishError pubblica un errore in real-time
func (s *Streamer) PublishError(err error, context map[string]interface{}) {
	errorData := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"error":     err.Error(),
		"context":   context,
	}

	s.hub.Broadcast(EventTypeError, errorData)
}
