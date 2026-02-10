package channels

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/biodoia/goleapifree/internal/notifications"
	"github.com/rs/zerolog/log"
)

// DesktopConfig configurazione per notifiche desktop
type DesktopConfig struct {
	AppName     string
	IconPath    string
	Urgency     string // low, normal, critical
	Timeout     int    // millisecondi
	Enabled     bool
}

// DesktopChannel implementa il canale di notifica desktop
type DesktopChannel struct {
	config *DesktopConfig
}

// NewDesktopChannel crea un nuovo canale desktop
func NewDesktopChannel(config *DesktopConfig) *DesktopChannel {
	if config.AppName == "" {
		config.AppName = "GoLeapAI"
	}
	if config.Urgency == "" {
		config.Urgency = "normal"
	}
	if config.Timeout == 0 {
		config.Timeout = 5000 // 5 secondi
	}

	return &DesktopChannel{
		config: config,
	}
}

// Start avvia il canale desktop
func (dc *DesktopChannel) Start() error {
	// Verifica se le notifiche desktop sono supportate
	if !dc.isSupported() {
		log.Warn().
			Str("os", runtime.GOOS).
			Msg("Desktop notifications not supported on this platform")
		return fmt.Errorf("desktop notifications not supported on %s", runtime.GOOS)
	}

	log.Info().
		Str("channel", "desktop").
		Str("os", runtime.GOOS).
		Msg("Desktop channel started")

	return nil
}

// Stop ferma il canale desktop
func (dc *DesktopChannel) Stop() error {
	log.Info().Msg("Desktop channel stopped")
	return nil
}

// Send invia una notifica desktop
func (dc *DesktopChannel) Send(ctx context.Context, event notifications.Event) error {
	if !dc.config.Enabled {
		return nil
	}

	title := dc.getTitle(event)
	message := event.Message()

	// Aggiungi dettagli chiave al messaggio
	details := dc.getKeyDetails(event)
	if details != "" {
		message = message + "\n" + details
	}

	return dc.sendNotification(ctx, title, message, dc.mapUrgency(event.Severity()))
}

// isSupported verifica se le notifiche desktop sono supportate
func (dc *DesktopChannel) isSupported() bool {
	switch runtime.GOOS {
	case "linux":
		// Verifica se notify-send è disponibile
		_, err := exec.LookPath("notify-send")
		return err == nil
	case "darwin":
		// macOS supporta osascript
		_, err := exec.LookPath("osascript")
		return err == nil
	case "windows":
		// Windows 10+ supporta toast notifications via PowerShell
		return true
	default:
		return false
	}
}

// sendNotification invia la notifica in base alla piattaforma
func (dc *DesktopChannel) sendNotification(ctx context.Context, title, message, urgency string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = dc.linuxNotification(title, message, urgency)
	case "darwin":
		cmd = dc.macNotification(title, message)
	case "windows":
		cmd = dc.windowsNotification(title, message)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Dir = "/"
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to send desktop notification")
		return fmt.Errorf("failed to send notification: %w", err)
	}

	log.Debug().
		Str("title", title).
		Str("platform", runtime.GOOS).
		Msg("Desktop notification sent")

	return nil
}

// linuxNotification crea comando per Linux (notify-send)
func (dc *DesktopChannel) linuxNotification(title, message, urgency string) *exec.Cmd {
	args := []string{
		"-a", dc.config.AppName,
		"-u", urgency,
		"-t", fmt.Sprintf("%d", dc.config.Timeout),
	}

	if dc.config.IconPath != "" {
		args = append(args, "-i", dc.config.IconPath)
	}

	args = append(args, title, message)

	return exec.Command("notify-send", args...)
}

// macNotification crea comando per macOS (osascript)
func (dc *DesktopChannel) macNotification(title, message string) *exec.Cmd {
	// Escape apici singoli
	title = strings.ReplaceAll(title, "'", "\\'")
	message = strings.ReplaceAll(message, "'", "\\'")

	script := fmt.Sprintf(`display notification "%s" with title "%s" sound name "default"`, message, title)

	return exec.Command("osascript", "-e", script)
}

// windowsNotification crea comando per Windows (PowerShell)
func (dc *DesktopChannel) windowsNotification(title, message string) *exec.Cmd {
	// Escape per PowerShell
	title = strings.ReplaceAll(title, `"`, `\"`)
	message = strings.ReplaceAll(message, `"`, `\"`)

	script := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$toastXml = [xml] $template.GetXml()
$toastXml.GetElementsByTagName("text")[0].AppendChild($toastXml.CreateTextNode("%s")) > $null
$toastXml.GetElementsByTagName("text")[1].AppendChild($toastXml.CreateTextNode("%s")) > $null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($toastXml.OuterXml)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("%s").Show($toast)
`, title, message, dc.config.AppName)

	return exec.Command("powershell", "-Command", script)
}

// getTitle genera il titolo della notifica
func (dc *DesktopChannel) getTitle(event notifications.Event) string {
	switch event.Severity() {
	case notifications.SeverityCritical:
		return "[CRITICAL] " + dc.config.AppName
	case notifications.SeverityError:
		return "[ERROR] " + dc.config.AppName
	case notifications.SeverityWarning:
		return "[WARNING] " + dc.config.AppName
	default:
		return dc.config.AppName
	}
}

// getKeyDetails estrae i dettagli chiave dall'evento
func (dc *DesktopChannel) getKeyDetails(event notifications.Event) string {
	meta := event.Metadata()
	details := make([]string, 0)

	// Aggiungi solo i campi più rilevanti
	if providerName, ok := meta["provider_name"].(string); ok {
		details = append(details, "Provider: "+providerName)
	}

	if errorRate, ok := meta["error_rate"].(float64); ok {
		details = append(details, fmt.Sprintf("Error Rate: %.2f%%", errorRate*100))
	}

	if usagePercent, ok := meta["usage_percent"].(float64); ok {
		details = append(details, fmt.Sprintf("Usage: %.2f%%", usagePercent))
	}

	if len(details) > 2 {
		details = details[:2] // Massimo 2 dettagli
	}

	return strings.Join(details, "\n")
}

// mapUrgency mappa la severity all'urgency per notify-send
func (dc *DesktopChannel) mapUrgency(severity notifications.Severity) string {
	switch severity {
	case notifications.SeverityCritical:
		return "critical"
	case notifications.SeverityError:
		return "critical"
	case notifications.SeverityWarning:
		return "normal"
	default:
		return "low"
	}
}

// TestNotification invia una notifica di test
func (dc *DesktopChannel) TestNotification() error {
	ctx := context.Background()
	return dc.sendNotification(ctx, "Test Notification", "Desktop notifications are working!", "low")
}
