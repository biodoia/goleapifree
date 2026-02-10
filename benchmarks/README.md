# GoLeapAI Free - Benchmark Suite

Comprehensive benchmark suite per misurare e ottimizzare le performance di GoLeapAI Free.

## Struttura

```
benchmarks/
├── provider_bench_test.go      # Provider performance benchmarks
├── routing_bench_test.go       # Routing strategy benchmarks
├── cache_bench_test.go         # Cache performance benchmarks
├── e2e_bench_test.go          # End-to-end integration benchmarks
├── load_test.go               # Load testing scenarios
├── run_benchmarks.sh          # Script per eseguire tutti i benchmark
└── README.md                  # Questa documentazione
```

## Provider Benchmarks

Misura le performance dei provider LLM:

```bash
go test -bench=BenchmarkProvider -benchmem -benchtime=10s
```

### Metriche Misurate

- **Latency**: Tempo di risposta end-to-end
- **Throughput**: Richieste al secondo (RPS)
- **Concurrency**: Performance con diversi livelli di concorrenza
- **Streaming vs Non-streaming**: Comparazione overhead
- **Memory Usage**: Allocazioni e consumo memoria
- **Request Sizes**: Performance con payload di diverse dimensioni

### Benchmark Principali

- `BenchmarkProviderChatCompletion` - Performance di base
- `BenchmarkProviderChatCompletion_Parallel` - Throughput parallelo
- `BenchmarkProviderLatency` - Misurazione latency dettagliata
- `BenchmarkProviderConcurrency` - Test concorrenza (1-500 goroutines)
- `BenchmarkProviderStreaming` - Performance streaming
- `BenchmarkMultipleProviders` - Comparazione tra provider

## Routing Benchmarks

Misura le performance del routing intelligente:

```bash
go test -bench=BenchmarkRouting -benchmem -benchtime=10s
```

### Metriche Misurate

- **Decision Latency**: Tempo per selezionare il provider
- **Strategy Performance**: Comparazione strategie (cost/latency/quality)
- **Cache Hit Ratio**: Impatto della cache sulle decisioni
- **Load Balancing**: Distribuzione del carico
- **Failover Time**: Tempo di failover su provider alternativo

### Benchmark Principali

- `BenchmarkRouterStrategySelection` - Velocità di selezione
- `BenchmarkRoutingStrategies` - Comparazione strategie
- `BenchmarkRoutingDecisionLatency` - Latency delle decisioni
- `BenchmarkRoutingWithDifferentProviderCounts` - Scalabilità
- `BenchmarkRoutingCacheHitRatio` - Impatto cache (0-99% hit ratio)

## Cache Benchmarks

Misura le performance del sistema di caching:

```bash
go test -bench=BenchmarkCache -benchmem -benchtime=10s
```

### Metriche Misurate

- **Read/Write Performance**: Velocità lettura/scrittura
- **Hit Rate Impact**: Impatto del hit rate (0-100%)
- **Memory Usage**: Consumo memoria per entry count
- **Eviction Efficiency**: Overhead dell'eviction
- **Semantic Search**: Performance ricerca semantica
- **Multi-Layer**: Overhead del multi-layer cache

### Benchmark Principali

- `BenchmarkCacheGet` - Performance lettura
- `BenchmarkCacheSet` - Performance scrittura
- `BenchmarkCacheHitRate` - Test hit rate (0%, 50%, 90%, 99%, 100%)
- `BenchmarkCacheValueSizes` - Performance con diverse dimensioni
- `BenchmarkSemanticCache` - Semantic cache performance
- `BenchmarkMultiLayerCache` - Memory vs Redis comparison

## End-to-End Benchmarks

Misura il ciclo completo delle richieste:

```bash
go test -bench=BenchmarkE2E -benchmem -benchtime=10s
```

### Metriche Misurate

