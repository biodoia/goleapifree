package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SemanticCache implementa un cache basato su similarity semantica
type SemanticCache struct {
	baseCache         Cache
	embeddings        map[string]*Embedding
	similarityIndex   *SimilarityIndex
	threshold         float64
	mu                sync.RWMutex
	vectorDB          VectorDB // Interfaccia per vector database (opzionale)
	useSimpleHash     bool
	stats             SemanticCacheStats
	embeddingProvider EmbeddingProvider
}

// SemanticCacheStats statistiche specifiche per semantic cache
type SemanticCacheStats struct {
	CacheStats
	SemanticHits       int64
	SemanticMisses     int64
	AverageSimilarity  float64
	EmbeddingCacheHits int64
}

// SemanticConfig configurazione per semantic cache
type SemanticConfig struct {
	BaseCache           Cache
	SimilarityThreshold float64 // Default: 0.95 (95% similarity)
	UseVectorDB         bool
	VectorDBEndpoint    string
	UseSimpleHash       bool // Fallback a hash semplice se non disponibile vector DB
	EmbeddingProvider   EmbeddingProvider
}

// Embedding rappresenta un embedding vettoriale
type Embedding struct {
	Vector    []float32
	Text      string
	Hash      string
	CreatedAt time.Time
}

// EmbeddingProvider interfaccia per generare embeddings
type EmbeddingProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// VectorDB interfaccia per vector database (Qdrant, Weaviate, etc.)
type VectorDB interface {
	Store(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	Search(ctx context.Context, vector []float32, limit int, threshold float64) ([]VectorSearchResult, error)
	Delete(ctx context.Context, id string) error
}

// VectorSearchResult risultato di una ricerca vettoriale
type VectorSearchResult struct {
	ID         string
	Score      float64
	Metadata   map[string]interface{}
	Vector     []float32
	Distance   float64
	Similarity float64
}

// SimilarityIndex mantiene un indice in-memory per similarity search
type SimilarityIndex struct {
	entries map[string]*IndexEntry
	mu      sync.RWMutex
}

// IndexEntry entry nell'indice di similarity
type IndexEntry struct {
	ID        string
	Embedding []float32
	CacheKey  string
	CreatedAt time.Time
}

// NewSemanticCache crea un nuovo semantic cache
func NewSemanticCache(config *SemanticConfig) (*SemanticCache, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = 0.95 // Default 95%
	}

	sc := &SemanticCache{
		baseCache:       config.BaseCache,
		embeddings:      make(map[string]*Embedding),
		similarityIndex: NewSimilarityIndex(),
		threshold:       config.SimilarityThreshold,
		useSimpleHash:   config.UseSimpleHash,
		stats:           SemanticCacheStats{},
		embeddingProvider: config.EmbeddingProvider,
	}

	// TODO: Inizializza vector DB se abilitato
	if config.UseVectorDB && config.VectorDBEndpoint != "" {
		log.Info().
			Str("endpoint", config.VectorDBEndpoint).
			Msg("Vector DB integration not yet implemented, using in-memory similarity")
		sc.useSimpleHash = true
	}

	log.Info().
		Float64("threshold", config.SimilarityThreshold).
		Bool("use_simple_hash", sc.useSimpleHash).
		Msg("Semantic cache initialized")

	return sc, nil
}

// Get cerca nel cache usando similarity semantica
func (s *SemanticCache) Get(ctx context.Context, prompt string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Modalità 1: Simple hash fallback
	if s.useSimpleHash {
		return s.getByHash(ctx, prompt)
	}

	// Modalità 2: Semantic similarity usando embeddings
	return s.getBySemanticSimilarity(ctx, prompt)
}

// getByHash cerca usando hash semplice del prompt
func (s *SemanticCache) getByHash(ctx context.Context, prompt string) ([]byte, error) {
	key := s.hashPrompt(prompt)
	data, err := s.baseCache.Get(ctx, key)

	if err == nil {
		s.stats.SemanticHits++
		s.stats.Hits++
		log.Debug().
			Str("key", key).
			Str("method", "hash").
			Msg("Semantic cache hit (hash)")
		return data, nil
	}

	s.stats.SemanticMisses++
	s.stats.Misses++
	return nil, ErrCacheMiss
}

