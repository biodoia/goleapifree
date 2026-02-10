# Sistema di Autenticazione e Middleware

Documentazione completa del sistema di autenticazione e middleware HTTP per GoLeapAI Gateway.

## Architettura

### Componenti Principali

1. **pkg/auth** - Gestione autenticazione
   - `jwt.go` - Token JWT (access + refresh)
   - `apikey.go` - Gestione API keys

2. **pkg/middleware** - Middleware HTTP
   - `auth.go` - Autenticazione e autorizzazione
   - `logging.go` - Logging strutturato
   - `cors.go` - Cross-Origin Resource Sharing
   - `recovery.go` - Panic recovery
   - `metrics.go` - Metriche Prometheus

3. **pkg/models/user.go** - Modelli database
   - `User` - Utenti del sistema
   - `APIKey` - Chiavi API

## Autenticazione

### JWT Token

Il sistema supporta autenticazione tramite JWT con access token e refresh token.

#### Registrazione
```bash
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password",
  "name": "John Doe"
}
```

Risposta:
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "role": "user"
  },
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer"
}
```

#### Login
```bash
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password"
}
```

#### Refresh Token
```bash
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc..."
}
```

#### Uso del Token
Aggiungi l'header Authorization a tutte le richieste protette:

```bash
Authorization: Bearer <access_token>
```

### API Keys

Le API keys permettono autenticazione senza scadenza a breve termine, ideale per integrazioni M2M.

#### Creazione API Key
```bash
POST /v1/user/apikeys
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "Production API Key",
  "permissions": ["read", "write"],
  "rate_limit": 100,
  "expires_in": 365
}
```

Risposta (la chiave in chiaro viene mostrata UNA SOLA VOLTA):
```json
{
  "message": "API key created successfully. Save it now, it won't be shown again!",
  "api_key": "gla_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "key_info": {
    "id": "uuid",
    "name": "Production API Key",
    "preview": "gla_xxxxxxxx...",
    "permissions": ["read", "write"],
    "rate_limit": 100,
    "expires_at": "2027-02-05T12:00:00Z",
    "created_at": "2026-02-05T12:00:00Z"
  }
}
```

#### Uso API Key
```bash
Authorization: ApiKey gla_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

#### Lista API Keys
```bash
GET /v1/user/apikeys
Authorization: Bearer <access_token>
```

#### Revoca API Key
```bash
DELETE /v1/user/apikeys/{id}
Authorization: Bearer <access_token>
```

## Middleware

### Recovery Middleware

Cattura i panic e previene crash del server:

```go
// Automaticamente configurato nel gateway
middleware.RecoveryWithLogger()
```

Features:
- Cattura tutti i panic
- Log dello stack trace completo
- Risposta JSON strutturata
- Tracciamento request ID

### Request ID Middleware

Genera e traccia un ID univoco per ogni richiesta:

```go
middleware.RequestID()
```

Features:
- UUID generato automaticamente
- Supporta header X-Request-ID in ingresso
- Aggiunge X-Request-ID in risposta
- Disponibile in tutti i log

### CORS Middleware

Gestisce Cross-Origin Resource Sharing:

```go
middleware.CORS(middleware.DefaultCORSConfig())
```

Configurazione di default:
- Origin: `*` (configura per produzione!)
- Methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
- Headers: Origin, Content-Type, Accept, Authorization, X-Request-ID, X-API-Key
- Credentials: true
- Max Age: 24h

Configurazione custom:
```go
middleware.CORSWithConfig(middleware.CORSConfig{
    AllowedOrigins: []string{"https://app.example.com"},
    AllowedMethods: []string{"GET", "POST"},
    AllowCredentials: true,
})
```

### Logging Middleware

Logging strutturato con zerolog:

```go
middleware.Logging(middleware.LoggingConfig{
    SkipPaths: []string{"/health", "/ready"},
})
```

Log di ogni richiesta con:
- Request ID
- Method, Path, IP
- User ID (se autenticato)
- Status code
- Latency
- Request/Response size

Esempio log:
```json
{
  "level": "info",
  "request_id": "123e4567-e89b-12d3-a456-426614174000",
  "method": "POST",
  "path": "/v1/chat/completions",
  "ip": "192.168.1.100",
  "user_id": "user-uuid",
  "status": 200,
  "latency": 1234,
  "latency_ms": 1234,
  "bytes_sent": 1024,
  "message": "request completed"
}
```

### Metrics Middleware

Metriche Prometheus per monitoring:

```go
middleware.Metrics(middleware.MetricsConfig{
    SkipPaths: []string{"/health", "/metrics"},
})
```

