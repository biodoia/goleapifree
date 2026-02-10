package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	// ErrBulkheadFull viene restituito quando il bulkhead è pieno
	ErrBulkheadFull = errors.New("bulkhead is full")

	// ErrBulkheadTimeout viene restituito quando si raggiunge il timeout in coda
	ErrBulkheadTimeout = errors.New("bulkhead queue timeout")
)

// BulkheadConfig contiene la configurazione del bulkhead
type BulkheadConfig struct {
	// MaxConcurrent numero massimo di richieste concorrenti
	MaxConcurrent int

	// MaxQueue dimensione massima della coda di attesa
	MaxQueue int

	// QueueTimeout timeout massimo per attendere in coda
	QueueTimeout time.Duration
}

// DefaultBulkheadConfig restituisce una configurazione di default
func DefaultBulkheadConfig() BulkheadConfig {
	return BulkheadConfig{
		MaxConcurrent: 10,
		MaxQueue:      20,
		QueueTimeout:  5 * time.Second,
	}
}

// Bulkhead implementa il pattern bulkhead per isolamento delle risorse
type Bulkhead struct {
	config BulkheadConfig

	semaphore chan struct{}
	queue     chan *bulkheadRequest

	mu              sync.RWMutex
	activeRequests  int
	queuedRequests  int
	totalRequests   int64
	totalRejected   int64
	totalTimedOut   int64
	totalCompleted  int64
}

// bulkheadRequest rappresenta una richiesta in coda
type bulkheadRequest struct {
	fn       func() error
	resultCh chan error
	ctx      context.Context
}

// NewBulkhead crea un nuovo bulkhead
func NewBulkhead(config BulkheadConfig) *Bulkhead {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = DefaultBulkheadConfig().MaxConcurrent
	}
	if config.MaxQueue < 0 {
		config.MaxQueue = DefaultBulkheadConfig().MaxQueue
	}
	if config.QueueTimeout <= 0 {
		config.QueueTimeout = DefaultBulkheadConfig().QueueTimeout
	}

	b := &Bulkhead{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
		queue:     make(chan *bulkheadRequest, config.MaxQueue),
	}

	// Avvia worker pool
	go b.processQueue()

	return b
}

// Execute esegue una funzione protetta dal bulkhead
func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
	b.mu.Lock()
	b.totalRequests++
	b.mu.Unlock()

	// Prova ad acquisire il semaforo direttamente
	select {
	case b.semaphore <- struct{}{}:
		// Richiesta accettata subito
		return b.executeRequest(ctx, fn)

	default:
		// Semaforo pieno, prova ad accodare
		return b.enqueueRequest(ctx, fn)
	}
}

// executeRequest esegue la richiesta
func (b *Bulkhead) executeRequest(ctx context.Context, fn func() error) error {
	b.mu.Lock()
	b.activeRequests++
	b.mu.Unlock()

	defer func() {
		<-b.semaphore

		b.mu.Lock()
		b.activeRequests--
		b.totalCompleted++
		b.mu.Unlock()
	}()

	// Esegui con context
	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// enqueueRequest accoda una richiesta
func (b *Bulkhead) enqueueRequest(ctx context.Context, fn func() error) error {
	req := &bulkheadRequest{
		fn:       fn,
		resultCh: make(chan error, 1),
		ctx:      ctx,
	}

	// Crea un timeout per la coda
	queueCtx, cancel := context.WithTimeout(ctx, b.config.QueueTimeout)
	defer cancel()

	b.mu.Lock()
	b.queuedRequests++
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.queuedRequests--
		b.mu.Unlock()
	}()

	// Prova ad accodare
	select {
	case b.queue <- req:
		// Richiesta accodata, attendi il risultato
		select {
		case err := <-req.resultCh:
			return err
		case <-queueCtx.Done():
			b.mu.Lock()
			b.totalTimedOut++
			b.mu.Unlock()
			return ErrBulkheadTimeout
		}

	case <-queueCtx.Done():
		// Timeout mentre si prova ad accodare
		b.mu.Lock()
		b.totalRejected++
		b.mu.Unlock()
		return ErrBulkheadFull

	case <-ctx.Done():
		// Context originale cancellato
		return ctx.Err()
	}
}

// processQueue processa le richieste in coda
func (b *Bulkhead) processQueue() {
	for req := range b.queue {
		// Attendi uno slot disponibile
		select {
		case b.semaphore <- struct{}{}:
			// Esegui la richiesta
			go func(r *bulkheadRequest) {
				err := b.executeRequest(r.ctx, r.fn)
				r.resultCh <- err
				close(r.resultCh)
			}(req)

		case <-req.ctx.Done():
			// Context cancellato mentre in coda
			req.resultCh <- req.ctx.Err()
			close(req.resultCh)
		}
	}
}

