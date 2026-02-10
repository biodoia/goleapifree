package embeddings

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockGenerator è un generatore di embeddings mock per testing
type MockGenerator struct {
	dimensions int
}

func NewMockGenerator(dimensions int) *MockGenerator {
	return &MockGenerator{dimensions: dimensions}
}

func (m *MockGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	// Genera embedding deterministico basato sul testo
	embedding := make([]float32, m.dimensions)
	hash := 0
	for _, c := range text {
		hash = (hash*31 + int(c)) % 1000
	}

	for i := range embedding {
		embedding[i] = float32((hash + i) % 100) / 100.0
	}

	return embedding, nil
}

func (m *MockGenerator) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Generate(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (m *MockGenerator) Dimensions() int {
	return m.dimensions
}

func (m *MockGenerator) ModelName() string {
	return "mock-generator"
}

// ExampleSemanticCache_BasicUsage dimostra l'uso base del semantic cache
func ExampleSemanticCache_BasicUsage() {
	ctx := context.Background()

	// Crea generatore mock
	generator := NewMockGenerator(384)

	// Crea semantic cache
	config := &SemanticCacheConfig{
		SimilarityThreshold: 0.9,
		DefaultTTL:          10 * time.Minute,
		MaxCacheSize:        100,
		EnableAutoCleanup:   false,
	}
	cache, _ := NewSemanticCache(generator, config)

	// Salva una risposta
	prompt1 := "What is the capital of France?"
	response1 := "The capital of France is Paris."
	cache.Set(ctx, prompt1, response1, 10*time.Minute)

	// Cerca con prompt identico
	result, found, _ := cache.Get(ctx, prompt1)
	fmt.Printf("Exact match found: %v\n", found)

	// Cerca con prompt simile (dovrebbe trovare match semantico)
	prompt2 := "What's the capital city of France?"
	result2, found2, _ := cache.Get(ctx, prompt2)
	fmt.Printf("Similar match found: %v\n", found2)
	if found2 {
		fmt.Printf("Response: %v\n", result2)
	}

	// Output:
	// Exact match found: true
}

// ExampleCacheMiddleware_Deduplication dimostra la deduplication semantica
func ExampleCacheMiddleware_Deduplication() {
	ctx := context.Background()

	// Setup
	generator := NewMockGenerator(384)
	semanticCache, _ := NewSemanticCache(generator, DefaultSemanticCacheConfig())
	middleware := NewCacheMiddleware(semanticCache, nil)

	// Crea richieste duplicate semanticamente
	requests := []*CompletionRequest{
		{Prompt: "What is AI?", Model: "gpt-3.5"},
		{Prompt: "What is artificial intelligence?", Model: "gpt-3.5"},
		{Prompt: "Explain AI", Model: "gpt-3.5"},
		{Prompt: "Tell me about Python", Model: "gpt-3.5"},
	}

	// Deduplica con threshold 0.9
	groups, _ := middleware.DeduplicateRequests(ctx, requests, 0.9)

	fmt.Printf("Original requests: %d\n", len(requests))
	fmt.Printf("Unique groups: %d\n", len(groups))

	// Output:
	// Original requests: 4
}

// ExampleVectorStore_Search dimostra la ricerca vettoriale
func ExampleVectorStore_Search() {
	ctx := context.Background()

	// Crea vector store
	config := DefaultVectorStoreConfig()
	store := NewInMemoryVectorStore(config)

	// Aggiungi vettori
	store.Add(ctx, "doc1", []float32{0.1, 0.2, 0.3}, map[string]interface{}{"type": "question"})
	store.Add(ctx, "doc2", []float32{0.15, 0.25, 0.35}, map[string]interface{}{"type": "answer"})
	store.Add(ctx, "doc3", []float32{0.9, 0.8, 0.7}, map[string]interface{}{"type": "unrelated"})

	// Cerca vettori simili
	query := []float32{0.12, 0.22, 0.32}
	results, _ := store.Search(ctx, query, 2, 0.8)

	fmt.Printf("Found %d results\n", len(results))
	for _, result := range results {
		fmt.Printf("ID: %s, Similarity: %.2f\n", result.ID, result.Similarity)
	}

	// Output:
	// Found results
}

// ExampleSimilarity_Calculations dimostra i calcoli di similarità
func ExampleSimilarity_Calculations() {
	vec1 := []float32{1.0, 0.0, 0.0}
	vec2 := []float32{0.9, 0.1, 0.0}
	vec3 := []float32{0.0, 1.0, 0.0}

	// Cosine similarity
	sim12 := CosineSimilarity(vec1, vec2)
	sim13 := CosineSimilarity(vec1, vec3)

	fmt.Printf("Similarity between similar vectors: %.2f\n", sim12)
	fmt.Printf("Similarity between orthogonal vectors: %.2f\n", sim13)

	// Output:
	// Similarity between similar vectors: 0.99
	// Similarity between orthogonal vectors: 0.00
}

// TestSemanticCache_Integration test di integrazione
func TestSemanticCache_Integration(t *testing.T) {
	ctx := context.Background()
	generator := NewMockGenerator(384)

	config := &SemanticCacheConfig{
		SimilarityThreshold: 0.85,
		DefaultTTL:          1 * time.Second,
		MaxCacheSize:        10,
		EnableAutoCleanup:   false,
	}

	cache, err := NewSemanticCache(generator, config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test 1: Set e Get
	prompt := "Hello world"
	response := "Hi there!"
	if err := cache.Set(ctx, prompt, response, 5*time.Second); err != nil {
		t.Errorf("Failed to set: %v", err)
	}

	result, found, err := cache.Get(ctx, prompt)
	if err != nil {
		t.Errorf("Failed to get: %v", err)
	}
	if !found {
		t.Errorf("Expected to find cached response")
	}
	if result != response {
		t.Errorf("Expected %v, got %v", response, result)
	}

	// Test 2: Miss
	_, found, _ = cache.Get(ctx, "completely different prompt")
	if found {
		t.Errorf("Should not find unrelated prompt")
	}

	// Test 3: Expiration
	cache.Set(ctx, "expire", "data", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	_, found, _ = cache.Get(ctx, "expire")
	if found {
		t.Errorf("Should not find expired item")
	}

	// Test 4: Stats
	stats := cache.Stats()
	if stats.TotalQueries < 2 {
		t.Errorf("Expected at least 2 queries, got %d", stats.TotalQueries)
	}
}

// TestVectorStore_Operations test delle operazioni del vector store
func TestVectorStore_Operations(t *testing.T) {
	ctx := context.Background()
	config := &VectorStoreConfig{
		MetricType: MetricCosine,
		Dimensions: 3,
	}
	store := NewInMemoryVectorStore(config)

	// Test Add
	vec1 := []float32{1.0, 0.0, 0.0}
	if err := store.Add(ctx, "v1", vec1, nil); err != nil {
		t.Errorf("Failed to add: %v", err)
	}

	// Test Get
	entry, err := store.Get(ctx, "v1")
	if err != nil {
		t.Errorf("Failed to get: %v", err)
	}
	if entry.ID != "v1" {
		t.Errorf("Expected ID v1, got %s", entry.ID)
	}

	// Test Search
	vec2 := []float32{0.9, 0.1, 0.0}
	store.Add(ctx, "v2", vec2, nil)

	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 2, 0.5)
	if err != nil {
		t.Errorf("Failed to search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Test Delete
	if err := store.Delete(ctx, "v1"); err != nil {
		t.Errorf("Failed to delete: %v", err)
	}

	if _, err := store.Get(ctx, "v1"); err == nil {
		t.Errorf("Should not find deleted vector")
	}

	// Test Clear
	if err := store.Clear(ctx); err != nil {
		t.Errorf("Failed to clear: %v", err)
	}
	if store.Size() != 0 {
		t.Errorf("Store should be empty, got size %d", store.Size())
	}
}

// TestCachedGenerator test del generatore con cache
func TestCachedGenerator(t *testing.T) {
	ctx := context.Background()
	baseGen := NewMockGenerator(384)
	cachedGen := NewCachedGenerator(baseGen)

	text := "test prompt"

	// First call - miss
	emb1, err := cachedGen.Generate(ctx, text)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	hits1, misses1, _ := cachedGen.CacheStats()

	// Second call - hit
	emb2, err := cachedGen.Generate(ctx, text)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	hits2, misses2, _ := cachedGen.CacheStats()

	if hits2 != hits1+1 {
		t.Errorf("Expected hits to increase by 1")
	}
	if misses2 != misses1 {
		t.Errorf("Expected misses to stay same")
	}

	// Embeddings should be identical
	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("Embeddings should be identical")
			break
		}
	}
}

// BenchmarkCosineSimilarity benchmark per cosine similarity
func BenchmarkCosineSimilarity(b *testing.B) {
	vec1 := make([]float32, 384)
	vec2 := make([]float32, 384)

	for i := range vec1 {
		vec1[i] = float32(i) / 384.0
		vec2[i] = float32(i+1) / 384.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineSimilarity(vec1, vec2)
	}
}

// BenchmarkBatchCosineSimilarity benchmark per batch cosine similarity
func BenchmarkBatchCosineSimilarity(b *testing.B) {
	query := make([]float32, 384)
	vectors := make([][]float32, 100)

	for i := range query {
		query[i] = float32(i) / 384.0
	}
	for i := range vectors {
		vectors[i] = make([]float32, 384)
		for j := range vectors[i] {
			vectors[i][j] = float32(j+i) / 384.0
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchCosineSimilarity(query, vectors)
	}
}

// BenchmarkVectorStoreSearch benchmark per vector store search
func BenchmarkVectorStoreSearch(b *testing.B) {
	ctx := context.Background()
	store := NewInMemoryVectorStore(DefaultVectorStoreConfig())

	// Popola con 1000 vettori
	for i := 0; i < 1000; i++ {
		vec := make([]float32, 384)
		for j := range vec {
			vec[j] = float32(i+j) / 1000.0
		}
		store.Add(ctx, fmt.Sprintf("v%d", i), vec, nil)
	}

	query := make([]float32, 384)
	for i := range query {
		query[i] = float32(i) / 384.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Search(ctx, query, 10, 0.8)
	}
}
