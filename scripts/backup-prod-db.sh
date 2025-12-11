#!/bin/bash
# Backup production database and transfer to local machine
# Usage: ./scripts/backup-prod-db.sh

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Configuration
LOCAL_BACKUP_DIR="$PROJECT_ROOT/local_backups/db"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILENAME="backup_${TIMESTAMP}.sql.gz"
RESTORE_FILE="latest.sql"

# =============================================================================
# MAIN
# =============================================================================

log_step "Starting production database backup"

# Get SSH host
SSH_HOST=$(get_ssh_host)
log_info "Production server: $SSH_HOST"

# Create local backup directory if it doesn't exist
mkdir -p "$LOCAL_BACKUP_DIR"

# Get database credentials from .env.prod
require_file "$PROD_ENV_FILE" "Production env file"

DB_NAME=$(grep "^DB_NAME=" "$PROD_ENV_FILE" | cut -d'=' -f2 | tr -d '\n\r')
DB_USER=$(grep "^DB_USER=" "$PROD_ENV_FILE" | cut -d'=' -f2 | tr -d '\n\r')

if [[ -z "$DB_NAME" ]] || [[ -z "$DB_USER" ]]; then
    log_error "DB_NAME or DB_USER not found in $PROD_ENV_FILE"
    exit 1
fi

log_info "Database: $DB_NAME"
log_info "User: $DB_USER"

# Create backup on remote server
log_step "Creating database dump on production server"
remote_exec "$SSH_HOST" "docker exec beef-postgres-prod pg_dump -U $DB_USER -d $DB_NAME | gzip > /tmp/$BACKUP_FILENAME"
log_success "Database dump created"

# Transfer backup to local machine
log_step "Transferring backup to local machine"
scp "${SSH_HOST}:/tmp/$BACKUP_FILENAME" "$LOCAL_BACKUP_DIR/"
log_success "Backup transferred to $LOCAL_BACKUP_DIR/$BACKUP_FILENAME"

# Clean up remote backup
log_step "Cleaning up remote backup file"
remote_exec "$SSH_HOST" "rm -f /tmp/$BACKUP_FILENAME"
log_success "Remote backup file removed"

# Unzip and create latest.sql
log_step "Extracting backup to $RESTORE_FILE"
gunzip -c "$LOCAL_BACKUP_DIR/$BACKUP_FILENAME" > "$LOCAL_BACKUP_DIR/$RESTORE_FILE"
log_success "Backup extracted to $LOCAL_BACKUP_DIR/$RESTORE_FILE"

# Show backup info
BACKUP_SIZE=$(du -h "$LOCAL_BACKUP_DIR/$BACKUP_FILENAME" | cut -f1)
RESTORE_SIZE=$(du -h "$LOCAL_BACKUP_DIR/$RESTORE_FILE" | cut -f1)
LINE_COUNT=$(wc -l < "$LOCAL_BACKUP_DIR/$RESTORE_FILE")

echo ""
log_success "Backup completed successfully!"
echo ""
echo "  Compressed backup: $LOCAL_BACKUP_DIR/$BACKUP_FILENAME ($BACKUP_SIZE)"
echo "  Restore file:      $LOCAL_BACKUP_DIR/$RESTORE_FILE ($RESTORE_SIZE, $LINE_COUNT lines)"
echo ""
echo "To restore to local dev (deletes existing data):"
echo ""
echo "  make down && docker volume rm infrastructure_postgres_data_dev && make up"
echo "  # Wait for postgres to start, then:"
echo "  # docker exec -i beef-postgres-dev psql -U \$DB_USER -d \$DB_NAME < local_backups/db/latest.sql"
echo ""
