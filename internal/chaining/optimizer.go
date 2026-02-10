package chaining

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/rs/zerolog/log"
)

// Optimizer ottimizza la selezione di pipeline e strategie
type Optimizer struct {
	profiles  map[string]*PerformanceProfile
	history   *ExecutionHistory
	weights   OptimizationWeights
	mu        sync.RWMutex
}

// OptimizationWeights definisce i pesi per l'ottimizzazione
type OptimizationWeights struct {
	Cost    float64 // Peso del costo (0-1)
	Latency float64 // Peso della latenza (0-1)
	Quality float64 // Peso della qualità (0-1)
}

// PerformanceProfile raccoglie metriche di performance per una configurazione
type PerformanceProfile struct {
	ConfigID        string
	Strategy        string
	Stages          []StageConfig
	AverageLatency  time.Duration
	AverageCost     float64
	AverageQuality  float64
	SuccessRate     float64
	ExecutionCount  int64
	LastUpdated     time.Time
}

// StageConfig rappresenta la configurazione di uno stage
type StageConfig struct {
	Provider string
	Model    string
	Settings map[string]interface{}
}

// ExecutionHistory memorizza la storia delle esecuzioni
type ExecutionHistory struct {
	executions []ExecutionRecord
	maxSize    int
	mu         sync.RWMutex
}

// ExecutionRecord rappresenta un record di esecuzione
type ExecutionRecord struct {
	Timestamp   time.Time
	ConfigID    string
	Request     *providers.ChatRequest
	Result      *PipelineResult
	Latency     time.Duration
	Cost        float64
	Quality     float64
	Success     bool
	Error       error
}

// NewOptimizer crea un nuovo optimizer
func NewOptimizer(weights OptimizationWeights) *Optimizer {
	// Normalizza i pesi
	total := weights.Cost + weights.Latency + weights.Quality
	if total == 0 {
		// Default: equilibrato
		weights = OptimizationWeights{
			Cost:    0.33,
			Latency: 0.33,
			Quality: 0.34,
		}
	} else {
		weights.Cost /= total
		weights.Latency /= total
		weights.Quality /= total
	}

	return &Optimizer{
		profiles: make(map[string]*PerformanceProfile),
		history:  NewExecutionHistory(1000),
		weights:  weights,
	}
}

// NewExecutionHistory crea una nuova history con dimensione massima
func NewExecutionHistory(maxSize int) *ExecutionHistory {
	return &ExecutionHistory{
		executions: make([]ExecutionRecord, 0, maxSize),
		maxSize:    maxSize,
	}
}

// SelectOptimalStrategy seleziona la strategia ottimale dato un obiettivo
func (o *Optimizer) SelectOptimalStrategy(ctx context.Context, req *providers.ChatRequest, objective string) (Strategy, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.profiles) == 0 {
		// Nessun profilo disponibile, usa euristica
		return o.selectByHeuristic(objective)
	}

	// Calcola score per ogni profilo
	var bestProfile *PerformanceProfile
	bestScore := float64(-1)

	for _, profile := range o.profiles {
		score := o.calculateScore(profile, objective)
		if score > bestScore {
			bestScore = score
			bestProfile = profile
		}
	}

	if bestProfile == nil {
		return o.selectByHeuristic(objective)
	}

	// Crea strategia dal profilo migliore
	return o.createStrategyFromProfile(bestProfile)
}

// calculateScore calcola uno score per un profilo basato sui pesi
func (o *Optimizer) calculateScore(profile *PerformanceProfile, objective string) float64 {
	if profile.ExecutionCount == 0 {
		return 0
	}

	// Normalizza metriche (0-1, dove 1 è migliore)
	latencyScore := 1.0 / (1.0 + profile.AverageLatency.Seconds())
	costScore := 1.0 / (1.0 + profile.AverageCost)
	qualityScore := profile.AverageQuality

	// Applica pesi
	score := o.weights.Latency*latencyScore +
		o.weights.Cost*costScore +
		o.weights.Quality*qualityScore

	// Penalizza configurazioni con basso success rate
	score *= profile.SuccessRate

	// Bonus per configurazioni testate più volte (confidence)
	confidenceBonus := math.Min(float64(profile.ExecutionCount)/100.0, 0.2)
	score += confidenceBonus

	return score
}

