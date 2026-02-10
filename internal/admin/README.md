# GoLeapAI Admin API

API completa per la gestione amministrativa di GoLeapAI.

## Autenticazione

Tutti gli endpoint richiedono autenticazione JWT con ruolo `admin`:

```bash
Authorization: Bearer <jwt_token>
```

## Endpoints

### Provider Management

#### GET /admin/providers
Lista tutti i provider con filtri opzionali.

**Query Parameters:**
- `status` - Filtra per stato (active, down, maintenance, deprecated)
- `type` - Filtra per tipo (free, freemium, paid, local)

**Response:**
```json
{
  "providers": [...],
  "count": 10
}
```

#### POST /admin/providers
Aggiunge un nuovo provider.

**Request Body:**
```json
{
  "name": "custom-provider",
  "type": "free",
  "base_url": "https://api.example.com",
  "auth_type": "api_key",
  "tier": 2,
  "supports_streaming": true,
  "supports_tools": false,
  "supports_json": true
}
```

#### PUT /admin/providers/:id
Aggiorna un provider esistente.

#### DELETE /admin/providers/:id
Elimina un provider e tutti i dati associati.

#### PUT /admin/providers/:id/test
Testa la connessione a un provider.

**Response:**
```json
{
  "status": "healthy",
  "latency_ms": 234,
  "provider": "openai"
}
```

#### POST /admin/providers/:id/toggle
Attiva/disattiva un provider.

---

### Account Management

#### GET /admin/accounts
Lista tutti gli account con filtri.

**Query Parameters:**
- `provider_id` - Filtra per provider
- `user_id` - Filtra per utente

#### POST /admin/accounts
Crea un nuovo account provider per un utente.

**Request Body:**
```json
{
  "user_id": "uuid",
  "provider_id": "uuid",
  "credentials": "encrypted_data",
  "quota_limit": 1000000,
  "expires_at": "2025-12-31T23:59:59Z"
}
```

#### PUT /admin/accounts/:id
Aggiorna un account esistente.

#### DELETE /admin/accounts/:id
Elimina un account.

---

### User Management

#### GET /admin/users
Lista tutti gli utenti.

**Query Parameters:**
- `role` - Filtra per ruolo (admin, user, viewer)
- `active` - Filtra per stato (true/false)
- `limit` - Numero di risultati (default: 50)
- `offset` - Offset per paginazione

**Response:**
```json
{
  "users": [...],
  "count": 25,
  "total": 150,
  "limit": 50,
  "offset": 0
}
```

