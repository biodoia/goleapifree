# GoLeapAI - Quick Start Guide

Deploy GoLeapAI in 5 minuti!

## Metodo 1: Docker (Consigliato)

```bash
# Clone e avvia
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree
make docker

# Verifica
curl http://localhost:8080/health
```

Fatto! GoLeapAI è ora in esecuzione su `http://localhost:8080`

### Servizi Disponibili

- **API Gateway**: http://localhost:8080
- **Grafana**: http://localhost:3000 (admin/goleapai)
- **Prometheus**: http://localhost:9091

## Metodo 2: Build Nativo

```bash
# Prerequisiti: Go 1.25+
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree

# Build e Run
make build
make run
```

## Metodo 3: Deployment Completo

```bash
# Build, test e deploy
./scripts/deploy.sh deploy

# Oppure usando Makefile
make deploy
```

## Test Rapido

```bash
# Health check
curl http://localhost:8080/health

# List providers
curl http://localhost:8080/api/providers

# Test completions (esempio)
curl -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Configurazione Base

File: `configs/config.yaml`

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  type: "sqlite"
  connection: "./data/goleapai.db"

providers:
  auto_discovery: true
```

## Comandi Utili

```bash
# Build
make build              # Build binary
make build-linux        # Build per Linux

# Test
make test               # Run tests
make test-coverage      # Con coverage

# Docker
make docker-build       # Build immagine
make docker-up          # Avvia servizi
make docker-down        # Stop servizi
make docker-logs        # Visualizza logs

# Deployment
make deploy             # Deploy completo
make install            # Installa come servizio
```

## Monitoring

```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# Grafana
open http://localhost:3000
# Login: admin/goleapai
```

## Troubleshooting

### Service non parte

```bash
# Check logs
docker-compose logs goleapai

# Oppure se nativo
./bin/goleapai --log-level debug
```

### Porta già in uso

Modifica `configs/config.yaml`:
```yaml
server:
  port: 8081  # Cambia porta
```

### Database locked

```bash
# Stop e riavvia
make docker-down
make docker-up
```

## Prossimi Passi

1. Leggi la [documentazione completa](DEPLOYMENT.md)
2. Configura [Nginx reverse proxy](nginx/goleapai.conf)
3. Setup [monitoring](monitoring/)
4. Configura [backup automatici](scripts/backup.sh)

## Supporto

- GitHub: https://github.com/biodoia/goleapifree
- Issues: https://github.com/biodoia/goleapifree/issues
- Docs: https://github.com/biodoia/goleapifree/docs
