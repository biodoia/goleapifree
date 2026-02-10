package notifications

import (
	"time"

	"github.com/google/uuid"
)

// EventType rappresenta il tipo di evento
type EventType string

const (
	EventProviderDown        EventType = "provider_down"
	EventQuotaExhausted      EventType = "quota_exhausted"
	EventHighErrorRate       EventType = "high_error_rate"
	EventCostThreshold       EventType = "cost_threshold"
	EventNewProviderDiscovered EventType = "new_provider_discovered"
	EventQuotaWarning        EventType = "quota_warning"
	EventProviderRecovered   EventType = "provider_recovered"
)

// Severity rappresenta la severità dell'evento
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Event rappresenta un evento del sistema
type Event interface {
	Type() EventType
	Severity() Severity
	Message() string
	Metadata() map[string]interface{}
	Timestamp() time.Time
}

// BaseEvent implementazione base di Event
type BaseEvent struct {
	EventType  EventType
	EventTime  time.Time
	Sev        Severity
	Msg        string
	Meta       map[string]interface{}
}

func (e *BaseEvent) Type() EventType {
	return e.EventType
}

func (e *BaseEvent) Severity() Severity {
	return e.Sev
}

func (e *BaseEvent) Message() string {
	return e.Msg
}

func (e *BaseEvent) Metadata() map[string]interface{} {
	return e.Meta
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.EventTime
}

// ProviderDownEvent evento quando un provider va down
type ProviderDownEvent struct {
	BaseEvent
	ProviderID   uuid.UUID
	ProviderName string
	DownSince    time.Time
	DownDuration time.Duration
	Reason       string
	LastError    string
}

// NewProviderDownEvent crea un nuovo ProviderDownEvent
func NewProviderDownEvent(providerID uuid.UUID, providerName string, downSince time.Time, reason, lastError string) *ProviderDownEvent {
	return &ProviderDownEvent{
		BaseEvent: BaseEvent{
			EventType: EventProviderDown,
			EventTime: time.Now(),
			Sev:       SeverityCritical,
			Msg:       "Provider is down",
			Meta: map[string]interface{}{
				"provider_id":   providerID.String(),
				"provider_name": providerName,
				"down_since":    downSince.Format(time.RFC3339),
				"down_duration": time.Since(downSince).String(),
			},
		},
		ProviderID:   providerID,
		ProviderName: providerName,
		DownSince:    downSince,
		DownDuration: time.Since(downSince),
		Reason:       reason,
		LastError:    lastError,
	}
}

// QuotaExhaustedEvent evento quando la quota è esaurita
type QuotaExhaustedEvent struct {
	BaseEvent
	AccountID    uuid.UUID
	ProviderID   uuid.UUID
	ProviderName string
	QuotaLimit   int64
	QuotaUsed    int64
	ResetAt      time.Time
}

// NewQuotaExhaustedEvent crea un nuovo QuotaExhaustedEvent
func NewQuotaExhaustedEvent(accountID, providerID uuid.UUID, providerName string, quotaLimit, quotaUsed int64, resetAt time.Time) *QuotaExhaustedEvent {
	return &QuotaExhaustedEvent{
		BaseEvent: BaseEvent{
			EventType: EventQuotaExhausted,
			EventTime: time.Now(),
			Sev:       SeverityError,
			Msg:       "Quota exhausted",
			Meta: map[string]interface{}{
				"account_id":    accountID.String(),
				"provider_id":   providerID.String(),
				"provider_name": providerName,
				"quota_limit":   quotaLimit,
				"quota_used":    quotaUsed,
				"reset_at":      resetAt.Format(time.RFC3339),
			},
		},
		AccountID:    accountID,
		ProviderID:   providerID,
		ProviderName: providerName,
		QuotaLimit:   quotaLimit,
		QuotaUsed:    quotaUsed,
		ResetAt:      resetAt,
	}
}

// HighErrorRateEvent evento quando il tasso di errori è alto
type HighErrorRateEvent struct {
	BaseEvent
	ProviderID   uuid.UUID
	ProviderName string
	ErrorRate    float64
	Threshold    float64
	ErrorCount   int64
	TotalRequests int64
	Window       time.Duration
}

// NewHighErrorRateEvent crea un nuovo HighErrorRateEvent
func NewHighErrorRateEvent(providerID uuid.UUID, providerName string, errorRate, threshold float64, errorCount, totalRequests int64, window time.Duration) *HighErrorRateEvent {
	return &HighErrorRateEvent{
		BaseEvent: BaseEvent{
			EventType: EventHighErrorRate,
			EventTime: time.Now(),
			Sev:       SeverityWarning,
			Msg:       "High error rate detected",
			Meta: map[string]interface{}{
				"provider_id":    providerID.String(),
				"provider_name":  providerName,
				"error_rate":     errorRate,
				"threshold":      threshold,
				"error_count":    errorCount,
				"total_requests": totalRequests,
				"window":         window.String(),
			},
		},
		ProviderID:    providerID,
		ProviderName:  providerName,
		ErrorRate:     errorRate,
		Threshold:     threshold,
		ErrorCount:    errorCount,
		TotalRequests: totalRequests,
		Window:        window,
	}
}

