package experiments

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExperimentStatus definisce lo stato di un esperimento
type ExperimentStatus string

const (
	ExperimentStatusDraft      ExperimentStatus = "draft"
	ExperimentStatusRunning    ExperimentStatus = "running"
	ExperimentStatusPaused     ExperimentStatus = "paused"
	ExperimentStatusCompleted  ExperimentStatus = "completed"
	ExperimentStatusRolledBack ExperimentStatus = "rolled_back"
)

// ExperimentType definisce il tipo di esperimento
type ExperimentType string

const (
	ExperimentTypeProvider      ExperimentType = "provider"       // Test diversi provider
	ExperimentTypeRouting       ExperimentType = "routing"        // Test strategie di routing
	ExperimentTypeCombination   ExperimentType = "combination"    // Test combinazioni di provider
	ExperimentTypeLoadBalancing ExperimentType = "load_balancing" // Test algoritmi di load balancing
)

// Experiment rappresenta un esperimento A/B
type Experiment struct {
	ID          uuid.UUID        `json:"id" gorm:"type:uuid;primary_key"`
	Name        string           `json:"name" gorm:"not null"`
	Description string           `json:"description"`
	Type        ExperimentType   `json:"type" gorm:"not null"`
	Status      ExperimentStatus `json:"status" gorm:"not null;default:'draft'"`

	// Configurazione
	Variants        []Variant         `json:"variants" gorm:"serializer:json"`
	TrafficSplit    map[string]int    `json:"traffic_split" gorm:"serializer:json"` // variant_id -> percentage
	TargetMetric    string            `json:"target_metric"`                         // success_rate, latency, cost
	MinSampleSize   int               `json:"min_sample_size"`                       // Minimo campione per significatività
	Filters         ExperimentFilters `json:"filters" gorm:"serializer:json"`

	// Risultati
	Results ExperimentResults `json:"results" gorm:"serializer:json"`

	// Controllo
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	WinnerID    string     `json:"winner_id"` // ID della variante vincente

	// Auto-rollout
	AutoRollout        bool    `json:"auto_rollout"`         // Auto-promuove il vincitore
	ConfidenceRequired float64 `json:"confidence_required"`  // Confidence level richiesto (es: 0.95)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Variant rappresenta una variante dell'esperimento
type Variant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"` // Configurazione specifica della variante
	IsControl   bool                   `json:"is_control"`
}

// ExperimentFilters definisce i filtri per l'esperimento
type ExperimentFilters struct {
	ModelIDs     []uuid.UUID `json:"model_ids,omitempty"`      // Solo per questi modelli
	UserIDs      []uuid.UUID `json:"user_ids,omitempty"`       // Solo per questi utenti
	UserSegments []string    `json:"user_segments,omitempty"`  // Solo per questi segmenti (es: "premium", "free")
	MinTokens    int         `json:"min_tokens,omitempty"`     // Solo richieste con almeno N token
	MaxTokens    int         `json:"max_tokens,omitempty"`     // Solo richieste con al massimo N token
	TimeWindows  []TimeWindow `json:"time_windows,omitempty"`  // Solo in certe finestre temporali
}

// TimeWindow definisce una finestra temporale
type TimeWindow struct {
	StartHour int `json:"start_hour"` // 0-23
	EndHour   int `json:"end_hour"`   // 0-23
	Days      []int `json:"days"`     // 0=Sunday, 6=Saturday
}

// ExperimentResults contiene i risultati aggregati
type ExperimentResults struct {
	TotalRequests int64                           `json:"total_requests"`
	VariantStats  map[string]*VariantStatistics   `json:"variant_stats"`
	Analysis      *StatisticalAnalysis            `json:"analysis,omitempty"`
	LastUpdated   time.Time                       `json:"last_updated"`
}

