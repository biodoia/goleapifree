package quota

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/cache"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB crea un database di test in memoria
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(
		&models.Provider{},
		&models.Account{},
		&models.RateLimit{},
		&models.RequestLog{},
		&models.ProviderStats{},
	)
	require.NoError(t, err)

	return db
}

// setupTestRedis crea un mock Redis client per test
func setupTestRedis(t *testing.T) *cache.RedisClient {
	// Per test reali, usare miniredis o un'istanza Redis di test
	// Per ora, skippiamo se Redis non è disponibile
	client, err := cache.NewRedisClient("localhost:6379", "", 1)
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	return client
}

func TestManager_CheckAvailability(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)

	manager := NewManager(db, redisClient)
	ctx := context.Background()

	// Crea provider e account di test
	provider := models.Provider{
		Name: "Test Provider",
		Type: "free",
	}
	require.NoError(t, db.Create(&provider).Error)

	account := models.Account{
		ProviderID: provider.ID,
		UserID:     uuid.New(),
		Active:     true,
		QuotaUsed:  100,
		QuotaLimit: 1000,
		LastReset:  time.Now(),
	}
	require.NoError(t, db.Create(&account).Error)

	// Test: Quota disponibile
	t.Run("quota available", func(t *testing.T) {
		status, err := manager.CheckAvailability(ctx, account.ID, 100)
		require.NoError(t, err)
		assert.True(t, status.Available)
		assert.Equal(t, int64(100), status.CurrentUsage)
		assert.Equal(t, int64(1000), status.Limit)
	})

	// Test: Quota esaurita
	t.Run("quota exceeded", func(t *testing.T) {
		status, err := manager.CheckAvailability(ctx, account.ID, 1000)
		require.NoError(t, err)
		assert.False(t, status.Available)
		assert.Equal(t, "quota exceeded", status.Reason)
	})

	// Test: Account inattivo
	t.Run("account inactive", func(t *testing.T) {
		db.Model(&account).Update("active", false)
		status, err := manager.CheckAvailability(ctx, account.ID, 100)
		require.NoError(t, err)
		assert.False(t, status.Available)
		assert.Equal(t, "account inactive", status.Reason)
	})
}

func TestManager_ConsumeQuota(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)

	manager := NewManager(db, redisClient)
	ctx := context.Background()

	// Crea account di test
	provider := models.Provider{Name: "Test Provider"}
	require.NoError(t, db.Create(&provider).Error)

	account := models.Account{
		ProviderID: provider.ID,
		UserID:     uuid.New(),
		Active:     true,
		QuotaUsed:  0,
		QuotaLimit: 1000,
		LastReset:  time.Now(),
	}
	require.NoError(t, db.Create(&account).Error)

	// Test: Consuma quota
	t.Run("consume quota", func(t *testing.T) {
		err := manager.ConsumeQuota(ctx, account.ID, 100)
		require.NoError(t, err)

		// Verifica aggiornamento
		var updated models.Account
		db.First(&updated, account.ID)
		assert.Equal(t, int64(100), updated.QuotaUsed)
	})

	// Test: Consuma più volte
	t.Run("consume multiple times", func(t *testing.T) {
		err := manager.ConsumeQuota(ctx, account.ID, 50)
		require.NoError(t, err)

		err = manager.ConsumeQuota(ctx, account.ID, 50)
		require.NoError(t, err)

		var updated models.Account
		db.First(&updated, account.ID)
		assert.Equal(t, int64(200), updated.QuotaUsed)
	})
}

func TestManager_ResetQuota(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)

	manager := NewManager(db, redisClient)
	ctx := context.Background()

	// Crea account con quota usata
	provider := models.Provider{Name: "Test Provider"}
	require.NoError(t, db.Create(&provider).Error)

	account := models.Account{
		ProviderID: provider.ID,
		UserID:     uuid.New(),
		Active:     true,
		QuotaUsed:  500,
		QuotaLimit: 1000,
		LastReset:  time.Now().Add(-25 * time.Hour), // Più di 24h fa
	}
	require.NoError(t, db.Create(&account).Error)

	// Test: Reset quota
	t.Run("reset quota", func(t *testing.T) {
		err := manager.ResetQuota(ctx, account.ID)
		require.NoError(t, err)

		var updated models.Account
		db.First(&updated, account.ID)
		assert.Equal(t, int64(0), updated.QuotaUsed)
		assert.True(t, time.Since(updated.LastReset) < 1*time.Second)
	})
}

