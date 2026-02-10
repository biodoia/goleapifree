package optimizer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// OptimizerConfig configurazione dell'optimizer
type OptimizerConfig struct {
	// Pesi per la funzione obiettivo (devono sommare a 1.0)
	CostWeight    float64 `json:"cost_weight"`    // Peso del costo (0-1)
	QualityWeight float64 `json:"quality_weight"` // Peso della qualità (0-1)
	LatencyWeight float64 `json:"latency_weight"` // Peso della latenza (0-1)

	// Soglie
	MinQualityScore float64 `json:"min_quality_score"` // Qualità minima accettabile
	MaxLatencyMs    int     `json:"max_latency_ms"`    // Latenza massima accettabile (ms)
	MinSuccessRate  float64 `json:"min_success_rate"`  // Success rate minimo

	// ML settings
	TrainingWindow  time.Duration `json:"training_window"`  // Finestra temporale per training
	MinTrainingSamples int        `json:"min_training_samples"` // Minimo campioni per training
	RetrainingInterval time.Duration `json:"retraining_interval"` // Intervallo retraining
}

// DefaultOptimizerConfig configurazione di default
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		CostWeight:         0.4,
		QualityWeight:      0.4,
		LatencyWeight:      0.2,
		MinQualityScore:    0.6,
		MaxLatencyMs:       5000,
		MinSuccessRate:     0.8,
		TrainingWindow:     24 * time.Hour,
		MinTrainingSamples: 50,
		RetrainingInterval: 6 * time.Hour,
	}
}

// ProviderScore rappresenta lo score di un provider per una richiesta
type ProviderScore struct {
	ProviderID   uuid.UUID
	ProviderName string
	ModelID      uuid.UUID
	ModelName    string

	// Scores individuali (0-1)
	CostScore    float64
	QualityScore float64
	LatencyScore float64

	// Score composito pesato
	TotalScore   float64

	// Metriche stimate
	EstimatedCost    float64
	EstimatedLatency int
	SuccessRate      float64

	// Reasoning
	Reason string
}

// OptimizationResult risultato dell'ottimizzazione
type OptimizationResult struct {
	BestProvider     *ProviderScore
	AlternativeProviders []*ProviderScore
	Savings          float64 // Risparmio stimato vs provider più costoso
	Timestamp        time.Time
}

