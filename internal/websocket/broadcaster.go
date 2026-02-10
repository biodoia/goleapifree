package websocket

import (
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
)

// Broadcaster fornisce metodi di convenienza per il broadcasting di eventi
type Broadcaster struct {
	handler *Handler
}

// NewBroadcaster crea un nuovo broadcaster
func NewBroadcaster(handler *Handler) *Broadcaster {
	return &Broadcaster{
		handler: handler,
	}
}

// LogInfo invia un log di livello info
func (b *Broadcaster) LogInfo(message, component string) {
	b.handler.BroadcastLogEvent("info", message, component, uuid.Nil, uuid.Nil, nil)
}

// LogError invia un log di livello error
func (b *Broadcaster) LogError(message, component string, fields map[string]string) {
	b.handler.BroadcastLogEvent("error", message, component, uuid.Nil, uuid.Nil, fields)
}

// LogWarn invia un log di livello warning
func (b *Broadcaster) LogWarn(message, component string) {
	b.handler.BroadcastLogEvent("warn", message, component, uuid.Nil, uuid.Nil, nil)
}

// LogDebug invia un log di livello debug
func (b *Broadcaster) LogDebug(message, component string, fields map[string]string) {
	b.handler.BroadcastLogEvent("debug", message, component, uuid.Nil, uuid.Nil, fields)
}

// ProviderHealthy notifica che un provider è sano
func (b *Broadcaster) ProviderHealthy(provider *models.Provider, latencyMs int) {
	b.handler.BroadcastProviderStatus(provider, "healthy", latencyMs, 1.0, "")
}

// ProviderDegraded notifica che un provider è degradato
func (b *Broadcaster) ProviderDegraded(provider *models.Provider, latencyMs int, successRate float64, message string) {
	b.handler.BroadcastProviderStatus(provider, "degraded", latencyMs, successRate, message)
}

// ProviderUnhealthy notifica che un provider non è disponibile
func (b *Broadcaster) ProviderUnhealthy(provider *models.Provider, message string) {
	b.handler.BroadcastProviderStatus(provider, "unhealthy", 0, 0.0, message)
}

// RequestCompleted notifica il completamento di una richiesta
func (b *Broadcaster) RequestCompleted(requestLog *models.RequestLog, providerName, modelName string) {
	b.handler.BroadcastRequestEvent(requestLog, providerName, modelName)
}

// StatsUpdated invia un aggiornamento delle statistiche
func (b *Broadcaster) StatsUpdated(window string) {
	b.handler.BroadcastStatsUpdate(window)
}

// StartPeriodicStatsUpdates avvia aggiornamenti periodici delle statistiche
func (b *Broadcaster) StartPeriodicStatsUpdates(interval time.Duration, window string) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				b.StatsUpdated(window)
			case <-stop:
				return
			}
		}
	}()

	return stop
}