// VariantStatistics contiene le statistiche di una variante
type VariantStatistics struct {
	VariantID     string  `json:"variant_id"`
	Requests      int64   `json:"requests"`
	Successes     int64   `json:"successes"`
	Failures      int64   `json:"failures"`
	SuccessRate   float64 `json:"success_rate"`

	// Latency
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	LatencySamples []int  `json:"latency_samples"` // Per calcoli statistici

	// Cost
	AvgCost   float64 `json:"avg_cost"`
	TotalCost float64 `json:"total_cost"`

	// Tokens
	AvgTokens   float64 `json:"avg_tokens"`
	TotalTokens int64   `json:"total_tokens"`

	// User satisfaction (se disponibile)
	SatisfactionScore float64 `json:"satisfaction_score,omitempty"`
	SatisfactionCount int64   `json:"satisfaction_count,omitempty"`
}

// StatisticalAnalysis contiene i risultati dell'analisi statistica
type StatisticalAnalysis struct {
	TestType         string                    `json:"test_type"` // chi_square, t_test, mann_whitney
	PValue           float64                   `json:"p_value"`
	IsSignificant    bool                      `json:"is_significant"`
	ConfidenceLevel  float64                   `json:"confidence_level"`
	EffectSize       float64                   `json:"effect_size"`
	RecommendedWinner string                   `json:"recommended_winner,omitempty"`
	Comparisons      []VariantComparison       `json:"comparisons"`
	Timestamp        time.Time                 `json:"timestamp"`
}

// VariantComparison rappresenta il confronto tra due varianti
type VariantComparison struct {
	VariantA      string  `json:"variant_a"`
	VariantB      string  `json:"variant_b"`
	Metric        string  `json:"metric"`
	DiffPercent   float64 `json:"diff_percent"`
	PValue        float64 `json:"p_value"`
	IsSignificant bool    `json:"is_significant"`
	BetterVariant string  `json:"better_variant"`
}

// BeforeCreate hook
func (e *Experiment) BeforeCreate() error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Results.VariantStats == nil {
		e.Results.VariantStats = make(map[string]*VariantStatistics)
	}
	return nil
}

// TableName specifica il nome della tabella
func (Experiment) TableName() string {
	return "experiments"
}

// UserBucketing gestisce l'assegnazione utenti alle varianti
type UserBucketing struct {
	experiment *Experiment
}

// NewUserBucketing crea un nuovo bucketing manager
func NewUserBucketing(exp *Experiment) *UserBucketing {
	return &UserBucketing{
		experiment: exp,
	}
}

// AssignVariant assegna una variante a un utente usando consistent hashing
func (ub *UserBucketing) AssignVariant(userID uuid.UUID) *Variant {
	if len(ub.experiment.Variants) == 0 {
		return nil
	}

	// Genera hash consistente da user_id + experiment_id
	hash := ub.consistentHash(userID)

	// Calcola bucket (0-100)
	bucket := hash % 100

	// Assegna variante in base al traffic split
	cumulative := 0
	for _, variant := range ub.experiment.Variants {
		if split, ok := ub.experiment.TrafficSplit[variant.ID]; ok {
			cumulative += split
			if bucket < cumulative {
				return &variant
			}
		}
	}

	// Fallback alla prima variante
	return &ub.experiment.Variants[0]
}

// consistentHash genera un hash consistente
func (ub *UserBucketing) consistentHash(userID uuid.UUID) int {
	// Combina user_id e experiment_id per hash stabile
	data := append(userID[:], ub.experiment.ID[:]...)
	hash := sha256.Sum256(data)

	// Converti primi 8 byte in uint64
	value := binary.BigEndian.Uint64(hash[:8])
	return int(value % 100)
}

