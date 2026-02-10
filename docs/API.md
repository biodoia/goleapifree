# GoLeapAI API Reference

Complete API documentation for GoLeapAI Gateway.

## Table of Contents

- [Authentication](#authentication)
- [OpenAI Compatible API](#openai-compatible-api)
- [Anthropic Compatible API](#anthropic-compatible-api)
- [Admin API](#admin-api)
- [Health & Monitoring](#health--monitoring)
- [Error Codes](#error-codes)
- [Rate Limits](#rate-limits)

## Base URL

```
http://localhost:8080
```

For production:
```
https://api.goleapai.io
```

## Authentication

GoLeapAI uses API key authentication compatible with both OpenAI and Anthropic standards.

### OpenAI Style

```bash
Authorization: Bearer <your-api-key>
```

### Anthropic Style

```bash
x-api-key: <your-api-key>
```

### Getting an API Key

```bash
# Create a new API key via admin endpoint
curl -X POST http://localhost:8080/admin/api-keys \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Application",
    "expires_in_days": 90
  }'
```

Response:
```json
{
  "api_key": "gla_1234567890abcdef",
  "name": "My Application",
  "created_at": "2026-02-05T10:00:00Z",
  "expires_at": "2026-05-06T10:00:00Z"
}
```

## OpenAI Compatible API

GoLeapAI implements the OpenAI API specification for maximum compatibility.

### Chat Completions

Create a chat completion using any available model.

**Endpoint:** `POST /v1/chat/completions`

**Headers:**
```
Authorization: Bearer <api-key>
Content-Type: application/json
```

**Request Body:**

```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Explain quantum computing in simple terms."
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "stream": false
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| model | string | Yes | Model to use (e.g., "gpt-4", "claude-3-5-sonnet") |
| messages | array | Yes | Array of message objects |
| temperature | float | No | Sampling temperature (0.0-2.0, default: 1.0) |
| max_tokens | integer | No | Maximum tokens to generate |
| top_p | float | No | Nucleus sampling threshold |
| frequency_penalty | float | No | Penalize repeated tokens (-2.0 to 2.0) |
| presence_penalty | float | No | Penalize existing tokens (-2.0 to 2.0) |
| stop | array/string | No | Stop sequences |
| stream | boolean | No | Enable streaming (default: false) |
| n | integer | No | Number of completions to generate |
| user | string | No | Unique user identifier |

**Response (Non-Streaming):**

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1738747200,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing is a revolutionary approach..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 150,
    "total_tokens": 175
  },
  "x_goleapai": {
    "provider": "groq",
    "latency_ms": 245,
    "cost_saved": 0.0035
  }
}
```

**Response (Streaming):**

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1738747200,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1738747200,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Quantum"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1738747200,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" computing"},"finish_reason":null}]}

...

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1738747200,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

**cURL Example:**

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer gla_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Write a haiku about AI"}
    ],
    "temperature": 0.8
  }'
```

**Python Example:**

```python
import openai

client = openai.OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="gla_your_api_key"
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(response.choices[0].message.content)
```

**JavaScript Example:**

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/v1',
  apiKey: 'gla_your_api_key'
});

const response = await client.chat.completions.create({
  model: 'gpt-4',
  messages: [
    { role: 'user', content: 'Explain quantum computing' }
  ]
});

console.log(response.choices[0].message.content);
```

### List Models

Get a list of all available models.

**Endpoint:** `GET /v1/models`

**Headers:**
```
Authorization: Bearer <api-key>
```

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1738747200,
      "owned_by": "openrouter",
      "permission": [],
      "root": "gpt-4",
      "parent": null,
      "x_goleapai": {
        "providers": ["openrouter", "groq"],
        "pricing": {
          "input_per_1k": 0.0,
          "output_per_1k": 0.0
        },
        "context_length": 8192,
        "modality": "chat"
      }
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model",
      "created": 1738747200,
      "owned_by": "anthropic",
      "x_goleapai": {
        "providers": ["openrouter"],
        "pricing": {
          "input_per_1k": 0.0,
          "output_per_1k": 0.0
        },
        "context_length": 200000,
        "modality": "chat"
      }
    }
  ]
}
```

**cURL Example:**

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer gla_your_api_key"
```

## Anthropic Compatible API

GoLeapAI supports Anthropic's Messages API for Claude models.

### Create Message

Create a message with Claude models.

**Endpoint:** `POST /v1/messages`

**Headers:**
```
x-api-key: <api-key>
anthropic-version: 2023-06-01
Content-Type: application/json
```

**Request Body:**

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "Explain quantum computing in simple terms."
    }
  ],
  "temperature": 0.7,
  "system": "You are a helpful AI assistant."
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| model | string | Yes | Claude model to use |
| messages | array | Yes | Array of message objects |
| max_tokens | integer | Yes | Maximum tokens to generate |
| temperature | float | No | Sampling temperature (0.0-1.0) |
| top_p | float | No | Nucleus sampling threshold |
| top_k | integer | No | Top-k sampling parameter |
| system | string | No | System prompt |
| stop_sequences | array | No | Stop sequences |
| stream | boolean | No | Enable streaming |
| metadata | object | No | Request metadata |

