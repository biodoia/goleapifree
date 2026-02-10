package embeddings

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

// SemanticCache implementa semantic caching usando embeddings
type SemanticCache struct {
	generator      EmbeddingGenerator
	vectorStore    VectorStore
	responseCache  map[string]*CachedResponse
	mu             sync.RWMutex
	config         *SemanticCacheConfig
	stats          SemanticCacheStats
	cleanupTicker  *time.Ticker
	cleanupStop    chan bool
}

// SemanticCacheConfig configurazione per semantic cache
type SemanticCacheConfig struct {
	SimilarityThreshold float64       // Threshold per considerare match (default: 0.9)
	DefaultTTL          time.Duration // TTL di default per cached items
	MaxCacheSize        int           // Numero massimo di items in cache
	CleanupInterval     time.Duration // Intervallo pulizia items scaduti
	EnableAutoCleanup   bool          // Abilita cleanup automatico
}

// DefaultSemanticCacheConfig restituisce configurazione di default
func DefaultSemanticCacheConfig() *SemanticCacheConfig {
	return &SemanticCacheConfig{
		SimilarityThreshold: 0.9,
		DefaultTTL:          10 * time.Minute,
		MaxCacheSize:        1000,
		CleanupInterval:     5 * time.Minute,
		EnableAutoCleanup:   true,
	}
}

// CachedResponse rappresenta una risposta cachata
type CachedResponse struct {
	ID         string
	Prompt     string
	Response   interface{}
	Embedding  []float32
	Metadata   map[string]interface{}
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Hits       int64
	LastHitAt  time.Time
}

// IsExpired controlla se la risposta è scaduta
func (c *CachedResponse) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// TTL restituisce il time-to-live rimanente
func (c *CachedResponse) TTL() time.Duration {
	if c.IsExpired() {
		return 0
	}
	return time.Until(c.ExpiresAt)
}

// SemanticCacheStats statistiche del semantic cache
type SemanticCacheStats struct {
	Hits           int64
	Misses         int64
	Adds           int64
	Evictions      int64
	Expirations    int64
	TotalQueries   int64
	AvgSimilarity  float64
	AvgLookupTime  time.Duration
}

