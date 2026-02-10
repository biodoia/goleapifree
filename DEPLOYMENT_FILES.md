# GoLeapAI - Deployment Files Overview

Elenco completo di tutti i file di deployment creati.

## Struttura File

```
goleapifree/
├── Dockerfile                          # Multi-stage build ottimizzato
├── .dockerignore                       # Esclusioni per Docker build
├── docker-compose.yml                  # Full stack production
├── docker-compose.dev.yml              # Stack per sviluppo
├── .env.example                        # Template variabili d'ambiente
├── Makefile                            # Build automation (aggiornato)
│
├── scripts/
│   ├── deploy.sh                       # Script deployment completo
│   ├── install.sh                      # Installazione systemd
│   ├── setup.sh                        # Setup ambiente sviluppo
│   ├── backup.sh                       # Backup database e config
│   ├── healthcheck.sh                  # Health check script
│   └── monitor.sh                      # Monitoring in tempo reale
│
├── systemd/
│   └── goleapai.service                # Systemd unit file
│
├── nginx/
│   └── goleapai.conf                   # Nginx reverse proxy config
│
├── monitoring/
│   ├── prometheus.yml                  # Prometheus config
│   ├── alerts.yml                      # Alert rules
│   └── grafana/
│       ├── datasources/
│       │   └── prometheus.yml          # Grafana datasource
│       └── dashboards/
│           ├── dashboard.yml           # Dashboard provisioning
│           └── goleapai-dashboard.json # Pre-built dashboard
│
├── k8s/
│   ├── deployment.yaml                 # K8s deployment principale
│   ├── postgres.yaml                   # PostgreSQL deployment
│   ├── redis.yaml                      # Redis deployment
│   └── README.md                       # K8s deployment guide
│
├── .github/
│   └── workflows/
│       └── deploy.yml                  # GitHub Actions CI/CD
│
├── DEPLOYMENT.md                       # Guida deployment completa
└── QUICKSTART.md                       # Quick start guide
```

## File Principali

### 1. Dockerfile
- Multi-stage build (builder + runtime)
- Immagine finale basata su Alpine (minimale)
- Health check integrato
- Ottimizzato per size (~20MB)
- CGO enabled per SQLite

### 2. docker-compose.yml
**Servizi inclusi:**
- GoLeapAI Gateway
- PostgreSQL 16
- Redis 7
- Prometheus
- Grafana

**Features:**
- Network isolation
- Volume persistence
- Health checks
- Auto-restart

### 3. docker-compose.dev.yml
Stack ridotto per sviluppo locale:
- Solo servizi esterni (Postgres, Redis, Prometheus, Grafana)
- Permette di eseguire GoLeapAI in locale con `go run`

### 4. Makefile
**Comandi disponibili:**
```bash
make help              # Mostra tutti i comandi
make build             # Build binary
make build-linux       # Build per Linux production
make test              # Run tests
make docker-build      # Build Docker image
make docker-up         # Start Docker services
make docker-down       # Stop Docker services
make docker            # Build + Up
make deploy            # Run deployment script
make install           # Install to system
```

## Script di Deployment

### deploy.sh
Script completo per deployment:
- Build binary
- Run tests
- Database migrations
- Start/stop services
- Health checks

**Comandi:**
```bash
./scripts/deploy.sh build
./scripts/deploy.sh test
./scripts/deploy.sh start
./scripts/deploy.sh stop
./scripts/deploy.sh deploy      # Full deployment
./scripts/deploy.sh docker-deploy
```

### install.sh
Installa GoLeapAI come servizio systemd:
- Crea utente di sistema
- Setup directories
- Installa binary e config
- Configura systemd service
- Enable e start service

**Utilizzo:**
```bash
sudo ./scripts/install.sh
```

### setup.sh
Setup ambiente di sviluppo:
- Verifica dipendenze
- Download Go modules
- Installa dev tools (templ, air, golangci-lint)
- Genera templates
- Avvia servizi Docker
- Run tests

**Utilizzo:**
```bash
./scripts/setup.sh
```

### backup.sh
Backup automatico:
- Database (SQLite o PostgreSQL)
- File di configurazione
- Retention policy (30 giorni default)

**Utilizzo:**
```bash
./scripts/backup.sh
# Oppure con custom retention
RETENTION_DAYS=7 ./scripts/backup.sh
```

