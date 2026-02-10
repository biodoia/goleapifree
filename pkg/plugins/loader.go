package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Loader gestisce il caricamento dinamico dei plugin
type Loader struct {
	registry      *Registry
	loadedPlugins map[string]*plugin.Plugin
	watchedDirs   map[string]bool
	mu            sync.RWMutex
	config        *LoaderConfig
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// LoaderConfig configurazione del loader
type LoaderConfig struct {
	PluginDir      string
	AutoLoad       bool
	HotReload      bool
	WatchInterval  time.Duration
	FilePattern    string
	MaxConcurrent  int
	RequireSymbol  string
	ValidatePlugin bool
}

// DefaultLoaderConfig restituisce una configurazione di default
func DefaultLoaderConfig() *LoaderConfig {
	return &LoaderConfig{
		PluginDir:      "./plugins",
		AutoLoad:       true,
		HotReload:      false,
		WatchInterval:  30 * time.Second,
		FilePattern:    "*.so",
		MaxConcurrent:  5,
		RequireSymbol:  "NewPlugin",
		ValidatePlugin: true,
	}
}

// NewLoader crea un nuovo loader di plugin
func NewLoader(registry *Registry, config *LoaderConfig) *Loader {
	if config == nil {
		config = DefaultLoaderConfig()
	}

	l := &Loader{
		registry:      registry,
		loadedPlugins: make(map[string]*plugin.Plugin),
		watchedDirs:   make(map[string]bool),
		config:        config,
		stopCh:        make(chan struct{}),
	}

	log.Info().
		Str("plugin_dir", config.PluginDir).
		Bool("auto_load", config.AutoLoad).
		Bool("hot_reload", config.HotReload).
		Msg("Plugin loader initialized")

	return l
}

// LoadPlugin carica un singolo plugin .so
func (l *Loader) LoadPlugin(ctx context.Context, path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Verifica che il file esista
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	// Carica il plugin .so
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Cerca il simbolo factory
	symbol, err := p.Lookup(l.config.RequireSymbol)
	if err != nil {
		return fmt.Errorf("plugin missing required symbol '%s': %w", l.config.RequireSymbol, err)
	}

	// Verifica che sia una factory valida
	factory, ok := symbol.(func() (Plugin, error))
	if !ok {
		return fmt.Errorf("invalid plugin factory signature")
	}

	// Crea l'istanza del plugin
	pluginInstance, err := factory()
	if err != nil {
		return fmt.Errorf("plugin factory failed: %w", err)
	}

	// Valida il plugin
	if l.config.ValidatePlugin {
		if err := ValidatePlugin(pluginInstance); err != nil {
			return fmt.Errorf("plugin validation failed: %w", err)
		}
	}

	// Registra il plugin nel registry
	pluginConfig := &PluginConfig{
		Name:    pluginInstance.Name(),
		Version: pluginInstance.Version(),
		Enabled: true,
		Config:  make(map[string]interface{}),
	}

	if err := l.registry.Register(ctx, pluginInstance, pluginConfig); err != nil {
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	// Salva il riferimento al plugin .so
	l.loadedPlugins[path] = p

	log.Info().
		Str("path", path).
		Str("name", pluginInstance.Name()).
		Str("version", pluginInstance.Version()).
		Msg("Plugin loaded successfully")

	return nil
}

// LoadFromDirectory carica tutti i plugin da una directory
func (l *Loader) LoadFromDirectory(ctx context.Context, dir string) error {
	// Verifica che la directory esista
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("plugin directory not found: %w", err)
	}

	// Trova tutti i file .so
	pattern := filepath.Join(dir, l.config.FilePattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find plugins: %w", err)
	}

	if len(matches) == 0 {
		log.Warn().Str("dir", dir).Msg("No plugins found in directory")
		return nil
	}

	log.Info().
		Str("dir", dir).
		Int("count", len(matches)).
		Msg("Found plugins to load")

	// Carica i plugin con concorrenza limitata
	sem := make(chan struct{}, l.config.MaxConcurrent)
	var wg sync.WaitGroup
	errCh := make(chan error, len(matches))

	for _, path := range matches {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := l.LoadPlugin(ctx, p); err != nil {
				errCh <- fmt.Errorf("%s: %w", filepath.Base(p), err)
			}
		}(path)
	}

	wg.Wait()
	close(errCh)

	// Raccogli eventuali errori
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
		log.Error().Err(err).Msg("Failed to load plugin")
	}

	if len(errors) > 0 {
		return fmt.Errorf("some plugins failed to load: %v", errors)
	}

	return nil
}

