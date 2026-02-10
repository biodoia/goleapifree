package providers

import (
	"context"
	"time"
)

// Provider è l'interfaccia base per tutti i provider LLM
type Provider interface {
	// ChatCompletion esegue una richiesta di chat completion
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Stream esegue una richiesta di chat completion con streaming
	Stream(ctx context.Context, req *ChatRequest, handler StreamHandler) error

	// Name restituisce il nome del provider
	Name() string

	// HealthCheck verifica lo stato di salute del provider
	HealthCheck(ctx context.Context) error

	// GetModels restituisce la lista dei modelli disponibili
	GetModels(ctx context.Context) ([]ModelInfo, error)

	// SupportsFeature verifica se il provider supporta una specifica feature
	SupportsFeature(feature Feature) bool
}

// Feature rappresenta una caratteristica del provider
type Feature string

const (
	FeatureStreaming    Feature = "streaming"
	FeatureTools        Feature = "tools"
	FeatureJSONMode     Feature = "json_mode"
	FeatureVision       Feature = "vision"
	FeatureFunctionCall Feature = "function_call"
	FeatureSystemMsg    Feature = "system_message"
)

// StreamHandler è la callback per gestire eventi di streaming
type StreamHandler func(chunk *StreamChunk) error

// StreamChunk rappresenta un chunk di risposta streaming
type StreamChunk struct {
	Delta         string                 // Contenuto incrementale
	FinishReason  string                 // Motivo di fine stream
	ToolCalls     []ToolCall            // Tool calls parziali
	Done          bool                   // Se true, lo stream è terminato
	Usage         *Usage                // Usage finale (solo nell'ultimo chunk)
	Metadata      map[string]interface{} // Metadati aggiuntivi
}

// ChatRequest rappresenta una richiesta generica di chat completion
type ChatRequest struct {
	Model            string                 `json:"model"`
	Messages         []Message              `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`

	// Tool calling
	Tools               []Tool                 `json:"tools,omitempty"`
	ToolChoice          interface{}            `json:"tool_choice,omitempty"`

	// JSON mode
	ResponseFormat      *ResponseFormat        `json:"response_format,omitempty"`

	// Metadata
	User                string                 `json:"user,omitempty"`
	Seed                *int                   `json:"seed,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// ChatResponse rappresenta una risposta generica di chat completion
type ChatResponse struct {
	ID               string                 `json:"id"`
	Object           string                 `json:"object"`
	Created          int64                  `json:"created"`
	Model            string                 `json:"model"`
	Choices          []Choice               `json:"choices"`
	Usage            Usage                  `json:"usage"`
	SystemFingerprint string                `json:"system_fingerprint,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// Message rappresenta un messaggio nella conversazione
type Message struct {
	Role       string      `json:"role"`       // system, user, assistant, tool
	Content    interface{} `json:"content"`    // string o array per multimodal
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// Choice rappresenta una scelta nella risposta
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Logprobs     *Logprobs `json:"logprobs,omitempty"`
}

// Usage rappresenta le statistiche di utilizzo
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Tool rappresenta uno strumento disponibile
type Tool struct {
	Type     string   `json:"type"` // "function"
	Function Function `json:"function"`
}

// Function rappresenta una funzione callable
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall rappresenta una chiamata a uno strumento
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall rappresenta una chiamata a funzione
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ResponseFormat specifica il formato della risposta
type ResponseFormat struct {
	Type string `json:"type"` // "text" o "json_object"
}

// Logprobs rappresenta le log probabilities
type Logprobs struct {
	Content []TokenLogprob `json:"content,omitempty"`
}

// TokenLogprob rappresenta la log probability di un token
type TokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

// ModelInfo contiene informazioni su un modello
type ModelInfo struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Provider      string                 `json:"provider"`
	ContextLength int                    `json:"context_length"`
	MaxTokens     int                    `json:"max_tokens"`
	Capabilities  map[string]bool        `json:"capabilities"`
	Pricing       *PricingInfo           `json:"pricing,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// PricingInfo contiene informazioni sui prezzi
type PricingInfo struct {
	InputPer1K  float64 `json:"input_per_1k"`
	OutputPer1K float64 `json:"output_per_1k"`
	Currency    string  `json:"currency"`
}

// BaseProvider fornisce funzionalità comuni per i provider
type BaseProvider struct {
	name         string
	baseURL      string
	apiKey       string
	timeout      time.Duration
	maxRetries   int
	capabilities map[Feature]bool
}

// NewBaseProvider crea un nuovo BaseProvider
func NewBaseProvider(name, baseURL, apiKey string) *BaseProvider {
	return &BaseProvider{
		name:       name,
		baseURL:    baseURL,
		apiKey:     apiKey,
		timeout:    30 * time.Second,
		maxRetries: 3,
		capabilities: map[Feature]bool{
			FeatureStreaming:    true,
			FeatureTools:        false,
			FeatureJSONMode:     true,
			FeatureVision:       false,
			FeatureFunctionCall: false,
			FeatureSystemMsg:    true,
		},
	}
}

// Name restituisce il nome del provider
func (b *BaseProvider) Name() string {
	return b.name
}

// SupportsFeature verifica se il provider supporta una feature
func (b *BaseProvider) SupportsFeature(feature Feature) bool {
	supported, exists := b.capabilities[feature]
	return exists && supported
}

// SetFeature imposta il supporto per una feature
func (b *BaseProvider) SetFeature(feature Feature, supported bool) {
	b.capabilities[feature] = supported
}

// SetTimeout imposta il timeout delle richieste
func (b *BaseProvider) SetTimeout(timeout time.Duration) {
	b.timeout = timeout
}

// SetMaxRetries imposta il numero massimo di retry
func (b *BaseProvider) SetMaxRetries(retries int) {
	b.maxRetries = retries
}

// GetBaseURL restituisce la base URL
func (b *BaseProvider) GetBaseURL() string {
	return b.baseURL
}

// GetAPIKey restituisce la API key
func (b *BaseProvider) GetAPIKey() string {
	return b.apiKey
}

// GetTimeout restituisce il timeout
func (b *BaseProvider) GetTimeout() time.Duration {
	return b.timeout
}

// GetMaxRetries restituisce il numero massimo di retry
func (b *BaseProvider) GetMaxRetries() int {
	return b.maxRetries
}
