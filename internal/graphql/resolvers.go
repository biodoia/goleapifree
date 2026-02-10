package graphql

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Resolver is the root resolver
type Resolver struct {
	db     *database.DB
	loader *DataLoader
}

// NewResolver creates a new GraphQL resolver
func NewResolver(db *database.DB) *Resolver {
	return &Resolver{
		db:     db,
		loader: NewDataLoader(db),
	}
}

// ================================================================================
// Query Resolvers
// ================================================================================

// Providers resolver with filtering and pagination
func (r *Resolver) Providers(ctx context.Context, filter *ProviderFilterInput, sort *ProviderSortInput, limit *int, offset *int) (*ProviderConnection, error) {
	query := r.db.DB.Model(&models.Provider{})

	// Apply filters
	if filter != nil {
		if filter.Type != nil {
			query = query.Where("type = ?", *filter.Type)
		}
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.Tier != nil {
			query = query.Where("tier = ?", *filter.Tier)
		}
		if filter.SupportStreaming != nil {
			query = query.Where("supports_streaming = ?", *filter.SupportStreaming)
		}
		if filter.MinHealthScore != nil {
			query = query.Where("health_score >= ?", *filter.MinHealthScore)
		}
		if filter.Search != nil && *filter.Search != "" {
			searchTerm := "%" + *filter.Search + "%"
			query = query.Where("name LIKE ? OR base_url LIKE ?", searchTerm, searchTerm)
		}
	}

	// Count total
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}

	// Apply sorting
	if sort != nil {
		orderBy := string(sort.Field)
		if sort.Direction == SortDirectionDesc {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
		query = query.Order(orderBy)
	} else {
		query = query.Order("name ASC")
	}

	// Apply pagination
	if offset != nil {
		query = query.Offset(*offset)
	}
	if limit != nil {
		query = query.Limit(*limit)
	} else {
		query = query.Limit(50)
	}

	var providers []models.Provider
	if err := query.Preload("Models").Preload("RateLimits").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch providers: %w", err)
	}

	// Build edges
	edges := make([]*ProviderEdge, len(providers))
	for i, provider := range providers {
		edges[i] = &ProviderEdge{
			Node:   &providers[i],
			Cursor: provider.ID.String(),
		}
	}

	// Build page info
	pageInfo := &PageInfo{
		HasNextPage:     false,
		HasPreviousPage: false,
	}

	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		lastCursor := edges[len(edges)-1].Cursor
		pageInfo.EndCursor = &lastCursor

		if offset != nil && *offset > 0 {
			pageInfo.HasPreviousPage = true
		}
		if limit != nil && int64(*offset+*limit) < totalCount {
			pageInfo.HasNextPage = true
		}
	}

	return &ProviderConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: int(totalCount),
	}, nil
}

// Provider resolver by ID
func (r *Resolver) Provider(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	return r.loader.LoadProvider(ctx, id)
}

// ProviderByName resolver
func (r *Resolver) ProviderByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.DB.Where("name = ?", name).
		Preload("Models").
		Preload("RateLimits").
		First(&provider).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch provider: %w", err)
	}
	return &provider, nil
}

// Models resolver with filtering and pagination
func (r *Resolver) Models(ctx context.Context, filter *ModelFilterInput, sort *ModelSortInput, limit *int, offset *int) (*ModelConnection, error) {
	query := r.db.DB.Model(&models.Model{})

	// Apply filters
	if filter != nil {
		if filter.ProviderID != nil {
			query = query.Where("provider_id = ?", *filter.ProviderID)
		}
		if filter.Modality != nil {
			query = query.Where("modality = ?", *filter.Modality)
		}
		if filter.IsFree != nil && *filter.IsFree {
			query = query.Where("input_price_per_1k = 0 AND output_price_per_1k = 0")
		}
		if filter.MinQualityScore != nil {
			query = query.Where("quality_score >= ?", *filter.MinQualityScore)
		}
		if filter.MinContextLength != nil {
			query = query.Where("context_length >= ?", *filter.MinContextLength)
		}
		if filter.Search != nil && *filter.Search != "" {
			searchTerm := "%" + *filter.Search + "%"
			query = query.Where("name LIKE ? OR display_name LIKE ? OR description LIKE ?",
				searchTerm, searchTerm, searchTerm)
		}
	}

	// Count total
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count models: %w", err)
	}

	// Apply sorting
	if sort != nil {
		orderBy := string(sort.Field)
		if sort.Direction == SortDirectionDesc {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
		query = query.Order(orderBy)
	} else {
		query = query.Order("name ASC")
	}

	// Apply pagination
	if offset != nil {
		query = query.Offset(*offset)
	}
	if limit != nil {
		query = query.Limit(*limit)
	} else {
		query = query.Limit(100)
	}

	var models []models.Model
	if err := query.Preload("Provider").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	// Build edges
	edges := make([]*ModelEdge, len(models))
	for i := range models {
		edges[i] = &ModelEdge{
			Node:   &models[i],
			Cursor: models[i].ID.String(),
		}
	}

	// Build page info
	pageInfo := &PageInfo{
		HasNextPage:     false,
		HasPreviousPage: false,
	}

	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		lastCursor := edges[len(edges)-1].Cursor
		pageInfo.EndCursor = &lastCursor

		if offset != nil && *offset > 0 {
			pageInfo.HasPreviousPage = true
		}
		if limit != nil && int64(*offset+*limit) < totalCount {
			pageInfo.HasNextPage = true
		}
	}

	return &ModelConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: int(totalCount),
	}, nil
}

