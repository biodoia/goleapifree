package security

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// Sanitizer gestisce la sanitizzazione degli input
type Sanitizer struct {
	// Configurazione
	maxInputLength int
	strictMode     bool
}

// NewSanitizer crea un nuovo sanitizer
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		maxInputLength: 10000,
		strictMode:     true,
	}
}

// WithMaxLength imposta la lunghezza massima degli input
func (s *Sanitizer) WithMaxLength(length int) *Sanitizer {
	s.maxInputLength = length
	return s
}

// WithStrictMode abilita/disabilita lo strict mode
func (s *Sanitizer) WithStrictMode(strict bool) *Sanitizer {
	s.strictMode = strict
	return s
}

// Pattern pericolosi per SQL injection
var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\bUNION\b.*\bSELECT\b)`),
	regexp.MustCompile(`(?i)(\bSELECT\b.*\bFROM\b)`),
	regexp.MustCompile(`(?i)(\bINSERT\b.*\bINTO\b)`),
	regexp.MustCompile(`(?i)(\bUPDATE\b.*\bSET\b)`),
	regexp.MustCompile(`(?i)(\bDELETE\b.*\bFROM\b)`),
	regexp.MustCompile(`(?i)(\bDROP\b.*\bTABLE\b)`),
	regexp.MustCompile(`(?i)(\bEXEC\b|\bEXECUTE\b)`),
	regexp.MustCompile(`(?i)(--|\#|\/\*|\*\/)`),
	regexp.MustCompile(`(?i)(\bOR\b.*=.*)`),
	regexp.MustCompile(`(?i)(\bAND\b.*=.*)`),
	regexp.MustCompile(`('.*OR.*'.*=.*')`),
	regexp.MustCompile(`(;.*\bDROP\b)`),
	regexp.MustCompile(`(\bxp_.*\b)`),
	regexp.MustCompile(`(\bsp_.*\b)`),
}

// Pattern pericolosi per command injection
var commandInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(;|\||&&|\$\(|\`)`),
	regexp.MustCompile(`(>\s*\/|<\s*\/)`),
	regexp.MustCompile(`(\.\./)`),
	regexp.MustCompile(`(^\s*\/)`),
	regexp.MustCompile(`(\bcat\b|\bls\b|\brm\b|\bmv\b|\bcp\b)`),
	regexp.MustCompile(`(\bcurl\b|\bwget\b)`),
	regexp.MustCompile(`(\bchmod\b|\bchown\b)`),
	regexp.MustCompile(`(\beval\b|\bexec\b)`),
}

// Pattern pericolosi per XSS
var xssPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(<script.*?>.*?</script>)`),
	regexp.MustCompile(`(?i)(<iframe.*?>.*?</iframe>)`),
	regexp.MustCompile(`(?i)(javascript:)`),
	regexp.MustCompile(`(?i)(on\w+\s*=)`),
	regexp.MustCompile(`(?i)(<img.*?on\w+.*?>)`),
	regexp.MustCompile(`(?i)(<svg.*?on\w+.*?>)`),
	regexp.MustCompile(`(?i)(<object.*?>)`),
	regexp.MustCompile(`(?i)(<embed.*?>)`),
	regexp.MustCompile(`(?i)(data:text/html)`),
	regexp.MustCompile(`(?i)(<base.*?>)`),
}

// Pattern per prompt injection (per AI/LLM)
var promptInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(ignore\s+(previous|above|all)\s+(instructions|prompts?))`),
	regexp.MustCompile(`(?i)(disregard\s+(previous|above|all)\s+(instructions|prompts?))`),
	regexp.MustCompile(`(?i)(system\s*:|\[system\])`),
	regexp.MustCompile(`(?i)(you\s+are\s+now|from\s+now\s+on)`),
	regexp.MustCompile(`(?i)(pretend\s+to\s+be|act\s+as)`),
	regexp.MustCompile(`(?i)(%%.*%%|\{\{.*\}\})`),
	regexp.MustCompile(`(?i)(\\x[0-9a-f]{2}|\\u[0-9a-f]{4})`),
	regexp.MustCompile(`(?i)(reveal\s+your|show\s+your)\s+(system|instructions|prompt)`),
}

// SanitizeSQL rimuove caratteri pericolosi per SQL injection
func (s *Sanitizer) SanitizeSQL(input string) string {
	if len(input) > s.maxInputLength {
		input = input[:s.maxInputLength]
	}

	// Rimuovi null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// In strict mode, blocca pattern pericolosi
	if s.strictMode {
		for _, pattern := range sqlInjectionPatterns {
			if pattern.MatchString(input) {
				// Rimuovi il pattern pericoloso
				input = pattern.ReplaceAllString(input, "")
			}
		}
	}

	// Escape caratteri speciali SQL
	input = strings.ReplaceAll(input, "'", "''")
	input = strings.ReplaceAll(input, "\\", "\\\\")

	return input
}

// ValidateNoSQLInjection verifica che l'input non contenga SQL injection
func (s *Sanitizer) ValidateNoSQLInjection(input string) bool {
	for _, pattern := range sqlInjectionPatterns {
		if pattern.MatchString(input) {
			return false
		}
	}
	return true
}

