# Implementazione Provider Anthropic Claude

## Overview

Sistema completo di provider per l'integrazione di Anthropic Claude API in GoLeapAI. Supporta sia richieste native Anthropic che conversione trasparente da formato OpenAI.

## Architettura

```
┌─────────────────────┐
│  Gateway GoLeapAI   │
└──────────┬──────────┘
           │
           ├─── Richiesta OpenAI format
           │    (POST /v1/chat/completions)
           │
           ▼
┌─────────────────────┐
│      Adapter        │  ◄── Converte OpenAI ↔ Anthropic
└──────────┬──────────┘
           │
           ├─── Richiesta Anthropic format
           │
           ▼
┌─────────────────────┐
│      Client         │  ◄── HTTP client per API Claude
└──────────┬──────────┘
           │
           ├─── HTTP/HTTPS
           │
           ▼
┌─────────────────────┐
│  Anthropic API      │
│  api.anthropic.com  │
└─────────────────────┘
```

## File Implementati

### 1. `types.go` (420 righe)
Definisce tutti i tipi per l'API Anthropic.

**Strutture principali:**
- `MessagesRequest`: Richiesta all'endpoint /v1/messages
- `MessagesResponse`: Risposta dall'API
- `Message`: Singolo messaggio nella conversazione
- `ContentBlock`: Blocco di contenuto (text, image, tool_use, tool_result)
- `StreamEvent`: Evento di streaming SSE
- `Error`: Errore API con metodi di utilità

**Costanti:**
- Modelli Claude (3 Opus, 3 Sonnet, 3 Haiku, 3.5 Sonnet, 3.5 Haiku)
- Tipi di errore API
- Valori di default

**Metodi di utilità:**
- `Validate()`: Validazione richieste
- `GetText()`, `GetAllText()`: Estrazione testo
- `HasToolUse()`, `GetToolUses()`: Gestione tool calling
- `IsRateLimitError()`, `IsAuthError()`, `IsRetryable()`: Classificazione errori

### 2. `client.go` (330 righe)
Client HTTP per l'API Anthropic.

**Funzionalità:**
- `CreateMessage()`: Richiesta sincrona
- `CreateMessageStream()`: Streaming con SSE
- `Health()`: Health check
- `CountTokens()`: Stima token (approssimativa)

**Caratteristiche:**
- Timeout configurabile (default 120s)
- Custom HTTP client support
- Rate limit parsing dagli header
- Error handling dettagliato
- Retry logic con backoff esponenziale

**Headers gestiti:**
- `X-API-Key`: Autenticazione
- `Anthropic-Version`: Versione API
- `Content-Type`: application/json
- `Accept`: text/event-stream (per streaming)
- Rate limit headers (requests/tokens limit/remaining/reset)

### 3. `adapter.go` (580 righe)
Adapter bidirezionale OpenAI ↔ Anthropic.

**Conversioni supportate:**

#### OpenAI → Anthropic
- `ConvertRequest()`: Converte richiesta completa
- System messages → `system` field separato
- Messages → Content blocks
- Tools → Anthropic tools format
- Stop sequences → Anthropic format
- Model mapping (gpt-4 → claude-3-opus, etc.)

#### Anthropic → OpenAI
- `ConvertResponse()`: Converte risposta completa
- Content blocks → OpenAI message
- Tool uses → OpenAI tool calls
- Stop reasons → Finish reasons
- Usage stats → OpenAI usage format

#### Streaming
- `ConvertStreamEvent()`: Converte eventi SSE
- Message start → Role delta
- Content delta → Text delta
- Message stop → Finish reason

**Mapping modelli predefiniti:**
```go
gpt-4          → claude-3-opus-20240229
gpt-4-turbo    → claude-3-5-sonnet-20241022
gpt-4o         → claude-3-5-sonnet-20241022
gpt-3.5-turbo  → claude-3-5-haiku-20241022
```

### 4. `provider.go` (290 righe)
Interfaccia di alto livello per il gateway.

**API pubblica:**
- `ChatCompletion()`: OpenAI-compatible request handler
- `ChatCompletionStream()`: OpenAI-compatible streaming
- `CreateMessage()`: Native Anthropic request
- `CreateMessageStream()`: Native Anthropic streaming
- `Health()`: Provider health check
- `EstimateCost()`: Stima costi per richiesta
- `GetModelInfo()`: Info su modello specifico
- `ListModels()`: Lista tutti i modelli

**Configurazione:**
```go
type ProviderConfig struct {
    APIKey     string        // Required
    BaseURL    string        // Default: https://api.anthropic.com
    APIVersion string        // Default: 2023-06-01
    Timeout    time.Duration // Default: 120s
    MaxRetries int           // Default: 3
    UserAgent  string        // Default: GoLeapAI/1.0
}
```

**Retry logic:**
- Exponential backoff (1s, 2s, 4s, ...)
- Solo per errori ritentabili (rate limit, overloaded, api_error)
- Fallisce subito per errori non ritentabili (auth, invalid_request)

