# Discovery System Usage Guide

## Quick Start

### 1. Run Discovery Once

```bash
# Using make
make discovery-run

# Or directly
go run ./cmd/backend discovery run
```

### 2. Start Discovery Service (Periodic)

```bash
# Start with default 24h interval
make discovery-start

# Or with custom interval
go run ./cmd/backend discovery start --interval=12h
```

### 3. View Statistics

```bash
make discovery-stats
```

### 4. Validate Single Endpoint

```bash
# Using make
make discovery-validate URL=https://api.example.com/v1

# Or directly
go run ./cmd/backend discovery validate https://api.example.com/v1
```

### 5. Verify Existing Providers

```bash
make discovery-verify
```

## Configuration

### Environment Variables

```bash
# GitHub token for higher rate limits (recommended)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxx"

# Optional: Configure discovery behavior
export DISCOVERY_ENABLED=true
export DISCOVERY_INTERVAL=24h
```

### Configuration File

Edit `configs/config.yaml`:

```yaml
providers:
  auto_discovery: true
  health_check_interval: 5m
  default_timeout: 30s
```

## Command Line Options

### Discovery Run

```bash
go run ./cmd/backend discovery run \
  --github=true \
  --scraper=true \
  --github-token="your-token" \
  --max-concurrent=5 \
  --min-score=0.6
```

### Discovery Start

```bash
go run ./cmd/backend discovery start \
  --interval=24h \
  --github=true \
  --scraper=true
```

### Validate Endpoint

```bash
go run ./cmd/backend discovery validate \
  https://api.example.com/v1 \
  --auth=api_key \
  --timeout=30s
```

## Output Examples

### Discovery Run Output

```
INFO Starting discovery run
INFO GitHub search completed term="free llm api" repos_found=25
INFO Scraping awesome list name="awesome-chatgpt-api"
INFO Candidates after deduplication after_dedup=42
INFO New candidates to validate new_candidates=15
INFO Candidate validated successfully name="ExampleAPI" health_score=0.85 latency_ms=450
INFO Discovery run completed validated=12 saved=12
```

### Statistics Output

```
=== Discovery Statistics ===

Providers by source:
  github          : 42
  scraper         : 15
  manual          : 8

Providers by status:
  active          : 50
  down            : 10
  maintenance     : 5

Discovered in last 7 days: 12
Average health score: 0.75
```

### Validation Output

```
=== Validation Results ===

URL:              https://api.example.com/v1
Valid:            true
Health Score:     0.85
Latency:          450ms
Compatibility:    openai
Supports Stream:  true
Supports JSON:    true
Supports Tools:   true

Available Models:
  - gpt-3.5-turbo
  - gpt-4
```

## Integration Examples

### Programmatic Usage

```go
import "github.com/biodoia/goleapifree/internal/discovery"

// Run discovery once
config := &discovery.DiscoveryConfig{
    Enabled:           true,
    GitHubToken:       os.Getenv("GITHUB_TOKEN"),
    GitHubEnabled:     true,
    ScraperEnabled:    true,
    MaxConcurrent:     5,
    ValidationTimeout: 30 * time.Second,
    MinHealthScore:    0.6,
}

engine := discovery.NewEngine(config, db, logger)
err := engine.RunDiscovery(context.Background())
```

### Validate Single Endpoint

```go
validator := discovery.NewValidator(30*time.Second, logger)

result, err := validator.ValidateEndpoint(
    ctx,
    "https://api.example.com/v1",
    models.AuthTypeAPIKey,
)

if result.IsValid {
    fmt.Printf("Health Score: %.2f\n", result.HealthScore)
}
```

## Testing

```bash
# Run all discovery tests
make test-discovery

# Run with coverage
go test -cover ./internal/discovery/...

# Run specific test
go test -v ./internal/discovery -run TestValidatorBasicConnectivity
```

## Troubleshooting

### No Providers Found

1. Check GitHub token is set: `echo $GITHUB_TOKEN`
2. Verify network connectivity
3. Lower minimum health score: `--min-score=0.3`
4. Check logs for errors

### GitHub Rate Limit

```bash
# Check rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/rate_limit
```

Solution: Add GitHub token or increase discovery interval

### Validation Timeouts

Increase timeout:
```bash
go run ./cmd/backend discovery validate \
  https://api.example.com/v1 \
  --timeout=60s
```

## Best Practices

1. **Use GitHub Token**: Increases rate limit from 60 to 5000 requests/hour
2. **Run Periodically**: Set appropriate interval (12-24h recommended)
3. **Monitor Health Scores**: Regularly verify existing providers
4. **Filter Results**: Adjust min-score based on your needs
5. **Check Logs**: Enable debug logging for troubleshooting

## Advanced Usage

### Custom Search Terms

```go
config := &discovery.DiscoveryConfig{
    DiscoverySearchTerms: []string{
        "custom llm api",
        "my specific search",
    },
}
```

### Scheduled Verification

```go
go discovery.ScheduleVerification(
    ctx,
    db,
    validator,
    6*time.Hour,
    logger,
)
```

### Get Statistics

```go
stats, err := discovery.GetDiscoveryStats(db)
fmt.Printf("Active providers: %d\n", stats["by_status"]["active"])
```
