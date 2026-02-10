package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NotificationPriority rappresenta la priorità di una notifica
type NotificationPriority string

const (
	PriorityHigh   NotificationPriority = "high"
	PriorityNormal NotificationPriority = "normal"
	PriorityLow    NotificationPriority = "low"
)

// Platform rappresenta la piattaforma mobile
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// PushNotification rappresenta una notifica push
type PushNotification struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	ImageURL    string                 `json:"image_url,omitempty"`
	IconURL     string                 `json:"icon_url,omitempty"`
	Sound       string                 `json:"sound,omitempty"`
	Badge       int                    `json:"badge,omitempty"`
	Priority    NotificationPriority   `json:"priority"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Category    string                 `json:"category,omitempty"`
	ClickAction string                 `json:"click_action,omitempty"`
	TTL         int                    `json:"ttl"` // Time to live in seconds
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

// DeviceToken rappresenta un token di dispositivo
type DeviceToken struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  Platform  `json:"platform"`
	AppID     string    `json:"app_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastUsed  time.Time `json:"last_used"`
}

// NotificationTemplate rappresenta un template di notifica
type NotificationTemplate struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Title    string                 `json:"title"`
	Body     string                 `json:"body"`
	Category string                 `json:"category"`
	Priority NotificationPriority   `json:"priority"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Sound    string                 `json:"sound,omitempty"`
}

// UserPreferences rappresenta le preferenze utente per le notifiche
type UserPreferences struct {
	UserID             string              `json:"user_id"`
	Enabled            bool                `json:"enabled"`
	QuietHoursStart    string              `json:"quiet_hours_start"` // Format: "22:00"
	QuietHoursEnd      string              `json:"quiet_hours_end"`   // Format: "08:00"
	CategoryPrefs      map[string]bool     `json:"category_prefs"`
	SoundEnabled       bool                `json:"sound_enabled"`
	VibrationEnabled   bool                `json:"vibration_enabled"`
	BadgeEnabled       bool                `json:"badge_enabled"`
	PreviewEnabled     bool                `json:"preview_enabled"`
	GroupingEnabled    bool                `json:"grouping_enabled"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// FCMPayload rappresenta un payload Firebase Cloud Messaging
type FCMPayload struct {
	To           string                 `json:"to,omitempty"`
	RegistrationIDs []string            `json:"registration_ids,omitempty"`
	Notification *FCMNotification       `json:"notification,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Priority     string                 `json:"priority,omitempty"`
	TimeToLive   int                    `json:"time_to_live,omitempty"`
	CollapseKey  string                 `json:"collapse_key,omitempty"`
}

// FCMNotification rappresenta la parte notification di FCM
type FCMNotification struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Sound       string `json:"sound,omitempty"`
	Badge       string `json:"badge,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
	ClickAction string `json:"click_action,omitempty"`
	Tag         string `json:"tag,omitempty"`
}

// APNsPayload rappresenta un payload Apple Push Notification service
type APNsPayload struct {
	Aps  *APNsAps               `json:"aps"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// APNsAps rappresenta la parte aps di APNs
type APNsAps struct {
	Alert            interface{} `json:"alert"`
	Badge            *int        `json:"badge,omitempty"`
	Sound            interface{} `json:"sound,omitempty"`
	ContentAvailable int         `json:"content-available,omitempty"`
	Category         string      `json:"category,omitempty"`
	ThreadID         string      `json:"thread-id,omitempty"`
	MutableContent   int         `json:"mutable-content,omitempty"`
}

// APNsAlert rappresenta l'alert di APNs
type APNsAlert struct {
	Title        string   `json:"title,omitempty"`
	Subtitle     string   `json:"subtitle,omitempty"`
	Body         string   `json:"body,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
}

// PushService gestisce l'invio di notifiche push
type PushService struct {
	fcmServerKey  string
	apnsCertPath  string
	apnsKeyID     string
	apnsTeamID    string
	devices       map[string]*DeviceToken
	templates     map[string]*NotificationTemplate
	preferences   map[string]*UserPreferences
	mu            sync.RWMutex
	sandbox       bool // Per testing APNs
}

// NewPushService crea un nuovo servizio push
func NewPushService(fcmServerKey, apnsCertPath, apnsKeyID, apnsTeamID string) *PushService {
	return &PushService{
		fcmServerKey: fcmServerKey,
		apnsCertPath: apnsCertPath,
		apnsKeyID:    apnsKeyID,
		apnsTeamID:   apnsTeamID,
		devices:      make(map[string]*DeviceToken),
		templates:    make(map[string]*NotificationTemplate),
		preferences:  make(map[string]*UserPreferences),
		sandbox:      false,
	}
}

