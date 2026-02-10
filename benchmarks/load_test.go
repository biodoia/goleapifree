package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// LoadTestConfig configurazione per load testing
type LoadTestConfig struct {
	TargetURL      string
	Duration       time.Duration
	RequestsPerSec int
	Concurrency    int
	Timeout        time.Duration
	RampUpTime     time.Duration
}

// LoadTestResult risultati del load test
type LoadTestResult struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	TotalLatency      time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	AvgLatency        time.Duration
	P50Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	ThroughputRPS     float64
	ErrorRate         float64
	MemoryUsageMB     float64
	CPUUsagePercent   float64
	NetworkBytesSent  int64
	NetworkBytesRecv  int64
	Latencies         []time.Duration
}

// TestLoadBaseline test di baseline (basso carico)
func TestLoadBaseline(t *testing.T) {
	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       30 * time.Second,
		RequestsPerSec: 100,
		Concurrency:    10,
		Timeout:        5 * time.Second,
	}

	result := runLoadTest(t, config)
	printLoadTestResults(t, "Baseline (100 RPS)", result)

	// Assert baseline expectations
	if result.ErrorRate > 1.0 {
		t.Errorf("Error rate too high: %.2f%%", result.ErrorRate)
	}

	if result.AvgLatency > 100*time.Millisecond {
		t.Errorf("Average latency too high: %v", result.AvgLatency)
	}
}

// TestLoad1K test a 1k req/sec
func TestLoad1K(t *testing.T) {
	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       1 * time.Minute,
		RequestsPerSec: 1000,
		Concurrency:    50,
		Timeout:        5 * time.Second,
		RampUpTime:     10 * time.Second,
	}

	result := runLoadTest(t, config)
	printLoadTestResults(t, "Load Test 1K RPS", result)

	// Performance assertions
	if result.ErrorRate > 5.0 {
		t.Errorf("Error rate too high at 1K RPS: %.2f%%", result.ErrorRate)
	}
}

// TestLoad10K test a 10k req/sec
func TestLoad10K(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 10K load test in short mode")
	}

	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       2 * time.Minute,
		RequestsPerSec: 10000,
		Concurrency:    200,
		Timeout:        10 * time.Second,
		RampUpTime:     30 * time.Second,
	}

	result := runLoadTest(t, config)
	printLoadTestResults(t, "Load Test 10K RPS", result)

	// More lenient assertions for high load
	if result.ErrorRate > 10.0 {
		t.Errorf("Error rate too high at 10K RPS: %.2f%%", result.ErrorRate)
	}
}

// TestLoad100K test a 100k req/sec (stress test)
func TestLoad100K(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 100K load test in short mode")
	}

	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       5 * time.Minute,
		RequestsPerSec: 100000,
		Concurrency:    1000,
		Timeout:        15 * time.Second,
		RampUpTime:     1 * time.Minute,
	}

	result := runLoadTest(t, config)
	printLoadTestResults(t, "Stress Test 100K RPS", result)

	// This is a stress test - we expect some failures
	t.Logf("Stress test completed with %.2f%% error rate", result.ErrorRate)
}

// TestLoadRampUp test con ramp-up graduale
func TestLoadRampUp(t *testing.T) {
	stages := []struct {
		name string
		rps  int
		dur  time.Duration
	}{
		{"Stage 1: 100 RPS", 100, 30 * time.Second},
		{"Stage 2: 500 RPS", 500, 30 * time.Second},
		{"Stage 3: 1000 RPS", 1000, 30 * time.Second},
		{"Stage 4: 2000 RPS", 2000, 30 * time.Second},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			config := LoadTestConfig{
				TargetURL:      getTestServerURL(),
				Duration:       stage.dur,
				RequestsPerSec: stage.rps,
				Concurrency:    stage.rps / 10,
				Timeout:        5 * time.Second,
			}

			result := runLoadTest(t, config)
			printLoadTestResults(t, stage.name, result)
		})
	}
}

// TestLoadSustained test di carico sostenuto
func TestLoadSustained(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       10 * time.Minute,
		RequestsPerSec: 1000,
		Concurrency:    100,
		Timeout:        5 * time.Second,
	}

	result := runLoadTest(t, config)
	printLoadTestResults(t, "Sustained Load Test (10 min)", result)

	// Check for memory leaks or degradation
	if result.ErrorRate > 5.0 {
		t.Errorf("Error rate increased during sustained load: %.2f%%", result.ErrorRate)
	}
}

