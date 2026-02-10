# GoLeapAI Monitoring Stack

Complete monitoring and observability solution for GoLeapAI with Prometheus, Grafana, and Alertmanager.

## Architecture

```
┌─────────────┐
│  GoLeapAI   │ ──metrics──> ┌────────────┐
│   (8080)    │              │ Prometheus │
└─────────────┘              │   (9090)   │
                             └──────┬─────┘
                                    │
                        ┌───────────┴──────────┐
                        │                      │
                   ┌────▼────┐          ┌─────▼────────┐
                   │ Grafana │          │ Alertmanager │
                   │ (3000)  │          │    (9093)    │
                   └─────────┘          └──────────────┘
```

## Dashboards

### 1. Overview Dashboard (`overview.json`)
**Purpose**: High-level system health and business metrics

**Metrics**:
- **Total Requests**: Real-time request rate (req/s)
- **Success Rate**: Percentage of successful requests (2xx status codes)
- **Average Latency**: P50 request latency in milliseconds
- **Top Providers**: Distribution of requests across AI providers
- **Cost Savings**: Total cost savings from using free providers
- **Active Users**: Number of unique users
- **Requests by Status**: Stacked time series of HTTP status codes

**Use Cases**:
- Quick health check
- Business metrics tracking
- Executive dashboards
- Daily standups

### 2. Provider Dashboard (`providers.json`)
**Purpose**: Monitor AI provider health and performance

**Metrics**:
- **Health Status**: Table showing each provider's health (Healthy/Down)
- **Requests per Provider**: Time series of request distribution
- **Latency Distribution**: Bar chart of P95 latency by provider
- **Error Rate**: Percentage of failed requests per provider
- **Quota Usage**: Bar gauge showing quota consumption (alerts at 80%, 90%)
- **Failover Events**: Histogram of provider failover occurrences

**Use Cases**:
- Provider reliability monitoring
- Capacity planning
- Quota management
- Failover analysis

**Variables**:
- `$provider`: Filter metrics by specific provider(s)

### 3. Performance Dashboard (`performance.json`)
**Purpose**: Deep dive into application performance

**Metrics**:
- **Latency Percentiles**: P50, P95, P99 request latency trends
- **Throughput**: Requests per second over time
- **Cache Hit Rate**: Gauge showing cache effectiveness
- **Database Query Time**: P95 query duration by query type
- **Database Connections**: Active vs idle connections
- **Memory Usage**: Heap allocation, in-use memory, stack usage
- **CPU Usage**: Process CPU utilization percentage
- **Goroutines**: Number of active goroutines
- **GC Rate**: Garbage collection frequency

**Use Cases**:
- Performance optimization
- Resource utilization analysis
- Bottleneck identification
- Capacity planning

## Alerts

### Critical Alerts (Immediate Response)
1. **ProviderDown**: Provider unavailable for 2+ minutes
2. **QuotaExhausted**: Provider quota >95% utilized
3. **ServiceInstanceDown**: GoLeapAI instance offline

### Warning Alerts (Action Required)
4. **HighErrorRate**: Error rate >5% for 5 minutes
5. **HighRequestLatency**: P95 latency >1s for 5 minutes
6. **QuotaNearLimit**: Provider quota >90% for 5 minutes
7. **HighMemoryUsage**: Memory usage >90% for 5 minutes
8. **ProviderFailoverSpike**: Excessive failover events
9. **DatabaseConnectionPoolExhausted**: DB connections >90% utilized
10. **HighCostBurn**: Cost burn rate >$10/hour

### Info Alerts (Informational)
11. **LowCacheHitRate**: Cache hit rate <50% for 10 minutes
12. **SlowDatabaseQueries**: P95 query time >500ms
13. **ProviderLatencyDegradation**: Provider P95 >2s

## Setup

### Prerequisites
- Docker & Docker Compose
- GoLeapAI application exposing `/metrics` endpoint
- Ports available: 3000 (Grafana), 9090 (Prometheus), 9093 (Alertmanager)

