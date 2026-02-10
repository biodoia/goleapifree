package stats

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *database.DB {
	cfg := &database.Config{
		Type:       "sqlite",
		Connection: ":memory:",
		LogLevel:   "silent",
	}

	db, err := database.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	err = db.AutoMigrate()
	require.NoError(t, err)

	return db
}

func createTestProvider(t *testing.T, db *database.DB) *models.Provider {
	provider := &models.Provider{
		Name:        "test-provider",
		Type:        models.ProviderTypeFree,
		Status:      models.ProviderStatusActive,
		BaseURL:     "https://test.example.com",
		AuthType:    models.AuthTypeAPIKey,
		HealthScore: 1.0,
	}

	err := db.Create(provider).Error
	require.NoError(t, err)

	return provider
}

func TestCollector(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := createTestProvider(t, db)
	collector := NewCollector(db, 10)

	t.Run("Record metrics", func(t *testing.T) {
		metrics := &RequestMetrics{
			ProviderID:    provider.ID,
			ModelID:       uuid.New(),
			UserID:        uuid.New(),
			Method:        "POST",
			Endpoint:      "/v1/chat/completions",
			StatusCode:    200,
			LatencyMs:     150,
			InputTokens:   100,
			OutputTokens:  50,
			Success:       true,
			EstimatedCost: 0.001,
			Timestamp:     time.Now(),
		}

		collector.Record(metrics)

		stats := collector.GetProviderMetrics(provider.ID)
		require.NotNil(t, stats)
		assert.Equal(t, int64(1), stats.TotalRequests)
		assert.Equal(t, int64(1), stats.SuccessCount)
		assert.Equal(t, int64(150), stats.TotalLatencyMs)
		assert.Equal(t, int64(150), stats.TotalTokens)
	})

	t.Run("Calculate success rate", func(t *testing.T) {
		// Add one more success
		collector.Record(&RequestMetrics{
			ProviderID: provider.ID,
			Success:    true,
			LatencyMs:  100,
		})

		// Add one failure
		collector.Record(&RequestMetrics{
			ProviderID: provider.ID,
			Success:    false,
			StatusCode: 500,
		})

		successRate := collector.CalculateSuccessRate(provider.ID)
		assert.InDelta(t, 0.666, successRate, 0.01)
	})

	t.Run("Calculate avg latency", func(t *testing.T) {
		avgLatency := collector.CalculateAvgLatency(provider.ID)
		// (150 + 100 + 0) / 3 = 83.33
		assert.InDelta(t, 83, avgLatency, 1)
	})

	t.Run("Get all metrics", func(t *testing.T) {
		allMetrics := collector.GetAllMetrics()
		assert.Len(t, allMetrics, 1)
		assert.Contains(t, allMetrics, provider.ID)
	})
}

func TestAggregator(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := createTestProvider(t, db)
	collector := NewCollector(db, 10)
	aggregator := NewAggregator(db, collector, 100*time.Millisecond, 7)

	t.Run("Aggregate window", func(t *testing.T) {
		// Record some metrics
		for i := 0; i < 5; i++ {
			collector.Record(&RequestMetrics{
				ProviderID: provider.ID,
				Success:    true,
				LatencyMs:  100 + i*10,
			})
		}

		// Trigger aggregation manually
		aggregator.aggregate()

		// Query aggregated data
		ctx := context.Background()
		end := time.Now()
		start := end.Add(-time.Hour)

		stats, err := aggregator.AggregateWindow(ctx, provider.ID, WindowMinute, start, end)
		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, int64(5), stats.TotalRequests)
		assert.Equal(t, 1.0, stats.SuccessRate)
	})

	t.Run("Get hourly stats", func(t *testing.T) {
		ctx := context.Background()
		stats, err := aggregator.GetHourlyStats(ctx, provider.ID, 1)
		require.NoError(t, err)
		assert.Len(t, stats, 1)
	})

	t.Run("Get daily stats", func(t *testing.T) {
		ctx := context.Background()
		stats, err := aggregator.GetDailyStats(ctx, provider.ID, 1)
		require.NoError(t, err)
		assert.Len(t, stats, 1)
	})
}

func TestPrometheusExporter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := createTestProvider(t, db)
	collector := NewCollector(db, 10)
	exporter := NewPrometheusExporter(db, collector, "test")

	t.Run("Record request", func(t *testing.T) {
		exporter.RecordRequest(provider.Name, "test-model", "success")
		// No panic means success
	})

	t.Run("Record duration", func(t *testing.T) {
		exporter.RecordDuration(provider.Name, "test-model", 123.45)
		// No panic means success
	})

	t.Run("Record error", func(t *testing.T) {
		exporter.RecordError(provider.Name, "timeout")
		// No panic means success
	})

	t.Run("Record tokens", func(t *testing.T) {
		exporter.RecordTokens(provider.Name, "test-model", "input", 100)
		exporter.RecordTokens(provider.Name, "test-model", "output", 50)
		// No panic means success
	})

	t.Run("Record complete request", func(t *testing.T) {
		collector.Record(&RequestMetrics{
			ProviderID: provider.ID,
			Success:    true,
			LatencyMs:  100,
		})

		exporter.RecordRequestComplete(
			provider.Name,
			"test-model",
			true,
			100,
			50,
			25,
			0.001,
			"",
		)
		// No panic means success
	})
}

