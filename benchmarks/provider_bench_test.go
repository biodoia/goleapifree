package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/internal/providers/openai"
)

// BenchmarkProviderChatCompletion misura le performance di base del provider
func BenchmarkProviderChatCompletion(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := provider.ChatCompletion(context.Background(), req)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
	}
}

// BenchmarkProviderChatCompletion_Parallel misura throughput in parallelo
func BenchmarkProviderChatCompletion_Parallel(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := provider.ChatCompletion(context.Background(), req)
			if err != nil {
				b.Logf("Request failed: %v", err)
			}
		}
	})
}

// BenchmarkProviderLatency misura la latency end-to-end
func BenchmarkProviderLatency(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	var totalLatency time.Duration
	var successCount int64

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := provider.ChatCompletion(context.Background(), req)
		latency := time.Since(start)

		if err == nil {
			totalLatency += latency
			atomic.AddInt64(&successCount, 1)
		}
	}

	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)
		b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")
		b.ReportMetric(float64(successCount)*1000/float64(b.Elapsed().Milliseconds()), "req/sec")
	}
}

// BenchmarkProviderConcurrency testa diversi livelli di concorrenza
func BenchmarkProviderConcurrency(b *testing.B) {
	concurrencyLevels := []int{1, 10, 50, 100, 500}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			provider := createTestProvider()
			req := createTestRequest()

			var wg sync.WaitGroup
			var successCount int64
			var errorCount int64

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := provider.ChatCompletion(context.Background(), req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}()

				// Rate limit to avoid overwhelming
				if (i+1)%concurrency == 0 {
					wg.Wait()
				}
			}

			wg.Wait()

			errorRate := float64(errorCount) / float64(successCount+errorCount) * 100
			b.ReportMetric(errorRate, "error_rate_%")
			b.ReportMetric(float64(successCount), "success_count")
		})
	}
}

// BenchmarkProviderStreaming misura performance dello streaming
func BenchmarkProviderStreaming(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()
	req.Stream = true

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		chunkCount := 0
		err := provider.Stream(context.Background(), req, func(chunk *providers.StreamChunk) error {
			chunkCount++
			return nil
		})
		if err != nil {
			b.Logf("Stream failed: %v", err)
		}
		b.ReportMetric(float64(chunkCount), "chunks_per_req")
	}
}

// BenchmarkProviderStreamingVsNonStreaming compara streaming vs non-streaming
func BenchmarkProviderStreamingVsNonStreaming(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	b.Run("NonStreaming", func(b *testing.B) {
		req.Stream = false
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := provider.ChatCompletion(context.Background(), req)
			if err != nil {
				b.Logf("Request failed: %v", err)
			}
		}
	})

	b.Run("Streaming", func(b *testing.B) {
		req.Stream = true
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := provider.Stream(context.Background(), req, func(chunk *providers.StreamChunk) error {
				return nil
			})
			if err != nil {
				b.Logf("Stream failed: %v", err)
			}
		}
	})
}

// BenchmarkProviderRequestSizes testa con diverse dimensioni di richieste
func BenchmarkProviderRequestSizes(b *testing.B) {
	provider := createTestProvider()

	sizes := []struct {
		name      string
		msgCount  int
		msgLength int
	}{
		{"Small_1msg_100chars", 1, 100},
		{"Medium_5msg_500chars", 5, 500},
		{"Large_10msg_1000chars", 10, 1000},
		{"XLarge_20msg_2000chars", 20, 2000},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			req := createTestRequestWithSize(size.msgCount, size.msgLength)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := provider.ChatCompletion(context.Background(), req)
				if err != nil {
					b.Logf("Request failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkProviderRetryLogic testa la logica di retry
func BenchmarkProviderRetryLogic(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := provider.ChatCompletion(ctx, req)
		cancel()
		if err != nil {
			b.Logf("Request with retry failed: %v", err)
		}
	}
}

// BenchmarkProviderHealthCheck misura overhead degli health check
func BenchmarkProviderHealthCheck(b *testing.B) {
	provider := createTestProvider()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := provider.HealthCheck(context.Background())
		if err != nil {
			b.Logf("Health check failed: %v", err)
		}
	}
}

// BenchmarkMultipleProviders compara performance di diversi provider
func BenchmarkMultipleProviders(b *testing.B) {
	providers := map[string]providers.Provider{
		"OpenAI":    createOpenAIProvider(),
		"Anthropic": createAnthropicProvider(),
		// Add more providers as needed
	}

	req := createTestRequest()

	for name, provider := range providers {
		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			var totalLatency time.Duration
			var successCount int64

			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, err := provider.ChatCompletion(context.Background(), req)
				latency := time.Since(start)

				if err == nil {
					totalLatency += latency
					atomic.AddInt64(&successCount, 1)
				}
			}

			if successCount > 0 {
				avgLatency := totalLatency / time.Duration(successCount)
				b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")
			}
		})
	}
}

