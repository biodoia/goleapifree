package tenants

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ContextKey tipo per le chiavi del context
type ContextKey string

const (
	// TenantIDKey chiave per l'ID tenant nel context
	TenantIDKey ContextKey = "tenant_id"
	// TenantKey chiave per il tenant completo nel context
	TenantKey ContextKey = "tenant"
)

// ExtractionStrategy strategia per estrarre il tenant dalla richiesta
type ExtractionStrategy string

const (
	// StrategySubdomain estrae il tenant dal subdomain
	StrategySubdomain ExtractionStrategy = "subdomain"
	// StrategyDomain estrae il tenant dal dominio custom
	StrategyDomain ExtractionStrategy = "domain"
	// StrategyHeader estrae il tenant dall'header X-Tenant-ID
	StrategyHeader ExtractionStrategy = "header"
	// StrategyPath estrae il tenant dal path (es. /tenant/{id}/...)
	StrategyPath ExtractionStrategy = "path"
)

// MiddlewareConfig configurazione del middleware tenant
type MiddlewareConfig struct {
	Manager           *Manager
	Strategy          ExtractionStrategy
	BaseDomain        string // es. "goleapai.com" per subdomain.goleapai.com
	RequireTenant     bool   // Se true, richiede sempre un tenant valido
	ValidateStatus    bool   // Se true, valida che il tenant sia attivo
	CheckQuotas       bool   // Se true, verifica le quote prima di procedere
	AllowedPaths      []string // Paths che non richiedono tenant
}

// Middleware middleware per gestire i tenant
func Middleware(config MiddlewareConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()

		// Check if path is allowed without tenant
		path := c.Path()
		for _, allowedPath := range config.AllowedPaths {
			if strings.HasPrefix(path, allowedPath) {
				return c.Next()
			}
		}

		// Extract tenant based on strategy
		var tenant *Tenant
		var err error

		switch config.Strategy {
		case StrategySubdomain:
			tenant, err = extractTenantFromSubdomain(c, config)
		case StrategyDomain:
			tenant, err = extractTenantFromDomain(c, config)
		case StrategyHeader:
			tenant, err = extractTenantFromHeader(c, config)
		case StrategyPath:
			tenant, err = extractTenantFromPath(c, config)
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "invalid tenant extraction strategy",
			})
		}

		if err != nil {
			log.Debug().Err(err).Str("strategy", string(config.Strategy)).
				Msg("Failed to extract tenant")

			if config.RequireTenant {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "tenant identification required",
				})
			}
			return c.Next()
		}

		// Validate tenant status
		if config.ValidateStatus {
			if !tenant.IsActive() {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "tenant is not active",
					"status": tenant.Status,
				})
			}

			if tenant.IsTrialExpired() {
				return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
					"error": "trial period has expired",
				})
			}

			if tenant.IsSubscriptionExpired() {
				return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
					"error": "subscription has expired",
				})
			}
		}

		// Check quotas
		if config.CheckQuotas {
			if !tenant.CanMakeRequest() {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error": "monthly request quota exceeded",
					"quota_used": tenant.UsageRequests,
					"quota_limit": tenant.QuotaMaxRequests,
					"reset_at": tenant.UsageResetAt,
				})
			}
		}

		// Inject tenant into context
		ctx = context.WithValue(ctx, TenantIDKey, tenant.ID.String())
		ctx = context.WithValue(ctx, TenantKey, tenant)
		c.SetContext(ctx)

		// Add response headers
		c.Set("X-Tenant-ID", tenant.ID.String())
		c.Set("X-Tenant-Plan", string(tenant.Plan))

		log.Debug().
			Str("tenant_id", tenant.ID.String()).
			Str("subdomain", tenant.Subdomain).
			Str("path", path).
			Msg("Tenant identified")

		return c.Next()
	}
}

// extractTenantFromSubdomain estrae il tenant dal subdomain
func extractTenantFromSubdomain(c fiber.Ctx, config MiddlewareConfig) (*Tenant, error) {
	host := c.Hostname()

	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Extract subdomain
	if !strings.HasSuffix(host, config.BaseDomain) {
		return nil, fmt.Errorf("invalid domain: %s", host)
	}

	subdomain := strings.TrimSuffix(host, "."+config.BaseDomain)
	if subdomain == "" || subdomain == host {
		return nil, fmt.Errorf("no subdomain found")
	}

	// Get tenant by subdomain
	tenant, err := config.Manager.GetBySubdomain(c.Context(), subdomain)
	if err != nil {
		return nil, fmt.Errorf("tenant not found for subdomain: %s", subdomain)
	}

	return tenant, nil
}

// extractTenantFromDomain estrae il tenant dal dominio custom
func extractTenantFromDomain(c fiber.Ctx, config MiddlewareConfig) (*Tenant, error) {
	host := c.Hostname()

	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Get tenant by domain
	tenant, err := config.Manager.GetByDomain(c.Context(), host)
	if err != nil {
		return nil, fmt.Errorf("tenant not found for domain: %s", host)
	}

	return tenant, nil
}

