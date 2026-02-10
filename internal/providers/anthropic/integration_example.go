package anthropic

import (
	"context"
	"fmt"
	"os"
	"time"
)

// IntegrationExample mostra come integrare il provider nel gateway GoLeapAI
type IntegrationExample struct {
	provider *Provider
}

// NewIntegrationExample crea un nuovo esempio di integrazione
func NewIntegrationExample() *IntegrationExample {
	// Configurazione del provider
	config := ProviderConfig{
		APIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL:    DefaultBaseURL,
		APIVersion: DefaultAPIVersion,
		Timeout:    120 * time.Second,
		MaxRetries: 3,
		UserAgent:  "GoLeapAI/1.0",
	}

	// Crea il provider
	provider := NewProvider(config)

	return &IntegrationExample{
		provider: provider,
	}
}

// HandleOpenAIRequest gestisce una richiesta OpenAI-compatible
// Questa è la funzione che il gateway chiamerebbe
func (ie *IntegrationExample) HandleOpenAIRequest(ctx context.Context, req *OpenAIRequest) (*OpenAIResponse, error) {
	// Il provider gestisce automaticamente la conversione
	return ie.provider.ChatCompletion(ctx, req)
}

// HandleOpenAIStreamRequest gestisce una richiesta streaming OpenAI-compatible
func (ie *IntegrationExample) HandleOpenAIStreamRequest(ctx context.Context, req *OpenAIRequest) (<-chan *OpenAIStreamChunk, <-chan error) {
	return ie.provider.ChatCompletionStream(ctx, req)
}

// HandleAnthropicRequest gestisce una richiesta nativa Anthropic
// Utile per endpoint /v1/messages
func (ie *IntegrationExample) HandleAnthropicRequest(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	return ie.provider.CreateMessage(ctx, req)
}

// ExampleGatewayIntegration mostra un esempio completo di integrazione
func ExampleGatewayIntegration() {
	example := NewIntegrationExample()

	// Esempio 1: Richiesta OpenAI-compatible che viene inoltrata a Claude
	fmt.Println("=== Esempio 1: OpenAI -> Claude ===")
	openaiReq := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful coding assistant."},
			{Role: "user", Content: "Explain what is a closure in JavaScript."},
		},
		MaxTokens:   1024,
		Temperature: ptrFloat64(0.7),
	}

	resp, err := example.HandleOpenAIRequest(context.Background(), openaiReq)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Model used: %s\n", resp.Model)
	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)
	fmt.Printf("Tokens: %d total (%d prompt + %d completion)\n",
		resp.Usage.TotalTokens,
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens)

	// Esempio 2: Streaming
	fmt.Println("\n=== Esempio 2: Streaming ===")
	streamReq := &OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Write a haiku about programming."},
		},
		MaxTokens: 100,
		Stream:    true,
	}

	chunkCh, errCh := example.HandleOpenAIStreamRequest(context.Background(), streamReq)

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				fmt.Println("\nStreaming completed")
				goto next
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				fmt.Print(chunk.Choices[0].Delta.Content)
			}
		case err := <-errCh:
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
				return
			}
		}
	}

next:
	// Esempio 3: Richiesta nativa Anthropic
	fmt.Println("\n=== Esempio 3: Native Anthropic API ===")
	anthropicReq := &MessagesRequest{
		Model:     ModelClaude35Haiku,
		MaxTokens: 1024,
		System:    "You are a helpful assistant specializing in Go programming.",
		Messages: []Message{
			{
				Role: MessageRoleUser,
				Content: []ContentBlock{
					NewTextContentBlock("What are goroutines?"),
				},
			},
		},
	}

	anthropicResp, err := example.HandleAnthropicRequest(context.Background(), anthropicReq)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", anthropicResp.GetText())
	fmt.Printf("Stop reason: %s\n", anthropicResp.StopReason)
	fmt.Printf("Usage: %d input, %d output tokens\n",
		anthropicResp.Usage.InputTokens,
		anthropicResp.Usage.OutputTokens)
}

