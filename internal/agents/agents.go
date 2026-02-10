package agents

import (
	"context"
	"fmt"

	"github.com/biodoia/goleapifree/internal/providers"
)

// AgentType rappresenta il tipo di agente specializzato
type AgentType string

const (
	AgentTypeCoding      AgentType = "coding"
	AgentTypeCreative    AgentType = "creative"
	AgentTypeAnalysis    AgentType = "analysis"
	AgentTypeTranslation AgentType = "translation"
	AgentTypeFast        AgentType = "fast"
	AgentTypeGeneral     AgentType = "general"
)

// Agent rappresenta un agente specializzato con preferenze di modelli
type Agent interface {
	// Type restituisce il tipo di agente
	Type() AgentType

	// Name restituisce il nome dell'agente
	Name() string

	// PreferredModels restituisce la lista ordinata di modelli preferiti
	PreferredModels() []string

	// CanHandle verifica se l'agente può gestire un determinato contesto
	CanHandle(ctx *TaskContext) bool

	// Execute esegue il task utilizzando il provider specificato
	Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error)
}

// BaseAgent fornisce funzionalità comuni per tutti gli agenti
type BaseAgent struct {
	agentType       AgentType
	name            string
	preferredModels []string
	capabilities    map[string]bool
}

// Type restituisce il tipo di agente
func (a *BaseAgent) Type() AgentType {
	return a.agentType
}

// Name restituisce il nome dell'agente
func (a *BaseAgent) Name() string {
	return a.name
}

// PreferredModels restituisce la lista di modelli preferiti
func (a *BaseAgent) PreferredModels() []string {
	return a.preferredModels
}

// CodingAgent - Specializzato in coding tasks
type CodingAgent struct {
	BaseAgent
}

// NewCodingAgent crea un nuovo CodingAgent
func NewCodingAgent() *CodingAgent {
	return &CodingAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeCoding,
			name:      "Coding Specialist",
			preferredModels: []string{
				"codestral-latest",
				"codestral-2405",
				"deepseek-coder",
				"deepseek-chat",
				"claude-3-5-sonnet-20241022",
				"claude-3-opus-20240229",
				"gpt-4-turbo",
				"gpt-4o",
				"gemini-1.5-pro",
			},
			capabilities: map[string]bool{
				"code_generation":   true,
				"code_review":       true,
				"debugging":         true,
				"refactoring":       true,
				"documentation":     true,
				"test_generation":   true,
			},
		},
	}
}

// CanHandle verifica se può gestire il task
func (a *CodingAgent) CanHandle(ctx *TaskContext) bool {
	return ctx.TaskType == TaskTypeCoding ||
		   ctx.HasKeywords([]string{"code", "function", "class", "debug", "refactor", "implement", "programming"})
}

// Execute esegue il task di coding
func (a *CodingAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	// Prepara il prompt ottimizzato per coding
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: float64Ptr(0.2), // Bassa temperatura per codice più deterministico
		MaxTokens:   intPtr(4096),
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("coding task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// CreativeAgent - Specializzato in creative tasks
type CreativeAgent struct {
	BaseAgent
}

// NewCreativeAgent crea un nuovo CreativeAgent
func NewCreativeAgent() *CreativeAgent {
	return &CreativeAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeCreative,
			name:      "Creative Specialist",
			preferredModels: []string{
				"claude-3-5-sonnet-20241022",
				"claude-3-opus-20240229",
				"gpt-4o",
				"gpt-4-turbo",
				"gemini-1.5-pro",
				"gemini-1.5-flash",
				"llama-3.1-70b",
			},
			capabilities: map[string]bool{
				"creative_writing": true,
				"storytelling":     true,
				"brainstorming":    true,
				"content_creation": true,
				"marketing":        true,
			},
		},
	}
}

// CanHandle verifica se può gestire il task
func (a *CreativeAgent) CanHandle(ctx *TaskContext) bool {
	return ctx.TaskType == TaskTypeCreative ||
		   ctx.HasKeywords([]string{"write", "story", "creative", "brainstorm", "imagine", "content", "marketing"})
}

// Execute esegue il task creativo
func (a *CreativeAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: float64Ptr(0.8), // Alta temperatura per più creatività
		MaxTokens:   intPtr(4096),
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creative task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// AnalysisAgent - Specializzato in analysis tasks
type AnalysisAgent struct {
	BaseAgent
}

// NewAnalysisAgent crea un nuovo AnalysisAgent
func NewAnalysisAgent() *AnalysisAgent {
	return &AnalysisAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeAnalysis,
			name:      "Analysis Specialist",
			preferredModels: []string{
				"claude-3-5-sonnet-20241022",
				"claude-3-opus-20240229",
				"gpt-4o",
				"gpt-4-turbo",
				"gemini-1.5-pro",
				"deepseek-chat",
			},
			capabilities: map[string]bool{
				"data_analysis":    true,
				"reasoning":        true,
				"problem_solving":  true,
				"research":         true,
				"summarization":    true,
			},
		},
	}
}

