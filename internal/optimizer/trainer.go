package optimizer

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog/log"
)

// Trainer esegue il training dei modelli ML
type Trainer struct {
	db        *database.DB
	optimizer *Optimizer
}

// NewTrainer crea un nuovo trainer
func NewTrainer(db *database.DB, optimizer *Optimizer) *Trainer {
	return &Trainer{
		db:        db,
		optimizer: optimizer,
	}
}

// Train esegue il training completo dei modelli
func (t *Trainer) Train(ctx context.Context) error {
	startTime := time.Now()

	log.Info().Msg("Starting model training")

	// 1. Raccogli dati di training
	data, err := t.collectTrainingData(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect training data: %w", err)
	}

	if len(data) < t.optimizer.config.MinTrainingSamples {
		log.Warn().
			Int("samples", len(data)).
			Int("required", t.optimizer.config.MinTrainingSamples).
			Msg("Not enough samples for training")
		return ErrInsufficientData
	}

	log.Info().
		Int("samples", len(data)).
		Msg("Collected training data")

	// 2. Estrai features
	features := t.extractFeatures(data)

	// 3. Train token prediction model
	if err := t.trainTokenPrediction(features); err != nil {
		log.Error().Err(err).Msg("Failed to train token prediction model")
		return err
	}

	// 4. Train cost prediction model
	if err := t.trainCostPrediction(features); err != nil {
		log.Error().Err(err).Msg("Failed to train cost prediction model")
		return err
	}

	// 5. Aggiorna modelli nell'optimizer
	t.updateOptimizerModels()

	duration := time.Since(startTime)
	log.Info().
		Dur("duration", duration).
		Int("samples", len(data)).
		Msg("Model training completed")

	return nil
}

// TrainingData rappresenta un campione di training
type TrainingData struct {
	RequestLog   models.RequestLog
	PromptLength int
	Complexity   float64
}

// collectTrainingData raccoglie dati dai log delle richieste
func (t *Trainer) collectTrainingData(ctx context.Context) ([]TrainingData, error) {
	var logs []models.RequestLog

	since := time.Now().Add(-t.optimizer.config.TrainingWindow)

	err := t.db.WithContext(ctx).
		Where("timestamp > ?", since).
		Where("success = ?", true). // Solo richieste di successo
		Where("input_tokens > 0").  // Solo con dati validi
		Order("timestamp DESC").
		Limit(10000). // Limita per performance
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	data := make([]TrainingData, len(logs))
	for i, log := range logs {
		data[i] = TrainingData{
			RequestLog:   log,
			PromptLength: estimatePromptLength(log.InputTokens),
			Complexity:   estimateComplexity(log.InputTokens, log.OutputTokens),
		}
	}

	return data, nil
}

// estimatePromptLength stima la lunghezza del prompt dai token
func estimatePromptLength(tokens int) int {
	// Assumi circa 4 caratteri per token (media per inglese)
	return tokens * 4
}

// estimateComplexity stima la complessità dalla distribuzione token
func estimateComplexity(inputTokens, outputTokens int) float64 {
	if inputTokens == 0 {
		return 0.0
	}

	// Complessità basata sul rapporto output/input
	ratio := float64(outputTokens) / float64(inputTokens)

	// Normalizza tra 0-1
	// Ratio alto = query complessa che genera molto output
	complexity := math.Min(ratio/3.0, 1.0)

	return complexity
}

// Features rappresenta le features estratte per il training
type Features struct {
	Samples []FeatureSample
}

// FeatureSample singolo campione con features
type FeatureSample struct {
	// Input features
	PromptLength int
	Complexity   float64

	// Target variables
	InputTokens  int
	OutputTokens int
	Cost         float64
	LatencyMs    int
}

// extractFeatures estrae features dai dati di training
func (t *Trainer) extractFeatures(data []TrainingData) *Features {
	features := &Features{
		Samples: make([]FeatureSample, len(data)),
	}

	for i, d := range data {
		features.Samples[i] = FeatureSample{
			PromptLength: d.PromptLength,
			Complexity:   d.Complexity,
			InputTokens:  d.RequestLog.InputTokens,
			OutputTokens: d.RequestLog.OutputTokens,
			Cost:         d.RequestLog.EstimatedCost,
			LatencyMs:    d.RequestLog.LatencyMs,
		}
	}

	return features
}

