package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// LiveMetrics rappresenta metriche aggregate in tempo reale
type LiveMetrics struct {
	// Request counting
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64

	// Rolling averages (ultimi N minuti)
	AvgLatencyMs       float64
	AvgTokensPerReq    float64
	AvgCostPerReq      float64

	// Real-time cost
	TotalCost          float64
	CostPerMinute      float64
	EstimatedHourlyCost float64

	// Active users
	ActiveUsers        int
	UniqueUsersToday   int

	// Provider-specific
	ProviderMetrics map[uuid.UUID]*ProviderLiveMetrics

	// Timestamps
	LastUpdated time.Time
	WindowStart time.Time
}

// ProviderLiveMetrics rappresenta metriche live per provider
type ProviderLiveMetrics struct {
	ProviderID    uuid.UUID
	RequestCount  int64
	SuccessRate   float64
	AvgLatencyMs  float64
	ErrorRate     float64
	TotalTokens   int64
	TotalCost     float64
	LastRequest   time.Time
	IsHealthy     bool
}

// RequestWindow rappresenta una finestra temporale per metriche rolling
type RequestWindow struct {
	Timestamp    time.Time
	Count        int64
	LatencySum   int64
	TokensSum    int64
	CostSum      float64
	SuccessCount int64
	ErrorCount   int64
}

