package gateway

import (
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/biodoia/goleapifree/pkg/middleware"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Register request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name"`
}

// Login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// CreateAPIKey request
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" validate:"required"`
	Permissions []string `json:"permissions"`
	RateLimit   int      `json:"rate_limit"`
	ExpiresIn   int      `json:"expires_in"` // Giorni, 0 = mai
}

// handleRegister gestisce la registrazione di un nuovo utente
func (g *Gateway) handleRegister(c fiber.Ctx) error {
	var req RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Verifica se l'utente esiste già
	existingUser, _ := g.db.GetUserByEmail(req.Email)
	if existingUser != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "user already exists",
		})
	}

	// Crea nuovo utente
	user := &models.User{
		Email:  req.Email,
		Name:   req.Name,
		Role:   models.RoleUser,
		Active: true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create user",
		})
	}

	if err := g.db.CreateUser(user); err != nil {
		log.Error().Err(err).Msg("failed to create user in database")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create user",
		})
	}

	// Genera token
	accessToken, err := g.jwtManager.GenerateAccessToken(user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		log.Error().Err(err).Msg("failed to generate access token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	refreshToken, err := g.jwtManager.GenerateRefreshToken(user.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("failed to generate refresh token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": fiber.Map{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	})
}

// handleLogin gestisce il login di un utente
func (g *Gateway) handleLogin(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Cerca l'utente
	user, err := g.db.GetUserByEmail(req.Email)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credentials",
		})
	}

	// Verifica la password
	if !user.CheckPassword(req.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credentials",
		})
	}

	// Verifica che l'utente sia attivo
	if !user.Active {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "account disabled",
		})
	}

	// Aggiorna ultimo login
	now := time.Now()
	user.LastLoginAt = &now
	g.db.UpdateUser(user)

	// Genera token
	accessToken, err := g.jwtManager.GenerateAccessToken(user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		log.Error().Err(err).Msg("failed to generate access token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	refreshToken, err := g.jwtManager.GenerateRefreshToken(user.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("failed to generate refresh token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	})
}

// handleRefreshToken gestisce il refresh del token
func (g *Gateway) handleRefreshToken(c fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Valida il refresh token
	userID, err := g.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid refresh token",
		})
	}

	// Ottieni l'utente
	user, err := g.db.GetUserByID(userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Genera nuovo access token
	accessToken, err := g.jwtManager.GenerateAccessToken(user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"access_token": accessToken,
		"token_type":   "Bearer",
	})
}

// handleGetProfile restituisce il profilo dell'utente
func (g *Gateway) handleGetProfile(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	user, err := g.db.GetUserByID(userID.String())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":                   user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"role":                 user.Role,
		"active":               user.Active,
		"email_verified":       user.EmailVerified,
		"quota_tokens":         user.QuotaTokens,
		"quota_tokens_used":    user.QuotaTokensUsed,
		"quota_requests":       user.QuotaRequests,
		"quota_requests_used":  user.QuotaRequestsUsed,
		"quota_reset_at":       user.QuotaResetAt,
		"last_login_at":        user.LastLoginAt,
		"created_at":           user.CreatedAt,
	})
}

