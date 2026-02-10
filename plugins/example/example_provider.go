package main

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/pkg/plugins"
)

// ExampleProviderPlugin implementa un provider custom di esempio
type ExampleProviderPlugin struct {
	name        string
	version     string
	description string
	provider    *ExampleProvider
	logger      plugins.Logger
	config      map[string]interface{}
}

// ExampleProvider implementa providers.Provider
type ExampleProvider struct {
	*providers.BaseProvider
	apiEndpoint string
	customConfig map[string]interface{}
}

// NewPlugin è la factory function richiesta dal plugin system
// Questa funzione DEVE essere esportata con questo nome esatto
func NewPlugin() (plugins.Plugin, error) {
	return &ExampleProviderPlugin{
		name:        "example-provider",
		version:     "1.0.0",
		description: "Example custom LLM provider plugin",
	}, nil
}

// Name restituisce il nome del plugin
func (p *ExampleProviderPlugin) Name() string {
	return p.name
}

// Version restituisce la versione del plugin
func (p *ExampleProviderPlugin) Version() string {
	return p.version
}

// Description restituisce la descrizione del plugin
func (p *ExampleProviderPlugin) Description() string {
	return p.description
}

// Type restituisce il tipo di plugin
func (p *ExampleProviderPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeProvider
}

// Metadata restituisce metadata aggiuntivi
func (p *ExampleProviderPlugin) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"author":      "GoLeapAI Team",
		"license":     "MIT",
		"homepage":    "https://github.com/biodoia/goleapifree",
		"supports":    []string{"chat", "streaming"},
		"models":      []string{"example-model-1", "example-model-2"},
		"region":      "global",
		"cost_tier":   "free",
	}
}

// Init inizializza il plugin
func (p *ExampleProviderPlugin) Init(ctx context.Context, deps *plugins.Dependencies) error {
	p.logger = deps.Logger
	p.config = deps.Config

	// Log inizializzazione
	p.logger.Info("Initializing example provider plugin", map[string]interface{}{
		"name":    p.name,
		"version": p.version,
	})

	// Recupera configurazione custom
	apiEndpoint := "https://api.example.com/v1"
	if endpoint, ok := p.config["api_endpoint"].(string); ok {
		apiEndpoint = endpoint
	}

	apiKey := ""
	if key, ok := p.config["api_key"].(string); ok {
		apiKey = key
	}

	// Crea il provider
	base := providers.NewBaseProvider(p.name, apiEndpoint, apiKey)

	// Configura feature supportate
	base.SetFeature(providers.FeatureStreaming, true)
	base.SetFeature(providers.FeatureJSONMode, true)
	base.SetFeature(providers.FeatureSystemMsg, true)
	base.SetFeature(providers.FeatureTools, false)

	p.provider = &ExampleProvider{
		BaseProvider: base,
		apiEndpoint:  apiEndpoint,
		customConfig: p.config,
	}

	// Registra hook di esempio
	if deps.Hooks != nil {
		deps.Hooks.OnRequest(p.onRequest)
		deps.Hooks.OnResponse(p.onResponse)
		deps.Hooks.OnError(p.onError)
	}

	p.logger.Info("Example provider plugin initialized successfully", nil)
	return nil
}

// Shutdown esegue il cleanup del plugin
func (p *ExampleProviderPlugin) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down example provider plugin", nil)
	// Cleanup resources qui
	return nil
}

// GetProvider restituisce l'istanza del provider
func (p *ExampleProviderPlugin) GetProvider() (providers.Provider, error) {
	if p.provider == nil {
		return nil, fmt.Errorf("provider not initialized")
	}
	return p.provider, nil
}

