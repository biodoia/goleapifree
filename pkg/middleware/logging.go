package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LoggingConfig configurazione del middleware di logging
type LoggingConfig struct {
	// Logger personalizzato (opzionale)
	Logger *zerolog.Logger
	// Skip paths che non devono essere loggati
	SkipPaths []string
	// Log request body (attenzione: può contenere dati sensibili)
	LogRequestBody bool
	// Log response body (attenzione: può essere molto grande)
	LogResponseBody bool
}

// RequestIDKey chiave per il request ID nel context
const RequestIDKey ContextKey = "request_id"

// RequestID middleware per generare e tracciare request ID
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Prova a ottenere request ID dall'header
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			// Genera nuovo UUID
			requestID = uuid.New().String()
		}

		// Aggiungi al context
		c.SetContext(c.Context())
		c.Locals(string(RequestIDKey), requestID)

		// Aggiungi all'header di risposta
		c.Set("X-Request-ID", requestID)

		return c.Next()
	}
}

// Logging middleware per logging strutturato delle richieste
func Logging(config LoggingConfig) fiber.Handler {
	// Usa il logger globale se non specificato
	logger := log.Logger
	if config.Logger != nil {
		logger = *config.Logger
	}

	// Crea mappa per skip paths
	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	return func(c fiber.Ctx) error {
		// Skip se il path è nella lista
		if skipMap[c.Path()] {
			return c.Next()
		}

		// Timestamp di inizio
		start := time.Now()

		// Ottieni request ID
		requestID, _ := c.Locals(string(RequestIDKey)).(string)
		if requestID == "" {
			requestID = uuid.New().String()
			c.Locals(string(RequestIDKey), requestID)
		}

		// Ottieni informazioni utente se disponibili
		userID, _ := c.Context().Value(UserIDKey).(string)
		userEmail, _ := c.Context().Value(UserEmailKey).(string)
		apiKeyID, _ := c.Context().Value(APIKeyIDKey).(string)

		// Log evento di inizio request
		logEvent := logger.Info().
			Str("request_id", requestID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent"))

		if userID != "" {
			logEvent = logEvent.Str("user_id", userID)
		}
		if userEmail != "" {
			logEvent = logEvent.Str("user_email", userEmail)
		}
		if apiKeyID != "" {
			logEvent = logEvent.Str("api_key_id", apiKeyID)
		}

		// Log request body se configurato
		if config.LogRequestBody {
			logEvent = logEvent.Bytes("request_body", c.Body())
		}

		logEvent.Msg("request started")

		// Esegui la richiesta
		err := c.Next()

		// Calcola latency
		latency := time.Since(start)
		status := c.Response().StatusCode()

		// Determina il livello di log in base allo status
		var logFunc func() *zerolog.Event
		switch {
		case status >= 500:
			logFunc = logger.Error
		case status >= 400:
			logFunc = logger.Warn
		default:
			logFunc = logger.Info
		}

		// Log evento di fine request
		logEvent = logFunc().
			Str("request_id", requestID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Dur("latency", latency).
			Dur("latency_ms", latency).
			Int64("bytes_sent", int64(len(c.Response().Body()))).
			Str("ip", c.IP())

		if userID != "" {
			logEvent = logEvent.Str("user_id", userID)
		}
		if apiKeyID != "" {
			logEvent = logEvent.Str("api_key_id", apiKeyID)
		}

		// Log response body se configurato e se c'è un errore
		if config.LogResponseBody && status >= 400 {
			logEvent = logEvent.Bytes("response_body", c.Response().Body())
		}

		if err != nil {
			logEvent = logEvent.Err(err)
		}

		logEvent.Msg("request completed")

		return err
	}
}

// GetRequestID estrae il request ID dal context
func GetRequestID(c fiber.Ctx) string {
	requestID, ok := c.Locals(string(RequestIDKey)).(string)
	if !ok {
		return ""
	}
	return requestID
}

// AccessLog middleware semplificato per access log in formato Apache-like
func AccessLog() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		log.Info().
			Str("remote_addr", c.IP()).
			Str("method", c.Method()).
			Str("uri", c.OriginalURL()).
			Str("protocol", c.Protocol()).
			Int("status", c.Response().StatusCode()).
			Int64("bytes_sent", int64(len(c.Response().Body()))).
			Str("referer", c.Get("Referer")).
			Str("user_agent", c.Get("User-Agent")).
			Dur("latency", time.Since(start)).
			Msg("access")

		return err
	}
}
