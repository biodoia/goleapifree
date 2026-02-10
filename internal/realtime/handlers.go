package realtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Handlers gestisce gli endpoint SSE
type Handlers struct {
	hub        *SSEHub
	streamer   *Streamer
	aggregator *Aggregator
}

// NewHandlers crea un nuovo gestore di handlers
func NewHandlers(hub *SSEHub, streamer *Streamer, aggregator *Aggregator) *Handlers {
	return &Handlers{
		hub:        hub,
		streamer:   streamer,
		aggregator: aggregator,
	}
}

// RegisterRoutes registra le route SSE
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	stream := router.Group("/stream")
	{
		stream.GET("/stats", h.StreamStats)
		stream.GET("/logs", h.StreamLogs)
		stream.GET("/providers", h.StreamProviders)
		stream.GET("/requests", h.StreamRequests)
		stream.GET("/all", h.StreamAll)
	}

	// API endpoints for metrics
	api := router.Group("/api/realtime")
	{
		api.GET("/metrics", h.GetMetrics)
		api.GET("/providers/:id/metrics", h.GetProviderMetrics)
		api.GET("/hub/stats", h.GetHubStats)
	}

	log.Info().Msg("SSE routes registered")
}

// StreamStats gestisce lo streaming delle statistiche
func (h *Handlers) StreamStats(c *gin.Context) {
	h.streamSSE(c, []EventType{EventTypeStats})
}

// StreamLogs gestisce lo streaming dei log
func (h *Handlers) StreamLogs(c *gin.Context) {
	h.streamSSE(c, []EventType{EventTypeLogs})
}

// StreamProviders gestisce lo streaming dello stato dei provider
func (h *Handlers) StreamProviders(c *gin.Context) {
	h.streamSSE(c, []EventType{EventTypeProviders})
}

// StreamRequests gestisce lo streaming delle richieste in tempo reale
func (h *Handlers) StreamRequests(c *gin.Context) {
	h.streamSSE(c, []EventType{EventTypeRequests})
}

// StreamAll gestisce lo streaming di tutti gli eventi
func (h *Handlers) StreamAll(c *gin.Context) {
	h.streamSSE(c, []EventType{EventTypeStats, EventTypeLogs, EventTypeProviders, EventTypeRequests})
}

// streamSSE è il gestore generico per SSE
func (h *Handlers) streamSSE(c *gin.Context, channels []EventType) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Access-Control-Allow-Origin", "*")

	// Generate client ID
	clientID := uuid.New().String()

	// Get last event ID if reconnecting
	lastEventID := c.GetHeader("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = c.Query("lastEventId")
	}

	log.Info().
		Str("client_id", clientID).
		Str("last_event_id", lastEventID).
		Interface("channels", channels).
		Msg("SSE client connecting")

	// Register client
	client := h.hub.RegisterClient(c.Request.Context(), clientID, channels)
	defer h.hub.UnregisterClient(client)

	// Send initial connection event
	initialEvent := &SSEEvent{
		ID:    h.hub.generateEventID(),
		Type:  EventTypeHeartbeat,
		Data: map[string]interface{}{
			"message":    "connected",
			"client_id":  clientID,
			"channels":   channels,
			"timestamp":  time.Now().Unix(),
			"retry":      15000, // Suggest 15s retry on disconnect
		},
		Retry: 15000,
	}

	if err := h.writeSSEEvent(c.Writer, initialEvent); err != nil {
		log.Error().Err(err).Msg("Failed to send initial event")
		return
	}

	c.Writer.Flush()

	// Setup context with timeout
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Stream events
	for {
		select {
		case event, ok := <-client.Channel:
			if !ok {
				log.Debug().Str("client_id", clientID).Msg("Client channel closed")
				return
			}

			if err := h.writeSSEEvent(c.Writer, event); err != nil {
				log.Error().
					Err(err).
					Str("client_id", clientID).
					Msg("Failed to write SSE event")
				return
			}

			c.Writer.Flush()

		case <-ctx.Done():
			log.Info().
				Str("client_id", clientID).
				Msg("SSE client disconnected")
			return
		}
	}
}

