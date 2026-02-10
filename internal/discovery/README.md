# Auto-Discovery System

Sistema automatico di scoperta di nuove API gratuite per LLM.

## Caratteristiche

### 1. Discovery Engine (`discovery.go`)
Core del sistema che coordina tutte le attività di discovery:
- **Scheduler con cron job**: Esegue discovery periodicamente (configurabile)
- **Gestione parallela**: Valida più candidati contemporaneamente
- **Deduplicazione**: Rimuove candidati duplicati
- **Filtraggio intelligente**: Esclude provider già esistenti
- **Auto-save**: Salva automaticamente provider validi nel database

### 2. GitHub Discovery (`github.go`)
Cerca repository su GitHub per trovare nuove API:
- **GitHub Search API**: Cerca repository con termini specifici
- **README parsing**: Analizza README per estrarre endpoint
- **Metadata extraction**: Estrae stelle, linguaggio, ultimo aggiornamento
- **Smart filtering**: Filtra repository spam o non rilevanti
- **Auth detection**: Rileva tipo di autenticazione dal README
- **Model detection**: Identifica modelli supportati (GPT, Claude, etc.)

#### Funzionalità GitHub
- Search per keywords: "free llm api", "ai proxy", "gpt4free", etc.
- Filtraggio per:
  - Minimo 5 stelle
  - Aggiornato negli ultimi 12 mesi
  - Esclusione repository test/demo/tutorial
- Estrazione endpoint da:
  - Sezioni "Usage" o "Configuration"
  - Pattern URL API comuni
  - Codice esempio

### 3. Validator (`validator.go`)
Valida endpoint scoperti per verificarne la funzionalità:
- **Connectivity test**: Verifica che l'endpoint risponda
- **Compatibility detection**: Rileva se è OpenAI/Anthropic compatible
- **Latency measurement**: Misura tempi di risposta
- **Feature detection**: Verifica streaming, tools, JSON support
- **Health score calculation**: Assegna punteggio 0.0-1.0
- **Rate limit detection**: Identifica limiti dell'API

#### Health Score
Il punteggio di salute si basa su:
- Base score (0.3): Connettività funzionante
- Compatibility (0.2): API riconoscibile (OpenAI/Anthropic)
- Latency (0.2): Risposta veloce (<500ms)
- Features (0.3): Streaming, JSON, Tools support
- Models (0.1): Lista modelli disponibili

### 4. Web Scraper (`scraper.go`)
Scrape awesome lists e altre risorse web:
- **Awesome lists**: awesome-chatgpt-api, awesome-free-chatgpt, etc.
- **Markdown parsing**: Estrae link da liste markdown
- **URL filtering**: Identifica solo endpoint API validi
- **Documentation scraping**: Cerca configurazioni API comuni

#### Awesome Lists Monitorate
- `awesome-chatgpt-api`: Lista curata di API ChatGPT
- `awesome-free-chatgpt`: Servizi ChatGPT gratuiti
- `awesome-ai-services`: Servizi AI generali
- Liste custom su GitHub

## Configurazione

### File di configurazione (`configs/discovery.example.yaml`)

```yaml
discovery:
  enabled: true
  interval: 24h
  github_token: "your_github_token"  # Opzionale ma raccomandato
  github_enabled: true
  scraper_enabled: true
  max_concurrent: 5
  validation_timeout: 30s
  min_health_score: 0.6
```

### Variabili d'ambiente

```bash
# GitHub token per rate limit più alti
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxx"

# Abilita/disabilita discovery
export DISCOVERY_ENABLED=true

# Intervallo discovery (formato Go duration)
export DISCOVERY_INTERVAL=24h
```

## Uso

### Avvio automatico

Il discovery engine si avvia automaticamente con il backend se abilitato:

```go
import "github.com/biodoia/goleapifree/internal/discovery"

// Inizializza discovery
engine, err := discovery.StartDiscoveryService(ctx, cfg, db, logger)
if err != nil {
    log.Fatal(err)
}
defer engine.Stop()
```

### Discovery manuale

```go
// Crea engine
config := &discovery.DiscoveryConfig{
    Enabled:           true,
    Interval:          24 * time.Hour,
    GitHubToken:       "your_token",
    GitHubEnabled:     true,
    ScraperEnabled:    true,
    MaxConcurrent:     5,
    ValidationTimeout: 30 * time.Second,
    MinHealthScore:    0.6,
}

engine := discovery.NewEngine(config, db, logger)

// Esegui discovery una tantum
err := engine.RunDiscovery(context.Background())
```

