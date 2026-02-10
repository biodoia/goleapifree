package tenants

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TenantStatus rappresenta lo stato di un tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusCanceled  TenantStatus = "canceled"
	TenantStatusTrial     TenantStatus = "trial"
)

// TenantPlan rappresenta il piano di un tenant
type TenantPlan string

const (
	PlanFree       TenantPlan = "free"
	PlanStarter    TenantPlan = "starter"
	PlanPro        TenantPlan = "pro"
	PlanEnterprise TenantPlan = "enterprise"
)

// Tenant rappresenta un tenant nel sistema multi-tenant
type Tenant struct {
	ID        uuid.UUID    `json:"id" gorm:"type:uuid;primary_key"`
	Name      string       `json:"name" gorm:"not null;index"`
	Subdomain string       `json:"subdomain" gorm:"uniqueIndex;not null"`
	Domain    string       `json:"domain" gorm:"uniqueIndex"`
	Status    TenantStatus `json:"status" gorm:"not null;default:'trial';index"`
	Plan      TenantPlan   `json:"plan" gorm:"not null;default:'free'"`

	// Owner information
	OwnerID    uuid.UUID `json:"owner_id" gorm:"type:uuid;not null;index"`
	OwnerEmail string    `json:"owner_email" gorm:"not null"`

	// Database isolation
	DatabaseName   string `json:"database_name" gorm:"not null;uniqueIndex"`
	DatabaseHost   string `json:"database_host" gorm:"not null"`
	DatabaseSchema string `json:"database_schema"`

	// Settings and metadata
	Settings datatypes.JSON `json:"settings" gorm:"type:jsonb"`
	Metadata datatypes.JSON `json:"metadata" gorm:"type:jsonb"`
	// Example settings: {"features": ["ai", "analytics"], "logo_url": "...", "theme": "dark"}

	// Billing information
	StripeCustomerID     string     `json:"stripe_customer_id"`
	StripeSubscriptionID string     `json:"stripe_subscription_id"`
	BillingEmail         string     `json:"billing_email"`
	TrialEndsAt          *time.Time `json:"trial_ends_at"`
	SubscriptionEndsAt   *time.Time `json:"subscription_ends_at"`

	// Quotas (per tenant limits)
	QuotaMaxUsers       int   `json:"quota_max_users" gorm:"default:5"`
	QuotaMaxAPIKeys     int   `json:"quota_max_api_keys" gorm:"default:10"`
	QuotaMaxRequests    int64 `json:"quota_max_requests" gorm:"default:10000"`   // Per month
	QuotaMaxTokens      int64 `json:"quota_max_tokens" gorm:"default:1000000"`   // Per month
	QuotaStorageGB      int   `json:"quota_storage_gb" gorm:"default:10"`        // GB
	QuotaRateLimitRPM   int   `json:"quota_rate_limit_rpm" gorm:"default:60"`    // Requests per minute
	QuotaRateLimitRPS   int   `json:"quota_rate_limit_rps" gorm:"default:10"`    // Requests per second

	// Current usage (reset monthly)
	UsageRequests     int64     `json:"usage_requests" gorm:"default:0"`
	UsageTokens       int64     `json:"usage_tokens" gorm:"default:0"`
	UsageStorageBytes int64     `json:"usage_storage_bytes" gorm:"default:0"`
	UsageResetAt      time.Time `json:"usage_reset_at"`

	// Status flags
	Active          bool `json:"active" gorm:"default:true;index"`
	IsolationEnabled bool `json:"isolation_enabled" gorm:"default:true"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// BeforeCreate hook
func (t *Tenant) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.DatabaseName == "" {
		t.DatabaseName = fmt.Sprintf("tenant_%s", t.ID.String()[:8])
	}
	if t.UsageResetAt.IsZero() {
		t.UsageResetAt = time.Now().AddDate(0, 1, 0) // Reset monthly
	}
	return nil
}

// TableName specifica il nome della tabella
func (Tenant) TableName() string {
	return "tenants"
}

// IsActive verifica se il tenant è attivo
func (t *Tenant) IsActive() bool {
	return t.Active && t.Status == TenantStatusActive
}

// IsTrialExpired verifica se il trial è scaduto
func (t *Tenant) IsTrialExpired() bool {
	if t.Status != TenantStatusTrial {
		return false
	}
	return t.TrialEndsAt != nil && time.Now().After(*t.TrialEndsAt)
}

// IsSubscriptionExpired verifica se la subscription è scaduta
func (t *Tenant) IsSubscriptionExpired() bool {
	return t.SubscriptionEndsAt != nil && time.Now().After(*t.SubscriptionEndsAt)
}

// HasFeature verifica se il tenant ha una feature
func (t *Tenant) HasFeature(feature string) bool {
	if t.Settings == nil {
		return false
	}
	// Parse settings JSON to check features
	// This is a placeholder - implement actual JSON parsing
	return true
}

// CanCreateUser verifica se il tenant può creare un nuovo utente
func (t *Tenant) CanCreateUser(currentUserCount int) bool {
	if t.QuotaMaxUsers == 0 {
		return true // Unlimited
	}
	return currentUserCount < t.QuotaMaxUsers
}

// CanMakeRequest verifica se il tenant può fare una richiesta
func (t *Tenant) CanMakeRequest() bool {
	if t.QuotaMaxRequests == 0 {
		return true // Unlimited
	}
	return t.UsageRequests < t.QuotaMaxRequests
}

// CanUseTokens verifica se il tenant può usare token
func (t *Tenant) CanUseTokens(tokens int64) bool {
	if t.QuotaMaxTokens == 0 {
		return true // Unlimited
	}
	return t.UsageTokens+tokens <= t.QuotaMaxTokens
}

// IncrementUsage incrementa l'utilizzo del tenant
func (t *Tenant) IncrementUsage(requests int64, tokens int64) {
	t.UsageRequests += requests
	t.UsageTokens += tokens
}

// ResetUsage resetta l'utilizzo mensile
func (t *Tenant) ResetUsage() {
	t.UsageRequests = 0
	t.UsageTokens = 0
	t.UsageResetAt = time.Now().AddDate(0, 1, 0)
}

// GetUsagePercent calcola la percentuale di utilizzo
func (t *Tenant) GetUsagePercent() (requests float64, tokens float64) {
	if t.QuotaMaxRequests > 0 {
		requests = float64(t.UsageRequests) / float64(t.QuotaMaxRequests) * 100
	}
	if t.QuotaMaxTokens > 0 {
		tokens = float64(t.UsageTokens) / float64(t.QuotaMaxTokens) * 100
	}
	return
}

// Manager gestisce i tenant
type Manager struct {
	db *gorm.DB
}

// NewManager crea un nuovo tenant manager
func NewManager(db *gorm.DB) *Manager {
	return &Manager{db: db}
}

// Create crea un nuovo tenant
func (m *Manager) Create(ctx context.Context, tenant *Tenant) error {
	if err := m.validateTenant(tenant); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check subdomain uniqueness
	var count int64
	if err := m.db.WithContext(ctx).Model(&Tenant{}).
		Where("subdomain = ?", tenant.Subdomain).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check subdomain: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("subdomain already exists: %s", tenant.Subdomain)
	}

	// Set plan-specific quotas
	m.setPlanQuotas(tenant)

	// Create tenant
	if err := m.db.WithContext(ctx).Create(tenant).Error; err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	// Initialize tenant database if isolation is enabled
	if tenant.IsolationEnabled {
		if err := m.initializeTenantDatabase(ctx, tenant); err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID.String()).
				Msg("Failed to initialize tenant database")
			// Don't fail tenant creation, just log the error
		}
	}

	log.Info().
		Str("tenant_id", tenant.ID.String()).
		Str("subdomain", tenant.Subdomain).
		Str("plan", string(tenant.Plan)).
		Msg("Tenant created")

	return nil
}

// GetByID ottiene un tenant per ID
func (m *Manager) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	var tenant Tenant
	if err := m.db.WithContext(ctx).First(&tenant, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &tenant, nil
}

// GetBySubdomain ottiene un tenant per subdomain
func (m *Manager) GetBySubdomain(ctx context.Context, subdomain string) (*Tenant, error) {
	var tenant Tenant
	if err := m.db.WithContext(ctx).
		Where("subdomain = ? AND active = ?", subdomain, true).
		First(&tenant).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant by subdomain: %w", err)
	}
	return &tenant, nil
}

// GetByDomain ottiene un tenant per dominio custom
func (m *Manager) GetByDomain(ctx context.Context, domain string) (*Tenant, error) {
	var tenant Tenant
	if err := m.db.WithContext(ctx).
		Where("domain = ? AND active = ?", domain, true).
		First(&tenant).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant by domain: %w", err)
	}
	return &tenant, nil
}

// Update aggiorna un tenant
func (m *Manager) Update(ctx context.Context, tenant *Tenant) error {
	if err := m.validateTenant(tenant); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := m.db.WithContext(ctx).Save(tenant).Error; err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	log.Info().
		Str("tenant_id", tenant.ID.String()).
		Msg("Tenant updated")

	return nil
}

// UpdatePlan aggiorna il piano di un tenant
func (m *Manager) UpdatePlan(ctx context.Context, tenantID uuid.UUID, newPlan TenantPlan) error {
	tenant, err := m.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	oldPlan := tenant.Plan
	tenant.Plan = newPlan
	m.setPlanQuotas(tenant)

	if err := m.Update(ctx, tenant); err != nil {
		return err
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Str("old_plan", string(oldPlan)).
		Str("new_plan", string(newPlan)).
		Msg("Tenant plan updated")

	return nil
}

// Suspend sospende un tenant
func (m *Manager) Suspend(ctx context.Context, tenantID uuid.UUID, reason string) error {
	tenant, err := m.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	tenant.Status = TenantStatusSuspended
	tenant.Active = false

	if err := m.Update(ctx, tenant); err != nil {
		return err
	}

	log.Warn().
		Str("tenant_id", tenantID.String()).
		Str("reason", reason).
		Msg("Tenant suspended")

	return nil
}

// Activate attiva un tenant
func (m *Manager) Activate(ctx context.Context, tenantID uuid.UUID) error {
	tenant, err := m.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	tenant.Status = TenantStatusActive
	tenant.Active = true

	if err := m.Update(ctx, tenant); err != nil {
		return err
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Msg("Tenant activated")

	return nil
}

// Delete elimina un tenant (soft delete)
func (m *Manager) Delete(ctx context.Context, tenantID uuid.UUID) error {
	if err := m.db.WithContext(ctx).Delete(&Tenant{}, "id = ?", tenantID).Error; err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Msg("Tenant deleted")

	return nil
}

// List lista tutti i tenant con paginazione
func (m *Manager) List(ctx context.Context, offset, limit int) ([]*Tenant, error) {
	var tenants []*Tenant

	query := m.db.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC")

	if err := query.Find(&tenants).Error; err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	return tenants, nil
}

// GetByOwner ottiene tutti i tenant di un owner
func (m *Manager) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Tenant, error) {
	var tenants []*Tenant
	if err := m.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("created_at DESC").
		Find(&tenants).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenants by owner: %w", err)
	}
	return tenants, nil
}

// RecordUsage registra l'utilizzo per un tenant
func (m *Manager) RecordUsage(ctx context.Context, tenantID uuid.UUID, requests int64, tokens int64) error {
	result := m.db.WithContext(ctx).
		Model(&Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"usage_requests": gorm.Expr("usage_requests + ?", requests),
			"usage_tokens":   gorm.Expr("usage_tokens + ?", tokens),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to record usage: %w", result.Error)
	}

	return nil
}

// CheckQuotaExpiration controlla e resetta le quote scadute
func (m *Manager) CheckQuotaExpiration(ctx context.Context) error {
	var tenants []*Tenant
	if err := m.db.WithContext(ctx).
		Where("usage_reset_at < ?", time.Now()).
		Find(&tenants).Error; err != nil {
		return fmt.Errorf("failed to find expired quotas: %w", err)
	}

	for _, tenant := range tenants {
		tenant.ResetUsage()
		if err := m.Update(ctx, tenant); err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID.String()).
				Msg("Failed to reset tenant quota")
		} else {
			log.Info().Str("tenant_id", tenant.ID.String()).
				Msg("Tenant quota reset")
		}
	}

	return nil
}

// validateTenant valida i dati del tenant
func (m *Manager) validateTenant(tenant *Tenant) error {
	if tenant.Name == "" {
		return fmt.Errorf("tenant name is required")
	}
	if tenant.Subdomain == "" {
		return fmt.Errorf("tenant subdomain is required")
	}
	if tenant.OwnerID == uuid.Nil {
		return fmt.Errorf("tenant owner_id is required")
	}
	if tenant.OwnerEmail == "" {
		return fmt.Errorf("tenant owner_email is required")
	}
	return nil
}

// setPlanQuotas imposta le quote in base al piano
func (m *Manager) setPlanQuotas(tenant *Tenant) {
	switch tenant.Plan {
	case PlanFree:
		tenant.QuotaMaxUsers = 3
		tenant.QuotaMaxAPIKeys = 5
		tenant.QuotaMaxRequests = 10000
		tenant.QuotaMaxTokens = 100000
		tenant.QuotaStorageGB = 1
		tenant.QuotaRateLimitRPM = 20
		tenant.QuotaRateLimitRPS = 2

	case PlanStarter:
		tenant.QuotaMaxUsers = 10
		tenant.QuotaMaxAPIKeys = 20
		tenant.QuotaMaxRequests = 100000
		tenant.QuotaMaxTokens = 1000000
		tenant.QuotaStorageGB = 10
		tenant.QuotaRateLimitRPM = 100
		tenant.QuotaRateLimitRPS = 10

	case PlanPro:
		tenant.QuotaMaxUsers = 50
		tenant.QuotaMaxAPIKeys = 100
		tenant.QuotaMaxRequests = 1000000
		tenant.QuotaMaxTokens = 10000000
		tenant.QuotaStorageGB = 100
		tenant.QuotaRateLimitRPM = 500
		tenant.QuotaRateLimitRPS = 50

	case PlanEnterprise:
		tenant.QuotaMaxUsers = 0 // Unlimited
		tenant.QuotaMaxAPIKeys = 0
		tenant.QuotaMaxRequests = 0
		tenant.QuotaMaxTokens = 0
		tenant.QuotaStorageGB = 0
		tenant.QuotaRateLimitRPM = 0
		tenant.QuotaRateLimitRPS = 0
	}
}

// initializeTenantDatabase inizializza il database per un tenant
func (m *Manager) initializeTenantDatabase(ctx context.Context, tenant *Tenant) error {
	// This is a placeholder for actual database initialization
	// In production, you would:
	// 1. Create a new database or schema
	// 2. Run migrations on the tenant database
	// 3. Seed initial data
	log.Info().
		Str("tenant_id", tenant.ID.String()).
		Str("database_name", tenant.DatabaseName).
		Msg("Initializing tenant database")

	return nil
}
