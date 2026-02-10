#!/bin/bash

# Monitoring script per GoLeapAI
# Monitora lo stato del servizio e mostra metriche in tempo reale

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
METRICS_URL="${METRICS_URL:-http://localhost:9090/metrics}"
REFRESH_INTERVAL="${REFRESH_INTERVAL:-2}"

clear_screen() {
    clear
    echo -e "${CYAN}"
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════════╗
║              GoLeapAI - Live Monitor                              ║
╚═══════════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

get_health() {
    local response=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/health")
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}HEALTHY${NC}"
    else
        echo -e "${RED}UNHEALTHY${NC}"
    fi
}

get_metric() {
    local metric_name=$1
    curl -s "$METRICS_URL" | grep "^$metric_name" | grep -v "#" | awk '{print $2}' | head -1
}

format_bytes() {
    local bytes=$1
    if [ -z "$bytes" ] || [ "$bytes" = "0" ]; then
        echo "0 B"
        return
    fi

    local units=("B" "KB" "MB" "GB")
    local unit=0

    while [ $(echo "$bytes >= 1024" | bc) -eq 1 ] && [ $unit -lt 3 ]; do
        bytes=$(echo "scale=2; $bytes / 1024" | bc)
        unit=$((unit + 1))
    done

    echo "$bytes ${units[$unit]}"
}

show_stats() {
    clear_screen

    # Health status
    echo -e "${CYAN}Service Status:${NC}"
    echo "  Health: $(get_health)"
    echo ""

    # Memory stats
    echo -e "${CYAN}Memory Usage:${NC}"
    local mem_alloc=$(get_metric "go_memstats_alloc_bytes")
    local mem_sys=$(get_metric "go_memstats_sys_bytes")
    echo "  Allocated: $(format_bytes $mem_alloc)"
    echo "  System: $(format_bytes $mem_sys)"
    echo ""

    # HTTP stats
    echo -e "${CYAN}HTTP Requests:${NC}"
    local http_requests=$(get_metric "http_requests_total")
    echo "  Total: ${http_requests:-0}"
    echo ""

    # Goroutines
    echo -e "${CYAN}Goroutines:${NC}"
    local goroutines=$(get_metric "go_goroutines")
    echo "  Active: ${goroutines:-0}"
    echo ""

    # Database
    echo -e "${CYAN}Database Connections:${NC}"
    local db_active=$(get_metric "database_connections_active")
    local db_idle=$(get_metric "database_connections_idle")
    echo "  Active: ${db_active:-0}"
    echo "  Idle: ${db_idle:-0}"
    echo ""

    # Cache
    echo -e "${CYAN}Cache Statistics:${NC}"
    local cache_hits=$(get_metric "cache_hits_total")
    local cache_misses=$(get_metric "cache_misses_total")
    echo "  Hits: ${cache_hits:-0}"
    echo "  Misses: ${cache_misses:-0}"

    if [ -n "$cache_hits" ] && [ -n "$cache_misses" ]; then
        local total=$((cache_hits + cache_misses))
        if [ $total -gt 0 ]; then
            local hit_rate=$(echo "scale=2; $cache_hits * 100 / $total" | bc)
            echo "  Hit Rate: ${hit_rate}%"
        fi
    fi
    echo ""

    # Providers
    echo -e "${CYAN}Providers:${NC}"
    curl -s "$METRICS_URL" | grep "^provider_health_status" | grep -v "#" | \
    while read line; do
        local provider=$(echo $line | grep -o 'provider="[^"]*"' | cut -d'"' -f2)
        local status=$(echo $line | awk '{print $NF}')
        if [ "$status" = "1" ]; then
            echo -e "  ${provider}: ${GREEN}UP${NC}"
        else
            echo -e "  ${provider}: ${RED}DOWN${NC}"
        fi
    done
    echo ""

    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo -e "${YELLOW}Refreshing every ${REFRESH_INTERVAL}s...${NC}"
}

# Trap Ctrl+C
trap 'echo ""; echo "Exiting..."; exit 0' INT

# Main loop
while true; do
    show_stats
    sleep "$REFRESH_INTERVAL"
done
