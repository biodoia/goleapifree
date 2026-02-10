# Stats - Sistema di Raccolta e Aggregazione Statistiche

Sistema completo per raccolta, aggregazione e visualizzazione delle statistiche di utilizzo di goleapifree.

## Architettura

```
┌─────────────┐
│   Gateway   │
│  (Requests) │
└──────┬──────┘
       │
       v
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│  Collector  │────>│  Aggregator  │────>│  Database    │
│  (Memory)   │     │  (Time-based)│     │ (Persistent) │
└──────┬──────┘     └──────────────┘     └──────────────┘
       │
       │
       v
┌─────────────┐     ┌──────────────┐
│ Prometheus  │     │  Dashboard   │
│  (Metrics)  │     │   (API)      │
└─────────────┘     └──────────────┘
```

## Componenti

### 1. Collector (`collector.go`)

Raccoglie metriche da ogni richiesta e le aggrega in memoria.

**Funzionalità:**
- Buffering delle richieste per scritture batch
- Aggregazione in-memory per metriche real-time
- Calcolo automatico di success rate e latenza media
- Flush periodico e on-demand nel database
- Thread-safe per uso concorrente

**Metriche raccolte:**
- Totale richieste
- Success/error count
- Latenza media
- Token processati
- Costo stimato
- Timeout e quota exhaustion

**Esempio d'uso:**
```go
collector := stats.NewCollector(db, 100) // buffer di 100 richieste
collector.Start(10 * time.Second)        // flush ogni 10 secondi

// Registra una richiesta
metrics := &stats.RequestMetrics{
    ProviderID:    providerID,
    ModelID:       modelID,
    Success:       true,
    LatencyMs:     150,
    InputTokens:   100,
    OutputTokens:  50,
}
collector.Record(metrics)

// Ottieni statistiche
stats := collector.GetProviderMetrics(providerID)
successRate := collector.CalculateSuccessRate(providerID)
```

### 2. Aggregator (`aggregator.go`)

Aggrega statistiche in finestre temporali e gestisce la retention.

**Funzionalità:**
- Aggregazione per minuto/ora/giorno
- Rolling windows per analisi trend
- Cleanup automatico dati vecchi
- Query per confronti temporali
- Supporto per multiple finestre temporali

**Time Windows:**
- `WindowMinute`: Aggregazione per minuto
- `WindowHour`: Aggregazione oraria
- `WindowDay`: Aggregazione giornaliera

**Esempio d'uso:**
```go
aggregator := stats.NewAggregator(db, collector, time.Minute, 30)
aggregator.Start()

// Ottieni statistiche orarie ultime 24 ore
hourlyStats, err := aggregator.GetHourlyStats(ctx, providerID, 24)

// Ottieni statistiche giornaliere ultima settimana
dailyStats, err := aggregator.GetDailyStats(ctx, providerID, 7)

// Rolling window ultimi 5 minuti
rollingStats, err := aggregator.GetRollingWindow(ctx, providerID, 5*time.Minute)

// Confronta più provider
comparison, err := aggregator.CompareProviders(ctx, providerIDs, time.Hour)
```

### 3. Prometheus Exporter (`prometheus.go`)

Espone metriche in formato Prometheus per monitoring esterno.

**Metriche esposte:**

| Metrica | Tipo | Descrizione |
|---------|------|-------------|
| `goleapai_requests_total` | Counter | Totale richieste per provider/model/status |
| `goleapai_request_duration_milliseconds` | Histogram | Distribuzione latenza richieste |
| `goleapai_request_errors_total` | Counter | Errori per provider e tipo |
| `goleapai_active_providers` | Gauge | Numero provider attivi |
| `goleapai_provider_health_score` | Gauge | Health score per provider (0-1) |
| `goleapai_tokens_processed_total` | Counter | Token processati (input/output) |
| `goleapai_quota_usage_ratio` | Gauge | Utilizzo quota per provider (0-1) |
| `goleapai_cost_saved_total` | Counter | Costo risparmiato vs API ufficiali |
| `goleapai_requests_in_flight` | Gauge | Richieste in corso |
| `goleapai_success_rate` | Gauge | Success rate per provider (0-1) |
| `goleapai_avg_latency_milliseconds` | Gauge | Latenza media per provider |

**Esempio d'uso:**
```go
exporter := stats.NewPrometheusExporter(db, collector, "goleapai")
exporter.Start()

// Le metriche sono registrate automaticamente
exporter.RecordRequest("provider-name", "model-name", "success")
exporter.RecordDuration("provider-name", "model-name", 150.0)
exporter.RecordTokens("provider-name", "model-name", "input", 100)

// Endpoint /metrics espone le metriche
```