// CanHandle verifica se può gestire il task
func (a *AnalysisAgent) CanHandle(ctx *TaskContext) bool {
	return ctx.TaskType == TaskTypeAnalysis ||
		   ctx.HasKeywords([]string{"analyze", "analysis", "research", "summarize", "reason", "explain", "understand"})
}

// Execute esegue il task di analisi
func (a *AnalysisAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: float64Ptr(0.3), // Temperatura moderata per analisi accurata
		MaxTokens:   intPtr(4096),
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("analysis task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// TranslationAgent - Specializzato in translation tasks
type TranslationAgent struct {
	BaseAgent
}

// NewTranslationAgent crea un nuovo TranslationAgent
func NewTranslationAgent() *TranslationAgent {
	return &TranslationAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeTranslation,
			name:      "Translation Specialist",
			preferredModels: []string{
				"gpt-4o",
				"gpt-4-turbo",
				"claude-3-5-sonnet-20241022",
				"gemini-1.5-pro",
				"llama-3.1-70b-multilingual",
				"aya-expanse-32b",
			},
			capabilities: map[string]bool{
				"translation":      true,
				"localization":     true,
				"multilingual":     true,
				"cultural_context": true,
			},
		},
	}
}

// CanHandle verifica se può gestire il task
func (a *TranslationAgent) CanHandle(ctx *TaskContext) bool {
	return ctx.TaskType == TaskTypeTranslation ||
		   ctx.HasKeywords([]string{"translate", "translation", "language", "localize", "multilingual"})
}

// Execute esegue il task di traduzione
func (a *TranslationAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: float64Ptr(0.3), // Temperatura bassa per traduzioni accurate
		MaxTokens:   intPtr(4096),
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("translation task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// FastAgent - Specializzato in fast response tasks
type FastAgent struct {
	BaseAgent
}

// NewFastAgent crea un nuovo FastAgent
func NewFastAgent() *FastAgent {
	return &FastAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeFast,
			name:      "Fast Response Specialist",
			preferredModels: []string{
				// Groq models (ultra-fast)
				"llama-3.1-70b-versatile",
				"llama-3.1-8b-instant",
				"mixtral-8x7b-32768",
				// Cerebras models (ultra-fast)
				"llama-3.3-70b",
				"llama-3.1-8b",
				// Other fast models
				"gemini-1.5-flash",
				"gpt-3.5-turbo",
				"claude-3-haiku-20240307",
			},
			capabilities: map[string]bool{
				"quick_response":   true,
				"simple_tasks":     true,
				"low_latency":      true,
				"high_throughput":  true,
			},
		},
	}
}

// CanHandle verifica se può gestire il task
func (a *FastAgent) CanHandle(ctx *TaskContext) bool {
	return ctx.TaskType == TaskTypeFast ||
		   ctx.RequiresFastResponse ||
		   ctx.HasKeywords([]string{"quick", "fast", "simple", "brief"})
}

// Execute esegue il task veloce
func (a *FastAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: float64Ptr(0.5), // Temperatura moderata
		MaxTokens:   intPtr(2048),    // Limita i token per velocità
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fast task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// GeneralAgent - Agente generico per task non specializzati
type GeneralAgent struct {
	BaseAgent
}

// NewGeneralAgent crea un nuovo GeneralAgent
func NewGeneralAgent() *GeneralAgent {
	return &GeneralAgent{
		BaseAgent: BaseAgent{
			agentType: AgentTypeGeneral,
			name:      "General Purpose Agent",
			preferredModels: []string{
				"gpt-4o",
				"claude-3-5-sonnet-20241022",
				"gemini-1.5-pro",
				"llama-3.1-70b",
				"gpt-4-turbo",
			},
			capabilities: map[string]bool{
				"general_purpose": true,
			},
		},
	}
}

// CanHandle può gestire qualsiasi task
func (a *GeneralAgent) CanHandle(ctx *TaskContext) bool {
	return true // Fallback per tutti i task
}

// Execute esegue il task generale
func (a *GeneralAgent) Execute(ctx context.Context, task *Task, provider providers.Provider) (*TaskResult, error) {
	req := &providers.ChatRequest{
		Model:       task.Model,
		Messages:    task.Messages,
		Temperature: task.Temperature,
		MaxTokens:   task.MaxTokens,
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("general task failed: %w", err)
	}

	return &TaskResult{
		AgentType: a.Type(),
		Content:   resp.Choices[0].Message.Content.(string),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Metadata: map[string]interface{}{
			"agent": a.Name(),
			"task_type": task.Type,
		},
	}, nil
}

// Helper functions
func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
