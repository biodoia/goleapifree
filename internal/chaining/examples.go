package chaining

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
)

// ExampleDraftRefineTransformer implementa un transformer per draft-refine
type ExampleDraftRefineTransformer struct {
	phase string // "draft" o "refine"
}

func (t *ExampleDraftRefineTransformer) TransformInput(ctx context.Context, input *StageInput) (*providers.ChatRequest, error) {
	req := &providers.ChatRequest{
		Model:    input.OriginalRequest.Model,
		Messages: make([]providers.Message, 0),
	}

	if t.phase == "draft" {
		// Per il draft, passa la richiesta originale con un prompt di velocità
		req.Messages = append(req.Messages, providers.Message{
			Role:    "system",
			Content: "You are a helpful assistant. Provide a quick, concise response focusing on key points.",
		})
		req.Messages = append(req.Messages, input.OriginalRequest.Messages...)

		// Parametri per velocità
		temp := 0.7
		maxTokens := 500
		req.Temperature = &temp
		req.MaxTokens = &maxTokens

	} else if t.phase == "refine" {
		// Per il refine, usa il draft come base
		if len(input.PreviousOutputs) == 0 {
			return nil, fmt.Errorf("refine phase requires previous draft output")
		}

		draftOutput := input.PreviousOutputs[0]
		draftContent := fmt.Sprintf("%v", draftOutput.Response.Choices[0].Message.Content)

		// Costruisci prompt di refinement
		req.Messages = append(req.Messages, providers.Message{
			Role: "system",
			Content: "You are a helpful assistant. Your task is to refine and improve the draft response, " +
				"making it more accurate, detailed, well-structured, and comprehensive.",
		})

		// Aggiungi richiesta originale e draft
		req.Messages = append(req.Messages, input.OriginalRequest.Messages...)
		req.Messages = append(req.Messages, providers.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Here is my draft response:\n\n%s", draftContent),
		})
		req.Messages = append(req.Messages, providers.Message{
			Role: "user",
			Content: "Please refine and improve this draft response. Make it more detailed, accurate, " +
				"and well-structured. Add examples where appropriate.",
		})

		// Parametri per qualità
		temp := 0.3
		maxTokens := 2000
		req.Temperature = &temp
		req.MaxTokens = &maxTokens
	}

	return req, nil
}

func (t *ExampleDraftRefineTransformer) TransformOutput(ctx context.Context, output *providers.ChatResponse) (*StageOutput, error) {
	return &StageOutput{
		Response: output,
		Metadata: map[string]interface{}{
			"phase": t.phase,
		},
	}, nil
}

// CreateExampleDraftRefinePipeline crea una pipeline di esempio draft-refine
// Draft: Groq Llama-8B (ultra-fast)
// Refine: Claude Sonnet (high quality)
func CreateExampleDraftRefinePipeline(groqProvider, claudeProvider providers.Provider) *Pipeline {
	pipeline := NewPipeline(NewDraftRefineStrategy("", ""))

	// Stage 1: Draft veloce con Groq
	pipeline.AddStage(Stage{
		Name:        "draft-groq-llama",
		Provider:    groqProvider,
		Model:       "llama-3.1-8b-instant",
		Transformer: &ExampleDraftRefineTransformer{phase: "draft"},
		Timeout:     5 * time.Second,
		MaxRetries:  2,
	})

	// Stage 2: Refine di qualità con Claude
	pipeline.AddStage(Stage{
		Name:        "refine-claude-sonnet",
		Provider:    claudeProvider,
		Model:       "claude-3-5-sonnet-20241022",
		Transformer: &ExampleDraftRefineTransformer{phase: "refine"},
		Timeout:     30 * time.Second,
		MaxRetries:  2,
	})

	return pipeline
}

// CreateExampleCascadePipeline crea una pipeline cascade
// Prova veloce prima, fallback a potente
func CreateExampleCascadePipeline(fastProvider, powerfulProvider providers.Provider) *Pipeline {
	pipeline := NewPipeline(NewCascadeStrategy(2*time.Second, true, 50))

	// Stage 1: Modello veloce (Groq)
	pipeline.AddStage(Stage{
		Name:        "fast-groq",
		Provider:    fastProvider,
		Model:       "llama-3.1-8b-instant",
		Transformer: &DefaultTransformer{},
		Timeout:     2 * time.Second,
		MaxRetries:  1,
		Optional:    false,
	})

	// Stage 2: Modello potente (fallback)
	pipeline.AddStage(Stage{
		Name:        "powerful-claude",
		Provider:    powerfulProvider,
		Model:       "claude-3-5-sonnet-20241022",
		Transformer: &DefaultTransformer{},
		Timeout:     30 * time.Second,
		MaxRetries:  2,
		Optional:    false,
	})

	return pipeline
}

