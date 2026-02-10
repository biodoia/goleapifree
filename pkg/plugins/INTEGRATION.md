# Plugin System Integration Guide

Guida per integrare il sistema di plugin in GoLeapAI.

## Integrazione nel Gateway

### 1. Modifica `internal/gateway/gateway.go`

Aggiungi il plugin system al Gateway:

```go
package gateway

import (
    "github.com/biodoia/goleapifree/pkg/plugins"
    // ...
)

type Gateway struct {
    config         *config.Config
    db             *database.DB
    app            *fiber.App
    router         *router.Router
    health         *health.Monitor
    pluginRegistry *plugins.Registry  // Aggiungi
    pluginLoader   *plugins.Loader    // Aggiungi
}

func New(cfg *config.Config, db *database.DB) (*Gateway, error) {
    // ... existing code ...

    // Inizializza plugin system
    pluginRegistry := plugins.NewRegistry(&plugins.RegistryConfig{
        PluginDir:      cfg.Plugins.Directory,
        AutoLoad:       cfg.Plugins.AutoLoad,
        HotReload:      cfg.Plugins.HotReload,
        ReloadInterval: cfg.Plugins.ReloadInterval,
        MaxPlugins:     cfg.Plugins.MaxPlugins,
        Timeout:        30 * time.Second,
    })

    pluginLoader := plugins.NewLoader(pluginRegistry, &plugins.LoaderConfig{
        PluginDir:     cfg.Plugins.Directory,
        AutoLoad:      cfg.Plugins.AutoLoad,
        HotReload:     cfg.Plugins.HotReload,
        WatchInterval: cfg.Plugins.ReloadInterval,
        FilePattern:   "*.so",
        MaxConcurrent: 5,
    })

    gw := &Gateway{
        config:         cfg,
        db:             db,
        app:            app,
        router:         r,
        health:         healthMonitor,
        pluginRegistry: pluginRegistry,
        pluginLoader:   pluginLoader,
    }

    // Carica plugin
    ctx := context.Background()
    if err := pluginLoader.LoadAll(ctx); err != nil {
        log.Warn().Err(err).Msg("Failed to load some plugins")
    }

    // Avvia hot reload se abilitato
    if cfg.Plugins.HotReload {
        pluginLoader.StartWatching(ctx)
    }

    // Applica middleware plugin
    gw.applyMiddlewarePlugins()

    // Setup routes
    gw.setupRoutes()

    return gw, nil
}

// applyMiddlewarePlugins applica i middleware dai plugin
func (g *Gateway) applyMiddlewarePlugins() {
    middlewares := g.pluginRegistry.GetByType(plugins.PluginTypeMiddleware)

    // Ordina per priorità
    sort.Slice(middlewares, func(i, j int) bool {
        mi, _ := middlewares[i].(plugins.MiddlewarePlugin)
        mj, _ := middlewares[j].(plugins.MiddlewarePlugin)
        return mi.Priority() > mj.Priority()
    })

    for _, plugin := range middlewares {
        mw, ok := plugin.(plugins.MiddlewarePlugin)
        if !ok {
            continue
        }

        if !g.pluginRegistry.IsEnabled(plugin.Name()) {
            continue
        }

        routes := mw.ApplyRoutes()
        if len(routes) == 0 {
            // Applica globalmente
            g.app.Use(mw.Handler())
        } else {
            // Applica solo a route specifiche
            for _, route := range routes {
                g.app.Use(route, mw.Handler())
            }
        }

        log.Info().
            Str("plugin", plugin.Name()).
            Int("priority", mw.Priority()).
            Strs("routes", routes).
            Msg("Applied middleware plugin")
    }
}

func (g *Gateway) Shutdown(ctx context.Context) error {
    // Stop plugin hot reload
    if g.pluginLoader != nil {
        g.pluginLoader.StopWatching()
    }

    // Shutdown plugins
    if g.pluginRegistry != nil {
        if err := g.pluginRegistry.ShutdownAll(ctx); err != nil {
            log.Error().Err(err).Msg("Failed to shutdown plugins")
        }
    }

    // ... existing shutdown code ...
}
```

### 2. Modifica `pkg/config/config.go`

Aggiungi configurazione per plugin:

```go
type Config struct {
    // ... existing fields ...

    Plugins PluginsConfig `mapstructure:"plugins"`
}

type PluginsConfig struct {
    Enabled        bool              `mapstructure:"enabled"`
    Directory      string            `mapstructure:"directory"`
    AutoLoad       bool              `mapstructure:"auto_load"`
    HotReload      bool              `mapstructure:"hot_reload"`
    ReloadInterval time.Duration     `mapstructure:"reload_interval"`
    MaxPlugins     int               `mapstructure:"max_plugins"`

    // Configurazioni per singoli plugin
    Configs map[string]PluginConfig `mapstructure:"configs"`
}

type PluginConfig struct {
    Enabled      bool                   `mapstructure:"enabled"`
    Priority     int                    `mapstructure:"priority"`
    Config       map[string]interface{} `mapstructure:"config"`
    Dependencies []string               `mapstructure:"dependencies"`
}
```