// UnloadPlugin scarica un plugin
func (l *Loader) UnloadPlugin(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Trova il path del plugin
	var pluginPath string
	for path := range l.loadedPlugins {
		if strings.Contains(path, name) {
			pluginPath = path
			break
		}
	}

	if pluginPath == "" {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Unregister dal registry
	if err := l.registry.Unregister(ctx, name); err != nil {
		return fmt.Errorf("failed to unregister plugin: %w", err)
	}

	// Rimuovi il riferimento
	delete(l.loadedPlugins, pluginPath)

	log.Info().Str("name", name).Msg("Plugin unloaded")
	return nil
}

// ReloadPlugin ricarica un plugin
func (l *Loader) ReloadPlugin(ctx context.Context, name string) error {
	// Trova il path del plugin
	l.mu.RLock()
	var pluginPath string
	for path := range l.loadedPlugins {
		if strings.Contains(path, name) {
			pluginPath = path
			break
		}
	}
	l.mu.RUnlock()

	if pluginPath == "" {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Unload
	if err := l.UnloadPlugin(ctx, name); err != nil {
		return fmt.Errorf("failed to unload plugin: %w", err)
	}

	// Reload
	if err := l.LoadPlugin(ctx, pluginPath); err != nil {
		return fmt.Errorf("failed to reload plugin: %w", err)
	}

	log.Info().Str("name", name).Msg("Plugin reloaded successfully")
	return nil
}

// StartWatching avvia il monitoraggio delle directory per hot reload
func (l *Loader) StartWatching(ctx context.Context) error {
	if !l.config.HotReload {
		return nil
	}

	l.wg.Add(1)
	go l.watchLoop(ctx)

	log.Info().
		Dur("interval", l.config.WatchInterval).
		Msg("Plugin hot reload watcher started")

	return nil
}

// StopWatching ferma il monitoraggio
func (l *Loader) StopWatching() {
	close(l.stopCh)
	l.wg.Wait()
	log.Info().Msg("Plugin watcher stopped")
}

// watchLoop loop di monitoraggio per hot reload
func (l *Loader) watchLoop(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.WatchInterval)
	defer ticker.Stop()

	// Mappa per tracciare i modtime dei file
	modTimes := make(map[string]time.Time)

	for {
		select {
		case <-ticker.C:
			l.checkForChanges(ctx, modTimes)
		case <-l.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkForChanges controlla se ci sono cambiamenti nei plugin
func (l *Loader) checkForChanges(ctx context.Context, modTimes map[string]time.Time) {
	l.mu.RLock()
	paths := make([]string, 0, len(l.loadedPlugins))
	for path := range l.loadedPlugins {
		paths = append(paths, path)
	}
	l.mu.RUnlock()

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to stat plugin file")
			continue
		}

		modTime := info.ModTime()
		lastModTime, exists := modTimes[path]

		// Se il file è cambiato, ricaricalo
		if exists && modTime.After(lastModTime) {
			log.Info().
				Str("path", path).
				Time("old_time", lastModTime).
				Time("new_time", modTime).
				Msg("Plugin file changed, reloading")

			name := filepath.Base(path)
			name = strings.TrimSuffix(name, filepath.Ext(name))

			if err := l.ReloadPlugin(ctx, name); err != nil {
				log.Error().Err(err).Str("name", name).Msg("Failed to hot reload plugin")
			}
		}

		modTimes[path] = modTime
	}
}

// LoadAll carica tutti i plugin configurati
func (l *Loader) LoadAll(ctx context.Context) error {
	if !l.config.AutoLoad {
		return nil
	}

	// Carica dalla directory principale
	if err := l.LoadFromDirectory(ctx, l.config.PluginDir); err != nil {
		return fmt.Errorf("failed to load plugins from main directory: %w", err)
	}

	// Cerca anche in sottodirectory
	subdirs, err := l.findSubdirectories(l.config.PluginDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to find subdirectories")
	}

	for _, dir := range subdirs {
		if err := l.LoadFromDirectory(ctx, dir); err != nil {
			log.Warn().Err(err).Str("dir", dir).Msg("Failed to load plugins from subdirectory")
		}
	}

	return nil
}

// findSubdirectories trova tutte le sottodirectory
func (l *Loader) findSubdirectories(root string) ([]string, error) {
	var dirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != root {
			dirs = append(dirs, path)
		}

		return nil
	})

	return dirs, err
}

// ListLoaded restituisce la lista dei plugin caricati
func (l *Loader) ListLoaded() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	paths := make([]string, 0, len(l.loadedPlugins))
	for path := range l.loadedPlugins {
		paths = append(paths, path)
	}

	return paths
}

// IsLoaded verifica se un plugin è caricato
func (l *Loader) IsLoaded(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for path := range l.loadedPlugins {
		if strings.Contains(path, name) {
			return true
		}
	}

	return false
}

// UnloadAll scarica tutti i plugin
func (l *Loader) UnloadAll(ctx context.Context) error {
	l.mu.RLock()
	names := make([]string, 0, len(l.loadedPlugins))
	for path := range l.loadedPlugins {
		name := filepath.Base(path)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		names = append(names, name)
	}
	l.mu.RUnlock()

	var errors []error
	for _, name := range names {
		if err := l.UnloadPlugin(ctx, name); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some plugins failed to unload: %v", errors)
	}

	log.Info().Int("count", len(names)).Msg("All plugins unloaded")
	return nil
}

// GetLoadedCount restituisce il numero di plugin caricati
func (l *Loader) GetLoadedCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.loadedPlugins)
}
