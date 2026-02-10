package graphql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
)

// DataLoader prevents N+1 queries by batching database requests
type DataLoader struct {
	db *database.DB

	// Provider loader
	providerLoader     *batchLoader[uuid.UUID, *models.Provider]
	providerLoaderOnce sync.Once

	// Model loader
	modelLoader     *batchLoader[uuid.UUID, *models.Model]
	modelLoaderOnce sync.Once

	// RateLimit loader
	rateLimitLoader     *batchLoader[uuid.UUID, []*models.RateLimit]
	rateLimitLoaderOnce sync.Once
}

// NewDataLoader creates a new data loader
func NewDataLoader(db *database.DB) *DataLoader {
	return &DataLoader{
		db: db,
	}
}

// LoadProvider loads a provider by ID with batching
func (dl *DataLoader) LoadProvider(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	dl.providerLoaderOnce.Do(func() {
		dl.providerLoader = newBatchLoader(
			10*time.Millisecond,
			func(ids []uuid.UUID) (map[uuid.UUID]*models.Provider, error) {
				var providers []models.Provider
				if err := dl.db.DB.Where("id IN ?", ids).
					Preload("Models").
					Preload("RateLimits").
					Find(&providers).Error; err != nil {
					return nil, err
				}

				result := make(map[uuid.UUID]*models.Provider)
				for i := range providers {
					result[providers[i].ID] = &providers[i]
				}
				return result, nil
			},
		)
	})

	return dl.providerLoader.Load(ctx, id)
}

// LoadModel loads a model by ID with batching
func (dl *DataLoader) LoadModel(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	dl.modelLoaderOnce.Do(func() {
		dl.modelLoader = newBatchLoader(
			10*time.Millisecond,
			func(ids []uuid.UUID) (map[uuid.UUID]*models.Model, error) {
				var models []models.Model
				if err := dl.db.DB.Where("id IN ?", ids).
					Preload("Provider").
					Find(&models).Error; err != nil {
					return nil, err
				}

				result := make(map[uuid.UUID]*models.Model)
				for i := range models {
					result[models[i].ID] = &models[i]
				}
				return result, nil
			},
		)
	})

	return dl.modelLoader.Load(ctx, id)
}

// LoadRateLimitsByProvider loads rate limits for a provider with batching
func (dl *DataLoader) LoadRateLimitsByProvider(ctx context.Context, providerID uuid.UUID) ([]*models.RateLimit, error) {
	dl.rateLimitLoaderOnce.Do(func() {
		dl.rateLimitLoader = newBatchLoader(
			10*time.Millisecond,
			func(ids []uuid.UUID) (map[uuid.UUID][]*models.RateLimit, error) {
				var rateLimits []models.RateLimit
				if err := dl.db.DB.Where("provider_id IN ?", ids).
					Find(&rateLimits).Error; err != nil {
					return nil, err
				}

				result := make(map[uuid.UUID][]*models.RateLimit)
				for i := range rateLimits {
					provID := rateLimits[i].ProviderID
					result[provID] = append(result[provID], &rateLimits[i])
				}
				return result, nil
			},
		)
	})

	return dl.rateLimitLoader.Load(ctx, providerID)
}

// ================================================================================
// Batch Loader Implementation
// ================================================================================

type batchLoader[K comparable, V any] struct {
	wait      time.Duration
	batchFunc func([]K) (map[K]V, error)

	mu      sync.Mutex
	batch   []K
	results map[K]chan result[V]
	timer   *time.Timer
}

type result[V any] struct {
	value V
	err   error
}

func newBatchLoader[K comparable, V any](
	wait time.Duration,
	batchFunc func([]K) (map[K]V, error),
) *batchLoader[K, V] {
	return &batchLoader[K, V]{
		wait:      wait,
		batchFunc: batchFunc,
		results:   make(map[K]chan result[V]),
	}
}

func (bl *batchLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
	// Create result channel
	resultCh := make(chan result[V], 1)

	bl.mu.Lock()

	// Add to batch
	bl.batch = append(bl.batch, key)
	bl.results[key] = resultCh

	// Start timer if not already running
	if bl.timer == nil {
		bl.timer = time.AfterFunc(bl.wait, func() {
			bl.execute()
		})
	}

	bl.mu.Unlock()

	// Wait for result
	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

func (bl *batchLoader[K, V]) execute() {
	bl.mu.Lock()

	// Get current batch
	batch := bl.batch
	results := bl.results

	// Reset state
	bl.batch = nil
	bl.results = make(map[K]chan result[V])
	bl.timer = nil

	bl.mu.Unlock()

	// Execute batch function
	values, err := bl.batchFunc(batch)

	// Send results
	for _, key := range batch {
		ch := results[key]
		if err != nil {
			ch <- result[V]{err: err}
		} else if value, ok := values[key]; ok {
			ch <- result[V]{value: value}
		} else {
			var zero V
			ch <- result[V]{err: fmt.Errorf("key not found in batch results")}
		}
		close(ch)
	}
}

// ================================================================================
// Context Helpers
// ================================================================================

type contextKey string

const dataLoaderKey contextKey = "dataloader"

// WithDataLoader adds data loader to context
func WithDataLoader(ctx context.Context, loader *DataLoader) context.Context {
	return context.WithValue(ctx, dataLoaderKey, loader)
}

// GetDataLoader retrieves data loader from context
func GetDataLoader(ctx context.Context) *DataLoader {
	loader, _ := ctx.Value(dataLoaderKey).(*DataLoader)
	return loader
}
