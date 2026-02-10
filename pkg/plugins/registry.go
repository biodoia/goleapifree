package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Registry gestisce la registrazione e il ciclo di vita dei plugin
type Registry struct {
	mu        sync.RWMutex
	plugins   map[string]*PluginInfo
	byType    map[PluginType][]string
	factories map[string]PluginFactory
	config    *RegistryConfig
	hooks     *HookRegistry
	deps      *Dependencies
}

// RegistryConfig configurazione del registry
type RegistryConfig struct {
	PluginDir      string
	AutoLoad       bool
	HotReload      bool
	ReloadInterval time.Duration
	MaxPlugins     int
	Timeout        time.Duration
}

// DefaultRegistryConfig restituisce una configurazione di default
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		PluginDir:      "./plugins",
		AutoLoad:       true,
		HotReload:      false,
		ReloadInterval: 30 * time.Second,
		MaxPlugins:     100,
		Timeout:        10 * time.Second,
	}
}

// NewRegistry crea un nuovo registry di plugin
func NewRegistry(config *RegistryConfig) *Registry {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	hooks := NewHookRegistry()

	r := &Registry{
		plugins:   make(map[string]*PluginInfo),
		byType:    make(map[PluginType][]string),
		factories: make(map[string]PluginFactory),
		config:    config,
		hooks:     hooks,
		deps: &Dependencies{
			Config:      make(map[string]interface{}),
			Logger:      newDefaultLogger(),
			Hooks:       hooks,
			PluginDir:   config.PluginDir,
			SharedState: make(map[string]interface{}),
		},
	}

	log.Info().
		Str("plugin_dir", config.PluginDir).
		Bool("auto_load", config.AutoLoad).
		Bool("hot_reload", config.HotReload).
		Msg("Plugin registry initialized")

	return r
}

// RegisterFactory registra una factory per creare plugin
func (r *Registry) RegisterFactory(name string, factory PluginFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("factory already registered: %s", name)
	}

	r.factories[name] = factory
	log.Debug().Str("name", name).Msg("Plugin factory registered")
	return nil
}

// Register registra e inizializza un plugin
func (r *Registry) Register(ctx context.Context, plugin Plugin, config *PluginConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Valida il plugin
	if err := ValidatePlugin(plugin); err != nil {
		return fmt.Errorf("invalid plugin: %w", err)
	}

	name := plugin.Name()

	// Verifica che non sia già registrato
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin already registered: %s", name)
	}

	// Verifica limiti
	if len(r.plugins) >= r.config.MaxPlugins {
		return ErrMaxPluginsReached
	}

	// Verifica dipendenze
	if config != nil && len(config.Dependencies) > 0 {
		if err := r.checkDependencies(config.Dependencies); err != nil {
			return fmt.Errorf("dependency check failed: %w", err)
		}
	}

	// Inizializza il plugin con timeout
	initCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Configura le dipendenze
	if config != nil && config.Config != nil {
		r.deps.Config = config.Config
	}

	if err := plugin.Init(initCtx, r.deps); err != nil {
		return fmt.Errorf("plugin initialization failed: %w", err)
	}

	// Crea info del plugin
	info := &PluginInfo{
		Plugin:   plugin,
		Config:   config,
		LoadedAt: time.Now(),
		LastUsed: time.Now(),
		State:    PluginStateLoaded,
		Metrics:  PluginMetrics{},
	}

	// Registra il plugin
	r.plugins[name] = info

	// Indicizza per tipo
	pluginType := plugin.Type()
	r.byType[pluginType] = append(r.byType[pluginType], name)

	// Trigger hook
	r.hooks.TriggerPluginLoaded(ctx, name, plugin)

	log.Info().
		Str("name", name).
		Str("version", plugin.Version()).
		Str("type", string(pluginType)).
		Msg("Plugin registered successfully")

	return nil
}

// Unregister rimuove un plugin dal registry
func (r *Registry) Unregister(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound
	}

	// Shutdown del plugin
	shutdownCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := info.Plugin.Shutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Str("name", name).Msg("Plugin shutdown error")
	}

	// Rimuovi dal registry
	delete(r.plugins, name)

	// Rimuovi dall'indice per tipo
	pluginType := info.Plugin.Type()
	r.removeFromTypeIndex(pluginType, name)

	// Trigger hook
	r.hooks.TriggerPluginUnloaded(ctx, name)

	info.State = PluginStateUnloaded

	log.Info().Str("name", name).Msg("Plugin unregistered")
	return nil
}

// Get recupera un plugin per nome
func (r *Registry) Get(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.plugins[name]
	if !exists {
		return nil, ErrPluginNotFound
	}

	// Aggiorna metriche
	info.LastUsed = time.Now()
	info.Metrics.Calls++

	return info.Plugin, nil
}

// GetByType recupera tutti i plugin di un determinato tipo
func (r *Registry) GetByType(pluginType PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, exists := r.byType[pluginType]
	if !exists {
		return nil
	}

	plugins := make([]Plugin, 0, len(names))
	for _, name := range names {
		if info, ok := r.plugins[name]; ok {
			plugins = append(plugins, info.Plugin)
		}
	}

	return plugins
}

