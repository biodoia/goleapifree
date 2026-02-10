package graphql

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SubscriptionManager manages GraphQL subscriptions and real-time events
type SubscriptionManager struct {
	db *database.DB

	// Event channels
	providerUpdates chan *ProviderUpdate
	requestEvents   chan *RequestEvent
	healthUpdates   chan []*ProviderHealthStatus

	// Subscriber management
	mu          sync.RWMutex
	subscribers map[string][]chan interface{}

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(db *database.DB) *SubscriptionManager {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &SubscriptionManager{
		db:              db,
		providerUpdates: make(chan *ProviderUpdate, 100),
		requestEvents:   make(chan *RequestEvent, 1000),
		healthUpdates:   make(chan []*ProviderHealthStatus, 10),
		subscribers:     make(map[string][]chan interface{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Start background workers
	go sm.monitorHealth()
	go sm.cleanupStaleSubscribers()

	return sm
}

// Close shuts down the subscription manager
func (sm *SubscriptionManager) Close() {
	sm.cancel()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close all subscriber channels
	for _, subs := range sm.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}

	close(sm.providerUpdates)
	close(sm.requestEvents)
	close(sm.healthUpdates)
}

// ================================================================================
// Subscription Resolvers
// ================================================================================

// LiveStats subscription - emits global stats at regular intervals
func (sm *SubscriptionManager) LiveStats(ctx context.Context, interval int) (<-chan *GlobalStats, error) {
	ch := make(chan *GlobalStats, 1)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		resolver := NewResolver(sm.db)

		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.ctx.Done():
				return
			case <-ticker.C:
				// Calculate stats
				stats, err := resolver.Stats(ctx, TimeRangeInput{
					Start: time.Now().Add(-24 * time.Hour),
					End:   time.Now(),
				})

				if err != nil {
					log.Error().Err(err).Msg("Failed to calculate stats for subscription")
					continue
				}

				select {
				case ch <- stats:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// ProviderUpdates subscription - emits provider status changes
func (sm *SubscriptionManager) ProviderUpdates(ctx context.Context, providerID *uuid.UUID) (<-chan *ProviderUpdate, error) {
	ch := make(chan *ProviderUpdate, 10)

	sm.subscribe("provider_updates", ch)

	go func() {
		defer func() {
			sm.unsubscribe("provider_updates", ch)
			close(ch)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.ctx.Done():
				return
			case update := <-sm.providerUpdates:
				// Filter by provider ID if specified
				if providerID != nil && update.Provider.ID != *providerID {
					continue
				}

				select {
				case ch <- update:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// RequestStream subscription - emits request events in real-time
func (sm *SubscriptionManager) RequestStream(ctx context.Context, providerID *uuid.UUID) (<-chan *RequestEvent, error) {
	ch := make(chan *RequestEvent, 100)

	sm.subscribe("request_stream", ch)

	go func() {
		defer func() {
			sm.unsubscribe("request_stream", ch)
			close(ch)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.ctx.Done():
				return
			case event := <-sm.requestEvents:
				// Filter by provider ID if specified
				if providerID != nil && event.ProviderID != *providerID {
					continue
				}

				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// HealthUpdates subscription - emits provider health status at regular intervals
func (sm *SubscriptionManager) HealthUpdates(ctx context.Context, interval int) (<-chan []*ProviderHealthStatus, error) {
	ch := make(chan []*ProviderHealthStatus, 1)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.ctx.Done():
				return
			case <-ticker.C:
				healthStatuses := sm.getProviderHealthStatuses()

				select {
				case ch <- healthStatuses:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// ================================================================================
// Event Publishers (called by gateway/handlers)
// ================================================================================

// PublishProviderUpdate publishes a provider status change event
func (sm *SubscriptionManager) PublishProviderUpdate(updateType string, provider *models.Provider) {
	update := &ProviderUpdate{
		Type:      updateType,
		Provider:  provider,
		Timestamp: time.Now(),
	}

	select {
	case sm.providerUpdates <- update:
	default:
		log.Warn().Msg("Provider updates channel full, dropping event")
	}
}

// PublishRequestEvent publishes a request completion event
func (sm *SubscriptionManager) PublishRequestEvent(event *RequestEvent) {
	select {
	case sm.requestEvents <- event:
	default:
		log.Warn().Msg("Request events channel full, dropping event")
	}
}

// ================================================================================
// Internal Helpers
// ================================================================================

func (sm *SubscriptionManager) subscribe(topic string, ch chan interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.subscribers[topic] = append(sm.subscribers[topic], ch)
}

func (sm *SubscriptionManager) unsubscribe(topic string, ch chan interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := sm.subscribers[topic]
	for i, subscriber := range subs {
		if subscriber == ch {
			sm.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func (sm *SubscriptionManager) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			healthStatuses := sm.getProviderHealthStatuses()

			select {
			case sm.healthUpdates <- healthStatuses:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (sm *SubscriptionManager) getProviderHealthStatuses() []*ProviderHealthStatus {
	var providers []models.Provider
	sm.db.DB.Where("status = ?", models.ProviderStatusActive).Find(&providers)

	resolver := NewResolver(sm.db)
	statuses := make([]*ProviderHealthStatus, 0, len(providers))

	for _, provider := range providers {
		status, err := resolver.ProviderHealth(context.Background(), provider.ID)
		if err != nil {
			log.Error().Err(err).Str("provider", provider.Name).Msg("Failed to get provider health")
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses
}

func (sm *SubscriptionManager) cleanupStaleSubscribers() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.mu.Lock()
			totalSubscribers := 0
			for _, subs := range sm.subscribers {
				totalSubscribers += len(subs)
			}
			sm.mu.Unlock()

			log.Debug().
				Int("total_subscribers", totalSubscribers).
				Msg("Active GraphQL subscriptions")
		}
	}
}

// ================================================================================
// Subscription Event Types
// ================================================================================

// CreateRequestEventFromLog creates a RequestEvent from a RequestLog model
func CreateRequestEventFromLog(log *models.RequestLog, providerName, modelName string) *RequestEvent {
	return &RequestEvent{
		ID:           log.ID,
		ProviderID:   log.ProviderID,
		ProviderName: providerName,
		ModelName:    modelName,
		StatusCode:   log.StatusCode,
		LatencyMs:    log.LatencyMs,
		Success:      log.Success,
		Timestamp:    log.Timestamp,
	}
}
