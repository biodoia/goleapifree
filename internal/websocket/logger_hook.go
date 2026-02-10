package websocket

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// LoggerHook è un hook per zerolog che invia i log via WebSocket
type LoggerHook struct {
	broadcaster *Broadcaster
	minLevel    zerolog.Level
	component   string
}

// NewLoggerHook crea un nuovo logger hook
func NewLoggerHook(broadcaster *Broadcaster, minLevel zerolog.Level, component string) *LoggerHook {
	return &LoggerHook{
		broadcaster: broadcaster,
		minLevel:    minLevel,
		component:   component,
	}
}

// Run implementa l'interfaccia zerolog.Hook
func (h *LoggerHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// Filtra per livello minimo
	if level < h.minLevel {
		return
	}

	// Converti livello zerolog a string
	levelStr := levelToString(level)

	// Estrai campi dal contesto (se disponibili)
	fields := make(map[string]string)

	// Broadcast log event
	if h.broadcaster != nil && h.broadcaster.handler != nil {
		h.broadcaster.handler.BroadcastLogEvent(
			levelStr,
			msg,
			h.component,
			uuid.Nil,
			uuid.Nil,
			fields,
		)
	}
}

// levelToString converte zerolog.Level a stringa
func levelToString(level zerolog.Level) string {
	switch level {
	case zerolog.DebugLevel:
		return "debug"
	case zerolog.InfoLevel:
		return "info"
	case zerolog.WarnLevel:
		return "warn"
	case zerolog.ErrorLevel:
		return "error"
	case zerolog.FatalLevel:
		return "error"
	case zerolog.PanicLevel:
		return "error"
	default:
		return "info"
	}
}

// LoggerWriter implementa io.Writer per catturare log e inviarli via WebSocket
type LoggerWriter struct {
	broadcaster *Broadcaster
	component   string
}

// NewLoggerWriter crea un nuovo writer per logger
func NewLoggerWriter(broadcaster *Broadcaster, component string) *LoggerWriter {
	return &LoggerWriter{
		broadcaster: broadcaster,
		component:   component,
	}
}

// Write implementa io.Writer
func (w *LoggerWriter) Write(p []byte) (n int, err error) {
	// Parse il messaggio di log (implementazione semplificata)
	msg := string(p)

	// Broadcast come info log
	if w.broadcaster != nil && w.broadcaster.handler != nil {
		w.broadcaster.handler.BroadcastLogEvent(
			"info",
			msg,
			w.component,
			uuid.Nil,
			uuid.Nil,
			nil,
		)
	}

	return len(p), nil
}

// ContextualLogger logger con contesto per WebSocket
type ContextualLogger struct {
	broadcaster *Broadcaster
	component   string
	providerID  uuid.UUID
	requestID   uuid.UUID
}

// NewContextualLogger crea un logger con contesto
func NewContextualLogger(broadcaster *Broadcaster, component string) *ContextualLogger {
	return &ContextualLogger{
		broadcaster: broadcaster,
		component:   component,
	}
}

// WithProvider imposta il provider ID nel contesto
func (l *ContextualLogger) WithProvider(providerID uuid.UUID) *ContextualLogger {
	return &ContextualLogger{
		broadcaster: l.broadcaster,
		component:   l.component,
		providerID:  providerID,
		requestID:   l.requestID,
	}
}

// WithRequest imposta il request ID nel contesto
func (l *ContextualLogger) WithRequest(requestID uuid.UUID) *ContextualLogger {
	return &ContextualLogger{
		broadcaster: l.broadcaster,
		component:   l.component,
		providerID:  l.providerID,
		requestID:   requestID,
	}
}

// Info log di livello info
func (l *ContextualLogger) Info(message string, fields map[string]string) {
	if l.broadcaster != nil && l.broadcaster.handler != nil {
		l.broadcaster.handler.BroadcastLogEvent(
			"info",
			message,
			l.component,
			l.providerID,
			l.requestID,
			fields,
		)
	}
}

// Error log di livello error
func (l *ContextualLogger) Error(message string, fields map[string]string) {
	if l.broadcaster != nil && l.broadcaster.handler != nil {
		l.broadcaster.handler.BroadcastLogEvent(
			"error",
			message,
			l.component,
			l.providerID,
			l.requestID,
			fields,
		)
	}
}

// Warn log di livello warning
func (l *ContextualLogger) Warn(message string, fields map[string]string) {
	if l.broadcaster != nil && l.broadcaster.handler != nil {
		l.broadcaster.handler.BroadcastLogEvent(
			"warn",
			message,
			l.component,
			l.providerID,
			l.requestID,
			fields,
		)
	}
}

// Debug log di livello debug
func (l *ContextualLogger) Debug(message string, fields map[string]string) {
	if l.broadcaster != nil && l.broadcaster.handler != nil {
		l.broadcaster.handler.BroadcastLogEvent(
			"debug",
			message,
			l.component,
			l.providerID,
			l.requestID,
			fields,
		)
	}
}
