package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/rs/zerolog/log"
)

// HookRegistry gestisce gli event hooks per i plugin
type HookRegistry struct {
	mu           sync.RWMutex
	onRequest    []RequestHook
	onResponse   []ResponseHook
	onError      []ErrorHook
	onSwitch     []ProviderSwitchHook
	onPluginLoad []PluginLoadHook
	onCacheHit   []CacheHook
	onMetric     []MetricHook
}

// RequestHook è chiamato prima di processare una richiesta
type RequestHook func(ctx context.Context, req *HookRequest) error

// ResponseHook è chiamato dopo aver ricevuto una risposta
type ResponseHook func(ctx context.Context, resp *HookResponse) error

// ErrorHook è chiamato quando si verifica un errore
type ErrorHook func(ctx context.Context, err *HookError) error

// ProviderSwitchHook è chiamato quando si cambia provider
type ProviderSwitchHook func(ctx context.Context, event *ProviderSwitchEvent) error

// PluginLoadHook è chiamato quando un plugin viene caricato/scaricato
type PluginLoadHook func(ctx context.Context, name string, plugin Plugin) error

// CacheHook è chiamato per eventi della cache
type CacheHook func(ctx context.Context, event *CacheEvent) error

// MetricHook è chiamato per registrare metriche
type MetricHook func(ctx context.Context, metric *Metric) error

// HookRequest contiene informazioni sulla richiesta
type HookRequest struct {
	RequestID    string
	Provider     string
	Model        string
	Request      *providers.ChatRequest
	StartTime    time.Time
	UserID       string
	Metadata     map[string]interface{}
	AttemptCount int
}

// HookResponse contiene informazioni sulla risposta
type HookResponse struct {
	RequestID      string
	Provider       string
	Model          string
	Response       *providers.ChatResponse
	Duration       time.Duration
	TokensUsed     int
	Cost           float64
	CacheHit       bool
	Metadata       map[string]interface{}
	ResponseStatus int
}

// HookError contiene informazioni sull'errore
type HookError struct {
	RequestID    string
	Provider     string
	Model        string
	Error        error
	ErrorType    ErrorType
	Duration     time.Duration
	Retryable    bool
	AttemptCount int
	Metadata     map[string]interface{}
}

// ErrorType tipo di errore
type ErrorType string

const (
	ErrorTypeNetwork      ErrorType = "network"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeAuth         ErrorType = "auth"
	ErrorTypeInvalidInput ErrorType = "invalid_input"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeServerError  ErrorType = "server_error"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// ProviderSwitchEvent evento di cambio provider
type ProviderSwitchEvent struct {
	RequestID    string
	FromProvider string
	ToProvider   string
	Reason       SwitchReason
	Model        string
	Timestamp    time.Time
	Metadata     map[string]interface{}
}

// SwitchReason motivo del cambio provider
type SwitchReason string

const (
	SwitchReasonFailover     SwitchReason = "failover"
	SwitchReasonLoadBalance  SwitchReason = "load_balance"
	SwitchReasonRateLimit    SwitchReason = "rate_limit"
	SwitchReasonCost         SwitchReason = "cost"
	SwitchReasonPerformance  SwitchReason = "performance"
	SwitchReasonAvailability SwitchReason = "availability"
)

// CacheEvent evento della cache
type CacheEvent struct {
	EventType CacheEventType
	Key       string
	Hit       bool
	Size      int64
	TTL       time.Duration
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// CacheEventType tipo di evento cache
type CacheEventType string

const (
	CacheEventHit    CacheEventType = "hit"
	CacheEventMiss   CacheEventType = "miss"
	CacheEventSet    CacheEventType = "set"
	CacheEventDelete CacheEventType = "delete"
	CacheEventEvict  CacheEventType = "evict"
)

// Metric metrica generica
type Metric struct {
	Name      string
	Value     float64
	Type      MetricType
	Tags      map[string]string
	Timestamp time.Time
	Unit      string
}

// MetricType tipo di metrica
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeTiming    MetricType = "timing"
)

// NewHookRegistry crea un nuovo registro di hook
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		onRequest:    make([]RequestHook, 0),
		onResponse:   make([]ResponseHook, 0),
		onError:      make([]ErrorHook, 0),
		onSwitch:     make([]ProviderSwitchHook, 0),
		onPluginLoad: make([]PluginLoadHook, 0),
		onCacheHit:   make([]CacheHook, 0),
		onMetric:     make([]MetricHook, 0),
	}
}

// OnRequest registra un hook per le richieste
func (h *HookRegistry) OnRequest(hook RequestHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onRequest = append(h.onRequest, hook)
	log.Debug().Msg("Request hook registered")
}

// OnResponse registra un hook per le risposte
func (h *HookRegistry) OnResponse(hook ResponseHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onResponse = append(h.onResponse, hook)
	log.Debug().Msg("Response hook registered")
}

// OnError registra un hook per gli errori
func (h *HookRegistry) OnError(hook ErrorHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onError = append(h.onError, hook)
	log.Debug().Msg("Error hook registered")
}

// OnProviderSwitch registra un hook per i cambi provider
func (h *HookRegistry) OnProviderSwitch(hook ProviderSwitchHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onSwitch = append(h.onSwitch, hook)
	log.Debug().Msg("Provider switch hook registered")
}

// OnPluginLoad registra un hook per il caricamento plugin
func (h *HookRegistry) OnPluginLoad(hook PluginLoadHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onPluginLoad = append(h.onPluginLoad, hook)
	log.Debug().Msg("Plugin load hook registered")
}

