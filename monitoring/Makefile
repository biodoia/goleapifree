.PHONY: help up down restart logs clean backup restore validate test

# Default target
help:
	@echo "GoLeapAI Monitoring Stack Management"
	@echo ""
	@echo "Available commands:"
	@echo "  make up          - Start monitoring stack"
	@echo "  make down        - Stop monitoring stack"
	@echo "  make restart     - Restart monitoring stack"
	@echo "  make logs        - View logs from all services"
	@echo "  make logs-prom   - View Prometheus logs"
	@echo "  make logs-graf   - View Grafana logs"
	@echo "  make logs-alert  - View Alertmanager logs"
	@echo "  make clean       - Remove all containers and volumes"
	@echo "  make backup      - Backup Prometheus and Grafana data"
	@echo "  make restore     - Restore from backup"
	@echo "  make validate    - Validate configuration files"
	@echo "  make test        - Test alert rules"
	@echo "  make reload-prom - Reload Prometheus configuration"
	@echo "  make reload-graf - Reload Grafana provisioning"
	@echo "  make status      - Show status of all services"

# Start the monitoring stack
up:
	@echo "Starting GoLeapAI monitoring stack..."
	docker-compose up -d
	@echo ""
	@echo "Services started successfully!"
	@echo "Grafana:       http://localhost:3000 (admin/admin)"
	@echo "Prometheus:    http://localhost:9090"
	@echo "Alertmanager:  http://localhost:9093"

# Stop the monitoring stack
down:
	@echo "Stopping GoLeapAI monitoring stack..."
	docker-compose down

# Restart the monitoring stack
restart:
	@echo "Restarting GoLeapAI monitoring stack..."
	docker-compose restart

# View logs from all services
logs:
	docker-compose logs -f

# View Prometheus logs
logs-prom:
	docker-compose logs -f prometheus

# View Grafana logs
logs-graf:
	docker-compose logs -f grafana

# View Alertmanager logs
logs-alert:
	docker-compose logs -f alertmanager

# Clean up everything (WARNING: deletes all data)
clean:
	@echo "WARNING: This will remove all containers and delete all monitoring data!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose down -v; \
		echo "All data cleaned successfully"; \
	else \
		echo "Clean cancelled"; \
	fi

# Backup monitoring data
backup:
	@echo "Creating backup of monitoring data..."
	@mkdir -p backups
	@BACKUP_DATE=$$(date +%Y%m%d_%H%M%S); \
	docker run --rm \
		-v goleapai-monitoring_prometheus_data:/data \
		-v $$(pwd)/backups:/backup \
		alpine tar czf /backup/prometheus-$$BACKUP_DATE.tar.gz /data; \
	docker run --rm \
		-v goleapai-monitoring_grafana_data:/data \
		-v $$(pwd)/backups:/backup \
		alpine tar czf /backup/grafana-$$BACKUP_DATE.tar.gz /data; \
	echo "Backup created in backups/ directory"

