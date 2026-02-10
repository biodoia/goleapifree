package chaining

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/rs/zerolog/log"
)

// Pipeline rappresenta una pipeline di modelli concatenati
type Pipeline struct {
	stages   []Stage
	strategy Strategy
	metrics  *PipelineMetrics
	mu       sync.RWMutex
}

// Stage rappresenta uno stadio della pipeline
type Stage struct {
	Name        string
	Provider    providers.Provider
	Model       string
	Transformer Transformer // Trasforma input/output tra stage
	Parallel    bool        // Se true, può essere eseguito in parallelo
	Optional    bool        // Se true, il fallimento non blocca la pipeline
	Timeout     time.Duration
	MaxRetries  int
}

// Transformer trasforma messaggi tra stage
type Transformer interface {
	// TransformInput prepara l'input per questo stage
	TransformInput(ctx context.Context, input *StageInput) (*providers.ChatRequest, error)

	// TransformOutput elabora l'output di questo stage
	TransformOutput(ctx context.Context, output *providers.ChatResponse) (*StageOutput, error)
}

// StageInput rappresenta l'input per uno stage
type StageInput struct {
	OriginalRequest *providers.ChatRequest
	PreviousOutputs []*StageOutput
	Context         map[string]interface{}
}

// StageOutput rappresenta l'output di uno stage
type StageOutput struct {
	StageName string
	Response  *providers.ChatResponse
	Metadata  map[string]interface{}
	Duration  time.Duration
	TokensUsed int
}

// PipelineResult rappresenta il risultato finale della pipeline
type PipelineResult struct {
	FinalResponse *providers.ChatResponse
	StageOutputs  []*StageOutput
	TotalDuration time.Duration
	TotalTokens   int
	TotalCost     float64
	Metadata      map[string]interface{}
}

// PipelineMetrics raccoglie metriche sulla pipeline
type PipelineMetrics struct {
	TotalExecutions  int64
	SuccessfulRuns   int64
	FailedRuns       int64
	AverageDuration  time.Duration
	TotalTokens      int64
	TotalCost        float64
	StageMetrics     map[string]*StageMetrics
	mu               sync.RWMutex
}

// StageMetrics rappresenta metriche per uno stage specifico
type StageMetrics struct {
	Executions      int64
	Failures        int64
	AverageDuration time.Duration
	TotalTokens     int64
}

// NewPipeline crea una nuova pipeline
func NewPipeline(strategy Strategy) *Pipeline {
	return &Pipeline{
		stages:   make([]Stage, 0),
		strategy: strategy,
		metrics: &PipelineMetrics{
			StageMetrics: make(map[string]*StageMetrics),
		},
	}
}

// AddStage aggiunge uno stage alla pipeline
func (p *Pipeline) AddStage(stage Stage) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stages = append(p.stages, stage)

	// Inizializza metriche per questo stage
	if _, exists := p.metrics.StageMetrics[stage.Name]; !exists {
		p.metrics.StageMetrics[stage.Name] = &StageMetrics{}
	}
}

// Execute esegue la pipeline
func (p *Pipeline) Execute(ctx context.Context, req *providers.ChatRequest) (*PipelineResult, error) {
	startTime := time.Now()

	log.Info().
		Str("strategy", fmt.Sprintf("%T", p.strategy)).
		Int("stages", len(p.stages)).
		Msg("Executing pipeline")

	// Esegui la pipeline usando la strategia configurata
	result, err := p.strategy.Execute(ctx, req, p.stages)
	if err != nil {
		p.recordFailure()
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Calcola durata totale
	result.TotalDuration = time.Since(startTime)

	// Aggiorna metriche
	p.updateMetrics(result)

	log.Info().
		Dur("duration", result.TotalDuration).
		Int("total_tokens", result.TotalTokens).
		Float64("total_cost", result.TotalCost).
		Msg("Pipeline executed successfully")

	return result, nil
}

// ExecuteStream esegue la pipeline con streaming
func (p *Pipeline) ExecuteStream(ctx context.Context, req *providers.ChatRequest, handler providers.StreamHandler) error {
	// Per lo streaming, usiamo solo l'ultimo stage della pipeline
	// dopo aver pre-processato con gli stage precedenti

	if len(p.stages) == 0 {
		return fmt.Errorf("pipeline has no stages")
	}

	// Se c'è un solo stage, esegui direttamente lo streaming
	if len(p.stages) == 1 {
		stage := p.stages[0]
		return stage.Provider.Stream(ctx, req, handler)
	}

	// Altrimenti esegui tutti gli stage tranne l'ultimo in modo normale
	intermediateStages := p.stages[:len(p.stages)-1]
	finalStage := p.stages[len(p.stages)-1]

	// Esegui stage intermedi
	stageInput := &StageInput{
		OriginalRequest: req,
		PreviousOutputs: make([]*StageOutput, 0),
		Context:         make(map[string]interface{}),
	}

	for _, stage := range intermediateStages {
		output, err := p.executeStage(ctx, stage, stageInput)
		if err != nil && !stage.Optional {
			return fmt.Errorf("intermediate stage %s failed: %w", stage.Name, err)
		}
		if output != nil {
			stageInput.PreviousOutputs = append(stageInput.PreviousOutputs, output)
		}
	}

	// Prepara input per lo stage finale
	finalReq, err := finalStage.Transformer.TransformInput(ctx, stageInput)
	if err != nil {
		return fmt.Errorf("failed to transform input for final stage: %w", err)
	}

	// Esegui lo stage finale con streaming
	return finalStage.Provider.Stream(ctx, finalReq, handler)
}

// executeStage esegue un singolo stage
func (p *Pipeline) executeStage(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error) {
	startTime := time.Now()

	log.Debug().
		Str("stage", stage.Name).
		Str("model", stage.Model).
		Msg("Executing stage")

	// Applica timeout se configurato
	if stage.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
	}

	// Trasforma input
	req, err := stage.Transformer.TransformInput(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("input transformation failed: %w", err)
	}

	// Esegui con retry
	var resp *providers.ChatResponse
	maxRetries := stage.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Warn().
				Str("stage", stage.Name).
				Int("attempt", attempt+1).
				Msg("Retrying stage execution")

			// Backoff esponenziale
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err = stage.Provider.ChatCompletion(ctx, req)
		if err == nil {
			break
		}

		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("stage failed after %d attempts: %w", maxRetries, err)
		}
	}

	// Trasforma output
	output, err := stage.Transformer.TransformOutput(ctx, resp)
	if err != nil {
		return nil, fmt.Errorf("output transformation failed: %w", err)
	}

	// Aggiungi metadati
	output.StageName = stage.Name
	output.Duration = time.Since(startTime)
	output.TokensUsed = resp.Usage.TotalTokens

	// Aggiorna metriche dello stage
	p.updateStageMetrics(stage.Name, output)

	log.Debug().
		Str("stage", stage.Name).
		Dur("duration", output.Duration).
		Int("tokens", output.TokensUsed).
		Msg("Stage completed")

	return output, nil
}

