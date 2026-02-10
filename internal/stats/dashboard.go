package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
)

// Dashboard fornisce dati per il dashboard
type Dashboard struct {
	db         *database.DB
	collector  *Collector
	aggregator *Aggregator
}

// NewDashboard crea un nuovo dashboard
func NewDashboard(db *database.DB, collector *Collector, aggregator *Aggregator) *Dashboard {
	return &Dashboard{
		db:         db,
		collector:  collector,
		aggregator: aggregator,
	}
}

// DashboardData rappresenta tutti i dati del dashboard
type DashboardData struct {
	Summary          *SummaryStats         `json:"summary"`
	ProviderStats    []*ProviderDashStats  `json:"provider_stats"`
	HourlyTrends     []*TrendPoint         `json:"hourly_trends"`
	DailyTrends      []*TrendPoint         `json:"daily_trends"`
	CostSavings      *CostSavingsData      `json:"cost_savings"`
	TopProviders     []*ProviderRanking    `json:"top_providers"`
	RecentErrors     []*ErrorSummary       `json:"recent_errors"`
	PerformanceChart *PerformanceChartData `json:"performance_chart"`
}

// SummaryStats statistiche di riepilogo
type SummaryStats struct {
	TotalRequests    int64   `json:"total_requests"`
	TotalProviders   int     `json:"total_providers"`
	ActiveProviders  int     `json:"active_providers"`
	AvgSuccessRate   float64 `json:"avg_success_rate"`
	AvgLatencyMs     int     `json:"avg_latency_ms"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalCostSaved   float64 `json:"total_cost_saved"`
	RequestsToday    int64   `json:"requests_today"`
	RequestsThisHour int64   `json:"requests_this_hour"`
}

// ProviderDashStats statistiche dettagliate per provider
type ProviderDashStats struct {
	ProviderID      uuid.UUID `json:"provider_id"`
	ProviderName    string    `json:"provider_name"`
	Status          string    `json:"status"`
	HealthScore     float64   `json:"health_score"`
	TotalRequests   int64     `json:"total_requests"`
	SuccessRate     float64   `json:"success_rate"`
	AvgLatencyMs    int       `json:"avg_latency_ms"`
	ErrorRate       float64   `json:"error_rate"`
	TotalTokens     int64     `json:"total_tokens"`
	CostSaved       float64   `json:"cost_saved"`
	LastRequestTime time.Time `json:"last_request_time"`
	Trend           string    `json:"trend"` // "up", "down", "stable"
}

// TrendPoint punto dati per grafici trend
type TrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	Requests     int64     `json:"requests"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatencyMs int       `json:"avg_latency_ms"`
	Errors       int64     `json:"errors"`
}

// CostSavingsData dati sui risparmi
type CostSavingsData struct {
	TotalSaved       float64            `json:"total_saved"`
	SavingsToday     float64            `json:"savings_today"`
	SavingsThisWeek  float64            `json:"savings_this_week"`
	SavingsThisMonth float64            `json:"savings_this_month"`
	ByProvider       map[string]float64 `json:"by_provider"`
	ComparedToPricing map[string]float64 `json:"compared_to_pricing"` // Risparmio per modello
}

// ProviderRanking classifica dei provider
type ProviderRanking struct {
	Rank         int     `json:"rank"`
	ProviderName string  `json:"provider_name"`
	Score        float64 `json:"score"` // Score composito
	Requests     int64   `json:"requests"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

// ErrorSummary riepilogo errori
type ErrorSummary struct {
	Timestamp    time.Time `json:"timestamp"`
	ProviderName string    `json:"provider_name"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int64     `json:"count"`
}

// PerformanceChartData dati per grafico performance
type PerformanceChartData struct {
	Labels     []string          `json:"labels"`
	Datasets   []*ChartDataset   `json:"datasets"`
}

// ChartDataset dataset per chart
type ChartDataset struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