// selectByHeuristic seleziona strategia con euristica semplice
func (o *Optimizer) selectByHeuristic(objective string) (Strategy, error) {
	switch objective {
	case "cost":
		// Usa cascade per minimizzare costi
		return NewCascadeStrategy(2*time.Second, true, 50), nil
	case "latency", "speed":
		// Usa cascade con timeout aggressivo
		return NewCascadeStrategy(1*time.Second, false, 20), nil
	case "quality":
		// Usa draft-refine per massima qualità
		return NewDraftRefineStrategy("", ""), nil
	case "balanced":
		// Usa draft-refine con timeout moderato
		return NewDraftRefineStrategy("", ""), nil
	case "consensus":
		// Usa parallel consensus per robustezza
		return NewParallelConsensusStrategy("majority"), nil
	default:
		return NewSequentialStrategy(), nil
	}
}

// createStrategyFromProfile crea una strategia da un profilo
func (o *Optimizer) createStrategyFromProfile(profile *PerformanceProfile) (Strategy, error) {
	switch profile.Strategy {
	case "draft-refine":
		return NewDraftRefineStrategy("", ""), nil
	case "parallel-consensus":
		return NewParallelConsensusStrategy("majority"), nil
	case "cascade":
		return NewCascadeStrategy(2*time.Second, true, 50), nil
	case "speculative-decoding":
		return NewSpeculativeDecodingStrategy(5, 0.8), nil
	case "sequential":
		return NewSequentialStrategy(), nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", profile.Strategy)
	}
}

// RecordExecution registra un'esecuzione e aggiorna i profili
func (o *Optimizer) RecordExecution(record ExecutionRecord) {
	// Aggiungi alla history
	o.history.Add(record)

	// Aggiorna profilo
	o.updateProfile(record)
}

// updateProfile aggiorna il profilo di performance
func (o *Optimizer) updateProfile(record ExecutionRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()

	profile, exists := o.profiles[record.ConfigID]
	if !exists {
		// Crea nuovo profilo
		profile = &PerformanceProfile{
			ConfigID:       record.ConfigID,
			Strategy:       extractStrategy(record.Result),
			Stages:         extractStages(record.Result),
			LastUpdated:    time.Now(),
		}
		o.profiles[record.ConfigID] = profile
	}

	// Aggiorna metriche con media mobile esponenziale
	alpha := 0.3 // Peso per nuove osservazioni
	profile.ExecutionCount++

	if profile.ExecutionCount == 1 {
		profile.AverageLatency = record.Latency
		profile.AverageCost = record.Cost
		profile.AverageQuality = record.Quality
		if record.Success {
			profile.SuccessRate = 1.0
		} else {
			profile.SuccessRate = 0.0
		}
	} else {
		// EMA
		profile.AverageLatency = time.Duration(
			float64(profile.AverageLatency)*(1-alpha) + float64(record.Latency)*alpha,
		)
		profile.AverageCost = profile.AverageCost*(1-alpha) + record.Cost*alpha
		profile.AverageQuality = profile.AverageQuality*(1-alpha) + record.Quality*alpha

		// Success rate
		if record.Success {
			profile.SuccessRate = profile.SuccessRate*(1-alpha) + alpha
		} else {
			profile.SuccessRate = profile.SuccessRate * (1 - alpha)
		}
	}

	profile.LastUpdated = time.Now()

	log.Debug().
		Str("config_id", record.ConfigID).
		Dur("avg_latency", profile.AverageLatency).
		Float64("avg_cost", profile.AverageCost).
		Float64("success_rate", profile.SuccessRate).
		Int64("executions", profile.ExecutionCount).
		Msg("Updated performance profile")
}