// updateMetrics aggiorna le metriche della pipeline
func (p *Pipeline) updateMetrics(result *PipelineResult) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.TotalExecutions++
	p.metrics.SuccessfulRuns++

	// Aggiorna durata media
	if p.metrics.TotalExecutions == 1 {
		p.metrics.AverageDuration = result.TotalDuration
	} else {
		p.metrics.AverageDuration = time.Duration(
			(int64(p.metrics.AverageDuration)*(p.metrics.TotalExecutions-1) +
				int64(result.TotalDuration)) / p.metrics.TotalExecutions,
		)
	}

	p.metrics.TotalTokens += int64(result.TotalTokens)
	p.metrics.TotalCost += result.TotalCost
}

// updateStageMetrics aggiorna le metriche di uno stage
func (p *Pipeline) updateStageMetrics(stageName string, output *StageOutput) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	stageMetrics := p.metrics.StageMetrics[stageName]
	stageMetrics.Executions++
	stageMetrics.TotalTokens += int64(output.TokensUsed)

	// Aggiorna durata media
	if stageMetrics.Executions == 1 {
		stageMetrics.AverageDuration = output.Duration
	} else {
		stageMetrics.AverageDuration = time.Duration(
			(int64(stageMetrics.AverageDuration)*(stageMetrics.Executions-1) +
				int64(output.Duration)) / stageMetrics.Executions,
		)
	}
}

// recordFailure registra un fallimento della pipeline
func (p *Pipeline) recordFailure() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.TotalExecutions++
	p.metrics.FailedRuns++
}

// GetMetrics restituisce le metriche correnti
func (p *Pipeline) GetMetrics() *PipelineMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	// Crea copia profonda per evitare race conditions
	metricsCopy := &PipelineMetrics{
		TotalExecutions:  p.metrics.TotalExecutions,
		SuccessfulRuns:   p.metrics.SuccessfulRuns,
		FailedRuns:       p.metrics.FailedRuns,
		AverageDuration:  p.metrics.AverageDuration,
		TotalTokens:      p.metrics.TotalTokens,
		TotalCost:        p.metrics.TotalCost,
		StageMetrics:     make(map[string]*StageMetrics),
	}

	for name, metrics := range p.metrics.StageMetrics {
		metricsCopy.StageMetrics[name] = &StageMetrics{
			Executions:      metrics.Executions,
			Failures:        metrics.Failures,
			AverageDuration: metrics.AverageDuration,
			TotalTokens:     metrics.TotalTokens,
		}
	}

	return metricsCopy
}

// DefaultTransformer è un transformer di base che passa i dati senza modifiche
type DefaultTransformer struct{}

func (t *DefaultTransformer) TransformInput(ctx context.Context, input *StageInput) (*providers.ChatRequest, error) {
	// Se ci sono output precedenti, usa l'ultimo come input
	if len(input.PreviousOutputs) > 0 {
		lastOutput := input.PreviousOutputs[len(input.PreviousOutputs)-1]

		// Crea una nuova richiesta con la risposta precedente come messaggio user
		req := &providers.ChatRequest{
			Model:       input.OriginalRequest.Model,
			Messages:    append(input.OriginalRequest.Messages, lastOutput.Response.Choices[0].Message),
			Temperature: input.OriginalRequest.Temperature,
			MaxTokens:   input.OriginalRequest.MaxTokens,
		}
		return req, nil
	}

	return input.OriginalRequest, nil
}

func (t *DefaultTransformer) TransformOutput(ctx context.Context, output *providers.ChatResponse) (*StageOutput, error) {
	return &StageOutput{
		Response: output,
		Metadata: make(map[string]interface{}),
	}, nil
}
