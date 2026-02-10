package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

func TestDeduplicateCandidates(t *testing.T) {
	logger := zerolog.Nop()
	config := &DiscoveryConfig{
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
	}

	engine := NewEngine(config, nil, logger)

	candidates := []Candidate{
		{Name: "API1", BaseURL: "https://api1.com"},
		{Name: "API2", BaseURL: "https://api2.com"},
		{Name: "API1-dup", BaseURL: "https://api1.com"},
		{Name: "API3", BaseURL: "https://api3.com"},
	}

	unique := engine.deduplicateCandidates(candidates)

	if len(unique) != 3 {
		t.Errorf("Expected 3 unique candidates, got %d", len(unique))
	}

	// Verifica che API1 sia presente una sola volta
	count := 0
	for _, c := range unique {
		if c.BaseURL == "https://api1.com" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 1 instance of API1, got %d", count)
	}
}

func TestFilterExisting(t *testing.T) {
	logger := zerolog.Nop()
	config := &DiscoveryConfig{
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
	}

	engine := NewEngine(config, nil, logger)

	candidates := []Candidate{
		{Name: "API1", BaseURL: "https://api1.com"},
		{Name: "API2", BaseURL: "https://api2.com"},
		{Name: "API3", BaseURL: "https://api3.com"},
	}

	existing := []models.Provider{
		{Name: "Existing1", BaseURL: "https://api1.com"},
	}

	filtered := engine.filterExisting(candidates, existing)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered candidates, got %d", len(filtered))
	}

	// Verifica che API1 sia stato filtrato
	for _, c := range filtered {
		if c.BaseURL == "https://api1.com" {
			t.Error("API1 should have been filtered out")
		}
	}
}

func TestCandidateToProvider(t *testing.T) {
	logger := zerolog.Nop()
	config := &DiscoveryConfig{
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
	}

	engine := NewEngine(config, nil, logger)

	candidate := Candidate{
		Name:        "TestAPI",
		BaseURL:     "https://testapi.com",
		AuthType:    models.AuthTypeAPIKey,
		Source:      "github",
		Description: "Test API",
		Stars:       150,
	}

	result := &ValidationResult{
		HealthScore:       0.8,
		LatencyMs:         500,
		SupportsStreaming: true,
		SupportsJSON:      true,
		SupportsTools:     true,
	}

	provider := engine.candidateToProvider(candidate, result)

	if provider.Name != "TestAPI" {
		t.Errorf("Expected name 'TestAPI', got '%s'", provider.Name)
	}

	if provider.BaseURL != "https://testapi.com" {
		t.Errorf("Expected baseURL 'https://testapi.com', got '%s'", provider.BaseURL)
	}

	if provider.AuthType != models.AuthTypeAPIKey {
		t.Errorf("Expected AuthType APIKey, got '%s'", provider.AuthType)
	}

	if provider.Tier != 2 {
		t.Errorf("Expected tier 2 (100+ stars), got %d", provider.Tier)
	}

	if provider.HealthScore != 0.8 {
		t.Errorf("Expected health score 0.8, got %f", provider.HealthScore)
	}

	if !provider.SupportsStreaming {
		t.Error("Expected supports_streaming to be true")
	}
}

func TestEngineStartStop(t *testing.T) {
	logger := zerolog.Nop()
	config := &DiscoveryConfig{
		Enabled:           true,
		Interval:          1 * time.Hour,
		MaxConcurrent:     5,
		ValidationTimeout: 30 * time.Second,
		MinHealthScore:    0.6,
	}

	engine := NewEngine(config, nil, logger)

	if engine.IsRunning() {
		t.Error("Engine should not be running initially")
	}

	ctx := context.Background()
	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Piccolo delay per permettere al goroutine di avviarsi
	time.Sleep(100 * time.Millisecond)

	if !engine.IsRunning() {
		t.Error("Engine should be running after Start()")
	}

	engine.Stop()

	// Piccolo delay per permettere al goroutine di fermarsi
	time.Sleep(100 * time.Millisecond)

	if engine.IsRunning() {
		t.Error("Engine should not be running after Stop()")
	}
}

func TestDefaultConfig(t *testing.T) {
	logger := zerolog.Nop()
	config := &DiscoveryConfig{}

	engine := NewEngine(config, nil, logger)

	if engine.config.MaxConcurrent != 5 {
		t.Errorf("Expected default MaxConcurrent=5, got %d", engine.config.MaxConcurrent)
	}

	if engine.config.ValidationTimeout != 30*time.Second {
		t.Errorf("Expected default ValidationTimeout=30s, got %v", engine.config.ValidationTimeout)
	}

	if engine.config.MinHealthScore != 0.6 {
		t.Errorf("Expected default MinHealthScore=0.6, got %f", engine.config.MinHealthScore)
	}

	if len(engine.config.DiscoverySearchTerms) == 0 {
		t.Error("Expected default search terms to be populated")
	}
}
