package notifications

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Rule rappresenta una regola di notifica
type Rule struct {
	ID          uuid.UUID
	Name        string
	EventType   EventType
	Enabled     bool
	Channels    []string // email, webhook, desktop, log
	Conditions  Conditions
	Cooldown    time.Duration
	UserPrefs   UserPreferences
	lastTrigger time.Time
	mu          sync.RWMutex
}

// Conditions rappresenta le condizioni per attivare una regola
type Conditions struct {
	MinSeverity      Severity
	ProviderIDs      []uuid.UUID // Se vuoto, applica a tutti
	ThresholdValue   float64
	ThresholdType    ThresholdType
	MinOccurrences   int
	TimeWindow       time.Duration
}

// ThresholdType tipo di soglia
type ThresholdType string

const (
	ThresholdNone         ThresholdType = "none"
	ThresholdGreaterThan  ThresholdType = "greater_than"
	ThresholdLessThan     ThresholdType = "less_than"
	ThresholdEquals       ThresholdType = "equals"
)

// UserPreferences preferenze utente per le notifiche
type UserPreferences struct {
	EmailEnabled    bool
	WebhookEnabled  bool
	DesktopEnabled  bool
	LogEnabled      bool
	QuietHoursStart string // HH:MM format
	QuietHoursEnd   string // HH:MM format
	MaxPerHour      int
}

// RuleEngine gestisce le regole di notifica
type RuleEngine struct {
	rules map[uuid.UUID]*Rule
	mu    sync.RWMutex

	// Tracking occorrenze
	occurrences map[string]*OccurrenceTracker
	occMu       sync.RWMutex

	// Rate limiting per user
	userRateLimits map[string]*RateLimiter
	rateMu         sync.RWMutex
}

// OccurrenceTracker traccia le occorrenze di eventi
type OccurrenceTracker struct {
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
}

// RateLimiter limita il numero di notifiche per periodo
type RateLimiter struct {
	Count     int
	Window    time.Time
	MaxPerWindow int
}

// NewRuleEngine crea un nuovo rule engine
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules:          make(map[uuid.UUID]*Rule),
		occurrences:    make(map[string]*OccurrenceTracker),
		userRateLimits: make(map[string]*RateLimiter),
	}
}

// AddRule aggiunge una regola
func (re *RuleEngine) AddRule(rule *Rule) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	re.rules[rule.ID] = rule
	log.Info().
		Str("rule_id", rule.ID.String()).
		Str("rule_name", rule.Name).
		Msg("Rule added")
}

// RemoveRule rimuove una regola
func (re *RuleEngine) RemoveRule(ruleID uuid.UUID) {
	re.mu.Lock()
	defer re.mu.Unlock()

	delete(re.rules, ruleID)
	log.Info().Str("rule_id", ruleID.String()).Msg("Rule removed")
}

// GetRule ottiene una regola
func (re *RuleEngine) GetRule(ruleID uuid.UUID) *Rule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	return re.rules[ruleID]
}

// ListRules lista tutte le regole
func (re *RuleEngine) ListRules() []*Rule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	rules := make([]*Rule, 0, len(re.rules))
	for _, rule := range re.rules {
		rules = append(rules, rule)
	}

	return rules
}

