package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
)

// BenchmarkCacheGet misura le performance di lettura
func BenchmarkCacheGet(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = c.Set(ctx, key, []byte("test value"), 5*time.Minute)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%1000)
		_, _ = c.Get(ctx, key)
	}
}

// BenchmarkCacheSet misura le performance di scrittura
func BenchmarkCacheSet(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()
	value := []byte("test value with some content")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = c.Set(ctx, key, value, 5*time.Minute)
	}
}

// BenchmarkCacheGetSet misura read/write mix
func BenchmarkCacheGetSet(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			// Write
			key := fmt.Sprintf("key-%d", i)
			_ = c.Set(ctx, key, []byte("value"), 5*time.Minute)
		} else {
			// Read
			key := fmt.Sprintf("key-%d", i-1)
			_, _ = c.Get(ctx, key)
		}
	}
}

// BenchmarkCacheConcurrentReads testa letture concorrenti
func BenchmarkCacheConcurrentReads(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = c.Set(ctx, key, []byte("test value"), 5*time.Minute)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			_, _ = c.Get(ctx, key)
			i++
		}
	})
}

// BenchmarkCacheConcurrentWrites testa scritture concorrenti
func BenchmarkCacheConcurrentWrites(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			_ = c.Set(ctx, key, value, 5*time.Minute)
			i++
		}
	})
}

// BenchmarkCacheHitRate misura l'impatto del hit rate
func BenchmarkCacheHitRate(b *testing.B) {
	hitRates := []float64{0.0, 0.5, 0.9, 0.99, 1.0}

	for _, hitRate := range hitRates {
		b.Run(fmt.Sprintf("HitRate_%.0f", hitRate*100), func(b *testing.B) {
			c := createTestCache()
			ctx := context.Background()

			// Pre-populate based on hit rate
			populateSize := int(float64(b.N) * hitRate)
			for i := 0; i < populateSize; i++ {
				key := fmt.Sprintf("key-%d", i)
				_ = c.Set(ctx, key, []byte("value"), 5*time.Minute)
			}

			var hits, misses int64

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%populateSize)
				_, err := c.Get(ctx, key)
				if err == nil {
					atomic.AddInt64(&hits, 1)
				} else {
					atomic.AddInt64(&misses, 1)
				}
			}

			actualHitRate := float64(hits) / float64(hits+misses) * 100
			b.ReportMetric(actualHitRate, "hit_rate_%")
		})
	}
}

// BenchmarkCacheValueSizes testa con diverse dimensioni di valori
func BenchmarkCacheValueSizes(b *testing.B) {
	sizes := []int{
		100,        // 100 bytes
		1024,       // 1 KB
		10240,      // 10 KB
		102400,     // 100 KB
		1024000,    // 1 MB
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			c := createTestCache()
			ctx := context.Background()
			value := make([]byte, size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				_ = c.Set(ctx, key, value, 5*time.Minute)
			}
		})
	}
}

// BenchmarkCacheEviction misura l'overhead dell'eviction
func BenchmarkCacheEviction(b *testing.B) {
	c := createTestCacheWithSize(1000) // Small cache to trigger eviction
	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = c.Set(ctx, key, value, 5*time.Minute)
	}

	stats := c.Stats()
	evictionRate := stats.EvictionRate
	b.ReportMetric(evictionRate*100, "eviction_rate_%")
}

// BenchmarkCacheTTL testa l'impatto del TTL
func BenchmarkCacheTTL(b *testing.B) {
	ttls := []time.Duration{
		1 * time.Second,
		1 * time.Minute,
		1 * time.Hour,
		24 * time.Hour,
	}

	for _, ttl := range ttls {
		b.Run(fmt.Sprintf("TTL_%s", ttl), func(b *testing.B) {
			c := createTestCache()
			ctx := context.Background()
			value := []byte("test value")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				_ = c.Set(ctx, key, value, ttl)
			}
		})
	}
}

// BenchmarkSemanticCache testa il semantic cache
func BenchmarkSemanticCache(b *testing.B) {
	sc := createTestSemanticCache()
	ctx := context.Background()

	prompts := []string{
		"What is the capital of France?",
		"What is the capital of Italy?",
		"What is the capital of Spain?",
		"Tell me about machine learning",
		"Explain neural networks",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		prompt := prompts[i%len(prompts)]

		// Try to get from cache
		_, err := sc.Get(ctx, prompt)
		if err != nil {
			// Cache miss, set new value
			_ = sc.Set(ctx, prompt, []byte("response"), 5*time.Minute)
		}
	}
}

// BenchmarkSemanticCacheSimilarity misura la ricerca di similarity
func BenchmarkSemanticCacheSimilarity(b *testing.B) {
	sc := createTestSemanticCache()
	ctx := context.Background()

	// Pre-populate with diverse prompts
	for i := 0; i < 100; i++ {
		prompt := fmt.Sprintf("This is test prompt number %d with some content", i)
		_ = sc.Set(ctx, prompt, []byte("response"), 5*time.Minute)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		prompt := fmt.Sprintf("This is test prompt number %d with some content", i%100)
		_, _ = sc.Get(ctx, prompt)
	}
}

