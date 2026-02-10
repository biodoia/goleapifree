package agents

import (
	"context"
	"fmt"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/internal/router"
)

// AgentRoutingStrategy implementa RoutingStrategy usando l'orchestrator multi-agent
type AgentRoutingStrategy struct {
	orchestrator *Orchestrator
}

// NewAgentRoutingStrategy crea una nuova AgentRoutingStrategy
func NewAgentRoutingStrategy(orchestrator *Orchestrator) *AgentRoutingStrategy {
	return &AgentRoutingStrategy{
		orchestrator: orchestrator,
	}
}

// SelectProvider implementa l'interfaccia RoutingStrategy
func (s *AgentRoutingStrategy) SelectProvider(req *router.Request) (*router.ProviderSelection, error) {
	// Converti Request in formato providers
	messages := make([]providers.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Estrai prompt dal primo messaggio user
	var prompt string
	for _, msg := range messages {
		if msg.Role == "user" {
			if content, ok := msg.Content.(string); ok {
				prompt = content
				break
			}
		}
	}

	if prompt == "" {
		return nil, fmt.Errorf("no user message found in request")
	}

	ctx := context.Background()

	// Seleziona agente appropriato
	agent, taskContext, err := s.orchestrator.SelectAgent(ctx, prompt, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to select agent: %w", err)
	}

	// Seleziona modello e provider
	modelID, provider, err := s.orchestrator.SelectModel(ctx, agent, taskContext)
	if err != nil {
		return nil, fmt.Errorf("failed to select model: %w", err)
	}

	// Calcola costo stimato (placeholder - da implementare con prezzi reali)
	estimatedCost := s.estimateCost(modelID, len(prompt))

	// Costruisci reason dettagliato
	reason := fmt.Sprintf(
		"agent_routing: agent=%s, task_type=%s, confidence=%.2f, model=%s",
		agent.Name(),
		taskContext.TaskType,
		taskContext.Confidence,
		modelID,
	)

	return &router.ProviderSelection{
		ProviderID:    provider.Name(),
		ModelID:       modelID,
		EstimatedCost: estimatedCost,
		Reason:        reason,
	}, nil
}

// estimateCost stima il costo di una richiesta
func (s *AgentRoutingStrategy) estimateCost(modelID string, promptLength int) float64 {
	// Costi approssimativi per 1K tokens (USD)
	costs := map[string]float64{
		// OpenAI
		"gpt-4o":                  0.0025,
		"gpt-4-turbo":             0.01,
		"gpt-3.5-turbo":           0.0005,

		// Anthropic
		"claude-3-5-sonnet-20241022": 0.003,
		"claude-3-opus-20240229":     0.015,
		"claude-3-haiku-20240307":    0.00025,

		// Codestral
		"codestral-latest": 0.001,
		"codestral-2405":   0.001,

		// DeepSeek
		"deepseek-coder": 0.0002,
		"deepseek-chat":  0.0002,

		// Gemini
		"gemini-1.5-pro":   0.00125,
		"gemini-1.5-flash": 0.000075,

		// Groq (free tier)
		"llama-3.1-70b-versatile": 0.0,
		"llama-3.1-8b-instant":    0.0,
		"mixtral-8x7b-32768":      0.0,

		// Cerebras (free tier)
		"llama-3.3-70b": 0.0,
		"llama-3.1-8b":  0.0,
	}

	// Stima tokens (circa 4 caratteri per token)
	estimatedTokens := float64(promptLength) / 4.0

	// Trova costo per questo modello
	costPer1K, exists := costs[modelID]
	if !exists {
		costPer1K = 0.002 // Default
	}

	// Calcola costo totale (input + output stimato)
	// Assumiamo output = 2x input
	totalTokens := estimatedTokens * 3.0
	return (totalTokens / 1000.0) * costPer1K
}

// IntegrateWithRouter integra il sistema di orchestration con il router
func IntegrateWithRouter(r *router.Router, orchestrator *Orchestrator) error {
	// Crea e registra la strategy basata su agenti
	strategy := NewAgentRoutingStrategy(orchestrator)

	// TODO: Implementare metodo per sostituire strategy nel router
	// r.SetStrategy(strategy)

	_ = strategy // Evita unused variable error per ora

	return nil
}

// AdvancedRoutingOptions opzioni avanzate per il routing
type AdvancedRoutingOptions struct {
	// Forza un tipo di agente specifico
	ForceAgentType *AgentType

	// Forza un modello specifico
	ForceModel *string

	// Abilita chain multi-step
	EnableChain bool

	// Tipo di chain da usare
	ChainType ChainType

	// Richiede alta qualità
	RequireHighQuality bool

	// Richiede risposta veloce
	RequireFastResponse bool

	// Budget massimo in USD
	MaxBudget *float64
}

// RouteWithOptions esegue routing avanzato con opzioni
func RouteWithOptions(
	ctx context.Context,
	orchestrator *Orchestrator,
	req *providers.ChatRequest,
	opts *AdvancedRoutingOptions,
) (*TaskResult, error) {
	// Se chain è abilitato, usa chain execution
	if opts.EnableChain {
		return executeWithChain(ctx, orchestrator, req, opts)
	}

	// Estrai prompt
	var prompt string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if content, ok := msg.Content.(string); ok {
				prompt = content
				break
			}
		}
	}

	// Se forza un tipo di agente specifico
	if opts.ForceAgentType != nil {
		agent := orchestrator.agents[*opts.ForceAgentType]
		if agent == nil {
			return nil, fmt.Errorf("agent type not found: %s", *opts.ForceAgentType)
		}

		taskContext := orchestrator.analyzer.Analyze(prompt, req.Messages)

		// Applica override dalle opzioni
		if opts.RequireHighQuality {
			taskContext.RequiresHighQuality = true
		}
		if opts.RequireFastResponse {
			taskContext.RequiresFastResponse = true
		}

		modelID := *opts.ForceModel
		if opts.ForceModel == nil {
			// Seleziona automaticamente il modello
			var provider providers.Provider
			var err error
			modelID, provider, err = orchestrator.SelectModel(ctx, agent, taskContext)
			if err != nil {
				return nil, err
			}
			_ = provider
		}

		// Trova provider per il modello
		provider, err := orchestrator.findProviderForModel(ctx, modelID)
		if err != nil {
			return nil, err
		}

		task := &Task{
			Type:        TaskType(*opts.ForceAgentType),
			Model:       modelID,
			Messages:    req.Messages,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
		}

		return agent.Execute(ctx, task, provider)
	}

	// Routing automatico standard
	return orchestrator.RouteRequest(ctx, req)
}

