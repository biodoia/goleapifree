package experiments

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MetricsCollector raccoglie metriche per gli esperimenti
type MetricsCollector struct {
	db          *database.DB
	experiments map[uuid.UUID]*Experiment
	mu          sync.RWMutex

	// Buffer per metriche in tempo reale
	metricsBuffer map[uuid.UUID]map[string]*VariantMetrics
	bufferMu      sync.Mutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// VariantMetrics rappresenta metriche in tempo reale per una variante
type VariantMetrics struct {
	VariantID      string
	Requests       []RequestMetric
	LatencySamples []int
	mu             sync.Mutex
}

// RequestMetric rappresenta la metrica di una singola richiesta
type RequestMetric struct {
	Timestamp    time.Time
	Success      bool
	LatencyMs    int
	Cost         float64
	Tokens       int
	ErrorMessage string
	Satisfaction *int // 1-5 rating, se disponibile
}

// NewMetricsCollector crea un nuovo collector per esperimenti
func NewMetricsCollector(db *database.DB) *MetricsCollector {
	return &MetricsCollector{
		db:            db,
		experiments:   make(map[uuid.UUID]*Experiment),
		metricsBuffer: make(map[uuid.UUID]map[string]*VariantMetrics),
		stopCh:        make(chan struct{}),
	}
}

// Start avvia il collector
func (mc *MetricsCollector) Start(aggregateInterval time.Duration) {
	mc.wg.Add(1)
	go mc.aggregateLoop(aggregateInterval)
	log.Info().Msg("Experiment metrics collector started")
}

// Stop ferma il collector
func (mc *MetricsCollector) Stop() {
	close(mc.stopCh)
	mc.wg.Wait()
	mc.flush()
	log.Info().Msg("Experiment metrics collector stopped")
}

// RegisterExperiment registra un esperimento per il tracking
func (mc *MetricsCollector) RegisterExperiment(exp *Experiment) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.experiments[exp.ID] = exp

	mc.bufferMu.Lock()
	mc.metricsBuffer[exp.ID] = make(map[string]*VariantMetrics)
	for _, variant := range exp.Variants {
		mc.metricsBuffer[exp.ID][variant.ID] = &VariantMetrics{
			VariantID:      variant.ID,
			Requests:       make([]RequestMetric, 0),
			LatencySamples: make([]int, 0),
		}
	}
	mc.bufferMu.Unlock()

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Str("experiment_name", exp.Name).
		Int("variants", len(exp.Variants)).
		Msg("Registered experiment for metrics collection")
}

// UnregisterExperiment rimuove un esperimento dal tracking
func (mc *MetricsCollector) UnregisterExperiment(expID uuid.UUID) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.experiments, expID)

	mc.bufferMu.Lock()
	delete(mc.metricsBuffer, expID)
	mc.bufferMu.Unlock()

	log.Info().
		Str("experiment_id", expID.String()).
		Msg("Unregistered experiment from metrics collection")
}

// RecordRequest registra una richiesta per una variante
func (mc *MetricsCollector) RecordRequest(expID uuid.UUID, variantID string, metric RequestMetric) error {
	mc.bufferMu.Lock()
	defer mc.bufferMu.Unlock()

	expMetrics, exists := mc.metricsBuffer[expID]
	if !exists {
		return nil // Esperimento non registrato, ignora silenziosamente
	}

	variantMetrics, exists := expMetrics[variantID]
	if !exists {
		return nil // Variante non trovata
	}

	variantMetrics.mu.Lock()
	variantMetrics.Requests = append(variantMetrics.Requests, metric)
	if metric.Success {
		variantMetrics.LatencySamples = append(variantMetrics.LatencySamples, metric.LatencyMs)
	}
	variantMetrics.mu.Unlock()

	return nil
}

// aggregateLoop aggrega periodicamente le metriche
func (mc *MetricsCollector) aggregateLoop(interval time.Duration) {
	defer mc.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.aggregate()
		case <-mc.stopCh:
			return
		}
	}
}