### Solo validazione

```go
// Valida un singolo endpoint
validator := discovery.NewValidator(30*time.Second, logger)

result, err := validator.ValidateEndpoint(
    ctx,
    "https://api.example.com/v1",
    models.AuthTypeAPIKey,
)

if result.IsValid {
    fmt.Printf("Health Score: %.2f\n", result.HealthScore)
    fmt.Printf("Latency: %dms\n", result.LatencyMs)
    fmt.Printf("Compatibility: %s\n", result.Compatibility)
}
```

## Flusso di lavoro

1. **Discovery**:
   - GitHub search per repository rilevanti
   - Web scraping di awesome lists
   - Estrazione endpoint da README e documentazione

2. **Filtering**:
   - Deduplicazione candidati
   - Rimozione provider già esistenti
   - Filtraggio spam e repository non rilevanti

3. **Validation**:
   - Test connettività endpoint
   - Rilevamento compatibilità API
   - Misurazione latenza e features
   - Calcolo health score

4. **Storage**:
   - Salvataggio provider validi (score > 0.6)
   - Metadata completo nel database
   - Tracking source (github/scraper)

## Metriche

### Provider Stats

```go
stats, err := discovery.GetDiscoveryStats(db)

// Output:
// {
//   "by_source": {
//     "github": 42,
//     "scraper": 15,
//     "manual": 8
//   },
//   "by_status": {
//     "active": 50,
//     "down": 10,
//     "maintenance": 5
//   },
//   "discovered_last_7_days": 12,
//   "avg_health_score": 0.75
// }
```

### Verifica Periodica

Il sistema verifica periodicamente i provider esistenti:

```go
// Avvia verifica periodica ogni 6 ore
go discovery.ScheduleVerification(
    ctx,
    db,
    validator,
    6*time.Hour,
    logger,
)
```

## Sicurezza

- **Rate Limiting**: Rispetta rate limit GitHub (5000 req/h con token)
- **Timeout**: Ogni validazione ha timeout configurabile
- **Concurrent Limit**: Numero massimo validazioni parallele
- **Spam Protection**: Filtraggio repository e URL sospetti

## Performance

- **Parallel Processing**: Validazione parallela con semaphore
- **Caching**: Deduplica candidati prima della validazione
- **Efficient Filtering**: Filtra prima di validare
- **Batch Operations**: Operazioni DB in batch quando possibile

## Estensioni future

- [ ] Reddit/Discord monitoring via API
- [ ] ML-based spam detection
- [ ] Automatic model list updates
- [ ] Provider quality scoring over time
- [ ] Community feedback integration
- [ ] Telegram channel monitoring
- [ ] Twitter/X API discovery
- [ ] Hacker News scraping

## Troubleshooting

### Discovery non trova nuovi provider

1. Verifica token GitHub sia valido
2. Controlla rate limit GitHub
3. Aumenta termini di ricerca
4. Abbassa `min_health_score`

### Molti false positives

1. Aumenta `min_health_score`
2. Aggiungi pattern di esclusione
3. Aumenta requisiti minimi (stars, age)

### Validazione lenta

1. Riduci `validation_timeout`
2. Aumenta `max_concurrent`
3. Disabilita scraper se non necessario

### GitHub rate limit

1. Aggiungi GitHub token
2. Aumenta `interval` discovery
3. Riduci `discovery_search_terms`

## Testing

```bash
# Esegui tutti i test
go test ./internal/discovery/...

# Test con coverage
go test -cover ./internal/discovery/...

# Test verbose
go test -v ./internal/discovery/...

# Skip integration tests
go test -short ./internal/discovery/...
```

## Logging

Il sistema usa zerolog per logging strutturato:

```
{"level":"info","component":"discovery","msg":"Starting discovery run"}
{"level":"info","component":"github_discovery","term":"free llm api","repos_found":25}
{"level":"info","component":"validator","url":"https://api.example.com","health_score":0.8}
{"level":"info","component":"discovery","validated":15,"saved":12,"msg":"Discovery completed"}
```

## License

Parte del progetto GoLeapAI Free
