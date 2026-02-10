# GoLeapAI Debug CLI

Advanced debugging and troubleshooting tool for GoLeapAI development and production environments.

## Overview

The Debug CLI provides comprehensive tools for:

- **Request Tracing**: Inspect request flow and decision points
- **Provider Testing**: Test provider connectivity and features
- **Routing Simulation**: Simulate routing decisions without making actual requests
- **Cache Inspection**: View cache statistics and manage cache entries
- **Performance Profiling**: CPU, memory, goroutine, and heap analysis
- **Configuration Validation**: Validate YAML, database, Redis, and provider connectivity

## Installation

```bash
# Build the debug tool
cd cmd/debug
go build -o goleapai-debug

# Or build from project root
make build-debug
```

## Usage

### Request Debugging

Inspect detailed information about a specific request:

```bash
# Inspect request by ID
goleapai-debug request <request-id>

# Output as JSON
goleapai-debug request <request-id> --json

# Verbose output
goleapai-debug request <request-id> --verbose
```

**Shows:**
- Request metadata and parameters
- Provider selection decision
- Response details and timing
- Error information
- Cache hit/miss information

### Provider Testing

Test provider connectivity and capabilities:

```bash
# Test a provider
goleapai-debug provider groq

# Test with health check
goleapai-debug provider groq --health

# List available models
goleapai-debug provider groq --models

# Full test (endpoint + health + models)
goleapai-debug provider groq --test --health --models
```

**Features:**
- HTTP connectivity test
- Authentication validation
- Model listing
- Feature detection
- Latency measurement

### Routing Simulation

Simulate routing decisions without making actual API calls:

```bash
# Simulate routing for a prompt
goleapai-debug routing "What is the capital of France?"

# Specify model and parameters
goleapai-debug routing "Hello" \
  --model gpt-4 \
  --max-tokens 1000 \
  --temperature 0.7

# Show all candidate providers
goleapai-debug routing "Test" --show-all

# Stream mode
goleapai-debug routing "Test" --stream
```

**Shows:**
- Provider selection reasoning
- Cost estimation
- Quality score
- Latency prediction
- Alternative providers

### Cache Inspection

Inspect and manage cache state:

```bash
# Show cache statistics
goleapai-debug cache --stats

# Inspect specific cache key
goleapai-debug cache --key "cache:key:here"

# Clear all cache
goleapai-debug cache --clear

# JSON output
goleapai-debug cache --stats --json
```

**Operations:**
- View cache statistics
- Inspect specific cache keys
- Clear cache
- Show hit/miss ratios

### Performance Profiling

#### CPU Profiling

Profile CPU usage for a specified duration:

```bash
# Profile CPU for 30 seconds (default)
goleapai-debug profile cpu

# Custom duration
goleapai-debug profile cpu --duration 60

# Analyze the profile
go tool pprof profiles/cpu_*.prof
go tool pprof -http=:8080 profiles/cpu_*.prof
```

#### Memory Profiling

Generate memory profile and statistics:

```bash
# Generate memory profile
goleapai-debug profile memory

# Analyze the profile
go tool pprof profiles/mem_*.prof
go tool pprof -http=:8080 profiles/mem_*.prof
```

**Shows:**
- Allocated memory
- Total allocations
- System memory
- Heap statistics
- GC statistics

#### Goroutine Analysis

Analyze goroutine usage:

```bash
# Analyze goroutines
goleapai-debug profile goroutines

# Analyze the profile
go tool pprof profiles/goroutine_*.prof
```

**Shows:**
- Active goroutine count
- GOMAXPROCS setting
- CPU count
- Warning for goroutine leaks

#### Heap Dump

Generate detailed heap dump:

```bash
# Generate heap dump
goleapai-debug profile heap

# Analyze with pprof
go tool pprof profiles/heap_*.prof
go tool pprof -http=:8080 profiles/heap_*.prof
```

**Shows:**
- Heap allocation details
- Memory pool usage
- GC statistics
- Heap fragmentation
- Object counts

### Configuration Validation

Validate system configuration and connectivity:

```bash
# Validate all
goleapai-debug validate

# Validate specific components
goleapai-debug validate --yaml
goleapai-debug validate --database
goleapai-debug validate --redis
goleapai-debug validate --providers

# Skip Redis check
goleapai-debug validate --redis=false
```

**Checks:**
- YAML configuration syntax
- Configuration value validity
- Database connection and schema
- Redis connection (if configured)
- Provider connectivity
- Cache functionality

## Global Flags

All commands support these global flags:

```bash
-c, --config string    Path to config file
-v, --verbose          Verbose output
-j, --json            Output as JSON
```

