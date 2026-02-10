package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// CORSConfig configurazione CORS
type CORSConfig struct {
	// AllowedOrigins lista degli origin permessi
	// Usa "*" per permettere tutti (non raccomandato in produzione)
	AllowedOrigins []string

	// AllowedMethods metodi HTTP permessi
	AllowedMethods []string

	// AllowedHeaders headers permessi nelle richieste
	AllowedHeaders []string

	// ExposedHeaders headers esposti al client
	ExposedHeaders []string

	// AllowCredentials permette l'invio di credenziali (cookies, authorization headers)
	AllowCredentials bool

	// MaxAge tempo di cache per preflight requests (in secondi)
	MaxAge int
}

// DefaultCORSConfig configurazione CORS di default
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			fiber.MethodGet,
			fiber.MethodPost,
			fiber.MethodPut,
			fiber.MethodPatch,
			fiber.MethodDelete,
			fiber.MethodHead,
			fiber.MethodOptions,
		},
		AllowedHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
			"X-API-Key",
		},
		ExposedHeaders: []string{
			"Content-Length",
			"X-Request-ID",
			"X-User-ID",
			"X-API-Key-ID",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24 ore
	}
}

// CORS middleware per gestire Cross-Origin Resource Sharing
func CORS(config CORSConfig) fiber.Handler {
	// Normalizza gli allowed origins
	allowOriginFunc := func(origin string) bool {
		for _, allowedOrigin := range config.AllowedOrigins {
			if allowedOrigin == "*" {
				return true
			}
			if allowedOrigin == origin {
				return true
			}
			// Supporta wildcard subdomain (*.example.com)
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := strings.TrimPrefix(allowedOrigin, "*")
				if strings.HasSuffix(origin, domain) {
					return true
				}
			}
		}
		return false
	}

	// Prepara le stringhe per gli headers
	allowMethods := strings.Join(config.AllowedMethods, ", ")
	allowHeaders := strings.Join(config.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(config.ExposedHeaders, ", ")

	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")

		// Se non c'è Origin header, non è una richiesta CORS
		if origin == "" {
			return c.Next()
		}

		// Verifica se l'origin è permesso
		if !allowOriginFunc(origin) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "origin not allowed",
			})
		}

		// Set CORS headers
		c.Set("Access-Control-Allow-Origin", origin)

		if config.AllowCredentials {
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight request
		if c.Method() == fiber.MethodOptions {
			c.Set("Access-Control-Allow-Methods", allowMethods)
			c.Set("Access-Control-Allow-Headers", allowHeaders)

			if config.MaxAge > 0 {
				c.Set("Access-Control-Max-Age", string(rune(config.MaxAge)))
			}

			// Preflight richiede 204 No Content
			return c.SendStatus(fiber.StatusNoContent)
		}

		// Set exposed headers per richieste normali
		if exposeHeaders != "" {
			c.Set("Access-Control-Expose-Headers", exposeHeaders)
		}

		return c.Next()
	}
}

// CORSWithConfig crea un middleware CORS con configurazione custom
func CORSWithConfig(config CORSConfig) fiber.Handler {
	// Applica defaults se non specificati
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = DefaultCORSConfig().AllowedOrigins
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = DefaultCORSConfig().AllowedMethods
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = DefaultCORSConfig().AllowedHeaders
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultCORSConfig().MaxAge
	}

	return CORS(config)
}
