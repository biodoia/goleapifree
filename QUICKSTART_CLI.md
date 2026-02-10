# GoLeapAI CLI Quick Start

Quick guide to get started with GoLeapAI command-line interface.

## Installation

```bash
# Clone the repository
git clone https://github.com/biodoia/goleapifree
cd goleapifree

# Install dependencies
make deps

# Build the binary
make build

# The binary will be in bin/goleapai
```

## First Steps

### 1. Check Version

```bash
./bin/goleapai version
```

### 2. Generate Configuration

```bash
# Generate default config
./bin/goleapai config generate -o config.yaml

# Or generate production config
./bin/goleapai config generate --env production -o prod.yaml
```

### 3. Initialize Database

```bash
# Run migrations and seed data
./bin/goleapai migrate up
./bin/goleapai migrate seed

# Check migration status
./bin/goleapai migrate status
```

### 4. Verify System Health

```bash
# Run comprehensive health check
./bin/goleapai doctor

# Check only database
./bin/goleapai doctor --check database
```

### 5. Start the Server

```bash
# Development mode (pretty logs, verbose)
./bin/goleapai serve --dev

# Or use make
make dev

# Production mode
./bin/goleapai serve -c prod.yaml
```

## Common Tasks

### Provider Management

```bash
# List all providers
./bin/goleapai providers list

# List only active providers
./bin/goleapai providers list --status active

# Add a custom provider
./bin/goleapai providers add \
  --name "MyAPI" \
  --url "https://api.example.com" \
  --type free

# Test a provider
./bin/goleapai providers test groq

# Test all providers
./bin/goleapai providers test --all

# Remove a provider
./bin/goleapai providers remove "MyAPI"

# Sync from auto-discovery
./bin/goleapai providers sync
```

### Statistics

```bash
# Show current statistics
./bin/goleapai stats show

# Show stats for specific provider
./bin/goleapai stats show --provider groq

# Show stats for date range
./bin/goleapai stats show \
  --from 2024-01-01 \
  --to 2024-01-31

# Export to CSV
./bin/goleapai stats export \
  --format csv \
  -o stats.csv

# Export to JSON
./bin/goleapai stats export \
  --format json \
  -o stats.json

# Reset statistics (with confirmation)
./bin/goleapai stats reset --confirm
```

### Configuration

```bash
# Show current configuration
./bin/goleapai config show

# Validate configuration
./bin/goleapai config validate -c config.yaml

# Generate new config
./bin/goleapai config generate -o new-config.yaml
```

### Database Migrations

```bash
# Run all pending migrations
./bin/goleapai migrate up

# Seed database with initial data
./bin/goleapai migrate seed

# Force re-seed
./bin/goleapai migrate seed --force

# Show migration status
./bin/goleapai migrate status

# Reset database (DESTRUCTIVE!)
./bin/goleapai migrate reset --confirm
```

## Development Workflow

### Using Make

```bash
# Show all available targets
make help

# Development mode
make dev

# Run tests
make test

# Format code
make fmt

# Build for all platforms
make build-all

# Initialize database
make init-db

# Run health check
make doctor
```

### Using Direct Commands

```bash
# Build
go build -o bin/goleapai ./cmd/backend

# Run with custom config
./bin/goleapai serve -c custom.yaml

# Enable verbose logging
./bin/goleapai serve --verbose

# Run without auto-migration
./bin/goleapai serve --migrate=false
```

## Production Deployment

### Setup

```bash
# 1. Generate production config
./bin/goleapai config generate --env production -o prod.yaml

# 2. Edit configuration
nano prod.yaml

# 3. Validate config
./bin/goleapai config validate -c prod.yaml

# 4. Run migrations
./bin/goleapai migrate up -c prod.yaml

# 5. Run health check
./bin/goleapai doctor -c prod.yaml

# 6. Start server
./bin/goleapai serve -c prod.yaml
```

### Systemd Service

Create `/etc/systemd/system/goleapai.service`:

