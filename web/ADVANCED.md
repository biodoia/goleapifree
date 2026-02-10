# GoLeapAI Web UI - Advanced Usage

## Custom Themes

### Creating a Custom Theme

1. Create theme CSS file `web/static/css/themes/custom.css`:

```css
/* Custom Theme - Cyberpunk */
:root {
    --phosphor-green: #FF00FF;
    --phosphor-dim: #AA00AA;
    --phosphor-glow: #FF00FF;
    --screen-bg: #110011;
    --screen-text: #FF00FF;

    /* Custom colors */
    --accent-1: #00FFFF;
    --accent-2: #FFFF00;
}
```

2. Update layout.templ to include theme switcher:

```go
templ Layout(title string) {
    <!DOCTYPE html>
    <html lang="en">
        <head>
            <meta charset="UTF-8"/>
            <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
            <title>{ title } - GoLeapAI CP437</title>
            <link rel="stylesheet" href="/static/css/cp437.css" id="theme-css"/>
            <script src="https://unpkg.com/htmx.org@1.9.10"></script>
            <script src="/static/js/htmx-config.js"></script>
        </head>
        <body>
            <!-- Theme Switcher -->
            <div class="theme-switcher">
                <button onclick="switchTheme('green')">Green</button>
                <button onclick="switchTheme('amber')">Amber</button>
                <button onclick="switchTheme('blue')">Blue</button>
            </div>
            <div class="screen">
                <div class="scanlines"></div>
                <div class="phosphor-glow"></div>
                <div class="container">
                    { children... }
                </div>
            </div>
        </body>
    </html>
}
```

3. Add theme switcher JS in `htmx-config.js`:

```javascript
function switchTheme(theme) {
    const themes = {
        green: {
            '--phosphor-green': '#33FF33',
            '--phosphor-dim': '#00AA00',
            '--phosphor-glow': '#00FF00',
            '--screen-bg': '#001100',
            '--screen-text': '#33FF33'
        },
        amber: {
            '--phosphor-green': '#FFBB00',
            '--phosphor-dim': '#CC8800',
            '--phosphor-glow': '#FFCC00',
            '--screen-bg': '#110800',
            '--screen-text': '#FFBB00'
        },
        blue: {
            '--phosphor-green': '#00AAFF',
            '--phosphor-dim': '#0066AA',
            '--phosphor-glow': '#00CCFF',
            '--screen-bg': '#000811',
            '--screen-text': '#00AAFF'
        }
    };

    const root = document.documentElement;
    Object.entries(themes[theme]).forEach(([key, value]) => {
        root.style.setProperty(key, value);
    });

    localStorage.setItem('theme', theme);
}

// Load saved theme
document.addEventListener('DOMContentLoaded', function() {
    const savedTheme = localStorage.getItem('theme') || 'green';
    switchTheme(savedTheme);
});
```

## Real-time Charts

### Adding ASCII Charts with Live Data

1. Create chart template `web/templates/charts.templ`:

```go
package templates

import "github.com/biodoia/goleapifree/pkg/models"

templ LiveChart(data []int) {
    <div class="live-chart">
        <pre class="ascii-chart">
            @renderASCIIChart(data)
        </pre>
    </div>
}

func renderASCIIChart(data []int) string {
    // Find max value for scaling
    max := 0
    for _, v := range data {
        if v > max {
            max = v
        }
    }

    // Build chart
    height := 20
    chart := ""

    for y := height; y >= 0; y-- {
        line := fmt.Sprintf("%3d │ ", (y * max / height))

        for _, v := range data {
            scaledValue := (v * height) / max
            if scaledValue >= y {
                line += "█"
            } else {
                line += " "
            }
        }

        chart += line + "\n"
    }

    chart += "    └" + strings.Repeat("─", len(data)) + "\n"

    return chart
}
```

2. Add WebSocket handler for chart data:

```go
func (s *WebUIServer) handleChartWebSocket(c *websocket.Conn) {
    defer c.Close()

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Get latest data
            data := s.getChartData()

            component := templates.LiveChart(data)
            html, err := templ.ToGoHTML(context.Background(), component)
            if err != nil {
                continue
            }

            if err := c.WriteMessage(websocket.TextMessage, []byte(html)); err != nil {
                return
            }
        }
    }
}
```

## Advanced Filtering

### Provider Filter UI

1. Add filter template:

```go
templ ProvidersFilter() {
    <div class="filters">
        <select id="type-filter" hx-get="/htmx/providers"
                hx-trigger="change" hx-include="[id='status-filter']"
                hx-target="#providers-list">
            <option value="">All Types</option>
            <option value="free">Free</option>
            <option value="freemium">Freemium</option>
            <option value="paid">Paid</option>
            <option value="local">Local</option>
        </select>

        <select id="status-filter" hx-get="/htmx/providers"
                hx-trigger="change" hx-include="[id='type-filter']"
                hx-target="#providers-list">
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="down">Down</option>
            <option value="maintenance">Maintenance</option>
        </select>
    </div>

    <div id="providers-list">
        <!-- Providers list will be loaded here -->
    </div>
}
```

2. Update handler:

```go
func (s *WebUIServer) handleProvidersPartial(c *fiber.Ctx) error {
    typeFilter := c.Query("type-filter")
    statusFilter := c.Query("status-filter")

    providers, err := s.db.GetFilteredProviders(typeFilter, statusFilter)
    if err != nil {
        return c.Status(500).SendString("Error loading providers")
    }

    component := templates.ProvidersList(providers)
    return renderTempl(c, component)
}
```

## Notifications System

### Toast Notifications

1. Add notification component:

```go
templ Notification(message string, notifType string) {
    <div class={ "notification", "notification-" + notifType }
         hx-swap-oob="afterbegin:#notifications">
        ╔═══════════════════════════════════════╗
        ║ { message }                           ║
        ╚═══════════════════════════════════════╝
    </div>
}
```

2. Add notification container to layout:

```go
<div id="notifications" class="notifications-container"></div>
```

3. Trigger from server:

```go
func (s *WebUIServer) handleToggleProvider(c *fiber.Ctx) error {
    id := c.Params("id")

    provider, err := s.db.GetProviderByID(id)
    if err != nil {
        notif := templates.Notification("Provider not found", "error")
        return renderTempl(c, notif)
    }

    // Toggle status
    provider.Status = toggleStatus(provider.Status)

    if err := s.db.UpdateProvider(provider); err != nil {
        notif := templates.Notification("Failed to update provider", "error")
        return renderTempl(c, notif)
    }

    // Return both updated row and notification
    row := templates.ProviderRow(provider)
    notif := templates.Notification("Provider updated successfully", "success")

    return renderTempl(c, templ.Join(row, notif))
}
```

## Export Functionality

### Export Logs as CSV

1. Add export button:

```go
templ LogsViewer(logs []models.RequestLog) {
    <div class="logs-viewer">
        <button hx-get="/htmx/logs/export"
                hx-target="this"
                class="btn btn-export">
            EXPORT CSV
        </button>

        <table class="logs-table">
            <!-- ... -->
        </table>
    </div>
}
```

2. Add export handler:

```go
func (s *WebUIServer) handleLogsExport(c *fiber.Ctx) error {
    logs, err := s.db.GetRecentLogs(1000)
    if err != nil {
        return c.Status(500).SendString("Error loading logs")
    }

    // Generate CSV
    csv := "Time,Status,Provider,Endpoint,Latency,Tokens,Result\n"
    for _, log := range logs {
        csv += fmt.Sprintf("%s,%d,%s,%s,%d,%d/%d,%t\n",
            log.Timestamp.Format(time.RFC3339),
            log.StatusCode,
            log.ProviderID,
            log.Endpoint,
            log.LatencyMs,
            log.InputTokens,
            log.OutputTokens,
            log.Success,
        )
    }

    c.Set("Content-Type", "text/csv")
    c.Set("Content-Disposition", "attachment; filename=logs.csv")
    return c.SendString(csv)
}
```

## Search & Autocomplete

### Provider Search

1. Add search component:

```go
templ ProviderSearch() {
    <div class="search-box">
        <input type="text"
               id="provider-search"
               placeholder="Search providers..."
               hx-get="/htmx/providers/search"
               hx-trigger="keyup changed delay:500ms"
               hx-target="#providers-list"
               class="search-input"/>
    </div>
}
```

2. Add search handler:

```go
func (s *WebUIServer) handleProviderSearch(c *fiber.Ctx) error {
    query := c.Query("provider-search")

    providers, err := s.db.SearchProviders(query)
    if err != nil {
        return c.Status(500).SendString("Error searching providers")
    }

    component := templates.ProvidersList(providers)
    return renderTempl(c, component)
}
```

## Performance Monitoring

### Add Performance Metrics

