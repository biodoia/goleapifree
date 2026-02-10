package agents

import (
	"context"
	"fmt"
	"sync"

	"github.com/biodoia/goleapifree/internal/providers"
)

// ChainType rappresenta il tipo di chain
type ChainType string

const (
	// ChainTypeSequential esegue gli agenti in sequenza
	ChainTypeSequential ChainType = "sequential"

	// ChainTypeParallel esegue gli agenti in parallelo
	ChainTypeParallel ChainType = "parallel"

	// ChainTypeDraftRefine pattern Draft → Refine
	ChainTypeDraftRefine ChainType = "draft_refine"

	// ChainTypeMultiStep reasoning multi-step
	ChainTypeMultiStep ChainType = "multi_step"

	// ChainTypeConsensus multiple agents votano sulla risposta migliore
	ChainTypeConsensus ChainType = "consensus"
)

// ChainStep rappresenta uno step nella chain
type ChainStep struct {
	// Nome dello step
	Name string

	// Agente da utilizzare
	Agent Agent

	// Prompt template per questo step
	PromptTemplate string

	// Dipendenze da step precedenti
	Dependencies []string

	// Metadata
	Metadata map[string]interface{}
}

// Chain rappresenta una catena di agenti
type Chain struct {
	// Tipo di chain
	Type ChainType

	// Steps della chain
	Steps []*ChainStep

	// Orchestrator per gestire i providers
	orchestrator *Orchestrator

	// Risultati intermedi
	intermediateResults map[string]*TaskResult

	mu sync.RWMutex
}

// ChainResult rappresenta il risultato finale di una chain
type ChainResult struct {
	// Risultato finale
	FinalResult *TaskResult

	// Risultati intermedi di ogni step
	IntermediateResults map[string]*TaskResult

	// Metadata della chain
	Metadata map[string]interface{}
}

// NewChain crea una nuova Chain
func NewChain(chainType ChainType, orchestrator *Orchestrator) *Chain {
	return &Chain{
		Type:                chainType,
		Steps:               []*ChainStep{},
		orchestrator:        orchestrator,
		intermediateResults: make(map[string]*TaskResult),
	}
}

// AddStep aggiunge uno step alla chain
func (c *Chain) AddStep(step *ChainStep) {
	c.Steps = append(c.Steps, step)
}

// Execute esegue la chain
func (c *Chain) Execute(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	switch c.Type {
	case ChainTypeSequential:
		return c.executeSequential(ctx, initialInput, messages)
	case ChainTypeParallel:
		return c.executeParallel(ctx, initialInput, messages)
	case ChainTypeDraftRefine:
		return c.executeDraftRefine(ctx, initialInput, messages)
	case ChainTypeMultiStep:
		return c.executeMultiStep(ctx, initialInput, messages)
	case ChainTypeConsensus:
		return c.executeConsensus(ctx, initialInput, messages)
	default:
		return nil, fmt.Errorf("unsupported chain type: %s", c.Type)
	}
}

// executeSequential esegue gli step in sequenza
func (c *Chain) executeSequential(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	currentInput := initialInput
	currentMessages := messages

	for i, step := range c.Steps {
		// Prepara il prompt per questo step
		prompt := c.preparePrompt(step, currentInput)

		// Crea i messaggi per questo step
		stepMessages := append([]providers.Message{}, currentMessages...)
		stepMessages = append(stepMessages, providers.Message{
			Role:    "user",
			Content: prompt,
		})

		// Seleziona modello e provider per questo step
		taskContext := c.orchestrator.analyzer.Analyze(prompt, stepMessages)
		modelID, provider, err := c.orchestrator.SelectModel(ctx, step.Agent, taskContext)
		if err != nil {
			return nil, fmt.Errorf("step %d failed to select model: %w", i, err)
		}

		// Crea task
		task := &Task{
			Type:     TaskType(step.Agent.Type()),
			Model:    modelID,
			Messages: stepMessages,
			Metadata: map[string]interface{}{
				"step_name": step.Name,
				"step_index": i,
			},
		}

		// Esegui step
		result, err := step.Agent.Execute(ctx, task, provider)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s) failed: %w", i, step.Name, err)
		}

		// Salva risultato intermedio
		c.intermediateResults[step.Name] = result

		// L'output diventa input per il prossimo step
		currentInput = result.Content
		currentMessages = append(currentMessages, providers.Message{
			Role:    "assistant",
			Content: result.Content,
		})
	}

	// Risultato finale è l'ultimo step
	lastStep := c.Steps[len(c.Steps)-1]
	finalResult := c.intermediateResults[lastStep.Name]

	return &ChainResult{
		FinalResult:         finalResult,
		IntermediateResults: c.intermediateResults,
		Metadata: map[string]interface{}{
			"chain_type": c.Type,
			"steps_count": len(c.Steps),
		},
	}, nil
}

