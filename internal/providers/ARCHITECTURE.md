# GoLeapAI Provider System - Architettura

## Overview

Il sistema di provider di GoLeapAI è progettato per supportare multipli provider LLM con un'interfaccia unificata OpenAI-compatible. 

```
┌─────────────────────────────────────────────────────────────┐
│                     GoLeapAI Gateway                        │
│                    (Fiber HTTP Server)                      │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       v
┌─────────────────────────────────────────────────────────────┐
│                  Provider Manager                           │
│  - Load balancing                                          │
│  - Health monitoring                                        │
│  - Metrics tracking                                         │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       v
┌─────────────────────────────────────────────────────────────┐
│                  Provider Registry                          │
│  - Provider registration                                    │
│  - Status management                                        │
│  - Metadata storage                                         │
└──────────────────────┬──────────────────────────────────────┘
                       │
         ┌─────────────┼─────────────┬─────────────┐
         v             v             v             v
    ┌────────┐   ┌─────────┐   ┌─────────┐   ┌────────┐
    │ OpenAI │   │  Groq   │   │Together │   │ Ollama │
    │ Client │   │ Client  │   │ Client  │   │ Client │
    └────┬───┘   └────┬────┘   └────┬────┘   └───┬────┘
         │            │             │            │
         v            v             v            v
    ┌────────────────────────────────────────────────┐
    │         OpenAI Compatible API                  │
    │   (Chat, Streaming, Tools, JSON Mode)          │
    └────────────────────────────────────────────────┘
```

## Componenti Principali

### 1. Base Interface (`base.go`)

Definisce l'interfaccia comune per tutti i provider:

```go
type Provider interface {
    ChatCompletion(ctx, req) (resp, error)
    Stream(ctx, req, handler) error
    HealthCheck(ctx) error
    GetModels(ctx) ([]ModelInfo, error)
    SupportsFeature(feature) bool
}
```

**Features supportate:**
- Streaming (SSE)
- Tool calling / Function calling
- JSON mode
- Vision (multimodal)
- System messages

### 2. Registry (`registry.go`)

Gestisce la registrazione e lo stato di tutti i provider:

**Funzionalità:**
- Registrazione dinamica di provider
- Status management (active, inactive, unhealthy, maintenance)
- Health check paralleli
- Metrics tracking (success/error count, latency)
- Thread-safe con RWMutex

**Metadata tracciati:**
- Provider name, type, status
- Last health check timestamp
- Error/Success counters
- Average latency
- Feature support

### 3. OpenAI Client (`openai/client.go`)

Implementazione completa dell'OpenAI API:

**Features:**
- HTTP client con resty/v2
- Automatic retry con exponential backoff
- Timeout configurabili
- Streaming SSE support
- Tool calling accumulation
- Error mapping dettagliato

**Error handling:**
- `ErrInvalidAPIKey` (401)
- `ErrRateLimitExceeded` (429)
- `ErrModelNotFound` (404)
- `ErrInvalidRequest` (400)
- `ErrServiceUnavailable` (503)

### 4. OpenAI Types (`openai/types.go`)

Strutture dati complete dell'API OpenAI:

```go
ChatCompletionRequest
ChatCompletionResponse
ChatCompletionStreamResponse
ChatMessage
Choice
StreamChoice
Usage
Tool
ToolCall
Function
FunctionCall
ContentPart (multimodal)
```

### 5. Provider Manager (`provider-manager/manager.go`)

Layer ad alto livello per orchestrare i provider:

**Funzionalità:**
- Load balancing automatico
- Failover tra provider
- Health check worker
- Statistics aggregation
- Configuration presets

## Data Flow

### Request Flow (Non-Streaming)

```
Client Request
    ↓
Gateway Handler
    ↓
Provider Manager
    ↓
Registry.Get(provider)
    ↓
Provider.ChatCompletion()
    ↓
HTTP Request (resty)
    ↓
OpenAI API / Compatible
    ↓
Response
    ↓
Convert to standard format
    ↓
Track metrics
    ↓
Return to client
```

