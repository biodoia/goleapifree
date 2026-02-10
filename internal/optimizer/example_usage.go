package optimizer

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
)

// ExampleUsage dimostra come utilizzare l'AI Cost Optimizer
func ExampleUsage() {
	// Setup database (esempio)
	db := &database.DB{} // Inizializzato dall'applicazione

	// 1. Crea e configura l'optimizer
	config := DefaultOptimizerConfig()
	config.CostWeight = 0.5    // Priorità al costo
	config.QualityWeight = 0.3  // Qualità importante ma secondaria
	config.LatencyWeight = 0.2  // Latenza meno critica

	optimizer := NewOptimizer(db, config)

	// 2. Avvia l'optimizer (background jobs per training e cache refresh)
	if err := optimizer.Start(); err != nil {
		fmt.Printf("Error starting optimizer: %v\n", err)
		return
	}
	defer optimizer.Stop()

	// 3. Ottimizza una richiesta
	ctx := context.Background()
	userID := uuid.New()

	request := &OptimizationRequest{
		UserID:           userID,
		Modality:         models.ModalityChat,
		PromptLength:     500,
		PromptComplexity: 0.7,
		MaxCost:          0.01,  // Max $0.01 per richiesta
		MaxLatencyMs:     3000,  // Max 3 secondi
	}

	result, err := optimizer.OptimizeRequest(ctx, request)
	if err != nil {
		fmt.Printf("Error optimizing request: %v\n", err)
		return
	}

	// 4. Usa il risultato
	fmt.Printf("\n=== OPTIMIZATION RESULT ===\n")
	fmt.Printf("Best Provider: %s\n", result.BestProvider.ProviderName)
	fmt.Printf("Best Model: %s\n", result.BestProvider.ModelName)
	fmt.Printf("Total Score: %.4f\n", result.BestProvider.TotalScore)
	fmt.Printf("Estimated Cost: $%.6f\n", result.BestProvider.EstimatedCost)
	fmt.Printf("Estimated Latency: %dms\n", result.BestProvider.EstimatedLatency)
	fmt.Printf("Success Rate: %.2f%%\n", result.BestProvider.SuccessRate*100)
	fmt.Printf("Savings: $%.6f\n", result.Savings)
	fmt.Printf("Reason: %s\n", result.BestProvider.Reason)

	if len(result.AlternativeProviders) > 0 {
		fmt.Printf("\n=== ALTERNATIVES ===\n")
		for i, alt := range result.AlternativeProviders {
			fmt.Printf("%d. %s / %s - Score: %.4f, Cost: $%.6f\n",
				i+1, alt.ProviderName, alt.ModelName, alt.TotalScore, alt.EstimatedCost)
		}
	}

	// 5. Ottieni raccomandazioni per ridurre costi
	recommendations, err := optimizer.GetRecommendations(ctx, userID)
	if err != nil {
		fmt.Printf("Error getting recommendations: %v\n", err)
		return
	}

	fmt.Printf("\n=== COST REDUCTION RECOMMENDATIONS ===\n")
	fmt.Printf("Total Potential Savings: $%.4f\n", recommendations.TotalPotentialSavings)
	for i, rec := range recommendations.Items {
		fmt.Printf("\n%d. [%s] %s\n", i+1, rec.Priority, rec.Title)
		fmt.Printf("   %s\n", rec.Description)
		fmt.Printf("   Estimated Savings: $%.4f\n", rec.EstimatedSavings)
	}
}

// ExamplePredictor dimostra l'uso del predictor
func ExamplePredictor() {
	db := &database.DB{}
	optimizer := NewOptimizer(db, nil)
	predictor := optimizer.predictor

	ctx := context.Background()

	// 1. Predici token count da lunghezza prompt
	promptLength := 1000
	complexity := 0.6
	inputTokens := predictor.PredictTokenCount(promptLength, complexity)
	fmt.Printf("Predicted input tokens: %d\n", inputTokens)

	// 2. Predici output tokens
	outputTokens := predictor.PredictOutputTokens(inputTokens, nil)
	fmt.Printf("Predicted output tokens: %d\n", outputTokens)

	// 3. Stima costo
	cost := predictor.EstimateCost(inputTokens, outputTokens)
	fmt.Printf("Estimated cost: $%.6f\n", cost)

	// 4. Predici miglior provider
	predReq := &PredictionRequest{
		Modality:     models.ModalityChat,
		PromptLength: promptLength,
		Complexity:   complexity,
	}

	prediction, err := predictor.PredictBestProvider(ctx, predReq)
	if err != nil {
		fmt.Printf("Error predicting provider: %v\n", err)
		return
	}

	fmt.Printf("\n=== PROVIDER PREDICTION ===\n")
	fmt.Printf("Provider: %s\n", prediction.ProviderName)
	fmt.Printf("Model: %s\n", prediction.ModelName)
	fmt.Printf("Estimated Cost: $%.6f\n", prediction.EstimatedCost)
	fmt.Printf("Quality Score: %.2f\n", prediction.QualityScore)
	fmt.Printf("Confidence: %.2f%%\n", prediction.Confidence*100)

	// 5. Suggerisci alternative più economiche
	var currentProvider models.Provider
	var currentModel models.Model

	db.First(&currentProvider)
	db.First(&currentModel)

	alternatives, err := predictor.SuggestCheaperAlternatives(
		ctx, &currentProvider, &currentModel, inputTokens, outputTokens,
	)
	if err != nil {
		fmt.Printf("Error getting alternatives: %v\n", err)
		return
	}

	fmt.Printf("\n=== CHEAPER ALTERNATIVES ===\n")
	for i, alt := range alternatives {
		fmt.Printf("%d. %s / %s\n", i+1, alt.ProviderName, alt.ModelName)
		fmt.Printf("   Cost: $%.6f (Save $%.6f / %.1f%%)\n",
			alt.EstimatedCost, alt.Savings, alt.SavingsPercent)
		fmt.Printf("   Quality: %.2f (diff: %+.2f)\n",
			alt.QualityScore, alt.QualityDiff)
		fmt.Printf("   Reason: %s\n", alt.Reason)
	}

	// 6. Analizza complessità prompt
	prompt := "Write a complex function in Go that implements a binary search tree with insert, delete, and search operations. Include comments and error handling."
	promptComplexity := predictor.AnalyzePromptComplexity(prompt)
	fmt.Printf("\nPrompt complexity: %.2f\n", promptComplexity)
}

