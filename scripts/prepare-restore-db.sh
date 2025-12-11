#!/bin/bash
# Prepare to restore a database backup to local dev environment
# This script modifies docker-compose.dev.yml to mount the restore file
# Usage: ./scripts/prepare-restore-db.sh <path-to-restore.sql>

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Configuration
DEV_COMPOSE_FILE="$PROJECT_ROOT/infrastructure/docker-compose.dev.yml"
LOCAL_BACKUP_DIR="$PROJECT_ROOT/local_backups/db"
RESTORE_FILE="$LOCAL_BACKUP_DIR/restore.sql"

# =============================================================================
# MAIN
# =============================================================================

BACKUP_PATH="$1"

if [[ -z "$BACKUP_PATH" ]]; then
    log_error "Usage: $0 <path-to-backup.sql>"
    echo ""
    echo "Example:"
    echo "  $0 local_backups/db/restore.sql"
    echo "  make prepare-restore-db BACKUP=local_backups/db/restore.sql"
    exit 1
fi

# Check if backup file exists
if [[ ! -f "$BACKUP_PATH" ]]; then
    log_error "Backup file not found: $BACKUP_PATH"
    exit 1
fi

log_step "Preparing to restore database"

# Create backup directory if needed
mkdir -p "$LOCAL_BACKUP_DIR"

# Copy backup to restore location if it's not already there
if [[ "$BACKUP_PATH" != "$RESTORE_FILE" ]]; then
    log_info "Copying backup to $RESTORE_FILE"
    cp "$BACKUP_PATH" "$RESTORE_FILE"
fi

# Check if docker-compose.dev.yml already has restore mount
if grep -q "local_backups/db/restore.sql" "$DEV_COMPOSE_FILE"; then
    log_warn "Restore mount already present in docker-compose.dev.yml"
else
    log_step "Modifying docker-compose.dev.yml"

    # Comment out the migrations mount and add restore mount
    # We use sed to find the migrations line and add the restore mount after it (commented migrations)
    sed -i 's|^\(\s*\)- \.\./apps/postgres/migrations:/docker-entrypoint-initdb.d:ro|\1# RESTORE: migrations disabled during restore\n\1# - ../apps/postgres/migrations:/docker-entrypoint-initdb.d:ro\n\1- ../local_backups/db/restore.sql:/docker-entrypoint-initdb.d/restore.sql:ro|' "$DEV_COMPOSE_FILE"

    log_success "docker-compose.dev.yml modified"
fi

# Show warning and instructions
echo ""
log_warn "WARNING: This will DELETE your local database volume!"
echo ""
echo "The restore process will:"
echo "  1. Delete the postgres_data_dev volume"
echo "  2. Start fresh postgres with the restore.sql"
echo "  3. Import all data from the backup"
echo ""
echo "To proceed with restore:"
echo ""
echo "  1. Stop containers:  make down"
echo "  2. Delete volume:    docker volume rm infrastructure_postgres_data_dev"
echo "  3. Start services:   make up"
echo "  4. Wait for restore to complete (check logs: make logs-postgres)"
echo "  5. Disable restore:  make disable-restore-db"
echo ""
log_info "After restore, run 'make disable-restore-db' to re-enable migrations for future startups"