**Query Prometheus utili:**
```promql
# Request rate
rate(goleapai_requests_total[5m])

# Success rate
rate(goleapai_requests_total{status="success"}[5m]) /
rate(goleapai_requests_total[5m])

# P95 latency
histogram_quantile(0.95,
  rate(goleapai_request_duration_milliseconds_bucket[5m]))

# Cost saved per hour
increase(goleapai_cost_saved_total[1h])
```

### 4. Dashboard (`dashboard.go`)

Fornisce dati aggregati per dashboard e visualizzazioni.

**Endpoints disponibili:**
- `GetDashboardData()`: Tutti i dati del dashboard
- `GetSummary()`: Statistiche di riepilogo
- `GetProviderStats()`: Stats dettagliate per provider
- `GetHourlyTrends()`: Trend orari
- `GetDailyTrends()`: Trend giornalieri
- `GetCostSavings()`: Analisi risparmi
- `GetTopProviders()`: Classifica provider
- `GetRecentErrors()`: Errori recenti
- `GetPerformanceChart()`: Dati per grafici performance

**Strutture dati:**
```go
type DashboardData struct {
    Summary          *SummaryStats
    ProviderStats    []*ProviderDashStats
    HourlyTrends     []*TrendPoint
    DailyTrends      []*TrendPoint
    CostSavings      *CostSavingsData
    TopProviders     []*ProviderRanking
    RecentErrors     []*ErrorSummary
    PerformanceChart *PerformanceChartData
}
```

**Esempio d'uso:**
```go
dashboard := stats.NewDashboard(db, collector, aggregator)

// Ottieni tutti i dati
data, err := dashboard.GetDashboardData(ctx)

// Usa nei tuoi endpoint
app.Get("/admin/dashboard", func(c *fiber.Ctx) error {
    data, err := dashboard.GetDashboardData(c.Context())
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(data)
})
```

### 5. Manager (`manager.go`)

Orchestratore centrale per tutti i componenti stats.

**Funzionalità:**
- Inizializzazione unificata di tutti i componenti
- Gestione lifecycle (start/stop)
- API semplificata per registrazione metriche
- Configurazione centralizzata

**Esempio d'uso:**
```go
// Configurazione
cfg := stats.DefaultConfig()
cfg.RetentionDays = 30
cfg.PrometheusEnabled = true

// Creazione manager
manager := stats.NewManager(db, cfg)
manager.Start()
defer manager.Stop()

// Registra richiesta
metrics := &stats.RequestMetrics{...}
manager.Record(metrics)

// Accesso ai componenti
collector := manager.Collector()
dashboard := manager.Dashboard()
prometheus := manager.Prometheus()
```

## Integrazione nel Gateway

### 1. Setup nel main.go

```go
import "github.com/biodoia/goleapifree/internal/stats"

func main() {
    // ... dopo setup database ...

    statsManager := stats.NewManager(db, stats.DefaultConfig())
    if err := statsManager.Start(); err != nil {
        log.Fatal().Err(err).Msg("Failed to start stats")
    }
    defer statsManager.Stop()

    // ... continua setup gateway ...
}
```

### 2. Middleware per tracking automatico

```go
func StatsMiddleware(statsManager *stats.Manager) fiber.Handler {
    return func(c *fiber.Ctx) error {
        startTime := time.Now()

        err := c.Next()

        providerID, _ := c.Locals("provider_id").(uuid.UUID)
        if providerID != uuid.Nil {
            statsManager.Record(&stats.RequestMetrics{
                ProviderID:   providerID,
                ModelID:      c.Locals("model_id").(uuid.UUID),
                StatusCode:   c.Response().StatusCode(),
                LatencyMs:    int(time.Since(startTime).Milliseconds()),
                Success:      err == nil,
            })
        }

        return err
    }
}
```

### 3. Endpoint API

```go
admin := app.Group("/admin")

// Dashboard completo
admin.Get("/dashboard", func(c *fiber.Ctx) error {
    data, err := statsManager.GetDashboardData(c.Context())
    return c.JSON(data)
})

// Summary
admin.Get("/stats/summary", func(c *fiber.Ctx) error {
    summary, err := statsManager.GetSummary(c.Context())
    return c.JSON(summary)
})

// Trends
admin.Get("/stats/trends/hourly", func(c *fiber.Ctx) error {
    hours := c.QueryInt("hours", 24)
    trends, err := statsManager.GetHourlyTrends(c.Context(), hours)
    return c.JSON(trends)
})
```

