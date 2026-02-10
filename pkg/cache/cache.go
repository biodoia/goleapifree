package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Cache è l'interfaccia base per tutti i layer di cache
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Stats() CacheStats
}

// CacheStats contiene statistiche sul cache
type CacheStats struct {
	Hits         int64
	Misses       int64
	Sets         int64
	Deletes      int64
	Size         int64
	EvictionRate float64
}

// HitRate calcola il tasso di hit del cache
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// Config configurazione del multi-layer cache
type Config struct {
	// Memory cache settings
	MemoryEnabled    bool
	MemoryMaxSize    int64 // in bytes
	MemoryMaxEntries int
	MemoryTTL        time.Duration

	// Redis settings
	RedisEnabled  bool
	RedisHost     string
	RedisPassword string
	RedisDB       int
	RedisTTL      time.Duration

	// LRU settings
	LRUEnabled   bool
	LRUMaxSize   int
	EvictionRate float64 // percentage (0.1 = 10%)
}

// DefaultConfig restituisce una configurazione di default
func DefaultConfig() *Config {
	return &Config{
		MemoryEnabled:    true,
		MemoryMaxSize:    100 * 1024 * 1024, // 100MB
		MemoryMaxEntries: 10000,
		MemoryTTL:        5 * time.Minute,

		RedisEnabled: false,
		RedisHost:    "localhost:6379",
		RedisDB:      0,
		RedisTTL:     30 * time.Minute,

		LRUEnabled:   true,
		LRUMaxSize:   1000,
		EvictionRate: 0.1,
	}
}

// MultiLayerCache implementa un cache multi-layer con memory + Redis
type MultiLayerCache struct {
	config *Config
	memory *MemoryCache
	redis  *RedisCache
	mu     sync.RWMutex
	stats  CacheStats
}

// NewMultiLayerCache crea un nuovo cache multi-layer
func NewMultiLayerCache(config *Config) (*MultiLayerCache, error) {
	if config == nil {
		config = DefaultConfig()
	}

	mlc := &MultiLayerCache{
		config: config,
		stats:  CacheStats{},
	}

	// Inizializza memory cache
	if config.MemoryEnabled {
		mlc.memory = NewMemoryCache(config.MemoryMaxEntries, config.MemoryTTL)
		log.Info().
			Int("max_entries", config.MemoryMaxEntries).
			Dur("ttl", config.MemoryTTL).
			Msg("Memory cache initialized")
	}

	// Inizializza Redis cache (se abilitato)
	if config.RedisEnabled {
		redisCache, err := NewRedisCache(config.RedisHost, config.RedisPassword, config.RedisDB)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Redis cache, continuing with memory-only")
		} else {
			mlc.redis = redisCache
			log.Info().
				Str("host", config.RedisHost).
				Int("db", config.RedisDB).
				Msg("Redis cache initialized")
		}
	}

	return mlc, nil
}

// Get recupera un valore dal cache (memory first, poi Redis)
func (m *MultiLayerCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prova prima la memory cache
	if m.memory != nil {
		if data, err := m.memory.Get(ctx, key); err == nil {
			m.stats.Hits++
			log.Debug().Str("key", key).Str("layer", "memory").Msg("Cache hit")
			return data, nil
		}
	}

	// Se non trovato in memory, prova Redis
	if m.redis != nil {
		if data, err := m.redis.Get(ctx, key); err == nil {
			m.stats.Hits++
			log.Debug().Str("key", key).Str("layer", "redis").Msg("Cache hit")

			// Promuovi in memory cache per successive hit
			if m.memory != nil {
				_ = m.memory.Set(ctx, key, data, m.config.MemoryTTL)
			}

			return data, nil
		}
	}

	m.stats.Misses++
	log.Debug().Str("key", key).Msg("Cache miss")
	return nil, ErrCacheMiss
}

// Set salva un valore in tutti i layer di cache
func (m *MultiLayerCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.Sets++

	// Salva in memory cache
	if m.memory != nil {
		if err := m.memory.Set(ctx, key, value, ttl); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("Failed to set memory cache")
		}
	}

	// Salva in Redis
	if m.redis != nil {
		if err := m.redis.Set(ctx, key, value, ttl); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("Failed to set Redis cache")
		}
	}

	log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Dur("ttl", ttl).
		Msg("Cache set")

	return nil
}

// Delete rimuove un valore da tutti i layer
func (m *MultiLayerCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.Deletes++

	if m.memory != nil {
		_ = m.memory.Delete(ctx, key)
	}

	if m.redis != nil {
		_ = m.redis.Delete(ctx, key)
	}

	log.Debug().Str("key", key).Msg("Cache delete")
	return nil
}

// Clear svuota tutti i layer di cache
func (m *MultiLayerCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.memory != nil {
		_ = m.memory.Clear(ctx)
	}

	if m.redis != nil {
		_ = m.redis.Clear(ctx)
	}

	log.Info().Msg("Cache cleared")
	return nil
}

// Stats restituisce le statistiche del cache
func (m *MultiLayerCache) Stats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// Close chiude le connessioni dei cache layer
func (m *MultiLayerCache) Close() error {
	if m.redis != nil {
		return m.redis.Close()
	}
	return nil
}

// HashKey genera un hash consistente per una chiave
func HashKey(parts ...interface{}) string {
	h := sha256.New()
	for _, part := range parts {
		data, _ := json.Marshal(part)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CacheEntry rappresenta un entry nel cache con metadata
type CacheEntry struct {
	Key       string
	Value     []byte
	CreatedAt time.Time
	ExpiresAt time.Time
	Hits      int64
	Size      int64
}

// IsExpired controlla se l'entry è scaduta
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// TTL restituisce il time-to-live rimanente
func (e *CacheEntry) TTL() time.Duration {
	if e.IsExpired() {
		return 0
	}
	return time.Until(e.ExpiresAt)
}

// Errori comuni
var (
	ErrCacheMiss     = fmt.Errorf("cache miss")
	ErrCacheFull     = fmt.Errorf("cache is full")
	ErrKeyNotFound   = fmt.Errorf("key not found")
	ErrInvalidConfig = fmt.Errorf("invalid cache configuration")
)
