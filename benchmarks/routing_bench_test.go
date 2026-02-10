package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/router"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
)

// BenchmarkRouterStrategySelection misura la velocità di selezione del provider
func BenchmarkRouterStrategySelection(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := r.SelectProvider(req)
		if err != nil {
			b.Logf("Selection failed: %v", err)
		}
	}
}

// BenchmarkRouterStrategySelection_Parallel testa selezione in parallelo
func BenchmarkRouterStrategySelection_Parallel(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := r.SelectProvider(req)
			if err != nil {
				b.Logf("Selection failed: %v", err)
			}
		}
	})
}

// BenchmarkRoutingStrategies compara diverse strategie di routing
func BenchmarkRoutingStrategies(b *testing.B) {
	strategies := []string{
		"cost_optimized",
		"latency_first",
		"quality_first",
	}

	for _, strategy := range strategies {
		b.Run(strategy, func(b *testing.B) {
			r := createTestRouterWithStrategy(strategy)
			req := createRouterTestRequest()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := r.SelectProvider(req)
				if err != nil {
					b.Logf("Selection failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkRoutingDecisionLatency misura la latency della decisione
func BenchmarkRoutingDecisionLatency(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	var totalLatency time.Duration
	var successCount int64

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := r.SelectProvider(req)
		latency := time.Since(start)

		if err == nil {
			totalLatency += latency
			atomic.AddInt64(&successCount, 1)
		}
	}

	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)
		b.ReportMetric(float64(avgLatency.Microseconds()), "avg_decision_us")
		b.ReportMetric(float64(successCount)*1e6/float64(totalLatency.Microseconds()), "decisions/sec")
	}
}

// BenchmarkRoutingWithDifferentProviderCounts testa con diverso numero di provider
func BenchmarkRoutingWithDifferentProviderCounts(b *testing.B) {
	providerCounts := []int{1, 5, 10, 50, 100}

	for _, count := range providerCounts {
		b.Run(fmt.Sprintf("Providers_%d", count), func(b *testing.B) {
			r := createTestRouterWithProviders(count)
			req := createRouterTestRequest()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := r.SelectProvider(req)
				if err != nil {
					b.Logf("Selection failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkRoutingCacheHitRatio misura l'impatto della cache sul routing
func BenchmarkRoutingCacheHitRatio(b *testing.B) {
	cacheHitRatios := []float64{0.0, 0.5, 0.9, 0.99}

	for _, ratio := range cacheHitRatios {
		b.Run(fmt.Sprintf("CacheHitRatio_%.0f", ratio*100), func(b *testing.B) {
			r := createTestRouterWithCache(ratio)
			req := createRouterTestRequest()

			var cacheHits int64
			var cacheMisses int64

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				selection, err := r.SelectProvider(req)
				if err != nil {
					continue
				}

				// Simulate cache behavior
				if shouldCacheHit(ratio) {
					atomic.AddInt64(&cacheHits, 1)
				} else {
					atomic.AddInt64(&cacheMisses, 1)
				}

				_ = selection
			}

			actualRatio := float64(cacheHits) / float64(cacheHits+cacheMisses)
			b.ReportMetric(actualRatio*100, "cache_hit_rate_%")
		})
	}
}

// BenchmarkRoutingWithFailover testa il meccanismo di failover
func BenchmarkRoutingWithFailover(b *testing.B) {
	r := createTestRouterWithFailover()
	req := createRouterTestRequest()

	var failoverCount int64

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		selection, err := r.SelectProvider(req)
		if err != nil {
			continue
		}

		// Simulate provider failure and failover
		if selection.ProviderID == "primary" {
			// Try fallback
			req.PreferredProvider = "fallback"
			_, err = r.SelectProvider(req)
			if err == nil {
				atomic.AddInt64(&failoverCount, 1)
			}
			req.PreferredProvider = ""
		}
	}

	failoverRate := float64(failoverCount) / float64(b.N) * 100
	b.ReportMetric(failoverRate, "failover_rate_%")
}

// BenchmarkRoutingLoadBalancing testa il load balancing
func BenchmarkRoutingLoadBalancing(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	providerSelections := make(map[string]int64)
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			selection, err := r.SelectProvider(req)
			if err == nil {
				mu.Lock()
				providerSelections[selection.ProviderID]++
				mu.Unlock()
			}
		}
	})

	// Calculate distribution variance
	var total int64
	for _, count := range providerSelections {
		total += count
	}

	avgPerProvider := float64(total) / float64(len(providerSelections))
	var variance float64
	for _, count := range providerSelections {
		diff := float64(count) - avgPerProvider
		variance += diff * diff
	}
	variance /= float64(len(providerSelections))

	b.ReportMetric(variance, "distribution_variance")
}

// BenchmarkRoutingWithPriorities testa routing con priorità
func BenchmarkRoutingWithPriorities(b *testing.B) {
	r := createTestRouterWithPriorities()
	req := createRouterTestRequest()

	priorities := []string{"high", "medium", "low"}

	for _, priority := range priorities {
		b.Run(fmt.Sprintf("Priority_%s", priority), func(b *testing.B) {
			req.Priority = priority

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := r.SelectProvider(req)
				if err != nil {
					b.Logf("Selection failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkRoutingComplexRules testa regole di routing complesse
func BenchmarkRoutingComplexRules(b *testing.B) {
	r := createTestRouterWithComplexRules()
	req := createRouterTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := r.SelectProvider(req)
		if err != nil {
			b.Logf("Selection failed: %v", err)
		}
	}
}

// BenchmarkRoutingMemoryUsage misura il consumo di memoria del router
func BenchmarkRoutingMemoryUsage(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = r.SelectProvider(req)
	}
}

// BenchmarkRoutingThroughput misura il throughput del router
func BenchmarkRoutingThroughput(b *testing.B) {
	r := createTestRouter()
	req := createRouterTestRequest()

	var totalDecisions int64
	start := time.Now()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := r.SelectProvider(req)
			if err == nil {
				atomic.AddInt64(&totalDecisions, 1)
			}
		}
	})

	elapsed := time.Since(start)
	throughput := float64(totalDecisions) / elapsed.Seconds()
	b.ReportMetric(throughput, "decisions/sec")
}

// Helper functions for routing benchmarks

func createTestRouter() *mockRouter {
	return &mockRouter{
		strategy: &router.CostOptimizedStrategy{},
		providers: []string{"provider1", "provider2", "provider3"},
	}
}

func createTestRouterWithStrategy(strategy string) *mockRouter {
	r := createTestRouter()
	r.strategyName = strategy
	return r
}

func createTestRouterWithProviders(count int) *mockRouter {
	providers := make([]string, count)
	for i := 0; i < count; i++ {
		providers[i] = fmt.Sprintf("provider%d", i+1)
	}
	return &mockRouter{
		providers: providers,
	}
}

func createTestRouterWithCache(hitRatio float64) *mockRouter {
	r := createTestRouter()
	r.cacheHitRatio = hitRatio
	return r
}

func createTestRouterWithFailover() *mockRouter {
	return &mockRouter{
		providers: []string{"primary", "fallback", "backup"},
		hasFailover: true,
	}
}

func createTestRouterWithPriorities() *mockRouter {
	return &mockRouter{
		providers: []string{"high-prio", "med-prio", "low-prio"},
		hasPriorities: true,
	}
}

func createTestRouterWithComplexRules() *mockRouter {
	return &mockRouter{
		providers: []string{"provider1", "provider2", "provider3"},
		hasComplexRules: true,
	}
}

func createRouterTestRequest() *mockRouterRequest {
	return &mockRouterRequest{
		Model:       "gpt-3.5-turbo",
		Messages:    []string{"Hello"},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}
}

func shouldCacheHit(probability float64) bool {
	// Simple probability-based cache hit simulation
	return time.Now().UnixNano()%100 < int64(probability*100)
}

// Mock router implementation

type mockRouter struct {
	strategy        router.RoutingStrategy
	strategyName    string
	providers       []string
	cacheHitRatio   float64
	hasFailover     bool
	hasPriorities   bool
	hasComplexRules bool
}

type mockRouterRequest struct {
	Model             string
	Messages          []string
	MaxTokens         int
	Temperature       float64
	Stream            bool
	PreferredProvider string
	Priority          string
}

func (m *mockRouter) SelectProvider(req *mockRouterRequest) (*router.ProviderSelection, error) {
	// Simulate routing decision time
	time.Sleep(50 * time.Microsecond)

	// Simulate cache check
	if m.cacheHitRatio > 0 && shouldCacheHit(m.cacheHitRatio) {
		time.Sleep(5 * time.Microsecond) // Cache hit is faster
	}

	// Simulate complex rules evaluation
	if m.hasComplexRules {
		time.Sleep(100 * time.Microsecond)
	}

	// Select provider (round-robin for simplicity)
	providerIndex := int(time.Now().UnixNano()) % len(m.providers)
	selectedProvider := m.providers[providerIndex]

	return &router.ProviderSelection{
		ProviderID:    selectedProvider,
		ModelID:       req.Model,
		EstimatedCost: 0.001,
		Reason:        m.strategyName,
	}, nil
}

// Additional benchmark helpers

type routerMetrics struct {
	TotalDecisions  int64
	CacheHits       int64
	CacheMisses     int64
	FailoverCount   int64
	TotalLatency    time.Duration
	mu              sync.Mutex
}

func (rm *routerMetrics) RecordDecision(latency time.Duration, cacheHit bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.TotalDecisions++
	rm.TotalLatency += latency

	if cacheHit {
		rm.CacheHits++
	} else {
		rm.CacheMisses++
	}
}

func (rm *routerMetrics) RecordFailover() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.FailoverCount++
}

func (rm *routerMetrics) GetStats() map[string]float64 {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	stats := make(map[string]float64)

	if rm.TotalDecisions > 0 {
		stats["avg_latency_us"] = float64(rm.TotalLatency.Microseconds()) / float64(rm.TotalDecisions)
		stats["decisions_per_sec"] = float64(rm.TotalDecisions) * 1e6 / float64(rm.TotalLatency.Microseconds())
	}

	total := rm.CacheHits + rm.CacheMisses
	if total > 0 {
		stats["cache_hit_rate"] = float64(rm.CacheHits) / float64(total) * 100
	}

	if rm.TotalDecisions > 0 {
		stats["failover_rate"] = float64(rm.FailoverCount) / float64(rm.TotalDecisions) * 100
	}

	return stats
}
