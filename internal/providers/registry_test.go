package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockProvider implementa Provider per i test
type MockProvider struct {
	name         string
	features     map[Feature]bool
	healthErr    error
	completionErr error
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		features: map[Feature]bool{
			FeatureStreaming: true,
			FeatureTools:     true,
			FeatureJSONMode:  true,
		},
	}
}

func (m *MockProvider) Name() string { return m.name }

func (m *MockProvider) SupportsFeature(f Feature) bool {
	if v, ok := m.features[f]; ok {
		return v
	}
	return false
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if m.completionErr != nil {
		return nil, m.completionErr
	}
	return &ChatResponse{
		ID:      "test-id",
		Model:   req.Model,
		Choices: []Choice{{Index: 0, Message: Message{Role: "assistant", Content: "test response"}}},
	}, nil
}

func (m *MockProvider) Stream(ctx context.Context, req *ChatRequest, handler StreamHandler) error {
	return handler(&StreamChunk{Delta: "test", Done: true})
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func (m *MockProvider) GetModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{ID: "test-model", Name: "Test Model"}}, nil
}

// Tests

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	err := registry.Register("test", provider, "mock")

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 provider, got %d", registry.Count())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	err := registry.Register("test", provider, "mock")
	if !errors.Is(err, ErrProviderAlreadyExists) {
		t.Errorf("Expected ErrProviderAlreadyExists, got %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	retrieved, err := registry.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected provider 'test', got '%s'", retrieved.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("Expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_GetFirst(t *testing.T) {
	registry := NewRegistry()

	provider1 := NewMockProvider("test1")
	provider2 := NewMockProvider("test2")

	registry.Register("test1", provider1, "mock")
	registry.Register("test2", provider2, "mock")

	first, err := registry.GetFirst()
	if err != nil {
		t.Fatalf("GetFirst failed: %v", err)
	}

	if first == nil {
		t.Error("Expected provider, got nil")
	}
}

func TestRegistry_GetFirstEmpty(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.GetFirst()
	if !errors.Is(err, ErrNoProvidersAvailable) {
		t.Errorf("Expected ErrNoProvidersAvailable, got %v", err)
	}
}

func TestRegistry_GetOrFirst(t *testing.T) {
	registry := NewRegistry()

	provider1 := NewMockProvider("test1")
	provider2 := NewMockProvider("test2")

	registry.Register("test1", provider1, "mock")
	registry.Register("test2", provider2, "mock")

	// Get specific provider
	p, err := registry.GetOrFirst("test1")
	if err != nil {
		t.Fatalf("GetOrFirst failed: %v", err)
	}
	if p.Name() != "test1" {
		t.Errorf("Expected test1, got %s", p.Name())
	}

	// Get first when specific not found
	p, err = registry.GetOrFirst("nonexistent")
	if err != nil {
		t.Fatalf("GetOrFirst fallback failed: %v", err)
	}
	if p == nil {
		t.Error("Expected provider, got nil")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	registry.Register("test1", NewMockProvider("test1"), "mock")
	registry.Register("test2", NewMockProvider("test2"), "mock")

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(list))
	}
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	err := registry.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 providers, got %d", registry.Count())
	}
}

func TestRegistry_SetStatus(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	err := registry.SetStatus("test", ProviderStatusInactive)
	if err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	meta, _ := registry.GetMetadata("test")
	if meta.Status != ProviderStatusInactive {
		t.Errorf("Expected inactive status, got %s", meta.Status)
	}
}

func TestRegistry_HealthCheck(t *testing.T) {
	registry := NewRegistry()

	// Healthy provider
	provider1 := NewMockProvider("healthy")
	provider1.healthErr = nil
	registry.Register("healthy", provider1, "mock")

	// Unhealthy provider
	provider2 := NewMockProvider("unhealthy")
	provider2.healthErr = errors.New("health check failed")
	registry.Register("unhealthy", provider2, "mock")

	ctx := context.Background()
	results := registry.HealthCheck(ctx)

	if results["healthy"] != nil {
		t.Errorf("Expected healthy provider to pass, got error: %v", results["healthy"])
	}

	if results["unhealthy"] == nil {
		t.Error("Expected unhealthy provider to fail")
	}

	// Check metadata
	meta1, _ := registry.GetMetadata("healthy")
	if meta1.HealthCheckStatus != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", meta1.HealthCheckStatus)
	}

	meta2, _ := registry.GetMetadata("unhealthy")
	if meta2.HealthCheckStatus != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", meta2.HealthCheckStatus)
	}
}

