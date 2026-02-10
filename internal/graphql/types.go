package graphql

import (
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/google/uuid"
)

// ================================================================================
// Connection Types
// ================================================================================

type ProviderConnection struct {
	Edges      []*ProviderEdge
	PageInfo   *PageInfo
	TotalCount int
}

type ProviderEdge struct {
	Node   *models.Provider
	Cursor string
}

type ModelConnection struct {
	Edges      []*ModelEdge
	PageInfo   *PageInfo
	TotalCount int
}

type ModelEdge struct {
	Node   *models.Model
	Cursor string
}

type PageInfo struct {
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     *string
	EndCursor       *string
}

// ================================================================================
// Filter Input Types
// ================================================================================

type ProviderFilterInput struct {
	Type             *string
	Status           *string
	Tier             *int
	SupportStreaming *bool
	MinHealthScore   *float64
	Search           *string
}

type ProviderSortInput struct {
	Field     ProviderSortField
	Direction SortDirection
}

type ProviderSortField string

const (
	ProviderSortFieldName        ProviderSortField = "name"
	ProviderSortFieldHealthScore ProviderSortField = "health_score"
	ProviderSortFieldAvgLatency  ProviderSortField = "avg_latency_ms"
	ProviderSortFieldCreatedAt   ProviderSortField = "created_at"
	ProviderSortFieldTier        ProviderSortField = "tier"
)

type ModelFilterInput struct {
	ProviderID        *uuid.UUID
	Modality          *string
	IsFree            *bool
	MinQualityScore   *float64
	MinContextLength  *int
	Search            *string
}

type ModelSortInput struct {
	Field     ModelSortField
	Direction SortDirection
}

type ModelSortField string

const (
	ModelSortFieldName          ModelSortField = "name"
	ModelSortFieldQualityScore  ModelSortField = "quality_score"
	ModelSortFieldSpeedScore    ModelSortField = "speed_score"
	ModelSortFieldContextLength ModelSortField = "context_length"
	ModelSortFieldCreatedAt     ModelSortField = "created_at"
)

type SortDirection string

const (
	SortDirectionAsc  SortDirection = "ASC"
	SortDirectionDesc SortDirection = "DESC"
)

type TimeRangeInput struct {
	Start    time.Time
	End      time.Time
	Interval *string
}

// ================================================================================
// Create/Update Input Types
// ================================================================================

type CreateProviderInput struct {
	Name              string
	Type              string
	BaseURL           string
	AuthType          string
	Tier              int
	Source            string
	SupportsStreaming bool
	SupportsTools     bool
	SupportsJSON      bool
}

type UpdateProviderInput struct {
	Name              *string
	Type              *string
	Status            *string
	BaseURL           *string
	AuthType          *string
	Tier              *int
	SupportsStreaming *bool
	SupportsTools     *bool
	SupportsJSON      *bool
}

type CreateModelInput struct {
	ProviderID       uuid.UUID
	Name             string
	DisplayName      *string
	Modality         string
	ContextLength    int
	MaxOutputTokens  int
	InputPricePer1k  float64
	OutputPricePer1k float64
	Capabilities     map[string]interface{}
	Description      *string
	Tags             []string
}

type UpdateModelInput struct {
	Name             *string
	DisplayName      *string
	Modality         *string
	ContextLength    *int
	MaxOutputTokens  *int
	InputPricePer1k  *float64
	OutputPricePer1k *float64
	Capabilities     map[string]interface{}
	QualityScore     *float64
	SpeedScore       *float64
	Description      *string
	Tags             []string
}

type ChatRequestInput struct {
	Model       string
	Messages    []ChatMessageInput
	Temperature *float64
	MaxTokens   *int
	Stream      bool
}

type ChatMessageInput struct {
	Role    string
	Content string
}

// ================================================================================
// Stats Types
// ================================================================================

type GlobalStats struct {
	TotalProviders     int
	ActiveProviders    int
	TotalModels        int
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	TotalTokens        int
	AvgLatencyMs       float64
	SuccessRate        float64
	CostSaved          float64
	TimeRange          *TimeRangeInput
	Timestamp          time.Time
}

type RealtimeMetrics struct {
	RequestsPerSecond float64
	ActiveConnections int
	QueuedRequests    int
	ProviderMetrics   []*ProviderRealtimeMetrics
	Timestamp         time.Time
}

type ProviderRealtimeMetrics struct {
	ProviderID          uuid.UUID
	ProviderName        string
	Status              string
	CurrentLatencyMs    int
	RequestsInProgress  int
	HealthScore         float64
}

// ================================================================================
// Health Types
// ================================================================================

type HealthStatus struct {
	Status    string
	Version   string
	Uptime    int
	Database  *ComponentHealth
	Cache     *ComponentHealth
	Providers *ComponentHealth
	Timestamp time.Time
}

type ComponentHealth struct {
	Status    string
	LatencyMs *int
	Message   *string
}

type ProviderHealthStatus struct {
	ProviderID          uuid.UUID
	ProviderName        string
	IsHealthy           bool
	HealthScore         float64
	LatencyMs           int
	LastCheck           time.Time
	LastSuccess         *time.Time
	ConsecutiveFailures int
	Details             *string
}

// ================================================================================
// Response Types
// ================================================================================

type ChatResponse struct {
	ID        string
	Model     string
	Provider  string
	Choices   []*ChatChoice
	Usage     *TokenUsage
	LatencyMs int
}

type ChatChoice struct {
	Index        int
	Message      *ChatMessage
	FinishReason *string
}

type ChatMessage struct {
	Role    string
	Content string
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ================================================================================
// Event Types (Subscriptions)
// ================================================================================

type ProviderUpdate struct {
	Type      string
	Provider  *models.Provider
	Timestamp time.Time
}

type RequestEvent struct {
	ID           uuid.UUID
	ProviderID   uuid.UUID
	ProviderName string
	ModelName    string
	StatusCode   int
	LatencyMs    int
	Success      bool
	Timestamp    time.Time
}