// executeParallel esegue gli step in parallelo
func (c *Chain) executeParallel(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	var wg sync.WaitGroup
	results := make(map[string]*TaskResult)
	errors := make(map[string]error)
	var mu sync.Mutex

	for i, step := range c.Steps {
		wg.Add(1)
		go func(idx int, s *ChainStep) {
			defer wg.Done()

			// Prepara il prompt per questo step
			prompt := c.preparePrompt(s, initialInput)

			// Crea i messaggi per questo step
			stepMessages := append([]providers.Message{}, messages...)
			stepMessages = append(stepMessages, providers.Message{
				Role:    "user",
				Content: prompt,
			})

			// Seleziona modello e provider
			taskContext := c.orchestrator.analyzer.Analyze(prompt, stepMessages)
			modelID, provider, err := c.orchestrator.SelectModel(ctx, s.Agent, taskContext)
			if err != nil {
				mu.Lock()
				errors[s.Name] = fmt.Errorf("failed to select model: %w", err)
				mu.Unlock()
				return
			}

			// Crea task
			task := &Task{
				Type:     TaskType(s.Agent.Type()),
				Model:    modelID,
				Messages: stepMessages,
				Metadata: map[string]interface{}{
					"step_name": s.Name,
					"step_index": idx,
				},
			}

			// Esegui step
			result, err := s.Agent.Execute(ctx, task, provider)
			if err != nil {
				mu.Lock()
				errors[s.Name] = err
				mu.Unlock()
				return
			}

			mu.Lock()
			results[s.Name] = result
			mu.Unlock()
		}(i, step)
	}

	wg.Wait()

	// Controlla errori
	if len(errors) > 0 {
		return nil, fmt.Errorf("parallel execution failed with %d errors", len(errors))
	}

	// Aggrega i risultati
	aggregatedResult := c.aggregateResults(results)

	return &ChainResult{
		FinalResult:         aggregatedResult,
		IntermediateResults: results,
		Metadata: map[string]interface{}{
			"chain_type": c.Type,
			"steps_count": len(c.Steps),
			"parallel_execution": true,
		},
	}, nil
}

// executeDraftRefine esegue pattern Draft → Refine
func (c *Chain) executeDraftRefine(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	if len(c.Steps) < 2 {
		return nil, fmt.Errorf("draft-refine chain requires at least 2 steps")
	}

	// Step 1: Draft (prima bozza veloce)
	draftStep := c.Steps[0]
	draftPrompt := c.preparePrompt(draftStep, initialInput)

	draftMessages := append([]providers.Message{}, messages...)
	draftMessages = append(draftMessages, providers.Message{
		Role:    "user",
		Content: draftPrompt,
	})

	// Usa un agente fast per la bozza
	fastAgent := c.orchestrator.agents[AgentTypeFast]
	taskContext := c.orchestrator.analyzer.Analyze(draftPrompt, draftMessages)
	modelID, provider, err := c.orchestrator.SelectModel(ctx, fastAgent, taskContext)
	if err != nil {
		return nil, fmt.Errorf("draft step failed to select model: %w", err)
	}

	draftTask := &Task{
		Type:     TaskTypeFast,
		Model:    modelID,
		Messages: draftMessages,
	}

	draftResult, err := fastAgent.Execute(ctx, draftTask, provider)
	if err != nil {
		return nil, fmt.Errorf("draft step failed: %w", err)
	}

	c.intermediateResults["draft"] = draftResult

	// Step 2: Refine (raffinamento con modello di qualità)
	refineStep := c.Steps[1]
	refinePrompt := fmt.Sprintf(
		"%s\n\nHere is a draft response:\n%s\n\nPlease refine and improve this response.",
		initialInput,
		draftResult.Content,
	)

	refineMessages := append([]providers.Message{}, messages...)
	refineMessages = append(refineMessages, providers.Message{
		Role:    "user",
		Content: refinePrompt,
	})

	refineContext := c.orchestrator.analyzer.Analyze(refinePrompt, refineMessages)
	refineContext.RequiresHighQuality = true // Forza alta qualità per refinement

	refineModelID, refineProvider, err := c.orchestrator.SelectModel(ctx, refineStep.Agent, refineContext)
	if err != nil {
		return nil, fmt.Errorf("refine step failed to select model: %w", err)
	}

	refineTask := &Task{
		Type:     TaskType(refineStep.Agent.Type()),
		Model:    refineModelID,
		Messages: refineMessages,
	}

	refineResult, err := refineStep.Agent.Execute(ctx, refineTask, refineProvider)
	if err != nil {
		return nil, fmt.Errorf("refine step failed: %w", err)
	}

	c.intermediateResults["refined"] = refineResult

	return &ChainResult{
		FinalResult:         refineResult,
		IntermediateResults: c.intermediateResults,
		Metadata: map[string]interface{}{
			"chain_type": ChainTypeDraftRefine,
			"draft_model": draftResult.Model,
			"refine_model": refineResult.Model,
		},
	}, nil
}

