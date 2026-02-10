package integration

import (
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/biodoia/goleapifree/tests/testhelpers"
)

func TestFailover_ProviderHealthDegradation(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create primary and backup providers
	primary := testhelpers.CreateTestProvider(t, db.DB, "primary-provider")
	backup := testhelpers.CreateTestProvider(t, db.DB, "backup-provider")

	testhelpers.CreateTestModel(t, db.DB, primary.ID, "primary-model")
	testhelpers.CreateTestModel(t, db.DB, backup.ID, "backup-model")

	// Verify both are initially available
	testhelpers.AssertTrue(t, primary.IsAvailable(), "Primary should be available")
	testhelpers.AssertTrue(t, backup.IsAvailable(), "Backup should be available")

	// Simulate primary provider degradation
	primary.HealthScore = 0.3
	primary.LastHealthCheck = time.Now()
	db.DB.Save(primary)

	// Verify primary is no longer available
	var updatedPrimary models.Provider
	db.DB.First(&updatedPrimary, "id = ?", primary.ID)
	testhelpers.AssertFalse(t, updatedPrimary.IsAvailable(),
		"Primary with low health should not be available")

	// Verify backup is still available
	var updatedBackup models.Provider
	db.DB.First(&updatedBackup, "id = ?", backup.ID)
	testhelpers.AssertTrue(t, updatedBackup.IsAvailable(),
		"Backup should still be available")
}

func TestFailover_ProviderStatusChange(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	provider := testhelpers.CreateTestProvider(t, db.DB, "test-provider")

	// Test different status transitions
	statuses := []struct {
		status    models.ProviderStatus
		available bool
	}{
		{models.ProviderStatusActive, true},
		{models.ProviderStatusMaintenance, false},
		{models.ProviderStatusDown, false},
		{models.ProviderStatusDeprecated, false},
		{models.ProviderStatusActive, true},
	}

	for _, tc := range statuses {
		provider.Status = tc.status
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)

		var updated models.Provider
		db.DB.First(&updated, "id = ?", provider.ID)

		testhelpers.AssertEqual(t, tc.available, updated.IsAvailable(),
			"Availability mismatch for status "+string(tc.status))
	}
}

func TestFailover_MultipleProviderRanking(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create providers with different health scores
	providers := []struct {
		name        string
		healthScore float64
		latency     int
		tier        int
	}{
		{"provider-A", 1.0, 100, 1},
		{"provider-B", 0.9, 150, 1},
		{"provider-C", 0.7, 200, 2},
		{"provider-D", 0.5, 300, 2},
		{"provider-E", 0.3, 500, 3},
	}

	for _, p := range providers {
		provider := testhelpers.CreateTestProvider(t, db.DB, p.name)
		provider.HealthScore = p.healthScore
		provider.AvgLatencyMs = p.latency
		provider.Tier = p.tier
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)
	}

	// Query best providers (health > 0.5, ordered by score)
	var bestProviders []models.Provider
	db.DB.Where("health_score > ? AND status = ?", 0.5, models.ProviderStatusActive).
		Order("health_score DESC, avg_latency_ms ASC").
		Find(&bestProviders)

	testhelpers.AssertEqual(t, 3, len(bestProviders), "Should have 3 good providers")
	testhelpers.AssertEqual(t, "provider-A", bestProviders[0].Name, "Best provider mismatch")
	testhelpers.AssertTrue(t, bestProviders[0].HealthScore > bestProviders[1].HealthScore,
		"Providers should be ordered by health score")
}

func TestFailover_LatencyBasedFailover(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create providers with different latencies
	fastProvider := testhelpers.CreateTestProvider(t, db.DB, "fast-provider")
	fastProvider.AvgLatencyMs = 50
	fastProvider.HealthScore = 0.9
	fastProvider.LastHealthCheck = time.Now()
	db.DB.Save(fastProvider)

	slowProvider := testhelpers.CreateTestProvider(t, db.DB, "slow-provider")
	slowProvider.AvgLatencyMs = 500
	slowProvider.HealthScore = 0.9
	slowProvider.LastHealthCheck = time.Now()
	db.DB.Save(slowProvider)

	// Query by latency for time-sensitive requests
	var fastProviders []models.Provider
	db.DB.Where("avg_latency_ms < ? AND status = ?", 200, models.ProviderStatusActive).
		Order("avg_latency_ms ASC").
		Find(&fastProviders)

	testhelpers.AssertEqual(t, 1, len(fastProviders), "Should have 1 fast provider")
	testhelpers.AssertEqual(t, "fast-provider", fastProviders[0].Name, "Fast provider mismatch")
}

func TestFailover_CircuitBreakerPattern(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	provider := testhelpers.CreateTestProvider(t, db.DB, "circuit-test")

	// Simulate multiple failures
	failures := []struct {
		healthScore float64
		status      models.ProviderStatus
	}{
		{0.9, models.ProviderStatusActive},
		{0.7, models.ProviderStatusActive},
		{0.5, models.ProviderStatusActive},
		{0.3, models.ProviderStatusDown}, // Circuit opens
	}

	for i, f := range failures {
		provider.HealthScore = f.healthScore
		provider.Status = f.status
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)

		var updated models.Provider
		db.DB.First(&updated, "id = ?", provider.ID)

		if i < len(failures)-1 {
			// Before circuit opens
			available := updated.IsAvailable()
			expected := f.healthScore > 0.5
			testhelpers.AssertEqual(t, expected, available,
				"Availability mismatch at failure "+string(rune('0'+i)))
		} else {
			// Circuit opened
			testhelpers.AssertFalse(t, updated.IsAvailable(),
				"Provider should be unavailable after circuit opens")
		}
	}
}

