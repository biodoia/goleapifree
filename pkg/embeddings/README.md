# Embeddings & Semantic Caching

Sistema completo di embedding generation e vector search per semantic caching nel progetto goleapifree.

## Caratteristiche

- **Embedding Generation**: Supporto per multiple API (Cohere, OpenAI)
- **Vector Storage**: In-memory vector store con cosine similarity search
- **Semantic Cache**: Cache intelligente basato su similarità semantica
- **Cache Middleware**: Integrazione con il sistema di caching esistente
- **Performance Optimization**: Batch processing, caching, operazioni ottimizzate

## Architettura

```
embeddings/
├── generator.go         - Generazione embeddings (Cohere, OpenAI)
├── vector_store.go      - Storage e ricerca vettoriale in-memory
├── semantic_cache.go    - Semantic caching con TTL e cleanup
├── similarity.go        - Calcoli di similarità (cosine, euclidean, etc.)
├── cache_middleware.go  - Middleware per integrazione
└── example_test.go      - Test ed esempi di utilizzo
```

## Quick Start

### 1. Embedding Generator

```go
import "github.com/biodoia/goleapifree/pkg/embeddings"

// Configurazione per Cohere
config := &embeddings.GeneratorConfig{
    Provider:   "cohere",
    APIKey:     "your-api-key",
    Model:      "embed-english-light-v3.0",
    Timeout:    30 * time.Second,
    MaxRetries: 3,
    BatchSize:  96,
}

// Crea generatore (automaticamente wrapped con cache)
generator, err := embeddings.NewGenerator(config)
if err != nil {
    log.Fatal(err)
}

// Genera embedding singolo
ctx := context.Background()
embedding, err := generator.Generate(ctx, "Hello world")

// Batch generation
texts := []string{"Text 1", "Text 2", "Text 3"}
embeddings, err := generator.GenerateBatch(ctx, texts)
```

### 2. Vector Store

```go
// Configurazione vector store
config := &embeddings.VectorStoreConfig{
    MetricType:   embeddings.MetricCosine,
    Dimensions:   384,  // Cohere embed-english-light-v3.0
    MaxSearchLog: 100,
}

store := embeddings.NewInMemoryVectorStore(config)

// Aggiungi vettori
ctx := context.Background()
store.Add(ctx, "doc1", embedding1, map[string]interface{}{
    "type": "question",
    "user": "john",
})

// Cerca K nearest neighbors
query := embedding  // vettore query
k := 10            // top 10 risultati
threshold := 0.8   // similarità minima

results, err := store.Search(ctx, query, k, threshold)
for _, result := range results {
    fmt.Printf("ID: %s, Similarity: %.2f\n", result.ID, result.Similarity)
}

// Ricerca con filtri sui metadata
results, err := store.SearchWithFilter(ctx, query, k, threshold, func(metadata map[string]interface{}) bool {
    return metadata["type"] == "question"
})
```

### 3. Semantic Cache

```go
// Configurazione semantic cache
config := &embeddings.SemanticCacheConfig{
    SimilarityThreshold: 0.9,              // Match se similarity > 0.9
    DefaultTTL:          10 * time.Minute, // TTL di default
    MaxCacheSize:        1000,             // Massimo 1000 items
    CleanupInterval:     5 * time.Minute,  // Cleanup ogni 5 min
    EnableAutoCleanup:   true,
}

cache, err := embeddings.NewSemanticCache(generator, config)
if err != nil {
    log.Fatal(err)
}

// Salva risposta
prompt := "What is the capital of France?"
response := "The capital of France is Paris."
cache.Set(ctx, prompt, response, 10*time.Minute)

// Recupera risposta (anche per prompts simili!)
prompt2 := "What's the capital city of France?"
result, found, err := cache.Get(ctx, prompt2)
if found {
    fmt.Printf("Cached response: %v\n", result)
}

// Trova prompts simili
similar, err := cache.FindSimilar(ctx, "France capital", 5)
for _, s := range similar {
    fmt.Printf("Prompt: %s, Similarity: %.2f\n", s.Prompt, s.Similarity)
}

// Statistiche
stats := cache.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate()*100)
fmt.Printf("Total queries: %d\n", stats.TotalQueries)
```