**Cost estimation:**
Prezzi per 1M token (USD):

| Modello | Input | Output |
|---------|-------|--------|
| Claude 3 Opus | $15 | $75 |
| Claude 3 Sonnet | $3 | $15 |
| Claude 3 Haiku | $0.25 | $1.25 |
| Claude 3.5 Sonnet | $3 | $15 |
| Claude 3.5 Haiku | $1 | $5 |

### 5. `adapter_test.go` (340 righe)
Test completi per l'adapter.

**Coverage:**
- Conversione richieste (semplici, con system, con parametri)
- Conversione messaggi (user, assistant, system, multi-turn)
- Conversione risposte
- Mapping modelli
- Mapping finish reasons
- Conversione tools
- Conversione stop sequences
- Validazione richieste
- Metodi di utilità MessagesResponse
- Metodi Error

**Tutti i test passano:** ✅

### 6. `example_test.go` (150 righe)
Esempi d'uso documentati.

**Esempi:**
- Richiesta sincrona base
- Streaming
- Multi-turn conversation
- Conversione OpenAI → Anthropic
- Conversione Anthropic → OpenAI

### 7. `integration_example.go` (280 righe)
Esempi completi di integrazione con il gateway.

**Scenari:**
- Gateway handling richieste OpenAI
- Streaming OpenAI-compatible
- Richieste native Anthropic
- Multi-turn conversations
- Tool/Function calling
- Health checks
- Cost estimation
- Model listing

## Utilizzo

### 1. Setup Base

```go
import "github.com/biodoia/goleapifree/internal/providers/anthropic"

// Configurazione
config := anthropic.ProviderConfig{
    APIKey:     "sk-ant-...",
    MaxRetries: 3,
    Timeout:    120 * time.Second,
}

// Crea provider
provider := anthropic.NewProvider(config)
```

### 2. Richiesta OpenAI-Compatible

```go
// Il gateway riceve richiesta OpenAI
req := &anthropic.OpenAIRequest{
    Model: "gpt-4",
    Messages: []anthropic.OpenAIMessage{
        {Role: "user", Content: "Hello!"},
    },
    MaxTokens: 1024,
}

// Il provider gestisce tutto automaticamente
resp, err := provider.ChatCompletion(ctx, req)
// resp è in formato OpenAI, pronto per essere restituito al client
```

### 3. Streaming

```go
chunkCh, errCh := provider.ChatCompletionStream(ctx, req)

for {
    select {
    case chunk := <-chunkCh:
        // chunk è in formato OpenAI SSE
        sendToClient(chunk)
    case err := <-errCh:
        handleError(err)
    }
}
```

### 4. Richiesta Native Anthropic

```go
// Per endpoint /v1/messages
req := &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Sonnet,
    MaxTokens: 1024,
    System:    "You are helpful",
    Messages: []anthropic.Message{
        {
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewTextContentBlock("Hello!"),
            },
        },
    },
}

resp, err := provider.CreateMessage(ctx, req)
```

## Integrazione con Gateway GoLeapAI

### Gateway Handler

```go
// internal/gateway/gateway.go

func (g *Gateway) handleChatCompletion(c fiber.Ctx) error {
    var req anthropic.OpenAIRequest
    if err := c.Bind().Body(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
    }

    // Seleziona provider (potrebbero essere multipli)
    provider := g.selectProvider("anthropic")

    // Gestisci streaming vs non-streaming
    if req.Stream {
        return g.handleStreamingResponse(c, provider, &req)
    }

    // Richiesta normale
    resp, err := provider.ChatCompletion(c.Context(), &req)
    if err != nil {
        return g.handleError(c, err)
    }

    return c.JSON(resp)
}

func (g *Gateway) handleMessages(c fiber.Ctx) error {
    var req anthropic.MessagesRequest
    if err := c.Bind().Body(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
    }

    provider := g.selectProvider("anthropic")

    if req.Stream {
        return g.handleAnthropicStreaming(c, provider, &req)
    }

    resp, err := provider.CreateMessage(c.Context(), &req)
    if err != nil {
        return g.handleError(c, err)
    }

    return c.JSON(resp)
}
```

### Provider Registry

```go
// internal/providers/registry.go

type Registry struct {
    providers map[string]*anthropic.Provider
}

func (r *Registry) Register(name string, provider *anthropic.Provider) {
    r.providers[name] = provider
}

func (r *Registry) Get(name string) *anthropic.Provider {
    return r.providers[name]
}

// Inizializzazione
func InitProviders(db *database.DB) *Registry {
    registry := &Registry{
        providers: make(map[string]*anthropic.Provider),
    }

    // Carica configurazioni provider dal database
    var providers []models.Provider
    db.Where("name = ?", "anthropic").Find(&providers)

    for _, p := range providers {
        config := anthropic.ProviderConfig{
            APIKey:  getAPIKey(p.ID), // Da vault/secret store
            BaseURL: p.BaseURL,
        }

        provider := anthropic.NewProvider(config)
        registry.Register(p.Name, provider)
    }

    return registry
}
```

