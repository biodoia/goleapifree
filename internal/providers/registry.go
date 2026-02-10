package providers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrProviderNotFound      = errors.New("provider not found")
	ErrProviderAlreadyExists = errors.New("provider already exists")
	ErrNoProvidersAvailable  = errors.New("no providers available")
)

// Registry gestisce tutti i provider disponibili
type Registry struct {
	providers map[string]Provider
	metadata  map[string]*ProviderMetadata
	mu        sync.RWMutex
}

// ProviderMetadata contiene metadata su un provider
type ProviderMetadata struct {
	Name              string
	Type              string // "openai", "anthropic", "local", etc.
	Status            ProviderStatus
	RegisteredAt      time.Time
	LastHealthCheck   time.Time
	HealthCheckStatus HealthStatus
	ErrorCount        int
	SuccessCount      int
	AvgLatency        time.Duration
	Features          map[Feature]bool
}

// ProviderStatus rappresenta lo stato di un provider
type ProviderStatus string

const (
	ProviderStatusActive      ProviderStatus = "active"
	ProviderStatusInactive    ProviderStatus = "inactive"
	ProviderStatusUnhealthy   ProviderStatus = "unhealthy"
	ProviderStatusMaintenance ProviderStatus = "maintenance"
)

// HealthStatus rappresenta lo stato di salute di un provider
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// NewRegistry crea un nuovo registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		metadata:  make(map[string]*ProviderMetadata),
	}
}

// Register registra un nuovo provider
func (r *Registry) Register(name string, provider Provider, providerType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyExists, name)
	}

	r.providers[name] = provider

	// Initialize metadata
	features := make(map[Feature]bool)
	for _, feature := range []Feature{
		FeatureStreaming,
		FeatureTools,
		FeatureJSONMode,
		FeatureVision,
		FeatureFunctionCall,
		FeatureSystemMsg,
	} {
		features[feature] = provider.SupportsFeature(feature)
	}

	r.metadata[name] = &ProviderMetadata{
		Name:              name,
		Type:              providerType,
		Status:            ProviderStatusActive,
		RegisteredAt:      time.Now(),
		HealthCheckStatus: HealthStatusUnknown,
		Features:          features,
	}

	log.Info().
		Str("provider", name).
		Str("type", providerType).
		Msg("Provider registered")

	return nil
}

// Unregister rimuove un provider dal registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	delete(r.providers, name)
	delete(r.metadata, name)

	log.Info().
		Str("provider", name).
		Msg("Provider unregistered")

	return nil
}

// Get restituisce un provider per nome
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	// Check if provider is available
	meta := r.metadata[name]
	if meta.Status != ProviderStatusActive {
		return nil, fmt.Errorf("provider %s is not active (status: %s)", name, meta.Status)
	}

	return provider, nil
}

// GetOrFirst restituisce il provider specificato, o il primo disponibile
func (r *Registry) GetOrFirst(name string) (Provider, error) {
	// Try to get specific provider
	if name != "" {
		provider, err := r.Get(name)
		if err == nil {
			return provider, nil
		}
	}

	// Get first available provider
	return r.GetFirst()
}

// GetFirst restituisce il primo provider disponibile
func (r *Registry) GetFirst() (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find first active and healthy provider
	for name, meta := range r.metadata {
		if meta.Status == ProviderStatusActive &&
		   meta.HealthCheckStatus != HealthStatusUnhealthy {
			return r.providers[name], nil
		}
	}

	// If no healthy provider, return first active
	for name, meta := range r.metadata {
		if meta.Status == ProviderStatusActive {
			return r.providers[name], nil
		}
	}

	return nil, ErrNoProvidersAvailable
}

// List restituisce tutti i provider registrati
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListActive restituisce solo i provider attivi
func (r *Registry) ListActive() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0)
	for name, meta := range r.metadata {
		if meta.Status == ProviderStatusActive {
			names = append(names, name)
		}
	}
	return names
}

