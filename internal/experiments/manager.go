package experiments

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Manager gestisce gli esperimenti A/B
type Manager struct {
	db               *database.DB
	metricsCollector *MetricsCollector
	analyzer         *StatisticalAnalyzer

	// Esperimenti attivi in memoria
	activeExperiments map[uuid.UUID]*Experiment
	mu                sync.RWMutex

	// Feature flags
	featureFlags map[string]*FeatureFlag
	flagsMu      sync.RWMutex

	// Background workers
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewManager crea un nuovo experiment manager
func NewManager(db *database.DB, confidenceLevel float64) *Manager {
	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		confidenceLevel = 0.95
	}

	return &Manager{
		db:                db,
		metricsCollector:  NewMetricsCollector(db),
		analyzer:          NewStatisticalAnalyzer(confidenceLevel),
		activeExperiments: make(map[uuid.UUID]*Experiment),
		featureFlags:      make(map[string]*FeatureFlag),
		stopCh:            make(chan struct{}),
	}
}

// Start avvia il manager
func (m *Manager) Start(ctx context.Context) error {
	// Avvia metrics collector
	m.metricsCollector.Start(30 * time.Second)

	// Carica esperimenti attivi dal database
	if err := m.loadActiveExperiments(ctx); err != nil {
		return fmt.Errorf("failed to load active experiments: %w", err)
	}

	// Carica feature flags
	if err := m.loadFeatureFlags(ctx); err != nil {
		return fmt.Errorf("failed to load feature flags: %w", err)
	}

	// Avvia background workers
	m.wg.Add(2)
	go m.analysisLoop()
	go m.autoRolloutLoop()

	log.Info().
		Int("active_experiments", len(m.activeExperiments)).
		Int("feature_flags", len(m.featureFlags)).
		Msg("Experiment manager started")

	return nil
}

// Stop ferma il manager
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
	m.metricsCollector.Stop()
	log.Info().Msg("Experiment manager stopped")
}

// CreateExperiment crea un nuovo esperimento
func (m *Manager) CreateExperiment(exp *Experiment) error {
	// Valida esperimento
	if err := m.validateExperiment(exp); err != nil {
		return fmt.Errorf("invalid experiment: %w", err)
	}

	// Inizializza risultati
	if exp.Results.VariantStats == nil {
		exp.Results.VariantStats = make(map[string]*VariantStatistics)
		for _, variant := range exp.Variants {
			exp.Results.VariantStats[variant.ID] = &VariantStatistics{
				VariantID:      variant.ID,
				LatencySamples: make([]int, 0),
			}
		}
	}

	// Salva nel database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.db.WithContext(ctx).Create(exp).Error; err != nil {
		return fmt.Errorf("failed to create experiment: %w", err)
	}

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Str("experiment_name", exp.Name).
		Str("type", string(exp.Type)).
		Msg("Created experiment")

	return nil
}

// StartExperiment avvia un esperimento
func (m *Manager) StartExperiment(expID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		return fmt.Errorf("experiment not found: %w", err)
	}

	if exp.Status != ExperimentStatusDraft && exp.Status != ExperimentStatusPaused {
		return fmt.Errorf("experiment must be in draft or paused status")
	}

	// Aggiorna status
	now := time.Now()
	exp.Status = ExperimentStatusRunning
	exp.StartedAt = &now

	if err := m.db.WithContext(ctx).Save(&exp).Error; err != nil {
		return fmt.Errorf("failed to start experiment: %w", err)
	}

	// Aggiungi agli esperimenti attivi
	m.mu.Lock()
	m.activeExperiments[exp.ID] = &exp
	m.mu.Unlock()

	// Registra nel metrics collector
	m.metricsCollector.RegisterExperiment(&exp)

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Str("experiment_name", exp.Name).
		Msg("Started experiment")

	return nil
}

