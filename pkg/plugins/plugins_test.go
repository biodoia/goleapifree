package plugins

import (
	"context"
	"testing"
	"time"
)

// MockPlugin per testing
type MockPlugin struct {
	name        string
	version     string
	description string
	pluginType  PluginType
	initCalled  bool
	shutdownCalled bool
}

func NewMockPlugin(name, version string, pType PluginType) *MockPlugin {
	return &MockPlugin{
		name:        name,
		version:     version,
		description: "Mock plugin for testing",
		pluginType:  pType,
	}
}

func (m *MockPlugin) Name() string        { return m.name }
func (m *MockPlugin) Version() string     { return m.version }
func (m *MockPlugin) Description() string { return m.description }
func (m *MockPlugin) Type() PluginType    { return m.pluginType }

func (m *MockPlugin) Init(ctx context.Context, deps *Dependencies) error {
	m.initCalled = true
	return nil
}

func (m *MockPlugin) Shutdown(ctx context.Context) error {
	m.shutdownCalled = true
	return nil
}

func (m *MockPlugin) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"test": true,
	}
}

// Test Registry

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry(nil)
	if registry == nil {
		t.Fatal("Registry should not be nil")
	}

	if len(registry.plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(registry.plugins))
	}
}

func TestRegisterPlugin(t *testing.T) {
	registry := NewRegistry(nil)
	ctx := context.Background()

	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)
	config := &PluginConfig{
		Name:    plugin.Name(),
		Version: plugin.Version(),
		Enabled: true,
	}

	err := registry.Register(ctx, plugin, config)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	if !plugin.initCalled {
		t.Error("Plugin Init was not called")
	}

	// Verifica che sia registrato
	retrieved, err := registry.Get(plugin.Name())
	if err != nil {
		t.Fatalf("Failed to get plugin: %v", err)
	}

	if retrieved.Name() != plugin.Name() {
		t.Errorf("Expected plugin name %s, got %s", plugin.Name(), retrieved.Name())
	}
}

func TestUnregisterPlugin(t *testing.T) {
	registry := NewRegistry(nil)
	ctx := context.Background()

	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)
	config := &PluginConfig{
		Name:    plugin.Name(),
		Version: plugin.Version(),
		Enabled: true,
	}

	registry.Register(ctx, plugin, config)

	err := registry.Unregister(ctx, plugin.Name())
	if err != nil {
		t.Fatalf("Failed to unregister plugin: %v", err)
	}

	if !plugin.shutdownCalled {
		t.Error("Plugin Shutdown was not called")
	}

	// Verifica che non sia più registrato
	_, err = registry.Get(plugin.Name())
	if err != ErrPluginNotFound {
		t.Errorf("Expected ErrPluginNotFound, got %v", err)
	}
}

func TestGetByType(t *testing.T) {
	registry := NewRegistry(nil)
	ctx := context.Background()

	// Registra plugin di diversi tipi
	provider1 := NewMockPlugin("provider-1", "1.0.0", PluginTypeProvider)
	provider2 := NewMockPlugin("provider-2", "1.0.0", PluginTypeProvider)
	middleware := NewMockPlugin("middleware-1", "1.0.0", PluginTypeMiddleware)

	registry.Register(ctx, provider1, &PluginConfig{Name: provider1.Name(), Enabled: true})
	registry.Register(ctx, provider2, &PluginConfig{Name: provider2.Name(), Enabled: true})
	registry.Register(ctx, middleware, &PluginConfig{Name: middleware.Name(), Enabled: true})

	// Recupera per tipo
	providers := registry.GetByType(PluginTypeProvider)
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}

	middlewares := registry.GetByType(PluginTypeMiddleware)
	if len(middlewares) != 1 {
		t.Errorf("Expected 1 middleware, got %d", len(middlewares))
	}
}

