# Notification System - GoLeapAI

Sistema di notifiche multi-canale completo per GoLeapAI, con supporto per email, webhook, notifiche desktop e log.

## Caratteristiche

### Canali di Notifica

1. **Email Channel** (`channels/email.go`)
   - Supporto SMTP con TLS/SSL
   - Template HTML personalizzabili
   - Batching automatico delle notifiche
   - Rate limiting configurabile
   - Autenticazione SMTP

2. **Webhook Channel** (`channels/webhook.go`)
   - POST JSON a URL configurabili
   - Retry logic con backoff esponenziale
   - Firma HMAC-SHA256 per verifica
   - Payload personalizzabili
   - Integrazioni pre-built: Slack, Discord, Microsoft Teams

3. **Desktop Channel** (`channels/desktop.go`)
   - Notifiche native del sistema operativo
   - Supporto multi-piattaforma (Linux, macOS, Windows)
   - Urgency mapping automatico
   - Configurazione timeout e icone

4. **Log Channel** (`channels/log.go`)
   - Log su file con rotazione automatica
   - Formati JSON e testo
   - Compressione backup
   - Output anche su console (opzionale)
   - Statistiche file di log

### Eventi Supportati

Tutti gli eventi sono definiti in `events.go`:

- **ProviderDownEvent**: Provider non disponibile
- **QuotaExhaustedEvent**: Quota esaurita
- **QuotaWarningEvent**: Quota vicino al limite (>90%)
- **HighErrorRateEvent**: Tasso di errori elevato (>10%)
- **CostThresholdEvent**: Soglia di costo superata
- **NewProviderDiscoveredEvent**: Nuovo provider scoperto
- **ProviderRecoveredEvent**: Provider ripristinato

### Rule Engine

Sistema di regole configurabile (`rules.go`):

- **Condizioni**: Severità minima, provider specifici, soglie, occorrenze
- **Cooldown**: Periodo minimo tra notifiche
- **Quiet Hours**: Finestre orarie in cui non inviare notifiche
- **Rate Limiting**: Massimo numero di notifiche per ora
- **User Preferences**: Preferenze utente per canale

## Architettura

```
notifications/
├── notifier.go           # Dispatcher principale
├── events.go             # Definizioni eventi
├── rules.go              # Rule engine e condizioni
├── example_usage.go      # Esempi di utilizzo
└── channels/
    ├── email.go          # Canale email
    ├── webhook.go        # Canale webhook
    ├── desktop.go        # Canale desktop
    └── log.go            # Canale log
```

## Utilizzo Base

### Setup Rapido

```go
import "github.com/biodoia/goleapifree/internal/notifications"

// Notifier con configurazione di default
notifier := notifications.DefaultNotifier()
notifier.Start()
defer notifier.Stop()

// Invia notifica
event := notifications.NewProviderDownEvent(
    providerID,
    "OpenAI Free",
    time.Now().Add(-6*time.Minute),
    "Health check failed",
    "Connection timeout",
)

notifier.Notify(context.Background(), event)
```

### Configurazione Completa

```go
// Configurazione notifier
config := &NotifierConfig{
    Enabled:        true,
    DefaultTimeout: 30 * time.Second,
    AsyncMode:      true,
    BufferSize:     1000,
}

// Configurazione email
emailConfig := &EmailConfig{
    SMTPHost: "smtp.gmail.com",
    SMTPPort: 587,
    Username: "your-email@gmail.com",
    Password: "your-app-password",
    From:     "noreply@goleapai.com",
    To:       []string{"admin@example.com"},
    UseTLS:   true,
}

// Configurazione webhook
webhookConfig := &WebhookConfig{
    URL:        "https://hooks.slack.com/services/YOUR/WEBHOOK",
    Method:     "POST",
    Secret:     "your-secret-key",
    MaxRetries: 3,
}

// Configurazione log
logConfig := &LogConfig{
    FilePath:       "./logs/notifications.log",
    Format:         "json",
    IncludeConsole: true,
}

// Build notifier
notifier := BuildNotifier(config, emailConfig, webhookConfig, nil, logConfig)
```

### Regole Personalizzate