// StopExperiment ferma un esperimento
func (m *Manager) StopExperiment(expID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		return fmt.Errorf("experiment not found: %w", err)
	}

	if exp.Status != ExperimentStatusRunning {
		return fmt.Errorf("experiment is not running")
	}

	// Esegui analisi finale
	analysis := m.analyzer.AnalyzeExperiment(&exp)
	if analysis != nil {
		exp.Results.Analysis = analysis
	}

	// Aggiorna status
	now := time.Now()
	exp.Status = ExperimentStatusCompleted
	exp.CompletedAt = &now

	if err := m.db.WithContext(ctx).Save(&exp).Error; err != nil {
		return fmt.Errorf("failed to stop experiment: %w", err)
	}

	// Rimuovi dagli esperimenti attivi
	m.mu.Lock()
	delete(m.activeExperiments, exp.ID)
	m.mu.Unlock()

	// Deregistra dal metrics collector
	m.metricsCollector.UnregisterExperiment(exp.ID)

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Str("experiment_name", exp.Name).
		Bool("has_winner", exp.WinnerID != "").
		Msg("Stopped experiment")

	return nil
}

// PauseExperiment mette in pausa un esperimento
func (m *Manager) PauseExperiment(expID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		return fmt.Errorf("experiment not found: %w", err)
	}

	if exp.Status != ExperimentStatusRunning {
		return fmt.Errorf("experiment is not running")
	}

	exp.Status = ExperimentStatusPaused

	if err := m.db.WithContext(ctx).Save(&exp).Error; err != nil {
		return fmt.Errorf("failed to pause experiment: %w", err)
	}

	// Rimuovi dagli esperimenti attivi
	m.mu.Lock()
	delete(m.activeExperiments, exp.ID)
	m.mu.Unlock()

	m.metricsCollector.UnregisterExperiment(exp.ID)

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Msg("Paused experiment")

	return nil
}

// GetVariantForUser ritorna la variante assegnata a un utente
func (m *Manager) GetVariantForUser(expID, userID uuid.UUID, req *ExperimentRequest) (*Variant, error) {
	exp, exists := m.getActiveExperiment(expID)
	if !exists {
		return nil, fmt.Errorf("experiment not active")
	}

	// Verifica filtri
	bucketing := NewUserBucketing(exp)
	if !bucketing.ShouldIncludeRequest(req) {
		return nil, nil // Request non passa i filtri
	}

	// Assegna variante
	variant := bucketing.AssignVariant(userID)
	return variant, nil
}

// RecordExperimentRequest registra una richiesta per un esperimento
func (m *Manager) RecordExperimentRequest(expID uuid.UUID, variantID string, metric RequestMetric) error {
	return m.metricsCollector.RecordRequest(expID, variantID, metric)
}

// GetExperimentResults ritorna i risultati di un esperimento
func (m *Manager) GetExperimentResults(expID uuid.UUID) (*ExperimentResults, error) {
	exp, exists := m.getActiveExperiment(expID)
	if !exists {
		// Prova a caricare dal database
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var dbExp Experiment
		if err := m.db.WithContext(ctx).First(&dbExp, "id = ?", expID).Error; err != nil {
			return nil, fmt.Errorf("experiment not found: %w", err)
		}
		return &dbExp.Results, nil
	}

	return &exp.Results, nil
}

// UpdateTrafficSplit aggiorna la distribuzione del traffico
func (m *Manager) UpdateTrafficSplit(expID uuid.UUID, trafficSplit map[string]int) error {
	// Valida che la somma sia 100
	total := 0
	for _, pct := range trafficSplit {
		total += pct
	}
	if total != 100 {
		return fmt.Errorf("traffic split must sum to 100, got %d", total)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		return fmt.Errorf("experiment not found: %w", err)
	}

	exp.TrafficSplit = trafficSplit

	if err := m.db.WithContext(ctx).Save(&exp).Error; err != nil {
		return fmt.Errorf("failed to update traffic split: %w", err)
	}

	// Aggiorna in memoria se attivo
	m.mu.Lock()
	if activeExp, ok := m.activeExperiments[expID]; ok {
		activeExp.TrafficSplit = trafficSplit
	}
	m.mu.Unlock()

	log.Info().
		Str("experiment_id", expID.String()).
		Interface("traffic_split", trafficSplit).
		Msg("Updated traffic split")

	return nil
}

// RolloutWinner promuove gradualmente il vincitore
func (m *Manager) RolloutWinner(expID uuid.UUID, targetPercentage int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		return fmt.Errorf("experiment not found: %w", err)
	}

	if exp.WinnerID == "" {
		return fmt.Errorf("no winner selected")
	}

	// Crea nuovo traffic split con percentuale target per il vincitore
	newSplit := make(map[string]int)
	remaining := 100 - targetPercentage
	otherVariants := len(exp.Variants) - 1

	for _, variant := range exp.Variants {
		if variant.ID == exp.WinnerID {
			newSplit[variant.ID] = targetPercentage
		} else if otherVariants > 0 {
			newSplit[variant.ID] = remaining / otherVariants
		}
	}

	return m.UpdateTrafficSplit(expID, newSplit)
}