// handleUpdateProfile aggiorna il profilo dell'utente
func (g *Gateway) handleUpdateProfile(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	user, err := g.db.GetUserByID(userID.String())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Aggiorna i campi
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" && req.Email != user.Email {
		// Verifica che la nuova email non sia già in uso
		existing, _ := g.db.GetUserByEmail(req.Email)
		if existing != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "email already in use",
			})
		}
		user.Email = req.Email
		user.EmailVerified = false // Richiede nuova verifica
	}

	if err := g.db.UpdateUser(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update profile",
		})
	}

	return c.JSON(fiber.Map{
		"message": "profile updated successfully",
		"user": fiber.Map{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

// handleListAPIKeys lista tutte le chiavi API dell'utente
func (g *Gateway) handleListAPIKeys(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	apiKeys, err := g.db.GetUserAPIKeys(userID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve api keys",
		})
	}

	return c.JSON(fiber.Map{
		"api_keys": apiKeys,
	})
}

// handleCreateAPIKey crea una nuova API key
func (g *Gateway) handleCreateAPIKey(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	var req CreateAPIKeyRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Calcola scadenza
	expiresIn := time.Duration(req.ExpiresIn) * 24 * time.Hour
	if req.ExpiresIn == 0 {
		expiresIn = 365 * 24 * time.Hour // 1 anno di default
	}

	// Genera la chiave
	apiKey, plainKey, err := g.apiKeyManager.GenerateAPIKey(
		userID,
		req.Name,
		req.Permissions,
		req.RateLimit,
		expiresIn,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate api key")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate api key",
		})
	}

	// Salva nel database
	dbKey := &models.APIKey{
		ID:          apiKey.ID,
		UserID:      apiKey.UserID,
		Name:        apiKey.Name,
		KeyHash:     g.apiKeyManager.HashAPIKey(plainKey), // SHA256 per lookup
		KeyBcrypt:   apiKey.KeyHash,                       // Bcrypt per validazione
		KeyPreview:  apiKey.KeyPreview,
		Permissions: strings.Join(apiKey.Permissions, ","),
		RateLimit:   apiKey.RateLimit,
		ExpiresAt:   apiKey.ExpiresAt,
		Active:      true,
	}

	if err := g.db.CreateAPIKey(dbKey); err != nil {
		log.Error().Err(err).Msg("failed to save api key")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create api key",
		})
	}

	// Restituisci la chiave in chiaro (unica volta!)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "API key created successfully. Save it now, it won't be shown again!",
		"api_key": plainKey,
		"key_info": fiber.Map{
			"id":          apiKey.ID,
			"name":        apiKey.Name,
			"preview":     apiKey.KeyPreview,
			"permissions": apiKey.Permissions,
			"rate_limit":  apiKey.RateLimit,
			"expires_at":  apiKey.ExpiresAt,
			"created_at":  apiKey.CreatedAt,
		},
	})
}

// handleRevokeAPIKey revoca una API key
func (g *Gateway) handleRevokeAPIKey(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing api key id",
		})
	}

	// Ottieni la chiave
	apiKeys, err := g.db.GetUserAPIKeys(userID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve api keys",
		})
	}

	var targetKey *models.APIKey
	for _, key := range apiKeys {
		if key.ID.String() == keyID {
			targetKey = &key
			break
		}
	}

	if targetKey == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "api key not found",
		})
	}

	// Revoca la chiave
	now := time.Now()
	targetKey.RevokedAt = &now
	targetKey.Active = false

	if err := g.db.UpdateAPIKey(targetKey); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to revoke api key",
		})
	}

	return c.JSON(fiber.Map{
		"message": "API key revoked successfully",
	})
}

// handleListUsers lista tutti gli utenti (admin only)
func (g *Gateway) handleListUsers(c fiber.Ctx) error {
	// TODO: Implementa paginazione
	return c.JSON(fiber.Map{
		"users": []string{},
	})
}

// handleMetricsInfo restituisce informazioni sulle metriche disponibili
func (g *Gateway) handleMetricsInfo(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"metrics": middleware.GetMetricsInfo(),
	})
}

// authMiddleware restituisce il middleware di autenticazione
func (g *Gateway) authMiddleware() fiber.Handler {
	return middleware.Auth(middleware.AuthConfig{
		JWTManager:    g.jwtManager,
		APIKeyManager: g.apiKeyManager,
		GetAPIKeyFunc: func(keyHash string) (*auth.APIKey, error) {
			dbKey, err := g.db.GetAPIKeyByHash(keyHash)
			if err != nil {
				return nil, err
			}

			// Parse permissions
			permissions := []string{}
			if dbKey.Permissions != "" {
				permissions = strings.Split(dbKey.Permissions, ",")
			}

			// Converti dal model DB al tipo auth
			return &auth.APIKey{
				ID:          dbKey.ID,
				UserID:      dbKey.UserID,
				Name:        dbKey.Name,
				KeyHash:     dbKey.KeyBcrypt,
				KeyPreview:  dbKey.KeyPreview,
				Permissions: permissions,
				RateLimit:   dbKey.RateLimit,
				ExpiresAt:   dbKey.ExpiresAt,
				RevokedAt:   dbKey.RevokedAt,
				LastUsedAt:  dbKey.LastUsedAt,
				CreatedAt:   dbKey.CreatedAt,
				UpdatedAt:   dbKey.UpdatedAt,
			}, nil
		},
		UserRateLimit: 60, // TODO: leggere da config
	})
}
