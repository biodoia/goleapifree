package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/health"
	"github.com/biodoia/goleapifree/internal/router"
	"github.com/biodoia/goleapifree/internal/websocket"
	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// Gateway è il core LLM gateway
type Gateway struct {
	config        *config.Config
	db            *database.DB
	app           *fiber.App
	router        *router.Router
	health        *health.Monitor
	jwtManager    *auth.JWTManager
	apiKeyManager *auth.APIKeyManager
	wsHub         *websocket.Hub
	wsHandler     *websocket.Handler
}

// New crea una nuova istanza del gateway
func New(cfg *config.Config, db *database.DB) (*Gateway, error) {
	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "GoLeapAI Gateway",
		ServerHeader: "GoLeapAI/1.0",
		ErrorHandler: customErrorHandler,
	})

	// Create router
	r, err := router.New(cfg, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Create health monitor
	healthMonitor := health.NewMonitor(db, cfg.Providers.HealthCheckInterval)

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:       getJWTSecret(cfg),
		Issuer:          "goleapai-gateway",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 7 * 24 * time.Hour,
	})

	// Initialize API key manager
	apiKeyManager := auth.NewAPIKeyManager()

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	wsHandler := websocket.NewHandler(wsHub, cfg, db)

	gw := &Gateway{
		config:        cfg,
		db:            db,
		app:           app,
		router:        r,
		health:        healthMonitor,
		jwtManager:    jwtManager,
		apiKeyManager: apiKeyManager,
		wsHub:         wsHub,
		wsHandler:     wsHandler,
	}

	// Setup middlewares
	gw.setupMiddlewares()

	// Setup routes
	gw.setupRoutes()

	return gw, nil
}

// customErrorHandler gestisce gli errori globali
func customErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	requestID := middleware.GetRequestID(c)

	return c.Status(code).JSON(fiber.Map{
		"error":      message,
		"request_id": requestID,
	})
}

// getJWTSecret ottiene il secret per JWT
func getJWTSecret(cfg *config.Config) string {
	// TODO: Leggere da environment variable o config
	// Per ora usa un default (da cambiare in produzione!)
	return "your-secret-key-change-in-production"
}

// setupMiddlewares configura i middleware globali
func (g *Gateway) setupMiddlewares() {
	// Recovery middleware (primo, per catturare tutti i panic)
	g.app.Use(middleware.RecoveryWithLogger())

	// Request ID middleware
	g.app.Use(middleware.RequestID())

	// CORS middleware
	g.app.Use(middleware.CORS(middleware.DefaultCORSConfig()))

	// Logging middleware
	g.app.Use(middleware.Logging(middleware.LoggingConfig{
		SkipPaths: []string{"/health", "/ready"},
	}))

	// TODO: Re-enable metrics middleware after porting to Fiber v3
	// g.app.Use(middleware.Metrics(middleware.MetricsConfig{
	// 	SkipPaths: []string{"/health", "/ready", "/metrics"},
	// }))
}

