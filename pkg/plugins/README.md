# GoLeapAI Plugin System

Sistema di plugin estensibile per GoLeapAI che permette di aggiungere funzionalità custom senza modificare il core.

## Caratteristiche

- **Caricamento Dinamico**: Carica plugin `.so` a runtime
- **Hot Reload**: Ricarica automatica dei plugin modificati
- **Dependency Injection**: Sistema DI per condividere risorse
- **Event Hooks**: Hook per eventi (request, response, error, etc.)
- **Type Safety**: Interfacce strongly-typed per ogni tipo di plugin
- **Plugin Registry**: Gestione centralizzata del ciclo di vita
- **Versioning**: Supporto per versioni e metadata

## Tipi di Plugin

### 1. ProviderPlugin
Plugin per provider LLM custom.

```go
type ProviderPlugin interface {
    Plugin
    GetProvider() (providers.Provider, error)
    HealthCheck(ctx context.Context) error
    GetModels(ctx context.Context) ([]providers.ModelInfo, error)
    EstimateCost(req *providers.ChatRequest) (float64, error)
}
```

### 2. MiddlewarePlugin
Plugin per middleware HTTP custom.

```go
type MiddlewarePlugin interface {
    Plugin
    Handler() fiber.Handler
    Priority() int
    ApplyRoutes() []string
}
```

### 3. RouterPlugin
Plugin per logica di routing custom.

```go
type RouterPlugin interface {
    Plugin
    RouteRequest(ctx context.Context, req *RouteRequest) (bool, error)
    SelectProvider(ctx context.Context, req *RouteRequest, available []string) (string, error)
    TransformRequest(ctx context.Context, req interface{}) (interface{}, error)
    TransformResponse(ctx context.Context, resp interface{}) (interface{}, error)
}
```

### 4. CachePlugin
Plugin per cache backend custom.

```go
type CachePlugin interface {
    Plugin
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Stats() CacheStats
    SupportsDistributed() bool
}
```

### 5. AuthPlugin
Plugin per autenticazione custom.

```go
type AuthPlugin interface {
    Plugin
    Authenticate(ctx context.Context, token string) (*AuthContext, error)
    Authorize(ctx context.Context, auth *AuthContext, action string, resource string) (bool, error)
    GetPermissions(ctx context.Context, userID string) ([]Permission, error)
}
```

### 6. MonitoringPlugin
Plugin per monitoring e metriche.

```go
type MonitoringPlugin interface {
    Plugin
    RecordMetric(name string, value float64, tags map[string]string)
    RecordEvent(name string, data map[string]interface{})
    StartTrace(ctx context.Context, name string) (context.Context, func())
    Export() ([]byte, error)
}
```

## Struttura Plugin

Ogni plugin deve implementare l'interfaccia `Plugin`:

```go
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Init(ctx context.Context, deps *Dependencies) error
    Shutdown(ctx context.Context) error
    Type() PluginType
    Metadata() map[string]interface{}
}
```

## Creare un Plugin

### 1. Struttura del Progetto

```
plugins/
└── myplugin/
    ├── myplugin.go
    ├── Makefile
    └── go.mod (opzionale)
```

### 2. Implementazione

```go
package main

import (
    "context"
    "github.com/biodoia/goleapifree/pkg/plugins"
)

type MyPlugin struct {
    name    string
    version string
    logger  plugins.Logger
}

// Factory function OBBLIGATORIA
func NewPlugin() (plugins.Plugin, error) {
    return &MyPlugin{
        name:    "myplugin",
        version: "1.0.0",
    }, nil
}

func (p *MyPlugin) Name() string { return p.name }
func (p *MyPlugin) Version() string { return p.version }
func (p *MyPlugin) Description() string { return "My custom plugin" }
func (p *MyPlugin) Type() plugins.PluginType { return plugins.PluginTypeProvider }

func (p *MyPlugin) Init(ctx context.Context, deps *plugins.Dependencies) error {
    p.logger = deps.Logger
    p.logger.Info("Plugin initialized", nil)
    return nil
}

func (p *MyPlugin) Shutdown(ctx context.Context) error {
    p.logger.Info("Plugin shutdown", nil)
    return nil
}

func (p *MyPlugin) Metadata() map[string]interface{} {
    return map[string]interface{}{
        "author": "Your Name",
        "license": "MIT",
    }
}
```

### 3. Compilazione

```bash
# Compila come plugin Go
go build -buildmode=plugin -o myplugin.so myplugin.go

# O usa il Makefile
make build
```

### 4. Installazione

