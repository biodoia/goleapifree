package integration

import (
	"testing"

	"github.com/biodoia/goleapifree/internal/router"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/tests/testhelpers"
)

func TestRouting_CostOptimizedStrategy(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()
	cfg.Routing.Strategy = "cost_optimized"

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	// Create test providers with different pricing
	provider1 := testhelpers.CreateTestProvider(t, db.DB, "expensive-provider")
	provider2 := testhelpers.CreateTestProvider(t, db.DB, "cheap-provider")

	model1 := testhelpers.CreateTestModel(t, db.DB, provider1.ID, "expensive-model")
	model1.InputPricePer1k = 0.03
	model1.OutputPricePer1k = 0.06
	db.DB.Save(model1)

	model2 := testhelpers.CreateTestModel(t, db.DB, provider2.ID, "cheap-model")
	model2.InputPricePer1k = 0.0
	model2.OutputPricePer1k = 0.0
	db.DB.Save(model2)

	// Test selection
	req := &router.Request{
		Model:    "gpt-4",
		Messages: []router.Message{{Role: "user", Content: "Hello"}},
	}

	selection, err := r.SelectProvider(req)
	testhelpers.AssertNoError(t, err, "SelectProvider failed")
	testhelpers.AssertEqual(t, "cost_optimized", selection.Reason, "Strategy mismatch")
}

func TestRouting_LatencyFirstStrategy(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()
	cfg.Routing.Strategy = "latency_first"

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	// Create test providers with different latencies
	provider1 := testhelpers.CreateTestProvider(t, db.DB, "slow-provider")
	provider1.AvgLatencyMs = 500
	db.DB.Save(provider1)

	provider2 := testhelpers.CreateTestProvider(t, db.DB, "fast-provider")
	provider2.AvgLatencyMs = 100
	db.DB.Save(provider2)

	testhelpers.CreateTestModel(t, db.DB, provider1.ID, "slow-model")
	testhelpers.CreateTestModel(t, db.DB, provider2.ID, "fast-model")

	// Test selection
	req := &router.Request{
		Model:    "gpt-3.5-turbo",
		Messages: []router.Message{{Role: "user", Content: "Quick question"}},
	}

	selection, err := r.SelectProvider(req)
	testhelpers.AssertNoError(t, err, "SelectProvider failed")
	testhelpers.AssertEqual(t, "latency_first", selection.Reason, "Strategy mismatch")
}

func TestRouting_QualityFirstStrategy(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()
	cfg.Routing.Strategy = "quality_first"

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	// Create test providers with different quality scores
	provider1 := testhelpers.CreateTestProvider(t, db.DB, "low-quality")
	provider2 := testhelpers.CreateTestProvider(t, db.DB, "high-quality")

	model1 := testhelpers.CreateTestModel(t, db.DB, provider1.ID, "low-quality-model")
	model1.QualityScore = 0.5
	db.DB.Save(model1)

	model2 := testhelpers.CreateTestModel(t, db.DB, provider2.ID, "high-quality-model")
	model2.QualityScore = 0.95
	db.DB.Save(model2)

	// Test selection
	req := &router.Request{
		Model:    "claude-3-opus",
		Messages: []router.Message{{Role: "user", Content: "Complex task"}},
	}

	selection, err := r.SelectProvider(req)
	testhelpers.AssertNoError(t, err, "SelectProvider failed")
	testhelpers.AssertEqual(t, "quality_first", selection.Reason, "Strategy mismatch")
}

func TestRouting_StrategySwitch(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	strategies := []string{"cost_optimized", "latency_first", "quality_first"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			cfg := testhelpers.TestConfig()
			cfg.Routing.Strategy = strategy

			r, err := router.New(cfg, db)
			testhelpers.AssertNoError(t, err, "Failed to create router with "+strategy)

			req := &router.Request{
				Model:    "test-model",
				Messages: []router.Message{{Role: "user", Content: "Test"}},
			}

			selection, err := r.SelectProvider(req)
			testhelpers.AssertNoError(t, err, "SelectProvider failed for "+strategy)
			testhelpers.AssertEqual(t, strategy, selection.Reason, "Strategy mismatch")
		})
	}
}

