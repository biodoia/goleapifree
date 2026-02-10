package middleware

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// ContextKey tipo per le chiavi del context
type ContextKey string

const (
	// UserIDKey chiave per l'ID utente nel context
	UserIDKey ContextKey = "user_id"
	// UserEmailKey chiave per l'email utente nel context
	UserEmailKey ContextKey = "user_email"
	// UserRoleKey chiave per il ruolo utente nel context
	UserRoleKey ContextKey = "user_role"
	// APIKeyIDKey chiave per l'ID della API key nel context
	APIKeyIDKey ContextKey = "api_key_id"
)

// AuthConfig configurazione del middleware di autenticazione
type AuthConfig struct {
	JWTManager    *auth.JWTManager
	APIKeyManager *auth.APIKeyManager
	// Funzione per ottenere una API key dal database
	GetAPIKeyFunc func(keyHash string) (*auth.APIKey, error)
	// Rate limiting globale (requests per secondo)
	GlobalRateLimit int
	// Rate limiting per utente (requests per minuto)
	UserRateLimit int
}

// userRateLimiter gestisce il rate limiting per utente
type userRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	limit    rate.Limit
	burst    int
}

func newUserRateLimiter(requestsPerMinute int) *userRateLimiter {
	return &userRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Limit(requestsPerMinute) / 60.0, // Converti a rate per secondo
		burst:    requestsPerMinute,
	}
}

func (rl *userRateLimiter) getLimiter(userID string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[userID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.limit, rl.burst)
		rl.limiters[userID] = limiter
		rl.mu.Unlock()
	}

	return limiter
}

// Cleanup rimuove i limiters non utilizzati
func (rl *userRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for userID, limiter := range rl.limiters {
		// Rimuovi se ha ancora tutti i tokens (non usato recentemente)
		if limiter.Tokens() == float64(rl.burst) {
			delete(rl.limiters, userID)
		}
	}
}

var (
	globalRateLimiter *userRateLimiter
	cleanupTicker     *time.Ticker
)

// Auth middleware per autenticazione JWT o API key
func Auth(config AuthConfig) fiber.Handler {
	// Inizializza rate limiter globale se configurato
	if config.UserRateLimit > 0 {
		globalRateLimiter = newUserRateLimiter(config.UserRateLimit)

		// Cleanup periodico ogni 5 minuti
		cleanupTicker = time.NewTicker(5 * time.Minute)
		go func() {
			for range cleanupTicker.C {
				globalRateLimiter.cleanup()
			}
		}()
	}

	return func(c fiber.Ctx) error {
		// Estrai token dall'header Authorization
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header",
			})
		}

		var userID, email, role string
		var apiKeyID string

		// Prova prima JWT
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

			claims, err := config.JWTManager.ValidateToken(token)
			if err != nil {
				log.Debug().Err(err).Msg("JWT validation failed")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "invalid or expired token",
				})
			}

			userID = claims.UserID
			email = claims.Email
			role = claims.Role
			apiKeyID = claims.ApiKeyID

		} else if strings.HasPrefix(authHeader, "ApiKey ") {
			// API Key authentication
			key := strings.TrimPrefix(authHeader, "ApiKey ")

			// Hash della chiave per lookup
			keyHash := config.APIKeyManager.HashAPIKey(key)

			// Ottieni la chiave dal database
			apiKey, err := config.GetAPIKeyFunc(keyHash)
			if err != nil {
				log.Debug().Err(err).Msg("API key lookup failed")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "invalid api key",
				})
			}

			// Valida la chiave
			if err := config.APIKeyManager.ValidateAPIKey(key, apiKey); err != nil {
				log.Debug().Err(err).Msg("API key validation failed")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			userID = apiKey.UserID.String()
			role = "api_key"
			apiKeyID = apiKey.ID.String()

			// Rate limiting specifico per API key
			if apiKey.RateLimit > 0 {
				limiter := rate.NewLimiter(rate.Limit(apiKey.RateLimit)/60.0, apiKey.RateLimit)
				if !limiter.Allow() {
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
						"error": "rate limit exceeded for api key",
					})
				}
			}

			// Aggiorna ultimo utilizzo (asincrono)
			go func() {
				apiKey.UpdateLastUsed()
				// TODO: Salva nel database
			}()

		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization format (use 'Bearer <token>' or 'ApiKey <key>')",
			})
		}

		// Rate limiting per utente
		if globalRateLimiter != nil {
			limiter := globalRateLimiter.getLimiter(userID)
			if !limiter.Allow() {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error": "rate limit exceeded",
				})
			}
		}

		// Inietta informazioni utente nel context
		ctx := context.WithValue(c.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserEmailKey, email)
		ctx = context.WithValue(ctx, UserRoleKey, role)
		if apiKeyID != "" {
			ctx = context.WithValue(ctx, APIKeyIDKey, apiKeyID)
		}
		c.SetContext(ctx)

		// Aggiungi headers di risposta
		c.Set("X-User-ID", userID)
		if apiKeyID != "" {
			c.Set("X-API-Key-ID", apiKeyID)
		}

		return c.Next()
	}
}

// OptionalAuth middleware per autenticazione opzionale
// Se presente un token valido, inietta le informazioni utente, altrimenti continua
func OptionalAuth(config AuthConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		// Prova a validare il token
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := config.JWTManager.ValidateToken(token)
			if err == nil {
				// Token valido, inietta nel context
				ctx := context.WithValue(c.Context(), UserIDKey, claims.UserID)
				ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
				ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
				c.SetContext(ctx)
				c.Set("X-User-ID", claims.UserID)
			}
		}

		return c.Next()
	}
}

// RequireRole middleware per verificare che l'utente abbia un determinato ruolo
func RequireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		userRole := c.Context().Value(UserRoleKey)
		if userRole == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient permissions",
			})
		}

		roleStr := userRole.(string)
		for _, role := range roles {
			if roleStr == role || roleStr == "admin" {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}
}

// GetUserID estrae l'ID utente dal context
func GetUserID(c fiber.Ctx) (uuid.UUID, error) {
	userID := c.Context().Value(UserIDKey)
	if userID == nil {
		return uuid.Nil, fiber.ErrUnauthorized
	}
	return uuid.Parse(userID.(string))
}

// GetUserEmail estrae l'email utente dal context
func GetUserEmail(c fiber.Ctx) (string, error) {
	email := c.Context().Value(UserEmailKey)
	if email == nil {
		return "", fiber.ErrUnauthorized
	}
	return email.(string), nil
}

// GetUserRole estrae il ruolo utente dal context
func GetUserRole(c fiber.Ctx) (string, error) {
	role := c.Context().Value(UserRoleKey)
	if role == nil {
		return "", fiber.ErrUnauthorized
	}
	return role.(string), nil
}
