# Real-Time Dashboard API

Sistema completo di streaming in tempo reale basato su Server-Sent Events (SSE) per monitorare statistiche, log e stato dei provider.

## Architettura

### Componenti

1. **SSE Hub**: Gestisce le connessioni client e il broadcasting degli eventi
2. **Streamer**: Pubblica periodicamente aggiornamenti dai vari data source
3. **Aggregator**: Calcola metriche aggregate in tempo reale
4. **Handlers**: Espone gli endpoint SSE via HTTP

### Flusso Dati

```
Request → Collector → Aggregator → Streamer → SSE Hub → Clients
                                                    ↓
                                            Browser/TUI/CLI
```

## Endpoints SSE

### GET /stream/stats

Stream di statistiche aggregate in tempo reale.

**Eventi inviati:**
- Metriche globali (richieste totali, costi, token)
- Statistiche per provider
- Success rate e latenze medie
- Aggiornamenti ogni 2 secondi

**Esempio evento:**
```json
{
  "id": "1738723890-1",
  "type": "stats",
  "data": {
    "timestamp": 1738723890,
    "total_requests": 15234,
    "total_tokens": 2534567,
    "total_cost": 12.45,
    "providers": [
      {
        "provider_id": "uuid-here",
        "total_requests": 5000,
        "success_rate": 0.95,
        "avg_latency_ms": 250,
        "total_tokens": 850000,
        "total_cost": 4.25,
        "error_count": 250,
        "timeout_count": 10,
        "quota_exhausted": 5
      }
    ]
  }
}
```

### GET /stream/logs

Stream di log delle richieste in tempo reale.

**Eventi inviati:**
- Nuovi log di richieste
- Dettagli completi (status, latency, token, costi)
- Aggiornamenti ogni 1 secondo

**Esempio evento:**
```json
{
  "id": "1738723890-2",
  "type": "logs",
  "data": {
    "timestamp": 1738723890,
    "logs": [
      {
        "id": "log-uuid",
        "provider_id": "provider-uuid",
        "model_id": "model-uuid",
        "user_id": "user-uuid",
        "method": "POST",
        "endpoint": "/v1/chat/completions",
        "status_code": 200,
        "latency_ms": 250,
        "input_tokens": 100,
        "output_tokens": 150,
        "success": true,
        "error_message": "",
        "estimated_cost": 0.002,
        "timestamp": 1738723890
      }
    ],
    "count": 1
  }
}
```

### GET /stream/providers

Stream dello stato dei provider.

**Eventi inviati:**
- Stato operativo dei provider
- Health score e metriche
- Modelli disponibili
- Aggiornamenti ogni 5 secondi

**Esempio evento:**
```json
{
  "id": "1738723890-3",
  "type": "providers",
  "data": {
    "timestamp": 1738723890,
    "providers": [
      {
        "id": "provider-uuid",
        "name": "OpenAI Free",
        "type": "free",
        "status": "active",
        "health_score": 0.95,
        "avg_latency_ms": 250,
        "supports_streaming": true,
        "supports_tools": true,
        "model_count": 5,
        "last_health_check": 1738723850,
        "realtime_requests": 5000,
        "realtime_success_rate": 0.95,
        "realtime_errors": 250
      }
    ],
    "total": 10
  }
}
```

### GET /stream/requests

Stream di singole richieste in tempo reale.

**Eventi inviati:**
- Ogni nuova richiesta al gateway
- Dettagli completi della richiesta
- Tempo reale (sub-secondo)

**Esempio evento:**
```json
{
  "id": "1738723890-4",
  "type": "requests",
  "data": {
    "id": "request-uuid",
    "provider_id": "provider-uuid",
    "model_id": "model-uuid",
    "user_id": "user-uuid",
    "method": "POST",
    "endpoint": "/v1/chat/completions",
    "status_code": 200,
    "latency_ms": 250,
    "input_tokens": 100,
    "output_tokens": 150,
    "success": true,
    "estimated_cost": 0.002,
    "timestamp": 1738723890
  }
}
```

### GET /stream/all

Stream di tutti gli eventi.

## API REST per Metriche

### GET /api/realtime/metrics

Ottieni lo snapshot corrente delle metriche aggregate.

**Risposta:**
```json
{
  "success": true,
  "data": {
    "total_requests": 15234,
    "successful_requests": 14484,
    "failed_requests": 750,
    "avg_latency_ms": 250.5,
    "avg_tokens_per_req": 250,
    "avg_cost_per_req": 0.0008,
    "total_cost": 12.45,
    "cost_per_minute": 0.25,
    "estimated_hourly_cost": 15.0,
    "active_users": 25,
    "unique_users_today": 150,
    "last_updated": 1738723890,
    "window_start": 1738720290,
    "providers": [...]
  }
}
```

### GET /api/realtime/providers/:id/metrics

