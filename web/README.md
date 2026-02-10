# GoLeapAI Web UI - Code Page 437 Edition

Web UI retrò con estetica terminale anni '80 per GoLeapAI Gateway.

## Caratteristiche

- **Estetica CP437**: Terminal anni '80 con font IBM VGA
- **Colori CGA/EGA**: Palette a 16 colori classica
- **Effetti CRT**: Scanlines, phosphor glow, flicker
- **HTMX**: Aggiornamenti real-time senza JavaScript complesso
- **WebSocket**: Live updates per statistiche
- **Responsive**: Funziona su desktop e tablet

## Struttura

```
web/
├── templates/           # Templ templates
│   ├── layout.templ    # Layout base con CP437
│   ├── dashboard.templ # Dashboard principale
│   ├── providers.templ # Lista provider
│   ├── stats.templ     # Statistiche
│   └── logs.templ      # Log viewer
├── static/
│   ├── css/
│   │   └── cp437.css   # Stili CP437
│   └── js/
│       └── htmx-config.js # Configurazione HTMX
└── README.md
```

## Esecuzione

```bash
# Build templates
cd /home/lisergico25/projects/goleapifree
templ generate

# Run Web UI
go run cmd/webui/main.go --port 8080
```

## URL

- Dashboard: http://localhost:8080/
- Providers: http://localhost:8080/htmx/providers
- Stats: http://localhost:8080/htmx/stats
- Logs: http://localhost:8080/htmx/logs
- WebSocket: ws://localhost:8080/ws

## Keyboard Shortcuts

- **F1**: Dashboard
- **F2**: Focus Providers
- **F3**: Focus Stats
- **F4**: Focus Logs
- **F5**: Refresh
- **F10**: Exit
- **ESC**: Cancel/Clear

## Temi

### Green Phosphor (Default)
Terminal verde fosforescente classico.

### Amber (Alternativo)
Per attivare il tema amber, modifica in `cp437.css`:

```css
/* Phosphor Amber Theme */
--phosphor-green: #FFBB00;
--phosphor-dim: #CC8800;
--phosphor-glow: #FFCC00;
--screen-bg: #110800;
--screen-text: #FFBB00;
```

## Dipendenze

### Go
```bash
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/websocket/v2
go get github.com/a-h/templ
```

### JavaScript (CDN)
- HTMX 1.9.10: https://unpkg.com/htmx.org@1.9.10

## HTMX Endpoints

### GET /htmx/providers
Restituisce HTML parziale con lista provider.

### GET /htmx/stats
Restituisce HTML parziale con statistiche.

### GET /htmx/logs
Restituisce HTML parziale con log recenti.

### POST /htmx/provider/:id/toggle
Toggle stato provider (active/down).

### POST /htmx/provider/:id/test
Esegue health check su provider.

## WebSocket Protocol

Il WebSocket invia aggiornamenti HTML ogni 2 secondi:

```javascript
ws://localhost:8080/ws
```

Riceve HTML per aggiornare `#stats-panel .panel-content`.

## Estetica CP437

### Font
- IBM VGA (Code Page 437)
- Fallback: Perfect DOS VGA 437, Px437, Courier New

### Colori CGA
```
Black:        #000000
Blue:         #0000AA
Green:        #00AA00
Cyan:         #00AAAA
Red:          #AA0000
Magenta:      #AA00AA
Brown:        #AA5500
Light Gray:   #AAAAAA
Dark Gray:    #555555
Light Blue:   #5555FF
Light Green:  #55FF55
Light Cyan:   #55FFFF
Light Red:    #FF5555
Light Magenta:#FF55FF
Yellow:       #FFFF55
White:        #FFFFFF
```

### ASCII Box Drawing
```
╔═══╗  ┌───┐
║   ║  │   │
╚═══╝  └───┘
```

### Effetti CRT
- **Scanlines**: Linee orizzontali animate
- **Phosphor Glow**: Bagliore radiale centrale
- **Flicker**: Sfarfallio sottile (0.15s)
- **Text Shadow**: Glow su testo importante

## Personalizzazione

### Modificare refresh rate
In `dashboard.templ`:

```html
hx-trigger="load, every 5s"  <!-- Cambia 5s -->
```

### Aggiungere nuovi panel
1. Crea template in `templates/`
2. Aggiungi endpoint in `cmd/webui/main.go`
3. Aggiungi sezione in `dashboard.templ`

### Modificare colori
Modifica CSS variables in `cp437.css`:

```css
:root {
    --phosphor-green: #33FF33;
    --screen-bg: #001100;
    /* ... */
}
```

## Troubleshooting

### Templates non compilano
```bash
go install github.com/a-h/templ/cmd/templ@latest
templ generate
```

### WebSocket non connette
Verifica che il server supporti upgrade WebSocket:
```go
app.Get("/ws", websocket.New(handler))
```

### Font non carica
Il font IBM VGA richiede download manuale.
Fallback funziona automaticamente.

## TODO

- [ ] Grafici ASCII dinamici
- [ ] Export logs CSV
- [ ] Filtri provider avanzati
- [ ] Multi-theme switcher
- [ ] Dark/Light mode toggle
- [ ] Keyboard navigation completa
- [ ] Provider config editor
- [ ] Real-time alerts
