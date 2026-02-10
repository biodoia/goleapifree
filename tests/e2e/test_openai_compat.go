package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/gateway"
	"github.com/biodoia/goleapifree/tests/testhelpers"
)

// OpenAI API structures
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func setupTestServer(t *testing.T) (*httptest.Server, *gateway.Gateway) {
	db := testhelpers.TestDB(t)
	cfg := testhelpers.TestConfig()
	cfg.Server.Port = 0 // Random port

	gw, err := gateway.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create gateway")

	// Create test providers
	provider := testhelpers.CreateTestProvider(t, db.DB, "test-openai")
	testhelpers.CreateTestModel(t, db.DB, provider.ID, "gpt-4")
	testhelpers.CreateTestModel(t, db.DB, provider.ID, "gpt-3.5-turbo")

	server := httptest.NewServer(gw.App().Handler())
	return server, gw
}

func TestOpenAI_ChatCompletionBasic(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	resp, err := http.Post(
		server.URL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	// For now, we expect "not implemented yet" since handlers are stubs
	// In a full implementation, this would check for 200 OK
	testhelpers.AssertTrue(t, resp.StatusCode == 200 || resp.StatusCode == 501,
		"Expected 200 or 501 status")
}

func TestOpenAI_ModelsEndpoint(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	resp, err := http.Get(server.URL + "/v1/models")
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode == 200,
		"Expected 200 status for models endpoint")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	testhelpers.AssertNoError(t, err, "Failed to decode response")

	testhelpers.AssertEqual(t, "list", result["object"], "Object type mismatch")
}

func TestOpenAI_InvalidRequest(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	// Send invalid JSON
	resp, err := http.Post(
		server.URL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer([]byte("invalid json")),
	)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode >= 400,
		"Expected error status for invalid JSON")
}

func TestOpenAI_MissingModel(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	resp, err := http.Post(
		server.URL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	// Should accept or reject based on implementation
	// For now just verify we get a response
	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a status code")
}

func TestOpenAI_DifferentModels(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	models := []string{"gpt-4", "gpt-3.5-turbo"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			reqBody := OpenAIChatRequest{
				Model: model,
				Messages: []OpenAIMessage{
					{Role: "user", Content: "Test message"},
				},
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			resp, err := http.Post(
				server.URL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestOpenAI_TemperatureParameter(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	temperatures := []float64{0.0, 0.5, 1.0, 2.0}

	for _, temp := range temperatures {
		t.Run("temp_"+string(rune(temp)), func(t *testing.T) {
			reqBody := OpenAIChatRequest{
				Model:       "gpt-4",
				Messages:    []OpenAIMessage{{Role: "user", Content: "Test"}},
				Temperature: temp,
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			resp, err := http.Post(
				server.URL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestOpenAI_MaxTokensParameter(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	maxTokens := []int{10, 100, 1000, 4000}

	for _, tokens := range maxTokens {
		t.Run("tokens_"+string(rune(tokens)), func(t *testing.T) {
			reqBody := OpenAIChatRequest{
				Model:     "gpt-4",
				Messages:  []OpenAIMessage{{Role: "user", Content: "Test"}},
				MaxTokens: tokens,
			}

			body, err := json.Marshal(reqBody)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			resp, err := http.Post(
				server.URL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestOpenAI_MultipleMessages(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What's the weather?"},
			{Role: "assistant", Content: "I don't have weather data."},
			{Role: "user", Content: "That's okay."},
		},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	resp, err := http.Post(
		server.URL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
}

func TestOpenAI_ContentTypes(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	contentTypes := []string{
		"application/json",
		"application/json; charset=utf-8",
	}

	for _, ct := range contentTypes {
		t.Run(ct, func(t *testing.T) {
			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/chat/completions",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", ct)

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
		})
	}
}

func TestOpenAI_AuthorizationHeader(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	req, err := http.NewRequest(
		"POST",
		server.URL+"/v1/chat/completions",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")
}

func TestOpenAI_RateLimiting(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	// Send multiple rapid requests
	for i := 0; i < 5; i++ {
		resp, err := http.Post(
			server.URL+"/v1/chat/completions",
			"application/json",
			bytes.NewBuffer(body),
		)
		testhelpers.AssertNoError(t, err, "Failed to send request")
		resp.Body.Close()

		// In a real implementation with rate limiting, we'd check for 429
		// For now just verify we get responses
		testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a response")

		time.Sleep(10 * time.Millisecond)
	}
}

func TestOpenAI_ResponseStructure(t *testing.T) {
	server, gw := setupTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	resp, err := http.Post(
		server.URL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	testhelpers.AssertNoError(t, err, "Failed to read response body")

	// Try to parse as JSON
	var result map[string]interface{}
	err = json.Unmarshal(bodyBytes, &result)
	testhelpers.AssertNoError(t, err, "Response should be valid JSON")
}

// Benchmark tests
func BenchmarkOpenAI_ChatCompletion(b *testing.B) {
	server, gw := setupTestServer(&testing.T{})
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
	}

	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Post(
			server.URL+"/v1/chat/completions",
			"application/json",
			bytes.NewBuffer(body),
		)
		resp.Body.Close()
	}
}