// OnCacheEvent registra un hook per eventi cache
func (h *HookRegistry) OnCacheEvent(hook CacheHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onCacheHit = append(h.onCacheHit, hook)
	log.Debug().Msg("Cache hook registered")
}

// OnMetric registra un hook per metriche
func (h *HookRegistry) OnMetric(hook MetricHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMetric = append(h.onMetric, hook)
	log.Debug().Msg("Metric hook registered")
}

// TriggerRequest esegue tutti gli hook di richiesta
func (h *HookRegistry) TriggerRequest(ctx context.Context, req *HookRequest) error {
	h.mu.RLock()
	hooks := make([]RequestHook, len(h.onRequest))
	copy(hooks, h.onRequest)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, req)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("request_id", req.RequestID).
				Msg("Request hook failed")
			// Continue con gli altri hook anche se uno fallisce
		}
	}

	return nil
}

// TriggerResponse esegue tutti gli hook di risposta
func (h *HookRegistry) TriggerResponse(ctx context.Context, resp *HookResponse) error {
	h.mu.RLock()
	hooks := make([]ResponseHook, len(h.onResponse))
	copy(hooks, h.onResponse)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, resp)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("request_id", resp.RequestID).
				Msg("Response hook failed")
		}
	}

	return nil
}

// TriggerError esegue tutti gli hook di errore
func (h *HookRegistry) TriggerError(ctx context.Context, hookErr *HookError) error {
	h.mu.RLock()
	hooks := make([]ErrorHook, len(h.onError))
	copy(hooks, h.onError)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, hookErr)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("request_id", hookErr.RequestID).
				Msg("Error hook failed")
		}
	}

	return nil
}

// TriggerProviderSwitch esegue tutti gli hook di switch provider
func (h *HookRegistry) TriggerProviderSwitch(ctx context.Context, event *ProviderSwitchEvent) error {
	h.mu.RLock()
	hooks := make([]ProviderSwitchHook, len(h.onSwitch))
	copy(hooks, h.onSwitch)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, event)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("from", event.FromProvider).
				Str("to", event.ToProvider).
				Msg("Provider switch hook failed")
		}
	}

	return nil
}

// TriggerPluginLoaded esegue gli hook di caricamento plugin
func (h *HookRegistry) TriggerPluginLoaded(ctx context.Context, name string, plugin Plugin) error {
	h.mu.RLock()
	hooks := make([]PluginLoadHook, len(h.onPluginLoad))
	copy(hooks, h.onPluginLoad)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, name, plugin)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("plugin", name).
				Msg("Plugin load hook failed")
		}
	}

	return nil
}

// TriggerPluginUnloaded esegue gli hook di scaricamento plugin
func (h *HookRegistry) TriggerPluginUnloaded(ctx context.Context, name string) error {
	h.mu.RLock()
	hooks := make([]PluginLoadHook, len(h.onPluginLoad))
	copy(hooks, h.onPluginLoad)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, name, nil)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("plugin", name).
				Msg("Plugin unload hook failed")
		}
	}

	return nil
}

// TriggerCacheEvent esegue gli hook di eventi cache
func (h *HookRegistry) TriggerCacheEvent(ctx context.Context, event *CacheEvent) error {
	h.mu.RLock()
	hooks := make([]CacheHook, len(h.onCacheHit))
	copy(hooks, h.onCacheHit)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, event)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("event_type", string(event.EventType)).
				Msg("Cache hook failed")
		}
	}

	return nil
}

// TriggerMetric esegue gli hook di metriche
func (h *HookRegistry) TriggerMetric(ctx context.Context, metric *Metric) error {
	h.mu.RLock()
	hooks := make([]MetricHook, len(h.onMetric))
	copy(hooks, h.onMetric)
	h.mu.RUnlock()

	for i, hook := range hooks {
		if err := h.executeWithTimeout(ctx, func() error {
			return hook(ctx, metric)
		}); err != nil {
			log.Warn().
				Err(err).
				Int("hook_index", i).
				Str("metric", metric.Name).
				Msg("Metric hook failed")
		}
	}

	return nil
}

// executeWithTimeout esegue una funzione con timeout
func (h *HookRegistry) executeWithTimeout(ctx context.Context, fn func() error) error {
	// Timeout di 5 secondi per gli hook
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("hook execution timeout: %w", ctx.Err())
	}
}

// Clear rimuove tutti gli hook registrati
func (h *HookRegistry) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.onRequest = make([]RequestHook, 0)
	h.onResponse = make([]ResponseHook, 0)
	h.onError = make([]ErrorHook, 0)
	h.onSwitch = make([]ProviderSwitchHook, 0)
	h.onPluginLoad = make([]PluginLoadHook, 0)
	h.onCacheHit = make([]CacheHook, 0)
	h.onMetric = make([]MetricHook, 0)

	log.Info().Msg("All hooks cleared")
}

// Stats restituisce statistiche sugli hook registrati
func (h *HookRegistry) Stats() HookStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HookStats{
		RequestHooks:        len(h.onRequest),
		ResponseHooks:       len(h.onResponse),
		ErrorHooks:          len(h.onError),
		ProviderSwitchHooks: len(h.onSwitch),
		PluginLoadHooks:     len(h.onPluginLoad),
		CacheHooks:          len(h.onCacheHit),
		MetricHooks:         len(h.onMetric),
	}
}

// HookStats statistiche sugli hook
type HookStats struct {
	RequestHooks        int
	ResponseHooks       int
	ErrorHooks          int
	ProviderSwitchHooks int
	PluginLoadHooks     int
	CacheHooks          int
	MetricHooks         int
}
