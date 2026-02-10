package chaining

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
)

// MockProvider è un provider di test
type MockProvider struct {
	name     string
	latency  time.Duration
	response string
	err      error
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Simula latenza
	time.Sleep(m.latency)

	return &providers.ChatResponse{
		ID:      "test-" + m.name,
		Model:   req.Model,
		Created: time.Now().Unix(),
		Choices: []providers.Choice{
			{
				Index: 0,
				Message: providers.Message{
					Role:    "assistant",
					Content: m.response,
				},
				FinishReason: "stop",
			},
		},
		Usage: providers.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *MockProvider) Stream(ctx context.Context, req *providers.ChatRequest, handler providers.StreamHandler) error {
	return nil
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockProvider) GetModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (m *MockProvider) SupportsFeature(feature providers.Feature) bool {
	return true
}

// TestDraftRefineStrategy testa la strategia draft-refine
func TestDraftRefineStrategy(t *testing.T) {
	// Setup mock providers
	draftProvider := &MockProvider{
		name:     "draft-fast",
		latency:  100 * time.Millisecond,
		response: "This is a quick draft response.",
	}

	refineProvider := &MockProvider{
		name:     "refine-quality",
		latency:  500 * time.Millisecond,
		response: "This is a refined, high-quality response with more details and better structure.",
	}

	// Crea pipeline
	pipeline := NewPipeline(NewDraftRefineStrategy("", ""))

	pipeline.AddStage(Stage{
		Name:        "draft",
		Provider:    draftProvider,
		Model:       "fast-model",
		Transformer: &ExampleDraftRefineTransformer{phase: "draft"},
		Timeout:     5 * time.Second,
		MaxRetries:  2,
	})

	pipeline.AddStage(Stage{
		Name:        "refine",
		Provider:    refineProvider,
		Model:       "quality-model",
		Transformer: &ExampleDraftRefineTransformer{phase: "refine"},
		Timeout:     10 * time.Second,
		MaxRetries:  2,
	})

	// Execute
	ctx := context.Background()
	req := &providers.ChatRequest{
		Model: "test-model",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "Explain AI",
			},
		},
	}

	result, err := pipeline.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Verifica risultati
	if len(result.StageOutputs) != 2 {
		t.Errorf("Expected 2 stage outputs, got %d", len(result.StageOutputs))
	}

	if result.FinalResponse == nil {
		t.Error("Expected final response, got nil")
	}

	if result.TotalTokens != 60 { // 30 tokens per stage
		t.Errorf("Expected 60 total tokens, got %d", result.TotalTokens)
	}

	t.Logf("Pipeline completed in %v", result.TotalDuration)
	t.Logf("Total tokens: %d", result.TotalTokens)
	t.Logf("Final response: %v", result.FinalResponse.Choices[0].Message.Content)
}

// TestCascadeStrategy testa la strategia cascade
func TestCascadeStrategy(t *testing.T) {
	// Fast provider che fallisce quality check
	fastProvider := &MockProvider{
		name:     "fast",
		latency:  50 * time.Millisecond,
		response: "Short.", // Troppo corta
	}

	// Slow provider di fallback
	slowProvider := &MockProvider{
		name:     "slow",
		latency:  200 * time.Millisecond,
		response: "This is a much longer and more detailed response that passes quality checks.",
	}

	// Crea pipeline con quality check
	pipeline := NewPipeline(NewCascadeStrategy(
		2*time.Second,
		true, // Quality check enabled
		20,   // Min 20 chars
	))

	pipeline.AddStage(Stage{
		Name:        "fast",
		Provider:    fastProvider,
		Model:       "fast-model",
		Transformer: &DefaultTransformer{},
		Timeout:     2 * time.Second,
		MaxRetries:  1,
	})

	pipeline.AddStage(Stage{
		Name:        "slow",
		Provider:    slowProvider,
		Model:       "slow-model",
		Transformer: &DefaultTransformer{},
		Timeout:     5 * time.Second,
		MaxRetries:  2,
	})

	ctx := context.Background()
	req := &providers.ChatRequest{
		Model: "test-model",
		Messages: []providers.Message{
			{Role: "user", Content: "Test"},
		},
	}

	result, err := pipeline.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Dovrebbe usare slow provider (fast ha fallito quality check)
	if len(result.StageOutputs) != 2 {
		t.Errorf("Expected 2 stage outputs, got %d", len(result.StageOutputs))
	}

	finalContent := result.FinalResponse.Choices[0].Message.Content.(string)
	if len(finalContent) < 20 {
		t.Error("Final response should pass quality check")
	}

	t.Logf("Used stages: %d", len(result.StageOutputs))
	t.Logf("Final response: %s", finalContent)
}

