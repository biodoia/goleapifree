package optimizer

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Predictor predice token count e costi per le richieste
type Predictor struct {
	db        *database.DB
	optimizer *Optimizer

	// Parametri del modello di predizione
	// Regressione lineare semplice: tokens = a + b*length + c*complexity
	tokenPredictionModel *TokenPredictionModel
	costPredictionModel  *CostPredictionModel
}

// TokenPredictionModel modello per predire token count
type TokenPredictionModel struct {
	Intercept          float64
	LengthCoefficient  float64
	ComplexityCoefficient float64

	// Output prediction
	OutputIntercept    float64
	OutputInputRatio   float64

	// Metadata
	TrainedSamples int
	TrainedAt      time.Time
	RMSE           float64 // Root Mean Square Error
}

// CostPredictionModel modello per predire costi
type CostPredictionModel struct {
	AvgCostPerInputToken  float64
	AvgCostPerOutputToken float64
	TrainedAt             time.Time
}

// NewPredictor crea un nuovo predictor
func NewPredictor(db *database.DB, optimizer *Optimizer) *Predictor {
	return &Predictor{
		db:        db,
		optimizer: optimizer,
		tokenPredictionModel: &TokenPredictionModel{
			// Valori iniziali basati su euristiche comuni
			Intercept:             50,   // Base tokens
			LengthCoefficient:     0.25, // ~1 token per 4 caratteri
			ComplexityCoefficient: 100,  // Bonus per complessità
			OutputIntercept:       100,
			OutputInputRatio:      1.5, // Output tipicamente 1.5x input
		},
		costPredictionModel: &CostPredictionModel{
			AvgCostPerInputToken:  0.000001, // $0.001 per 1k tokens
			AvgCostPerOutputToken: 0.000003, // $0.003 per 1k tokens
		},
	}
}

// PredictTokenCount predice il numero di input tokens da lunghezza e complessità
func (p *Predictor) PredictTokenCount(promptLength int, complexity float64) int {
	if promptLength <= 0 {
		return 0
	}

	model := p.tokenPredictionModel

	// Regressione lineare: tokens = intercept + length*coeff + complexity*coeff
	prediction := model.Intercept +
		(float64(promptLength) * model.LengthCoefficient) +
		(complexity * model.ComplexityCoefficient)

	tokens := int(math.Round(prediction))
	if tokens < 1 {
		tokens = 1
	}

	log.Debug().
		Int("prompt_length", promptLength).
		Float64("complexity", complexity).
		Int("predicted_tokens", tokens).
		Msg("Predicted token count")

	return tokens
}

// PredictOutputTokens predice il numero di output tokens
func (p *Predictor) PredictOutputTokens(inputTokens int, userHistory *UserHistory) int {
	model := p.tokenPredictionModel

	var prediction float64

	if userHistory != nil && userHistory.TotalRequests > 0 {
		// Usa storico utente se disponibile
		avgRatio := float64(userHistory.AvgOutputTokens) / float64(userHistory.AvgInputTokens)
		prediction = float64(inputTokens) * avgRatio
	} else {
		// Usa modello generale
		prediction = model.OutputIntercept + (float64(inputTokens) * model.OutputInputRatio)
	}

	tokens := int(math.Round(prediction))
	if tokens < 1 {
		tokens = 1
	}

	return tokens
}

// EstimateCost stima il costo di una richiesta
func (p *Predictor) EstimateCost(inputTokens, outputTokens int) float64 {
	model := p.costPredictionModel

	inputCost := float64(inputTokens) * model.AvgCostPerInputToken
	outputCost := float64(outputTokens) * model.AvgCostPerOutputToken

	return inputCost + outputCost
}

// PredictBestProvider predice il miglior provider per una richiesta
func (p *Predictor) PredictBestProvider(ctx context.Context, req *PredictionRequest) (*ProviderPrediction, error) {
	// Predici token count
	inputTokens := req.InputTokens
	if inputTokens == 0 {
		inputTokens = p.PredictTokenCount(req.PromptLength, req.Complexity)
	}

	outputTokens := req.OutputTokens
	if outputTokens == 0 {
		outputTokens = p.PredictOutputTokens(inputTokens, req.UserHistory)
	}

	// Ottieni provider disponibili
	var providers []models.Provider
	query := p.db.WithContext(ctx).
		Preload("Models").
		Where("status = ?", models.ProviderStatusActive)

	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}

	// Trova il provider con il miglior costo/qualità
	var bestProvider *models.Provider
	var bestModel *models.Model
	bestScore := -1.0

	for i := range providers {
		provider := &providers[i]
		for j := range provider.Models {
			model := &provider.Models[j]

			if model.Modality != req.Modality {
				continue
			}

			// Calcola score
			cost := model.EstimateCost(inputTokens, outputTokens)
			quality := model.QualityScore

			// Score semplice: qualità / costo (massimizza qualità, minimizza costo)
			score := quality / (cost + 0.0001) // +epsilon per evitare divisione per 0

			if score > bestScore {
				bestScore = score
				bestProvider = provider
				bestModel = model
			}
		}
	}

	if bestProvider == nil {
		return nil, ErrNoSuitableProvider
	}

	estimatedCost := bestModel.EstimateCost(inputTokens, outputTokens)

	return &ProviderPrediction{
		ProviderID:       bestProvider.ID,
		ProviderName:     bestProvider.Name,
		ModelID:          bestModel.ID,
		ModelName:        bestModel.Name,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		EstimatedCost:    estimatedCost,
		QualityScore:     bestModel.QualityScore,
		Confidence:       p.calculateConfidence(bestScore),
	}, nil
}

