package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Tracker traccia l'utilizzo delle API
type Tracker struct {
	db *gorm.DB
}

// NewTracker crea un nuovo usage tracker
func NewTracker(db *gorm.DB) *Tracker {
	return &Tracker{
		db: db,
	}
}

// TrackRequest traccia una richiesta API
func (t *Tracker) TrackRequest(ctx context.Context, req *TrackingRequest) error {
	// Crea log della richiesta
	requestLog := &models.RequestLog{
		ProviderID:    req.ProviderID,
		ModelID:       req.ModelID,
		UserID:        req.UserID,
		Method:        req.Method,
		Endpoint:      req.Endpoint,
		StatusCode:    req.StatusCode,
		LatencyMs:     req.LatencyMs,
		InputTokens:   req.InputTokens,
		OutputTokens:  req.OutputTokens,
		Success:       req.Success,
		ErrorMessage:  req.ErrorMessage,
		EstimatedCost: req.EstimatedCost,
		Timestamp:     time.Now(),
	}

	if err := t.db.WithContext(ctx).Create(requestLog).Error; err != nil {
		return fmt.Errorf("failed to track request: %w", err)
	}

	// Aggiorna statistiche aggregate in background
	go t.updateStats(req)

	return nil
}

// TrackTokenUsage traccia l'utilizzo di token
func (t *Tracker) TrackTokenUsage(ctx context.Context, accountID uuid.UUID, inputTokens, outputTokens int) error {
	totalTokens := int64(inputTokens + outputTokens)

	// Aggiorna quota usata nell'account
	result := t.db.WithContext(ctx).
		Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("quota_used", gorm.Expr("quota_used + ?", totalTokens))

	if result.Error != nil {
		return fmt.Errorf("failed to track token usage: %w", result.Error)
	}

	log.Debug().
		Str("account_id", accountID.String()).
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Msg("Token usage tracked")

	return nil
}

// CalculateCost calcola il costo stimato di una richiesta
func (t *Tracker) CalculateCost(ctx context.Context, modelID uuid.UUID, inputTokens, outputTokens int) (float64, error) {
	var model models.Model
	if err := t.db.WithContext(ctx).First(&model, "id = ?", modelID).Error; err != nil {
		return 0, fmt.Errorf("failed to get model: %w", err)
	}

	// Calcola costo basato sui prezzi del modello
	// Nota: assumiamo che il modello abbia campi per i prezzi
	// Se non li ha, ritorniamo 0 (free API)
	cost := float64(0)

	// TODO: implementare calcolo costo se il modello ha informazioni di pricing
	// cost = (float64(inputTokens) * model.InputTokenPrice) + (float64(outputTokens) * model.OutputTokenPrice)

	return cost, nil
}

// GetUsageStats ottiene statistiche di utilizzo per un account
func (t *Tracker) GetUsageStats(ctx context.Context, accountID uuid.UUID, from, to time.Time) (*UsageStats, error) {
	var stats UsageStats

	// Query per aggregare le statistiche
	err := t.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			COUNT(*) as total_requests,
			SUM(CASE WHEN success = true THEN 1 ELSE 0 END) as successful_requests,
			SUM(input_tokens) as total_input_tokens,
			SUM(output_tokens) as total_output_tokens,
			SUM(estimated_cost) as total_cost,
			AVG(latency_ms) as avg_latency_ms
		`).
		Where("user_id = ? AND timestamp BETWEEN ? AND ?", accountID, from, to).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Calcola success rate
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests)
	}

	stats.Period = Period{
		From: from,
		To:   to,
	}

	return &stats, nil
}

// GetProviderStats ottiene statistiche per un provider
func (t *Tracker) GetProviderStats(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*ProviderUsageStats, error) {
	var stats ProviderUsageStats

	// Query per aggregare le statistiche del provider
	err := t.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			COUNT(*) as total_requests,
			SUM(CASE WHEN success = true THEN 1 ELSE 0 END) as successful_requests,
			SUM(input_tokens + output_tokens) as total_tokens,
			SUM(estimated_cost) as total_cost,
			AVG(latency_ms) as avg_latency_ms,
			COUNT(DISTINCT user_id) as unique_users
		`).
		Where("provider_id = ? AND timestamp BETWEEN ? AND ?", providerID, from, to).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get provider stats: %w", err)
	}

	// Calcola success rate
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests)
	}

	stats.ProviderID = providerID
	stats.Period = Period{
		From: from,
		To:   to,
	}

	return &stats, nil
}