// ExampleAnalyzer dimostra l'uso dell'analyzer
func ExampleAnalyzer() {
	db := &database.DB{}
	analyzer := NewAnalyzer(db)

	ctx := context.Background()
	userID := uuid.New()

	// 1. Analizza pattern utente
	patterns := analyzer.AnalyzeUserPatterns(ctx, userID, 7*24*time.Hour)
	if patterns != nil {
		fmt.Printf("\n=== USER PATTERNS (Last 7 days) ===\n")
		fmt.Printf("Total Requests: %d\n", patterns.TotalRequests)
		fmt.Printf("Avg Requests/Day: %.2f\n", patterns.AvgRequestsPerDay)
		fmt.Printf("Peak Hours: %v\n", patterns.PeakHours)
		fmt.Printf("Peak Days: %v\n", patterns.PeakDays)

		fmt.Printf("\n--- Token Usage ---\n")
		fmt.Printf("Avg Input Tokens: %d\n", patterns.AvgInputTokens)
		fmt.Printf("Avg Output Tokens: %d\n", patterns.AvgOutputTokens)
		fmt.Printf("Total Tokens: %d\n", patterns.TotalInputTokens+patterns.TotalOutputTokens)

		fmt.Printf("\n--- Cost Analysis ---\n")
		fmt.Printf("Total Cost: $%.4f\n", patterns.TotalCost)
		fmt.Printf("Avg Cost/Request: $%.6f\n", patterns.AvgCostPerRequest)

		fmt.Printf("\n--- Popular Models ---\n")
		for i, model := range patterns.PopularModels {
			fmt.Printf("%d. %s - %.1f%% (%d requests)\n",
				i+1, model.ModelName, model.Percentage, model.RequestCount)
		}

		fmt.Printf("\n--- Behavior ---\n")
		fmt.Printf("Repetitive Queries: %.1f%%\n", patterns.RepetitiveQueriesRatio*100)
		fmt.Printf("Complexity Score: %.2f\n", patterns.ComplexityScore)
		fmt.Printf("Success Rate: %.2f%%\n", patterns.SuccessRate*100)
		fmt.Printf("Avg Latency: %dms\n", patterns.AvgLatencyMs)
	}

	// 2. Rileva peak hours globali
	peakHours := analyzer.DetectPeakHours(ctx, 24*time.Hour)
	fmt.Printf("\n=== SYSTEM PEAK HOURS ===\n")
	fmt.Printf("Peak hours: %v\n", peakHours)

	// 3. Analizza cost trends
	trends, err := analyzer.AnalyzeCostTrends(ctx, 30*24*time.Hour)
	if err == nil {
		fmt.Printf("\n=== COST TRENDS (Last 30 days) ===\n")
		fmt.Printf("Total Cost: $%.4f\n", trends.TotalCost)
		fmt.Printf("Avg Cost: $%.6f\n", trends.AvgCost)
		fmt.Printf("Trend: %s (%.2f%%)\n", trends.Trend, trends.TrendPercent)
		fmt.Printf("Min Cost: $%.6f\n", trends.MinCost)
		fmt.Printf("Max Cost: $%.6f\n", trends.MaxCost)
	}

	// 4. Traccia modelli popolari
	popular, err := analyzer.TrackPopularModels(ctx, 7*24*time.Hour, 10)
	if err == nil {
		fmt.Printf("\n=== POPULAR MODELS (Last 7 days) ===\n")
		for i, model := range popular {
			fmt.Printf("%d. %s\n", i+1, model.ModelName)
			fmt.Printf("   Requests: %d (%.1f%%)\n", model.RequestCount, model.Percentage)
			fmt.Printf("   Avg Cost: $%.6f\n", model.AvgCost)
			fmt.Printf("   Total Cost: $%.4f\n", model.TotalCost)
			fmt.Printf("   Success Rate: %.2f%%\n", model.SuccessRate*100)
		}
	}
}