// Optimizer sistema di ottimizzazione ML per la selezione dei provider
type Optimizer struct {
	db        *database.DB
	config    *OptimizerConfig
	predictor *Predictor
	trainer   *Trainer
	analyzer  *Analyzer

	// Cache delle performance dei provider
	providerCache map[uuid.UUID]*ProviderPerformance
	cacheMu       sync.RWMutex

	// ML models
	modelMu sync.RWMutex

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ProviderPerformance cache delle performance di un provider
type ProviderPerformance struct {
	ProviderID   uuid.UUID
	ModelID      uuid.UUID
	SuccessRate  float64
	AvgLatencyMs int
	AvgCost      float64
	QualityScore float64
	TotalRequests int64
	LastUpdated  time.Time
}

// NewOptimizer crea un nuovo optimizer
func NewOptimizer(db *database.DB, config *OptimizerConfig) *Optimizer {
	if config == nil {
		config = DefaultOptimizerConfig()
	}

	// Normalizza i pesi
	totalWeight := config.CostWeight + config.QualityWeight + config.LatencyWeight
	if totalWeight > 0 {
		config.CostWeight /= totalWeight
		config.QualityWeight /= totalWeight
		config.LatencyWeight /= totalWeight
	}

	o := &Optimizer{
		db:            db,
		config:        config,
		providerCache: make(map[uuid.UUID]*ProviderPerformance),
		stopCh:        make(chan struct{}),
	}

	// Inizializza componenti ML
	o.predictor = NewPredictor(db, o)
	o.trainer = NewTrainer(db, o)
	o.analyzer = NewAnalyzer(db)

	return o
}

// Start avvia l'optimizer
func (o *Optimizer) Start() error {
	// Carica cache iniziale
	if err := o.refreshCache(); err != nil {
		log.Warn().Err(err).Msg("Failed to load initial cache")
	}

	// Avvia training iniziale
	go func() {
		if err := o.trainer.Train(context.Background()); err != nil {
			log.Error().Err(err).Msg("Initial training failed")
		}
	}()

	// Avvia background jobs
	o.wg.Add(2)
	go o.cacheRefreshLoop()
	go o.retrainingLoop()

	log.Info().
		Float64("cost_weight", o.config.CostWeight).
		Float64("quality_weight", o.config.QualityWeight).
		Float64("latency_weight", o.config.LatencyWeight).
		Msg("Cost optimizer started")

	return nil
}

// Stop ferma l'optimizer
func (o *Optimizer) Stop() {
	close(o.stopCh)
	o.wg.Wait()
	log.Info().Msg("Cost optimizer stopped")
}

// OptimizeRequest trova il miglior provider per una richiesta
func (o *Optimizer) OptimizeRequest(ctx context.Context, req *OptimizationRequest) (*OptimizationResult, error) {
	startTime := time.Now()

	// 1. Ottieni provider disponibili
	providers, err := o.getAvailableProviders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no available providers found")
	}

	// 2. Predici token count se non fornito
	if req.EstimatedInputTokens == 0 {
		req.EstimatedInputTokens = o.predictor.PredictTokenCount(req.PromptLength, req.PromptComplexity)
	}
	if req.EstimatedOutputTokens == 0 {
		req.EstimatedOutputTokens = o.predictor.PredictOutputTokens(req.EstimatedInputTokens, req.UserHistory)
	}

	// 3. Calcola score per ogni provider
	scores := make([]*ProviderScore, 0, len(providers))
	for _, provider := range providers {
		score := o.calculateProviderScore(req, provider)
		if score != nil {
			scores = append(scores, score)
		}
	}

	if len(scores) == 0 {
		return nil, fmt.Errorf("no suitable providers found")
	}

	// 4. Ordina per score totale
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	// 5. Calcola risparmio
	maxCost := 0.0
	for _, score := range scores {
		if score.EstimatedCost > maxCost {
			maxCost = score.EstimatedCost
		}
	}
	savings := maxCost - scores[0].EstimatedCost

	// Alternative providers (top 3 escluso il migliore)
	alternatives := make([]*ProviderScore, 0, 3)
	for i := 1; i < len(scores) && i <= 3; i++ {
		alternatives = append(alternatives, scores[i])
	}

	result := &OptimizationResult{
		BestProvider:         scores[0],
		AlternativeProviders: alternatives,
		Savings:              savings,
		Timestamp:            time.Now(),
	}

	log.Debug().
		Str("best_provider", scores[0].ProviderName).
		Str("best_model", scores[0].ModelName).
		Float64("score", scores[0].TotalScore).
		Float64("savings", savings).
		Dur("duration", time.Since(startTime)).
		Msg("Request optimized")

	return result, nil
}

// OptimizationRequest richiesta di ottimizzazione
type OptimizationRequest struct {
	UserID              uuid.UUID
	Modality            models.Modality
	RequiredCapabilities []string

	// Prompt info
	PromptLength     int
	PromptComplexity float64 // 0-1

	// Token estimates
	EstimatedInputTokens  int
	EstimatedOutputTokens int

	// User context
	UserHistory *UserHistory

	// Constraints
	MaxCost      float64
	MaxLatencyMs int
}

// UserHistory storico utente per predictions
type UserHistory struct {
	TotalRequests     int64
	AvgInputTokens    int
	AvgOutputTokens   int
	PreferredModels   []uuid.UUID
	PeakHours         []int
	AvgPromptLength   int
}

// getAvailableProviders ottiene i provider disponibili per la richiesta
func (o *Optimizer) getAvailableProviders(ctx context.Context, req *OptimizationRequest) ([]*models.Provider, error) {
	var providers []*models.Provider

	query := o.db.WithContext(ctx).
		Preload("Models").
		Where("status = ?", models.ProviderStatusActive).
		Where("health_score > ?", 0.5)

	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}

	// Filtra provider che hanno modelli compatibili
	compatible := make([]*models.Provider, 0)
	for _, provider := range providers {
		hasCompatible := false
		for _, model := range provider.Models {
			if model.Modality == req.Modality {
				hasCompatible = true
				break
			}
		}
		if hasCompatible {
			compatible = append(compatible, provider)
		}
	}

	return compatible, nil
}