```ini
[Unit]
Description=GoLeapAI Gateway
After=network.target

[Service]
Type=simple
User=goleapai
WorkingDirectory=/opt/goleapai
ExecStart=/opt/goleapai/bin/goleapai serve -c /etc/goleapai/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable goleapai
sudo systemctl start goleapai
sudo systemctl status goleapai
```

## Monitoring

### Real-time Logs

```bash
# Follow logs (if using systemd)
journalctl -u goleapai -f

# With verbose output
./bin/goleapai serve --dev --verbose
```

### Health Checks

```bash
# Check system health
./bin/goleapai doctor

# Check specific provider
./bin/goleapai doctor --provider groq

# Verbose diagnostics
./bin/goleapai doctor --verbose
```

### Statistics

```bash
# View current stats
./bin/goleapai stats show

# Export for analysis
./bin/goleapai stats export \
  --from $(date -d '30 days ago' +%Y-%m-%d) \
  --to $(date +%Y-%m-%d) \
  -o monthly-stats.csv
```

## Troubleshooting

### Database Issues

```bash
# Check migration status
./bin/goleapai migrate status

# Reset and recreate
./bin/goleapai migrate reset --confirm
./bin/goleapai migrate up
./bin/goleapai migrate seed
```

### Provider Issues

```bash
# Test all providers
./bin/goleapai providers test --all

# Check system health
./bin/goleapai doctor --check providers

# Re-sync providers
./bin/goleapai providers sync
```

### Configuration Issues

```bash
# Validate configuration
./bin/goleapai config validate

# Show current config
./bin/goleapai config show

# Generate fresh config
./bin/goleapai config generate -o fresh-config.yaml
```

## Advanced Usage

### Custom Database

```bash
# PostgreSQL
./bin/goleapai serve -c postgres.yaml

# With environment variables
GOLEAPAI_DATABASE_TYPE=postgres \
GOLEAPAI_DATABASE_CONNECTION="host=localhost user=..." \
./bin/goleapai serve
```

### Multiple Instances

```bash
# Instance 1 (port 8080)
./bin/goleapai serve -c config1.yaml

# Instance 2 (port 8081)
./bin/goleapai serve -c config2.yaml
```

### Backup & Restore

```bash
# Backup database (SQLite)
cp data/goleapai.db data/goleapai.backup.db

# Export all stats
./bin/goleapai stats export -o backup-stats.json

# Restore
cp data/goleapai.backup.db data/goleapai.db
```

## Shell Completion

### Bash

```bash
# Install completion
make completion

# Or manually
source scripts/completion.bash

# Add to .bashrc for persistence
echo "source /path/to/goleapai/scripts/completion.bash" >> ~/.bashrc
```

### Usage

```bash
# Tab completion works for all commands
goleapai <TAB>
goleapai providers <TAB>
goleapai stats <TAB>

# Flag completion
goleapai serve --<TAB>
goleapai providers list --status <TAB>
```

## Environment Variables

Override any config value:

```bash
# Server settings
export GOLEAPAI_SERVER_PORT=8080
export GOLEAPAI_SERVER_HOST=0.0.0.0

# Database settings
export GOLEAPAI_DATABASE_TYPE=postgres
export GOLEAPAI_DATABASE_CONNECTION="host=localhost..."

# Run with env vars
./bin/goleapai serve
```

## Help & Documentation

```bash
# Global help
./bin/goleapai --help

# Command-specific help
./bin/goleapai serve --help
./bin/goleapai providers --help
./bin/goleapai stats --help

# Subcommand help
./bin/goleapai providers list --help
./bin/goleapai stats export --help
```

## Next Steps

- Read the [full CLI documentation](docs/CLI_COMMANDS.md)
- Explore the [API documentation](docs/API.md)
- Check the [architecture guide](docs/ARCHITECTURE.md)
- Join the community on GitHub

## Support

- GitHub Issues: https://github.com/biodoia/goleapifree/issues
- Documentation: https://goleapai.dev/docs
- Discord: https://discord.gg/goleapai
