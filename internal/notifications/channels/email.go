package channels

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/notifications"
	"github.com/rs/zerolog/log"
)

// EmailConfig configurazione per il canale email
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	From         string
	To           []string
	UseTLS       bool
	BatchSize    int
	BatchTimeout time.Duration
	RateLimit    int // max emails per minuto
}

// EmailChannel implementa il canale di notifica email
type EmailChannel struct {
	config *EmailConfig
	auth   smtp.Auth

	// Batching
	batch      []*EmailNotification
	batchMu    sync.Mutex
	batchTimer *time.Timer

	// Rate limiting
	rateLimiter *time.Ticker
	sendQueue   chan *EmailNotification
	stopCh      chan struct{}
	wg          sync.WaitGroup

	// Templates
	templates map[notifications.EventType]*template.Template
}

// EmailNotification rappresenta una notifica email
type EmailNotification struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
	Event   notifications.Event
}

// NewEmailChannel crea un nuovo canale email
func NewEmailChannel(config *EmailConfig) *EmailChannel {
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.BatchTimeout <= 0 {
		config.BatchTimeout = 5 * time.Minute
	}
	if config.RateLimit <= 0 {
		config.RateLimit = 60 // 60 email/min default
	}

	ec := &EmailChannel{
		config:    config,
		batch:     make([]*EmailNotification, 0, config.BatchSize),
		sendQueue: make(chan *EmailNotification, 100),
		stopCh:    make(chan struct{}),
		templates: make(map[notifications.EventType]*template.Template),
	}

	// Setup SMTP auth se configurato
	if config.Username != "" && config.Password != "" {
		ec.auth = smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)
	}

	// Carica templates
	ec.loadTemplates()

	return ec
}

// Start avvia il canale email
func (ec *EmailChannel) Start() error {
	// Rate limiter: permette N email al minuto
	interval := time.Minute / time.Duration(ec.config.RateLimit)
	ec.rateLimiter = time.NewTicker(interval)

	// Worker per processare la coda
	ec.wg.Add(1)
	go ec.worker()

	log.Info().
		Str("channel", "email").
		Str("smtp_host", ec.config.SMTPHost).
		Int("rate_limit", ec.config.RateLimit).
		Msg("Email channel started")

	return nil
}

// Stop ferma il canale email
func (ec *EmailChannel) Stop() error {
	close(ec.stopCh)
	ec.wg.Wait()

	if ec.rateLimiter != nil {
		ec.rateLimiter.Stop()
	}

	// Flush batch finale
	ec.flushBatch()

	log.Info().Msg("Email channel stopped")
	return nil
}

// Send invia una notifica
func (ec *EmailChannel) Send(ctx context.Context, event notifications.Event) error {
	// Crea notifica email
	notification := ec.createNotification(event)

	// Aggiungi alla coda
	select {
	case ec.sendQueue <- notification:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("email queue is full")
	}
}

// worker processa la coda di invio
func (ec *EmailChannel) worker() {
	defer ec.wg.Done()

	for {
		select {
		case <-ec.stopCh:
			return
		case notification := <-ec.sendQueue:
			// Aspetta il rate limiter
			<-ec.rateLimiter.C

			// Aggiungi al batch
			ec.addToBatch(notification)
		}
	}
}

// addToBatch aggiunge una notifica al batch
func (ec *EmailChannel) addToBatch(notification *EmailNotification) {
	ec.batchMu.Lock()
	defer ec.batchMu.Unlock()

	ec.batch = append(ec.batch, notification)

	// Se il batch è pieno, flush
	if len(ec.batch) >= ec.config.BatchSize {
		go ec.flushBatch()
	} else if ec.batchTimer == nil {
		// Imposta timer per flush automatico
		ec.batchTimer = time.AfterFunc(ec.config.BatchTimeout, ec.flushBatch)
	}
}

// flushBatch invia tutte le notifiche nel batch
func (ec *EmailChannel) flushBatch() {
	ec.batchMu.Lock()
	defer ec.batchMu.Unlock()

	if len(ec.batch) == 0 {
		return
	}

	// Reset timer
	if ec.batchTimer != nil {
		ec.batchTimer.Stop()
		ec.batchTimer = nil
	}

	// Invia batch
	for _, notification := range ec.batch {
		if err := ec.sendEmail(notification); err != nil {
			log.Error().
				Err(err).
				Str("subject", notification.Subject).
				Msg("Failed to send email")
		}
	}

	log.Info().
		Int("count", len(ec.batch)).
		Msg("Email batch sent")

	// Reset batch
	ec.batch = make([]*EmailNotification, 0, ec.config.BatchSize)
}

// sendEmail invia una singola email
func (ec *EmailChannel) sendEmail(notification *EmailNotification) error {
	// Costruisci messaggio
	msg := ec.buildMessage(notification)

	// Connetti al server SMTP
	addr := fmt.Sprintf("%s:%d", ec.config.SMTPHost, ec.config.SMTPPort)

	var err error
	if ec.config.UseTLS {
		err = ec.sendWithTLS(addr, msg, notification.To)
	} else {
		err = smtp.SendMail(addr, ec.auth, ec.config.From, notification.To, []byte(msg))
	}

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Debug().
		Strs("to", notification.To).
		Str("subject", notification.Subject).
		Msg("Email sent successfully")

	return nil
}