func TestRegistry_RecordSuccess(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	registry.RecordSuccess("test", 100*time.Millisecond)

	meta, _ := registry.GetMetadata("test")
	if meta.SuccessCount != 1 {
		t.Errorf("Expected success count 1, got %d", meta.SuccessCount)
	}

	if meta.AvgLatency != 100*time.Millisecond {
		t.Errorf("Expected latency 100ms, got %v", meta.AvgLatency)
	}
}

func TestRegistry_RecordError(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	registry.RecordError("test")

	meta, _ := registry.GetMetadata("test")
	if meta.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", meta.ErrorCount)
	}
}

func TestRegistry_GetStats(t *testing.T) {
	registry := NewRegistry()

	provider1 := NewMockProvider("test1")
	provider2 := NewMockProvider("test2")

	registry.Register("test1", provider1, "mock")
	registry.Register("test2", provider2, "mock")

	registry.RecordSuccess("test1", 100*time.Millisecond)
	registry.RecordError("test2")

	stats := registry.GetStats()

	if stats.TotalProviders != 2 {
		t.Errorf("Expected 2 total providers, got %d", stats.TotalProviders)
	}

	if stats.ActiveProviders != 2 {
		t.Errorf("Expected 2 active providers, got %d", stats.ActiveProviders)
	}

	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", stats.TotalRequests)
	}

	if stats.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.TotalErrors)
	}
}

func TestRegistry_Clear(t *testing.T) {
	registry := NewRegistry()

	registry.Register("test1", NewMockProvider("test1"), "mock")
	registry.Register("test2", NewMockProvider("test2"), "mock")

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Expected 0 providers after clear, got %d", registry.Count())
	}
}

func TestRegistry_GetMetadata(t *testing.T) {
	registry := NewRegistry()

	provider := NewMockProvider("test")
	registry.Register("test", provider, "mock")

	meta, err := registry.GetMetadata("test")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if meta.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", meta.Name)
	}

	if meta.Type != "mock" {
		t.Errorf("Expected type 'mock', got '%s'", meta.Type)
	}

	if meta.Status != ProviderStatusActive {
		t.Errorf("Expected active status, got %s", meta.Status)
	}
}

func TestRegistry_GetAllMetadata(t *testing.T) {
	registry := NewRegistry()

	registry.Register("test1", NewMockProvider("test1"), "mock")
	registry.Register("test2", NewMockProvider("test2"), "mock")

	allMeta := registry.GetAllMetadata()

	if len(allMeta) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(allMeta))
	}

	if allMeta["test1"] == nil {
		t.Error("Expected metadata for test1")
	}

	if allMeta["test2"] == nil {
		t.Error("Expected metadata for test2")
	}
}

func TestRegistry_ListActive(t *testing.T) {
	registry := NewRegistry()

	registry.Register("test1", NewMockProvider("test1"), "mock")
	registry.Register("test2", NewMockProvider("test2"), "mock")
	registry.Register("test3", NewMockProvider("test3"), "mock")

	// Set test2 as inactive
	registry.SetStatus("test2", ProviderStatusInactive)

	active := registry.ListActive()

	if len(active) != 2 {
		t.Errorf("Expected 2 active providers, got %d", len(active))
	}

	// Check that test2 is not in active list
	for _, name := range active {
		if name == "test2" {
			t.Error("test2 should not be in active list")
		}
	}
}

func TestBaseProvider_Features(t *testing.T) {
	base := NewBaseProvider("test", "http://test.com", "key")

	// Test default features
	if !base.SupportsFeature(FeatureStreaming) {
		t.Error("Expected streaming to be supported by default")
	}

	// Test setting features
	base.SetFeature(FeatureTools, true)
	if !base.SupportsFeature(FeatureTools) {
		t.Error("Expected tools to be supported after setting")
	}

	base.SetFeature(FeatureTools, false)
	if base.SupportsFeature(FeatureTools) {
		t.Error("Expected tools to not be supported after unsetting")
	}
}

func TestBaseProvider_Configuration(t *testing.T) {
	base := NewBaseProvider("test", "http://test.com", "key")

	// Test timeout
	base.SetTimeout(60 * time.Second)
	if base.GetTimeout() != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", base.GetTimeout())
	}

	// Test max retries
	base.SetMaxRetries(5)
	if base.GetMaxRetries() != 5 {
		t.Errorf("Expected max retries 5, got %d", base.GetMaxRetries())
	}

	// Test name
	if base.Name() != "test" {
		t.Errorf("Expected name 'test', got '%s'", base.Name())
	}

	// Test base URL
	if base.GetBaseURL() != "http://test.com" {
		t.Errorf("Expected base URL 'http://test.com', got '%s'", base.GetBaseURL())
	}

	// Test API key
	if base.GetAPIKey() != "key" {
		t.Errorf("Expected API key 'key', got '%s'", base.GetAPIKey())
	}
}