// BenchmarkProviderMemoryUsage misura il consumo di memoria
func BenchmarkProviderMemoryUsage(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = provider.ChatCompletion(context.Background(), req)
	}
}

// BenchmarkProviderThroughput misura il throughput massimo
func BenchmarkProviderThroughput(b *testing.B) {
	provider := createTestProvider()
	req := createTestRequest()

	concurrency := 100
	var wg sync.WaitGroup
	var totalRequests int64
	var totalLatency int64

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqStart := time.Now()
			_, err := provider.ChatCompletion(context.Background(), req)
			if err == nil {
				atomic.AddInt64(&totalRequests, 1)
				atomic.AddInt64(&totalLatency, int64(time.Since(reqStart).Milliseconds()))
			}
		}()

		if (i+1)%concurrency == 0 {
			wg.Wait()
		}
	}

	wg.Wait()
	elapsed := time.Since(start)

	throughput := float64(totalRequests) / elapsed.Seconds()
	avgLatency := float64(totalLatency) / float64(totalRequests)

	b.ReportMetric(throughput, "req/sec")
	b.ReportMetric(avgLatency, "avg_latency_ms")
}

// Helper functions

func createTestProvider() providers.Provider {
	// Create a mock or test provider
	// For real benchmarks, use actual provider with test credentials
	base := providers.NewBaseProvider("test", "http://localhost:8080", "test-key")
	return &mockProvider{BaseProvider: base}
}

func createOpenAIProvider() providers.Provider {
	// Create OpenAI provider for benchmarking
	return openai.NewOpenAIProvider("test-key")
}

func createAnthropicProvider() providers.Provider {
	// Create Anthropic provider for benchmarking
	base := providers.NewBaseProvider("anthropic", "https://api.anthropic.com", "test-key")
	return &mockProvider{BaseProvider: base}
}

func createTestRequest() *providers.ChatRequest {
	return &providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	}
}

func createTestRequestWithSize(msgCount, msgLength int) *providers.ChatRequest {
	messages := make([]providers.Message, msgCount)
	content := generateString(msgLength)

	for i := 0; i < msgCount; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = providers.Message{
			Role:    role,
			Content: content,
		}
	}

	return &providers.ChatRequest{
		Model:    "gpt-3.5-turbo",
		Messages: messages,
	}
}

func generateString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return string(b)
}

// Mock provider for testing
type mockProvider struct {
	*providers.BaseProvider
}

func (m *mockProvider) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// Simulate network latency
	time.Sleep(10 * time.Millisecond)

	return &providers.ChatResponse{
		ID:      "test-123",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []providers.Choice{
			{
				Index: 0,
				Message: providers.Message{
					Role:    "assistant",
					Content: "This is a test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: providers.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req *providers.ChatRequest, handler providers.StreamHandler) error {
	// Simulate streaming
	chunks := []string{"This ", "is ", "a ", "test ", "response"}

	for _, chunk := range chunks {
		time.Sleep(5 * time.Millisecond)
		if err := handler(&providers.StreamChunk{
			Delta: chunk,
			Done:  false,
		}); err != nil {
			return err
		}
	}

	// Send final chunk
	return handler(&providers.StreamChunk{
		Done: true,
		Usage: &providers.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	})
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	time.Sleep(1 * time.Millisecond)
	return nil
}

func (m *mockProvider) GetModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{
		{
			ID:            "test-model",
			Name:          "Test Model",
			Provider:      "test",
			ContextLength: 4096,
			MaxTokens:     2048,
		},
	}, nil
}
