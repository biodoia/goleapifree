package anthropic

import (
	"encoding/json"
	"testing"
)

func TestAdapter_ConvertRequest(t *testing.T) {
	adapter := NewAdapter()

	tests := []struct {
		name    string
		req     *OpenAIRequest
		wantErr bool
	}{
		{
			name: "simple text message",
			req: &OpenAIRequest{
				Model: "gpt-4",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 1024,
			},
			wantErr: false,
		},
		{
			name: "with system message",
			req: &OpenAIRequest{
				Model: "gpt-4",
				Messages: []OpenAIMessage{
					{Role: "system", Content: "You are helpful"},
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: 1024,
			},
			wantErr: false,
		},
		{
			name: "with temperature and top_p",
			req: &OpenAIRequest{
				Model:       "gpt-4",
				Messages:    []OpenAIMessage{{Role: "user", Content: "Hello"}},
				MaxTokens:   1024,
				Temperature: ptrFloat64(0.7),
				TopP:        ptrFloat64(0.9),
			},
			wantErr: false,
		},
		{
			name: "with stop sequences",
			req: &OpenAIRequest{
				Model:     "gpt-4",
				Messages:  []OpenAIMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: 1024,
				Stop:      []string{"\n", "END"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.ConvertRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if got.Model == "" {
					t.Error("ConvertRequest() model is empty")
				}
				if len(got.Messages) == 0 {
					t.Error("ConvertRequest() messages is empty")
				}
			}
		})
	}
}

func TestAdapter_ConvertMessages(t *testing.T) {
	adapter := NewAdapter()

	tests := []struct {
		name         string
		messages     []OpenAIMessage
		wantSystem   string
		wantMsgCount int
		wantErr      bool
	}{
		{
			name: "single user message",
			messages: []OpenAIMessage{
				{Role: "user", Content: "Hello"},
			},
			wantSystem:   "",
			wantMsgCount: 1,
			wantErr:      false,
		},
		{
			name: "system + user messages",
			messages: []OpenAIMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
			wantSystem:   "You are helpful",
			wantMsgCount: 1,
			wantErr:      false,
		},
		{
			name: "multiple system messages",
			messages: []OpenAIMessage{
				{Role: "system", Content: "Part 1"},
				{Role: "system", Content: "Part 2"},
				{Role: "user", Content: "Hello"},
			},
			wantSystem:   "Part 1\n\nPart 2",
			wantMsgCount: 1,
			wantErr:      false,
		},
		{
			name: "user + assistant conversation",
			messages: []OpenAIMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			wantSystem:   "",
			wantMsgCount: 3,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, messages, err := adapter.convertMessages(tt.messages)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if system != tt.wantSystem {
				t.Errorf("convertMessages() system = %v, want %v", system, tt.wantSystem)
			}
			if len(messages) != tt.wantMsgCount {
				t.Errorf("convertMessages() message count = %v, want %v", len(messages), tt.wantMsgCount)
			}
		})
	}
}

func TestAdapter_ConvertResponse(t *testing.T) {
	adapter := NewAdapter()

	anthropicResp := &MessagesResponse{
		ID:    "msg_123",
		Model: ModelClaude35Sonnet,
		Role:  MessageRoleAssistant,
		Content: []ContentBlock{
			{Type: ContentBlockTypeText, Text: "Hello! How can I help?"},
		},
		StopReason: StopReasonEndTurn,
		Usage: Usage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	openaiResp := adapter.ConvertResponse(anthropicResp)

	if openaiResp.ID != "msg_123" {
		t.Errorf("ID = %v, want msg_123", openaiResp.ID)
	}
	if openaiResp.Object != "chat.completion" {
		t.Errorf("Object = %v, want chat.completion", openaiResp.Object)
	}
	if len(openaiResp.Choices) != 1 {
		t.Errorf("Choices count = %v, want 1", len(openaiResp.Choices))
	}
	if openaiResp.Choices[0].Message.Content != "Hello! How can I help?" {
		t.Errorf("Content = %v, want 'Hello! How can I help?'", openaiResp.Choices[0].Message.Content)
	}
	if openaiResp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %v, want stop", openaiResp.Choices[0].FinishReason)
	}
	if openaiResp.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %v, want 30", openaiResp.Usage.TotalTokens)
	}
}

