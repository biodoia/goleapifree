package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client è il client per l'API Anthropic Claude
type Client struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
	userAgent  string
}

// ClientOption è un'opzione per configurare il client
type ClientOption func(*Client)

// WithBaseURL imposta l'URL base personalizzato
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithAPIVersion imposta la versione dell'API
func WithAPIVersion(version string) ClientOption {
	return func(c *Client) {
		c.apiVersion = version
	}
}

// WithHTTPClient imposta un client HTTP personalizzato
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithUserAgent imposta un user agent personalizzato
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// NewClient crea un nuovo client Anthropic
func NewClient(apiKey string, opts ...ClientOption) *Client {
	client := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		userAgent: "GoLeapAI/1.0",
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// CreateMessage invia una richiesta all'endpoint /v1/messages
func (c *Client) CreateMessage(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	// Valida la richiesta
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Prepara il body JSON
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Crea la richiesta HTTP
	httpReq, err := c.newRequest(ctx, "POST", "/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	// Esegui la richiesta
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// Leggi il rate limit info
	rateLimitInfo := c.parseRateLimitHeaders(httpResp.Header)
	if rateLimitInfo != nil {
		log.Debug().
			Int("requests_remaining", rateLimitInfo.RequestsRemaining).
			Int("tokens_remaining", rateLimitInfo.TokensRemaining).
			Msg("Rate limit info")
	}

	// Gestisci errori HTTP
	if httpResp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(httpResp)
	}

	// Decodifica la risposta
	var resp MessagesResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// CreateMessageStream invia una richiesta streaming all'endpoint /v1/messages
func (c *Client) CreateMessageStream(ctx context.Context, req *MessagesRequest) (<-chan StreamEvent, <-chan error) {
	eventCh := make(chan StreamEvent, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		// Forza streaming
		req.Stream = true

		// Valida la richiesta
		if err := req.Validate(); err != nil {
			errCh <- fmt.Errorf("invalid request: %w", err)
			return
		}

		// Prepara il body JSON
		bodyBytes, err := json.Marshal(req)
		if err != nil {
			errCh <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		// Crea la richiesta HTTP
		httpReq, err := c.newRequest(ctx, "POST", "/v1/messages", bytes.NewReader(bodyBytes))
		if err != nil {
			errCh <- err
			return
		}

		// Imposta header per streaming
		httpReq.Header.Set("Accept", "text/event-stream")

		// Esegui la richiesta
		httpResp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("failed to execute request: %w", err)
			return
		}
		defer httpResp.Body.Close()

		// Gestisci errori HTTP
		if httpResp.StatusCode != http.StatusOK {
			errCh <- c.handleErrorResponse(httpResp)
			return
		}

		// Processa gli eventi SSE
		if err := c.processStreamEvents(ctx, httpResp.Body, eventCh); err != nil {
			errCh <- err
			return
		}
	}()

	return eventCh, errCh
}

// processStreamEvents processa gli eventi Server-Sent Events
func (c *Client) processStreamEvents(ctx context.Context, body io.Reader, eventCh chan<- StreamEvent) error {
	scanner := bufio.NewScanner(body)
	var eventType string
	var eventData strings.Builder

	for scanner.Scan() {
		// Controlla cancellazione del contesto
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Linea vuota = fine evento
		if line == "" {
			if eventType != "" && eventData.Len() > 0 {
				if err := c.parseAndSendEvent(eventType, eventData.String(), eventCh); err != nil {
					log.Error().Err(err).Str("event_type", eventType).Msg("Failed to parse stream event")
				}
			}
			eventType = ""
			eventData.Reset()
			continue
		}

		// Parse SSE line
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			eventData.WriteString(data)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// parseAndSendEvent decodifica e invia un evento di streaming
func (c *Client) parseAndSendEvent(eventType, data string, eventCh chan<- StreamEvent) error {
	var event StreamEvent

	switch StreamEventType(eventType) {
	case StreamEventMessageStart:
		var msgStart struct {
			Type    string           `json:"type"`
			Message MessagesResponse `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &msgStart); err != nil {
			return err
		}
		event = StreamEvent{
			Type:    msgStart.Type,
			Message: &msgStart.Message,
		}

	case StreamEventContentBlockStart:
		var blockStart struct {
			Type         string       `json:"type"`
			Index        int          `json:"index"`
			ContentBlock ContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &blockStart); err != nil {
			return err
		}
		event = StreamEvent{
			Type:         blockStart.Type,
			Index:        blockStart.Index,
			ContentBlock: &blockStart.ContentBlock,
		}

	case StreamEventContentBlockDelta:
		var blockDelta struct {
			Type  string      `json:"type"`
			Index int         `json:"index"`
			Delta StreamDelta `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &blockDelta); err != nil {
			return err
		}
		event = StreamEvent{
			Type:  blockDelta.Type,
			Index: blockDelta.Index,
			Delta: &blockDelta.Delta,
		}

	case StreamEventContentBlockStop:
		var blockStop struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
		}
		if err := json.Unmarshal([]byte(data), &blockStop); err != nil {
			return err
		}
		event = StreamEvent{
			Type:  blockStop.Type,
			Index: blockStop.Index,
		}

	case StreamEventMessageDelta:
		var msgDelta struct {
			Type  string      `json:"type"`
			Delta StreamDelta `json:"delta"`
			Usage Usage       `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &msgDelta); err != nil {
			return err
		}
		event = StreamEvent{
			Type:  msgDelta.Type,
			Delta: &msgDelta.Delta,
			Usage: &msgDelta.Usage,
		}

	case StreamEventMessageStop:
		event = StreamEvent{
			Type: string(StreamEventMessageStop),
		}

	case StreamEventPing:
		// Ignora ping events
		return nil

	case StreamEventError:
		var errResp ErrorResponse
		if err := json.Unmarshal([]byte(data), &errResp); err != nil {
			return err
		}
		return &errResp.Error

	default:
		log.Debug().Str("event_type", eventType).Msg("Unknown stream event type")
		return nil
	}

	eventCh <- event
	return nil
}

// newRequest crea una nuova richiesta HTTP
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Headers obbligatori
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Anthropic-Version", c.apiVersion)
	req.Header.Set("User-Agent", c.userAgent)

	return req, nil
}

// handleErrorResponse gestisce le risposte di errore
func (c *Client) handleErrorResponse(resp *http.Response) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response: %w", err)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return &errResp.Error
}

// parseRateLimitHeaders estrae informazioni sul rate limit dagli headers
func (c *Client) parseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	info := &RateLimitInfo{}

	// Requests
	if v := headers.Get("anthropic-ratelimit-requests-limit"); v != "" {
		info.RequestsLimit, _ = strconv.Atoi(v)
	}
	if v := headers.Get("anthropic-ratelimit-requests-remaining"); v != "" {
		info.RequestsRemaining, _ = strconv.Atoi(v)
	}
	if v := headers.Get("anthropic-ratelimit-requests-reset"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			info.RequestsReset = t
		}
	}

	// Tokens
	if v := headers.Get("anthropic-ratelimit-tokens-limit"); v != "" {
		info.TokensLimit, _ = strconv.Atoi(v)
	}
	if v := headers.Get("anthropic-ratelimit-tokens-remaining"); v != "" {
		info.TokensRemaining, _ = strconv.Atoi(v)
	}
	if v := headers.Get("anthropic-ratelimit-tokens-reset"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			info.TokensReset = t
		}
	}

	// Retry-After
	if v := headers.Get("retry-after"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	return info
}

// CountTokens conta approssimativamente i token in una richiesta
// Nota: questa è una stima approssimativa. Per un conteggio preciso,
// usare l'API ufficiale di tokenization di Anthropic
func (c *Client) CountTokens(req *MessagesRequest) int {
	totalTokens := 0

	// System prompt
	if req.System != "" {
		totalTokens += estimateTokens(req.System)
	}

	// Messages
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == ContentBlockTypeText {
				totalTokens += estimateTokens(block.Text)
			}
		}
	}

	// Overhead per messaggi e struttura
	totalTokens += len(req.Messages) * 10

	return totalTokens
}

// estimateTokens stima il numero di token in un testo
// Approssimazione: 1 token ~= 4 caratteri
func estimateTokens(text string) int {
	return len(text) / 4
}

// Health verifica la salute dell'API
func (c *Client) Health(ctx context.Context) error {
	// Crea una richiesta minima per verificare la connettività
	req := &MessagesRequest{
		Model:     ModelClaude35Haiku,
		MaxTokens: 10,
		Messages: []Message{
			{
				Role:    MessageRoleUser,
				Content: []ContentBlock{NewTextContentBlock("Hi")},
			},
		},
	}

	_, err := c.CreateMessage(ctx, req)
	return err
}