// trainTokenPrediction addestra il modello di predizione token (regressione lineare)
func (t *Trainer) trainTokenPrediction(features *Features) error {
	if len(features.Samples) == 0 {
		return ErrNoData
	}

	// Prepara dati per regressione lineare multipla
	// Y = a + b*X1 + c*X2
	// Y = tokens, X1 = length, X2 = complexity

	n := len(features.Samples)

	// Somme per il calcolo dei coefficienti
	sumInputTokens := 0.0
	sumOutputTokens := 0.0
	sumLength := 0.0
	sumComplexity := 0.0
	sumLengthSq := 0.0
	sumComplexitySq := 0.0
	sumLengthComplexity := 0.0
	sumInputLength := 0.0
	sumInputComplexity := 0.0
	sumOutputInput := 0.0
	sumInputSq := 0.0

	for _, sample := range features.Samples {
		inputTokens := float64(sample.InputTokens)
		outputTokens := float64(sample.OutputTokens)
		length := float64(sample.PromptLength)
		complexity := sample.Complexity

		sumInputTokens += inputTokens
		sumOutputTokens += outputTokens
		sumLength += length
		sumComplexity += complexity
		sumLengthSq += length * length
		sumComplexitySq += complexity * complexity
		sumLengthComplexity += length * complexity
		sumInputLength += inputTokens * length
		sumInputComplexity += inputTokens * complexity
		sumOutputInput += outputTokens * inputTokens
		sumInputSq += inputTokens * inputTokens
	}

	// Calcola medie
	avgInputTokens := sumInputTokens / float64(n)
	avgOutputTokens := sumOutputTokens / float64(n)
	avgLength := sumLength / float64(n)
	avgComplexity := sumComplexity / float64(n)

	// Regressione lineare semplificata
	// Calcola coefficiente per length
	lengthVariance := (sumLengthSq / float64(n)) - (avgLength * avgLength)
	lengthCovariance := (sumInputLength / float64(n)) - (avgInputTokens * avgLength)

	lengthCoeff := 0.0
	if lengthVariance > 0 {
		lengthCoeff = lengthCovariance / lengthVariance
	}

	// Calcola coefficiente per complexity
	complexityVariance := (sumComplexitySq / float64(n)) - (avgComplexity * avgComplexity)
	complexityCovariance := (sumInputComplexity / float64(n)) - (avgInputTokens * avgComplexity)

	complexityCoeff := 0.0
	if complexityVariance > 0 {
		complexityCoeff = complexityCovariance / complexityVariance
	}

	// Calcola intercept
	intercept := avgInputTokens - (lengthCoeff * avgLength) - (complexityCoeff * avgComplexity)

	// Output prediction: output = intercept + ratio * input
	inputVariance := (sumInputSq / float64(n)) - (avgInputTokens * avgInputTokens)
	outputCovariance := (sumOutputInput / float64(n)) - (avgOutputTokens * avgInputTokens)

	outputRatio := 1.5 // Default
	if inputVariance > 0 {
		outputRatio = outputCovariance / inputVariance
	}

	outputIntercept := avgOutputTokens - (outputRatio * avgInputTokens)

	// Calcola RMSE (Root Mean Square Error)
	rmse := t.calculateRMSE(features, intercept, lengthCoeff, complexityCoeff)

	// Aggiorna modello
	model := &TokenPredictionModel{
		Intercept:             intercept,
		LengthCoefficient:     lengthCoeff,
		ComplexityCoefficient: complexityCoeff,
		OutputIntercept:       outputIntercept,
		OutputInputRatio:      outputRatio,
		TrainedSamples:        n,
		TrainedAt:             time.Now(),
		RMSE:                  rmse,
	}

	t.optimizer.predictor.tokenPredictionModel = model

	log.Info().
		Float64("intercept", intercept).
		Float64("length_coeff", lengthCoeff).
		Float64("complexity_coeff", complexityCoeff).
		Float64("output_ratio", outputRatio).
		Float64("rmse", rmse).
		Int("samples", n).
		Msg("Token prediction model trained")

	return nil
}

// calculateRMSE calcola il Root Mean Square Error
func (t *Trainer) calculateRMSE(features *Features, intercept, lengthCoeff, complexityCoeff float64) float64 {
	sumSquaredError := 0.0

	for _, sample := range features.Samples {
		predicted := intercept +
			(float64(sample.PromptLength) * lengthCoeff) +
			(sample.Complexity * complexityCoeff)

		error := float64(sample.InputTokens) - predicted
		sumSquaredError += error * error
	}

	mse := sumSquaredError / float64(len(features.Samples))
	return math.Sqrt(mse)
}

// trainCostPrediction addestra il modello di predizione costo
func (t *Trainer) trainCostPrediction(features *Features) error {
	if len(features.Samples) == 0 {
		return ErrNoData
	}

	// Calcola costo medio per token
	totalInputTokens := 0.0
	totalOutputTokens := 0.0
	totalCost := 0.0

	for _, sample := range features.Samples {
		totalInputTokens += float64(sample.InputTokens)
		totalOutputTokens += float64(sample.OutputTokens)
		totalCost += sample.Cost
	}

	// Stima costo per token
	// Assumi che input e output abbiano costi diversi
	// Usa rapporto tipico: output costa ~3x input

	avgCostPerToken := 0.0
	if (totalInputTokens + totalOutputTokens) > 0 {
		avgCostPerToken = totalCost / (totalInputTokens + totalOutputTokens)
	}

	// Split tra input e output (output più costoso)
	avgInputCost := avgCostPerToken * 0.75  // 75% del costo medio
	avgOutputCost := avgCostPerToken * 2.25 // 225% del costo medio

	model := &CostPredictionModel{
		AvgCostPerInputToken:  avgInputCost,
		AvgCostPerOutputToken: avgOutputCost,
		TrainedAt:             time.Now(),
	}

	t.optimizer.predictor.costPredictionModel = model

	log.Info().
		Float64("avg_input_cost", avgInputCost).
		Float64("avg_output_cost", avgOutputCost).
		Float64("total_cost", totalCost).
		Msg("Cost prediction model trained")

	return nil
}

