#!/bin/bash
set -e

# Setup script per ambiente di sviluppo GoLeapAI

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
cat << "EOF"
╔═══════════════════════════════════════════════════════════════════╗
║              GoLeapAI - Setup Script                              ║
╚═══════════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check Go installation
log_info "Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    log_info "Go found: $GO_VERSION"
else
    log_warn "Go not found. Please install Go 1.25+"
    exit 1
fi

# Check Docker installation
log_info "Checking Docker installation..."
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version)
    log_info "Docker found: $DOCKER_VERSION"
else
    log_warn "Docker not found (optional)"
fi

# Create directories
log_info "Creating project directories..."
mkdir -p data logs bin backups

# Download Go dependencies
log_info "Downloading Go dependencies..."
go mod download

# Install development tools
log_info "Installing development tools..."

# Install templ if needed
if ! command -v templ &> /dev/null; then
    log_info "Installing templ..."
    go install github.com/a-h/templ/cmd/templ@latest
fi

# Install air for hot reload (optional)
if ! command -v air &> /dev/null; then
    log_info "Installing air (optional)..."
    go install github.com/cosmtrek/air@latest || log_warn "Air installation failed (optional)"
fi

# Install golangci-lint (optional)
if ! command -v golangci-lint &> /dev/null; then
    log_info "Installing golangci-lint (optional)..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest || log_warn "golangci-lint installation failed (optional)"
fi

# Create .env from example if not exists
if [ ! -f ".env" ] && [ -f ".env.example" ]; then
    log_info "Creating .env file..."
    cp .env.example .env
    log_warn "Please configure .env file"
fi

# Generate templ templates
if command -v templ &> /dev/null; then
    log_info "Generating templ templates..."
    templ generate || log_warn "Templ generation failed"
fi

# Start development services with Docker
if command -v docker-compose &> /dev/null; then
    log_info "Starting development services (Postgres, Redis, Prometheus, Grafana)..."
    docker-compose -f docker-compose.dev.yml up -d
    sleep 5
    log_info "Services started:"
    echo "  - PostgreSQL: localhost:5432"
    echo "  - Redis: localhost:6379"
    echo "  - Prometheus: http://localhost:9091"
    echo "  - Grafana: http://localhost:3000 (admin/admin)"
else
    log_warn "docker-compose not found. Skipping services startup."
fi

# Run tests
log_info "Running tests..."
go test ./... || log_warn "Some tests failed"

# Build binaries
log_info "Building binaries..."
make build || go build -o bin/goleapai ./cmd/backend

echo ""
log_info "Setup completed successfully!"
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo "  1. Configure .env file if needed"
echo "  2. Run: make backend (for API Gateway)"
echo "  3. Run: make webui (for Web UI)"
echo "  4. Run: make tui (for Terminal UI)"
echo ""
echo -e "${CYAN}Development services:${NC}"
echo "  - Grafana: http://localhost:3000 (admin/admin)"
echo "  - Prometheus: http://localhost:9091"
echo ""
echo -e "${GREEN}Happy coding!${NC}"
