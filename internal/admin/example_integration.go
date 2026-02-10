package admin

import (
	"time"

	"github.com/biodoia/goleapifree/internal/health"
	"github.com/biodoia/goleapifree/pkg/auth"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/middleware"
	"github.com/gofiber/fiber/v3"
)

// SetupAdminAPI configura l'API admin completa su un'app Fiber esistente
// Esempio di utilizzo nel main gateway
func SetupAdminAPI(
	app *fiber.App,
	db *database.DB,
	cfg *config.Config,
	healthMonitor *health.Monitor,
	jwtManager *auth.JWTManager,
) {
	// Crea admin handlers
	adminHandlers := NewAdminHandlers(db, cfg, healthMonitor)

	// Configura middleware di autenticazione per admin
	authConfig := middleware.AuthConfig{
		JWTManager:    jwtManager,
		APIKeyManager: auth.NewAPIKeyManager(),
		GetAPIKeyFunc: func(keyHash string) (*auth.APIKey, error) {
			// Ottieni API key dal database
			apiKey, err := db.GetAPIKeyByHash(keyHash)
			if err != nil {
				return nil, err
			}

			// Converti models.APIKey a auth.APIKey
			return &auth.APIKey{
				ID:          apiKey.ID,
				UserID:      apiKey.UserID,
				Name:        apiKey.Name,
				KeyHash:     apiKey.KeyBcrypt, // Use bcrypt hash for validation
				KeyPreview:  apiKey.KeyPreview,
				RateLimit:   apiKey.RateLimit,
				ExpiresAt:   apiKey.ExpiresAt,
				RevokedAt:   apiKey.RevokedAt,
				LastUsedAt:  apiKey.LastUsedAt,
				CreatedAt:   apiKey.CreatedAt,
				UpdatedAt:   apiKey.UpdatedAt,
			}, nil
		},
		GlobalRateLimit: 1000,  // 1000 req/s globale
		UserRateLimit:   60,    // 60 req/min per utente
	}

	// Registra tutte le route admin
	adminHandlers.RegisterRoutes(app, authConfig)

	// Avvia manutenzione programmata (opzionale)
	go adminHandlers.maintenance.ScheduledMaintenance()
}

// CreateDefaultAdminUser crea un utente admin di default se non esiste
func CreateDefaultAdminUser(db *database.DB, email, password string) error {
	// Controlla se esiste già un admin
	var adminCount int64
	db.Model(&database.DB{}).Where("role = ?", "admin").Count(&adminCount)

	if adminCount > 0 {
		return nil // Admin già esistente
	}

	userManager := NewUserManager(db)

	// Crea utente admin
	user := database.User{
		Email:         email,
		Name:          "System Administrator",
		Role:          "admin",
		Active:        true,
		QuotaTokens:   0, // Unlimited
		QuotaRequests: 0, // Unlimited
	}

	if err := user.SetPassword(password); err != nil {
		return err
	}

	if err := db.CreateUser(&user); err != nil {
		return err
	}

	// Genera API key per l'admin
	_, _, err := userManager.GenerateAPIKeyForUser(
		user.ID,
		"Admin Default Key",
		[]string{"*"}, // Tutti i permessi
		0,             // Rate limit illimitato
		0,             // Nessuna scadenza
	)

	return err
}

// Example: Come integrare nel main gateway
/*
func main() {
	// Load config
	cfg, err := config.Load("./config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Connect to database
	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Create default admin user (first time only)
	if err := admin.CreateDefaultAdminUser(db, "admin@goleapai.local", "ChangeMe123!"); err != nil {
		log.Warn().Err(err).Msg("Failed to create default admin user")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "GoLeapAI Gateway",
	})

	// Create JWT manager
	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:       os.Getenv("JWT_SECRET"),
		Issuer:          "goleapai-gateway",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 7 * 24 * time.Hour,
	})

	// Create health monitor
	healthMonitor := health.NewMonitor(db, 5*time.Minute)
	healthMonitor.Start()

	// Setup admin API
	admin.SetupAdminAPI(app, db, cfg, healthMonitor, jwtManager)

	// Setup other routes (gateway, public endpoints, etc.)
	setupGatewayRoutes(app, db, cfg)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("Starting GoLeapAI Gateway")
	if err := app.Listen(addr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
*/

// LoginRequest rappresenta una richiesta di login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse rappresenta una risposta di login
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	} `json:"user"`
}

// SetupAuthEndpoints configura gli endpoint di autenticazione
func SetupAuthEndpoints(app *fiber.App, db *database.DB, jwtManager *auth.JWTManager) {
	auth := app.Group("/auth")

	// POST /auth/login - Login con email e password
	auth.Post("/login", func(c fiber.Ctx) error {
		var req LoginRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Trova utente
		user, err := db.GetUserByEmail(req.Email)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid credentials",
			})
		}

		// Verifica password
		if !user.CheckPassword(req.Password) {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid credentials",
			})
		}

		// Verifica che l'utente sia attivo
		if !user.Active {
			return c.Status(403).JSON(fiber.Map{
				"error": "Account is disabled",
			})
		}

		// Genera token JWT
		accessToken, err := jwtManager.GenerateAccessToken(
			user.ID.String(),
			user.Email,
			string(user.Role),
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to generate access token",
			})
		}

		refreshToken, err := jwtManager.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to generate refresh token",
			})
		}

		// Aggiorna ultimo login
		now := time.Now()
		user.LastLoginAt = &now
		db.UpdateUser(user)

		// Prepara risposta
		response := LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		}
		response.User.ID = user.ID.String()
		response.User.Email = user.Email
		response.User.Name = user.Name
		response.User.Role = string(user.Role)

		return c.JSON(response)
	})

	// POST /auth/refresh - Refresh access token
	auth.Post("/refresh", func(c fiber.Ctx) error {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}

		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Valida refresh token
		userID, err := jwtManager.ValidateRefreshToken(req.RefreshToken)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid or expired refresh token",
			})
		}

		// Ottieni utente
		user, err := db.GetUserByID(userID)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		// Genera nuovo access token
		accessToken, err := jwtManager.GenerateAccessToken(
			user.ID.String(),
			user.Email,
			string(user.Role),
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to generate access token",
			})
		}

		return c.JSON(fiber.Map{
			"access_token": accessToken,
			"expires_at":   time.Now().Add(15 * time.Minute),
		})
	})

	// GET /auth/me - Ottieni informazioni utente corrente
	auth.Get("/me", middleware.Auth(middleware.AuthConfig{
		JWTManager:    jwtManager,
		APIKeyManager: auth.NewAPIKeyManager(),
	}), func(c fiber.Ctx) error {
		userID, err := middleware.GetUserID(c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		user, err := db.GetUserByID(userID.String())
		if err != nil {
			return c.Status(404).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		return c.JSON(fiber.Map{
			"id":                user.ID,
			"email":             user.Email,
			"name":              user.Name,
			"role":              user.Role,
			"active":            user.Active,
			"quota_tokens":      user.QuotaTokens,
			"quota_tokens_used": user.QuotaTokensUsed,
			"quota_requests":    user.QuotaRequests,
			"quota_requests_used": user.QuotaRequestsUsed,
			"quota_reset_at":    user.QuotaResetAt,
		})
	})
}