// ShouldIncludeRequest verifica se una richiesta rientra nei filtri
func (ub *UserBucketing) ShouldIncludeRequest(req *ExperimentRequest) bool {
	filters := ub.experiment.Filters

	// Filtra per model
	if len(filters.ModelIDs) > 0 {
		found := false
		for _, modelID := range filters.ModelIDs {
			if modelID == req.ModelID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filtra per user
	if len(filters.UserIDs) > 0 {
		found := false
		for _, userID := range filters.UserIDs {
			if userID == req.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filtra per user segment
	if len(filters.UserSegments) > 0 && req.UserSegment != "" {
		found := false
		for _, segment := range filters.UserSegments {
			if segment == req.UserSegment {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filtra per tokens
	totalTokens := req.InputTokens + req.OutputTokens
	if filters.MinTokens > 0 && totalTokens < filters.MinTokens {
		return false
	}
	if filters.MaxTokens > 0 && totalTokens > filters.MaxTokens {
		return false
	}

	// Filtra per time window
	if len(filters.TimeWindows) > 0 {
		now := time.Now()
		inWindow := false
		for _, window := range filters.TimeWindows {
			if ub.isInTimeWindow(now, window) {
				inWindow = true
				break
			}
		}
		if !inWindow {
			return false
		}
	}

	return true
}

// isInTimeWindow verifica se un timestamp è in una finestra temporale
func (ub *UserBucketing) isInTimeWindow(t time.Time, window TimeWindow) bool {
	// Verifica giorno della settimana
	if len(window.Days) > 0 {
		dayMatch := false
		weekday := int(t.Weekday())
		for _, day := range window.Days {
			if day == weekday {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Verifica ora
	hour := t.Hour()
	if window.EndHour > window.StartHour {
		// Finestra normale (es: 9-17)
		return hour >= window.StartHour && hour < window.EndHour
	} else if window.EndHour < window.StartHour {
		// Finestra a cavallo di mezzanotte (es: 22-6)
		return hour >= window.StartHour || hour < window.EndHour
	}

	return true
}

// ExperimentRequest rappresenta una richiesta nell'esperimento
type ExperimentRequest struct {
	UserID       uuid.UUID
	ModelID      uuid.UUID
	UserSegment  string
	InputTokens  int
	OutputTokens int
	Timestamp    time.Time
}

// FeatureFlag rappresenta un feature flag semplice
type FeatureFlag struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:false"`

	// Rollout graduale
	RolloutPercentage int                    `json:"rollout_percentage"` // 0-100
	UserWhitelist     []uuid.UUID            `json:"user_whitelist" gorm:"serializer:json"`
	UserBlacklist     []uuid.UUID            `json:"user_blacklist" gorm:"serializer:json"`
	Config            map[string]interface{} `json:"config" gorm:"serializer:json"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook for FeatureFlag
func (f *FeatureFlag) BeforeCreate() error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// TableName specifica il nome della tabella
func (FeatureFlag) TableName() string {
	return "feature_flags"
}

// IsEnabledForUser verifica se il flag è abilitato per un utente
func (f *FeatureFlag) IsEnabledForUser(userID uuid.UUID) bool {
	if !f.Enabled {
		return false
	}

	// Verifica blacklist
	for _, blockedID := range f.UserBlacklist {
		if blockedID == userID {
			return false
		}
	}

	// Verifica whitelist
	if len(f.UserWhitelist) > 0 {
		for _, allowedID := range f.UserWhitelist {
			if allowedID == userID {
				return true
			}
		}
		// Se c'è una whitelist ma l'utente non è in essa, disabilita
		return false
	}

	// Verifica rollout percentage
	if f.RolloutPercentage < 100 {
		hash := sha256.Sum256(append(userID[:], f.ID[:]...))
		value := binary.BigEndian.Uint64(hash[:8])
		bucket := int(value % 100)
		return bucket < f.RolloutPercentage
	}

	return true
}

// GetConfigValue ritorna un valore di configurazione
func (f *FeatureFlag) GetConfigValue(key string) (interface{}, bool) {
	if f.Config == nil {
		return nil, false
	}
	val, ok := f.Config[key]
	return val, ok
}

// MarshalJSON personalizza la serializzazione JSON
func (e *Experiment) MarshalJSON() ([]byte, error) {
	type Alias Experiment
	return json.Marshal(&struct {
		*Alias
		TrafficSplitReadable map[string]string `json:"traffic_split_readable"`
	}{
		Alias: (*Alias)(e),
		TrafficSplitReadable: func() map[string]string {
			m := make(map[string]string)
			for k, v := range e.TrafficSplit {
				m[k] = string(rune(v)) + "%"
			}
			return m
		}(),
	})
}