// executeWithChain esegue usando chain
func executeWithChain(
	ctx context.Context,
	orchestrator *Orchestrator,
	req *providers.ChatRequest,
	opts *AdvancedRoutingOptions,
) (*TaskResult, error) {
	var prompt string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if content, ok := msg.Content.(string); ok {
				prompt = content
				break
			}
		}
	}

	// Crea chain basata sul tipo richiesto
	var chain *Chain

	switch opts.ChainType {
	case ChainTypeDraftRefine:
		chain = NewChainBuilder(ChainTypeDraftRefine, orchestrator).
			WithStep("draft", orchestrator.agents[AgentTypeFast], "%s").
			WithStep("refine", orchestrator.agents[AgentTypeGeneral], "%s").
			Build()

	case ChainTypeMultiStep:
		chain = NewChain(ChainTypeMultiStep, orchestrator)

	case ChainTypeConsensus:
		chain = NewChainBuilder(ChainTypeConsensus, orchestrator).
			WithStep("agent1", orchestrator.agents[AgentTypeGeneral], "%s").
			WithStep("agent2", orchestrator.agents[AgentTypeAnalysis], "%s").
			WithStep("agent3", orchestrator.agents[AgentTypeCreative], "%s").
			Build()

	default:
		chain = NewChainBuilder(ChainTypeSequential, orchestrator).
			WithStep("main", orchestrator.agents[AgentTypeGeneral], "%s").
			Build()
	}

	result, err := chain.Execute(ctx, prompt, req.Messages)
	if err != nil {
		return nil, err
	}

	return result.FinalResult, nil
}
