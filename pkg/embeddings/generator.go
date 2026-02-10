package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EmbeddingGenerator genera embeddings per testi
type EmbeddingGenerator interface {
	Generate(ctx context.Context, text string) ([]float32, error)
	GenerateBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

// GeneratorConfig configurazione per il generatore di embeddings
type GeneratorConfig struct {
	Provider   string        // "cohere", "openai", "huggingface"
	APIKey     string        // API key del provider
	Model      string        // Nome del modello
	Timeout    time.Duration // Timeout per le richieste
	MaxRetries int           // Numero massimo di retry
	BatchSize  int           // Dimensione batch per batch processing
}

// DefaultGeneratorConfig restituisce una configurazione di default
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		Provider:   "cohere",
		Model:      "embed-english-light-v3.0",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		BatchSize:  96,
	}
}

// CachedGenerator wrapper che aggiunge caching a un generatore
type CachedGenerator struct {
	generator EmbeddingGenerator
	cache     map[string][]float32
	mu        sync.RWMutex
	hits      int64
	misses    int64
}

// NewCachedGenerator crea un nuovo generatore con cache
func NewCachedGenerator(generator EmbeddingGenerator) *CachedGenerator {
	return &CachedGenerator{
		generator: generator,
		cache:     make(map[string][]float32),
	}
}

// Generate genera un embedding con caching
func (c *CachedGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	c.mu.RLock()
	if embedding, ok := c.cache[text]; ok {
		c.hits++
		c.mu.RUnlock()
		log.Debug().Str("text", text[:min(50, len(text))]).Msg("Embedding cache hit")
		return embedding, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.misses++
	c.mu.Unlock()

	embedding, err := c.generator.Generate(ctx, text)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[text] = embedding
	c.mu.Unlock()

	log.Debug().Str("text", text[:min(50, len(text))]).Msg("Embedding generated and cached")
	return embedding, nil
}

// GenerateBatch genera embeddings per un batch di testi
func (c *CachedGenerator) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)

	// Controlla cache
	c.mu.RLock()
	for i, text := range texts {
		if embedding, ok := c.cache[text]; ok {
			results[i] = embedding
			c.hits++
		} else {
			uncachedIndices = append(uncachedIndices, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}
	c.mu.RUnlock()

	// Genera embeddings per testi non cachati
	if len(uncachedTexts) > 0 {
		c.mu.Lock()
		c.misses += int64(len(uncachedTexts))
		c.mu.Unlock()

		embeddings, err := c.generator.GenerateBatch(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		// Salva in cache e riempi risultati
		c.mu.Lock()
		for i, embedding := range embeddings {
			idx := uncachedIndices[i]
			results[idx] = embedding
			c.cache[uncachedTexts[i]] = embedding
		}
		c.mu.Unlock()
	}

	log.Debug().
		Int("total", len(texts)).
		Int("cached", len(texts)-len(uncachedTexts)).
		Int("generated", len(uncachedTexts)).
		Msg("Batch embeddings generated")

	return results, nil
}

// Dimensions restituisce la dimensione degli embeddings
func (c *CachedGenerator) Dimensions() int {
	return c.generator.Dimensions()
}

// ModelName restituisce il nome del modello
func (c *CachedGenerator) ModelName() string {
	return c.generator.ModelName()
}

// CacheStats restituisce statistiche sul cache
func (c *CachedGenerator) CacheStats() (hits, misses int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.cache)
}

// ClearCache svuota il cache
func (c *CachedGenerator) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float32)
	log.Info().Msg("Embedding cache cleared")
}

// CohereGenerator implementa generatore per Cohere
type CohereGenerator struct {
	config *GeneratorConfig
	client *http.Client
}

// NewCohereGenerator crea un nuovo generatore Cohere
func NewCohereGenerator(config *GeneratorConfig) *CohereGenerator {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	return &CohereGenerator{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Generate genera un embedding usando Cohere API
func (c *CohereGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.GenerateBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// GenerateBatch genera embeddings per un batch di testi
func (c *CohereGenerator) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts array")
	}

	requestBody := map[string]interface{}{
		"texts":      texts,
		"model":      c.config.Model,
		"input_type": "search_query",
		"truncate":   "END",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.cohere.ai/v1/embed", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	var lastErr error

	// Retry logic
	for i := 0; i < c.config.MaxRetries; i++ {
		resp, err = c.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		if i < c.config.MaxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed after %d retries: %w", c.config.MaxRetries, lastErr)
	}
	defer resp.Body.Close()

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("unexpected number of embeddings: got %d, expected %d", len(result.Embeddings), len(texts))
	}

	log.Debug().
		Int("count", len(texts)).
		Str("model", c.config.Model).
		Msg("Generated embeddings via Cohere")

	return result.Embeddings, nil
}

// Dimensions restituisce la dimensione degli embeddings
func (c *CohereGenerator) Dimensions() int {
	// Cohere embed-english-light-v3.0 genera embeddings di 384 dimensioni
	// embed-english-v3.0 genera 1024 dimensioni
	if c.config.Model == "embed-english-light-v3.0" {
		return 384
	}
	return 1024
}

// ModelName restituisce il nome del modello
func (c *CohereGenerator) ModelName() string {
	return c.config.Model
}

// OpenAIGenerator implementa generatore per OpenAI
type OpenAIGenerator struct {
	config *GeneratorConfig
	client *http.Client
}

// NewOpenAIGenerator crea un nuovo generatore OpenAI
func NewOpenAIGenerator(config *GeneratorConfig) *OpenAIGenerator {
	if config == nil {
		config = DefaultGeneratorConfig()
		config.Provider = "openai"
		config.Model = "text-embedding-3-small"
	}

	return &OpenAIGenerator{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Generate genera un embedding usando OpenAI API
func (o *OpenAIGenerator) Generate(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := o.GenerateBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// GenerateBatch genera embeddings per un batch di testi
func (o *OpenAIGenerator) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts array")
	}

	requestBody := map[string]interface{}{
		"input": texts,
		"model": o.config.Model,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range result.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	log.Debug().
		Int("count", len(texts)).
		Str("model", o.config.Model).
		Msg("Generated embeddings via OpenAI")

	return embeddings, nil
}

// Dimensions restituisce la dimensione degli embeddings
func (o *OpenAIGenerator) Dimensions() int {
	// text-embedding-3-small: 1536 dimensioni
	// text-embedding-3-large: 3072 dimensioni
	if o.config.Model == "text-embedding-3-large" {
		return 3072
	}
	return 1536
}

// ModelName restituisce il nome del modello
func (o *OpenAIGenerator) ModelName() string {
	return o.config.Model
}

// NewGenerator crea un nuovo generatore basato sulla configurazione
func NewGenerator(config *GeneratorConfig) (EmbeddingGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	var generator EmbeddingGenerator

	switch config.Provider {
	case "cohere":
		generator = NewCohereGenerator(config)
	case "openai":
		generator = NewOpenAIGenerator(config)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	// Wrappa con cache
	return NewCachedGenerator(generator), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