**Response:**

```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Quantum computing is a revolutionary approach to computation..."
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 25,
    "output_tokens": 150
  }
}
```

**cURL Example:**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: gla_your_api_key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Write a haiku about AI"}
    ]
  }'
```

**Python Example (Anthropic SDK):**

```python
from anthropic import Anthropic

client = Anthropic(
    base_url="http://localhost:8080/v1",
    api_key="gla_your_api_key"
)

message = client.messages.create(
    model="claude-3-5-sonnet-20241022",
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(message.content[0].text)
```

## Admin API

Administrative endpoints for managing the gateway.

### List Providers

Get all configured providers with their status.

**Endpoint:** `GET /admin/providers`

**Headers:**
```
Authorization: Bearer <admin-api-key>
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| type | string | Filter by type (free, freemium, paid, local) |
| status | string | Filter by status (active, down, maintenance) |
| tier | integer | Filter by tier (1, 2, 3) |

**Response:**

```json
{
  "providers": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Groq",
      "type": "freemium",
      "status": "active",
      "base_url": "https://api.groq.com/openai/v1",
      "tier": 1,
      "health_score": 0.95,
      "avg_latency_ms": 145,
      "last_health_check": "2026-02-05T10:30:00Z",
      "supports_streaming": true,
      "supports_tools": true,
      "models_count": 5
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "OpenRouter Free",
      "type": "free",
      "status": "active",
      "base_url": "https://openrouter.ai/api/v1",
      "tier": 1,
      "health_score": 0.92,
      "avg_latency_ms": 180,
      "last_health_check": "2026-02-05T10:30:00Z",
      "supports_streaming": true,
      "supports_tools": true,
      "models_count": 25
    }
  ],
  "total": 12,
  "active": 10,
  "down": 1,
  "maintenance": 1
}
```

**cURL Example:**

```bash
curl http://localhost:8080/admin/providers?type=free&status=active \
  -H "Authorization: Bearer gla_admin_key"
```

### Get Provider Stats

Get detailed statistics for a specific provider.

**Endpoint:** `GET /admin/providers/{provider_id}/stats`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| period | string | Time period (hour, day, week, month) |
| start_date | string | ISO 8601 start date |
| end_date | string | ISO 8601 end date |

**Response:**

```json
{
  "provider_id": "550e8400-e29b-41d4-a716-446655440000",
  "provider_name": "Groq",
  "period": "day",
  "stats": {
    "total_requests": 1234,
    "successful_requests": 1215,
    "failed_requests": 19,
    "success_rate": 0.985,
    "avg_latency_ms": 145,
    "p50_latency_ms": 120,
    "p95_latency_ms": 250,
    "p99_latency_ms": 380,
    "total_tokens": 125000,
    "total_cost_saved": 12.45,
    "quota_exhausted_count": 3,
    "timeout_count": 2,
    "error_distribution": {
      "429": 3,
      "500": 2,
      "timeout": 2
    }
  },
  "hourly_breakdown": [
    {
      "hour": "2026-02-05T00:00:00Z",
      "requests": 45,
      "success_rate": 0.98,
      "avg_latency_ms": 140
    }
  ]
}
```

### Gateway Statistics

Get overall gateway statistics.

**Endpoint:** `GET /admin/stats`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| period | string | Time period (hour, day, week, month) |

**Response:**

```json
{
  "period": "day",
  "timestamp": "2026-02-05T10:00:00Z",
  "overall": {
    "total_requests": 15234,
    "successful_requests": 14998,
    "failed_requests": 236,
    "success_rate": 0.985,
    "avg_latency_ms": 165,
    "total_tokens": 1850000,
    "total_cost_saved": 185.50
  },
  "providers": {
    "active": 10,
    "down": 1,
    "maintenance": 1
  },
  "top_providers": [
    {
      "name": "Groq",
      "requests": 5234,
      "success_rate": 0.98
    },
    {
      "name": "OpenRouter",
      "requests": 4123,
      "success_rate": 0.96
    }
  ],
  "top_models": [
    {
      "name": "gpt-4",
      "requests": 3456,
      "avg_latency_ms": 180
    },
    {
      "name": "llama-3-70b",
      "requests": 2987,
      "avg_latency_ms": 145
    }
  ]
}
```

**cURL Example:**

```bash
curl http://localhost:8080/admin/stats?period=day \
  -H "Authorization: Bearer gla_admin_key"
```

### Add Provider

Add a new provider to the system.

**Endpoint:** `POST /admin/providers`

**Request Body:**

```json
{
  "name": "Custom Provider",
  "type": "free",
  "base_url": "https://api.customprovider.com/v1",
  "auth_type": "api_key",
  "tier": 2,
  "supports_streaming": true,
  "supports_tools": false,
  "supports_json": true,
  "api_key": "provider_api_key_123"
}
```

**Response:**

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "name": "Custom Provider",
  "status": "active",
  "created_at": "2026-02-05T10:00:00Z"
}
```

### Update Provider

Update provider configuration.

**Endpoint:** `PUT /admin/providers/{provider_id}`

**Request Body:**

```json
{
  "status": "maintenance",
  "tier": 1,
  "api_key": "new_api_key"
}
```

### Delete Provider

Remove a provider from the system.

**Endpoint:** `DELETE /admin/providers/{provider_id}`

**Response:**

```json
{
  "success": true,
  "message": "Provider deleted successfully"
}
```

## Health & Monitoring

### Health Check

Basic health check endpoint.

**Endpoint:** `GET /health`

**Response:**

```json
{
  "status": "healthy",
  "timestamp": 1738747200,
  "version": "1.0.0"
}
```

### Readiness Check

Check if the service is ready to accept requests.

**Endpoint:** `GET /ready`

**Response (Ready):**

```json
{
  "ready": true,
  "timestamp": 1738747200,
  "database": "connected",
  "providers_available": 10
}
```

**Response (Not Ready):**

```json
{
  "ready": false,
  "error": "database connection failed",
  "timestamp": 1738747200
}
```

### Prometheus Metrics

Metrics endpoint for Prometheus scraping.

**Endpoint:** `GET /metrics`

**Response (Prometheus format):**

```
# HELP goleapai_requests_total Total number of requests
# TYPE goleapai_requests_total counter
goleapai_requests_total{provider="groq",model="llama-3-70b",status="success"} 1234