// calculateProviderScore calcola lo score per un provider
func (o *Optimizer) calculateProviderScore(req *OptimizationRequest, provider *models.Provider) *ProviderScore {
	// Trova il miglior modello per questo provider
	var bestModel *models.Model
	for i := range provider.Models {
		if provider.Models[i].Modality == req.Modality {
			if bestModel == nil || provider.Models[i].QualityScore > bestModel.QualityScore {
				bestModel = &provider.Models[i]
			}
		}
	}

	if bestModel == nil {
		return nil
	}

	// Ottieni performance dal cache
	perf := o.getProviderPerformance(provider.ID, bestModel.ID)

	// Applica filtri
	if perf.SuccessRate < o.config.MinSuccessRate {
		return nil
	}
	if perf.AvgLatencyMs > o.config.MaxLatencyMs {
		return nil
	}
	if bestModel.QualityScore < o.config.MinQualityScore {
		return nil
	}

	// Calcola costo stimato
	estimatedCost := bestModel.EstimateCost(req.EstimatedInputTokens, req.EstimatedOutputTokens)
	if req.MaxCost > 0 && estimatedCost > req.MaxCost {
		return nil
	}

	// Calcola scores individuali (normalizzati 0-1)
	costScore := o.calculateCostScore(estimatedCost)
	qualityScore := bestModel.QualityScore
	latencyScore := o.calculateLatencyScore(perf.AvgLatencyMs)

	// Score composito pesato
	totalScore := (costScore * o.config.CostWeight) +
		(qualityScore * o.config.QualityWeight) +
		(latencyScore * o.config.LatencyWeight)

	// Bonus per success rate alto
	totalScore *= perf.SuccessRate

	reason := fmt.Sprintf("Cost: %.4f, Quality: %.2f, Latency: %dms, Success: %.2f%%",
		estimatedCost, qualityScore, perf.AvgLatencyMs, perf.SuccessRate*100)

	return &ProviderScore{
		ProviderID:       provider.ID,
		ProviderName:     provider.Name,
		ModelID:          bestModel.ID,
		ModelName:        bestModel.Name,
		CostScore:        costScore,
		QualityScore:     qualityScore,
		LatencyScore:     latencyScore,
		TotalScore:       totalScore,
		EstimatedCost:    estimatedCost,
		EstimatedLatency: perf.AvgLatencyMs,
		SuccessRate:      perf.SuccessRate,
		Reason:           reason,
	}
}

// calculateCostScore calcola lo score del costo (più basso = meglio)
func (o *Optimizer) calculateCostScore(cost float64) float64 {
	// Score = 1.0 per costo 0, decresce esponenzialmente
	// Usando una funzione sigmoid inversa
	if cost == 0 {
		return 1.0
	}

	// Normalizza rispetto a un costo di riferimento ($0.01 per request)
	normalizedCost := cost / 0.01
	score := 1.0 / (1.0 + normalizedCost)

	return math.Max(0, math.Min(1, score))
}

// calculateLatencyScore calcola lo score della latenza (più basso = meglio)
func (o *Optimizer) calculateLatencyScore(latencyMs int) float64 {
	if latencyMs <= 0 {
		return 1.0
	}

	// Score = 1.0 per latenza < 500ms, decresce linearmente fino a MaxLatencyMs
	targetLatency := 500.0
	maxLatency := float64(o.config.MaxLatencyMs)

	if float64(latencyMs) <= targetLatency {
		return 1.0
	}

	score := 1.0 - ((float64(latencyMs) - targetLatency) / (maxLatency - targetLatency))
	return math.Max(0, math.Min(1, score))
}

// getProviderPerformance ottiene le performance dal cache
func (o *Optimizer) getProviderPerformance(providerID, modelID uuid.UUID) *ProviderPerformance {
	o.cacheMu.RLock()
	defer o.cacheMu.RUnlock()

	key := providerID // Semplificato: usa solo provider ID
	if perf, exists := o.providerCache[key]; exists {
		return perf
	}

	// Default performance se non in cache
	return &ProviderPerformance{
		ProviderID:    providerID,
		ModelID:       modelID,
		SuccessRate:   0.9,
		AvgLatencyMs:  2000,
		AvgCost:       0.001,
		QualityScore:  0.7,
		TotalRequests: 0,
		LastUpdated:   time.Now(),
	}
}

// refreshCache aggiorna la cache delle performance
func (o *Optimizer) refreshCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query per aggregare statistiche
	var logs []models.RequestLog
	since := time.Now().Add(-o.config.TrainingWindow)

	err := o.db.WithContext(ctx).
		Where("timestamp > ?", since).
		Find(&logs).Error

	if err != nil {
		return err
	}

	// Aggrega per provider
	perfMap := make(map[uuid.UUID]*ProviderPerformance)

	for _, log := range logs {
		perf, exists := perfMap[log.ProviderID]
		if !exists {
			perf = &ProviderPerformance{
				ProviderID: log.ProviderID,
				ModelID:    log.ModelID,
			}
			perfMap[log.ProviderID] = perf
		}

		perf.TotalRequests++
		if log.Success {
			perf.SuccessRate += 1.0
		}
		perf.AvgLatencyMs += log.LatencyMs
		perf.AvgCost += log.EstimatedCost
	}

	// Calcola medie
	for _, perf := range perfMap {
		if perf.TotalRequests > 0 {
			perf.SuccessRate /= float64(perf.TotalRequests)
			perf.AvgLatencyMs /= int(perf.TotalRequests)
			perf.AvgCost /= float64(perf.TotalRequests)
		}
		perf.LastUpdated = time.Now()
	}

	// Aggiorna cache
	o.cacheMu.Lock()
	o.providerCache = perfMap
	o.cacheMu.Unlock()

	log.Debug().
		Int("providers_cached", len(perfMap)).
		Msg("Provider performance cache refreshed")

	return nil
}

