package notifications

import (
	"context"
	"time"

	"github.com/biodoia/goleapifree/internal/notifications/channels"
	"github.com/google/uuid"
)

// ExampleBasicUsage mostra l'uso base del sistema di notifiche
func ExampleBasicUsage() {
	// Configura notifier con solo log channel
	config := &NotifierConfig{
		Enabled:        true,
		DefaultTimeout: 30 * time.Second,
		AsyncMode:      true,
		BufferSize:     1000,
	}

	logConfig := &channels.LogConfig{
		FilePath:       "./logs/notifications.log",
		MaxFileSize:    10 * 1024 * 1024,
		Format:         "json",
		IncludeConsole: true,
	}

	notifier := BuildNotifier(config, nil, nil, nil, logConfig)

	// Avvia notifier
	notifier.Start()
	defer notifier.Stop()

	// Invia notifica di provider down
	event := NewProviderDownEvent(
		uuid.New(),
		"OpenAI Free",
		time.Now().Add(-10*time.Minute),
		"Health check failed",
		"Connection timeout",
	)

	ctx := context.Background()
	notifier.Notify(ctx, event)
}

// ExampleMultiChannel mostra l'uso con più canali
func ExampleMultiChannel() {
	// Configura notifier
	config := &NotifierConfig{
		Enabled:        true,
		DefaultTimeout: 30 * time.Second,
		AsyncMode:      true,
	}

	// Email channel
	emailConfig := &channels.EmailConfig{
		SMTPHost: "smtp.gmail.com",
		SMTPPort: 587,
		Username: "your-email@gmail.com",
		Password: "your-password",
		From:     "noreply@goleapai.com",
		To:       []string{"admin@example.com"},
		UseTLS:   true,
	}

	// Webhook channel (Slack)
	webhookConfig := &channels.WebhookConfig{
		URL:        "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
		Method:     "POST",
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	}

	// Log channel
	logConfig := &channels.LogConfig{
		FilePath:       "./logs/notifications.log",
		Format:         "json",
		IncludeConsole: true,
	}

	// Desktop channel
	desktopConfig := &channels.DesktopConfig{
		AppName: "GoLeapAI",
		Enabled: true,
	}

	// Crea notifier con tutti i canali
	notifier := BuildNotifier(config, emailConfig, webhookConfig, desktopConfig, logConfig)

	// Avvia
	notifier.Start()
	defer notifier.Stop()

	// Invia vari tipi di eventi
	ctx := context.Background()

	// 1. Provider down
	notifier.Notify(ctx, NewProviderDownEvent(
		uuid.New(),
		"Anthropic Free",
		time.Now().Add(-6*time.Minute),
		"Multiple failed health checks",
		"503 Service Unavailable",
	))

	// 2. Quota warning
	notifier.Notify(ctx, NewQuotaWarningEvent(
		uuid.New(),
		uuid.New(),
		"OpenAI Free",
		0.92, // 92%
		10000,
		9200,
		time.Now().Add(12*time.Hour),
	))

	// 3. High error rate
	notifier.Notify(ctx, NewHighErrorRateEvent(
		uuid.New(),
		"Claude API",
		0.15, // 15%
		0.10, // threshold 10%
		150,
		1000,
		10*time.Minute,
	))

	// 4. New provider discovered
	notifier.Notify(ctx, NewNewProviderDiscoveredEvent(
		uuid.New(),
		"Gemini Free",
		"https://generativelanguage.googleapis.com",
		"github",
		[]string{"streaming", "json"},
	))
}

// ExampleCustomRules mostra come creare regole personalizzate
func ExampleCustomRules() {
	notifier := DefaultNotifier()
	notifier.Start()
	defer notifier.Stop()

	// Regola personalizzata: notifica solo errori critici di notte
	criticalNightRule := &Rule{
		ID:        uuid.New(),
		Name:      "Critical Errors at Night",
		EventType: "",
		Enabled:   true,
		Channels:  []string{"email", "webhook"},
		Conditions: Conditions{
			MinSeverity: SeverityCritical,
		},
		Cooldown: 30 * time.Minute,
		UserPrefs: UserPreferences{
			EmailEnabled:    true,
			WebhookEnabled:  true,
			QuietHoursStart: "08:00",
			QuietHoursEnd:   "22:00",
			MaxPerHour:      5,
		},
	}

	notifier.AddRule(criticalNightRule)

	// Regola per specifici provider
	specificProviderRule := &Rule{
		ID:        uuid.New(),
		Name:      "Production Provider Alerts",
		EventType: EventProviderDown,
		Enabled:   true,
		Channels:  []string{"email", "webhook", "desktop"},
		Conditions: Conditions{
			MinSeverity: SeverityError,
			ProviderIDs: []uuid.UUID{
				uuid.MustParse("12345678-1234-1234-1234-123456789012"),
			},
			MinOccurrences: 2,
			TimeWindow:     5 * time.Minute,
		},
		Cooldown: 10 * time.Minute,
		UserPrefs: UserPreferences{
			EmailEnabled:   true,
			WebhookEnabled: true,
			DesktopEnabled: true,
			MaxPerHour:     10,
		},
	}

	notifier.AddRule(specificProviderRule)
}