# HELP goleapai_request_duration_seconds Request duration in seconds
# TYPE goleapai_request_duration_seconds histogram
goleapai_request_duration_seconds_bucket{provider="groq",le="0.1"} 234
goleapai_request_duration_seconds_bucket{provider="groq",le="0.5"} 987
goleapai_request_duration_seconds_bucket{provider="groq",le="1.0"} 1200
goleapai_request_duration_seconds_sum{provider="groq"} 145.6
goleapai_request_duration_seconds_count{provider="groq"} 1234

# HELP goleapai_provider_health_score Provider health score
# TYPE goleapai_provider_health_score gauge
goleapai_provider_health_score{provider="groq"} 0.95
goleapai_provider_health_score{provider="openrouter"} 0.92

# HELP goleapai_cost_saved_total Total cost saved vs official APIs
# TYPE goleapai_cost_saved_total counter
goleapai_cost_saved_total{provider="groq"} 45.67
goleapai_cost_saved_total{provider="openrouter"} 32.12
```

## Error Codes

GoLeapAI uses standard HTTP status codes and provides detailed error messages.

### Standard Errors

| Status Code | Error Type | Description |
|-------------|------------|-------------|
| 400 | Bad Request | Invalid request format or parameters |
| 401 | Unauthorized | Missing or invalid API key |
| 403 | Forbidden | API key doesn't have required permissions |
| 404 | Not Found | Resource or endpoint not found |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server-side error |
| 502 | Bad Gateway | Provider returned invalid response |
| 503 | Service Unavailable | No providers available |
| 504 | Gateway Timeout | Provider request timed out |

### Error Response Format

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "The model 'gpt-5' does not exist",
    "code": "model_not_found",
    "param": "model"
  },
  "x_goleapai": {
    "request_id": "req_abc123",
    "timestamp": 1738747200
  }
}
```