// writeSSEEvent scrive un evento SSE nel writer
func (h *Handlers) writeSSEEvent(w http.ResponseWriter, event *SSEEvent) error {
	formatted, err := event.FormatSSE()
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, formatted)
	return err
}

// GetMetrics restituisce le metriche aggregate correnti
func (h *Handlers) GetMetrics(c *gin.Context) {
	metrics := h.aggregator.GetMetrics()

	// Convert to JSON-friendly format
	response := map[string]interface{}{
		"total_requests":       metrics.TotalRequests,
		"successful_requests":  metrics.SuccessfulRequests,
		"failed_requests":      metrics.FailedRequests,
		"avg_latency_ms":       metrics.AvgLatencyMs,
		"avg_tokens_per_req":   metrics.AvgTokensPerReq,
		"avg_cost_per_req":     metrics.AvgCostPerReq,
		"total_cost":           metrics.TotalCost,
		"cost_per_minute":      metrics.CostPerMinute,
		"estimated_hourly_cost": metrics.EstimatedHourlyCost,
		"active_users":         metrics.ActiveUsers,
		"unique_users_today":   metrics.UniqueUsersToday,
		"last_updated":         metrics.LastUpdated.Unix(),
		"window_start":         metrics.WindowStart.Unix(),
	}

	// Add provider metrics
	providerMetrics := make([]map[string]interface{}, 0, len(metrics.ProviderMetrics))
	for _, pm := range metrics.ProviderMetrics {
		providerMetrics = append(providerMetrics, map[string]interface{}{
			"provider_id":    pm.ProviderID.String(),
			"request_count":  pm.RequestCount,
			"success_rate":   pm.SuccessRate,
			"avg_latency_ms": pm.AvgLatencyMs,
			"error_rate":     pm.ErrorRate,
			"total_tokens":   pm.TotalTokens,
			"total_cost":     pm.TotalCost,
			"last_request":   pm.LastRequest.Unix(),
			"is_healthy":     pm.IsHealthy,
		})
	}
	response["providers"] = providerMetrics

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetProviderMetrics restituisce le metriche per un provider specifico
func (h *Handlers) GetProviderMetrics(c *gin.Context) {
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid provider ID",
		})
		return
	}

	metrics := h.aggregator.GetProviderMetrics(providerID)
	if metrics == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "provider metrics not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"provider_id":    metrics.ProviderID.String(),
			"request_count":  metrics.RequestCount,
			"success_rate":   metrics.SuccessRate,
			"avg_latency_ms": metrics.AvgLatencyMs,
			"error_rate":     metrics.ErrorRate,
			"total_tokens":   metrics.TotalTokens,
			"total_cost":     metrics.TotalCost,
			"last_request":   metrics.LastRequest.Unix(),
			"is_healthy":     metrics.IsHealthy,
		},
	})
}

// GetHubStats restituisce le statistiche dell'hub SSE
func (h *Handlers) GetHubStats(c *gin.Context) {
	stats := h.hub.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ParseChannels converte una stringa di canali in slice di EventType
func ParseChannels(channelsStr string) []EventType {
	if channelsStr == "" {
		return []EventType{EventTypeStats, EventTypeLogs, EventTypeProviders, EventTypeRequests}
	}

	parts := strings.Split(channelsStr, ",")
	channels := make([]EventType, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "stats":
			channels = append(channels, EventTypeStats)
		case "logs":
			channels = append(channels, EventTypeLogs)
		case "providers":
			channels = append(channels, EventTypeProviders)
		case "requests":
			channels = append(channels, EventTypeRequests)
		}
	}

	if len(channels) == 0 {
		return []EventType{EventTypeStats}
	}

	return channels
}
