package anthropic

import (
	"encoding/json"
	"time"
)

// ContentBlockType definisce i tipi di blocco di contenuto
type ContentBlockType string

const (
	ContentBlockTypeText       ContentBlockType = "text"
	ContentBlockTypeImage      ContentBlockType = "image"
	ContentBlockTypeToolUse    ContentBlockType = "tool_use"
	ContentBlockTypeToolResult ContentBlockType = "tool_result"
)

// MessageRole definisce i ruoli dei messaggi
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

// StopReason indica perché il modello ha smesso di generare
type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
)

// MessagesRequest rappresenta una richiesta all'API Messages
type MessagesRequest struct {
	Model         string           `json:"model"`
	Messages      []Message        `json:"messages"`
	MaxTokens     int              `json:"max_tokens"`
	System        string           `json:"system,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	TopK          *int             `json:"top_k,omitempty"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	Stream        bool             `json:"stream,omitempty"`
	Metadata      *Metadata        `json:"metadata,omitempty"`
	Tools         []Tool           `json:"tools,omitempty"`
}

// Message rappresenta un messaggio nella conversazione
type Message struct {
	Role    MessageRole    `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock è un blocco di contenuto che può essere testo, immagine o tool use/result
type ContentBlock struct {
	Type ContentBlockType `json:"type"`

	// Per type="text"
	Text string `json:"text,omitempty"`

	// Per type="image"
	Source *ImageSource `json:"source,omitempty"`

	// Per type="tool_use"
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Per type="tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ImageSource rappresenta una sorgente di immagine
type ImageSource struct {
	Type      string `json:"type"` // "base64" o "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Tool rappresenta una tool disponibile per il modello
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Metadata contiene metadati opzionali per la richiesta
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// MessagesResponse rappresenta la risposta dall'API Messages
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // "message"
	Role         MessageRole    `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   StopReason     `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage contiene informazioni sull'utilizzo dei token
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent rappresenta un evento di streaming
type StreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta *StreamDelta    `json:"delta,omitempty"`
	Message *MessagesResponse `json:"message,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
	Usage *Usage          `json:"usage,omitempty"`
}

// StreamDelta contiene aggiornamenti incrementali durante lo streaming
type StreamDelta struct {
	Type       ContentBlockType `json:"type,omitempty"`
	Text       string           `json:"text,omitempty"`
	StopReason StopReason       `json:"stop_reason,omitempty"`
}

// ErrorResponse rappresenta una risposta di errore dall'API
type ErrorResponse struct {
	Type  string `json:"type"` // "error"
	Error Error  `json:"error"`
}

// Error contiene i dettagli dell'errore
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamEventType definisce i tipi di eventi di streaming
type StreamEventType string

const (
	StreamEventMessageStart        StreamEventType = "message_start"
	StreamEventMessageDelta        StreamEventType = "message_delta"
	StreamEventMessageStop         StreamEventType = "message_stop"
	StreamEventContentBlockStart   StreamEventType = "content_block_start"
	StreamEventContentBlockDelta   StreamEventType = "content_block_delta"
	StreamEventContentBlockStop    StreamEventType = "content_block_stop"
	StreamEventPing                StreamEventType = "ping"
	StreamEventError               StreamEventType = "error"
)

// Anthropic API error types
const (
	ErrorTypeInvalidRequest     = "invalid_request_error"
	ErrorTypeAuthentication     = "authentication_error"
	ErrorTypePermission         = "permission_error"
	ErrorTypeNotFound           = "not_found_error"
	ErrorTypeRateLimit          = "rate_limit_error"
	ErrorTypeAPIError           = "api_error"
	ErrorTypeOverloaded         = "overloaded_error"
)

// Anthropic API constants
const (
	DefaultAPIVersion  = "2023-06-01"
	DefaultBaseURL     = "https://api.anthropic.com"
	DefaultMaxTokens   = 4096
	DefaultTemperature = 1.0
	DefaultTopP        = 0.999
	DefaultTopK        = 250
)

// Anthropic Models
const (
	ModelClaude3Opus     = "claude-3-opus-20240229"
	ModelClaude3Sonnet   = "claude-3-sonnet-20240229"
	ModelClaude3Haiku    = "claude-3-haiku-20240307"
	ModelClaude35Sonnet  = "claude-3-5-sonnet-20241022"
	ModelClaude35Haiku   = "claude-3-5-haiku-20241022"
)

