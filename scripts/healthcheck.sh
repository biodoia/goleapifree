#!/bin/bash

# Health check script for GoLeapAI
# Returns 0 if healthy, 1 if unhealthy

# Configuration
HOST="${HEALTH_CHECK_HOST:-localhost}"
PORT="${HEALTH_CHECK_PORT:-8080}"
TIMEOUT="${HEALTH_CHECK_TIMEOUT:-5}"
ENDPOINT="${HEALTH_CHECK_ENDPOINT:-/health}"

# Make request
response=$(curl -s -o /dev/null -w "%{http_code}" \
    --max-time "$TIMEOUT" \
    "http://${HOST}:${PORT}${ENDPOINT}")

# Check response
if [ "$response" = "200" ]; then
    echo "OK: Service is healthy (HTTP $response)"
    exit 0
else
    echo "ERROR: Service is unhealthy (HTTP $response)"
    exit 1
fi