```go
templ PerformancePanel() {
    <div class="panel">
        <div class="panel-header">
            ╔═══════════════════════════════════════╗
            ║         PERFORMANCE                   ║
            ╚═══════════════════════════════════════╝
        </div>
        <div class="panel-content"
             hx-get="/htmx/performance"
             hx-trigger="every 1s"
             hx-swap="innerHTML">
            <div class="loading">█ LOADING...</div>
        </div>
    </div>
}

templ PerformanceMetrics(metrics *PerformanceMetrics) {
    <div class="metrics-grid">
        <div class="metric">
            <div class="metric-label">CPU</div>
            <div class="metric-value">{ fmt.Sprintf("%.1f%%", metrics.CPU) }</div>
            @ProgressBar(metrics.CPU)
        </div>

        <div class="metric">
            <div class="metric-label">MEMORY</div>
            <div class="metric-value">{ fmt.Sprintf("%.1f%%", metrics.Memory) }</div>
            @ProgressBar(metrics.Memory)
        </div>

        <div class="metric">
            <div class="metric-label">GOROUTINES</div>
            <div class="metric-value">{ fmt.Sprintf("%d", metrics.Goroutines) }</div>
        </div>
    </div>
}

templ ProgressBar(percent float64) {
    <div class="progress-bar">
        <div class="progress-bar-fill"
             style={ fmt.Sprintf("width: %.0f%%", percent) }>
            { strings.Repeat("█", int(percent/5)) }
        </div>
    </div>
}
```

## Security

### Add API Key Management

1. API Key template:

```go
templ APIKeyManager() {
    <div class="panel">
        <div class="panel-header">
            ╔═══════════════════════════════════════╗
            ║         API KEYS                      ║
            ╚═══════════════════════════════════════╝
        </div>
        <div class="panel-content">
            <button hx-post="/htmx/apikey/create"
                    hx-target="#apikey-list"
                    class="btn">
                CREATE NEW KEY
            </button>

            <div id="apikey-list"
                 hx-get="/htmx/apikeys"
                 hx-trigger="load">
                <div class="loading">█ LOADING...</div>
            </div>
        </div>
    </div>
}
```

2. Rate limiting display:

```go
templ RateLimitStatus(limits []models.RateLimit) {
    <table class="ascii-table">
        <thead>
            <tr>
                <th>PROVIDER</th>
                <th>USED</th>
                <th>LIMIT</th>
                <th>RESET</th>
            </tr>
        </thead>
        <tbody>
            for _, limit := range limits {
                <tr>
                    <td>{ limit.Provider.Name }</td>
                    <td>{ fmt.Sprintf("%d", limit.UsedRequests) }</td>
                    <td>{ fmt.Sprintf("%d", limit.MaxRequests) }</td>
                    <td>{ limit.ResetAt.Format("15:04") }</td>
                </tr>
            }
        </tbody>
    </table>
}
```

## Development Tips

### Hot Reload

Install Air:
```bash
go install github.com/cosmtrek/air@latest
```

Create `.air.webui.toml`:
```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = ["--port", "8080"]
  bin = "./tmp/main"
  cmd = "templ generate && go build -o ./tmp/main ./cmd/webui"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go", "_templ.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = ["cmd", "internal", "pkg", "web"]
  include_ext = ["go", "templ", "css", "js"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

Run:
```bash
make dev-webui
```

### Debugging

Enable debug logging:
```go
log.Logger = log.Output(zerolog.ConsoleWriter{
    Out: os.Stderr,
    TimeFormat: time.RFC3339,
})
zerolog.SetGlobalLevel(zerolog.DebugLevel)
```

### Testing HTMX Endpoints

Use curl:
```bash
# Test provider toggle
curl -X POST http://localhost:8080/htmx/provider/abc123/toggle

# Test stats
curl http://localhost:8080/htmx/stats

# Test logs
curl http://localhost:8080/htmx/logs
```

## Production Deployment

### Nginx Configuration

```nginx
upstream goleapai_webui {
    server localhost:8080;
}

server {
    listen 80;
    server_name goleapai.example.com;

    location / {
        proxy_pass http://goleapai_webui;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location /static/ {
        alias /path/to/goleapifree/web/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### Systemd Service

Create `/etc/systemd/system/goleapai-webui.service`:

```ini
[Unit]
Description=GoLeapAI Web UI
After=network.target

[Service]
Type=simple
User=goleapai
WorkingDirectory=/opt/goleapai
ExecStart=/opt/goleapai/bin/goleapai-webui --config /etc/goleapai/webui.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable goleapai-webui
sudo systemctl start goleapai-webui
```
