package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SSEEvent rappresenta un evento SSE ricevuto
type SSEEvent struct {
	ID    string          `json:"id"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
	Retry int             `json:"retry,omitempty"`
}

// SSEClient è un client per Server-Sent Events
type SSEClient struct {
	baseURL      string
	client       *http.Client
	handlers     map[string]EventHandler
	handlersMu   sync.RWMutex
	connected    bool
	connectedMu  sync.RWMutex
	lastEventID  string
	lastEventMu  sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// EventHandler è una funzione che gestisce un evento SSE
type EventHandler func(event SSEEvent)

// NewSSEClient crea un nuovo client SSE
func NewSSEClient(baseURL string) *SSEClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &SSEClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 0, // No timeout for SSE
		},
		handlers: make(map[string]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// On registra un handler per un tipo di evento
func (c *SSEClient) On(eventType string, handler EventHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()

	c.handlers[eventType] = handler
}

// Connect si connette allo stream SSE
func (c *SSEClient) Connect(endpoint string) error {
	url := c.baseURL + endpoint

	c.wg.Add(1)
	go c.streamLoop(url)

	return nil
}

// streamLoop è il loop principale di streaming
func (c *SSEClient) streamLoop(url string) {
	defer c.wg.Done()

	retryDelay := time.Second
	maxRetryDelay := time.Minute

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		err := c.stream(url)
		if err != nil {
			log.Error().Err(err).Str("url", url).Msg("SSE stream error")

			c.setConnected(false)

			// Exponential backoff
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(retryDelay):
				retryDelay *= 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
			}
		} else {
			// Connection closed normally
			return
		}
	}
}

// stream gestisce una singola connessione SSE
func (c *SSEClient) stream(url string) error {
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Aggiungi Last-Event-ID se presente
	c.lastEventMu.Lock()
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}
	c.lastEventMu.Unlock()

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	c.setConnected(true)
	log.Info().Str("url", url).Msg("SSE connected")

	scanner := bufio.NewScanner(resp.Body)
	var event SSEEvent

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line means event complete
		if line == "" {
			if event.Event != "" {
				c.handleEvent(event)

				// Update last event ID
				if event.ID != "" {
					c.lastEventMu.Lock()
					c.lastEventID = event.ID
					c.lastEventMu.Unlock()
				}
			}
			event = SSEEvent{}
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}

		field, value := parts[0], parts[1]

		switch field {
		case "id":
			event.ID = value
		case "event":
			event.Event = value
		case "data":
			event.Data = json.RawMessage(value)
		case "retry":
			// Parse retry time
			var retry int
			fmt.Sscanf(value, "%d", &retry)
			event.Retry = retry
		}
	}

	return scanner.Err()
}

// handleEvent gestisce un evento ricevuto
func (c *SSEClient) handleEvent(event SSEEvent) {
	c.handlersMu.RLock()
	handler, exists := c.handlers[event.Event]
	c.handlersMu.RUnlock()

	if exists {
		// Esegui handler in goroutine separata
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Str("event_type", event.Event).
						Msg("Event handler panicked")
				}
			}()

			handler(event)
		}()
	}
}

// IsConnected ritorna true se connesso
func (c *SSEClient) IsConnected() bool {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

// setConnected imposta lo stato di connessione
func (c *SSEClient) setConnected(connected bool) {
	c.connectedMu.Lock()
	c.connected = connected
	c.connectedMu.Unlock()
}

// Close chiude il client
func (c *SSEClient) Close() {
	c.cancel()
	c.wg.Wait()
	log.Info().Msg("SSE client closed")
}

// StatsData rappresenta i dati delle statistiche
type StatsData struct {
	Timestamp     int64                    `json:"timestamp"`
	TotalRequests int64                    `json:"total_requests"`
	TotalTokens   int64                    `json:"total_tokens"`
	TotalCost     float64                  `json:"total_cost"`
	Providers     []map[string]interface{} `json:"providers"`
}

// LogsData rappresenta i dati dei log
type LogsData struct {
	Timestamp int64                    `json:"timestamp"`
	Logs      []map[string]interface{} `json:"logs"`
	Count     int                      `json:"count"`
}

// ProvidersData rappresenta i dati dei provider
type ProvidersData struct {
	Timestamp int64                    `json:"timestamp"`
	Providers []map[string]interface{} `json:"providers"`
	Total     int                      `json:"total"`
}

// Example usage functions

// ConnectStats si connette allo stream delle statistiche
func (c *SSEClient) ConnectStats(handler func(StatsData)) error {
	c.On("stats", func(event SSEEvent) {
		var data StatsData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal stats data")
			return
		}
		handler(data)
	})

	c.On("heartbeat", func(event SSEEvent) {
		log.Debug().Msg("Heartbeat received")
	})

	return c.Connect("/stream/stats")
}

// ConnectLogs si connette allo stream dei log
func (c *SSEClient) ConnectLogs(handler func(LogsData)) error {
	c.On("logs", func(event SSEEvent) {
		var data LogsData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal logs data")
			return
		}
		handler(data)
	})

	return c.Connect("/stream/logs")
}

// ConnectProviders si connette allo stream dei provider
func (c *SSEClient) ConnectProviders(handler func(ProvidersData)) error {
	c.On("providers", func(event SSEEvent) {
		var data ProvidersData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal providers data")
			return
		}
		handler(data)
	})

	return c.Connect("/stream/providers")
}

// ConnectAll si connette a tutti gli stream
func (c *SSEClient) ConnectAll(
	statsHandler func(StatsData),
	logsHandler func(LogsData),
	providersHandler func(ProvidersData),
) error {
	c.On("stats", func(event SSEEvent) {
		var data StatsData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal stats data")
			return
		}
		if statsHandler != nil {
			statsHandler(data)
		}
	})

	c.On("logs", func(event SSEEvent) {
		var data LogsData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal logs data")
			return
		}
		if logsHandler != nil {
			logsHandler(data)
		}
	})

	c.On("providers", func(event SSEEvent) {
		var data ProvidersData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal providers data")
			return
		}
		if providersHandler != nil {
			providersHandler(data)
		}
	})

	c.On("heartbeat", func(event SSEEvent) {
		log.Debug().Msg("Heartbeat received")
	})

	return c.Connect("/stream/all")
}
