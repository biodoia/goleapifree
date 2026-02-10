package cache

import (
	"context"
	"testing"
	"time"
)

func TestCacheStats_HitRate(t *testing.T) {
	tests := []struct {
		name  string
		stats CacheStats
		want  float64
	}{
		{
			name: "50% hit rate",
			stats: CacheStats{
				Hits:   50,
				Misses: 50,
			},
			want: 0.5,
		},
		{
			name: "100% hit rate",
			stats: CacheStats{
				Hits:   100,
				Misses: 0,
			},
			want: 1.0,
		},
		{
			name: "0% hit rate",
			stats: CacheStats{
				Hits:   0,
				Misses: 100,
			},
			want: 0.0,
		},
		{
			name: "no requests",
			stats: CacheStats{
				Hits:   0,
				Misses: 0,
			},
			want: 0.0,
		},
		{
			name: "high hit rate",
			stats: CacheStats{
				Hits:   95,
				Misses: 5,
			},
			want: 0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.HitRate()
			if got != tt.want {
				t.Errorf("HitRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if !cfg.MemoryEnabled {
		t.Error("Memory cache should be enabled by default")
	}

	if cfg.MemoryMaxSize <= 0 {
		t.Error("MemoryMaxSize should be positive")
	}

	if cfg.MemoryMaxEntries <= 0 {
		t.Error("MemoryMaxEntries should be positive")
	}

	if cfg.MemoryTTL <= 0 {
		t.Error("MemoryTTL should be positive")
	}

	if cfg.RedisEnabled {
		t.Error("Redis should be disabled by default")
	}

	if !cfg.LRUEnabled {
		t.Error("LRU should be enabled by default")
	}
}

func TestMultiLayerCache_New(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "default config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "custom config",
			config: &Config{
				MemoryEnabled:    true,
				MemoryMaxSize:    50 * 1024 * 1024,
				MemoryMaxEntries: 5000,
				MemoryTTL:        10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "memory only",
			config: &Config{
				MemoryEnabled: true,
				RedisEnabled:  false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewMultiLayerCache(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMultiLayerCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if cache == nil && !tt.wantErr {
				t.Error("NewMultiLayerCache() returned nil cache")
			}

			if cache != nil {
				defer cache.Close()
			}
		})
	}
}

func TestMultiLayerCache_SetGet(t *testing.T) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		t.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "test-key"
	value := []byte("test-value")
	ttl := 5 * time.Minute

	// Test Set
	if err := cache.Set(ctx, key, value, ttl); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	// Test Get
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get() = %s, want %s", got, value)
	}

	// Check stats
	stats := cache.Stats()
	if stats.Sets != 1 {
		t.Errorf("Stats.Sets = %d, want 1", stats.Sets)
	}

	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
}

func TestMultiLayerCache_Miss(t *testing.T) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		t.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "nonexistent-key"

	_, err = cache.Get(ctx, key)
	if err != ErrCacheMiss {
		t.Errorf("Get() error = %v, want %v", err, ErrCacheMiss)
	}

	// Check stats
	stats := cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
}

func TestMultiLayerCache_Delete(t *testing.T) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		t.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "test-key"
	value := []byte("test-value")

	// Set value
	if err := cache.Set(ctx, key, value, 5*time.Minute); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Delete value
	if err := cache.Delete(ctx, key); err != nil {
		t.Errorf("Delete() failed: %v", err)
	}

	// Verify deletion
	_, err = cache.Get(ctx, key)
	if err != ErrCacheMiss {
		t.Errorf("After Delete(), Get() error = %v, want %v", err, ErrCacheMiss)
	}

	// Check stats
	stats := cache.Stats()
	if stats.Deletes != 1 {
		t.Errorf("Stats.Deletes = %d, want 1", stats.Deletes)
	}
}

func TestMultiLayerCache_Clear(t *testing.T) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		t.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set multiple values
	for i := 0; i < 10; i++ {
		key := "test-key-" + string(rune(i))
		value := []byte("test-value")
		if err := cache.Set(ctx, key, value, 5*time.Minute); err != nil {
			t.Fatalf("Set() failed: %v", err)
		}
	}

	// Clear cache
	if err := cache.Clear(ctx); err != nil {
		t.Errorf("Clear() failed: %v", err)
	}

	// Verify all keys are gone
	for i := 0; i < 10; i++ {
		key := "test-key-" + string(rune(i))
		_, err := cache.Get(ctx, key)
		if err != ErrCacheMiss {
			t.Errorf("After Clear(), Get() error = %v, want %v", err, ErrCacheMiss)
		}
	}
}

func TestHashKey(t *testing.T) {
	tests := []struct {
		name  string
		parts []interface{}
	}{
		{
			name:  "single string",
			parts: []interface{}{"key1"},
		},
		{
			name:  "multiple strings",
			parts: []interface{}{"key1", "key2", "key3"},
		},
		{
			name:  "mixed types",
			parts: []interface{}{"key1", 123, true, 45.67},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashKey(tt.parts...)
			hash2 := HashKey(tt.parts...)

			if hash1 != hash2 {
				t.Error("HashKey should be deterministic")
			}

			if len(hash1) != 64 { // SHA256 produces 64 hex chars
				t.Errorf("HashKey length = %d, want 64", len(hash1))
			}
		})
	}

	// Test different inputs produce different hashes
	hash1 := HashKey("key1")
	hash2 := HashKey("key2")
	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		entry *CacheEntry
		want  bool
	}{
		{
			name: "not expired",
			entry: &CacheEntry{
				ExpiresAt: now.Add(1 * time.Hour),
			},
			want: false,
		},
		{
			name: "expired",
			entry: &CacheEntry{
				ExpiresAt: now.Add(-1 * time.Hour),
			},
			want: true,
		},
		{
			name: "just expired",
			entry: &CacheEntry{
				ExpiresAt: now.Add(-1 * time.Second),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheEntry_TTL(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		entry     *CacheEntry
		wantZero  bool
	}{
		{
			name: "has TTL",
			entry: &CacheEntry{
				ExpiresAt: now.Add(1 * time.Hour),
			},
			wantZero: false,
		},
		{
			name: "expired",
			entry: &CacheEntry{
				ExpiresAt: now.Add(-1 * time.Hour),
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := tt.entry.TTL()
			if (ttl == 0) != tt.wantZero {
				t.Errorf("TTL() = %v, wantZero %v", ttl, tt.wantZero)
			}
		})
	}
}

// Benchmark tests
func BenchmarkMultiLayerCache_Set(b *testing.B) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		b.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench-key"
		_ = cache.Set(ctx, key, value, 5*time.Minute)
	}
}

func BenchmarkMultiLayerCache_Get(b *testing.B) {
	cache, err := NewMultiLayerCache(DefaultConfig())
	if err != nil {
		b.Fatalf("NewMultiLayerCache() failed: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "bench-key"
	value := []byte("benchmark-value")

	_ = cache.Set(ctx, key, value, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, key)
	}
}

func BenchmarkHashKey(b *testing.B) {
	parts := []interface{}{"key1", "key2", 123, true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashKey(parts...)
	}
}
