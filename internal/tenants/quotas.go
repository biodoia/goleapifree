package tenants

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// QuotaType rappresenta il tipo di quota
type QuotaType string

const (
	QuotaTypeRequests QuotaType = "requests"
	QuotaTypeTokens   QuotaType = "tokens"
	QuotaTypeUsers    QuotaType = "users"
	QuotaTypeAPIKeys  QuotaType = "api_keys"
	QuotaTypeStorage  QuotaType = "storage"
)

// QuotaCheckResult rappresenta il risultato di un controllo quota
type QuotaCheckResult struct {
	Allowed       bool      `json:"allowed"`
	QuotaType     QuotaType `json:"quota_type"`
	Current       int64     `json:"current"`
	Limit         int64     `json:"limit"`
	Remaining     int64     `json:"remaining"`
	UsagePercent  float64   `json:"usage_percent"`
	ResetAt       time.Time `json:"reset_at"`
	WarningLevel  bool      `json:"warning_level"`  // True if > 80%
	CriticalLevel bool      `json:"critical_level"` // True if > 95%
}

// QuotaOverageAction rappresenta l'azione da intraprendere per un overage
type QuotaOverageAction string

const (
	ActionAllow      QuotaOverageAction = "allow"       // Permetti e addebita
	ActionThrottle   QuotaOverageAction = "throttle"    // Rallenta le richieste
	ActionBlock      QuotaOverageAction = "block"       // Blocca le richieste
	ActionNotify     QuotaOverageAction = "notify"      // Notifica e continua
)

// QuotaConfig configurazione per la gestione delle quote
type QuotaConfig struct {
	WarningThreshold  float64            // Soglia di warning (es. 0.8 = 80%)
	CriticalThreshold float64            // Soglia critica (es. 0.95 = 95%)
	OverageAction     QuotaOverageAction // Azione da intraprendere per overage
	OverageMultiplier float64            // Moltiplicatore per addebito overage (es. 1.5 = +50%)
	EnableSoftLimits  bool               // Permetti soft limits
}

// QuotaManager gestisce le quote dei tenant
type QuotaManager struct {
	db              *gorm.DB
	config          QuotaConfig
	billingManager  *BillingManager
	mu              sync.RWMutex

	// Callbacks
	onWarning       func(tenant *Tenant, quotaType QuotaType, percent float64)
	onCritical      func(tenant *Tenant, quotaType QuotaType, percent float64)
	onOverage       func(tenant *Tenant, quotaType QuotaType, overage int64)
	onQuotaExceeded func(tenant *Tenant, quotaType QuotaType)
}

// NewQuotaManager crea un nuovo quota manager
func NewQuotaManager(db *gorm.DB, billingManager *BillingManager, config QuotaConfig) *QuotaManager {
	if config.WarningThreshold == 0 {
		config.WarningThreshold = 0.8
	}
	if config.CriticalThreshold == 0 {
		config.CriticalThreshold = 0.95
	}
	if config.OverageMultiplier == 0 {
		config.OverageMultiplier = 1.5
	}

	qm := &QuotaManager{
		db:             db,
		config:         config,
		billingManager: billingManager,
	}

	// Start periodic quota check
	go qm.periodicQuotaCheck()

	return qm
}

// SetCallbacks imposta le callback per gli eventi di quota
func (qm *QuotaManager) SetCallbacks(
	onWarning func(*Tenant, QuotaType, float64),
	onCritical func(*Tenant, QuotaType, float64),
	onOverage func(*Tenant, QuotaType, int64),
	onQuotaExceeded func(*Tenant, QuotaType),
) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.onWarning = onWarning
	qm.onCritical = onCritical
	qm.onOverage = onOverage
	qm.onQuotaExceeded = onQuotaExceeded
}

