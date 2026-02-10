package chaining

import (
	"context"
	"testing"
	"time"
)

// TestLoRARegistry testa il registry degli adapter
func TestLoRARegistry(t *testing.T) {
	registry := NewLoRARegistry()

	// Crea adapter di test
	adapter := &LoRAAdapter{
		ID:          "test-adapter-1",
		Name:        "Test Adapter",
		Description: "Test description",
		BaseModel:   "llama-3.1-70b",
		Task:        "code",
		Path:        "/models/test.safetensors",
		SizeBytes:   100 * 1024 * 1024, // 100MB
		Metadata: map[string]interface{}{
			"version": "1.0",
		},
	}

	// Test Register
	err := registry.Register(adapter)
	if err != nil {
		t.Fatalf("Failed to register adapter: %v", err)
	}

	// Test Get
	retrieved, err := registry.Get("test-adapter-1")
	if err != nil {
		t.Fatalf("Failed to get adapter: %v", err)
	}

	if retrieved.ID != adapter.ID {
		t.Errorf("Expected ID %s, got %s", adapter.ID, retrieved.ID)
	}

	// Test duplicate registration
	err = registry.Register(adapter)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Test GetByTask
	adapters := registry.GetByTask("code")
	if len(adapters) != 1 {
		t.Errorf("Expected 1 adapter for task 'code', got %d", len(adapters))
	}

	// Test GetByBaseModel
	adapters = registry.GetByBaseModel("llama-3.1-70b")
	if len(adapters) != 1 {
		t.Errorf("Expected 1 adapter for base model, got %d", len(adapters))
	}

	// Test List
	allAdapters := registry.List()
	if len(allAdapters) != 1 {
		t.Errorf("Expected 1 total adapter, got %d", len(allAdapters))
	}

	// Test Unregister
	err = registry.Unregister("test-adapter-1")
	if err != nil {
		t.Fatalf("Failed to unregister adapter: %v", err)
	}

	// Verifica rimozione
	_, err = registry.Get("test-adapter-1")
	if err == nil {
		t.Error("Expected error for unregistered adapter")
	}

	t.Log("LoRARegistry tests passed")
}

