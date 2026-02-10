package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/internal/providers/openai"
	"github.com/rs/zerolog/log"
)

// ProviderManager gestisce l'inizializzazione e l'uso dei provider
type ProviderManager struct {
	registry *providers.Registry
	mu       sync.RWMutex
}

// NewProviderManager crea un nuovo manager
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		registry: providers.NewRegistry(),
	}
}

// RegisterOpenAICompatible registra un provider OpenAI-compatible
func (pm *ProviderManager) RegisterOpenAICompatible(name, baseURL, apiKey string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	client := openai.NewClient(name, baseURL, apiKey)
	return pm.registry.Register(name, client, "openai")
}

// RegisterWithConfig registra un provider con configurazione avanzata
func (pm *ProviderManager) RegisterWithConfig(config ProviderConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	client := openai.NewClient(config.Name, config.BaseURL, config.APIKey)

	// Apply custom configuration
	if config.Timeout > 0 {
		client.SetTimeout(config.Timeout)
	}
	if config.MaxRetries > 0 {
		client.SetMaxRetries(config.MaxRetries)
	}

	// Set features
	for feature, supported := range config.Features {
		client.SetFeature(feature, supported)
	}

	return pm.registry.Register(config.Name, client, config.Type)
}

// ProviderConfig contiene la configurazione di un provider
type ProviderConfig struct {
	Name       string
	Type       string
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	MaxRetries int
	Features   map[providers.Feature]bool
}

// GetProvider restituisce un provider per nome
func (pm *ProviderManager) GetProvider(name string) (providers.Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.registry.Get(name)
}

// GetOrFirstProvider restituisce il provider specificato o il primo disponibile
func (pm *ProviderManager) GetOrFirstProvider(name string) (providers.Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.registry.GetOrFirst(name)
}

// ListProviders restituisce tutti i provider registrati
func (pm *ProviderManager) ListProviders() []ProviderInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := pm.registry.List()
	allMeta := pm.registry.GetAllMetadata()

	infos := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		if meta, ok := allMeta[name]; ok {
			infos = append(infos, ProviderInfo{
				Name:              meta.Name,
				Type:              meta.Type,
				Status:            string(meta.Status),
				HealthStatus:      string(meta.HealthCheckStatus),
				LastHealthCheck:   meta.LastHealthCheck,
				ErrorCount:        meta.ErrorCount,
				SuccessCount:      meta.SuccessCount,
				AvgLatency:        meta.AvgLatency,
				Features:          meta.Features,
			})
		}
	}

	return infos
}

// ProviderInfo contiene informazioni su un provider
type ProviderInfo struct {
	Name              string
	Type              string
	Status            string
	HealthStatus      string
	LastHealthCheck   time.Time
	ErrorCount        int
	SuccessCount      int
	AvgLatency        time.Duration
	Features          map[providers.Feature]bool
}

// ChatCompletion esegue una richiesta usando un provider specifico o il primo disponibile
func (pm *ProviderManager) ChatCompletion(ctx context.Context, providerName string, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	provider, err := pm.GetOrFirstProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	start := time.Now()
	resp, err := provider.ChatCompletion(ctx, req)
	latency := time.Since(start)

	if err != nil {
		pm.registry.RecordError(provider.Name())
		return nil, err
	}

	pm.registry.RecordSuccess(provider.Name(), latency)
	return resp, nil
}

// Stream esegue una richiesta streaming usando un provider specifico
func (pm *ProviderManager) Stream(ctx context.Context, providerName string, req *providers.ChatRequest, handler providers.StreamHandler) error {
	provider, err := pm.GetOrFirstProvider(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	start := time.Now()
	err = provider.Stream(ctx, req, handler)
	latency := time.Since(start)

	if err != nil {
		pm.registry.RecordError(provider.Name())
		return err
	}

	pm.registry.RecordSuccess(provider.Name(), latency)
	return nil
}

// HealthCheckAll esegue health check su tutti i provider
func (pm *ProviderManager) HealthCheckAll(ctx context.Context) map[string]error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.registry.HealthCheck(ctx)
}

