package middleware

import (
	"testing"
	"time"

	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func TestRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	app.Get("/test", func(c fiber.Ctx) error {
		requestID := GetRequestID(c)
		if requestID == "" {
			t.Error("Request ID should not be empty")
		}
		return c.SendString("OK")
	})

	// TODO: Add actual HTTP test
}

func TestRecovery(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryWithLogger())

	app.Get("/panic", func(c fiber.Ctx) error {
		panic("test panic")
	})

	// TODO: Add actual HTTP test to verify recovery
}

func TestJWTManager(t *testing.T) {
	manager := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:       "test-secret",
		Issuer:          "test",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 24 * time.Hour,
	})

	userID := uuid.New().String()
	email := "test@example.com"
	role := "user"

	// Test access token generation
	token, err := manager.GenerateAccessToken(userID, email, role)
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	// Test token validation
	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("Expected email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("Expected role %s, got %s", role, claims.Role)
	}

	// Test refresh token
	refreshToken, err := manager.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	validatedUserID, err := manager.ValidateRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("Failed to validate refresh token: %v", err)
	}

	if validatedUserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, validatedUserID)
	}
}

func TestAPIKeyManager(t *testing.T) {
	manager := auth.NewAPIKeyManager()
	userID := uuid.New()

	// Test key generation
	apiKey, plainKey, err := manager.GenerateAPIKey(
		userID,
		"Test Key",
		[]string{"read", "write"},
		60,
		365*24*time.Hour,
	)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if apiKey.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, apiKey.UserID)
	}

	if plainKey == "" {
		t.Error("Plain key should not be empty")
	}

	// Test key validation
	err = manager.ValidateAPIKey(plainKey, apiKey)
	if err != nil {
		t.Errorf("Failed to validate API key: %v", err)
	}

	// Test invalid key
	err = manager.ValidateAPIKey("invalid_key", apiKey)
	if err == nil {
		t.Error("Expected validation to fail for invalid key")
	}

	// Test key hash
	hash := manager.HashAPIKey(plainKey)
	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// Test key validation after revocation
	manager.RevokeAPIKey(apiKey)
	err = manager.ValidateAPIKey(plainKey, apiKey)
	if err != auth.ErrAPIKeyRevoked {
		t.Error("Expected key to be revoked")
	}
}

func TestAPIKeyPermissions(t *testing.T) {
	apiKey := &auth.APIKey{
		Permissions: []string{"read", "write"},
	}

	if !apiKey.HasPermission("read") {
		t.Error("Expected key to have read permission")
	}

	if !apiKey.HasPermission("write") {
		t.Error("Expected key to have write permission")
	}

	if apiKey.HasPermission("admin") {
		t.Error("Expected key to not have admin permission")
	}

	// Test wildcard permission
	wildcardKey := &auth.APIKey{
		Permissions: []string{"*"},
	}

	if !wildcardKey.HasPermission("anything") {
		t.Error("Expected wildcard key to have any permission")
	}
}

func TestAPIKeyExpiration(t *testing.T) {
	// Expired key
	expiredKey := &auth.APIKey{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !expiredKey.IsExpired() {
		t.Error("Expected key to be expired")
	}

	// Future expiry
	futureKey := &auth.APIKey{
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if futureKey.IsExpired() {
		t.Error("Expected key to not be expired")
	}
}

func BenchmarkJWTGeneration(b *testing.B) {
	manager := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:       "test-secret",
		Issuer:          "test",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 24 * time.Hour,
	})

	userID := uuid.New().String()
	email := "test@example.com"
	role := "user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GenerateAccessToken(userID, email, role)
		if err != nil {
			b.Fatalf("Failed to generate token: %v", err)
		}
	}
}

func BenchmarkJWTValidation(b *testing.B) {
	manager := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:       "test-secret",
		Issuer:          "test",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 24 * time.Hour,
	})

	token, _ := manager.GenerateAccessToken(uuid.New().String(), "test@example.com", "user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.ValidateToken(token)
		if err != nil {
			b.Fatalf("Failed to validate token: %v", err)
		}
	}
}

func BenchmarkAPIKeyGeneration(b *testing.B) {
	manager := auth.NewAPIKeyManager()
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := manager.GenerateAPIKey(
			userID,
			"Test Key",
			[]string{"read", "write"},
			60,
			365*24*time.Hour,
		)
		if err != nil {
			b.Fatalf("Failed to generate API key: %v", err)
		}
	}
}
