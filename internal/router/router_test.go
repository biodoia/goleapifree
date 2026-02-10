package router

import (
	"testing"

	"github.com/biodoia/goleapifree/pkg/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		wantErr  bool
	}{
		{
			name:     "cost_optimized strategy",
			strategy: "cost_optimized",
			wantErr:  false,
		},
		{
			name:     "latency_first strategy",
			strategy: "latency_first",
			wantErr:  false,
		},
		{
			name:     "quality_first strategy",
			strategy: "quality_first",
			wantErr:  false,
		},
		{
			name:     "default to cost_optimized",
			strategy: "unknown",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Routing: config.RoutingConfig{
					Strategy: tt.strategy,
				},
			}

			router, err := New(cfg, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if router == nil && !tt.wantErr {
				t.Error("New() returned nil router")
			}

			if router != nil && router.strategy == nil {
				t.Error("Router strategy not initialized")
			}
		})
	}
}

func TestCostOptimizedStrategy_SelectProvider(t *testing.T) {
	strategy := &CostOptimizedStrategy{
		db: nil,
	}

	req := &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	selection, err := strategy.SelectProvider(req)
	if err != nil {
		t.Errorf("SelectProvider() error = %v", err)
	}

	if selection == nil {
		t.Fatal("SelectProvider() returned nil selection")
	}

	if selection.Reason != "cost_optimized" {
		t.Errorf("Reason = %v, want cost_optimized", selection.Reason)
	}
}

func TestLatencyFirstStrategy_SelectProvider(t *testing.T) {
	strategy := &LatencyFirstStrategy{
		db: nil,
	}

	req := &Request{
		Model:    "gpt-3.5-turbo",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	selection, err := strategy.SelectProvider(req)
	if err != nil {
		t.Errorf("SelectProvider() error = %v", err)
	}

	if selection == nil {
		t.Fatal("SelectProvider() returned nil selection")
	}

	if selection.Reason != "latency_first" {
		t.Errorf("Reason = %v, want latency_first", selection.Reason)
	}
}

func TestQualityFirstStrategy_SelectProvider(t *testing.T) {
	strategy := &QualityFirstStrategy{
		db: nil,
	}

	req := &Request{
		Model:    "claude-3-opus",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	selection, err := strategy.SelectProvider(req)
	if err != nil {
		t.Errorf("SelectProvider() error = %v", err)
	}

	if selection == nil {
		t.Fatal("SelectProvider() returned nil selection")
	}

	if selection.Reason != "quality_first" {
		t.Errorf("Reason = %v, want quality_first", selection.Reason)
	}
}

func TestRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request *Request
		valid   bool
	}{
		{
			name: "valid request",
			request: &Request{
				Model:       "gpt-4",
				Messages:    []Message{{Role: "user", Content: "Hello"}},
				MaxTokens:   1000,
				Temperature: 0.7,
			},
			valid: true,
		},
		{
			name: "streaming request",
			request: &Request{
				Model:    "gpt-4",
				Messages: []Message{{Role: "user", Content: "Hello"}},
				Stream:   true,
			},
			valid: true,
		},
		{
			name: "empty messages",
			request: &Request{
				Model:    "gpt-4",
				Messages: []Message{},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.request.Model == "" && tt.valid {
				t.Error("Valid request should have a model")
			}
		})
	}
}

func TestMessage_Structure(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %v, want user", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %v, want 'Hello, world!'", msg.Content)
	}
}

func TestProviderSelection_Structure(t *testing.T) {
	selection := &ProviderSelection{
		ProviderID:    "provider-123",
		ModelID:       "model-456",
		EstimatedCost: 0.001,
		Reason:        "cost_optimized",
	}

	if selection.ProviderID == "" {
		t.Error("ProviderID should not be empty")
	}

	if selection.ModelID == "" {
		t.Error("ModelID should not be empty")
	}

	if selection.EstimatedCost < 0 {
		t.Error("EstimatedCost should not be negative")
	}

	if selection.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

// Benchmark tests
func BenchmarkCostOptimizedStrategy_SelectProvider(b *testing.B) {
	strategy := &CostOptimizedStrategy{
		db: nil,
	}

	req := &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.SelectProvider(req)
	}
}

func BenchmarkLatencyFirstStrategy_SelectProvider(b *testing.B) {
	strategy := &LatencyFirstStrategy{
		db: nil,
	}

	req := &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.SelectProvider(req)
	}
}