func TestEnableDisablePlugin(t *testing.T) {
	registry := NewRegistry(nil)
	ctx := context.Background()

	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)
	config := &PluginConfig{
		Name:    plugin.Name(),
		Version: plugin.Version(),
		Enabled: true,
	}

	registry.Register(ctx, plugin, config)

	// Disabilita
	err := registry.Disable(plugin.Name())
	if err != nil {
		t.Fatalf("Failed to disable plugin: %v", err)
	}

	if registry.IsEnabled(plugin.Name()) {
		t.Error("Plugin should be disabled")
	}

	// Abilita
	err = registry.Enable(plugin.Name())
	if err != nil {
		t.Fatalf("Failed to enable plugin: %v", err)
	}

	if !registry.IsEnabled(plugin.Name()) {
		t.Error("Plugin should be enabled")
	}
}

// Test HookRegistry

func TestHookRegistry(t *testing.T) {
	hooks := NewHookRegistry()

	called := false
	hooks.OnRequest(func(ctx context.Context, req *HookRequest) error {
		called = true
		return nil
	})

	ctx := context.Background()
	req := &HookRequest{
		RequestID: "test-123",
		Provider:  "test-provider",
		Model:     "test-model",
	}

	err := hooks.TriggerRequest(ctx, req)
	if err != nil {
		t.Fatalf("TriggerRequest failed: %v", err)
	}

	if !called {
		t.Error("Hook was not called")
	}
}

func TestHookStats(t *testing.T) {
	hooks := NewHookRegistry()

	hooks.OnRequest(func(ctx context.Context, req *HookRequest) error { return nil })
	hooks.OnRequest(func(ctx context.Context, req *HookRequest) error { return nil })
	hooks.OnResponse(func(ctx context.Context, resp *HookResponse) error { return nil })

	stats := hooks.Stats()
	if stats.RequestHooks != 2 {
		t.Errorf("Expected 2 request hooks, got %d", stats.RequestHooks)
	}
	if stats.ResponseHooks != 1 {
		t.Errorf("Expected 1 response hook, got %d", stats.ResponseHooks)
	}
}

// Test PluginBuilder

func TestPluginBuilder(t *testing.T) {
	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)

	builder := NewPluginBuilder(plugin).
		WithConfig("api_key", "test-key").
		WithConfig("timeout", "30s").
		WithPriority(10).
		WithDependency("dep1").
		WithDependency("dep2")

	builtPlugin, config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if builtPlugin.Name() != plugin.Name() {
		t.Errorf("Expected plugin name %s, got %s", plugin.Name(), builtPlugin.Name())
	}

	if config.Priority != 10 {
		t.Errorf("Expected priority 10, got %d", config.Priority)
	}

	if len(config.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(config.Dependencies))
	}

	if config.Config["api_key"] != "test-key" {
		t.Error("Config api_key not set correctly")
	}
}

// Test PluginWrapper

func TestPluginWrapper(t *testing.T) {
	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)

	beforeCalled := false
	afterCalled := false

	wrapped := WrapPlugin(plugin).
		BeforeInit(func(ctx context.Context) error {
			beforeCalled = true
			return nil
		}).
		AfterInit(func(ctx context.Context) error {
			afterCalled = true
			return nil
		})

	ctx := context.Background()
	deps := &Dependencies{
		Config:      make(map[string]interface{}),
		Logger:      newDefaultLogger(),
		Hooks:       NewHookRegistry(),
		SharedState: make(map[string]interface{}),
	}

	err := wrapped.Init(ctx, deps)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !beforeCalled {
		t.Error("Before hook was not called")
	}

	if !afterCalled {
		t.Error("After hook was not called")
	}

	if !plugin.initCalled {
		t.Error("Plugin Init was not called")
	}
}

// Test Utilities

func TestGetString(t *testing.T) {
	config := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	val := GetString(config, "key1", "default")
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%s'", val)
	}

	val = GetString(config, "key2", "default")
	if val != "default" {
		t.Errorf("Expected 'default', got '%s'", val)
	}

	val = GetString(config, "key3", "default")
	if val != "default" {
		t.Errorf("Expected 'default', got '%s'", val)
	}
}

func TestGetInt(t *testing.T) {
	config := map[string]interface{}{
		"key1": 42,
		"key2": "not-an-int",
	}

	val := GetInt(config, "key1", 0)
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}

	val = GetInt(config, "key2", 99)
	if val != 99 {
		t.Errorf("Expected 99, got %d", val)
	}
}

