package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/notifications/channels"
	"github.com/rs/zerolog/log"
)

// Channel interfaccia per i canali di notifica
type Channel interface {
	Start() error
	Stop() error
	Send(ctx context.Context, event Event) error
}

// NotifierConfig configurazione per il notifier
type NotifierConfig struct {
	Enabled        bool
	DefaultTimeout time.Duration
	AsyncMode      bool // Se true, le notifiche sono inviate in modo asincrono
	BufferSize     int  // Dimensione buffer per modalità asincrona
}

// Notifier dispatcher principale per le notifiche
type Notifier struct {
	config *NotifierConfig

	// Canali registrati
	channels map[string]Channel
	chMu     sync.RWMutex

	// Rule engine
	ruleEngine *RuleEngine

	// Modalità asincrona
	eventQueue chan eventWithContext
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// Metriche
	metrics *NotificationMetrics
	metMu   sync.RWMutex
}

// eventWithContext evento con context per processamento asincrono
type eventWithContext struct {
	event Event
	ctx   context.Context
}

// NotificationMetrics metriche delle notifiche
type NotificationMetrics struct {
	TotalSent       int64
	TotalFailed     int64
	ByChannel       map[string]*ChannelMetrics
	ByEventType     map[EventType]int64
	BySeverity      map[Severity]int64
	LastNotification time.Time
}

// ChannelMetrics metriche per canale
type ChannelMetrics struct {
	Sent       int64
	Failed     int64
	AvgLatency time.Duration
	LastSent   time.Time
}

// NewNotifier crea un nuovo notifier
func NewNotifier(config *NotifierConfig) *Notifier {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}

	n := &Notifier{
		config:     config,
		channels:   make(map[string]Channel),
		ruleEngine: NewRuleEngine(),
		stopCh:     make(chan struct{}),
		metrics: &NotificationMetrics{
			ByChannel:   make(map[string]*ChannelMetrics),
			ByEventType: make(map[EventType]int64),
			BySeverity:  make(map[Severity]int64),
		},
	}

	if config.AsyncMode {
		n.eventQueue = make(chan eventWithContext, config.BufferSize)
	}

	return n
}

// Start avvia il notifier
func (n *Notifier) Start() error {
	if !n.config.Enabled {
		log.Info().Msg("Notifier disabled")
		return nil
	}

	// Avvia tutti i canali
	n.chMu.RLock()
	defer n.chMu.RUnlock()

	for name, ch := range n.channels {
		if err := ch.Start(); err != nil {
			log.Error().
				Err(err).
				Str("channel", name).
				Msg("Failed to start channel")
			return fmt.Errorf("failed to start channel %s: %w", name, err)
		}
	}

	// Avvia worker asincrono se necessario
	if n.config.AsyncMode {
		n.wg.Add(1)
		go n.asyncWorker()
	}

	log.Info().
		Int("channels", len(n.channels)).
		Bool("async", n.config.AsyncMode).
		Msg("Notifier started")

	return nil
}

// Stop ferma il notifier
func (n *Notifier) Stop() error {
	if !n.config.Enabled {
		return nil
	}

	// Ferma worker asincrono
	if n.config.AsyncMode {
		close(n.stopCh)
		n.wg.Wait()
	}

	// Ferma tutti i canali
	n.chMu.RLock()
	defer n.chMu.RUnlock()

	for name, ch := range n.channels {
		if err := ch.Stop(); err != nil {
			log.Error().
				Err(err).
				Str("channel", name).
				Msg("Failed to stop channel")
		}
	}

	log.Info().Msg("Notifier stopped")
	return nil
}

// RegisterChannel registra un canale di notifica
func (n *Notifier) RegisterChannel(name string, channel Channel) {
	n.chMu.Lock()
	defer n.chMu.Unlock()

	n.channels[name] = channel

	// Inizializza metriche per il canale
	n.metMu.Lock()
	n.metrics.ByChannel[name] = &ChannelMetrics{}
	n.metMu.Unlock()

	log.Info().
		Str("channel", name).
		Msg("Channel registered")
}

