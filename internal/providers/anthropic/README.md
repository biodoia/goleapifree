# Anthropic Claude Provider

Provider per l'integrazione con l'API Anthropic Claude in GoLeapAI.

## Caratteristiche

- **Client nativo Anthropic**: Supporto completo per l'API Messages di Claude
- **Streaming**: Supporto per risposte in streaming con Server-Sent Events
- **Multi-turn conversations**: Gestione di conversazioni multi-turno
- **System prompts**: Supporto per system prompts separati
- **Tool calling**: Supporto per function calling (tools)
- **Adapter OpenAI**: Conversione trasparente tra formati OpenAI e Anthropic
- **Rate limiting**: Parsing automatico degli header di rate limit
- **Error handling**: Gestione dettagliata degli errori specifici di Claude

## Installazione

```bash
go get github.com/biodoia/goleapifree/internal/providers/anthropic
```

## Uso Base

### Client Nativo Anthropic

```go
package main

import (
    "context"
    "fmt"
    "github.com/biodoia/goleapifree/internal/providers/anthropic"
)

func main() {
    // Crea il client
    client := anthropic.NewClient("your-api-key")

    // Prepara la richiesta
    req := &anthropic.MessagesRequest{
        Model:     anthropic.ModelClaude35Sonnet,
        MaxTokens: 1024,
        Messages: []anthropic.Message{
            {
                Role: anthropic.MessageRoleUser,
                Content: []anthropic.ContentBlock{
                    anthropic.NewTextContentBlock("Explain AI in simple terms."),
                },
            },
        },
    }

    // Invia la richiesta
    resp, err := client.CreateMessage(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.GetText())
}
```

### Streaming

```go
req := &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Haiku,
    MaxTokens: 1024,
    Messages: []anthropic.Message{
        {
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewTextContentBlock("Write a story about AI."),
            },
        },
    },
}

eventCh, errCh := client.CreateMessageStream(context.Background(), req)

for {
    select {
    case event, ok := <-eventCh:
        if !ok {
            return
        }
        if event.Delta != nil && event.Delta.Text != "" {
            fmt.Print(event.Delta.Text)
        }
    case err := <-errCh:
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
    }
}
```

### System Prompts

```go
req := &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Sonnet,
    MaxTokens: 1024,
    System:    "You are a helpful coding assistant specialized in Go.",
    Messages: []anthropic.Message{
        {
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewTextContentBlock("How do I implement error handling in Go?"),
            },
        },
    },
}
```

### Multi-turn Conversations

```go
messages := []anthropic.Message{
    {
        Role: anthropic.MessageRoleUser,
        Content: []anthropic.ContentBlock{
            anthropic.NewTextContentBlock("What's the capital of France?"),
        },
    },
}

// Prima risposta
resp1, _ := client.CreateMessage(ctx, &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Haiku,
    MaxTokens: 100,
    Messages:  messages,
})

// Aggiungi alla conversazione
messages = append(messages, anthropic.Message{
    Role:    anthropic.MessageRoleAssistant,
    Content: resp1.Content,
})

messages = append(messages, anthropic.Message{
    Role: anthropic.MessageRoleUser,
    Content: []anthropic.ContentBlock{
        anthropic.NewTextContentBlock("What's its population?"),
    },
})

// Seconda risposta con contesto
resp2, _ := client.CreateMessage(ctx, &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Haiku,
    MaxTokens: 100,
    Messages:  messages,
})
```

## Adapter OpenAI→Anthropic

L'adapter permette di usare richieste in formato OpenAI che vengono automaticamente convertite per Claude.

### Conversione Richiesta

```go
adapter := anthropic.NewAdapter()

// Richiesta OpenAI-compatible
openaiReq := &anthropic.OpenAIRequest{
    Model: "gpt-4",
    Messages: []anthropic.OpenAIMessage{
        {
            Role:    "system",
            Content: "You are a helpful assistant.",
        },
        {
            Role:    "user",
            Content: "Hello!",
        },
    },
    MaxTokens:   1024,
    Temperature: ptrFloat64(0.7),
}

// Converti in formato Anthropic
anthropicReq, err := adapter.ConvertRequest(openaiReq)

// Usa con il client Anthropic
resp, err := client.CreateMessage(ctx, anthropicReq)
```

### Conversione Risposta

```go
// Risposta da Claude
anthropicResp, err := client.CreateMessage(ctx, req)

// Converti in formato OpenAI
openaiResp := adapter.ConvertResponse(anthropicResp)

// Ora è compatibile con client OpenAI
fmt.Println(openaiResp.Choices[0].Message.Content)
fmt.Println(openaiResp.Usage.TotalTokens)
```

### Mapping Modelli

L'adapter include mapping predefiniti:

- `gpt-4` → `claude-3-opus-20240229`
- `gpt-4-turbo` → `claude-3-5-sonnet-20241022`
- `gpt-4o` → `claude-3-5-sonnet-20241022`
- `gpt-3.5-turbo` → `claude-3-5-haiku-20241022`

Puoi personalizzare i mapping:

```go
adapter.SetModelMapping(map[string]string{
    "my-model": anthropic.ModelClaude3Opus,
})
```

## Tool Calling (Function Calling)

```go
tools := []anthropic.Tool{
    {
        Name:        "get_weather",
        Description: "Get the current weather in a location",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "The city and state, e.g. San Francisco, CA"
                }
            },
            "required": ["location"]
        }`),
    },
}