// extractTenantFromHeader estrae il tenant dall'header
func extractTenantFromHeader(c fiber.Ctx, config MiddlewareConfig) (*Tenant, error) {
	tenantID := c.Get("X-Tenant-ID")
	if tenantID == "" {
		return nil, fmt.Errorf("missing X-Tenant-ID header")
	}

	id, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID format: %s", tenantID)
	}

	tenant, err := config.Manager.GetByID(c.Context(), id)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	return tenant, nil
}

// extractTenantFromPath estrae il tenant dal path
func extractTenantFromPath(c fiber.Ctx, config MiddlewareConfig) (*Tenant, error) {
	tenantID := c.Params("tenant_id")
	if tenantID == "" {
		return nil, fmt.Errorf("missing tenant_id in path")
	}

	id, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID format: %s", tenantID)
	}

	tenant, err := config.Manager.GetByID(c.Context(), id)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	return tenant, nil
}

// RequireTenant middleware che richiede un tenant valido
func RequireTenant() fiber.Handler {
	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tenant context required",
			})
		}
		return c.Next()
	}
}

// RequireActiveTenant middleware che richiede un tenant attivo
func RequireActiveTenant() fiber.Handler {
	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tenant context required",
			})
		}

		if !tenant.IsActive() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "tenant is not active",
				"status": tenant.Status,
			})
		}

		return c.Next()
	}
}

// RequirePlan middleware che richiede un piano specifico o superiore
func RequirePlan(minPlan TenantPlan) fiber.Handler {
	planOrder := map[TenantPlan]int{
		PlanFree:       0,
		PlanStarter:    1,
		PlanPro:        2,
		PlanEnterprise: 3,
	}

	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tenant context required",
			})
		}

		currentOrder := planOrder[tenant.Plan]
		requiredOrder := planOrder[minPlan]

		if currentOrder < requiredOrder {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient plan",
				"current_plan": tenant.Plan,
				"required_plan": minPlan,
			})
		}

		return c.Next()
	}
}

// IsolationMiddleware middleware per prevenire cross-tenant data access
func IsolationMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			log.Warn().
				Str("path", c.Path()).
				Str("method", c.Method()).
				Msg("Request without tenant context")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tenant isolation required",
			})
		}

		// Inject tenant ID into all database queries
		// This would be handled by the database layer
		log.Debug().
			Str("tenant_id", tenant.ID.String()).
			Str("path", c.Path()).
			Msg("Tenant isolation enforced")

		return c.Next()
	}
}

// RateLimitByTenant middleware per rate limiting per tenant
func RateLimitByTenant() fiber.Handler {
	// This would integrate with a rate limiter
	// For now, just check the tenant's configured limits
	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			return c.Next()
		}

		// Add rate limit headers
		c.Set("X-RateLimit-Limit-RPM", fmt.Sprintf("%d", tenant.QuotaRateLimitRPM))
		c.Set("X-RateLimit-Limit-RPS", fmt.Sprintf("%d", tenant.QuotaRateLimitRPS))

		// Actual rate limiting would be implemented here
		// using Redis or similar

		return c.Next()
	}
}

// UsageTrackingMiddleware middleware per tracciare l'utilizzo
func UsageTrackingMiddleware(manager *Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		tenant := GetTenant(c)
		if tenant == nil {
			return c.Next()
		}

		// Process request
		err := c.Next()

		// Track usage after request completes
		go func() {
			// In a real implementation, you would extract tokens from response
			tokens := int64(0) // Placeholder
			requests := int64(1)

			if err := manager.RecordUsage(context.Background(), tenant.ID, requests, tokens); err != nil {
				log.Error().Err(err).
					Str("tenant_id", tenant.ID.String()).
					Msg("Failed to record tenant usage")
			}
		}()

		return err
	}
}

// GetTenant estrae il tenant dal context
func GetTenant(c fiber.Ctx) *Tenant {
	tenant := c.Context().Value(TenantKey)
	if tenant == nil {
		return nil
	}
	t, ok := tenant.(*Tenant)
	if !ok {
		return nil
	}
	return t
}

// GetTenantID estrae l'ID tenant dal context
func GetTenantID(c fiber.Ctx) (uuid.UUID, error) {
	tenantID := c.Context().Value(TenantIDKey)
	if tenantID == nil {
		return uuid.Nil, fmt.Errorf("tenant ID not found in context")
	}
	return uuid.Parse(tenantID.(string))
}

// MustGetTenant estrae il tenant dal context (panic se non trovato)
func MustGetTenant(c fiber.Ctx) *Tenant {
	tenant := GetTenant(c)
	if tenant == nil {
		panic("tenant not found in context")
	}
	return tenant
}

// WithTenant inietta un tenant nel context (utile per testing)
func WithTenant(c fiber.Ctx, tenant *Tenant) {
	ctx := context.WithValue(c.Context(), TenantIDKey, tenant.ID.String())
	ctx = context.WithValue(ctx, TenantKey, tenant)
	c.SetContext(ctx)
}