// Add aggiunge un record alla history
func (h *ExecutionHistory) Add(record ExecutionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.executions = append(h.executions, record)

	// Mantieni dimensione massima (FIFO)
	if len(h.executions) > h.maxSize {
		h.executions = h.executions[len(h.executions)-h.maxSize:]
	}
}

// GetRecent restituisce gli ultimi N record
func (h *ExecutionHistory) GetRecent(n int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n > len(h.executions) {
		n = len(h.executions)
	}

	start := len(h.executions) - n
	result := make([]ExecutionRecord, n)
	copy(result, h.executions[start:])

	return result
}

// GetByConfigID filtra record per config ID
func (h *ExecutionHistory) GetByConfigID(configID string) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []ExecutionRecord
	for _, record := range h.executions {
		if record.ConfigID == configID {
			result = append(result, record)
		}
	}

	return result
}

// extractStrategy estrae il nome della strategia dal risultato
func extractStrategy(result *PipelineResult) string {
	if result == nil || result.Metadata == nil {
		return "unknown"
	}

	if strategy, ok := result.Metadata["strategy"].(string); ok {
		return strategy
	}

	return "unknown"
}

// extractStages estrae la configurazione degli stage dal risultato
func extractStages(result *PipelineResult) []StageConfig {
	if result == nil || len(result.StageOutputs) == 0 {
		return nil
	}

	configs := make([]StageConfig, len(result.StageOutputs))
	for i, output := range result.StageOutputs {
		configs[i] = StageConfig{
			Provider: output.StageName,
			Model:    output.Response.Model,
			Settings: output.Metadata,
		}
	}

	return configs
}

// GetProfiles restituisce tutti i profili di performance
func (o *Optimizer) GetProfiles() map[string]*PerformanceProfile {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Copia profonda
	profiles := make(map[string]*PerformanceProfile)
	for id, profile := range o.profiles {
		profileCopy := *profile
		profileCopy.Stages = make([]StageConfig, len(profile.Stages))
		copy(profileCopy.Stages, profile.Stages)
		profiles[id] = &profileCopy
	}

	return profiles
}

// GetStatistics calcola statistiche aggregate
func (o *Optimizer) GetStatistics() OptimizerStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	stats := OptimizerStats{
		TotalProfiles:    len(o.profiles),
		TotalExecutions:  0,
		AverageLatency:   0,
		AverageCost:      0,
		AverageQuality:   0,
		OverallSuccessRate: 0,
	}

	if len(o.profiles) == 0 {
		return stats
	}

	var totalLatency time.Duration
	var totalCost float64
	var totalQuality float64
	var totalSuccess float64

	for _, profile := range o.profiles {
		stats.TotalExecutions += profile.ExecutionCount
		totalLatency += time.Duration(float64(profile.AverageLatency) * float64(profile.ExecutionCount))
		totalCost += profile.AverageCost * float64(profile.ExecutionCount)
		totalQuality += profile.AverageQuality * float64(profile.ExecutionCount)
		totalSuccess += profile.SuccessRate * float64(profile.ExecutionCount)
	}

	if stats.TotalExecutions > 0 {
		stats.AverageLatency = totalLatency / time.Duration(stats.TotalExecutions)
		stats.AverageCost = totalCost / float64(stats.TotalExecutions)
		stats.AverageQuality = totalQuality / float64(stats.TotalExecutions)
		stats.OverallSuccessRate = totalSuccess / float64(stats.TotalExecutions)
	}

	return stats
}

// OptimizerStats rappresenta statistiche aggregate dell'optimizer
type OptimizerStats struct {
	TotalProfiles      int
	TotalExecutions    int64
	AverageLatency     time.Duration
	AverageCost        float64
	AverageQuality     float64
	OverallSuccessRate float64
}