// RegisterDevice registra un nuovo dispositivo
func (ps *PushService) RegisterDevice(ctx context.Context, device *DeviceToken) error {
	if device.Token == "" {
		return errors.New("device token is required")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	device.Enabled = true

	ps.devices[device.Token] = device
	return nil
}

// UnregisterDevice rimuove un dispositivo
func (ps *PushService) UnregisterDevice(ctx context.Context, token string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.devices, token)
	return nil
}

// UpdateDeviceToken aggiorna un token esistente
func (ps *PushService) UpdateDeviceToken(ctx context.Context, oldToken, newToken string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	device, exists := ps.devices[oldToken]
	if !exists {
		return errors.New("device not found")
	}

	delete(ps.devices, oldToken)
	device.Token = newToken
	device.UpdatedAt = time.Now()
	ps.devices[newToken] = device

	return nil
}

// GetUserDevices ottiene tutti i dispositivi di un utente
func (ps *PushService) GetUserDevices(userID string) []*DeviceToken {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	devices := make([]*DeviceToken, 0)
	for _, device := range ps.devices {
		if device.UserID == userID && device.Enabled {
			devices = append(devices, device)
		}
	}

	return devices
}

// SendNotification invia una notifica a dispositivi specifici
func (ps *PushService) SendNotification(ctx context.Context, notification *PushNotification, tokens []string) error {
	if len(tokens) == 0 {
		return errors.New("no tokens provided")
	}

	// Raggruppa i token per piattaforma
	iosTokens := make([]string, 0)
	androidTokens := make([]string, 0)

	ps.mu.RLock()
	for _, token := range tokens {
		if device, exists := ps.devices[token]; exists && device.Enabled {
			switch device.Platform {
			case PlatformIOS:
				iosTokens = append(iosTokens, token)
			case PlatformAndroid:
				androidTokens = append(androidTokens, token)
			}
		}
	}
	ps.mu.RUnlock()

	// Invia a ciascuna piattaforma
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	if len(androidTokens) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ps.sendToFCM(ctx, notification, androidTokens); err != nil {
				errChan <- fmt.Errorf("FCM error: %w", err)
			}
		}()
	}

	if len(iosTokens) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ps.sendToAPNs(ctx, notification, iosTokens); err != nil {
				errChan <- fmt.Errorf("APNs error: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Raccogli eventuali errori
	for err := range errChan {
		return err
	}

	return nil
}

// SendToUser invia una notifica a tutti i dispositivi di un utente
func (ps *PushService) SendToUser(ctx context.Context, userID string, notification *PushNotification) error {
	// Verifica preferenze utente
	if !ps.shouldSendNotification(userID, notification) {
		return nil
	}

	devices := ps.GetUserDevices(userID)
	if len(devices) == 0 {
		return errors.New("no devices found for user")
	}

	tokens := make([]string, len(devices))
	for i, device := range devices {
		tokens[i] = device.Token
	}

	return ps.SendNotification(ctx, notification, tokens)
}

// SendFromTemplate invia una notifica da un template
func (ps *PushService) SendFromTemplate(ctx context.Context, userID, templateID string, data map[string]interface{}) error {
	ps.mu.RLock()
	template, exists := ps.templates[templateID]
	ps.mu.RUnlock()

	if !exists {
		return errors.New("template not found")
	}

	notification := &PushNotification{
		ID:       generateNotificationID(),
		Title:    interpolateTemplate(template.Title, data),
		Body:     interpolateTemplate(template.Body, data),
		Category: template.Category,
		Priority: template.Priority,
		Sound:    template.Sound,
		Data:     mergeData(template.Data, data),
		TTL:      3600,
	}

	return ps.SendToUser(ctx, userID, notification)
}

// sendToFCM invia notifiche tramite Firebase Cloud Messaging
func (ps *PushService) sendToFCM(ctx context.Context, notification *PushNotification, tokens []string) error {
	payload := &FCMPayload{
		RegistrationIDs: tokens,
		Notification: &FCMNotification{
			Title:       notification.Title,
			Body:        notification.Body,
			Sound:       notification.Sound,
			ClickAction: notification.ClickAction,
			Icon:        notification.IconURL,
		},
		Data:        notification.Data,
		Priority:    string(notification.Priority),
		TimeToLive:  notification.TTL,
		CollapseKey: notification.Category,
	}

	// In produzione, qui invieresti la richiesta a FCM
	// https://fcm.googleapis.com/fcm/send
	_, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Simulazione dell'invio
	fmt.Printf("FCM: Sending notification to %d devices\n", len(tokens))
	return nil
}

// sendToAPNs invia notifiche tramite Apple Push Notification service
func (ps *PushService) sendToAPNs(ctx context.Context, notification *PushNotification, tokens []string) error {
	alert := &APNsAlert{
		Title: notification.Title,
		Body:  notification.Body,
	}

	payload := &APNsPayload{
		Aps: &APNsAps{
			Alert:    alert,
			Sound:    notification.Sound,
			Category: notification.Category,
		},
		Data: notification.Data,
	}

	if notification.Badge > 0 {
		badge := notification.Badge
		payload.Aps.Badge = &badge
	}

	// In produzione, qui invieresti la richiesta ad APNs
	// https://api.push.apple.com/3/device/<token>
	_, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Simulazione dell'invio
	fmt.Printf("APNs: Sending notification to %d devices\n", len(tokens))
	return nil
}

// shouldSendNotification verifica se inviare la notifica in base alle preferenze
func (ps *PushService) shouldSendNotification(userID string, notification *PushNotification) bool {
	ps.mu.RLock()
	prefs, exists := ps.preferences[userID]
	ps.mu.RUnlock()

	if !exists || !prefs.Enabled {
		return false
	}

	// Verifica quiet hours
	if ps.isQuietHours(prefs) {
		return false
	}

	// Verifica preferenze per categoria
	if notification.Category != "" {
		if enabled, exists := prefs.CategoryPrefs[notification.Category]; exists && !enabled {
			return false
		}
	}

	return true
}

// isQuietHours verifica se siamo nelle ore di silenzio
func (ps *PushService) isQuietHours(prefs *UserPreferences) bool {
	if prefs.QuietHoursStart == "" || prefs.QuietHoursEnd == "" {
		return false
	}

	now := time.Now()
	currentTime := now.Format("15:04")

	// Gestisci il caso in cui le quiet hours attraversano la mezzanotte
	if prefs.QuietHoursStart > prefs.QuietHoursEnd {
		return currentTime >= prefs.QuietHoursStart || currentTime < prefs.QuietHoursEnd
	}

	return currentTime >= prefs.QuietHoursStart && currentTime < prefs.QuietHoursEnd
}

// SetUserPreferences imposta le preferenze utente
func (ps *PushService) SetUserPreferences(userID string, prefs *UserPreferences) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	prefs.UserID = userID
	prefs.UpdatedAt = time.Now()
	ps.preferences[userID] = prefs
}

