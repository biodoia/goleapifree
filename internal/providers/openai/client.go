package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidAPIKey     = errors.New("invalid API key")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrModelNotFound     = errors.New("model not found")
	ErrInvalidRequest    = errors.New("invalid request")
	ErrServiceUnavailable = errors.New("service unavailable")
)

// Client implementa un client OpenAI-compatible
type Client struct {
	*providers.BaseProvider
	httpClient *resty.Client
}

// NewClient crea un nuovo client OpenAI
func NewClient(name, baseURL, apiKey string) *Client {
	base := providers.NewBaseProvider(name, baseURL, apiKey)

	// Configura capabilities per OpenAI
	base.SetFeature(providers.FeatureStreaming, true)
	base.SetFeature(providers.FeatureTools, true)
	base.SetFeature(providers.FeatureJSONMode, true)
	base.SetFeature(providers.FeatureVision, true)
	base.SetFeature(providers.FeatureFunctionCall, true)

	client := &Client{
		BaseProvider: base,
		httpClient:   resty.New(),
	}

	client.configureHTTPClient()
	return client
}

// configureHTTPClient configura il client HTTP con retry e timeout
func (c *Client) configureHTTPClient() {
	c.httpClient.
		SetBaseURL(c.GetBaseURL()).
		SetTimeout(c.GetTimeout()).
		SetRetryCount(c.GetMaxRetries()).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Retry on 5xx errors and specific 4xx errors
			if r == nil {
				return true
			}
			return r.StatusCode() >= 500 ||
				   r.StatusCode() == 429 || // Rate limit
				   r.StatusCode() == 408    // Request timeout
		}).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	// Set API key if present
	if c.GetAPIKey() != "" {
		c.httpClient.SetHeader("Authorization", "Bearer "+c.GetAPIKey())
	}

	// Add request/response logging
	c.httpClient.OnBeforeRequest(func(client *resty.Client, req *resty.Request) error {
		log.Debug().
			Str("provider", c.Name()).
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("OpenAI API request")
		return nil
	})

	c.httpClient.OnAfterResponse(func(client *resty.Client, resp *resty.Response) error {
		log.Debug().
			Str("provider", c.Name()).
			Int("status", resp.StatusCode()).
			Dur("duration", resp.Time()).
			Msg("OpenAI API response")
		return nil
	})
}

// ChatCompletion esegue una richiesta di chat completion
func (c *Client) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// Converti la richiesta generica in formato OpenAI
	openaiReq := c.convertToOpenAIRequest(req)

	var openaiResp ChatCompletionResponse
	var errResp ErrorResponse

	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetBody(openaiReq).
		SetResult(&openaiResp).
		SetError(&errResp).
		Post("/v1/chat/completions")

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle errors
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp.StatusCode(), &errResp)
	}

	// Converti la risposta OpenAI in formato generico
	return c.convertFromOpenAIResponse(&openaiResp), nil
}

// Stream esegue una richiesta di chat completion con streaming
func (c *Client) Stream(ctx context.Context, req *providers.ChatRequest, handler providers.StreamHandler) error {
	// Converti la richiesta e abilita streaming
	openaiReq := c.convertToOpenAIRequest(req)
	openaiReq.Stream = true

	// Crea la richiesta HTTP
	httpReq, err := c.createStreamRequest(ctx, openaiReq)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Esegui la richiesta
	httpResp, err := c.httpClient.GetClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Verifica errori HTTP
	if httpResp.StatusCode != http.StatusOK {
		return c.handleStreamError(httpResp)
	}

	// Process SSE stream
	return c.processStream(httpResp.Body, handler)
}

// createStreamRequest crea una richiesta HTTP per lo streaming
func (c *Client) createStreamRequest(ctx context.Context, req *ChatCompletionRequest) (*http.Request, error) {
	url := c.GetBaseURL() + "/v1/chat/completions"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	if c.GetAPIKey() != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.GetAPIKey())
	}

	return httpReq, nil
}

// processStream processa lo stream SSE
func (c *Client) processStream(body io.Reader, handler providers.StreamHandler) error {
	scanner := bufio.NewScanner(body)

	// Accumula tool calls parziali
	toolCallsBuilder := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse SSE format: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			// Send final chunk
			return handler(&providers.StreamChunk{
				Done: true,
			})
		}

		// Parse JSON chunk
		var streamResp ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			log.Warn().Err(err).Str("data", data).Msg("Failed to parse stream chunk")
			continue
		}

		// Convert to generic chunk
		chunk := c.convertStreamChunk(&streamResp, toolCallsBuilder)

		// Call handler
		if err := handler(chunk); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