// AutoTune ottimizza automaticamente i pesi basandosi sulla storia
func (o *Optimizer) AutoTune() {
	recent := o.history.GetRecent(100)
	if len(recent) < 10 {
		// Non abbastanza dati per auto-tuning
		return
	}

	// Analizza correlazioni tra pesi e risultati
	// Per ora implementazione semplificata: adatta pesi in base ai fallimenti

	successfulCount := 0
	for _, record := range recent {
		if record.Success {
			successfulCount++
		}
	}

	successRate := float64(successfulCount) / float64(len(recent))

	o.mu.Lock()
	defer o.mu.Unlock()

	// Se il success rate è basso, aumenta peso qualità
	if successRate < 0.8 {
		o.weights.Quality = math.Min(o.weights.Quality*1.1, 0.6)
		o.weights.Cost = o.weights.Cost * 0.95
		o.weights.Latency = o.weights.Latency * 0.95
	}

	// Normalizza
	total := o.weights.Cost + o.weights.Latency + o.weights.Quality
	o.weights.Cost /= total
	o.weights.Latency /= total
	o.weights.Quality /= total

	log.Info().
		Float64("cost_weight", o.weights.Cost).
		Float64("latency_weight", o.weights.Latency).
		Float64("quality_weight", o.weights.Quality).
		Float64("success_rate", successRate).
		Msg("Auto-tuned optimizer weights")
}

// RecommendPipeline raccomanda una pipeline ottimale
func (o *Optimizer) RecommendPipeline(ctx context.Context, req *providers.ChatRequest, constraints PipelineConstraints) (*PipelineRecommendation, error) {
	// Seleziona strategia ottimale
	strategy, err := o.SelectOptimalStrategy(ctx, req, constraints.Objective)
	if err != nil {
		return nil, fmt.Errorf("failed to select strategy: %w", err)
	}

	// Trova profilo migliore per questa strategia
	o.mu.RLock()
	var bestProfile *PerformanceProfile
	bestScore := float64(-1)

	for _, profile := range o.profiles {
		if profile.Strategy != strategy.Name() {
			continue
		}

		// Verifica constraints
		if constraints.MaxLatency > 0 && profile.AverageLatency > constraints.MaxLatency {
			continue
		}
		if constraints.MaxCost > 0 && profile.AverageCost > constraints.MaxCost {
			continue
		}

		score := o.calculateScore(profile, constraints.Objective)
		if score > bestScore {
			bestScore = score
			bestProfile = profile
		}
	}
	o.mu.RUnlock()

	recommendation := &PipelineRecommendation{
		Strategy:          strategy.Name(),
		ExpectedLatency:   0,
		ExpectedCost:      0,
		ExpectedQuality:   0,
		Confidence:        0,
		Reason:            "",
	}

	if bestProfile != nil {
		recommendation.ExpectedLatency = bestProfile.AverageLatency
		recommendation.ExpectedCost = bestProfile.AverageCost
		recommendation.ExpectedQuality = bestProfile.AverageQuality
		recommendation.Confidence = math.Min(float64(bestProfile.ExecutionCount)/100.0, 1.0)
		recommendation.Reason = fmt.Sprintf("Based on %d executions with %.1f%% success rate",
			bestProfile.ExecutionCount, bestProfile.SuccessRate*100)
	} else {
		recommendation.Reason = "No historical data available, using heuristic"
		recommendation.Confidence = 0.3
	}

	return recommendation, nil
}

// PipelineConstraints definisce vincoli per la selezione della pipeline
type PipelineConstraints struct {
	Objective  string        // "cost", "latency", "quality", "balanced"
	MaxLatency time.Duration // Massima latenza accettabile
	MaxCost    float64       // Massimo costo accettabile
	MinQuality float64       // Qualità minima richiesta
}

// PipelineRecommendation rappresenta una raccomandazione
type PipelineRecommendation struct {
	Strategy        string
	ExpectedLatency time.Duration
	ExpectedCost    float64
	ExpectedQuality float64
	Confidence      float64
	Reason          string
}
