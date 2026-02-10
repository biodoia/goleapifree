# GoLeapAI TUI - Cyberpunk Dashboard

Terminal User Interface avanzata per GoLeapAI Gateway, costruita con [FrameGoTUI](https://github.com/biodoia/framegotui).

## Features

### 🎨 Tema Cyberpunk
- Colori neon (pink/cyan)
- Bordi e decorazioni cyberpunk
- Animazioni e effetti visivi
- Stile glitch e neon

### 📊 Dashboard View (Shortcut: 1)
- **Real-time Statistics**: Visualizzazione live delle metriche principali
- **System Health**: Stato operativo di tutti i provider
- **Activity Chart**: Grafico ASCII delle richieste nelle ultime 24h
- **Recent Logs**: Stream in tempo reale degli ultimi eventi

Componenti utilizzati:
- `Box` per contenitori con bordi personalizzati
- `Progress` per indicatori di utilizzo
- ASCII charts per visualizzazione trend

### 🔌 Providers View (Shortcut: 2)
- **Provider List**: Tabella interattiva con tutti i provider
- **Filtri**: Per status (active/inactive) e tier
- **Provider Details**: Vista dettagliata con:
  - Informazioni base (nome, tipo, URL)
  - Metriche di salute (health score, latency)
  - Capabilities (streaming, tools, JSON mode)
  - Lista modelli disponibili
- **Actions**: Test connection, enable/disable, edit

Componenti utilizzati:
- `Table` per lista provider con sorting e selezione
- `Modal` per conferme e dettagli

### 📈 Statistics View (Shortcut: 3)
- **Overview**: Metriche aggregate per periodo selezionabile
  - Total requests
  - Success rate
  - Average latency
  - Cost saved
- **Visual Charts**:
  - Request volume (bar chart ASCII)
  - Success rate trend (line chart ASCII)
- **Cost Savings Calculator**:
  - Confronto con costi OpenAI
  - Proiezioni mensili
  - Risparmio percentuale
- **Request Distribution**: Per provider con progress bars

Componenti utilizzati:
- `Box` per sezioni organizzate
- ASCII charts (bar e line) per visualizzazioni

### 📝 Logs View (Shortcut: 4)
- **Live Log Streaming**: Tail -f style con auto-refresh
- **Color Coding**: Per severity e status
  - ✓ Success (verde)
  - ⚠ Warning (giallo)
  - ✗ Error (rosso)
- **Filters**:
  - By level (success/error/all)
  - By provider
  - Search text
- **Auto-scroll**: Attiva/disattiva seguire automaticamente i nuovi log
- **Detailed Info**:
  - Timestamp con millisecondi
  - HTTP status code
  - Latency con color coding
  - Tokens (input/output)
  - Error messages

Componenti utilizzati:
- Scroll buffer per log illimitati
- Color styling per categorizzazione

## Shortcuts

### Navigation
- `1-4`: Switch tra viste (Dashboard/Providers/Stats/Logs)
- `↑↓` o `k/j`: Navigate su/giù
- `←→` o `h/l`: Navigate sinistra/destra
- `Enter`: Select/Confirm
- `ESC`: Back/Cancel

### Global
- `?`: Help modal
- `q` o `Ctrl+C`: Quit
- `r`: Refresh current view

### View-Specific

**Providers View:**
- `f`: Cycle filter (all → active → inactive)
- `t`: Test connection for selected provider
- `e`: Edit provider
- `d`: Disable/Enable provider

**Stats View:**
- `1`: Last 1 hour
- `2`: Last 24 hours
- `3`: Last 7 days
- `4`: Last 30 days

**Logs View:**
- `Home/End`: Jump to top/bottom
- `PgUp/PgDown`: Scroll page
- `a`: Toggle auto-scroll
- `f`: Cycle filter (all → success → error)
- `c`: Clear logs

## Usage

### Build
```bash
go build -o bin/goleapai-tui ./cmd/tui/
```

### Run
```bash
# With default config
./bin/goleapai-tui

# With custom config
./bin/goleapai-tui --config /path/to/config.yaml
```

### Development
```bash
# Run directly
go run ./cmd/tui/main.go

# With config
go run ./cmd/tui/main.go -c config.yaml
```

## Configuration

La TUI utilizza la stessa configurazione del backend GoLeapAI:

```yaml
database:
  type: sqlite
  connection: ./data/goleapai.db
  max_conns: 25

server:
  port: 8080
  host: 0.0.0.0

monitoring:
  logging:
    level: info
    format: json
```

## Architecture

```
cmd/tui/
├── main.go              # Entry point, main model
└── views/
    ├── types.go         # Shared types (TickMsg)
    ├── dashboard.go     # Dashboard view
    ├── providers.go     # Providers management
    ├── stats.go         # Statistics & analytics
    └── logs.go          # Live logs viewer
```

### Main Model
Il `MainModel` gestisce:
- Routing tra viste
- Window sizing
- Header/Footer rendering
- Modal overlay
- Global shortcuts

### View Pattern
Ogni view implementa:
```go
type View struct {
    db     *gorm.DB
    width  int
    height int
    // ... view-specific state
}

func (v *View) Init() tea.Cmd
func (v *View) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (v *View) View() string
func (v *View) SetSize(width, height int)
```

## FrameGoTUI Components Used

### Box
Container con bordi personalizzabili:
```go
box := components.NewBox(
    components.WithTitle("TITLE"),
    components.WithBorderColor(lipgloss.Color("#FF10F0")),
    components.WithPadding(1, 2, 1, 2),
)
```

### Table
Tabella interattiva con sorting:
```go
table := components.NewTable(
    columns,
    components.WithRows(rows),
    components.WithTableHeight(20),
    components.WithStriped(true),
)
```

### Progress
Progress bar con gradient:
```go
progress := components.NewProgressBar(
    components.WithProgress(0.75),
    components.WithProgressWidth(40),
    components.WithGradient(true),
)
```

### Modal
Dialog modale per conferme:
```go
modal := components.NewModal(
    "Confirm Action",
    "Are you sure?",
    components.ModalTypeConfirm,
)
modal.Show()
```

### Spinner
Loading indicator animato:
```go
spinner := components.NewSpinner(
    components.WithSpinnerFrames(components.SpinnerNeon),
    components.WithSpinnerMessage("Loading..."),
)
```

## Styling

### Color Palette (Cyberpunk)
```go
NeonPink    = "#FF10F0"  // Primary accent
NeonCyan    = "#00FFFF"  // Secondary accent
NeonYellow  = "#FFFF00"  // Warnings
NeonGreen   = "#00FF00"  // Success
NeonRed     = "#FF0000"  // Errors
DarkNavy    = "#0A0E27"  // Background
SurfaceBlue = "#1A1A2E"  // Surface
```

### Style Examples
```go
titleStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#00FFFF")).
    Bold(true).
    MarginBottom(1)

successStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#00FF00"))

errorStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FF0000")).
    Bold(true)
```

## Performance

- **Auto-refresh**: 2 secondi per dashboard e logs
- **Lazy loading**: Solo la vista attiva viene aggiornata
- **Database pooling**: Max 25 connessioni
- **Efficient rendering**: Solo aree modificate vengono ri-renderizzate

## Screenshots

```
╔══════════════════════════════════════════════════════════════╗
║  ▓▓▓ GoLeapAI Gateway - Cyberpunk Dashboard ▓▓▓             ║
╚══════════════════════════════════════════════════════════════╝

┌─────────────────────────────────┬────────────────────────────┐
│ SYSTEM STATISTICS               │ TOP PROVIDERS              │
│                                 │                            │
│ Total Providers:     12         │ ● Groq      ████████░ 92%  │
│ Active Providers:    10         │ ● OpenRouter ████████░ 85% │
│ Total Requests:   1,234         │ ● Cerebras   ███████░░ 78% │
│ Success Rate:      98.5%        │                            │
│ Avg Latency:       145ms        │                            │
│ Cost Saved:      $12.45         │                            │
└─────────────────────────────────┴────────────────────────────┘

[1] Dashboard [2] Providers [3] Stats [4] Logs [?] Help [q] Quit
```

## Troubleshooting

### Database Connection Issues
```bash
# Check database exists
ls -la ./data/goleapai.db

# Run migrations
./bin/goleapai-backend migrate
```

### Display Issues
```bash
# Ensure terminal supports ANSI colors
echo $TERM

# Should be: xterm-256color or similar
```

### Performance Issues
```bash
# Reduce auto-refresh interval in code
# Or filter logs to reduce data volume
```

## Future Enhancements

- [ ] Real-time WebSocket updates
- [ ] Interactive charts con mouse support
- [ ] Export statistics to CSV/JSON
- [ ] Provider configuration editor
- [ ] Alert configuration interface
- [ ] Custom dashboard layouts
- [ ] Themes switcher (Cyberpunk/Matrix/Neon)
- [ ] Multi-pane layout support

## Contributing

Contributi benvenuti! Per aggiungere nuove view:

1. Creare file in `cmd/tui/views/`
2. Implementare interface `tea.Model`
3. Aggiungere shortcut in `main.go`
4. Aggiornare questo README

## License

Stesso della repo principale GoLeapAI.
