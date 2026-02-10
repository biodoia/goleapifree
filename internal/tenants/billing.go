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

// BillingCycle rappresenta il ciclo di fatturazione
type BillingCycle string

const (
	CycleMonthly BillingCycle = "monthly"
	CycleYearly  BillingCycle = "yearly"
)

// InvoiceStatus rappresenta lo stato di una fattura
type InvoiceStatus string

const (
	InvoiceStatusDraft    InvoiceStatus = "draft"
	InvoiceStatusPending  InvoiceStatus = "pending"
	InvoiceStatusPaid     InvoiceStatus = "paid"
	InvoiceStatusOverdue  InvoiceStatus = "overdue"
	InvoiceStatusCanceled InvoiceStatus = "canceled"
)

// PaymentStatus rappresenta lo stato di un pagamento
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// Invoice rappresenta una fattura
type Invoice struct {
	ID          uuid.UUID     `json:"id" gorm:"type:uuid;primary_key"`
	TenantID    uuid.UUID     `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Number      string        `json:"number" gorm:"uniqueIndex;not null"`
	Status      InvoiceStatus `json:"status" gorm:"not null;default:'draft'"`

	// Billing period
	PeriodStart time.Time `json:"period_start" gorm:"not null"`
	PeriodEnd   time.Time `json:"period_end" gorm:"not null"`

	// Amounts (in cents)
	SubtotalCents int64 `json:"subtotal_cents" gorm:"not null;default:0"`
	TaxCents      int64 `json:"tax_cents" gorm:"default:0"`
	TotalCents    int64 `json:"total_cents" gorm:"not null;default:0"`
	Currency      string `json:"currency" gorm:"not null;default:'USD'"`

	// Line items stored as JSON
	LineItems datatypes.JSON `json:"line_items" gorm:"type:jsonb"`
	// Example: [{"description": "Pro Plan", "quantity": 1, "unit_price": 2900, "total": 2900}]

	// Payment information
	PaidAt            *time.Time `json:"paid_at"`
	PaymentMethod     string     `json:"payment_method"`
	StripeInvoiceID   string     `json:"stripe_invoice_id"`
	StripePaymentID   string     `json:"stripe_payment_id"`

	// Dates
	DueDate   time.Time `json:"due_date" gorm:"not null"`
	IssuedAt  time.Time `json:"issued_at" gorm:"not null"`

	// Additional fields
	Notes    string         `json:"notes"`
	Metadata datatypes.JSON `json:"metadata" gorm:"type:jsonb"`

	// Relations
	Tenant   Tenant    `json:"tenant" gorm:"foreignKey:TenantID"`
	Payments []Payment `json:"payments,omitempty" gorm:"foreignKey:InvoiceID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Payment rappresenta un pagamento
type Payment struct {
	ID            uuid.UUID     `json:"id" gorm:"type:uuid;primary_key"`
	InvoiceID     uuid.UUID     `json:"invoice_id" gorm:"type:uuid;not null;index"`
	TenantID      uuid.UUID     `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Status        PaymentStatus `json:"status" gorm:"not null;default:'pending'"`

	// Amount (in cents)
	AmountCents int64  `json:"amount_cents" gorm:"not null"`
	Currency    string `json:"currency" gorm:"not null;default:'USD'"`

	// Payment provider
	Provider        string `json:"provider" gorm:"not null;default:'stripe'"`
	ProviderID      string `json:"provider_id"`
	PaymentMethod   string `json:"payment_method"`

	// Metadata
	FailureCode    string         `json:"failure_code"`
	FailureMessage string         `json:"failure_message"`
	Metadata       datatypes.JSON `json:"metadata" gorm:"type:jsonb"`

	ProcessedAt *time.Time `json:"processed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UsageRecord rappresenta un record di utilizzo
type UsageRecord struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	TenantID uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`

	// Usage metrics
	Requests int64 `json:"requests" gorm:"not null;default:0"`
	Tokens   int64 `json:"tokens" gorm:"not null;default:0"`
	Storage  int64 `json:"storage" gorm:"not null;default:0"` // in bytes

	// Period
	PeriodStart time.Time `json:"period_start" gorm:"not null;index"`
	PeriodEnd   time.Time `json:"period_end" gorm:"not null"`

	// Additional metrics
	Metadata datatypes.JSON `json:"metadata" gorm:"type:jsonb"`

	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate hook per Invoice
func (i *Invoice) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	if i.Number == "" {
		i.Number = fmt.Sprintf("INV-%s", i.ID.String()[:8])
	}
	if i.IssuedAt.IsZero() {
		i.IssuedAt = time.Now()
	}
	return nil
}

// BeforeCreate hook per Payment
func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook per UsageRecord
func (u *UsageRecord) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// TableName specifica il nome della tabella
func (Invoice) TableName() string {
	return "invoices"
}

// TableName specifica il nome della tabella
func (Payment) TableName() string {
	return "payments"
}

// TableName specifica il nome della tabella
func (UsageRecord) TableName() string {
	return "usage_records"
}

// IsOverdue verifica se la fattura è scaduta
func (i *Invoice) IsOverdue() bool {
	return i.Status == InvoiceStatusPending && time.Now().After(i.DueDate)
}

// MarkAsPaid segna la fattura come pagata
func (i *Invoice) MarkAsPaid() {
	now := time.Now()
	i.Status = InvoiceStatusPaid
	i.PaidAt = &now
}

// BillingManager gestisce la fatturazione dei tenant
type BillingManager struct {
	db *gorm.DB
}

// NewBillingManager crea un nuovo billing manager
func NewBillingManager(db *gorm.DB) *BillingManager {
	return &BillingManager{db: db}
}

// RecordUsage registra l'utilizzo per un tenant
func (bm *BillingManager) RecordUsage(ctx context.Context, tenantID uuid.UUID, requests, tokens, storage int64) error {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	// Check if record exists for this period
	var record UsageRecord
	err := bm.db.WithContext(ctx).
		Where("tenant_id = ? AND period_start = ?", tenantID, periodStart).
		First(&record).Error

	if err == gorm.ErrRecordNotFound {
		// Create new record
		record = UsageRecord{
			TenantID:    tenantID,
			Requests:    requests,
			Tokens:      tokens,
			Storage:     storage,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
		}
		return bm.db.WithContext(ctx).Create(&record).Error
	} else if err != nil {
		return fmt.Errorf("failed to get usage record: %w", err)
	}

	// Update existing record
	return bm.db.WithContext(ctx).
		Model(&record).
		Updates(map[string]interface{}{
			"requests": gorm.Expr("requests + ?", requests),
			"tokens":   gorm.Expr("tokens + ?", tokens),
			"storage":  gorm.Expr("storage + ?", storage),
		}).Error
}

// GetUsage ottiene l'utilizzo per un tenant in un periodo
func (bm *BillingManager) GetUsage(ctx context.Context, tenantID uuid.UUID, periodStart time.Time) (*UsageRecord, error) {
	var record UsageRecord
	err := bm.db.WithContext(ctx).
		Where("tenant_id = ? AND period_start = ?", tenantID, periodStart).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetUsageHistory ottiene lo storico utilizzo per un tenant
func (bm *BillingManager) GetUsageHistory(ctx context.Context, tenantID uuid.UUID, months int) ([]*UsageRecord, error) {
	var records []*UsageRecord
	startDate := time.Now().AddDate(0, -months, 0)

	err := bm.db.WithContext(ctx).
		Where("tenant_id = ? AND period_start >= ?", tenantID, startDate).
		Order("period_start DESC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GenerateInvoice genera una fattura per un tenant
func (bm *BillingManager) GenerateInvoice(ctx context.Context, tenantID uuid.UUID, periodStart, periodEnd time.Time) (*Invoice, error) {
	// Get tenant
	var tenant Tenant
	if err := bm.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get usage for period
	var usage UsageRecord
	err := bm.db.WithContext(ctx).
		Where("tenant_id = ? AND period_start = ?", tenantID, periodStart).
		First(&usage).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	// Calculate invoice amount based on plan and usage
	amount := bm.calculateInvoiceAmount(&tenant, &usage)

	// Generate invoice number
	invoiceNumber := bm.generateInvoiceNumber(tenantID)

	// Create invoice
	invoice := &Invoice{
		TenantID:      tenantID,
		Number:        invoiceNumber,
		Status:        InvoiceStatusPending,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		SubtotalCents: amount,
		TaxCents:      0, // Calculate tax based on tenant location
		TotalCents:    amount,
		Currency:      "USD",
		IssuedAt:      time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30), // 30 days
	}

	// Calculate tax (placeholder - implement actual tax calculation)
	invoice.TaxCents = invoice.SubtotalCents / 10 // 10% example
	invoice.TotalCents = invoice.SubtotalCents + invoice.TaxCents

	// Create line items
	lineItems := bm.buildLineItems(&tenant, &usage)
	if lineItemsJSON, err := datatypes.NewJSONType(lineItems).MarshalJSON(); err == nil {
		invoice.LineItems = lineItemsJSON
	}

	if err := bm.db.WithContext(ctx).Create(invoice).Error; err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	log.Info().
		Str("tenant_id", tenantID.String()).
		Str("invoice_id", invoice.ID.String()).
		Str("invoice_number", invoice.Number).
		Int64("total_cents", invoice.TotalCents).
		Msg("Invoice generated")

	return invoice, nil
}

// ProcessPayment processa un pagamento per una fattura
func (bm *BillingManager) ProcessPayment(ctx context.Context, invoiceID uuid.UUID, paymentProviderID string) (*Payment, error) {
	// Get invoice
	var invoice Invoice
	if err := bm.db.WithContext(ctx).First(&invoice, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	// Create payment record
	payment := &Payment{
		InvoiceID:   invoiceID,
		TenantID:    invoice.TenantID,
		Status:      PaymentStatusPending,
		AmountCents: invoice.TotalCents,
		Currency:    invoice.Currency,
		Provider:    "stripe",
		ProviderID:  paymentProviderID,
	}

	if err := bm.db.WithContext(ctx).Create(payment).Error; err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// Process payment with Stripe (placeholder)
	// In production, integrate with actual payment provider
	success := true // Placeholder

	if success {
		now := time.Now()
		payment.Status = PaymentStatusSucceeded
		payment.ProcessedAt = &now

		invoice.MarkAsPaid()
		invoice.StripePaymentID = paymentProviderID

		// Update both payment and invoice
		if err := bm.db.WithContext(ctx).Save(payment).Error; err != nil {
			return nil, fmt.Errorf("failed to update payment: %w", err)
		}
		if err := bm.db.WithContext(ctx).Save(&invoice).Error; err != nil {
			return nil, fmt.Errorf("failed to update invoice: %w", err)
		}

		log.Info().
			Str("invoice_id", invoiceID.String()).
			Str("payment_id", payment.ID.String()).
			Msg("Payment processed successfully")
	} else {
		payment.Status = PaymentStatusFailed
		payment.FailureMessage = "Payment declined"
		bm.db.WithContext(ctx).Save(payment)

		log.Error().
			Str("invoice_id", invoiceID.String()).
			Str("payment_id", payment.ID.String()).
			Msg("Payment failed")
	}

	return payment, nil
}

// GetInvoice ottiene una fattura per ID
func (bm *BillingManager) GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*Invoice, error) {
	var invoice Invoice
	err := bm.db.WithContext(ctx).
		Preload("Payments").
		First(&invoice, "id = ?", invoiceID).Error
	if err != nil {
		return nil, err
	}
	return &invoice, nil
}

// ListInvoices lista le fatture per un tenant
func (bm *BillingManager) ListInvoices(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]*Invoice, error) {
	var invoices []*Invoice
	err := bm.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&invoices).Error
	if err != nil {
		return nil, err
	}
	return invoices, nil
}

// CheckOverdueInvoices controlla e gestisce le fatture scadute
func (bm *BillingManager) CheckOverdueInvoices(ctx context.Context) error {
	var invoices []*Invoice
	err := bm.db.WithContext(ctx).
		Where("status = ? AND due_date < ?", InvoiceStatusPending, time.Now()).
		Find(&invoices).Error
	if err != nil {
		return fmt.Errorf("failed to find overdue invoices: %w", err)
	}

	for _, invoice := range invoices {
		invoice.Status = InvoiceStatusOverdue
		if err := bm.db.WithContext(ctx).Save(invoice).Error; err != nil {
			log.Error().Err(err).Str("invoice_id", invoice.ID.String()).
				Msg("Failed to mark invoice as overdue")
		} else {
			log.Warn().
				Str("invoice_id", invoice.ID.String()).
				Str("tenant_id", invoice.TenantID.String()).
				Msg("Invoice marked as overdue")

			// TODO: Send notification to tenant
		}
	}

	return nil
}

// generateInvoiceNumber genera un numero di fattura unico
func (bm *BillingManager) generateInvoiceNumber(tenantID uuid.UUID) string {
	now := time.Now()
	return fmt.Sprintf("INV-%s-%04d%02d",
		tenantID.String()[:8],
		now.Year(),
		now.Month(),
	)
}

// calculateInvoiceAmount calcola l'importo della fattura
func (bm *BillingManager) calculateInvoiceAmount(tenant *Tenant, usage *UsageRecord) int64 {
	// Base plan price (in cents)
	var basePrice int64
	switch tenant.Plan {
	case PlanFree:
		basePrice = 0
	case PlanStarter:
		basePrice = 2900 // $29.00
	case PlanPro:
		basePrice = 9900 // $99.00
	case PlanEnterprise:
		basePrice = 49900 // $499.00
	}

	// Add usage-based charges
	var usageCharges int64

	// Additional requests beyond quota
	if tenant.QuotaMaxRequests > 0 && usage.Requests > tenant.QuotaMaxRequests {
		overageRequests := usage.Requests - tenant.QuotaMaxRequests
		usageCharges += overageRequests * 1 // $0.01 per request
	}

	// Additional tokens beyond quota
	if tenant.QuotaMaxTokens > 0 && usage.Tokens > tenant.QuotaMaxTokens {
		overageTokens := usage.Tokens - tenant.QuotaMaxTokens
		usageCharges += (overageTokens / 1000) * 10 // $0.10 per 1K tokens
	}

	return basePrice + usageCharges
}

// buildLineItems costruisce le voci della fattura
func (bm *BillingManager) buildLineItems(tenant *Tenant, usage *UsageRecord) []map[string]interface{} {
	items := []map[string]interface{}{}

	// Base plan
	var planPrice int64
	switch tenant.Plan {
	case PlanStarter:
		planPrice = 2900
	case PlanPro:
		planPrice = 9900
	case PlanEnterprise:
		planPrice = 49900
	}

	if planPrice > 0 {
		items = append(items, map[string]interface{}{
			"description": fmt.Sprintf("%s Plan", tenant.Plan),
			"quantity":    1,
			"unit_price":  planPrice,
			"total":       planPrice,
		})
	}

	// Usage overages
	if tenant.QuotaMaxRequests > 0 && usage.Requests > tenant.QuotaMaxRequests {
		overageRequests := usage.Requests - tenant.QuotaMaxRequests
		total := overageRequests * 1
		items = append(items, map[string]interface{}{
			"description": fmt.Sprintf("Additional Requests (%d)", overageRequests),
			"quantity":    overageRequests,
			"unit_price":  1,
			"total":       total,
		})
	}

	return items
}