- **Full Request Cycle**: Tempo totale end-to-end
- **OpenAI Compatibility**: Overhead della compatibilità
- **Middleware Overhead**: Impatto dei middleware
- **Streaming vs Non-streaming**: Comparazione
- **Payload Sizes**: Performance con diverse dimensioni
- **Latency Percentiles**: P50, P95, P99

### Benchmark Principali

- `BenchmarkE2EFullRequest` - Ciclo completo
- `BenchmarkE2EWithAuth` - Overhead autenticazione
- `BenchmarkE2EOpenAICompat` - Compatibilità OpenAI
- `BenchmarkE2ERequestPipeline` - Breakdown del pipeline
- `BenchmarkE2EConcurrentRequests` - Performance concorrente
- `BenchmarkE2ELatencyP99` - Distribuzione latency

## Load Testing

Test di carico e stress testing:

```bash
# Baseline (100 RPS)
go test -run=TestLoadBaseline -timeout=5m

# 1K RPS
go test -run=TestLoad1K -timeout=10m

# 10K RPS
go test -run=TestLoad10K -timeout=15m

# 100K RPS (stress test)
go test -run=TestLoad100K -timeout=30m

# Breaking point test
go test -run=TestLoadBreakingPoint -timeout=60m
```

### Scenari di Test

1. **Baseline** (100 RPS, 30s)
   - Test di controllo con basso carico
   - Stabilisce le performance di riferimento

2. **1K RPS** (1000 req/sec, 1 min)
   - Carico medio-alto
   - Simula traffico production

3. **10K RPS** (10000 req/sec, 2 min)
   - Alto carico
   - Test di scalabilità

4. **100K RPS** (100000 req/sec, 5 min)
   - Stress test
   - Identifica limiti del sistema

5. **Ramp-Up Test**
   - Incremento graduale: 100 → 500 → 1K → 2K RPS
   - Identifica punti di degrado

6. **Sustained Load** (1K RPS, 10 min)
   - Test carico sostenuto
   - Identifica memory leak e degrado

7. **Burst Traffic**
   - Picchi di traffico: 5K, 10K, 15K RPS
   - Test resilienza a spike

8. **Breaking Point**
   - Incremento automatico fino al fallimento
   - Identifica il throughput massimo

### Configurazione

Imposta la URL del server:

```bash
export LOAD_TEST_URL=http://localhost:8080
```

## Esecuzione Completa

Script per eseguire tutti i benchmark:

```bash
chmod +x run_benchmarks.sh
./run_benchmarks.sh
```

Lo script esegue:

1. Provider benchmarks
2. Routing benchmarks
3. Cache benchmarks
4. E2E benchmarks
5. Load tests (baseline, 1K, 10K)
6. Genera report finale in `benchmark_report.txt`

## Interpretazione Risultati

### Provider Benchmarks

```
BenchmarkProviderChatCompletion-8        1000    1234567 ns/op    1234 B/op    12 allocs/op
```

- `1000`: Numero di iterazioni
- `1234567 ns/op`: Tempo medio per operazione (nanoseconds)
- `1234 B/op`: Bytes allocati per operazione
- `12 allocs/op`: Numero di allocazioni per operazione

### Metriche Custom

```
avg_latency_ms: 15.23
req/sec: 65.43
error_rate_%: 1.2
```

### Target Performance

| Metrica | Target | Excellent | Acceptable | Poor |
|---------|--------|-----------|------------|------|
| Avg Latency (Provider) | <50ms | <100ms | <200ms | >200ms |
| Routing Decision | <100μs | <500μs | <1ms | >1ms |
| Cache Get | <10μs | <50μs | <100μs | >100μs |
| Cache Hit Rate | >95% | >90% | >80% | <80% |
| E2E Latency | <100ms | <200ms | <500ms | >500ms |
| Throughput (1K RPS) | <1% errors | <5% errors | <10% errors | >10% errors |
| P99 Latency | <200ms | <500ms | <1s | >1s |