// TestLoRAPool testa il pool di adapter
func TestLoRAPool(t *testing.T) {
	registry := NewLoRARegistry()
	pool := NewLoRAPool(registry, 2, 200) // Max 2 adapter, 200MB

	// Registra adapter di test
	adapter1 := &LoRAAdapter{
		ID:        "adapter-1",
		Name:      "Adapter 1",
		BaseModel: "llama-3.1-70b",
		Task:      "code",
		Path:      "/models/adapter1.safetensors",
		SizeBytes: 100 * 1024 * 1024, // 100MB
	}

	adapter2 := &LoRAAdapter{
		ID:        "adapter-2",
		Name:      "Adapter 2",
		BaseModel: "llama-3.1-70b",
		Task:      "math",
		Path:      "/models/adapter2.safetensors",
		SizeBytes: 80 * 1024 * 1024, // 80MB
	}

	adapter3 := &LoRAAdapter{
		ID:        "adapter-3",
		Name:      "Adapter 3",
		BaseModel: "llama-3.1-8b",
		Task:      "translation",
		Path:      "/models/adapter3.safetensors",
		SizeBytes: 60 * 1024 * 1024, // 60MB
	}

	registry.Register(adapter1)
	registry.Register(adapter2)
	registry.Register(adapter3)

	ctx := context.Background()

	// Test Load
	loaded1, err := pool.Load(ctx, "adapter-1")
	if err != nil {
		t.Fatalf("Failed to load adapter-1: %v", err)
	}

	if loaded1.Adapter.ID != "adapter-1" {
		t.Errorf("Expected adapter-1, got %s", loaded1.Adapter.ID)
	}

	// Verifica statistiche
	stats := pool.GetStats()
	if stats.LoadedCount != 1 {
		t.Errorf("Expected 1 loaded adapter, got %d", stats.LoadedCount)
	}

	if stats.CurrentMemory != 100 {
		t.Errorf("Expected 100MB memory, got %d", stats.CurrentMemory)
	}

	// Load secondo adapter
	loaded2, err := pool.Load(ctx, "adapter-2")
	if err != nil {
		t.Fatalf("Failed to load adapter-2: %v", err)
	}

	stats = pool.GetStats()
	if stats.LoadedCount != 2 {
		t.Errorf("Expected 2 loaded adapters, got %d", stats.LoadedCount)
	}

	if stats.CurrentMemory != 180 {
		t.Errorf("Expected 180MB memory, got %d", stats.CurrentMemory)
	}

	// Sleep per differenziare i timestamp
	time.Sleep(10 * time.Millisecond)

	// Load terzo adapter - dovrebbe evictare il primo (LRU)
	_, err = pool.Load(ctx, "adapter-3")
	if err != nil {
		t.Fatalf("Failed to load adapter-3: %v", err)
	}

	stats = pool.GetStats()

	// Ancora 2 adapter (max)
	if stats.LoadedCount != 2 {
		t.Errorf("Expected 2 loaded adapters after eviction, got %d", stats.LoadedCount)
	}

	// Verifica che adapter-1 sia stato evicted
	_, exists := pool.Get("adapter-1")
	if exists {
		t.Error("adapter-1 should have been evicted")
	}

	// Verifica che adapter-2 e adapter-3 siano presenti
	_, exists = pool.Get("adapter-2")
	if !exists {
		t.Error("adapter-2 should still be loaded")
	}

	_, exists = pool.Get("adapter-3")
	if !exists {
		t.Error("adapter-3 should be loaded")
	}

	// Test Unload
	err = pool.Unload("adapter-2")
	if err != nil {
		t.Fatalf("Failed to unload adapter-2: %v", err)
	}

	stats = pool.GetStats()
	if stats.LoadedCount != 1 {
		t.Errorf("Expected 1 loaded adapter after unload, got %d", stats.LoadedCount)
	}

	// Test doppio Load (dovrebbe usare cache)
	loaded3a, _ := pool.Load(ctx, "adapter-3")
	loaded3b, _ := pool.Load(ctx, "adapter-3")

	if loaded3a.UseCount+1 != loaded3b.UseCount {
		t.Error("Second load should increment use count")
	}

	t.Log("LoRAPool tests passed")
}

// TestLoRAManager testa il manager completo
func TestLoRAManager(t *testing.T) {
	manager := NewLoRAManager(3, 500) // Max 3 adapter, 500MB

	// Registra adapter
	codeAdapter := &LoRAAdapter{
		ID:        "code-adapter",
		Name:      "Code Specialist",
		BaseModel: "llama-3.1-70b",
		Task:      "code",
		Path:      "/models/code.safetensors",
		SizeBytes: 150 * 1024 * 1024,
	}

	mathAdapter := &LoRAAdapter{
		ID:        "math-adapter",
		Name:      "Math Specialist",
		BaseModel: "llama-3.1-70b",
		Task:      "math",
		Path:      "/models/math.safetensors",
		SizeBytes: 120 * 1024 * 1024,
	}

	err := manager.RegisterAdapter(codeAdapter)
	if err != nil {
		t.Fatalf("Failed to register code adapter: %v", err)
	}

	err = manager.RegisterAdapter(mathAdapter)
	if err != nil {
		t.Fatalf("Failed to register math adapter: %v", err)
	}

	// Test ListAdapters
	adapters := manager.ListAdapters()
	if len(adapters) != 2 {
		t.Errorf("Expected 2 adapters, got %d", len(adapters))
	}

	// Test GetAdaptersByTask
	codeAdapters := manager.GetAdaptersByTask("code")
	if len(codeAdapters) != 1 {
		t.Errorf("Expected 1 code adapter, got %d", len(codeAdapters))
	}

	// Test LoadAdapter
	ctx := context.Background()
	loaded, err := manager.LoadAdapter(ctx, "code-adapter")
	if err != nil {
		t.Fatalf("Failed to load adapter: %v", err)
	}

	if loaded.Adapter.ID != "code-adapter" {
		t.Errorf("Expected code-adapter, got %s", loaded.Adapter.ID)
	}

	// Test GetAdapter
	retrieved, exists := manager.GetAdapter("code-adapter")
	if !exists {
		t.Error("Adapter should be loaded")
	}

	if retrieved.UseCount < 1 {
		t.Error("Use count should be at least 1")
	}

	// Test GetStats
	stats := manager.GetStats()
	if stats.LoadedCount != 1 {
		t.Errorf("Expected 1 loaded adapter, got %d", stats.LoadedCount)
	}

	t.Log("LoRAManager tests passed")
}