func TestFailover_GracefulRecovery(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	provider := testhelpers.CreateTestProvider(t, db.DB, "recovery-test")

	// Simulate provider going down
	provider.Status = models.ProviderStatusDown
	provider.HealthScore = 0.2
	provider.LastHealthCheck = time.Now()
	db.DB.Save(provider)

	var downProvider models.Provider
	db.DB.First(&downProvider, "id = ?", provider.ID)
	testhelpers.AssertFalse(t, downProvider.IsAvailable(), "Provider should be down")

	// Simulate gradual recovery
	recoverySteps := []float64{0.4, 0.6, 0.8, 1.0}
	for _, score := range recoverySteps {
		provider.HealthScore = score
		if score > 0.5 {
			provider.Status = models.ProviderStatusActive
		}
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)

		var recovering models.Provider
		db.DB.First(&recovering, "id = ?", provider.ID)

		if score > 0.5 {
			testhelpers.AssertTrue(t, recovering.IsAvailable(),
				"Provider should be available during recovery")
		}
	}
}

func TestFailover_StaleHealthCheck(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	provider := testhelpers.CreateTestProvider(t, db.DB, "stale-test")

	// Set old health check time
	provider.HealthScore = 1.0
	provider.Status = models.ProviderStatusActive
	provider.LastHealthCheck = time.Now().Add(-15 * time.Minute)
	db.DB.Save(provider)

	var staleProvider models.Provider
	db.DB.First(&staleProvider, "id = ?", provider.ID)

	// Provider with stale health check should not be available
	testhelpers.AssertFalse(t, staleProvider.IsAvailable(),
		"Provider with stale health check should not be available")
}

func TestFailover_PriorityTiers(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create providers in different tiers
	tiers := []struct {
		tier int
		name string
	}{
		{1, "tier1-provider"},
		{2, "tier2-provider"},
		{3, "tier3-provider"},
	}

	for _, tc := range tiers {
		provider := testhelpers.CreateTestProvider(t, db.DB, tc.name)
		provider.Tier = tc.tier
		provider.HealthScore = 0.9
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)
	}

	// Query with tier-based failover
	var orderedProviders []models.Provider
	db.DB.Where("status = ?", models.ProviderStatusActive).
		Order("tier ASC, health_score DESC").
		Find(&orderedProviders)

	testhelpers.AssertEqual(t, 3, len(orderedProviders), "Should have 3 providers")
	testhelpers.AssertEqual(t, 1, orderedProviders[0].Tier, "First should be tier 1")
	testhelpers.AssertEqual(t, 2, orderedProviders[1].Tier, "Second should be tier 2")
	testhelpers.AssertEqual(t, 3, orderedProviders[2].Tier, "Third should be tier 3")
}

func TestFailover_GeographicFailover(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create providers in different regions (simulated via latency)
	regions := []struct {
		name    string
		latency int
	}{
		{"us-east", 50},
		{"us-west", 100},
		{"eu-central", 150},
		{"ap-southeast", 250},
	}

	for _, region := range regions {
		provider := testhelpers.CreateTestProvider(t, db.DB, region.name)
		provider.AvgLatencyMs = region.latency
		provider.HealthScore = 0.9
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)
	}

	// Select closest region (lowest latency)
	var closestProviders []models.Provider
	db.DB.Where("status = ?", models.ProviderStatusActive).
		Order("avg_latency_ms ASC").
		Limit(2).
		Find(&closestProviders)

	testhelpers.AssertEqual(t, 2, len(closestProviders), "Should have 2 closest providers")
	testhelpers.AssertEqual(t, "us-east", closestProviders[0].Name, "Closest should be us-east")
	testhelpers.AssertTrue(t, closestProviders[0].AvgLatencyMs < closestProviders[1].AvgLatencyMs,
		"Should be ordered by latency")
}

func TestFailover_WeightedSelection(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	// Create providers with different health scores (weights)
	providers := []struct {
		name   string
		weight float64
	}{
		{"high-weight", 1.0},
		{"medium-weight", 0.7},
		{"low-weight", 0.5},
	}

	for _, p := range providers {
		provider := testhelpers.CreateTestProvider(t, db.DB, p.name)
		provider.HealthScore = p.weight
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)
	}

	// Query providers weighted by health score
	var weightedProviders []models.Provider
	db.DB.Where("health_score >= ? AND status = ?", 0.5, models.ProviderStatusActive).
		Order("health_score DESC").
		Find(&weightedProviders)

	testhelpers.AssertEqual(t, 3, len(weightedProviders), "Should have 3 providers")
	testhelpers.AssertEqual(t, "high-weight", weightedProviders[0].Name,
		"Highest weight should be first")
}

// Benchmark tests
func BenchmarkFailover_ProviderSelection(b *testing.B) {
	db := testhelpers.TestDB(&testing.T{})

	// Create many providers
	for i := 0; i < 100; i++ {
		provider := testhelpers.CreateTestProvider(&testing.T{}, db.DB, "provider-"+string(rune('0'+i)))
		provider.HealthScore = float64(i%10) / 10.0
		provider.AvgLatencyMs = 50 + (i % 10 * 50)
		provider.LastHealthCheck = time.Now()
		db.DB.Save(provider)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var providers []models.Provider
		db.DB.Where("health_score > ? AND status = ?", 0.5, models.ProviderStatusActive).
			Order("health_score DESC, avg_latency_ms ASC").
			Limit(3).
			Find(&providers)
	}
}
