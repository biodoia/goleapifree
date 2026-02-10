package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EventType rappresenta il tipo di evento SSE
type EventType string

const (
	EventTypeStats     EventType = "stats"
	EventTypeLogs      EventType = "logs"
	EventTypeProviders EventType = "providers"
	EventTypeRequests  EventType = "requests"
	EventTypeHeartbeat EventType = "heartbeat"
	EventTypeError     EventType = "error"
)

// SSEEvent rappresenta un evento Server-Sent Events
type SSEEvent struct {
	ID    string      `json:"id"`
	Type  EventType   `json:"type"`
	Data  interface{} `json:"data"`
	Retry int         `json:"retry,omitempty"` // milliseconds
}

// SSEClient rappresenta un client SSE connesso
type SSEClient struct {
	ID            string
	Channel       chan *SSEEvent
	Channels      []EventType // Canali sottoscritti
	LastEventID   string
	Connected     time.Time
	LastHeartbeat time.Time
	Context       context.Context
	Cancel        context.CancelFunc
}

// SSEHub gestisce tutti i client SSE e il broadcasting
type SSEHub struct {
	// Registered clients
	clients map[string]*SSEClient
	mu      sync.RWMutex

	// Client subscriptions per channel
	subscriptions map[EventType]map[string]*SSEClient
	subMu         sync.RWMutex

	// Channels
	register   chan *SSEClient
	unregister chan *SSEClient
	broadcast  chan *BroadcastMessage

	// Event ID generator
	eventIDCounter uint64
	eventIDMu      sync.Mutex

	// Configuration
	heartbeatInterval time.Duration
	clientTimeout     time.Duration
	bufferSize        int

	// Shutdown
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// BroadcastMessage rappresenta un messaggio da broadcastare
type BroadcastMessage struct {
	Event    *SSEEvent
	Channels []EventType // Se vuoto, broadcast a tutti
}

// NewSSEHub crea un nuovo hub SSE
func NewSSEHub(heartbeatInterval, clientTimeout time.Duration, bufferSize int) *SSEHub {
	if heartbeatInterval == 0 {
		heartbeatInterval = 15 * time.Second
	}
	if clientTimeout == 0 {
		clientTimeout = 5 * time.Minute
	}
	if bufferSize == 0 {
		bufferSize = 100
	}

	return &SSEHub{
		clients:           make(map[string]*SSEClient),
		subscriptions:     make(map[EventType]map[string]*SSEClient),
		register:          make(chan *SSEClient, 10),
		unregister:        make(chan *SSEClient, 10),
		broadcast:         make(chan *BroadcastMessage, bufferSize),
		heartbeatInterval: heartbeatInterval,
		clientTimeout:     clientTimeout,
		bufferSize:        bufferSize,
		shutdown:          make(chan struct{}),
	}
}

// Start avvia l'hub SSE
func (h *SSEHub) Start() {
	h.wg.Add(1)
	go h.run()
	log.Info().Msg("SSE hub started")
}

// Stop ferma l'hub SSE
func (h *SSEHub) Stop() {
	close(h.shutdown)
	h.wg.Wait()

	// Close all clients
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		client.Cancel()
		close(client.Channel)
	}

	log.Info().Msg("SSE hub stopped")
}

// run è il loop principale dell'hub
func (h *SSEHub) run() {
	defer h.wg.Done()

	heartbeatTicker := time.NewTicker(h.heartbeatInterval)
	defer heartbeatTicker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-heartbeatTicker.C:
			h.sendHeartbeat()

		case <-cleanupTicker.C:
			h.cleanupStaleClients()

		case <-h.shutdown:
			return
		}
	}
}

// RegisterClient registra un nuovo client
func (h *SSEHub) RegisterClient(ctx context.Context, clientID string, channels []EventType) *SSEClient {
	ctx, cancel := context.WithCancel(ctx)

	client := &SSEClient{
		ID:            clientID,
		Channel:       make(chan *SSEEvent, h.bufferSize),
		Channels:      channels,
		Connected:     time.Now(),
		LastHeartbeat: time.Now(),
		Context:       ctx,
		Cancel:        cancel,
	}

	h.register <- client
	return client
}

// UnregisterClient rimuove un client
func (h *SSEHub) UnregisterClient(client *SSEClient) {
	h.unregister <- client
}

// registerClient registra un client internamente
func (h *SSEHub) registerClient(client *SSEClient) {
	h.mu.Lock()
	h.clients[client.ID] = client
	h.mu.Unlock()

	// Subscribe to channels
	h.subMu.Lock()
	for _, channel := range client.Channels {
		if h.subscriptions[channel] == nil {
			h.subscriptions[channel] = make(map[string]*SSEClient)
		}
		h.subscriptions[channel][client.ID] = client
	}
	h.subMu.Unlock()

	log.Info().
		Str("client_id", client.ID).
		Interface("channels", client.Channels).
		Msg("SSE client registered")
}