// GetDashboardData ottiene tutti i dati del dashboard
func (d *Dashboard) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	summary, err := d.GetSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	providerStats, err := d.GetProviderStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider stats: %w", err)
	}

	hourlyTrends, err := d.GetHourlyTrends(ctx, 24)
	if err != nil {
		return nil, fmt.Errorf("failed to get hourly trends: %w", err)
	}

	dailyTrends, err := d.GetDailyTrends(ctx, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily trends: %w", err)
	}

	costSavings, err := d.GetCostSavings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost savings: %w", err)
	}

	topProviders := d.GetTopProviders(providerStats, 5)

	recentErrors, err := d.GetRecentErrors(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent errors: %w", err)
	}

	perfChart, err := d.GetPerformanceChart(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance chart: %w", err)
	}

	return &DashboardData{
		Summary:          summary,
		ProviderStats:    providerStats,
		HourlyTrends:     hourlyTrends,
		DailyTrends:      dailyTrends,
		CostSavings:      costSavings,
		TopProviders:     topProviders,
		RecentErrors:     recentErrors,
		PerformanceChart: perfChart,
	}, nil
}

// GetSummary ottiene statistiche di riepilogo
func (d *Dashboard) GetSummary(ctx context.Context) (*SummaryStats, error) {
	var totalProviders int64
	var activeProviders int64

	d.db.WithContext(ctx).Model(&models.Provider{}).Count(&totalProviders)
	d.db.WithContext(ctx).Model(&models.Provider{}).
		Where("status = ?", models.ProviderStatusActive).
		Count(&activeProviders)

	// Aggregate from collector
	allMetrics := d.collector.GetAllMetrics()

	var (
		totalRequests  int64
		totalTokens    int64
		totalCostSaved float64
		totalSuccesses int64
		totalLatencyMs int64
	)

	for _, metrics := range allMetrics {
		totalRequests += metrics.TotalRequests
		totalTokens += metrics.TotalTokens
		totalCostSaved += metrics.TotalCost
		totalSuccesses += metrics.SuccessCount
		totalLatencyMs += metrics.TotalLatencyMs
	}

	avgSuccessRate := 0.0
	avgLatency := 0
	if totalRequests > 0 {
		avgSuccessRate = float64(totalSuccesses) / float64(totalRequests)
		avgLatency = int(totalLatencyMs / totalRequests)
	}

	// Get today's requests
	requestsToday, _ := d.getRequestsInPeriod(ctx, 24*time.Hour)
	requestsThisHour, _ := d.getRequestsInPeriod(ctx, time.Hour)

	return &SummaryStats{
		TotalRequests:    totalRequests,
		TotalProviders:   int(totalProviders),
		ActiveProviders:  int(activeProviders),
		AvgSuccessRate:   avgSuccessRate,
		AvgLatencyMs:     avgLatency,
		TotalTokens:      totalTokens,
		TotalCostSaved:   totalCostSaved,
		RequestsToday:    requestsToday,
		RequestsThisHour: requestsThisHour,
	}, nil
}