// ExampleWebhookWithSlack mostra l'integrazione con Slack
func ExampleWebhookWithSlack() {
	// Webhook personalizzato per Slack
	slackConfig := &channels.WebhookConfig{
		URL:        "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
		Method:     "POST",
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	}

	slackChannel := channels.NewWebhookChannelWithCustomPayload(
		slackConfig,
		channels.SlackPayloadBuilder,
	)

	config := &NotifierConfig{
		Enabled:        true,
		DefaultTimeout: 30 * time.Second,
		AsyncMode:      false, // Sincrono per questo esempio
	}

	notifier := NewNotifier(config)
	notifier.RegisterChannel("slack", slackChannel)
	notifier.Start()
	defer notifier.Stop()

	// Invia notifica
	event := NewHighErrorRateEvent(
		uuid.New(),
		"GPT-4 Free",
		0.25,
		0.10,
		250,
		1000,
		15*time.Minute,
	)

	notifier.Notify(context.Background(), event)
}

// ExampleWebhookWithDiscord mostra l'integrazione con Discord
func ExampleWebhookWithDiscord() {
	discordConfig := &channels.WebhookConfig{
		URL:        "https://discord.com/api/webhooks/YOUR/WEBHOOK",
		Method:     "POST",
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	}

	discordChannel := channels.NewWebhookChannelWithCustomPayload(
		discordConfig,
		channels.DiscordPayloadBuilder,
	)

	config := &NotifierConfig{
		Enabled:        true,
		DefaultTimeout: 30 * time.Second,
	}

	notifier := NewNotifier(config)
	notifier.RegisterChannel("discord", discordChannel)
	notifier.Start()
	defer notifier.Stop()
}

// ExampleIntegrationWithQuotaManager mostra integrazione con quota manager
func ExampleIntegrationWithQuotaManager() {
	// Crea notifier
	notifier := DefaultNotifier()
	notifier.Start()
	defer notifier.Stop()

	// Questa funzione verrebbe chiamata dal quota manager quando quota > 90%
	onQuotaWarning := func(accountID, providerID uuid.UUID, providerName string, usagePercent float64, quotaLimit, quotaUsed int64, resetAt time.Time) {
		event := NewQuotaWarningEvent(
			accountID,
			providerID,
			providerName,
			usagePercent,
			quotaLimit,
			quotaUsed,
			resetAt,
		)

		notifier.Notify(context.Background(), event)
	}

	// Simula chiamata
	onQuotaWarning(
		uuid.New(),
		uuid.New(),
		"Claude Free",
		0.95,
		1000000,
		950000,
		time.Now().Add(8*time.Hour),
	)
}

// ExampleIntegrationWithHealthMonitor mostra integrazione con health monitor
func ExampleIntegrationWithHealthMonitor() {
	notifier := DefaultNotifier()
	notifier.Start()
	defer notifier.Stop()

	// Questa funzione verrebbe chiamata dal health monitor
	onProviderDown := func(providerID uuid.UUID, providerName string, downSince time.Time, reason, lastError string) {
		// Notifica solo se down > 5 minuti
		if time.Since(downSince) > 5*time.Minute {
			event := NewProviderDownEvent(
				providerID,
				providerName,
				downSince,
				reason,
				lastError,
			)

			notifier.Notify(context.Background(), event)
		}
	}

	// Simula chiamata
	onProviderDown(
		uuid.New(),
		"Mistral Free",
		time.Now().Add(-7*time.Minute),
		"Consecutive health check failures",
		"Connection refused",
	)
}

// ExampleIntegrationWithStatsCollector mostra integrazione con stats collector
func ExampleIntegrationWithStatsCollector() {
	notifier := DefaultNotifier()
	notifier.Start()
	defer notifier.Stop()

	// Questa funzione verrebbe chiamata dal stats collector
	onHighErrorRate := func(providerID uuid.UUID, providerName string, errorRate float64, errorCount, totalRequests int64) {
		// Notifica solo se error rate > 10%
		if errorRate > 0.10 {
			event := NewHighErrorRateEvent(
				providerID,
				providerName,
				errorRate,
				0.10,
				errorCount,
				totalRequests,
				10*time.Minute,
			)

			notifier.Notify(context.Background(), event)
		}
	}

	// Simula chiamata
	onHighErrorRate(
		uuid.New(),
		"GPT-3.5 Free",
		0.18,
		180,
		1000,
	)
}

// ExampleMetrics mostra come ottenere metriche
func ExampleMetrics() {
	notifier := DefaultNotifier()
	notifier.Start()
	defer notifier.Stop()

	// Invia alcune notifiche
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		event := NewProviderDownEvent(
			uuid.New(),
			"Test Provider",
			time.Now(),
			"Test",
			"Test error",
		)
		notifier.Notify(ctx, event)
	}

	// Aspetta che vengano processate (modalità asincrona)
	time.Sleep(1 * time.Second)

	// Ottieni metriche
	metrics := notifier.GetMetrics()

	// Stampa metriche
	println("Total sent:", metrics.TotalSent)
	println("Total failed:", metrics.TotalFailed)

	for chName, chMetrics := range metrics.ByChannel {
		println("Channel:", chName)
		println("  Sent:", chMetrics.Sent)
		println("  Failed:", chMetrics.Failed)
		println("  Avg Latency:", chMetrics.AvgLatency.String())
	}

	for eventType, count := range metrics.ByEventType {
		println("Event Type:", eventType, "Count:", count)
	}
}
