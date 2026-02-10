# GoLeapAI Provider System

Sistema modulare per gestire provider LLM multipli con supporto OpenAI-compatible API.

## Architettura

```
internal/providers/
├── base.go              # Interface base e tipi comuni
├── registry.go          # Registry per gestire provider multipli
├── openai/
│   ├── client.go        # Client OpenAI con retry e streaming
│   └── types.go         # Tipi specifici OpenAI API
└── examples_test.go     # Esempi di utilizzo
```

## Features

- **OpenAI-Compatible API**: Supporto completo per OpenAI API standard
- **Streaming**: Server-Sent Events (SSE) per risposte in streaming
- **Tool Calling**: Supporto completo per function calling
- **JSON Mode**: Risposte in formato JSON strutturato
- **Retry Logic**: Retry automatico con backoff esponenziale
- **Rate Limiting**: Gestione automatica dei rate limits
- **Health Checks**: Monitoraggio della salute dei provider
- **Provider Registry**: Gestione centralizzata di provider multipli
- **Fallback Automatico**: Failover tra provider in caso di errori

## Utilizzo Base

### 1. Client Singolo

```go
import (
    "github.com/biodoia/goleapifree/internal/providers"
    "github.com/biodoia/goleapifree/internal/providers/openai"
)

// Crea un client
client := openai.NewClient(
    "openai",
    "https://api.openai.com",
    "sk-your-api-key",
)

// Chat completion
req := &providers.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []providers.Message{
        {Role: "user", Content: "Hello!"},
    },
}

resp, err := client.ChatCompletion(context.Background(), req)
```

### 2. Streaming

```go
err := client.Stream(ctx, req, func(chunk *providers.StreamChunk) error {
    if chunk.Done {
        fmt.Println("Stream completed")
        return nil
    }
    fmt.Print(chunk.Delta)
    return nil
})
```

### 3. Tool Calling

```go
tools := []providers.Tool{
    {
        Type: "function",
        Function: providers.Function{
            Name: "get_weather",
            Description: "Get weather for a location",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "location": map[string]string{
                        "type": "string",
                    },
                },
                "required": []string{"location"},
            },
        },
    },
}

req := &providers.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []providers.Message{
        {Role: "user", Content: "What's the weather in Paris?"},
    },
    Tools: tools,
    ToolChoice: "auto",
}
```

### 4. JSON Mode

```go
req := &providers.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []providers.Message{
        {Role: "user", Content: "Generate a person JSON"},
    },
    ResponseFormat: &providers.ResponseFormat{
        Type: "json_object",
    },
}
```

## Provider Registry

### Registrazione Provider

```go
registry := providers.NewRegistry()

// Registra più provider
registry.Register("openai", openaiClient, "openai")
registry.Register("anthropic", anthropicClient, "anthropic")
registry.Register("local", localClient, "local")
```

### Utilizzo Registry

```go
// Get provider specifico
provider, err := registry.Get("openai")

// Get primo disponibile
provider, err := registry.GetFirst()

// Get con fallback
provider, err := registry.GetOrFirst("preferred-provider")

// Lista provider
providers := registry.List()
activeProviders := registry.ListActive()
```

### Health Checks

```go
// Health check su tutti i provider
results := registry.HealthCheck(context.Background())

for name, err := range results {
    if err != nil {
        log.Printf("Provider %s unhealthy: %v", name, err)
    }
}

// Get statistics
stats := registry.GetStats()
fmt.Printf("Total: %d, Active: %d, Healthy: %d\n",
    stats.TotalProviders,
    stats.ActiveProviders,
    stats.HealthyProviders,
)
```

### Tracking Errors e Success

```go
start := time.Now()
resp, err := provider.ChatCompletion(ctx, req)
latency := time.Since(start)

if err != nil {
    registry.RecordError(provider.Name())
} else {
    registry.RecordSuccess(provider.Name(), latency)
}
```

## Configurazione

### Timeout e Retry

```go
client := openai.NewClient("name", "url", "key")

// Imposta timeout
client.SetTimeout(60 * time.Second)

// Imposta max retry
client.SetMaxRetries(5)
```

### Capabilities

```go
// Verifica feature support
if client.SupportsFeature(providers.FeatureStreaming) {
    // Use streaming
}

// Configura capabilities per provider custom
client.SetFeature(providers.FeatureTools, false)
client.SetFeature(providers.FeatureVision, true)
```

## Provider Compatibili

Qualsiasi provider che espone un'API compatibile con OpenAI può essere usato:

- **OpenAI**: GPT-3.5, GPT-4, GPT-4 Vision
- **Anthropic**: Claude (via OpenAI-compatible proxy)
- **Groq**: API OpenAI-compatible
- **Together AI**: API OpenAI-compatible
- **Ollama**: Local models con API OpenAI-compatible
- **LM Studio**: Local server OpenAI-compatible
- **Custom providers**: Qualsiasi endpoint che implementa l'API OpenAI

### Esempio Provider Custom

```go
// Provider custom con API OpenAI-compatible
client := openai.NewClient(
    "my-custom-llm",
    "https://my-api.com",
    "custom-key",
)

// Configura secondo le capabilities del provider
client.SetFeature(providers.FeatureTools, false)
client.SetTimeout(120 * time.Second)

// Usa normalmente
resp, err := client.ChatCompletion(ctx, req)
```

## Error Handling

```go
resp, err := client.ChatCompletion(ctx, req)
if err != nil {
    switch {
    case errors.Is(err, openai.ErrInvalidAPIKey):
        // Handle invalid API key
    case errors.Is(err, openai.ErrRateLimitExceeded):
        // Handle rate limit
    case errors.Is(err, openai.ErrModelNotFound):
        // Handle model not found
    case errors.Is(err, openai.ErrServiceUnavailable):
        // Handle service down
    default:
        // Handle generic error
    }
}
```

## Best Practices

1. **Usa il Registry per prod**: Gestione centralizzata di provider multipli
2. **Implementa Fallback**: Configura provider di backup
3. **Monitor Health**: Esegui health check periodici
4. **Track Metrics**: Registra successi ed errori
5. **Configure Timeouts**: Imposta timeout appropriati per il tuo use case
6. **Handle Streaming Errors**: Gestisci errori nella callback di streaming
7. **Validate Responses**: Verifica sempre che la risposta sia completa

## Performance

- **HTTP Client**: Usa `resty/v2` con connection pooling
- **Retry Logic**: Backoff esponenziale automatico
- **Streaming**: SSE con parsing efficiente
- **Concurrent Health Checks**: Goroutine parallele per health check
- **Thread-Safe**: Registry thread-safe con sync.RWMutex

## Testing

Vedi `examples_test.go` per esempi completi di utilizzo.

## TODO

- [ ] Supporto per più tipi di autenticazione (OAuth2)
- [ ] Rate limiting personalizzato per provider
- [ ] Caching delle risposte
- [ ] Metriche Prometheus
- [ ] Circuit breaker pattern
- [ ] Load balancing tra provider
- [ ] Request/Response middleware
- [ ] Provider-specific optimizations