// GradualRollout esegue un rollout graduale del vincitore
func (m *Manager) GradualRollout(expID uuid.UUID, steps []int, stepDuration time.Duration) error {
	for i, targetPct := range steps {
		if err := m.RolloutWinner(expID, targetPct); err != nil {
			return fmt.Errorf("rollout step %d failed: %w", i, err)
		}

		log.Info().
			Str("experiment_id", expID.String()).
			Int("step", i+1).
			Int("percentage", targetPct).
			Msg("Gradual rollout step completed")

		// Attendi prima del prossimo step (tranne ultimo)
		if i < len(steps)-1 {
			time.Sleep(stepDuration)
		}
	}

	return nil
}

// analysisLoop esegue periodicamente l'analisi degli esperimenti attivi
func (m *Manager) analysisLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.analyzeActiveExperiments()
		case <-m.stopCh:
			return
		}
	}
}

// analyzeActiveExperiments analizza tutti gli esperimenti attivi
func (m *Manager) analyzeActiveExperiments() {
	m.mu.RLock()
	experiments := make([]*Experiment, 0, len(m.activeExperiments))
	for _, exp := range m.activeExperiments {
		experiments = append(experiments, exp)
	}
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, exp := range experiments {
		// Verifica se ha campione sufficiente
		if !m.analyzer.HasSufficientSampleSize(exp) {
			continue
		}

		// Esegui analisi
		analysis := m.analyzer.AnalyzeExperiment(exp)
		if analysis == nil {
			continue
		}

		exp.Results.Analysis = analysis

		// Salva nel database
		if err := m.db.WithContext(ctx).Save(exp).Error; err != nil {
			log.Error().
				Err(err).
				Str("experiment_id", exp.ID.String()).
				Msg("Failed to save experiment analysis")
		}

		log.Info().
			Str("experiment_id", exp.ID.String()).
			Bool("significant", analysis.IsSignificant).
			Str("recommended_winner", analysis.RecommendedWinner).
			Msg("Analyzed experiment")
	}
}

// autoRolloutLoop gestisce l'auto-rollout dei vincitori
func (m *Manager) autoRolloutLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAutoRollout()
		case <-m.stopCh:
			return
		}
	}
}

// checkAutoRollout verifica e esegue auto-rollout
func (m *Manager) checkAutoRollout() {
	m.mu.RLock()
	experiments := make([]*Experiment, 0, len(m.activeExperiments))
	for _, exp := range m.activeExperiments {
		if exp.AutoRollout {
			experiments = append(experiments, exp)
		}
	}
	m.mu.RUnlock()

	for _, exp := range experiments {
		if exp.Results.Analysis == nil {
			continue
		}

		analysis := exp.Results.Analysis

		// Verifica se c'è un vincitore significativo
		if !analysis.IsSignificant || analysis.RecommendedWinner == "" {
			continue
		}

		// Verifica confidence level
		if exp.ConfidenceRequired > 0 && analysis.ConfidenceLevel < exp.ConfidenceRequired {
			continue
		}

		// Verifica che p-value sia sotto soglia
		if analysis.PValue >= (1.0 - m.analyzer.confidenceLevel) {
			continue
		}

		// Seleziona vincitore
		exp.WinnerID = analysis.RecommendedWinner

		// Esegui rollout graduale automatico
		log.Info().
			Str("experiment_id", exp.ID.String()).
			Str("winner_id", exp.WinnerID).
			Msg("Auto-rollout: Starting gradual rollout")

		// Rollout graduale: 25% -> 50% -> 75% -> 100%
		steps := []int{25, 50, 75, 100}
		if err := m.GradualRollout(exp.ID, steps, 1*time.Hour); err != nil {
			log.Error().
				Err(err).
				Str("experiment_id", exp.ID.String()).
				Msg("Auto-rollout failed")
			continue
		}

		// Ferma esperimento dopo rollout completo
		if err := m.StopExperiment(exp.ID); err != nil {
			log.Error().
				Err(err).
				Str("experiment_id", exp.ID.String()).
				Msg("Failed to stop experiment after auto-rollout")
		}
	}
}

