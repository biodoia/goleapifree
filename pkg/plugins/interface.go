package plugins

import (
	"context"
	"time"

	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/gofiber/fiber/v3"
)

// Plugin è l'interfaccia base per tutti i plugin
type Plugin interface {
	// Name restituisce il nome univoco del plugin
	Name() string

	// Version restituisce la versione del plugin
	Version() string

	// Description restituisce una descrizione del plugin
	Description() string

	// Init inizializza il plugin con le dipendenze
	Init(ctx context.Context, deps *Dependencies) error

	// Shutdown esegue il cleanup del plugin
	Shutdown(ctx context.Context) error

	// Type restituisce il tipo di plugin
	Type() PluginType

	// Metadata restituisce metadata aggiuntivi del plugin
	Metadata() map[string]interface{}
}

// PluginType rappresenta il tipo di plugin
type PluginType string

const (
	PluginTypeProvider   PluginType = "provider"
	PluginTypeMiddleware PluginType = "middleware"
	PluginTypeRouter     PluginType = "router"
	PluginTypeCache      PluginType = "cache"
	PluginTypeAuth       PluginType = "auth"
	PluginTypeMonitoring PluginType = "monitoring"
	PluginTypeTransform  PluginType = "transform"
)

// Dependencies contiene le dipendenze iniettate nei plugin
type Dependencies struct {
	Config      map[string]interface{}
	Logger      Logger
	Hooks       *HookRegistry
	PluginDir   string
	SharedState map[string]interface{}
}

// Logger interfaccia per logging nei plugin
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
}

// ProviderPlugin interfaccia per plugin che forniscono provider LLM custom
type ProviderPlugin interface {
	Plugin

	// GetProvider restituisce l'istanza del provider
	GetProvider() (providers.Provider, error)

	// HealthCheck verifica lo stato del provider
	HealthCheck(ctx context.Context) error

	// GetModels restituisce i modelli supportati
	GetModels(ctx context.Context) ([]providers.ModelInfo, error)

	// EstimateCost stima il costo di una richiesta
	EstimateCost(req *providers.ChatRequest) (float64, error)
}

// MiddlewarePlugin interfaccia per plugin middleware HTTP
type MiddlewarePlugin interface {
	Plugin

	// Handler restituisce il middleware Fiber
	Handler() fiber.Handler

	// Priority restituisce la priorità di esecuzione (0-100, maggiore = prima)
	Priority() int

	// ApplyRoutes specifica su quali route applicare il middleware
	// Se nil, si applica a tutte le route
	ApplyRoutes() []string
}

// RouterPlugin interfaccia per plugin che aggiungono logica di routing custom
type RouterPlugin interface {
	Plugin

	// RouteRequest decide se il plugin può gestire la richiesta
	RouteRequest(ctx context.Context, req *RouteRequest) (bool, error)

	// SelectProvider seleziona il provider migliore per la richiesta
	SelectProvider(ctx context.Context, req *RouteRequest, available []string) (string, error)

	// TransformRequest trasforma la richiesta prima dell'invio
	TransformRequest(ctx context.Context, req interface{}) (interface{}, error)

	// TransformResponse trasforma la risposta prima di restituirla
	TransformResponse(ctx context.Context, resp interface{}) (interface{}, error)
}

// RouteRequest contiene informazioni per il routing
type RouteRequest struct {
	Model       string
	Provider    string
	Messages    []providers.Message
	Stream      bool
	Metadata    map[string]interface{}
	UserID      string
	Priority    int
	Constraints *RouteConstraints
}

// RouteConstraints vincoli di routing
type RouteConstraints struct {
	MaxLatency      time.Duration
	MaxCost         float64
	RequiredFeatures []providers.Feature
	PreferredRegion string
	Fallback        bool
}