// HitRate calcola il tasso di hit
func (s *SemanticCacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// NewSemanticCache crea un nuovo semantic cache
func NewSemanticCache(generator EmbeddingGenerator, config *SemanticCacheConfig) (*SemanticCache, error) {
	if config == nil {
		config = DefaultSemanticCacheConfig()
	}

	// Crea vector store
	vectorConfig := &VectorStoreConfig{
		MetricType:   MetricCosine,
		Dimensions:   generator.Dimensions(),
		MaxSearchLog: 100,
	}
	vectorStore := NewInMemoryVectorStore(vectorConfig)

	sc := &SemanticCache{
		generator:     generator,
		vectorStore:   vectorStore,
		responseCache: make(map[string]*CachedResponse),
		config:        config,
		stats:         SemanticCacheStats{},
		cleanupStop:   make(chan bool),
	}

	// Avvia cleanup automatico
	if config.EnableAutoCleanup {
		sc.startAutoCleanup()
	}

	log.Info().
		Float64("threshold", config.SimilarityThreshold).
		Dur("ttl", config.DefaultTTL).
		Int("max_size", config.MaxCacheSize).
		Msg("Semantic cache initialized")

	return sc, nil
}

// Get cerca una risposta cachata semanticamente simile
func (sc *SemanticCache) Get(ctx context.Context, prompt string) (interface{}, bool, error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		sc.mu.Lock()
		sc.stats.TotalQueries++
		sc.stats.AvgLookupTime = (sc.stats.AvgLookupTime*time.Duration(sc.stats.TotalQueries-1) + elapsed) / time.Duration(sc.stats.TotalQueries)
		sc.mu.Unlock()
	}()

	// Genera embedding per il prompt
	embedding, err := sc.generator.Generate(ctx, prompt)
	if err != nil {
		return nil, false, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Cerca vettori simili
	results, err := sc.vectorStore.Search(ctx, embedding, 1, sc.config.SimilarityThreshold)
	if err != nil {
		return nil, false, fmt.Errorf("failed to search vectors: %w", err)
	}

	if len(results) == 0 {
		sc.mu.Lock()
		sc.stats.Misses++
		sc.mu.Unlock()

		log.Debug().
			Str("prompt", prompt[:min(50, len(prompt))]).
			Msg("Semantic cache miss")
		return nil, false, nil
	}

	// Recupera la risposta cachata
	sc.mu.RLock()
	cached, ok := sc.responseCache[results[0].ID]
	sc.mu.RUnlock()

	if !ok {
		sc.mu.Lock()
		sc.stats.Misses++
		sc.mu.Unlock()
		return nil, false, nil
	}

	// Controlla se è scaduta
	if cached.IsExpired() {
		sc.mu.Lock()
		sc.stats.Misses++
		sc.stats.Expirations++
		sc.mu.Unlock()

		// Rimuovi item scaduto
		_ = sc.Delete(ctx, results[0].ID)

		log.Debug().
			Str("id", results[0].ID).
			Msg("Cached item expired")
		return nil, false, nil
	}

	// Cache hit!
	sc.mu.Lock()
	sc.stats.Hits++
	cached.Hits++
	cached.LastHitAt = time.Now()
	sc.mu.Unlock()

	log.Debug().
		Str("prompt", prompt[:min(50, len(prompt))]).
		Float64("similarity", results[0].Similarity).
		Msg("Semantic cache hit")

	return cached.Response, true, nil
}

// Set aggiunge una risposta al cache
func (sc *SemanticCache) Set(ctx context.Context, prompt string, response interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = sc.config.DefaultTTL
	}

	// Genera embedding per il prompt
	embedding, err := sc.generator.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Genera ID univoco
	id := generateID(prompt)

	// Controlla limite cache size
	sc.mu.RLock()
	currentSize := len(sc.responseCache)
	sc.mu.RUnlock()

	if currentSize >= sc.config.MaxCacheSize {
		// Evict oldest item
		sc.evictOldest()
	}

	now := time.Now()
	cached := &CachedResponse{
		ID:        id,
		Prompt:    prompt,
		Response:  response,
		Embedding: embedding,
		Metadata: map[string]interface{}{
			"model": sc.generator.ModelName(),
		},
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Hits:      0,
	}

	// Salva nel vector store
	if err := sc.vectorStore.Add(ctx, id, embedding, cached.Metadata); err != nil {
		return fmt.Errorf("failed to add to vector store: %w", err)
	}

	// Salva nel response cache
	sc.mu.Lock()
	sc.responseCache[id] = cached
	sc.stats.Adds++
	sc.mu.Unlock()

	log.Debug().
		Str("id", id).
		Str("prompt", prompt[:min(50, len(prompt))]).
		Dur("ttl", ttl).
		Msg("Added to semantic cache")

	return nil
}

// Delete rimuove un item dal cache
func (sc *SemanticCache) Delete(ctx context.Context, id string) error {
	sc.mu.Lock()
	delete(sc.responseCache, id)
	sc.mu.Unlock()

	if err := sc.vectorStore.Delete(ctx, id); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("Failed to delete from vector store")
	}

	return nil
}

// Clear svuota il cache
func (sc *SemanticCache) Clear(ctx context.Context) error {
	sc.mu.Lock()
	sc.responseCache = make(map[string]*CachedResponse)
	sc.mu.Unlock()

	if err := sc.vectorStore.Clear(ctx); err != nil {
		return err
	}

	log.Info().Msg("Semantic cache cleared")
	return nil
}

// Stats restituisce statistiche del cache
func (sc *SemanticCache) Stats() SemanticCacheStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.stats
}

// Size restituisce il numero di items nel cache
func (sc *SemanticCache) Size() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.responseCache)
}

// evictOldest rimuove l'item più vecchio dal cache
func (sc *SemanticCache) evictOldest() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	var oldestID string
	var oldestTime time.Time

	for id, cached := range sc.responseCache {
		if oldestID == "" || cached.CreatedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = cached.CreatedAt
		}
	}

	if oldestID != "" {
		delete(sc.responseCache, oldestID)
		sc.stats.Evictions++
		_ = sc.vectorStore.Delete(context.Background(), oldestID)

		log.Debug().
			Str("id", oldestID).
			Msg("Evicted oldest item from cache")
	}
}