// CostThresholdEvent evento quando viene superata una soglia di costo
type CostThresholdEvent struct {
	BaseEvent
	ProviderID   uuid.UUID
	ProviderName string
	CurrentCost  float64
	Threshold    float64
	Period       string
	Currency     string
}

// NewCostThresholdEvent crea un nuovo CostThresholdEvent
func NewCostThresholdEvent(providerID uuid.UUID, providerName string, currentCost, threshold float64, period, currency string) *CostThresholdEvent {
	return &CostThresholdEvent{
		BaseEvent: BaseEvent{
			EventType: EventCostThreshold,
			EventTime: time.Now(),
			Sev:       SeverityWarning,
			Msg:       "Cost threshold exceeded",
			Meta: map[string]interface{}{
				"provider_id":   providerID.String(),
				"provider_name": providerName,
				"current_cost":  currentCost,
				"threshold":     threshold,
				"period":        period,
				"currency":      currency,
			},
		},
		ProviderID:   providerID,
		ProviderName: providerName,
		CurrentCost:  currentCost,
		Threshold:    threshold,
		Period:       period,
		Currency:     currency,
	}
}

// NewProviderDiscoveredEvent evento quando viene scoperto un nuovo provider
type NewProviderDiscoveredEvent struct {
	BaseEvent
	ProviderID   uuid.UUID
	ProviderName string
	BaseURL      string
	Source       string
	Capabilities []string
}

// NewNewProviderDiscoveredEvent crea un nuovo NewProviderDiscoveredEvent
func NewNewProviderDiscoveredEvent(providerID uuid.UUID, providerName, baseURL, source string, capabilities []string) *NewProviderDiscoveredEvent {
	return &NewProviderDiscoveredEvent{
		BaseEvent: BaseEvent{
			EventType: EventNewProviderDiscovered,
			EventTime: time.Now(),
			Sev:       SeverityInfo,
			Msg:       "New provider discovered",
			Meta: map[string]interface{}{
				"provider_id":   providerID.String(),
				"provider_name": providerName,
				"base_url":      baseURL,
				"source":        source,
				"capabilities":  capabilities,
			},
		},
		ProviderID:   providerID,
		ProviderName: providerName,
		BaseURL:      baseURL,
		Source:       source,
		Capabilities: capabilities,
	}
}

// QuotaWarningEvent evento quando la quota raggiunge una soglia di warning
type QuotaWarningEvent struct {
	BaseEvent
	AccountID    uuid.UUID
	ProviderID   uuid.UUID
	ProviderName string
	UsagePercent float64
	QuotaLimit   int64
	QuotaUsed    int64
	ResetAt      time.Time
}

// NewQuotaWarningEvent crea un nuovo QuotaWarningEvent
func NewQuotaWarningEvent(accountID, providerID uuid.UUID, providerName string, usagePercent float64, quotaLimit, quotaUsed int64, resetAt time.Time) *QuotaWarningEvent {
	return &QuotaWarningEvent{
		BaseEvent: BaseEvent{
			EventType: EventQuotaWarning,
			EventTime: time.Now(),
			Sev:       SeverityWarning,
			Msg:       "Quota warning threshold reached",
			Meta: map[string]interface{}{
				"account_id":     accountID.String(),
				"provider_id":    providerID.String(),
				"provider_name":  providerName,
				"usage_percent":  usagePercent,
				"quota_limit":    quotaLimit,
				"quota_used":     quotaUsed,
				"reset_at":       resetAt.Format(time.RFC3339),
			},
		},
		AccountID:    accountID,
		ProviderID:   providerID,
		ProviderName: providerName,
		UsagePercent: usagePercent,
		QuotaLimit:   quotaLimit,
		QuotaUsed:    quotaUsed,
		ResetAt:      resetAt,
	}
}

// ProviderRecoveredEvent evento quando un provider si riprende
type ProviderRecoveredEvent struct {
	BaseEvent
	ProviderID   uuid.UUID
	ProviderName string
	DownDuration time.Duration
	HealthScore  float64
}

// NewProviderRecoveredEvent crea un nuovo ProviderRecoveredEvent
func NewProviderRecoveredEvent(providerID uuid.UUID, providerName string, downDuration time.Duration, healthScore float64) *ProviderRecoveredEvent {
	return &ProviderRecoveredEvent{
		BaseEvent: BaseEvent{
			EventType: EventProviderRecovered,
			EventTime: time.Now(),
			Sev:       SeverityInfo,
			Msg:       "Provider recovered",
			Meta: map[string]interface{}{
				"provider_id":   providerID.String(),
				"provider_name": providerName,
				"down_duration": downDuration.String(),
				"health_score":  healthScore,
			},
		},
		ProviderID:   providerID,
		ProviderName: providerName,
		DownDuration: downDuration,
		HealthScore:  healthScore,
	}
}
