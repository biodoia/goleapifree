package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CacheManager gestisce multipli cache layers e fornisce API unificate
type CacheManager struct {
	memory    *MemoryCache
	redis     *RedisCache
	semantic  *SemanticCache
	response  *ResponseCache
	multiLayer *MultiLayerCache
	mu        sync.RWMutex
}

// CacheManagerConfig configurazione per il cache manager
type CacheManagerConfig struct {
	MemoryConfig   *Config
	SemanticConfig *SemanticConfig
	ResponseConfig *ResponseCacheConfig
}

// NewCacheManager crea un nuovo cache manager
func NewCacheManager(config *CacheManagerConfig) (*CacheManager, error) {
	if config == nil {
		config = &CacheManagerConfig{
			MemoryConfig: DefaultConfig(),
		}
	}

	cm := &CacheManager{}

	// Inizializza multi-layer cache
	multiLayer, err := NewMultiLayerCache(config.MemoryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-layer cache: %w", err)
	}
	cm.multiLayer = multiLayer
	cm.memory = multiLayer.memory
	cm.redis = multiLayer.redis

	// Inizializza semantic cache se configurato
	if config.SemanticConfig != nil {
		if config.SemanticConfig.BaseCache == nil {
			config.SemanticConfig.BaseCache = multiLayer
		}
		semantic, err := NewSemanticCache(config.SemanticConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create semantic cache: %w", err)
		}
		cm.semantic = semantic
	}

	// Inizializza response cache se configurato
	if config.ResponseConfig != nil {
		if config.ResponseConfig.BaseCache == nil {
			config.ResponseConfig.BaseCache = multiLayer
		}
		if config.ResponseConfig.SemanticCache == nil && cm.semantic != nil {
			config.ResponseConfig.SemanticCache = cm.semantic
			config.ResponseConfig.UseSemanticCache = true
		}
		response, err := NewResponseCache(config.ResponseConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create response cache: %w", err)
		}
		cm.response = response
	}

	return cm, nil
}

// GetMemoryCache restituisce il memory cache
func (cm *CacheManager) GetMemoryCache() *MemoryCache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.memory
}

// GetRedisCache restituisce il redis cache
func (cm *CacheManager) GetRedisCache() *RedisCache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.redis
}

// GetSemanticCache restituisce il semantic cache
func (cm *CacheManager) GetSemanticCache() *SemanticCache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.semantic
}

// GetResponseCache restituisce il response cache
func (cm *CacheManager) GetResponseCache() *ResponseCache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.response
}

// GetMultiLayerCache restituisce il multi-layer cache
func (cm *CacheManager) GetMultiLayerCache() *MultiLayerCache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.multiLayer
}

// GetAllStats restituisce statistiche aggregate da tutti i cache
func (cm *CacheManager) GetAllStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]interface{})

	if cm.multiLayer != nil {
		stats["multi_layer"] = cm.multiLayer.Stats()
	}

	if cm.memory != nil {
		stats["memory"] = cm.memory.Stats()
	}

	if cm.redis != nil {
		stats["redis"] = cm.redis.Stats()
	}

	if cm.semantic != nil {
		stats["semantic"] = cm.semantic.SemanticStats()
	}

	if cm.response != nil {
		stats["response"] = cm.response.ResponseStats()
	}

	return stats
}

// Close chiude tutte le connessioni
func (cm *CacheManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.multiLayer != nil {
		if err := cm.multiLayer.Close(); err != nil {
			return err
		}
	}

	return nil
}

// CacheTier rappresenta un livello di cache
type CacheTier int

const (
	TierMemory CacheTier = iota
	TierRedis
	TierSemantic
	TierResponse
)

// String restituisce la rappresentazione stringa del tier
func (t CacheTier) String() string {
	switch t {
	case TierMemory:
		return "memory"
	case TierRedis:
		return "redis"
	case TierSemantic:
		return "semantic"
	case TierResponse:
		return "response"
	default:
		return "unknown"
	}
}

// CacheOperation rappresenta un'operazione sul cache
type CacheOperation struct {
	Tier      CacheTier
	Operation string // get, set, delete, clear
	Key       string
	Success   bool
	Duration  time.Duration
	Error     error
}

// CacheObserver interfaccia per osservare operazioni sul cache
type CacheObserver interface {
	OnOperation(op *CacheOperation)
}

// LoggingObserver implementa un observer che logga le operazioni
type LoggingObserver struct{}

// OnOperation logga l'operazione
func (o *LoggingObserver) OnOperation(op *CacheOperation) {
	if op.Success {
		log.Debug().
			Str("tier", op.Tier.String()).
			Str("operation", op.Operation).
			Str("key", op.Key).
			Dur("duration", op.Duration).
			Msg("Cache operation successful")
	} else {
		log.Warn().
			Str("tier", op.Tier.String()).
			Str("operation", op.Operation).
			Str("key", op.Key).
			Err(op.Error).
			Msg("Cache operation failed")
	}
}

// CacheWarmer interfaccia per pre-popolare il cache
type CacheWarmer interface {
	Warmup(ctx context.Context) error
}

// CommonQueriesWarmer pre-popola il cache con query comuni
type CommonQueriesWarmer struct {
	cache    *ResponseCache
	queries  []*ChatCompletionRequest
	provider LLMProvider
}

// LLMProvider interfaccia per chiamare LLM
type LLMProvider interface {
	ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (interface{}, error)
}

