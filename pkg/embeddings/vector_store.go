package embeddings

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// VectorStore interfaccia per storage di vettori
type VectorStore interface {
	Add(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	Search(ctx context.Context, query []float32, k int, threshold float64) ([]SearchResult, error)
	Get(ctx context.Context, id string) (*VectorEntry, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	Clear(ctx context.Context) error
	Size() int
	Stats() VectorStoreStats
}

// SearchResult rappresenta un risultato di ricerca vettoriale
type SearchResult struct {
	ID         string
	Vector     []float32
	Metadata   map[string]interface{}
	Similarity float64
	Distance   float64
}

// VectorEntry rappresenta un entry nel vector store
type VectorEntry struct {
	ID        string
	Vector    []float32
	Metadata  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
	Hits      int64
}

// VectorStoreStats statistiche del vector store
type VectorStoreStats struct {
	TotalVectors  int
	TotalDims     int
	Searches      int64
	Additions     int64
	Deletions     int64
	AvgSearchTime time.Duration
}

// InMemoryVectorStore implementazione in-memory del vector store
type InMemoryVectorStore struct {
	vectors      map[string]*VectorEntry
	mu           sync.RWMutex
	metric       SimilarityMetric
	metricType   MetricType
	dimensions   int
	stats        VectorStoreStats
	searchTimes  []time.Duration
	maxSearchLog int
}

// VectorStoreConfig configurazione per il vector store
type VectorStoreConfig struct {
	MetricType   MetricType
	Dimensions   int
	MaxSearchLog int // numero massimo di search times da tenere per avg
}

// DefaultVectorStoreConfig restituisce configurazione di default
func DefaultVectorStoreConfig() *VectorStoreConfig {
	return &VectorStoreConfig{
		MetricType:   MetricCosine,
		Dimensions:   384, // Cohere embed-english-light-v3.0
		MaxSearchLog: 100,
	}
}

// NewInMemoryVectorStore crea un nuovo vector store in-memory
func NewInMemoryVectorStore(config *VectorStoreConfig) *InMemoryVectorStore {
	if config == nil {
		config = DefaultVectorStoreConfig()
	}

	return &InMemoryVectorStore{
		vectors:      make(map[string]*VectorEntry),
		metric:       GetMetric(config.MetricType),
		metricType:   config.MetricType,
		dimensions:   config.Dimensions,
		searchTimes:  make([]time.Duration, 0, config.MaxSearchLog),
		maxSearchLog: config.MaxSearchLog,
	}
}

// Add aggiunge un vettore al store
func (s *InMemoryVectorStore) Add(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	if len(vector) != s.dimensions {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimensions, len(vector))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if existing, ok := s.vectors[id]; ok {
		// Update existing
		existing.Vector = vector
		existing.Metadata = metadata
		existing.UpdatedAt = now
	} else {
		// Add new
		s.vectors[id] = &VectorEntry{
			ID:        id,
			Vector:    vector,
			Metadata:  metadata,
			CreatedAt: now,
			UpdatedAt: now,
			Hits:      0,
		}
		s.stats.Additions++
	}

	log.Debug().
		Str("id", id).
		Int("dimensions", len(vector)).
		Msg("Vector added to store")

	return nil
}

// Search cerca i k vettori più simili al query vector
func (s *InMemoryVectorStore) Search(ctx context.Context, query []float32, k int, threshold float64) ([]SearchResult, error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		s.mu.Lock()
		s.stats.Searches++
		if len(s.searchTimes) >= s.maxSearchLog {
			// Remove oldest
			s.searchTimes = s.searchTimes[1:]
		}
		s.searchTimes = append(s.searchTimes, elapsed)
		s.mu.Unlock()
	}()

	if len(query) != s.dimensions {
		return nil, fmt.Errorf("query dimension mismatch: expected %d, got %d", s.dimensions, len(query))
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.vectors) == 0 {
		return []SearchResult{}, nil
	}

	// Costruisci array di vettori e ID
	ids := make([]string, 0, len(s.vectors))
	vectors := make([][]float32, 0, len(s.vectors))
	entries := make([]*VectorEntry, 0, len(s.vectors))

	for id, entry := range s.vectors {
		ids = append(ids, id)
		vectors = append(vectors, entry.Vector)
		entries = append(entries, entry)
	}

	// Calcola similarità usando batch processing
	similarities := BatchCosineSimilarity(query, vectors)

	// Trova top K
	results := make([]SearchResult, 0, k)
	topIndices := make([]int, 0, k)

	// Selection sort parziale per trovare top K
	for i := 0; i < min(k, len(similarities)); i++ {
		maxIdx := -1
		maxSim := threshold

		for j, sim := range similarities {
			// Skip già selezionati
			skip := false
			for _, idx := range topIndices {
				if idx == j {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			if sim > maxSim {
				maxSim = sim
				maxIdx = j
			}
		}

		if maxIdx >= 0 {
			topIndices = append(topIndices, maxIdx)
			entry := entries[maxIdx]
			entry.Hits++ // Increment hit counter

			results = append(results, SearchResult{
				ID:         ids[maxIdx],
				Vector:     entry.Vector,
				Metadata:   entry.Metadata,
				Similarity: maxSim,
				Distance:   1.0 - maxSim,
			})
		}
	}

	log.Debug().
		Int("results", len(results)).
		Int("total", len(s.vectors)).
		Dur("duration", time.Since(start)).
		Msg("Vector search completed")

	return results, nil
}

// Get recupera un vettore per ID
func (s *InMemoryVectorStore) Get(ctx context.Context, id string) (*VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.vectors[id]
	if !ok {
		return nil, fmt.Errorf("vector not found: %s", id)
	}

	return entry, nil
}

// Delete rimuove un vettore dal store
func (s *InMemoryVectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.vectors[id]; !ok {
		return fmt.Errorf("vector not found: %s", id)
	}

	delete(s.vectors, id)
	s.stats.Deletions++

	log.Debug().Str("id", id).Msg("Vector deleted from store")
	return nil
}

// Update aggiorna un vettore esistente
func (s *InMemoryVectorStore) Update(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	if len(vector) != s.dimensions {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimensions, len(vector))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.vectors[id]
	if !ok {
		return fmt.Errorf("vector not found: %s", id)
	}

	entry.Vector = vector
	entry.Metadata = metadata
	entry.UpdatedAt = time.Now()

	log.Debug().Str("id", id).Msg("Vector updated")
	return nil
}

// Clear svuota il vector store
func (s *InMemoryVectorStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vectors = make(map[string]*VectorEntry)
	s.searchTimes = make([]time.Duration, 0, s.maxSearchLog)

	log.Info().Msg("Vector store cleared")
	return nil
}

// Size restituisce il numero di vettori nel store
func (s *InMemoryVectorStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vectors)
}

// Stats restituisce statistiche del vector store
func (s *InMemoryVectorStore) Stats() VectorStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.stats
	stats.TotalVectors = len(s.vectors)
	stats.TotalDims = s.dimensions

	// Calcola tempo medio di search
	if len(s.searchTimes) > 0 {
		var total time.Duration
		for _, t := range s.searchTimes {
			total += t
		}
		stats.AvgSearchTime = total / time.Duration(len(s.searchTimes))
	}

	return stats
}

// GetAllVectors restituisce tutti i vettori (utile per debug/export)
func (s *InMemoryVectorStore) GetAllVectors() map[string]*VectorEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copia per evitare race conditions
	result := make(map[string]*VectorEntry, len(s.vectors))
	for id, entry := range s.vectors {
		// Deep copy dell'entry
		entryCopy := *entry
		vectorCopy := make([]float32, len(entry.Vector))
		copy(vectorCopy, entry.Vector)
		entryCopy.Vector = vectorCopy

		if entry.Metadata != nil {
			metadataCopy := make(map[string]interface{})
			for k, v := range entry.Metadata {
				metadataCopy[k] = v
			}
			entryCopy.Metadata = metadataCopy
		}

		result[id] = &entryCopy
	}

	return result
}