### Quick Start

1. **Configure Prometheus Target**

   Edit `prometheus/prometheus.yml`:
   ```yaml
   scrape_configs:
     - job_name: 'goleapai'
       static_configs:
         - targets:
             - 'your-app-host:8080'  # Update with your app address
   ```

2. **Configure Alert Notifications**

   Edit `prometheus/alertmanager.yml`:
   ```yaml
   global:
     smtp_smarthost: 'smtp.example.com:587'
     smtp_from: 'alerts@your-domain.com'
     smtp_auth_username: 'your-email@example.com'
     smtp_auth_password: 'your-password'

   receivers:
     - name: 'default-receiver'
       email_configs:
         - to: 'team@your-domain.com'
   ```

3. **Start the Stack**
   ```bash
   cd monitoring
   docker-compose up -d
   ```

4. **Access Services**
   - Grafana: http://localhost:3000 (admin/admin)
   - Prometheus: http://localhost:9090
   - Alertmanager: http://localhost:9093

### First Time Setup

1. **Login to Grafana**
   - URL: http://localhost:3000
   - Username: `admin`
   - Password: `admin`
   - Change password on first login

2. **Verify Data Source**
   - Go to Configuration > Data Sources
   - Prometheus should be pre-configured
   - Click "Test" to verify connection

3. **Access Dashboards**
   - Navigate to Dashboards > Browse
   - Find "GoLeapAI" folder
   - Open any dashboard

## Metrics Reference

### Application Metrics
```promql
# Request metrics
goleapai_requests_total                          # Counter: Total requests
goleapai_request_duration_seconds_bucket         # Histogram: Request duration
goleapai_request_duration_seconds_sum
goleapai_request_duration_seconds_count

# Provider metrics
goleapai_provider_requests_total                 # Counter: Requests per provider
goleapai_provider_health_status                  # Gauge: 1=healthy, 0=down
goleapai_provider_quota_used                     # Gauge: Current quota usage
goleapai_provider_quota_limit                    # Gauge: Quota limit
goleapai_provider_failover_total                 # Counter: Failover events
goleapai_provider_request_duration_seconds_bucket

# Cache metrics
goleapai_cache_hits_total                        # Counter: Cache hits
goleapai_cache_misses_total                      # Counter: Cache misses

# Database metrics
goleapai_db_query_duration_seconds_bucket        # Histogram: Query duration
goleapai_db_connections_active                   # Gauge: Active connections
goleapai_db_connections_idle                     # Gauge: Idle connections

# Cost metrics
goleapai_cost_total                              # Counter: Total cost
goleapai_cost_savings_total                      # Counter: Total savings
```

### System Metrics (from Go runtime)
```promql
go_goroutines                                    # Number of goroutines
go_memstats_heap_alloc_bytes                    # Heap allocated bytes
go_memstats_heap_inuse_bytes                    # Heap in-use bytes
go_memstats_heap_sys_bytes                      # Heap system bytes
go_memstats_stack_inuse_bytes                   # Stack in-use bytes
go_gc_duration_seconds                          # GC duration
process_cpu_seconds_total                       # CPU usage
```

## Common Queries

### Request Rate by Endpoint
```promql
sum by (endpoint, method) (rate(goleapai_requests_total[5m]))
```

### Success Rate
```promql
100 * (
  sum(rate(goleapai_requests_total{status=~"2.."}[5m]))
  /
  sum(rate(goleapai_requests_total[5m]))
)
```

### P95 Latency
```promql
histogram_quantile(0.95,
  sum(rate(goleapai_request_duration_seconds_bucket[5m])) by (le)
) * 1000
```

### Cache Hit Rate
```promql
100 * (
  sum(rate(goleapai_cache_hits_total[5m]))
  /
  (sum(rate(goleapai_cache_hits_total[5m])) + sum(rate(goleapai_cache_misses_total[5m])))
)
```

### Provider Health
```promql
goleapai_provider_health_status
```

