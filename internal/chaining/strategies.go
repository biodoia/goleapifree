package chaining

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/rs/zerolog/log"
)

// Strategy definisce come eseguire una pipeline
type Strategy interface {
	// Execute esegue la pipeline secondo la strategia
	Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error)

	// Name restituisce il nome della strategia
	Name() string
}

// DraftRefineStrategy esegue draft con modello veloce e refine con modello potente
type DraftRefineStrategy struct {
	draftPrompt  string // Prompt per lo stage di draft
	refinePrompt string // Prompt per lo stage di refine
}

// NewDraftRefineStrategy crea una nuova strategia draft-refine
func NewDraftRefineStrategy(draftPrompt, refinePrompt string) *DraftRefineStrategy {
	if draftPrompt == "" {
		draftPrompt = "Provide a quick draft response to the following request."
	}
	if refinePrompt == "" {
		refinePrompt = "Refine and improve the following draft response, making it more accurate, detailed, and well-structured."
	}

	return &DraftRefineStrategy{
		draftPrompt:  draftPrompt,
		refinePrompt: refinePrompt,
	}
}

func (s *DraftRefineStrategy) Name() string {
	return "draft-refine"
}

func (s *DraftRefineStrategy) Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error) {
	if len(stages) < 2 {
		return nil, fmt.Errorf("draft-refine strategy requires at least 2 stages")
	}

	result := &PipelineResult{
		StageOutputs: make([]*StageOutput, 0, len(stages)),
		Metadata:     make(map[string]interface{}),
	}

	// Stage 1: Draft (modello veloce)
	draftStage := stages[0]
	draftInput := &StageInput{
		OriginalRequest: req,
		PreviousOutputs: make([]*StageOutput, 0),
		Context: map[string]interface{}{
			"phase": "draft",
		},
	}

	draftOutput, err := s.executeStageSafe(ctx, draftStage, draftInput)
	if err != nil {
		return nil, fmt.Errorf("draft stage failed: %w", err)
	}

	result.StageOutputs = append(result.StageOutputs, draftOutput)
	result.TotalTokens += draftOutput.TokensUsed

	// Stage 2+: Refine (modelli potenti)
	for i := 1; i < len(stages); i++ {
		refineStage := stages[i]
		refineInput := &StageInput{
			OriginalRequest: req,
			PreviousOutputs: result.StageOutputs,
			Context: map[string]interface{}{
				"phase":        "refine",
				"refinement":   i,
				"draft_output": draftOutput.Response.Choices[0].Message.Content,
			},
		}

		refineOutput, err := s.executeStageSafe(ctx, refineStage, refineInput)
		if err != nil {
			return nil, fmt.Errorf("refine stage %d failed: %w", i, err)
		}

		result.StageOutputs = append(result.StageOutputs, refineOutput)
		result.TotalTokens += refineOutput.TokensUsed
	}

	// L'output finale è l'ultimo refine
	result.FinalResponse = result.StageOutputs[len(result.StageOutputs)-1].Response
	result.Metadata["strategy"] = s.Name()
	result.Metadata["draft_tokens"] = draftOutput.TokensUsed
	result.Metadata["refine_count"] = len(stages) - 1

	return result, nil
}

func (s *DraftRefineStrategy) executeStageSafe(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error) {
	startTime := time.Now()

	req, err := stage.Transformer.TransformInput(ctx, input)
	if err != nil {
		return nil, err
	}

	resp, err := stage.Provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	output, err := stage.Transformer.TransformOutput(ctx, resp)
	if err != nil {
		return nil, err
	}

	output.StageName = stage.Name
	output.Duration = time.Since(startTime)
	output.TokensUsed = resp.Usage.TotalTokens

	return output, nil
}

// ParallelConsensusStrategy esegue più modelli in parallelo e combina i risultati
type ParallelConsensusStrategy struct {
	votingMethod string // "majority", "weighted", "best_quality"
}