// GetProviderStats ottiene statistiche per ogni provider
func (d *Dashboard) GetProviderStats(ctx context.Context) ([]*ProviderDashStats, error) {
	var providers []models.Provider
	if err := d.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, err
	}

	stats := make([]*ProviderDashStats, 0, len(providers))

	for _, provider := range providers {
		metrics := d.collector.GetProviderMetrics(provider.ID)
		if metrics == nil {
			metrics = &AggregatedMetrics{ProviderID: provider.ID}
		}

		successRate := 0.0
		errorRate := 0.0
		avgLatency := 0
		if metrics.TotalRequests > 0 {
			successRate = float64(metrics.SuccessCount) / float64(metrics.TotalRequests)
			errorRate = float64(metrics.ErrorCount) / float64(metrics.TotalRequests)
			avgLatency = int(metrics.TotalLatencyMs / metrics.TotalRequests)
		}

		// Calculate trend
		trend := d.calculateTrend(ctx, provider.ID)

		stat := &ProviderDashStats{
			ProviderID:      provider.ID,
			ProviderName:    provider.Name,
			Status:          string(provider.Status),
			HealthScore:     provider.HealthScore,
			TotalRequests:   metrics.TotalRequests,
			SuccessRate:     successRate,
			AvgLatencyMs:    avgLatency,
			ErrorRate:       errorRate,
			TotalTokens:     metrics.TotalTokens,
			CostSaved:       metrics.TotalCost,
			LastRequestTime: metrics.LastUpdated,
			Trend:           trend,
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// GetHourlyTrends ottiene trend orari
func (d *Dashboard) GetHourlyTrends(ctx context.Context, hours int) ([]*TrendPoint, error) {
	trends := make([]*TrendPoint, 0, hours)
	now := time.Now()

	for i := hours - 1; i >= 0; i-- {
		hourStart := now.Add(time.Duration(-i) * time.Hour).Truncate(time.Hour)
		hourEnd := hourStart.Add(time.Hour)

		point, err := d.getTrendPoint(ctx, hourStart, hourEnd)
		if err != nil {
			return nil, err
		}

		trends = append(trends, point)
	}

	return trends, nil
}

// GetDailyTrends ottiene trend giornalieri
func (d *Dashboard) GetDailyTrends(ctx context.Context, days int) ([]*TrendPoint, error) {
	trends := make([]*TrendPoint, 0, days)
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		dayStart := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		dayEnd := dayStart.AddDate(0, 0, 1)

		point, err := d.getTrendPoint(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}

		trends = append(trends, point)
	}

	return trends, nil
}

// getTrendPoint calcola un punto del trend per un periodo
func (d *Dashboard) getTrendPoint(ctx context.Context, start, end time.Time) (*TrendPoint, error) {
	var stats []models.ProviderStats
	if err := d.db.WithContext(ctx).
		Where("timestamp >= ? AND timestamp < ?", start, end).
		Find(&stats).Error; err != nil {
		return nil, err
	}

	var (
		totalRequests  int64
		successCount   int64
		totalLatencyMs int64
		errorCount     int64
	)

	for _, stat := range stats {
		totalRequests += stat.TotalRequests
		successCount += int64(float64(stat.TotalRequests) * stat.SuccessRate)
		totalLatencyMs += int64(stat.AvgLatencyMs) * stat.TotalRequests
		errorCount += stat.ErrorCount
	}

	successRate := 0.0
	avgLatency := 0
	if totalRequests > 0 {
		successRate = float64(successCount) / float64(totalRequests)
		avgLatency = int(totalLatencyMs / totalRequests)
	}

	return &TrendPoint{
		Timestamp:    start,
		Requests:     totalRequests,
		SuccessRate:  successRate,
		AvgLatencyMs: avgLatency,
		Errors:       errorCount,
	}, nil
}

// GetCostSavings calcola i risparmi
func (d *Dashboard) GetCostSavings(ctx context.Context) (*CostSavingsData, error) {
	var providers []models.Provider
	if err := d.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, err
	}

	byProvider := make(map[string]float64)
	var totalSaved float64

	for _, provider := range providers {
		if metrics := d.collector.GetProviderMetrics(provider.ID); metrics != nil {
			byProvider[provider.Name] = metrics.TotalCost
			totalSaved += metrics.TotalCost
		}
	}

	savingsToday, _ := d.getCostSavingsInPeriod(ctx, 24*time.Hour)
	savingsWeek, _ := d.getCostSavingsInPeriod(ctx, 7*24*time.Hour)
	savingsMonth, _ := d.getCostSavingsInPeriod(ctx, 30*24*time.Hour)

	return &CostSavingsData{
		TotalSaved:       totalSaved,
		SavingsToday:     savingsToday,
		SavingsThisWeek:  savingsWeek,
		SavingsThisMonth: savingsMonth,
		ByProvider:       byProvider,
		ComparedToPricing: d.calculateComparedPricing(ctx),
	}, nil
}

// GetTopProviders ottiene i top provider per performance
func (d *Dashboard) GetTopProviders(providerStats []*ProviderDashStats, limit int) []*ProviderRanking {
	// Calculate composite score: (successRate * 0.4) + (1 - normalizedLatency * 0.3) + (healthScore * 0.3)
	rankings := make([]*ProviderRanking, 0, len(providerStats))

	// Find max latency for normalization
	maxLatency := 0
	for _, stat := range providerStats {
		if stat.AvgLatencyMs > maxLatency {
			maxLatency = stat.AvgLatencyMs
		}
	}
	if maxLatency == 0 {
		maxLatency = 1
	}

	for _, stat := range providerStats {
		if stat.TotalRequests == 0 {
			continue
		}

		normalizedLatency := float64(stat.AvgLatencyMs) / float64(maxLatency)
		score := (stat.SuccessRate * 0.4) + ((1.0 - normalizedLatency) * 0.3) + (stat.HealthScore * 0.3)

		rankings = append(rankings, &ProviderRanking{
			ProviderName: stat.ProviderName,
			Score:        score,
			Requests:     stat.TotalRequests,
			SuccessRate:  stat.SuccessRate,
			AvgLatencyMs: stat.AvgLatencyMs,
		})
	}

	// Sort by score
	for i := 0; i < len(rankings)-1; i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[j].Score > rankings[i].Score {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}

	// Assign ranks and limit
	for i := range rankings {
		rankings[i].Rank = i + 1
		if i+1 >= limit {
			rankings = rankings[:i+1]
			break
		}
	}

	return rankings
}