// NewTextContentBlock crea un nuovo blocco di contenuto testuale
func NewTextContentBlock(text string) ContentBlock {
	return ContentBlock{
		Type: ContentBlockTypeText,
		Text: text,
	}
}

// NewImageContentBlock crea un nuovo blocco di contenuto immagine
func NewImageContentBlock(mediaType, data string) ContentBlock {
	return ContentBlock{
		Type: ContentBlockTypeImage,
		Source: &ImageSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      data,
		},
	}
}

// NewToolUseContentBlock crea un nuovo blocco di tool use
func NewToolUseContentBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{
		Type:  ContentBlockTypeToolUse,
		ID:    id,
		Name:  name,
		Input: input,
	}
}

// NewToolResultContentBlock crea un nuovo blocco di tool result
func NewToolResultContentBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{
		Type:      ContentBlockTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// GetText estrae il testo dal primo blocco di contenuto testuale
func (m *MessagesResponse) GetText() string {
	for _, block := range m.Content {
		if block.Type == ContentBlockTypeText {
			return block.Text
		}
	}
	return ""
}

// GetAllText concatena tutto il testo dai blocchi di contenuto
func (m *MessagesResponse) GetAllText() string {
	var text string
	for _, block := range m.Content {
		if block.Type == ContentBlockTypeText {
			text += block.Text
		}
	}
	return text
}

// HasToolUse verifica se la risposta contiene tool use
func (m *MessagesResponse) HasToolUse() bool {
	for _, block := range m.Content {
		if block.Type == ContentBlockTypeToolUse {
			return true
		}
	}
	return false
}

// GetToolUses restituisce tutti i blocchi di tool use
func (m *MessagesResponse) GetToolUses() []ContentBlock {
	var toolUses []ContentBlock
	for _, block := range m.Content {
		if block.Type == ContentBlockTypeToolUse {
			toolUses = append(toolUses, block)
		}
	}
	return toolUses
}

// Validate valida la richiesta
func (r *MessagesRequest) Validate() error {
	if r.Model == "" {
		return &Error{Type: ErrorTypeInvalidRequest, Message: "model is required"}
	}
	if len(r.Messages) == 0 {
		return &Error{Type: ErrorTypeInvalidRequest, Message: "messages is required"}
	}
	if r.MaxTokens <= 0 {
		return &Error{Type: ErrorTypeInvalidRequest, Message: "max_tokens must be positive"}
	}
	if r.MaxTokens > 200000 {
		return &Error{Type: ErrorTypeInvalidRequest, Message: "max_tokens exceeds maximum"}
	}

	// Valida alternanza ruoli
	for i, msg := range r.Messages {
		if msg.Role != MessageRoleUser && msg.Role != MessageRoleAssistant {
			return &Error{
				Type:    ErrorTypeInvalidRequest,
				Message: "message role must be 'user' or 'assistant'",
			}
		}

		// Il primo messaggio deve essere user
		if i == 0 && msg.Role != MessageRoleUser {
			return &Error{
				Type:    ErrorTypeInvalidRequest,
				Message: "first message must have role 'user'",
			}
		}

		if len(msg.Content) == 0 {
			return &Error{
				Type:    ErrorTypeInvalidRequest,
				Message: "message content cannot be empty",
			}
		}
	}

	return nil
}

// Error implementa l'interfaccia error
func (e *Error) Error() string {
	return e.Message
}

// IsRateLimitError verifica se l'errore è di rate limit
func (e *Error) IsRateLimitError() bool {
	return e.Type == ErrorTypeRateLimit
}

// IsAuthError verifica se l'errore è di autenticazione
func (e *Error) IsAuthError() bool {
	return e.Type == ErrorTypeAuthentication || e.Type == ErrorTypePermission
}

// IsRetryable verifica se l'errore è ritentabile
func (e *Error) IsRetryable() bool {
	return e.Type == ErrorTypeRateLimit ||
	       e.Type == ErrorTypeOverloaded ||
	       e.Type == ErrorTypeAPIError
}

// RateLimitInfo contiene informazioni sul rate limiting
type RateLimitInfo struct {
	RequestsLimit     int
	RequestsRemaining int
	RequestsReset     time.Time
	TokensLimit       int
	TokensRemaining   int
	TokensReset       time.Time
	RetryAfter        time.Duration
}