// Model resolver by ID
func (r *Resolver) Model(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	return r.loader.LoadModel(ctx, id)
}

// ModelsByProvider resolver
func (r *Resolver) ModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]*models.Model, error) {
	var models []*models.Model
	if err := r.db.DB.Where("provider_id = ?", providerID).
		Preload("Provider").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	return models, nil
}

// Stats resolver
func (r *Resolver) Stats(ctx context.Context, timeRange TimeRangeInput) (*GlobalStats, error) {
	stats := &GlobalStats{
		TimeRange: &timeRange,
		Timestamp: time.Now(),
	}

	// Count providers
	var totalProviders, activeProviders int64
	r.db.DB.Model(&models.Provider{}).Count(&totalProviders)
	r.db.DB.Model(&models.Provider{}).Where("status = ?", models.ProviderStatusActive).Count(&activeProviders)

	stats.TotalProviders = int(totalProviders)
	stats.ActiveProviders = int(activeProviders)

	// Count models
	var totalModels int64
	r.db.DB.Model(&models.Model{}).Count(&totalModels)
	stats.TotalModels = int(totalModels)

	// Aggregate request stats
	var requestStats struct {
		TotalRequests      int64
		SuccessfulRequests int64
		TotalTokens        int64
		AvgLatencyMs       float64
		CostSaved          float64
	}

	query := r.db.DB.Model(&models.RequestLog{}).
		Where("timestamp BETWEEN ? AND ?", timeRange.Start, timeRange.End)

	query.Count(&requestStats.TotalRequests)
	query.Where("success = ?", true).Count(&requestStats.SuccessfulRequests)

	r.db.DB.Model(&models.RequestLog{}).
		Where("timestamp BETWEEN ? AND ?", timeRange.Start, timeRange.End).
		Select("COALESCE(SUM(input_tokens + output_tokens), 0) as total_tokens, "+
			"COALESCE(AVG(latency_ms), 0) as avg_latency_ms, "+
			"COALESCE(SUM(estimated_cost), 0) as cost_saved").
		Scan(&requestStats)

	stats.TotalRequests = int(requestStats.TotalRequests)
	stats.SuccessfulRequests = int(requestStats.SuccessfulRequests)
	stats.FailedRequests = int(requestStats.TotalRequests - requestStats.SuccessfulRequests)
	stats.TotalTokens = int(requestStats.TotalTokens)
	stats.AvgLatencyMs = requestStats.AvgLatencyMs
	stats.CostSaved = requestStats.CostSaved

	if requestStats.TotalRequests > 0 {
		stats.SuccessRate = float64(requestStats.SuccessfulRequests) / float64(requestStats.TotalRequests)
	}

	return stats, nil
}

// ProviderStats resolver
func (r *Resolver) ProviderStats(ctx context.Context, providerID uuid.UUID, timeRange TimeRangeInput) (*models.ProviderStats, error) {
	var stats models.ProviderStats

	if err := r.db.DB.Where("provider_id = ? AND timestamp BETWEEN ? AND ?",
		providerID, timeRange.Start, timeRange.End).
		Order("timestamp DESC").
		First(&stats).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return empty stats
			return &models.ProviderStats{
				ProviderID: providerID,
				Timestamp:  time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to fetch provider stats: %w", err)
	}

	return &stats, nil
}

// RealtimeMetrics resolver
func (r *Resolver) RealtimeMetrics(ctx context.Context) (*RealtimeMetrics, error) {
	metrics := &RealtimeMetrics{
		Timestamp: time.Now(),
	}

	// Calculate requests per second (last minute)
	oneMinuteAgo := time.Now().Add(-time.Minute)
	var requestCount int64
	r.db.DB.Model(&models.RequestLog{}).
		Where("timestamp > ?", oneMinuteAgo).
		Count(&requestCount)

	metrics.RequestsPerSecond = float64(requestCount) / 60.0

	// Get provider metrics
	var providers []models.Provider
	r.db.DB.Where("status = ?", models.ProviderStatusActive).
		Find(&providers)

	providerMetrics := make([]*ProviderRealtimeMetrics, len(providers))
	for i, provider := range providers {
		providerMetrics[i] = &ProviderRealtimeMetrics{
			ProviderID:          provider.ID,
			ProviderName:        provider.Name,
			Status:              string(provider.Status),
			CurrentLatencyMs:    provider.AvgLatencyMs,
			RequestsInProgress:  0, // Would need real-time tracking
			HealthScore:         provider.HealthScore,
		}
	}

	metrics.ProviderMetrics = providerMetrics

	return metrics, nil
}

