package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// RecoveryConfig configurazione del middleware di recovery
type RecoveryConfig struct {
	// EnableStackTrace abilita il log dello stack trace
	EnableStackTrace bool

	// StackTraceHandler funzione custom per gestire lo stack trace
	StackTraceHandler func(c fiber.Ctx, err interface{}, stack []byte)

	// Custom error response
	ErrorResponse func(c fiber.Ctx, err interface{}) error
}

// DefaultRecoveryConfig configurazione di default
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, err interface{}, stack []byte) {
			requestID := GetRequestID(c)

			log.Error().
				Str("request_id", requestID).
				Str("method", c.Method()).
				Str("path", c.Path()).
				Str("ip", c.IP()).
				Interface("panic", err).
				Bytes("stack", stack).
				Msg("panic recovered")
		},
		ErrorResponse: func(c fiber.Ctx, err interface{}) error {
			requestID := GetRequestID(c)

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":      "internal server error",
				"message":    "an unexpected error occurred",
				"request_id": requestID,
			})
		},
	}
}

// Recovery middleware per catturare i panic e rispondere con un errore 500
func Recovery(config ...RecoveryConfig) fiber.Handler {
	// Usa config di default se non specificato
	cfg := DefaultRecoveryConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c fiber.Ctx) (err error) {
		// Defer per catturare i panic
		defer func() {
			if r := recover(); r != nil {
				// Ottieni stack trace
				var stack []byte
				if cfg.EnableStackTrace {
					stack = debug.Stack()
				}

				// Chiama l'handler dello stack trace
				if cfg.StackTraceHandler != nil {
					cfg.StackTraceHandler(c, r, stack)
				}

				// Rispondi con errore
				if cfg.ErrorResponse != nil {
					err = cfg.ErrorResponse(c, r)
				} else {
					err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "internal server error",
					})
				}
			}
		}()

		// Continua con la chain
		return c.Next()
	}
}

// RecoveryWithLogger middleware di recovery con logger personalizzato
func RecoveryWithLogger() fiber.Handler {
	return Recovery(RecoveryConfig{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, err interface{}, stack []byte) {
			requestID := GetRequestID(c)
			userID, _ := c.Context().Value(UserIDKey).(string)

			logEvent := log.Error().
				Str("request_id", requestID).
				Str("method", c.Method()).
				Str("path", c.Path()).
				Str("ip", c.IP()).
				Interface("panic", err)

			if userID != "" {
				logEvent = logEvent.Str("user_id", userID)
			}

			if stack != nil {
				logEvent = logEvent.Bytes("stack", stack)
			}

			logEvent.Msg("panic recovered")
		},
		ErrorResponse: func(c fiber.Ctx, err interface{}) error {
			requestID := GetRequestID(c)

			// Converti panic in stringa per logging
			var errMsg string
			if e, ok := err.(error); ok {
				errMsg = e.Error()
			} else {
				errMsg = fmt.Sprintf("%v", err)
			}

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":      "internal_server_error",
				"message":    "an unexpected error occurred",
				"request_id": requestID,
				"details":    errMsg,
			})
		},
	})
}

// GracefulPanic helper per panic controllati con messaggio custom
func GracefulPanic(message string, err error) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", message, err))
	}
}
