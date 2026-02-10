package websocket

import (
	"encoding/json"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Handler gestisce gli endpoint WebSocket
type Handler struct {
	hub    *Hub
	config *config.Config
	db     *database.DB
}

// NewHandler crea un nuovo handler WebSocket
func NewHandler(hub *Hub, cfg *config.Config, db *database.DB) *Handler {
	return &Handler{
		hub:    hub,
		config: cfg,
		db:     db,
	}
}

// HandleWebSocket gestisce la connessione WebSocket generica
func (h *Handler) HandleWebSocket(c fiber.Ctx) error {
	// Verifica header Upgrade
	if c.Get("Upgrade") != "websocket" {
		return fiber.ErrUpgradeRequired
	}

	// TODO: Implementare upgrade WebSocket con Fiber v3
	// Per ora ritorniamo un errore placeholder
	return c.SendString("WebSocket endpoint")
}

// HandleLogsWebSocket gestisce /ws/logs - Live log streaming
func (h *Handler) HandleLogsWebSocket(c fiber.Ctx) error {
	if c.Get("Upgrade") != "websocket" {
		return fiber.ErrUpgradeRequired
	}

	// Query params per filtraggio
	level := c.Query("level")      // debug, info, warn, error
	component := c.Query("component")
	providerID := c.Query("provider_id")

	log.Info().
		Str("level", level).
		Str("component", component).
		Str("provider_id", providerID).
		Msg("New logs WebSocket connection")

	// TODO: Implementare connessione WebSocket
	// Il client verrà automaticamente sottoscritto al canale "logs"

	return c.SendString("Logs WebSocket endpoint")
}

// HandleStatsWebSocket gestisce /ws/stats - Real-time statistics
func (h *Handler) HandleStatsWebSocket(c fiber.Ctx) error {
	if c.Get("Upgrade") != "websocket" {
		return fiber.ErrUpgradeRequired
	}

	// Query params per configurazione
	window := c.Query("window", "1m") // 1m, 5m, 1h, 24h
	interval := c.Query("interval", "5") // secondi tra aggiornamenti

	log.Info().
		Str("window", window).
		Str("interval", interval).
		Msg("New stats WebSocket connection")

	// TODO: Implementare connessione WebSocket
	// Il client verrà automaticamente sottoscritto al canale "stats"
	// E riceverà aggiornamenti ogni N secondi

	return c.SendString("Stats WebSocket endpoint")
}

// HandleProvidersWebSocket gestisce /ws/providers - Provider status updates
func (h *Handler) HandleProvidersWebSocket(c fiber.Ctx) error {
	if c.Get("Upgrade") != "websocket" {
		return fiber.ErrUpgradeRequired
	}

	log.Info().Msg("New providers WebSocket connection")

	// TODO: Implementare connessione WebSocket
	// Il client verrà automaticamente sottoscritto al canale "providers"

	return c.SendString("Providers WebSocket endpoint")
}

// HandleRequestsWebSocket gestisce /ws/requests - Request monitoring
func (h *Handler) HandleRequestsWebSocket(c fiber.Ctx) error {
	if c.Get("Upgrade") != "websocket" {
		return fiber.ErrUpgradeRequired
	}

	// Query params per filtraggio
	providerID := c.Query("provider_id")
	modelID := c.Query("model_id")
	onlyErrors := c.Query("only_errors") == "true"

	log.Info().
		Str("provider_id", providerID).
		Str("model_id", modelID).
		Bool("only_errors", onlyErrors).
		Msg("New requests WebSocket connection")

	// TODO: Implementare connessione WebSocket
	// Il client verrà automaticamente sottoscritto al canale "requests"

	return c.SendString("Requests WebSocket endpoint")
}

// BroadcastLogEvent invia un evento di log a tutti i client sottoscritti
func (h *Handler) BroadcastLogEvent(level, message, component string, providerID, requestID uuid.UUID, fields map[string]string) {
	logEvent := LogEvent{
		Level:      level,
		Message:    message,
		Component:  component,
		ProviderID: providerID,
		RequestID:  requestID,
		Fields:     fields,
		Timestamp:  time.Now(),
	}

	event, err := NewEvent(EventTypeLog, logEvent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create log event")
		return
	}

	h.hub.Broadcast(ChannelLogs, event)
}

// BroadcastProviderStatus invia un aggiornamento sullo stato di un provider
func (h *Handler) BroadcastProviderStatus(provider *models.Provider, status string, latencyMs int, successRate float64, message string) {
	statusEvent := ProviderStatusEvent{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		Status:       status,
		Available:    provider.IsAvailable(),
		LatencyMs:    latencyMs,
		SuccessRate:  successRate,
		Message:      message,
	}

	event, err := NewEvent(EventTypeProviderStatus, statusEvent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create provider status event")
		return
	}

	h.hub.Broadcast(ChannelProviders, event)
}

// BroadcastRequestEvent invia un evento di richiesta
func (h *Handler) BroadcastRequestEvent(requestLog *models.RequestLog, providerName, modelName string) {
	requestEvent := RequestEvent{
		RequestID:    requestLog.ID,
		ProviderID:   requestLog.ProviderID,
		ProviderName: providerName,
		ModelID:      requestLog.ModelID,
		ModelName:    modelName,
		Method:       requestLog.Method,
		Endpoint:     requestLog.Endpoint,
		StatusCode:   requestLog.StatusCode,
		LatencyMs:    requestLog.LatencyMs,
		InputTokens:  requestLog.InputTokens,
		OutputTokens: requestLog.OutputTokens,
		Success:      requestLog.Success,
		ErrorMessage: requestLog.ErrorMessage,
		Timestamp:    requestLog.Timestamp,
	}

	event, err := NewEvent(EventTypeRequest, requestEvent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create request event")
		return
	}

	h.hub.Broadcast(ChannelRequests, event)
}

// BroadcastStatsUpdate invia un aggiornamento delle statistiche
func (h *Handler) BroadcastStatsUpdate(window string) {
	// Calcola statistiche dal database
	stats, err := h.calculateStats(window)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate stats")
		return
	}

	event, err := NewEvent(EventTypeStatsUpdate, stats)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create stats event")
		return
	}

	h.hub.Broadcast(ChannelStats, event)
}