// GetUserPreferences ottiene le preferenze utente
func (ps *PushService) GetUserPreferences(userID string) *UserPreferences {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if prefs, exists := ps.preferences[userID]; exists {
		return prefs
	}

	// Preferenze di default
	return &UserPreferences{
		UserID:           userID,
		Enabled:          true,
		SoundEnabled:     true,
		VibrationEnabled: true,
		BadgeEnabled:     true,
		PreviewEnabled:   true,
		GroupingEnabled:  true,
		CategoryPrefs:    make(map[string]bool),
	}
}

// AddTemplate aggiunge un template di notifica
func (ps *PushService) AddTemplate(template *NotificationTemplate) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.templates[template.ID] = template
}

// GetTemplate ottiene un template
func (ps *PushService) GetTemplate(templateID string) (*NotificationTemplate, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	template, exists := ps.templates[templateID]
	return template, exists
}

// interpolateTemplate sostituisce le variabili nel template
func interpolateTemplate(template string, data map[string]interface{}) string {
	result := template
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = fmt.Sprintf("%s", value)
	}
	return result
}

// mergeData unisce due map di dati
func mergeData(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// generateNotificationID genera un ID univoco per la notifica
func generateNotificationID() string {
	return fmt.Sprintf("notif_%d", time.Now().UnixNano())
}

// BroadcastNotification invia una notifica a tutti gli utenti (con cautela!)
func (ps *PushService) BroadcastNotification(ctx context.Context, notification *PushNotification, platform *Platform) error {
	ps.mu.RLock()
	tokens := make([]string, 0)
	for token, device := range ps.devices {
		if device.Enabled {
			if platform == nil || device.Platform == *platform {
				tokens = append(tokens, token)
			}
		}
	}
	ps.mu.RUnlock()

	if len(tokens) == 0 {
		return errors.New("no devices found for broadcast")
	}

	// Invia in batch per evitare sovraccarico
	batchSize := 1000
	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}

		batch := tokens[i:end]
		if err := ps.SendNotification(ctx, notification, batch); err != nil {
			return err
		}
	}

	return nil
}
