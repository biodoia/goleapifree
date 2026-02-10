package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

func TestValidatorBasicConnectivity(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewValidator(10*time.Second, logger)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	ctx := context.Background()
	result := &ValidationResult{}

	err := validator.testConnectivity(ctx, server.URL, result)
	if err != nil {
		t.Fatalf("Connectivity test failed: %v", err)
	}

	if result.LatencyMs <= 0 {
		t.Error("Expected positive latency")
	}
}

func TestCalculateHealthScore(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewValidator(10*time.Second, logger)

	tests := []struct {
		name          string
		result        ValidationResult
		expectedMin   float64
		expectedMax   float64
	}{
		{
			name: "Basic connectivity only",
			result: ValidationResult{
				LatencyMs: 1000,
			},
			expectedMin: 0.3,
			expectedMax: 0.5,
		},
		{
			name: "Low latency with features",
			result: ValidationResult{
				Compatibility:     "openai",
				LatencyMs:         300,
				SupportsStreaming: true,
				SupportsJSON:      true,
				SupportsTools:     true,
				AvailableModels:   []string{"gpt-3.5-turbo"},
			},
			expectedMin: 0.9,
			expectedMax: 1.1,
		},
		{
			name: "High latency",
			result: ValidationResult{
				Compatibility: "openai",
				LatencyMs:     3000,
				SupportsJSON:  true,
			},
			expectedMin: 0.5,
			expectedMax: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator.calculateHealthScore(&tt.result)

			if tt.result.HealthScore < tt.expectedMin {
				t.Errorf("Health score %f below expected minimum %f",
					tt.result.HealthScore, tt.expectedMin)
			}

			if tt.result.HealthScore > tt.expectedMax {
				t.Errorf("Health score %f above expected maximum %f",
					tt.result.HealthScore, tt.expectedMax)
			}
		})
	}
}

func TestDetectCompatibility(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewValidator(10*time.Second, logger)

	tests := []struct {
		name              string
		baseURL           string
		expectedCompat    string
	}{
		{
			name:           "OpenAI URL",
			baseURL:        "https://api.openai.com/v1",
			expectedCompat: "openai",
		},
		{
			name:           "OpenAI-like completions",
			baseURL:        "https://api.example.com/v1/chat/completions",
			expectedCompat: "openai",
		},
		{
			name:           "Anthropic URL",
			baseURL:        "https://api.anthropic.com/v1",
			expectedCompat: "anthropic",
		},
		{
			name:           "Claude messages",
			baseURL:        "https://api.example.com/v1/messages",
			expectedCompat: "anthropic",
		},
		{
			name:           "Unknown API",
			baseURL:        "https://api.example.com/v1",
			expectedCompat: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := &ValidationResult{}

			validator.detectCompatibility(ctx, tt.baseURL, models.AuthTypeNone, result)

			if result.Compatibility != tt.expectedCompat {
				t.Errorf("Expected compatibility '%s', got '%s'",
					tt.expectedCompat, result.Compatibility)
			}
		})
	}
}

func TestAddAuthHeader(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewValidator(10*time.Second, logger)

	tests := []struct {
		name         string
		authType     models.AuthType
		key          string
		expectedAuth string
	}{
		{
			name:         "API Key",
			authType:     models.AuthTypeAPIKey,
			key:          "test-key",
			expectedAuth: "Bearer test-key",
		},
		{
			name:         "Bearer Token",
			authType:     models.AuthTypeBearer,
			key:          "test-token",
			expectedAuth: "Bearer test-token",
		},
		{
			name:         "No Auth",
			authType:     models.AuthTypeNone,
			key:          "",
			expectedAuth: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://example.com", nil)
			validator.addAuthHeader(req, tt.authType, tt.key)

			authHeader := req.Header.Get("Authorization")
			if authHeader != tt.expectedAuth {
				t.Errorf("Expected Authorization header '%s', got '%s'",
					tt.expectedAuth, authHeader)
			}
		})
	}
}

func TestMeasureLatency(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewValidator(10*time.Second, logger)

	// Mock server con delay controllato
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	avgMs, err := validator.MeasureLatency(ctx, server.URL, 3)

	if err != nil {
		t.Fatalf("MeasureLatency failed: %v", err)
	}

	// Latenza dovrebbe essere circa 50ms
	if avgMs < 40 || avgMs > 100 {
		t.Errorf("Expected latency around 50ms, got %dms", avgMs)
	}
}

func TestValidateEndpointIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.Nop()
	validator := NewValidator(30*time.Second, logger)

	// Mock OpenAI-compatible server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "gpt-3.5-turbo"}]}`))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	result, err := validator.ValidateEndpoint(ctx, server.URL, models.AuthTypeNone)

	if err != nil {
		t.Fatalf("ValidateEndpoint failed: %v", err)
	}

	if !result.IsValid {
		t.Error("Expected endpoint to be valid")
	}

	if result.HealthScore <= 0.3 {
		t.Errorf("Expected health score > 0.3, got %f", result.HealthScore)
	}

	if result.Compatibility == "unknown" {
		t.Error("Expected compatibility to be detected")
	}
}