// CheckQuota verifica se un tenant può utilizzare una determinata quota
func (qm *QuotaManager) CheckQuota(ctx context.Context, tenantID uuid.UUID, quotaType QuotaType, amount int64) (*QuotaCheckResult, error) {
	// Get tenant
	var tenant Tenant
	if err := qm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	var current, limit int64

	// Get current usage and limit based on quota type
	switch quotaType {
	case QuotaTypeRequests:
		current = tenant.UsageRequests
		limit = tenant.QuotaMaxRequests

	case QuotaTypeTokens:
		current = tenant.UsageTokens
		limit = tenant.QuotaMaxTokens

	case QuotaTypeUsers:
		// Would need to count users - placeholder
		current = 0
		limit = int64(tenant.QuotaMaxUsers)

	case QuotaTypeAPIKeys:
		// Would need to count API keys - placeholder
		current = 0
		limit = int64(tenant.QuotaMaxAPIKeys)

	case QuotaTypeStorage:
		current = tenant.UsageStorageBytes
		limit = int64(tenant.QuotaStorageGB) * 1024 * 1024 * 1024

	default:
		return nil, fmt.Errorf("unsupported quota type: %s", quotaType)
	}

	// Check if quota is unlimited
	if limit == 0 {
		return &QuotaCheckResult{
			Allowed:   true,
			QuotaType: quotaType,
			Current:   current,
			Limit:     0,
			Remaining: -1, // Unlimited
			ResetAt:   tenant.UsageResetAt,
		}, nil
	}

	// Calculate usage
	newUsage := current + amount
	remaining := limit - newUsage
	usagePercent := float64(newUsage) / float64(limit)

	result := &QuotaCheckResult{
		QuotaType:     quotaType,
		Current:       current,
		Limit:         limit,
		Remaining:     remaining,
		UsagePercent:  usagePercent * 100,
		ResetAt:       tenant.UsageResetAt,
		WarningLevel:  usagePercent >= qm.config.WarningThreshold,
		CriticalLevel: usagePercent >= qm.config.CriticalThreshold,
	}

	// Check if allowed based on quota
	if newUsage <= limit {
		result.Allowed = true

		// Trigger warning or critical callbacks
		if result.CriticalLevel {
			qm.triggerCritical(&tenant, quotaType, usagePercent)
		} else if result.WarningLevel {
			qm.triggerWarning(&tenant, quotaType, usagePercent)
		}

		return result, nil
	}

	// Quota exceeded - determine action
	overage := newUsage - limit

	switch qm.config.OverageAction {
	case ActionAllow:
		// Allow with overage charges
		result.Allowed = true
		qm.triggerOverage(&tenant, quotaType, overage)

	case ActionNotify:
		// Allow but notify
		result.Allowed = true
		qm.triggerOverage(&tenant, quotaType, overage)

	case ActionThrottle:
		// Throttle but allow (implementation would slow down requests)
		result.Allowed = true

	case ActionBlock:
		// Block the request
		result.Allowed = false
		qm.triggerQuotaExceeded(&tenant, quotaType)

	default:
		result.Allowed = false
	}

	return result, nil
}

// ConsumeQuota consuma una quota per un tenant
func (qm *QuotaManager) ConsumeQuota(ctx context.Context, tenantID uuid.UUID, quotaType QuotaType, amount int64) error {
	// Check quota first
	result, err := qm.CheckQuota(ctx, tenantID, quotaType, amount)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("quota exceeded for %s", quotaType)
	}

	// Update usage based on quota type
	updates := make(map[string]interface{})

	switch quotaType {
	case QuotaTypeRequests:
		updates["usage_requests"] = gorm.Expr("usage_requests + ?", amount)

	case QuotaTypeTokens:
		updates["usage_tokens"] = gorm.Expr("usage_tokens + ?", amount)

	case QuotaTypeStorage:
		updates["usage_storage_bytes"] = gorm.Expr("usage_storage_bytes + ?", amount)

	default:
		return fmt.Errorf("unsupported quota type for consumption: %s", quotaType)
	}

	// Update tenant
	if err := qm.db.WithContext(ctx).
		Model(&Tenant{}).
		Where("id = ?", tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to consume quota: %w", err)
	}

	// Record usage for billing
	if qm.billingManager != nil {
		var requests, tokens, storage int64
		switch quotaType {
		case QuotaTypeRequests:
			requests = amount
		case QuotaTypeTokens:
			tokens = amount
		case QuotaTypeStorage:
			storage = amount
		}

		go func() {
			if err := qm.billingManager.RecordUsage(context.Background(), tenantID, requests, tokens, storage); err != nil {
				log.Error().Err(err).
					Str("tenant_id", tenantID.String()).
					Msg("Failed to record usage for billing")
			}
		}()
	}

	return nil
}

// GetQuotaStatus ottiene lo status di tutte le quote per un tenant
func (qm *QuotaManager) GetQuotaStatus(ctx context.Context, tenantID uuid.UUID) (map[QuotaType]*QuotaCheckResult, error) {
	// Get tenant
	var tenant Tenant
	if err := qm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	status := make(map[QuotaType]*QuotaCheckResult)

	// Check all quota types
	quotaTypes := []QuotaType{
		QuotaTypeRequests,
		QuotaTypeTokens,
		QuotaTypeStorage,
	}

	for _, quotaType := range quotaTypes {
		result, err := qm.CheckQuota(ctx, tenantID, quotaType, 0)
		if err != nil {
			log.Error().Err(err).
				Str("tenant_id", tenantID.String()).
				Str("quota_type", string(quotaType)).
				Msg("Failed to check quota")
			continue
		}
		status[quotaType] = result
	}

	return status, nil
}

