package openai

// OpenAI API Types - Compatibili con OpenAI API standard

// ChatCompletionRequest rappresenta una richiesta OpenAI API
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64     `json:"logit_bias,omitempty"`
	User             string                 `json:"user,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`

	// Tool calling
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"` // "none", "auto", or {"type": "function", "function": {"name": "..."}}

	// Response format
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// Advanced parameters
	LogProbs       *bool `json:"logprobs,omitempty"`
	TopLogProbs    *int  `json:"top_logprobs,omitempty"`
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
}

// ChatCompletionResponse rappresenta una risposta OpenAI API
type ChatCompletionResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []Choice       `json:"choices"`
	Usage             Usage          `json:"usage"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// ChatCompletionStreamResponse rappresenta un chunk di streaming
type ChatCompletionStreamResponse struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	SystemFingerprint string        `json:"system_fingerprint,omitempty"`
	Usage             *Usage        `json:"usage,omitempty"` // Solo nell'ultimo chunk con stream_options
}

// ChatMessage rappresenta un messaggio nella conversazione
type ChatMessage struct {
	Role       string        `json:"role"` // system, user, assistant, tool
	Content    interface{}   `json:"content"` // string o ContentPart[]
	Name       string        `json:"name,omitempty"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// ContentPart rappresenta una parte di contenuto multimodale
type ContentPart struct {
	Type     string        `json:"type"` // "text" o "image_url"
	Text     string        `json:"text,omitempty"`
	ImageURL *ImageURL     `json:"image_url,omitempty"`
}

// ImageURL rappresenta un'immagine
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// Choice rappresenta una scelta nella risposta
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"` // "stop", "length", "tool_calls", "content_filter"
	Logprobs     *LogProbs   `json:"logprobs,omitempty"`
}

// StreamChoice rappresenta una scelta nello streaming
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Logprobs     *LogProbs   `json:"logprobs,omitempty"`
}

// Usage rappresenta le statistiche di utilizzo token
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Tool rappresenta uno strumento disponibile
type Tool struct {
	Type     string   `json:"type"` // sempre "function"
	Function Function `json:"function"`
}

// Function rappresenta una funzione callable
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"` // JSON Schema
}

// ToolCall rappresenta una chiamata a uno strumento
type ToolCall struct {
	Index    *int         `json:"index,omitempty"` // Solo nello streaming
	ID       string       `json:"id"`
	Type     string       `json:"type"` // sempre "function"
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

// LogProbs rappresenta le log probabilities
type LogProbs struct {
	Content []TokenLogProb `json:"content,omitempty"`
}

// TokenLogProb rappresenta la log probability di un token
type TokenLogProb struct {
	Token       string      `json:"token"`
	Logprob     float64     `json:"logprob"`
	Bytes       []int       `json:"bytes,omitempty"`
	TopLogprobs []TopLogProb `json:"top_logprobs,omitempty"`
}

// TopLogProb rappresenta una top log probability
type TopLogProb struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// ModelsResponse rappresenta la lista di modelli disponibili
type ModelsResponse struct {
	Object string       `json:"object"`
	Data   []ModelData  `json:"data"`
}

// ModelData rappresenta i dati di un modello
type ModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ErrorResponse rappresenta un errore dall'API
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contiene i dettagli dell'errore
type ErrorDetail struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   string      `json:"param,omitempty"`
	Code    interface{} `json:"code,omitempty"` // può essere string o int
}

// StreamOptions configura lo streaming
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}