Ottieni le metriche per un provider specifico.

### GET /api/realtime/hub/stats

Ottieni le statistiche dell'hub SSE (client connessi, sottoscrizioni).

## Esempi Client

### Browser JavaScript

```html
<!DOCTYPE html>
<html>
<head>
    <title>Real-time Dashboard</title>
</head>
<body>
    <h1>Live Statistics</h1>
    <div id="stats"></div>
    <div id="logs"></div>

    <script>
        // Connect to stats stream
        const statsSource = new EventSource('http://localhost:8080/stream/stats');

        statsSource.addEventListener('stats', (e) => {
            const data = JSON.parse(e.data);
            document.getElementById('stats').innerHTML =
                `<h2>Stats (${new Date(data.timestamp * 1000).toLocaleTimeString()})</h2>
                 <p>Total Requests: ${data.total_requests}</p>
                 <p>Total Cost: $${data.total_cost.toFixed(4)}</p>
                 <p>Providers: ${data.providers.length}</p>`;
        });

        statsSource.addEventListener('heartbeat', (e) => {
            console.log('Heartbeat:', e.data);
        });

        statsSource.onerror = (e) => {
            console.error('SSE error:', e);
            // EventSource auto-reconnects
        };

        // Connect to logs stream
        const logsSource = new EventSource('http://localhost:8080/stream/logs');

        logsSource.addEventListener('logs', (e) => {
            const data = JSON.parse(e.data);
            const logsHtml = data.logs.map(log =>
                `<div class="log ${log.success ? 'success' : 'error'}">
                    ${new Date(log.timestamp * 1000).toLocaleTimeString()} -
                    ${log.method} ${log.endpoint} -
                    ${log.status_code} -
                    ${log.latency_ms}ms
                 </div>`
            ).join('');

            document.getElementById('logs').innerHTML =
                `<h2>Recent Logs</h2>${logsHtml}`;
        });

        // Cleanup on page unload
        window.addEventListener('beforeunload', () => {
            statsSource.close();
            logsSource.close();
        });
    </script>

    <style>
        .log {
            padding: 5px;
            margin: 2px;
            border-left: 3px solid #ccc;
        }
        .log.success { border-color: #4caf50; }
        .log.error { border-color: #f44336; }
    </style>
</body>
</html>
```

### Go Client

```go
package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

type SSEEvent struct {
    ID    string          `json:"id"`
    Type  string          `json:"type"`
    Data  json.RawMessage `json:"data"`
}

func streamSSE(ctx context.Context, url string, handler func(SSEEvent)) error {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }

    req.Header.Set("Accept", "text/event-stream")
    req.Header.Set("Cache-Control", "no-cache")

    client := &http.Client{
        Timeout: 0, // No timeout for streaming
    }

    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    scanner := bufio.NewScanner(resp.Body)
    var event SSEEvent

    for scanner.Scan() {
        line := scanner.Text()

        if line == "" {
            // Event complete
            if event.Type != "" {
                handler(event)
            }
            event = SSEEvent{}
            continue
        }

        parts := strings.SplitN(line, ": ", 2)
        if len(parts) != 2 {
            continue
        }

        field, value := parts[0], parts[1]

        switch field {
        case "id":
            event.ID = value
        case "event":
            event.Type = value
        case "data":
            event.Data = json.RawMessage(value)
        }
    }

    return scanner.Err()
}

func main() {
    ctx := context.Background()

    // Stream stats
    go func() {
        url := "http://localhost:8080/stream/stats"
        err := streamSSE(ctx, url, func(event SSEEvent) {
            if event.Type == "stats" {
                var data map[string]interface{}
                json.Unmarshal(event.Data, &data)
                fmt.Printf("Stats Update: %v requests, $%.4f cost\n",
                    data["total_requests"], data["total_cost"])
            }
        })
        if err != nil {
            fmt.Printf("Error streaming stats: %v\n", err)
        }
    }()

    // Keep running
    select {}
}
```

### Python Client

```python
import sseclient
import requests
import json

def stream_stats():
    url = 'http://localhost:8080/stream/stats'
    response = requests.get(url, stream=True)
    client = sseclient.SSEClient(response)

    for event in client.events():
        if event.event == 'stats':
            data = json.loads(event.data)
            print(f"Stats: {data['total_requests']} requests, "
                  f"${data['total_cost']:.4f} cost")
        elif event.event == 'heartbeat':
            print("♥")

if __name__ == '__main__':
    try:
        stream_stats()
    except KeyboardInterrupt:
        print("\nDisconnected")
```

### cURL

```bash
# Stream stats
curl -N -H "Accept: text/event-stream" \
  http://localhost:8080/stream/stats

# Stream logs
curl -N -H "Accept: text/event-stream" \
  http://localhost:8080/stream/logs

# Stream all events
curl -N -H "Accept: text/event-stream" \
  http://localhost:8080/stream/all
```