// NewCommonQueriesWarmer crea un nuovo warmer
func NewCommonQueriesWarmer(cache *ResponseCache, provider LLMProvider, queries []*ChatCompletionRequest) *CommonQueriesWarmer {
	return &CommonQueriesWarmer{
		cache:    cache,
		queries:  queries,
		provider: provider,
	}
}

// Warmup esegue il warmup
func (w *CommonQueriesWarmer) Warmup(ctx context.Context) error {
	for i, query := range w.queries {
		log.Info().
			Int("index", i+1).
			Int("total", len(w.queries)).
			Str("model", query.Model).
			Msg("Warming up cache")

		// Chiama LLM
		response, err := w.provider.ChatCompletion(ctx, query)
		if err != nil {
			log.Warn().Err(err).Int("index", i).Msg("Warmup query failed")
			continue
		}

		// Cacha la response
		if err := w.cache.Set(ctx, query, response, 24*time.Hour); err != nil {
			log.Warn().Err(err).Int("index", i).Msg("Failed to cache warmup response")
		}
	}

	return nil
}

// CacheEvictionPolicy politica di eviction
type CacheEvictionPolicy int

const (
	EvictionLRU CacheEvictionPolicy = iota // Least Recently Used
	EvictionLFU                             // Least Frequently Used
	EvictionFIFO                            // First In First Out
	EvictionTTL                             // Time To Live
)

// String restituisce la rappresentazione stringa
func (p CacheEvictionPolicy) String() string {
	switch p {
	case EvictionLRU:
		return "LRU"
	case EvictionLFU:
		return "LFU"
	case EvictionFIFO:
		return "FIFO"
	case EvictionTTL:
		return "TTL"
	default:
		return "unknown"
	}
}

// CacheInvalidator gestisce l'invalidazione del cache
type CacheInvalidator struct {
	cache Cache
	rules []InvalidationRule
}

// InvalidationRule regola di invalidazione
type InvalidationRule interface {
	ShouldInvalidate(key string, value []byte) bool
}

// NewCacheInvalidator crea un nuovo invalidator
func NewCacheInvalidator(cache Cache) *CacheInvalidator {
	return &CacheInvalidator{
		cache: cache,
		rules: make([]InvalidationRule, 0),
	}
}

// AddRule aggiunge una regola di invalidazione
func (i *CacheInvalidator) AddRule(rule InvalidationRule) {
	i.rules = append(i.rules, rule)
}

// Invalidate invalida il cache secondo le regole
func (i *CacheInvalidator) Invalidate(ctx context.Context) error {
	// TODO: Implementa invalidazione basata su regole
	return nil
}

// TTLBasedRule regola basata su TTL
type TTLBasedRule struct {
	MaxAge time.Duration
}

// ShouldInvalidate verifica se la entry dovrebbe essere invalidata
func (r *TTLBasedRule) ShouldInvalidate(key string, value []byte) bool {
	// TODO: Implementa check TTL
	return false
}

// PatternBasedRule regola basata su pattern
type PatternBasedRule struct {
	Pattern string
}

// ShouldInvalidate verifica se la entry dovrebbe essere invalidata
func (r *PatternBasedRule) ShouldInvalidate(key string, value []byte) bool {
	// TODO: Implementa pattern matching
	return false
}

// CacheSerializer interfaccia per serializzazione custom
type CacheSerializer interface {
	Serialize(v interface{}) ([]byte, error)
	Deserialize(data []byte, v interface{}) error
}

// JSONSerializer serializer JSON
type JSONSerializer struct{}

// Serialize serializza in JSON
func (s *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Deserialize deserializza da JSON
func (s *JSONSerializer) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// CacheKeyGenerator interfaccia per generare chiavi di cache
type CacheKeyGenerator interface {
	Generate(parts ...interface{}) string
}

// DefaultKeyGenerator generatore di chiavi di default
type DefaultKeyGenerator struct{}

// Generate genera una chiave
func (g *DefaultKeyGenerator) Generate(parts ...interface{}) string {
	return HashKey(parts...)
}

// PrefixedKeyGenerator generatore con prefix
type PrefixedKeyGenerator struct {
	Prefix string
}

// Generate genera una chiave con prefix
func (g *PrefixedKeyGenerator) Generate(parts ...interface{}) string {
	baseKey := HashKey(parts...)
	return g.Prefix + ":" + baseKey
}

// CacheMonitor monitora lo stato del cache
type CacheMonitor struct {
	cache    Cache
	interval time.Duration
	stopCh   chan struct{}
	observer CacheObserver
}

// NewCacheMonitor crea un nuovo monitor
func NewCacheMonitor(cache Cache, interval time.Duration, observer CacheObserver) *CacheMonitor {
	return &CacheMonitor{
		cache:    cache,
		interval: interval,
		stopCh:   make(chan struct{}),
		observer: observer,
	}
}

// Start avvia il monitoring
func (m *CacheMonitor) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.check()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop ferma il monitoring
func (m *CacheMonitor) Stop() {
	close(m.stopCh)
}

// check esegue un check del cache
func (m *CacheMonitor) check() {
	stats := m.cache.Stats()

	log.Debug().
		Int64("hits", stats.Hits).
		Int64("misses", stats.Misses).
		Float64("hit_rate", stats.HitRate()).
		Int64("size", stats.Size).
		Msg("Cache health check")
}