// GetTopModels ottiene i modelli più utilizzati
func (t *Tracker) GetTopModels(ctx context.Context, limit int, from, to time.Time) ([]ModelUsage, error) {
	var results []ModelUsage

	err := t.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			model_id,
			COUNT(*) as request_count,
			SUM(input_tokens + output_tokens) as total_tokens
		`).
		Where("timestamp BETWEEN ? AND ?", from, to).
		Group("model_id").
		Order("request_count DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top models: %w", err)
	}

	return results, nil
}

// GetErrorStats ottiene statistiche sugli errori
func (t *Tracker) GetErrorStats(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*ErrorStats, error) {
	var stats ErrorStats

	// Conta errori per tipo
	type ErrorCount struct {
		ErrorMessage string
		Count        int64
	}

	var errorCounts []ErrorCount
	err := t.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select("error_message, COUNT(*) as count").
		Where("provider_id = ? AND success = false AND timestamp BETWEEN ? AND ?", providerID, from, to).
		Group("error_message").
		Order("count DESC").
		Limit(10).
		Scan(&errorCounts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get error stats: %w", err)
	}

	// Conta totale errori
	var totalErrors int64
	t.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Where("provider_id = ? AND success = false AND timestamp BETWEEN ? AND ?", providerID, from, to).
		Count(&totalErrors)

	stats.TotalErrors = totalErrors
	stats.ErrorsByType = make(map[string]int64)
	for _, ec := range errorCounts {
		stats.ErrorsByType[ec.ErrorMessage] = ec.Count
	}

	stats.Period = Period{
		From: from,
		To:   to,
	}

	return &stats, nil
}

// updateStats aggiorna le statistiche aggregate
func (t *Tracker) updateStats(req *TrackingRequest) {
	ctx := context.Background()

	// Trova o crea entry per oggi
	today := time.Now().Truncate(24 * time.Hour)

	var stats models.ProviderStats
	err := t.db.WithContext(ctx).
		Where("provider_id = ? AND DATE(timestamp) = DATE(?)", req.ProviderID, today).
		First(&stats).Error

	if err == gorm.ErrRecordNotFound {
		// Crea nuova entry
		stats = models.ProviderStats{
			ProviderID:    req.ProviderID,
			Timestamp:     today,
			TotalRequests: 0,
			TotalTokens:   0,
		}
	} else if err != nil {
		log.Error().Err(err).Msg("Failed to load provider stats")
		return
	}

	// Aggiorna statistiche
	stats.TotalRequests++
	stats.TotalTokens += int64(req.InputTokens + req.OutputTokens)

	if req.Success {
		// Aggiorna success rate (media mobile)
		if stats.TotalRequests == 1 {
			stats.SuccessRate = 1.0
		} else {
			stats.SuccessRate = (stats.SuccessRate*float64(stats.TotalRequests-1) + 1.0) / float64(stats.TotalRequests)
		}

		// Aggiorna latency media
		if stats.AvgLatencyMs == 0 {
			stats.AvgLatencyMs = req.LatencyMs
		} else {
			stats.AvgLatencyMs = (stats.AvgLatencyMs*int(stats.TotalRequests-1) + req.LatencyMs) / int(stats.TotalRequests)
		}
	} else {
		stats.ErrorCount++

		// Aggiorna success rate
		if stats.TotalRequests > 1 {
			stats.SuccessRate = (stats.SuccessRate * float64(stats.TotalRequests-1)) / float64(stats.TotalRequests)
		} else {
			stats.SuccessRate = 0
		}
	}

	stats.CostSaved += req.EstimatedCost

	// Salva statistiche
	if err := t.db.WithContext(ctx).Save(&stats).Error; err != nil {
		log.Error().Err(err).Msg("Failed to update provider stats")
	}
}

// TrackingRequest rappresenta una richiesta da tracciare
type TrackingRequest struct {
	ProviderID    uuid.UUID
	ModelID       uuid.UUID
	UserID        uuid.UUID
	Method        string
	Endpoint      string
	StatusCode    int
	LatencyMs     int
	InputTokens   int
	OutputTokens  int
	Success       bool
	ErrorMessage  string
	EstimatedCost float64
}

// UsageStats rappresenta statistiche di utilizzo
type UsageStats struct {
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	SuccessRate        float64 `json:"success_rate"`
	TotalInputTokens   int64   `json:"total_input_tokens"`
	TotalOutputTokens  int64   `json:"total_output_tokens"`
	TotalCost          float64 `json:"total_cost"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	Period             Period  `json:"period"`
}

// ProviderUsageStats rappresenta statistiche per un provider
type ProviderUsageStats struct {
	ProviderID         uuid.UUID `json:"provider_id"`
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	SuccessRate        float64   `json:"success_rate"`
	TotalTokens        int64     `json:"total_tokens"`
	TotalCost          float64   `json:"total_cost"`
	AvgLatencyMs       float64   `json:"avg_latency_ms"`
	UniqueUsers        int64     `json:"unique_users"`
	Period             Period    `json:"period"`
}

// ModelUsage rappresenta l'utilizzo di un modello
type ModelUsage struct {
	ModelID      uuid.UUID `json:"model_id"`
	RequestCount int64     `json:"request_count"`
	TotalTokens  int64     `json:"total_tokens"`
}

// ErrorStats rappresenta statistiche sugli errori
type ErrorStats struct {
	TotalErrors  int64            `json:"total_errors"`
	ErrorsByType map[string]int64 `json:"errors_by_type"`
	Period       Period           `json:"period"`
}

// Period rappresenta un periodo temporale
type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