// convertStreamChunk converte un chunk OpenAI in formato generico
func (c *Client) convertStreamChunk(resp *ChatCompletionStreamResponse, toolCallsBuilder map[int]*ToolCall) *providers.StreamChunk {
	if len(resp.Choices) == 0 {
		return &providers.StreamChunk{Done: false}
	}

	choice := resp.Choices[0]
	delta := choice.Delta

	chunk := &providers.StreamChunk{
		FinishReason: choice.FinishReason,
		Done:         choice.FinishReason != "",
	}

	// Extract content
	if delta.Content != nil {
		if content, ok := delta.Content.(string); ok {
			chunk.Delta = content
		}
	}

	// Handle tool calls (accumulate deltas)
	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			index := 0
			if tc.Index != nil {
				index = *tc.Index
			}

			// Initialize or update tool call
			if _, exists := toolCallsBuilder[index]; !exists {
				toolCallsBuilder[index] = &ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			} else {
				// Append to existing
				if tc.Function.Name != "" {
					toolCallsBuilder[index].Function.Name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCallsBuilder[index].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		// Convert to provider format
		chunk.ToolCalls = make([]providers.ToolCall, 0, len(toolCallsBuilder))
		for _, tc := range toolCallsBuilder {
			chunk.ToolCalls = append(chunk.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: providers.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	// Add usage if present (final chunk)
	if resp.Usage != nil {
		chunk.Usage = &providers.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return chunk
}

// HealthCheck verifica lo stato del provider
func (c *Client) HealthCheck(ctx context.Context) error {
	var result ModelsResponse
	var errResp ErrorResponse

	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetError(&errResp).
		Get("/v1/models")

	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.IsError() {
		return c.handleErrorResponse(resp.StatusCode(), &errResp)
	}

	return nil
}

// GetModels restituisce la lista dei modelli disponibili
func (c *Client) GetModels(ctx context.Context) ([]providers.ModelInfo, error) {
	var result ModelsResponse
	var errResp ErrorResponse

	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetResult(&result).
		SetError(&errResp).
		Get("/v1/models")

	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	if resp.IsError() {
		return nil, c.handleErrorResponse(resp.StatusCode(), &errResp)
	}

	// Convert to generic format
	models := make([]providers.ModelInfo, len(result.Data))
	for i, model := range result.Data {
		models[i] = providers.ModelInfo{
			ID:       model.ID,
			Name:     model.ID,
			Provider: c.Name(),
			Capabilities: map[string]bool{
				"streaming": c.SupportsFeature(providers.FeatureStreaming),
				"tools":     c.SupportsFeature(providers.FeatureTools),
				"json_mode": c.SupportsFeature(providers.FeatureJSONMode),
				"vision":    c.SupportsFeature(providers.FeatureVision),
			},
		}
	}

	return models, nil
}

// convertToOpenAIRequest converte una richiesta generica in formato OpenAI
func (c *Client) convertToOpenAIRequest(req *providers.ChatRequest) *ChatCompletionRequest {
	openaiReq := &ChatCompletionRequest{
		Model:            req.Model,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		MaxTokens:        req.MaxTokens,
		Stream:           req.Stream,
		Stop:             req.Stop,
		PresencePenalty:  req.PresencePenalty,
		FrequencyPenalty: req.FrequencyPenalty,
		User:             req.User,
		Seed:             req.Seed,
	}

	// Convert messages
	openaiReq.Messages = make([]ChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		openaiReq.Messages[i] = ChatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}

		// Convert tool calls
		if len(msg.ToolCalls) > 0 {
			openaiReq.Messages[i].ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				openaiReq.Messages[i].ToolCalls[j] = ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		openaiReq.Tools = make([]Tool, len(req.Tools))
		for i, tool := range req.Tools {
			openaiReq.Tools[i] = Tool{
				Type: tool.Type,
				Function: Function{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
		openaiReq.ToolChoice = req.ToolChoice
	}

	// Convert response format
	if req.ResponseFormat != nil {
		openaiReq.ResponseFormat = &ResponseFormat{
			Type: req.ResponseFormat.Type,
		}
	}

	return openaiReq
}

// convertFromOpenAIResponse converte una risposta OpenAI in formato generico
func (c *Client) convertFromOpenAIResponse(resp *ChatCompletionResponse) *providers.ChatResponse {
	choices := make([]providers.Choice, len(resp.Choices))
	for i, choice := range resp.Choices {
		msg := providers.Message{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
			Name:    choice.Message.Name,
		}

		// Convert tool calls
		if len(choice.Message.ToolCalls) > 0 {
			msg.ToolCalls = make([]providers.ToolCall, len(choice.Message.ToolCalls))
			for j, tc := range choice.Message.ToolCalls {
				msg.ToolCalls[j] = providers.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: providers.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		choices[i] = providers.Choice{
			Index:        choice.Index,
			Message:      msg,
			FinishReason: choice.FinishReason,
		}
	}

	return &providers.ChatResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: providers.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		SystemFingerprint: resp.SystemFingerprint,
	}
}

// handleErrorResponse gestisce gli errori dalla risposta API
func (c *Client) handleErrorResponse(statusCode int, errResp *ErrorResponse) error {
	if errResp.Error.Message == "" {
		return fmt.Errorf("API error: status %d", statusCode)
	}

	baseErr := fmt.Errorf("%s (type: %s)", errResp.Error.Message, errResp.Error.Type)

	switch statusCode {
	case 401:
		return fmt.Errorf("%w: %v", ErrInvalidAPIKey, baseErr)
	case 429:
		return fmt.Errorf("%w: %v", ErrRateLimitExceeded, baseErr)
	case 404:
		return fmt.Errorf("%w: %v", ErrModelNotFound, baseErr)
	case 400:
		return fmt.Errorf("%w: %v", ErrInvalidRequest, baseErr)
	case 503:
		return fmt.Errorf("%w: %v", ErrServiceUnavailable, baseErr)
	default:
		return baseErr
	}
}

// handleStreamError gestisce gli errori nello streaming
func (c *Client) handleStreamError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("stream error: status %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("stream error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return c.handleErrorResponse(resp.StatusCode, &errResp)
}