// UnregisterChannel rimuove un canale
func (n *Notifier) UnregisterChannel(name string) {
	n.chMu.Lock()
	defer n.chMu.Unlock()

	delete(n.channels, name)
	log.Info().Str("channel", name).Msg("Channel unregistered")
}

// Notify invia una notifica
func (n *Notifier) Notify(ctx context.Context, event Event) error {
	if !n.config.Enabled {
		return nil
	}

	// Modalità asincrona
	if n.config.AsyncMode {
		select {
		case n.eventQueue <- eventWithContext{event: event, ctx: ctx}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			return fmt.Errorf("event queue is full")
		}
	}

	// Modalità sincrona
	return n.processEvent(ctx, event)
}

// asyncWorker processa eventi in modo asincrono
func (n *Notifier) asyncWorker() {
	defer n.wg.Done()

	for {
		select {
		case <-n.stopCh:
			// Processa eventi rimanenti
			for len(n.eventQueue) > 0 {
				ec := <-n.eventQueue
				n.processEvent(ec.ctx, ec.event)
			}
			return
		case ec := <-n.eventQueue:
			n.processEvent(ec.ctx, ec.event)
		}
	}
}

// processEvent processa un evento applicando le regole
func (n *Notifier) processEvent(ctx context.Context, event Event) error {
	start := time.Now()

	// Aggiorna metriche
	n.updateMetrics(event, true)

	// Ottieni regole applicabili
	rules := n.ruleEngine.ListRules()
	if len(rules) == 0 {
		log.Debug().Msg("No rules configured, skipping notification")
		return nil
	}

	// Applica regole
	sent := false
	var lastErr error

	for _, rule := range rules {
		if !n.ruleEngine.ShouldNotify(event, rule) {
			continue
		}

		// Ottieni canali attivi per questa regola
		activeChannels := n.ruleEngine.GetActiveChannels(rule)
		if len(activeChannels) == 0 {
			continue
		}

		// Invia su ogni canale
		for _, chName := range activeChannels {
			if err := n.sendToChannel(ctx, chName, event); err != nil {
				log.Error().
					Err(err).
					Str("channel", chName).
					Str("rule", rule.Name).
					Msg("Failed to send notification")
				lastErr = err
			} else {
				sent = true
			}
		}

		// Marca regola come triggata
		n.ruleEngine.MarkTriggered(rule)
	}

	if sent {
		log.Info().
			Str("event_type", string(event.Type())).
			Str("severity", string(event.Severity())).
			Dur("duration", time.Since(start)).
			Msg("Notification sent")
	}

	return lastErr
}

// sendToChannel invia su un canale specifico
func (n *Notifier) sendToChannel(ctx context.Context, channelName string, event Event) error {
	n.chMu.RLock()
	channel, exists := n.channels[channelName]
	n.chMu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", channelName)
	}

	// Crea context con timeout
	sendCtx, cancel := context.WithTimeout(ctx, n.config.DefaultTimeout)
	defer cancel()

	// Invia
	start := time.Now()
	err := channel.Send(sendCtx, event)
	duration := time.Since(start)

	// Aggiorna metriche canale
	n.updateChannelMetrics(channelName, err == nil, duration)

	return err
}

// AddRule aggiunge una regola di notifica
func (n *Notifier) AddRule(rule *Rule) {
	n.ruleEngine.AddRule(rule)
}

// RemoveRule rimuove una regola
func (n *Notifier) RemoveRule(ruleID string) {
	// TODO: implementa parsing UUID
	log.Warn().Str("rule_id", ruleID).Msg("RemoveRule not implemented")
}

// ListRules lista tutte le regole
func (n *Notifier) ListRules() []*Rule {
	return n.ruleEngine.ListRules()
}