func TestAdapter_MapModel(t *testing.T) {
	adapter := NewAdapter()

	tests := []struct {
		input string
		want  string
	}{
		{"gpt-4", ModelClaude3Opus},
		{"gpt-4-turbo", ModelClaude35Sonnet},
		{"gpt-4o", ModelClaude35Sonnet},
		{"gpt-3.5-turbo", ModelClaude35Haiku},
		{"claude-3-opus", ModelClaude3Opus},
		{"claude-3.5-sonnet", ModelClaude35Sonnet},
		{"unknown-model", ModelClaude35Sonnet}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := adapter.mapModel(tt.input)
			if got != tt.want {
				t.Errorf("mapModel(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAdapter_MapFinishReason(t *testing.T) {
	adapter := NewAdapter()

	tests := []struct {
		input StopReason
		want  string
	}{
		{StopReasonEndTurn, "stop"},
		{StopReasonMaxTokens, "length"},
		{StopReasonStopSequence, "stop"},
		{StopReasonToolUse, "tool_calls"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := adapter.mapFinishReason(tt.input)
			if got != tt.want {
				t.Errorf("mapFinishReason(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAdapter_ConvertTools(t *testing.T) {
	adapter := NewAdapter()

	openaiTools := []OpenAITool{
		{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        "get_weather",
				Description: "Get weather info",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	}

	tools, err := adapter.convertTools(openaiTools)
	if err != nil {
		t.Errorf("convertTools() error = %v", err)
		return
	}

	if len(tools) != 1 {
		t.Errorf("tools count = %v, want 1", len(tools))
	}
	if tools[0].Name != "get_weather" {
		t.Errorf("tool name = %v, want get_weather", tools[0].Name)
	}
}

func TestAdapter_ConvertStopSequences(t *testing.T) {
	adapter := NewAdapter()

	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{
			name:  "single string",
			input: "STOP",
			want:  []string{"STOP"},
		},
		{
			name:  "string array",
			input: []string{"STOP", "END"},
			want:  []string{"STOP", "END"},
		},
		{
			name:  "interface array",
			input: []interface{}{"STOP", "END"},
			want:  []string{"STOP", "END"},
		},
		{
			name:  "nil",
			input: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.convertStopSequences(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("convertStopSequences() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("convertStopSequences()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMessagesRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *MessagesRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &MessagesRequest{
				Model:     ModelClaude35Haiku,
				MaxTokens: 1024,
				Messages: []Message{
					{
						Role:    MessageRoleUser,
						Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Hi"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: &MessagesRequest{
				MaxTokens: 1024,
				Messages: []Message{
					{Role: MessageRoleUser, Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Hi"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "missing messages",
			req: &MessagesRequest{
				Model:     ModelClaude35Haiku,
				MaxTokens: 1024,
			},
			wantErr: true,
		},
		{
			name: "invalid max_tokens",
			req: &MessagesRequest{
				Model:     ModelClaude35Haiku,
				MaxTokens: 0,
				Messages: []Message{
					{Role: MessageRoleUser, Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Hi"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "max_tokens too large",
			req: &MessagesRequest{
				Model:     ModelClaude35Haiku,
				MaxTokens: 300000,
				Messages: []Message{
					{Role: MessageRoleUser, Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Hi"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "first message not user",
			req: &MessagesRequest{
				Model:     ModelClaude35Haiku,
				MaxTokens: 1024,
				Messages: []Message{
					{Role: MessageRoleAssistant, Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Hi"}}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessagesResponse_GetText(t *testing.T) {
	resp := &MessagesResponse{
		Content: []ContentBlock{
			{Type: ContentBlockTypeText, Text: "Hello"},
			{Type: ContentBlockTypeText, Text: " World"},
		},
	}

	text := resp.GetText()
	if text != "Hello" {
		t.Errorf("GetText() = %v, want Hello", text)
	}

	allText := resp.GetAllText()
	if allText != "Hello World" {
		t.Errorf("GetAllText() = %v, want 'Hello World'", allText)
	}
}

func TestMessagesResponse_HasToolUse(t *testing.T) {
	tests := []struct {
		name    string
		content []ContentBlock
		want    bool
	}{
		{
			name: "has tool use",
			content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Let me check"},
				{Type: ContentBlockTypeToolUse, ID: "tool_1", Name: "get_weather"},
			},
			want: true,
		},
		{
			name: "no tool use",
			content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Just text"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &MessagesResponse{Content: tt.content}
			if got := resp.HasToolUse(); got != tt.want {
				t.Errorf("HasToolUse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_Methods(t *testing.T) {
	tests := []struct {
		name         string
		errType      string
		wantRateErr  bool
		wantAuthErr  bool
		wantRetryable bool
	}{
		{
			name:         "rate limit error",
			errType:      ErrorTypeRateLimit,
			wantRateErr:  true,
			wantAuthErr:  false,
			wantRetryable: true,
		},
		{
			name:         "auth error",
			errType:      ErrorTypeAuthentication,
			wantRateErr:  false,
			wantAuthErr:  true,
			wantRetryable: false,
		},
		{
			name:         "overloaded error",
			errType:      ErrorTypeOverloaded,
			wantRateErr:  false,
			wantAuthErr:  false,
			wantRetryable: true,
		},
		{
			name:         "invalid request",
			errType:      ErrorTypeInvalidRequest,
			wantRateErr:  false,
			wantAuthErr:  false,
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Type: tt.errType, Message: "test error"}

			if got := err.IsRateLimitError(); got != tt.wantRateErr {
				t.Errorf("IsRateLimitError() = %v, want %v", got, tt.wantRateErr)
			}
			if got := err.IsAuthError(); got != tt.wantAuthErr {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.wantAuthErr)
			}
			if got := err.IsRetryable(); got != tt.wantRetryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.wantRetryable)
			}
		})
	}
}
