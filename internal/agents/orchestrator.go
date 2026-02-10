package agents

import (
	"context"
	"fmt"
	"sync"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
)

// Orchestrator gestisce la selezione e l'esecuzione degli agenti
type Orchestrator struct {
	config   *config.Config
	db       *database.DB
	mu       sync.RWMutex

	// Registry degli agenti disponibili
	agents map[AgentType]Agent

	// Context analyzer per determinare il tipo di task
	analyzer *ContextAnalyzer

	// Provider registry per gestire i providers disponibili
	providers map[string]providers.Provider

	// Fallback chains per ogni tipo di agente
	fallbackChains map[AgentType][]string

	// Model availability cache
	modelAvailability map[string]bool
}

// OrchestratorConfig configurazione per l'orchestrator
type OrchestratorConfig struct {
	// Abilita fallback automatico
	EnableFallback bool

	// Numero massimo di retry per fallback
	MaxFallbackRetries int

	// Abilita caching delle disponibilità dei modelli
	EnableModelCache bool

	// Timeout per health check dei modelli
	HealthCheckTimeout int
}

// NewOrchestrator crea un nuovo Orchestrator
func NewOrchestrator(cfg *config.Config, db *database.DB) *Orchestrator {
	o := &Orchestrator{
		config:            cfg,
		db:                db,
		agents:            make(map[AgentType]Agent),
		analyzer:          NewContextAnalyzer(),
		providers:         make(map[string]providers.Provider),
		fallbackChains:    make(map[AgentType][]string),
		modelAvailability: make(map[string]bool),
	}

	// Registra agenti specializzati
	o.registerAgents()

	// Configura fallback chains
	o.setupFallbackChains()

	return o
}

// registerAgents registra tutti gli agenti disponibili
func (o *Orchestrator) registerAgents() {
	o.agents[AgentTypeCoding] = NewCodingAgent()
	o.agents[AgentTypeCreative] = NewCreativeAgent()
	o.agents[AgentTypeAnalysis] = NewAnalysisAgent()
	o.agents[AgentTypeTranslation] = NewTranslationAgent()
	o.agents[AgentTypeFast] = NewFastAgent()
	o.agents[AgentTypeGeneral] = NewGeneralAgent()
}

// setupFallbackChains configura le catene di fallback per ogni tipo di agente
func (o *Orchestrator) setupFallbackChains() {
	// Coding Agent fallback chain
	o.fallbackChains[AgentTypeCoding] = []string{
		"codestral-latest",
		"deepseek-coder",
		"deepseek-chat",
		"claude-3-5-sonnet-20241022",
		"gpt-4o",
		"gemini-1.5-pro",
		"llama-3.1-70b",
	}

	// Creative Agent fallback chain
	o.fallbackChains[AgentTypeCreative] = []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
		"gpt-4o",
		"gemini-1.5-pro",
		"llama-3.1-70b",
		"gpt-4-turbo",
	}

	// Analysis Agent fallback chain
	o.fallbackChains[AgentTypeAnalysis] = []string{
		"claude-3-5-sonnet-20241022",
		"gpt-4o",
		"gemini-1.5-pro",
		"claude-3-opus-20240229",
		"deepseek-chat",
		"llama-3.1-70b",
	}

	// Translation Agent fallback chain
	o.fallbackChains[AgentTypeTranslation] = []string{
		"gpt-4o",
		"claude-3-5-sonnet-20241022",
		"gemini-1.5-pro",
		"gpt-4-turbo",
		"llama-3.1-70b-multilingual",
	}

	// Fast Agent fallback chain
	o.fallbackChains[AgentTypeFast] = []string{
		"llama-3.1-70b-versatile", // Groq
		"llama-3.3-70b",           // Cerebras
		"llama-3.1-8b-instant",    // Groq
		"gemini-1.5-flash",
		"gpt-3.5-turbo",
		"claude-3-haiku-20240307",
	}

	// General Agent fallback chain
	o.fallbackChains[AgentTypeGeneral] = []string{
		"gpt-4o",
		"claude-3-5-sonnet-20241022",
		"gemini-1.5-pro",
		"llama-3.1-70b",
		"gpt-4-turbo",
		"deepseek-chat",
	}
}

// RegisterProvider registra un provider nell'orchestrator
func (o *Orchestrator) RegisterProvider(provider providers.Provider) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.providers[provider.Name()] = provider
}

// SelectAgent seleziona l'agente più appropriato per il task
func (o *Orchestrator) SelectAgent(ctx context.Context, prompt string, messages []providers.Message) (Agent, *TaskContext, error) {
	// Analizza il contesto
	taskContext := o.analyzer.Analyze(prompt, messages)

	// Seleziona l'agente basato sul task type
	agent, exists := o.agents[AgentType(taskContext.TaskType)]
	if !exists {
		// Fallback su general agent
		agent = o.agents[AgentTypeGeneral]
	}

	return agent, taskContext, nil
}