// HealthCheck verifica lo stato del provider
func (p *ExampleProviderPlugin) HealthCheck(ctx context.Context) error {
	// Implementa un health check custom
	p.logger.Debug("Running health check", nil)

	// Simula un health check
	select {
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetModels restituisce i modelli supportati
func (p *ExampleProviderPlugin) GetModels(ctx context.Context) ([]providers.ModelInfo, error) {
	models := []providers.ModelInfo{
		{
			ID:            "example-model-1",
			Name:          "Example Model 1",
			Provider:      p.name,
			ContextLength: 8192,
			MaxTokens:     4096,
			Capabilities: map[string]bool{
				"streaming":  true,
				"json_mode":  true,
				"vision":     false,
			},
			Pricing: &providers.PricingInfo{
				InputPer1K:  0.0,
				OutputPer1K: 0.0,
				Currency:    "USD",
			},
			Metadata: map[string]interface{}{
				"tier": "free",
			},
		},
		{
			ID:            "example-model-2",
			Name:          "Example Model 2",
			Provider:      p.name,
			ContextLength: 32768,
			MaxTokens:     8192,
			Capabilities: map[string]bool{
				"streaming":  true,
				"json_mode":  true,
				"vision":     false,
			},
			Pricing: &providers.PricingInfo{
				InputPer1K:  0.001,
				OutputPer1K: 0.002,
				Currency:    "USD",
			},
			Metadata: map[string]interface{}{
				"tier": "pro",
			},
		},
	}

	return models, nil
}

// EstimateCost stima il costo di una richiesta
func (p *ExampleProviderPlugin) EstimateCost(req *providers.ChatRequest) (float64, error) {
	// Implementa la stima del costo
	// Per l'esempio, assumiamo ~4 caratteri per token
	totalChars := 0
	for _, msg := range req.Messages {
		if content, ok := msg.Content.(string); ok {
			totalChars += len(content)
		}
	}

	inputTokens := totalChars / 4
	outputTokens := 0
	if req.MaxTokens != nil {
		outputTokens = *req.MaxTokens
	} else {
		outputTokens = 1000 // Default
	}

	// Costo per il modello di esempio (se non free)
	inputCost := float64(inputTokens) / 1000.0 * 0.001
	outputCost := float64(outputTokens) / 1000.0 * 0.002

	return inputCost + outputCost, nil
}

// Hook handlers

func (p *ExampleProviderPlugin) onRequest(ctx context.Context, req *plugins.HookRequest) error {
	p.logger.Debug("Request hook triggered", map[string]interface{}{
		"request_id": req.RequestID,
		"provider":   req.Provider,
		"model":      req.Model,
	})
	return nil
}

func (p *ExampleProviderPlugin) onResponse(ctx context.Context, resp *plugins.HookResponse) error {
	p.logger.Debug("Response hook triggered", map[string]interface{}{
		"request_id": resp.RequestID,
		"duration":   resp.Duration.String(),
		"tokens":     resp.TokensUsed,
		"cost":       resp.Cost,
	})
	return nil
}

func (p *ExampleProviderPlugin) onError(ctx context.Context, err *plugins.HookError) error {
	p.logger.Error("Error hook triggered", map[string]interface{}{
		"request_id": err.RequestID,
		"error":      err.Error.Error(),
		"error_type": string(err.ErrorType),
		"retryable":  err.Retryable,
	})
	return nil
}

// Implementazione del Provider interface

func (ep *ExampleProvider) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// Implementa la logica di chat completion
	// Questo è solo un esempio che restituisce una risposta mock

	response := &providers.ChatResponse{
		ID:      "example-" + time.Now().Format("20060102150405"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []providers.Choice{
			{
				Index: 0,
				Message: providers.Message{
					Role:    "assistant",
					Content: "This is an example response from the custom provider plugin.",
				},
				FinishReason: "stop",
			},
		},
		Usage: providers.Usage{
			PromptTokens:     100,
			CompletionTokens: 20,
			TotalTokens:      120,
		},
	}

	return response, nil
}

func (ep *ExampleProvider) Stream(ctx context.Context, req *providers.ChatRequest, handler providers.StreamHandler) error {
	// Implementa lo streaming
	// Esempio: simula streaming con chunks

	chunks := []string{
		"This ",
		"is ",
		"a ",
		"streaming ",
		"response ",
		"from ",
		"the ",
		"custom ",
		"provider ",
		"plugin.",
	}

	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			streamChunk := &providers.StreamChunk{
				Delta:        chunk,
				Done:         i == len(chunks)-1,
				FinishReason: "",
			}

			if streamChunk.Done {
				streamChunk.FinishReason = "stop"
				streamChunk.Usage = &providers.Usage{
					PromptTokens:     100,
					CompletionTokens: 10,
					TotalTokens:      110,
				}
			}

			if err := handler(streamChunk); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ep *ExampleProvider) HealthCheck(ctx context.Context) error {
	// Implementa health check specifico del provider
	return nil
}

func (ep *ExampleProvider) GetModels(ctx context.Context) ([]providers.ModelInfo, error) {
	// Delega al plugin
	return nil, fmt.Errorf("use plugin GetModels instead")
}