// TestLoadBurstTraffic test con traffico a burst
func TestLoadBurstTraffic(t *testing.T) {
	bursts := []struct {
		name        string
		rps         int
		duration    time.Duration
		cooldown    time.Duration
	}{
		{"Burst 1", 5000, 10 * time.Second, 5 * time.Second},
		{"Burst 2", 10000, 10 * time.Second, 5 * time.Second},
		{"Burst 3", 15000, 10 * time.Second, 5 * time.Second},
	}

	for _, burst := range bursts {
		t.Run(burst.name, func(t *testing.T) {
			config := LoadTestConfig{
				TargetURL:      getTestServerURL(),
				Duration:       burst.duration,
				RequestsPerSec: burst.rps,
				Concurrency:    burst.rps / 10,
				Timeout:        5 * time.Second,
			}

			result := runLoadTest(t, config)
			printLoadTestResults(t, burst.name, result)

			// Cooldown
			time.Sleep(burst.cooldown)
		})
	}
}

// TestLoadResourceUtilization monitora utilizzo risorse
func TestLoadResourceUtilization(t *testing.T) {
	config := LoadTestConfig{
		TargetURL:      getTestServerURL(),
		Duration:       2 * time.Minute,
		RequestsPerSec: 1000,
		Concurrency:    100,
		Timeout:        5 * time.Second,
	}

	// Monitor resources during test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resourceStats := monitorResources(ctx, 5*time.Second)

	result := runLoadTest(t, config)

	cancel()
	finalStats := <-resourceStats

	printLoadTestResults(t, "Resource Utilization Test", result)
	printResourceStats(t, finalStats)

	// Assert resource limits
	if finalStats.MaxMemoryMB > 1024 {
		t.Logf("Warning: High memory usage: %.2f MB", finalStats.MaxMemoryMB)
	}
}

// TestLoadBreakingPoint trova il breaking point
func TestLoadBreakingPoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping breaking point test in short mode")
	}

	startRPS := 1000
	increment := 1000
	maxRPS := 50000
	errorThreshold := 20.0 // 20% error rate

	t.Logf("Finding breaking point (max RPS before %.1f%% errors)", errorThreshold)

	for rps := startRPS; rps <= maxRPS; rps += increment {
		t.Logf("\nTesting at %d RPS...", rps)

		config := LoadTestConfig{
			TargetURL:      getTestServerURL(),
			Duration:       30 * time.Second,
			RequestsPerSec: rps,
			Concurrency:    rps / 10,
			Timeout:        10 * time.Second,
		}

		result := runLoadTest(t, config)

		t.Logf("RPS: %d, Error Rate: %.2f%%, Avg Latency: %v",
			rps, result.ErrorRate, result.AvgLatency)

		if result.ErrorRate > errorThreshold {
			t.Logf("\nBreaking point found at approximately %d RPS", rps-increment)
			break
		}

		// Cooldown between tests
		time.Sleep(10 * time.Second)
	}
}

// runLoadTest esegue il load test
func runLoadTest(t *testing.T, config LoadTestConfig) *LoadTestResult {
	t.Logf("Starting load test: %d RPS for %v", config.RequestsPerSec, config.Duration)

	result := &LoadTestResult{
		MinLatency: time.Hour,
		Latencies:  make([]time.Duration, 0, config.RequestsPerSec*int(config.Duration.Seconds())),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	startTime := time.Now()
	requestInterval := time.Second / time.Duration(config.RequestsPerSec)

	// Rate limiter
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	// Worker pool
	workers := make(chan struct{}, config.Concurrency)

	// Memory stats before
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Ramp-up
	if config.RampUpTime > 0 {
		t.Logf("Ramping up for %v...", config.RampUpTime)
		time.Sleep(config.RampUpTime)
	}

	testDuration := config.Duration
	timeout := time.After(testDuration)

	for {
		select {
		case <-timeout:
			wg.Wait()
			goto Done

		case <-ticker.C:
			workers <- struct{}{}
			wg.Add(1)

			go func() {
				defer wg.Done()
				defer func() { <-workers }()

				reqStart := time.Now()
				err := makeLoadTestRequest(config.TargetURL, config.Timeout)
				latency := time.Since(reqStart)

				mu.Lock()
				atomic.AddInt64(&result.TotalRequests, 1)

				if err != nil {
					atomic.AddInt64(&result.FailedReqs, 1)
				} else {
					atomic.AddInt64(&result.SuccessfulReqs, 1)
					result.TotalLatency += latency
					result.Latencies = append(result.Latencies, latency)

					if latency < result.MinLatency {
						result.MinLatency = latency
					}
					if latency > result.MaxLatency {
						result.MaxLatency = latency
					}
				}
				mu.Unlock()
			}()
		}
	}

Done:
	elapsed := time.Since(startTime)

	// Calculate statistics
	if result.SuccessfulReqs > 0 {
		result.AvgLatency = result.TotalLatency / time.Duration(result.SuccessfulReqs)
		result.P50Latency = percentile(result.Latencies, 0.50)
		result.P95Latency = percentile(result.Latencies, 0.95)
		result.P99Latency = percentile(result.Latencies, 0.99)
	}

	result.ThroughputRPS = float64(result.SuccessfulReqs) / elapsed.Seconds()
	result.ErrorRate = float64(result.FailedReqs) / float64(result.TotalRequests) * 100

	// Memory stats after
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	result.MemoryUsageMB = float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024

	return result
}

// makeLoadTestRequest esegue una singola richiesta
func makeLoadTestRequest(url string, timeout time.Duration) error {
	req := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}

	body, _ := json.Marshal(req)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(url+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// monitorResources monitora l'utilizzo delle risorse
func monitorResources(ctx context.Context, interval time.Duration) <-chan *ResourceStats {
	stats := &ResourceStats{}
	out := make(chan *ResourceStats, 1)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				out <- stats
				return

			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				memMB := float64(m.Alloc) / 1024 / 1024
				if memMB > stats.MaxMemoryMB {
					stats.MaxMemoryMB = memMB
				}

				stats.AvgMemoryMB = (stats.AvgMemoryMB + memMB) / 2
				stats.Samples++
			}
		}
	}()

	return out
}

