# GoLeapAI CLI Commands

Complete reference for all GoLeapAI command-line interface commands.

## Table of Contents

- [Global Flags](#global-flags)
- [serve - Start Gateway](#serve---start-gateway)
- [providers - Provider Management](#providers---provider-management)
- [stats - Statistics](#stats---statistics)
- [config - Configuration](#config---configuration)
- [migrate - Database Migrations](#migrate---database-migrations)
- [doctor - Health Diagnostics](#doctor---health-diagnostics)

---

## Global Flags

Available for all commands:

```bash
-c, --config string      Path to config file
-l, --log-level string   Log level (debug, info, warn, error) (default: info)
```

---

## serve - Start Gateway

Start the GoLeapAI gateway server with all features enabled.

### Usage

```bash
goleapai serve [flags]
```

### Flags

- `--dev` - Enable development mode (pretty logging, hot reload)
- `-v, --verbose` - Enable verbose logging (debug level)
- `--migrate` - Auto-run database migrations on startup (default: true)

### Examples

```bash
# Start server with default settings
goleapai serve

# Start in development mode with verbose logging
goleapai serve --dev --verbose

# Start with auto-migration enabled
goleapai serve --migrate

# Start with custom config
goleapai serve -c /path/to/config.yaml

# Start without auto-migration
goleapai serve --migrate=false
```

### Features

- Graceful shutdown on SIGINT/SIGTERM
- HTTP/3 support (configurable)
- Auto-migration support
- Health monitoring
- Prometheus metrics

---

## providers - Provider Management

Manage LLM provider configurations.

### Subcommands

#### list - List all providers

```bash
goleapai providers list [flags]
```

**Flags:**
- `--status string` - Filter by status (active, deprecated, down)
- `--json` - Output as JSON

**Examples:**
```bash
# List all providers
goleapai providers list

# List only active providers
goleapai providers list --status active

# List with JSON output
goleapai providers list --json
```

---

#### add - Add a new provider

```bash
goleapai providers add [flags]
```

**Flags:**
- `--name string` - Provider name (required)
- `--url string` - Provider base URL (required)
- `--type string` - Provider type (free, freemium, paid) (default: free)
- `--auth-type string` - Authentication type (none, api_key, bearer, oauth2) (default: api_key)

**Examples:**
```bash
# Add a new provider
goleapai providers add --name "MyAPI" --url "https://api.example.com" --type free

# Add with authentication
goleapai providers add --name "MyAPI" --url "https://api.example.com" --auth-type api_key
```

---

#### remove - Remove a provider

```bash
goleapai providers remove [provider-name]
```

**Examples:**
```bash
# Remove by name
goleapai providers remove groq

# Remove by ID
goleapai providers remove 550e8400-e29b-41d4-a716-446655440000
```

---

#### test - Test provider connection

```bash
goleapai providers test [provider-name] [flags]
```

**Flags:**
- `--all` - Test all providers

**Examples:**
```bash
# Test a single provider
goleapai providers test groq

# Test all providers
goleapai providers test --all
```

---

#### sync - Sync providers from discovery

```bash
goleapai providers sync [flags]
```

**Flags:**
- `--source string` - Sync from specific source (github, scraper)

**Examples:**
```bash
# Sync all providers
goleapai providers sync

# Sync from specific source
goleapai providers sync --source github
```

---

## stats - Statistics

View and manage aggregated statistics.

### Subcommands

#### show - Show aggregated statistics

```bash
goleapai stats show [flags]
```

**Flags:**
- `--provider string` - Filter by provider name
- `--from string` - Start date (YYYY-MM-DD)
- `--to string` - End date (YYYY-MM-DD)
- `--json` - Output as JSON

**Examples:**
```bash
# Show all stats
goleapai stats show

# Show stats for specific provider
goleapai stats show --provider groq

# Show stats for date range
goleapai stats show --from 2024-01-01 --to 2024-01-31

# Show in JSON format
goleapai stats show --json
```

---

#### export - Export statistics to file

```bash
goleapai stats export [flags]
```

**Flags:**
- `--provider string` - Filter by provider name
- `--from string` - Start date (YYYY-MM-DD)
- `--to string` - End date (YYYY-MM-DD)
- `--format string` - Export format (csv, json) (default: csv)
- `-o, --output string` - Output file path (required)

**Examples:**
```bash
# Export to CSV
goleapai stats export --format csv -o stats.csv

# Export to JSON
goleapai stats export --format json -o stats.json

# Export specific provider
goleapai stats export --provider groq -o groq-stats.csv

# Export date range
goleapai stats export --from 2024-01-01 --to 2024-01-31 -o january-stats.csv
```

---

#### reset - Reset statistics

```bash
goleapai stats reset [flags]
```

**Flags:**
- `--provider string` - Reset stats for specific provider
- `--confirm` - Confirm reset action (required)

**Examples:**
```bash
# Reset all stats (requires confirmation)
goleapai stats reset --confirm

# Reset stats for specific provider
goleapai stats reset --provider groq --confirm
```

---

## config - Configuration

Manage GoLeapAI configuration files.

### Subcommands

#### show - Show current configuration

```bash
goleapai config show
```

**Examples:**
```bash
# Show default config
goleapai config show

# Show specific config file
goleapai config show -c /path/to/config.yaml
```

---

#### validate - Validate configuration file

```bash
goleapai config validate
```

**Examples:**
```bash
# Validate default config
goleapai config validate

# Validate specific config
goleapai config validate -c config.yaml
```

---

#### generate - Generate template configuration

```bash
goleapai config generate [flags]
```

**Flags:**
- `-o, --output string` - Output file path (stdout if not specified)
- `--env string` - Environment (development, production) (default: development)

**Examples:**
```bash
# Generate to stdout
goleapai config generate

# Generate to file
goleapai config generate -o config.yaml

# Generate production config
goleapai config generate --env production -o prod.yaml
```

---

## migrate - Database Migrations

Manage database schema migrations.

### Subcommands

#### up - Run pending migrations

```bash
goleapai migrate up
```

**Examples:**
```bash
# Run migrations
goleapai migrate up

# Run migrations with specific config
goleapai migrate up -c config.yaml
```

---

#### down - Rollback migrations

```bash
goleapai migrate down [flags]
```

**Flags:**
- `--confirm` - Confirm rollback action

**Examples:**
```bash
# Rollback last migration
goleapai migrate down --confirm
```

---

#### reset - Reset database

```bash
goleapai migrate reset [flags]
```

**Flags:**
- `--confirm` - Confirm reset action (required)

**Examples:**
```bash
# Reset database (requires confirmation)
goleapai migrate reset --confirm
```

**Warning:** This will delete all data!

---

#### seed - Seed initial data

```bash
goleapai migrate seed [flags]
```

**Flags:**
- `--force` - Force re-seed even if data exists

**Examples:**
```bash
# Seed database
goleapai migrate seed

# Force re-seed
goleapai migrate seed --force
```

---

#### status - Show migration status

```bash
goleapai migrate status
```

**Examples:**
```bash
# Show migration status
goleapai migrate status
```

---

## doctor - Health Diagnostics

Run comprehensive health checks on the GoLeapAI system.

### Usage

```bash
goleapai doctor [flags]
```

### Flags

- `--check string` - Run specific check (database, redis, providers)
- `--provider string` - Check specific provider
- `-v, --verbose` - Verbose output

### Examples

```bash
# Run full diagnostic
goleapai doctor

# Check only database
goleapai doctor --check database

# Check specific provider
goleapai doctor --provider groq

# Verbose output
goleapai doctor --verbose
```

### Health Checks

1. **Database** - Connection, ping, schema validation
2. **Redis** - Connection and availability (optional)
3. **Providers** - HTTP connectivity and health status

---

## Quick Start Examples

### Initial Setup

```bash
# 1. Generate configuration
goleapai config generate -o config.yaml

# 2. Run migrations and seed
goleapai migrate up
goleapai migrate seed

# 3. Verify system health
goleapai doctor

# 4. Start the server
goleapai serve --dev
```

### Development Workflow

```bash
# Start in development mode
goleapai serve --dev --verbose

# In another terminal, check providers
goleapai providers list

# Add a custom provider
goleapai providers add --name "CustomAPI" --url "https://api.custom.com"

# Test the provider
goleapai providers test CustomAPI

# View statistics
goleapai stats show
```

### Production Deployment

```bash
# Generate production config
goleapai config generate --env production -o prod.yaml

# Validate configuration
goleapai config validate -c prod.yaml

# Run migrations
goleapai migrate up -c prod.yaml

# Run health check
goleapai doctor -c prod.yaml

# Start server
goleapai serve -c prod.yaml
```

### Monitoring & Maintenance

```bash
# Check system health
goleapai doctor

# View statistics
goleapai stats show

# Export monthly stats
goleapai stats export --from 2024-01-01 --to 2024-01-31 -o january.csv

# Sync providers from discovery
goleapai providers sync

# Test all providers
goleapai providers test --all
```

---

## Configuration File

Example `config.yaml`:

```yaml
server:
  port: 8080
  host: 0.0.0.0
  http3: true
  tls:
    enabled: false
    cert: ""
    key: ""

database:
  type: sqlite
  connection: ./data/goleapai.db
  max_conns: 25
  log_level: warn

redis:
  host: localhost:6379
  password: ""
  db: 0

providers:
  auto_discovery: true
  health_check_interval: 5m
  default_timeout: 30s

routing:
  strategy: cost_optimized
  failover_enabled: true
  max_retries: 3

monitoring:
  prometheus:
    enabled: true
    port: 9090
  logging:
    level: info
    format: json
```

---

## Environment Variables

All configuration can be overridden with environment variables:

```bash
GOLEAPAI_SERVER_PORT=8080
GOLEAPAI_SERVER_HOST=0.0.0.0
GOLEAPAI_DATABASE_TYPE=postgres
GOLEAPAI_DATABASE_CONNECTION="host=localhost user=goleapai password=secret dbname=goleapai"
```

---

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Configuration error
- `3` - Database error
- `4` - Network error

---

## Support

For issues and questions:
- GitHub: https://github.com/biodoia/goleapifree
- Documentation: https://goleapai.dev/docs