// NewParallelConsensusStrategy crea una nuova strategia di consensus parallelo
func NewParallelConsensusStrategy(votingMethod string) *ParallelConsensusStrategy {
	if votingMethod == "" {
		votingMethod = "majority"
	}

	return &ParallelConsensusStrategy{
		votingMethod: votingMethod,
	}
}

func (s *ParallelConsensusStrategy) Name() string {
	return "parallel-consensus"
}

func (s *ParallelConsensusStrategy) Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error) {
	if len(stages) < 2 {
		return nil, fmt.Errorf("parallel-consensus strategy requires at least 2 stages")
	}

	result := &PipelineResult{
		StageOutputs: make([]*StageOutput, 0, len(stages)),
		Metadata: map[string]interface{}{
			"strategy":      s.Name(),
			"voting_method": s.votingMethod,
		},
	}

	// Esegui tutti gli stage in parallelo
	var wg sync.WaitGroup
	outputChan := make(chan *StageOutput, len(stages))
	errorChan := make(chan error, len(stages))

	for _, stage := range stages {
		wg.Add(1)
		go func(st Stage) {
			defer wg.Done()

			input := &StageInput{
				OriginalRequest: req,
				PreviousOutputs: make([]*StageOutput, 0),
				Context: map[string]interface{}{
					"phase": "parallel",
				},
			}

			output, err := s.executeStageSafe(ctx, st, input)
			if err != nil {
				errorChan <- err
				return
			}

			outputChan <- output
		}(stage)
	}

	// Aspetta che tutti completino
	go func() {
		wg.Wait()
		close(outputChan)
		close(errorChan)
	}()

	// Raccogli risultati
	for output := range outputChan {
		result.StageOutputs = append(result.StageOutputs, output)
		result.TotalTokens += output.TokensUsed
	}

	// Controlla errori
	for err := range errorChan {
		log.Warn().Err(err).Msg("Stage failed in parallel execution")
	}

	if len(result.StageOutputs) == 0 {
		return nil, fmt.Errorf("all parallel stages failed")
	}

	// Combina risultati secondo il metodo di voting
	finalResponse, err := s.combineResponses(result.StageOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to combine responses: %w", err)
	}

	result.FinalResponse = finalResponse
	result.Metadata["successful_stages"] = len(result.StageOutputs)
	result.Metadata["total_stages"] = len(stages)

	return result, nil
}

func (s *ParallelConsensusStrategy) executeStageSafe(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error) {
	startTime := time.Now()

	req, err := stage.Transformer.TransformInput(ctx, input)
	if err != nil {
		return nil, err
	}

	resp, err := stage.Provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	output, err := stage.Transformer.TransformOutput(ctx, resp)
	if err != nil {
		return nil, err
	}

	output.StageName = stage.Name
	output.Duration = time.Since(startTime)
	output.TokensUsed = resp.Usage.TotalTokens

	return output, nil
}

func (s *ParallelConsensusStrategy) combineResponses(outputs []*StageOutput) (*providers.ChatResponse, error) {
	if len(outputs) == 0 {
		return nil, fmt.Errorf("no outputs to combine")
	}

	switch s.votingMethod {
	case "majority":
		return s.majorityVote(outputs)
	case "weighted":
		return s.weightedVote(outputs)
	case "best_quality":
		return s.bestQuality(outputs)
	default:
		return outputs[0].Response, nil
	}
}

func (s *ParallelConsensusStrategy) majorityVote(outputs []*StageOutput) (*providers.ChatResponse, error) {
	// Per semplicità, usa la risposta più lunga (euristica)
	bestOutput := outputs[0]
	maxLength := len(fmt.Sprintf("%v", outputs[0].Response.Choices[0].Message.Content))

	for _, output := range outputs[1:] {
		length := len(fmt.Sprintf("%v", output.Response.Choices[0].Message.Content))
		if length > maxLength {
			maxLength = length
			bestOutput = output
		}
	}

	return bestOutput.Response, nil
}