// StartHealthCheckWorker avvia un worker che esegue health check periodici
func (pm *ProviderManager) StartHealthCheckWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", interval).
		Msg("Started health check worker")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Health check worker stopped")
			return
		case <-ticker.C:
			results := pm.HealthCheckAll(ctx)

			healthyCount := 0
			totalCount := 0
			for provider, err := range results {
				totalCount++
				if err != nil {
					log.Warn().
						Err(err).
						Str("provider", provider).
						Msg("Provider unhealthy")
				} else {
					healthyCount++
				}
			}

			log.Debug().
				Int("healthy", healthyCount).
				Int("total", totalCount).
				Msg("Health check completed")
		}
	}
}

// GetStats restituisce statistiche aggregate
func (pm *ProviderManager) GetStats() providers.RegistryStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.registry.GetStats()
}

// SetProviderStatus imposta lo stato di un provider
func (pm *ProviderManager) SetProviderStatus(name string, status providers.ProviderStatus) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.registry.SetStatus(name, status)
}

// RemoveProvider rimuove un provider
func (pm *ProviderManager) RemoveProvider(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.registry.Unregister(name)
}

// LoadBalancedRequest esegue una richiesta con load balancing tra provider disponibili
func (pm *ProviderManager) LoadBalancedRequest(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	pm.mu.RLock()
	activeProviders := pm.registry.ListActive()
	pm.mu.RUnlock()

	if len(activeProviders) == 0 {
		return nil, providers.ErrNoProvidersAvailable
	}

	// Prova ogni provider in ordine
	var lastErr error
	for _, providerName := range activeProviders {
		provider, err := pm.GetProvider(providerName)
		if err != nil {
			continue
		}

		start := time.Now()
		resp, err := provider.ChatCompletion(ctx, req)
		latency := time.Since(start)

		if err != nil {
			lastErr = err
			pm.registry.RecordError(provider.Name())
			log.Warn().
				Err(err).
				Str("provider", provider.Name()).
				Msg("Provider request failed, trying next")
			continue
		}

		pm.registry.RecordSuccess(provider.Name(), latency)
		log.Debug().
			Str("provider", provider.Name()).
			Dur("latency", latency).
			Msg("Request succeeded")

		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}

	return nil, providers.ErrNoProvidersAvailable
}

// DefaultProviderConfigs contiene configurazioni predefinite per provider comuni
var DefaultProviderConfigs = map[string]ProviderConfig{
	"openai": {
		Name:       "openai",
		Type:       "openai",
		BaseURL:    "https://api.openai.com",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Features: map[providers.Feature]bool{
			providers.FeatureStreaming:    true,
			providers.FeatureTools:        true,
			providers.FeatureJSONMode:     true,
			providers.FeatureVision:       true,
			providers.FeatureFunctionCall: true,
			providers.FeatureSystemMsg:    true,
		},
	},
	"groq": {
		Name:       "groq",
		Type:       "openai",
		BaseURL:    "https://api.groq.com/openai",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Features: map[providers.Feature]bool{
			providers.FeatureStreaming: true,
			providers.FeatureTools:     true,
			providers.FeatureJSONMode:  true,
			providers.FeatureVision:    false,
		},
	},
	"together": {
		Name:       "together",
		Type:       "openai",
		BaseURL:    "https://api.together.xyz",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Features: map[providers.Feature]bool{
			providers.FeatureStreaming: true,
			providers.FeatureTools:     false,
			providers.FeatureJSONMode:  true,
			providers.FeatureVision:    false,
		},
	},
	"ollama": {
		Name:       "ollama",
		Type:       "openai",
		BaseURL:    "http://localhost:11434",
		Timeout:    120 * time.Second, // Longer timeout for local models
		MaxRetries: 1,
		Features: map[providers.Feature]bool{
			providers.FeatureStreaming: true,
			providers.FeatureTools:     false,
			providers.FeatureJSONMode:  true,
			providers.FeatureVision:    false,
		},
	},
}

// LoadDefaultProviders carica i provider predefiniti con le API key fornite
func (pm *ProviderManager) LoadDefaultProviders(apiKeys map[string]string) error {
	for name, config := range DefaultProviderConfigs {
		if apiKey, ok := apiKeys[name]; ok && apiKey != "" {
			config.APIKey = apiKey
			if err := pm.RegisterWithConfig(config); err != nil {
				log.Warn().
					Err(err).
					Str("provider", name).
					Msg("Failed to register provider")
			} else {
				log.Info().
					Str("provider", name).
					Msg("Registered provider")
			}
		}
	}
	return nil
}
