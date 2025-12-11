#!/bin/bash
# Disable restore mount and re-enable migrations in docker-compose.dev.yml
# Run this after successfully restoring a backup
# Usage: ./scripts/disable-restore-db.sh

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Configuration
DEV_COMPOSE_FILE="$PROJECT_ROOT/infrastructure/docker-compose.dev.yml"

# =============================================================================
# MAIN
# =============================================================================

log_step "Disabling restore mount in docker-compose.dev.yml"

# Check if there's a restore mount to disable
if ! grep -q "local_backups/db/restore.sql" "$DEV_COMPOSE_FILE"; then
    log_warn "No restore mount found in docker-compose.dev.yml"
    log_info "Nothing to disable"
    exit 0
fi

# Remove the restore mount line
sed -i '/local_backups\/db\/restore.sql/d' "$DEV_COMPOSE_FILE"

# Remove the "RESTORE: migrations disabled" comment
sed -i '/# RESTORE: migrations disabled during restore/d' "$DEV_COMPOSE_FILE"

# Uncomment the migrations mount
sed -i 's|^\(\s*\)# - \.\./apps/postgres/migrations:/docker-entrypoint-initdb.d:ro|\1- ../apps/postgres/migrations:/docker-entrypoint-initdb.d:ro|' "$DEV_COMPOSE_FILE"

log_success "Restore mount removed"
log_success "Migrations mount re-enabled"

echo ""
echo "docker-compose.dev.yml has been restored to normal state."
echo "Migrations will run on the next fresh container start."
echo ""
echo "Note: The restore.sql file is still at local_backups/db/restore.sql"
echo "      You can delete it manually if no longer needed."