// executeMultiStep esegue reasoning multi-step
func (c *Chain) executeMultiStep(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	// Multi-step reasoning: scompone il problema in step logici

	// Step 1: Analizza e scomponi il problema
	analyzePrompt := fmt.Sprintf(
		"Break down this problem into logical steps:\n%s\n\nProvide a numbered list of steps to solve this.",
		initialInput,
	)

	analysisAgent := c.orchestrator.agents[AgentTypeAnalysis]
	analyzeMessages := append([]providers.Message{}, messages...)
	analyzeMessages = append(analyzeMessages, providers.Message{
		Role:    "user",
		Content: analyzePrompt,
	})

	taskContext := c.orchestrator.analyzer.Analyze(analyzePrompt, analyzeMessages)
	modelID, provider, err := c.orchestrator.SelectModel(ctx, analysisAgent, taskContext)
	if err != nil {
		return nil, fmt.Errorf("analysis step failed: %w", err)
	}

	analyzeTask := &Task{
		Type:     TaskTypeAnalysis,
		Model:    modelID,
		Messages: analyzeMessages,
	}

	analyzeResult, err := analysisAgent.Execute(ctx, analyzeTask, provider)
	if err != nil {
		return nil, fmt.Errorf("problem decomposition failed: %w", err)
	}

	c.intermediateResults["decomposition"] = analyzeResult

	// Step 2: Esegui ogni step
	solvePrompt := fmt.Sprintf(
		"Original problem:\n%s\n\nSteps to follow:\n%s\n\nNow solve the problem step by step.",
		initialInput,
		analyzeResult.Content,
	)

	generalAgent := c.orchestrator.agents[AgentTypeGeneral]
	solveMessages := append([]providers.Message{}, messages...)
	solveMessages = append(solveMessages, providers.Message{
		Role:    "user",
		Content: solvePrompt,
	})

	solveContext := c.orchestrator.analyzer.Analyze(solvePrompt, solveMessages)
	solveModelID, solveProvider, err := c.orchestrator.SelectModel(ctx, generalAgent, solveContext)
	if err != nil {
		return nil, fmt.Errorf("solve step failed: %w", err)
	}

	solveTask := &Task{
		Type:     TaskTypeGeneral,
		Model:    solveModelID,
		Messages: solveMessages,
	}

	solveResult, err := generalAgent.Execute(ctx, solveTask, solveProvider)
	if err != nil {
		return nil, fmt.Errorf("problem solving failed: %w", err)
	}

	c.intermediateResults["solution"] = solveResult

	return &ChainResult{
		FinalResult:         solveResult,
		IntermediateResults: c.intermediateResults,
		Metadata: map[string]interface{}{
			"chain_type": ChainTypeMultiStep,
			"decomposition_model": analyzeResult.Model,
			"solution_model": solveResult.Model,
		},
	}, nil
}