// Aggregator aggrega metriche in real-time
type Aggregator struct {
	// Current metrics
	metrics LiveMetrics
	mu      sync.RWMutex

	// Rolling windows (per calcolare medie)
	windows     []*RequestWindow
	windowSize  time.Duration
	windowCount int
	windowMu    sync.RWMutex

	// Active users tracking
	activeUsers    map[uuid.UUID]time.Time
	uniqueUsers    map[uuid.UUID]bool
	usersMu        sync.RWMutex
	userTimeout    time.Duration

	// Cleanup
	cleanupTicker *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewAggregator crea un nuovo aggregator
func NewAggregator(windowSize time.Duration, windowCount int, userTimeout time.Duration) *Aggregator {
	if windowSize == 0 {
		windowSize = 1 * time.Minute
	}
	if windowCount == 0 {
		windowCount = 60 // 60 finestre di 1 minuto = 1 ora
	}
	if userTimeout == 0 {
		userTimeout = 5 * time.Minute
	}

	agg := &Aggregator{
		metrics: LiveMetrics{
			ProviderMetrics: make(map[uuid.UUID]*ProviderLiveMetrics),
			WindowStart:     time.Now(),
		},
		windows:      make([]*RequestWindow, 0, windowCount),
		windowSize:   windowSize,
		windowCount:  windowCount,
		activeUsers:  make(map[uuid.UUID]time.Time),
		uniqueUsers:  make(map[uuid.UUID]bool),
		userTimeout:  userTimeout,
		stopCh:       make(chan struct{}),
	}

	return agg
}

// Start avvia l'aggregator
func (a *Aggregator) Start() {
	a.cleanupTicker = time.NewTicker(1 * time.Minute)
	a.wg.Add(1)
	go a.cleanupLoop()

	log.Info().Msg("Real-time aggregator started")
}

// Stop ferma l'aggregator
func (a *Aggregator) Stop() {
	if a.cleanupTicker != nil {
		a.cleanupTicker.Stop()
	}
	close(a.stopCh)
	a.wg.Wait()

	log.Info().Msg("Real-time aggregator stopped")
}

// RecordRequest registra una nuova richiesta
func (a *Aggregator) RecordRequest(req *RequestRecord) {
	now := time.Now()

	// Update global metrics
	a.mu.Lock()
	a.metrics.TotalRequests++
	if req.Success {
		a.metrics.SuccessfulRequests++
	} else {
		a.metrics.FailedRequests++
	}
	a.metrics.TotalCost += req.Cost
	a.metrics.LastUpdated = now
	a.mu.Unlock()

	// Update provider metrics
	a.updateProviderMetrics(req)

	// Add to rolling window
	a.addToWindow(req, now)

	// Track user activity
	if req.UserID != uuid.Nil {
		a.trackUser(req.UserID, now)
	}

	// Recalculate aggregates
	a.recalculateAggregates()
}

// updateProviderMetrics aggiorna le metriche per provider
func (a *Aggregator) updateProviderMetrics(req *RequestRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	providerMetrics, exists := a.metrics.ProviderMetrics[req.ProviderID]
	if !exists {
		providerMetrics = &ProviderLiveMetrics{
			ProviderID: req.ProviderID,
			IsHealthy:  true,
		}
		a.metrics.ProviderMetrics[req.ProviderID] = providerMetrics
	}

	providerMetrics.RequestCount++
	providerMetrics.TotalTokens += int64(req.Tokens)
	providerMetrics.TotalCost += req.Cost
	providerMetrics.LastRequest = req.Timestamp

	// Update success/error rate
	if req.Success {
		successCount := int64(float64(providerMetrics.RequestCount) * providerMetrics.SuccessRate)
		successCount++
		providerMetrics.SuccessRate = float64(successCount) / float64(providerMetrics.RequestCount)
	} else {
		errorCount := int64(float64(providerMetrics.RequestCount) * providerMetrics.ErrorRate)
		errorCount++
		providerMetrics.ErrorRate = float64(errorCount) / float64(providerMetrics.RequestCount)
	}

	// Update average latency (exponential moving average)
	alpha := 0.2 // Smoothing factor
	if providerMetrics.AvgLatencyMs == 0 {
		providerMetrics.AvgLatencyMs = float64(req.LatencyMs)
	} else {
		providerMetrics.AvgLatencyMs = alpha*float64(req.LatencyMs) + (1-alpha)*providerMetrics.AvgLatencyMs
	}

	// Health check
	providerMetrics.IsHealthy = providerMetrics.SuccessRate > 0.7 &&
		providerMetrics.AvgLatencyMs < 10000
}

// addToWindow aggiunge la richiesta alla finestra rolling
func (a *Aggregator) addToWindow(req *RequestRecord, now time.Time) {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()

	// Find or create current window
	var currentWindow *RequestWindow
	if len(a.windows) > 0 {
		lastWindow := a.windows[len(a.windows)-1]
		if now.Sub(lastWindow.Timestamp) < a.windowSize {
			currentWindow = lastWindow
		}
	}

	if currentWindow == nil {
		currentWindow = &RequestWindow{
			Timestamp: now,
		}
		a.windows = append(a.windows, currentWindow)

		// Mantieni solo le ultime N finestre
		if len(a.windows) > a.windowCount {
			a.windows = a.windows[1:]
		}
	}

	// Update window
	currentWindow.Count++
	currentWindow.LatencySum += int64(req.LatencyMs)
	currentWindow.TokensSum += int64(req.Tokens)
	currentWindow.CostSum += req.Cost
	if req.Success {
		currentWindow.SuccessCount++
	} else {
		currentWindow.ErrorCount++
	}
}

// trackUser traccia l'attività dell'utente
func (a *Aggregator) trackUser(userID uuid.UUID, timestamp time.Time) {
	a.usersMu.Lock()
	defer a.usersMu.Unlock()

	a.activeUsers[userID] = timestamp
	a.uniqueUsers[userID] = true
}

// recalculateAggregates ricalcola le metriche aggregate
func (a *Aggregator) recalculateAggregates() {
	a.windowMu.RLock()
	defer a.windowMu.RUnlock()

	if len(a.windows) == 0 {
		return
	}

	var totalCount int64
	var totalLatency int64
	var totalTokens int64
	var totalCost float64

	for _, window := range a.windows {
		totalCount += window.Count
		totalLatency += window.LatencySum
		totalTokens += window.TokensSum
		totalCost += window.CostSum
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if totalCount > 0 {
		a.metrics.AvgLatencyMs = float64(totalLatency) / float64(totalCount)
		a.metrics.AvgTokensPerReq = float64(totalTokens) / float64(totalCount)
		a.metrics.AvgCostPerReq = totalCost / float64(totalCount)
	}

	// Calculate cost per minute and estimated hourly cost
	windowDuration := time.Since(a.windows[0].Timestamp)
	if windowDuration.Minutes() > 0 {
		a.metrics.CostPerMinute = totalCost / windowDuration.Minutes()
		a.metrics.EstimatedHourlyCost = a.metrics.CostPerMinute * 60
	}

	// Update active users count
	a.usersMu.RLock()
	a.metrics.ActiveUsers = len(a.activeUsers)
	a.metrics.UniqueUsersToday = len(a.uniqueUsers)
	a.usersMu.RUnlock()
}

// GetMetrics restituisce le metriche correnti
func (a *Aggregator) GetMetrics() LiveMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Deep copy
	metrics := a.metrics
	metrics.ProviderMetrics = make(map[uuid.UUID]*ProviderLiveMetrics)
	for id, pm := range a.metrics.ProviderMetrics {
		pmCopy := *pm
		metrics.ProviderMetrics[id] = &pmCopy
	}

	return metrics
}

// GetProviderMetrics restituisce le metriche per un provider
func (a *Aggregator) GetProviderMetrics(providerID uuid.UUID) *ProviderLiveMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if metrics, exists := a.metrics.ProviderMetrics[providerID]; exists {
		copy := *metrics
		return &copy
	}

	return nil
}

