package tenants

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&Tenant{}, &Invoice{}, &Payment{}, &UsageRecord{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func TestTenantCreation(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	ctx := context.Background()

	tenant := &Tenant{
		Name:       "Test Company",
		Subdomain:  "testco",
		Plan:       PlanStarter,
		OwnerID:    uuid.New(),
		OwnerEmail: "owner@testco.com",
	}

	err := manager.Create(ctx, tenant)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	if tenant.ID == uuid.Nil {
		t.Error("tenant ID should be set")
	}

	if tenant.DatabaseName == "" {
		t.Error("database name should be set")
	}

	// Verify quotas are set based on plan
	if tenant.QuotaMaxUsers != 10 {
		t.Errorf("expected QuotaMaxUsers to be 10, got %d", tenant.QuotaMaxUsers)
	}
}

func TestTenantGetBySubdomain(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	ctx := context.Background()

	tenant := &Tenant{
		Name:       "Test Company",
		Subdomain:  "testco",
		Plan:       PlanPro,
		OwnerID:    uuid.New(),
		OwnerEmail: "owner@testco.com",
	}

	if err := manager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	retrieved, err := manager.GetBySubdomain(ctx, "testco")
	if err != nil {
		t.Fatalf("failed to get tenant by subdomain: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Error("retrieved tenant ID doesn't match")
	}
}

func TestTenantPlanUpgrade(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	ctx := context.Background()

	tenant := &Tenant{
		Name:       "Test Company",
		Subdomain:  "testco",
		Plan:       PlanStarter,
		OwnerID:    uuid.New(),
		OwnerEmail: "owner@testco.com",
	}

	if err := manager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Upgrade to Pro
	if err := manager.UpdatePlan(ctx, tenant.ID, PlanPro); err != nil {
		t.Fatalf("failed to upgrade plan: %v", err)
	}

	// Verify upgrade
	updated, err := manager.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("failed to get updated tenant: %v", err)
	}

	if updated.Plan != PlanPro {
		t.Errorf("expected plan to be Pro, got %s", updated.Plan)
	}

	// Verify quotas were updated
	if updated.QuotaMaxUsers != 50 {
		t.Errorf("expected QuotaMaxUsers to be 50, got %d", updated.QuotaMaxUsers)
	}
}

func TestTenantQuotaChecks(t *testing.T) {
	db := setupTestDB(t)
	tenant := &Tenant{
		Name:            "Test Company",
		Subdomain:       "testco",
		Plan:            PlanStarter,
		OwnerID:         uuid.New(),
		OwnerEmail:      "owner@testco.com",
		QuotaMaxRequests: 1000,
		UsageRequests:   500,
	}

	if !tenant.CanMakeRequest() {
		t.Error("tenant should be able to make requests")
	}

	tenant.UsageRequests = 1000
	if tenant.CanMakeRequest() {
		t.Error("tenant should not be able to make requests")
	}
}

func TestUsageRecording(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	ctx := context.Background()

	tenant := &Tenant{
		Name:       "Test Company",
		Subdomain:  "testco",
		Plan:       PlanStarter,
		OwnerID:    uuid.New(),
		OwnerEmail: "owner@testco.com",
	}

	if err := manager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Record usage
	if err := manager.RecordUsage(ctx, tenant.ID, 100, 1000); err != nil {
		t.Fatalf("failed to record usage: %v", err)
	}

	// Verify usage was recorded
	updated, err := manager.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("failed to get updated tenant: %v", err)
	}

	if updated.UsageRequests != 100 {
		t.Errorf("expected UsageRequests to be 100, got %d", updated.UsageRequests)
	}

	if updated.UsageTokens != 1000 {
		t.Errorf("expected UsageTokens to be 1000, got %d", updated.UsageTokens)
	}
}