### Streaming Flow

```
Client Request (Stream: true)
    ↓
Gateway Handler
    ↓
Provider Manager.Stream()
    ↓
Provider.Stream() with callback
    ↓
HTTP Request (SSE)
    ↓
Parse SSE events
    ↓
Accumulate tool calls
    ↓
For each chunk:
    ↓
    StreamHandler callback
        ↓
        Write to response stream
```

### Health Check Flow

```
Health Check Timer
    ↓
Registry.HealthCheck(ctx)
    ↓
Parallel goroutines for each provider
    ↓
Provider.HealthCheck()
    ↓
Update metadata:
    - LastHealthCheck timestamp
    - HealthCheckStatus
    - ErrorCount/SuccessCount
    ↓
Log results
```

## Configuration

### Provider Registration

```go
// Simple
pm.RegisterOpenAICompatible(name, url, key)

// Advanced
pm.RegisterWithConfig(ProviderConfig{
    Name:       "custom",
    BaseURL:    "https://api.custom.com",
    APIKey:     "key",
    Timeout:    60 * time.Second,
    MaxRetries: 5,
    Features: map[Feature]bool{
        FeatureStreaming: true,
        FeatureTools:     false,
    },
})

// From presets
pm.LoadDefaultProviders(apiKeys)
```

### Default Configurations

Provider preconfigurati:
- **OpenAI**: Full features, 30s timeout
- **Groq**: Fast inference, no vision
- **Together**: Open source models
- **Ollama**: Local models, 120s timeout

## Error Handling

### Retry Strategy

1. Automatic retry con exponential backoff
2. Retry su 5xx errors
3. Retry su 429 (rate limit)
4. Retry su 408 (timeout)
5. Max retries configurabile (default: 3)

### Fallback Strategy

```go
// Try primary provider
resp, err := pm.ChatCompletion(ctx, "primary", req)

// On error, load balance to others
if err != nil {
    resp, err = pm.LoadBalancedRequest(ctx, req)
}
```

## Performance

### HTTP Client

- Connection pooling (resty)
- Keep-alive connections
- Configurable timeouts
- Parallel requests support

### Registry

- RWMutex per thread-safety
- Lock granulare (read vs write)
- Parallel health checks

### Streaming

- Efficient SSE parsing
- Incremental tool call building
- Low memory footprint

## Monitoring

### Metrics Tracked

Per provider:
- Request count (success/error)
- Average latency
- Error rate
- Health status
- Last health check time

Aggregate:
- Total providers
- Active providers
- Healthy providers
- Overall success rate

### Health Status

- **Healthy**: Last check passed
- **Unhealthy**: Last check failed or >5 consecutive errors
- **Unknown**: Never checked

## Extensibility

### Adding New Provider Types

1. Implement `Provider` interface
2. Add to registry
3. Configure features
4. (Optional) Add preset config

### Custom Features

```go
// Define custom feature
const FeatureCustom Feature = "custom_feature"

// Set on provider
provider.SetFeature(FeatureCustom, true)

// Check before use
if provider.SupportsFeature(FeatureCustom) {
    // Use custom feature
}
```

## Security

### API Key Management

- Keys stored in memory only
- No logging of sensitive data
- Environment variable loading
- Support for secret managers

### Request Validation

- Input sanitization
- Rate limiting (delegated to provider)
- Timeout enforcement
- Context cancellation

## Testing

### Unit Tests

- MockProvider for testing
- Registry tests (20+ test cases)
- Feature flag tests
- Thread-safety tests

### Integration Tests

- Real provider testing (optional)
- Health check verification
- Streaming validation

## Future Enhancements

- [ ] Circuit breaker pattern
- [ ] Response caching
- [ ] Request queuing
- [ ] Advanced load balancing (weighted, latency-based)
- [ ] Prometheus metrics export
- [ ] Request/response middleware
- [ ] Provider-specific optimizations
- [ ] OAuth2 authentication
- [ ] WebSocket streaming
- [ ] Batch requests