// GetStats restituisce le statistiche del bulkhead
func (b *Bulkhead) GetStats() BulkheadStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BulkheadStats{
		MaxConcurrent:   b.config.MaxConcurrent,
		MaxQueue:        b.config.MaxQueue,
		ActiveRequests:  b.activeRequests,
		QueuedRequests:  b.queuedRequests,
		TotalRequests:   b.totalRequests,
		TotalRejected:   b.totalRejected,
		TotalTimedOut:   b.totalTimedOut,
		TotalCompleted:  b.totalCompleted,
		AvailableSlots:  b.config.MaxConcurrent - b.activeRequests,
		AvailableQueue:  b.config.MaxQueue - b.queuedRequests,
	}
}

// BulkheadStats contiene le statistiche del bulkhead
type BulkheadStats struct {
	MaxConcurrent  int
	MaxQueue       int
	ActiveRequests int
	QueuedRequests int
	TotalRequests  int64
	TotalRejected  int64
	TotalTimedOut  int64
	TotalCompleted int64
	AvailableSlots int
	AvailableQueue int
}

// IsFull verifica se il bulkhead è pieno
func (b *Bulkhead) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.activeRequests >= b.config.MaxConcurrent &&
		b.queuedRequests >= b.config.MaxQueue
}

// Close chiude il bulkhead
func (b *Bulkhead) Close() {
	close(b.queue)
}

// PerProviderBulkhead gestisce bulkhead per ogni provider
type PerProviderBulkhead struct {
	config    BulkheadConfig
	mu        sync.RWMutex
	bulkheads map[string]*Bulkhead
}

// NewPerProviderBulkhead crea un nuovo manager di bulkhead per provider
func NewPerProviderBulkhead(config BulkheadConfig) *PerProviderBulkhead {
	return &PerProviderBulkhead{
		config:    config,
		bulkheads: make(map[string]*Bulkhead),
	}
}

// Execute esegue una funzione con bulkhead per uno specifico provider
func (ppb *PerProviderBulkhead) Execute(ctx context.Context, provider string, fn func() error) error {
	bulkhead := ppb.getOrCreate(provider)
	return bulkhead.Execute(ctx, fn)
}

// getOrCreate ottiene o crea un bulkhead per un provider
func (ppb *PerProviderBulkhead) getOrCreate(provider string) *Bulkhead {
	ppb.mu.RLock()
	bulkhead, exists := ppb.bulkheads[provider]
	ppb.mu.RUnlock()

	if exists {
		return bulkhead
	}

	ppb.mu.Lock()
	defer ppb.mu.Unlock()

	// Double-check dopo aver acquisito il write lock
	if bulkhead, exists := ppb.bulkheads[provider]; exists {
		return bulkhead
	}

	bulkhead = NewBulkhead(ppb.config)
	ppb.bulkheads[provider] = bulkhead

	log.Debug().
		Str("provider", provider).
		Int("max_concurrent", ppb.config.MaxConcurrent).
		Int("max_queue", ppb.config.MaxQueue).
		Msg("Created bulkhead for provider")

	return bulkhead
}

// GetBulkhead restituisce il bulkhead per un provider
func (ppb *PerProviderBulkhead) GetBulkhead(provider string) (*Bulkhead, bool) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	bulkhead, exists := ppb.bulkheads[provider]
	return bulkhead, exists
}

// GetAllStats restituisce le statistiche di tutti i bulkhead
func (ppb *PerProviderBulkhead) GetAllStats() map[string]BulkheadStats {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	stats := make(map[string]BulkheadStats, len(ppb.bulkheads))
	for provider, bulkhead := range ppb.bulkheads {
		stats[provider] = bulkhead.GetStats()
	}

	return stats
}

// IsProviderAvailable verifica se un provider ha slot disponibili
func (ppb *PerProviderBulkhead) IsProviderAvailable(provider string) bool {
	bulkhead, exists := ppb.GetBulkhead(provider)
	if !exists {
		return true // Se non esiste ancora il bulkhead, consideriamo il provider disponibile
	}

	return !bulkhead.IsFull()
}

// Close chiude tutti i bulkhead
func (ppb *PerProviderBulkhead) Close() {
	ppb.mu.Lock()
	defer ppb.mu.Unlock()

	for provider, bulkhead := range ppb.bulkheads {
		bulkhead.Close()
		log.Debug().
			Str("provider", provider).
			Msg("Closed bulkhead for provider")
	}

	log.Info().Msg("All bulkheads closed")
}
