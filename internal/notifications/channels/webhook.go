package channels

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/biodoia/goleapifree/internal/notifications"
	"github.com/rs/zerolog/log"
)

// WebhookConfig configurazione per il canale webhook
type WebhookConfig struct {
	URL            string
	Method         string // POST, PUT
	Secret         string // Per firma HMAC
	Headers        map[string]string
	Timeout        time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	RetryBackoff   float64 // Moltiplicatore per backoff esponenziale
}

// WebhookChannel implementa il canale di notifica webhook
type WebhookChannel struct {
	config *WebhookConfig
	client *http.Client
}

// WebhookPayload rappresenta il payload inviato al webhook
type WebhookPayload struct {
	EventType  string                 `json:"event_type"`
	Severity   string                 `json:"severity"`
	Message    string                 `json:"message"`
	Timestamp  string                 `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata"`
	Signature  string                 `json:"signature,omitempty"`
}

// NewWebhookChannel crea un nuovo canale webhook
func NewWebhookChannel(config *WebhookConfig) *WebhookChannel {
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 2.0
	}

	return &WebhookChannel{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Start avvia il canale webhook
func (wc *WebhookChannel) Start() error {
	log.Info().
		Str("channel", "webhook").
		Str("url", wc.config.URL).
		Msg("Webhook channel started")
	return nil
}

// Stop ferma il canale webhook
func (wc *WebhookChannel) Stop() error {
	log.Info().Msg("Webhook channel stopped")
	return nil
}

// Send invia una notifica
func (wc *WebhookChannel) Send(ctx context.Context, event notifications.Event) error {
	// Crea payload
	payload := wc.createPayload(event)

	// Serializza a JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Invia con retry logic
	return wc.sendWithRetry(ctx, jsonData)
}

// createPayload crea il payload dal evento
func (wc *WebhookChannel) createPayload(event notifications.Event) *WebhookPayload {
	payload := &WebhookPayload{
		EventType: string(event.Type()),
		Severity:  string(event.Severity()),
		Message:   event.Message(),
		Timestamp: event.Timestamp().Format(time.RFC3339),
		Metadata:  event.Metadata(),
	}

	// Aggiungi firma HMAC se secret è configurato
	if wc.config.Secret != "" {
		payload.Signature = wc.generateSignature(payload)
	}

	return payload
}

// generateSignature genera una firma HMAC del payload
func (wc *WebhookChannel) generateSignature(payload *WebhookPayload) string {
	// Serializza payload senza signature
	data, err := json.Marshal(struct {
		EventType string                 `json:"event_type"`
		Severity  string                 `json:"severity"`
		Message   string                 `json:"message"`
		Timestamp string                 `json:"timestamp"`
		Metadata  map[string]interface{} `json:"metadata"`
	}{
		EventType: payload.EventType,
		Severity:  payload.Severity,
		Message:   payload.Message,
		Timestamp: payload.Timestamp,
		Metadata:  payload.Metadata,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal payload for signature")
		return ""
	}

	// Calcola HMAC-SHA256
	h := hmac.New(sha256.New, []byte(wc.config.Secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// sendWithRetry invia la richiesta con retry logic
func (wc *WebhookChannel) sendWithRetry(ctx context.Context, data []byte) error {
	var lastErr error
	delay := wc.config.RetryDelay

	for attempt := 0; attempt <= wc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying webhook request")

			select {
			case <-time.After(delay):
				// Backoff esponenziale
				delay = time.Duration(float64(delay) * wc.config.RetryBackoff)
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Invia richiesta
		if err := wc.sendRequest(ctx, data); err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Msg("Webhook request failed")
			continue
		}

		// Successo
		log.Info().
			Str("url", wc.config.URL).
			Int("attempts", attempt+1).
			Msg("Webhook sent successfully")
		return nil
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", wc.config.MaxRetries+1, lastErr)
}

// sendRequest invia una singola richiesta HTTP
func (wc *WebhookChannel) sendRequest(ctx context.Context, data []byte) error {
	// Crea richiesta
	req, err := http.NewRequestWithContext(ctx, wc.config.Method, wc.config.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoLeapAI-Notifier/1.0")

	// Custom headers
	for key, value := range wc.config.Headers {
		req.Header.Set(key, value)
	}

	// Se c'è un secret, aggiungi come header
	if wc.config.Secret != "" {
		// Estrai signature dal payload
		var payload WebhookPayload
		if err := json.Unmarshal(data, &payload); err == nil {
			req.Header.Set("X-Webhook-Signature", payload.Signature)
		}
	}

	// Invia richiesta
	resp, err := wc.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Leggi response body
	body, _ := io.ReadAll(resp.Body)

	// Verifica status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Debug().
		Int("status", resp.StatusCode).
		Str("response", string(body)).
		Msg("Webhook response")

	return nil
}

// VerifySignature verifica la firma di un webhook ricevuto
func VerifySignature(secret string, payload []byte, signature string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// CustomPayloadBuilder permette di costruire payload personalizzati
type CustomPayloadBuilder func(event notifications.Event) interface{}

// WebhookChannelWithCustomPayload canale webhook con payload personalizzato
type WebhookChannelWithCustomPayload struct {
	*WebhookChannel
	payloadBuilder CustomPayloadBuilder
}

// NewWebhookChannelWithCustomPayload crea un canale con payload personalizzato
func NewWebhookChannelWithCustomPayload(config *WebhookConfig, builder CustomPayloadBuilder) *WebhookChannelWithCustomPayload {
	return &WebhookChannelWithCustomPayload{
		WebhookChannel: NewWebhookChannel(config),
		payloadBuilder: builder,
	}
}

// Send invia con payload personalizzato
func (wc *WebhookChannelWithCustomPayload) Send(ctx context.Context, event notifications.Event) error {
	// Usa builder personalizzato
	payload := wc.payloadBuilder(event)

	// Serializza
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal custom payload: %w", err)
	}

	// Invia
	return wc.sendWithRetry(ctx, jsonData)
}

// SlackPayloadBuilder builder per payload Slack
func SlackPayloadBuilder(event notifications.Event) interface{} {
	color := "good"
	switch event.Severity() {
	case notifications.SeverityCritical:
		color = "danger"
	case notifications.SeverityError:
		color = "danger"
	case notifications.SeverityWarning:
		color = "warning"
	}

	fields := make([]map[string]interface{}, 0)
	for key, value := range event.Metadata() {
		fields = append(fields, map[string]interface{}{
			"title": key,
			"value": fmt.Sprintf("%v", value),
			"short": true,
		})
	}

	return map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":      color,
				"title":      event.Message(),
				"text":       fmt.Sprintf("Severity: %s", event.Severity()),
				"fields":     fields,
				"footer":     "GoLeapAI Notification",
				"footer_icon": "https://platform.slack-edge.com/img/default_application_icon.png",
				"ts":         event.Timestamp().Unix(),
			},
		},
	}
}

// DiscordPayloadBuilder builder per payload Discord
func DiscordPayloadBuilder(event notifications.Event) interface{} {
	color := 3066993 // Verde
	switch event.Severity() {
	case notifications.SeverityCritical:
		color = 15158332 // Rosso
	case notifications.SeverityError:
		color = 15158332 // Rosso
	case notifications.SeverityWarning:
		color = 16776960 // Giallo
	}

	fields := make([]map[string]interface{}, 0)
	for key, value := range event.Metadata() {
		fields = append(fields, map[string]interface{}{
			"name":   key,
			"value":  fmt.Sprintf("%v", value),
			"inline": true,
		})
	}

	return map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       event.Message(),
				"description": fmt.Sprintf("Severity: %s", event.Severity()),
				"color":       color,
				"fields":      fields,
				"footer": map[string]interface{}{
					"text": "GoLeapAI Notification",
				},
				"timestamp": event.Timestamp().Format(time.RFC3339),
			},
		},
	}
}

// TeamsPayloadBuilder builder per payload Microsoft Teams
func TeamsPayloadBuilder(event notifications.Event) interface{} {
	themeColor := "00FF00" // Verde
	switch event.Severity() {
	case notifications.SeverityCritical:
		themeColor = "FF0000" // Rosso
	case notifications.SeverityError:
		themeColor = "FF0000" // Rosso
	case notifications.SeverityWarning:
		themeColor = "FFFF00" // Giallo
	}

	facts := make([]map[string]interface{}, 0)
	for key, value := range event.Metadata() {
		facts = append(facts, map[string]interface{}{
			"name":  key,
			"value": fmt.Sprintf("%v", value),
		})
	}

	return map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"summary":    event.Message(),
		"themeColor": themeColor,
		"title":      event.Message(),
		"sections": []map[string]interface{}{
			{
				"activityTitle":    "GoLeapAI Notification",
				"activitySubtitle": event.Timestamp().Format(time.RFC3339),
				"facts":            facts,
			},
		},
	}
}