### monitor.sh
Monitoring in tempo reale:
- Service health
- Memory usage
- HTTP requests
- Database connections
- Cache statistics
- Provider status

**Utilizzo:**
```bash
./scripts/monitor.sh
# Oppure con custom URL
API_URL=http://production:8080 ./scripts/monitor.sh
```

## Systemd Service

### goleapai.service
**Features:**
- Auto-restart on failure
- Resource limits
- Security hardening
- Journal logging
- Graceful shutdown

**Comandi:**
```bash
sudo systemctl start goleapai
sudo systemctl stop goleapai
sudo systemctl restart goleapai
sudo systemctl status goleapai
sudo journalctl -u goleapai -f
```

## Nginx Configuration

### goleapai.conf
**Features:**
- SSL/TLS termination
- Rate limiting
- Security headers
- Reverse proxy
- Static file serving
- Metrics endpoint (interno)

**Setup:**
```bash
sudo cp nginx/goleapai.conf /etc/nginx/sites-available/
sudo ln -s /etc/nginx/sites-available/goleapai.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## Monitoring Stack

### Prometheus
- Scrape metrics da GoLeapAI
- Alert rules configurate
- Retention: default

### Grafana
- Pre-configured datasource
- Dashboard GoLeapAI inclusa
- Login: admin/goleapai

### Metrics Disponibili
- HTTP request rate
- Response time (p50, p95, p99)
- Error rate
- Memory usage
- Database connections
- Cache hit rate
- Provider health

## Kubernetes Deployment

### deployment.yaml
**Include:**
- Namespace
- ConfigMap
- Secrets
- Deployment (3 replicas)
- Service (ClusterIP)
- Ingress
- HPA (3-10 replicas)

### postgres.yaml
- PVC (10Gi)
- Deployment
- Service

### redis.yaml
- PVC (5Gi)
- Deployment
- Service

**Deploy:**
```bash
kubectl apply -f k8s/
kubectl get all -n goleapai
```

## GitHub Actions CI/CD

### deploy.yml
**Jobs:**
- Build & Test
- Docker Build & Push
- Security Scan (Trivy)
- Deploy to Production

**Triggers:**
- Push to main/production
- Pull requests

## Environment Variables

Tutte le configurazioni possono essere sovrascritte via env vars:

```bash
SERVER_PORT=8080
DATABASE_TYPE=postgres
DATABASE_CONNECTION=postgresql://...
REDIS_HOST=localhost:6379
MONITORING_PROMETHEUS_ENABLED=true
```

Vedi `.env.example` per la lista completa.

## Quick Reference

### Development
```bash
./scripts/setup.sh              # Setup iniziale
make build                      # Build
make run                        # Run local
make test                       # Test
```

### Docker
```bash
make docker                     # Build + Run
make docker-logs                # View logs
make docker-down                # Stop
```

### Production (Native)
```bash
make build-linux                # Build for production
sudo ./scripts/install.sh       # Install systemd
sudo systemctl start goleapai   # Start
```

### Production (Docker)
```bash
docker-compose up -d            # Start all services
docker-compose logs -f goleapai # View logs
docker-compose ps               # Status
```

### Monitoring
```bash
./scripts/monitor.sh            # Live monitor
curl http://localhost:9090/metrics  # Raw metrics
# Grafana: http://localhost:3000
```

### Backup
```bash
./scripts/backup.sh             # Manual backup
# Setup cron per backup automatici
```

## Support & Documentation

- **Quick Start**: [QUICKSTART.md](QUICKSTART.md)
- **Full Deployment Guide**: [DEPLOYMENT.md](DEPLOYMENT.md)
- **Kubernetes Guide**: [k8s/README.md](k8s/README.md)
- **Main README**: [README.md](README.md)

## Checklist Deployment Production

- [ ] Build testato
- [ ] Test passati
- [ ] Docker image creata
- [ ] Secrets configurati
- [ ] Database setup
- [ ] Nginx configurato
- [ ] SSL/TLS attivo
- [ ] Monitoring attivo
- [ ] Backup configurati
- [ ] Health check funzionante
- [ ] Logs monitoring
- [ ] Alert configurati
- [ ] Firewall regole
- [ ] Resource limits
- [ ] Auto-scaling (K8s)

Tutti i file sono pronti per il deployment in produzione!