func (s *ParallelConsensusStrategy) weightedVote(outputs []*StageOutput) (*providers.ChatResponse, error) {
	// Peso basato su velocità (inversamente proporzionale alla durata)
	bestOutput := outputs[0]
	bestScore := float64(0)

	for _, output := range outputs {
		// Score: qualità percepita / durata
		length := len(fmt.Sprintf("%v", output.Response.Choices[0].Message.Content))
		score := float64(length) / output.Duration.Seconds()

		if score > bestScore {
			bestScore = score
			bestOutput = output
		}
	}

	return bestOutput.Response, nil
}

func (s *ParallelConsensusStrategy) bestQuality(outputs []*StageOutput) (*providers.ChatResponse, error) {
	// Usa la risposta più lunga e dettagliata
	return s.majorityVote(outputs)
}

// SpeculativeDecodingStrategy usa un modello veloce per generare tokens e uno lento per verificare
type SpeculativeDecodingStrategy struct {
	maxSpeculativeTokens int
	acceptanceThreshold  float64
}

// NewSpeculativeDecodingStrategy crea una nuova strategia di speculative decoding
func NewSpeculativeDecodingStrategy(maxTokens int, threshold float64) *SpeculativeDecodingStrategy {
	if maxTokens == 0 {
		maxTokens = 5
	}
	if threshold == 0 {
		threshold = 0.8
	}

	return &SpeculativeDecodingStrategy{
		maxSpeculativeTokens: maxTokens,
		acceptanceThreshold:  threshold,
	}
}

func (s *SpeculativeDecodingStrategy) Name() string {
	return "speculative-decoding"
}

func (s *SpeculativeDecodingStrategy) Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error) {
	if len(stages) < 2 {
		return nil, fmt.Errorf("speculative-decoding strategy requires at least 2 stages")
	}

	// Per ora implementazione semplificata: usa draft-refine
	// Una vera implementazione richiederebbe token-level speculation
	draftRefine := NewDraftRefineStrategy("", "")
	return draftRefine.Execute(ctx, req, stages)
}

// CascadeStrategy prova il modello veloce, se fallisce usa quello lento
type CascadeStrategy struct {
	fastThreshold    time.Duration // Se il fast completa entro questo tempo, usa il risultato
	qualityCheck     bool          // Se true, verifica la qualità prima di accettare
	minResponseLen   int           // Lunghezza minima della risposta per considerarla valida
}

// NewCascadeStrategy crea una nuova strategia a cascata
func NewCascadeStrategy(fastThreshold time.Duration, qualityCheck bool, minResponseLen int) *CascadeStrategy {
	if fastThreshold == 0 {
		fastThreshold = 2 * time.Second
	}
	if minResponseLen == 0 {
		minResponseLen = 50
	}

	return &CascadeStrategy{
		fastThreshold:  fastThreshold,
		qualityCheck:   qualityCheck,
		minResponseLen: minResponseLen,
	}
}

func (s *CascadeStrategy) Name() string {
	return "cascade"
}

func (s *CascadeStrategy) Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error) {
	if len(stages) == 0 {
		return nil, fmt.Errorf("cascade strategy requires at least 1 stage")
	}

	result := &PipelineResult{
		StageOutputs: make([]*StageOutput, 0, len(stages)),
		Metadata: map[string]interface{}{
			"strategy": s.Name(),
		},
	}

	// Prova stage in sequenza fino a trovarne uno che funziona
	for i, stage := range stages {
		log.Debug().
			Str("stage", stage.Name).
			Int("stage_index", i).
			Msg("Trying cascade stage")

		input := &StageInput{
			OriginalRequest: req,
			PreviousOutputs: result.StageOutputs,
			Context: map[string]interface{}{
				"phase":       "cascade",
				"stage_index": i,
			},
		}

		output, err := s.executeStageSafe(ctx, stage, input)
		if err != nil {
			log.Warn().
				Err(err).
				Str("stage", stage.Name).
				Msg("Stage failed, trying next")
			continue
		}

		result.StageOutputs = append(result.StageOutputs, output)
		result.TotalTokens += output.TokensUsed

		// Verifica qualità se richiesto
		if s.qualityCheck {
			if !s.isQualityAcceptable(output) {
				log.Debug().
					Str("stage", stage.Name).
					Msg("Quality check failed, trying next stage")
				continue
			}
		}

		// Se siamo qui, lo stage ha avuto successo
		result.FinalResponse = output.Response
		result.Metadata["successful_stage"] = i
		result.Metadata["stages_tried"] = i + 1

		log.Info().
			Str("stage", stage.Name).
			Int("stage_index", i).
			Msg("Cascade stage succeeded")

		return result, nil
	}

	return nil, fmt.Errorf("all cascade stages failed")
}

