#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
APP_NAME="goleapai"
BUILD_DIR="./bin"
BINARY_NAME="goleapai"
CONFIG_FILE="./configs/config.yaml"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_dependencies() {
    log_info "Checking dependencies..."

    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi

    if ! command -v docker &> /dev/null; then
        log_warn "Docker is not installed (optional for Docker deployment)"
    fi

    log_info "Dependencies check passed"
}

build_binary() {
    log_info "Building binary..."

    mkdir -p "$BUILD_DIR"

    CGO_ENABLED=1 go build \
        -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
        -o "$BUILD_DIR/$BINARY_NAME" \
        ./cmd/backend

    if [ $? -eq 0 ]; then
        log_info "Binary built successfully: $BUILD_DIR/$BINARY_NAME"
    else
        log_error "Build failed"
        exit 1
    fi
}

run_tests() {
    log_info "Running tests..."

    go test -v -race ./...

    if [ $? -eq 0 ]; then
        log_info "All tests passed"
    else
        log_error "Tests failed"
        exit 1
    fi
}

run_migrations() {
    log_info "Running database migrations..."

    # Le migration vengono eseguite automaticamente all'avvio
    log_info "Migrations will run automatically on startup"
}

start_services() {
    log_info "Starting services..."

    if [ -f "$BUILD_DIR/$BINARY_NAME" ]; then
        "$BUILD_DIR/$BINARY_NAME" --config "$CONFIG_FILE" &
        PID=$!
        echo $PID > "$BUILD_DIR/goleapai.pid"
        log_info "Service started with PID: $PID"
    else
        log_error "Binary not found: $BUILD_DIR/$BINARY_NAME"
        exit 1
    fi
}

stop_services() {
    log_info "Stopping services..."

    if [ -f "$BUILD_DIR/goleapai.pid" ]; then
        PID=$(cat "$BUILD_DIR/goleapai.pid")
        if kill -0 $PID 2>/dev/null; then
            kill $PID
            log_info "Service stopped (PID: $PID)"
            rm "$BUILD_DIR/goleapai.pid"
        else
            log_warn "Process $PID not found"
            rm "$BUILD_DIR/goleapai.pid"
        fi
    else
        log_warn "PID file not found"
    fi
}

health_check() {
    log_info "Running health checks..."

    MAX_RETRIES=10
    RETRY_COUNT=0

    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        if curl -f http://localhost:8080/health &> /dev/null; then
            log_info "Health check passed"
            return 0
        fi

        RETRY_COUNT=$((RETRY_COUNT + 1))
        log_warn "Health check failed (attempt $RETRY_COUNT/$MAX_RETRIES)"
        sleep 2
    done

    log_error "Health check failed after $MAX_RETRIES attempts"
    return 1
}

docker_build() {
    log_info "Building Docker image..."

    docker build -t goleapai:latest .

    if [ $? -eq 0 ]; then
        log_info "Docker image built successfully"
    else
        log_error "Docker build failed"
        exit 1
    fi
}

docker_deploy() {
    log_info "Deploying with Docker Compose..."

    docker-compose up -d

    if [ $? -eq 0 ]; then
        log_info "Docker deployment successful"
    else
        log_error "Docker deployment failed"
        exit 1
    fi
}

show_usage() {
    cat << EOF
Usage: $0 [COMMAND]

Commands:
    build           Build the binary
    test            Run tests
    start           Start the service
    stop            Stop the service
    restart         Restart the service
    health          Run health check
    docker-build    Build Docker image
    docker-deploy   Deploy with Docker Compose
    deploy          Full deployment (build + start + health)

Examples:
    $0 build
    $0 deploy
    $0 docker-deploy
EOF
}

# Main
main() {
    case "$1" in
        build)
            check_dependencies
            build_binary
            ;;
        test)
            check_dependencies
            run_tests
            ;;
        start)
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            stop_services
            sleep 2
            start_services
            ;;
        health)
            health_check
            ;;
        docker-build)
            docker_build
            ;;
        docker-deploy)
            docker_deploy
            ;;
        deploy)
            check_dependencies
            run_tests
            build_binary
            run_migrations
            stop_services 2>/dev/null || true
            sleep 2
            start_services
            sleep 5
            health_check
            ;;
        *)
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