// CreateExampleParallelConsensusPipeline crea una pipeline con consensus parallelo
func CreateExampleParallelConsensusPipeline(providers []providers.Provider, models []string) *Pipeline {
	if len(providers) != len(models) {
		panic("providers and models must have same length")
	}

	pipeline := NewPipeline(NewParallelConsensusStrategy("majority"))

	for i := 0; i < len(providers); i++ {
		pipeline.AddStage(Stage{
			Name:        fmt.Sprintf("parallel-%d", i),
			Provider:    providers[i],
			Model:       models[i],
			Transformer: &DefaultTransformer{},
			Parallel:    true,
			Timeout:     15 * time.Second,
			MaxRetries:  1,
		})
	}

	return pipeline
}

// LoRATaskTransformer trasforma input basandosi sul task e adapter LoRA
type LoRATaskTransformer struct {
	manager *LoRAManager
	task    string
}

func NewLoRATaskTransformer(manager *LoRAManager, task string) *LoRATaskTransformer {
	return &LoRATaskTransformer{
		manager: manager,
		task:    task,
	}
}

func (t *LoRATaskTransformer) TransformInput(ctx context.Context, input *StageInput) (*providers.ChatRequest, error) {
	req := &providers.ChatRequest{
		Model:    input.OriginalRequest.Model,
		Messages: input.OriginalRequest.Messages,
	}

	// Prova a caricare adapter per questo task
	adapters := t.manager.GetAdaptersByTask(t.task)
	if len(adapters) > 0 {
		// Usa il primo adapter disponibile
		loaded, err := t.manager.LoadAdapter(ctx, adapters[0].ID)
		if err == nil {
			// Aggiungi metadati dell'adapter
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["lora_adapter"] = loaded.Adapter.ID
			req.Metadata["lora_task"] = t.task
		}
	}

	return req, nil
}

func (t *LoRATaskTransformer) TransformOutput(ctx context.Context, output *providers.ChatResponse) (*StageOutput, error) {
	return &StageOutput{
		Response: output,
		Metadata: map[string]interface{}{
			"task": t.task,
		},
	}, nil
}

// CreateExampleLoRAPipeline crea una pipeline con supporto LoRA
func CreateExampleLoRAPipeline(provider providers.Provider, loraManager *LoRAManager) *Pipeline {
	pipeline := NewPipeline(NewSequentialStrategy())

	// Stage con LoRA adapter per code generation
	pipeline.AddStage(Stage{
		Name:        "code-with-lora",
		Provider:    provider,
		Model:       "llama-3.1-70b",
		Transformer: NewLoRATaskTransformer(loraManager, "code"),
		Timeout:     20 * time.Second,
		MaxRetries:  2,
	})

	return pipeline
}