// aggregate aggrega le metriche buffer negli esperimenti
func (mc *MetricsCollector) aggregate() {
	mc.bufferMu.Lock()
	defer mc.bufferMu.Unlock()

	for expID, variantMetrics := range mc.metricsBuffer {
		exp, exists := mc.getExperiment(expID)
		if !exists {
			continue
		}

		// Aggrega ogni variante
		for variantID, metrics := range variantMetrics {
			metrics.mu.Lock()
			if len(metrics.Requests) == 0 {
				metrics.mu.Unlock()
				continue
			}

			// Calcola statistiche
			stats := mc.calculateStats(metrics)

			// Aggiorna le statistiche dell'esperimento
			if exp.Results.VariantStats == nil {
				exp.Results.VariantStats = make(map[string]*VariantStatistics)
			}

			existingStats, ok := exp.Results.VariantStats[variantID]
			if !ok {
				existingStats = &VariantStatistics{
					VariantID:      variantID,
					LatencySamples: make([]int, 0),
				}
				exp.Results.VariantStats[variantID] = existingStats
			}

			// Merge delle statistiche
			mc.mergeStats(existingStats, stats)

			// Pulisci buffer processato
			metrics.Requests = make([]RequestMetric, 0)
			metrics.mu.Unlock()
		}

		exp.Results.TotalRequests = mc.calculateTotalRequests(exp.Results.VariantStats)
		exp.Results.LastUpdated = time.Now()

		// Aggiorna experiment in memoria
		mc.updateExperiment(exp)
	}

	// Flush su database
	mc.flush()
}

// calculateStats calcola le statistiche da un set di metriche
func (mc *MetricsCollector) calculateStats(metrics *VariantMetrics) *VariantStatistics {
	stats := &VariantStatistics{
		VariantID:      metrics.VariantID,
		LatencySamples: make([]int, 0, len(metrics.LatencySamples)),
	}

	var totalLatency int64
	var totalCost float64
	var totalTokens int64
	var totalSatisfaction float64
	var satisfactionCount int64

	for _, req := range metrics.Requests {
		stats.Requests++

		if req.Success {
			stats.Successes++
			totalLatency += int64(req.LatencyMs)
		} else {
			stats.Failures++
		}

		totalCost += req.Cost
		totalTokens += int64(req.Tokens)

		if req.Satisfaction != nil {
			totalSatisfaction += float64(*req.Satisfaction)
			satisfactionCount++
		}
	}

	// Calcola medie
	if stats.Requests > 0 {
		stats.SuccessRate = float64(stats.Successes) / float64(stats.Requests)
		stats.AvgCost = totalCost / float64(stats.Requests)
		stats.AvgTokens = float64(totalTokens) / float64(stats.Requests)
		stats.TotalCost = totalCost
		stats.TotalTokens = totalTokens
	}

	if stats.Successes > 0 {
		stats.AvgLatencyMs = float64(totalLatency) / float64(stats.Successes)
		stats.TotalLatencyMs = totalLatency
	}

	if satisfactionCount > 0 {
		stats.SatisfactionScore = totalSatisfaction / float64(satisfactionCount)
		stats.SatisfactionCount = satisfactionCount
	}

	// Calcola percentili
	if len(metrics.LatencySamples) > 0 {
		samples := make([]int, len(metrics.LatencySamples))
		copy(samples, metrics.LatencySamples)
		sort.Ints(samples)

		stats.LatencySamples = samples
		stats.P50LatencyMs = float64(mc.percentile(samples, 50))
		stats.P95LatencyMs = float64(mc.percentile(samples, 95))
		stats.P99LatencyMs = float64(mc.percentile(samples, 99))
	}

	return stats
}