func (s *CascadeStrategy) executeStageSafe(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error) {
	startTime := time.Now()

	// Applica timeout per stage veloci
	if stage.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
	}

	req, err := stage.Transformer.TransformInput(ctx, input)
	if err != nil {
		return nil, err
	}

	resp, err := stage.Provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	output, err := stage.Transformer.TransformOutput(ctx, resp)
	if err != nil {
		return nil, err
	}

	output.StageName = stage.Name
	output.Duration = time.Since(startTime)
	output.TokensUsed = resp.Usage.TotalTokens

	return output, nil
}

func (s *CascadeStrategy) isQualityAcceptable(output *StageOutput) bool {
	if len(output.Response.Choices) == 0 {
		return false
	}

	content := fmt.Sprintf("%v", output.Response.Choices[0].Message.Content)

	// Verifica lunghezza minima
	if len(content) < s.minResponseLen {
		return false
	}

	// Verifica che non sia un errore
	if output.Response.Choices[0].FinishReason == "error" {
		return false
	}

	return true
}

// SequentialStrategy esegue stage in sequenza, passando output come input
type SequentialStrategy struct{}

func NewSequentialStrategy() *SequentialStrategy {
	return &SequentialStrategy{}
}

func (s *SequentialStrategy) Name() string {
	return "sequential"
}

func (s *SequentialStrategy) Execute(ctx context.Context, req *providers.ChatRequest, stages []Stage) (*PipelineResult, error) {
	result := &PipelineResult{
		StageOutputs: make([]*StageOutput, 0, len(stages)),
		Metadata: map[string]interface{}{
			"strategy": s.Name(),
		},
	}

	for i, stage := range stages {
		input := &StageInput{
			OriginalRequest: req,
			PreviousOutputs: result.StageOutputs,
			Context: map[string]interface{}{
				"phase":       "sequential",
				"stage_index": i,
			},
		}

		output, err := s.executeStageSafe(ctx, stage, input)
		if err != nil {
			if stage.Optional {
				log.Warn().
					Err(err).
					Str("stage", stage.Name).
					Msg("Optional stage failed, continuing")
				continue
			}
			return nil, fmt.Errorf("stage %s failed: %w", stage.Name, err)
		}

		result.StageOutputs = append(result.StageOutputs, output)
		result.TotalTokens += output.TokensUsed
	}

	if len(result.StageOutputs) == 0 {
		return nil, fmt.Errorf("no stages produced output")
	}

	result.FinalResponse = result.StageOutputs[len(result.StageOutputs)-1].Response
	return result, nil
}

func (s *SequentialStrategy) executeStageSafe(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error) {
	startTime := time.Now()

	req, err := stage.Transformer.TransformInput(ctx, input)
	if err != nil {
		return nil, err
	}

	resp, err := stage.Provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	output, err := stage.Transformer.TransformOutput(ctx, resp)
	if err != nil {
		return nil, err
	}

	output.StageName = stage.Name
	output.Duration = time.Since(startTime)
	output.TokensUsed = resp.Usage.TotalTokens

	return output, nil
}