// ExampleTrainer dimostra l'uso del trainer
func ExampleTrainer() {
	db := &database.DB{}
	optimizer := NewOptimizer(db, nil)
	trainer := optimizer.trainer

	ctx := context.Background()

	// 1. Esegui training
	fmt.Printf("Starting model training...\n")
	if err := trainer.Train(ctx); err != nil {
		fmt.Printf("Training error: %v\n", err)
		return
	}

	// 2. Valuta il modello
	eval, err := trainer.EvaluateModel(ctx)
	if err != nil {
		fmt.Printf("Evaluation error: %v\n", err)
		return
	}

	fmt.Printf("\n=== MODEL EVALUATION ===\n")
	fmt.Printf("Total Samples: %d\n", eval.TotalSamples)
	fmt.Printf("MAE (Mean Absolute Error): %.2f\n", eval.MAE)
	fmt.Printf("RMSE (Root Mean Square Error): %.2f\n", eval.RMSE)
	fmt.Printf("Accuracy (within 10%%): %.2f%%\n", eval.Accuracy*100)

	// 3. Ottieni metriche training
	metrics := trainer.GetTrainingMetrics()

	fmt.Printf("\n=== TRAINING METRICS ===\n")
	fmt.Printf("Token Model:\n")
	fmt.Printf("  Trained Samples: %d\n", metrics.TokenModel.TrainedSamples)
	fmt.Printf("  Trained At: %s\n", metrics.TokenModel.TrainedAt.Format(time.RFC3339))
	fmt.Printf("  RMSE: %.2f\n", metrics.TokenModel.RMSE)
	fmt.Printf("  Intercept: %.2f\n", metrics.TokenModel.Intercept)
	fmt.Printf("  Length Coefficient: %.4f\n", metrics.TokenModel.LengthCoeff)
	fmt.Printf("  Complexity Coefficient: %.2f\n", metrics.TokenModel.ComplexityCoeff)

	fmt.Printf("\nCost Model:\n")
	fmt.Printf("  Trained At: %s\n", metrics.CostModel.TrainedAt.Format(time.RFC3339))
	fmt.Printf("  Avg Input Cost/Token: $%.8f\n", metrics.CostModel.AvgInputCostPerToken)
	fmt.Printf("  Avg Output Cost/Token: $%.8f\n", metrics.CostModel.AvgOutputCostPerToken)
}

// ExampleIntegration esempio di integrazione completa
func ExampleIntegration() {
	db := &database.DB{}

	// Setup optimizer con configurazione custom
	config := &OptimizerConfig{
		CostWeight:         0.6, // Massima priorità al costo
		QualityWeight:      0.3,
		LatencyWeight:      0.1,
		MinQualityScore:    0.5,
		MaxLatencyMs:       10000,
		MinSuccessRate:     0.75,
		TrainingWindow:     24 * time.Hour,
		MinTrainingSamples: 100,
		RetrainingInterval: 6 * time.Hour,
	}

	optimizer := NewOptimizer(db, config)
	optimizer.Start()
	defer optimizer.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Scenario: Utente fa una richiesta
	fmt.Printf("\n=== SCENARIO: Ottimizzazione Richiesta Utente ===\n\n")

	// 1. Analizza il prompt
	prompt := "Explain how machine learning works in simple terms"
	complexity := optimizer.predictor.AnalyzePromptComplexity(prompt)
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Printf("Complexity: %.2f\n", complexity)

	// 2. Crea richiesta di ottimizzazione
	request := &OptimizationRequest{
		UserID:           userID,
		Modality:         models.ModalityChat,
		PromptLength:     len(prompt),
		PromptComplexity: complexity,
	}

	// 3. Ottimizza
	result, err := optimizer.OptimizeRequest(ctx, request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// 4. Mostra risultati
	fmt.Printf("\nOTTIMIZZAZIONE COMPLETATA:\n")
	fmt.Printf("Provider consigliato: %s / %s\n",
		result.BestProvider.ProviderName,
		result.BestProvider.ModelName)
	fmt.Printf("Costo stimato: $%.6f\n", result.BestProvider.EstimatedCost)
	fmt.Printf("Latenza stimata: %dms\n", result.BestProvider.EstimatedLatency)
	fmt.Printf("Risparmio vs alternativa più costosa: $%.6f\n", result.Savings)

	// 5. Mostra raccomandazioni generali
	recs, _ := optimizer.GetRecommendations(ctx, userID)
	if recs != nil && len(recs.Items) > 0 {
		fmt.Printf("\nRACCOMANDAZIONI PER RIDURRE I COSTI:\n")
		for _, rec := range recs.Items {
			fmt.Printf("- [%s] %s\n", rec.Priority, rec.Title)
			fmt.Printf("  Risparmio stimato: $%.4f\n", rec.EstimatedSavings)
		}
		fmt.Printf("\nRisparmio totale potenziale: $%.4f\n", recs.TotalPotentialSavings)
	}
}
