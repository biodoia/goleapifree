package websocket

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Hub gestisce tutte le connessioni WebSocket e il broadcasting
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Client subscriptions to channels
	subscriptions map[Channel]map[*Client]bool

	// Inbound messages from clients
	broadcast chan *Message

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex per proteggere l'accesso concorrente
	mu sync.RWMutex

	// Shutdown channel
	shutdown chan struct{}
}

// Message rappresenta un messaggio da broadcastare
type Message struct {
	Event   *Event
	Channel Channel
}

// NewHub crea un nuovo hub WebSocket
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		subscriptions: make(map[Channel]map[*Client]bool),
		broadcast:     make(chan *Message, 256),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		shutdown:      make(chan struct{}),
	}
}

// Run avvia il loop principale dell'hub
func (h *Hub) Run() {
	log.Info().Msg("WebSocket hub started")

	// Ticker per heartbeat
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-heartbeat.C:
			h.sendHeartbeat()

		case <-h.shutdown:
			log.Info().Msg("WebSocket hub shutting down")
			h.closeAllClients()
			return
		}
	}
}

// registerClient registra un nuovo client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	log.Info().
		Str("client_id", client.ID).
		Msg("WebSocket client registered")
}

// unregisterClient rimuove un client
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		// Rimuovi da tutte le sottoscrizioni
		for channel := range h.subscriptions {
			delete(h.subscriptions[channel], client)
		}

		delete(h.clients, client)
		close(client.send)

		log.Info().
			Str("client_id", client.ID).
			Msg("WebSocket client unregistered")
	}
}

// broadcastMessage invia un messaggio a tutti i client sottoscritti
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Converti evento in JSON
	data, err := message.Event.ToJSON()
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize event")
		return
	}

	// Trova i client sottoscritti al canale
	var recipients []*Client

	if subscribers, ok := h.subscriptions[message.Channel]; ok {
		for client := range subscribers {
			recipients = append(recipients, client)
		}
	}

	// Includi anche i client sottoscritti al canale "all"
	if allSubscribers, ok := h.subscriptions[ChannelAll]; ok {
		for client := range allSubscribers {
			// Evita duplicati
			alreadyIncluded := false
			for _, r := range recipients {
				if r == client {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				recipients = append(recipients, client)
			}
		}
	}

	// Invia ai destinatari
	for _, client := range recipients {
		select {
		case client.send <- data:
			// Messaggio inviato
		default:
			// Buffer pieno, chiudi il client
			h.unregisterClient(client)
		}
	}

	log.Debug().
		Str("channel", string(message.Channel)).
		Int("recipients", len(recipients)).
		Msg("Message broadcast")
}

// Subscribe sottoscrive un client a un canale
func (h *Hub) Subscribe(client *Client, channel Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscriptions[channel] == nil {
		h.subscriptions[channel] = make(map[*Client]bool)
	}

	h.subscriptions[channel][client] = true

	log.Info().
		Str("client_id", client.ID).
		Str("channel", string(channel)).
		Msg("Client subscribed to channel")
}

// Unsubscribe rimuove la sottoscrizione di un client da un canale
func (h *Hub) Unsubscribe(client *Client, channel Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subscribers, ok := h.subscriptions[channel]; ok {
		delete(subscribers, client)
	}

	log.Info().
		Str("client_id", client.ID).
		Str("channel", string(channel)).
		Msg("Client unsubscribed from channel")
}

// Broadcast invia un evento a un canale
func (h *Hub) Broadcast(channel Channel, event *Event) {
	select {
	case h.broadcast <- &Message{Event: event, Channel: channel}:
		// Messaggio accodato
	default:
		log.Warn().
			Str("channel", string(channel)).
			Msg("Broadcast channel full, message dropped")
	}
}

// sendHeartbeat invia ping a tutti i client
func (h *Hub) sendHeartbeat() {
	event, err := NewEvent(EventTypePing, nil)
	if err != nil {
		return
	}

	data, err := event.ToJSON()
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
			// Ping inviato
		default:
			// Client non risponde
		}
	}
}

// closeAllClients chiude tutti i client connessi
func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
	}

	h.clients = make(map[*Client]bool)
	h.subscriptions = make(map[Channel]map[*Client]bool)
}

// Stop ferma l'hub
func (h *Hub) Stop() {
	close(h.shutdown)
}

// GetStats restituisce statistiche sull'hub
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HubStats{
		TotalClients:  len(h.clients),
		Subscriptions: make(map[string]int),
	}

	for channel, subscribers := range h.subscriptions {
		stats.Subscriptions[string(channel)] = len(subscribers)
	}

	return stats
}

// HubStats statistiche dell'hub
type HubStats struct {
	TotalClients  int            `json:"total_clients"`
	Subscriptions map[string]int `json:"subscriptions"`
}