// calculateStats calcola statistiche aggregate
func (h *Handler) calculateStats(window string) (*StatsUpdateEvent, error) {
	// Determina il periodo di tempo
	var since time.Time
	switch window {
	case "1m":
		since = time.Now().Add(-1 * time.Minute)
	case "5m":
		since = time.Now().Add(-5 * time.Minute)
	case "1h":
		since = time.Now().Add(-1 * time.Hour)
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	default:
		since = time.Now().Add(-5 * time.Minute)
		window = "5m"
	}

	// Statistiche globali
	var totalRequests int64
	var totalTokens int64
	var successCount int64
	var totalLatency int64

	err := h.db.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ?", since).
		Count(&totalRequests).Error
	if err != nil {
		return nil, err
	}

	// Calcola totale token
	var tokenSum struct {
		Total int64
	}
	err = h.db.DB.Model(&models.RequestLog{}).
		Select("SUM(input_tokens + output_tokens) as total").
		Where("timestamp >= ?", since).
		Scan(&tokenSum).Error
	if err == nil {
		totalTokens = tokenSum.Total
	}

	// Calcola success rate
	err = h.db.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ? AND success = ?", since, true).
		Count(&successCount).Error
	if err == nil && totalRequests > 0 {
		// Success rate verrà calcolato dopo
	}

	// Calcola latency media
	var latencyAvg struct {
		Avg float64
	}
	err = h.db.DB.Model(&models.RequestLog{}).
		Select("AVG(latency_ms) as avg").
		Where("timestamp >= ?", since).
		Scan(&latencyAvg).Error
	if err == nil {
		totalLatency = int64(latencyAvg.Avg)
	}

	// Calcola requests per minute
	duration := time.Since(since).Minutes()
	requestsPerMin := 0
	if duration > 0 {
		requestsPerMin = int(float64(totalRequests) / duration)
	}

	// Statistiche per provider
	providerStats, err := h.calculateProviderStats(since)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate provider stats")
		providerStats = []ProviderStat{}
	}

	successRate := 0.0
	if totalRequests > 0 {
		successRate = float64(successCount) / float64(totalRequests)
	}

	return &StatsUpdateEvent{
		TotalRequests:  totalRequests,
		TotalTokens:    totalTokens,
		SuccessRate:    successRate,
		AvgLatencyMs:   int(totalLatency),
		RequestsPerMin: requestsPerMin,
		ProviderStats:  providerStats,
		CalculatedAt:   time.Now(),
		TimeWindow:     window,
	}, nil
}

