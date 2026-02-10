package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

// Validator valida endpoint API
type Validator struct {
	timeout    time.Duration
	httpClient *http.Client
	logger     zerolog.Logger
}

// ValidationResult contiene i risultati della validazione
type ValidationResult struct {
	IsValid           bool
	HealthScore       float64
	LatencyMs         int
	SupportsStreaming bool
	SupportsTools     bool
	SupportsJSON      bool
	ErrorMessage      string
	Compatibility     string // "openai", "anthropic", "unknown"
	AvailableModels   []string
}

// NewValidator crea un nuovo validator
func NewValidator(timeout time.Duration, logger zerolog.Logger) *Validator {
	return &Validator{
		timeout: timeout,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Permetti fino a 5 redirect
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		logger: logger.With().Str("component", "validator").Logger(),
	}
}

// ValidateEndpoint valida un endpoint API
func (v *Validator) ValidateEndpoint(ctx context.Context, baseURL string, authType models.AuthType) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:     false,
		HealthScore: 0.0,
	}

	v.logger.Debug().
		Str("url", baseURL).
		Str("auth", string(authType)).
		Msg("Validating endpoint")

	// 1. Test di connettività base
	if err := v.testConnectivity(ctx, baseURL, result); err != nil {
		result.ErrorMessage = fmt.Sprintf("connectivity test failed: %v", err)
		return result, err
	}

	// 2. Determina compatibilità (OpenAI, Anthropic, etc.)
	v.detectCompatibility(ctx, baseURL, authType, result)

	// 3. Test endpoint specifici
	if result.Compatibility == "openai" {
		v.testOpenAIEndpoint(ctx, baseURL, authType, result)
	} else if result.Compatibility == "anthropic" {
		v.testAnthropicEndpoint(ctx, baseURL, authType, result)
	}

	// 4. Calcola health score finale
	v.calculateHealthScore(result)

	result.IsValid = result.HealthScore > 0.3

	v.logger.Debug().
		Str("url", baseURL).
		Float64("health_score", result.HealthScore).
		Bool("valid", result.IsValid).
		Msg("Validation completed")

	return result, nil
}

// testConnectivity testa la connettività base dell'endpoint
func (v *Validator) testConnectivity(ctx context.Context, baseURL string, result *ValidationResult) error {
	start := time.Now()

	// Prova una semplice GET request
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	result.LatencyMs = int(time.Since(start).Milliseconds())

	// Accetta sia 200 OK che 401/403 (endpoint protetti)
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusUnauthorized &&
		resp.StatusCode != http.StatusForbidden &&
		resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// detectCompatibility rileva la compatibilità dell'API
func (v *Validator) detectCompatibility(ctx context.Context, baseURL string, authType models.AuthType, result *ValidationResult) {
	// Prova a rilevare dalla struttura URL
	urlLower := strings.ToLower(baseURL)

	if strings.Contains(urlLower, "openai") ||
		strings.Contains(urlLower, "/v1/chat/completions") ||
		strings.Contains(urlLower, "/v1/completions") {
		result.Compatibility = "openai"
		return
	}

	if strings.Contains(urlLower, "anthropic") ||
		strings.Contains(urlLower, "/v1/messages") ||
		strings.Contains(urlLower, "claude") {
		result.Compatibility = "anthropic"
		return
	}

	// Prova a rilevare facendo richieste di test
	if v.testOpenAIStructure(ctx, baseURL, authType) {
		result.Compatibility = "openai"
		return
	}

	if v.testAnthropicStructure(ctx, baseURL, authType) {
		result.Compatibility = "anthropic"
		return
	}

	result.Compatibility = "unknown"
}

// testOpenAIStructure testa se l'endpoint è compatibile con OpenAI
func (v *Validator) testOpenAIStructure(ctx context.Context, baseURL string, authType models.AuthType) bool {
	// Prova endpoint /v1/models
	modelsURL := strings.TrimSuffix(baseURL, "/") + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return false
	}

	v.addAuthHeader(req, authType, "dummy-key")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Se otteniamo 200, 401, o 403, probabilmente è OpenAI-compatible
	if resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden {

		// Prova a parsare la risposta
		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			// Controlla struttura OpenAI
			if _, ok := data["data"]; ok {
				return true
			}
		}
	}

	return false
}