```go
// Regola: Notifica email quando quota > 90%
quotaRule := &Rule{
    ID:        uuid.New(),
    Name:      "Quota Warning",
    EventType: EventQuotaWarning,
    Enabled:   true,
    Channels:  []string{"email", "log"},
    Conditions: Conditions{
        MinSeverity:    SeverityWarning,
        ThresholdValue: 0.90,
        ThresholdType:  ThresholdGreaterThan,
    },
    Cooldown: 1 * time.Hour,
    UserPrefs: UserPreferences{
        EmailEnabled: true,
        LogEnabled:   true,
        MaxPerHour:   5,
    },
}

notifier.AddRule(quotaRule)

// Regola: Provider down > 5 minuti
downRule := &Rule{
    ID:        uuid.New(),
    Name:      "Provider Down Alert",
    EventType: EventProviderDown,
    Enabled:   true,
    Channels:  []string{"email", "webhook", "desktop"},
    Conditions: Conditions{
        MinSeverity:    SeverityCritical,
        MinOccurrences: 3,
        TimeWindow:     5 * time.Minute,
    },
    Cooldown: 15 * time.Minute,
    UserPrefs: UserPreferences{
        EmailEnabled:   true,
        WebhookEnabled: true,
        DesktopEnabled: true,
        MaxPerHour:     10,
    },
}

notifier.AddRule(downRule)
```

## Integrazioni

### Con Quota Manager

```go
// In quota/manager.go
func (m *Manager) notifyQuotaWarning(account *models.Account, usagePercent float64) {
    event := notifications.NewQuotaWarningEvent(
        account.ID,
        account.ProviderID,
        "Provider Name",
        usagePercent,
        account.QuotaLimit,
        account.QuotaUsed,
        account.LastReset.Add(24*time.Hour),
    )

    m.notifier.Notify(context.Background(), event)
}
```

### Con Health Monitor

```go
// In health/monitor.go
func (m *Monitor) checkProvider(provider *models.Provider) {
    if !provider.IsHealthy() && time.Since(provider.LastHealthy) > 5*time.Minute {
        event := notifications.NewProviderDownEvent(
            provider.ID,
            provider.Name,
            provider.LastHealthy,
            "Health check failed",
            provider.LastError,
        )

        m.notifier.Notify(context.Background(), event)
    }
}
```

### Con Stats Collector

```go
// In stats/collector.go
func (c *Collector) checkErrorRate(providerID uuid.UUID) {
    stats := c.GetProviderStats(providerID)
    errorRate := 1.0 - stats.SuccessRate

    if errorRate > 0.10 {
        event := notifications.NewHighErrorRateEvent(
            providerID,
            stats.ProviderName,
            errorRate,
            0.10,
            stats.ErrorCount,
            stats.TotalRequests,
            10*time.Minute,
        )

        c.notifier.Notify(context.Background(), event)
    }
}
```

## Webhook Integrazioni

### Slack

```go
slackConfig := &WebhookConfig{
    URL: "https://hooks.slack.com/services/YOUR/WEBHOOK",
}

slackChannel := NewWebhookChannelWithCustomPayload(
    slackConfig,
    SlackPayloadBuilder,
)

notifier.RegisterChannel("slack", slackChannel)
```

### Discord

```go
discordConfig := &WebhookConfig{
    URL: "https://discord.com/api/webhooks/YOUR/WEBHOOK",
}

discordChannel := NewWebhookChannelWithCustomPayload(
    discordConfig,
    DiscordPayloadBuilder,
)

notifier.RegisterChannel("discord", discordChannel)
```

### Microsoft Teams

```go
teamsConfig := &WebhookConfig{
    URL: "https://outlook.office.com/webhook/YOUR/WEBHOOK",
}

teamsChannel := NewWebhookChannelWithCustomPayload(
    teamsConfig,
    TeamsPayloadBuilder,
)

notifier.RegisterChannel("teams", teamsChannel)
```

### Custom Webhook

```go
customBuilder := func(event Event) interface{} {
    return map[string]interface{}{
        "alert": map[string]interface{}{
            "type":     event.Type(),
            "severity": event.Severity(),
            "message":  event.Message(),
            "time":     event.Timestamp(),
            "data":     event.Metadata(),
        },
    }
}

customChannel := NewWebhookChannelWithCustomPayload(
    webhookConfig,
    customBuilder,
)
```

## Configurazione YAML

Esempio di configurazione in `config.yaml`:

```yaml
notifications:
  enabled: true
  async_mode: true
  buffer_size: 1000
  default_timeout: 30s

  email:
    enabled: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    username: your-email@gmail.com
    password: ${EMAIL_PASSWORD}
    from: noreply@goleapai.com
    to:
      - admin@example.com
    use_tls: true
    batch_size: 10
    batch_timeout: 5m
    rate_limit: 60

  webhook:
    enabled: true
    url: https://hooks.slack.com/services/YOUR/WEBHOOK
    secret: ${WEBHOOK_SECRET}
    max_retries: 3
    timeout: 10s

  desktop:
    enabled: true
    app_name: GoLeapAI
    urgency: normal

  log:
    enabled: true
    file_path: ./logs/notifications.log
    max_file_size: 10485760  # 10MB
    max_backups: 5
    format: json
    include_console: true

  rules:
    - name: "Provider Down > 5min"
      event_type: provider_down
      enabled: true
      channels: [email, webhook, log]
      conditions:
        min_severity: critical
        threshold_value: 300  # 5 minuti in secondi
        threshold_type: greater_than
      cooldown: 15m
      user_prefs:
        email_enabled: true
        webhook_enabled: true
        log_enabled: true
        max_per_hour: 10

    - name: "Quota > 90%"
      event_type: quota_warning
      enabled: true
      channels: [email, log]
      conditions:
        min_severity: warning
        threshold_value: 0.90
      cooldown: 1h
      user_prefs:
        email_enabled: true
        max_per_hour: 5

    - name: "Error Rate > 10%"
      event_type: high_error_rate
      enabled: true
      channels: [email, webhook]
      conditions:
        min_severity: warning
        threshold_value: 0.10
        min_occurrences: 3
        time_window: 10m
      cooldown: 30m
```

## Metriche

```go
// Ottieni metriche
metrics := notifier.GetMetrics()

fmt.Printf("Total sent: %d\n", metrics.TotalSent)
fmt.Printf("Total failed: %d\n", metrics.TotalFailed)

// Metriche per canale
for channel, chMetrics := range metrics.ByChannel {
    fmt.Printf("Channel %s:\n", channel)
    fmt.Printf("  Sent: %d\n", chMetrics.Sent)
    fmt.Printf("  Failed: %d\n", chMetrics.Failed)
    fmt.Printf("  Avg Latency: %v\n", chMetrics.AvgLatency)
}

// Metriche per tipo di evento
for eventType, count := range metrics.ByEventType {
    fmt.Printf("Event %s: %d\n", eventType, count)
}
```

## Testing

```go
// Test notifica desktop
desktopChannel := NewDesktopChannel(&DesktopConfig{
    AppName: "GoLeapAI Test",
    Enabled: true,
})
desktopChannel.TestNotification()

// Test notifica log
logChannel := NewLogChannel(&LogConfig{
    FilePath: "./test.log",
    Format:   "json",
})
logChannel.Start()

event := NewProviderDownEvent(
    uuid.New(),
    "Test Provider",
    time.Now(),
    "Test",
    "Test error",
)

logChannel.Send(context.Background(), event)

// Leggi ultime 10 righe
lines, _ := logChannel.Tail(10)
for _, line := range lines {
    fmt.Println(line)
}
```

## Best Practices

1. **Modalità Asincrona**: Usa `AsyncMode: true` in produzione per non bloccare l'applicazione
2. **Cooldown**: Configura cooldown appropriati per evitare spam
3. **Rate Limiting**: Imposta `MaxPerHour` per proteggere i canali
4. **Quiet Hours**: Configura quiet hours per notifiche non critiche
5. **Severità**: Usa severity appropriate per gli eventi
6. **Template Email**: Personalizza i template HTML per eventi specifici
7. **Webhook Secrets**: Usa sempre secret per webhook in produzione
8. **Log Rotation**: Configura rotazione automatica per evitare file troppo grandi
9. **Metriche**: Monitora le metriche per identificare problemi
10. **Testing**: Testa le notifiche prima del deployment

## Troubleshooting

### Email non inviate

- Verifica credenziali SMTP
- Controlla firewall per porta 587/465
- Verifica TLS/SSL settings
- Controlla rate limit

### Webhook falliscono

- Verifica URL webhook
- Controlla timeout
- Verifica firma HMAC
- Controlla logs per errori specifici

### Desktop notifications non funzionano

- Linux: Installa `notify-send` (`sudo apt install libnotify-bin`)
- macOS: Verifica permessi notifiche
- Windows: Richiede Windows 10+

### Log file troppo grande

- Riduci `MaxFileSize`
- Aumenta frequenza rotazione
- Abilita compressione

## License

Parte di GoLeapAI - Free LLM Gateway