// loadActiveExperiments carica esperimenti attivi dal database
func (m *Manager) loadActiveExperiments(ctx context.Context) error {
	var experiments []Experiment
	if err := m.db.WithContext(ctx).
		Where("status = ?", ExperimentStatusRunning).
		Find(&experiments).Error; err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range experiments {
		exp := &experiments[i]
		m.activeExperiments[exp.ID] = exp
		m.metricsCollector.RegisterExperiment(exp)
	}

	return nil
}

// loadFeatureFlags carica feature flags dal database
func (m *Manager) loadFeatureFlags(ctx context.Context) error {
	var flags []FeatureFlag
	if err := m.db.WithContext(ctx).Find(&flags).Error; err != nil {
		return err
	}

	m.flagsMu.Lock()
	defer m.flagsMu.Unlock()

	for i := range flags {
		flag := &flags[i]
		m.featureFlags[flag.Name] = flag
	}

	return nil
}

// validateExperiment valida la configurazione di un esperimento
func (m *Manager) validateExperiment(exp *Experiment) error {
	if exp.Name == "" {
		return fmt.Errorf("experiment name is required")
	}

	if len(exp.Variants) < 2 {
		return fmt.Errorf("at least 2 variants required")
	}

	// Valida traffic split
	if len(exp.TrafficSplit) == 0 {
		// Genera split uniforme
		pct := 100 / len(exp.Variants)
		exp.TrafficSplit = make(map[string]int)
		for _, variant := range exp.Variants {
			exp.TrafficSplit[variant.ID] = pct
		}
	}

	total := 0
	for _, pct := range exp.TrafficSplit {
		total += pct
	}
	if total != 100 {
		return fmt.Errorf("traffic split must sum to 100, got %d", total)
	}

	return nil
}

// Helper functions
func (m *Manager) getActiveExperiment(expID uuid.UUID) (*Experiment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exp, ok := m.activeExperiments[expID]
	return exp, ok
}

// GetExperiment carica un esperimento dal database
func (m *Manager) GetExperiment(expID uuid.UUID) (*Experiment, error) {
	// Prova prima dalla cache in memoria
	if exp, ok := m.getActiveExperiment(expID); ok {
		return exp, nil
	}

	// Altrimenti carica dal database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exp Experiment
	if err := m.db.WithContext(ctx).First(&exp, "id = ?", expID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("experiment not found")
		}
		return nil, err
	}

	return &exp, nil
}

// ListExperiments lista tutti gli esperimenti con filtri
func (m *Manager) ListExperiments(status ExperimentStatus, limit, offset int) ([]Experiment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := m.db.WithContext(ctx)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var experiments []Experiment
	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&experiments).Error; err != nil {
		return nil, err
	}

	return experiments, nil
}

// Feature Flag methods

// GetFeatureFlag ritorna un feature flag
func (m *Manager) GetFeatureFlag(name string) (*FeatureFlag, bool) {
	m.flagsMu.RLock()
	defer m.flagsMu.RUnlock()
	flag, ok := m.featureFlags[name]
	return flag, ok
}

// IsFeatureEnabled verifica se un feature flag è abilitato per un utente
func (m *Manager) IsFeatureEnabled(name string, userID uuid.UUID) bool {
	flag, ok := m.GetFeatureFlag(name)
	if !ok {
		return false
	}
	return flag.IsEnabledForUser(userID)
}

// CreateFeatureFlag crea un nuovo feature flag
func (m *Manager) CreateFeatureFlag(flag *FeatureFlag) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.db.WithContext(ctx).Create(flag).Error; err != nil {
		return err
	}

	m.flagsMu.Lock()
	m.featureFlags[flag.Name] = flag
	m.flagsMu.Unlock()

	return nil
}

// UpdateFeatureFlag aggiorna un feature flag
func (m *Manager) UpdateFeatureFlag(flag *FeatureFlag) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.db.WithContext(ctx).Save(flag).Error; err != nil {
		return err
	}

	m.flagsMu.Lock()
	m.featureFlags[flag.Name] = flag
	m.flagsMu.Unlock()

	return nil
}
