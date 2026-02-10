package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

// DiscoveryConfig contiene la configurazione del discovery engine
type DiscoveryConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Interval             time.Duration `yaml:"interval"`
	GitHubToken          string        `yaml:"github_token"`
	GitHubEnabled        bool          `yaml:"github_enabled"`
	ScraperEnabled       bool          `yaml:"scraper_enabled"`
	MaxConcurrent        int           `yaml:"max_concurrent"`
	ValidationTimeout    time.Duration `yaml:"validation_timeout"`
	MinHealthScore       float64       `yaml:"min_health_score"`
	DiscoverySearchTerms []string      `yaml:"discovery_search_terms"`
}

// Engine è il motore principale per l'auto-discovery
type Engine struct {
	config    *DiscoveryConfig
	db        *database.DB
	github    *GitHubDiscovery
	scraper   *WebScraper
	validator *Validator
	logger    zerolog.Logger

	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
}

// Candidate rappresenta un potenziale provider scoperto
type Candidate struct {
	Name        string
	BaseURL     string
	AuthType    models.AuthType
	Source      string
	RepoURL     string
	Description string
	Stars       int
	Language    string
	LastUpdate  time.Time

	// Metadata estratto dal README
	Models            []string
	SupportedFormats  []string
	RateLimitInfo     string
	DocumentationURL  string
}

// NewEngine crea un nuovo discovery engine
func NewEngine(config *DiscoveryConfig, db *database.DB, logger zerolog.Logger) *Engine {
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 5
	}
	if config.ValidationTimeout == 0 {
		config.ValidationTimeout = 30 * time.Second
	}
	if config.MinHealthScore == 0 {
		config.MinHealthScore = 0.6
	}
	if len(config.DiscoverySearchTerms) == 0 {
		config.DiscoverySearchTerms = []string{
			"free llm api",
			"free ai api",
			"free gpt api",
			"free claude api",
			"ai proxy free",
			"llm gateway",
		}
	}

	validator := NewValidator(config.ValidationTimeout, logger)

	engine := &Engine{
		config:    config,
		db:        db,
		validator: validator,
		logger:    logger.With().Str("component", "discovery").Logger(),
	}

	// Inizializza GitHub discovery se abilitato
	if config.GitHubEnabled && config.GitHubToken != "" {
		engine.github = NewGitHubDiscovery(config.GitHubToken, logger)
	}

	// Inizializza scraper se abilitato
	if config.ScraperEnabled {
		engine.scraper = NewWebScraper(logger)
	}

	return engine
}

// Start avvia il discovery engine con scheduler
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = true
	e.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	e.logger.Info().Msg("Starting discovery engine")

	// Esegui discovery iniziale
	go func() {
		if err := e.RunDiscovery(ctx); err != nil {
			e.logger.Error().Err(err).Msg("Initial discovery failed")
		}
	}()

	// Scheduler per discovery periodico
	ticker := time.NewTicker(e.config.Interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				e.logger.Info().Msg("Discovery engine stopped")
				return
			case <-ticker.C:
				e.logger.Info().Msg("Running scheduled discovery")
				if err := e.RunDiscovery(ctx); err != nil {
					e.logger.Error().Err(err).Msg("Scheduled discovery failed")
				}
			}
		}
	}()

	return nil
}

// Stop ferma il discovery engine
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.logger.Info().Msg("Stopping discovery engine")
	if e.cancel != nil {
		e.cancel()
	}
	e.running = false
}

// RunDiscovery esegue un ciclo completo di discovery
func (e *Engine) RunDiscovery(ctx context.Context) error {
	startTime := time.Now()
	e.logger.Info().Msg("Starting discovery run")

	var candidates []Candidate
	var mu sync.Mutex
	var wg sync.WaitGroup

	// GitHub discovery
	if e.github != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			githubCandidates, err := e.github.DiscoverAPIs(ctx, e.config.DiscoverySearchTerms)
			if err != nil {
				e.logger.Error().Err(err).Msg("GitHub discovery failed")
				return
			}
			mu.Lock()
			candidates = append(candidates, githubCandidates...)
			mu.Unlock()
			e.logger.Info().Int("count", len(githubCandidates)).Msg("GitHub discovery completed")
		}()
	}

	// Web scraping
	if e.scraper != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scrapedCandidates, err := e.scraper.ScrapeAwesomeLists(ctx)
			if err != nil {
				e.logger.Error().Err(err).Msg("Web scraping failed")
				return
			}
			mu.Lock()
			candidates = append(candidates, scrapedCandidates...)
			mu.Unlock()
			e.logger.Info().Int("count", len(scrapedCandidates)).Msg("Web scraping completed")
		}()
	}

	wg.Wait()

	e.logger.Info().
		Int("total_candidates", len(candidates)).
		Dur("duration", time.Since(startTime)).
		Msg("Discovery phase completed")

	// Valida e salva i candidati
	if len(candidates) > 0 {
		return e.processAndSaveCandidates(ctx, candidates)
	}

	return nil
}