// ShouldNotify determina se una notifica dovrebbe essere inviata
func (re *RuleEngine) ShouldNotify(event Event, rule *Rule) bool {
	rule.mu.RLock()
	defer rule.mu.RUnlock()

	// Check se la regola è abilitata
	if !rule.Enabled {
		return false
	}

	// Check tipo di evento
	if rule.EventType != "" && rule.EventType != event.Type() {
		return false
	}

	// Check severità minima
	if !re.checkSeverity(event.Severity(), rule.Conditions.MinSeverity) {
		return false
	}

	// Check provider ID se specificato
	if len(rule.Conditions.ProviderIDs) > 0 {
		providerID := re.extractProviderID(event)
		if providerID != uuid.Nil && !re.containsProvider(rule.Conditions.ProviderIDs, providerID) {
			return false
		}
	}

	// Check cooldown
	if rule.Cooldown > 0 && time.Since(rule.lastTrigger) < rule.Cooldown {
		log.Debug().
			Str("rule_id", rule.ID.String()).
			Dur("cooldown", rule.Cooldown).
			Msg("Rule in cooldown period")
		return false
	}

	// Check occorrenze minime
	if rule.Conditions.MinOccurrences > 1 {
		occKey := re.getOccurrenceKey(event, rule)
		if !re.checkOccurrences(occKey, rule.Conditions.MinOccurrences, rule.Conditions.TimeWindow) {
			return false
		}
	}

	// Check quiet hours
	if !re.checkQuietHours(rule.UserPrefs) {
		log.Debug().
			Str("rule_id", rule.ID.String()).
			Msg("In quiet hours period")
		return false
	}

	// Check rate limit per user
	if rule.UserPrefs.MaxPerHour > 0 {
		if !re.checkRateLimit(rule.ID.String(), rule.UserPrefs.MaxPerHour) {
			log.Debug().
				Str("rule_id", rule.ID.String()).
				Int("max_per_hour", rule.UserPrefs.MaxPerHour).
				Msg("User rate limit exceeded")
			return false
		}
	}

	return true
}

// MarkTriggered marca una regola come triggata
func (re *RuleEngine) MarkTriggered(rule *Rule) {
	rule.mu.Lock()
	defer rule.mu.Unlock()

	rule.lastTrigger = time.Now()
}

// checkSeverity verifica se la severità è sufficiente
func (re *RuleEngine) checkSeverity(eventSev, minSev Severity) bool {
	severityLevel := map[Severity]int{
		SeverityInfo:     1,
		SeverityWarning:  2,
		SeverityError:    3,
		SeverityCritical: 4,
	}

	return severityLevel[eventSev] >= severityLevel[minSev]
}

// extractProviderID estrae il provider ID dall'evento
func (re *RuleEngine) extractProviderID(event Event) uuid.UUID {
	meta := event.Metadata()
	if providerIDStr, ok := meta["provider_id"].(string); ok {
		if providerID, err := uuid.Parse(providerIDStr); err == nil {
			return providerID
		}
	}
	return uuid.Nil
}

// containsProvider verifica se un provider è nella lista
func (re *RuleEngine) containsProvider(providers []uuid.UUID, providerID uuid.UUID) bool {
	for _, p := range providers {
		if p == providerID {
			return true
		}
	}
	return false
}

// getOccurrenceKey genera una chiave per il tracking delle occorrenze
func (re *RuleEngine) getOccurrenceKey(event Event, rule *Rule) string {
	return rule.ID.String() + ":" + string(event.Type())
}

// checkOccurrences verifica se ci sono abbastanza occorrenze
func (re *RuleEngine) checkOccurrences(key string, minOccurrences int, window time.Duration) bool {
	re.occMu.Lock()
	defer re.occMu.Unlock()

	tracker, exists := re.occurrences[key]
	if !exists {
		tracker = &OccurrenceTracker{
			Count:     1,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
		}
		re.occurrences[key] = tracker
		return minOccurrences <= 1
	}

	// Reset se fuori dalla finestra
	if time.Since(tracker.FirstSeen) > window {
		tracker.Count = 1
		tracker.FirstSeen = time.Now()
		tracker.LastSeen = time.Now()
		return minOccurrences <= 1
	}

	// Incrementa count
	tracker.Count++
	tracker.LastSeen = time.Now()

	return tracker.Count >= minOccurrences
}

// checkQuietHours verifica se siamo in quiet hours
func (re *RuleEngine) checkQuietHours(prefs UserPreferences) bool {
	if prefs.QuietHoursStart == "" || prefs.QuietHoursEnd == "" {
		return true
	}

	now := time.Now()
	currentTime := now.Format("15:04")

	// Se start < end: periodo normale (es. 22:00 - 08:00)
	// Se start > end: periodo che attraversa la mezzanotte
	if prefs.QuietHoursStart < prefs.QuietHoursEnd {
		return currentTime < prefs.QuietHoursStart || currentTime >= prefs.QuietHoursEnd
	}

	return currentTime < prefs.QuietHoursStart && currentTime >= prefs.QuietHoursEnd
}

