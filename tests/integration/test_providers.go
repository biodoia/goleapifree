package integration

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/biodoia/goleapifree/tests/testhelpers"
	"github.com/google/uuid"
)

func TestProviders_CRUD(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Create
	provider := &models.Provider{
		Name:              "test-openai",
		Type:              models.ProviderTypeFree,
		Status:            models.ProviderStatusActive,
		BaseURL:           "https://api.openai.com",
		AuthType:          models.AuthTypeAPIKey,
		Tier:              1,
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsJSON:      true,
		HealthScore:       1.0,
		LastHealthCheck:   time.Now(),
	}

	if err := provider.BeforeCreate(); err != nil {
		t.Fatalf("BeforeCreate failed: %v", err)
	}

	if err := db.WithContext(ctx).Create(provider).Error; err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Read
	var retrieved models.Provider
	if err := db.WithContext(ctx).First(&retrieved, "id = ?", provider.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve provider: %v", err)
	}

	testhelpers.AssertEqual(t, provider.Name, retrieved.Name, "Provider name mismatch")
	testhelpers.AssertEqual(t, provider.BaseURL, retrieved.BaseURL, "Provider BaseURL mismatch")

	// Update
	retrieved.HealthScore = 0.8
	if err := db.WithContext(ctx).Save(&retrieved).Error; err != nil {
		t.Fatalf("Failed to update provider: %v", err)
	}

	var updated models.Provider
	if err := db.WithContext(ctx).First(&updated, "id = ?", provider.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve updated provider: %v", err)
	}

	testhelpers.AssertEqual(t, 0.8, updated.HealthScore, "Health score not updated")

	// Delete
	if err := db.WithContext(ctx).Delete(&updated).Error; err != nil {
		t.Fatalf("Failed to delete provider: %v", err)
	}

	var deleted models.Provider
	err := db.WithContext(ctx).First(&deleted, "id = ?", provider.ID).Error
	testhelpers.AssertError(t, err, "Should error when retrieving deleted provider")
}

func TestProviders_MultipleWithModels(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Create multiple providers
	providers := []*models.Provider{
		testhelpers.CreateTestProvider(t, db.DB, "openai"),
		testhelpers.CreateTestProvider(t, db.DB, "anthropic"),
		testhelpers.CreateTestProvider(t, db.DB, "groq"),
	}

	// Create models for each provider
	for _, provider := range providers {
		for i := 0; i < 3; i++ {
			modelName := provider.Name + "-model-" + string(rune('0'+i))
			testhelpers.CreateTestModel(t, db.DB, provider.ID, modelName)
		}
	}

	// Query providers with models
	var retrieved []models.Provider
	if err := db.WithContext(ctx).Preload("Models").Find(&retrieved).Error; err != nil {
		t.Fatalf("Failed to retrieve providers with models: %v", err)
	}

	testhelpers.AssertEqual(t, 3, len(retrieved), "Should have 3 providers")

	for _, provider := range retrieved {
		testhelpers.AssertEqual(t, 3, len(provider.Models), "Each provider should have 3 models")
	}
}

func TestProviders_HealthCheck(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Create provider
	provider := testhelpers.CreateTestProvider(t, db.DB, "test-provider")

	// Simulate health check updates
	updates := []struct {
		healthScore float64
		latency     int
		status      models.ProviderStatus
	}{
		{1.0, 100, models.ProviderStatusActive},
		{0.9, 150, models.ProviderStatusActive},
		{0.7, 200, models.ProviderStatusActive},
		{0.4, 500, models.ProviderStatusDown},
	}

	for _, update := range updates {
		provider.HealthScore = update.healthScore
		provider.AvgLatencyMs = update.latency
		provider.Status = update.status
		provider.LastHealthCheck = time.Now()

		if err := db.WithContext(ctx).Save(provider).Error; err != nil {
			t.Fatalf("Failed to update health metrics: %v", err)
		}

		// Verify IsAvailable logic
		var retrieved models.Provider
		if err := db.WithContext(ctx).First(&retrieved, "id = ?", provider.ID).Error; err != nil {
			t.Fatalf("Failed to retrieve provider: %v", err)
		}

		available := retrieved.IsAvailable()
		expectedAvailable := update.status == models.ProviderStatusActive &&
			update.healthScore > 0.5

		testhelpers.AssertEqual(t, expectedAvailable, available, "IsAvailable mismatch")
	}
}

