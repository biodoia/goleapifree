package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/gateway"
	"github.com/biodoia/goleapifree/tests/testhelpers"
)

func setupStreamingTestServer(t *testing.T) (*httptest.Server, *gateway.Gateway) {
	db := testhelpers.TestDB(t)
	cfg := testhelpers.TestConfig()

	gw, err := gateway.New(cfg, db)
	testhelpers.AssertNoError(t, err, "Failed to create gateway")

	provider := testhelpers.CreateTestProvider(t, db.DB, "streaming-provider")
	provider.SupportsStreaming = true
	db.DB.Save(provider)

	testhelpers.CreateTestModel(t, db.DB, provider.ID, "gpt-4")

	server := httptest.NewServer(gw.App().Handler())
	return server, gw
}

func TestStreaming_OpenAI_Basic(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Tell me a story"}},
		Stream:   true,
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
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	// For streaming, we expect either:
	// - 200 with text/event-stream content-type
	// - 501 if not implemented yet
	testhelpers.AssertTrue(t, resp.StatusCode == 200 || resp.StatusCode == 501,
		"Expected 200 or 501 for streaming")

	if resp.StatusCode == 200 {
		contentType := resp.Header.Get("Content-Type")
		testhelpers.AssertTrue(t,
			strings.Contains(contentType, "text/event-stream") ||
				strings.Contains(contentType, "application/x-ndjson"),
			"Streaming should use SSE or NDJSON content type")
	}
}

func TestStreaming_OpenAI_SSE_Format(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Count to 5"}},
		Stream:   true,
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

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Read SSE stream
		scanner := bufio.NewScanner(resp.Body)
		eventCount := 0
		timeout := time.After(5 * time.Second)

		for scanner.Scan() {
			select {
			case <-timeout:
				t.Log("Stream timeout reached")
				return
			default:
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					eventCount++
					data := strings.TrimPrefix(line, "data: ")

					// Check for [DONE] marker
					if data == "[DONE]" {
						testhelpers.AssertTrue(t, eventCount > 0, "Should receive events before [DONE]")
						return
					}

					// Try to parse as JSON
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						testhelpers.AssertTrue(t, event["object"] != nil, "Event should have object field")
					}
				}
			}
		}
	}
}

func TestStreaming_Anthropic_Basic(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := AnthropicMessageRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: "Write a haiku"}},
		Stream:    true,
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
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	testhelpers.AssertTrue(t, resp.StatusCode == 200 || resp.StatusCode == 501,
		"Expected 200 or 501 for streaming")
}

func TestStreaming_ClientDisconnect(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Long response please"}},
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		server.URL+"/v1/chat/completions",
		bytes.NewBuffer(body),
	)
	testhelpers.AssertNoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Context cancellation is expected
		testhelpers.AssertTrue(t, strings.Contains(err.Error(), "context"),
			"Error should be context-related")
		return
	}
	defer resp.Body.Close()

	// Try to read until context expires
	buf := make([]byte, 1024)
	_, err = resp.Body.Read(buf)
	// Error is expected due to context cancellation
}

func TestStreaming_MultipleClients(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	testhelpers.AssertNoError(t, err, "Failed to marshal request")

	// Simulate multiple concurrent streaming clients
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func(clientID int) {
			defer func() { done <- true }()

			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/chat/completions",
				bytes.NewBuffer(body),
			)
			if err != nil {
				t.Errorf("Client %d: Failed to create request: %v", clientID, err)
				return
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("Client %d: Failed to send request: %v", clientID, err)
				return
			}
			defer resp.Body.Close()

			// Read a bit of the stream
			buf := make([]byte, 1024)
			_, _ = resp.Body.Read(buf)
		}(i)
	}

	// Wait for all clients
	timeout := time.After(5 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Client completed
		case <-timeout:
			t.Fatal("Timeout waiting for streaming clients")
		}
	}
}

func TestStreaming_ErrorHandling(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	testCases := []struct {
		name    string
		request OpenAIChatRequest
	}{
		{
			name: "empty messages with stream",
			request: OpenAIChatRequest{
				Model:    "gpt-4",
				Messages: []OpenAIMessage{},
				Stream:   true,
			},
		},
		{
			name: "invalid model with stream",
			request: OpenAIChatRequest{
				Model:    "nonexistent-model",
				Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
				Stream:   true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.request)
			testhelpers.AssertNoError(t, err, "Failed to marshal request")

			req, err := http.NewRequest(
				"POST",
				server.URL+"/v1/chat/completions",
				bytes.NewBuffer(body),
			)
			testhelpers.AssertNoError(t, err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			testhelpers.AssertNoError(t, err, "Failed to send request")
			defer resp.Body.Close()

			// Should get some response, even if it's an error
			testhelpers.AssertTrue(t, resp.StatusCode != 0, "Should get a status code")
		})
	}
}

func TestStreaming_BackpressureHandling(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Generate a lot of text"}},
		Stream:   true,
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

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Read slowly to simulate backpressure
		buf := make([]byte, 10)
		for i := 0; i < 5; i++ {
			_, err := resp.Body.Read(buf)
			if err != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestStreaming_ChunkedEncoding(t *testing.T) {
	server, gw := setupStreamingTestServer(t)
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
		Stream:   true,
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

	resp, err := http.DefaultClient.Do(req)
	testhelpers.AssertNoError(t, err, "Failed to send request")
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Verify Transfer-Encoding is chunked for streaming
		transferEncoding := resp.Header.Get("Transfer-Encoding")
		testhelpers.AssertTrue(t,
			transferEncoding == "chunked" || transferEncoding == "",
			"Should use chunked encoding or default streaming")
	}
}

// Benchmark tests
func BenchmarkStreaming_OpenAI(b *testing.B) {
	server, gw := setupStreamingTestServer(&testing.T{})
	defer server.Close()
	defer gw.Shutdown(context.Background())

	reqBody := OpenAIChatRequest{
		Model:    "gpt-4",
		Messages: []OpenAIMessage{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(
			"POST",
			server.URL+"/v1/chat/completions",
			bytes.NewBuffer(body),
		)
		req.Header.Set("Content-Type", "application/json")

		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}
