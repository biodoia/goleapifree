package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType rappresenta il tipo di evento WebSocket
type EventType string

const (
	// Event types
	EventTypeProviderStatus EventType = "provider_status"
	EventTypeRequest        EventType = "request"
	EventTypeLog            EventType = "log"
	EventTypeStatsUpdate    EventType = "stats_update"
	EventTypeError          EventType = "error"
	EventTypePing           EventType = "ping"
	EventTypePong           EventType = "pong"
	EventTypeSubscribe      EventType = "subscribe"
	EventTypeUnsubscribe    EventType = "unsubscribe"
)

// Event è la struttura base per tutti gli eventi WebSocket
type Event struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewEvent crea un nuovo evento
func NewEvent(eventType EventType, data interface{}) (*Event, error) {
	var rawData json.RawMessage
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = bytes
	}

	return &Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      rawData,
	}, nil
}

// ToJSON serializza l'evento in JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ProviderStatusEvent contiene informazioni sullo stato di un provider
type ProviderStatusEvent struct {
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Status       string    `json:"status"` // healthy, degraded, unhealthy
	Available    bool      `json:"available"`
	LatencyMs    int       `json:"latency_ms"`
	SuccessRate  float64   `json:"success_rate"`
	Message      string    `json:"message,omitempty"`
}

// RequestEvent contiene informazioni su una richiesta
type RequestEvent struct {
	RequestID    uuid.UUID `json:"request_id"`
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	ModelID      uuid.UUID `json:"model_id"`
	ModelName    string    `json:"model_name"`
	Method       string    `json:"method"`
	Endpoint     string    `json:"endpoint"`
	StatusCode   int       `json:"status_code"`
	LatencyMs    int       `json:"latency_ms"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// LogEvent contiene un messaggio di log
type LogEvent struct {
	Level      string            `json:"level"` // debug, info, warn, error
	Message    string            `json:"message"`
	Component  string            `json:"component,omitempty"`
	ProviderID uuid.UUID         `json:"provider_id,omitempty"`
	RequestID  uuid.UUID         `json:"request_id,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}

// StatsUpdateEvent contiene statistiche aggregate aggiornate
type StatsUpdateEvent struct {
	// Statistiche globali
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	SuccessRate     float64 `json:"success_rate"`
	AvgLatencyMs    int     `json:"avg_latency_ms"`
	RequestsPerMin  int     `json:"requests_per_min"`

	// Statistiche per provider
	ProviderStats []ProviderStat `json:"provider_stats"`

	// Timestamp del calcolo
	CalculatedAt time.Time `json:"calculated_at"`
	TimeWindow   string    `json:"time_window"` // 1m, 5m, 1h, 24h
}

// ProviderStat statistiche per singolo provider
type ProviderStat struct {
	ProviderID    uuid.UUID `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	Requests      int64     `json:"requests"`
	Tokens        int64     `json:"tokens"`
	SuccessRate   float64   `json:"success_rate"`
	AvgLatencyMs  int       `json:"avg_latency_ms"`
	ErrorCount    int64     `json:"error_count"`
	Available     bool      `json:"available"`
}

// ErrorEvent contiene informazioni su un errore
type ErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SubscribeMessage messaggio di sottoscrizione a un canale
type SubscribeMessage struct {
	Channels []string `json:"channels"`
}

// UnsubscribeMessage messaggio di desottoscrizione da un canale
type UnsubscribeMessage struct {
	Channels []string `json:"channels"`
}

// Channel rappresenta i canali disponibili per la sottoscrizione
type Channel string

const (
	ChannelLogs      Channel = "logs"
	ChannelStats     Channel = "stats"
	ChannelProviders Channel = "providers"
	ChannelRequests  Channel = "requests"
	ChannelAll       Channel = "all"
)

// IsValidChannel verifica se un canale è valido
func IsValidChannel(ch string) bool {
	switch Channel(ch) {
	case ChannelLogs, ChannelStats, ChannelProviders, ChannelRequests, ChannelAll:
		return true
	default:
		return false
	}
}
