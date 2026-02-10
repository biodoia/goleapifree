package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"

	"github.com/rs/zerolog/log"
)

// Profiler gestisce il profiling delle performance
type Profiler struct {
	outputDir string
}

// NewProfiler crea un nuovo profiler
func NewProfiler(outputDir string) *Profiler {
	if outputDir == "" {
		outputDir = "./profiles"
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create profile directory")
	}

	return &Profiler{
		outputDir: outputDir,
	}
}

// runCPUProfile esegue il profiling della CPU
func runCPUProfile(duration int) error {
	profiler := NewProfiler("")

	fmt.Printf("Starting CPU profile for %d seconds...\n", duration)

	filename := filepath.Join(profiler.outputDir, fmt.Sprintf("cpu_%s.prof", time.Now().Format("20060102_150405")))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CPU profile: %w", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	defer pprof.StopCPUProfile()

	// Profile for specified duration
	fmt.Printf("Profiling CPU usage...\n")
	fmt.Printf("(Application should be running and handling requests)\n\n")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; i < duration; i++ {
		<-ticker.C
		progress := float64(i+1) / float64(duration) * 100
		bar := makeProgressBar(progress, 40)
		fmt.Printf("\rProgress: %s %.0f%%", bar, progress)
	}

	fmt.Printf("\n\n✓ CPU profile saved to: %s\n", filename)
	fmt.Println("\nAnalyze with:")
	fmt.Printf("  go tool pprof %s\n", filename)
	fmt.Printf("  go tool pprof -http=:8080 %s\n", filename)

	return nil
}

// runMemoryProfile esegue il profiling della memoria
func runMemoryProfile() error {
	profiler := NewProfiler("")

	fmt.Println("Generating memory profile...")

	filename := filepath.Join(profiler.outputDir, fmt.Sprintf("mem_%s.prof", time.Now().Format("20060102_150405")))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create memory profile: %w", err)
	}
	defer f.Close()

	// Force GC to get accurate stats
	runtime.GC()

	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("failed to write heap profile: %w", err)
	}

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("\n✓ Memory profile saved to: %s\n\n", filename)

	// Display memory statistics
	fmt.Println("Memory Statistics:")
	fmt.Println("━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Allocated:        %s\n", formatBytes(m.Alloc))
	fmt.Printf("  Total Allocated:  %s\n", formatBytes(m.TotalAlloc))
	fmt.Printf("  System:           %s\n", formatBytes(m.Sys))
	fmt.Printf("  Lookups:          %d\n", m.Lookups)
	fmt.Printf("  Mallocs:          %d\n", m.Mallocs)
	fmt.Printf("  Frees:            %d\n", m.Frees)
	fmt.Println()
	fmt.Println("Heap Statistics:")
	fmt.Printf("  Heap Allocated:   %s\n", formatBytes(m.HeapAlloc))
	fmt.Printf("  Heap System:      %s\n", formatBytes(m.HeapSys))
	fmt.Printf("  Heap Idle:        %s\n", formatBytes(m.HeapIdle))
	fmt.Printf("  Heap In Use:      %s\n", formatBytes(m.HeapInuse))
	fmt.Printf("  Heap Released:    %s\n", formatBytes(m.HeapReleased))
	fmt.Printf("  Heap Objects:     %d\n", m.HeapObjects)
	fmt.Println()
	fmt.Println("GC Statistics:")
	fmt.Printf("  GC Runs:          %d\n", m.NumGC)
	fmt.Printf("  Last GC:          %s\n", time.Unix(0, int64(m.LastGC)).Format(time.RFC3339))
	fmt.Printf("  GC CPU Fraction:  %.4f\n", m.GCCPUFraction)
	fmt.Printf("  Pause Total:      %s\n", time.Duration(m.PauseTotalNs))

	fmt.Println("\nAnalyze with:")
	fmt.Printf("  go tool pprof %s\n", filename)
	fmt.Printf("  go tool pprof -http=:8080 %s\n", filename)

	return nil
}

