package anthropic

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Provider rappresenta un provider Anthropic per GoLeapAI
type Provider struct {
	client  *Client
	adapter *Adapter
	config  ProviderConfig
}

// ProviderConfig contiene la configurazione del provider
type ProviderConfig struct {
	APIKey     string
	BaseURL    string
	APIVersion string
	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
}

// NewProvider crea un nuovo provider Anthropic
func NewProvider(config ProviderConfig) *Provider {
	// Default configuration
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.APIVersion == "" {
		config.APIVersion = DefaultAPIVersion
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.UserAgent == "" {
		config.UserAgent = "GoLeapAI/1.0"
	}

	// Create HTTP client with custom timeout
	// httpClient := &http.Client{
	// 	Timeout: config.Timeout,
	// }

	// Create Anthropic client
	client := NewClient(
		config.APIKey,
		WithBaseURL(config.BaseURL),
		WithAPIVersion(config.APIVersion),
		WithUserAgent(config.UserAgent),
	)

	// Create adapter
	adapter := NewAdapter()

	return &Provider{
		client:  client,
		adapter: adapter,
		config:  config,
	}
}

// ChatCompletion gestisce una richiesta di chat completion OpenAI-compatible
func (p *Provider) ChatCompletion(ctx context.Context, req *OpenAIRequest) (*OpenAIResponse, error) {
	// Converti richiesta OpenAI -> Anthropic
	anthropicReq, err := p.adapter.ConvertRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	// Log della richiesta
	log.Debug().
		Str("model", anthropicReq.Model).
		Int("max_tokens", anthropicReq.MaxTokens).
		Int("messages", len(anthropicReq.Messages)).
		Bool("stream", anthropicReq.Stream).
		Msg("Sending request to Anthropic")

	// Invia richiesta con retry logic
	var anthropicResp *MessagesResponse
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Debug().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Retrying request")
			time.Sleep(backoff)
		}

		anthropicResp, lastErr = p.client.CreateMessage(ctx, anthropicReq)
		if lastErr == nil {
			break
		}

		// Controlla se l'errore è ritentabile
		if apiErr, ok := lastErr.(*Error); ok {
			if !apiErr.IsRetryable() {
				// Errore non ritentabile, fallisci subito
				break
			}
			log.Debug().
				Str("error_type", apiErr.Type).
				Str("error_msg", apiErr.Message).
				Msg("Retryable error occurred")
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", p.config.MaxRetries+1, lastErr)
	}

	// Log della risposta
	log.Debug().
		Str("id", anthropicResp.ID).
		Str("stop_reason", string(anthropicResp.StopReason)).
		Int("input_tokens", anthropicResp.Usage.InputTokens).
		Int("output_tokens", anthropicResp.Usage.OutputTokens).
		Msg("Received response from Anthropic")

	// Converti risposta Anthropic -> OpenAI
	openaiResp := p.adapter.ConvertResponse(anthropicResp)

	return openaiResp, nil
}

// ChatCompletionStream gestisce una richiesta di chat completion in streaming
func (p *Provider) ChatCompletionStream(ctx context.Context, req *OpenAIRequest) (<-chan *OpenAIStreamChunk, <-chan error) {
	chunkCh := make(chan *OpenAIStreamChunk, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		// Converti richiesta
		anthropicReq, err := p.adapter.ConvertRequest(req)
		if err != nil {
			errCh <- fmt.Errorf("failed to convert request: %w", err)
			return
		}

		// Forza streaming
		anthropicReq.Stream = true

		// Log della richiesta
		log.Debug().
			Str("model", anthropicReq.Model).
			Int("max_tokens", anthropicReq.MaxTokens).
			Msg("Starting streaming request to Anthropic")

		// Avvia streaming
		eventCh, streamErrCh := p.client.CreateMessageStream(ctx, anthropicReq)

		// Processa eventi
		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}

				// Converti evento Anthropic -> OpenAI
				chunk := p.adapter.ConvertStreamEvent(event)
				if chunk != nil && len(chunk.Choices) > 0 {
					chunkCh <- chunk
				}

			case err := <-streamErrCh:
				if err != nil {
					errCh <- err
					return
				}

			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
	}()

	return chunkCh, errCh
}

// CreateMessage invia una richiesta diretta all'API Anthropic (no conversione)
func (p *Provider) CreateMessage(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	return p.client.CreateMessage(ctx, req)
}

// CreateMessageStream invia una richiesta streaming diretta (no conversione)
func (p *Provider) CreateMessageStream(ctx context.Context, req *MessagesRequest) (<-chan StreamEvent, <-chan error) {
	return p.client.CreateMessageStream(ctx, req)
}

// Health verifica lo stato di salute del provider
func (p *Provider) Health(ctx context.Context) error {
	return p.client.Health(ctx)
}

