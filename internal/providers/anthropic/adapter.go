package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAI-compatible types per l'adapter

// OpenAIRequest rappresenta una richiesta OpenAI-compatible
type OpenAIRequest struct {
	Model            string                 `json:"model"`
	Messages         []OpenAIMessage        `json:"messages"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                int                    `json:"n,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"` // string o []string
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64     `json:"logit_bias,omitempty"`
	User             string                 `json:"user,omitempty"`
	Tools            []OpenAITool           `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat   *OpenAIResponseFormat  `json:"response_format,omitempty"`
}

// OpenAIMessage rappresenta un messaggio OpenAI
type OpenAIMessage struct {
	Role         string                 `json:"role"`
	Content      interface{}            `json:"content"` // string o []OpenAIContent
	Name         string                 `json:"name,omitempty"`
	ToolCalls    []OpenAIToolCall       `json:"tool_calls,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
}

// OpenAIContent rappresenta contenuto multimodale OpenAI
type OpenAIContent struct {
	Type     string            `json:"type"` // "text" o "image_url"
	Text     string            `json:"text,omitempty"`
	ImageURL *OpenAIImageURL   `json:"image_url,omitempty"`
}

// OpenAIImageURL rappresenta un'immagine OpenAI
type OpenAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// OpenAITool rappresenta una tool OpenAI
type OpenAITool struct {
	Type     string              `json:"type"` // "function"
	Function OpenAIFunctionDef   `json:"function"`
}

// OpenAIFunctionDef definisce una funzione
type OpenAIFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// OpenAIToolCall rappresenta una chiamata a tool
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIFunctionCall rappresenta una chiamata a funzione
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenAIResponseFormat specifica il formato della risposta
type OpenAIResponseFormat struct {
	Type string `json:"type"` // "text" o "json_object"
}

// OpenAIResponse rappresenta una risposta OpenAI-compatible
type OpenAIResponse struct {
	ID                string                `json:"id"`
	Object            string                `json:"object"` // "chat.completion"
	Created           int64                 `json:"created"`
	Model             string                `json:"model"`
	Choices           []OpenAIChoice        `json:"choices"`
	Usage             OpenAIUsage           `json:"usage"`
	SystemFingerprint string                `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice rappresenta una scelta nella risposta
type OpenAIChoice struct {
	Index        int            `json:"index"`
	Message      OpenAIMessage  `json:"message"`
	FinishReason string         `json:"finish_reason"`
	LogProbs     interface{}    `json:"logprobs,omitempty"`
}

// OpenAIUsage contiene informazioni sull'utilizzo
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk rappresenta un chunk di streaming OpenAI
type OpenAIStreamChunk struct {
	ID                string                `json:"id"`
	Object            string                `json:"object"` // "chat.completion.chunk"
	Created           int64                 `json:"created"`
	Model             string                `json:"model"`
	Choices           []OpenAIStreamChoice  `json:"choices"`
	SystemFingerprint string                `json:"system_fingerprint,omitempty"`
}

// OpenAIStreamChoice rappresenta una scelta in streaming
type OpenAIStreamChoice struct {
	Index        int                  `json:"index"`
	Delta        OpenAIDelta          `json:"delta"`
	FinishReason *string              `json:"finish_reason"`
	LogProbs     interface{}          `json:"logprobs,omitempty"`
}

// OpenAIDelta rappresenta un aggiornamento incrementale
type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// Adapter converte richieste e risposte tra formato OpenAI e Anthropic
type Adapter struct {
	// Mapping dei modelli OpenAI -> Anthropic
	modelMapping map[string]string
}

// NewAdapter crea un nuovo adapter
func NewAdapter() *Adapter {
	return &Adapter{
		modelMapping: map[string]string{
			"gpt-4":             ModelClaude3Opus,
			"gpt-4-turbo":       ModelClaude35Sonnet,
			"gpt-4o":            ModelClaude35Sonnet,
			"gpt-3.5-turbo":     ModelClaude35Haiku,
			"claude-3-opus":     ModelClaude3Opus,
			"claude-3-sonnet":   ModelClaude3Sonnet,
			"claude-3-haiku":    ModelClaude3Haiku,
			"claude-3.5-sonnet": ModelClaude35Sonnet,
			"claude-3.5-haiku":  ModelClaude35Haiku,
		},
	}
}

// ConvertRequest converte una richiesta OpenAI in formato Anthropic
func (a *Adapter) ConvertRequest(openaiReq *OpenAIRequest) (*MessagesRequest, error) {
	// Map del modello
	model := a.mapModel(openaiReq.Model)

	// Estrai system message e converti messaggi
	systemPrompt, messages, err := a.convertMessages(openaiReq.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Prepara la richiesta Anthropic
	req := &MessagesRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   openaiReq.MaxTokens,
		System:      systemPrompt,
		Temperature: openaiReq.Temperature,
		TopP:        openaiReq.TopP,
		Stream:      openaiReq.Stream,
	}

	// Default max_tokens se non specificato
	if req.MaxTokens == 0 {
		req.MaxTokens = DefaultMaxTokens
	}

	// Converti stop sequences
	if openaiReq.Stop != nil {
		req.StopSequences = a.convertStopSequences(openaiReq.Stop)
	}

	// Converti tools
	if len(openaiReq.Tools) > 0 {
		req.Tools, err = a.convertTools(openaiReq.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
	}

	// Metadata
	if openaiReq.User != "" {
		req.Metadata = &Metadata{
			UserID: openaiReq.User,
		}
	}

	return req, nil
}

// ConvertResponse converte una risposta Anthropic in formato OpenAI
func (a *Adapter) ConvertResponse(anthropicResp *MessagesResponse) *OpenAIResponse {
	// Converti content in messaggio OpenAI
	message := a.convertContentToMessage(anthropicResp.Content)

	// Map finish reason
	finishReason := a.mapFinishReason(anthropicResp.StopReason)

	return &OpenAIResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: 0, // Anthropic non fornisce timestamp
		Model:   anthropicResp.Model,
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: OpenAIUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}
}

// ConvertStreamEvent converte un evento di streaming Anthropic in formato OpenAI
func (a *Adapter) ConvertStreamEvent(event StreamEvent) *OpenAIStreamChunk {
	chunk := &OpenAIStreamChunk{
		Object:  "chat.completion.chunk",
		Created: 0,
	}

	switch event.Type {
	case string(StreamEventMessageStart):
		if event.Message != nil {
			chunk.ID = event.Message.ID
			chunk.Model = event.Message.Model
			chunk.Choices = []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIDelta{
						Role: string(event.Message.Role),
					},
				},
			}
		}

	case string(StreamEventContentBlockDelta):
		if event.Delta != nil && event.Delta.Type == ContentBlockTypeText {
			chunk.Choices = []OpenAIStreamChoice{
				{
					Index: event.Index,
					Delta: OpenAIDelta{
						Content: event.Delta.Text,
					},
				},
			}
		}

	case string(StreamEventMessageDelta):
		if event.Delta != nil && event.Delta.StopReason != "" {
			finishReason := a.mapFinishReason(event.Delta.StopReason)
			chunk.Choices = []OpenAIStreamChoice{
				{
					Index:        0,
					Delta:        OpenAIDelta{},
					FinishReason: &finishReason,
				},
			}
		}

	case string(StreamEventMessageStop):
		finishReason := "stop"
		chunk.Choices = []OpenAIStreamChoice{
			{
				Index:        0,
				Delta:        OpenAIDelta{},
				FinishReason: &finishReason,
			},
		}
	}

	return chunk
}