// ResetQuota resetta le quote per un tenant
func (qm *QuotaManager) ResetQuota(ctx context.Context, tenantID uuid.UUID) error {
	var tenant Tenant
	if err := qm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	tenant.ResetUsage()

	if err := qm.db.WithContext(ctx).Save(&tenant).Error; err != nil {
		return fmt.Errorf("failed to reset quota: %w", err)
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Msg("Quota reset")

	return nil
}

// UpgradePlan gestisce l'upgrade del piano con nuove quote
func (qm *QuotaManager) UpgradePlan(ctx context.Context, tenantID uuid.UUID, newPlan TenantPlan) error {
	var tenant Tenant
	if err := qm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	oldPlan := tenant.Plan
	tenant.Plan = newPlan

	// Update quotas based on new plan
	manager := NewManager(qm.db)
	manager.setPlanQuotas(&tenant)

	if err := qm.db.WithContext(ctx).Save(&tenant).Error; err != nil {
		return fmt.Errorf("failed to upgrade plan: %w", err)
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Str("old_plan", string(oldPlan)).
		Str("new_plan", string(newPlan)).
		Msg("Plan upgraded")

	return nil
}

// CalculateOverageCharges calcola i costi per l'overage
func (qm *QuotaManager) CalculateOverageCharges(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var tenant Tenant
	if err := qm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return 0, fmt.Errorf("failed to get tenant: %w", err)
	}

	var totalCharges int64

	// Calculate overage for requests
	if tenant.QuotaMaxRequests > 0 && tenant.UsageRequests > tenant.QuotaMaxRequests {
		overage := tenant.UsageRequests - tenant.QuotaMaxRequests
		// $0.01 per request * overage multiplier
		charges := int64(float64(overage) * qm.config.OverageMultiplier)
		totalCharges += charges
	}

	// Calculate overage for tokens
	if tenant.QuotaMaxTokens > 0 && tenant.UsageTokens > tenant.QuotaMaxTokens {
		overage := tenant.UsageTokens - tenant.QuotaMaxTokens
		// $0.10 per 1K tokens * overage multiplier
		charges := int64(float64(overage/1000) * 10 * qm.config.OverageMultiplier)
		totalCharges += charges
	}

	return totalCharges, nil
}

// periodicQuotaCheck controlla periodicamente le quote scadute
func (qm *QuotaManager) periodicQuotaCheck() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()

		// Find tenants with expired quotas
		var tenants []*Tenant
		if err := qm.db.WithContext(ctx).
			Where("usage_reset_at < ? AND active = ?", time.Now(), true).
			Find(&tenants).Error; err != nil {
			log.Error().Err(err).Msg("Failed to find tenants for quota reset")
			continue
		}

		for _, tenant := range tenants {
			if err := qm.ResetQuota(ctx, tenant.ID); err != nil {
				log.Error().Err(err).
					Str("tenant_id", tenant.ID.String()).
					Msg("Failed to reset quota")
			}
		}

		if len(tenants) > 0 {
			log.Info().Int("count", len(tenants)).Msg("Periodic quota reset completed")
		}
	}
}

// triggerWarning notifica warning threshold
func (qm *QuotaManager) triggerWarning(tenant *Tenant, quotaType QuotaType, percent float64) {
	qm.mu.RLock()
	callback := qm.onWarning
	qm.mu.RUnlock()

	if callback != nil {
		go callback(tenant, quotaType, percent)
	}

	log.Warn().
		Str("tenant_id", tenant.ID.String()).
		Str("quota_type", string(quotaType)).
		Float64("usage_percent", percent*100).
		Msg("Quota warning threshold reached")
}

// triggerCritical notifica critical threshold
func (qm *QuotaManager) triggerCritical(tenant *Tenant, quotaType QuotaType, percent float64) {
	qm.mu.RLock()
	callback := qm.onCritical
	qm.mu.RUnlock()

	if callback != nil {
		go callback(tenant, quotaType, percent)
	}

	log.Error().
		Str("tenant_id", tenant.ID.String()).
		Str("quota_type", string(quotaType)).
		Float64("usage_percent", percent*100).
		Msg("Quota critical threshold reached")
}

// triggerOverage notifica overage
func (qm *QuotaManager) triggerOverage(tenant *Tenant, quotaType QuotaType, overage int64) {
	qm.mu.RLock()
	callback := qm.onOverage
	qm.mu.RUnlock()

	if callback != nil {
		go callback(tenant, quotaType, overage)
	}

	log.Warn().
		Str("tenant_id", tenant.ID.String()).
		Str("quota_type", string(quotaType)).
		Int64("overage", overage).
		Msg("Quota overage detected")
}

// triggerQuotaExceeded notifica quota exceeded
func (qm *QuotaManager) triggerQuotaExceeded(tenant *Tenant, quotaType QuotaType) {
	qm.mu.RLock()
	callback := qm.onQuotaExceeded
	qm.mu.RUnlock()

	if callback != nil {
		go callback(tenant, quotaType)
	}

	log.Error().
		Str("tenant_id", tenant.ID.String()).
		Str("quota_type", string(quotaType)).
		Msg("Quota exceeded")
}
