package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// UserManager gestisce gli utenti e le API key
type UserManager struct {
	db            *database.DB
	apiKeyManager *auth.APIKeyManager
}

// NewUserManager crea un nuovo user manager
func NewUserManager(db *database.DB) *UserManager {
	return &UserManager{
		db:            db,
		apiKeyManager: auth.NewAPIKeyManager(),
	}
}

// ListUsers restituisce tutti gli utenti
func (h *AdminHandlers) ListUsers(c fiber.Ctx) error {
	var users []models.User

	query := h.db.DB

	// Filter by role
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	// Filter by active status
	if active := c.Query("active"); active != "" {
		query = query.Where("active = ?", active == "true")
	}

	// Pagination
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	if err := query.Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve users",
		})
	}

	// Count total
	var total int64
	h.db.Model(&models.User{}).Count(&total)

	return c.JSON(fiber.Map{
		"users":  users,
		"count":  len(users),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// CreateUser crea un nuovo utente
func (h *AdminHandlers) CreateUser(c fiber.Ctx) error {
	var req struct {
		Email           string           `json:"email"`
		Password        string           `json:"password"`
		Name            string           `json:"name"`
		Role            models.UserRole  `json:"role"`
		QuotaTokens     int64            `json:"quota_tokens"`
		QuotaRequests   int64            `json:"quota_requests"`
		GenerateAPIKey  bool             `json:"generate_api_key"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate email
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return c.Status(400).JSON(fiber.Map{
			"error": "Valid email is required",
		})
	}

	// Validate password
	if req.Password == "" || len(req.Password) < 8 {
		return c.Status(400).JSON(fiber.Map{
			"error": "Password must be at least 8 characters",
		})
	}

	// Check if user already exists
	var existingUser models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "User with this email already exists",
		})
	}

	// Set default role
	if req.Role == "" {
		req.Role = models.RoleUser
	}

	// Create user
	user := models.User{
		ID:            uuid.New(),
		Email:         req.Email,
		Name:          req.Name,
		Role:          req.Role,
		Active:        true,
		QuotaTokens:   req.QuotaTokens,
		QuotaRequests: req.QuotaRequests,
		QuotaResetAt:  time.Now().AddDate(0, 1, 0),
	}

	// Hash password
	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	// Save user
	if err := h.db.Create(&user).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	log.Info().Str("email", user.Email).Msg("User created successfully")

	response := fiber.Map{
		"message": "User created successfully",
		"user":    user,
	}

	// Generate API key if requested
	if req.GenerateAPIKey {
		apiKey, keyString, err := h.userManager.GenerateAPIKeyForUser(user.ID, "Default API Key", []string{"*"}, 0, 0)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate API key")
		} else {
			response["api_key"] = keyString
			response["api_key_preview"] = apiKey.KeyPreview
		}
	}

	return c.Status(201).JSON(response)
}

// UpdateUser aggiorna un utente esistente
func (h *AdminHandlers) UpdateUser(c fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	var req struct {
		Name            *string          `json:"name"`
		Email           *string          `json:"email"`
		Password        *string          `json:"password"`
		Role            *models.UserRole `json:"role"`
		Active          *bool            `json:"active"`
		QuotaTokens     *int64           `json:"quota_tokens"`
		QuotaRequests   *int64           `json:"quota_requests"`
		QuotaTokensUsed *int64           `json:"quota_tokens_used"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update fields
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		// Check if email is already taken
		var existingUser models.User
		if err := h.db.Where("email = ? AND id != ?", *req.Email, userID).First(&existingUser).Error; err == nil {
			return c.Status(409).JSON(fiber.Map{
				"error": "Email already in use",
			})
		}
		user.Email = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 8 {
			return c.Status(400).JSON(fiber.Map{
				"error": "Password must be at least 8 characters",
			})
		}
		if err := user.SetPassword(*req.Password); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to update password",
			})
		}
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Active != nil {
		user.Active = *req.Active
	}
	if req.QuotaTokens != nil {
		user.QuotaTokens = *req.QuotaTokens
	}
	if req.QuotaRequests != nil {
		user.QuotaRequests = *req.QuotaRequests
	}
	if req.QuotaTokensUsed != nil {
		user.QuotaTokensUsed = *req.QuotaTokensUsed
	}

	if err := h.db.Save(&user).Error; err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to update user",
		})
	}

	log.Info().Str("email", user.Email).Msg("User updated successfully")

	return c.JSON(fiber.Map{
		"message": "User updated successfully",
		"user":    user,
	})
}

// DeleteUser elimina un utente
func (h *AdminHandlers) DeleteUser(c fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Prevent deleting yourself
	currentUserID := c.Context().Value("user_id")
	if currentUserID != nil && currentUserID.(string) == userID.String() {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot delete your own account",
		})
	}

	// Delete associated API keys
	h.db.Where("user_id = ?", userID).Delete(&models.APIKey{})

	// Delete user
	if err := h.db.Delete(&user).Error; err != nil {
		log.Error().Err(err).Msg("Failed to delete user")
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to delete user",
		})
	}

	log.Info().Str("email", user.Email).Msg("User deleted successfully")

	return c.JSON(fiber.Map{
		"message": "User deleted successfully",
	})
}