// unregisterClient rimuove un client internamente
func (h *SSEHub) unregisterClient(client *SSEClient) {
	h.mu.Lock()
	if _, exists := h.clients[client.ID]; exists {
		delete(h.clients, client.ID)
		client.Cancel()
		close(client.Channel)
	}
	h.mu.Unlock()

	// Unsubscribe from all channels
	h.subMu.Lock()
	for channel := range h.subscriptions {
		delete(h.subscriptions[channel], client.ID)
	}
	h.subMu.Unlock()

	log.Info().
		Str("client_id", client.ID).
		Msg("SSE client unregistered")
}

// Broadcast invia un evento a tutti i client sottoscritti
func (h *SSEHub) Broadcast(eventType EventType, data interface{}) {
	event := &SSEEvent{
		ID:   h.generateEventID(),
		Type: eventType,
		Data: data,
	}

	message := &BroadcastMessage{
		Event:    event,
		Channels: []EventType{eventType},
	}

	select {
	case h.broadcast <- message:
		// Message queued
	default:
		log.Warn().
			Str("event_type", string(eventType)).
			Msg("Broadcast channel full, event dropped")
	}
}

// BroadcastToAll invia un evento a tutti i client connessi
func (h *SSEHub) BroadcastToAll(eventType EventType, data interface{}) {
	event := &SSEEvent{
		ID:   h.generateEventID(),
		Type: eventType,
		Data: data,
	}

	message := &BroadcastMessage{
		Event:    event,
		Channels: nil, // nil = broadcast to all
	}

	select {
	case h.broadcast <- message:
		// Message queued
	default:
		log.Warn().
			Str("event_type", string(eventType)).
			Msg("Broadcast channel full, event dropped")
	}
}

// broadcastMessage invia un messaggio ai client appropriati
func (h *SSEHub) broadcastMessage(message *BroadcastMessage) {
	h.subMu.RLock()
	defer h.subMu.RUnlock()

	recipients := make(map[string]*SSEClient)

	if len(message.Channels) == 0 {
		// Broadcast to all clients
		h.mu.RLock()
		for id, client := range h.clients {
			recipients[id] = client
		}
		h.mu.RUnlock()
	} else {
		// Broadcast to subscribed clients
		for _, channel := range message.Channels {
			if subscribers, ok := h.subscriptions[channel]; ok {
				for id, client := range subscribers {
					recipients[id] = client
				}
			}
		}
	}

	// Send to recipients
	sent := 0
	for _, client := range recipients {
		select {
		case client.Channel <- message.Event:
			sent++
		default:
			// Buffer full, disconnect client
			go h.UnregisterClient(client)
			log.Warn().
				Str("client_id", client.ID).
				Msg("Client buffer full, disconnecting")
		}
	}

	log.Debug().
		Str("event_type", string(message.Event.Type)).
		Int("recipients", len(recipients)).
		Int("sent", sent).
		Msg("Event broadcast")
}

// sendHeartbeat invia heartbeat a tutti i client
func (h *SSEHub) sendHeartbeat() {
	h.mu.RLock()
	clients := make([]*SSEClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	event := &SSEEvent{
		ID:   h.generateEventID(),
		Type: EventTypeHeartbeat,
		Data: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	for _, client := range clients {
		select {
		case client.Channel <- event:
			client.LastHeartbeat = time.Now()
		default:
			// Client not responding
		}
	}
}

// cleanupStaleClients rimuove i client inattivi
func (h *SSEHub) cleanupStaleClients() {
	h.mu.RLock()
	staleClients := make([]*SSEClient, 0)
	now := time.Now()

	for _, client := range h.clients {
		if now.Sub(client.LastHeartbeat) > h.clientTimeout {
			staleClients = append(staleClients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range staleClients {
		log.Info().
			Str("client_id", client.ID).
			Dur("inactive", time.Since(client.LastHeartbeat)).
			Msg("Disconnecting stale client")
		h.UnregisterClient(client)
	}
}

// generateEventID genera un ID univoco per l'evento
func (h *SSEHub) generateEventID() string {
	h.eventIDMu.Lock()
	defer h.eventIDMu.Unlock()

	h.eventIDCounter++
	return fmt.Sprintf("%d-%d", time.Now().Unix(), h.eventIDCounter)
}

// GetStats restituisce statistiche sull'hub
func (h *SSEHub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.subMu.RLock()
	defer h.subMu.RUnlock()

	stats := HubStats{
		TotalClients:  len(h.clients),
		Subscriptions: make(map[string]int),
	}

	for channel, subscribers := range h.subscriptions {
		stats.Subscriptions[string(channel)] = len(subscribers)
	}

	return stats
}

// HubStats rappresenta le statistiche dell'hub
type HubStats struct {
	TotalClients  int            `json:"total_clients"`
	Subscriptions map[string]int `json:"subscriptions"`
}

// FormatSSE formatta un evento SSE secondo lo standard
func (e *SSEEvent) FormatSSE() (string, error) {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return "", err
	}

	var formatted string
	if e.ID != "" {
		formatted += fmt.Sprintf("id: %s\n", e.ID)
	}
	formatted += fmt.Sprintf("event: %s\n", e.Type)
	formatted += fmt.Sprintf("data: %s\n", string(data))
	if e.Retry > 0 {
		formatted += fmt.Sprintf("retry: %d\n", e.Retry)
	}
	formatted += "\n"

	return formatted, nil
}
