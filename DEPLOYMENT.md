# GoLeapAI - Deployment Guide

Guida completa al deployment di GoLeapAI in ambiente di produzione.

## Indice

- [Requisiti](#requisiti)
- [Deployment con Docker](#deployment-con-docker)
- [Deployment Nativo](#deployment-nativo)
- [Configurazione Nginx](#configurazione-nginx)
- [Monitoring](#monitoring)
- [Backup e Manutenzione](#backup-e-manutenzione)

## Requisiti

### Software Richiesto

- Go 1.25+ (per build nativa)
- Docker & Docker Compose (per deployment Docker)
- PostgreSQL 16+ (opzionale, usa SQLite di default)
- Redis 7+ (opzionale, per caching distribuito)
- Nginx (per reverse proxy in produzione)

### Porte Necessarie

- `8080` - API Gateway
- `9090` - Prometheus Metrics
- `5432` - PostgreSQL (opzionale)
- `6379` - Redis (opzionale)
- `3000` - Grafana (opzionale)

## Deployment con Docker

### Quick Start

```bash
# 1. Clone repository
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree

# 2. Build e avvio
make docker

# 3. Verifica status
docker-compose ps

# 4. Test health check
curl http://localhost:8080/health
```

### Comandi Docker

```bash
# Build immagine
make docker-build

# Avvia servizi
make docker-up

# Stop servizi
make docker-down

# Visualizza logs
make docker-logs

# Riavvia tutto
docker-compose restart
```

### Configurazione Docker

Modifica `docker-compose.yml` per personalizzare:

- Porte esposte
- Variabili d'ambiente
- Volumi per persistenza
- Risorse (CPU/RAM limits)

## Deployment Nativo

### Build e Installazione

```bash
# 1. Build binary
make build-linux

# 2. Installa come servizio systemd
sudo make install
sudo make install-systemd

# 3. Avvia servizio
sudo systemctl start goleapai
sudo systemctl enable goleapai

# 4. Verifica status
sudo systemctl status goleapai
```

### Script di Deployment

```bash
# Deployment completo (build + test + deploy)
./scripts/deploy.sh deploy

# Solo build
./scripts/deploy.sh build

# Health check
./scripts/deploy.sh health
```

### Installazione Manuale

```bash
# 1. Crea utente sistema
sudo useradd --system --no-create-home goleapai

# 2. Crea directories
sudo mkdir -p /opt/goleapai/{bin,configs,data,logs}
sudo chown -R goleapai:goleapai /opt/goleapai

# 3. Copia binary e config
sudo cp bin/goleapai /opt/goleapai/bin/
sudo cp configs/config.yaml /opt/goleapai/configs/

# 4. Installa servizio systemd
sudo cp systemd/goleapai.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable goleapai
sudo systemctl start goleapai
```

## Configurazione Nginx

### Installazione Reverse Proxy

```bash
# 1. Installa Nginx
sudo apt install nginx  # Debian/Ubuntu
sudo yum install nginx  # RHEL/CentOS

# 2. Copia configurazione
sudo cp nginx/goleapai.conf /etc/nginx/sites-available/
sudo ln -s /etc/nginx/sites-available/goleapai.conf /etc/nginx/sites-enabled/

# 3. Test configurazione
sudo nginx -t

# 4. Riavvia Nginx
sudo systemctl restart nginx
```

### SSL/TLS con Let's Encrypt

```bash
# 1. Installa Certbot
sudo apt install certbot python3-certbot-nginx

# 2. Ottieni certificato
sudo certbot --nginx -d goleapai.example.com

# 3. Auto-renewal
sudo certbot renew --dry-run
```

### Configurazione Custom

Modifica `/etc/nginx/sites-available/goleapai.conf`:

- `server_name`: Il tuo dominio
- `ssl_certificate`: Path ai certificati SSL
- Rate limiting zones
- Timeout e buffer sizes

## Monitoring

### Prometheus

Accedi a: `http://localhost:9091`

Queries utili:
```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Response time p95
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### Grafana

Accedi a: `http://localhost:3000`

- Username: `admin`
- Password: `goleapai`

Dashboard pre-configurata disponibile in:
`monitoring/grafana/dashboards/goleapai-dashboard.json`

### Logs

```bash
# Logs systemd
sudo journalctl -u goleapai -f

# Logs Docker
docker-compose logs -f goleapai

# Logs applicazione
tail -f /opt/goleapai/logs/goleapai.log
```

## Database

### PostgreSQL Setup

```bash
# 1. Crea database
sudo -u postgres psql
CREATE DATABASE goleapai;
CREATE USER goleapai WITH PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE goleapai TO goleapai;
\q

# 2. Configura connection string
DATABASE_CONNECTION=postgresql://goleapai:secure_password@localhost:5432/goleapai
```

### SQLite (Default)

```bash
# Database file location
./data/goleapai.db

# Backup
cp ./data/goleapai.db ./backups/goleapai-$(date +%Y%m%d).db
```

## Backup e Manutenzione

### Backup Database

```bash
# PostgreSQL
pg_dump -U goleapai goleapai > backup-$(date +%Y%m%d).sql

# SQLite
sqlite3 ./data/goleapai.db ".backup './backups/backup-$(date +%Y%m%d).db'"
```

### Update Application

```bash
# 1. Pull nuove modifiche
git pull origin main

# 2. Build nuova versione
make build

# 3. Test
make test

# 4. Riavvia servizio
sudo systemctl restart goleapai
```

### Rotazione Logs

Crea `/etc/logrotate.d/goleapai`:

```
/opt/goleapai/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 goleapai goleapai
    sharedscripts
    postrotate
        systemctl reload goleapai > /dev/null 2>&1 || true
    endscript
}
```

## Troubleshooting

### Service Non Parte

```bash
# Check logs
sudo journalctl -u goleapai -n 50

# Check permissions
ls -la /opt/goleapai

# Test binary manualmente
sudo -u goleapai /opt/goleapai/bin/goleapai --config /opt/goleapai/configs/config.yaml
```

### High Memory Usage

```bash
# Check metrics
curl http://localhost:9090/metrics | grep go_memstats

# Restart service
sudo systemctl restart goleapai
```

### Database Connection Issues

```bash
# Test PostgreSQL connection
psql -U goleapai -h localhost -d goleapai

# Check connection string
grep DATABASE_CONNECTION /etc/goleapai/environment
```

## Performance Tuning

### System Limits

Aggiungi a `/etc/security/limits.conf`:

```
goleapai soft nofile 65536
goleapai hard nofile 65536
goleapai soft nproc 4096
goleapai hard nproc 4096
```

### Database Connection Pool

Modifica `configs/config.yaml`:

```yaml
database:
  max_conns: 50  # Aumenta per alto traffico
  max_idle_conns: 10
  conn_max_lifetime: 3600s
```

### Redis Configuration

```bash
# Modifica /etc/redis/redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
```

## Security Checklist

- [ ] Firewall configurato (ufw/iptables)
- [ ] SSL/TLS abilitato
- [ ] Rate limiting attivo
- [ ] Logs monitoring attivo
- [ ] Database password sicure
- [ ] JWT secret personalizzato
- [ ] Backup automatici configurati
- [ ] Update regolari del sistema
- [ ] Nginx security headers attivi

## Supporto

Per problemi o domande:

- GitHub Issues: https://github.com/biodoia/goleapifree/issues
- Documentation: https://github.com/biodoia/goleapifree/docs

## License

MIT License - See LICENSE file