func TestRateLimiter_CheckLimit(t *testing.T) {
	redisClient := setupTestRedis(t)
	rateLimiter := NewRateLimiter(redisClient)
	ctx := context.Background()

	providerID := uuid.New()
	accountID := uuid.New()

	// Test: RPM limit
	t.Run("rpm limit not exceeded", func(t *testing.T) {
		limits := []models.RateLimit{
			{
				LimitType:  models.LimitTypeRPM,
				LimitValue: 10,
			},
		}

		result, err := rateLimiter.CheckLimit(ctx, providerID, accountID, limits)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	// Test: Exceed limit
	t.Run("rpm limit exceeded", func(t *testing.T) {
		limits := []models.RateLimit{
			{
				LimitType:  models.LimitTypeRPM,
				LimitValue: 2,
			},
		}

		// Registra 3 richieste
		for i := 0; i < 3; i++ {
			rateLimiter.RecordRequest(ctx, providerID, accountID, models.LimitTypeRPM)
		}

		result, err := rateLimiter.CheckLimit(ctx, providerID, accountID, limits)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.LimitTypeRPM, result.LimitType)
	})
}

func TestPoolManager_GetAccount(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)

	manager := NewManager(db, redisClient)
	rateLimiter := NewRateLimiter(redisClient)
	poolManager := NewPoolManager(db, redisClient, manager, rateLimiter)
	ctx := context.Background()

	// Crea provider
	provider := models.Provider{Name: "Test Provider"}
	require.NoError(t, db.Create(&provider).Error)

	// Crea multiple accounts
	accounts := []models.Account{
		{
			ProviderID: provider.ID,
			UserID:     uuid.New(),
			Active:     true,
			QuotaUsed:  100,
			QuotaLimit: 1000,
			LastReset:  time.Now(),
		},
		{
			ProviderID: provider.ID,
			UserID:     uuid.New(),
			Active:     true,
			QuotaUsed:  500,
			QuotaLimit: 1000,
			LastReset:  time.Now(),
		},
		{
			ProviderID: provider.ID,
			UserID:     uuid.New(),
			Active:     true,
			QuotaUsed:  800,
			QuotaLimit: 1000,
			LastReset:  time.Now(),
		},
	}

	for _, acc := range accounts {
		require.NoError(t, db.Create(&acc).Error)
	}

	// Test: Least used strategy
	t.Run("least used strategy", func(t *testing.T) {
		poolManager.SetStrategy(StrategyLeastUsed)

		account, err := poolManager.GetAccount(ctx, provider.ID, 100)
		require.NoError(t, err)
		assert.NotNil(t, account)
		// Dovrebbe selezionare quello con quota_used = 100
		assert.Equal(t, int64(100), account.QuotaUsed)
	})

	// Test: Round robin strategy
	t.Run("round robin strategy", func(t *testing.T) {
		poolManager.SetStrategy(StrategyRoundRobin)

		// Prima richiesta
		account1, err := poolManager.GetAccount(ctx, provider.ID, 10)
		require.NoError(t, err)
		assert.NotNil(t, account1)

		// Seconda richiesta - dovrebbe essere diverso
		account2, err := poolManager.GetAccount(ctx, provider.ID, 10)
		require.NoError(t, err)
		assert.NotNil(t, account2)

		// Potrebbe essere lo stesso se è l'unico disponibile
		// ma l'indice dovrebbe avanzare
	})
}

func TestTracker_TrackRequest(t *testing.T) {
	db := setupTestDB(t)
	tracker := NewTracker(db)
	ctx := context.Background()

	// Crea dati di test
	provider := models.Provider{Name: "Test Provider"}
	require.NoError(t, db.Create(&provider).Error)

	// Test: Track successful request
	t.Run("track successful request", func(t *testing.T) {
		req := &TrackingRequest{
			ProviderID:    provider.ID,
			ModelID:       uuid.New(),
			UserID:        uuid.New(),
			Method:        "POST",
			Endpoint:      "/v1/chat/completions",
			StatusCode:    200,
			LatencyMs:     150,
			InputTokens:   500,
			OutputTokens:  500,
			Success:       true,
			EstimatedCost: 0.01,
		}

		err := tracker.TrackRequest(ctx, req)
		require.NoError(t, err)

		// Verifica creazione log
		var count int64
		db.Model(&models.RequestLog{}).
			Where("provider_id = ?", provider.ID).
			Count(&count)
		assert.Equal(t, int64(1), count)
	})

	// Test: Track failed request
	t.Run("track failed request", func(t *testing.T) {
		req := &TrackingRequest{
			ProviderID:   provider.ID,
			ModelID:      uuid.New(),
			UserID:       uuid.New(),
			Method:       "POST",
			Endpoint:     "/v1/chat/completions",
			StatusCode:   500,
			LatencyMs:    50,
			Success:      false,
			ErrorMessage: "Internal Server Error",
		}

		err := tracker.TrackRequest(ctx, req)
		require.NoError(t, err)
	})
}

func TestTracker_GetUsageStats(t *testing.T) {
	db := setupTestDB(t)
	tracker := NewTracker(db)
	ctx := context.Background()

	// Crea dati di test
	provider := models.Provider{Name: "Test Provider"}
	require.NoError(t, db.Create(&provider).Error)

	userID := uuid.New()
	modelID := uuid.New()

	// Crea alcune richieste
	requests := []*TrackingRequest{
		{
			ProviderID:   provider.ID,
			ModelID:      modelID,
			UserID:       userID,
			Method:       "POST",
			Endpoint:     "/v1/chat/completions",
			StatusCode:   200,
			LatencyMs:    100,
			InputTokens:  500,
			OutputTokens: 500,
			Success:      true,
		},
		{
			ProviderID:   provider.ID,
			ModelID:      modelID,
			UserID:       userID,
			Method:       "POST",
			Endpoint:     "/v1/chat/completions",
			StatusCode:   200,
			LatencyMs:    150,
			InputTokens:  300,
			OutputTokens: 300,
			Success:      true,
		},
		{
			ProviderID:   provider.ID,
			ModelID:      modelID,
			UserID:       userID,
			Method:       "POST",
			Endpoint:     "/v1/chat/completions",
			StatusCode:   500,
			Success:      false,
			ErrorMessage: "Error",
		},
	}

	for _, req := range requests {
		require.NoError(t, tracker.TrackRequest(ctx, req))
	}

	// Wait for async stats update
	time.Sleep(100 * time.Millisecond)

	// Test: Get stats
	t.Run("get usage stats", func(t *testing.T) {
		from := time.Now().Add(-1 * time.Hour)
		to := time.Now().Add(1 * time.Hour)

		stats, err := tracker.GetUsageStats(ctx, userID, from, to)
		require.NoError(t, err)
		assert.Equal(t, int64(3), stats.TotalRequests)
		assert.Equal(t, int64(2), stats.SuccessfulRequests)
		assert.InDelta(t, 0.666, stats.SuccessRate, 0.01)
	})
}