// setupRoutes configura le route HTTP
func (g *Gateway) setupRoutes() {
	// Public endpoints (no auth)
	g.app.Get("/health", g.handleHealth)
	g.app.Get("/ready", g.handleReady)

	// Metrics (Prometheus)
	if g.config.Monitoring.Prometheus.Enabled {
		g.app.Get("/metrics", middleware.PrometheusHandler())
	}

	// Auth endpoints
	authGroup := g.app.Group("/auth")
	authGroup.Post("/register", g.handleRegister)
	authGroup.Post("/login", g.handleLogin)
	authGroup.Post("/refresh", g.handleRefreshToken)

	// API v1 (requires authentication)
	api := g.app.Group("/v1", g.authMiddleware())

	// OpenAI compatible endpoints
	api.Post("/chat/completions", g.handleChatCompletion)
	api.Get("/models", g.handleListModels)

	// Anthropic compatible endpoints
	api.Post("/messages", g.handleMessages)

	// User endpoints
	user := api.Group("/user")
	user.Get("/profile", g.handleGetProfile)
	user.Put("/profile", g.handleUpdateProfile)
	user.Get("/apikeys", g.handleListAPIKeys)
	user.Post("/apikeys", g.handleCreateAPIKey)
	user.Delete("/apikeys/:id", g.handleRevokeAPIKey)

	// Admin endpoints (requires admin role)
	admin := g.app.Group("/admin", g.authMiddleware(), middleware.RequireRole("admin"))
	admin.Get("/providers", g.handleListProviders)
	admin.Get("/stats", g.handleStats)
	admin.Get("/users", g.handleListUsers)
	admin.Get("/metrics-info", g.handleMetricsInfo)

	// WebSocket endpoints (requires authentication)
	ws := g.app.Group("/ws", g.authMiddleware())
	ws.Get("/logs", g.wsHandler.HandleLogsWebSocket)
	ws.Get("/stats", g.wsHandler.HandleStatsWebSocket)
	ws.Get("/providers", g.wsHandler.HandleProvidersWebSocket)
	ws.Get("/requests", g.wsHandler.HandleRequestsWebSocket)

	// WebSocket admin
	admin.Get("/ws/stats", g.wsHandler.GetHubStats)
	admin.Post("/ws/broadcast", g.wsHandler.HandleTestBroadcast)
}

// Start avvia il gateway
func (g *Gateway) Start() error {
	// Start health monitoring
	g.health.Start()

	// Start WebSocket hub
	go g.wsHub.Run()
	log.Info().Msg("WebSocket hub started")

	// Build server address
	addr := fmt.Sprintf("%s:%d", g.config.Server.Host, g.config.Server.Port)

	// Start server
	// TODO: Add TLS support for Fiber v3
	return g.app.Listen(addr)
}

// Shutdown esegue lo shutdown graceful del gateway
func (g *Gateway) Shutdown(ctx context.Context) error {
	// Stop health monitoring
	g.health.Stop()

	// Stop WebSocket hub
	g.wsHub.Stop()
	log.Info().Msg("WebSocket hub stopped")

	// Shutdown HTTP server
	if err := g.app.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	log.Info().Msg("Gateway shutdown completed")
	return nil
}

// handleHealth endpoint di health check
func (g *Gateway) handleHealth(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// handleReady endpoint di readiness check
func (g *Gateway) handleReady(c fiber.Ctx) error {
	// Check database connection
	sqlDB, err := g.db.DB.DB()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"ready": false,
			"error": "database connection failed",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(503).JSON(fiber.Map{
			"ready": false,
			"error": "database ping failed",
		})
	}

	return c.JSON(fiber.Map{
		"ready":     true,
		"timestamp": time.Now().Unix(),
	})
}

// handleMetrics espone metriche Prometheus
func (g *Gateway) handleMetrics(c fiber.Ctx) error {
	// TODO: Implement Prometheus metrics export
	return c.SendString("# HELP goleapai_requests_total Total requests\n")
}

// handleChatCompletion gestisce richieste OpenAI-compatible
func (g *Gateway) handleChatCompletion(c fiber.Ctx) error {
	// TODO: Implement chat completion routing
	return c.JSON(fiber.Map{
		"error": "not implemented yet",
	})
}

// handleMessages gestisce richieste Anthropic-compatible
func (g *Gateway) handleMessages(c fiber.Ctx) error {
	// TODO: Implement Anthropic messages routing
	return c.JSON(fiber.Map{
		"error": "not implemented yet",
	})
}

// handleListModels lista tutti i modelli disponibili
func (g *Gateway) handleListModels(c fiber.Ctx) error {
	// TODO: Implement model listing
	return c.JSON(fiber.Map{
		"object": "list",
		"data":   []string{},
	})
}

// handleListProviders lista tutti i provider
func (g *Gateway) handleListProviders(c fiber.Ctx) error {
	// TODO: Implement provider listing
	return c.JSON(fiber.Map{
		"providers": []string{},
	})
}

// handleStats restituisce statistiche aggregate
func (g *Gateway) handleStats(c fiber.Ctx) error {
	// TODO: Implement stats retrieval
	return c.JSON(fiber.Map{
		"stats": fiber.Map{},
	})
}
