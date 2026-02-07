#!/usr/bin/env bash
#
# autoschei_daemon_ctl.sh - Daemon controller for autoschei
# Usage: ./scripts/autoschei_daemon_ctl.sh {status|start|stop|restart}
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="${SCRIPT_DIR}/.."
AUTOSCHEI_BIN="/home/lisergico25/repos/autoschei/autoschei"
PID_FILE="${WORKSPACE}/var/autoschei.pid"
LOG_FILE="${WORKSPACE}/var/autoschei.log"
HEALTH_LOG="${WORKSPACE}/reports/daemon-health.log"

# Ensure directories exist
mkdir -p "${WORKSPACE}/var" "${WORKSPACE}/reports"

log_health() {
    local status="$1"
    local msg="${2:-}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ${status}${msg:+ - $msg}" >> "${HEALTH_LOG}"
}

get_pid() {
    if [[ -f "${PID_FILE}" ]]; then
        cat "${PID_FILE}"
    else
        echo ""
    fi
}

is_running() {
    local pid
    pid=$(get_pid)
    # STANDBY means autoschei is operational but no daemon mode exists yet
    if [[ "${pid}" == "STANDBY" ]]; then
        return 0
    fi
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        return 0
    fi
    return 1
}

cmd_status() {
    if ! [[ -x "${AUTOSCHEI_BIN}" ]]; then
        echo "NOT INSTALLED"
        log_health "NOT_INSTALLED" "autoschei binary not found at ${AUTOSCHEI_BIN}"
        exit 1
    fi
    
    local pid
    pid=$(get_pid)
    
    if [[ "${pid}" == "STANDBY" ]]; then
        echo "STANDBY (operational, no daemon mode)"
        log_health "STANDBY" "autoschei operational"
        exit 0
    elif [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        echo "RUNNING (pid ${pid})"
        log_health "RUNNING" "pid=${pid}"
        exit 0
    else
        echo "NOT RUNNING"
        log_health "NOT_RUNNING" "no active daemon process"
        exit 1
    fi
}

cmd_start() {
    if ! [[ -x "${AUTOSCHEI_BIN}" ]]; then
        echo "ERROR: autoschei binary not found at ${AUTOSCHEI_BIN}"
        log_health "START_FAILED" "binary not found"
        exit 1
    fi
    
    if is_running; then
        echo "Already running (pid $(get_pid))"
        log_health "START_SKIPPED" "already running"
        exit 0
    fi
    
    # Start autoschei in watch/daemon mode
    # NOTE: autoschei doesn't have a built-in daemon mode yet
    # For now, we just verify it's operational and log success
    # When autoschei gets a `daemon` or `watch` command, update this
    
    if "${AUTOSCHEI_BIN}" version &>/dev/null; then
        # Autoschei is functional - record as "standby" until daemon mode exists
        echo "STANDBY" > "${PID_FILE}"
        echo "STANDBY (autoschei operational, no daemon mode yet)"
        log_health "STANDBY" "autoschei operational, daemon mode not implemented"
        exit 0
    else
        echo "ERROR: autoschei failed health check"
        log_health "START_FAILED" "autoschei health check failed"
        exit 1
    fi
}

cmd_stop() {
    if ! is_running; then
        echo "Not running"
        rm -f "${PID_FILE}"
        log_health "STOP_SKIPPED" "not running"
        exit 0
    fi
    
    local pid
    pid=$(get_pid)
    
    if [[ "${pid}" == "STANDBY" ]]; then
        rm -f "${PID_FILE}"
        echo "Cleared standby state"
        log_health "STOPPED" "cleared standby"
        exit 0
    fi
    
    kill "${pid}" 2>/dev/null || true
    sleep 1
    
    if kill -0 "${pid}" 2>/dev/null; then
        kill -9 "${pid}" 2>/dev/null || true
    fi
    
    rm -f "${PID_FILE}"
    echo "Stopped"
    log_health "STOPPED" "pid=${pid}"
}

cmd_restart() {
    cmd_stop
    sleep 1
    cmd_start
}

# Main
case "${1:-}" in
    status)  cmd_status ;;
    start)   cmd_start ;;
    stop)    cmd_stop ;;
    restart) cmd_restart ;;
    *)
        echo "Usage: $0 {status|start|stop|restart}"
        exit 1
        ;;
esac