// PredictionRequest richiesta di predizione
type PredictionRequest struct {
	Modality     models.Modality
	PromptLength int
	Complexity   float64
	InputTokens  int // Se già noto
	OutputTokens int // Se già noto
	UserHistory  *UserHistory
}

// ProviderPrediction predizione del miglior provider
type ProviderPrediction struct {
	ProviderID    uuid.UUID
	ProviderName  string
	ModelID       uuid.UUID
	ModelName     string
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64
	QualityScore  float64
	Confidence    float64 // 0-1
}

// calculateConfidence calcola la confidenza della predizione
func (p *Predictor) calculateConfidence(score float64) float64 {
	// Confidenza basata sul numero di campioni di training
	sampleConfidence := 0.5
	if p.tokenPredictionModel.TrainedSamples > 100 {
		sampleConfidence = 0.8
	} else if p.tokenPredictionModel.TrainedSamples > 50 {
		sampleConfidence = 0.7
	}

	// Confidenza basata sulla recency del training
	timeConfidence := 1.0
	if !p.tokenPredictionModel.TrainedAt.IsZero() {
		hoursSinceTraining := time.Since(p.tokenPredictionModel.TrainedAt).Hours()
		if hoursSinceTraining > 24 {
			timeConfidence = 0.8
		}
		if hoursSinceTraining > 72 {
			timeConfidence = 0.6
		}
	}

	return sampleConfidence * timeConfidence
}

// SuggestCheaperAlternatives suggerisce alternative più economiche
func (p *Predictor) SuggestCheaperAlternatives(ctx context.Context, currentProvider *models.Provider, currentModel *models.Model, inputTokens, outputTokens int) ([]*Alternative, error) {
	currentCost := currentModel.EstimateCost(inputTokens, outputTokens)

	// Trova alternative più economiche
	var providers []models.Provider
	err := p.db.WithContext(ctx).
		Preload("Models").
		Where("status = ?", models.ProviderStatusActive).
		Where("id != ?", currentProvider.ID).
		Find(&providers).Error

	if err != nil {
		return nil, err
	}

	alternatives := make([]*Alternative, 0)

	for i := range providers {
		provider := &providers[i]
		for j := range provider.Models {
			model := &provider.Models[j]

			if model.Modality != currentModel.Modality {
				continue
			}

			cost := model.EstimateCost(inputTokens, outputTokens)

			// Solo se più economico
			if cost < currentCost {
				savings := currentCost - cost
				savingsPercent := (savings / currentCost) * 100

				// Calcola quality difference
				qualityDiff := model.QualityScore - currentModel.QualityScore

				alternatives = append(alternatives, &Alternative{
					ProviderID:     provider.ID,
					ProviderName:   provider.Name,
					ModelID:        model.ID,
					ModelName:      model.Name,
					EstimatedCost:  cost,
					Savings:        savings,
					SavingsPercent: savingsPercent,
					QualityScore:   model.QualityScore,
					QualityDiff:    qualityDiff,
					Reason:         p.generateAlternativeReason(savings, savingsPercent, qualityDiff),
				})
			}
		}
	}

	// Ordina per savings
	for i := 0; i < len(alternatives); i++ {
		for j := i + 1; j < len(alternatives); j++ {
			if alternatives[j].Savings > alternatives[i].Savings {
				alternatives[i], alternatives[j] = alternatives[j], alternatives[i]
			}
		}
	}

	// Limita a top 5
	if len(alternatives) > 5 {
		alternatives = alternatives[:5]
	}

	return alternatives, nil
}

// Alternative rappresenta un'alternativa più economica
type Alternative struct {
	ProviderID     uuid.UUID
	ProviderName   string
	ModelID        uuid.UUID
	ModelName      string
	EstimatedCost  float64
	Savings        float64
	SavingsPercent float64
	QualityScore   float64
	QualityDiff    float64 // Positivo = migliore, negativo = peggiore
	Reason         string
}