// CachePlugin interfaccia per plugin cache backend custom
type CachePlugin interface {
	Plugin

	// Get recupera un valore dalla cache
	Get(ctx context.Context, key string) ([]byte, error)

	// Set salva un valore nella cache
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete rimuove un valore dalla cache
	Delete(ctx context.Context, key string) error

	// Clear svuota la cache
	Clear(ctx context.Context) error

	// Stats restituisce statistiche della cache
	Stats() CacheStats

	// SupportsDistributed indica se la cache è distribuita
	SupportsDistributed() bool
}

// CacheStats statistiche cache
type CacheStats struct {
	Hits         int64
	Misses       int64
	Sets         int64
	Deletes      int64
	Size         int64
	Entries      int64
	EvictionRate float64
	Latency      time.Duration
}

// AuthPlugin interfaccia per plugin di autenticazione custom
type AuthPlugin interface {
	Plugin

	// Authenticate autentica una richiesta
	Authenticate(ctx context.Context, token string) (*AuthContext, error)

	// Authorize autorizza un'azione
	Authorize(ctx context.Context, auth *AuthContext, action string, resource string) (bool, error)

	// GetPermissions restituisce i permessi di un utente
	GetPermissions(ctx context.Context, userID string) ([]Permission, error)
}

// AuthContext contesto di autenticazione
type AuthContext struct {
	UserID      string
	AccountID   string
	Roles       []string
	Permissions []string
	Metadata    map[string]interface{}
	ExpiresAt   time.Time
}

// Permission rappresenta un permesso
type Permission struct {
	Action   string
	Resource string
	Granted  bool
}

// MonitoringPlugin interfaccia per plugin di monitoring
type MonitoringPlugin interface {
	Plugin

	// RecordMetric registra una metrica
	RecordMetric(name string, value float64, tags map[string]string)

	// RecordEvent registra un evento
	RecordEvent(name string, data map[string]interface{})

	// StartTrace avvia una trace
	StartTrace(ctx context.Context, name string) (context.Context, func())

	// Export esporta metriche (es. Prometheus, StatsD)
	Export() ([]byte, error)
}

// TransformPlugin interfaccia per plugin di trasformazione dati
type TransformPlugin interface {
	Plugin

	// TransformRequest trasforma una richiesta
	TransformRequest(ctx context.Context, req interface{}) (interface{}, error)

	// TransformResponse trasforma una risposta
	TransformResponse(ctx context.Context, resp interface{}) (interface{}, error)

	// SupportsFormat indica se supporta un formato
	SupportsFormat(format string) bool
}

// PluginFactory funzione factory per creare plugin
type PluginFactory func() (Plugin, error)

// PluginConfig configurazione di un plugin
type PluginConfig struct {
	Name        string
	Version     string
	Enabled     bool
	Priority    int
	Config      map[string]interface{}
	Dependencies []string // Altri plugin richiesti
	LoadAfter   []string // Plugin da caricare dopo
}

// PluginInfo informazioni su un plugin caricato
type PluginInfo struct {
	Plugin      Plugin
	Config      *PluginConfig
	LoadedAt    time.Time
	LastUsed    time.Time
	Metrics     PluginMetrics
	State       PluginState
	Error       error
}

// PluginState stato del plugin
type PluginState string

const (
	PluginStateLoaded   PluginState = "loaded"
	PluginStateActive   PluginState = "active"
	PluginStateInactive PluginState = "inactive"
	PluginStateError    PluginState = "error"
	PluginStateUnloaded PluginState = "unloaded"
)

// PluginMetrics metriche di utilizzo plugin
type PluginMetrics struct {
	Calls       int64
	Errors      int64
	TotalTime   time.Duration
	AverageTime time.Duration
	LastError   error
	LastCall    time.Time
}

// ValidatePlugin valida che un plugin implementi correttamente l'interfaccia
func ValidatePlugin(p Plugin) error {
	if p.Name() == "" {
		return ErrInvalidPluginName
	}
	if p.Version() == "" {
		return ErrInvalidPluginVersion
	}
	if p.Type() == "" {
		return ErrInvalidPluginType
	}
	return nil
}