// GetMetrics ottiene le metriche
func (n *Notifier) GetMetrics() *NotificationMetrics {
	n.metMu.RLock()
	defer n.metMu.RUnlock()

	// Crea copia
	metrics := &NotificationMetrics{
		TotalSent:        n.metrics.TotalSent,
		TotalFailed:      n.metrics.TotalFailed,
		ByChannel:        make(map[string]*ChannelMetrics),
		ByEventType:      make(map[EventType]int64),
		BySeverity:       make(map[Severity]int64),
		LastNotification: n.metrics.LastNotification,
	}

	for k, v := range n.metrics.ByChannel {
		metrics.ByChannel[k] = &ChannelMetrics{
			Sent:       v.Sent,
			Failed:     v.Failed,
			AvgLatency: v.AvgLatency,
			LastSent:   v.LastSent,
		}
	}

	for k, v := range n.metrics.ByEventType {
		metrics.ByEventType[k] = v
	}

	for k, v := range n.metrics.BySeverity {
		metrics.BySeverity[k] = v
	}

	return metrics
}

// updateMetrics aggiorna le metriche generali
func (n *Notifier) updateMetrics(event Event, success bool) {
	n.metMu.Lock()
	defer n.metMu.Unlock()

	if success {
		n.metrics.TotalSent++
	} else {
		n.metrics.TotalFailed++
	}

	n.metrics.ByEventType[event.Type()]++
	n.metrics.BySeverity[event.Severity()]++
	n.metrics.LastNotification = time.Now()
}

// updateChannelMetrics aggiorna le metriche di un canale
func (n *Notifier) updateChannelMetrics(channelName string, success bool, duration time.Duration) {
	n.metMu.Lock()
	defer n.metMu.Unlock()

	metrics, exists := n.metrics.ByChannel[channelName]
	if !exists {
		metrics = &ChannelMetrics{}
		n.metrics.ByChannel[channelName] = metrics
	}

	if success {
		metrics.Sent++
		metrics.LastSent = time.Now()

		// Calcola media mobile della latenza
		if metrics.AvgLatency == 0 {
			metrics.AvgLatency = duration
		} else {
			metrics.AvgLatency = (metrics.AvgLatency + duration) / 2
		}
	} else {
		metrics.Failed++
	}
}

// BuildNotifier costruisce un notifier con configurazione completa
func BuildNotifier(config *NotifierConfig, emailConfig *channels.EmailConfig, webhookConfig *channels.WebhookConfig, desktopConfig *channels.DesktopConfig, logConfig *channels.LogConfig) *Notifier {
	notifier := NewNotifier(config)

	// Registra canali
	if emailConfig != nil {
		emailChannel := channels.NewEmailChannel(emailConfig)
		notifier.RegisterChannel("email", emailChannel)
	}

	if webhookConfig != nil {
		webhookChannel := channels.NewWebhookChannel(webhookConfig)
		notifier.RegisterChannel("webhook", webhookChannel)
	}

	if desktopConfig != nil {
		desktopChannel := channels.NewDesktopChannel(desktopConfig)
		notifier.RegisterChannel("desktop", desktopChannel)
	}

	if logConfig != nil {
		logChannel := channels.NewLogChannel(logConfig)
		notifier.RegisterChannel("log", logChannel)
	}

	// Aggiungi regole di default
	for _, rule := range DefaultRules() {
		notifier.AddRule(rule)
	}

	return notifier
}

// DefaultNotifier crea un notifier con configurazione di default
func DefaultNotifier() *Notifier {
	config := &NotifierConfig{
		Enabled:        true,
		DefaultTimeout: 30 * time.Second,
		AsyncMode:      true,
		BufferSize:     1000,
	}

	logConfig := &channels.LogConfig{
		FilePath:       "./logs/notifications.log",
		MaxFileSize:    10 * 1024 * 1024,
		MaxBackups:     5,
		Format:         "json",
		IncludeConsole: true,
	}

	desktopConfig := &channels.DesktopConfig{
		AppName: "GoLeapAI",
		Enabled: true,
	}

	return BuildNotifier(config, nil, nil, desktopConfig, logConfig)
}
