package plugins

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// PluginBuilder aiuta a costruire plugin con pattern builder
type PluginBuilder struct {
	plugin      Plugin
	config      *PluginConfig
	dependencies []string
	hooks       []func(*HookRegistry)
	validators  []func(Plugin) error
}

// NewPluginBuilder crea un nuovo builder
func NewPluginBuilder(plugin Plugin) *PluginBuilder {
	return &PluginBuilder{
		plugin: plugin,
		config: &PluginConfig{
			Name:    plugin.Name(),
			Version: plugin.Version(),
			Enabled: true,
			Config:  make(map[string]interface{}),
		},
		dependencies: make([]string, 0),
		hooks:        make([]func(*HookRegistry), 0),
		validators:   make([]func(Plugin) error, 0),
	}
}

// WithConfig aggiunge configurazione
func (b *PluginBuilder) WithConfig(key string, value interface{}) *PluginBuilder {
	b.config.Config[key] = value
	return b
}

// WithConfigs aggiunge multiple configurazioni
func (b *PluginBuilder) WithConfigs(configs map[string]interface{}) *PluginBuilder {
	for k, v := range configs {
		b.config.Config[k] = v
	}
	return b
}

// WithPriority imposta la priorità
func (b *PluginBuilder) WithPriority(priority int) *PluginBuilder {
	b.config.Priority = priority
	return b
}

// WithDependency aggiunge una dipendenza
func (b *PluginBuilder) WithDependency(dep string) *PluginBuilder {
	b.dependencies = append(b.dependencies, dep)
	return b
}

// WithDependencies aggiunge multiple dipendenze
func (b *PluginBuilder) WithDependencies(deps []string) *PluginBuilder {
	b.dependencies = append(b.dependencies, deps...)
	return b
}

// WithHook aggiunge un hook customizer
func (b *PluginBuilder) WithHook(hookFn func(*HookRegistry)) *PluginBuilder {
	b.hooks = append(b.hooks, hookFn)
	return b
}

// WithValidator aggiunge un validatore custom
func (b *PluginBuilder) WithValidator(validator func(Plugin) error) *PluginBuilder {
	b.validators = append(b.validators, validator)
	return b
}

// Build costruisce e valida il plugin
func (b *PluginBuilder) Build() (Plugin, *PluginConfig, error) {
	// Valida il plugin base
	if err := ValidatePlugin(b.plugin); err != nil {
		return nil, nil, fmt.Errorf("base validation failed: %w", err)
	}

	// Esegui validatori custom
	for _, validator := range b.validators {
		if err := validator(b.plugin); err != nil {
			return nil, nil, fmt.Errorf("custom validation failed: %w", err)
		}
	}

	// Imposta dipendenze
	b.config.Dependencies = b.dependencies

	return b.plugin, b.config, nil
}

// Register costruisce e registra il plugin in un registry
func (b *PluginBuilder) Register(ctx context.Context, registry *Registry) error {
	plugin, config, err := b.Build()
	if err != nil {
		return err
	}

	// Registra hook
	for _, hookFn := range b.hooks {
		hookFn(registry.GetHooks())
	}

	return registry.Register(ctx, plugin, config)
}

// PluginWrapper wrapper generico per plugin
type PluginWrapper struct {
	Plugin
	beforeInit    func(context.Context) error
	afterInit     func(context.Context) error
	beforeShutdown func(context.Context) error
	afterShutdown  func(context.Context) error
}

// WrapPlugin crea un wrapper per un plugin
func WrapPlugin(plugin Plugin) *PluginWrapper {
	return &PluginWrapper{
		Plugin: plugin,
	}
}

// BeforeInit imposta callback prima dell'init
func (w *PluginWrapper) BeforeInit(fn func(context.Context) error) *PluginWrapper {
	w.beforeInit = fn
	return w
}

// AfterInit imposta callback dopo l'init
func (w *PluginWrapper) AfterInit(fn func(context.Context) error) *PluginWrapper {
	w.afterInit = fn
	return w
}

// BeforeShutdown imposta callback prima dello shutdown
func (w *PluginWrapper) BeforeShutdown(fn func(context.Context) error) *PluginWrapper {
	w.beforeShutdown = fn
	return w
}

// AfterShutdown imposta callback dopo lo shutdown
func (w *PluginWrapper) AfterShutdown(fn func(context.Context) error) *PluginWrapper {
	w.afterShutdown = fn
	return w
}

// Init override con lifecycle hooks
func (w *PluginWrapper) Init(ctx context.Context, deps *Dependencies) error {
	if w.beforeInit != nil {
		if err := w.beforeInit(ctx); err != nil {
			return fmt.Errorf("before init hook failed: %w", err)
		}
	}

	if err := w.Plugin.Init(ctx, deps); err != nil {
		return err
	}

	if w.afterInit != nil {
		if err := w.afterInit(ctx); err != nil {
			return fmt.Errorf("after init hook failed: %w", err)
		}
	}

	return nil
}

// Shutdown override con lifecycle hooks
func (w *PluginWrapper) Shutdown(ctx context.Context) error {
	if w.beforeShutdown != nil {
		if err := w.beforeShutdown(ctx); err != nil {
			return fmt.Errorf("before shutdown hook failed: %w", err)
		}
	}

	if err := w.Plugin.Shutdown(ctx); err != nil {
		return err
	}

	if w.afterShutdown != nil {
		if err := w.afterShutdown(ctx); err != nil {
			return fmt.Errorf("after shutdown hook failed: %w", err)
		}
	}

	return nil
}

// PluginMetadataBuilder aiuta a costruire metadata
type PluginMetadataBuilder struct {
	metadata map[string]interface{}
}