# Restore from backup
restore:
	@echo "Available backups:"
	@ls -lh backups/*.tar.gz 2>/dev/null || echo "No backups found"
	@echo ""
	@read -p "Enter backup date (YYYYMMDD_HHMMSS): " BACKUP_DATE; \
	if [ -f "backups/prometheus-$$BACKUP_DATE.tar.gz" ]; then \
		docker run --rm \
			-v goleapai-monitoring_prometheus_data:/data \
			-v $$(pwd)/backups:/backup \
			alpine sh -c "rm -rf /data/* && tar xzf /backup/prometheus-$$BACKUP_DATE.tar.gz -C /"; \
		echo "Prometheus data restored"; \
	fi; \
	if [ -f "backups/grafana-$$BACKUP_DATE.tar.gz" ]; then \
		docker run --rm \
			-v goleapai-monitoring_grafana_data:/data \
			-v $$(pwd)/backups:/backup \
			alpine sh -c "rm -rf /data/* && tar xzf /backup/grafana-$$BACKUP_DATE.tar.gz -C /"; \
		echo "Grafana data restored"; \
	fi

# Validate configuration files
validate:
	@echo "Validating Prometheus configuration..."
	@docker run --rm \
		-v $$(pwd)/prometheus:/etc/prometheus \
		prom/prometheus:latest \
		promtool check config /etc/prometheus/prometheus.yml
	@echo ""
	@echo "Validating Prometheus alerts..."
	@docker run --rm \
		-v $$(pwd)/prometheus:/etc/prometheus \
		prom/prometheus:latest \
		promtool check rules /etc/prometheus/alerts.yml
	@echo ""
	@echo "Validating Alertmanager configuration..."
	@docker run --rm \
		-v $$(pwd)/prometheus:/etc/alertmanager \
		prom/alertmanager:latest \
		amtool check-config /etc/alertmanager/alertmanager.yml
	@echo ""
	@echo "All configurations valid!"

# Test alert rules
test:
	@echo "Testing alert rules..."
	@docker run --rm \
		-v $$(pwd)/prometheus:/etc/prometheus \
		prom/prometheus:latest \
		promtool test rules /etc/prometheus/alerts.yml
	@echo "Alert rules tested successfully!"

# Reload Prometheus configuration
reload-prom:
	@echo "Reloading Prometheus configuration..."
	@curl -X POST http://localhost:9090/-/reload || echo "Failed to reload. Is Prometheus running?"
	@echo "Prometheus configuration reloaded!"

# Reload Grafana provisioning
reload-graf:
	@echo "Reloading Grafana provisioning..."
	docker-compose restart grafana
	@echo "Grafana restarted!"

# Show status of all services
status:
	@echo "Monitoring Stack Status:"
	@echo "------------------------"
	@docker-compose ps
	@echo ""
	@echo "Service URLs:"
	@echo "  Grafana:       http://localhost:3000"
	@echo "  Prometheus:    http://localhost:9090"
	@echo "  Alertmanager:  http://localhost:9093"
	@echo "  Node Exporter: http://localhost:9100"
	@echo ""
	@echo "Prometheus Targets Status:"
	@curl -s http://localhost:9090/api/v1/targets | \
		grep -o '"health":"[^"]*"' | sort | uniq -c || \
		echo "  Unable to fetch (is Prometheus running?)"

# View active alerts
alerts:
	@echo "Active Alerts:"
	@echo "-------------"
	@curl -s http://localhost:9090/api/v1/alerts | \
		jq -r '.data.alerts[] | "\(.labels.alertname): \(.state) - \(.annotations.summary)"' || \
		echo "Unable to fetch alerts (is Prometheus running?)"

# Quick health check
health:
	@echo "Health Check:"
	@echo "------------"
	@echo -n "Prometheus:    "; \
		curl -s http://localhost:9090/-/healthy > /dev/null && echo "✓ Healthy" || echo "✗ Unhealthy"
	@echo -n "Grafana:       "; \
		curl -s http://localhost:3000/api/health > /dev/null && echo "✓ Healthy" || echo "✗ Unhealthy"
	@echo -n "Alertmanager:  "; \
		curl -s http://localhost:9093/-/healthy > /dev/null && echo "✓ Healthy" || echo "✗ Unhealthy"
	@echo -n "Node Exporter: "; \
		curl -s http://localhost:9100/metrics > /dev/null && echo "✓ Healthy" || echo "✗ Unhealthy"

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	@command -v docker >/dev/null 2>&1 || { echo "Docker is not installed. Please install Docker."; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "Docker Compose is not installed. Please install Docker Compose."; exit 1; }
	@echo "All dependencies are installed!"

# Update images
update:
	@echo "Pulling latest images..."
	docker-compose pull
	@echo "Images updated! Run 'make restart' to apply changes."
