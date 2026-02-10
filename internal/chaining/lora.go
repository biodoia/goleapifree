package chaining

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// LoRAAdapter rappresenta un adapter LoRA
type LoRAAdapter struct {
	ID          string
	Name        string
	Description string
	BaseModel   string                 // Modello base su cui è trainato
	Task        string                 // Task per cui è ottimizzato (es. "code", "math", "translation")
	Path        string                 // Path al file adapter
	Metadata    map[string]interface{} // Metadati aggiuntivi
	LoadedAt    time.Time
	LastUsedAt  time.Time
	UseCount    int64
	SizeBytes   int64
}

// LoRARegistry gestisce gli adapter LoRA disponibili
type LoRARegistry struct {
	adapters map[string]*LoRAAdapter
	mu       sync.RWMutex
}

// NewLoRARegistry crea un nuovo registry di adapter
func NewLoRARegistry() *LoRARegistry {
	return &LoRARegistry{
		adapters: make(map[string]*LoRAAdapter),
	}
}

// Register registra un nuovo adapter
func (r *LoRARegistry) Register(adapter *LoRAAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if adapter.ID == "" {
		return fmt.Errorf("adapter ID cannot be empty")
	}

	if _, exists := r.adapters[adapter.ID]; exists {
		return fmt.Errorf("adapter %s already registered", adapter.ID)
	}

	adapter.LoadedAt = time.Now()
	r.adapters[adapter.ID] = adapter

	log.Info().
		Str("adapter_id", adapter.ID).
		Str("name", adapter.Name).
		Str("base_model", adapter.BaseModel).
		Str("task", adapter.Task).
		Msg("LoRA adapter registered")

	return nil
}

// Get recupera un adapter per ID
func (r *LoRARegistry) Get(id string) (*LoRAAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, exists := r.adapters[id]
	if !exists {
		return nil, fmt.Errorf("adapter %s not found", id)
	}

	return adapter, nil
}

// GetByTask recupera adapter per task
func (r *LoRARegistry) GetByTask(task string) []*LoRAAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*LoRAAdapter
	for _, adapter := range r.adapters {
		if adapter.Task == task {
			result = append(result, adapter)
		}
	}

	return result
}

// GetByBaseModel recupera adapter per modello base
func (r *LoRARegistry) GetByBaseModel(baseModel string) []*LoRAAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*LoRAAdapter
	for _, adapter := range r.adapters {
		if adapter.BaseModel == baseModel {
			result = append(result, adapter)
		}
	}

	return result
}

// List restituisce tutti gli adapter
func (r *LoRARegistry) List() []*LoRAAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*LoRAAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		result = append(result, adapter)
	}

	return result
}

// Unregister rimuove un adapter
func (r *LoRARegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[id]; !exists {
		return fmt.Errorf("adapter %s not found", id)
	}

	delete(r.adapters, id)

	log.Info().
		Str("adapter_id", id).
		Msg("LoRA adapter unregistered")

	return nil
}

// MarkUsed aggiorna l'ultimo utilizzo di un adapter
func (r *LoRARegistry) MarkUsed(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if adapter, exists := r.adapters[id]; exists {
		adapter.LastUsedAt = time.Now()
		adapter.UseCount++
	}
}

// LoRAPool gestisce il caricamento dinamico degli adapter
type LoRAPool struct {
	registry      *LoRARegistry
	loaded        map[string]*LoadedAdapter
	maxLoaded     int   // Massimo numero di adapter caricati contemporaneamente
	maxMemoryMB   int64 // Massima memoria utilizzabile (MB)
	currentMemory int64 // Memoria attualmente utilizzata (MB)
	mu            sync.RWMutex
}

// LoadedAdapter rappresenta un adapter caricato in memoria
type LoadedAdapter struct {
	Adapter    *LoRAAdapter
	LoadedAt   time.Time
	LastUsedAt time.Time
	UseCount   int64
	Data       interface{} // Dati dell'adapter (implementation-specific)
}

// NewLoRAPool crea un nuovo pool di adapter
func NewLoRAPool(registry *LoRARegistry, maxLoaded int, maxMemoryMB int64) *LoRAPool {
	return &LoRAPool{
		registry:    registry,
		loaded:      make(map[string]*LoadedAdapter),
		maxLoaded:   maxLoaded,
		maxMemoryMB: maxMemoryMB,
	}
}

