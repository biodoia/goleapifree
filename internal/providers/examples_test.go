package providers_test

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/internal/providers/openai"
)

// Example di utilizzo base del client OpenAI
func Example_openAIClientBasic() {
	// Crea un client OpenAI
	client := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-your-api-key",
	)

	// Prepara una richiesta
	req := &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "What is the capital of France?",
			},
		},
		Temperature: ptr(0.7),
		MaxTokens:   ptr(100),
	}

	// Esegui la richiesta
	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Stampa la risposta
	if len(resp.Choices) > 0 {
		content := resp.Choices[0].Message.Content
		fmt.Printf("Response: %v\n", content)
	}
}

// Example di streaming
func Example_openAIClientStreaming() {
	client := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-your-api-key",
	)

	req := &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "Write a short poem about Go programming.",
			},
		},
		Stream: true,
	}

	ctx := context.Background()
	err := client.Stream(ctx, req, func(chunk *providers.StreamChunk) error {
		if chunk.Done {
			fmt.Println("\n[Stream completed]")
			if chunk.Usage != nil {
				fmt.Printf("Tokens used: %d\n", chunk.Usage.TotalTokens)
			}
			return nil
		}

		fmt.Print(chunk.Delta)
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// Example di tool calling
func Example_openAIClientToolCalling() {
	client := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-your-api-key",
	)

	// Define tools
	tools := []providers.Tool{
		{
			Type: "function",
			Function: providers.Function{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city name",
						},
						"unit": map[string]interface{}{
							"type": "string",
							"enum": []string{"celsius", "fahrenheit"},
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	req := &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "What's the weather in Paris?",
			},
		},
		Tools:      tools,
		ToolChoice: "auto",
	}

	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Check for tool calls
	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				fmt.Printf("Tool call: %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			}
		}
	}
}

// Example di JSON mode
func Example_openAIClientJSONMode() {
	client := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-your-api-key",
	)

	req := &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that outputs JSON.",
			},
			{
				Role:    "user",
				Content: "Generate a person with name and age",
			},
		},
		ResponseFormat: &providers.ResponseFormat{
			Type: "json_object",
		},
	}

	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("JSON: %v\n", resp.Choices[0].Message.Content)
	}
}

// Example di utilizzo del Registry
func Example_registry() {
	registry := providers.NewRegistry()

	// Registra più provider
	openaiClient := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-key-1",
	)
	registry.Register("openai", openaiClient, "openai")

	anthropicClient := openai.NewClient( // Può essere OpenAI-compatible
		"anthropic",
		"https://api.anthropic.com",
		"sk-key-2",
	)
	registry.Register("anthropic", anthropicClient, "anthropic")

	// Lista provider
	fmt.Println("Providers:", registry.List())

	// Get provider specifico
	provider, err := registry.Get("openai")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Usa il provider
	req := &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	ctx := context.Background()
	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %v\n", resp.Choices[0].Message.Content)
}

// Example di health check
func Example_registryHealthCheck() {
	registry := providers.NewRegistry()

	// Registra provider
	client := openai.NewClient(
		"openai",
		"https://api.openai.com",
		"sk-your-api-key",
	)
	registry.Register("openai", client, "openai")

	// Esegui health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := registry.HealthCheck(ctx)

	for provider, err := range results {
		if err != nil {
			fmt.Printf("Provider %s: unhealthy - %v\n", provider, err)
		} else {
			fmt.Printf("Provider %s: healthy\n", provider)
		}
	}

	// Get stats
	stats := registry.GetStats()
	fmt.Printf("Stats: %d total, %d active, %d healthy\n",
		stats.TotalProviders,
		stats.ActiveProviders,
		stats.HealthyProviders,
	)
}

// Example di fallback automatico
func Example_registryAutoFallback() {
	registry := providers.NewRegistry()

	// Registra più provider come fallback
	registry.Register("primary", openai.NewClient("primary", "https://api1.com", "key1"), "openai")
	registry.Register("backup", openai.NewClient("backup", "https://api2.com", "key2"), "openai")

	req := &providers.ChatRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []providers.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()

	// Prova primary, fallback automatico a backup se fallisce
	provider, err := registry.GetOrFirst("primary")
	if err != nil {
		provider, err = registry.GetFirst()
	}

	if err != nil {
		fmt.Printf("No providers available: %v\n", err)
		return
	}

	start := time.Now()
	resp, err := provider.ChatCompletion(ctx, req)
	latency := time.Since(start)

	if err != nil {
		registry.RecordError(provider.Name())
		fmt.Printf("Error: %v\n", err)
	} else {
		registry.RecordSuccess(provider.Name(), latency)
		fmt.Printf("Success: %v\n", resp.Choices[0].Message.Content)
	}
}

// Example di custom provider compatible con OpenAI
func Example_customProvider() {
	// Provider custom che espone API OpenAI-compatible
	client := openai.NewClient(
		"custom-llm",
		"https://my-custom-api.com",
		"custom-api-key",
	)

	// Configura timeout e retry personalizzati
	client.SetTimeout(60 * time.Second)
	client.SetMaxRetries(5)

	// Configura capabilities
	client.SetFeature(providers.FeatureTools, false) // Non supporta tools
	client.SetFeature(providers.FeatureVision, true) // Supporta vision

	// Usa normalmente
	req := &providers.ChatRequest{
		Model:    "custom-model-v1",
		Messages: []providers.Message{{Role: "user", Content: "Test"}},
	}

	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %v\n", resp.Choices[0].Message.Content)
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}