// cacheRefreshLoop loop per refresh periodico della cache
func (o *Optimizer) cacheRefreshLoop() {
	defer o.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := o.refreshCache(); err != nil {
				log.Error().Err(err).Msg("Failed to refresh cache")
			}
		case <-o.stopCh:
			return
		}
	}
}

// retrainingLoop loop per retraining periodico del modello
func (o *Optimizer) retrainingLoop() {
	defer o.wg.Done()

	ticker := time.NewTicker(o.config.RetrainingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := o.trainer.Train(ctx); err != nil {
				log.Error().Err(err).Msg("Retraining failed")
			}
			cancel()
		case <-o.stopCh:
			return
		}
	}
}

// GetRecommendations ottiene raccomandazioni per ridurre i costi
func (o *Optimizer) GetRecommendations(ctx context.Context, userID uuid.UUID) (*CostRecommendations, error) {
	// Analizza pattern utente
	patterns := o.analyzer.AnalyzeUserPatterns(ctx, userID, 7*24*time.Hour)

	recommendations := &CostRecommendations{
		UserID:    userID,
		Timestamp: time.Now(),
	}

	// Raccomandazioni basate sui pattern
	if patterns != nil {
		// Suggerisci modelli più economici
		if patterns.AvgCostPerRequest > 0.005 {
			recommendations.Items = append(recommendations.Items, RecommendationItem{
				Type:        "switch_model",
				Title:       "Usa modelli più economici",
				Description: fmt.Sprintf("Il tuo costo medio per richiesta è $%.4f. Considera modelli più economici per query semplici.", patterns.AvgCostPerRequest),
				EstimatedSavings: patterns.TotalCost * 0.3, // 30% savings
				Priority:    "high",
			})
		}

		// Suggerisci batch processing per peak hours
		if len(patterns.PeakHours) > 0 {
			recommendations.Items = append(recommendations.Items, RecommendationItem{
				Type:        "batch_requests",
				Title:       "Batch requests durante peak hours",
				Description: fmt.Sprintf("Hai picchi di utilizzo alle ore %v. Considera batch processing.", patterns.PeakHours),
				EstimatedSavings: patterns.TotalCost * 0.15,
				Priority:    "medium",
			})
		}

		// Suggerisci cache per query ripetitive
		if patterns.RepetitiveQueriesRatio > 0.2 {
			recommendations.Items = append(recommendations.Items, RecommendationItem{
				Type:        "enable_caching",
				Title:       "Abilita caching per query ripetitive",
				Description: fmt.Sprintf("%.1f%% delle tue query sono ripetitive. Il caching può ridurre significativamente i costi.", patterns.RepetitiveQueriesRatio*100),
				EstimatedSavings: patterns.TotalCost * patterns.RepetitiveQueriesRatio,
				Priority:    "high",
			})
		}

		// Suggerisci ottimizzazione prompt
		if patterns.AvgInputTokens > 2000 {
			recommendations.Items = append(recommendations.Items, RecommendationItem{
				Type:        "optimize_prompts",
				Title:       "Ottimizza la lunghezza dei prompt",
				Description: fmt.Sprintf("I tuoi prompt hanno in media %d token. Prompt più concisi riducono i costi.", patterns.AvgInputTokens),
				EstimatedSavings: patterns.TotalCost * 0.2,
				Priority:    "medium",
			})
		}
	}

	// Calcola risparmio totale
	for _, item := range recommendations.Items {
		recommendations.TotalPotentialSavings += item.EstimatedSavings
	}

	return recommendations, nil
}

// CostRecommendations raccomandazioni per ottimizzare i costi
type CostRecommendations struct {
	UserID                uuid.UUID
	Items                 []RecommendationItem
	TotalPotentialSavings float64
	Timestamp             time.Time
}

// RecommendationItem singola raccomandazione
type RecommendationItem struct {
	Type             string  // "switch_model", "batch_requests", "enable_caching", etc.
	Title            string
	Description      string
	EstimatedSavings float64
	Priority         string // "high", "medium", "low"
}
