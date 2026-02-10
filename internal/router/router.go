package router

import (
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
)

// Router gestisce il routing intelligente delle richieste
type Router struct {
	config   *config.Config
	db       *database.DB
	strategy RoutingStrategy
}

// RoutingStrategy definisce la strategia di routing
type RoutingStrategy interface {
	SelectProvider(req *Request) (*ProviderSelection, error)
}

// Request rappresenta una richiesta al gateway
type Request struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// Message rappresenta un messaggio
type Message struct {
	Role    string
	Content string
}

// ProviderSelection rappresenta il provider selezionato
type ProviderSelection struct {
	ProviderID   string
	ModelID      string
	EstimatedCost float64
	Reason       string
}

// New crea un nuovo router
func New(cfg *config.Config, db *database.DB) (*Router, error) {
	r := &Router{
		config: cfg,
		db:     db,
	}

	// Initialize routing strategy
	switch cfg.Routing.Strategy {
	case "cost_optimized":
		r.strategy = &CostOptimizedStrategy{db: db}
	case "latency_first":
		r.strategy = &LatencyFirstStrategy{db: db}
	case "quality_first":
		r.strategy = &QualityFirstStrategy{db: db}
	default:
		r.strategy = &CostOptimizedStrategy{db: db}
	}

	return r, nil
}

// Cost-optimized strategy
type CostOptimizedStrategy struct {
	db *database.DB
}

func (s *CostOptimizedStrategy) SelectProvider(req *Request) (*ProviderSelection, error) {
	// TODO: Implement cost-optimized provider selection
	return &ProviderSelection{
		Reason: "cost_optimized",
	}, nil
}

// Latency-first strategy
type LatencyFirstStrategy struct {
	db *database.DB
}

func (s *LatencyFirstStrategy) SelectProvider(req *Request) (*ProviderSelection, error) {
	// TODO: Implement latency-first provider selection
	return &ProviderSelection{
		Reason: "latency_first",
	}, nil
}

// Quality-first strategy
type QualityFirstStrategy struct {
	db *database.DB
}

func (s *QualityFirstStrategy) SelectProvider(req *Request) (*ProviderSelection, error) {
	// TODO: Implement quality-first provider selection
	return &ProviderSelection{
		Reason: "quality_first",
	}, nil
}