// runGoroutineProfile esegue l'analisi delle goroutine
func runGoroutineProfile() error {
	profiler := NewProfiler("")

	fmt.Println("Analyzing goroutines...")

	filename := filepath.Join(profiler.outputDir, fmt.Sprintf("goroutine_%s.prof", time.Now().Format("20060102_150405")))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create goroutine profile: %w", err)
	}
	defer f.Close()

	if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write goroutine profile: %w", err)
	}

	// Display goroutine statistics
	numGoroutines := runtime.NumGoroutine()

	fmt.Printf("\n✓ Goroutine profile saved to: %s\n\n", filename)

	fmt.Println("Goroutine Statistics:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Active Goroutines: %d\n", numGoroutines)
	fmt.Printf("  GOMAXPROCS:        %d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("  CPUs:              %d\n", runtime.NumCPU())

	// Show goroutine breakdown
	fmt.Println("\nGoroutine Breakdown:")
	profiles := pprof.Profiles()
	for _, p := range profiles {
		if p.Name() == "goroutine" {
			fmt.Printf("  Total: %d goroutines\n", p.Count())
		}
	}

	// Warning for high goroutine count
	if numGoroutines > 10000 {
		fmt.Println("\n⚠️  WARNING: High number of goroutines detected!")
		fmt.Println("   This may indicate a goroutine leak.")
	} else if numGoroutines > 1000 {
		fmt.Println("\n⚠️  NOTICE: Elevated goroutine count")
	} else {
		fmt.Println("\n✓ Goroutine count is normal")
	}

	fmt.Println("\nAnalyze with:")
	fmt.Printf("  go tool pprof %s\n", filename)
	fmt.Printf("  go tool pprof -http=:8080 %s\n", filename)

	return nil
}

// runHeapDump esegue un dump completo dell'heap
func runHeapDump() error {
	profiler := NewProfiler("")

	fmt.Println("Generating heap dump...")

	filename := filepath.Join(profiler.outputDir, fmt.Sprintf("heap_%s.prof", time.Now().Format("20060102_150405")))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create heap dump: %w", err)
	}
	defer f.Close()

	// Force GC before dump
	fmt.Println("Running garbage collection...")
	runtime.GC()

	fmt.Println("Writing heap dump...")
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("failed to write heap dump: %w", err)
	}

	// Get detailed memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("\n✓ Heap dump saved to: %s\n\n", filename)

	// Display detailed heap analysis
	fmt.Println("Heap Analysis:")
	fmt.Println("━━━━━━━━━━━━━━")

	fmt.Println("\nMemory Usage:")
	fmt.Printf("  %-25s %15s\n", "Metric", "Value")
	fmt.Println("  " + repeatString("─", 42))
	fmt.Printf("  %-25s %15s\n", "Heap Allocated", formatBytes(m.HeapAlloc))
	fmt.Printf("  %-25s %15s\n", "Heap In Use", formatBytes(m.HeapInuse))
	fmt.Printf("  %-25s %15s\n", "Heap Idle", formatBytes(m.HeapIdle))
	fmt.Printf("  %-25s %15s\n", "Heap Released", formatBytes(m.HeapReleased))
	fmt.Printf("  %-25s %15d\n", "Heap Objects", m.HeapObjects)

	fmt.Println("\nMemory Pools:")
	fmt.Printf("  %-25s %15s\n", "Stack In Use", formatBytes(m.StackInuse))
	fmt.Printf("  %-25s %15s\n", "Stack System", formatBytes(m.StackSys))
	fmt.Printf("  %-25s %15s\n", "MSpan In Use", formatBytes(m.MSpanInuse))
	fmt.Printf("  %-25s %15s\n", "MCache In Use", formatBytes(m.MCacheInuse))

	fmt.Println("\nGarbage Collection:")
	fmt.Printf("  %-25s %15d\n", "GC Cycles", m.NumGC)
	fmt.Printf("  %-25s %15d\n", "Forced GC", m.NumForcedGC)
	fmt.Printf("  %-25s %15.4f%%\n", "GC CPU Usage", m.GCCPUFraction*100)

	if m.NumGC > 0 {
		fmt.Printf("  %-25s %15s\n", "Last Pause", time.Duration(m.PauseNs[(m.NumGC+255)%256]))
		fmt.Printf("  %-25s %15s\n", "Average Pause", time.Duration(m.PauseTotalNs/uint64(m.NumGC)))
	}

	// Calculate heap fragmentation
	used := float64(m.HeapInuse)
	total := float64(m.HeapSys)
	fragmentation := (1 - (used / total)) * 100

	fmt.Println("\nFragmentation:")
	fmt.Printf("  %-25s %15.2f%%\n", "Heap Fragmentation", fragmentation)

	if fragmentation > 30 {
		fmt.Println("\n⚠️  WARNING: High heap fragmentation detected!")
	}

	fmt.Println("\nAnalyze with:")
	fmt.Printf("  go tool pprof %s\n", filename)
	fmt.Printf("  go tool pprof -http=:8080 %s\n", filename)
	fmt.Println("\nUseful commands:")
	fmt.Println("  (pprof) top       - Show top memory consumers")
	fmt.Println("  (pprof) list func - Show source code for func")
	fmt.Println("  (pprof) web       - Open graphical view")

	return nil
}