```bash
# Copia nella directory plugins
cp myplugin.so /path/to/goleapifree/plugins/

# O usa il Makefile
make install
```

## Uso del Plugin System

### Inizializzazione

```go
package main

import (
    "context"
    "github.com/biodoia/goleapifree/pkg/plugins"
)

func main() {
    // Crea registry
    registry := plugins.NewRegistry(plugins.DefaultRegistryConfig())

    // Crea loader
    loader := plugins.NewLoader(registry, plugins.DefaultLoaderConfig())

    // Carica tutti i plugin
    ctx := context.Background()
    if err := loader.LoadAll(ctx); err != nil {
        panic(err)
    }

    // Avvia hot reload (opzionale)
    loader.StartWatching(ctx)

    // Usa i plugin...
}
```

### Recuperare un Plugin

```go
// Per nome
plugin, err := registry.Get("myplugin")
if err != nil {
    // handle error
}

// Per tipo
providers := registry.GetByType(plugins.PluginTypeProvider)
for _, p := range providers {
    // use provider plugin
}
```

### Hook System

```go
// Registra hook per richieste
registry.GetHooks().OnRequest(func(ctx context.Context, req *plugins.HookRequest) error {
    log.Printf("Request: %s -> %s", req.Model, req.Provider)
    return nil
})

// Hook per risposte
registry.GetHooks().OnResponse(func(ctx context.Context, resp *plugins.HookResponse) error {
    log.Printf("Response: %d tokens in %s", resp.TokensUsed, resp.Duration)
    return nil
})

// Hook per errori
registry.GetHooks().OnError(func(ctx context.Context, err *plugins.HookError) error {
    log.Printf("Error: %s (retryable: %v)", err.Error, err.Retryable)
    return nil
})

// Hook per cambio provider
registry.GetHooks().OnProviderSwitch(func(ctx context.Context, event *plugins.ProviderSwitchEvent) error {
    log.Printf("Switched from %s to %s: %s", event.FromProvider, event.ToProvider, event.Reason)
    return nil
})
```

## Configurazione Plugin

```yaml
plugins:
  enabled: true
  directory: "./plugins"
  auto_load: true
  hot_reload: false
  max_plugins: 100

  configs:
    example-provider:
      enabled: true
      priority: 10
      config:
        api_endpoint: "https://api.example.com/v1"
        api_key: "${EXAMPLE_API_KEY}"
        timeout: 30s
        max_retries: 3
```

## Event Hooks

Gli hook vengono eseguiti in modo asincrono e non bloccante:

- `OnRequest`: Prima di processare una richiesta
- `OnResponse`: Dopo aver ricevuto una risposta
- `OnError`: Quando si verifica un errore
- `OnProviderSwitch`: Quando si cambia provider
- `OnPluginLoad`: Quando un plugin viene caricato
- `OnCacheEvent`: Per eventi della cache
- `OnMetric`: Per registrare metriche

## Best Practices

1. **Gestione Errori**: Gestisci sempre gli errori nei plugin
2. **Logging**: Usa il logger iniettato per consistenza
3. **Context**: Rispetta i context per cancellazione
4. **Cleanup**: Implementa Shutdown per cleanup risorse
5. **Versioning**: Usa semantic versioning per i plugin
6. **Testing**: Testa i plugin prima del deploy
7. **Dependencies**: Minimizza le dipendenze esterne

## Esempi

Vedi `plugins/example/` per un esempio completo di:
- Provider plugin custom
- Hook handlers
- Dependency injection
- Configuration management

## Troubleshooting

### Plugin non si carica

```bash
# Verifica che il simbolo NewPlugin sia esportato
nm -D myplugin.so | grep NewPlugin

# Verifica le dipendenze
ldd myplugin.so
```

### Version mismatch

Assicurati che il plugin sia compilato con la stessa versione di Go del programma principale:

```bash
go version  # Verifica versione Go
go build -buildmode=plugin ...
```

### Symbol not found

Il plugin deve essere compilato con:
- Stessa versione di Go
- Stesso GOPATH
- Stesse dipendenze

## API Reference

Vedi i file:
- `interface.go` - Interfacce plugin
- `registry.go` - Gestione registry
- `loader.go` - Caricamento plugin
- `hooks.go` - Sistema di hook

## Contribuire

Per aggiungere nuovi tipi di plugin:

1. Definisci l'interfaccia in `interface.go`
2. Aggiungi il tipo in `PluginType`
3. Documenta l'interfaccia
4. Crea un esempio in `plugins/examples/`

## License

MIT License - See LICENSE file for details