// testAnthropicStructure testa se l'endpoint è compatibile con Anthropic
func (v *Validator) testAnthropicStructure(ctx context.Context, baseURL string, authType models.AuthType) bool {
	// Prova endpoint /v1/messages
	messagesURL := strings.TrimSuffix(baseURL, "/") + "/v1/messages"

	// Prepara un payload minimo
	payload := map[string]interface{}{
		"model":      "claude-3-haiku-20240307",
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", messagesURL, bytes.NewReader(body))
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	v.addAuthHeader(req, authType, "dummy-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Se otteniamo 200, 401, 403, o 400 (bad request per dummy key), probabilmente è Anthropic-compatible
	return resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden ||
		resp.StatusCode == http.StatusBadRequest
}

// testOpenAIEndpoint testa funzionalità specifiche OpenAI
func (v *Validator) testOpenAIEndpoint(ctx context.Context, baseURL string, authType models.AuthType, result *ValidationResult) {
	// Test lista modelli
	modelsURL := strings.TrimSuffix(baseURL, "/") + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return
	}

	v.addAuthHeader(req, authType, "dummy-key")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			result.AvailableModels = make([]string, 0, len(data.Data))
			for _, model := range data.Data {
				result.AvailableModels = append(result.AvailableModels, model.ID)
			}
		}
	}

	// Assume supporto per streaming e JSON
	result.SupportsStreaming = true
	result.SupportsJSON = true
	result.SupportsTools = true // La maggior parte supporta function calling
}

// testAnthropicEndpoint testa funzionalità specifiche Anthropic
func (v *Validator) testAnthropicEndpoint(ctx context.Context, baseURL string, authType models.AuthType, result *ValidationResult) {
	// Anthropic non ha un endpoint /models, ma supporta modelli noti
	result.AvailableModels = []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}

	result.SupportsStreaming = true
	result.SupportsJSON = true
	result.SupportsTools = true
}

// calculateHealthScore calcola il punteggio di salute finale
func (v *Validator) calculateHealthScore(result *ValidationResult) {
	score := 0.0

	// Base score se la connessione funziona
	score += 0.3

	// Bonus per compatibilità nota
	if result.Compatibility != "unknown" {
		score += 0.2
	}

	// Bonus per latenza bassa
	if result.LatencyMs > 0 {
		if result.LatencyMs < 500 {
			score += 0.2
		} else if result.LatencyMs < 1000 {
			score += 0.1
		}
	}

	// Bonus per funzionalità
	if result.SupportsStreaming {
		score += 0.1
	}
	if result.SupportsJSON {
		score += 0.1
	}
	if result.SupportsTools {
		score += 0.1
	}

	// Bonus per modelli disponibili
	if len(result.AvailableModels) > 0 {
		score += 0.1
	}

	result.HealthScore = score
}

// addAuthHeader aggiunge l'header di autenticazione appropriato
func (v *Validator) addAuthHeader(req *http.Request, authType models.AuthType, key string) {
	switch authType {
	case models.AuthTypeAPIKey:
		req.Header.Set("Authorization", "Bearer "+key)
	case models.AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+key)
	case models.AuthTypeNone:
		// Nessuna auth
	}
}

// TestRateLimit verifica i rate limit di un endpoint
func (v *Validator) TestRateLimit(ctx context.Context, baseURL string, authType models.AuthType) (rateLimit int, err error) {
	// Fai una richiesta e controlla gli header
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return 0, err
	}

	v.addAuthHeader(req, authType, "dummy-key")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Controlla header comuni per rate limit
	rateLimitHeaders := []string{
		"X-RateLimit-Limit",
		"RateLimit-Limit",
		"X-Rate-Limit-Limit",
	}

	for _, header := range rateLimitHeaders {
		if value := resp.Header.Get(header); value != "" {
			var limit int
			if _, err := fmt.Sscanf(value, "%d", &limit); err == nil {
				return limit, nil
			}
		}
	}

	// Nessun rate limit trovato
	return 0, nil
}

// MeasureLatency misura la latenza media di un endpoint
func (v *Validator) MeasureLatency(ctx context.Context, baseURL string, samples int) (avgMs int, err error) {
	if samples <= 0 {
		samples = 5
	}

	var totalMs int64
	successCount := 0

	for i := 0; i < samples; i++ {
		start := time.Now()

		req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
		if err != nil {
			continue
		}

		resp, err := v.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		elapsed := time.Since(start).Milliseconds()
		totalMs += elapsed
		successCount++

		// Piccolo delay tra le richieste
		time.Sleep(100 * time.Millisecond)
	}

	if successCount == 0 {
		return 0, fmt.Errorf("all latency measurements failed")
	}

	return int(totalMs / int64(successCount)), nil
}
