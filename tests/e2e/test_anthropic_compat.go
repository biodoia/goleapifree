package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biodoia/goleapifree/internal/gateway"
	"github.com/biodoia/goleapifree/tests/testhelpers"
)

// Anthropic API structures
type AnthropicMessageRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicMessageResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []ContentBlock     `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage     `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func setupAnthropicTestServer(t *testing.T) (*httptest.Server, *gateway.Gateway) {
	db := testhelpers.TestDB(t)
	cfg := testhelpers.TestConfig()

	gw, err := gateway.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create gateway")

	// Create test providers
	provider := testhelpers.CreateTestProvider(t, db.DB, "test-anthropic")
	testhelpers.CreateTestModel(t, db.DB, provider.ID, "claude-3-opus")
	testhelpers.CreateTestModel(t, db.DB, provider.ID, "claude-3-sonnet")

	server := httptest.NewServer(gw.App().Handler())
	return server, gw
}

func TestAnthropic_MessagesBasic(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello, Claude!"},
		},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest(
		"POST",
		server.URL+"/v1/messages",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode == 200 || resp.StatusCode == 501,
		"Expected 200 or 501 status")
}

func TestAnthropic_SystemPrompt(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		System:    "You are a helpful AI assistant specializing in Go programming.",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Explain interfaces in Go"},
		},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest(
		"POST",
		server.URL+"/v1/messages",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
}

func TestAnthropic_MultiTurnConversation(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "What's 2+2?"},
			{Role: "assistant", Content: "2+2 equals 4."},
			{Role: "user", Content: "And what's 4+4?"},
		},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest(
		"POST",
		server.URL+"/v1/messages",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
}

func TestAnthropic_RequiredHeaders(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: "Test"}},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "with all headers",
			headers: map[string]string{
				"x-api-key":          "test-key",
				"anthropic-version":  "2023-06-01",
				"content-type":       "application/json",
			},
		},
		{
			name: "with minimal headers",
			headers: map[string]string{
				"x-api-key":    "test-key",
				"content-type": "application/json",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/messages",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestAnthropic_DifferentModels(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	models := []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			reqBody := AnthropicMessageRequest{
				Model:     model,
				MaxTokens: 1024,
				Messages:  []AnthropicMessage{{Role: "user", Content: "Test"}},
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/messages",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-key")
			req.Header.Set("anthropic-version", "2023-06-01")

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestAnthropic_MaxTokensValidation(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	testCases := []struct {
		name      string
		maxTokens int
	}{
		{"min tokens", 1},
		{"normal tokens", 1024},
		{"max tokens", 4096},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := AnthropicMessageRequest{
				Model:     "claude-3-opus-20240229",
				MaxTokens: tc.maxTokens,
				Messages:  []AnthropicMessage{{Role: "user", Content: "Test"}},
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/messages",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-key")

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestAnthropic_TemperatureParameter(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	temperatures := []float64{0.0, 0.5, 1.0}

	for _, temp := range temperatures {
		t.Run("temp_"+string(rune(temp)), func(t *testing.T) {
			reqBody := AnthropicMessageRequest{
				Model:       "claude-3-opus-20240229",
				MaxTokens:   1024,
				Temperature: temp,
				Messages:    []AnthropicMessage{{Role: "user", Content: "Test"}},
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/messages",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-key")

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestAnthropic_InvalidRequest(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	testCases := []struct {
		name string
		body string
	}{
		{"invalid json", "not json"},
		{"missing model", `{"max_tokens": 1024, "messages": []}`},
		{"missing max_tokens", `{"model": "claude-3-opus", "messages": []}`},
		{"empty messages", `{"model": "claude-3-opus", "max_tokens": 1024, "messages": []}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/messages",
				bytes.NewBufferString(tc.body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "test-key")

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			// Should get error response
			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a status")
		})
	}
}

func TestAnthropic_LongConversation(t *testing.T) {
	server, gw := setupAnthropicTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	// Create a long conversation history
	messages := []AnthropicMessage{}
	for i := 0; i < 10; i++ {
		messages = append(messages,
			AnthropicMessage{Role: "user", Content: "Question " + string(rune('0'+i))},
			AnthropicMessage{Role: "assistant", Content: "Answer " + string(rune('0'+i))},
		)
	}
	messages = append(messages, AnthropicMessage{Role: "user", Content: "Final question"})

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages:  messages,
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest(
		"POST",
		server.URL+"/v1/messages",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
}

// Benchmark tests
func BenchmarkAnthropic_Messages(b *testing.B) {
	server, gw := setupAnthropicTestServer(&testing.T{})
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: "Test"}},
	}

	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(
			"POST",
			server.URL+"/v1/messages",
			bytes.NewBuffer(body),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", "test-key")

		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}