func TestDashboard(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := createTestProvider(t, db)
	collector := NewCollector(db, 10)
	aggregator := NewAggregator(db, collector, 100*time.Millisecond, 7)
	dashboard := NewDashboard(db, collector, aggregator)

	// Record some test data
	for i := 0; i < 10; i++ {
		collector.Record(&RequestMetrics{
			ProviderID:    provider.ID,
			Success:       i < 8, // 80% success rate
			LatencyMs:     100 + i*10,
			InputTokens:   50,
			OutputTokens:  25,
			EstimatedCost: 0.001,
		})
	}

	ctx := context.Background()

	t.Run("Get summary", func(t *testing.T) {
		summary, err := dashboard.GetSummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, 1, summary.TotalProviders)
		assert.Equal(t, 1, summary.ActiveProviders)
		assert.InDelta(t, 0.8, summary.AvgSuccessRate, 0.01)
	})

	t.Run("Get provider stats", func(t *testing.T) {
		stats, err := dashboard.GetProviderStats(ctx)
		require.NoError(t, err)
		assert.Len(t, stats, 1)

		stat := stats[0]
		assert.Equal(t, provider.Name, stat.ProviderName)
		assert.InDelta(t, 0.8, stat.SuccessRate, 0.01)
		assert.InDelta(t, 0.2, stat.ErrorRate, 0.01)
	})

	t.Run("Get hourly trends", func(t *testing.T) {
		trends, err := dashboard.GetHourlyTrends(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, trends, 1)
	})

	t.Run("Get cost savings", func(t *testing.T) {
		savings, err := dashboard.GetCostSavings(ctx)
		require.NoError(t, err)
		require.NotNil(t, savings)

		assert.InDelta(t, 0.01, savings.TotalSaved, 0.001)
		assert.Contains(t, savings.ByProvider, provider.Name)
	})

	t.Run("Get top providers", func(t *testing.T) {
		stats, err := dashboard.GetProviderStats(ctx)
		require.NoError(t, err)

		topProviders := dashboard.GetTopProviders(stats, 5)
		assert.Len(t, topProviders, 1)
		assert.Equal(t, 1, topProviders[0].Rank)
	})
}

func TestManager(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := createTestProvider(t, db)
	cfg := DefaultConfig()
	cfg.PrometheusEnabled = false // Disable for testing
	cfg.CollectorFlushInterval = 100 * time.Millisecond
	cfg.AggregationInterval = 100 * time.Millisecond

	manager := NewManager(db, cfg)

	t.Run("Start and stop", func(t *testing.T) {
		err := manager.Start()
		require.NoError(t, err)
		assert.True(t, manager.IsStarted())

		manager.Stop()
		assert.False(t, manager.IsStarted())
	})

	t.Run("Record metrics", func(t *testing.T) {
		err := manager.Start()
		require.NoError(t, err)
		defer manager.Stop()

		metrics := &RequestMetrics{
			ProviderID:    provider.ID,
			Success:       true,
			LatencyMs:     100,
			InputTokens:   50,
			OutputTokens:  25,
			EstimatedCost: 0.001,
		}

		manager.Record(metrics)

		stats := manager.Collector().GetProviderMetrics(provider.ID)
		require.NotNil(t, stats)
		assert.Equal(t, int64(1), stats.TotalRequests)
	})

	t.Run("Get dashboard data", func(t *testing.T) {
		err := manager.Start()
		require.NoError(t, err)
		defer manager.Stop()

		ctx := context.Background()
		data, err := manager.GetDashboardData(ctx)
		require.NoError(t, err)
		require.NotNil(t, data)

		assert.NotNil(t, data.Summary)
		assert.NotNil(t, data.ProviderStats)
		assert.NotNil(t, data.CostSavings)
	})
}

func BenchmarkCollectorRecord(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer db.Close()

	provider := createTestProvider(&testing.T{}, db)
	collector := NewCollector(db, 1000)

	metrics := &RequestMetrics{
		ProviderID:    provider.ID,
		Success:       true,
		LatencyMs:     100,
		InputTokens:   50,
		OutputTokens:  25,
		EstimatedCost: 0.001,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Record(metrics)
	}
}

func BenchmarkAggregatorWindow(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer db.Close()

	provider := createTestProvider(&testing.T{}, db)
	collector := NewCollector(db, 1000)
	aggregator := NewAggregator(db, collector, time.Minute, 7)

	// Prepare data
	for i := 0; i < 100; i++ {
		collector.Record(&RequestMetrics{
			ProviderID: provider.ID,
			Success:    true,
			LatencyMs:  100,
		})
	}
	aggregator.aggregate()

	ctx := context.Background()
	end := time.Now()
	start := end.Add(-time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = aggregator.AggregateWindow(ctx, provider.ID, WindowMinute, start, end)
	}
}