// convertMessages converte messaggi OpenAI in formato Anthropic
// Restituisce: (system_prompt, messages, error)
func (a *Adapter) convertMessages(openaiMessages []OpenAIMessage) (string, []Message, error) {
	var systemPrompt string
	var messages []Message

	for _, msg := range openaiMessages {
		// Gestisci system messages separatamente
		if msg.Role == "system" {
			content, err := a.extractTextContent(msg.Content)
			if err != nil {
				return "", nil, err
			}
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += content
			continue
		}

		// Converti ruolo
		var role MessageRole
		switch msg.Role {
		case "user":
			role = MessageRoleUser
		case "assistant":
			role = MessageRoleAssistant
		case "tool":
			// I messaggi tool vengono convertiti in messaggi user con tool_result
			role = MessageRoleUser
		default:
			return "", nil, fmt.Errorf("unsupported role: %s", msg.Role)
		}

		// Converti content
		var contentBlocks []ContentBlock

		// Gestisci tool calls (assistant message)
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				contentBlocks = append(contentBlocks, NewToolUseContentBlock(
					tc.ID,
					tc.Function.Name,
					json.RawMessage(tc.Function.Arguments),
				))
			}
		}

		// Gestisci tool results
		if msg.ToolCallID != "" {
			content, err := a.extractTextContent(msg.Content)
			if err != nil {
				return "", nil, err
			}
			contentBlocks = append(contentBlocks, NewToolResultContentBlock(
				msg.ToolCallID,
				content,
				false,
			))
		}

		// Gestisci contenuto normale
		if len(contentBlocks) == 0 {
			blocks, err := a.convertContent(msg.Content)
			if err != nil {
				return "", nil, err
			}
			contentBlocks = blocks
		}

		messages = append(messages, Message{
			Role:    role,
			Content: contentBlocks,
		})
	}

	// Assicurati che il primo messaggio sia user
	if len(messages) > 0 && messages[0].Role != MessageRoleUser {
		return "", nil, fmt.Errorf("first message must be from user")
	}

	return systemPrompt, messages, nil
}