// List restituisce informazioni su tutti i plugin registrati
func (r *Registry) List() []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]*PluginInfo, 0, len(r.plugins))
	for _, info := range r.plugins {
		infos = append(infos, info)
	}

	// Ordina per nome
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Plugin.Name() < infos[j].Plugin.Name()
	})

	return infos
}

// ListByType restituisce informazioni sui plugin di un tipo
func (r *Registry) ListByType(pluginType PluginType) []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, exists := r.byType[pluginType]
	if !exists {
		return nil
	}

	infos := make([]*PluginInfo, 0, len(names))
	for _, name := range names {
		if info, ok := r.plugins[name]; ok {
			infos = append(infos, info)
		}
	}

	return infos
}

// GetInfo restituisce informazioni su un plugin
func (r *Registry) GetInfo(name string) (*PluginInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.plugins[name]
	if !exists {
		return nil, ErrPluginNotFound
	}

	return info, nil
}

// Enable abilita un plugin
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound
	}

	if info.Config != nil {
		info.Config.Enabled = true
	}
	info.State = PluginStateActive

	log.Info().Str("name", name).Msg("Plugin enabled")
	return nil
}

// Disable disabilita un plugin
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound
	}

	if info.Config != nil {
		info.Config.Enabled = false
	}
	info.State = PluginStateInactive

	log.Info().Str("name", name).Msg("Plugin disabled")
	return nil
}

// IsEnabled verifica se un plugin è abilitato
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.plugins[name]
	if !exists {
		return false
	}

	if info.Config != nil {
		return info.Config.Enabled
	}

	return info.State == PluginStateActive
}

// Reload ricarica un plugin
func (r *Registry) Reload(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound
	}

	// Shutdown del plugin corrente
	shutdownCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := info.Plugin.Shutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Str("name", name).Msg("Plugin shutdown error during reload")
	}

	// Re-inizializza
	initCtx, initCancel := context.WithTimeout(ctx, r.config.Timeout)
	defer initCancel()

	if err := info.Plugin.Init(initCtx, r.deps); err != nil {
		info.State = PluginStateError
		info.Error = err
		return fmt.Errorf("plugin re-initialization failed: %w", err)
	}

	info.LoadedAt = time.Now()
	info.State = PluginStateActive
	info.Error = nil

	log.Info().Str("name", name).Msg("Plugin reloaded successfully")
	return nil
}

// ShutdownAll esegue lo shutdown di tutti i plugin
func (r *Registry) ShutdownAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error

	for name, info := range r.plugins {
		shutdownCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)

		if err := info.Plugin.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Str("name", name).Msg("Plugin shutdown failed")
			errors = append(errors, fmt.Errorf("%s: %w", name, err))
		}

		cancel()
		info.State = PluginStateUnloaded
	}

	log.Info().Int("count", len(r.plugins)).Msg("All plugins shutdown")

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// GetHooks restituisce il registro degli hook
func (r *Registry) GetHooks() *HookRegistry {
	return r.hooks
}

// GetDependencies restituisce le dipendenze condivise
func (r *Registry) GetDependencies() *Dependencies {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deps
}

// UpdateConfig aggiorna la configurazione di un plugin
func (r *Registry) UpdateConfig(name string, config map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound
	}

	if info.Config == nil {
		info.Config = &PluginConfig{}
	}

	info.Config.Config = config

	log.Info().Str("name", name).Msg("Plugin config updated")
	return nil
}

// GetMetrics restituisce le metriche di un plugin
func (r *Registry) GetMetrics(name string) (*PluginMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.plugins[name]
	if !exists {
		return nil, ErrPluginNotFound
	}

	return &info.Metrics, nil
}

// checkDependencies verifica che tutte le dipendenze siano soddisfatte
func (r *Registry) checkDependencies(deps []string) error {
	for _, dep := range deps {
		if _, exists := r.plugins[dep]; !exists {
			return fmt.Errorf("missing dependency: %s", dep)
		}
	}
	return nil
}

// removeFromTypeIndex rimuove un plugin dall'indice per tipo
func (r *Registry) removeFromTypeIndex(pluginType PluginType, name string) {
	names := r.byType[pluginType]
	for i, n := range names {
		if n == name {
			r.byType[pluginType] = append(names[:i], names[i+1:]...)
			break
		}
	}
}

// defaultLogger implementazione di base del logger
type defaultLogger struct{}

func newDefaultLogger() Logger {
	return &defaultLogger{}
}

func (l *defaultLogger) Debug(msg string, fields map[string]interface{}) {
	log.Debug().Fields(fields).Msg(msg)
}

func (l *defaultLogger) Info(msg string, fields map[string]interface{}) {
	log.Info().Fields(fields).Msg(msg)
}

func (l *defaultLogger) Warn(msg string, fields map[string]interface{}) {
	log.Warn().Fields(fields).Msg(msg)
}

func (l *defaultLogger) Error(msg string, fields map[string]interface{}) {
	log.Error().Fields(fields).Msg(msg)
}

// Errori comuni
var (
	ErrPluginNotFound       = fmt.Errorf("plugin not found")
	ErrMaxPluginsReached    = fmt.Errorf("maximum number of plugins reached")
	ErrInvalidPluginName    = fmt.Errorf("invalid plugin name")
	ErrInvalidPluginVersion = fmt.Errorf("invalid plugin version")
	ErrInvalidPluginType    = fmt.Errorf("invalid plugin type")
)