// ResetUserQuota resetta le quote di un utente
func (h *AdminHandlers) ResetUserQuota(c fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Reset quota
	user.ResetQuota()

	if err := h.db.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to reset quota",
		})
	}

	log.Info().Str("email", user.Email).Msg("User quota reset successfully")

	return c.JSON(fiber.Map{
		"message": "Quota reset successfully",
		"user":    user,
	})
}

// ListUserAPIKeys restituisce le API keys di un utente
func (h *AdminHandlers) ListUserAPIKeys(c fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var apiKeys []models.APIKey
	if err := h.db.Where("user_id = ?", userID).Find(&apiKeys).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve API keys",
		})
	}

	return c.JSON(fiber.Map{
		"api_keys": apiKeys,
		"count":    len(apiKeys),
	})
}

// GenerateAPIKeyForUser genera una nuova API key per un utente
func (um *UserManager) GenerateAPIKeyForUser(userID uuid.UUID, name string, permissions []string, rateLimit int, expiresInDays int) (*models.APIKey, string, error) {
	// Generate random bytes for the key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Encode to base64
	keySecret := base64.RawURLEncoding.EncodeToString(keyBytes)
	fullKey := fmt.Sprintf("gla_%s", keySecret)

	// Create SHA256 hash for fast lookup
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	// Create bcrypt hash for validation
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash key: %w", err)
	}

	// Create preview
	preview := fullKey
	if len(fullKey) > 12 {
		preview = fullKey[:12] + "..."
	}

	// Determine expiration
	var expiresAt time.Time
	if expiresInDays > 0 {
		expiresAt = time.Now().AddDate(0, 0, expiresInDays)
	} else {
		expiresAt = time.Now().AddDate(10, 0, 0) // 10 years default
	}

	// Set default rate limit
	if rateLimit == 0 {
		rateLimit = 60 // 60 requests per minute
	}

	// Permissions to comma-separated string
	permStr := strings.Join(permissions, ",")

	apiKey := &models.APIKey{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		KeyHash:     keyHash,
		KeyBcrypt:   string(bcryptHash),
		KeyPreview:  preview,
		Permissions: permStr,
		RateLimit:   rateLimit,
		Active:      true,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to database
	if err := um.db.Create(apiKey).Error; err != nil {
		return nil, "", fmt.Errorf("failed to save API key: %w", err)
	}

	log.Info().
		Str("user_id", userID.String()).
		Str("key_name", name).
		Msg("API key generated successfully")

	return apiKey, fullKey, nil
}

// RevokeAPIKey revoca una API key
func (um *UserManager) RevokeAPIKey(keyID uuid.UUID) error {
	var apiKey models.APIKey
	if err := um.db.First(&apiKey, keyID).Error; err != nil {
		return fmt.Errorf("API key not found: %w", err)
	}

	now := time.Now()
	apiKey.RevokedAt = &now
	apiKey.Active = false
	apiKey.UpdatedAt = now

	if err := um.db.Save(&apiKey).Error; err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	log.Info().Str("key_id", keyID.String()).Msg("API key revoked")

	return nil
}

// ValidateAPIKey valida una API key
func (um *UserManager) ValidateAPIKey(key string) (*models.APIKey, error) {
	// Create hash for lookup
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	// Find key in database
	var apiKey models.APIKey
	if err := um.db.Where("key_hash = ?", keyHash).Preload("User").First(&apiKey).Error; err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	// Check if key is valid
	if !apiKey.IsValid() {
		if apiKey.IsRevoked() {
			return nil, fmt.Errorf("API key has been revoked")
		}
		if apiKey.IsExpired() {
			return nil, fmt.Errorf("API key has expired")
		}
		return nil, fmt.Errorf("API key is not active")
	}

	// Validate bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyBcrypt), []byte(key)); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last used timestamp and usage count (async)
	go func() {
		apiKey.UpdateUsage()
		um.db.Save(&apiKey)
	}()

	return &apiKey, nil
}

// GetAPIKeyByHash ottiene una API key dal suo hash
func (um *UserManager) GetAPIKeyByHash(keyHash string) (*models.APIKey, error) {
	var apiKey models.APIKey
	if err := um.db.Where("key_hash = ?", keyHash).Preload("User").First(&apiKey).Error; err != nil {
		return nil, fmt.Errorf("API key not found: %w", err)
	}
	return &apiKey, nil
}

// ListAPIKeys restituisce tutte le API keys con filtri opzionali
func (um *UserManager) ListAPIKeys(userID *uuid.UUID, activeOnly bool) ([]models.APIKey, error) {
	query := um.db.DB

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if activeOnly {
		query = query.Where("active = ?", true).Where("revoked_at IS NULL")
	}

	var keys []models.APIKey
	if err := query.Preload("User").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	return keys, nil
}

// CleanupExpiredKeys elimina le chiavi scadute
func (um *UserManager) CleanupExpiredKeys() (int64, error) {
	result := um.db.Where("expires_at < ? AND expires_at != ?", time.Now(), time.Time{}).Delete(&models.APIKey{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup expired keys: %w", result.Error)
	}

	log.Info().Int64("count", result.RowsAffected).Msg("Expired API keys cleaned up")

	return result.RowsAffected, nil
}