// GetRecentErrors ottiene errori recenti
func (d *Dashboard) GetRecentErrors(ctx context.Context, limit int) ([]*ErrorSummary, error) {
	var logs []models.RequestLog
	if err := d.db.WithContext(ctx).
		Where("success = ?", false).
		Order("timestamp DESC").
		Limit(limit).
		Preload("Provider").
		Find(&logs).Error; err != nil {
		return nil, err
	}

	errors := make([]*ErrorSummary, 0, len(logs))
	for _, log := range logs {
		errorType := "unknown"
		if log.StatusCode == 429 {
			errorType = "rate_limit"
		} else if log.StatusCode == 504 || log.StatusCode == 408 {
			errorType = "timeout"
		} else if log.StatusCode >= 500 {
			errorType = "server_error"
		} else if log.StatusCode >= 400 {
			errorType = "client_error"
		}

		errors = append(errors, &ErrorSummary{
			Timestamp:    log.Timestamp,
			ProviderName: "", // Need to load provider
			ErrorType:    errorType,
			ErrorMessage: log.ErrorMessage,
			Count:        1,
		})
	}

	return errors, nil
}

// GetPerformanceChart ottiene dati per grafico performance
func (d *Dashboard) GetPerformanceChart(ctx context.Context) (*PerformanceChartData, error) {
	hourlyTrends, err := d.GetHourlyTrends(ctx, 12)
	if err != nil {
		return nil, err
	}

	labels := make([]string, len(hourlyTrends))
	requestsData := make([]float64, len(hourlyTrends))
	successRateData := make([]float64, len(hourlyTrends))
	latencyData := make([]float64, len(hourlyTrends))

	for i, point := range hourlyTrends {
		labels[i] = point.Timestamp.Format("15:04")
		requestsData[i] = float64(point.Requests)
		successRateData[i] = point.SuccessRate * 100
		latencyData[i] = float64(point.AvgLatencyMs)
	}

	return &PerformanceChartData{
		Labels: labels,
		Datasets: []*ChartDataset{
			{Label: "Requests", Data: requestsData},
			{Label: "Success Rate (%)", Data: successRateData},
			{Label: "Avg Latency (ms)", Data: latencyData},
		},
	}, nil
}

// Helper functions

func (d *Dashboard) getRequestsInPeriod(ctx context.Context, period time.Duration) (int64, error) {
	cutoff := time.Now().Add(-period)
	var count int64
	err := d.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("timestamp >= ?", cutoff).
		Count(&count).Error
	return count, err
}

func (d *Dashboard) getCostSavingsInPeriod(ctx context.Context, period time.Duration) (float64, error) {
	cutoff := time.Now().Add(-period)
	var result struct {
		Total float64
	}
	err := d.db.WithContext(ctx).Model(&models.RequestLog{}).
		Select("COALESCE(SUM(estimated_cost), 0) as total").
		Where("timestamp >= ?", cutoff).
		Scan(&result).Error
	return result.Total, err
}

func (d *Dashboard) calculateTrend(ctx context.Context, providerID uuid.UUID) string {
	// Compare last hour vs previous hour
	now := time.Now()
	lastHour := now.Add(-time.Hour)
	prevHour := now.Add(-2 * time.Hour)

	var lastCount, prevCount int64
	d.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("provider_id = ? AND timestamp >= ?", providerID, lastHour).
		Count(&lastCount)
	d.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("provider_id = ? AND timestamp >= ? AND timestamp < ?", providerID, prevHour, lastHour).
		Count(&prevCount)

	if lastCount > prevCount*11/10 { // +10%
		return "up"
	} else if lastCount < prevCount*9/10 { // -10%
		return "down"
	}
	return "stable"
}

func (d *Dashboard) calculateComparedPricing(ctx context.Context) map[string]float64 {
	// This would compare with official API pricing
	// For now, return empty map
	return make(map[string]float64)
}
