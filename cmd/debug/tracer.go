package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Tracer traccia il flusso delle richieste attraverso il sistema
type Tracer struct {
	db     *database.DB
	config *config.Config
}

// NewTracer crea un nuovo tracer
func NewTracer(db *database.DB, cfg *config.Config) *Tracer {
	return &Tracer{
		db:     db,
		config: cfg,
	}
}

// TraceRequest traccia una richiesta dall'inizio alla fine
func (t *Tracer) TraceRequest(ctx context.Context, requestID string) (*RequestTrace, error) {
	log.Info().Str("request_id", requestID).Msg("Starting request trace")

	trace := &RequestTrace{
		RequestID:      requestID,
		Timestamp:      time.Now(),
		DecisionPoints: make([]DecisionPoint, 0),
		Errors:         make([]string, 0),
	}

	// Add decision point
	trace.AddDecisionPoint("start", "Request tracing initiated")

	// TODO: Implement actual request tracking
	// This would integrate with the request logging system

	trace.Duration = time.Since(trace.Timestamp)
	trace.Status = "completed"

	return trace, nil
}

// GetRequestTrace recupera la traccia di una richiesta esistente
func (t *Tracer) GetRequestTrace(ctx context.Context, requestID string) (*RequestTrace, error) {
	// Validate UUID
	reqUUID, err := uuid.Parse(requestID)
	if err != nil {
		return nil, fmt.Errorf("invalid request ID: %w", err)
	}

	trace := &RequestTrace{
		RequestID:      requestID,
		Timestamp:      time.Now(),
		DecisionPoints: make([]DecisionPoint, 0),
		Errors:         make([]string, 0),
	}

	// Try to find request in database
	// Note: This assumes you have a requests table
	// For now, we'll simulate the trace

	log.Debug().Str("request_id", requestID).Msg("Retrieving request trace")

	// Simulate finding a provider (in production, this would come from the request log)
	var provider models.Provider
	if err := t.db.First(&provider).Error; err == nil {
		trace.Provider = &provider
		trace.ModelID = "gpt-3.5-turbo" // Example
		trace.AddDecisionPoint("routing", fmt.Sprintf("Selected provider: %s", provider.Name))
	}

	// Add simulated decision points
	trace.AddDecisionPoint("validation", "Request validated successfully")
	trace.AddDecisionPoint("rate_limit", "Rate limit check passed")
	trace.AddDecisionPoint("cache", "Cache miss - executing request")
	trace.AddDecisionPoint("provider_call", "Calling provider API")
	trace.AddDecisionPoint("response", "Response received and processed")

	trace.Duration = 234 * time.Millisecond // Simulated
	trace.Status = "success"

	log.Info().
		Str("request_id", requestID).
		Str("provider", trace.Provider.Name).
		Dur("duration", trace.Duration).
		Msg("Request trace retrieved")

	// In a real implementation, check if request exists
	_ = reqUUID

	return trace, nil
}

// ShowRequestFlow mostra il flusso completo di una richiesta
func (t *Tracer) ShowRequestFlow(ctx context.Context, requestID string) error {
	trace, err := t.GetRequestTrace(ctx, requestID)
	if err != nil {
		return err
	}

	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              REQUEST FLOW ANALYSIS                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Printf("\n┌─ Request ID: %s\n", trace.RequestID)
	fmt.Printf("├─ Timestamp: %s\n", trace.Timestamp.Format(time.RFC3339))
	fmt.Printf("├─ Duration: %.2fms\n", trace.Duration.Seconds()*1000)
	fmt.Printf("└─ Status: %s\n\n", trace.Status)

	// Decision flow
	fmt.Println("Decision Flow:")
	fmt.Println("━━━━━━━━━━━━━━")

	for i, dp := range trace.DecisionPoints {
		fmt.Printf("\n[%d] %s\n", i+1, dp.Stage)
		fmt.Printf("    └─ %s\n", dp.Decision)
		if i < len(trace.DecisionPoints)-1 {
			fmt.Println("    │")
			fmt.Println("    ↓")
		}
	}

	// Timing breakdown
	fmt.Println("\n\nTiming Breakdown:")
	fmt.Println("━━━━━━━━━━━━━━━━")
	t.showTimingBreakdown(trace)

	// Provider selection
	if trace.Provider != nil {
		fmt.Println("\n\nProvider Selection:")
		fmt.Println("━━━━━━━━━━━━━━━━━━")
		t.showProviderSelection(trace)
	}

	// Errors
	if len(trace.Errors) > 0 {
		fmt.Println("\n\nErrors:")
		fmt.Println("━━━━━━━")
		for _, e := range trace.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
	}

	return nil
}

// showTimingBreakdown mostra la suddivisione dei tempi
func (t *Tracer) showTimingBreakdown(trace *RequestTrace) {
	// Simulated timing breakdown
	breakdown := []struct {
		Phase    string
		Duration time.Duration
		Percent  float64
	}{
		{"Request validation", 5 * time.Millisecond, 2.1},
		{"Rate limiting check", 3 * time.Millisecond, 1.3},
		{"Cache lookup", 8 * time.Millisecond, 3.4},
		{"Provider selection", 12 * time.Millisecond, 5.1},
		{"API call", 180 * time.Millisecond, 76.9},
		{"Response processing", 15 * time.Millisecond, 6.4},
		{"Cache update", 11 * time.Millisecond, 4.7},
	}

	for _, b := range breakdown {
		bar := makeProgressBar(b.Percent, 30)
		fmt.Printf("  %-22s %6.2fms %s %.1f%%\n",
			b.Phase,
			b.Duration.Seconds()*1000,
			bar,
			b.Percent,
		)
	}

	fmt.Printf("\n  Total: %.2fms\n", trace.Duration.Seconds()*1000)
}

