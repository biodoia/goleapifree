package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TUIClient client WebSocket specializzato per il TUI
type TUIClient struct {
	client      *WebSocketClient
	mu          sync.RWMutex
	latestStats *StatsUpdateEvent
	providers   map[string]*ProviderStatusEvent
	requests    []*RequestEvent
	logs        []*LogEvent
	maxLogs     int
	maxRequests int
	callbacks   TUICallbacks
}

// TUICallbacks callback per aggiornamenti TUI
type TUICallbacks struct {
	OnStatsUpdate    func(*StatsUpdateEvent)
	OnProviderUpdate func(*ProviderStatusEvent)
	OnRequest        func(*RequestEvent)
	OnLog            func(*LogEvent)
	OnError          func(error)
}

// NewTUIClient crea un nuovo client TUI
func NewTUIClient(url string, callbacks TUICallbacks) *TUIClient {
	client := NewWebSocketClient(url)

	tui := &TUIClient{
		client:      client,
		providers:   make(map[string]*ProviderStatusEvent),
		requests:    make([]*RequestEvent, 0),
		logs:        make([]*LogEvent, 0),
		maxLogs:     1000,
		maxRequests: 500,
		callbacks:   callbacks,
	}

	// Registra gli handler
	client.OnEvent(EventTypeStatsUpdate, tui.handleStatsUpdate)
	client.OnEvent(EventTypeProviderStatus, tui.handleProviderStatus)
	client.OnEvent(EventTypeRequest, tui.handleRequest)
	client.OnEvent(EventTypeLog, tui.handleLog)
	client.OnEvent(EventTypeError, tui.handleError)

	return tui
}

// Connect connette al server WebSocket
func (t *TUIClient) Connect() error {
	return t.client.Connect()
}

// Start avvia il client e sottoscrive ai canali
func (t *TUIClient) Start(ctx context.Context, channels ...Channel) error {
	if err := t.client.Start(); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	// Sottoscrivi ai canali richiesti
	if len(channels) == 0 {
		channels = []Channel{ChannelAll}
	}

	if err := t.client.Subscribe(channels...); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Info().
		Int("channels", len(channels)).
		Msg("TUI WebSocket client started")

	return nil
}

// handleStatsUpdate gestisce aggiornamenti statistiche
func (t *TUIClient) handleStatsUpdate(event *Event) error {
	var stats StatsUpdateEvent
	if err := json.Unmarshal(event.Data, &stats); err != nil {
		return err
	}

	t.mu.Lock()
	t.latestStats = &stats
	t.mu.Unlock()

	if t.callbacks.OnStatsUpdate != nil {
		t.callbacks.OnStatsUpdate(&stats)
	}

	return nil
}

// handleProviderStatus gestisce aggiornamenti stato provider
func (t *TUIClient) handleProviderStatus(event *Event) error {
	var status ProviderStatusEvent
	if err := json.Unmarshal(event.Data, &status); err != nil {
		return err
	}

	t.mu.Lock()
	t.providers[status.ProviderID.String()] = &status
	t.mu.Unlock()

	if t.callbacks.OnProviderUpdate != nil {
		t.callbacks.OnProviderUpdate(&status)
	}

	return nil
}

// handleRequest gestisce eventi richiesta
func (t *TUIClient) handleRequest(event *Event) error {
	var req RequestEvent
	if err := json.Unmarshal(event.Data, &req); err != nil {
		return err
	}

	t.mu.Lock()
	t.requests = append(t.requests, &req)
	// Mantieni solo le ultime N richieste
	if len(t.requests) > t.maxRequests {
		t.requests = t.requests[len(t.requests)-t.maxRequests:]
	}
	t.mu.Unlock()

	if t.callbacks.OnRequest != nil {
		t.callbacks.OnRequest(&req)
	}

	return nil
}

