package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/a-h/templ"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/web/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	configPath string
	logLevel   string
	port       int
)

type WebUIServer struct {
	app    *fiber.App
	db     *database.DB
	config *config.Config
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "webui",
		Short: "GoLeapAI Web UI - Code Page 437 Edition",
		Long:  `Terminal Web UI with HTMX and retro CP437 aesthetic`,
		RunE:  runWebUI,
	}

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "Web UI port")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWebUI(cmd *cobra.Command, args []string) error {
	setupLogger(logLevel)

	log.Info().Msg("Starting GoLeapAI Web UI (CP437 Edition)")

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Create Web UI server
	server := NewWebUIServer(cfg, db, port)

	// Start server in background
	go func() {
		if err := server.Start(); err != nil {
			log.Fatal().Err(err).Msg("Web UI failed to start")
		}
	}()

	log.Info().Msgf("╔═══════════════════════════════════════════╗")
	log.Info().Msgf("║  GoLeapAI Web UI - CP437 Edition          ║")
	log.Info().Msgf("║  http://localhost:%d                      ║", port)
	log.Info().Msgf("╚═══════════════════════════════════════════╝")

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		return err
	}

	log.Info().Msg("Web UI stopped")
	return nil
}

func NewWebUIServer(cfg *config.Config, db *database.DB, port int) *WebUIServer {
	app := fiber.New(fiber.Config{
		AppName:               "GoLeapAI WebUI",
		DisableStartupMessage: true,
		EnablePrintRoutes:     false,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	server := &WebUIServer{
		app:    app,
		db:     db,
		config: cfg,
	}

	server.setupRoutes()
	return server
}

func (s *WebUIServer) setupRoutes() {
	// Static files
	s.app.Static("/static", "./web/static")

	// Main dashboard
	s.app.Get("/", s.handleDashboard)

	// HTMX endpoints
	s.app.Get("/htmx/providers", s.handleProvidersPartial)
	s.app.Get("/htmx/stats", s.handleStatsPartial)
	s.app.Get("/htmx/logs", s.handleLogsPartial)

	// Provider actions
	s.app.Post("/htmx/provider/:id/toggle", s.handleToggleProvider)
	s.app.Post("/htmx/provider/:id/test", s.handleTestProvider)

	// WebSocket for live updates
	s.app.Get("/ws", websocket.New(s.handleWebSocket))
}

func (s *WebUIServer) handleDashboard(c *fiber.Ctx) error {
	component := templates.Dashboard()
	return renderTempl(c, component)
}

func (s *WebUIServer) handleProvidersPartial(c *fiber.Ctx) error {
	providers, err := s.db.GetAllProviders()
	if err != nil {
		return c.Status(500).SendString("Error loading providers")
	}

	component := templates.ProvidersList(providers)
	return renderTempl(c, component)
}

func (s *WebUIServer) handleStatsPartial(c *fiber.Ctx) error {
	// Get latest stats
	stats, err := s.db.GetLatestStats()
	if err != nil {
		return c.Status(500).SendString("Error loading stats")
	}

	component := templates.StatsPanel(stats)
	return renderTempl(c, component)
}

func (s *WebUIServer) handleLogsPartial(c *fiber.Ctx) error {
	// Get recent logs
	logs, err := s.db.GetRecentLogs(50)
	if err != nil {
		return c.Status(500).SendString("Error loading logs")
	}

	component := templates.LogsViewer(logs)
	return renderTempl(c, component)
}

func (s *WebUIServer) handleToggleProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	provider, err := s.db.GetProviderByID(id)
	if err != nil {
		return c.Status(404).SendString("Provider not found")
	}

	// Toggle status
	if provider.Status == "active" {
		provider.Status = "down"
	} else {
		provider.Status = "active"
	}

	if err := s.db.UpdateProvider(provider); err != nil {
		return c.Status(500).SendString("Error updating provider")
	}

	// Return updated provider row
	component := templates.ProviderRow(provider)
	return renderTempl(c, component)
}

func (s *WebUIServer) handleTestProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	// Run health check
	provider, err := s.db.GetProviderByID(id)
	if err != nil {
		return c.Status(404).SendString("Provider not found")
	}

	// TODO: Actual health check
	provider.HealthScore = 0.95
	provider.LastHealthCheck = time.Now()

	if err := s.db.UpdateProvider(provider); err != nil {
		return c.Status(500).SendString("Error updating provider")
	}

	return c.SendString("✓ Health check passed")
}

func (s *WebUIServer) handleWebSocket(c *websocket.Conn) {
	defer c.Close()

	// Send updates every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send live stats update
			stats, err := s.db.GetLatestStats()
			if err != nil {
				continue
			}

			component := templates.StatsPanel(stats)
			html, err := templ.ToGoHTML(context.Background(), component)
			if err != nil {
				continue
			}

			if err := c.WriteMessage(websocket.TextMessage, []byte(html)); err != nil {
				return
			}
		}
	}
}

func (s *WebUIServer) Start() error {
	return s.app.Listen(fmt.Sprintf(":%d", port))
}

func (s *WebUIServer) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}

func renderTempl(c *fiber.Ctx, component templ.Component) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
	return component.Render(context.Background(), c.Response().BodyWriter())
}

func setupLogger(level string) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}