## TUI Integration

Il client TUI può connettersi agli stream SSE per visualizzare dati in tempo reale:

```go
// In cmd/tui/views/dashboard.go

func (d *DashboardView) connectSSE() {
    go func() {
        url := "http://localhost:8080/stream/stats"
        streamSSE(context.Background(), url, func(event SSEEvent) {
            // Update TUI display
            d.updateStats(event.Data)
            d.app.Draw()
        })
    }()
}
```

## Caratteristiche

### Auto-Reconnect

Il client EventSource riconnette automaticamente in caso di disconnessione:

```javascript
const source = new EventSource(url);

source.onerror = (error) => {
    console.log('Connection lost, auto-reconnecting...');
    // EventSource riconnette automaticamente
    // con backoff esponenziale
};
```

### Last-Event-ID

Supporto per riprendere da un evento specifico dopo la riconnessione:

```javascript
// Il browser invia automaticamente Last-Event-ID header
const source = new EventSource(url);

// O manualmente con query param
const source = new EventSource(url + '?lastEventId=' + lastId);
```

### Heartbeat

Eventi heartbeat automatici ogni 15 secondi per mantenere la connessione viva:

```json
{
  "id": "1738723890-5",
  "type": "heartbeat",
  "data": {
    "timestamp": 1738723890
  }
}
```

### Multiple Channels

Sottoscrizione a canali specifici:

```javascript
// Solo stats
const statsSource = new EventSource('/stream/stats');

// Solo logs
const logsSource = new EventSource('/stream/logs');

// Tutti gli eventi
const allSource = new EventSource('/stream/all');
```

## Configurazione

```go
// In cmd/backend/main.go o serve.go

import "github.com/biodoia/goleapifree/internal/realtime"

// Create components
hub := realtime.NewSSEHub(
    15*time.Second,  // heartbeat interval
    5*time.Minute,   // client timeout
    100,             // buffer size
)

aggregator := realtime.NewAggregator(
    1*time.Minute,   // window size
    60,              // window count (60 windows = 1 hour)
    5*time.Minute,   // user timeout
)

streamer := realtime.NewStreamer(hub, collector, db)
streamer.SetIntervals(
    2*time.Second,   // stats interval
    1*time.Second,   // logs interval
    5*time.Second,   // providers interval
)

handlers := realtime.NewHandlers(hub, streamer, aggregator)

// Start services
hub.Start()
aggregator.Start()
streamer.Start()

// Register routes
handlers.RegisterRoutes(router)

// Cleanup on shutdown
defer func() {
    streamer.Stop()
    aggregator.Stop()
    hub.Stop()
}()
```

## Performance

### Scalabilità

- **Buffer size**: Configurabile per bilanciare memoria vs latenza
- **Client timeout**: Disconnessione automatica client inattivi
- **Window management**: Rolling windows per metriche aggregate
- **Batch operations**: Eventi broadcastati in batch quando possibile

### Ottimizzazioni

1. **Buffering**: Eventi bufferizzati per ridurre overhead
2. **Compression**: Header `Content-Encoding: gzip` se supportato
3. **Selective Broadcasting**: Solo ai client sottoscritti ai canali specifici
4. **Memory Cleanup**: Pulizia periodica di client e window obsolete

### Limiti Consigliati

- **Max clients**: 1000 connessioni simultanee
- **Buffer size**: 100-500 eventi per client
- **Heartbeat**: 15-30 secondi
- **Window size**: 1-5 minuti
- **Window count**: 60-300 (1-5 ore di dati)

## Monitoring

### Hub Stats

```bash
curl http://localhost:8080/api/realtime/hub/stats
```

```json
{
  "success": true,
  "data": {
    "total_clients": 25,
    "subscriptions": {
      "stats": 15,
      "logs": 8,
      "providers": 5,
      "requests": 3
    }
  }
}
```

### Metriche Prometheus

```prometheus
# SSE clients connected
sse_clients_total 25

# Events sent per channel
sse_events_sent_total{channel="stats"} 1500
sse_events_sent_total{channel="logs"} 3000
sse_events_sent_total{channel="providers"} 600

# Client disconnections
sse_disconnections_total{reason="timeout"} 10
sse_disconnections_total{reason="error"} 5
```

## Troubleshooting

### Client Non Riceve Eventi

1. Verifica headers CORS
2. Controlla firewall/proxy
3. Verifica client timeout
4. Controlla logs server

### Disconnessioni Frequenti

1. Aumenta heartbeat interval
2. Verifica network stability
3. Controlla buffer size
4. Review client timeout

### Memoria Elevata

1. Riduci buffer size
2. Riduci window count
3. Aumenta cleanup frequency
4. Limita client connessi

### Eventi Duplicati

1. Usa Last-Event-ID
2. Verifica reconnection logic
3. Implementa deduplication lato client
