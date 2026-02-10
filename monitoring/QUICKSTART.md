# GoLeapAI Monitoring - Quick Start Guide

Get your monitoring stack up and running in 5 minutes!

## Prerequisites

- Docker installed
- Docker Compose installed
- GoLeapAI running and exposing metrics on `/metrics`

## Installation

### 1. Run the Setup Script

```bash
cd monitoring
./setup.sh
```

This will:
- Validate all configuration files
- Pull Docker images
- Start all services
- Perform health checks

### 2. Configure Your Application

Edit `prometheus/prometheus.yml` and update the GoLeapAI target:

```yaml
scrape_configs:
  - job_name: 'goleapai'
    static_configs:
      - targets:
          - 'localhost:8080'  # Change to your app's address
```

Reload Prometheus:
```bash
make reload-prom
```

### 3. Access Grafana

1. Open http://localhost:3000
2. Login with `admin` / `admin`
3. Change the default password
4. Navigate to **Dashboards → Browse → GoLeapAI**

## Available Dashboards

### Overview Dashboard
**Best for**: Daily monitoring, team standups, executive reports

Key metrics:
- Total request rate
- Success rate (gauge)
- Average latency
- Provider usage distribution
- Cost savings
- Active users

### Provider Dashboard
**Best for**: Provider health monitoring, quota management

Key metrics:
- Health status per provider
- Request distribution
- Latency by provider
- Error rates
- Quota usage (with alerts)
- Failover events

### Performance Dashboard
**Best for**: Performance optimization, troubleshooting

Key metrics:
- Latency percentiles (P50, P95, P99)
- Throughput
- Cache hit rate
- Database query times
- Memory usage
- CPU usage
- Goroutines
- GC rate

## Alert Configuration

### Email Alerts

Edit `prometheus/alertmanager.yml`:

```yaml
global:
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@yourcompany.com'
  smtp_auth_username: 'alerts@yourcompany.com'
  smtp_auth_password: 'your-app-password'

receivers:
  - name: 'default-receiver'
    email_configs:
      - to: 'team@yourcompany.com'
```

### Slack Alerts

Add to the receiver configuration:

```yaml
receivers:
  - name: 'critical-alerts'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/XXX/YYY/ZZZ'
        channel: '#alerts-critical'
```

Reload Alertmanager:
```bash
docker-compose restart alertmanager
```

## Common Commands

```bash
# Start everything
make up

# Check status
make status

# View logs
make logs

# View active alerts
make alerts

# Health check
make health

# Backup data
make backup

# Stop everything
make down
```

## Verify Installation

1. **Check Prometheus targets**:
   - Go to http://localhost:9090/targets
   - All targets should show as "UP"

2. **Check Grafana**:
   - Go to http://localhost:3000
   - Navigate to Dashboards → Browse
   - You should see 3 dashboards in the "GoLeapAI" folder

3. **Check Alerts**:
   - Go to http://localhost:9090/alerts
   - All alert rules should show as "Inactive" (green)

## Troubleshooting

### No Data in Dashboards

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check if your app is exposing metrics
curl http://localhost:8080/metrics

# View Prometheus logs
make logs-prom
```

### Alerts Not Working

```bash
# Check alert rules
curl http://localhost:9090/api/v1/rules

# Check Alertmanager
curl http://localhost:9093/api/v2/status

# View Alertmanager logs
make logs-alert
```

### Configuration Errors

```bash
# Validate all configs
make validate
```

## Integration with GoLeapAI

Your application must expose metrics in Prometheus format. Required metrics:

```go
// Example metrics (see internal/monitoring/ for implementation)
goleapai_requests_total
goleapai_request_duration_seconds
goleapai_provider_requests_total
goleapai_provider_health_status
goleapai_provider_quota_used
goleapai_provider_quota_limit
goleapai_cache_hits_total
goleapai_cache_misses_total
goleapai_db_query_duration_seconds
```

## What's Included

```
monitoring/
├── grafana/
│   ├── dashboards/
│   │   ├── overview.json         # Main dashboard
│   │   ├── providers.json        # Provider monitoring
│   │   └── performance.json      # Performance metrics
│   └── provisioning/
│       ├── datasources.yml       # Prometheus connection
│       ├── dashboards.yml        # Dashboard loading
│       └── alerting.yml          # Alert rules (Grafana)
├── prometheus/
│   ├── prometheus.yml            # Main config
│   ├── alerts.yml                # Alert rules (15 rules)
│   └── alertmanager.yml          # Alert routing
├── docker-compose.yml            # Service orchestration
├── Makefile                      # Management commands
├── setup.sh                      # One-command setup
└── README.md                     # Full documentation
```

## Alerts Summary

15 alert rules covering:

**Critical** (immediate response):
- Provider down
- Service instance down
- Quota exhausted (>95%)

**Warning** (action required):
- High error rate (>5%)
- High latency (P95 >1s)
- Quota near limit (>90%)
- High memory usage (>90%)
- Failover spike
- Database pool exhausted
- High cost burn rate

**Info** (informational):
- Low cache hit rate
- Slow database queries
- Provider latency degradation

## Next Steps

1. **Customize Dashboards**: Click "Save As" in Grafana to create your own versions
2. **Add Custom Alerts**: Edit `prometheus/alerts.yml` with your thresholds
3. **Set Up Notifications**: Configure Slack/PagerDuty/Email in alertmanager.yml
4. **Create Recording Rules**: Add frequently-used queries to prometheus.yml
5. **Monitor Costs**: Track usage patterns to optimize provider selection

## Support

- Full documentation: `README.md`
- Prometheus docs: https://prometheus.io/docs
- Grafana docs: https://grafana.com/docs
- Issues: [Your issue tracker]

## Tips

- Use time range shortcuts in Grafana (Last 1h, Last 6h, etc.)
- Pin important dashboards to your favorites
- Set up dashboard playlists for TV displays
- Use dashboard variables to filter by provider/endpoint
- Export dashboards as JSON for version control
- Set up dashboard annotations for deployments

Enjoy monitoring! 🚀
