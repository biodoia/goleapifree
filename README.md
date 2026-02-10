# 🚀 GoLeapAI Free - LLM Gateway Unificato

> *Il gateway AI definitivo che aggrega TUTTE le API gratuite del mondo*

```
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   ▓▓▓   GoLeapAI - Gateway per Democratizzare l'AI           ║
║   ▓▓▓                                                         ║
║   ░░░   150+ API gratuite | Multi-provider | Auto-discovery  ║
║   ░░░   OpenAI Compatible | Anthropic Support | Local Models ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
```

## 🎯 Caratteristiche Principali

### 🌐 Multi-Provider Support
- **OpenAI Compatible** - Endpoint standard OpenAI
- **Anthropic Compatible** - Claude API support
- **150+ Free APIs** - Database pre-popolato di API gratuite
- **Local Models** - Ollama, llama.cpp con GPU support
- **Auto-Discovery** - Scansione automatica nuove API

### 🧠 Intelligent Routing
- **Context-Aware** - Routing basato su contesto (code, creative, analysis)
- **Multi-Agent** - Orchestrazione agenti specializzati
- **Auto-Failover** - Cambio automatico su quota exhausted
- **Load Balancing** - Distribuzione carico ottimale
- **Cost Optimizer** - Selezione provider per costo/qualità/latenza

### 📊 Monitoring & Analytics
- **Real-time Stats** - Dashboard statistiche in tempo reale
- **Health Monitoring** - Controllo salute provider ogni 5 minuti
- **Quota Tracking** - Monitoraggio quote per account
- **Prometheus Metrics** - Esportazione metriche
- **Cost Savings** - Calcolo risparmi vs API ufficiali

### 🎨 Dual Frontend

#### TUI (Terminal UI)
- **FrameGoTUI** - Framework cyberpunk-themed
- **Bubble Tea** - Architettura Elm
- **Live Dashboard** - Statistiche real-time
- **Log Streaming** - Visualizzazione log in diretta
- **Auto-Configuration** - Setup automatico CLI tools

#### Web UI
- **HTMX** - Interattività senza JavaScript pesante
- **HTTP/3** - Protocollo QUIC ad alte prestazioni
- **Code Page 437** - Estetica retro terminal
- **Templ** - Type-safe templating in Go
- **Fiber/Echo** - Web server ultrarapido

## 🏗️ Architettura

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI/TUI (Bubble Tea)                      │
│              Web UI (HTMX + Code Page 437)                   │
├─────────────────────────────────────────────────────────────┤
│                    API Gateway Layer                         │
│  ┌──────────┬──────────┬──────────┬──────────────────────┐  │
│  │ OpenAI   │Anthropic │  Google  │   Multi-Provider     │  │
│  │ Compat   │  Compat  │  Vertex  │   Router             │  │
│  └──────────┴──────────┴──────────┴──────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│              Intelligent Routing Engine                      │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ • Context-Aware Agent Orchestration                   │   │
│  │ • Load Balancing & Failover                          │   │
│  │ • Quota Monitoring & Auto-Switch                     │   │
│  │ • Cost/Latency/Quality Optimizer                     │   │
│  └──────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│                Provider Management Layer                     │
│  ┌────────────┬─────────────┬──────────────────────────┐   │
│  │ Free APIs  │  Paid APIs  │  Local Models (Ollama)   │   │
│  │ Database   │  Database   │  llama.cpp / GPU         │   │
│  └────────────┴─────────────┴──────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│              Storage & Caching Layer                         │
│  ┌──────────┬──────────┬──────────┬──────────────────┐     │
│  │ SQLite/  │  Redis   │ Vector   │  Prometheus      │     │
│  │PostgreSQL│  Cache   │ ChromemGo│  Metrics         │     │
│  └──────────┴──────────┴──────────┴──────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree

# Build backend
go build -o bin/goleapai cmd/backend/main.go

# Build TUI
go build -o bin/goleapai-tui cmd/tui/main.go

# Build WebUI
go build -o bin/goleapai-web cmd/webui/main.go
```

### Run Backend

```bash
# Start backend gateway
./bin/goleapai serve --port 8080

# With config file
./bin/goleapai serve --config configs/production.yaml

# Development mode with hot reload
air -c .air.toml
```

### Run TUI

```bash
# Launch TUI dashboard
./bin/goleapai-tui

# Or via main binary
./bin/goleapai tui
```

### Run Web UI

```bash
# Start web interface on port 3000
./bin/goleapai-web --port 3000 --http3

# With TLS for HTTP/3
./bin/goleapai-web --port 443 --tls-cert cert.pem --tls-key key.pem
```

## 📖 Usage Examples

### OpenAI Compatible Endpoint

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Anthropic Compatible Endpoint

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Using Go Client

```go
package main

import (
    "github.com/biodoia/goleapifree/pkg/client"
)

func main() {
    client := client.New("http://localhost:8080")

    resp, err := client.Chat(ctx, &client.ChatRequest{
        Model: "gpt-4",
        Messages: []client.Message{
            {Role: "user", Content: "Explain quantum computing"},
        },
    })

    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

## 🗂️ Project Structure

```
goleapifree/
├── cmd/
│   ├── backend/         # Backend gateway server
│   ├── tui/             # TUI application
│   └── webui/           # Web UI server
├── internal/
│   ├── gateway/         # Core gateway logic
│   ├── providers/       # Provider implementations
│   ├── router/          # Intelligent routing
│   ├── health/          # Health monitoring
│   ├── quota/           # Quota management
│   ├── stats/           # Statistics collection
│   ├── auth/            # Authentication
│   └── discovery/       # Auto-discovery
├── pkg/
│   ├── models/          # Data models
│   ├── database/        # Database layer
│   ├── cache/           # Caching layer
│   ├── llm/             # LLM client interfaces
│   ├── config/          # Configuration
│   └── middleware/      # HTTP middleware
├── web/
│   ├── templates/       # Templ templates
│   └── static/          # CSS, JS, fonts (CP437)
├── configs/             # Configuration files
├── scripts/             # Build & deployment scripts
└── docs/                # Documentation
```

## 🛠️ Configuration

### Backend Config (`configs/backend.yaml`)

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  http3: true
  tls:
    enabled: true
    cert: "certs/server.crt"
    key: "certs/server.key"

database:
  type: "postgres"  # or "sqlite"
  connection: "postgres://user:pass@localhost:5432/goleapai"

redis:
  host: "localhost:6379"
  password: ""
  db: 0

providers:
  auto_discovery: true
  health_check_interval: "5m"
  default_timeout: "30s"

routing:
  strategy: "cost_optimized"  # or "latency_first", "quality_first"
  failover_enabled: true
  max_retries: 3

monitoring:
  prometheus:
    enabled: true
    port: 9090
  logging:
    level: "info"
    format: "json"
```

## 📚 Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Provider Guide](docs/PROVIDERS.md)
- [TUI Guide](docs/TUI.md)
- [Web UI Guide](docs/WEBUI.md)
- [Deployment](docs/DEPLOYMENT.md)

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md)

## 📄 License

MIT License - See [LICENSE](LICENSE)

## 🙏 Credits

Built with:
- [FrameGoTUI](https://github.com/biodoia/framegotui) - Cyberpunk TUI framework
- [Charm](https://charm.sh) - TUI ecosystem
- [Fiber](https://gofiber.io) - Web framework
- [GORM](https://gorm.io) - ORM

---

<div align="center">

**Made with 💜 for the AI community**

*Democratizing access to AI, one free API at a time*

</div>
