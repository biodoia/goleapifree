package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/gateway"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/gofiber/fiber/v3"
)

// BenchmarkE2EFullRequest misura il ciclo completo della richiesta
func BenchmarkE2EFullRequest(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	req := createE2ETestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := makeRequest(server.URL+"/v1/chat/completions", req)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkE2EWithAuth misura l'overhead dell'autenticazione
func BenchmarkE2EWithAuth(b *testing.B) {
	server := createTestServerWithAuth()
	defer server.Close()

	req := createE2ETestRequest()
	req.Headers = map[string]string{
		"Authorization": "Bearer test-token",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := makeRequestWithHeaders(server.URL+"/v1/chat/completions", req, req.Headers)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkE2EOpenAICompat misura l'overhead della compatibilità OpenAI
func BenchmarkE2EOpenAICompat(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	req := &OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := makeOpenAIRequest(server.URL+"/v1/chat/completions", req)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkE2EStreamingVsNonStreaming compara streaming vs non-streaming
func BenchmarkE2EStreamingVsNonStreaming(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	b.Run("NonStreaming", func(b *testing.B) {
		req := createE2ETestRequest()
		req.Stream = false

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := makeRequest(server.URL+"/v1/chat/completions", req)
			if err != nil {
				b.Logf("Request failed: %v", err)
			}
		}
	})

	b.Run("Streaming", func(b *testing.B) {
		req := createE2ETestRequest()
		req.Stream = true

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := makeStreamingRequest(server.URL+"/v1/chat/completions", req)
			if err != nil {
				b.Logf("Stream failed: %v", err)
			}
		}
	})
}

// BenchmarkE2ERequestPipeline misura l'intero pipeline
func BenchmarkE2ERequestPipeline(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	req := createE2ETestRequest()

	var totalLatency time.Duration
	var routingLatency time.Duration
	var providerLatency time.Duration
	var serializationLatency time.Duration

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()

		// Measure routing decision
		routingStart := time.Now()
		// Simulate routing
		time.Sleep(50 * time.Microsecond)
		routingLatency += time.Since(routingStart)

		// Measure provider call
		providerStart := time.Now()
		resp, err := makeRequest(server.URL+"/v1/chat/completions", req)
		providerLatency += time.Since(providerStart)

		// Measure serialization
		serializationStart := time.Now()
		if err == nil {
			_ = json.Marshal(resp)
		}
		serializationLatency += time.Since(serializationStart)

		totalLatency += time.Since(start)
	}

	b.ReportMetric(float64(totalLatency.Milliseconds())/float64(b.N), "total_ms")
	b.ReportMetric(float64(routingLatency.Microseconds())/float64(b.N), "routing_us")
	b.ReportMetric(float64(providerLatency.Milliseconds())/float64(b.N), "provider_ms")
	b.ReportMetric(float64(serializationLatency.Microseconds())/float64(b.N), "serialization_us")
}

// BenchmarkE2EConcurrentRequests testa richieste concurrent
func BenchmarkE2EConcurrentRequests(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	concurrencyLevels := []int{1, 10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			req := createE2ETestRequest()

			var wg sync.WaitGroup
			var successCount int64
			var errorCount int64

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := makeRequest(server.URL+"/v1/chat/completions", req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}()

				if (i+1)%concurrency == 0 {
					wg.Wait()
				}
			}

			wg.Wait()

			errorRate := float64(errorCount) / float64(successCount+errorCount) * 100
			b.ReportMetric(errorRate, "error_rate_%")
		})
	}
}

// BenchmarkE2EMiddlewareOverhead misura l'overhead dei middleware
func BenchmarkE2EMiddlewareOverhead(b *testing.B) {
	b.Run("NoMiddleware", func(b *testing.B) {
		server := createTestServerWithoutMiddleware()
		defer server.Close()

		req := createE2ETestRequest()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = makeRequest(server.URL+"/v1/chat/completions", req)
		}
	})

	b.Run("WithMiddleware", func(b *testing.B) {
		server := createTestServerWithMiddleware()
		defer server.Close()

		req := createE2ETestRequest()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = makeRequest(server.URL+"/v1/chat/completions", req)
		}
	})
}

// BenchmarkE2EPayloadSizes testa diverse dimensioni di payload
func BenchmarkE2EPayloadSizes(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	sizes := []struct {
		name    string
		msgLen  int
		msgCount int
	}{
		{"Small_100B", 100, 1},
		{"Medium_1KB", 1024, 1},
		{"Large_10KB", 10240, 1},
		{"XLarge_100KB", 102400, 1},
		{"Conversation_10msgs", 500, 10},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			req := createE2ETestRequestWithSize(size.msgLen, size.msgCount)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := makeRequest(server.URL+"/v1/chat/completions", req)
				if err != nil {
					b.Logf("Request failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkE2ECaching misura l'impatto del caching
func BenchmarkE2ECaching(b *testing.B) {
	b.Run("WithoutCache", func(b *testing.B) {
		server := createTestServerWithoutCache()
		defer server.Close()

		req := createE2ETestRequest()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = makeRequest(server.URL+"/v1/chat/completions", req)
		}
	})

	b.Run("WithCache", func(b *testing.B) {
		server := createTestServerWithCache()
		defer server.Close()

		req := createE2ETestRequest()
		b.ResetTimer()

		var cacheHits int64
		var cacheMisses int64

		for i := 0; i < b.N; i++ {
			// Use same request to test cache
			resp, _ := makeRequest(server.URL+"/v1/chat/completions", req)
			if resp != nil && resp.Cached {
				atomic.AddInt64(&cacheHits, 1)
			} else {
				atomic.AddInt64(&cacheMisses, 1)
			}
		}

		hitRate := float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
		b.ReportMetric(hitRate, "cache_hit_rate_%")
	})
}

// BenchmarkE2EThroughput misura il throughput massimo
func BenchmarkE2EThroughput(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	req := createE2ETestRequest()

	var totalRequests int64
	start := time.Now()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := makeRequest(server.URL+"/v1/chat/completions", req)
			if err == nil {
				atomic.AddInt64(&totalRequests, 1)
			}
		}
	})

	elapsed := time.Since(start)
	throughput := float64(totalRequests) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/sec")
}