// CreateOptimizedPipeline crea una pipeline ottimizzata basata su constraints
func CreateOptimizedPipeline(
	ctx context.Context,
	optimizer *Optimizer,
	providers map[string]providers.Provider,
	req *providers.ChatRequest,
	constraints PipelineConstraints,
) (*Pipeline, error) {
	// Ottieni raccomandazione
	recommendation, err := optimizer.RecommendPipeline(ctx, req, constraints)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation: %w", err)
	}

	// Crea pipeline basata sulla strategia raccomandata
	var strategy Strategy
	switch recommendation.Strategy {
	case "draft-refine":
		strategy = NewDraftRefineStrategy("", "")
	case "cascade":
		strategy = NewCascadeStrategy(2*time.Second, true, 50)
	case "parallel-consensus":
		strategy = NewParallelConsensusStrategy("majority")
	default:
		strategy = NewSequentialStrategy()
	}

	pipeline := NewPipeline(strategy)

	// Aggiungi stage basati sull'obiettivo
	switch constraints.Objective {
	case "cost":
		// Solo provider economici
		if p, ok := providers["groq"]; ok {
			pipeline.AddStage(Stage{
				Name:        "groq-economical",
				Provider:    p,
				Model:       "llama-3.1-8b-instant",
				Transformer: &DefaultTransformer{},
				Timeout:     5 * time.Second,
				MaxRetries:  2,
			})
		}

	case "latency":
		// Solo provider veloci
		if p, ok := providers["groq"]; ok {
			pipeline.AddStage(Stage{
				Name:        "groq-fast",
				Provider:    p,
				Model:       "llama-3.1-8b-instant",
				Transformer: &DefaultTransformer{},
				Timeout:     2 * time.Second,
				MaxRetries:  1,
			})
		}

	case "quality":
		// Draft + Refine per massima qualità
		if groq, ok := providers["groq"]; ok {
			pipeline.AddStage(Stage{
				Name:        "draft-fast",
				Provider:    groq,
				Model:       "llama-3.1-8b-instant",
				Transformer: &ExampleDraftRefineTransformer{phase: "draft"},
				Timeout:     5 * time.Second,
				MaxRetries:  2,
			})
		}
		if claude, ok := providers["anthropic"]; ok {
			pipeline.AddStage(Stage{
				Name:        "refine-quality",
				Provider:    claude,
				Model:       "claude-3-5-sonnet-20241022",
				Transformer: &ExampleDraftRefineTransformer{phase: "refine"},
				Timeout:     30 * time.Second,
				MaxRetries:  2,
			})
		}

	default:
		// Balanced: cascade
		if groq, ok := providers["groq"]; ok {
			pipeline.AddStage(Stage{
				Name:        "try-fast-first",
				Provider:    groq,
				Model:       "llama-3.1-8b-instant",
				Transformer: &DefaultTransformer{},
				Timeout:     3 * time.Second,
				MaxRetries:  1,
			})
		}
		if claude, ok := providers["anthropic"]; ok {
			pipeline.AddStage(Stage{
				Name:        "fallback-quality",
				Provider:    claude,
				Model:       "claude-3-5-haiku-20241022",
				Transformer: &DefaultTransformer{},
				Timeout:     15 * time.Second,
				MaxRetries:  2,
			})
		}
	}

	return pipeline, nil
}

// InitializeLoRAAdapters inizializza alcuni adapter LoRA di esempio
func InitializeLoRAAdapters(manager *LoRAManager) error {
	// Adapter per code generation
	codeAdapter := &LoRAAdapter{
		ID:          "lora-code-llama-70b",
		Name:        "Code Generation Specialist",
		Description: "LoRA adapter optimized for code generation and completion",
		BaseModel:   "llama-3.1-70b",
		Task:        "code",
		Path:        "/models/lora/code-llama-70b.safetensors",
		SizeBytes:   150 * 1024 * 1024, // 150MB
		Metadata: map[string]interface{}{
			"languages": []string{"python", "javascript", "go", "rust"},
			"version":   "1.0",
		},
	}

	// Adapter per math
	mathAdapter := &LoRAAdapter{
		ID:          "lora-math-llama-70b",
		Name:        "Math Problem Solver",
		Description: "LoRA adapter optimized for mathematical reasoning",
		BaseModel:   "llama-3.1-70b",
		Task:        "math",
		Path:        "/models/lora/math-llama-70b.safetensors",
		SizeBytes:   120 * 1024 * 1024, // 120MB
		Metadata: map[string]interface{}{
			"subjects": []string{"algebra", "calculus", "statistics"},
			"version":  "1.0",
		},
	}

	// Adapter per translation
	translationAdapter := &LoRAAdapter{
		ID:          "lora-translate-llama-8b",
		Name:        "Language Translation",
		Description: "LoRA adapter for multilingual translation",
		BaseModel:   "llama-3.1-8b-instant",
		Task:        "translation",
		Path:        "/models/lora/translate-llama-8b.safetensors",
		SizeBytes:   80 * 1024 * 1024, // 80MB
		Metadata: map[string]interface{}{
			"languages": []string{"en", "es", "fr", "de", "it", "pt"},
			"version":   "1.0",
		},
	}

	// Registra adapter
	if err := manager.RegisterAdapter(codeAdapter); err != nil {
		return err
	}
	if err := manager.RegisterAdapter(mathAdapter); err != nil {
		return err
	}
	if err := manager.RegisterAdapter(translationAdapter); err != nil {
		return err
	}

	return nil
}