### 3. Aggiorna `config.yaml`

```yaml
plugins:
  enabled: true
  directory: "./plugins"
  auto_load: true
  hot_reload: false
  reload_interval: 30s
  max_plugins: 100

  configs:
    example-provider:
      enabled: true
      priority: 10
      config:
        api_endpoint: "https://api.example.com/v1"
        api_key: "${EXAMPLE_API_KEY}"
        timeout: "30s"
        max_retries: 3

    rate-limiter:
      enabled: true
      priority: 80
      config:
        max_requests: 100
        window: "1m"
        identifier: "ip"
        routes:
          - "/v1/*"
          - "/api/*"
```

## Integrazione Provider Plugin nel Router

### Modifica `internal/router/router.go`

```go
package router

import (
    "github.com/biodoia/goleapifree/pkg/plugins"
    "github.com/biodoia/goleapifree/internal/providers"
)

type Router struct {
    // ... existing fields ...
    pluginRegistry *plugins.Registry
}

func New(cfg *config.Config, db *database.DB, pluginRegistry *plugins.Registry) (*Router, error) {
    r := &Router{
        // ... existing fields ...
        pluginRegistry: pluginRegistry,
    }

    // Registra provider da plugin
    r.registerPluginProviders()

    return r, nil
}

func (r *Router) registerPluginProviders() {
    providerPlugins := r.pluginRegistry.GetByType(plugins.PluginTypeProvider)

    for _, plugin := range providerPlugins {
        pp, ok := plugin.(plugins.ProviderPlugin)
        if !ok {
            continue
        }

        if !r.pluginRegistry.IsEnabled(plugin.Name()) {
            continue
        }

        provider, err := pp.GetProvider()
        if err != nil {
            log.Error().
                Err(err).
                Str("plugin", plugin.Name()).
                Msg("Failed to get provider from plugin")
            continue
        }

        // Registra il provider nel router
        r.providers[plugin.Name()] = provider

        log.Info().
            Str("plugin", plugin.Name()).
            Str("version", plugin.Version()).
            Msg("Registered provider from plugin")
    }
}

// Usa i plugin durante il routing
func (r *Router) Route(ctx context.Context, req *RouteRequest) (providers.Provider, error) {
    // Trigger hook di richiesta
    if r.pluginRegistry != nil {
        hookReq := &plugins.HookRequest{
            RequestID: req.ID,
            Provider:  req.PreferredProvider,
            Model:     req.Model,
            Request:   req.Request,
            StartTime: time.Now(),
            UserID:    req.UserID,
        }
        r.pluginRegistry.GetHooks().TriggerRequest(ctx, hookReq)
    }

    // ... existing routing logic ...

    // Usa router plugin se disponibile
    routerPlugins := r.pluginRegistry.GetByType(plugins.PluginTypeRouter)
    for _, plugin := range routerPlugins {
        rp, ok := plugin.(plugins.RouterPlugin)
        if !ok {
            continue
        }

        if !r.pluginRegistry.IsEnabled(plugin.Name()) {
            continue
        }

        // Chiedi al plugin se può gestire questa richiesta
        canHandle, err := rp.RouteRequest(ctx, &plugins.RouteRequest{
            Model:    req.Model,
            Provider: req.PreferredProvider,
            Messages: req.Request.Messages,
            Stream:   req.Request.Stream,
        })

        if err == nil && canHandle {
            // Plugin può gestire, usa la sua logica di selezione
            available := r.getAvailableProviders()
            providerName, err := rp.SelectProvider(ctx, &plugins.RouteRequest{
                Model:    req.Model,
                Provider: req.PreferredProvider,
            }, available)

            if err == nil && providerName != "" {
                if provider, exists := r.providers[providerName]; exists {
                    return provider, nil
                }
            }
        }
    }

    // Fallback alla logica di routing standard
    return r.selectProvider(ctx, req)
}
```

## Hook per Monitoring

### Integrazione in `internal/stats/collector.go`

```go
func (c *Collector) setupPluginHooks(registry *plugins.Registry) {
    hooks := registry.GetHooks()

    // Hook per richieste
    hooks.OnRequest(func(ctx context.Context, req *plugins.HookRequest) error {
        c.RecordRequest(req.Provider, req.Model)
        return nil
    })

    // Hook per risposte
    hooks.OnResponse(func(ctx context.Context, resp *plugins.HookResponse) error {
        c.RecordResponse(resp.Provider, resp.Model, resp.Duration, resp.TokensUsed)
        c.RecordCost(resp.Provider, resp.Model, resp.Cost)
        return nil
    })

    // Hook per errori
    hooks.OnError(func(ctx context.Context, err *plugins.HookError) error {
        c.RecordError(err.Provider, err.Model, string(err.ErrorType))
        return nil
    })

    // Hook per cambio provider
    hooks.OnProviderSwitch(func(ctx context.Context, event *plugins.ProviderSwitchEvent) error {
        c.RecordProviderSwitch(event.FromProvider, event.ToProvider, string(event.Reason))
        return nil
    })
}
```