// BenchmarkSemanticCacheEmbedding misura la generazione di embeddings
func BenchmarkSemanticCacheEmbedding(b *testing.B) {
	sc := createTestSemanticCache()
	ctx := context.Background()

	prompts := []string{
		"Short prompt",
		"Medium length prompt with more words and context",
		"Very long prompt with many words that describes a complex scenario in detail with multiple sentences and various topics",
	}

	for _, prompt := range prompts {
		b.Run(fmt.Sprintf("Length_%d", len(prompt)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = sc.Set(ctx, prompt, []byte("response"), 5*time.Minute)
			}
		})
	}
}

// BenchmarkCacheMemoryUsage misura il consumo di memoria
func BenchmarkCacheMemoryUsage(b *testing.B) {
	cacheSizes := []int{100, 1000, 10000, 100000}

	for _, size := range cacheSizes {
		b.Run(fmt.Sprintf("Entries_%d", size), func(b *testing.B) {
			c := createTestCacheWithSize(size)
			ctx := context.Background()
			value := []byte("test value with some content")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%size)
				_ = c.Set(ctx, key, value, 5*time.Minute)
			}

			stats := c.Stats()
			b.ReportMetric(float64(stats.Size), "bytes")
		})
	}
}

// BenchmarkMultiLayerCache testa il multi-layer cache
func BenchmarkMultiLayerCache(b *testing.B) {
	b.Run("MemoryOnly", func(b *testing.B) {
		c := createMemoryOnlyCache()
		benchmarkCacheOperations(b, c)
	})

	b.Run("MultiLayer", func(b *testing.B) {
		c := createMultiLayerCache()
		benchmarkCacheOperations(b, c)
	})
}

// BenchmarkCacheThroughput misura il throughput
func BenchmarkCacheThroughput(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()
	value := []byte("test value")

	var totalOps int64
	start := time.Now()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			if i%2 == 0 {
				_ = c.Set(ctx, key, value, 5*time.Minute)
			} else {
				_, _ = c.Get(ctx, key)
			}
			atomic.AddInt64(&totalOps, 1)
			i++
		}
	})

	elapsed := time.Since(start)
	throughput := float64(totalOps) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// BenchmarkCacheLatency misura la latency delle operazioni
func BenchmarkCacheLatency(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()
	value := []byte("test value")

	var totalLatency time.Duration
	var ops int64

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)

		start := time.Now()
		_ = c.Set(ctx, key, value, 5*time.Minute)
		setLatency := time.Since(start)

		start = time.Now()
		_, _ = c.Get(ctx, key)
		getLatency := time.Since(start)

		totalLatency += setLatency + getLatency
		ops += 2
	}

	avgLatency := totalLatency / time.Duration(ops)
	b.ReportMetric(float64(avgLatency.Microseconds()), "avg_latency_us")
}

// Helper functions

func createTestCache() cache.Cache {
	config := cache.DefaultConfig()
	config.MemoryEnabled = true
	config.RedisEnabled = false

	c, _ := cache.NewMultiLayerCache(config)
	return c
}

func createTestCacheWithSize(size int) cache.Cache {
	config := cache.DefaultConfig()
	config.MemoryEnabled = true
	config.MemoryMaxEntries = size
	config.RedisEnabled = false

	c, _ := cache.NewMultiLayerCache(config)
	return c
}

func createTestSemanticCache() *cache.SemanticCache {
	baseCache := createTestCache()

	config := &cache.SemanticConfig{
		BaseCache:           baseCache,
		SimilarityThreshold: 0.95,
		UseSimpleHash:       true,
		EmbeddingProvider:   &cache.SimpleEmbeddingProvider{},
	}

	sc, _ := cache.NewSemanticCache(config)
	return sc
}

func createMemoryOnlyCache() cache.Cache {
	config := cache.DefaultConfig()
	config.MemoryEnabled = true
	config.RedisEnabled = false

	c, _ := cache.NewMultiLayerCache(config)
	return c
}

func createMultiLayerCache() cache.Cache {
	config := cache.DefaultConfig()
	config.MemoryEnabled = true
	config.RedisEnabled = false // Set to true in production with Redis

	c, _ := cache.NewMultiLayerCache(config)
	return c
}

func benchmarkCacheOperations(b *testing.B, c cache.Cache) {
	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)

		// Set
		_ = c.Set(ctx, key, value, 5*time.Minute)

		// Get
		_, _ = c.Get(ctx, key)
	}
}

// Additional benchmarks for specific cache features

// BenchmarkCacheCompression testa con compressione
func BenchmarkCacheCompression(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()

	// Generate compressible data
	value := make([]byte, 10240)
	for i := range value {
		value[i] = byte(i % 10) // Highly compressible
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = c.Set(ctx, key, value, 5*time.Minute)
	}
}

// BenchmarkCacheBulkOperations testa operazioni bulk
func BenchmarkCacheBulkOperations(b *testing.B) {
	c := createTestCache()
	ctx := context.Background()
	value := []byte("test value")

	batchSizes := []int{10, 100, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Simulate bulk set
				for j := 0; j < batchSize; j++ {
					key := fmt.Sprintf("key-%d-%d", i, j)
					_ = c.Set(ctx, key, value, 5*time.Minute)
				}
			}
		})
	}
}