// BatchAdd aggiunge multipli vettori in un'unica operazione
func (s *InMemoryVectorStore) BatchAdd(ctx context.Context, entries map[string]struct {
	Vector   []float32
	Metadata map[string]interface{}
}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, data := range entries {
		if len(data.Vector) != s.dimensions {
			return fmt.Errorf("vector dimension mismatch for %s: expected %d, got %d", id, s.dimensions, len(data.Vector))
		}

		if existing, ok := s.vectors[id]; ok {
			existing.Vector = data.Vector
			existing.Metadata = data.Metadata
			existing.UpdatedAt = now
		} else {
			s.vectors[id] = &VectorEntry{
				ID:        id,
				Vector:    data.Vector,
				Metadata:  data.Metadata,
				CreatedAt: now,
				UpdatedAt: now,
				Hits:      0,
			}
			s.stats.Additions++
		}
	}

	log.Debug().
		Int("count", len(entries)).
		Msg("Batch vectors added to store")

	return nil
}

// SearchWithFilter cerca vettori con filtri sui metadata
func (s *InMemoryVectorStore) SearchWithFilter(
	ctx context.Context,
	query []float32,
	k int,
	threshold float64,
	filter func(metadata map[string]interface{}) bool,
) ([]SearchResult, error) {
	if len(query) != s.dimensions {
		return nil, fmt.Errorf("query dimension mismatch: expected %d, got %d", s.dimensions, len(query))
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filtra vettori
	ids := make([]string, 0)
	vectors := make([][]float32, 0)
	entries := make([]*VectorEntry, 0)

	for id, entry := range s.vectors {
		if filter == nil || filter(entry.Metadata) {
			ids = append(ids, id)
			vectors = append(vectors, entry.Vector)
			entries = append(entries, entry)
		}
	}

	if len(vectors) == 0 {
		return []SearchResult{}, nil
	}

	// Calcola similarità
	similarities := BatchCosineSimilarity(query, vectors)

	// Trova top K
	results := make([]SearchResult, 0, k)
	topIndices := make([]int, 0, k)

	for i := 0; i < min(k, len(similarities)); i++ {
		maxIdx := -1
		maxSim := threshold

		for j, sim := range similarities {
			skip := false
			for _, idx := range topIndices {
				if idx == j {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			if sim > maxSim {
				maxSim = sim
				maxIdx = j
			}
		}

		if maxIdx >= 0 {
			topIndices = append(topIndices, maxIdx)
			entry := entries[maxIdx]

			results = append(results, SearchResult{
				ID:         ids[maxIdx],
				Vector:     entry.Vector,
				Metadata:   entry.Metadata,
				Similarity: maxSim,
				Distance:   1.0 - maxSim,
			})
		}
	}

	return results, nil
}