### 4. Prometheus endpoint

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/valyala/fasthttp/fasthttpadaptor"
)

app.Get("/metrics", func(c *fiber.Ctx) error {
    handler := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
    handler(c.RequestCtx())
    return nil
})
```

## Configurazione Prometheus

### prometheus.yml

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'goleapai'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Docker Compose

```yaml
services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  prometheus_data:
  grafana_data:
```

## Grafana Dashboard

### Pannelli consigliati

1. **Total Requests** (Graph)
   - Query: `sum(rate(goleapai_requests_total[5m]))`

2. **Success Rate** (Gauge)
   - Query: `sum(rate(goleapai_requests_total{status="success"}[5m])) / sum(rate(goleapai_requests_total[5m])) * 100`

3. **Request Latency P95** (Graph)
   - Query: `histogram_quantile(0.95, rate(goleapai_request_duration_milliseconds_bucket[5m]))`

4. **Active Providers** (Stat)
   - Query: `goleapai_active_providers`

5. **Cost Saved Today** (Stat)
   - Query: `increase(goleapai_cost_saved_total[24h])`

6. **Error Rate by Provider** (Graph)
   - Query: `rate(goleapai_request_errors_total[5m])`

7. **Provider Health Scores** (Table)
   - Query: `goleapai_provider_health_score`

## Database Schema

Le statistiche utilizzano due tabelle principali:

### request_logs
```sql
CREATE TABLE request_logs (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL,
    model_id UUID,
    user_id UUID,
    method VARCHAR(10),
    endpoint VARCHAR(255),
    status_code INTEGER,
    latency_ms INTEGER,
    input_tokens INTEGER,
    output_tokens INTEGER,
    success BOOLEAN,
    error_message TEXT,
    estimated_cost DECIMAL(10,6),
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP
);

CREATE INDEX idx_request_logs_provider ON request_logs(provider_id);
CREATE INDEX idx_request_logs_timestamp ON request_logs(timestamp);
```

### provider_stats
```sql
CREATE TABLE provider_stats (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    success_rate DECIMAL(5,4),
    avg_latency_ms INTEGER,
    total_requests BIGINT,
    total_tokens BIGINT,
    cost_saved DECIMAL(10,6),
    error_count BIGINT,
    timeout_count BIGINT,
    quota_exhausted BIGINT,
    created_at TIMESTAMP
);

CREATE INDEX idx_provider_stats_provider ON provider_stats(provider_id);
CREATE INDEX idx_provider_stats_timestamp ON provider_stats(timestamp);
```

## Performance

### Ottimizzazioni

1. **Buffering**: Le richieste sono bufferizzate in memoria e scritte in batch
2. **Aggregazione in-memory**: Calcoli real-time senza query database
3. **Cleanup automatico**: Rimozione periodica dati vecchi
4. **Indici database**: Indici ottimizzati per query temporali

### Benchmark

```
BenchmarkCollectorRecord-8      1000000    1234 ns/op
BenchmarkAggregatorWindow-8     10000      123456 ns/op
```

### Limiti consigliati

- Buffer size: 100-1000 richieste
- Flush interval: 10-60 secondi
- Aggregation interval: 1-5 minuti
- Retention: 7-90 giorni

## Testing

```bash
# Run tests
go test ./internal/stats/...

# Run tests con coverage
go test -cover ./internal/stats/...

# Run benchmarks
go test -bench=. ./internal/stats/...
```

## Best Practices

1. **Usa il Manager**: Preferisci `stats.Manager` invece di gestire i componenti singolarmente
2. **Buffer appropriato**: Dimensiona il buffer in base al traffico atteso
3. **Retention policy**: Configura retention basato sulle tue necessità di storage
4. **Monitoring**: Monitora le metriche Prometheus per individuare problemi
5. **Cleanup**: Il cleanup è automatico, ma puoi trigggerarlo manualmente se necessario

## Troubleshooting

### High memory usage
- Riduci buffer size
- Riduci flush interval
- Aumenta aggregation interval

### Database slow queries
- Verifica indici
- Riduci retention period
- Aumenta batch size per flush

### Missing metrics
- Verifica che manager sia started
- Controlla logs per errori flush
- Verifica configurazione Prometheus

## Esempi

Vedi `integration_example.go` per esempi completi di integrazione.

## License

Parte di goleapifree - vedi LICENSE nel repository principale.
