# Web UI Guide

Complete guide for the GoLeapAI Web Interface.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Getting Started](#getting-started)
- [Interface Description](#interface-description)
- [Configuration](#configuration)
- [Customization](#customization)
- [Themes](#themes)
- [Advanced Features](#advanced-features)

## Overview

The GoLeapAI Web UI provides a browser-based interface for managing and monitoring your LLM gateway. Built with modern web technologies including HTMX for dynamic interactions and inspired by retro terminal aesthetics.

### Technology Stack

- **HTMX** - Dynamic HTML without heavy JavaScript
- **Templ** - Type-safe Go templates
- **HTTP/3 (QUIC)** - Next-generation protocol
- **Code Page 437** - Classic terminal fonts
- **Fiber v3** - Ultra-fast web framework

### Design Philosophy

- **Minimal JavaScript** - HTMX for interactivity
- **Fast & Lightweight** - Server-side rendering
- **Retro Aesthetics** - Terminal-inspired UI
- **Responsive** - Works on all devices

## Features

### Dashboard

Real-time overview of your gateway:

```
╔═══════════════════════════════════════════════════════════════╗
║                  GoLeapAI Gateway Dashboard                   ║
╚═══════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────┐
│ SYSTEM STATUS                                                │
├─────────────────────────────────────────────────────────────┤
│ Status:       ● ONLINE                                       │
│ Uptime:       2d 15h 34m                                     │
│ Version:      v1.0.0                                         │
│ Providers:    12 active, 1 down                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STATISTICS (Last 24h)                                        │
├─────────────────────────────────────────────────────────────┤
│ Total Requests:     15,234                                   │
│ Success Rate:       98.5%                                    │
│ Avg Latency:        165ms                                    │
│ Total Tokens:       1,850,000                                │
│ Cost Saved:         $185.50                                  │
└─────────────────────────────────────────────────────────────┘
```

**Features:**
- Real-time metrics updates
- Interactive charts
- Provider health status
- Recent activity log

### Provider Management

```
╔═══════════════════════════════════════════════════════════════╗
║                      Provider Management                       ║
╚═══════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────┐
│ PROVIDERS                                         [+ Add New] │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ ● Groq                                     [Edit]     │   │
│ │ Status: Active | Tier: 1 | Health: 95%               │   │
│ │ Requests: 5,234 | Latency: 145ms                     │   │
│ │ [View Details] [Test Connection] [Disable]           │   │
│ └──────────────────────────────────────────────────────┘   │
│                                                              │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ ● OpenRouter Free                          [Edit]     │   │
│ │ Status: Active | Tier: 1 | Health: 92%               │   │
│ │ Requests: 4,123 | Latency: 180ms                     │   │
│ │ [View Details] [Test Connection] [Disable]           │   │
│ └──────────────────────────────────────────────────────┘   │
│                                                              │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ ⚠ Google AI Studio                         [Edit]     │   │
│ │ Status: Active | Tier: 1 | Health: 65% (SLOW)        │   │
│ │ Requests: 2,876 | Latency: 320ms                     │   │
│ │ [View Details] [Test Connection] [Disable]           │   │
│ └──────────────────────────────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Features:**
- Add/edit/delete providers
- Test connections
- View detailed statistics
- Configure API keys
- Set provider tiers
- Enable/disable providers

### Model Catalog

```
╔═══════════════════════════════════════════════════════════════╗
║                         Model Catalog                          ║
╚═══════════════════════════════════════════════════════════════╝

Filters: [All] [Chat] [Completion] [Embedding] [Multimodal]
Providers: [All] [Free Only] [Tier 1] [Tier 2]

┌─────────────────────────────────────────────────────────────┐
│ MODEL                 PROVIDER        CONTEXT    PRICE       │
├─────────────────────────────────────────────────────────────┤
│ gpt-4                 OpenRouter      8K         FREE        │
│ claude-3-5-sonnet     OpenRouter      200K       FREE        │
│ llama-3-70b           Groq            8K         FREE        │
│ gemini-pro            Google AI       1M         FREE        │
│ mixtral-8x7b          Groq            32K        FREE        │
│ llama-3-8b            Cerebras        8K         FREE        │
└─────────────────────────────────────────────────────────────┘

[Load More]
```

**Features:**
- Browse available models
- Filter by type/provider
- View model specifications
- Test models directly
- Compare pricing

### Analytics Dashboard

```
╔═══════════════════════════════════════════════════════════════╗
║                          Analytics                             ║
╚═══════════════════════════════════════════════════════════════╝

Time Range: [Last Hour] [Today] [Week] [Month] [Custom]

┌─────────────────────────────────────────────────────────────┐
│ REQUESTS OVER TIME                                           │
│                                                              │
│  1000 ┤                                              ╭─╮     │
│   800 ┤                                    ╭─╮       │ │     │
│   600 ┤                          ╭─╮       │ │   ╭─╮ │ │     │
│   400 ┤                ╭─╮       │ │   ╭─╮ │ │   │ │ │ │     │
│   200 ┤    ╭─╮     ╭─╮ │ │   ╭─╮ │ │   │ │ │ │   │ │ │ │     │
│     0 ┴────┴─┴─────┴─┴─┴─┴───┴─┴─┴─┴───┴─┴─┴─┴───┴─┴─┴─┴     │
│       00:00    06:00    12:00    18:00    24:00              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ TOP PROVIDERS                                                │
│                                                              │
│ Groq              ████████████████████░ 45%  (5,234 req)    │
│ OpenRouter        ███████████████░░░░░ 32%  (3,712 req)     │
│ Cerebras          ██████░░░░░░░░░░░░░░ 15%  (1,743 req)     │
│ Google AI Studio  ███░░░░░░░░░░░░░░░░░  8%  (   928 req)    │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ LATENCY DISTRIBUTION                                         │
│                                                              │
│ < 100ms   ████████████░░░░░░░░░░ 45%                        │
│ 100-200ms ████████████████████░░ 35%                        │
│ 200-500ms ████████░░░░░░░░░░░░░ 15%                         │
│ > 500ms   ██░░░░░░░░░░░░░░░░░░░  5%                         │
└─────────────────────────────────────────────────────────────┘
```

**Features:**
- Request volume charts
- Provider usage breakdown
- Latency distribution
- Error rate tracking
- Cost savings calculator
- Export data as CSV/JSON

### Settings Panel

```
╔═══════════════════════════════════════════════════════════════╗
║                           Settings                             ║
╚═══════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────┐
│ ROUTING CONFIGURATION                                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Strategy:                                                    │
│ ○ Cost Optimized     ● Latency First     ○ Quality First    │
│                                                              │
│ Failover Settings:                                           │
│ [✓] Enable automatic failover                               │
│ Max retries: [3]                                             │
│ Retry delay: [1000] ms                                       │
│                                                              │
│ [Save Changes]                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ MONITORING                                                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Health Check Interval: [5] minutes                           │
│ Log Level: [Info ▼]                                          │
│ [✓] Enable Prometheus metrics                               │
│ Metrics port: [9090]                                         │
│                                                              │
│ [Save Changes]                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ API KEYS                                                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Your API Keys:                                               │
│                                                              │
│ gla_1234...abcd   Production      Last used: 2m ago  [⊗]    │
│ gla_5678...efgh   Development     Last used: 1h ago  [⊗]    │
│                                                              │
│ [+ Generate New Key]                                         │
└─────────────────────────────────────────────────────────────┘
```

**Features:**
- Configure routing strategy
- Adjust health check intervals
- Manage API keys
- Set rate limits
- Configure logging
- Database settings

### API Playground

```
╔═══════════════════════════════════════════════════════════════╗
║                        API Playground                          ║
╚═══════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────┐
│ REQUEST                                                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Endpoint: [POST /v1/chat/completions ▼]                     │
│                                                              │
│ Model: [gpt-4 ▼]                                             │
│                                                              │
│ Message:                                                     │
│ ┌────────────────────────────────────────────────────────┐ │
│ │ Explain quantum computing in simple terms.             │ │
│ │                                                        │ │
│ │                                                        │ │
│ └────────────────────────────────────────────────────────┘ │
│                                                              │
│ Temperature: [0.7] ─────────────────────o                   │
│ Max Tokens:  [1000]                                          │
│                                                              │
│ [✓] Streaming                                                │
│                                                              │
│ [Send Request]                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ RESPONSE                                                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Status: 200 OK                                               │
│ Provider: groq                                               │
│ Latency: 245ms                                               │
│                                                              │
│ ┌────────────────────────────────────────────────────────┐ │
│ │ Quantum computing is a revolutionary approach to       │ │
│ │ computation that leverages quantum mechanics...        │ │
│ │                                                        │ │
│ └────────────────────────────────────────────────────────┘ │
│                                                              │
│ Tokens: 150 (input: 25, output: 125)                        │
│ Cost: FREE (saved: $0.0035)                                  │
│                                                              │
│ [Copy Response] [View Raw JSON]                              │
└─────────────────────────────────────────────────────────────┘
```

**Features:**
- Test API endpoints
- Interactive request builder
- Real-time response display
- View raw JSON
- Copy curl commands
- Save request templates

## Getting Started

### Installation

```bash
# Clone repository
git clone https://github.com/biodoia/goleapifree.git
cd goleapifree

# Build Web UI
go build -o bin/goleapai-web cmd/webui/main.go

# Run
./bin/goleapai-web --port 3000
```

### Access Web UI

Open browser and navigate to:
```
http://localhost:3000
```

Default credentials (change on first login):
```
Username: admin
Password: admin
```

### Quick Setup

1. **Configure Providers**
   - Navigate to Providers page
   - Click "Add New Provider"
   - Enter provider details and API key
   - Test connection
   - Save

2. **Set Routing Strategy**
   - Go to Settings
   - Choose routing strategy
   - Configure failover settings
   - Save changes

3. **Start Using**
   - Use API Playground to test
   - Monitor dashboard for activity
   - View analytics for insights

## Interface Description

### Navigation Bar

```
┌─────────────────────────────────────────────────────────────┐
│ GoLeapAI                                        Admin ▼ [☰] │
├─────────────────────────────────────────────────────────────┤
│ [Dashboard] [Providers] [Models] [Analytics] [Settings]     │
└─────────────────────────────────────────────────────────────┘
```

- **Dashboard** - Overview and statistics
- **Providers** - Manage API providers
- **Models** - Browse available models
- **Analytics** - Detailed metrics and charts
- **Settings** - Configuration options
- **Admin Menu** - User settings, logout

### Status Indicators

- **● Green** - Healthy, operational
- **⚠ Yellow** - Warning, degraded performance
- **✕ Red** - Down, unavailable
- **○ Gray** - Disabled

### Interactive Elements

All interactive elements use HTMX for seamless updates:

- **Buttons** - Trigger actions without page reload
- **Forms** - Submit with live validation
- **Charts** - Auto-refresh with new data
- **Tables** - Sortable, filterable

## Configuration

### Web UI Config

Edit `configs/webui.yaml`:

```yaml
webui:
  port: 3000
  host: "0.0.0.0"

  # TLS
  tls:
    enabled: true
    cert: "certs/webui.crt"
    key: "certs/webui.key"

  # Session
  session:
    secret: "change-this-secret-key"
    max_age: 86400  # 24 hours

  # Authentication
  auth:
    enabled: true
    require_login: true
    default_user: "admin"
    default_password: "admin"  # Change on first login!

  # UI Settings
  ui:
    theme: "cyberpunk"
    refresh_interval: 5000  # ms
    items_per_page: 25
    date_format: "2006-01-02 15:04:05"

  # Features
  features:
    playground: true
    analytics: true
    admin_panel: true
```

### Environment Variables

```bash
# Web UI
WEBUI_PORT=3000
WEBUI_HOST=0.0.0.0
WEBUI_TLS_ENABLED=true

# Session
SESSION_SECRET=your-secret-key
SESSION_MAX_AGE=86400

# Auth
AUTH_ENABLED=true
DEFAULT_USER=admin
DEFAULT_PASSWORD=changeme

# Backend API
BACKEND_URL=http://localhost:8080
BACKEND_API_KEY=your-backend-api-key
```

### Start with Custom Config

```bash
./bin/goleapai-web \
  --config configs/webui.yaml \
  --port 3000 \
  --tls-cert certs/webui.crt \
  --tls-key certs/webui.key
```

## Customization

### Themes

GoLeapAI Web UI supports multiple themes:

#### Cyberpunk (Default)

Classic terminal aesthetic with neon accents:
- Colors: Cyan, magenta, green
- Font: Code Page 437
- Style: Retro terminal

#### Matrix

Green on black:
- Colors: Various shades of green
- Font: Monospace
- Style: Matrix-inspired

#### Hacker

Dark with syntax highlighting:
- Colors: Orange, blue, white
- Font: Fira Code
- Style: Modern hacker aesthetic

#### Custom Theme

Create `web/static/css/theme-custom.css`:

```css
:root {
  --primary-color: #00ff00;
  --secondary-color: #0000ff;
  --background-color: #000000;
  --text-color: #ffffff;
  --border-color: #00ff00;
  --error-color: #ff0000;
  --success-color: #00ff00;
  --warning-color: #ffff00;
}

.dashboard {
  background: var(--background-color);
  color: var(--text-color);
  font-family: 'Courier New', monospace;
}

.panel {
  border: 2px solid var(--border-color);
  box-shadow: 0 0 10px var(--primary-color);
}
```

Load theme in `configs/webui.yaml`:

```yaml
ui:
  theme: "custom"
  custom_css: "web/static/css/theme-custom.css"
```

### Custom Logo

Replace logo file:

```bash
cp your-logo.png web/static/images/logo.png
```

Or configure in settings:

```yaml
ui:
  logo: "/static/images/custom-logo.png"
  logo_alt: "Your Company"
```

### Custom Fonts

Add font files to `web/static/fonts/`:

```css
@font-face {
  font-family: 'CustomFont';
  src: url('/static/fonts/CustomFont.woff2') format('woff2');
}

body {
  font-family: 'CustomFont', monospace;
}
```

### Dashboard Widgets

Customize dashboard widgets in `configs/webui.yaml`:

```yaml
dashboard:
  widgets:
    - type: "system_status"
      position: 1
      enabled: true

    - type: "statistics"
      position: 2
      enabled: true
      config:
        period: "24h"
        metrics:
          - "total_requests"
          - "success_rate"
          - "avg_latency"
          - "cost_saved"

    - type: "provider_health"
      position: 3
      enabled: true
      config:
        show_inactive: false
        sort_by: "health_score"

    - type: "recent_logs"
      position: 4
      enabled: true
      config:
        limit: 10
        level: "info"
```

## Advanced Features

### Real-Time Updates

Web UI uses Server-Sent Events (SSE) for real-time updates:

```javascript
// Automatic connection
const eventSource = new EventSource('/api/stream/events');

eventSource.addEventListener('stats', (event) => {
  const data = JSON.parse(event.data);
  updateDashboard(data);
});

eventSource.addEventListener('provider_status', (event) => {
  const data = JSON.parse(event.data);
  updateProviderStatus(data);
});
```

Configure update frequency:

```yaml
ui:
  refresh_interval: 5000  # 5 seconds
  enable_realtime: true
```

### Custom Alerts

Configure desktop notifications:

```yaml
alerts:
  enabled: true
  desktop_notifications: true

  rules:
    - name: "Provider Down"
      condition: "provider.status == 'down'"
      severity: "critical"
      notification: true

    - name: "High Latency"
      condition: "provider.avg_latency_ms > 1000"
      severity: "warning"
      notification: true

    - name: "Quota Warning"
      condition: "quota_used / quota_limit > 0.9"
      severity: "warning"
      notification: true
```

### Export & Reports

Generate reports:

```bash
# Via UI
Dashboard → Analytics → Export → [CSV/JSON/PDF]

# Via API
curl http://localhost:3000/api/reports/generate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "type": "daily",
    "format": "pdf",
    "date": "2026-02-05"
  }'
```

### Keyboard Shortcuts

- `Ctrl+/` - Show shortcuts help
- `Ctrl+K` - Quick search
- `Ctrl+D` - Go to Dashboard
- `Ctrl+P` - Go to Providers
- `Ctrl+M` - Go to Models
- `Ctrl+A` - Go to Analytics
- `Ctrl+S` - Go to Settings
- `Esc` - Close modals

### Mobile Support

Web UI is fully responsive:

- **Desktop** - Full feature set
- **Tablet** - Optimized layout
- **Mobile** - Essential features, touch-friendly

### API Access

Web UI provides REST API for automation:

```bash
# Get dashboard stats
curl http://localhost:3000/api/dashboard/stats \
  -H "Authorization: Bearer $TOKEN"

# Add provider
curl -X POST http://localhost:3000/api/providers \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Custom Provider",
    "base_url": "https://api.example.com",
    "api_key": "key123"
  }'

# Update settings
curl -X PUT http://localhost:3000/api/settings \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "routing": {
      "strategy": "latency_first"
    }
  }'
```

## Troubleshooting

### Web UI won't start

```bash
# Check port availability
lsof -i :3000

# Check logs
./bin/goleapai-web --log-level debug

# Verify backend connection
curl http://localhost:8080/health
```

### Can't login

```bash
# Reset admin password
./bin/goleapai-web reset-password \
  --user admin \
  --password newpassword
```

### Charts not updating

- Check browser console for errors
- Verify SSE connection in Network tab
- Check `refresh_interval` setting
- Ensure backend is responding

### Slow performance

- Enable caching
- Reduce refresh interval
- Disable unnecessary widgets
- Check backend response times

## Conclusion

The GoLeapAI Web UI provides a powerful, user-friendly interface for managing your LLM gateway. With real-time monitoring, comprehensive analytics, and extensive customization options, you have complete control over your AI infrastructure.

For issues or feature requests, visit: https://github.com/biodoia/goleapifree/issues
