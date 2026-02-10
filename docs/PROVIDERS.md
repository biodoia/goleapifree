# Provider Guide

Complete guide for working with AI providers in GoLeapAI.

## Table of Contents

- [Supported Providers](#supported-providers)
- [Adding Providers](#adding-providers)
- [Provider Configuration](#provider-configuration)
- [Provider Tiers](#provider-tiers)
- [Health Monitoring](#health-monitoring)
- [Troubleshooting](#troubleshooting)

## Supported Providers

GoLeapAI comes pre-configured with 150+ free and freemium AI providers.

### Tier 1 - Premium Free Providers

High-quality, officially supported free APIs.

#### OpenRouter Free

**Type:** Free
**Base URL:** `https://openrouter.ai/api/v1`
**Auth:** API Key
**Models:** 25+ including GPT-4, Claude 3.5, Llama 3, Gemini

**Features:**
- Streaming: Yes
- Tools/Functions: Yes
- JSON Mode: Yes
- Context: Up to 200K tokens

**Setup:**
```bash
# Get API key from https://openrouter.ai/keys
export OPENROUTER_API_KEY="sk-or-v1-..."

# Add to GoLeapAI
curl -X POST http://localhost:8080/admin/providers \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{
    "name": "OpenRouter Free",
    "type": "free",
    "base_url": "https://openrouter.ai/api/v1",
    "api_key": "'$OPENROUTER_API_KEY'"
  }'
```

#### Groq

**Type:** Freemium
**Base URL:** `https://api.groq.com/openai/v1`
**Auth:** API Key
**Models:** Llama 3 70B, Mixtral 8x7B, Gemma 7B

**Features:**
- Streaming: Yes
- Tools/Functions: Yes
- Speed: Ultra-fast (145ms avg)
- Free Tier: 14,400 requests/day

**Rate Limits:**
- 30 requests/minute
- 14,400 requests/day
- 50,000 tokens/minute

**Setup:**
```bash
# Get API key from https://console.groq.com/keys
export GROQ_API_KEY="gsk_..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Groq",
    "type": "freemium",
    "base_url": "https://api.groq.com/openai/v1",
    "api_key": "'$GROQ_API_KEY'",
    "tier": 1
  }'
```

#### Cerebras

**Type:** Freemium
**Base URL:** `https://api.cerebras.ai/v1`
**Auth:** API Key
**Models:** Llama 3 70B, Llama 3 8B

**Features:**
- Streaming: Yes
- Speed: Extremely fast (120ms avg)
- Free Tier: Generous limits

**Setup:**
```bash
export CEREBRAS_API_KEY="csk_..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Cerebras",
    "type": "freemium",
    "base_url": "https://api.cerebras.ai/v1",
    "api_key": "'$CEREBRAS_API_KEY'",
    "tier": 1
  }'
```

#### Google AI Studio

**Type:** Free
**Base URL:** `https://generativelanguage.googleapis.com/v1beta`
**Auth:** API Key
**Models:** Gemini Pro, Gemini Pro Vision

**Features:**
- Streaming: Yes
- Multimodal: Yes (vision)
- Context: 1M tokens
- Free Tier: 60 requests/minute

**Setup:**
```bash
# Get API key from https://makersuite.google.com/app/apikey
export GOOGLE_API_KEY="AIza..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Google AI Studio",
    "type": "free",
    "base_url": "https://generativelanguage.googleapis.com/v1beta",
    "api_key": "'$GOOGLE_API_KEY'",
    "tier": 1
  }'
```

#### GitHub Models

**Type:** Free
**Base URL:** `https://models.inference.ai.azure.com`
**Auth:** GitHub Personal Access Token
**Models:** GPT-4, GPT-3.5, Phi-3, Llama 3

**Features:**
- Streaming: Yes
- Free for developers
- GitHub integration

**Setup:**
```bash
# Get token from https://github.com/settings/tokens
export GITHUB_TOKEN="ghp_..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "GitHub Models",
    "type": "free",
    "base_url": "https://models.inference.ai.azure.com",
    "api_key": "'$GITHUB_TOKEN'",
    "tier": 1
  }'
```

#### Mistral La Plateforme

**Type:** Freemium
**Base URL:** `https://api.mistral.ai/v1`
**Auth:** API Key
**Models:** Mistral Large, Mistral Medium, Mistral Small

**Features:**
- Streaming: Yes
- Tools/Functions: Yes
- JSON Mode: Yes

**Setup:**
```bash
export MISTRAL_API_KEY="..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Mistral",
    "type": "freemium",
    "base_url": "https://api.mistral.ai/v1",
    "api_key": "'$MISTRAL_API_KEY'",
    "tier": 1
  }'
```

#### Cohere Free Tier

**Type:** Freemium
**Base URL:** `https://api.cohere.ai/v1`
**Auth:** Bearer Token
**Models:** Command, Command Light

**Features:**
- Streaming: Yes
- Embeddings: Yes
- Free Tier: 1000 calls/month

**Setup:**
```bash
export COHERE_API_KEY="..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Cohere",
    "type": "freemium",
    "base_url": "https://api.cohere.ai/v1",
    "api_key": "'$COHERE_API_KEY'",
    "auth_type": "bearer",
    "tier": 1
  }'
```

#### Cloudflare Workers AI

**Type:** Free
**Base URL:** `https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run`
**Auth:** Bearer Token
**Models:** Llama 2, CodeLlama, Mistral

**Features:**
- Serverless execution
- No rate limits on free tier
- Global edge network

**Setup:**
```bash
export CF_ACCOUNT_ID="..."
export CF_API_TOKEN="..."

curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "Cloudflare Workers AI",
    "type": "free",
    "base_url": "https://api.cloudflare.com/client/v4/accounts/'$CF_ACCOUNT_ID'/ai",
    "api_key": "'$CF_API_TOKEN'",
    "tier": 1
  }'
```

### Tier 2 - Community/Proxy Providers

Community-maintained free proxy services.

#### zukijourney

**Type:** Free
**Base URL:** `https://zukijourney.xyzbot.net/v1`
**Auth:** None
**Models:** Various GPT models

**Note:** No authentication required, community-provided.

```bash
# Pre-configured in GoLeapAI seed data
# Check status:
curl http://localhost:8080/admin/providers | jq '.providers[] | select(.name=="zukijourney")'
```

#### ElectronHub

**Type:** Free
**Base URL:** `https://api.electronhub.top/v1`
**Auth:** None
**Models:** GPT-3.5, GPT-4 variants

#### NagaAI

**Type:** Free
**Base URL:** `https://api.naga.ac/v1`
**Auth:** None
**Models:** Various models

### Local Providers

Run models locally without any API limits.

#### Ollama

**Type:** Local
**Base URL:** `http://localhost:11434/v1`
**Auth:** None
**Models:** Any model you download locally

**Features:**
- No rate limits
- Complete privacy
- GPU acceleration
- Custom models

**Setup:**

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull models
ollama pull llama3
ollama pull codellama
ollama pull mistral

# Ollama automatically configured in GoLeapAI
# Verify:
curl http://localhost:8080/admin/providers | jq '.providers[] | select(.name=="Ollama Local")'
```

**Recommended Models:**
```bash
ollama pull llama3:70b      # Best quality
ollama pull llama3:8b       # Fast, good quality
ollama pull codellama:34b   # Coding tasks
ollama pull mistral:7b      # Balanced
ollama pull phi3:mini       # Smallest, fastest
```

#### llama.cpp

**Type:** Local
**Base URL:** `http://localhost:8080` (llama.cpp server)

**Setup:**
```bash
# Clone and build
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make

# Run server
./server -m models/llama-2-7b.Q4_K_M.gguf -c 2048

# Add to GoLeapAI
curl -X POST http://localhost:8080/admin/providers \
  -d '{
    "name": "llama.cpp",
    "type": "local",
    "base_url": "http://localhost:8080",
    "tier": 1
  }'
```

## Adding Providers

### Via API

Add providers programmatically:

```bash
curl -X POST http://localhost:8080/admin/providers \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Custom Provider",
    "type": "free",
    "base_url": "https://api.example.com/v1",
    "auth_type": "api_key",
    "api_key": "your_api_key",
    "tier": 2,
    "supports_streaming": true,
    "supports_tools": false,
    "supports_json": true
  }'
```

### Via Configuration File

Edit `configs/providers.yaml`:

```yaml
providers:
  - name: "My Custom Provider"
    type: "free"
    base_url: "https://api.mycustomprovider.com/v1"
    auth_type: "api_key"
    api_key_env: "MY_PROVIDER_KEY"
    tier: 2
    capabilities:
      streaming: true
      tools: false
      json: true
```

Then reload:
```bash
./bin/goleapai reload-config
```

### Via Database Seed

Edit `pkg/database/seed.go` and add to `getFreeAPIProviders()`:

```go
{
    ID:          uuid.New(),
    Name:        "My Provider",
    Type:        models.ProviderTypeFree,
    Status:      models.ProviderStatusActive,
    BaseURL:     "https://api.myprovider.com/v1",
    AuthType:    models.AuthTypeAPIKey,
    Tier:        2,
    DiscoveredAt: now,
    Source:      "manual",
    SupportsStreaming: true,
    SupportsTools:     false,
    SupportsJSON:      true,
},
```

Rebuild and restart:
```bash
go build -o bin/goleapai cmd/backend/main.go
./bin/goleapai serve
```

## Provider Configuration

### Authentication Types

GoLeapAI supports multiple authentication methods:

#### API Key (Header)

Most common, used by OpenAI-compatible APIs:

```yaml
auth_type: "api_key"
```

Request format:
```
Authorization: Bearer <api_key>
```

#### Bearer Token

Used by some providers like Cohere:

```yaml
auth_type: "bearer"
```

Request format:
```
Authorization: Bearer <token>
```

#### OAuth2

For providers requiring OAuth flow:

```yaml
auth_type: "oauth2"
credentials:
  client_id: "..."
  client_secret: "..."
  refresh_token: "..."
```

#### None

For open proxies or local models:

```yaml
auth_type: "none"
```

### Rate Limits Configuration

Define rate limits for each provider:

```bash
curl -X POST http://localhost:8080/admin/providers/{provider_id}/limits \
  -d '{
    "limits": [
      {
        "type": "rpm",
        "value": 30
      },
      {
        "type": "rpd",
        "value": 14400
      },
      {
        "type": "tpm",
        "value": 50000
      }
    ]
  }'
```

Limit types:
- `rpm` - Requests per minute
- `rph` - Requests per hour
- `rpd` - Requests per day
- `tpm` - Tokens per minute
- `tpd` - Tokens per day
- `concurrent` - Concurrent requests

### Model Configuration

Add models for a provider:

```bash
curl -X POST http://localhost:8080/admin/providers/{provider_id}/models \
  -d '{
    "name": "llama-3-70b",
    "display_name": "Llama 3 70B",
    "modality": "chat",
    "context_length": 8192,
    "max_output_tokens": 4096,
    "input_price_per_1k": 0.0,
    "output_price_per_1k": 0.0,
    "capabilities": {
      "streaming": true,
      "tools": true,
      "json_mode": true,
      "vision": false
    },
    "tags": ["fast", "coding", "general"]
  }'
```

## Provider Tiers

Providers are organized into tiers for routing priority:

### Tier 1 - Premium

- Official free APIs
- High reliability
- Good performance
- Well-maintained

**Examples:** Groq, Google AI Studio, OpenRouter, Cerebras

**Routing Priority:** Highest

### Tier 2 - Standard

- Community providers
- Moderate reliability
- Variable performance

**Examples:** zukijourney, ElectronHub, NagaAI

**Routing Priority:** Medium

### Tier 3 - Experimental

- Newly discovered providers
- Unverified reliability
- Testing phase

**Routing Priority:** Lowest

### Configuring Tier

```bash
curl -X PUT http://localhost:8080/admin/providers/{provider_id} \
  -d '{"tier": 1}'
```

## Health Monitoring

GoLeapAI continuously monitors provider health.

### Health Check Process

Every 5 minutes (configurable):

1. HTTP request to provider's base URL
2. Measure response latency
3. Calculate health score
4. Update provider status

### Health Score Calculation

```
health_score = (success_rate * 0.4) +
               (latency_score * 0.3) +
               (uptime_score * 0.3)
```

Where:
- `success_rate` = successful_requests / total_requests (last 24h)
- `latency_score` = 1.0 - (avg_latency / 5000ms)
- `uptime_score` = successful_checks / total_checks (last 24h)

### Provider Status

- **Active** - Healthy and available (health_score > 0.5)
- **Down** - Failing health checks (health_score < 0.3)
- **Maintenance** - Manually disabled
- **Deprecated** - Scheduled for removal

### Manual Health Check

```bash
# Trigger immediate health check for all providers
curl -X POST http://localhost:8080/admin/health-check

# Check specific provider
curl -X POST http://localhost:8080/admin/providers/{provider_id}/health-check
```

### View Health Status

```bash
curl http://localhost:8080/admin/providers/{provider_id}/health
```

Response:
```json
{
  "provider": "Groq",
  "status": "active",
  "health_score": 0.95,
  "avg_latency_ms": 145,
  "last_check": "2026-02-05T10:30:00Z",
  "uptime_24h": 0.98,
  "success_rate_24h": 0.985,
  "last_error": null
}
```

## Troubleshooting

### Provider Not Available

**Problem:** Provider shows as "down" in dashboard

**Solutions:**

1. Check health status:
```bash
curl http://localhost:8080/admin/providers | jq '.providers[] | select(.status=="down")'
```

2. Manual health check:
```bash
curl -X POST http://localhost:8080/admin/providers/{provider_id}/health-check
```

3. Check API key validity:
```bash
# Test directly
curl https://api.groq.com/openai/v1/models \
  -H "Authorization: Bearer $GROQ_API_KEY"
```

4. Update API key:
```bash
curl -X PUT http://localhost:8080/admin/providers/{provider_id} \
  -d '{"api_key": "new_key_here"}'
```

### Quota Exhausted

**Problem:** "quota_exhausted" errors in logs

**Solutions:**

1. Check quota usage:
```bash
curl http://localhost:8080/admin/providers/{provider_id}/quota
```

2. Add multiple accounts:
```bash
curl -X POST http://localhost:8080/admin/providers/{provider_id}/accounts \
  -d '{
    "api_key": "second_api_key",
    "quota_limit": 14400
  }'
```

3. Enable auto-failover (should be default):
```yaml
routing:
  failover_enabled: true
  max_retries: 3
```

### Slow Response Times

**Problem:** High latency from provider

**Solutions:**

1. Check provider stats:
```bash
curl http://localhost:8080/admin/providers/{provider_id}/stats
```

2. Switch routing strategy to latency-first:
```yaml
routing:
  strategy: "latency_first"
```

3. Lower provider tier:
```bash
curl -X PUT http://localhost:8080/admin/providers/{provider_id} \
  -d '{"tier": 3}'
```

### Authentication Errors

**Problem:** 401/403 errors from provider

**Solutions:**

1. Verify API key format:
```bash
# OpenAI format
Authorization: Bearer sk-...

# Anthropic format
x-api-key: sk-ant-...

# Custom
X-API-Key: ...
```

2. Check auth type:
```bash
curl http://localhost:8080/admin/providers/{provider_id} | jq '.auth_type'
```

3. Update auth configuration:
```bash
curl -X PUT http://localhost:8080/admin/providers/{provider_id} \
  -d '{
    "auth_type": "bearer",
    "api_key": "new_key"
  }'
```

### Model Not Found

**Problem:** "model not found" errors

**Solutions:**

1. List available models:
```bash
curl http://localhost:8080/v1/models
```

2. Check provider's models:
```bash
curl http://localhost:8080/admin/providers/{provider_id}/models
```

3. Sync models from provider:
```bash
curl -X POST http://localhost:8080/admin/providers/{provider_id}/sync-models
```

### Provider Discovery

**Problem:** New provider not auto-discovered

**Solutions:**

1. Enable auto-discovery:
```yaml
providers:
  auto_discovery: true
```

2. Manual provider scan:
```bash
curl -X POST http://localhost:8080/admin/discover-providers
```

3. Add manually (see [Adding Providers](#adding-providers))

## Best Practices

### Multiple Accounts

For high-volume usage, configure multiple accounts per provider:

```bash
# Add additional accounts
for i in 1 2 3; do
  curl -X POST http://localhost:8080/admin/providers/{provider_id}/accounts \
    -d "{\"api_key\": \"key_$i\"}"
done
```

GoLeapAI will automatically rotate between accounts.

### Monitoring

Set up monitoring alerts:

```yaml
monitoring:
  alerts:
    - type: "provider_down"
      threshold: 3  # alert after 3 failed checks
      webhook: "https://hooks.slack.com/..."

    - type: "quota_low"
      threshold: 0.9  # alert at 90% quota
      webhook: "https://hooks.slack.com/..."
```

### Backup Providers

Always configure backup providers for critical models:

```yaml
# Ensure multiple providers for GPT-4
providers:
  - name: "OpenRouter"
    models: ["gpt-4"]
  - name: "GitHub Models"
    models: ["gpt-4"]
```

### Testing New Providers

Test new providers before production:

```bash
# Add as tier 3
curl -X POST http://localhost:8080/admin/providers \
  -d '{"name": "New Provider", "tier": 3}'

# Monitor for 24h
sleep 86400

# Check stats
curl http://localhost:8080/admin/providers/{provider_id}/stats

# Promote if stable
curl -X PUT http://localhost:8080/admin/providers/{provider_id} \
  -d '{"tier": 2}'
```

## Contributing Providers

Found a new free API? Contribute to GoLeapAI!

1. Test the provider:
```bash
curl https://new-provider.com/v1/chat/completions \
  -H "Authorization: Bearer test_key" \
  -d '{"model":"test","messages":[{"role":"user","content":"Hi"}]}'
```

2. Create provider config:
```yaml
name: "New Provider"
type: "free"
base_url: "https://new-provider.com/v1"
auth_type: "api_key"
tier: 3
supports_streaming: true
supports_tools: false
```

3. Submit PR to https://github.com/biodoia/goleapifree

Include:
- Provider name and URL
- Authentication method
- Available models
- Rate limits
- Test results

## Conclusion

GoLeapAI's provider system is designed for flexibility and reliability. With 150+ pre-configured providers and easy addition of new ones, you have access to the entire free AI ecosystem through a single, intelligent gateway.
