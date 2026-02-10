package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Routing: config.RoutingConfig{
			Strategy: "cost_optimized",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval: "5m",
		},
		Monitoring: config.MonitoringConfig{
			Prometheus: config.PrometheusConfig{
				Enabled: false,
			},
		},
	}

	gateway, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if gateway == nil {
		t.Fatal("New() returned nil gateway")
	}

	if gateway.config == nil {
		t.Error("Gateway config not set")
	}

	if gateway.app == nil {
		t.Error("Gateway app not initialized")
	}

	if gateway.router == nil {
		t.Error("Gateway router not initialized")
	}

	if gateway.health == nil {
		t.Error("Gateway health monitor not initialized")
	}
}

func TestGateway_Shutdown(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8081,
		},
		Routing: config.RoutingConfig{
			Strategy: "cost_optimized",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval: "5m",
		},
		Monitoring: config.MonitoringConfig{
			Prometheus: config.PrometheusConfig{
				Enabled: false,
			},
		},
	}

	gateway, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start health monitoring
	gateway.health.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := gateway.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestGateway_Routes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8082,
		},
		Routing: config.RoutingConfig{
			Strategy: "cost_optimized",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval: "5m",
		},
		Monitoring: config.MonitoringConfig{
			Prometheus: config.PrometheusConfig{
				Enabled: true,
			},
		},
	}

	gateway, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if gateway.app == nil {
		t.Fatal("Gateway app not initialized")
	}

	// Check that routes are registered
	routes := gateway.app.GetRoutes(true)
	if len(routes) == 0 {
		t.Error("No routes registered")
	}

	expectedRoutes := []string{
		"/v1/chat/completions",
		"/v1/messages",
		"/v1/models",
		"/health",
		"/ready",
		"/metrics",
		"/admin/providers",
		"/admin/stats",
	}

	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s not found", expected)
		}
	}
}

func TestGateway_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8083,
				},
				Routing: config.RoutingConfig{
					Strategy: "cost_optimized",
				},
				Providers: config.ProvidersConfig{
					HealthCheckInterval: "5m",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with prometheus",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8084,
				},
				Routing: config.RoutingConfig{
					Strategy: "latency_first",
				},
				Providers: config.ProvidersConfig{
					HealthCheckInterval: "10m",
				},
				Monitoring: config.MonitoringConfig{
					Prometheus: config.PrometheusConfig{
						Enabled: true,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := New(tt.config, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}

			if gateway == nil && !tt.wantErr {
				t.Error("New() returned nil gateway")
			}
		})
	}
}

// Integration test for gateway lifecycle
func TestGateway_Lifecycle(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8085,
		},
		Routing: config.RoutingConfig{
			Strategy: "cost_optimized",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval: "5m",
		},
		Monitoring: config.MonitoringConfig{
			Prometheus: config.PrometheusConfig{
				Enabled: false,
			},
		},
	}

	// Create gateway
	gateway, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start health monitoring
	gateway.health.Start()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := gateway.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

// Benchmark tests
func BenchmarkGateway_New(b *testing.B) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8086,
		},
		Routing: config.RoutingConfig{
			Strategy: "cost_optimized",
		},
		Providers: config.ProvidersConfig{
			HealthCheckInterval: "5m",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(cfg, nil)
	}
}