// processAndSaveCandidates valida e salva i candidati nel database
func (e *Engine) processAndSaveCandidates(ctx context.Context, candidates []Candidate) error {
	e.logger.Info().Int("count", len(candidates)).Msg("Processing candidates")

	// Deduplicazione
	candidates = e.deduplicateCandidates(candidates)
	e.logger.Info().Int("after_dedup", len(candidates)).Msg("Candidates after deduplication")

	// Filtra candidati già esistenti nel database
	existingProviders, err := e.getExistingProviders()
	if err != nil {
		return err
	}
	candidates = e.filterExisting(candidates, existingProviders)
	e.logger.Info().Int("new_candidates", len(candidates)).Msg("New candidates to validate")

	// Validazione parallela con limite di concorrenza
	semaphore := make(chan struct{}, e.config.MaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	validProviders := make([]models.Provider, 0)

	for _, candidate := range candidates {
		wg.Add(1)
		go func(c Candidate) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			e.logger.Debug().
				Str("name", c.Name).
				Str("url", c.BaseURL).
				Msg("Validating candidate")

			result, err := e.validator.ValidateEndpoint(ctx, c.BaseURL, c.AuthType)
			if err != nil {
				e.logger.Warn().
					Err(err).
					Str("name", c.Name).
					Str("url", c.BaseURL).
					Msg("Candidate validation failed")
				return
			}

			// Controlla se passa il threshold di health score
			if result.HealthScore < e.config.MinHealthScore {
				e.logger.Info().
					Str("name", c.Name).
					Float64("health_score", result.HealthScore).
					Float64("min_score", e.config.MinHealthScore).
					Msg("Candidate rejected due to low health score")
				return
			}

			// Crea provider dal candidato validato
			provider := e.candidateToProvider(c, result)

			mu.Lock()
			validProviders = append(validProviders, provider)
			mu.Unlock()

			e.logger.Info().
				Str("name", c.Name).
				Float64("health_score", result.HealthScore).
				Int("latency_ms", result.LatencyMs).
				Msg("Candidate validated successfully")
		}(candidate)
	}

	wg.Wait()

	// Salva i provider validi nel database
	if len(validProviders) > 0 {
		for _, provider := range validProviders {
			if err := e.db.Create(&provider).Error; err != nil {
				e.logger.Error().
					Err(err).
					Str("provider", provider.Name).
					Msg("Failed to save provider")
				continue
			}
			e.logger.Info().
				Str("provider", provider.Name).
				Str("url", provider.BaseURL).
				Msg("New provider saved to database")
		}
	}

	e.logger.Info().
		Int("validated", len(validProviders)).
		Int("saved", len(validProviders)).
		Msg("Discovery run completed")

	return nil
}

// deduplicateCandidates rimuove candidati duplicati
func (e *Engine) deduplicateCandidates(candidates []Candidate) []Candidate {
	seen := make(map[string]bool)
	unique := make([]Candidate, 0)

	for _, c := range candidates {
		key := c.BaseURL
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}

	return unique
}

// getExistingProviders recupera i provider esistenti dal database
func (e *Engine) getExistingProviders() ([]models.Provider, error) {
	var providers []models.Provider
	err := e.db.Find(&providers).Error
	return providers, err
}

// filterExisting filtra candidati già presenti nel database
func (e *Engine) filterExisting(candidates []Candidate, existing []models.Provider) []Candidate {
	existingURLs := make(map[string]bool)
	for _, p := range existing {
		existingURLs[p.BaseURL] = true
	}

	filtered := make([]Candidate, 0)
	for _, c := range candidates {
		if !existingURLs[c.BaseURL] {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// candidateToProvider converte un candidato validato in un Provider
func (e *Engine) candidateToProvider(c Candidate, result *ValidationResult) models.Provider {
	// Determina il tier basato su vari fattori
	tier := 3 // experimental di default
	if c.Stars > 1000 {
		tier = 1 // premium
	} else if c.Stars > 100 {
		tier = 2 // standard
	}

	return models.Provider{
		Name:              c.Name,
		Type:              models.ProviderTypeFree,
		Status:            models.ProviderStatusActive,
		BaseURL:           c.BaseURL,
		AuthType:          c.AuthType,
		Tier:              tier,
		DiscoveredAt:      time.Now(),
		LastVerified:      time.Now(),
		Source:            c.Source,
		SupportsStreaming: result.SupportsStreaming,
		SupportsTools:     result.SupportsTools,
		SupportsJSON:      result.SupportsJSON,
		LastHealthCheck:   time.Now(),
		HealthScore:       result.HealthScore,
		AvgLatencyMs:      result.LatencyMs,
	}
}

// IsRunning restituisce true se il discovery engine è in esecuzione
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