// generateAlternativeReason genera una motivazione per l'alternativa
func (p *Predictor) generateAlternativeReason(savings, savingsPercent, qualityDiff float64) string {
	reason := ""

	if savingsPercent >= 50 {
		reason = "Risparmio significativo"
	} else if savingsPercent >= 20 {
		reason = "Buon risparmio"
	} else {
		reason = "Risparmio moderato"
	}

	if qualityDiff > 0.1 {
		reason += " con qualità superiore"
	} else if qualityDiff < -0.1 {
		reason += " con qualità leggermente inferiore"
	} else {
		reason += " con qualità simile"
	}

	return reason
}

// AnalyzePromptComplexity analizza la complessità di un prompt
func (p *Predictor) AnalyzePromptComplexity(prompt string) float64 {
	if len(prompt) == 0 {
		return 0.0
	}

	complexity := 0.0

	// Fattori che aumentano complessità
	// 1. Lunghezza (baseline)
	lengthFactor := math.Min(float64(len(prompt))/1000.0, 1.0)
	complexity += lengthFactor * 0.3

	// 2. Presenza di codice (backticks)
	codeBlocks := 0
	for i := 0; i < len(prompt)-2; i++ {
		if prompt[i:i+3] == "```" {
			codeBlocks++
		}
	}
	if codeBlocks > 0 {
		complexity += 0.2
	}

	// 3. Presenza di markdown/formattazione
	if hasMarkdown(prompt) {
		complexity += 0.1
	}

	// 4. Presenza di domande (?)
	questionCount := 0
	for _, char := range prompt {
		if char == '?' {
			questionCount++
		}
	}
	if questionCount > 0 {
		complexity += math.Min(float64(questionCount)*0.05, 0.2)
	}

	// 5. Presenza di numeri/dati
	hasNumbers := false
	for _, char := range prompt {
		if char >= '0' && char <= '9' {
			hasNumbers = true
			break
		}
	}
	if hasNumbers {
		complexity += 0.1
	}

	return math.Min(complexity, 1.0)
}

// hasMarkdown verifica se il testo contiene markdown
func hasMarkdown(text string) bool {
	markdownIndicators := []string{"**", "__", "##", "- ", "* ", "1. ", "[", "]("}
	for _, indicator := range markdownIndicators {
		if contains(text, indicator) {
			return true
		}
	}
	return false
}

// contains verifica se una stringa contiene una sottostringa
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// EstimateLatency stima la latenza per un provider
func (p *Predictor) EstimateLatency(providerID uuid.UUID, inputTokens, outputTokens int) int {
	// Ottieni performance dal cache dell'optimizer
	if p.optimizer != nil {
		perf := p.optimizer.getProviderPerformance(providerID, uuid.Nil)
		if perf.TotalRequests > 0 {
			// Aggiusta per dimensione richiesta
			baseLatency := perf.AvgLatencyMs
			tokenFactor := float64(inputTokens+outputTokens) / 1000.0
			adjustedLatency := float64(baseLatency) * (1.0 + tokenFactor*0.1)
			return int(adjustedLatency)
		}
	}

	// Fallback: stima basata su token count
	// Assumi ~100 tokens/sec
	estimatedSeconds := float64(inputTokens+outputTokens) / 100.0
	return int(estimatedSeconds * 1000)
}

// GetModelStats ottiene statistiche per un modello
func (p *Predictor) GetModelStats(ctx context.Context, modelID uuid.UUID, duration time.Duration) (*ModelStats, error) {
	var logs []models.RequestLog

	since := time.Now().Add(-duration)
	err := p.db.WithContext(ctx).
		Where("model_id = ?", modelID).
		Where("timestamp > ?", since).
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, ErrNoData
	}

	stats := &ModelStats{
		ModelID:      modelID,
		TotalRequests: int64(len(logs)),
	}

	successCount := 0
	for _, log := range logs {
		stats.TotalInputTokens += int64(log.InputTokens)
		stats.TotalOutputTokens += int64(log.OutputTokens)
		stats.TotalCost += log.EstimatedCost
		stats.TotalLatencyMs += int64(log.LatencyMs)

		if log.Success {
			successCount++
		}
	}

	stats.AvgInputTokens = int(stats.TotalInputTokens / stats.TotalRequests)
	stats.AvgOutputTokens = int(stats.TotalOutputTokens / stats.TotalRequests)
	stats.AvgCost = stats.TotalCost / float64(stats.TotalRequests)
	stats.AvgLatencyMs = int(stats.TotalLatencyMs / stats.TotalRequests)
	stats.SuccessRate = float64(successCount) / float64(stats.TotalRequests)

	return stats, nil
}

// ModelStats statistiche per un modello
type ModelStats struct {
	ModelID          uuid.UUID
	TotalRequests    int64
	TotalInputTokens int64
	TotalOutputTokens int64
	AvgInputTokens   int
	AvgOutputTokens  int
	TotalCost        float64
	AvgCost          float64
	TotalLatencyMs   int64
	AvgLatencyMs     int
	SuccessRate      float64
}

// Errori comuni
var (
	ErrNoSuitableProvider = fmt.Errorf("no suitable provider found")
	ErrNoData            = fmt.Errorf("no data available")
)