// executeConsensus esegue multiple agents e trova consenso
func (c *Chain) executeConsensus(ctx context.Context, initialInput string, messages []providers.Message) (*ChainResult, error) {
	if len(c.Steps) < 2 {
		return nil, fmt.Errorf("consensus chain requires at least 2 agents")
	}

	// Esegui tutti gli agenti in parallelo
	parallelResult, err := c.executeParallel(ctx, initialInput, messages)
	if err != nil {
		return nil, fmt.Errorf("consensus parallel execution failed: %w", err)
	}

	// Analizza i risultati per trovare consenso
	consensusPrompt := "Compare these different responses and provide the best combined answer:\n\n"
	for stepName, result := range parallelResult.IntermediateResults {
		consensusPrompt += fmt.Sprintf("Response from %s:\n%s\n\n", stepName, result.Content)
	}
	consensusPrompt += "Provide a synthesized response that combines the best aspects of all responses."

	// Usa analysis agent per il consenso
	analysisAgent := c.orchestrator.agents[AgentTypeAnalysis]
	consensusMessages := append([]providers.Message{}, messages...)
	consensusMessages = append(consensusMessages, providers.Message{
		Role:    "user",
		Content: consensusPrompt,
	})

	taskContext := c.orchestrator.analyzer.Analyze(consensusPrompt, consensusMessages)
	modelID, provider, err := c.orchestrator.SelectModel(ctx, analysisAgent, taskContext)
	if err != nil {
		return nil, fmt.Errorf("consensus step failed: %w", err)
	}

	consensusTask := &Task{
		Type:     TaskTypeAnalysis,
		Model:    modelID,
		Messages: consensusMessages,
	}

	consensusResult, err := analysisAgent.Execute(ctx, consensusTask, provider)
	if err != nil {
		return nil, fmt.Errorf("consensus synthesis failed: %w", err)
	}

	return &ChainResult{
		FinalResult:         consensusResult,
		IntermediateResults: parallelResult.IntermediateResults,
		Metadata: map[string]interface{}{
			"chain_type": ChainTypeConsensus,
			"agents_count": len(c.Steps),
			"consensus_model": consensusResult.Model,
		},
	}, nil
}

// preparePrompt prepara il prompt per uno step
func (c *Chain) preparePrompt(step *ChainStep, input string) string {
	if step.PromptTemplate != "" {
		// TODO: Implementare template rendering più sofisticato
		return fmt.Sprintf(step.PromptTemplate, input)
	}
	return input
}

// aggregateResults aggrega i risultati paralleli
func (c *Chain) aggregateResults(results map[string]*TaskResult) *TaskResult {
	// Semplice aggregazione: combina tutti i contenuti
	var combinedContent string
	var totalUsage providers.Usage

	for stepName, result := range results {
		combinedContent += fmt.Sprintf("## %s\n%s\n\n", stepName, result.Content)
		totalUsage.PromptTokens += result.Usage.PromptTokens
		totalUsage.CompletionTokens += result.Usage.CompletionTokens
		totalUsage.TotalTokens += result.Usage.TotalTokens
	}

	return &TaskResult{
		AgentType: AgentTypeGeneral,
		Content:   combinedContent,
		Model:     "aggregated",
		Usage:     totalUsage,
		Metadata: map[string]interface{}{
			"aggregated": true,
			"sources_count": len(results),
		},
	}
}

// ChainBuilder helper per costruire chains facilmente
type ChainBuilder struct {
	chain *Chain
}

// NewChainBuilder crea un nuovo ChainBuilder
func NewChainBuilder(chainType ChainType, orchestrator *Orchestrator) *ChainBuilder {
	return &ChainBuilder{
		chain: NewChain(chainType, orchestrator),
	}
}

// WithStep aggiunge uno step alla chain
func (cb *ChainBuilder) WithStep(name string, agent Agent, promptTemplate string) *ChainBuilder {
	step := &ChainStep{
		Name:           name,
		Agent:          agent,
		PromptTemplate: promptTemplate,
		Metadata:       make(map[string]interface{}),
	}
	cb.chain.AddStep(step)
	return cb
}

// Build costruisce la chain
func (cb *ChainBuilder) Build() *Chain {
	return cb.chain
}
