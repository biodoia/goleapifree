package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAccount_BeforeCreate(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		wantErr bool
	}{
		{
			name: "generates UUID and sets LastReset",
			account: &Account{
				UserID:     uuid.New(),
				ProviderID: uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "keeps existing UUID",
			account: &Account{
				ID:         uuid.New(),
				UserID:     uuid.New(),
				ProviderID: uuid.New(),
				LastReset:  time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.account.ID
			originalLastReset := tt.account.LastReset

			err := tt.account.BeforeCreate()

			if (err != nil) != tt.wantErr {
				t.Errorf("BeforeCreate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.account.ID == uuid.Nil {
				t.Error("ID should not be nil after BeforeCreate()")
			}

			if originalID != uuid.Nil && tt.account.ID != originalID {
				t.Error("Existing ID should not be changed")
			}

			if tt.account.LastReset.IsZero() {
				t.Error("LastReset should be set after BeforeCreate()")
			}

			if !originalLastReset.IsZero() && tt.account.LastReset != originalLastReset {
				t.Error("Existing LastReset should not be changed")
			}
		})
	}
}

func TestAccount_IsQuotaAvailable(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "unlimited quota",
			account: &Account{
				QuotaLimit: 0,
				QuotaUsed:  1000,
			},
			want: true,
		},
		{
			name: "quota available",
			account: &Account{
				QuotaLimit: 10000,
				QuotaUsed:  5000,
			},
			want: true,
		},
		{
			name: "quota at limit",
			account: &Account{
				QuotaLimit: 10000,
				QuotaUsed:  10000,
			},
			want: false,
		},
		{
			name: "quota exceeded",
			account: &Account{
				QuotaLimit: 10000,
				QuotaUsed:  15000,
			},
			want: false,
		},
		{
			name: "zero quota used",
			account: &Account{
				QuotaLimit: 10000,
				QuotaUsed:  0,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.IsQuotaAvailable(); got != tt.want {
				t.Errorf("IsQuotaAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "no expiration set",
			account: &Account{
				ExpiresAt: time.Time{},
			},
			want: false,
		},
		{
			name: "not expired yet",
			account: &Account{
				ExpiresAt: now.Add(24 * time.Hour),
			},
			want: false,
		},
		{
			name: "expired",
			account: &Account{
				ExpiresAt: now.Add(-24 * time.Hour),
			},
			want: true,
		},
		{
			name: "expires right now",
			account: &Account{
				ExpiresAt: now.Add(-1 * time.Second),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_TableName(t *testing.T) {
	a := Account{}
	if got := a.TableName(); got != "accounts" {
		t.Errorf("TableName() = %v, want accounts", got)
	}
}

// Integration test for account lifecycle
func TestAccount_Lifecycle(t *testing.T) {
	account := &Account{
		UserID:     uuid.New(),
		ProviderID: uuid.New(),
		QuotaLimit: 10000,
		Active:     true,
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	// Test creation
	if err := account.BeforeCreate(); err != nil {
		t.Fatalf("BeforeCreate() failed: %v", err)
	}

	// Verify initial state
	if account.ID == uuid.Nil {
		t.Error("ID not generated")
	}

	if account.LastReset.IsZero() {
		t.Error("LastReset not set")
	}

	// Test quota check
	if !account.IsQuotaAvailable() {
		t.Error("New account should have quota available")
	}

	// Test expiration check
	if account.IsExpired() {
		t.Error("Account should not be expired")
	}

	// Simulate quota consumption
	account.QuotaUsed = 9999
	if !account.IsQuotaAvailable() {
		t.Error("Account should still have quota available")
	}

	account.QuotaUsed = 10000
	if account.IsQuotaAvailable() {
		t.Error("Account should have no quota available")
	}
}

// Benchmark tests
func BenchmarkAccount_IsQuotaAvailable(b *testing.B) {
	account := &Account{
		QuotaLimit: 10000,
		QuotaUsed:  5000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = account.IsQuotaAvailable()
	}
}

func BenchmarkAccount_IsExpired(b *testing.B) {
	account := &Account{
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = account.IsExpired()
	}
}