// cleanupLoop pulisce i dati obsoleti
func (a *Aggregator) cleanupLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.cleanupTicker.C:
			a.cleanupStaleUsers()
			a.cleanupOldWindows()

		case <-a.stopCh:
			return
		}
	}
}

// cleanupStaleUsers rimuove gli utenti inattivi
func (a *Aggregator) cleanupStaleUsers() {
	a.usersMu.Lock()
	defer a.usersMu.Unlock()

	now := time.Now()
	for userID, lastActivity := range a.activeUsers {
		if now.Sub(lastActivity) > a.userTimeout {
			delete(a.activeUsers, userID)
		}
	}
}

// cleanupOldWindows rimuove le finestre troppo vecchie
func (a *Aggregator) cleanupOldWindows() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(a.windowCount) * a.windowSize)

	newWindows := make([]*RequestWindow, 0, len(a.windows))
	for _, window := range a.windows {
		if window.Timestamp.After(cutoff) {
			newWindows = append(newWindows, window)
		}
	}

	a.windows = newWindows
}

// Reset resetta tutte le metriche
func (a *Aggregator) Reset() {
	a.mu.Lock()
	a.windowMu.Lock()
	a.usersMu.Lock()
	defer a.mu.Unlock()
	defer a.windowMu.Unlock()
	defer a.usersMu.Unlock()

	a.metrics = LiveMetrics{
		ProviderMetrics: make(map[uuid.UUID]*ProviderLiveMetrics),
		WindowStart:     time.Now(),
		LastUpdated:     time.Now(),
	}

	a.windows = make([]*RequestWindow, 0, a.windowCount)
	a.activeUsers = make(map[uuid.UUID]time.Time)
	// Non resettare uniqueUsers perché conta gli utenti unici del giorno

	log.Info().Msg("Aggregator metrics reset")
}

// ResetDaily resetta le metriche giornaliere
func (a *Aggregator) ResetDaily() {
	a.usersMu.Lock()
	defer a.usersMu.Unlock()

	a.uniqueUsers = make(map[uuid.UUID]bool)
	log.Info().Msg("Daily metrics reset")
}

// RequestRecord rappresenta un record di richiesta da aggregare
type RequestRecord struct {
	ProviderID uuid.UUID
	UserID     uuid.UUID
	LatencyMs  int
	Tokens     int
	Cost       float64
	Success    bool
	Timestamp  time.Time
}