func TestBillingInvoiceGeneration(t *testing.T) {
	db := setupTestDB(t)
	billingManager := NewBillingManager(db)
	tenantManager := NewManager(db)
	ctx := context.Background()

	// Create tenant
	tenant := &Tenant{
		Name:       "Test Company",
		Subdomain:  "testco",
		Plan:       PlanStarter,
		OwnerID:    uuid.New(),
		OwnerEmail: "owner@testco.com",
	}

	if err := tenantManager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Record usage
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	if err := billingManager.RecordUsage(ctx, tenant.ID, 1000, 10000, 0); err != nil {
		t.Fatalf("failed to record usage: %v", err)
	}

	// Generate invoice
	invoice, err := billingManager.GenerateInvoice(ctx, tenant.ID, periodStart, periodEnd)
	if err != nil {
		t.Fatalf("failed to generate invoice: %v", err)
	}

	if invoice.ID == uuid.Nil {
		t.Error("invoice ID should be set")
	}

	if invoice.Number == "" {
		t.Error("invoice number should be set")
	}

	if invoice.TotalCents == 0 {
		t.Error("invoice total should not be zero")
	}

	if invoice.Status != InvoiceStatusPending {
		t.Errorf("expected invoice status to be pending, got %s", invoice.Status)
	}
}

func TestQuotaManager(t *testing.T) {
	db := setupTestDB(t)
	billingManager := NewBillingManager(db)
	quotaManager := NewQuotaManager(db, billingManager, QuotaConfig{
		WarningThreshold:  0.8,
		CriticalThreshold: 0.95,
		OverageAction:     ActionAllow,
	})
	tenantManager := NewManager(db)
	ctx := context.Background()

	// Create tenant
	tenant := &Tenant{
		Name:            "Test Company",
		Subdomain:       "testco",
		Plan:            PlanStarter,
		OwnerID:         uuid.New(),
		OwnerEmail:      "owner@testco.com",
		QuotaMaxRequests: 1000,
		UsageRequests:   0,
	}

	if err := tenantManager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Check quota - should be allowed
	result, err := quotaManager.CheckQuota(ctx, tenant.ID, QuotaTypeRequests, 100)
	if err != nil {
		t.Fatalf("failed to check quota: %v", err)
	}

	if !result.Allowed {
		t.Error("quota should be allowed")
	}

	if result.WarningLevel {
		t.Error("should not be at warning level yet")
	}

	// Consume quota
	if err := quotaManager.ConsumeQuota(ctx, tenant.ID, QuotaTypeRequests, 900); err != nil {
		t.Fatalf("failed to consume quota: %v", err)
	}

	// Check quota again - should be at warning level
	result, err = quotaManager.CheckQuota(ctx, tenant.ID, QuotaTypeRequests, 0)
	if err != nil {
		t.Fatalf("failed to check quota: %v", err)
	}

	if !result.WarningLevel {
		t.Error("should be at warning level")
	}

	// Try to exceed quota
	result, err = quotaManager.CheckQuota(ctx, tenant.ID, QuotaTypeRequests, 200)
	if err != nil {
		t.Fatalf("failed to check quota: %v", err)
	}

	// With ActionAllow, should still be allowed
	if !result.Allowed {
		t.Error("with ActionAllow, overage should be allowed")
	}
}

func TestQuotaReset(t *testing.T) {
	db := setupTestDB(t)
	tenantManager := NewManager(db)
	ctx := context.Background()

	// Create tenant
	tenant := &Tenant{
		Name:          "Test Company",
		Subdomain:     "testco",
		Plan:          PlanStarter,
		OwnerID:       uuid.New(),
		OwnerEmail:    "owner@testco.com",
		UsageRequests: 500,
		UsageTokens:   5000,
	}

	if err := tenantManager.Create(ctx, tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Verify usage is set
	if tenant.UsageRequests != 500 {
		t.Error("usage should be 500")
	}

	// Get usage percent
	requestsPercent, tokensPercent := tenant.GetUsagePercent()
	if requestsPercent == 0 {
		t.Error("requests percent should not be zero")
	}
	if tokensPercent == 0 {
		t.Error("tokens percent should not be zero")
	}

	// Reset usage
	tenant.ResetUsage()
	if err := tenantManager.Update(ctx, tenant); err != nil {
		t.Fatalf("failed to update tenant: %v", err)
	}

	// Verify reset
	if tenant.UsageRequests != 0 {
		t.Error("usage should be reset to 0")
	}

	if tenant.UsageTokens != 0 {
		t.Error("tokens should be reset to 0")
	}
}
