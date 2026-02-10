package security

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventType definisce il tipo di evento di sicurezza
type EventType string

const (
	EventTypeLogin            EventType = "login"
	EventTypeLogout           EventType = "logout"
	EventTypeFailedLogin      EventType = "failed_login"
	EventTypeAPIAccess        EventType = "api_access"
	EventTypeUnauthorized     EventType = "unauthorized"
	EventTypeSuspicious       EventType = "suspicious"
	EventTypeRateLimitHit     EventType = "rate_limit_hit"
	EventTypeIPBlocked        EventType = "ip_blocked"
	EventTypeDataAccess       EventType = "data_access"
	EventTypeDataModification EventType = "data_modification"
	EventTypeConfigChange     EventType = "config_change"
	EventTypeSecurityViolation EventType = "security_violation"
	EventTypeDDoS             EventType = "ddos_attempt"
)

// Severity definisce la severità dell'evento
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// AuditEvent rappresenta un evento di audit
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   EventType              `json:"event_type"`
	Severity    Severity               `json:"severity"`
	UserID      string                 `json:"user_id,omitempty"`
	IP          string                 `json:"ip,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CountryCode string                 `json:"country_code,omitempty"`
}

// AuditLogger gestisce il logging degli eventi di sicurezza
type AuditLogger struct {
	mu              sync.RWMutex
	logFile         *os.File
	events          []AuditEvent
	maxEvents       int
	alertThresholds map[EventType]int
	alertCounts     map[string]int // key: IP_EventType
	alertCallbacks  []AlertCallback
	anomalyDetector *AnomalyDetector
}

// AlertCallback è chiamato quando viene rilevata un'anomalia
type AlertCallback func(event AuditEvent)

// NewAuditLogger crea un nuovo audit logger
func NewAuditLogger(logFilePath string) (*AuditLogger, error) {
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	al := &AuditLogger{
		logFile:         file,
		events:          make([]AuditEvent, 0),
		maxEvents:       10000,
		alertThresholds: make(map[EventType]int),
		alertCounts:     make(map[string]int),
		alertCallbacks:  make([]AlertCallback, 0),
		anomalyDetector: NewAnomalyDetector(),
	}

	// Configura threshold di default
	al.SetAlertThreshold(EventTypeFailedLogin, 5)
	al.SetAlertThreshold(EventTypeUnauthorized, 10)
	al.SetAlertThreshold(EventTypeSuspicious, 3)
	al.SetAlertThreshold(EventTypeRateLimitHit, 10)

	return al, nil
}

// SetAlertThreshold imposta la soglia di alert per un tipo di evento
func (al *AuditLogger) SetAlertThreshold(eventType EventType, threshold int) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.alertThresholds[eventType] = threshold
}

// AddAlertCallback aggiunge un callback per gli alert
func (al *AuditLogger) AddAlertCallback(callback AlertCallback) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.alertCallbacks = append(al.alertCallbacks, callback)
}

// Log registra un evento di audit
func (al *AuditLogger) Log(event AuditEvent) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Genera ID se non presente
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Imposta timestamp se non presente
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Aggiungi alla lista in memoria
	al.events = append(al.events, event)

	// Limita la dimensione della lista
	if len(al.events) > al.maxEvents {
		al.events = al.events[len(al.events)-al.maxEvents:]
	}

	// Scrivi su file
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := al.logFile.Write(append(eventJSON, '\n')); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	// Verifica threshold e alert
	al.checkAlerts(event)

	// Rileva anomalie
	if al.anomalyDetector.IsAnomaly(event) {
		al.triggerAlert(event)
	}

	return nil
}

// checkAlerts verifica se un evento supera le soglie di alert
func (al *AuditLogger) checkAlerts(event AuditEvent) {
	threshold, exists := al.alertThresholds[event.EventType]
	if !exists {
		return
	}

	key := fmt.Sprintf("%s_%s", event.IP, event.EventType)
	al.alertCounts[key]++

	if al.alertCounts[key] >= threshold {
		al.triggerAlert(event)
		// Reset counter
		al.alertCounts[key] = 0
	}
}

// triggerAlert chiama tutti i callback di alert
func (al *AuditLogger) triggerAlert(event AuditEvent) {
	for _, callback := range al.alertCallbacks {
		go callback(event)
	}
}

// LogLogin registra un login
func (al *AuditLogger) LogLogin(userID, ip, userAgent string, success bool) error {
	eventType := EventTypeLogin
	severity := SeverityInfo
	message := "User login successful"

	if !success {
		eventType = EventTypeFailedLogin
		severity = SeverityWarning
		message = "User login failed"
	}

	return al.Log(AuditEvent{
		EventType: eventType,
		Severity:  severity,
		UserID:    userID,
		IP:        ip,
		UserAgent: userAgent,
		Message:   message,
		Action:    "login",
		Status:    statusFromBool(success),
	})
}

// LogAPIAccess registra un accesso API
func (al *AuditLogger) LogAPIAccess(userID, ip, resource, method string, statusCode int) error {
	severity := SeverityInfo
	if statusCode >= 400 {
		severity = SeverityWarning
	}
	if statusCode >= 500 {
		severity = SeverityError
	}

	return al.Log(AuditEvent{
		EventType: EventTypeAPIAccess,
		Severity:  severity,
		UserID:    userID,
		IP:        ip,
		Resource:  resource,
		Action:    method,
		Status:    fmt.Sprintf("%d", statusCode),
		Message:   fmt.Sprintf("API access: %s %s", method, resource),
	})
}

// LogUnauthorized registra un accesso non autorizzato
func (al *AuditLogger) LogUnauthorized(ip, resource, reason string) error {
	return al.Log(AuditEvent{
		EventType: EventTypeUnauthorized,
		Severity:  SeverityWarning,
		IP:        ip,
		Resource:  resource,
		Message:   fmt.Sprintf("Unauthorized access attempt: %s", reason),
		Metadata: map[string]interface{}{
			"reason": reason,
		},
	})
}

// LogSuspicious registra attività sospetta
func (al *AuditLogger) LogSuspicious(ip, userAgent, reason string, metadata map[string]interface{}) error {
	return al.Log(AuditEvent{
		EventType: EventTypeSuspicious,
		Severity:  SeverityError,
		IP:        ip,
		UserAgent: userAgent,
		Message:   fmt.Sprintf("Suspicious activity detected: %s", reason),
		Metadata:  metadata,
	})
}

// LogSecurityViolation registra una violazione di sicurezza
func (al *AuditLogger) LogSecurityViolation(ip, userID, violation string, metadata map[string]interface{}) error {
	return al.Log(AuditEvent{
		EventType: EventTypeSecurityViolation,
		Severity:  SeverityCritical,
		IP:        ip,
		UserID:    userID,
		Message:   fmt.Sprintf("Security violation: %s", violation),
		Metadata:  metadata,
	})
}

// LogDataAccess registra un accesso ai dati
func (al *AuditLogger) LogDataAccess(userID, ip, resource, action string) error {
	return al.Log(AuditEvent{
		EventType: EventTypeDataAccess,
		Severity:  SeverityInfo,
		UserID:    userID,
		IP:        ip,
		Resource:  resource,
		Action:    action,
		Message:   fmt.Sprintf("Data access: %s on %s", action, resource),
	})
}

// GetEvents ritorna gli eventi registrati
func (al *AuditLogger) GetEvents(filter EventFilter) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	filtered := make([]AuditEvent, 0)
	for _, event := range al.events {
		if filter.Matches(event) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// EventFilter filtra eventi di audit
type EventFilter struct {
	EventType *EventType
	Severity  *Severity
	UserID    string
	IP        string
	StartTime *time.Time
	EndTime   *time.Time
}

// Matches verifica se un evento corrisponde al filtro
func (ef EventFilter) Matches(event AuditEvent) bool {
	if ef.EventType != nil && event.EventType != *ef.EventType {
		return false
	}

	if ef.Severity != nil && event.Severity != *ef.Severity {
		return false
	}

	if ef.UserID != "" && event.UserID != ef.UserID {
		return false
	}

	if ef.IP != "" && event.IP != ef.IP {
		return false
	}

	if ef.StartTime != nil && event.Timestamp.Before(*ef.StartTime) {
		return false
	}

	if ef.EndTime != nil && event.Timestamp.After(*ef.EndTime) {
		return false
	}

	return true
}

// Close chiude l'audit logger
func (al *AuditLogger) Close() error {
	return al.logFile.Close()
}

// AnomalyDetector rileva anomalie nel comportamento
type AnomalyDetector struct {
	mu              sync.RWMutex
	patterns        map[string]*userPattern
	learningPeriod  time.Duration
	anomalyThreshold float64
}

type userPattern struct {
	requestCount    int
	avgRequestsPerHour float64
	commonResources map[string]int
	commonTimes     []int // ore del giorno
	lastSeen        time.Time
}

// NewAnomalyDetector crea un nuovo detector di anomalie
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		patterns:         make(map[string]*userPattern),
		learningPeriod:   24 * time.Hour,
		anomalyThreshold: 3.0, // 3x la media
	}
}

// IsAnomaly verifica se un evento è anomalo
func (ad *AnomalyDetector) IsAnomaly(event AuditEvent) bool {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	key := event.UserID
	if key == "" {
		key = event.IP
	}

	pattern, exists := ad.patterns[key]
	if !exists {
		// Crea nuovo pattern
		pattern = &userPattern{
			requestCount:    1,
			commonResources: make(map[string]int),
			commonTimes:     make([]int, 0),
			lastSeen:        time.Now(),
		}
		ad.patterns[key] = pattern
		return false
	}

	// Verifica se è trascorso il periodo di learning
	if time.Since(pattern.lastSeen) < ad.learningPeriod {
		// Ancora in fase di learning
		pattern.requestCount++
		if event.Resource != "" {
			pattern.commonResources[event.Resource]++
		}
		pattern.commonTimes = append(pattern.commonTimes, event.Timestamp.Hour())
		pattern.lastSeen = time.Now()
		return false
	}

	// Calcola media richieste per ora
	pattern.avgRequestsPerHour = float64(pattern.requestCount) / time.Since(pattern.lastSeen).Hours()

	// Verifica anomalie
	isAnomaly := false

	// Risorsa insolita
	if event.Resource != "" {
		if _, seen := pattern.commonResources[event.Resource]; !seen {
			isAnomaly = true
		}
	}

	// Orario insolito
	hour := event.Timestamp.Hour()
	hourSeen := false
	for _, h := range pattern.commonTimes {
		if h == hour {
			hourSeen = true
			break
		}
	}
	if !hourSeen {
		isAnomaly = true
	}

	// Aggiorna pattern
	pattern.requestCount++
	pattern.lastSeen = time.Now()

	return isAnomaly
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func statusFromBool(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}