// ProfileAllocation esegue il profiling delle allocazioni
func ProfileAllocation(outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("./profiles/alloc_%s.prof", time.Now().Format("20060102_150405"))
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create allocation profile: %w", err)
	}
	defer f.Close()

	if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write allocation profile: %w", err)
	}

	fmt.Printf("✓ Allocation profile saved to: %s\n", outputPath)
	return nil
}

// ProfileMutex esegue il profiling dei mutex
func ProfileMutex(outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("./profiles/mutex_%s.prof", time.Now().Format("20060102_150405"))
	}

	runtime.SetMutexProfileFraction(1)
	defer runtime.SetMutexProfileFraction(0)

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create mutex profile: %w", err)
	}
	defer f.Close()

	if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write mutex profile: %w", err)
	}

	fmt.Printf("✓ Mutex profile saved to: %s\n", outputPath)
	return nil
}

// ProfileBlock esegue il profiling dei blocchi
func ProfileBlock(outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("./profiles/block_%s.prof", time.Now().Format("20060102_150405"))
	}

	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0)

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create block profile: %w", err)
	}
	defer f.Close()

	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write block profile: %w", err)
	}

	fmt.Printf("✓ Block profile saved to: %s\n", outputPath)
	return nil
}

// StartTrace avvia il tracing dell'esecuzione
func StartTrace(duration int, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("./profiles/trace_%s.out", time.Now().Format("20060102_150405"))
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create trace file: %w", err)
	}
	defer f.Close()

	if err := trace.Start(f); err != nil {
		return fmt.Errorf("failed to start trace: %w", err)
	}
	defer trace.Stop()

	fmt.Printf("Tracing for %d seconds...\n", duration)
	time.Sleep(time.Duration(duration) * time.Second)

	fmt.Printf("\n✓ Trace saved to: %s\n", outputPath)
	fmt.Println("\nView with:")
	fmt.Printf("  go tool trace %s\n", outputPath)

	return nil
}

// GetRuntimeStats restituisce statistiche runtime
func GetRuntimeStats() *RuntimeStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &RuntimeStats{
		Goroutines:    runtime.NumGoroutine(),
		GOMAXPROCS:    runtime.GOMAXPROCS(0),
		NumCPU:        runtime.NumCPU(),
		MemAllocated:  m.Alloc,
		MemTotal:      m.TotalAlloc,
		MemSys:        m.Sys,
		NumGC:         m.NumGC,
		GCCPUFraction: m.GCCPUFraction,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapObjects:   m.HeapObjects,
	}
}

// Helper functions

// formatBytes formatta i byte in modo leggibile
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// repeatString ripete una stringa n volte
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// RuntimeStats contiene statistiche runtime
type RuntimeStats struct {
	Goroutines    int     `json:"goroutines"`
	GOMAXPROCS    int     `json:"gomaxprocs"`
	NumCPU        int     `json:"num_cpu"`
	MemAllocated  uint64  `json:"mem_allocated"`
	MemTotal      uint64  `json:"mem_total"`
	MemSys        uint64  `json:"mem_sys"`
	NumGC         uint32  `json:"num_gc"`
	GCCPUFraction float64 `json:"gc_cpu_fraction"`
	HeapAlloc     uint64  `json:"heap_alloc"`
	HeapSys       uint64  `json:"heap_sys"`
	HeapObjects   uint64  `json:"heap_objects"`
}