// SelectModel seleziona il miglior modello per un agente specifico
func (o *Orchestrator) SelectModel(ctx context.Context, agent Agent, taskContext *TaskContext) (string, providers.Provider, error) {
	preferredModels := agent.PreferredModels()

	// Se il task richiede alta qualità, usa i primi modelli della lista
	if taskContext.RequiresHighQuality {
		preferredModels = preferredModels[:min(3, len(preferredModels))]
	}

	// Prova i modelli preferiti in ordine
	for _, modelID := range preferredModels {
		// Verifica disponibilità nel cache (se abilitato)
		if available, cached := o.modelAvailability[modelID]; cached && !available {
			continue
		}

		// Trova il provider che supporta questo modello
		provider, err := o.findProviderForModel(ctx, modelID)
		if err == nil {
			// Aggiorna cache
			o.modelAvailability[modelID] = true
			return modelID, provider, nil
		}

		// Segna come non disponibile nel cache
		o.modelAvailability[modelID] = false
	}

	// Se nessun modello preferito è disponibile, usa fallback chain
	fallbackChain := o.fallbackChains[agent.Type()]
	for _, modelID := range fallbackChain {
		provider, err := o.findProviderForModel(ctx, modelID)
		if err == nil {
			return modelID, provider, nil
		}
	}

	return "", nil, fmt.Errorf("no available model found for agent type: %s", agent.Type())
}

// findProviderForModel trova il provider che supporta un determinato modello
func (o *Orchestrator) findProviderForModel(ctx context.Context, modelID string) (providers.Provider, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Prova ogni provider registrato
	for _, provider := range o.providers {
		// Health check
		if err := provider.HealthCheck(ctx); err != nil {
			continue
		}

		// Verifica se il provider supporta il modello
		models, err := provider.GetModels(ctx)
		if err != nil {
			continue
		}

		for _, model := range models {
			if model.ID == modelID || model.Name == modelID {
				return provider, nil
			}
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", modelID)
}

// Execute esegue un task utilizzando l'agente e il modello più appropriati
func (o *Orchestrator) Execute(ctx context.Context, prompt string, messages []providers.Message) (*TaskResult, error) {
	// Seleziona agente
	agent, taskContext, err := o.SelectAgent(ctx, prompt, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to select agent: %w", err)
	}

	// Seleziona modello e provider
	modelID, provider, err := o.SelectModel(ctx, agent, taskContext)
	if err != nil {
		return nil, fmt.Errorf("failed to select model: %w", err)
	}

	// Crea task
	task := &Task{
		Type:     TaskType(agent.Type()),
		Model:    modelID,
		Messages: messages,
		Metadata: map[string]interface{}{
			"task_context": taskContext,
			"agent_type":   agent.Type(),
			"confidence":   taskContext.Confidence,
		},
	}

	// Esegui task
	result, err := agent.Execute(ctx, task, provider)
	if err != nil {
		// Prova con fallback se abilitato
		if o.config.Routing.FailoverEnabled {
			return o.executeWithFallback(ctx, agent, task, taskContext)
		}
		return nil, fmt.Errorf("task execution failed: %w", err)
	}

	return result, nil
}

// executeWithFallback esegue il task con fallback automatico
func (o *Orchestrator) executeWithFallback(ctx context.Context, agent Agent, task *Task, taskContext *TaskContext) (*TaskResult, error) {
	fallbackChain := o.fallbackChains[agent.Type()]
	maxRetries := o.config.Routing.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	retriesCount := 0

	for _, modelID := range fallbackChain {
		if retriesCount >= maxRetries {
			break
		}

		// Trova provider per questo modello
		provider, err := o.findProviderForModel(ctx, modelID)
		if err != nil {
			lastErr = err
			retriesCount++
			continue
		}

		// Aggiorna task con nuovo modello
		task.Model = modelID

		// Prova esecuzione
		result, err := agent.Execute(ctx, task, provider)
		if err == nil {
			// Successo! Aggiungi info sul fallback
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["fallback_used"] = true
			result.Metadata["retries"] = retriesCount
			result.Metadata["original_model_failed"] = task.Model
			return result, nil
		}

		lastErr = err
		retriesCount++
	}

	return nil, fmt.Errorf("all fallback attempts failed after %d retries: %w", retriesCount, lastErr)
}

// GetAgentInfo restituisce informazioni su tutti gli agenti registrati
func (o *Orchestrator) GetAgentInfo() map[AgentType]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	info := make(map[AgentType]interface{})

	for agentType, agent := range o.agents {
		info[agentType] = map[string]interface{}{
			"name":             agent.Name(),
			"type":             agent.Type(),
			"preferred_models": agent.PreferredModels(),
			"fallback_chain":   o.fallbackChains[agentType],
		}
	}

	return info
}

// GetModelAvailability restituisce lo stato di disponibilità dei modelli
func (o *Orchestrator) GetModelAvailability() map[string]bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	availability := make(map[string]bool)
	for model, available := range o.modelAvailability {
		availability[model] = available
	}

	return availability
}

// ClearModelCache pulisce il cache di disponibilità dei modelli
func (o *Orchestrator) ClearModelCache() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.modelAvailability = make(map[string]bool)
}

// RouteRequest è un helper che gestisce routing completo di una richiesta
func (o *Orchestrator) RouteRequest(ctx context.Context, req *providers.ChatRequest) (*TaskResult, error) {
	// Estrae prompt dal primo messaggio user
	var prompt string
	for _, msg := range req.Messages {
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

	return o.Execute(ctx, prompt, req.Messages)
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