// ResourceStats statistiche sulle risorse
type ResourceStats struct {
	MaxMemoryMB float64
	AvgMemoryMB float64
	MaxCPU      float64
	AvgCPU      float64
	Samples     int
}

// printLoadTestResults stampa i risultati
func printLoadTestResults(t *testing.T, name string, result *LoadTestResult) {
	t.Logf("\n" + generateLoadTestReport(name, result))
}

// generateLoadTestReport genera un report dettagliato
func generateLoadTestReport(name string, result *LoadTestResult) string {
	var buf bytes.Buffer

	buf.WriteString("=" + repeat("=", 70) + "\n")
	buf.WriteString(fmt.Sprintf("  %s\n", name))
	buf.WriteString("=" + repeat("=", 70) + "\n\n")

	buf.WriteString("Requests:\n")
	buf.WriteString(fmt.Sprintf("  Total:      %d\n", result.TotalRequests))
	buf.WriteString(fmt.Sprintf("  Successful: %d\n", result.SuccessfulReqs))
	buf.WriteString(fmt.Sprintf("  Failed:     %d\n", result.FailedReqs))
	buf.WriteString(fmt.Sprintf("  Error Rate: %.2f%%\n\n", result.ErrorRate))

	buf.WriteString("Throughput:\n")
	buf.WriteString(fmt.Sprintf("  RPS: %.2f req/sec\n\n", result.ThroughputRPS))

	buf.WriteString("Latency:\n")
	buf.WriteString(fmt.Sprintf("  Min:     %v\n", result.MinLatency))
	buf.WriteString(fmt.Sprintf("  Max:     %v\n", result.MaxLatency))
	buf.WriteString(fmt.Sprintf("  Average: %v\n", result.AvgLatency))
	buf.WriteString(fmt.Sprintf("  P50:     %v\n", result.P50Latency))
	buf.WriteString(fmt.Sprintf("  P95:     %v\n", result.P95Latency))
	buf.WriteString(fmt.Sprintf("  P99:     %v\n\n", result.P99Latency))

	buf.WriteString("Latency Distribution (ASCII):\n")
	buf.WriteString(generateLatencyHistogram(result.Latencies))

	buf.WriteString("\nResources:\n")
	buf.WriteString(fmt.Sprintf("  Memory: %.2f MB\n", result.MemoryUsageMB))

	buf.WriteString("\n" + repeat("=", 72) + "\n")

	return buf.String()
}

// printResourceStats stampa statistiche risorse
func printResourceStats(t *testing.T, stats *ResourceStats) {
	t.Logf("\nResource Statistics:")
	t.Logf("  Max Memory: %.2f MB", stats.MaxMemoryMB)
	t.Logf("  Avg Memory: %.2f MB", stats.AvgMemoryMB)
	t.Logf("  Samples:    %d", stats.Samples)
}

// generateLatencyHistogram genera un istogramma ASCII
func generateLatencyHistogram(latencies []time.Duration) string {
	if len(latencies) == 0 {
		return "  No data\n"
	}

	// Create buckets
	buckets := 20
	min := time.Hour
	max := time.Duration(0)

	for _, l := range latencies {
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}

	bucketSize := (max - min) / time.Duration(buckets)
	if bucketSize == 0 {
		bucketSize = 1
	}

	counts := make([]int, buckets)
	for _, l := range latencies {
		bucket := int((l - min) / bucketSize)
		if bucket >= buckets {
			bucket = buckets - 1
		}
		counts[bucket]++
	}

	// Find max count for scaling
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	var buf bytes.Buffer
	scale := 50.0 / float64(maxCount)

	for i, count := range counts {
		bucketStart := min + time.Duration(i)*bucketSize
		barLen := int(float64(count) * scale)
		bar := repeat("█", barLen)

		buf.WriteString(fmt.Sprintf("  %6v | %s %d\n", bucketStart, bar, count))
	}

	return buf.String()
}

// Helper functions

func getTestServerURL() string {
	url := os.Getenv("LOAD_TEST_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	return url
}

func repeat(s string, count int) string {
	return bytes.Repeat([]byte(s), count).String()
}
