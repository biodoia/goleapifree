package anthropic_test

import (
	"context"
	"fmt"
	"os"

	"github.com/biodoia/goleapifree/internal/providers/anthropic"
)

// Helper function for tests
func ptrFloat64(f float64) *float64 {
	return &f
}

// ExampleClient_CreateMessage dimostra come usare il client Anthropic
func ExampleClient_CreateMessage() {
	client := anthropic.NewClient(os.Getenv("ANTHROPIC_API_KEY"))

	req := &anthropic.MessagesRequest{
		Model:     anthropic.ModelClaude35Haiku,
		MaxTokens: 1024,
		Messages: []anthropic.Message{
			{
				Role: anthropic.MessageRoleUser,
				Content: []anthropic.ContentBlock{
					anthropic.NewTextContentBlock("Explain quantum computing in simple terms."),
				},
			},
		},
	}

	resp, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", resp.GetText())
	fmt.Printf("Tokens used: %d input, %d output\n",
		resp.Usage.InputTokens,
		resp.Usage.OutputTokens)
}

// ExampleClient_CreateMessageStream dimostra lo streaming
func ExampleClient_CreateMessageStream() {
	client := anthropic.NewClient(os.Getenv("ANTHROPIC_API_KEY"))

	req := &anthropic.MessagesRequest{
		Model:     anthropic.ModelClaude35Sonnet,
		MaxTokens: 1024,
		Messages: []anthropic.Message{
			{
				Role: anthropic.MessageRoleUser,
				Content: []anthropic.ContentBlock{
					anthropic.NewTextContentBlock("Write a haiku about coding."),
				},
			},
		},
	}

	eventCh, errCh := client.CreateMessageStream(context.Background(), req)

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return
			}

			if event.Delta != nil && event.Delta.Text != "" {
				fmt.Print(event.Delta.Text)
			}

		case err := <-errCh:
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
				return
			}
		}
	}
}

// ExampleClient_multiTurnConversation dimostra una conversazione multi-turn
func ExampleClient_multiTurnConversation() {
	client := anthropic.NewClient(os.Getenv("ANTHROPIC_API_KEY"))

	messages := []anthropic.Message{
		{
			Role: anthropic.MessageRoleUser,
			Content: []anthropic.ContentBlock{
				anthropic.NewTextContentBlock("Hello! What's your name?"),
			},
		},
	}

	// Prima richiesta
	resp1, err := client.CreateMessage(context.Background(), &anthropic.MessagesRequest{
		Model:     anthropic.ModelClaude35Haiku,
		MaxTokens: 100,
		Messages:  messages,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Claude: %s\n", resp1.GetText())

	// Aggiungi la risposta alla conversazione
	messages = append(messages, anthropic.Message{
		Role:    anthropic.MessageRoleAssistant,
		Content: resp1.Content,
	})

	// Seconda richiesta
	messages = append(messages, anthropic.Message{
		Role: anthropic.MessageRoleUser,
		Content: []anthropic.ContentBlock{
			anthropic.NewTextContentBlock("Can you help me write Python code?"),
		},
	})

	resp2, err := client.CreateMessage(context.Background(), &anthropic.MessagesRequest{
		Model:     anthropic.ModelClaude35Haiku,
		MaxTokens: 500,
		Messages:  messages,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Claude: %s\n", resp2.GetText())
}

// ExampleAdapter_ConvertRequest dimostra la conversione OpenAI->Anthropic
func ExampleAdapter_ConvertRequest() {
	adapter := anthropic.NewAdapter()

	openaiReq := &anthropic.OpenAIRequest{
		Model: "gpt-4",
		Messages: []anthropic.OpenAIMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello!",
			},
		},
		MaxTokens:   1024,
		Temperature: ptrFloat64(0.7),
	}

	anthropicReq, err := adapter.ConvertRequest(openaiReq)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Model: %s\n", anthropicReq.Model)
	fmt.Printf("System: %s\n", anthropicReq.System)
	fmt.Printf("Messages: %d\n", len(anthropicReq.Messages))
}

// ExampleAdapter_ConvertResponse dimostra la conversione Anthropic->OpenAI
func ExampleAdapter_ConvertResponse() {
	adapter := anthropic.NewAdapter()

	anthropicResp := &anthropic.MessagesResponse{
		ID:    "msg_123",
		Model: anthropic.ModelClaude35Sonnet,
		Role:  anthropic.MessageRoleAssistant,
		Content: []anthropic.ContentBlock{
			anthropic.NewTextContentBlock("Hello! How can I help you today?"),
		},
		StopReason: anthropic.StopReasonEndTurn,
		Usage: anthropic.Usage{
			InputTokens:  10,
			OutputTokens: 15,
		},
	}

	openaiResp := adapter.ConvertResponse(anthropicResp)

	fmt.Printf("ID: %s\n", openaiResp.ID)
	fmt.Printf("Model: %s\n", openaiResp.Model)
	fmt.Printf("Content: %s\n", openaiResp.Choices[0].Message.Content)
	fmt.Printf("Total tokens: %d\n", openaiResp.Usage.TotalTokens)
}