// handleLog gestisce eventi log
func (t *TUIClient) handleLog(event *Event) error {
	var logEvent LogEvent
	if err := json.Unmarshal(event.Data, &logEvent); err != nil {
		return err
	}

	t.mu.Lock()
	t.logs = append(t.logs, &logEvent)
	// Mantieni solo gli ultimi N log
	if len(t.logs) > t.maxLogs {
		t.logs = t.logs[len(t.logs)-t.maxLogs:]
	}
	t.mu.Unlock()

	if t.callbacks.OnLog != nil {
		t.callbacks.OnLog(&logEvent)
	}

	return nil
}

// handleError gestisce eventi errore
func (t *TUIClient) handleError(event *Event) error {
	var errEvent ErrorEvent
	if err := json.Unmarshal(event.Data, &errEvent); err != nil {
		return err
	}

	if t.callbacks.OnError != nil {
		t.callbacks.OnError(fmt.Errorf("%s: %s", errEvent.Code, errEvent.Message))
	}

	return nil
}

// GetLatestStats restituisce le ultime statistiche
func (t *TUIClient) GetLatestStats() *StatsUpdateEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.latestStats
}

// GetProviders restituisce tutti i provider
func (t *TUIClient) GetProviders() map[string]*ProviderStatusEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	providers := make(map[string]*ProviderStatusEvent)
	for k, v := range t.providers {
		providers[k] = v
	}
	return providers
}

// GetRecentRequests restituisce le richieste recenti
func (t *TUIClient) GetRecentRequests(limit int) []*RequestEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.requests) {
		limit = len(t.requests)
	}

	requests := make([]*RequestEvent, limit)
	copy(requests, t.requests[len(t.requests)-limit:])
	return requests
}

// GetRecentLogs restituisce i log recenti
func (t *TUIClient) GetRecentLogs(limit int, level string) []*LogEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	filtered := make([]*LogEvent, 0)
	for _, l := range t.logs {
		if level == "" || l.Level == level {
			filtered = append(filtered, l)
		}
	}

	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}

	logs := make([]*LogEvent, limit)
	copy(logs, filtered[len(filtered)-limit:])
	return logs
}

// Close chiude la connessione
func (t *TUIClient) Close() error {
	return t.client.Close()
}

// MockTUIClient client mock per testing
type MockTUIClient struct {
	stats     *StatsUpdateEvent
	providers map[string]*ProviderStatusEvent
}

// NewMockTUIClient crea un client mock
func NewMockTUIClient() *MockTUIClient {
	return &MockTUIClient{
		stats: &StatsUpdateEvent{
			TotalRequests:  1000,
			TotalTokens:    50000,
			SuccessRate:    0.95,
			AvgLatencyMs:   150,
			RequestsPerMin: 20,
			ProviderStats: []ProviderStat{
				{
					ProviderName: "OpenAI Free",
					Requests:     500,
					SuccessRate:  0.98,
					AvgLatencyMs: 120,
					Available:    true,
				},
				{
					ProviderName: "Claude Free",
					Requests:     300,
					SuccessRate:  0.92,
					AvgLatencyMs: 180,
					Available:    true,
				},
				{
					ProviderName: "Gemini Free",
					Requests:     200,
					SuccessRate:  0.90,
					AvgLatencyMs: 200,
					Available:    false,
				},
			},
			CalculatedAt: time.Now(),
			TimeWindow:   "5m",
		},
		providers: make(map[string]*ProviderStatusEvent),
	}
}

// GetLatestStats mock
func (m *MockTUIClient) GetLatestStats() *StatsUpdateEvent {
	return m.stats
}

// GetProviders mock
func (m *MockTUIClient) GetProviders() map[string]*ProviderStatusEvent {
	return m.providers
}

// SimulateUpdate simula un aggiornamento
func (m *MockTUIClient) SimulateUpdate() {
	m.stats.TotalRequests++
	m.stats.TotalTokens += 100
	m.stats.CalculatedAt = time.Now()
}