// convertContent converte il contenuto di un messaggio OpenAI
func (a *Adapter) convertContent(content interface{}) ([]ContentBlock, error) {
	switch v := content.(type) {
	case string:
		return []ContentBlock{NewTextContentBlock(v)}, nil

	case []interface{}:
		var blocks []ContentBlock
		for _, item := range v {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			contentType, _ := itemMap["type"].(string)
			switch contentType {
			case "text":
				if text, ok := itemMap["text"].(string); ok {
					blocks = append(blocks, NewTextContentBlock(text))
				}

			case "image_url":
				if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
					url, _ := imageURL["url"].(string)
					if strings.HasPrefix(url, "data:") {
						// Parse data URL: data:image/png;base64,xxxxx
						parts := strings.SplitN(url, ",", 2)
						if len(parts) == 2 {
							mediaType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
							blocks = append(blocks, NewImageContentBlock(mediaType, parts[1]))
						}
					}
				}
			}
		}
		return blocks, nil

	default:
		return nil, fmt.Errorf("unsupported content type: %T", content)
	}
}

// extractTextContent estrae il testo da un contenuto OpenAI
func (a *Adapter) extractTextContent(content interface{}) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil

	case []interface{}:
		var texts []string
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["type"] == "text" {
					if text, ok := itemMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, "\n"), nil

	default:
		return "", fmt.Errorf("unsupported content type: %T", content)
	}
}

// convertContentToMessage converte content blocks Anthropic in messaggio OpenAI
func (a *Adapter) convertContentToMessage(blocks []ContentBlock) OpenAIMessage {
	var content string
	var toolCalls []OpenAIToolCall

	for _, block := range blocks {
		switch block.Type {
		case ContentBlockTypeText:
			content += block.Text

		case ContentBlockTypeToolUse:
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: OpenAIFunctionCall{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	msg := OpenAIMessage{
		Role:    "assistant",
		Content: content,
	}

	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return msg
}

// convertTools converte tools OpenAI in formato Anthropic
func (a *Adapter) convertTools(openaiTools []OpenAITool) ([]Tool, error) {
	tools := make([]Tool, len(openaiTools))

	for i, t := range openaiTools {
		if t.Type != "function" {
			return nil, fmt.Errorf("unsupported tool type: %s", t.Type)
		}

		tools[i] = Tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		}
	}

	return tools, nil
}

// convertStopSequences converte stop sequences da OpenAI a Anthropic
func (a *Adapter) convertStopSequences(stop interface{}) []string {
	switch v := stop.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []interface{}:
		var sequences []string
		for _, s := range v {
			if str, ok := s.(string); ok {
				sequences = append(sequences, str)
			}
		}
		return sequences
	default:
		return nil
	}
}

// mapModel mappa un modello OpenAI a uno Anthropic
func (a *Adapter) mapModel(openaiModel string) string {
	if anthropicModel, ok := a.modelMapping[openaiModel]; ok {
		return anthropicModel
	}

	// Se il modello è già un modello Anthropic, restituiscilo
	if strings.HasPrefix(openaiModel, "claude-") {
		return openaiModel
	}

	// Default a Claude 3.5 Sonnet
	return ModelClaude35Sonnet
}

// mapFinishReason mappa un finish reason Anthropic a OpenAI
func (a *Adapter) mapFinishReason(anthropicReason StopReason) string {
	switch anthropicReason {
	case StopReasonEndTurn:
		return "stop"
	case StopReasonMaxTokens:
		return "length"
	case StopReasonStopSequence:
		return "stop"
	case StopReasonToolUse:
		return "tool_calls"
	default:
		return "stop"
	}
}

// SetModelMapping imposta un mapping personalizzato dei modelli
func (a *Adapter) SetModelMapping(mapping map[string]string) {
	for k, v := range mapping {
		a.modelMapping[k] = v
	}
}
