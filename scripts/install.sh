#!/bin/bash
set -e

# Installation script for GoLeapAI
# This script installs GoLeapAI as a systemd service

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
INSTALL_DIR="/opt/goleapai"
BINARY_NAME="goleapai"
SERVICE_USER="goleapai"
SERVICE_GROUP="goleapai"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root or with sudo"
        exit 1
    fi
}

create_user() {
    log_info "Creating service user..."

    if ! id -u "$SERVICE_USER" > /dev/null 2>&1; then
        useradd --system --no-create-home --shell /bin/false "$SERVICE_USER"
        log_info "User $SERVICE_USER created"
    else
        log_info "User $SERVICE_USER already exists"
    fi
}

create_directories() {
    log_info "Creating installation directories..."

    mkdir -p "$INSTALL_DIR"/{bin,configs,data,logs}
    chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"
    chmod 750 "$INSTALL_DIR"

    log_info "Directories created"
}

install_binary() {
    log_info "Installing binary..."

    if [ ! -f "./bin/$BINARY_NAME" ]; then
        log_error "Binary not found. Please run 'make build' first"
        exit 1
    fi

    cp "./bin/$BINARY_NAME" "$INSTALL_DIR/bin/"
    chown "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR/bin/$BINARY_NAME"
    chmod 750 "$INSTALL_DIR/bin/$BINARY_NAME"

    log_info "Binary installed"
}

install_config() {
    log_info "Installing configuration..."

    cp configs/config.yaml "$INSTALL_DIR/configs/"
    chown "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR/configs/config.yaml"
    chmod 640 "$INSTALL_DIR/configs/config.yaml"

    # Create environment file
    if [ -f ".env" ]; then
        mkdir -p /etc/goleapai
        cp .env /etc/goleapai/environment
        chmod 600 /etc/goleapai/environment
    fi

    log_info "Configuration installed"
}

install_systemd() {
    log_info "Installing systemd service..."

    cp systemd/goleapai.service /etc/systemd/system/
    chmod 644 /etc/systemd/system/goleapai.service

    systemctl daemon-reload
    log_info "Systemd service installed"
}

enable_service() {
    log_info "Enabling service..."

    systemctl enable goleapai.service
    log_info "Service enabled"
}

start_service() {
    log_info "Starting service..."

    systemctl start goleapai.service
    sleep 2

    if systemctl is-active --quiet goleapai.service; then
        log_info "Service started successfully"
    else
        log_error "Service failed to start. Check logs with: journalctl -u goleapai"
        exit 1
    fi
}

show_status() {
    log_info "Service status:"
    systemctl status goleapai.service --no-pager
}

main() {
    log_info "Installing GoLeapAI..."

    check_root
    create_user
    create_directories
    install_binary
    install_config
    install_systemd
    enable_service
    start_service
    show_status

    log_info "Installation complete!"
    echo ""
    log_info "Useful commands:"
    echo "  - View logs: sudo journalctl -u goleapai -f"
    echo "  - Stop service: sudo systemctl stop goleapai"
    echo "  - Restart service: sudo systemctl restart goleapai"
    echo "  - Check status: sudo systemctl status goleapai"
}

main
