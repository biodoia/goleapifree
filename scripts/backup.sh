#!/bin/bash
set -e

# Backup script for GoLeapAI

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
BACKUP_DIR="${BACKUP_DIR:-./backups}"
DB_FILE="${DB_FILE:-./data/goleapai.db}"
CONFIG_DIR="${CONFIG_DIR:-./configs}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

create_backup_dir() {
    mkdir -p "$BACKUP_DIR"
    log_info "Backup directory: $BACKUP_DIR"
}

backup_database() {
    log_info "Backing up database..."

    if [ -f "$DB_FILE" ]; then
        # SQLite backup
        DB_BACKUP="$BACKUP_DIR/goleapai-db-$TIMESTAMP.db"
        cp "$DB_FILE" "$DB_BACKUP"
        gzip "$DB_BACKUP"
        log_info "Database backed up to: $DB_BACKUP.gz"
    else
        log_warn "Database file not found: $DB_FILE"
    fi
}

backup_postgres() {
    log_info "Backing up PostgreSQL database..."

    if command -v pg_dump &> /dev/null; then
        PG_BACKUP="$BACKUP_DIR/goleapai-pg-$TIMESTAMP.sql"
        pg_dump -U goleapai goleapai > "$PG_BACKUP"
        gzip "$PG_BACKUP"
        log_info "PostgreSQL database backed up to: $PG_BACKUP.gz"
    else
        log_warn "pg_dump not found, skipping PostgreSQL backup"
    fi
}

backup_config() {
    log_info "Backing up configuration..."

    if [ -d "$CONFIG_DIR" ]; then
        CONFIG_BACKUP="$BACKUP_DIR/goleapai-config-$TIMESTAMP.tar.gz"
        tar -czf "$CONFIG_BACKUP" -C "$CONFIG_DIR" .
        log_info "Configuration backed up to: $CONFIG_BACKUP"
    else
        log_warn "Config directory not found: $CONFIG_DIR"
    fi
}

cleanup_old_backups() {
    log_info "Cleaning up old backups (older than $RETENTION_DAYS days)..."

    find "$BACKUP_DIR" -type f -name "goleapai-*" -mtime +$RETENTION_DAYS -delete
    log_info "Old backups removed"
}

main() {
    log_info "Starting backup process..."

    create_backup_dir
    backup_database
    backup_config

    # Uncomment for PostgreSQL backup
    # backup_postgres

    cleanup_old_backups

    log_info "Backup completed successfully"
}

main