// percentile calcola il percentile da un array ordinato
func (mc *MetricsCollector) percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}

	index := int(float64(len(sorted)) * float64(p) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// mergeStats unisce le nuove statistiche con quelle esistenti
func (mc *MetricsCollector) mergeStats(existing *VariantStatistics, new *VariantStatistics) {
	// Accumula contatori
	existing.Requests += new.Requests
	existing.Successes += new.Successes
	existing.Failures += new.Failures
	existing.TotalLatencyMs += new.TotalLatencyMs
	existing.TotalCost += new.TotalCost
	existing.TotalTokens += new.TotalTokens

	// Ricalcola medie
	if existing.Requests > 0 {
		existing.SuccessRate = float64(existing.Successes) / float64(existing.Requests)
		existing.AvgCost = existing.TotalCost / float64(existing.Requests)
		existing.AvgTokens = float64(existing.TotalTokens) / float64(existing.Requests)
	}

	if existing.Successes > 0 {
		existing.AvgLatencyMs = float64(existing.TotalLatencyMs) / float64(existing.Successes)
	}

	// Merge latency samples (mantieni solo ultimi N)
	maxSamples := 10000
	existing.LatencySamples = append(existing.LatencySamples, new.LatencySamples...)
	if len(existing.LatencySamples) > maxSamples {
		existing.LatencySamples = existing.LatencySamples[len(existing.LatencySamples)-maxSamples:]
	}

	// Ricalcola percentili se abbiamo samples
	if len(existing.LatencySamples) > 0 {
		samples := make([]int, len(existing.LatencySamples))
		copy(samples, existing.LatencySamples)
		sort.Ints(samples)

		existing.P50LatencyMs = float64(mc.percentile(samples, 50))
		existing.P95LatencyMs = float64(mc.percentile(samples, 95))
		existing.P99LatencyMs = float64(mc.percentile(samples, 99))
	}

	// Merge satisfaction
	if new.SatisfactionCount > 0 {
		totalSatisfaction := (existing.SatisfactionScore * float64(existing.SatisfactionCount)) +
			(new.SatisfactionScore * float64(new.SatisfactionCount))
		existing.SatisfactionCount += new.SatisfactionCount
		existing.SatisfactionScore = totalSatisfaction / float64(existing.SatisfactionCount)
	}
}

// calculateTotalRequests calcola il totale delle richieste
func (mc *MetricsCollector) calculateTotalRequests(stats map[string]*VariantStatistics) int64 {
	var total int64
	for _, s := range stats {
		total += s.Requests
	}
	return total
}

// flush salva gli esperimenti aggiornati nel database
func (mc *MetricsCollector) flush() {
	mc.mu.RLock()
	experiments := make([]*Experiment, 0, len(mc.experiments))
	for _, exp := range mc.experiments {
		experiments = append(experiments, exp)
	}
	mc.mu.RUnlock()

	if len(experiments) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, exp := range experiments {
		if err := mc.db.WithContext(ctx).Save(exp).Error; err != nil {
			log.Error().
				Err(err).
				Str("experiment_id", exp.ID.String()).
				Msg("Failed to save experiment metrics")
		}
	}

	log.Debug().
		Int("experiments", len(experiments)).
		Msg("Flushed experiment metrics to database")
}

// GetExperimentStats ritorna le statistiche attuali di un esperimento
func (mc *MetricsCollector) GetExperimentStats(expID uuid.UUID) (map[string]*VariantStatistics, bool) {
	exp, exists := mc.getExperiment(expID)
	if !exists {
		return nil, false
	}

	return exp.Results.VariantStats, true
}

// GetVariantStats ritorna le statistiche di una specifica variante
func (mc *MetricsCollector) GetVariantStats(expID uuid.UUID, variantID string) (*VariantStatistics, bool) {
	exp, exists := mc.getExperiment(expID)
	if !exists {
		return nil, false
	}

	stats, ok := exp.Results.VariantStats[variantID]
	return stats, ok
}

// CompareVariants confronta due varianti per una metrica
func (mc *MetricsCollector) CompareVariants(expID uuid.UUID, variantA, variantB, metric string) (*VariantComparison, error) {
	statsA, okA := mc.GetVariantStats(expID, variantA)
	statsB, okB := mc.GetVariantStats(expID, variantB)

	if !okA || !okB {
		return nil, nil
	}

	comparison := &VariantComparison{
		VariantA: variantA,
		VariantB: variantB,
		Metric:   metric,
	}

	var valueA, valueB float64

	switch metric {
	case "success_rate":
		valueA = statsA.SuccessRate
		valueB = statsB.SuccessRate
		comparison.BetterVariant = variantA
		if valueB > valueA {
			comparison.BetterVariant = variantB
		}
	case "latency":
		valueA = statsA.AvgLatencyMs
		valueB = statsB.AvgLatencyMs
		comparison.BetterVariant = variantA
		if valueB < valueA {
			comparison.BetterVariant = variantB
		}
	case "cost":
		valueA = statsA.AvgCost
		valueB = statsB.AvgCost
		comparison.BetterVariant = variantA
		if valueB < valueA {
			comparison.BetterVariant = variantB
		}
	case "satisfaction":
		valueA = statsA.SatisfactionScore
		valueB = statsB.SatisfactionScore
		comparison.BetterVariant = variantA
		if valueB > valueA {
			comparison.BetterVariant = variantB
		}
	default:
		return nil, nil
	}

	if valueA > 0 {
		comparison.DiffPercent = ((valueB - valueA) / valueA) * 100
	}

	return comparison, nil
}

// Helper functions
func (mc *MetricsCollector) getExperiment(expID uuid.UUID) (*Experiment, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	exp, ok := mc.experiments[expID]
	return exp, ok
}

func (mc *MetricsCollector) updateExperiment(exp *Experiment) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.experiments[exp.ID] = exp
}