// BenchmarkE2ELatencyP99 misura la latency al 99° percentile
func BenchmarkE2ELatencyP99(b *testing.B) {
	server := createTestServer()
	defer server.Close()

	req := createE2ETestRequest()
	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := makeRequest(server.URL+"/v1/chat/completions", req)
		latency := time.Since(start)

		if err == nil {
			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()
		}
	}

	// Calculate percentiles
	if len(latencies) > 0 {
		p50 := percentile(latencies, 0.50)
		p95 := percentile(latencies, 0.95)
		p99 := percentile(latencies, 0.99)

		b.ReportMetric(float64(p50.Milliseconds()), "p50_ms")
		b.ReportMetric(float64(p95.Milliseconds()), "p95_ms")
		b.ReportMetric(float64(p99.Milliseconds()), "p99_ms")
	}
}

// Helper functions

type E2ETestRequest struct {
	Model    string            `json:"model"`
	Messages []E2EMessage      `json:"messages"`
	Stream   bool              `json:"stream,omitempty"`
	Headers  map[string]string `json:"-"`
}

type E2EMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type E2ETestResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []E2EChoice      `json:"choices"`
	Cached  bool             `json:"cached,omitempty"`
}

type E2EChoice struct {
	Message E2EMessage `json:"message"`
}

type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func createTestServer() *httptest.Server {
	app := fiber.New()

	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		// Simulate processing time
		time.Sleep(10 * time.Millisecond)

		return c.JSON(fiber.Map{
			"id":      "test-123",
			"model":   "gpt-3.5-turbo",
			"choices": []fiber.Map{
				{
					"message": fiber.Map{
						"role":    "assistant",
						"content": "This is a test response",
					},
				},
			},
		})
	})

	return httptest.NewServer(app)
}

func createTestServerWithAuth() *httptest.Server {
	app := fiber.New()

	// Auth middleware
	app.Use(func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	})

	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		time.Sleep(10 * time.Millisecond)
		return c.JSON(fiber.Map{"id": "test-123"})
	})

	return httptest.NewServer(app)
}

func createTestServerWithoutMiddleware() *httptest.Server {
	app := fiber.New()
	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": "test-123"})
	})
	return httptest.NewServer(app)
}

func createTestServerWithMiddleware() *httptest.Server {
	app := fiber.New()

	// Add multiple middleware
	app.Use(func(c fiber.Ctx) error { return c.Next() }) // Logging
	app.Use(func(c fiber.Ctx) error { return c.Next() }) // Metrics
	app.Use(func(c fiber.Ctx) error { return c.Next() }) // Recovery

	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": "test-123"})
	})

	return httptest.NewServer(app)
}

func createTestServerWithoutCache() *httptest.Server {
	return createTestServer()
}

func createTestServerWithCache() *httptest.Server {
	app := fiber.New()

	// Simple cache middleware
	cache := make(map[string]interface{})
	var mu sync.RWMutex

	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		body := c.Body()
		key := string(body)

		mu.RLock()
		cached, exists := cache[key]
		mu.RUnlock()

		if exists {
			return c.JSON(cached)
		}

		response := fiber.Map{
			"id":     "test-123",
			"cached": false,
		}

		mu.Lock()
		cache[key] = response
		mu.Unlock()

		return c.JSON(response)
	})

	return httptest.NewServer(app)
}

func createE2ETestRequest() *E2ETestRequest {
	return &E2ETestRequest{
		Model: "gpt-3.5-turbo",
		Messages: []E2EMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
}

func createE2ETestRequestWithSize(msgLen, msgCount int) *E2ETestRequest {
	messages := make([]E2EMessage, msgCount)
	content := generateString(msgLen)

	for i := 0; i < msgCount; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = E2EMessage{
			Role:    role,
			Content: content,
		}
	}

	return &E2ETestRequest{
		Model:    "gpt-3.5-turbo",
		Messages: messages,
	}
}

func makeRequest(url string, req *E2ETestRequest) (*E2ETestResponse, error) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result E2ETestResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func makeRequestWithHeaders(url string, req *E2ETestRequest, headers map[string]string) (*E2ETestResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result E2ETestResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func makeOpenAIRequest(url string, req *OpenAIRequest) (*E2ETestResponse, error) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result E2ETestResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func makeStreamingRequest(url string, req *E2ETestRequest) error {
	req.Stream = true
	body, _ := json.Marshal(req)

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read stream
	_, _ = io.ReadAll(resp.Body)
	return nil
}

func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple sort
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Bubble sort for simplicity
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