## Grafici ASCII

I report includono grafici ASCII per visualizzare:

- Distribuzione latency
- Hit rate cache
- Distribution load balancing
- Resource utilization

Esempio:

```
Latency Distribution (ASCII):
  10ms  | ████████████████████████████ 280
  20ms  | ████████████████████████████████████████ 400
  30ms  | ██████████████████████ 220
  40ms  | ████████ 80
  50ms  | ██ 20
```

## Ottimizzazioni Suggerite

Basate sui risultati dei benchmark:

### Se Latency Alta

1. Enable caching aggressivo
2. Ottimizza routing strategy
3. Usa connection pooling
4. Riduci middleware overhead

### Se Throughput Basso

1. Aumenta concurrency
2. Abilita HTTP/2
3. Usa worker pools
4. Ottimizza database queries

### Se Memory Usage Alto

1. Riduci cache size
2. Abilita eviction policies
3. Usa object pooling
4. Ottimizza struct sizes

### Se Error Rate Alto

1. Implementa circuit breaker
2. Aumenta timeout
3. Abilita retry logic
4. Migliora health checks

## Continuous Benchmarking

Integrazione con CI/CD:

```yaml
# .github/workflows/benchmark.yml
name: Benchmarks
on: [push, pull_request]
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: ./benchmarks/run_benchmarks.sh
      - uses: benchmark-action/github-action-benchmark@v1
```

## Profiling

Per analisi approfondite:

```bash
# CPU profiling
go test -bench=BenchmarkProvider -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkCache -memprofile=mem.prof
go tool pprof mem.prof

# Trace
go test -bench=BenchmarkE2E -trace=trace.out
go tool trace trace.out
```

## Comparazione Performance

Compara branch o versioni:

```bash
# Baseline
git checkout main
go test -bench=. -benchmem > baseline.txt

# Feature branch
git checkout feature-branch
go test -bench=. -benchmem > feature.txt

# Compare
benchstat baseline.txt feature.txt
```

## Best Practices

1. **Warmup**: I benchmark includono warmup automatico
2. **Multiple Runs**: Esegui almeno 3 volte per risultati stabili
3. **Isolated Environment**: Esegui su macchine dedicate
4. **Consistent Load**: Disabilita processi in background
5. **Realistic Data**: Usa payload realistici
6. **Monitor Resources**: Controlla CPU, memoria, network

## Troubleshooting

### Benchmark Troppo Lenti

```bash
go test -bench=. -benchtime=1s  # Riduce tempo
go test -bench=BenchmarkSpecific # Esegui specifici
```

### Risultati Instabili

```bash
go test -bench=. -count=10  # Multiple runs
go test -bench=. -cpu=1     # Fixed CPU count
```

### Memory Issues

```bash
go test -bench=. -timeout=30m  # Aumenta timeout
ulimit -n 10000               # Aumenta file descriptors
```

## Report Automatici

I benchmark generano automaticamente:

1. **benchmark_report.txt** - Report testuale completo
2. **benchmark_results.json** - Dati in formato JSON
3. **benchmark_graphs.txt** - Grafici ASCII
4. **recommendations.txt** - Raccomandazioni automatiche

## Metriche Avanzate

### Resource Utilization

- CPU usage per request
- Memory allocations
- Network bandwidth
- Disk I/O
- Goroutine count

### Quality Metrics

- Error rate by type
- Retry success rate
- Failover time
- Recovery time
- Circuit breaker trips

### Business Metrics

- Cost per request
- Token usage
- Provider distribution
- Cache savings
- SLA compliance

## Contribuire

Per aggiungere nuovi benchmark:

1. Segui la convenzione `Benchmark<Component><Test>`
2. Usa `b.ReportMetric()` per metriche custom
3. Documenta target performance
4. Aggiungi test al README

## License

MIT - Vedi LICENSE file per dettagli
