package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Client rappresenta un client WebSocket connesso
type Client struct {
	// ID univoco del client
	ID string

	// Hub a cui il client appartiene
	hub *Hub

	// WebSocket connection (interfaccia generica per supportare diversi framework)
	conn interface{}

	// Send channel per messaggi in uscita
	send chan []byte

	// Subscribed channels
	channels map[Channel]bool

	// Mutex per proteggere l'accesso concorrente
	mu sync.RWMutex

	// Timestamp ultima attività
	lastActivity time.Time

	// User agent e metadata
	UserAgent string
	RemoteIP  string
}

// NewClient crea un nuovo client WebSocket
func NewClient(hub *Hub, conn interface{}) *Client {
	return &Client{
		ID:           uuid.New().String(),
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		channels:     make(map[Channel]bool),
		lastActivity: time.Now(),
	}
}

// ReadPump legge messaggi dal WebSocket
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	for {
		// Leggi messaggio (da implementare con il framework specifico)
		// Per ora usiamo un placeholder
		select {
		case <-time.After(60 * time.Second):
			// Timeout di inattività
			return
		}
	}
}

// WritePump scrive messaggi al WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Hub ha chiuso il canale
				return
			}

			// Scrivi messaggio (da implementare con il framework specifico)
			_ = message
			c.lastActivity = time.Now()

		case <-ticker.C:
			// Verifica timeout
			if time.Since(c.lastActivity) > 90*time.Second {
				log.Warn().
					Str("client_id", c.ID).
					Msg("Client timeout, closing connection")
				return
			}
		}
	}
}

// HandleMessage gestisce un messaggio ricevuto dal client
func (c *Client) HandleMessage(data []byte) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		log.Error().
			Err(err).
			Str("client_id", c.ID).
			Msg("Failed to parse client message")
		return
	}

	switch event.Type {
	case EventTypeSubscribe:
		c.handleSubscribe(event.Data)
	case EventTypeUnsubscribe:
		c.handleUnsubscribe(event.Data)
	case EventTypePong:
		c.lastActivity = time.Now()
	default:
		log.Warn().
			Str("client_id", c.ID).
			Str("event_type", string(event.Type)).
			Msg("Unknown event type from client")
	}
}

// handleSubscribe gestisce richieste di sottoscrizione
func (c *Client) handleSubscribe(data json.RawMessage) {
	var msg SubscribeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Error().
			Err(err).
			Str("client_id", c.ID).
			Msg("Failed to parse subscribe message")
		return
	}

	for _, ch := range msg.Channels {
		if !IsValidChannel(ch) {
			log.Warn().
				Str("client_id", c.ID).
				Str("channel", ch).
				Msg("Invalid channel")
			continue
		}

		channel := Channel(ch)
		c.hub.Subscribe(c, channel)

		c.mu.Lock()
		c.channels[channel] = true
		c.mu.Unlock()
	}
}

// handleUnsubscribe gestisce richieste di desottoscrizione
func (c *Client) handleUnsubscribe(data json.RawMessage) {
	var msg UnsubscribeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Error().
			Err(err).
			Str("client_id", c.ID).
			Msg("Failed to parse unsubscribe message")
		return
	}

	for _, ch := range msg.Channels {
		channel := Channel(ch)
		c.hub.Unsubscribe(c, channel)

		c.mu.Lock()
		delete(c.channels, channel)
		c.mu.Unlock()
	}
}

// Send invia un messaggio al client
func (c *Client) Send(data []byte) error {
	select {
	case c.send <- data:
		return nil
	default:
		return ErrClientBufferFull
	}
}

// Close chiude la connessione del client
func (c *Client) Close() {
	c.hub.unregister <- c
}

// WebSocketClient client per connessioni in uscita (per TUI/WebUI)
type WebSocketClient struct {
	url          string
	conn         *client.Client
	hub          *Hub
	reconnect    bool
	reconnectDelay time.Duration
	handlers     map[EventType]EventHandler
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// EventHandler funzione per gestire eventi
type EventHandler func(*Event) error

// NewWebSocketClient crea un nuovo client WebSocket per connessioni in uscita
func NewWebSocketClient(url string) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketClient{
		url:            url,
		conn:           client.New(),
		reconnect:      true,
		reconnectDelay: 5 * time.Second,
		handlers:       make(map[EventType]EventHandler),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect stabilisce la connessione WebSocket
func (wc *WebSocketClient) Connect() error {
	// Implementazione della connessione WebSocket
	log.Info().
		Str("url", wc.url).
		Msg("Connecting to WebSocket server")

	// TODO: Implementare connessione reale
	return nil
}

// Subscribe sottoscrive a uno o più canali
func (wc *WebSocketClient) Subscribe(channels ...Channel) error {
	channelNames := make([]string, len(channels))
	for i, ch := range channels {
		channelNames[i] = string(ch)
	}

	msg := SubscribeMessage{
		Channels: channelNames,
	}

	event, err := NewEvent(EventTypeSubscribe, msg)
	if err != nil {
		return err
	}

	return wc.sendEvent(event)
}

// Unsubscribe rimuove la sottoscrizione da uno o più canali
func (wc *WebSocketClient) Unsubscribe(channels ...Channel) error {
	channelNames := make([]string, len(channels))
	for i, ch := range channels {
		channelNames[i] = string(ch)
	}

	msg := UnsubscribeMessage{
		Channels: channelNames,
	}

	event, err := NewEvent(EventTypeUnsubscribe, msg)
	if err != nil {
		return err
	}

	return wc.sendEvent(event)
}

// OnEvent registra un handler per un tipo di evento
func (wc *WebSocketClient) OnEvent(eventType EventType, handler EventHandler) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.handlers[eventType] = handler
}

// sendEvent invia un evento al server
func (wc *WebSocketClient) sendEvent(event *Event) error {
	data, err := event.ToJSON()
	if err != nil {
		return err
	}

	// TODO: Implementare invio reale
	_ = data
	return nil
}

// Start avvia il loop di lettura/scrittura
func (wc *WebSocketClient) Start() error {
	if err := wc.Connect(); err != nil {
		return err
	}

	go wc.readLoop()
	go wc.pingLoop()

	return nil
}

// readLoop loop di lettura messaggi
func (wc *WebSocketClient) readLoop() {
	for {
		select {
		case <-wc.ctx.Done():
			return
		default:
			// TODO: Implementare lettura reale
			time.Sleep(1 * time.Second)
		}
	}
}

// pingLoop invia ping periodici
func (wc *WebSocketClient) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wc.ctx.Done():
			return
		case <-ticker.C:
			event, err := NewEvent(EventTypePing, nil)
			if err != nil {
				continue
			}
			_ = wc.sendEvent(event)
		}
	}
}

// handleEvent gestisce un evento ricevuto
func (wc *WebSocketClient) handleEvent(event *Event) {
	wc.mu.RLock()
	handler, ok := wc.handlers[event.Type]
	wc.mu.RUnlock()

	if ok {
		if err := handler(event); err != nil {
			log.Error().
				Err(err).
				Str("event_type", string(event.Type)).
				Msg("Error handling event")
		}
	}
}

// Close chiude la connessione
func (wc *WebSocketClient) Close() error {
	wc.cancel()
	return nil
}

// Errori
var (
	ErrClientBufferFull = &WebSocketError{Code: "buffer_full", Message: "Client buffer is full"}
)

// WebSocketError errore WebSocket
type WebSocketError struct {
	Code    string
	Message string
}

func (e *WebSocketError) Error() string {
	return e.Message
}
