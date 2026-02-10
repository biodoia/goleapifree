package optimizer

import (
	"context"
	"sort"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Analyzer analizza pattern di utilizzo e comportamenti degli utenti
type Analyzer struct {
	db *database.DB
}

// NewAnalyzer crea un nuovo analyzer
func NewAnalyzer(db *database.DB) *Analyzer {
	return &Analyzer{
		db: db,
	}
}

// UserPatterns pattern di utilizzo di un utente
type UserPatterns struct {
	UserID    uuid.UUID
	TimeRange time.Duration

	// Request patterns
	TotalRequests        int64
	AvgRequestsPerDay    float64
	AvgRequestsPerHour   float64
	PeakHours            []int // Ore con più traffico
	PeakDays             []time.Weekday

	// Token patterns
	AvgInputTokens       int
	AvgOutputTokens      int
	TotalInputTokens     int64
	TotalOutputTokens    int64
	MaxInputTokens       int
	MaxOutputTokens      int

	// Cost patterns
	TotalCost            float64
	AvgCostPerRequest    float64
	MaxCostPerRequest    float64

	// Model preferences
	PopularModels        []ModelUsage
	PopularProviders     []ProviderUsage

	// Behavior patterns
	RepetitiveQueriesRatio float64 // % di query simili/ripetitive
	AvgPromptLength        int
	ComplexityScore        float64 // 0-1

	// Success metrics
	SuccessRate          float64
	AvgLatencyMs         int

	Timestamp time.Time
}

// ModelUsage utilizzo di un modello
type ModelUsage struct {
	ModelID      uuid.UUID
	ModelName    string
	RequestCount int64
	Percentage   float64
	AvgCost      float64
}

// ProviderUsage utilizzo di un provider
type ProviderUsage struct {
	ProviderID   uuid.UUID
	ProviderName string
	RequestCount int64
	Percentage   float64
	AvgLatency   int
	SuccessRate  float64
}

// AnalyzeUserPatterns analizza i pattern di un utente
func (a *Analyzer) AnalyzeUserPatterns(ctx context.Context, userID uuid.UUID, timeRange time.Duration) *UserPatterns {
	startTime := time.Now()
	since := time.Now().Add(-timeRange)

	// Recupera log dell'utente
	var logs []models.RequestLog
	err := a.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("timestamp > ?", since).
		Order("timestamp ASC").
		Find(&logs).Error

	if err != nil || len(logs) == 0 {
		log.Debug().
			Err(err).
			Str("user_id", userID.String()).
			Msg("No logs found for user")
		return nil
	}

	patterns := &UserPatterns{
		UserID:    userID,
		TimeRange: timeRange,
		Timestamp: time.Now(),
	}

	// Analizza i log
	a.analyzeRequestPatterns(logs, patterns)
	a.analyzeTokenPatterns(logs, patterns)
	a.analyzeCostPatterns(logs, patterns)
	a.analyzeModelPreferences(logs, patterns)
	a.analyzeBehaviorPatterns(logs, patterns)
	a.analyzeSuccessMetrics(logs, patterns)

	duration := time.Since(startTime)
	log.Debug().
		Str("user_id", userID.String()).
		Int64("requests", patterns.TotalRequests).
		Dur("duration", duration).
		Msg("User patterns analyzed")

	return patterns
}

// analyzeRequestPatterns analizza pattern delle richieste
func (a *Analyzer) analyzeRequestPatterns(logs []models.RequestLog, patterns *UserPatterns) {
	patterns.TotalRequests = int64(len(logs))

	if len(logs) == 0 {
		return
	}

	// Calcola medie temporali
	firstRequest := logs[0].Timestamp
	lastRequest := logs[len(logs)-1].Timestamp
	duration := lastRequest.Sub(firstRequest)

	if duration.Hours() > 0 {
		patterns.AvgRequestsPerHour = float64(len(logs)) / duration.Hours()
		patterns.AvgRequestsPerDay = patterns.AvgRequestsPerHour * 24
	}

	// Analizza distribuzione oraria
	hourCounts := make(map[int]int)
	dayCounts := make(map[time.Weekday]int)

	for _, log := range logs {
		hour := log.Timestamp.Hour()
		hourCounts[hour]++

		day := log.Timestamp.Weekday()
		dayCounts[day]++
	}

	// Trova peak hours (top 3 ore)
	type hourCount struct {
		hour  int
		count int
	}
	hourSlice := make([]hourCount, 0, len(hourCounts))
	for hour, count := range hourCounts {
		hourSlice = append(hourSlice, hourCount{hour, count})
	}
	sort.Slice(hourSlice, func(i, j int) bool {
		return hourSlice[i].count > hourSlice[j].count
	})

	patterns.PeakHours = make([]int, 0, 3)
	for i := 0; i < 3 && i < len(hourSlice); i++ {
		patterns.PeakHours = append(patterns.PeakHours, hourSlice[i].hour)
	}

	// Trova peak days (top 3 giorni)
	type dayCount struct {
		day   time.Weekday
		count int
	}
	daySlice := make([]dayCount, 0, len(dayCounts))
	for day, count := range dayCounts {
		daySlice = append(daySlice, dayCount{day, count})
	}
	sort.Slice(daySlice, func(i, j int) bool {
		return daySlice[i].count > daySlice[j].count
	})

	patterns.PeakDays = make([]time.Weekday, 0, 3)
	for i := 0; i < 3 && i < len(daySlice); i++ {
		patterns.PeakDays = append(patterns.PeakDays, daySlice[i].day)
	}
}

// analyzeTokenPatterns analizza pattern dei token
func (a *Analyzer) analyzeTokenPatterns(logs []models.RequestLog, patterns *UserPatterns) {
	if len(logs) == 0 {
		return
	}

	totalInput := int64(0)
	totalOutput := int64(0)
	maxInput := 0
	maxOutput := 0

	for _, log := range logs {
		totalInput += int64(log.InputTokens)
		totalOutput += int64(log.OutputTokens)

		if log.InputTokens > maxInput {
			maxInput = log.InputTokens
		}
		if log.OutputTokens > maxOutput {
			maxOutput = log.OutputTokens
		}
	}

	patterns.TotalInputTokens = totalInput
	patterns.TotalOutputTokens = totalOutput
	patterns.AvgInputTokens = int(totalInput / int64(len(logs)))
	patterns.AvgOutputTokens = int(totalOutput / int64(len(logs)))
	patterns.MaxInputTokens = maxInput
	patterns.MaxOutputTokens = maxOutput

	// Stima prompt length (4 char per token)
	patterns.AvgPromptLength = patterns.AvgInputTokens * 4
}

// analyzeCostPatterns analizza pattern dei costi
func (a *Analyzer) analyzeCostPatterns(logs []models.RequestLog, patterns *UserPatterns) {
	if len(logs) == 0 {
		return
	}

	totalCost := 0.0
	maxCost := 0.0

	for _, log := range logs {
		totalCost += log.EstimatedCost
		if log.EstimatedCost > maxCost {
			maxCost = log.EstimatedCost
		}
	}

	patterns.TotalCost = totalCost
	patterns.AvgCostPerRequest = totalCost / float64(len(logs))
	patterns.MaxCostPerRequest = maxCost
}

// analyzeModelPreferences analizza preferenze di modelli e provider
func (a *Analyzer) analyzeModelPreferences(logs []models.RequestLog, patterns *UserPatterns) {
	if len(logs) == 0 {
		return
	}

	// Conta utilizzo per modello
	modelCounts := make(map[uuid.UUID]int64)
	modelCosts := make(map[uuid.UUID]float64)

	// Conta utilizzo per provider
	providerCounts := make(map[uuid.UUID]int64)
	providerLatencies := make(map[uuid.UUID][]int)
	providerSuccesses := make(map[uuid.UUID]int64)

	for _, log := range logs {
		modelCounts[log.ModelID]++
		modelCosts[log.ModelID] += log.EstimatedCost

		providerCounts[log.ProviderID]++
		providerLatencies[log.ProviderID] = append(providerLatencies[log.ProviderID], log.LatencyMs)
		if log.Success {
			providerSuccesses[log.ProviderID]++
		}
	}

	// Popola popular models
	patterns.PopularModels = make([]ModelUsage, 0)
	for modelID, count := range modelCounts {
		usage := ModelUsage{
			ModelID:      modelID,
			RequestCount: count,
			Percentage:   float64(count) / float64(len(logs)) * 100,
			AvgCost:      modelCosts[modelID] / float64(count),
		}

		// Ottieni nome modello dal DB
		var model models.Model
		if err := a.db.First(&model, "id = ?", modelID).Error; err == nil {
			usage.ModelName = model.Name
		}

		patterns.PopularModels = append(patterns.PopularModels, usage)
	}

	// Ordina per usage
	sort.Slice(patterns.PopularModels, func(i, j int) bool {
		return patterns.PopularModels[i].RequestCount > patterns.PopularModels[j].RequestCount
	})

	// Limita a top 5
	if len(patterns.PopularModels) > 5 {
		patterns.PopularModels = patterns.PopularModels[:5]
	}

	// Popola popular providers
	patterns.PopularProviders = make([]ProviderUsage, 0)
	for providerID, count := range providerCounts {
		// Calcola avg latency
		latencies := providerLatencies[providerID]
		sumLatency := 0
		for _, lat := range latencies {
			sumLatency += lat
		}
		avgLatency := 0
		if len(latencies) > 0 {
			avgLatency = sumLatency / len(latencies)
		}

		// Calcola success rate
		successRate := float64(providerSuccesses[providerID]) / float64(count)

		usage := ProviderUsage{
			ProviderID:   providerID,
			RequestCount: count,
			Percentage:   float64(count) / float64(len(logs)) * 100,
			AvgLatency:   avgLatency,
			SuccessRate:  successRate,
		}

		// Ottieni nome provider dal DB
		var provider models.Provider
		if err := a.db.First(&provider, "id = ?", providerID).Error; err == nil {
			usage.ProviderName = provider.Name
		}

		patterns.PopularProviders = append(patterns.PopularProviders, usage)
	}

	// Ordina per usage
	sort.Slice(patterns.PopularProviders, func(i, j int) bool {
		return patterns.PopularProviders[i].RequestCount > patterns.PopularProviders[j].RequestCount
	})

	// Limita a top 5
	if len(patterns.PopularProviders) > 5 {
		patterns.PopularProviders = patterns.PopularProviders[:5]
	}
}

// analyzeBehaviorPatterns analizza comportamenti dell'utente
func (a *Analyzer) analyzeBehaviorPatterns(logs []models.RequestLog, patterns *UserPatterns) {
	if len(logs) == 0 {
		return
	}

	// Analizza query ripetitive (basato su token count simili)
	// Semplificato: considera "ripetitive" richieste con stesso input token count
	tokenCountMap := make(map[int]int)
	for _, log := range logs {
		tokenCountMap[log.InputTokens]++
	}

	repetitiveCount := 0
	for _, count := range tokenCountMap {
		if count > 1 {
			repetitiveCount += count
		}
	}

	patterns.RepetitiveQueriesRatio = float64(repetitiveCount) / float64(len(logs))

	// Calcola complexity score (basato su distribuzione token)
	if patterns.AvgInputTokens > 0 {
		outputInputRatio := float64(patterns.AvgOutputTokens) / float64(patterns.AvgInputTokens)
		// Normalizza tra 0-1 (ratio alto = più complesso)
		patterns.ComplexityScore = min(outputInputRatio/3.0, 1.0)
	}
}

// analyzeSuccessMetrics analizza metriche di successo
func (a *Analyzer) analyzeSuccessMetrics(logs []models.RequestLog, patterns *UserPatterns) {
	if len(logs) == 0 {
		return
	}

	successCount := 0
	totalLatency := 0

	for _, log := range logs {
		if log.Success {
			successCount++
		}
		totalLatency += log.LatencyMs
	}

	patterns.SuccessRate = float64(successCount) / float64(len(logs))
	patterns.AvgLatencyMs = totalLatency / len(logs)
}

// min helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// DetectPeakHours rileva le ore di picco nel sistema
func (a *Analyzer) DetectPeakHours(ctx context.Context, duration time.Duration) []int {
	since := time.Now().Add(-duration)

	var logs []models.RequestLog
	err := a.db.WithContext(ctx).
		Where("timestamp > ?", since).
		Find(&logs).Error

	if err != nil {
		return nil
	}

	hourCounts := make(map[int]int)
	for _, log := range logs {
		hour := log.Timestamp.Hour()
		hourCounts[hour]++
	}

	// Ordina ore per count
	type hourCount struct {
		hour  int
		count int
	}
	hours := make([]hourCount, 0, len(hourCounts))
	for hour, count := range hourCounts {
		hours = append(hours, hourCount{hour, count})
	}
	sort.Slice(hours, func(i, j int) bool {
		return hours[i].count > hours[j].count
	})

	// Top 5 ore
	peakHours := make([]int, 0, 5)
	for i := 0; i < 5 && i < len(hours); i++ {
		peakHours = append(peakHours, hours[i].hour)
	}

	return peakHours
}

// AnalyzeCostTrends analizza trend dei costi nel tempo
func (a *Analyzer) AnalyzeCostTrends(ctx context.Context, duration time.Duration) (*CostTrends, error) {
	since := time.Now().Add(-duration)

	var logs []models.RequestLog
	err := a.db.WithContext(ctx).
		Where("timestamp > ?", since).
		Order("timestamp ASC").
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, ErrNoData
	}

	trends := &CostTrends{
		Duration:  duration,
		Timestamp: time.Now(),
	}

	// Calcola statistiche
	totalCost := 0.0
	minCost := logs[0].EstimatedCost
	maxCost := logs[0].EstimatedCost

	for _, log := range logs {
		totalCost += log.EstimatedCost
		if log.EstimatedCost < minCost {
			minCost = log.EstimatedCost
		}
		if log.EstimatedCost > maxCost {
			maxCost = log.EstimatedCost
		}
	}

	trends.TotalCost = totalCost
	trends.AvgCost = totalCost / float64(len(logs))
	trends.MinCost = minCost
	trends.MaxCost = maxCost

	// Calcola trend (regressione lineare semplice)
	// Dividi in buckets temporali
	bucketDuration := duration / 10 // 10 buckets
	buckets := make([]float64, 10)
	bucketCounts := make([]int, 10)

	for _, log := range logs {
		timeSinceStart := log.Timestamp.Sub(since)
		bucketIdx := int(timeSinceStart / bucketDuration)
		if bucketIdx >= 10 {
			bucketIdx = 9
		}
		buckets[bucketIdx] += log.EstimatedCost
		bucketCounts[bucketIdx]++
	}

	// Calcola medie per bucket
	for i := 0; i < 10; i++ {
		if bucketCounts[i] > 0 {
			buckets[i] /= float64(bucketCounts[i])
		}
	}

	// Trend: confronta prima e ultima metà
	firstHalfAvg := 0.0
	secondHalfAvg := 0.0
	firstHalfCount := 0
	secondHalfCount := 0

	for i := 0; i < 5; i++ {
		if bucketCounts[i] > 0 {
			firstHalfAvg += buckets[i]
			firstHalfCount++
		}
	}
	for i := 5; i < 10; i++ {
		if bucketCounts[i] > 0 {
			secondHalfAvg += buckets[i]
			secondHalfCount++
		}
	}

	if firstHalfCount > 0 {
		firstHalfAvg /= float64(firstHalfCount)
	}
	if secondHalfCount > 0 {
		secondHalfAvg /= float64(secondHalfCount)
	}

	if firstHalfAvg > 0 {
		trends.TrendPercent = ((secondHalfAvg - firstHalfAvg) / firstHalfAvg) * 100
	}

	if trends.TrendPercent > 5 {
		trends.Trend = "increasing"
	} else if trends.TrendPercent < -5 {
		trends.Trend = "decreasing"
	} else {
		trends.Trend = "stable"
	}

	trends.DataPoints = buckets

	return trends, nil
}

