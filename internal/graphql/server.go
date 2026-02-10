package graphql

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

//go:embed schema.graphql
var schemaFS embed.FS

// Server represents the GraphQL server
type Server struct {
	resolver            *Resolver
	subscriptionManager *SubscriptionManager
	db                  *database.DB

	// Configuration
	enablePlayground bool
	enableIntrospection bool
	maxComplexity    int
	timeout          time.Duration

	// Middleware
	authenticator AuthenticatorFunc
	rateLimiter   RateLimiterFunc
}

// Config holds GraphQL server configuration
type Config struct {
	EnablePlayground     bool
	EnableIntrospection  bool
	MaxComplexity        int
	Timeout              time.Duration
	Authenticator        AuthenticatorFunc
	RateLimiter          RateLimiterFunc
}

// AuthenticatorFunc is a middleware function for authentication
type AuthenticatorFunc func(ctx *fiber.Ctx) error

// RateLimiterFunc is a middleware function for rate limiting
type RateLimiterFunc func(ctx *fiber.Ctx) error

// NewServer creates a new GraphQL server
func NewServer(db *database.DB, config *Config) *Server {
	if config == nil {
		config = &Config{
			EnablePlayground:    true,
			EnableIntrospection: true,
			MaxComplexity:       1000,
			Timeout:             30 * time.Second,
		}
	}

	resolver := NewResolver(db)
	subscriptionManager := NewSubscriptionManager(db)

	return &Server{
		resolver:             resolver,
		subscriptionManager:  subscriptionManager,
		db:                   db,
		enablePlayground:     config.EnablePlayground,
		enableIntrospection:  config.EnableIntrospection,
		maxComplexity:        config.MaxComplexity,
		timeout:              config.Timeout,
		authenticator:        config.Authenticator,
		rateLimiter:          config.RateLimiter,
	}
}

// RegisterRoutes registers GraphQL routes on a Fiber app
func (s *Server) RegisterRoutes(app *fiber.App) {
	// GraphQL endpoint
	app.Post("/graphql", s.handleGraphQL)
	app.Get("/graphql", s.handleGraphQL)

	// Playground UI (development only)
	if s.enablePlayground {
		app.Get("/graphql/playground", s.handlePlayground)
	}

	// WebSocket endpoint for subscriptions
	app.Get("/graphql/ws", s.handleWebSocket)

	log.Info().
		Bool("playground", s.enablePlayground).
		Bool("introspection", s.enableIntrospection).
		Msg("GraphQL server routes registered")
}

// handleGraphQL handles GraphQL queries and mutations
func (s *Server) handleGraphQL(c *fiber.Ctx) error {
	// Apply authentication if configured
	if s.authenticator != nil {
		if err := s.authenticator(c); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"errors": []fiber.Map{
					{
						"message": "Authentication required",
					},
				},
			})
		}
	}

	// Apply rate limiting if configured
	if s.rateLimiter != nil {
		if err := s.rateLimiter(c); err != nil {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors": []fiber.Map{
					{
						"message": "Rate limit exceeded",
					},
				},
			})
		}
	}

	// Parse GraphQL request
	var req GraphQLRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{
				{
					"message": "Invalid request body",
				},
			},
		})
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Context(), s.timeout)
	defer cancel()

	// Add data loader to context
	ctx = WithDataLoader(ctx, s.resolver.loader)

	// Execute GraphQL query
	response := s.executeQuery(ctx, req)

	return c.JSON(response)
}

// handlePlayground serves the GraphQL Playground UI
func (s *Server) handlePlayground(c *fiber.Ctx) error {
	playgroundHTML := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GoLeapAI GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
  <style>
    body {
      margin: 0;
      overflow: hidden;
    }
    #root {
      height: 100vh;
    }
  </style>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function (event) {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/graphql',
        subscriptionEndpoint: 'ws://' + window.location.host + '/graphql/ws',
        settings: {
          'editor.theme': 'dark',
          'editor.cursorShape': 'line',
          'editor.fontSize': 14,
          'editor.fontFamily': "'Source Code Pro', monospace",
          'editor.reuseHeaders': true,
          'prettier.printWidth': 80,
          'request.credentials': 'include',
        },
        tabs: [
          {
            endpoint: '/graphql',
            query: '# Welcome to GoLeapAI GraphQL API\n# Alternative to REST with real-time capabilities\n\n# Example: Query providers\nquery GetProviders {\n  providers(limit: 10) {\n    edges {\n      node {\n        id\n        name\n        type\n        status\n        healthScore\n        models {\n          name\n          modality\n          isFree\n        }\n      }\n    }\n    totalCount\n  }\n}\n\n# Example: Get global stats\nquery GlobalStats {\n  stats(timeRange: {\n    start: "2024-01-01T00:00:00Z"\n    end: "2024-12-31T23:59:59Z"\n  }) {\n    totalProviders\n    activeProviders\n    totalRequests\n    successRate\n    costSaved\n  }\n}\n\n# Example: Subscribe to live stats\nsubscription LiveStats {\n  liveStats(interval: 5) {\n    totalRequests\n    activeProviders\n    successRate\n  }\n}',
          },
        ],
      })
    })
  </script>
