package stats

// Questo file contiene esempi di integrazione del sistema stats nel gateway.
// NON è codice da eseguire direttamente, ma serve come riferimento.

/*
// 1. INIZIALIZZAZIONE nel main.go o gateway.go

import (
	"github.com/biodoia/goleapifree/internal/stats"
	"github.com/biodoia/goleapifree/pkg/database"
)

func main() {
	// ... dopo aver creato il database ...

	// Crea il manager delle statistiche
	statsConfig := stats.DefaultConfig()
	statsConfig.PrometheusEnabled = true
	statsConfig.RetentionDays = 30

	statsManager := stats.NewManager(db, statsConfig)

	// Avvia il manager
	if err := statsManager.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start stats manager")
	}
	defer statsManager.Stop()

	// ... continua con il setup del gateway ...
}

// 2. REGISTRAZIONE RICHIESTE nel router o handler

func handleRequest(c *fiber.Ctx, statsManager *stats.Manager) error {
	startTime := time.Now()
	providerID := getProviderID(c) // implementare
	modelID := getModelID(c)       // implementare
	userID := getUserID(c)         // implementare

	// Registra inizio richiesta
	statsManager.RecordStart(c.Context(), providerID)

	// Esegui la richiesta
	response, err := executeRequest(c)

	latencyMs := int(time.Since(startTime).Milliseconds())
	success := err == nil
	statusCode := getStatusCode(response)

	// Registra metriche
	metrics := &stats.RequestMetrics{
		ProviderID:    providerID,
		ModelID:       modelID,
		UserID:        userID,
		Method:        c.Method(),
		Endpoint:      c.Path(),
		StatusCode:    statusCode,
		LatencyMs:     latencyMs,
		InputTokens:   getInputTokens(response),
		OutputTokens:  getOutputTokens(response),
		Success:       success,
		ErrorMessage:  getErrorMessage(err),
		EstimatedCost: calculateCost(response),
		Timestamp:     startTime,
	}

	statsManager.Record(metrics)

	return c.JSON(response)
}

// 3. ENDPOINT PER DASHBOARD nel gateway.go

func (g *Gateway) setupStatsRoutes(statsManager *stats.Manager) {
	admin := g.app.Group("/admin")

	// Dashboard data completo
	admin.Get("/dashboard", func(c *fiber.Ctx) error {
		data, err := statsManager.GetDashboardData(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	// Summary
	admin.Get("/stats/summary", func(c *fiber.Ctx) error {
		summary, err := statsManager.GetSummary(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(summary)
	})

	// Trends
	admin.Get("/stats/trends/hourly", func(c *fiber.Ctx) error {
		hours := c.QueryInt("hours", 24)
		trends, err := statsManager.GetHourlyTrends(c.Context(), hours)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(trends)
	})

	admin.Get("/stats/trends/daily", func(c *fiber.Ctx) error {
		days := c.QueryInt("days", 7)
		trends, err := statsManager.GetDailyTrends(c.Context(), days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(trends)
	})

	// Cost savings
	admin.Get("/stats/savings", func(c *fiber.Ctx) error {
		savings, err := statsManager.GetCostSavings(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(savings)
	})

	// Provider stats
	admin.Get("/stats/providers/:id", func(c *fiber.Ctx) error {
		providerID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid provider id"})
		}

		stats, err := statsManager.GetProviderStats(c.Context(), providerID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(stats)
	})
}

// 4. PROMETHEUS METRICS ENDPOINT

import (
	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func (g *Gateway) setupPrometheusEndpoint() {
	// Prometheus metrics endpoint
	g.app.Get("/metrics", func(c *fiber.Ctx) error {
		handler := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
		handler(c.RequestCtx())
		return nil
	})
}

// 5. MIDDLEWARE PER AUTO-TRACKING

func StatsMiddleware(statsManager *stats.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip per endpoint non-API
		if !strings.HasPrefix(c.Path(), "/v1/") {
			return c.Next()
		}

		startTime := time.Now()

		// Esegui la richiesta
		err := c.Next()

		// Estrai informazioni dal context (impostate da handler precedenti)
		providerID, _ := c.Locals("provider_id").(uuid.UUID)
		modelID, _ := c.Locals("model_id").(uuid.UUID)
		userID, _ := c.Locals("user_id").(uuid.UUID)

		if providerID != uuid.Nil {
			latencyMs := int(time.Since(startTime).Milliseconds())
			success := err == nil && c.Response().StatusCode() < 400

			metrics := &stats.RequestMetrics{
				ProviderID:    providerID,
				ModelID:       modelID,
				UserID:        userID,
				Method:        c.Method(),
				Endpoint:      c.Path(),
				StatusCode:    c.Response().StatusCode(),
				LatencyMs:     latencyMs,
				InputTokens:   c.Locals("input_tokens").(int),
				OutputTokens:  c.Locals("output_tokens").(int),
				Success:       success,
				ErrorMessage:  getErrorFromResponse(c),
				EstimatedCost: c.Locals("estimated_cost").(float64),
				Timestamp:     startTime,
			}

			statsManager.Record(metrics)
		}

		return err
	}
}

// 6. CONFIGURAZIONE PROMETHEUS ESTERNO

// Nel docker-compose.yml o Kubernetes config:
//
// prometheus:
//   image: prom/prometheus:latest
//   ports:
//     - "9090:9090"
//   volumes:
//     - ./prometheus.yml:/etc/prometheus/prometheus.yml
//   command:
//     - '--config.file=/etc/prometheus/prometheus.yml'
//
// File prometheus.yml:
//
// global:
//   scrape_interval: 15s
//
// scrape_configs:
//   - job_name: 'goleapai'
//     static_configs:
//       - targets: ['gateway:8080']
//     metrics_path: '/metrics'

// 7. QUERY ESEMPIO PROMETHEUS

// Queries utili in Prometheus:
//
// 1. Request rate per provider:
//    rate(goleapai_requests_total[5m])
//
// 2. Success rate:
//    rate(goleapai_requests_total{status="success"}[5m]) /
//    rate(goleapai_requests_total[5m])
//
// 3. Latency p95:
//    histogram_quantile(0.95,
//      rate(goleapai_request_duration_milliseconds_bucket[5m]))
//
// 4. Error rate:
//    rate(goleapai_request_errors_total[5m])
//
// 5. Tokens per second:
//    rate(goleapai_tokens_processed_total[1m])
//
// 6. Cost saved per hour:
//    increase(goleapai_cost_saved_total[1h])

// 8. GRAFANA DASHBOARD ESEMPIO

// Pannelli consigliati:
//
// 1. Total Requests (Graph)
//    Query: sum(rate(goleapai_requests_total[5m]))
//
// 2. Success Rate (Gauge)
//    Query: sum(rate(goleapai_requests_total{status="success"}[5m])) /
//           sum(rate(goleapai_requests_total[5m])) * 100
//
// 3. Request Latency (Graph)
//    Query: histogram_quantile(0.95,
//             rate(goleapai_request_duration_milliseconds_bucket[5m]))
//
// 4. Provider Health (Table)
//    Query: goleapai_provider_health_score
//
// 5. Active Providers (Stat)
//    Query: goleapai_active_providers
//
// 6. Cost Saved (Stat)
//    Query: increase(goleapai_cost_saved_total[24h])
//
// 7. Error Rate by Provider (Graph)
//    Query: rate(goleapai_request_errors_total[5m])
//
// 8. Tokens Processed (Counter)
//    Query: increase(goleapai_tokens_processed_total[1h])

// 9. CLEANUP PERIODICO

import "time"

func setupStatsCleanup(statsManager *stats.Manager) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			// Il cleanup è automatico nell'aggregator,
			// ma possiamo anche fare cleanup manuale se necessario
			log.Info().Msg("Daily stats cleanup triggered")
		}
	}()
}

// 10. ESEMPI DI USO NEL ROUTER

// Nel router.go, quando si seleziona un provider:
func (r *Router) selectProvider(ctx context.Context, request *Request) (*Provider, error) {
	// Ottieni statistiche per ogni provider
	providers := r.getAvailableProviders()

	bestProvider := providers[0]
	bestScore := 0.0

	for _, provider := range providers {
		stats := r.statsManager.Collector().GetProviderMetrics(provider.ID)
		if stats == nil {
			continue
		}

		// Calcola score basato su success rate e latenza
		successRate := r.statsManager.Collector().CalculateSuccessRate(provider.ID)
		avgLatency := r.statsManager.Collector().CalculateAvgLatency(provider.ID)

		score := (successRate * 0.7) + ((1000.0 - float64(avgLatency)) / 1000.0 * 0.3)

		if score > bestScore {
			bestScore = score
			bestProvider = provider
		}
	}

	return bestProvider, nil
}
*/