// Cleanup rimuove items scaduti
func (sc *SemanticCache) Cleanup(ctx context.Context) int {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	expired := make([]string, 0)
	for id, cached := range sc.responseCache {
		if cached.IsExpired() {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		delete(sc.responseCache, id)
		_ = sc.vectorStore.Delete(ctx, id)
		sc.stats.Expirations++
	}

	if len(expired) > 0 {
		log.Debug().
			Int("count", len(expired)).
			Msg("Cleaned up expired cache items")
	}

	return len(expired)
}

// startAutoCleanup avvia il cleanup automatico
func (sc *SemanticCache) startAutoCleanup() {
	sc.cleanupTicker = time.NewTicker(sc.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-sc.cleanupTicker.C:
				sc.Cleanup(context.Background())
			case <-sc.cleanupStop:
				sc.cleanupTicker.Stop()
				return
			}
		}
	}()

	log.Debug().
		Dur("interval", sc.config.CleanupInterval).
		Msg("Auto cleanup started")
}

// Close ferma il cleanup automatico
func (sc *SemanticCache) Close() error {
	if sc.config.EnableAutoCleanup {
		close(sc.cleanupStop)
	}
	return nil
}

// FindSimilar trova prompts simili a quello dato
func (sc *SemanticCache) FindSimilar(ctx context.Context, prompt string, k int) ([]SimilarPrompt, error) {
	// Genera embedding
	embedding, err := sc.generator.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Cerca vettori simili
	results, err := sc.vectorStore.Search(ctx, embedding, k, 0.5) // threshold più basso per trovare simili
	if err != nil {
		return nil, err
	}

	similar := make([]SimilarPrompt, 0, len(results))
	sc.mu.RLock()
	for _, result := range results {
		if cached, ok := sc.responseCache[result.ID]; ok && !cached.IsExpired() {
			similar = append(similar, SimilarPrompt{
				ID:         result.ID,
				Prompt:     cached.Prompt,
				Similarity: result.Similarity,
				Hits:       cached.Hits,
				CreatedAt:  cached.CreatedAt,
			})
		}
	}
	sc.mu.RUnlock()

	return similar, nil
}

// SimilarPrompt rappresenta un prompt simile
type SimilarPrompt struct {
	ID         string
	Prompt     string
	Similarity float64
	Hits       int64
	CreatedAt  time.Time
}

// GetTopHits restituisce gli items più popolari
func (sc *SemanticCache) GetTopHits(n int) []CachedResponse {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	items := make([]CachedResponse, 0, len(sc.responseCache))
	for _, cached := range sc.responseCache {
		if !cached.IsExpired() {
			items = append(items, *cached)
		}
	}

	// Sort by hits (selection sort per primi n)
	for i := 0; i < min(n, len(items)); i++ {
		maxIdx := i
		for j := i + 1; j < len(items); j++ {
			if items[j].Hits > items[maxIdx].Hits {
				maxIdx = j
			}
		}
		if maxIdx != i {
			items[i], items[maxIdx] = items[maxIdx], items[i]
		}
	}

	if len(items) > n {
		items = items[:n]
	}

	return items
}

// generateID genera un ID univoco per un prompt
func generateID(prompt string) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	h.Write([]byte(time.Now().String()))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// MarshalCacheEntry serializza un cache entry
func MarshalCacheEntry(entry *CachedResponse) ([]byte, error) {
	return json.Marshal(entry)
}

// UnmarshalCacheEntry deserializza un cache entry
func UnmarshalCacheEntry(data []byte) (*CachedResponse, error) {
	var entry CachedResponse
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Export esporta il cache in formato JSON
func (sc *SemanticCache) Export() ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	data := make(map[string]*CachedResponse)
	for id, cached := range sc.responseCache {
		if !cached.IsExpired() {
			data[id] = cached
		}
	}

	return json.Marshal(data)
}

// Import importa il cache da formato JSON
func (sc *SemanticCache) Import(ctx context.Context, data []byte) error {
	var entries map[string]*CachedResponse
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	for id, entry := range entries {
		if !entry.IsExpired() {
			sc.responseCache[id] = entry
			_ = sc.vectorStore.Add(ctx, id, entry.Embedding, entry.Metadata)
		}
	}

	log.Info().
		Int("imported", len(entries)).
		Msg("Cache imported")

	return nil
}
