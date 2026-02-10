package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProvider_BeforeCreate(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name: "generates UUID if nil",
			provider: &Provider{
				Name:    "test-provider",
				BaseURL: "https://api.test.com",
			},
			wantErr: false,
		},
		{
			name: "keeps existing UUID",
			provider: &Provider{
				ID:      uuid.New(),
				Name:    "test-provider",
				BaseURL: "https://api.test.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.provider.ID
			err := tt.provider.BeforeCreate()

			if (err != nil) != tt.wantErr {
				t.Errorf("BeforeCreate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.provider.ID == uuid.Nil {
				t.Error("ID should not be nil after BeforeCreate()")
			}

			if originalID != uuid.Nil && tt.provider.ID != originalID {
				t.Error("Existing ID should not be changed")
			}

			if tt.provider.DiscoveredAt.IsZero() {
				t.Error("DiscoveredAt should be set after BeforeCreate()")
			}
		})
	}
}

func TestProvider_IsAvailable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		provider *Provider
		want     bool
	}{
		{
			name: "active provider with good health",
			provider: &Provider{
				Status:          ProviderStatusActive,
				HealthScore:     0.9,
				LastHealthCheck: now.Add(-5 * time.Minute),
			},
			want: true,
		},
		{
			name: "inactive provider",
			provider: &Provider{
				Status:          ProviderStatusDown,
				HealthScore:     0.9,
				LastHealthCheck: now.Add(-5 * time.Minute),
			},
			want: false,
		},
		{
			name: "low health score",
			provider: &Provider{
				Status:          ProviderStatusActive,
				HealthScore:     0.3,
				LastHealthCheck: now.Add(-5 * time.Minute),
			},
			want: false,
		},
		{
			name: "stale health check",
			provider: &Provider{
				Status:          ProviderStatusActive,
				HealthScore:     0.9,
				LastHealthCheck: now.Add(-15 * time.Minute),
			},
			want: false,
		},
		{
			name: "deprecated provider",
			provider: &Provider{
				Status:          ProviderStatusDeprecated,
				HealthScore:     1.0,
				LastHealthCheck: now,
			},
			want: false,
		},
		{
			name: "maintenance mode",
			provider: &Provider{
				Status:          ProviderStatusMaintenance,
				HealthScore:     1.0,
				LastHealthCheck: now,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.IsAvailable(); got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		provType ProviderType
		expected string
	}{
		{"free type", ProviderTypeFree, "free"},
		{"freemium type", ProviderTypeFreemium, "freemium"},
		{"paid type", ProviderTypePaid, "paid"},
		{"local type", ProviderTypeLocal, "local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.provType) != tt.expected {
				t.Errorf("ProviderType = %v, want %v", tt.provType, tt.expected)
			}
		})
	}
}

func TestProviderStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   ProviderStatus
		expected string
	}{
		{"active status", ProviderStatusActive, "active"},
		{"deprecated status", ProviderStatusDeprecated, "deprecated"},
		{"down status", ProviderStatusDown, "down"},
		{"maintenance status", ProviderStatusMaintenance, "maintenance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("ProviderStatus = %v, want %v", tt.status, tt.expected)
			}
		})
	}
}

func TestAuthType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		authType AuthType
		expected string
	}{
		{"none auth", AuthTypeNone, "none"},
		{"api key auth", AuthTypeAPIKey, "api_key"},
		{"bearer auth", AuthTypeBearer, "bearer"},
		{"oauth2 auth", AuthTypeOAuth2, "oauth2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.authType) != tt.expected {
				t.Errorf("AuthType = %v, want %v", tt.authType, tt.expected)
			}
		})
	}
}

func TestProvider_TableName(t *testing.T) {
	p := Provider{}
	if got := p.TableName(); got != "providers" {
		t.Errorf("TableName() = %v, want providers", got)
	}
}

// Benchmark tests
func BenchmarkProvider_IsAvailable(b *testing.B) {
	provider := &Provider{
		Status:          ProviderStatusActive,
		HealthScore:     0.9,
		LastHealthCheck: time.Now().Add(-5 * time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.IsAvailable()
	}
}

func BenchmarkProvider_BeforeCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider := &Provider{
			Name:    "test-provider",
			BaseURL: "https://api.test.com",
		}
		_ = provider.BeforeCreate()
	}
}
