package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// MemoryCache implementa un cache in-memory con LRU eviction
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]*list.Element
	lru        *list.List
	maxEntries int
	defaultTTL time.Duration
	stats      CacheStats
}

// memoryEntry rappresenta un'entry nel cache con LRU metadata
type memoryEntry struct {
	key       string
	value     []byte
	expiresAt time.Time
	hits      int64
}

// NewMemoryCache crea un nuovo cache in-memory
func NewMemoryCache(maxEntries int, defaultTTL time.Duration) *MemoryCache {
	mc := &MemoryCache{
		entries:    make(map[string]*list.Element),
		lru:        list.New(),
		maxEntries: maxEntries,
		defaultTTL: defaultTTL,
		stats:      CacheStats{},
	}

	// Avvia il cleanup periodico
	go mc.cleanupExpired()

	return mc
}

// Get recupera un valore dal cache
func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	elem, exists := m.entries[key]
	if !exists {
		m.stats.Misses++
		return nil, ErrCacheMiss
	}

	entry := elem.Value.(*memoryEntry)

	// Controlla se è scaduto
	if time.Now().After(entry.expiresAt) {
		m.removeElement(elem)
		m.stats.Misses++
		return nil, ErrCacheMiss
	}

	// Aggiorna LRU (muovi in testa)
	m.lru.MoveToFront(elem)
	entry.hits++
	m.stats.Hits++

	return entry.value, nil
}

// Set salva un valore nel cache
func (m *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ttl == 0 {
		ttl = m.defaultTTL
	}

	// Se la chiave esiste già, aggiorna
	if elem, exists := m.entries[key]; exists {
		entry := elem.Value.(*memoryEntry)
		entry.value = value
		entry.expiresAt = time.Now().Add(ttl)
		m.lru.MoveToFront(elem)
		m.stats.Sets++
		return nil
	}

	// Evict se necessario
	if m.lru.Len() >= m.maxEntries {
		m.evictOldest()
	}

	// Aggiungi nuova entry
	entry := &memoryEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
		hits:      0,
	}

	elem := m.lru.PushFront(entry)
	m.entries[key] = elem
	m.stats.Sets++
	m.stats.Size += int64(len(value))

	return nil
}

// Delete rimuove un valore dal cache
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, exists := m.entries[key]; exists {
		m.removeElement(elem)
		m.stats.Deletes++
	}

	return nil
}

// Clear svuota il cache
func (m *MemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*list.Element)
	m.lru.Init()
	m.stats.Size = 0

	return nil
}

// Stats restituisce le statistiche
func (m *MemoryCache) Stats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// evictOldest rimuove l'entry meno recentemente usata (LRU)
func (m *MemoryCache) evictOldest() {
	elem := m.lru.Back()
	if elem != nil {
		m.removeElement(elem)
		m.stats.EvictionRate++
	}
}

// removeElement rimuove un elemento dal cache
func (m *MemoryCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*memoryEntry)
	delete(m.entries, entry.key)
	m.lru.Remove(elem)
	m.stats.Size -= int64(len(entry.value))
}

// cleanupExpired rimuove periodicamente le entry scadute
func (m *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		var toRemove []*list.Element

		// Identifica entry scadute
		for elem := m.lru.Back(); elem != nil; elem = elem.Prev() {
			entry := elem.Value.(*memoryEntry)
			if now.After(entry.expiresAt) {
				toRemove = append(toRemove, elem)
			}
		}

		// Rimuovi entry scadute
		for _, elem := range toRemove {
			m.removeElement(elem)
		}

		m.mu.Unlock()
	}
}

// Size restituisce il numero di entry nel cache
func (m *MemoryCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lru.Len()
}

// GetWithInfo recupera un valore con metadata aggiuntive
func (m *MemoryCache) GetWithInfo(ctx context.Context, key string) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	elem, exists := m.entries[key]
	if !exists {
		return nil, ErrCacheMiss
	}

	entry := elem.Value.(*memoryEntry)

	// Controlla se è scaduto
	if time.Now().After(entry.expiresAt) {
		m.removeElement(elem)
		return nil, ErrCacheMiss
	}

	// Aggiorna LRU
	m.lru.MoveToFront(elem)
	entry.hits++

	return &CacheEntry{
		Key:       entry.key,
		Value:     entry.value,
		ExpiresAt: entry.expiresAt,
		Hits:      entry.hits,
		Size:      int64(len(entry.value)),
	}, nil
}