### Provider-Specific Errors

**No Available Provider:**
```json
{
  "error": {
    "type": "service_unavailable",
    "message": "No providers available for model 'gpt-4'",
    "code": "no_provider_available",
    "details": {
      "requested_model": "gpt-4",
      "checked_providers": 3,
      "failure_reasons": {
        "groq": "quota_exhausted",
        "openrouter": "health_check_failed",
        "cerebras": "model_not_supported"
      }
    }
  }
}
```

**Quota Exhausted:**
```json
{
  "error": {
    "type": "quota_exceeded",
    "message": "Provider quota exhausted, trying alternate provider",
    "code": "quota_exhausted",
    "retry_after": 3600
  }
}
```

## Rate Limits

GoLeapAI implements rate limiting at multiple levels.

### Global Rate Limits

Default limits per API key:

- **Free Tier:** 60 requests/minute, 1000 requests/day
- **Pro Tier:** 600 requests/minute, 100000 requests/day
- **Enterprise:** Custom limits

### Rate Limit Headers

All responses include rate limit information:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1738747260
X-RateLimit-Period: minute
```

### Rate Limit Response

When rate limit is exceeded:

**Status:** 429 Too Many Requests

```json
{
  "error": {
    "type": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Try again in 15 seconds.",
    "code": "rate_limit",
    "retry_after": 15
  }
}
```

### Provider Rate Limits

Each provider has its own rate limits tracked separately:

```bash
# Check provider limits
curl http://localhost:8080/admin/providers/{provider_id}/limits \
  -H "Authorization: Bearer gla_admin_key"
```

Response:
```json
{
  "provider": "groq",
  "limits": [
    {
      "type": "rpm",
      "limit": 30,
      "current": 12,
      "reset_at": "2026-02-05T10:01:00Z"
    },
    {
      "type": "rpd",
      "limit": 14400,
      "current": 3456,
      "reset_at": "2026-02-06T00:00:00Z"
    },
    {
      "type": "tpm",
      "limit": 50000,
      "current": 12345,
      "reset_at": "2026-02-05T10:01:00Z"
    }
  ]
}
```

## WebSocket API (Planned)

Real-time updates via WebSocket connection.

**Endpoint:** `ws://localhost:8080/v1/realtime`

**Coming Soon** - Real-time streaming, live stats, and provider status updates.

## SDK Examples

### Go

```go
package main

import (
    "context"
    "fmt"
    "github.com/biodoia/goleapifree/pkg/client"
)

func main() {
    client := client.New("http://localhost:8080", "gla_your_api_key")

    resp, err := client.Chat(context.Background(), &client.ChatRequest{
        Model: "gpt-4",
        Messages: []client.Message{
            {Role: "user", Content: "Hello!"},
        },
    })

    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### cURL Cheat Sheet

```bash
# Chat completion
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}]}'

# List models
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer $API_KEY"

# Provider stats
curl http://localhost:8080/admin/stats \
  -H "Authorization: Bearer $ADMIN_KEY"

# Health check
curl http://localhost:8080/health
```

## Conclusion

The GoLeapAI API provides a powerful, OpenAI-compatible interface to 150+ free AI providers with intelligent routing, automatic failover, and comprehensive monitoring. Use the standard OpenAI or Anthropic SDKs by simply changing the base URL!