// TestParallelConsensusStrategy testa la strategia parallel consensus
func TestParallelConsensusStrategy(t *testing.T) {
	// Crea 3 provider con risposte diverse
	provider1 := &MockProvider{
		name:     "provider1",
		latency:  100 * time.Millisecond,
		response: "Response from provider 1",
	}

	provider2 := &MockProvider{
		name:     "provider2",
		latency:  150 * time.Millisecond,
		response: "Response from provider 2 with more details",
	}

	provider3 := &MockProvider{
		name:     "provider3",
		latency:  200 * time.Millisecond,
		response: "Response from provider 3 with even more details and information",
	}

	pipeline := NewPipeline(NewParallelConsensusStrategy("majority"))

	pipeline.AddStage(Stage{
		Name:        "p1",
		Provider:    provider1,
		Model:       "model1",
		Transformer: &DefaultTransformer{},
		Parallel:    true,
		Timeout:     5 * time.Second,
	})

	pipeline.AddStage(Stage{
		Name:        "p2",
		Provider:    provider2,
		Model:       "model2",
		Transformer: &DefaultTransformer{},
		Parallel:    true,
		Timeout:     5 * time.Second,
	})

	pipeline.AddStage(Stage{
		Name:        "p3",
		Provider:    provider3,
		Model:       "model3",
		Transformer: &DefaultTransformer{},
		Parallel:    true,
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()
	req := &providers.ChatRequest{
		Model:    "test-model",
		Messages: []providers.Message{{Role: "user", Content: "Test"}},
	}

	result, err := pipeline.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Tutti e 3 i provider dovrebbero essere eseguiti
	if len(result.StageOutputs) != 3 {
		t.Errorf("Expected 3 stage outputs, got %d", len(result.StageOutputs))
	}

	// La durata totale dovrebbe essere circa quella del più lento (parallelo)
	if result.TotalDuration > 300*time.Millisecond {
		t.Logf("Warning: parallel execution took longer than expected: %v", result.TotalDuration)
	}

	t.Logf("Parallel execution completed in %v", result.TotalDuration)
	t.Logf("Final response: %v", result.FinalResponse.Choices[0].Message.Content)
}

// TestSequentialStrategy testa la strategia sequenziale
func TestSequentialStrategy(t *testing.T) {
	provider1 := &MockProvider{
		name:     "step1",
		latency:  50 * time.Millisecond,
		response: "Step 1 output",
	}

	provider2 := &MockProvider{
		name:     "step2",
		latency:  50 * time.Millisecond,
		response: "Step 2 output",
	}

	pipeline := NewPipeline(NewSequentialStrategy())

	pipeline.AddStage(Stage{
		Name:        "step1",
		Provider:    provider1,
		Model:       "model1",
		Transformer: &DefaultTransformer{},
	})

	pipeline.AddStage(Stage{
		Name:        "step2",
		Provider:    provider2,
		Model:       "model2",
		Transformer: &DefaultTransformer{},
	})

	ctx := context.Background()
	req := &providers.ChatRequest{
		Model:    "test-model",
		Messages: []providers.Message{{Role: "user", Content: "Test"}},
	}

	result, err := pipeline.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	if len(result.StageOutputs) != 2 {
		t.Errorf("Expected 2 stage outputs, got %d", len(result.StageOutputs))
	}

	// La durata totale dovrebbe essere la somma (sequenziale)
	if result.TotalDuration < 100*time.Millisecond {
		t.Error("Sequential execution should take at least 100ms")
	}

	t.Logf("Sequential execution completed in %v", result.TotalDuration)
}

// TestPipelineMetrics testa la raccolta di metriche
func TestPipelineMetrics(t *testing.T) {
	provider := &MockProvider{
		name:     "test",
		latency:  100 * time.Millisecond,
		response: "Test response",
	}

	pipeline := NewPipeline(NewSequentialStrategy())
	pipeline.AddStage(Stage{
		Name:        "test-stage",
		Provider:    provider,
		Model:       "test-model",
		Transformer: &DefaultTransformer{},
	})

	ctx := context.Background()
	req := &providers.ChatRequest{
		Model:    "test-model",
		Messages: []providers.Message{{Role: "user", Content: "Test"}},
	}

	// Esegui pipeline più volte
	for i := 0; i < 5; i++ {
		_, err := pipeline.Execute(ctx, req)
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
	}

	// Verifica metriche
	metrics := pipeline.GetMetrics()

	if metrics.TotalExecutions != 5 {
		t.Errorf("Expected 5 total executions, got %d", metrics.TotalExecutions)
	}

	if metrics.SuccessfulRuns != 5 {
		t.Errorf("Expected 5 successful runs, got %d", metrics.SuccessfulRuns)
	}

	if metrics.FailedRuns != 0 {
		t.Errorf("Expected 0 failed runs, got %d", metrics.FailedRuns)
	}

	if metrics.TotalTokens != 150 { // 30 tokens * 5 executions
		t.Errorf("Expected 150 total tokens, got %d", metrics.TotalTokens)
	}

	// Verifica metriche dello stage
	stageMetrics := metrics.StageMetrics["test-stage"]
	if stageMetrics == nil {
		t.Fatal("Expected stage metrics for test-stage")
	}

	if stageMetrics.Executions != 5 {
		t.Errorf("Expected 5 stage executions, got %d", stageMetrics.Executions)
	}

	t.Logf("Total executions: %d", metrics.TotalExecutions)
	t.Logf("Success rate: %.2f%%", float64(metrics.SuccessfulRuns)/float64(metrics.TotalExecutions)*100)
	t.Logf("Average duration: %v", metrics.AverageDuration)
	t.Logf("Total tokens: %d", metrics.TotalTokens)
}