// Load carica un adapter in memoria
func (p *LoRAPool) Load(ctx context.Context, adapterID string) (*LoadedAdapter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Se già caricato, restituisci l'adapter esistente
	if loaded, exists := p.loaded[adapterID]; exists {
		loaded.LastUsedAt = time.Now()
		loaded.UseCount++
		p.registry.MarkUsed(adapterID)
		return loaded, nil
	}

	// Recupera adapter dal registry
	adapter, err := p.registry.Get(adapterID)
	if err != nil {
		return nil, err
	}

	// Verifica limiti di memoria
	adapterSizeMB := adapter.SizeBytes / (1024 * 1024)
	if p.currentMemory+adapterSizeMB > p.maxMemoryMB {
		// Evict adapter meno utilizzati
		if err := p.evictLRU(adapterSizeMB); err != nil {
			return nil, fmt.Errorf("failed to free memory for adapter: %w", err)
		}
	}

	// Verifica numero massimo di adapter
	if len(p.loaded) >= p.maxLoaded {
		// Evict adapter meno recente
		if err := p.evictOldest(); err != nil {
			return nil, fmt.Errorf("failed to evict adapter: %w", err)
		}
	}

	// Carica adapter (placeholder - la vera implementazione dipenderebbe dal backend)
	loaded := &LoadedAdapter{
		Adapter:    adapter,
		LoadedAt:   time.Now(),
		LastUsedAt: time.Now(),
		UseCount:   1,
		Data:       nil, // TODO: Load actual adapter data
	}

	p.loaded[adapterID] = loaded
	p.currentMemory += adapterSizeMB
	p.registry.MarkUsed(adapterID)

	log.Info().
		Str("adapter_id", adapterID).
		Int64("size_mb", adapterSizeMB).
		Int64("total_memory_mb", p.currentMemory).
		Msg("LoRA adapter loaded")

	return loaded, nil
}

// Unload scarica un adapter dalla memoria
func (p *LoRAPool) Unload(adapterID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	loaded, exists := p.loaded[adapterID]
	if !exists {
		return fmt.Errorf("adapter %s not loaded", adapterID)
	}

	adapterSizeMB := loaded.Adapter.SizeBytes / (1024 * 1024)
	delete(p.loaded, adapterID)
	p.currentMemory -= adapterSizeMB

	log.Info().
		Str("adapter_id", adapterID).
		Int64("freed_mb", adapterSizeMB).
		Int64("total_memory_mb", p.currentMemory).
		Msg("LoRA adapter unloaded")

	return nil
}

// Get recupera un adapter caricato
func (p *LoRAPool) Get(adapterID string) (*LoadedAdapter, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	loaded, exists := p.loaded[adapterID]
	if exists {
		loaded.LastUsedAt = time.Now()
		loaded.UseCount++
	}

	return loaded, exists
}

// evictLRU rimuove gli adapter meno recentemente usati per liberare memoria
func (p *LoRAPool) evictLRU(requiredMB int64) error {
	// Trova adapter meno recentemente usati
	type adapterAge struct {
		id         string
		lastUsed   time.Time
		sizeMB     int64
	}

	var candidates []adapterAge
	for id, loaded := range p.loaded {
		sizeMB := loaded.Adapter.SizeBytes / (1024 * 1024)
		candidates = append(candidates, adapterAge{
			id:       id,
			lastUsed: loaded.LastUsedAt,
			sizeMB:   sizeMB,
		})
	}

	// Ordina per ultimo utilizzo
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].lastUsed.After(candidates[j].lastUsed) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Rimuovi adapter finché non abbiamo abbastanza memoria
	freedMB := int64(0)
	for _, candidate := range candidates {
		if freedMB >= requiredMB {
			break
		}

		if err := p.Unload(candidate.id); err == nil {
			freedMB += candidate.sizeMB
		}
	}

	if freedMB < requiredMB {
		return fmt.Errorf("could not free enough memory: freed %d MB, required %d MB", freedMB, requiredMB)
	}

	return nil
}

// evictOldest rimuove l'adapter caricato da più tempo
func (p *LoRAPool) evictOldest() error {
	var oldestID string
	var oldestTime time.Time

	for id, loaded := range p.loaded {
		if oldestID == "" || loaded.LoadedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = loaded.LoadedAt
		}
	}

	if oldestID == "" {
		return fmt.Errorf("no adapter to evict")
	}

	return p.Unload(oldestID)
}

