# Development Guide

Guide for developers contributing to GoLeapAI.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Code Style](#code-style)
- [Testing](#testing)
- [Building](#building)
- [Contributing](#contributing)
- [Debug Tips](#debug-tips)

## Getting Started

### Prerequisites

**Required:**
- Go 1.21 or later
- Git
- Make (optional but recommended)

**Recommended:**
- VS Code with Go extension
- Docker & Docker Compose
- PostgreSQL (for testing)
- Redis (for caching tests)

### Clone Repository

```bash
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree
```

## Development Setup

### Install Dependencies

```bash
# Download Go modules
go mod download

# Install development tools
go install github.com/cosmtrek/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install gotest.tools/gotestsum@latest
```

### Configure Environment

Create `.env.development`:

```bash
# Server
SERVER_PORT=8080
SERVER_HOST=localhost

# Database
DATABASE_TYPE=sqlite
DATABASE_CONNECTION=./data/dev.db
DATABASE_LOG_LEVEL=info

# Logging
LOG_LEVEL=debug
LOG_FORMAT=console

# Development
DEV_MODE=true
HOT_RELOAD=true
```

### Start Development Server

```bash
# With hot reload
air -c .air.toml

# Or manually
go run cmd/backend/main.go serve --log-level debug
```

### Development with Docker

```bash
# Start development stack
docker-compose -f docker-compose.dev.yml up

# This includes:
# - GoLeapAI with hot reload
# - PostgreSQL
# - Redis
# - Adminer (database UI)
```

`docker-compose.dev.yml`:

```yaml
version: '3.8'

services:
  goleapai:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
      - "2345:2345"  # Delve debugger
    volumes:
      - .:/app
      - go-modules:/go/pkg/mod
    environment:
      - DATABASE_TYPE=postgres
      - DATABASE_CONNECTION=postgres://dev:dev@postgres:5432/goleapai_dev
      - REDIS_HOST=redis:6379
      - LOG_LEVEL=debug
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=goleapai_dev
      - POSTGRES_USER=dev
      - POSTGRES_PASSWORD=dev
    ports:
      - "5432:5432"
    volumes:
      - postgres-dev:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  adminer:
    image: adminer
    ports:
      - "8081:8080"

volumes:
  go-modules:
  postgres-dev:
```

### Hot Reload Configuration

`.air.toml`:

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = ["serve", "--log-level", "debug"]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/backend"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html", "yaml"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
```

## Project Structure

```
goleapifree/
├── cmd/                    # Main applications
│   ├── backend/           # Backend gateway server
│   │   └── main.go
│   ├── tui/               # TUI application
│   │   └── main.go
│   └── webui/             # Web UI server (planned)
│       └── main.go
│
├── internal/              # Private application code
│   ├── gateway/          # Core gateway logic
│   │   ├── gateway.go
│   │   └── handlers.go
│   ├── router/           # Intelligent routing
│   │   ├── router.go
│   │   └── strategies.go
│   ├── health/           # Health monitoring
│   │   └── monitor.go
│   ├── providers/        # Provider implementations
│   │   ├── openai.go
│   │   ├── anthropic.go
│   │   └── base.go
│   ├── auth/             # Authentication
│   │   └── auth.go
│   └── discovery/        # Auto-discovery
│       └── scanner.go
│
├── pkg/                   # Public library code
│   ├── models/           # Data models
│   │   ├── provider.go
│   │   ├── model.go
│   │   ├── account.go
│   │   ├── rate_limit.go
│   │   └── stats.go
│   ├── database/         # Database layer
│   │   ├── database.go
│   │   └── seed.go
│   ├── config/           # Configuration
│   │   └── config.go
│   ├── client/           # Go client SDK
│   │   └── client.go
│   └── middleware/       # HTTP middleware
│       └── middleware.go
│
├── web/                   # Web UI assets
│   ├── templates/        # Templ templates
│   └── static/           # CSS, JS, fonts
│
├── configs/              # Configuration files
│   ├── config.yaml
│   └── production.yaml
│
├── scripts/              # Build & deployment scripts
│   ├── build.sh
│   ├── test.sh
│   └── deploy.sh
│
├── docs/                 # Documentation
│   ├── ARCHITECTURE.md
│   ├── API.md
│   ├── PROVIDERS.md
│   └── DEPLOYMENT.md
│
├── tests/                # Additional tests
│   ├── integration/
│   └── e2e/
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

### Package Guidelines

**`cmd/`** - Entry points, minimal logic, just wiring

**`internal/`** - Business logic, not importable by other projects

**`pkg/`** - Reusable libraries, can be imported by others

**`web/`** - Frontend assets, templates

## Code Style

### Go Code Style

Follow standard Go conventions:

```go
// Use gofmt
gofmt -w .

// Use goimports
goimports -w .

// Run linter
golangci-lint run
```

### Naming Conventions

**Packages:**
```go
package gateway  // lowercase, no underscores
```

**Types:**
```go
type ProviderManager struct {}  // PascalCase
type providerCache struct {}    // camelCase for private
```

**Functions:**
```go
func NewGateway() {}           // Exported: PascalCase
func handleRequest() {}        // Private: camelCase
```

**Variables:**
```go
var MaxRetries = 3             // Exported: PascalCase
var defaultTimeout = 30        // Private: camelCase
```

**Constants:**
```go
const (
    StatusActive = "active"     // Exported: PascalCase
    defaultPort  = 8080        // Private: camelCase
)
```

### Comments

```go
// Package gateway implements the core LLM gateway functionality.
package gateway

// Gateway is the main HTTP server handling all requests.
// It manages routing, health monitoring, and provider selection.
type Gateway struct {
    config *config.Config
    db     *database.DB
}

// New creates a new Gateway instance with the given configuration.
// Returns an error if initialization fails.
func New(cfg *config.Config) (*Gateway, error) {
    // Implementation
}
```

### Error Handling

```go
// Good: wrap errors with context
if err := db.Connect(); err != nil {
    return fmt.Errorf("failed to connect to database: %w", err)
}

// Bad: lose context
if err := db.Connect(); err != nil {
    return err
}

// Use custom error types when needed
var ErrProviderNotFound = errors.New("provider not found")

if provider == nil {
    return ErrProviderNotFound
}
```

### Logging

Use zerolog for structured logging:

```go
import "github.com/rs/zerolog/log"

// Info level
log.Info().
    Str("provider", "groq").
    Int("latency_ms", 145).
    Msg("Request completed")

// Error level
log.Error().
    Err(err).
    Str("provider", provider.Name).
    Msg("Provider health check failed")

// Debug level
log.Debug().
    Interface("request", req).
    Msg("Processing request")
```

## Testing

### Unit Tests

```go
// provider_test.go
package models

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestProviderIsAvailable(t *testing.T) {
    provider := &Provider{
        Status:      ProviderStatusActive,
        HealthScore: 0.8,
        LastHealthCheck: time.Now(),
    }

    assert.True(t, provider.IsAvailable())
}

func TestProviderNotAvailable(t *testing.T) {
    provider := &Provider{
        Status:      ProviderStatusDown,
        HealthScore: 0.2,
    }

    assert.False(t, provider.IsAvailable())
}
```

Run tests:

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Verbose
go test -v ./...

# Specific package
go test ./pkg/models

# With race detector
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

```go
// integration_test.go
//go:build integration

package tests

import (
    "testing"
    "github.com/biodoia/goleapifree/pkg/database"
)

func TestDatabaseConnection(t *testing.T) {
    cfg := &database.Config{
        Type:       "sqlite",
        Connection: ":memory:",
    }

    db, err := database.New(cfg)
    assert.NoError(t, err)
    defer db.Close()

    err = db.AutoMigrate()
    assert.NoError(t, err)
}
```

Run integration tests:

```bash
# Run only integration tests
go test -tags=integration ./tests/integration

# Run with test database
DATABASE_TYPE=postgres \
DATABASE_CONNECTION="postgres://test:test@localhost:5432/test" \
go test -tags=integration ./tests/integration
```

### Table-Driven Tests

```go
func TestModelEstimateCost(t *testing.T) {
    tests := []struct {
        name         string
        model        *Model
        inputTokens  int
        outputTokens int
        expectedCost float64
    }{
        {
            name: "free model",
            model: &Model{
                InputPricePer1k:  0.0,
                OutputPricePer1k: 0.0,
            },
            inputTokens:  1000,
            outputTokens: 500,
            expectedCost: 0.0,
        },
        {
            name: "paid model",
            model: &Model{
                InputPricePer1k:  0.01,
                OutputPricePer1k: 0.03,
            },
            inputTokens:  1000,
            outputTokens: 500,
            expectedCost: 0.025,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cost := tt.model.EstimateCost(tt.inputTokens, tt.outputTokens)
            assert.Equal(t, tt.expectedCost, cost)
        })
    }
}
```

### Benchmark Tests

```go
func BenchmarkRouterSelectProvider(b *testing.B) {
    router := setupTestRouter()
    req := &router.Request{
        Model: "gpt-4",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        router.SelectProvider(req)
    }
}
```

Run benchmarks:

```bash
go test -bench=. ./...
go test -bench=. -benchmem ./...  # with memory stats
```

## Building

### Development Build

```bash
# Build backend
go build -o bin/goleapai cmd/backend/main.go

# Build TUI
go build -o bin/goleapai-tui cmd/tui/main.go

# Build all
make build
```

### Production Build

```bash
# Optimized build
go build -ldflags="-s -w" -o bin/goleapai cmd/backend/main.go

# With version info
VERSION=v1.0.0
go build -ldflags="-s -w -X main.version=$VERSION" -o bin/goleapai cmd/backend/main.go

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/goleapai-linux cmd/backend/main.go

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o bin/goleapai.exe cmd/backend/main.go

# Or use Makefile
make release
```

### Makefile

```makefile
.PHONY: build test lint clean

# Variables
BINARY_NAME=goleapai
VERSION?=dev
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Build
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) cmd/backend/main.go
	go build $(LDFLAGS) -o bin/$(BINARY_NAME)-tui cmd/tui/main.go

# Test
test:
	go test -v -race -cover ./...

# Integration tests
test-integration:
	go test -v -tags=integration ./tests/integration

# Lint
lint:
	golangci-lint run

# Format
fmt:
	gofmt -w .
	goimports -w .

# Clean
clean:
	rm -rf bin/ tmp/

# Dev server
dev:
	air -c .air.toml

# Docker build
docker-build:
	docker build -t goleapai/goleapai:$(VERSION) .

# Release (cross-compile)
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 cmd/backend/main.go
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 cmd/backend/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 cmd/backend/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 cmd/backend/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe cmd/backend/main.go
```

Usage:

```bash
make build
make test
make lint
make dev
make release
```

## Contributing

### Workflow

1. **Fork the repository**

2. **Create a feature branch**
```bash
git checkout -b feature/amazing-feature
```

3. **Make your changes**
```bash
# Edit files
# Add tests
# Update documentation
```

4. **Run tests and linting**
```bash
make test
make lint
```

5. **Commit your changes**
```bash
git add .
git commit -m "feat: add amazing feature"
```

Follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `style:` - Formatting
- `refactor:` - Code restructuring
- `test:` - Adding tests
- `chore:` - Maintenance

6. **Push to your fork**
```bash
git push origin feature/amazing-feature
```

7. **Create Pull Request**

### Pull Request Guidelines

**Title:** Clear, descriptive, follows conventional commits

**Description:**
```markdown
## Description
Brief description of changes

## Motivation
Why is this change needed?

## Changes
- Added feature X
- Fixed bug Y
- Updated documentation

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Tests pass
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
```

### Code Review Process

1. Automated checks run (tests, linting)
2. Code review by maintainers
3. Revisions if needed
4. Approval and merge

## Debug Tips

### Enable Debug Logging

```bash
# Environment variable
export LOG_LEVEL=debug

# Command line
./goleapai serve --log-level debug

# Or in config
logging:
  level: debug
```

### Use Delve Debugger

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug main
dlv debug cmd/backend/main.go

# Debug with arguments
dlv debug cmd/backend/main.go -- serve --log-level debug

# Debug tests
dlv test ./pkg/models
```

In VS Code, `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Backend",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/backend",
            "args": ["serve", "--log-level", "debug"],
            "env": {
                "LOG_LEVEL": "debug"
            }
        },
        {
            "name": "Debug Test",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}/pkg/models"
        }
    ]
}
```

### Database Debugging

```bash
# SQLite
sqlite3 data/goleapai.db
.schema
SELECT * FROM providers;

# PostgreSQL
psql -U goleapai -d goleapai
\dt
SELECT * FROM providers;
```

### HTTP Request Debugging

```bash
# Verbose curl
curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer key" \
  -d '{"model":"gpt-4","messages":[]}'

# Use httpie (better than curl)
http POST localhost:8080/v1/chat/completions \
  Authorization:"Bearer key" \
  model=gpt-4 \
  messages:='[{"role":"user","content":"Hi"}]'

# Or use Postman/Insomnia
```

### Common Issues

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
# Kill it
kill -9 <PID>
```

**Database locked (SQLite):**
```bash
# Close all connections or delete db file
rm data/goleapai.db
# Restart to recreate
```

**Module issues:**
```bash
# Clean and download
go clean -modcache
go mod download
go mod tidy
```

**Build errors:**
```bash
# Clean build cache
go clean -cache
# Rebuild
go build -v ./...
```

## Resources

### Go Documentation

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go by Example](https://gobyexample.com/)

### Libraries Used

- [Fiber](https://docs.gofiber.io/) - Web framework
- [GORM](https://gorm.io/docs/) - ORM
- [Viper](https://github.com/spf13/viper) - Configuration
- [Zerolog](https://github.com/rs/zerolog) - Logging
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI

### Community

- GitHub Issues: https://github.com/biodoia/goleapifree/issues
- Discussions: https://github.com/biodoia/goleapifree/discussions
- Discord: (coming soon)

## Conclusion

Thank you for contributing to GoLeapAI! Follow these guidelines to ensure smooth collaboration and maintain code quality.

Happy coding!