// checkRateLimit verifica il rate limit
func (re *RuleEngine) checkRateLimit(userID string, maxPerHour int) bool {
	re.rateMu.Lock()
	defer re.rateMu.Unlock()

	limiter, exists := re.userRateLimits[userID]
	if !exists {
		limiter = &RateLimiter{
			Count:        1,
			Window:       time.Now(),
			MaxPerWindow: maxPerHour,
		}
		re.userRateLimits[userID] = limiter
		return true
	}

	// Reset se è passata un'ora
	if time.Since(limiter.Window) >= time.Hour {
		limiter.Count = 1
		limiter.Window = time.Now()
		return true
	}

	// Incrementa count
	limiter.Count++

	return limiter.Count <= maxPerHour
}

// GetActiveChannels ottiene i canali attivi per una regola
func (re *RuleEngine) GetActiveChannels(rule *Rule) []string {
	channels := make([]string, 0)

	for _, ch := range rule.Channels {
		switch ch {
		case "email":
			if rule.UserPrefs.EmailEnabled {
				channels = append(channels, ch)
			}
		case "webhook":
			if rule.UserPrefs.WebhookEnabled {
				channels = append(channels, ch)
			}
		case "desktop":
			if rule.UserPrefs.DesktopEnabled {
				channels = append(channels, ch)
			}
		case "log":
			if rule.UserPrefs.LogEnabled {
				channels = append(channels, ch)
			}
		}
	}

	return channels
}

// DefaultRules crea le regole di default
func DefaultRules() []*Rule {
	return []*Rule{
		{
			ID:        uuid.New(),
			Name:      "Provider Down > 5min",
			EventType: EventProviderDown,
			Enabled:   true,
			Channels:  []string{"email", "webhook", "log"},
			Conditions: Conditions{
				MinSeverity:    SeverityCritical,
				ThresholdValue: 5 * 60, // 5 minuti in secondi
				ThresholdType:  ThresholdGreaterThan,
			},
			Cooldown: 15 * time.Minute,
			UserPrefs: UserPreferences{
				EmailEnabled:   true,
				WebhookEnabled: true,
				LogEnabled:     true,
				MaxPerHour:     10,
			},
		},
		{
			ID:        uuid.New(),
			Name:      "Quota > 90%",
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
		},
		{
			ID:        uuid.New(),
			Name:      "Error Rate > 10%",
			EventType: EventHighErrorRate,
			Enabled:   true,
			Channels:  []string{"email", "webhook", "log"},
			Conditions: Conditions{
				MinSeverity:    SeverityWarning,
				ThresholdValue: 0.10,
				ThresholdType:  ThresholdGreaterThan,
				MinOccurrences: 3,
				TimeWindow:     10 * time.Minute,
			},
			Cooldown: 30 * time.Minute,
			UserPrefs: UserPreferences{
				EmailEnabled:   true,
				WebhookEnabled: true,
				LogEnabled:     true,
				MaxPerHour:     10,
			},
		},
		{
			ID:        uuid.New(),
			Name:      "New Provider Discovered",
			EventType: EventNewProviderDiscovered,
			Enabled:   true,
			Channels:  []string{"email", "log"},
			Conditions: Conditions{
				MinSeverity: SeverityInfo,
			},
			Cooldown: 0,
			UserPrefs: UserPreferences{
				EmailEnabled: true,
				LogEnabled:   true,
				MaxPerHour:   20,
			},
		},
		{
			ID:        uuid.New(),
			Name:      "Quota Exhausted",
			EventType: EventQuotaExhausted,
			Enabled:   true,
			Channels:  []string{"email", "webhook", "log"},
			Conditions: Conditions{
				MinSeverity: SeverityError,
			},
			Cooldown: 2 * time.Hour,
			UserPrefs: UserPreferences{
				EmailEnabled:   true,
				WebhookEnabled: true,
				LogEnabled:     true,
				MaxPerHour:     5,
			},
		},
	}
}