### 4. Cache Middleware

```go
// Setup middleware
middleware := embeddings.NewCacheMiddleware(cache, nil)

// GetOrCompute pattern
result, err := middleware.GetOrCompute(
    ctx,
    "Explain quantum computing",
    10*time.Minute,
    func(ctx context.Context) (interface{}, error) {
        // Chiamata API costosa solo se cache miss
        return callExpensiveAPI(ctx)
    },
)

// Deduplication di batch requests
requests := []*embeddings.CompletionRequest{
    {Prompt: "What is AI?", Model: "gpt-3.5"},
    {Prompt: "What is artificial intelligence?", Model: "gpt-3.5"},
    {Prompt: "Explain AI", Model: "gpt-3.5"},
}

// Deduplica richieste semanticamente simili (threshold 0.9)
groups, err := middleware.DeduplicateRequests(ctx, requests, 0.9)
fmt.Printf("Reduced %d requests to %d unique groups\n", len(requests), len(groups))

// Invalida cache entries simili
count, err := middleware.InvalidateSimilar(ctx, "AI explanation", 0.85)
fmt.Printf("Invalidated %d similar entries\n", count)
```

## Similarity Metrics

### Cosine Similarity (default)
```go
similarity := embeddings.CosineSimilarity(vec1, vec2)
// Restituisce valore tra -1 e 1 (1 = identici)
```

### Euclidean Distance
```go
distance := embeddings.EuclideanDistance(vec1, vec2)
// Minore = più simili
```

### Dot Product
```go
dotProduct := embeddings.DotProduct(vec1, vec2)
```

### Batch Processing (ottimizzato)
```go
query := embedding
vectors := [][]float32{vec1, vec2, vec3}
similarities := embeddings.BatchCosineSimilarity(query, vectors)
```

## Provider Support

### Cohere (Free Tier)
```go
config := &embeddings.GeneratorConfig{
    Provider: "cohere",
    APIKey:   os.Getenv("COHERE_API_KEY"),
    Model:    "embed-english-light-v3.0", // 384 dims
    // o "embed-english-v3.0"              // 1024 dims
}
```

### OpenAI
```go
config := &embeddings.GeneratorConfig{
    Provider: "openai",
    APIKey:   os.Getenv("OPENAI_API_KEY"),
    Model:    "text-embedding-3-small", // 1536 dims
    // o "text-embedding-3-large"         // 3072 dims
}
```

## Performance

### Batch Processing
```go
// Evita chiamate multiple - usa batch
texts := []string{"text1", "text2", "text3"}
embeddings, err := generator.GenerateBatch(ctx, texts)

// Invece di:
// for _, text := range texts {
//     emb, _ := generator.Generate(ctx, text)
// }
```

### Caching
Il generatore è automaticamente wrapped con un cache layer:
```go
generator, _ := embeddings.NewGenerator(config)
// Successive chiamate con stesso testo usano cache

// Statistiche cache
if cached, ok := generator.(*embeddings.CachedGenerator); ok {
    hits, misses, size := cached.CacheStats()
    fmt.Printf("Cache hits: %d, misses: %d, size: %d\n", hits, misses, size)
}
```

### Vector Search Optimization
```go
// Usa BatchCosineSimilarity per ricerche su grandi dataset
similarities := embeddings.BatchCosineSimilarity(query, vectors)

// TopKSimilar usa partial sorting per efficienza
results := embeddings.TopKSimilar(query, vectors, k, metric)
```

## Use Cases