// NewMetadataBuilder crea un nuovo builder per metadata
func NewMetadataBuilder() *PluginMetadataBuilder {
	return &PluginMetadataBuilder{
		metadata: make(map[string]interface{}),
	}
}

// WithAuthor imposta l'autore
func (b *PluginMetadataBuilder) WithAuthor(author string) *PluginMetadataBuilder {
	b.metadata["author"] = author
	return b
}

// WithLicense imposta la licenza
func (b *PluginMetadataBuilder) WithLicense(license string) *PluginMetadataBuilder {
	b.metadata["license"] = license
	return b
}

// WithHomepage imposta l'homepage
func (b *PluginMetadataBuilder) WithHomepage(url string) *PluginMetadataBuilder {
	b.metadata["homepage"] = url
	return b
}

// WithTags aggiunge tags
func (b *PluginMetadataBuilder) WithTags(tags ...string) *PluginMetadataBuilder {
	b.metadata["tags"] = tags
	return b
}

// With aggiunge un campo custom
func (b *PluginMetadataBuilder) With(key string, value interface{}) *PluginMetadataBuilder {
	b.metadata[key] = value
	return b
}

// Build costruisce i metadata
func (b *PluginMetadataBuilder) Build() map[string]interface{} {
	return b.metadata
}

// CastPlugin utility per fare type assertion sicuro
func CastPlugin[T Plugin](plugin Plugin) (T, error) {
	var zero T
	if plugin == nil {
		return zero, fmt.Errorf("plugin is nil")
	}

	casted, ok := plugin.(T)
	if !ok {
		return zero, fmt.Errorf("plugin %s is not of expected type", plugin.Name())
	}

	return casted, nil
}

// GetPluginType restituisce il tipo runtime di un plugin
func GetPluginType(plugin Plugin) string {
	return reflect.TypeOf(plugin).String()
}

// IsPluginType verifica se un plugin è di un determinato tipo
func IsPluginType(plugin Plugin, pluginType PluginType) bool {
	return plugin.Type() == pluginType
}

// MeasurePluginInit misura il tempo di inizializzazione
func MeasurePluginInit(plugin Plugin, ctx context.Context, deps *Dependencies) (time.Duration, error) {
	start := time.Now()
	err := plugin.Init(ctx, deps)
	duration := time.Since(start)
	return duration, err
}

// MeasurePluginShutdown misura il tempo di shutdown
func MeasurePluginShutdown(plugin Plugin, ctx context.Context) (time.Duration, error) {
	start := time.Now()
	err := plugin.Shutdown(ctx)
	duration := time.Since(start)
	return duration, err
}

// PluginChain gestisce una catena di plugin
type PluginChain struct {
	plugins []Plugin
}

// NewPluginChain crea una nuova catena
func NewPluginChain() *PluginChain {
	return &PluginChain{
		plugins: make([]Plugin, 0),
	}
}

// Add aggiunge un plugin alla catena
func (c *PluginChain) Add(plugin Plugin) *PluginChain {
	c.plugins = append(c.plugins, plugin)
	return c
}

// InitAll inizializza tutti i plugin nella catena
func (c *PluginChain) InitAll(ctx context.Context, deps *Dependencies) error {
	for i, plugin := range c.plugins {
		if err := plugin.Init(ctx, deps); err != nil {
			return fmt.Errorf("plugin %d (%s) init failed: %w", i, plugin.Name(), err)
		}
	}
	return nil
}

// ShutdownAll esegue shutdown di tutti i plugin (ordine inverso)
func (c *PluginChain) ShutdownAll(ctx context.Context) error {
	// Shutdown in ordine inverso
	for i := len(c.plugins) - 1; i >= 0; i-- {
		if err := c.plugins[i].Shutdown(ctx); err != nil {
			return fmt.Errorf("plugin %s shutdown failed: %w", c.plugins[i].Name(), err)
		}
	}
	return nil
}

// Len restituisce il numero di plugin nella catena
func (c *PluginChain) Len() int {
	return len(c.plugins)
}

// ConfigValidator validatore per configurazione plugin
type ConfigValidator struct {
	required []string
	optional []string
}

// NewConfigValidator crea un nuovo validatore di config
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		required: make([]string, 0),
		optional: make([]string, 0),
	}
}

// RequireField aggiunge un campo required
func (v *ConfigValidator) RequireField(field string) *ConfigValidator {
	v.required = append(v.required, field)
	return v
}

// OptionalField aggiunge un campo opzionale
func (v *ConfigValidator) OptionalField(field string) *ConfigValidator {
	v.optional = append(v.optional, field)
	return v
}

// Validate valida la configurazione
func (v *ConfigValidator) Validate(config map[string]interface{}) error {
	for _, field := range v.required {
		if _, exists := config[field]; !exists {
			return fmt.Errorf("required field missing: %s", field)
		}
	}
	return nil
}

// GetString utility per ottenere string da config
func GetString(config map[string]interface{}, key string, defaultValue string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultValue
}

// GetInt utility per ottenere int da config
func GetInt(config map[string]interface{}, key string, defaultValue int) int {
	if val, ok := config[key].(int); ok {
		return val
	}
	return defaultValue
}

// GetBool utility per ottenere bool da config
func GetBool(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

// GetDuration utility per ottenere duration da config
func GetDuration(config map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if val, ok := config[key].(string); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	if val, ok := config[key].(time.Duration); ok {
		return val
	}
	return defaultValue
}

// SafeInit esegue Init con panic recovery
func SafeInit(plugin Plugin, ctx context.Context, deps *Dependencies) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin init panicked: %v", r)
		}
	}()

	return plugin.Init(ctx, deps)
}

// SafeShutdown esegue Shutdown con panic recovery
func SafeShutdown(plugin Plugin, ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin shutdown panicked: %v", r)
		}
	}()

	return plugin.Shutdown(ctx)
}
