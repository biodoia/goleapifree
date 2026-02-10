#!/bin/bash

set -e

echo "╔═══════════════════════════════════════╗"
echo "║  GoLeapAI Monitoring Stack Setup     ║"
echo "╚═══════════════════════════════════════╝"
echo ""

# Check for required tools
echo "→ Checking dependencies..."
command -v docker >/dev/null 2>&1 || { echo "✗ Docker is required but not installed. Aborting."; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "✗ Docker Compose is required but not installed. Aborting."; exit 1; }
echo "✓ Dependencies satisfied"
echo ""

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "→ Creating .env file from template..."
    cp .env.example .env
    echo "✓ Created .env file"
    echo "⚠  Please edit .env file with your settings"
    echo ""
fi

# Validate configurations
echo "→ Validating Prometheus configuration..."
docker run --rm \
    -v $(pwd)/prometheus:/etc/prometheus \
    prom/prometheus:latest \
    promtool check config /etc/prometheus/prometheus.yml > /dev/null 2>&1
echo "✓ Prometheus config valid"

echo "→ Validating Prometheus alerts..."
docker run --rm \
    -v $(pwd)/prometheus:/etc/prometheus \
    prom/prometheus:latest \
    promtool check rules /etc/prometheus/alerts.yml > /dev/null 2>&1
echo "✓ Alert rules valid"

echo "→ Validating Alertmanager configuration..."
docker run --rm \
    -v $(pwd)/prometheus:/etc/alertmanager \
    prom/alertmanager:latest \
    amtool check-config /etc/alertmanager/alertmanager.yml > /dev/null 2>&1
echo "✓ Alertmanager config valid"
echo ""

# Create backup directory
echo "→ Creating backup directory..."
mkdir -p backups
echo "✓ Backup directory created"
echo ""

# Pull images
echo "→ Pulling Docker images..."
docker-compose pull
echo "✓ Images pulled"
echo ""

# Start services
echo "→ Starting monitoring stack..."
docker-compose up -d
echo "✓ Services started"
echo ""

# Wait for services to be ready
echo "→ Waiting for services to be ready..."
sleep 10

# Health check
echo "→ Performing health check..."
PROM_HEALTHY=$(curl -s http://localhost:9090/-/healthy > /dev/null 2>&1 && echo "yes" || echo "no")
GRAF_HEALTHY=$(curl -s http://localhost:3000/api/health > /dev/null 2>&1 && echo "yes" || echo "no")
ALERT_HEALTHY=$(curl -s http://localhost:9093/-/healthy > /dev/null 2>&1 && echo "yes" || echo "no")

echo ""
echo "Service Status:"
echo "  Prometheus:    $([ "$PROM_HEALTHY" = "yes" ] && echo "✓ Healthy" || echo "✗ Unhealthy")"
echo "  Grafana:       $([ "$GRAF_HEALTHY" = "yes" ] && echo "✓ Healthy" || echo "✗ Unhealthy")"
echo "  Alertmanager:  $([ "$ALERT_HEALTHY" = "yes" ] && echo "✓ Healthy" || echo "✗ Unhealthy")"
echo ""

# Display access information
echo "╔═══════════════════════════════════════╗"
echo "║  Setup Complete!                      ║"
echo "╚═══════════════════════════════════════╝"
echo ""
echo "Access the services:"
echo "  📊 Grafana:       http://localhost:3000"
echo "     Username: admin"
echo "     Password: admin (change on first login)"
echo ""
echo "  📈 Prometheus:    http://localhost:9090"
echo "  🔔 Alertmanager:  http://localhost:9093"
echo ""
echo "Next steps:"
echo "  1. Update prometheus/prometheus.yml with your GoLeapAI metrics endpoint"
echo "  2. Configure alertmanager.yml with your notification settings"
echo "  3. Access Grafana and explore the dashboards"
echo ""
echo "Useful commands:"
echo "  make logs        - View all logs"
echo "  make status      - Check service status"
echo "  make alerts      - View active alerts"
echo "  make backup      - Backup monitoring data"
echo "  make help        - Show all available commands"
echo ""