## Caratteristiche Avanzate

### 1. Tool Calling

```go
tools := []anthropic.Tool{
    {
        Name:        "search",
        Description: "Search the web",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "query": {"type": "string"}
            },
            "required": ["query"]
        }`),
    },
}

req.Tools = tools
resp, _ := provider.CreateMessage(ctx, req)

if resp.HasToolUse() {
    for _, toolUse := range resp.GetToolUses() {
        // Esegui la tool
        result := executeTool(toolUse.Name, toolUse.Input)

        // Continua la conversazione con il risultato
        req.Messages = append(req.Messages, anthropic.Message{
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewToolResultContentBlock(
                    toolUse.ID,
                    result,
                    false,
                ),
            },
        })
    }
}
```

### 2. Vision (Immagini)

```go
// Base64 encoded image
imageData := "iVBORw0KGgoAAAANS..."

req := &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Sonnet,
    MaxTokens: 1024,
    Messages: []anthropic.Message{
        {
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewTextContentBlock("What's in this image?"),
                anthropic.NewImageContentBlock("image/png", imageData),
            },
        },
    },
}
```

### 3. Error Handling

```go
resp, err := provider.ChatCompletion(ctx, req)
if err != nil {
    if apiErr, ok := err.(*anthropic.Error); ok {
        switch {
        case apiErr.IsRateLimitError():
            // Attendi e riprova
            time.Sleep(apiErr.RetryAfter)
            return retry(req)

        case apiErr.IsAuthError():
            // Chiave API non valida
            return fmt.Errorf("invalid API key")

        case apiErr.IsRetryable():
            // Riprova con backoff
            return retryWithBackoff(req)

        default:
            // Errore non recuperabile
            return err
        }
    }
}
```

### 4. Rate Limiting

Il client estrae automaticamente gli header di rate limiting:

```
anthropic-ratelimit-requests-limit: 1000
anthropic-ratelimit-requests-remaining: 999
anthropic-ratelimit-requests-reset: 2024-01-01T00:00:00Z
anthropic-ratelimit-tokens-limit: 100000
anthropic-ratelimit-tokens-remaining: 95000
anthropic-ratelimit-tokens-reset: 2024-01-01T00:00:00Z
retry-after: 60
```

Questi possono essere usati per implementare rate limiting lato gateway.

## Testing

### Unit Tests

```bash
# Esegui tutti i test
go test ./internal/providers/anthropic/...

# Con coverage
go test -cover ./internal/providers/anthropic/...

# Con dettagli
go test -v ./internal/providers/anthropic/...
```

### Integration Tests

Richiede `ANTHROPIC_API_KEY`:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
go test -v ./internal/providers/anthropic/... -run Integration
```

## Metriche e Monitoring

Il provider può essere integrato con Prometheus:

```go
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "anthropic_requests_total",
            Help: "Total Anthropic API requests",
        },
        []string{"model", "status"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "anthropic_request_duration_seconds",
            Help: "Anthropic API request duration",
        },
        []string{"model"},
    )

    tokensUsed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "anthropic_tokens_total",
            Help: "Total tokens used",
        },
        []string{"model", "type"}, // type: input/output
    )
)
```

## Limiti e Considerazioni

### API Limits (Anthropic)
- **Context window**: 200K tokens (tutti i modelli)
- **Max output**: 4K (Claude 3), 8K (Claude 3.5)
- **Rate limits**: Variano per tier (vedi documentazione Anthropic)
- **Timeout**: Default 120s, configurabile

### Differenze OpenAI vs Anthropic
1. **System messages**: Anthropic usa campo separato `system`
2. **Message alternation**: Anthropic richiede alternanza user/assistant
3. **First message**: Deve essere sempre user
4. **Images**: Formato diverso (base64 vs URL)
5. **Tools**: Schema leggermente diverso
6. **Streaming**: SSE format differente

### Performance
- **Latency**: Tipicamente 1-3s per risposta
- **Throughput**: Limitato da rate limits API
- **Memory**: ~10MB per istanza provider
- **Concurrency**: Thread-safe, può gestire richieste parallele

## Roadmap

### Completato ✅
- [x] Client base Anthropic
- [x] Streaming support
- [x] Adapter OpenAI↔Anthropic
- [x] Tool calling
- [x] Vision support
- [x] Error handling
- [x] Rate limiting
- [x] Multi-turn conversations
- [x] Cost estimation
- [x] Unit tests

### Prossimi Step 🚀
- [ ] Integration tests con API reale
- [ ] Caching delle risposte
- [ ] Prompt caching (feature Anthropic)
- [ ] Batch processing
- [ ] Advanced retry strategies
- [ ] Circuit breaker pattern
- [ ] Request/response middleware
- [ ] Detailed metrics e tracing
- [ ] Admin UI per config provider

## License

Parte di GoLeapAI - MIT License