// getBySemanticSimilarity cerca usando similarity semantica
func (s *SemanticCache) getBySemanticSimilarity(ctx context.Context, prompt string) ([]byte, error) {
	// Genera embedding per il prompt
	embedding, err := s.getOrCreateEmbedding(ctx, prompt)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get embedding, falling back to hash")
		return s.getByHash(ctx, prompt)
	}

	// Cerca entry simili
	similar := s.similarityIndex.FindSimilar(embedding.Vector, s.threshold)
	if len(similar) > 0 {
		// Usa la più simile
		best := similar[0]
		data, err := s.baseCache.Get(ctx, best.Entry.CacheKey)
		if err == nil {
			s.stats.SemanticHits++
			s.stats.Hits++
			s.stats.AverageSimilarity = (s.stats.AverageSimilarity + best.Similarity) / 2

			log.Debug().
				Str("key", best.Entry.CacheKey).
				Float64("similarity", best.Similarity).
				Str("method", "semantic").
				Msg("Semantic cache hit")

			return data, nil
		}
	}

	s.stats.SemanticMisses++
	s.stats.Misses++
	return nil, ErrCacheMiss
}

// Set salva nel cache con metadata semantica
func (s *SemanticCache) Set(ctx context.Context, prompt string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.Sets++

	var cacheKey string

	// Modalità hash semplice
	if s.useSimpleHash {
		cacheKey = s.hashPrompt(prompt)
		return s.baseCache.Set(ctx, cacheKey, value, ttl)
	}

	// Modalità semantica
	embedding, err := s.getOrCreateEmbedding(ctx, prompt)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create embedding, using hash fallback")
		cacheKey = s.hashPrompt(prompt)
	} else {
		cacheKey = embedding.Hash

		// Aggiungi all'indice di similarity
		entry := &IndexEntry{
			ID:        embedding.Hash,
			Embedding: embedding.Vector,
			CacheKey:  cacheKey,
			CreatedAt: time.Now(),
		}
		s.similarityIndex.Add(entry)
	}

	// Salva nel base cache
	return s.baseCache.Set(ctx, cacheKey, value, ttl)
}

// Delete rimuove un valore dal cache
func (s *SemanticCache) Delete(ctx context.Context, prompt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.hashPrompt(prompt)

	// Rimuovi dall'indice se presente
	if emb, ok := s.embeddings[key]; ok {
		s.similarityIndex.Remove(emb.Hash)
		delete(s.embeddings, key)
	}

	return s.baseCache.Delete(ctx, key)
}

// Clear svuota il cache
func (s *SemanticCache) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.embeddings = make(map[string]*Embedding)
	s.similarityIndex = NewSimilarityIndex()

	return s.baseCache.Clear(ctx)
}

// Stats restituisce le statistiche
func (s *SemanticCache) Stats() CacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats.CacheStats
}

// SemanticStats restituisce statistiche estese
func (s *SemanticCache) SemanticStats() SemanticCacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// getOrCreateEmbedding ottiene o crea un embedding per il testo
func (s *SemanticCache) getOrCreateEmbedding(ctx context.Context, text string) (*Embedding, error) {
	hash := s.hashPrompt(text)

	// Check cache
	if emb, ok := s.embeddings[hash]; ok {
		s.stats.EmbeddingCacheHits++
		return emb, nil
	}

	// Genera nuovo embedding
	if s.embeddingProvider == nil {
		// Usa embedding semplificato (placeholder)
		vector := s.simpleEmbedding(text)
		emb := &Embedding{
			Vector:    vector,
			Text:      text,
			Hash:      hash,
			CreatedAt: time.Now(),
		}
		s.embeddings[hash] = emb
		return emb, nil
	}

	// Usa provider esterno per embedding
	vector, err := s.embeddingProvider.GetEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	emb := &Embedding{
		Vector:    vector,
		Text:      text,
		Hash:      hash,
		CreatedAt: time.Now(),
	}
	s.embeddings[hash] = emb

	return emb, nil
}