func TestGetDuration(t *testing.T) {
	config := map[string]interface{}{
		"key1": "30s",
		"key2": 5 * time.Second,
		"key3": "invalid",
	}

	val := GetDuration(config, "key1", 0)
	if val != 30*time.Second {
		t.Errorf("Expected 30s, got %v", val)
	}

	val = GetDuration(config, "key2", 0)
	if val != 5*time.Second {
		t.Errorf("Expected 5s, got %v", val)
	}

	val = GetDuration(config, "key3", 10*time.Second)
	if val != 10*time.Second {
		t.Errorf("Expected 10s (default), got %v", val)
	}
}

func TestPluginChain(t *testing.T) {
	chain := NewPluginChain()

	plugin1 := NewMockPlugin("plugin-1", "1.0.0", PluginTypeProvider)
	plugin2 := NewMockPlugin("plugin-2", "1.0.0", PluginTypeProvider)

	chain.Add(plugin1).Add(plugin2)

	if chain.Len() != 2 {
		t.Errorf("Expected chain length 2, got %d", chain.Len())
	}

	ctx := context.Background()
	deps := &Dependencies{
		Config:      make(map[string]interface{}),
		Logger:      newDefaultLogger(),
		Hooks:       NewHookRegistry(),
		SharedState: make(map[string]interface{}),
	}

	err := chain.InitAll(ctx, deps)
	if err != nil {
		t.Fatalf("InitAll failed: %v", err)
	}

	if !plugin1.initCalled || !plugin2.initCalled {
		t.Error("Not all plugins were initialized")
	}

	err = chain.ShutdownAll(ctx)
	if err != nil {
		t.Fatalf("ShutdownAll failed: %v", err)
	}

	if !plugin1.shutdownCalled || !plugin2.shutdownCalled {
		t.Error("Not all plugins were shutdown")
	}
}

func TestConfigValidator(t *testing.T) {
	validator := NewConfigValidator().
		RequireField("api_key").
		RequireField("endpoint").
		OptionalField("timeout")

	// Config valida
	validConfig := map[string]interface{}{
		"api_key":  "test-key",
		"endpoint": "https://api.example.com",
		"timeout":  "30s",
	}

	err := validator.Validate(validConfig)
	if err != nil {
		t.Errorf("Valid config failed validation: %v", err)
	}

	// Config invalida (manca campo required)
	invalidConfig := map[string]interface{}{
		"api_key": "test-key",
	}

	err = validator.Validate(invalidConfig)
	if err == nil {
		t.Error("Invalid config passed validation")
	}
}

func TestValidatePlugin(t *testing.T) {
	// Plugin valido
	validPlugin := NewMockPlugin("test", "1.0.0", PluginTypeProvider)
	err := ValidatePlugin(validPlugin)
	if err != nil {
		t.Errorf("Valid plugin failed validation: %v", err)
	}

	// Plugin con nome vuoto
	invalidPlugin := NewMockPlugin("", "1.0.0", PluginTypeProvider)
	err = ValidatePlugin(invalidPlugin)
	if err != ErrInvalidPluginName {
		t.Errorf("Expected ErrInvalidPluginName, got %v", err)
	}

	// Plugin con versione vuota
	invalidPlugin = NewMockPlugin("test", "", PluginTypeProvider)
	err = ValidatePlugin(invalidPlugin)
	if err != ErrInvalidPluginVersion {
		t.Errorf("Expected ErrInvalidPluginVersion, got %v", err)
	}
}

// Benchmark

func BenchmarkRegistryGet(b *testing.B) {
	registry := NewRegistry(nil)
	ctx := context.Background()

	plugin := NewMockPlugin("test-plugin", "1.0.0", PluginTypeProvider)
	config := &PluginConfig{Name: plugin.Name(), Enabled: true}
	registry.Register(ctx, plugin, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("test-plugin")
	}
}

func BenchmarkHookTrigger(b *testing.B) {
	hooks := NewHookRegistry()
	hooks.OnRequest(func(ctx context.Context, req *HookRequest) error { return nil })

	ctx := context.Background()
	req := &HookRequest{
		RequestID: "test-123",
		Provider:  "test-provider",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hooks.TriggerRequest(ctx, req)
	}
}