## Examples

### Debugging a Failed Request

```bash
# Get request trace
goleapai-debug request abc123-def456 -v

# Check provider that was selected
goleapai-debug provider groq --health

# Validate configuration
goleapai-debug validate
```

### Performance Investigation

```bash
# Check current memory usage
goleapai-debug profile memory

# Profile CPU for 1 minute during load test
goleapai-debug profile cpu --duration 60

# Check for goroutine leaks
goleapai-debug profile goroutines

# Generate full heap dump
goleapai-debug profile heap
```

### Provider Troubleshooting

```bash
# Test specific provider
goleapai-debug provider openai --test --models

# Simulate routing to see which provider would be selected
goleapai-debug routing "Test prompt" --show-all

# Validate all provider connectivity
goleapai-debug validate --providers
```

### Cache Analysis

```bash
# Check cache statistics
goleapai-debug cache --stats

# Inspect specific cached response
goleapai-debug cache --key "response:abc123"

# Clear cache to test fresh routing
goleapai-debug cache --clear
```

## Output Formats

### Standard Output

Human-readable formatted output with colors and tables.

### JSON Output

Machine-readable JSON for integration with other tools:

```bash
goleapai-debug request <id> --json | jq .
goleapai-debug cache --stats --json | jq '.hit_rate'
```

## Profiling Tips

### Using pprof Web Interface

```bash
# Start HTTP server for interactive analysis
go tool pprof -http=:8080 profiles/cpu_*.prof
```

Navigate to http://localhost:8080 to see:
- Flame graphs
- Call graphs
- Top functions
- Source code view

### Common pprof Commands

```
(pprof) top           # Show top consumers
(pprof) top10         # Show top 10
(pprof) list funcName # Show source code
(pprof) web           # Open graph in browser
(pprof) png           # Generate PNG graph
(pprof) pdf           # Generate PDF report
```

## Integration with Production

### Health Check Script

```bash
#!/bin/bash
# health-check.sh

goleapai-debug validate --json > /tmp/health.json

if [ $? -eq 0 ]; then
    echo "Health check passed"
    exit 0
else
    echo "Health check failed"
    cat /tmp/health.json | jq .
    exit 1
fi
```

### Automated Profiling

```bash
#!/bin/bash
# profile-cron.sh - Run from cron every hour

DATE=$(date +%Y%m%d_%H%M%S)
goleapai-debug profile memory > /var/log/goleapai/profile_${DATE}.log
goleapai-debug cache --stats --json > /var/log/goleapai/cache_${DATE}.json
```

### Monitoring Integration

Export metrics to monitoring systems:

```bash
# Export cache stats to Prometheus format
goleapai-debug cache --stats --json | \
  jq -r '"cache_hit_rate \(.hit_rate)\ncache_size \(.size)"'
```

## Troubleshooting

### "Failed to connect to database"

- Check database configuration in config.yaml
- Verify database is running: `systemctl status postgresql`
- Test connection manually: `psql -h localhost -U user -d dbname`

### "Provider not found"

- List available providers in database
- Run migrations: `goleapai migrate`
- Seed providers: `goleapai migrate seed`

### "Redis connection failed"

- Check Redis is running: `redis-cli ping`
- Verify Redis configuration in config.yaml
- Redis is optional - system works without it

### "Permission denied" for profiles directory

```bash
# Create profiles directory
mkdir -p profiles
chmod 755 profiles
```

## Development

### Building

```bash
# Build debug tool
go build -o goleapai-debug

# Build with race detector
go build -race -o goleapai-debug

# Build for production
go build -ldflags="-s -w" -o goleapai-debug
```

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Benchmark
go test -bench=. ./...
```

## Advanced Usage

### Custom Tracing

Trace a request through the entire system:

```go
tracer := NewTracer(db, config)
trace, err := tracer.TraceRequest(ctx, requestID)
if err != nil {
    log.Fatal(err)
}

tracer.ShowRequestFlow(ctx, requestID)
```

### Provider Comparison

Compare multiple providers:

```go
tracer := NewTracer(db, config)
providers := []string{"groq", "openai", "anthropic"}
tracer.CompareProviders(ctx, providers)
```

### Custom Validators

Add custom validation checks:

```go
validator := NewValidator(configPath)
validator.ValidateYAML()
validator.ValidateDatabase()
validator.ValidateProviders()
```

## See Also

- [API Documentation](../../docs/API.md)
- [Architecture](../../docs/ARCHITECTURE.md)
- [Backend Commands](../backend/README.md)
- [TUI Interface](../tui/README.md)

## License

Same as GoLeapAI project.
