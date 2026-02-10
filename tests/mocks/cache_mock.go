package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
)

// MockCache is a mock implementation of the Cache interface
type MockCache struct {
	mu      sync.RWMutex
	data    map[string][]byte
	stats   cache.CacheStats
	ttls    map[string]time.Time
	failure bool
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
		stats: cache.CacheStats{
			Hits:    0,
			Misses:  0,
			Sets:    0,
			Deletes: 0,
		},
	}
}

// Get retrieves a value from the cache
func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.failure {
		return nil, &MockError{Message: "mock cache failure"}
	}

	// Check if key exists and not expired
	if data, exists := m.data[key]; exists {
		if expiry, hasExpiry := m.ttls[key]; hasExpiry {
			if time.Now().After(expiry) {
				m.mu.RUnlock()
				m.mu.Lock()
				delete(m.data, key)
				delete(m.ttls, key)
				m.stats.Misses++
				m.mu.Unlock()
				m.mu.RLock()
				return nil, cache.ErrCacheMiss
			}
		}
		m.mu.RUnlock()
		m.mu.Lock()
		m.stats.Hits++
		m.mu.Unlock()
		m.mu.RLock()
		return data, nil
	}

	m.mu.RUnlock()
	m.mu.Lock()
	m.stats.Misses++
	m.mu.Unlock()
	m.mu.RLock()
	return nil, cache.ErrCacheMiss
}

// Set stores a value in the cache
func (m *MockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failure {
		return &MockError{Message: "mock cache failure"}
	}

	m.data[key] = value
	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}
	m.stats.Sets++
	return nil
}

// Delete removes a value from the cache
func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failure {
		return &MockError{Message: "mock cache failure"}
	}

	delete(m.data, key)
	delete(m.ttls, key)
	m.stats.Deletes++
	return nil
}

// Clear removes all values from the cache
func (m *MockCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failure {
		return &MockError{Message: "mock cache failure"}
	}

	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	return nil
}

// Stats returns cache statistics
func (m *MockCache) Stats() cache.CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SetFailure configures the cache to fail
func (m *MockCache) SetFailure(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failure = fail
}

// Reset resets the cache state
func (m *MockCache) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	m.stats = cache.CacheStats{}
	m.failure = false
}

// Size returns the number of items in the cache
func (m *MockCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
