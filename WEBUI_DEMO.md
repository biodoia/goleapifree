# GoLeapAI Web UI Demo - Code Page 437 Edition

## Overview

Web UI retrò con estetica terminale IBM PC anni '80, completamente funzionale con HTMX e WebSocket.

## Quick Start

```bash
# Installa dipendenze
make install-deps

# Genera template e avvia Web UI
make webui

# Oppure usa lo script
./scripts/run-webui.sh
```

Apri browser: http://localhost:8080

## Aspetto Visivo

### Tema: Green Phosphor Terminal
```
╔═══════════════════════════════════════════════════════════════════╗
║                     █▀▀ █▀█ █   █▀▀ ▄▀█ █▀█ ▄▀█ █                ║
║                     █▄█ █▄█ █▄▄ ██▄ █▀█ █▀▀ █▀█ █                ║
║                  FREE LLM GATEWAY - CP437 EDITION                 ║
╚═══════════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────────┐
│ [F1] Dashboard │ [F2] Providers │ [F3] Stats │ [F4] Logs        │
└─────────────────────────────────────────────────────────────────┘
```

### Colori CGA
- Background: Verde scuro fosforescente (#001100)
- Testo: Verde chiaro (#33FF33)
- Accenti: Palette CGA a 16 colori
- Effetti: Scanlines, phosphor glow, CRT flicker

## Sezioni

### 1. Providers Panel
Visualizza tutti i provider LLM disponibili:

```
╔═══════════════════════════════════════════════════════════════════╗
║                          PROVIDERS                                 ║
╚═══════════════════════════════════════════════════════════════════╝

STATUS  | PROVIDER      | TYPE | HEALTH | LATENCY | ACTIONS
───────────────────────────────────────────────────────────────────
█ ACTIVE | DeepSeek      | FREE | 95%    | 120ms   | [TOGGLE][TEST]
█ ACTIVE | HuggingChat   | FREE | 87%    | 250ms   | [TOGGLE][TEST]
█ DOWN   | OpenRouter    | FREM | --     | --      | [TOGGLE][TEST]
```

Funzionalità:
- Toggle stato provider (ACTIVE/DOWN)
- Test health check
- Visualizzazione real-time stato
- Auto-refresh ogni 5s

### 2. Stats Panel
Statistiche aggregate sistema:

```
╔═══════════════════════════════════════════════════════════════════╗
║                        STATISTICS                                  ║
╚═══════════════════════════════════════════════════════════════════╝

TOTAL REQUESTS        SUCCESS RATE
15.2K                 98.7%
████████████████░░░░  █████████████████░░░

AVG LATENCY           COST SAVED
150ms                 $245.50
██████████░░░░░░░░░░  ███████████████████░

PROVIDERS STATUS
█ Active: 12  █ Down: 2  █ Warning: 1

REQUEST TIMELINE (LAST HOUR)
20 │                    █
15 │          █         █
10 │    █     █    █    █
 5 │    █     █    █    █  █
 0 │────┴─────┴────┴────┴──┴─────
    00   10   20   30   40  50 min
```

Auto-refresh: 2s (WebSocket)

### 3. Logs Panel
Visualizzazione log richieste recenti:

```
╔═══════════════════════════════════════════════════════════════════╗
║                        RECENT LOGS                                 ║
╚═══════════════════════════════════════════════════════════════════╝

TIME     | STATUS | PROVIDER    | ENDPOINT       | LATENCY | TOKENS
────────────────────────────────────────────────────────────────────
14:23:45 | 200    | abc123...   | /v1/chat       | 120ms   | 50/200
14:23:42 | 200    | def456...   | /v1/chat       | 98ms    | 30/150
14:23:40 | 500    | ghi789...   | /v1/chat       | 5000ms  | 0/0
14:23:38 | 200    | abc123...   | /v1/models     | 45ms    | 0/0
```

Indicatori:
- ✓ OK - Richiesta riuscita (verde)
- ✗ ERR - Richiesta fallita (rosso)
- Colori status code:
  - 2xx: Verde
  - 4xx: Giallo
  - 5xx: Rosso

Auto-refresh: 3s

### 4. Footer
Status bar bottom:

```
┌─────────────────────────────────────────────────────────────────┐
│ Status: █ ONLINE │ Uptime: 24h 15m │ Requests/s: 42             │
└─────────────────────────────────────────────────────────────────┘
```

## Funzionalità HTMX

### Auto-Refresh
Ogni sezione si auto-aggiorna:
- Providers: 5s
- Stats: 2s (WebSocket)
- Logs: 3s

### Interattività
- Toggle provider senza reload
- Test health check inline
- Notifiche terminal-style
- Smooth updates

### WebSocket
Live updates per stats panel:
```javascript
ws://localhost:8080/ws
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| F1  | Dashboard (home) |
| F2  | Focus Providers |
| F3  | Focus Stats |
| F4  | Focus Logs |
| F5  | Refresh |
| F10 | Exit |
| ESC | Cancel/Clear |

## Effetti CRT

### Scanlines
Linee orizzontali animate che scorrono lentamente:
```css
background: repeating-linear-gradient(
    0deg,
    rgba(0, 0, 0, 0.15),
    rgba(0, 0, 0, 0.15) 1px,
    transparent 1px,
    transparent 2px
);
```

### Phosphor Glow
Bagliore radiale centrale:
```css
background: radial-gradient(
    ellipse at center,
    rgba(0, 255, 0, 0.1) 0%,
    transparent 60%
);
```

### Text Shadow
Glow sui testi importanti:
```css
text-shadow:
    0 0 5px var(--phosphor-glow),
    0 0 10px var(--phosphor-glow);
```

### Flicker
Sfarfallio sottile del monitor:
```css
animation: flicker 0.15s infinite;
```

## Personalizzazione

### Cambiare Tema
Modifica `web/static/css/cp437.css`:

**Amber Theme:**
```css
--phosphor-green: #FFBB00;
--phosphor-dim: #CC8800;
--phosphor-glow: #FFCC00;
--screen-bg: #110800;
--screen-text: #FFBB00;
```

**Blue Theme:**
```css
--phosphor-green: #00AAFF;
--phosphor-dim: #0066AA;
--phosphor-glow: #00CCFF;
--screen-bg: #000811;
--screen-text: #00AAFF;
```

### Aggiungere Pannelli

1. Crea template in `web/templates/`:
```go
// web/templates/custom.templ
package templates

templ CustomPanel() {
    <div class="panel">
        <div class="panel-header">
            ╔═══════════════════════════════════════╗
            ║         CUSTOM PANEL                  ║
            ╚═══════════════════════════════════════╝
        </div>
        <div class="panel-content">
            // Your content
        </div>
    </div>
}
```

2. Aggiungi endpoint in `cmd/webui/main.go`:
```go
s.app.Get("/htmx/custom", s.handleCustomPanel)

func (s *WebUIServer) handleCustomPanel(c *fiber.Ctx) error {
    component := templates.CustomPanel()
    return renderTempl(c, component)
}
```

3. Aggiungi al dashboard in `web/templates/dashboard.templ`:
```html
<section class="panel" id="custom-panel">
    <div class="panel-content"
         hx-get="/htmx/custom"
         hx-trigger="load"
         hx-swap="innerHTML">
        <div class="loading">█ LOADING...</div>
    </div>
</section>
```

## Troubleshooting

### Templates non si aggiornano
```bash
make generate
# oppure
templ generate
```

### Port già in uso
```bash
# Usa porta diversa
go run cmd/webui/main.go --port 8081
```

### WebSocket non connette
Verifica firewall e che il server supporti upgrade WebSocket.

### Font non carica
Il font IBM VGA richiede download manuale.
Fallback su Courier New funziona comunque.

## Browser Support

- ✅ Chrome 90+
- ✅ Firefox 88+
- ✅ Safari 14+
- ✅ Edge 90+
- ✅ Brave, Vivaldi, Arc

Richiede:
- WebSocket support
- CSS Grid
- CSS Variables
- CSS Animations

## Performance

- Lightweight: ~50KB HTML + 15KB CSS + 5KB JS
- No framework pesanti
- HTMX: 14KB (minified)
- WebSocket per live updates
- Render lato server con Templ

## Next Steps

1. Configura database (SQLite o Postgres)
2. Aggiungi provider in seed data
3. Avvia backend gateway
4. Apri Web UI
5. Monitora dashboard real-time

## Links

- Backend: http://localhost:8080 (gateway API)
- Web UI: http://localhost:8080 (web interface)
- Prometheus: http://localhost:9090 (metrics)
- WebSocket: ws://localhost:8080/ws

## Contributing

Per aggiungere funzionalità:
1. Crea template in `web/templates/`
2. Genera con `make generate`
3. Aggiungi endpoint in `cmd/webui/main.go`
4. Testa e commit

## License

MIT - Vedi LICENSE file