req := &anthropic.MessagesRequest{
    Model:     anthropic.ModelClaude35Sonnet,
    MaxTokens: 1024,
    Tools:     tools,
    Messages: []anthropic.Message{
        {
            Role: anthropic.MessageRoleUser,
            Content: []anthropic.ContentBlock{
                anthropic.NewTextContentBlock("What's the weather in Paris?"),
            },
        },
    },
}

resp, err := client.CreateMessage(ctx, req)

// Controlla se Claude vuole usare una tool
if resp.HasToolUse() {
    toolUses := resp.GetToolUses()
    for _, toolUse := range toolUses {
        fmt.Printf("Tool: %s\n", toolUse.Name)
        fmt.Printf("Input: %s\n", string(toolUse.Input))
    }
}
```

## Modelli Supportati

```go
const (
    ModelClaude3Opus     = "claude-3-opus-20240229"
    ModelClaude3Sonnet   = "claude-3-sonnet-20240229"
    ModelClaude3Haiku    = "claude-3-haiku-20240307"
    ModelClaude35Sonnet  = "claude-3-5-sonnet-20241022"
    ModelClaude35Haiku   = "claude-3-5-haiku-20241022"
)
```

### Caratteristiche dei Modelli

| Modello | Context | Max Output | Uso Ideale |
|---------|---------|------------|------------|
| Claude 3.5 Sonnet | 200K | 8K | Bilanciato, versatile |
| Claude 3.5 Haiku | 200K | 8K | Veloce, economico |
| Claude 3 Opus | 200K | 4K | Massima qualità |
| Claude 3 Sonnet | 200K | 4K | Bilanciato legacy |
| Claude 3 Haiku | 200K | 4K | Veloce legacy |

## Configurazione Avanzata

### Custom HTTP Client

```go
import "net/http"

httpClient := &http.Client{
    Timeout: 300 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}

client := anthropic.NewClient(
    "your-api-key",
    anthropic.WithHTTPClient(httpClient),
    anthropic.WithAPIVersion("2023-06-01"),
    anthropic.WithUserAgent("MyApp/1.0"),
)
```

### Base URL Personalizzato

Per usare un proxy o endpoint custom:

```go
client := anthropic.NewClient(
    "your-api-key",
    anthropic.WithBaseURL("https://my-proxy.example.com"),
)
```

## Error Handling

```go
resp, err := client.CreateMessage(ctx, req)
if err != nil {
    if apiErr, ok := err.(*anthropic.Error); ok {
        switch {
        case apiErr.IsRateLimitError():
            // Gestisci rate limit
            fmt.Println("Rate limit exceeded")
        case apiErr.IsAuthError():
            // Gestisci errore di autenticazione
            fmt.Println("Invalid API key")
        case apiErr.IsRetryable():
            // Riprova
            fmt.Println("Retryable error, waiting...")
        default:
            fmt.Printf("API error: %s\n", apiErr.Message)
        }
    }
    return
}
```

### Tipi di Errori

- `invalid_request_error`: Richiesta malformata
- `authentication_error`: API key non valida
- `permission_error`: Accesso negato
- `not_found_error`: Risorsa non trovata
- `rate_limit_error`: Limite di rate superato
- `api_error`: Errore interno dell'API
- `overloaded_error`: API sovraccarica

## Rate Limiting

Il client estrae automaticamente le informazioni sui rate limit dagli header di risposta:

```go
// Gli header vengono parsati automaticamente
// anthropic-ratelimit-requests-limit
// anthropic-ratelimit-requests-remaining
// anthropic-ratelimit-requests-reset
// anthropic-ratelimit-tokens-limit
// anthropic-ratelimit-tokens-remaining
// anthropic-ratelimit-tokens-reset
// retry-after
```

## Health Check

```go
err := client.Health(context.Background())
if err != nil {
    fmt.Println("API is down")
} else {
    fmt.Println("API is healthy")
}
```

## Token Counting

Stima approssimativa dei token:

```go
tokenCount := client.CountTokens(req)
fmt.Printf("Estimated tokens: %d\n", tokenCount)
```

## Best Practices

1. **Riutilizza il client**: Crea un'unica istanza del client e riutilizzala
2. **Gestisci gli errori**: Sempre controllare errori e implementare retry logic
3. **Rate limiting**: Monitora i rate limit e implementa backoff
4. **Streaming per long-form**: Usa streaming per testi lunghi per migliorare UX
5. **System prompts**: Usa system prompts per istruzioni persistenti
6. **Context window**: Monitora la lunghezza del contesto (200K tokens max)
7. **Temperature**: Usa 0-0.5 per task deterministici, 0.7-1.0 per creatività

## Integrazione con GoLeapAI Gateway

Il provider Anthropic si integra automaticamente con il gateway GoLeapAI:

```go
// Il gateway usa l'adapter per accettare richieste OpenAI
// e inoltrarle a Claude in modo trasparente

// Richiesta OpenAI-compatible al gateway
POST /v1/chat/completions
{
    "model": "gpt-4",
    "messages": [...]
}

// Il gateway:
// 1. Riceve la richiesta OpenAI
// 2. Usa l'adapter per convertirla in formato Anthropic
// 3. Inoltra a Claude API
// 4. Converte la risposta in formato OpenAI
// 5. Restituisce al client
```

## Licenza

Parte del progetto GoLeapAI - MIT License