### Quota Usage Percentage
```promql
(goleapai_provider_quota_used / goleapai_provider_quota_limit) * 100
```

## Alert Configuration

### Email Notifications
Configure in `prometheus/alertmanager.yml`:
```yaml
receivers:
  - name: 'critical-alerts'
    email_configs:
      - to: 'oncall@example.com'
        headers:
          Subject: '[CRITICAL] {{ .GroupLabels.alertname }}'
```

### Slack Notifications
Add webhook URL:
```yaml
receivers:
  - name: 'critical-alerts'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/XXX/YYY/ZZZ'
        channel: '#alerts-critical'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

### PagerDuty Integration
```yaml
receivers:
  - name: 'critical-alerts'
    pagerduty_configs:
      - service_key: 'your-pagerduty-key'
        description: '{{ .GroupLabels.alertname }}'
```

## Maintenance

### Retention Policy
- **Prometheus**: 30 days (configurable in docker-compose.yml)
- **Grafana**: Unlimited (dashboards are in Git)

### Backup
```bash
# Backup Prometheus data
docker run --rm -v goleapai-monitoring_prometheus_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/prometheus-backup-$(date +%Y%m%d).tar.gz /data

# Backup Grafana data
docker run --rm -v goleapai-monitoring_grafana_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/grafana-backup-$(date +%Y%m%d).tar.gz /data
```

### Update Dashboards
1. Edit JSON files in `grafana/dashboards/`
2. Reload Grafana provisioning:
   ```bash
   docker-compose restart grafana
   ```

### Update Alerts
1. Edit `prometheus/alerts.yml`
2. Reload Prometheus configuration:
   ```bash
   curl -X POST http://localhost:9090/-/reload
   ```

## Troubleshooting

### No Data in Dashboards
1. Check Prometheus targets: http://localhost:9090/targets
2. Verify GoLeapAI metrics endpoint: http://your-app:8080/metrics
3. Check Prometheus logs: `docker logs goleapai-prometheus`

### Alerts Not Firing
1. Verify alert rules: http://localhost:9090/alerts
2. Check Alertmanager status: http://localhost:9093
3. Review Alertmanager logs: `docker logs goleapai-alertmanager`

### Dashboard Not Loading
1. Check Grafana logs: `docker logs goleapai-grafana`
2. Verify provisioning: http://localhost:3000/datasources
3. Ensure Prometheus is reachable from Grafana

### High Memory Usage
1. Reduce Prometheus retention: Change `--storage.tsdb.retention.time=30d`
2. Decrease scrape frequency: Change `scrape_interval` in prometheus.yml
3. Add query limits in Grafana data source settings

## Performance Tuning

### Prometheus
```yaml
# prometheus.yml
global:
  scrape_interval: 15s      # Increase for lower load
  evaluation_interval: 15s  # Increase for lower CPU usage
```

### Grafana
- Enable query caching in data source settings
- Use recording rules for complex queries
- Limit time range in dashboards

### Recording Rules
Create `prometheus/recording-rules.yml`:
```yaml
groups:
  - name: goleapai_recording_rules
    interval: 30s
    rules:
      - record: job:goleapai_request_rate:5m
        expr: sum(rate(goleapai_requests_total[5m]))

      - record: job:goleapai_error_rate:5m
        expr: |
          sum(rate(goleapai_requests_total{status=~"[45].."}[5m]))
          / sum(rate(goleapai_requests_total[5m]))
```

## Integration with GoLeapAI

Your GoLeapAI application must expose a `/metrics` endpoint that returns Prometheus-formatted metrics. Example implementation is in `internal/monitoring/`.

Required metrics:
- `goleapai_requests_total`
- `goleapai_request_duration_seconds`
- `goleapai_provider_*`
- `goleapai_cache_*`
- `goleapai_db_*`

## Support

For issues or questions:
- Check Prometheus documentation: https://prometheus.io/docs
- Grafana documentation: https://grafana.com/docs
- GoLeapAI monitoring guide: [Link to your docs]

## License

Same as GoLeapAI project
