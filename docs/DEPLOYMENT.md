# Deployment Guide

Complete guide for deploying GoLeapAI in production.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Deployment Options](#deployment-options)
- [Docker Deployment](#docker-deployment)
- [Systemd Service](#systemd-service)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Reverse Proxy Setup](#reverse-proxy-setup)
- [Environment Variables](#environment-variables)
- [Production Checklist](#production-checklist)
- [Scaling](#scaling)
- [Monitoring](#monitoring)
- [Backup & Recovery](#backup--recovery)

## Prerequisites

### System Requirements

**Minimum:**
- CPU: 2 cores
- RAM: 1 GB
- Storage: 10 GB
- OS: Linux, macOS, or Windows

**Recommended:**
- CPU: 4+ cores
- RAM: 4 GB
- Storage: 50 GB SSD
- OS: Linux (Ubuntu 22.04 LTS or newer)

### Software Dependencies

**Required:**
- Go 1.21+ (for building from source)
- PostgreSQL 14+ or SQLite 3.35+ (database)
- Redis 7+ (optional, for caching)

**Optional:**
- Docker 24+
- Kubernetes 1.28+
- Nginx or Caddy (reverse proxy)
- Prometheus (metrics)
- Grafana (dashboards)

## Deployment Options

### 1. Binary Deployment

Simplest option - single binary with SQLite.

**Pros:**
- No dependencies
- Fast startup
- Easy updates

**Cons:**
- Single point of failure
- Limited scalability

**Use case:** Development, small deployments (<100 req/min)

### 2. Docker Deployment

Containerized deployment with PostgreSQL.

**Pros:**
- Isolated environment
- Easy to replicate
- Version control

**Cons:**
- Docker overhead
- More complex setup

**Use case:** Medium deployments (100-1000 req/min)

### 3. Kubernetes Deployment

Cloud-native with auto-scaling.

**Pros:**
- High availability
- Auto-scaling
- Rolling updates

**Cons:**
- Complex setup
- Higher costs

**Use case:** Large deployments (>1000 req/min)

## Docker Deployment

### Dockerfile

Create `Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /goleapai cmd/backend/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /goleapai .

# Copy configs
COPY configs/ ./configs/

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/goleapai"]
CMD ["serve"]
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  goleapai:
    build: .
    container_name: goleapai
    ports:
      - "8080:8080"
      - "9090:9090"  # Prometheus metrics
    environment:
      - DATABASE_TYPE=postgres
      - DATABASE_CONNECTION=postgres://goleapai:password@postgres:5432/goleapai
      - REDIS_HOST=redis:6379
      - LOG_LEVEL=info
    depends_on:
      - postgres
      - redis
    restart: unless-stopped
    volumes:
      - ./configs:/app/configs:ro
      - goleapai-data:/app/data
    networks:
      - goleapai-network

  postgres:
    image: postgres:16-alpine
    container_name: goleapai-postgres
    environment:
      - POSTGRES_DB=goleapai
      - POSTGRES_USER=goleapai
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres-data:/var/lib/postgresql/data
    restart: unless-stopped
    networks:
      - goleapai-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U goleapai"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: goleapai-redis
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    restart: unless-stopped
    networks:
      - goleapai-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  goleapai-data:
  postgres-data:
  redis-data:

networks:
  goleapai-network:
    driver: bridge
```

### Build and Run

```bash
# Build images
docker-compose build

# Start services
docker-compose up -d

# View logs
docker-compose logs -f goleapai

# Check status
docker-compose ps

# Stop services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Docker Commands

```bash
# Run standalone (SQLite)
docker run -d \
  --name goleapai \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  goleapai/goleapai:latest

# Run with environment variables
docker run -d \
  --name goleapai \
  -p 8080:8080 \
  -e DATABASE_TYPE=postgres \
  -e DATABASE_CONNECTION="postgres://user:pass@host:5432/db" \
  -e LOG_LEVEL=info \
  goleapai/goleapai:latest

# Execute commands in container
docker exec -it goleapai /app/goleapai --help

# View logs
docker logs -f goleapai

# Update to latest
docker pull goleapai/goleapai:latest
docker-compose up -d
```

## Systemd Service

For running as a system service on Linux.

### Service File

Create `/etc/systemd/system/goleapai.service`:

```ini
[Unit]
Description=GoLeapAI Gateway
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=goleapai
Group=goleapai
WorkingDirectory=/opt/goleapai
ExecStart=/opt/goleapai/bin/goleapai serve --config /opt/goleapai/configs/production.yaml
Restart=on-failure
RestartSec=5s

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/goleapai/data

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=goleapai

# Resource limits
LimitNOFILE=65535
LimitNPROC=4096

# Environment
Environment="LOG_LEVEL=info"
EnvironmentFile=-/etc/goleapai/environment

[Install]
WantedBy=multi-user.target
```

### Setup Steps

```bash
# Create user
sudo useradd -r -s /bin/false -d /opt/goleapai goleapai

# Create directories
sudo mkdir -p /opt/goleapai/{bin,configs,data}
sudo chown -R goleapai:goleapai /opt/goleapai

# Copy binary
sudo cp bin/goleapai /opt/goleapai/bin/
sudo chmod +x /opt/goleapai/bin/goleapai

# Copy config
sudo cp configs/production.yaml /opt/goleapai/configs/
sudo chown goleapai:goleapai /opt/goleapai/configs/production.yaml

# Create environment file
sudo cat > /etc/goleapai/environment <<EOF
DATABASE_TYPE=postgres
DATABASE_CONNECTION=postgres://goleapai:password@localhost:5432/goleapai
REDIS_HOST=localhost:6379
LOG_LEVEL=info
EOF

# Install service
sudo systemctl daemon-reload
sudo systemctl enable goleapai
sudo systemctl start goleapai

# Check status
sudo systemctl status goleapai

# View logs
sudo journalctl -u goleapai -f
```

### Service Management

```bash
# Start service
sudo systemctl start goleapai

# Stop service
sudo systemctl stop goleapai

# Restart service
sudo systemctl restart goleapai

# Reload configuration
sudo systemctl reload goleapai

# Enable on boot
sudo systemctl enable goleapai

# Disable on boot
sudo systemctl disable goleapai

# Check status
sudo systemctl status goleapai

# View logs (last 100 lines)
sudo journalctl -u goleapai -n 100

# Follow logs
sudo journalctl -u goleapai -f

# View logs for today
sudo journalctl -u goleapai --since today
```

## Kubernetes Deployment

### Deployment YAML

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goleapai
  namespace: goleapai
  labels:
    app: goleapai
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  selector:
    matchLabels:
      app: goleapai
  template:
    metadata:
      labels:
        app: goleapai
    spec:
      containers:
      - name: goleapai
        image: goleapai/goleapai:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: DATABASE_TYPE
          value: "postgres"
        - name: DATABASE_CONNECTION
          valueFrom:
            secretKeyRef:
              name: goleapai-secrets
              key: database-url
        - name: REDIS_HOST
          value: "redis-service:6379"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: config
          mountPath: /app/configs
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: goleapai-config

---
apiVersion: v1
kind: Service
metadata:
  name: goleapai-service
  namespace: goleapai
spec:
  type: LoadBalancer
  selector:
    app: goleapai
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: goleapai-config
  namespace: goleapai
data:
  config.yaml: |
    server:
      port: 8080
      host: "0.0.0.0"
    routing:
      strategy: "cost_optimized"
      failover_enabled: true
    monitoring:
      prometheus:
        enabled: true
        port: 9090

---
apiVersion: v1
kind: Secret
metadata:
  name: goleapai-secrets
  namespace: goleapai
type: Opaque
stringData:
  database-url: "postgres://user:password@postgres-service:5432/goleapai"
```

### HorizontalPodAutoscaler

Create `k8s/hpa.yaml`:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: goleapai-hpa
  namespace: goleapai
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: goleapai
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Ingress

Create `k8s/ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: goleapai-ingress
  namespace: goleapai
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.goleapai.io
    secretName: goleapai-tls
  rules:
  - host: api.goleapai.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: goleapai-service
            port:
              number: 80
```

### Deploy to Kubernetes

```bash
# Create namespace
kubectl create namespace goleapai

# Apply configurations
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/ingress.yaml

# Check deployment
kubectl -n goleapai get deployments
kubectl -n goleapai get pods
kubectl -n goleapai get services

# View logs
kubectl -n goleapai logs -l app=goleapai -f

# Scale manually
kubectl -n goleapai scale deployment goleapai --replicas=5

# Update deployment
kubectl -n goleapai set image deployment/goleapai goleapai=goleapai/goleapai:v1.1.0

# Rollback
kubectl -n goleapai rollout undo deployment/goleapai
```

## Reverse Proxy Setup

### Nginx

Create `/etc/nginx/sites-available/goleapai`:

```nginx
# Rate limiting
limit_req_zone $binary_remote_addr zone=goleapai_limit:10m rate=100r/s;

# Upstream
upstream goleapai_backend {
    least_conn;
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    # Add more backends for load balancing
    # server 127.0.0.1:8081 max_fails=3 fail_timeout=30s;
    # server 127.0.0.1:8082 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 80;
    listen [::]:80;
    server_name api.goleapai.io;

    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name api.goleapai.io;

    # SSL certificates
    ssl_certificate /etc/letsencrypt/live/api.goleapai.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.goleapai.io/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logging
    access_log /var/log/nginx/goleapai_access.log;
    error_log /var/log/nginx/goleapai_error.log;

    # Rate limiting
    limit_req zone=goleapai_limit burst=20 nodelay;

    # Client settings
    client_max_body_size 10M;
    client_body_timeout 60s;

    location / {
        proxy_pass http://goleapai_backend;
        proxy_http_version 1.1;

        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        # Buffering
        proxy_buffering off;
    }

    # Metrics endpoint (restrict access)
    location /metrics {
        allow 10.0.0.0/8;  # Internal network
        deny all;
        proxy_pass http://goleapai_backend;
    }

    # Health check
    location /health {
        access_log off;
        proxy_pass http://goleapai_backend;
    }
}
```

Enable and restart:

```bash
# Enable site
sudo ln -s /etc/nginx/sites-available/goleapai /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload
sudo systemctl reload nginx

# Get SSL certificate
sudo certbot --nginx -d api.goleapai.io
```

### Caddy

Create `Caddyfile`:

```caddy
api.goleapai.io {
    # Automatic HTTPS

    # Rate limiting
    rate_limit {
        zone dynamic {
            key {remote_host}
            events 100
            window 1s
        }
    }

    # Reverse proxy
    reverse_proxy localhost:8080 {
        # Health check
        health_uri /health
        health_interval 10s
        health_timeout 5s

        # Load balancing
        lb_policy least_conn

        # Headers
        header_up Host {host}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }

    # Logging
    log {
        output file /var/log/caddy/goleapai.log
        format json
    }

    # Security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        X-XSS-Protection "1; mode=block"
    }
}
```

Run Caddy:

```bash
# Start Caddy
caddy run --config Caddyfile

# Or as service
sudo systemctl enable caddy
sudo systemctl start caddy
```

## Environment Variables

### Core Variables

```bash
# Server
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_HTTP3=true

# Database
DATABASE_TYPE=postgres  # or sqlite
DATABASE_CONNECTION=postgres://user:pass@host:5432/db
DATABASE_MAX_CONNS=25
DATABASE_LOG_LEVEL=warn

# Redis
REDIS_HOST=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Providers
PROVIDERS_AUTO_DISCOVERY=true
PROVIDERS_HEALTH_CHECK_INTERVAL=5m
PROVIDERS_DEFAULT_TIMEOUT=30s

# Routing
ROUTING_STRATEGY=cost_optimized  # latency_first, quality_first
ROUTING_FAILOVER_ENABLED=true
ROUTING_MAX_RETRIES=3

# Monitoring
MONITORING_PROMETHEUS_ENABLED=true
MONITORING_PROMETHEUS_PORT=9090
MONITORING_LOG_LEVEL=info
MONITORING_LOG_FORMAT=json

# TLS (optional)
TLS_ENABLED=false
TLS_CERT_PATH=/path/to/cert.pem
TLS_KEY_PATH=/path/to/key.pem
```

### Provider API Keys

```bash
# Tier 1 providers
GROQ_API_KEY=gsk_...
OPENROUTER_API_KEY=sk-or-v1-...
CEREBRAS_API_KEY=csk_...
GOOGLE_API_KEY=AIza...
GITHUB_TOKEN=ghp_...
MISTRAL_API_KEY=...
COHERE_API_KEY=...
CLOUDFLARE_API_TOKEN=...
CLOUDFLARE_ACCOUNT_ID=...
```

### Example .env File

Create `.env`:

```bash
# Server
SERVER_PORT=8080
SERVER_HOST=0.0.0.0

# Database
DATABASE_TYPE=postgres
DATABASE_CONNECTION=postgres://goleapai:secretpassword@localhost:5432/goleapai

# Redis
REDIS_HOST=localhost:6379

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Provider API Keys
GROQ_API_KEY=gsk_your_key_here
OPENROUTER_API_KEY=sk-or-v1-your_key_here
GOOGLE_API_KEY=AIza_your_key_here

# Routing
ROUTING_STRATEGY=cost_optimized
```

Load environment:

```bash
# Export all variables
export $(cat .env | xargs)

# Or use with systemd
EnvironmentFile=/opt/goleapai/.env
```

## Production Checklist

### Security

- [ ] Enable HTTPS/TLS
- [ ] Use strong database passwords
- [ ] Encrypt provider API keys in database
- [ ] Configure firewall rules
- [ ] Enable rate limiting
- [ ] Set up fail2ban for SSH
- [ ] Disable debug logging
- [ ] Use secrets management (Vault, AWS Secrets Manager)
- [ ] Regular security updates

### Performance

- [ ] Use PostgreSQL instead of SQLite
- [ ] Enable Redis caching
- [ ] Configure connection pooling
- [ ] Optimize database indices
- [ ] Enable HTTP/2 or HTTP/3
- [ ] Use CDN for static assets
- [ ] Configure proper timeouts
- [ ] Enable compression

### Reliability

- [ ] Set up health checks
- [ ] Configure auto-restart (systemd/Docker)
- [ ] Enable failover routing
- [ ] Multiple provider accounts
- [ ] Database backups
- [ ] Log rotation
- [ ] Monitoring alerts
- [ ] Resource limits (ulimit, cgroups)

### Monitoring

- [ ] Prometheus metrics enabled
- [ ] Grafana dashboards configured
- [ ] Log aggregation (ELK, Loki)
- [ ] Alert manager configured
- [ ] Uptime monitoring (UptimeRobot)
- [ ] APM tool integrated (optional)

### Operations

- [ ] Documented deployment process
- [ ] CI/CD pipeline configured
- [ ] Automated backups
- [ ] Disaster recovery plan
- [ ] Capacity planning
- [ ] Update strategy
- [ ] Rollback procedure

## Scaling

### Horizontal Scaling

Run multiple instances behind load balancer:

```bash
# Start multiple instances
./goleapai serve --port 8080 &
./goleapai serve --port 8081 &
./goleapai serve --port 8082 &

# Or with Docker Compose
docker-compose up --scale goleapai=3
```

Nginx load balancing:

```nginx
upstream goleapai_cluster {
    least_conn;
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
}
```

### Vertical Scaling

Increase resources per instance:

```yaml
# Docker Compose
services:
  goleapai:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
```

### Database Scaling

**Read Replicas:**
```yaml
database:
  primary: postgres://primary:5432/goleapai
  replicas:
    - postgres://replica1:5432/goleapai
    - postgres://replica2:5432/goleapai
```

**Connection Pooling:**
```yaml
database:
  max_conns: 100
  max_idle_conns: 25
  conn_max_lifetime: 1h
```

## Monitoring

### Prometheus Configuration

`prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'goleapai'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9090']
```

### Grafana Dashboard

Import dashboard from `configs/grafana-dashboard.json` or create custom:

**Key Metrics:**
- Total requests
- Success rate
- Average latency
- Provider health scores
- Cost saved
- Error rate by provider

### Alerting

`alertmanager.yml`:

```yaml
route:
  receiver: 'slack'

receivers:
  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#alerts'
```

## Backup & Recovery

### Database Backup

```bash
# PostgreSQL backup
pg_dump -U goleapai -d goleapai > backup_$(date +%Y%m%d).sql

# Automated daily backups
cat > /etc/cron.daily/goleapai-backup <<EOF
#!/bin/bash
pg_dump -U goleapai -d goleapai | gzip > /backups/goleapai_$(date +%Y%m%d).sql.gz
# Keep only last 7 days
find /backups -name "goleapai_*.sql.gz" -mtime +7 -delete
EOF
chmod +x /etc/cron.daily/goleapai-backup
```

### Restore

```bash
# Restore PostgreSQL
psql -U goleapai -d goleapai < backup_20260205.sql

# Or from compressed
gunzip -c backup_20260205.sql.gz | psql -U goleapai -d goleapai
```

### Configuration Backup

```bash
# Backup configs
tar -czf goleapai-config-$(date +%Y%m%d).tar.gz \
  /opt/goleapai/configs \
  /etc/goleapai \
  /etc/systemd/system/goleapai.service
```

## Conclusion

GoLeapAI is production-ready with multiple deployment options. Choose the approach that best fits your infrastructure and scale requirements. Remember to follow the production checklist for a secure, reliable deployment.

For support, visit https://github.com/biodoia/goleapifree/issues