// GetStats restituisce statistiche sul pool
func (p *LoRAPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStats{
		LoadedCount:   len(p.loaded),
		MaxLoaded:     p.maxLoaded,
		CurrentMemory: p.currentMemory,
		MaxMemory:     p.maxMemoryMB,
		Adapters:      p.getAdapterStats(),
	}
}

func (p *LoRAPool) getAdapterStats() []AdapterStats {
	stats := make([]AdapterStats, 0, len(p.loaded))

	for id, loaded := range p.loaded {
		stats = append(stats, AdapterStats{
			ID:         id,
			Name:       loaded.Adapter.Name,
			LoadedAt:   loaded.LoadedAt,
			LastUsedAt: loaded.LastUsedAt,
			UseCount:   loaded.UseCount,
			SizeMB:     loaded.Adapter.SizeBytes / (1024 * 1024),
		})
	}

	return stats
}

// PoolStats rappresenta le statistiche del pool
type PoolStats struct {
	LoadedCount   int
	MaxLoaded     int
	CurrentMemory int64
	MaxMemory     int64
	Adapters      []AdapterStats
}

// AdapterStats rappresenta le statistiche di un adapter
type AdapterStats struct {
	ID         string
	Name       string
	LoadedAt   time.Time
	LastUsedAt time.Time
	UseCount   int64
	SizeMB     int64
}

// LoRAManager coordina registry e pool
type LoRAManager struct {
	registry *LoRARegistry
	pool     *LoRAPool
}

// NewLoRAManager crea un nuovo manager
func NewLoRAManager(maxLoaded int, maxMemoryMB int64) *LoRAManager {
	registry := NewLoRARegistry()
	pool := NewLoRAPool(registry, maxLoaded, maxMemoryMB)

	return &LoRAManager{
		registry: registry,
		pool:     pool,
	}
}

// RegisterAdapter registra un nuovo adapter
func (m *LoRAManager) RegisterAdapter(adapter *LoRAAdapter) error {
	return m.registry.Register(adapter)
}

// LoadAdapter carica un adapter per l'uso
func (m *LoRAManager) LoadAdapter(ctx context.Context, adapterID string) (*LoadedAdapter, error) {
	return m.pool.Load(ctx, adapterID)
}

// UnloadAdapter scarica un adapter
func (m *LoRAManager) UnloadAdapter(adapterID string) error {
	return m.pool.Unload(adapterID)
}

// GetAdapter recupera un adapter caricato
func (m *LoRAManager) GetAdapter(adapterID string) (*LoadedAdapter, bool) {
	return m.pool.Get(adapterID)
}

// ListAdapters lista tutti gli adapter disponibili
func (m *LoRAManager) ListAdapters() []*LoRAAdapter {
	return m.registry.List()
}

// GetAdaptersByTask recupera adapter per task
func (m *LoRAManager) GetAdaptersByTask(task string) []*LoRAAdapter {
	return m.registry.GetByTask(task)
}

// GetStats restituisce statistiche del pool
func (m *LoRAManager) GetStats() PoolStats {
	return m.pool.GetStats()
}

// AutoSelectAdapter seleziona automaticamente l'adapter migliore per un task
func (m *LoRAManager) AutoSelectAdapter(ctx context.Context, task string, baseModel string) (*LoadedAdapter, error) {
	// Cerca adapter per task e modello base
	adapters := m.registry.GetByTask(task)
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no adapter found for task: %s", task)
	}

	// Filtra per modello base
	var compatible []*LoRAAdapter
	for _, adapter := range adapters {
		if adapter.BaseModel == baseModel {
			compatible = append(compatible, adapter)
		}
	}

	if len(compatible) == 0 {
		return nil, fmt.Errorf("no compatible adapter found for model: %s", baseModel)
	}

	// Seleziona l'adapter più utilizzato (euristica semplice)
	bestAdapter := compatible[0]
	for _, adapter := range compatible[1:] {
		if adapter.UseCount > bestAdapter.UseCount {
			bestAdapter = adapter
		}
	}

	// Carica l'adapter selezionato
	return m.pool.Load(ctx, bestAdapter.ID)
}