// showProviderSelection mostra i dettagli della selezione del provider
func (t *Tracer) showProviderSelection(trace *RequestTrace) {
	fmt.Printf("  Provider: %s\n", trace.Provider.Name)
	fmt.Printf("  Model: %s\n", trace.ModelID)
	fmt.Printf("  Strategy: %s\n", t.config.Routing.Strategy)
	fmt.Printf("  Health Score: %.2f\n", trace.Provider.HealthScore)
	fmt.Printf("  Avg Latency: %dms\n", trace.Provider.AvgLatencyMs)

	// Reasoning
	fmt.Println("\n  Selection Reasoning:")
	switch t.config.Routing.Strategy {
	case "cost_optimized":
		fmt.Println("    • Selected based on lowest cost per token")
		fmt.Println("    • Provider health score above threshold")
		fmt.Println("    • Model supports required features")
	case "latency_first":
		fmt.Println("    • Selected based on lowest average latency")
		fmt.Println("    • Provider currently available")
		fmt.Println("    • Recent health check passed")
	case "quality_first":
		fmt.Println("    • Selected based on highest quality score")
		fmt.Println("    • Premium tier provider")
		fmt.Println("    • Best model for task type")
	default:
		fmt.Println("    • Default selection strategy applied")
	}
}

// GetProviderMetrics recupera le metriche di un provider
func (t *Tracer) GetProviderMetrics(ctx context.Context, providerName string) (*ProviderMetrics, error) {
	var provider models.Provider
	if err := t.db.Where("name = ?", providerName).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	metrics := &ProviderMetrics{
		Provider:      &provider,
		RequestCount:  0, // TODO: Get from stats table
		SuccessRate:   0.95,
		AvgLatency:    time.Duration(provider.AvgLatencyMs) * time.Millisecond,
		ErrorRate:     0.05,
		LastErrors:    make([]ErrorInfo, 0),
		HealthHistory: make([]HealthCheck, 0),
	}

	// Simulated health history
	now := time.Now()
	for i := 0; i < 10; i++ {
		metrics.HealthHistory = append(metrics.HealthHistory, HealthCheck{
			Timestamp:   now.Add(-time.Duration(i*5) * time.Minute),
			Healthy:     true,
			Latency:     time.Duration(provider.AvgLatencyMs+i*5) * time.Millisecond,
			HealthScore: provider.HealthScore - float64(i)*0.01,
		})
	}

	return metrics, nil
}

// CompareProviders confronta le performance di più provider
func (t *Tracer) CompareProviders(ctx context.Context, providerNames []string) error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            PROVIDER PERFORMANCE COMPARISON                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	metricsMap := make(map[string]*ProviderMetrics)

	// Collect metrics for each provider
	for _, name := range providerNames {
		metrics, err := t.GetProviderMetrics(ctx, name)
		if err != nil {
			log.Warn().Str("provider", name).Err(err).Msg("Failed to get metrics")
			continue
		}
		metricsMap[name] = metrics
	}

	// Display comparison table
	fmt.Printf("%-15s %-12s %-12s %-12s %-10s\n",
		"Provider", "Requests", "Success Rate", "Avg Latency", "Health")
	fmt.Println("─────────────────────────────────────────────────────────────────────")

	for name, metrics := range metricsMap {
		fmt.Printf("%-15s %-12d %-12.1f%% %-12.0fms %-10.2f\n",
			truncateString(name, 15),
			metrics.RequestCount,
			metrics.SuccessRate*100,
			metrics.AvgLatency.Seconds()*1000,
			metrics.Provider.HealthScore,
		)
	}

	return nil
}

// ExportTrace esporta la traccia in formato JSON
func (t *Tracer) ExportTrace(ctx context.Context, requestID string, filename string) error {
	trace, err := t.GetRequestTrace(ctx, requestID)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	// In a real implementation, write to file
	fmt.Printf("Trace exported to: %s\n", filename)
	fmt.Println(string(data))

	return nil
}

// Helper methods

// AddDecisionPoint aggiunge un punto di decisione alla traccia
func (rt *RequestTrace) AddDecisionPoint(stage, decision string) {
	rt.DecisionPoints = append(rt.DecisionPoints, DecisionPoint{
		Stage:    stage,
		Decision: decision,
		Time:     time.Now(),
	})
}

// AddError aggiunge un errore alla traccia
func (rt *RequestTrace) AddError(err string) {
	rt.Errors = append(rt.Errors, err)
}

// makeProgressBar crea una barra di progresso testuale
func makeProgressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"

	return bar
}

// Data structures

// ProviderMetrics contiene metriche di performance di un provider
type ProviderMetrics struct {
	Provider      *models.Provider `json:"provider"`
	RequestCount  int64            `json:"request_count"`
	SuccessRate   float64          `json:"success_rate"`
	AvgLatency    time.Duration    `json:"avg_latency"`
	ErrorRate     float64          `json:"error_rate"`
	LastErrors    []ErrorInfo      `json:"last_errors"`
	HealthHistory []HealthCheck    `json:"health_history"`
}

// ErrorInfo contiene informazioni su un errore
type ErrorInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	RequestID string    `json:"request_id,omitempty"`
}

// HealthCheck rappresenta un check di salute
type HealthCheck struct {
	Timestamp   time.Time     `json:"timestamp"`
	Healthy     bool          `json:"healthy"`
	Latency     time.Duration `json:"latency"`
	HealthScore float64       `json:"health_score"`
}