// sendWithTLS invia email con TLS
func (ec *EmailChannel) sendWithTLS(addr, msg string, to []string) error {
	// Connessione TLS
	tlsConfig := &tls.Config{
		ServerName: ec.config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect with TLS: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, ec.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// Auth
	if ec.auth != nil {
		if err := client.Auth(ec.auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(ec.config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return nil
}

// buildMessage costruisce il messaggio email
func (ec *EmailChannel) buildMessage(notification *EmailNotification) string {
	msg := bytes.NewBufferString("")

	msg.WriteString(fmt.Sprintf("From: %s\r\n", ec.config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", notification.To[0]))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", notification.Subject))

	if notification.IsHTML {
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	}

	msg.WriteString("\r\n")
	msg.WriteString(notification.Body)

	return msg.String()
}

// createNotification crea una notifica email dall'evento
func (ec *EmailChannel) createNotification(event notifications.Event) *EmailNotification {
	subject := ec.generateSubject(event)
	body := ec.generateBody(event)

	return &EmailNotification{
		To:      ec.config.To,
		Subject: subject,
		Body:    body,
		IsHTML:  true,
		Event:   event,
	}
}

// generateSubject genera il subject dell'email
func (ec *EmailChannel) generateSubject(event notifications.Event) string {
	prefix := "[GoLeapAI]"

	switch event.Severity() {
	case notifications.SeverityCritical:
		prefix += " [CRITICAL]"
	case notifications.SeverityError:
		prefix += " [ERROR]"
	case notifications.SeverityWarning:
		prefix += " [WARNING]"
	case notifications.SeverityInfo:
		prefix += " [INFO]"
	}

	return fmt.Sprintf("%s %s", prefix, event.Message())
}

// generateBody genera il body dell'email usando template
func (ec *EmailChannel) generateBody(event notifications.Event) string {
	tmpl, exists := ec.templates[event.Type()]
	if !exists {
		// Fallback a template generico
		return ec.generateGenericBody(event)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, event); err != nil {
		log.Error().Err(err).Msg("Failed to execute template")
		return ec.generateGenericBody(event)
	}

	return buf.String()
}

// generateGenericBody genera un body generico
func (ec *EmailChannel) generateGenericBody(event notifications.Event) string {
	var buf bytes.Buffer

	buf.WriteString("<html><body>")
	buf.WriteString(fmt.Sprintf("<h2>%s</h2>", event.Message()))
	buf.WriteString(fmt.Sprintf("<p><strong>Severity:</strong> %s</p>", event.Severity()))
	buf.WriteString(fmt.Sprintf("<p><strong>Time:</strong> %s</p>", event.Timestamp().Format(time.RFC3339)))
	buf.WriteString("<h3>Details:</h3><ul>")

	for key, value := range event.Metadata() {
		buf.WriteString(fmt.Sprintf("<li><strong>%s:</strong> %v</li>", key, value))
	}

	buf.WriteString("</ul></body></html>")

	return buf.String()
}

// loadTemplates carica i template HTML
func (ec *EmailChannel) loadTemplates() {
	// Template per ProviderDown
	ec.templates[notifications.EventProviderDown] = template.Must(template.New("provider_down").Parse(`
<html>
<body style="font-family: Arial, sans-serif;">
	<h2 style="color: #d32f2f;">Provider Down Alert</h2>
	<p><strong>Provider:</strong> {{.ProviderName}}</p>
	<p><strong>Down Since:</strong> {{.DownSince.Format "2006-01-02 15:04:05"}}</p>
	<p><strong>Duration:</strong> {{.DownDuration}}</p>
	<p><strong>Reason:</strong> {{.Reason}}</p>
	<p><strong>Last Error:</strong> {{.LastError}}</p>
	<hr>
	<p style="color: #666; font-size: 12px;">GoLeapAI Notification System</p>
</body>
</html>
`))

	// Template per QuotaExhausted
	ec.templates[notifications.EventQuotaExhausted] = template.Must(template.New("quota_exhausted").Parse(`
<html>
<body style="font-family: Arial, sans-serif;">
	<h2 style="color: #f57c00;">Quota Exhausted</h2>
	<p><strong>Provider:</strong> {{.ProviderName}}</p>
	<p><strong>Limit:</strong> {{.QuotaLimit}}</p>
	<p><strong>Used:</strong> {{.QuotaUsed}}</p>
	<p><strong>Reset At:</strong> {{.ResetAt.Format "2006-01-02 15:04:05"}}</p>
	<hr>
	<p style="color: #666; font-size: 12px;">GoLeapAI Notification System</p>
</body>
</html>
`))

	// Template per HighErrorRate
	ec.templates[notifications.EventHighErrorRate] = template.Must(template.New("high_error_rate").Parse(`
<html>
<body style="font-family: Arial, sans-serif;">
	<h2 style="color: #f57c00;">High Error Rate Detected</h2>
	<p><strong>Provider:</strong> {{.ProviderName}}</p>
	<p><strong>Error Rate:</strong> {{printf "%.2f%%" (mul .ErrorRate 100)}}</p>
	<p><strong>Threshold:</strong> {{printf "%.2f%%" (mul .Threshold 100)}}</p>
	<p><strong>Errors:</strong> {{.ErrorCount}} / {{.TotalRequests}}</p>
	<p><strong>Window:</strong> {{.Window}}</p>
	<hr>
	<p style="color: #666; font-size: 12px;">GoLeapAI Notification System</p>
</body>
</html>
`))

	// Template per NewProviderDiscovered
	ec.templates[notifications.EventNewProviderDiscovered] = template.Must(template.New("new_provider").Parse(`
<html>
<body style="font-family: Arial, sans-serif;">
	<h2 style="color: #388e3c;">New Provider Discovered</h2>
	<p><strong>Name:</strong> {{.ProviderName}}</p>
	<p><strong>URL:</strong> {{.BaseURL}}</p>
	<p><strong>Source:</strong> {{.Source}}</p>
	<p><strong>Capabilities:</strong> {{range .Capabilities}}{{.}}, {{end}}</p>
	<hr>
	<p style="color: #666; font-size: 12px;">GoLeapAI Notification System</p>
</body>
</html>
`))
}