// simpleEmbedding crea un embedding semplificato basato su features testuali
// NOTA: Questo è un placeholder - per produzione usare modelli reali
func (s *SemanticCache) simpleEmbedding(text string) []float32 {
	// Estrai features semplici
	words := strings.Fields(strings.ToLower(text))
	wordCount := float32(len(words))
	avgWordLen := float32(0)
	if wordCount > 0 {
		totalLen := 0
		for _, w := range words {
			totalLen += len(w)
		}
		avgWordLen = float32(totalLen) / wordCount
	}

	// Crea un vettore di dimensione fissa con features
	vector := make([]float32, 128) // Dimensione embedding
	vector[0] = wordCount / 100.0  // Normalizza
	vector[1] = avgWordLen / 10.0
	vector[2] = float32(len(text)) / 1000.0

	// Aggiungi hash dei primi caratteri
	for i, char := range text {
		if i >= 125 {
			break
		}
		vector[i+3] = float32(char%256) / 256.0
	}

	return vector
}

// hashPrompt genera un hash del prompt
func (s *SemanticCache) hashPrompt(prompt string) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	return "sem:" + hex.EncodeToString(h.Sum(nil))
}

// NewSimilarityIndex crea un nuovo indice di similarity
func NewSimilarityIndex() *SimilarityIndex {
	return &SimilarityIndex{
		entries: make(map[string]*IndexEntry),
	}
}

// Add aggiunge un'entry all'indice
func (si *SimilarityIndex) Add(entry *IndexEntry) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.entries[entry.ID] = entry
}

// Remove rimuove un'entry dall'indice
func (si *SimilarityIndex) Remove(id string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	delete(si.entries, id)
}

// FindSimilar trova entry simili sopra la threshold
func (si *SimilarityIndex) FindSimilar(vector []float32, threshold float64) []SimilarResult {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var results []SimilarResult

	for _, entry := range si.entries {
		similarity := CosineSimilarity(vector, entry.Embedding)
		if similarity >= threshold {
			results = append(results, SimilarResult{
				Entry:      entry,
				Similarity: similarity,
			})
		}
	}

	// Ordina per similarity decrescente
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// SimilarResult risultato di similarity search
type SimilarResult struct {
	Entry      *IndexEntry
	Similarity float64
}

// CosineSimilarity calcola la cosine similarity tra due vettori
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SimpleEmbeddingProvider implementazione semplice per testing
type SimpleEmbeddingProvider struct{}

// GetEmbedding genera un embedding semplice
func (p *SimpleEmbeddingProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Implementazione placeholder
	words := strings.Fields(strings.ToLower(text))
	vector := make([]float32, 128)

	for i, word := range words {
		if i >= 128 {
			break
		}
		hash := sha256.Sum256([]byte(word))
		vector[i] = float32(hash[0]) / 255.0
	}

	return vector, nil
}

// NormalizePrompt normalizza un prompt per comparazione
func NormalizePrompt(prompt string) string {
	// Rimuovi whitespace extra
	normalized := strings.TrimSpace(prompt)
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Converti in lowercase
	normalized = strings.ToLower(normalized)

	return normalized
}

// PromptFingerprint genera un fingerprint strutturato del prompt
func PromptFingerprint(prompt string) map[string]interface{} {
	words := strings.Fields(strings.ToLower(prompt))

	fingerprint := map[string]interface{}{
		"word_count":    len(words),
		"char_count":    len(prompt),
		"avg_word_len":  0.0,
		"has_question":  strings.Contains(prompt, "?"),
		"has_code":      strings.Contains(prompt, "```"),
		"first_words":   []string{},
		"normalized":    NormalizePrompt(prompt),
	}

	if len(words) > 0 {
		totalLen := 0
		for _, w := range words {
			totalLen += len(w)
		}
		fingerprint["avg_word_len"] = float64(totalLen) / float64(len(words))

		// Prime 5 parole come feature
		maxWords := 5
		if len(words) < maxWords {
			maxWords = len(words)
		}
		fingerprint["first_words"] = words[:maxWords]
	}

	return fingerprint
}

// SerializeFingerprint serializza un fingerprint per storage
func SerializeFingerprint(fp map[string]interface{}) (string, error) {
	data, err := json.Marshal(fp)
	if err != nil {
		return "", fmt.Errorf("failed to serialize fingerprint: %w", err)
	}
	return string(data), nil
}