Metriche esposte:
- `goleapai_http_requests_total` - Totale richieste HTTP
- `goleapai_http_request_duration_seconds` - Durata richieste
- `goleapai_http_request_size_bytes` - Dimensione request
- `goleapai_http_response_size_bytes` - Dimensione response
- `goleapai_http_active_requests` - Richieste attive
- `goleapai_http_errors_total` - Totale errori
- `goleapai_apikey_usage_total` - Uso API keys
- `goleapai_rate_limit_hits_total` - Rate limit hits

Endpoint metriche:
```bash
GET /metrics
```

### Auth Middleware

Autenticazione e rate limiting:

```go
middleware.Auth(middleware.AuthConfig{
    JWTManager:    jwtManager,
    APIKeyManager: apiKeyManager,
    GetAPIKeyFunc: getAPIKeyFromDB,
    UserRateLimit: 60, // req/min
})
```

Features:
- Supporto JWT Bearer token
- Supporto API Key
- Rate limiting per utente
- Rate limiting per API key
- Context injection (user ID, email, role)

### Role-Based Access Control

Middleware per verificare ruoli utente:

```go
middleware.RequireRole("admin")
```

Ruoli disponibili:
- `admin` - Accesso completo
- `user` - Utente standard
- `viewer` - Solo lettura

## Endpoints

### Pubblici (no auth)
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /metrics` - Metriche Prometheus
- `POST /auth/register` - Registrazione
- `POST /auth/login` - Login
- `POST /auth/refresh` - Refresh token

### Autenticati (JWT o API Key)
- `POST /v1/chat/completions` - Chat completion
- `POST /v1/messages` - Anthropic messages
- `GET /v1/models` - Lista modelli

### User Management
- `GET /v1/user/profile` - Profilo utente
- `PUT /v1/user/profile` - Aggiorna profilo
- `GET /v1/user/apikeys` - Lista API keys
- `POST /v1/user/apikeys` - Crea API key
- `DELETE /v1/user/apikeys/:id` - Revoca API key

### Admin (solo admin)
- `GET /admin/providers` - Lista provider
- `GET /admin/stats` - Statistiche
- `GET /admin/users` - Lista utenti
- `GET /admin/metrics-info` - Info metriche

## Rate Limiting

### Globale
- Configurabile per utente (default: 60 req/min)
- Token bucket algorithm
- Cleanup automatico dei limiter inattivi

### Per API Key
- Rate limit specifico per chiave
- Override del rate limit utente
- Tracking automatico degli utilizzi

## Security Best Practices

### JWT Secret
**IMPORTANTE**: Cambia il JWT secret in produzione!

```bash
# Environment variable
export JWT_SECRET="your-secret-key-here"
```

O nel file di configurazione.

### API Keys
- Le chiavi sono hashatee con bcrypt per storage sicuro
- SHA256 hash per lookup veloce nel database
- Preview per identificazione in UI
- Revocabile in qualsiasi momento

### Password
- Minimo 8 caratteri (configura validazione)
- Hash con bcrypt
- Never logged o esposto in JSON

### CORS
In produzione, configura allowed origins specifici:

```yaml
cors:
  allowed_origins:
    - "https://app.example.com"
    - "https://admin.example.com"
```

### Rate Limiting
Configura rate limits appropriati per prevenire abuse:

```yaml
auth:
  user_rate_limit: 60  # requests per minute
  api_key_rate_limit: 100
```

## Testing

### Test Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

### Test Autenticazione
```bash
curl -X GET http://localhost:8080/v1/user/profile \
  -H "Authorization: Bearer <access_token>"
```

### Test API Key
```bash
curl -X GET http://localhost:8080/v1/models \
  -H "Authorization: ApiKey gla_xxxxxxxxxxxxxxxx"
```

### Test Metriche
```bash
curl http://localhost:8080/metrics
```

## Monitoring

### Logs
I log sono in formato JSON strutturato, compatibili con:
- ELK Stack
- Grafana Loki
- CloudWatch Logs
- Google Cloud Logging

### Metrics
Le metriche Prometheus possono essere:
- Scrape da Prometheus server
- Visualizzate in Grafana
- Utilizzate per alerting

Dashboard Grafana consigliata:
- Request rate & latency
- Error rate
- Active connections
- API key usage
- Rate limit hits

## Troubleshooting

### 401 Unauthorized
- Token scaduto: usa `/auth/refresh`
- Token invalido: rifare login
- API key revocata: crea nuova chiave

### 403 Forbidden
- Permissions insufficienti
- Account disabilitato
- Email non verificata

### 429 Too Many Requests
- Rate limit raggiunto
- Attendi prima del prossimo request
- Considera upgrade a rate limit più alto

### 500 Internal Server Error
- Check logs per stack trace
- Verificare connessione database
- Controllare metriche per anomalie