// TestAutoSelectAdapter testa la selezione automatica
func TestAutoSelectAdapter(t *testing.T) {
	manager := NewLoRAManager(5, 1000)

	// Registra adapter per diversi task
	adapters := []*LoRAAdapter{
		{
			ID:        "code-1",
			Name:      "Code Adapter 1",
			BaseModel: "llama-3.1-70b",
			Task:      "code",
			Path:      "/models/code1.safetensors",
			SizeBytes: 100 * 1024 * 1024,
			UseCount:  10, // Più usato
		},
		{
			ID:        "code-2",
			Name:      "Code Adapter 2",
			BaseModel: "llama-3.1-70b",
			Task:      "code",
			Path:      "/models/code2.safetensors",
			SizeBytes: 100 * 1024 * 1024,
			UseCount:  5, // Meno usato
		},
		{
			ID:        "math-1",
			Name:      "Math Adapter",
			BaseModel: "llama-3.1-70b",
			Task:      "math",
			Path:      "/models/math1.safetensors",
			SizeBytes: 100 * 1024 * 1024,
			UseCount:  3,
		},
	}

	for _, adapter := range adapters {
		if err := manager.RegisterAdapter(adapter); err != nil {
			t.Fatalf("Failed to register adapter %s: %v", adapter.ID, err)
		}
	}

	ctx := context.Background()

	// Test auto-select per code task
	selected, err := manager.AutoSelectAdapter(ctx, "code", "llama-3.1-70b")
	if err != nil {
		t.Fatalf("Failed to auto-select adapter: %v", err)
	}

	// Dovrebbe selezionare code-1 (più usato)
	if selected.Adapter.ID != "code-1" {
		t.Errorf("Expected code-1 (most used), got %s", selected.Adapter.ID)
	}

	// Test auto-select per task inesistente
	_, err = manager.AutoSelectAdapter(ctx, "translation", "llama-3.1-70b")
	if err == nil {
		t.Error("Expected error for non-existent task")
	}

	// Test auto-select per base model incompatibile
	_, err = manager.AutoSelectAdapter(ctx, "code", "gpt-4")
	if err == nil {
		t.Error("Expected error for incompatible base model")
	}

	t.Log("AutoSelectAdapter tests passed")
}

// TestLoRAPoolMemoryManagement testa la gestione della memoria
func TestLoRAPoolMemoryManagement(t *testing.T) {
	registry := NewLoRARegistry()
	pool := NewLoRAPool(registry, 10, 200) // Max 200MB

	// Registra adapter che superano il limite di memoria
	for i := 0; i < 5; i++ {
		adapter := &LoRAAdapter{
			ID:        string(rune('a' + i)),
			Name:      "Adapter " + string(rune('A'+i)),
			BaseModel: "test-model",
			Task:      "test",
			Path:      "/models/test.safetensors",
			SizeBytes: 80 * 1024 * 1024, // 80MB ciascuno
		}
		registry.Register(adapter)
	}

	ctx := context.Background()

	// Load primo adapter
	pool.Load(ctx, "a")
	time.Sleep(5 * time.Millisecond)

	// Load secondo adapter
	pool.Load(ctx, "b")
	time.Sleep(5 * time.Millisecond)

	stats := pool.GetStats()
	if stats.CurrentMemory != 160 {
		t.Errorf("Expected 160MB, got %d", stats.CurrentMemory)
	}

	// Load terzo adapter - dovrebbe evictare il primo
	pool.Load(ctx, "c")

	stats = pool.GetStats()

	// Memoria dovrebbe essere circa 160MB (b + c)
	if stats.CurrentMemory > 200 {
		t.Errorf("Memory limit exceeded: %dMB", stats.CurrentMemory)
	}

	// Adapter 'a' dovrebbe essere stato evicted
	_, exists := pool.Get("a")
	if exists {
		t.Error("Adapter 'a' should have been evicted")
	}

	t.Log("Memory management tests passed")
}