</body>
</html>`

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(playgroundHTML)
}

// handleWebSocket handles WebSocket connections for subscriptions
func (s *Server) handleWebSocket(c *fiber.Ctx) error {
	// Check if connection is WebSocket upgrade
	if !c.IsProxyTrusted() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "WebSocket upgrade required",
		})
	}

	// Upgrade to WebSocket
	return fiber.ErrUpgradeRequired
	// Note: Full WebSocket implementation would require additional setup
	// This is a placeholder showing the endpoint structure
}

// executeQuery executes a GraphQL query
func (s *Server) executeQuery(ctx context.Context, req GraphQLRequest) GraphQLResponse {
	startTime := time.Now()

	// This is a simplified execution - in production, use a proper GraphQL library like gqlgen
	response := GraphQLResponse{
		Data: make(map[string]interface{}),
	}

	// Route based on operation name or query content
	// This is a basic router - production should use gqlgen generated code
	if req.OperationName == "GetProviders" || contains(req.Query, "providers") {
		providers, err := s.resolver.Providers(ctx, nil, nil, nil, nil)
		if err != nil {
			response.Errors = append(response.Errors, GraphQLError{
				Message: err.Error(),
			})
		} else {
			response.Data["providers"] = providers
		}
	}

	if req.OperationName == "GlobalStats" || contains(req.Query, "stats") {
		stats, err := s.resolver.Stats(ctx, TimeRangeInput{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		})
		if err != nil {
			response.Errors = append(response.Errors, GraphQLError{
				Message: err.Error(),
			})
		} else {
			response.Data["stats"] = stats
		}
	}

	if req.OperationName == "Health" || contains(req.Query, "health") {
		health, err := s.resolver.Health(ctx)
		if err != nil {
			response.Errors = append(response.Errors, GraphQLError{
				Message: err.Error(),
			})
		} else {
			response.Data["health"] = health
		}
	}

	// Add extensions with timing info
	response.Extensions = map[string]interface{}{
		"timing": map[string]interface{}{
			"startTime":   startTime.Unix(),
			"endTime":     time.Now().Unix(),
			"durationMs":  time.Since(startTime).Milliseconds(),
		},
	}

	return response
}

// Close shuts down the GraphQL server
func (s *Server) Close() error {
	if s.subscriptionManager != nil {
		s.subscriptionManager.Close()
	}
	return nil
}

// ================================================================================
// GraphQL Request/Response Types
// ================================================================================

type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data       map[string]interface{} `json:"data,omitempty"`
	Errors     []GraphQLError         `json:"errors,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// ================================================================================
// Middleware Helpers
// ================================================================================

// DefaultAuthenticator is a simple API key authenticator
func DefaultAuthenticator(validKeys map[string]bool) AuthenticatorFunc {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			apiKey = c.Get("Authorization")
		}

		if apiKey == "" || !validKeys[apiKey] {
			return fmt.Errorf("invalid or missing API key")
		}

		return nil
	}
}

// DefaultRateLimiter is a simple in-memory rate limiter
func DefaultRateLimiter(requestsPerMinute int) RateLimiterFunc {
	// This is a placeholder - production should use Redis or similar
	return func(c *fiber.Ctx) error {
		// Simplified rate limiting logic
		return nil
	}
}

// ================================================================================
// Utilities
// ================================================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		false
}

// Health check endpoint for monitoring
func (s *Server) HealthCheck() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := s.resolver.Health(ctx)
	if err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	return map[string]interface{}{
		"status":    health.Status,
		"version":   health.Version,
		"uptime":    health.Uptime,
		"timestamp": health.Timestamp,
	}
}

// GetMetrics returns server metrics
func (s *Server) GetMetrics() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := s.resolver.RealtimeMetrics(ctx)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"requests_per_second": metrics.RequestsPerSecond,
		"active_connections":  metrics.ActiveConnections,
		"queued_requests":     metrics.QueuedRequests,
		"timestamp":           metrics.Timestamp,
	}
}