// calculateProviderStats calcola statistiche per provider
func (h *Handler) calculateProviderStats(since time.Time) ([]ProviderStat, error) {
	// Query per statistiche aggregate per provider
	type ProviderAgg struct {
		ProviderID   uuid.UUID
		Requests     int64
		Tokens       int64
		SuccessCount int64
		AvgLatency   float64
		ErrorCount   int64
	}

	var aggs []ProviderAgg
	err := h.db.DB.Model(&models.RequestLog{}).
		Select(`provider_id,
			COUNT(*) as requests,
			SUM(input_tokens + output_tokens) as tokens,
			SUM(CASE WHEN success THEN 1 ELSE 0 END) as success_count,
			AVG(latency_ms) as avg_latency,
			SUM(CASE WHEN NOT success THEN 1 ELSE 0 END) as error_count`).
		Where("timestamp >= ?", since).
		Group("provider_id").
		Scan(&aggs).Error
	if err != nil {
		return nil, err
	}

	// Carica informazioni sui provider
	var providers []models.Provider
	err = h.db.DB.Find(&providers).Error
	if err != nil {
		return nil, err
	}

	providerMap := make(map[uuid.UUID]*models.Provider)
	for i := range providers {
		providerMap[providers[i].ID] = &providers[i]
	}

	// Costruisci risultati
	stats := make([]ProviderStat, 0, len(aggs))
	for _, agg := range aggs {
		provider, ok := providerMap[agg.ProviderID]
		if !ok {
			continue
		}

		successRate := 0.0
		if agg.Requests > 0 {
			successRate = float64(agg.SuccessCount) / float64(agg.Requests)
		}

		stats = append(stats, ProviderStat{
			ProviderID:   agg.ProviderID,
			ProviderName: provider.Name,
			Requests:     agg.Requests,
			Tokens:       agg.Tokens,
			SuccessRate:  successRate,
			AvgLatencyMs: int(agg.AvgLatency),
			ErrorCount:   agg.ErrorCount,
			Available:    provider.IsAvailable(),
		})
	}

	return stats, nil
}

// GetHubStats restituisce statistiche sull'hub WebSocket
func (h *Handler) GetHubStats(c fiber.Ctx) error {
	stats := h.hub.GetStats()
	return c.JSON(stats)
}

// HandleTestBroadcast endpoint di test per broadcast
func (h *Handler) HandleTestBroadcast(c fiber.Ctx) error {
	var req struct {
		Channel string          `json:"channel"`
		Type    string          `json:"type"`
		Data    json.RawMessage `json:"data"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if !IsValidChannel(req.Channel) {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid channel",
		})
	}

	event := &Event{
		Type:      EventType(req.Type),
		Timestamp: time.Now(),
		Data:      req.Data,
	}

	h.hub.Broadcast(Channel(req.Channel), event)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Event broadcast",
	})
}