// ExampleMultiTurnConversation mostra una conversazione multi-turno
func ExampleMultiTurnConversation() {
	example := NewIntegrationExample()

	fmt.Println("=== Multi-turn Conversation ===")

	// Prima richiesta
	messages := []OpenAIMessage{
		{Role: "user", Content: "Hi! My name is Alice."},
	}

	req := &OpenAIRequest{
		Model:     "gpt-4",
		Messages:  messages,
		MaxTokens: 100,
	}

	resp, err := example.HandleOpenAIRequest(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Claude: %s\n", resp.Choices[0].Message.Content)

	// Aggiungi risposta alla conversazione
	messages = append(messages, OpenAIMessage{
		Role:    "assistant",
		Content: resp.Choices[0].Message.Content,
	})

	// Seconda richiesta con contesto
	messages = append(messages, OpenAIMessage{
		Role:    "user",
		Content: "What's my name?",
	})

	req.Messages = messages
	resp2, err := example.HandleOpenAIRequest(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Claude: %s\n", resp2.Choices[0].Message.Content)
}

// ExampleToolCalling mostra l'uso di function calling
func ExampleToolCalling() {
	example := NewIntegrationExample()

	fmt.Println("=== Tool Calling Example ===")

	// Definisci tools
	tools := []OpenAITool{
		{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        "get_weather",
				Description: "Get the current weather in a given location",
				Parameters:  []byte(`{"type":"object","properties":{"location":{"type":"string","description":"The city and state, e.g. San Francisco, CA"},"unit":{"type":"string","enum":["celsius","fahrenheit"]}},"required":["location"]}`),
			},
		},
		{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        "get_time",
				Description: "Get the current time in a given timezone",
				Parameters:  []byte(`{"type":"object","properties":{"timezone":{"type":"string","description":"The IANA timezone name, e.g. America/New_York"}},"required":["timezone"]}`),
			},
		},
	}

	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "What's the weather like in Paris and what time is it there?"},
		},
		MaxTokens: 500,
		Tools:     tools,
	}

	resp, err := example.HandleOpenAIRequest(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Controlla se Claude vuole usare delle tools
	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		fmt.Println("Claude wants to use tools:")
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			fmt.Printf("- %s: %s\n", tc.Function.Name, tc.Function.Arguments)
		}
	} else {
		fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)
	}
}

// ExampleHealthCheck mostra come verificare lo stato del provider
func ExampleHealthCheck() {
	example := NewIntegrationExample()

	fmt.Println("=== Health Check ===")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := example.provider.Health(ctx); err != nil {
		fmt.Printf("Provider is unhealthy: %v\n", err)
	} else {
		fmt.Println("Provider is healthy")
	}
}

// ExampleCostEstimation mostra come stimare i costi
func ExampleCostEstimation() {
	example := NewIntegrationExample()

	fmt.Println("=== Cost Estimation ===")

	req := &MessagesRequest{
		Model:     ModelClaude35Sonnet,
		MaxTokens: 2000,
		Messages: []Message{
			{
				Role: MessageRoleUser,
				Content: []ContentBlock{
					NewTextContentBlock("Write a detailed explanation of quantum computing."),
				},
			},
		},
	}

	cost := example.provider.EstimateCost(req, req.Model)
	fmt.Printf("Estimated cost: $%.6f\n", cost)

	// Mostra info del modello
	modelInfo := example.provider.GetModelInfo(req.Model)
	fmt.Printf("Model: %s\n", modelInfo.DisplayName)
	fmt.Printf("Context window: %d tokens\n", modelInfo.ContextWindow)
	fmt.Printf("Max output: %d tokens\n", modelInfo.MaxOutputTokens)
	fmt.Printf("Input price: $%.2f per 1M tokens\n", modelInfo.InputPrice)
	fmt.Printf("Output price: $%.2f per 1M tokens\n", modelInfo.OutputPrice)
}

// ExampleListModels mostra come ottenere la lista dei modelli
func ExampleListModels() {
	example := NewIntegrationExample()

	fmt.Println("=== Available Models ===")

	models := example.provider.ListModels()
	for _, model := range models {
		fmt.Printf("\n%s (%s)\n", model.DisplayName, model.Name)
		fmt.Printf("  Context: %d tokens\n", model.ContextWindow)
		fmt.Printf("  Max output: %d tokens\n", model.MaxOutputTokens)
		fmt.Printf("  Pricing: $%.2f/$%.2f per 1M tokens (in/out)\n",
			model.InputPrice, model.OutputPrice)
		fmt.Printf("  Vision: %v, Tools: %v\n",
			model.SupportsVision, model.SupportsTools)
	}
}
