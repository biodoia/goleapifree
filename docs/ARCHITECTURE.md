# GoLeapAI Architecture

## Overview

GoLeapAI is an intelligent LLM gateway that aggregates 150+ free and freemium AI APIs, providing a unified interface compatible with OpenAI and Anthropic standards. The system employs intelligent routing, automatic failover, and comprehensive monitoring to democratize access to AI.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                              │
│  ┌──────────────┬──────────────┬──────────────────────────────┐ │
│  │ CLI/TUI      │  Web UI      │  External Applications       │ │
│  │ (Bubble Tea) │  (HTMX)      │  (OpenAI SDK, Anthropic SDK) │ │
│  └──────────────┴──────────────┴──────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      API GATEWAY LAYER                           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Fiber v3 HTTP Server (HTTP/3 + TLS)                      │   │
│  │  • OpenAI Compatible Endpoints (/v1/chat/completions)    │   │
│  │  • Anthropic Compatible Endpoints (/v1/messages)         │   │
│  │  • Admin Endpoints (/admin/*)                            │   │
│  │  • Health & Metrics (/health, /metrics)                  │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                   INTELLIGENT ROUTING ENGINE                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Router (context-aware provider selection)                │   │
│  │  • Cost-Optimized Strategy                               │   │
│  │  • Latency-First Strategy                                │   │
│  │  • Quality-First Strategy                                │   │
│  │  • Auto-Failover & Retry Logic                           │   │
│  │  • Load Balancing                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                  PROVIDER MANAGEMENT LAYER                       │
│  ┌─────────────┬─────────────┬──────────────────────────────┐  │
│  │ Free APIs   │  Paid APIs  │  Local Models                │  │
│  │ • OpenRouter│  • OpenAI   │  • Ollama                    │  │
│  │ • Groq      │  • Anthropic│  • llama.cpp                 │  │
│  │ • Cerebras  │  • Mistral  │  • vLLM                      │  │
│  │ • Google AI │  • Cohere   │  • LocalAI                   │  │
│  │ • GitHub    │             │                              │  │
│  │ • 150+      │             │                              │  │
│  └─────────────┴─────────────┴──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                  MONITORING & HEALTH LAYER                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Health Monitor (periodic checks every 5m)                │   │
│  │  • Provider availability checking                        │   │
│  │  • Latency measurement                                   │   │
│  │  • Quota tracking                                        │   │
│  │  • Health score calculation                              │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    STORAGE & CACHING LAYER                       │
│  ┌──────────────┬───────────┬───────────┬──────────────────┐   │
│  │ SQLite/      │  Redis    │ Vector DB │  Prometheus      │   │
│  │ PostgreSQL   │  Cache    │ (planned) │  Metrics         │   │
│  │              │           │           │                  │   │
│  │ • Providers  │ • Sessions│ • Embeddings│• Request counts│   │
│  │ • Models     │ • Quotes  │ • Semantic │• Latencies     │   │
│  │ • Accounts   │ • Rate    │   search  │• Error rates   │   │
│  │ • Stats      │   limits  │           │• Provider stats│   │
│  │ • Logs       │           │           │                  │   │
│  └──────────────┴───────────┴───────────┴──────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### 1. Gateway Core (`internal/gateway`)

The gateway is the main HTTP server built on Fiber v3, handling all incoming requests.

**Responsibilities:**
- HTTP request handling
- Authentication & authorization
- Request routing to appropriate handlers
- Response formatting
- Graceful shutdown

**Key Components:**
```go
type Gateway struct {
    config *config.Config      // Application configuration
    db     *database.DB        // Database connection
    app    *fiber.App         // Fiber HTTP server
    router *router.Router     // Intelligent router
    health *health.Monitor    // Health monitoring
}
```

### 2. Intelligent Router (`internal/router`)

The router implements smart provider selection based on configurable strategies.

**Routing Strategies:**

1. **Cost-Optimized** (default)
   - Prioritizes free providers
   - Considers pricing tiers
   - Minimizes API costs

2. **Latency-First**
   - Selects fastest providers
   - Uses historical latency data
   - Optimizes for response time

3. **Quality-First**
   - Prioritizes higher-tier providers
   - Uses quality scores
   - Balances quality vs availability

**Selection Algorithm:**
```
1. Filter available providers (healthy, quota available)
2. Apply routing strategy scoring
3. Select highest-scoring provider
4. If fails, retry with next provider (failover)
5. Log selection decision and metrics
```

### 3. Health Monitor (`internal/health`)

Continuous monitoring of all providers to maintain system reliability.

**Features:**
- Periodic health checks (configurable interval, default 5m)
- Latency measurement
- Health score calculation (0.0-1.0)
- Status updates (active, down, maintenance)
- Concurrent checking with goroutines

**Health Score Calculation:**
```
score = (success_rate * 0.4) +
        (latency_score * 0.3) +
        (uptime_score * 0.3)
```

### 4. Database Layer (`pkg/database`)

GORM-based abstraction supporting both SQLite and PostgreSQL.

**Supported Databases:**
- **SQLite** - Default, zero-config, local development
- **PostgreSQL** - Production, high-performance, scalable

**Features:**
- Auto-migrations
- Database seeding with 150+ free APIs
- Connection pooling
- Query optimization

### 5. Provider Management (`pkg/models`)

Data models representing the provider ecosystem.

**Key Models:**

- **Provider** - LLM API provider
- **Model** - Available AI models
- **Account** - User accounts for providers
- **RateLimit** - Rate limiting rules
- **ProviderStats** - Aggregated statistics
- **RequestLog** - Individual request logs

## Data Flow

### Chat Completion Request Flow

```
┌─────────┐
│ Client  │
└────┬────┘
     │ POST /v1/chat/completions
     ↓
┌────────────────┐
│ Gateway        │
│ • Auth check   │
│ • Parse request│
└────┬───────────┘
     │
     ↓
┌────────────────┐
│ Router         │
│ • Analyze req  │
│ • Select       │
│   provider     │
└────┬───────────┘
     │
     ↓
┌────────────────┐
│ Provider       │
│ • Check quota  │
│ • Make request │
└────┬───────────┘
     │
     ↓
┌────────────────┐
│ Response       │
│ • Transform    │
│ • Log metrics  │
│ • Return       │
└────┬───────────┘
     │
     ↓
┌─────────┐
│ Client  │
└─────────┘
```

### Health Check Flow

```
┌─────────────┐
│ Ticker (5m) │
└──────┬──────┘
       │
       ↓
┌──────────────────┐
│ Monitor          │
│ • Get all active │
│   providers      │
└──────┬───────────┘
       │
       ↓
┌──────────────────┐
│ For each provider│
│ (concurrent)     │
│ • HTTP check     │
│ • Measure latency│
│ • Update score   │
└──────┬───────────┘
       │
       ↓
┌──────────────────┐
│ Database         │
│ • Update metrics │
│ • Set status     │
└──────────────────┘
```

## Database Schema

### Providers Table

```sql
CREATE TABLE providers (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    type VARCHAR(50) NOT NULL,  -- 'free', 'freemium', 'paid', 'local'
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    base_url VARCHAR(512) NOT NULL,
    auth_type VARCHAR(50) NOT NULL DEFAULT 'api_key',
    tier INTEGER NOT NULL DEFAULT 3,  -- 1=premium, 2=standard, 3=experimental

    -- Discovery
    discovered_at TIMESTAMP NOT NULL,
    last_verified TIMESTAMP,
    source VARCHAR(100),

    -- Capabilities
    supports_streaming BOOLEAN DEFAULT true,
    supports_tools BOOLEAN DEFAULT false,
    supports_json BOOLEAN DEFAULT true,

    -- Health
    last_health_check TIMESTAMP,
    health_score FLOAT DEFAULT 1.0,
    avg_latency_ms INTEGER,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_providers_status ON providers(status);
CREATE INDEX idx_providers_type ON providers(type);
CREATE INDEX idx_providers_tier ON providers(tier);
```

### Models Table

```sql
CREATE TABLE models (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES providers(id),
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    modality VARCHAR(50) NOT NULL,  -- 'chat', 'completion', 'embedding', etc.

    -- Specifications
    context_length INTEGER,
    max_output_tokens INTEGER,
    input_price_per_1k FLOAT DEFAULT 0.0,
    output_price_per_1k FLOAT DEFAULT 0.0,

    -- Capabilities (JSONB)
    capabilities JSONB,

    -- Quality
    quality_score FLOAT DEFAULT 0.5,
    speed_score FLOAT DEFAULT 0.5,

    -- Metadata
    description TEXT,
    tags JSONB,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_models_provider_id ON models(provider_id);
CREATE INDEX idx_models_name ON models(name);
CREATE INDEX idx_models_modality ON models(modality);
```

### Accounts Table

```sql
CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    provider_id UUID NOT NULL REFERENCES providers(id),

    -- Credentials (encrypted JSONB)
    credentials JSONB NOT NULL,

    -- Quota
    quota_used BIGINT DEFAULT 0,
    quota_limit BIGINT,
    last_reset TIMESTAMP,

    -- Status
    active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_accounts_user_id ON accounts(user_id);
CREATE INDEX idx_accounts_provider_id ON accounts(provider_id);
```

### Rate Limits Table

```sql
CREATE TABLE rate_limits (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES providers(id),

    limit_type VARCHAR(50) NOT NULL,  -- 'rpm', 'rph', 'rpd', 'tpm', 'tpd', 'concurrent'
    limit_value INTEGER NOT NULL,
    reset_interval INTEGER,  -- seconds (0 for daily)

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_rate_limits_provider_id ON rate_limits(provider_id);
```

### Provider Stats Table

```sql
CREATE TABLE provider_stats (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES providers(id),
    timestamp TIMESTAMP NOT NULL,

    -- Metrics
    success_rate FLOAT,
    avg_latency_ms INTEGER,
    total_requests BIGINT,
    total_tokens BIGINT,
    cost_saved FLOAT,

    -- Errors
    error_count BIGINT,
    timeout_count BIGINT,
    quota_exhausted BIGINT,

    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_provider_stats_provider_id ON provider_stats(provider_id);
CREATE INDEX idx_provider_stats_timestamp ON provider_stats(timestamp);
```

### Request Logs Table

```sql
CREATE TABLE request_logs (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES providers(id),
    model_id UUID REFERENCES models(id),
    user_id UUID,

    -- Request
    method VARCHAR(10),
    endpoint VARCHAR(512),
    status_code INTEGER,
    latency_ms INTEGER,
    input_tokens INTEGER,
    output_tokens INTEGER,

    -- Status
    success BOOLEAN,
    error_message TEXT,

    -- Cost
    estimated_cost FLOAT,

    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_request_logs_provider_id ON request_logs(provider_id);
CREATE INDEX idx_request_logs_user_id ON request_logs(user_id);
CREATE INDEX idx_request_logs_timestamp ON request_logs(timestamp);
```

## Deployment Options

### 1. Standalone Binary

Single binary deployment with embedded SQLite.

```
┌─────────────────┐
│  goleapai       │
│  (standalone)   │
│                 │
│  • HTTP server  │
│  • SQLite DB    │
│  • All features │
└─────────────────┘
```

**Pros:**
- Simple deployment
- Zero dependencies
- Fast startup

**Cons:**
- Limited scalability
- Single point of failure

### 2. Distributed Architecture

Multiple instances with shared PostgreSQL.

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│ goleapai #1 │   │ goleapai #2 │   │ goleapai #3 │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └─────────────────┴─────────────────┘
                         │
                    ┌────┴────┐
                    │PostgreSQL│
                    └────┬────┘
                         │
                    ┌────┴────┐
                    │  Redis  │
                    └─────────┘
```

**Pros:**
- High availability
- Horizontal scaling
- Load distribution

**Cons:**
- Complex deployment
- Requires infrastructure

### 3. Kubernetes Deployment

Cloud-native deployment with auto-scaling.

```
┌─────────────────────────────────────┐
│         Kubernetes Cluster          │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  Ingress / Load Balancer     │  │
│  └────────────┬─────────────────┘  │
│               │                     │
│  ┌────────────┴─────────────────┐  │
│  │  goleapai Deployment         │  │
│  │  (3 replicas, auto-scale)    │  │
│  └────────────┬─────────────────┘  │
│               │                     │
│  ┌────────────┴─────────────────┐  │
│  │  PostgreSQL StatefulSet      │  │
│  └──────────────────────────────┘  │
│               │                     │
│  ┌────────────┴─────────────────┐  │
│  │  Redis StatefulSet           │  │
│  └──────────────────────────────┘  │
│               │                     │
│  ┌────────────┴─────────────────┐  │
│  │  Prometheus Monitoring       │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
```

## Configuration Architecture

Configuration is managed through a hierarchical system:

1. **Default values** - Hardcoded in `pkg/config/config.go`
2. **YAML file** - `configs/config.yaml` or custom path
3. **Environment variables** - Override any setting

**Priority:** ENV VARS > YAML FILE > DEFAULTS

## Security Architecture

### Authentication Flow

```
Request → API Key Validation → Rate Limit Check → Provider Selection
```

### Data Protection

- **Credentials encryption** - Provider API keys encrypted at rest
- **TLS/HTTPS** - All external communication encrypted
- **Token sanitization** - Sensitive data removed from logs
- **Rate limiting** - Per-user and per-provider limits

## Performance Optimizations

### Caching Strategy

```
Request → Cache Check → [Hit: Return] / [Miss: Forward → Cache Store]
```

**Cache Layers:**
1. **In-memory** - Hot data (models list, provider status)
2. **Redis** - Session data, rate limits
3. **Database** - Persistent storage

### Connection Pooling

- HTTP client pool for provider requests
- Database connection pool (configurable)
- Reusable HTTP/2 connections

## Monitoring Architecture

### Metrics Collection

```
Request → Handler → Metrics Update → Prometheus Export
```

**Key Metrics:**
- `goleapai_requests_total{provider, model, status}`
- `goleapai_request_duration_seconds{provider, model}`
- `goleapai_provider_health_score{provider}`
- `goleapai_cost_saved_total{provider}`

### Logging Strategy

**Log Levels:**
- **DEBUG** - Detailed routing decisions
- **INFO** - Request processing, health checks
- **WARN** - Provider issues, quota warnings
- **ERROR** - Failed requests, system errors

**Output Formats:**
- **JSON** - Production (structured logging)
- **Console** - Development (pretty-printed)

## Scalability Considerations

### Horizontal Scaling

The system is designed to scale horizontally:

- **Stateless gateway** - No local state, all in DB/Redis
- **Shared configuration** - Via database or config server
- **Distributed health checks** - Coordinated via Redis locks

### Vertical Scaling

Resource requirements per instance:

- **CPU** - 2-4 cores recommended
- **Memory** - 1-2 GB minimum, 4 GB recommended
- **Storage** - 10 GB for logs and local cache

## Future Architecture Enhancements

### Planned Additions

1. **Vector Database Integration**
   - Semantic model search
   - Embedding caching
   - RAG support

2. **Message Queue**
   - Async request processing
   - Background job processing
   - Event streaming

3. **Service Mesh**
   - Advanced traffic management
   - Circuit breaking
   - Distributed tracing

4. **Multi-Region Deployment**
   - Geographic load balancing
   - Edge caching
   - Reduced latency

## Conclusion

GoLeapAI's architecture is designed for:

- **Flexibility** - Multiple deployment options
- **Reliability** - Health monitoring and failover
- **Performance** - Caching and connection pooling
- **Scalability** - Horizontal and vertical scaling
- **Observability** - Comprehensive metrics and logging

The modular design allows easy extension with new providers, routing strategies, and features while maintaining backward compatibility.