// SanitizeXSS rimuove caratteri pericolosi per XSS
func (s *Sanitizer) SanitizeXSS(input string) string {
	if len(input) > s.maxInputLength {
		input = input[:s.maxInputLength]
	}

	// HTML escape
	input = html.EscapeString(input)

	// In strict mode, rimuovi pattern pericolosi
	if s.strictMode {
		for _, pattern := range xssPatterns {
			input = pattern.ReplaceAllString(input, "")
		}
	}

	return input
}

// ValidateNoXSS verifica che l'input non contenga XSS
func (s *Sanitizer) ValidateNoXSS(input string) bool {
	for _, pattern := range xssPatterns {
		if pattern.MatchString(input) {
			return false
		}
	}
	return true
}

// SanitizeCommandInjection rimuove caratteri pericolosi per command injection
func (s *Sanitizer) SanitizeCommandInjection(input string) string {
	if len(input) > s.maxInputLength {
		input = input[:s.maxInputLength]
	}

	// Rimuovi null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// In strict mode, blocca pattern pericolosi
	if s.strictMode {
		for _, pattern := range commandInjectionPatterns {
			input = pattern.ReplaceAllString(input, "")
		}
	}

	return input
}

// ValidateNoCommandInjection verifica che l'input non contenga command injection
func (s *Sanitizer) ValidateNoCommandInjection(input string) bool {
	for _, pattern := range commandInjectionPatterns {
		if pattern.MatchString(input) {
			return false
		}
	}
	return true
}

// DetectPromptInjection rileva tentativi di prompt injection
func (s *Sanitizer) DetectPromptInjection(input string) bool {
	for _, pattern := range promptInjectionPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// SanitizePromptInjection rimuove pattern di prompt injection
func (s *Sanitizer) SanitizePromptInjection(input string) string {
	if len(input) > s.maxInputLength {
		input = input[:s.maxInputLength]
	}

	for _, pattern := range promptInjectionPatterns {
		input = pattern.ReplaceAllString(input, "")
	}

	return input
}

// SanitizeFilename rimuove caratteri pericolosi dai nomi di file
func (s *Sanitizer) SanitizeFilename(filename string) string {
	// Rimuovi path traversal
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")

	// Rimuovi caratteri non stampabili
	filename = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) && r != '/' && r != '\\' && r != ':' && r != '*' && r != '?' && r != '"' && r != '<' && r != '>' && r != '|' {
			return r
		}
		return -1
	}, filename)

	// Limita lunghezza
	if len(filename) > 255 {
		filename = filename[:255]
	}

	return filename
}

// SanitizeEmail valida e pulisce un indirizzo email
func (s *Sanitizer) SanitizeEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))

	// Rimuovi caratteri non validi
	var result strings.Builder
	for _, r := range email {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '@' || r == '.' || r == '-' || r == '_' || r == '+' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ValidateEmail verifica che l'email sia valida
func (s *Sanitizer) ValidateEmail(email string) bool {
	// Regex semplice per validazione email
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// SanitizeURL pulisce e valida un URL
func (s *Sanitizer) SanitizeURL(url string) string {
	url = strings.TrimSpace(url)

	// Blocca javascript: e data: URLs
	lowerURL := strings.ToLower(url)
	if strings.HasPrefix(lowerURL, "javascript:") || strings.HasPrefix(lowerURL, "data:") {
		return ""
	}

	return url
}

// ValidateURL verifica che l'URL sia valido e sicuro
func (s *Sanitizer) ValidateURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=]+$`)
	return urlRegex.MatchString(url)
}

// SanitizeAlphanumeric rimuove tutti i caratteri non alfanumerici
func (s *Sanitizer) SanitizeAlphanumeric(input string) string {
	var result strings.Builder
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// SanitizeWhitespace normalizza gli spazi bianchi
func (s *Sanitizer) SanitizeWhitespace(input string) string {
	// Sostituisci sequenze di whitespace con un singolo spazio
	space := regexp.MustCompile(`\s+`)
	input = space.ReplaceAllString(input, " ")

	return strings.TrimSpace(input)
}

// RemoveNullBytes rimuove null bytes dall'input
func (s *Sanitizer) RemoveNullBytes(input string) string {
	return strings.ReplaceAll(input, "\x00", "")
}

// SanitizeGeneric applica tutte le sanitizzazioni di base
func (s *Sanitizer) SanitizeGeneric(input string) string {
	// Limita lunghezza
	if len(input) > s.maxInputLength {
		input = input[:s.maxInputLength]
	}

	// Rimuovi null bytes
	input = s.RemoveNullBytes(input)

	// Normalizza whitespace
	input = s.SanitizeWhitespace(input)

	// HTML escape se in strict mode
	if s.strictMode {
		input = html.EscapeString(input)
	}

	return input
}

// ValidateInput esegue tutte le validazioni
func (s *Sanitizer) ValidateInput(input string) (valid bool, reason string) {
	if len(input) > s.maxInputLength {
		return false, "input too long"
	}

	if !s.ValidateNoSQLInjection(input) {
		return false, "SQL injection detected"
	}

	if !s.ValidateNoXSS(input) {
		return false, "XSS detected"
	}

	if !s.ValidateNoCommandInjection(input) {
		return false, "command injection detected"
	}

	if s.DetectPromptInjection(input) {
		return false, "prompt injection detected"
	}

	return true, ""
}