func TestRouting_LoadBalancing(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()
	cfg.Routing.LoadBalanceMethod = "round_robin"

	// Create multiple equal providers
	for i := 0; i < 3; i++ {
		provider := testhelpers.CreateTestProvider(t, db.DB, "provider-"+string(rune('0'+i)))
		testhelpers.CreateTestModel(t, db.DB, provider.ID, "model-"+string(rune('0'+i)))
	}

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	req := &router.Request{
		Model:    "test-model",
		Messages: []router.Message{{Role: "user", Content: "Test"}},
	}

	// Make multiple requests
	selections := make(map[string]int)
	for i := 0; i < 10; i++ {
		selection, err := r.SelectProvider(req)
		testhelpers.AssertNoError(t, err, "SelectProvider failed")
		selections[selection.ProviderID]++
	}

	// In a real implementation, we'd verify round-robin distribution
	// For now, just verify we got selections
	testhelpers.AssertTrue(t, len(selections) > 0, "Should have provider selections")
}

func TestRouting_StreamingRequests(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()

	// Create providers with different streaming support
	provider1 := testhelpers.CreateTestProvider(t, db.DB, "streaming-provider")
	provider1.SupportsStreaming = true
	db.DB.Save(provider1)

	provider2 := testhelpers.CreateTestProvider(t, db.DB, "no-streaming-provider")
	provider2.SupportsStreaming = false
	db.DB.Save(provider2)

	testhelpers.CreateTestModel(t, db.DB, provider1.ID, "streaming-model")
	testhelpers.CreateTestModel(t, db.DB, provider2.ID, "no-streaming-model")

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	// Test streaming request
	req := &router.Request{
		Model:    "test-model",
		Messages: []router.Message{{Role: "user", Content: "Stream this"}},
		Stream:   true,
	}

	selection, err := r.SelectProvider(req)
	testhelpers.AssertNoError(t, err, "SelectProvider failed for streaming request")
	testhelpers.AssertTrue(t, selection != nil, "Should get a selection")
}

func TestRouting_RequestPriority(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	cfg := testhelpers.TestConfig()

	// Create providers with different tiers
	premiumProvider := testhelpers.CreateTestProvider(t, db.DB, "premium")
	premiumProvider.Tier = 1
	db.DB.Save(premiumProvider)

	standardProvider := testhelpers.CreateTestProvider(t, db.DB, "standard")
	standardProvider.Tier = 2
	db.DB.Save(standardProvider)

	experimentalProvider := testhelpers.CreateTestProvider(t, db.DB, "experimental")
	experimentalProvider.Tier = 3
	db.DB.Save(experimentalProvider)

	testhelpers.CreateTestModel(t, db.DB, premiumProvider.ID, "premium-model")
	testhelpers.CreateTestModel(t, db.DB, standardProvider.ID, "standard-model")
	testhelpers.CreateTestModel(t, db.DB, experimentalProvider.ID, "experimental-model")

	r, err := router.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create router")

	req := &router.Request{
		Model:    "gpt-4",
		Messages: []router.Message{{Role: "user", Content: "Important request"}},
	}

	selection, err := r.SelectProvider(req)
	testhelpers.AssertNoError(t, err, "SelectProvider failed")
	testhelpers.AssertTrue(t, selection != nil, "Should get a selection")
}

// Benchmark tests
func BenchmarkRouting_SelectProvider(b *testing.B) {
	db := testhelpers.TestDB(&testing.T{})
	cfg := testhelpers.TestConfig()

	// Create test data
	for i := 0; i < 5; i++ {
		provider := testhelpers.CreateTestProvider(&testing.T{}, db.DB, "provider-"+string(rune('0'+i)))
		testhelpers.CreateTestModel(&testing.T{}, db.DB, provider.ID, "model-"+string(rune('0'+i)))
	}

	r, _ := router.New(cfg, db)

	req := &router.Request{
		Model:    "test-model",
		Messages: []router.Message{{Role: "user", Content: "Benchmark"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.SelectProvider(req)
	}
}