// GetClient restituisce il client Anthropic sottostante
func (p *Provider) GetClient() *Client {
	return p.client
}

// GetAdapter restituisce l'adapter
func (p *Provider) GetAdapter() *Adapter {
	return p.adapter
}

// GetConfig restituisce la configurazione del provider
func (p *Provider) GetConfig() ProviderConfig {
	return p.config
}

// EstimateCost stima il costo di una richiesta
func (p *Provider) EstimateCost(req *MessagesRequest, model string) float64 {
	// Prezzi per 1M di token (in USD)
	prices := map[string]struct {
		input  float64
		output float64
	}{
		ModelClaude3Opus: {
			input:  15.00,  // $15 per 1M input tokens
			output: 75.00,  // $75 per 1M output tokens
		},
		ModelClaude3Sonnet: {
			input:  3.00,   // $3 per 1M input tokens
			output: 15.00,  // $15 per 1M output tokens
		},
		ModelClaude3Haiku: {
			input:  0.25,   // $0.25 per 1M input tokens
			output: 1.25,   // $1.25 per 1M output tokens
		},
		ModelClaude35Sonnet: {
			input:  3.00,   // $3 per 1M input tokens
			output: 15.00,  // $15 per 1M output tokens
		},
		ModelClaude35Haiku: {
			input:  1.00,   // $1 per 1M input tokens
			output: 5.00,   // $5 per 1M output tokens
		},
	}

	price, ok := prices[model]
	if !ok {
		// Default to Claude 3.5 Sonnet pricing
		price = prices[ModelClaude35Sonnet]
	}

	// Stima token in input
	inputTokens := p.client.CountTokens(req)

	// Stima token in output (usa max_tokens come worst case)
	outputTokens := req.MaxTokens

	// Calcola costi
	inputCost := (float64(inputTokens) / 1_000_000) * price.input
	outputCost := (float64(outputTokens) / 1_000_000) * price.output

	return inputCost + outputCost
}

// GetModelInfo restituisce informazioni su un modello
func (p *Provider) GetModelInfo(model string) ModelInfo {
	models := map[string]ModelInfo{
		ModelClaude3Opus: {
			Name:            ModelClaude3Opus,
			DisplayName:     "Claude 3 Opus",
			ContextWindow:   200000,
			MaxOutputTokens: 4096,
			InputPrice:      15.00,
			OutputPrice:     75.00,
			SupportsVision:  true,
			SupportsTools:   true,
		},
		ModelClaude3Sonnet: {
			Name:            ModelClaude3Sonnet,
			DisplayName:     "Claude 3 Sonnet",
			ContextWindow:   200000,
			MaxOutputTokens: 4096,
			InputPrice:      3.00,
			OutputPrice:     15.00,
			SupportsVision:  true,
			SupportsTools:   true,
		},
		ModelClaude3Haiku: {
			Name:            ModelClaude3Haiku,
			DisplayName:     "Claude 3 Haiku",
			ContextWindow:   200000,
			MaxOutputTokens: 4096,
			InputPrice:      0.25,
			OutputPrice:     1.25,
			SupportsVision:  true,
			SupportsTools:   true,
		},
		ModelClaude35Sonnet: {
			Name:            ModelClaude35Sonnet,
			DisplayName:     "Claude 3.5 Sonnet",
			ContextWindow:   200000,
			MaxOutputTokens: 8192,
			InputPrice:      3.00,
			OutputPrice:     15.00,
			SupportsVision:  true,
			SupportsTools:   true,
		},
		ModelClaude35Haiku: {
			Name:            ModelClaude35Haiku,
			DisplayName:     "Claude 3.5 Haiku",
			ContextWindow:   200000,
			MaxOutputTokens: 8192,
			InputPrice:      1.00,
			OutputPrice:     5.00,
			SupportsVision:  true,
			SupportsTools:   true,
		},
	}

	if info, ok := models[model]; ok {
		return info
	}

	// Default
	return models[ModelClaude35Sonnet]
}

// ListModels restituisce la lista di modelli disponibili
func (p *Provider) ListModels() []ModelInfo {
	return []ModelInfo{
		p.GetModelInfo(ModelClaude35Sonnet),
		p.GetModelInfo(ModelClaude35Haiku),
		p.GetModelInfo(ModelClaude3Opus),
		p.GetModelInfo(ModelClaude3Sonnet),
		p.GetModelInfo(ModelClaude3Haiku),
	}
}

// ModelInfo contiene informazioni su un modello
type ModelInfo struct {
	Name            string
	DisplayName     string
	ContextWindow   int
	MaxOutputTokens int
	InputPrice      float64 // USD per 1M tokens
	OutputPrice     float64 // USD per 1M tokens
	SupportsVision  bool
	SupportsTools   bool
}