### 1. Duplicate Request Detection
```go
// Rileva richieste duplicate semanticamente
deduplicator := embeddings.NewSemanticDeduplication(middleware, 0.9)
responses, err := deduplicator.Process(ctx, requests)
// Riduce costi API processando solo richieste uniche
```

### 2. Response Caching
```go
// Cache risposte per prompts simili
cache.Set(ctx, "Explain neural networks", response, 1*time.Hour)
// Successivi prompts simili usano cache:
// "What are neural networks?"
// "Tell me about neural nets"
```

### 3. Smart Rate Limiting
```go
// Identifica utenti che fanno domande simili ripetutamente
similar, _ := cache.FindSimilar(ctx, userPrompt, 10)
if len(similar) > 5 {
    // Potenziale abuse - rate limit
}
```

### 4. Analytics
```go
// Trova prompts più popolari
topHits := cache.GetTopHits(10)
for _, item := range topHits {
    fmt.Printf("Prompt: %s, Hits: %d\n", item.Prompt, item.Hits)
}

// Statistiche
stats := cache.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate()*100)
```

## Configuration Best Practices

### Similarity Threshold
- **0.95-1.0**: Quasi identici (strict matching)
- **0.85-0.94**: Molto simili (recommended per cache)
- **0.70-0.84**: Simili (per deduplication)
- **< 0.70**: Potenzialmente non correlati

### TTL Settings
```go
// Risposte stabili (fatti, definizioni)
ttl := 1 * time.Hour

// Risposte time-sensitive (news, prezzi)
ttl := 5 * time.Minute

// Risposte computazionalmente costose
ttl := 24 * time.Hour
```

### Cache Size
```go
config.MaxCacheSize = 1000  // Small deployment
config.MaxCacheSize = 10000 // Medium deployment
config.MaxCacheSize = 50000 // Large deployment
```

## Monitoring

```go
// Semantic cache stats
stats := cache.Stats()
log.Printf("Hit rate: %.2f%%", stats.HitRate()*100)
log.Printf("Total queries: %d", stats.TotalQueries)
log.Printf("Avg lookup time: %v", stats.AvgLookupTime)

// Vector store stats
vstats := store.Stats()
log.Printf("Total vectors: %d", vstats.TotalVectors)
log.Printf("Avg search time: %v", vstats.AvgSearchTime)

// Generator cache stats
if cached, ok := generator.(*embeddings.CachedGenerator); ok {
    hits, misses, size := cached.CacheStats()
    hitRate := float64(hits) / float64(hits+misses) * 100
    log.Printf("Generator cache hit rate: %.2f%%", hitRate)
}
```

## Testing

```go
// Mock generator per testing
generator := embeddings.NewMockGenerator(384)

// Test semantic cache
cache, _ := embeddings.NewSemanticCache(generator, config)
// ... test logic
```

Vedi `example_test.go` per esempi completi e benchmark.

## Limitations

- **In-Memory Only**: Vector store attualmente solo in-memory (nessuna persistenza)
- **Linear Search**: Search è O(n) - considera soluzioni come FAISS per large scale
- **No Clustering**: Nessun clustering automatico dei vettori
- **Single Node**: Non distribuito (per multi-node usa Redis + embeddings separati)

## Future Improvements

- [ ] Persistent vector store (SQLite, PostgreSQL con pgvector)
- [ ] Approximate Nearest Neighbor (ANN) search (HNSW, IVF)
- [ ] Vector clustering e indexing
- [ ] Support per più provider (HuggingFace, Azure, etc.)
- [ ] Distributed semantic cache
- [ ] Compression degli embeddings
- [ ] Fine-tuning support

## Examples

Vedi `example_test.go` per esempi completi:
- `ExampleSemanticCache_BasicUsage`
- `ExampleCacheMiddleware_Deduplication`
- `ExampleVectorStore_Search`
- `ExampleSimilarity_Calculations`

## License

Parte del progetto goleapifree.