// CostTrends trend dei costi nel tempo
type CostTrends struct {
	Duration     time.Duration
	TotalCost    float64
	AvgCost      float64
	MinCost      float64
	MaxCost      float64
	Trend        string  // "increasing", "decreasing", "stable"
	TrendPercent float64 // % change
	DataPoints   []float64 // Serie temporale
	Timestamp    time.Time
}

// TrackPopularModels traccia i modelli più popolari
func (a *Analyzer) TrackPopularModels(ctx context.Context, duration time.Duration, limit int) ([]ModelPopularity, error) {
	since := time.Now().Add(-duration)

	var logs []models.RequestLog
	err := a.db.WithContext(ctx).
		Where("timestamp > ?", since).
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	// Aggrega per modello
	modelStats := make(map[uuid.UUID]*ModelPopularity)

	for _, log := range logs {
		stats, exists := modelStats[log.ModelID]
		if !exists {
			stats = &ModelPopularity{
				ModelID: log.ModelID,
			}
			modelStats[log.ModelID] = stats
		}

		stats.RequestCount++
		stats.TotalTokens += int64(log.InputTokens + log.OutputTokens)
		stats.TotalCost += log.EstimatedCost

		if log.Success {
			stats.SuccessCount++
		}
	}

	// Converti in slice e calcola metriche
	popular := make([]ModelPopularity, 0, len(modelStats))
	totalRequests := int64(len(logs))

	for modelID, stats := range modelStats {
		stats.Percentage = float64(stats.RequestCount) / float64(totalRequests) * 100
		stats.AvgCost = stats.TotalCost / float64(stats.RequestCount)
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.RequestCount)

		// Ottieni nome modello
		var model models.Model
		if err := a.db.First(&model, "id = ?", modelID).Error; err == nil {
			stats.ModelName = model.Name
		}

		popular = append(popular, *stats)
	}

	// Ordina per request count
	sort.Slice(popular, func(i, j int) bool {
		return popular[i].RequestCount > popular[j].RequestCount
	})

	// Limita risultati
	if limit > 0 && len(popular) > limit {
		popular = popular[:limit]
	}

	return popular, nil
}

// ModelPopularity popolarità di un modello
type ModelPopularity struct {
	ModelID      uuid.UUID
	ModelName    string
	RequestCount int64
	Percentage   float64
	TotalTokens  int64
	TotalCost    float64
	AvgCost      float64
	SuccessCount int64
	SuccessRate  float64
}