#### POST /admin/users
Crea un nuovo utente.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "secure_password",
  "name": "John Doe",
  "role": "user",
  "quota_tokens": 100000,
  "quota_requests": 1000,
  "generate_api_key": true
}
```

**Response:**
```json
{
  "message": "User created successfully",
  "user": {...},
  "api_key": "gla_xxxxx...",
  "api_key_preview": "gla_xxxxx..."
}
```

#### PUT /admin/users/:id
Aggiorna un utente esistente.

**Request Body:**
```json
{
  "name": "Jane Doe",
  "role": "admin",
  "active": true,
  "quota_tokens": 200000
}
```

#### DELETE /admin/users/:id
Elimina un utente e tutte le sue API key.

#### POST /admin/users/:id/reset-quota
Resetta le quote mensili di un utente.

#### GET /admin/users/:id/api-keys
Lista tutte le API key di un utente.

---

### Statistics

#### GET /admin/stats
Statistiche globali dettagliate.

**Query Parameters:**
- `range` - Intervallo temporale (1h, 24h, 7d, 30d)

**Response:**
```json
{
  "time_range": "24h",
  "timestamp": "2025-02-05T12:00:00Z",
  "total_providers": 25,
  "active_providers": 20,
  "total_users": 150,
  "total_requests": 50000,
  "success_rate": 0.98,
  "avg_latency_ms": 450,
  "total_tokens": 5000000,
  "cost_saved": 1250.50
}
```

#### GET /admin/stats/providers
Statistiche per provider.

#### GET /admin/stats/users
Statistiche per utente con dettagli di utilizzo.

#### GET /admin/stats/requests
Log delle richieste con filtri.

**Query Parameters:**
- `limit` - Numero di risultati (default: 100)
- `offset` - Offset per paginazione
- `user_id` - Filtra per utente
- `provider_id` - Filtra per provider
- `success` - Filtra per successo (true/false)

---

### Configuration

#### GET /admin/config
Restituisce la configurazione corrente (sanitizzata).

**Response:**
```json
{
  "server": {
    "port": 8080,
    "host": "0.0.0.0",
    "http3": true
  },
  "routing": {
    "strategy": "cost_optimized",
    "failover_enabled": true,
    "max_retries": 3
  },
  ...
}
```

#### POST /admin/config
Aggiorna la configurazione.

**Request Body:**
```json
{
  "routing": {
    "strategy": "latency_first",
    "max_retries": 5
  },
  "providers": {
    "auto_discovery": true
  }
}
```

---

### Backup & Restore

#### POST /admin/backup
Crea un backup completo del database.

**Request Body:**
```json
{
  "include_users": true,
  "include_logs": false,
  "compress": true,
  "description": "Weekly backup"
}
```

**Response:**
```json
{
  "message": "Backup created successfully",
  "backup": {
    "id": "uuid",
    "timestamp": "2025-02-05T12:00:00Z",
    "file_path": "./backups/backup_20250205_120000_abc123.json.gz",
    "size": 1048576,
    "provider_count": 25,
    "user_count": 150
  }
}
```

#### GET /admin/backup
Lista tutti i backup disponibili.

**Response:**
```json
{
  "backups": [
    {
      "id": "uuid",
      "timestamp": "2025-02-05T12:00:00Z",
      "size": 1048576,
      "compressed": true,
      "file_path": "./backups/backup_xxx.json.gz"
    }
  ],
  "count": 5
}
```

#### POST /admin/restore
Ripristina da un backup.

**Request Body:**
```json
{
  "backup_id": "uuid_or_short_id",
  "restore_users": true,
  "restore_config": false,
  "clear_existing": false
}
```

#### POST /admin/export
Esporta la configurazione dei provider in JSON.

**Response:** File JSON scaricabile

#### POST /admin/import
Importa provider da un file JSON.

**Form Data:**
- `file` - File JSON da importare

**Response:**
```json
{
  "message": "Providers imported successfully",
  "imported": 15,
  "skipped": 3,
  "total": 18
}
```

---

### Maintenance

#### POST /admin/maintenance/clear-cache
Pulisce la cache e forza garbage collection.

**Response:**
```json
{
  "message": "Cache cleared successfully",
  "cleared": 1234
}
```

#### POST /admin/maintenance/prune-logs
Elimina log vecchi.

**Request Body:**
```json
{
  "older_than_days": 30,
  "keep_count": 10000,
  "dry_run": false
}
```

**Response:**
```json
{
  "message": "Logs pruned successfully",
  "deleted": 45000,
  "dry_run": false
}
```

#### POST /admin/maintenance/reindex
Ricostruisce gli indici del database.

#### POST /admin/maintenance/health-check
Forza health check su tutti i provider.

**Request Body:**
```json
{
  "parallel": true
}
```

**Response:**
```json
{
  "message": "Health checks completed",
  "results": [
    {
      "provider_id": "uuid",
      "provider_name": "openai",
      "status": "healthy",
      "latency_ms": 234,
      "health_score": 1.0
    }
  ],
  "total": 25
}
```

#### GET /admin/maintenance/status
Stato del sistema e metriche di manutenzione.

**Response:**
```json
{
  "last_cache_clear": "2025-02-05T10:00:00Z",
  "last_log_prune": "2025-02-04T03:00:00Z",
  "last_reindex": "2025-02-03T02:00:00Z",
  "last_health_check": "2025-02-05T11:00:00Z",
  "provider_count": 25,
  "active_providers": 20,
  "user_count": 150,
  "request_log_count": 500000,
  "database_size_bytes": 104857600,
  "memory_usage_bytes": 52428800,
  "goroutine_count": 45,
  "uptime": "72h30m15s"
}
```

---

## Esempi di Utilizzo

### Creare un nuovo utente con API key

```bash
curl -X POST http://localhost:8080/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "SecurePass123!",
    "name": "New User",
    "role": "user",
    "quota_tokens": 100000,
    "generate_api_key": true
  }'
```

### Backup completo del database

```bash
curl -X POST http://localhost:8080/admin/backup \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "include_users": true,
    "compress": true
  }'
```

### Health check di tutti i provider

```bash
curl -X POST http://localhost:8080/admin/maintenance/health-check \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "parallel": true
  }'
```

### Statistiche dettagliate

```bash
curl -X GET "http://localhost:8080/admin/stats?range=24h" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Sicurezza

- **Autenticazione obbligatoria**: Tutti gli endpoint richiedono JWT valido
- **Ruolo admin richiesto**: Solo utenti con ruolo `admin` possono accedere
- **Rate limiting**: Implementato a livello di API key e utente
- **Audit logging**: Tutte le operazioni admin vengono registrate

## Note

- I backup vengono salvati in `./backups/` per default
- Le password sono hashate con bcrypt
- Le API key usano SHA256 per lookup + bcrypt per validazione
- Le credenziali sensibili non vengono mai esposte nelle response JSON
- Le quote vengono resettate automaticamente ogni mese