func TestProviders_Filtering(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Create providers with different statuses
	statuses := []models.ProviderStatus{
		models.ProviderStatusActive,
		models.ProviderStatusDeprecated,
		models.ProviderStatusDown,
		models.ProviderStatusMaintenance,
	}

	for i, status := range statuses {
		provider := testhelpers.CreateTestProvider(t, db.DB, "provider-"+string(rune('0'+i)))
		provider.Status = status
		if err := db.WithContext(ctx).Save(provider).Error; err != nil {
			t.Fatalf("Failed to update provider status: %v", err)
		}
	}

	// Query only active providers
	var activeProviders []models.Provider
	if err := db.WithContext(ctx).
		Where("status = ?", models.ProviderStatusActive).
		Find(&activeProviders).Error; err != nil {
		t.Fatalf("Failed to query active providers: %v", err)
	}

	testhelpers.AssertEqual(t, 1, len(activeProviders), "Should have 1 active provider")

	// Query by type
	var freeProviders []models.Provider
	if err := db.WithContext(ctx).
		Where("type = ?", models.ProviderTypeFree).
		Find(&freeProviders).Error; err != nil {
		t.Fatalf("Failed to query free providers: %v", err)
	}

	testhelpers.AssertEqual(t, 4, len(freeProviders), "Should have 4 free providers")
}

func TestProviders_HealthScoreQueries(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Create providers with different health scores
	scores := []float64{0.3, 0.5, 0.7, 0.9, 1.0}
	for i, score := range scores {
		provider := testhelpers.CreateTestProvider(t, db.DB, "provider-"+string(rune('0'+i)))
		provider.HealthScore = score
		if err := db.WithContext(ctx).Save(provider).Error; err != nil {
			t.Fatalf("Failed to update health score: %v", err)
		}
	}

	// Query providers with good health (> 0.7)
	var healthyProviders []models.Provider
	if err := db.WithContext(ctx).
		Where("health_score > ?", 0.7).
		Order("health_score DESC").
		Find(&healthyProviders).Error; err != nil {
		t.Fatalf("Failed to query healthy providers: %v", err)
	}

	testhelpers.AssertEqual(t, 3, len(healthyProviders), "Should have 3 healthy providers")
	testhelpers.AssertTrue(t, healthyProviders[0].HealthScore >= healthyProviders[1].HealthScore,
		"Should be ordered by health score")
}

func TestProviders_ConcurrentAccess(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()
	provider := testhelpers.CreateTestProvider(t, db.DB, "concurrent-test")

	// Simulate concurrent health check updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(iteration int) {
			defer func() { done <- true }()

			var p models.Provider
			if err := db.WithContext(ctx).First(&p, "id = ?", provider.ID).Error; err != nil {
				t.Errorf("Failed to retrieve provider: %v", err)
				return
			}

			p.HealthScore = float64(iteration) / 10.0
			p.LastHealthCheck = time.Now()

			if err := db.WithContext(ctx).Save(&p).Error; err != nil {
				t.Errorf("Failed to update provider: %v", err)
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	var final models.Provider
	if err := db.WithContext(ctx).First(&final, "id = ?", provider.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve final provider state: %v", err)
	}

	testhelpers.AssertTrue(t, final.HealthScore >= 0.0 && final.HealthScore <= 1.0,
		"Health score should be valid")
}

func TestProviders_Discovery(t *testing.T) {
	db := testhelpers.TestDB(t)
	defer testhelpers.CleanupDB(t, db)

	ctx := context.Background()

	// Simulate discovering new providers
	sources := []string{"github", "manual", "scraper"}

	for i, source := range sources {
		provider := &models.Provider{
			ID:          uuid.New(),
			Name:        "discovered-" + source,
			Type:        models.ProviderTypeFree,
			Status:      models.ProviderStatusActive,
			BaseURL:     "https://api." + source + ".com",
			AuthType:    models.AuthTypeAPIKey,
			Source:      source,
			DiscoveredAt: time.Now().Add(time.Duration(-i) * time.Hour),
		}

		if err := db.WithContext(ctx).Create(provider).Error; err != nil {
			t.Fatalf("Failed to create discovered provider: %v", err)
		}
	}

	// Query by discovery source
	var githubProviders []models.Provider
	if err := db.WithContext(ctx).
		Where("source = ?", "github").
		Find(&githubProviders).Error; err != nil {
		t.Fatalf("Failed to query GitHub providers: %v", err)
	}

	testhelpers.AssertEqual(t, 1, len(githubProviders), "Should have 1 GitHub provider")

	// Query recently discovered (last 2 hours)
	cutoff := time.Now().Add(-2 * time.Hour)
	var recentProviders []models.Provider
	if err := db.WithContext(ctx).
		Where("discovered_at > ?", cutoff).
		Order("discovered_at DESC").
		Find(&recentProviders).Error; err != nil {
		t.Fatalf("Failed to query recent providers: %v", err)
	}

	testhelpers.AssertEqual(t, 3, len(recentProviders), "Should have 3 recent providers")
}