## Admin API per Plugin Management

### Aggiungi endpoint in `internal/gateway/gateway.go`

```go
func (g *Gateway) setupRoutes() {
    // ... existing routes ...

    // Plugin management endpoints
    if g.config.Plugins.Enabled {
        pluginAPI := g.app.Group("/admin/plugins")
        pluginAPI.Get("/", g.handleListPlugins)
        pluginAPI.Get("/:name", g.handleGetPlugin)
        pluginAPI.Post("/:name/enable", g.handleEnablePlugin)
        pluginAPI.Post("/:name/disable", g.handleDisablePlugin)
        pluginAPI.Post("/:name/reload", g.handleReloadPlugin)
        pluginAPI.Get("/:name/metrics", g.handlePluginMetrics)
        pluginAPI.Get("/hooks/stats", g.handleHookStats)
    }
}

func (g *Gateway) handleListPlugins(c fiber.Ctx) error {
    infos := g.pluginRegistry.List()

    plugins := make([]fiber.Map, 0, len(infos))
    for _, info := range infos {
        plugins = append(plugins, fiber.Map{
            "name":        info.Plugin.Name(),
            "version":     info.Plugin.Version(),
            "type":        string(info.Plugin.Type()),
            "description": info.Plugin.Description(),
            "enabled":     info.Config.Enabled,
            "state":       string(info.State),
            "loaded_at":   info.LoadedAt,
            "last_used":   info.LastUsed,
            "metrics":     info.Metrics,
        })
    }

    return c.JSON(fiber.Map{
        "plugins": plugins,
        "count":   len(plugins),
    })
}

func (g *Gateway) handleGetPlugin(c fiber.Ctx) error {
    name := c.Params("name")

    info, err := g.pluginRegistry.GetInfo(name)
    if err != nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "plugin not found",
        })
    }

    return c.JSON(fiber.Map{
        "name":        info.Plugin.Name(),
        "version":     info.Plugin.Version(),
        "type":        string(info.Plugin.Type()),
        "description": info.Plugin.Description(),
        "enabled":     info.Config.Enabled,
        "state":       string(info.State),
        "loaded_at":   info.LoadedAt,
        "last_used":   info.LastUsed,
        "metrics":     info.Metrics,
        "metadata":    info.Plugin.Metadata(),
    })
}

func (g *Gateway) handleEnablePlugin(c fiber.Ctx) error {
    name := c.Params("name")

    if err := g.pluginRegistry.Enable(name); err != nil {
        return c.Status(500).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": "plugin enabled",
        "name":    name,
    })
}

func (g *Gateway) handleDisablePlugin(c fiber.Ctx) error {
    name := c.Params("name")

    if err := g.pluginRegistry.Disable(name); err != nil {
        return c.Status(500).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": "plugin disabled",
        "name":    name,
    })
}

func (g *Gateway) handleReloadPlugin(c fiber.Ctx) error {
    name := c.Params("name")

    if err := g.pluginRegistry.Reload(c.Context(), name); err != nil {
        return c.Status(500).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": "plugin reloaded",
        "name":    name,
    })
}

func (g *Gateway) handlePluginMetrics(c fiber.Ctx) error {
    name := c.Params("name")

    metrics, err := g.pluginRegistry.GetMetrics(name)
    if err != nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "plugin not found",
        })
    }

    return c.JSON(fiber.Map{
        "plugin":  name,
        "metrics": metrics,
    })
}

func (g *Gateway) handleHookStats(c fiber.Ctx) error {
    stats := g.pluginRegistry.GetHooks().Stats()

    return c.JSON(fiber.Map{
        "hooks": stats,
    })
}
```

## Testing

```bash
# Compila e testa il plugin di esempio
cd plugins/example
make build
make verify

# Installa il plugin
make install

# Avvia GoLeapAI
cd ../..
go run cmd/backend/main.go serve

# Testa gli endpoint plugin
curl http://localhost:8080/admin/plugins
curl http://localhost:8080/admin/plugins/example-provider
```

## Best Practices

1. **Validazione**: Valida sempre la configurazione dei plugin
2. **Errori**: Gestisci gracefully errori nei plugin senza crashare il gateway
3. **Logging**: Usa il logger iniettato per consistenza
4. **Monitoring**: Integra metriche dei plugin in Prometheus
5. **Security**: Valida e sanitizza input dai plugin
6. **Performance**: Monitor performance overhead dei plugin
7. **Hot Reload**: Usa con cautela in produzione

## Troubleshooting

### Plugin non si carica
- Verifica che `plugins.enabled: true` in config
- Controlla i log per errori di caricamento
- Verifica che il file .so sia nella directory corretta

### Middleware non viene applicato
- Verifica che `ApplyRoutes()` ritorni le route corrette
- Controlla la priorità del middleware
- Verifica che il plugin sia abilitato

### Hook non viene chiamato
- Verifica che l'hook sia registrato durante `Init()`
- Controlla che l'evento venga triggato nel codice
- Verifica timeout degli hook (default 5s)