// Health resolver
func (r *Resolver) Health(ctx context.Context) (*HealthStatus, error) {
	health := &HealthStatus{
		Status:    "healthy",
		Version:   "1.0.0",
		Uptime:    int(time.Since(startTime).Seconds()),
		Timestamp: time.Now(),
	}

	// Check database
	sqlDB, err := r.db.DB.DB()
	if err == nil && sqlDB.Ping() == nil {
		health.Database = &ComponentHealth{
			Status:  "healthy",
			Message: stringPtr("Connected"),
		}
	} else {
		health.Database = &ComponentHealth{
			Status:  "unhealthy",
			Message: stringPtr("Connection failed"),
		}
		health.Status = "degraded"
	}

	// Check cache (placeholder)
	health.Cache = &ComponentHealth{
		Status:  "healthy",
		Message: stringPtr("OK"),
	}

	// Check providers
	var activeCount int64
	r.db.DB.Model(&models.Provider{}).
		Where("status = ?", models.ProviderStatusActive).
		Count(&activeCount)

	if activeCount > 0 {
		health.Providers = &ComponentHealth{
			Status:  "healthy",
			Message: stringPtr(fmt.Sprintf("%d active providers", activeCount)),
		}
	} else {
		health.Providers = &ComponentHealth{
			Status:  "degraded",
			Message: stringPtr("No active providers"),
		}
	}

	return health, nil
}

// ProviderHealth resolver
func (r *Resolver) ProviderHealth(ctx context.Context, providerID uuid.UUID) (*ProviderHealthStatus, error) {
	var provider models.Provider
	if err := r.db.DB.First(&provider, "id = ?", providerID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch provider: %w", err)
	}

	// Count consecutive failures
	var consecutiveFailures int64
	r.db.DB.Model(&models.RequestLog{}).
		Where("provider_id = ? AND success = ?", providerID, false).
		Where("timestamp > ?", time.Now().Add(-5*time.Minute)).
		Count(&consecutiveFailures)

	// Find last successful request
	var lastLog models.RequestLog
	err := r.db.DB.Where("provider_id = ? AND success = ?", providerID, true).
		Order("timestamp DESC").
		First(&lastLog).Error

	var lastSuccess *time.Time
	if err == nil {
		lastSuccess = &lastLog.Timestamp
	}

	return &ProviderHealthStatus{
		ProviderID:          provider.ID,
		ProviderName:        provider.Name,
		IsHealthy:           provider.IsAvailable(),
		HealthScore:         provider.HealthScore,
		LatencyMs:           provider.AvgLatencyMs,
		LastCheck:           provider.LastHealthCheck,
		LastSuccess:         lastSuccess,
		ConsecutiveFailures: int(consecutiveFailures),
		Details:             stringPtr(fmt.Sprintf("Status: %s", provider.Status)),
	}, nil
}

// ================================================================================
// Mutation Resolvers
// ================================================================================

// CreateProvider mutation
func (r *Resolver) CreateProvider(ctx context.Context, input CreateProviderInput) (*models.Provider, error) {
	provider := &models.Provider{
		Name:              input.Name,
		Type:              models.ProviderType(input.Type),
		BaseURL:           input.BaseURL,
		AuthType:          models.AuthType(input.AuthType),
		Tier:              input.Tier,
		Source:            input.Source,
		Status:            models.ProviderStatusActive,
		SupportsStreaming: input.SupportsStreaming,
		SupportsTools:     input.SupportsTools,
		SupportsJSON:      input.SupportsJSON,
		DiscoveredAt:      time.Now(),
		HealthScore:       1.0,
	}

	if err := r.db.DB.Create(provider).Error; err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	return provider, nil
}

// UpdateProvider mutation
func (r *Resolver) UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.DB.First(&provider, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to find provider: %w", err)
	}

	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Type != nil {
		updates["type"] = *input.Type
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.BaseURL != nil {
		updates["base_url"] = *input.BaseURL
	}
	if input.AuthType != nil {
		updates["auth_type"] = *input.AuthType
	}
	if input.Tier != nil {
		updates["tier"] = *input.Tier
	}
	if input.SupportsStreaming != nil {
		updates["supports_streaming"] = *input.SupportsStreaming
	}
	if input.SupportsTools != nil {
		updates["supports_tools"] = *input.SupportsTools
	}
	if input.SupportsJSON != nil {
		updates["supports_json"] = *input.SupportsJSON
	}

	if err := r.db.DB.Model(&provider).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update provider: %w", err)
	}

	return &provider, nil
}

// DeleteProvider mutation
func (r *Resolver) DeleteProvider(ctx context.Context, id uuid.UUID) (bool, error) {
	if err := r.db.DB.Delete(&models.Provider{}, "id = ?", id).Error; err != nil {
		return false, fmt.Errorf("failed to delete provider: %w", err)
	}
	return true, nil
}

// Helper functions
var startTime = time.Now()

func stringPtr(s string) *string {
	return &s
}