// updateOptimizerModels aggiorna i modelli nell'optimizer
func (t *Trainer) updateOptimizerModels() {
	t.optimizer.modelMu.Lock()
	defer t.optimizer.modelMu.Unlock()

	// I modelli sono già aggiornati nel predictor
	// Questo metodo può essere usato per ulteriori sincronizzazioni
	log.Debug().Msg("Updated optimizer models")
}

// EvaluateModel valuta la performance del modello su dati di test
func (t *Trainer) EvaluateModel(ctx context.Context) (*ModelEvaluation, error) {
	// Raccogli dati di test (ultimi 1000 record)
	var logs []models.RequestLog

	err := t.db.WithContext(ctx).
		Where("success = ?", true).
		Where("input_tokens > 0").
		Order("timestamp DESC").
		Limit(1000).
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, ErrNoData
	}

	eval := &ModelEvaluation{
		TotalSamples: len(logs),
		Timestamp:    time.Now(),
	}

	sumAbsError := 0.0
	sumSquaredError := 0.0
	correctPredictions := 0

	for _, log := range logs {
		// Predici token count
		promptLength := estimatePromptLength(log.InputTokens)
		complexity := estimateComplexity(log.InputTokens, log.OutputTokens)

		predicted := t.optimizer.predictor.PredictTokenCount(promptLength, complexity)
		actual := log.InputTokens

		// Calcola errori
		error := float64(actual - predicted)
		absError := math.Abs(error)
		squaredError := error * error

		sumAbsError += absError
		sumSquaredError += squaredError

		// Conta predizioni "corrette" (entro 10% dell'attuale)
		if absError <= float64(actual)*0.1 {
			correctPredictions++
		}
	}

	eval.MAE = sumAbsError / float64(len(logs))
	eval.RMSE = math.Sqrt(sumSquaredError / float64(len(logs)))
	eval.Accuracy = float64(correctPredictions) / float64(len(logs))

	log.Info().
		Float64("mae", eval.MAE).
		Float64("rmse", eval.RMSE).
		Float64("accuracy", eval.Accuracy).
		Int("samples", len(logs)).
		Msg("Model evaluation completed")

	return eval, nil
}

// ModelEvaluation valutazione della performance del modello
type ModelEvaluation struct {
	TotalSamples int
	MAE          float64 // Mean Absolute Error
	RMSE         float64 // Root Mean Square Error
	Accuracy     float64 // % predizioni entro 10%
	Timestamp    time.Time
}

// PerformIncrementalTraining esegue training incrementale con nuovi dati
func (t *Trainer) PerformIncrementalTraining(ctx context.Context, newData []TrainingData) error {
	if len(newData) == 0 {
		return nil
	}

	log.Info().
		Int("new_samples", len(newData)).
		Msg("Starting incremental training")

	// Estrai features dai nuovi dati
	features := t.extractFeatures(newData)

	// Retrain (per semplicità, ritraina completamente)
	// In un sistema più sofisticato, si potrebbe fare update incrementale
	if err := t.trainTokenPrediction(features); err != nil {
		return err
	}

	if err := t.trainCostPrediction(features); err != nil {
		return err
	}

	log.Info().Msg("Incremental training completed")

	return nil
}

// GetTrainingMetrics ottiene metriche sul training
func (t *Trainer) GetTrainingMetrics() *TrainingMetrics {
	tokenModel := t.optimizer.predictor.tokenPredictionModel
	costModel := t.optimizer.predictor.costPredictionModel

	return &TrainingMetrics{
		TokenModel: TokenModelMetrics{
			TrainedSamples: tokenModel.TrainedSamples,
			TrainedAt:      tokenModel.TrainedAt,
			RMSE:           tokenModel.RMSE,
			Intercept:      tokenModel.Intercept,
			LengthCoeff:    tokenModel.LengthCoefficient,
			ComplexityCoeff: tokenModel.ComplexityCoefficient,
		},
		CostModel: CostModelMetrics{
			TrainedAt:             costModel.TrainedAt,
			AvgInputCostPerToken:  costModel.AvgCostPerInputToken,
			AvgOutputCostPerToken: costModel.AvgCostPerOutputToken,
		},
	}
}

// TrainingMetrics metriche del training
type TrainingMetrics struct {
	TokenModel TokenModelMetrics
	CostModel  CostModelMetrics
}

// TokenModelMetrics metriche del modello token
type TokenModelMetrics struct {
	TrainedSamples  int
	TrainedAt       time.Time
	RMSE            float64
	Intercept       float64
	LengthCoeff     float64
	ComplexityCoeff float64
}

// CostModelMetrics metriche del modello costo
type CostModelMetrics struct {
	TrainedAt             time.Time
	AvgInputCostPerToken  float64
	AvgOutputCostPerToken float64
}

// Errori
var (
	ErrInsufficientData = fmt.Errorf("insufficient training data")
)