// GetMetadata restituisce i metadata di un provider
func (r *Registry) GetMetadata(name string) (*ProviderMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, exists := r.metadata[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	// Return a copy to prevent modifications
	metaCopy := *meta
	metaCopy.Features = make(map[Feature]bool)
	for k, v := range meta.Features {
		metaCopy.Features[k] = v
	}

	return &metaCopy, nil
}

// GetAllMetadata restituisce i metadata di tutti i provider
func (r *Registry) GetAllMetadata() map[string]*ProviderMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ProviderMetadata, len(r.metadata))
	for name, meta := range r.metadata {
		metaCopy := *meta
		metaCopy.Features = make(map[Feature]bool)
		for k, v := range meta.Features {
			metaCopy.Features[k] = v
		}
		result[name] = &metaCopy
	}

	return result
}

// SetStatus imposta lo stato di un provider
func (r *Registry) SetStatus(name string, status ProviderStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta, exists := r.metadata[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}

	oldStatus := meta.Status
	meta.Status = status

	log.Info().
		Str("provider", name).
		Str("old_status", string(oldStatus)).
		Str("new_status", string(status)).
		Msg("Provider status changed")

	return nil
}

// HealthCheck esegue health check su tutti i provider
func (r *Registry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	providerNames := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providerNames = append(providerNames, name)
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup

	for _, name := range providerNames {
		wg.Add(1)
		go func(providerName string) {
			defer wg.Done()

			r.mu.RLock()
			provider := r.providers[providerName]
			r.mu.RUnlock()

			start := time.Now()
			err := provider.HealthCheck(ctx)
			latency := time.Since(start)

			r.mu.Lock()
			meta := r.metadata[providerName]
			meta.LastHealthCheck = time.Now()

			if err != nil {
				meta.ErrorCount++
				meta.HealthCheckStatus = HealthStatusUnhealthy
				results[providerName] = err

				log.Warn().
					Err(err).
					Str("provider", providerName).
					Msg("Provider health check failed")
			} else {
				meta.SuccessCount++
				meta.HealthCheckStatus = HealthStatusHealthy
				meta.AvgLatency = (meta.AvgLatency + latency) / 2

				log.Debug().
					Str("provider", providerName).
					Dur("latency", latency).
					Msg("Provider health check succeeded")
			}
			r.mu.Unlock()
		}(name)
	}

	wg.Wait()
	return results
}

// RecordSuccess registra un'operazione riuscita
func (r *Registry) RecordSuccess(name string, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if meta, exists := r.metadata[name]; exists {
		meta.SuccessCount++
		if meta.AvgLatency == 0 {
			meta.AvgLatency = latency
		} else {
			meta.AvgLatency = (meta.AvgLatency + latency) / 2
		}
	}
}

// RecordError registra un errore
func (r *Registry) RecordError(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if meta, exists := r.metadata[name]; exists {
		meta.ErrorCount++

		// Set to unhealthy after multiple consecutive errors
		if meta.ErrorCount > 5 && meta.HealthCheckStatus != HealthStatusUnhealthy {
			meta.HealthCheckStatus = HealthStatusUnhealthy
			log.Warn().
				Str("provider", name).
				Int("error_count", meta.ErrorCount).
				Msg("Provider marked as unhealthy")
		}
	}
}

// GetStats restituisce statistiche aggregate
func (r *Registry) GetStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		TotalProviders:   len(r.providers),
		ActiveProviders:  0,
		HealthyProviders: 0,
		TotalRequests:    0,
		TotalErrors:      0,
	}

	var totalLatency time.Duration
	var healthyCount int

	for _, meta := range r.metadata {
		if meta.Status == ProviderStatusActive {
			stats.ActiveProviders++
		}
		if meta.HealthCheckStatus == HealthStatusHealthy {
			stats.HealthyProviders++
		}

		stats.TotalRequests += meta.SuccessCount + meta.ErrorCount
		stats.TotalErrors += meta.ErrorCount

		if meta.AvgLatency > 0 {
			totalLatency += meta.AvgLatency
			healthyCount++
		}
	}

	if healthyCount > 0 {
		stats.AvgLatency = totalLatency / time.Duration(healthyCount)
	}

	return stats
}

// RegistryStats contiene statistiche del registry
type RegistryStats struct {
	TotalProviders   int
	ActiveProviders  int
	HealthyProviders int
	TotalRequests    int
	TotalErrors      int
	AvgLatency       time.Duration
}

// Count restituisce il numero totale di provider
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// Clear rimuove tutti i provider
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers = make(map[string]Provider)
	r.metadata = make(map[string]*ProviderMetadata)

	log.Info().Msg("Registry cleared")
}
