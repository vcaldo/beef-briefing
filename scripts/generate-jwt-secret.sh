#!/bin/bash
# Generate JWT secret key for Mini App authentication and update .env.prod

set -euo pipefail

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROD_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.prod"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================
log_error() {
    echo -e "${RED}✗ Error: $1${NC}" >&2
}

log_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

log_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# =============================================================================
# MAIN FUNCTIONS
# =============================================================================

# Generate a secure random hex key
generate_jwt_secret() {
    openssl rand -hex 32
}

# Update or create JWT_SECRET_KEY in .env.prod
update_env_file() {
    local jwt_secret="$1"

    if grep -q '^JWT_SECRET_KEY=' "$PROD_ENV_FILE"; then
        # Check if it's empty or has a placeholder
        current_value=$(grep '^JWT_SECRET_KEY=' "$PROD_ENV_FILE" | cut -d'=' -f2)
        if [ -n "$current_value" ]; then
            echo -n "JWT_SECRET_KEY already has a value. Overwrite? [y/N]: "
            read -r confirm
            if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
                log_info "Keeping existing JWT_SECRET_KEY"
                return 0
            fi
        fi
        # Update existing entry
        sed -i "s/^JWT_SECRET_KEY=.*/JWT_SECRET_KEY=$jwt_secret/" "$PROD_ENV_FILE"
        log_success "Updated JWT_SECRET_KEY in $PROD_ENV_FILE"
    else
        # Add new entry
        echo "" >> "$PROD_ENV_FILE"
        echo "# JWT secret for Mini App authentication (generated $(date '+%Y-%m-%d %H:%M:%S'))" >> "$PROD_ENV_FILE"
        echo "JWT_SECRET_KEY=$jwt_secret" >> "$PROD_ENV_FILE"
        log_success "Added JWT_SECRET_KEY to $PROD_ENV_FILE"
    fi
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

main() {
    echo "========================================="
    echo "JWT Secret Key Generator"
    echo "========================================="
    echo ""

    if [ ! -f "$PROD_ENV_FILE" ]; then
        log_error "$PROD_ENV_FILE not found"
        echo "Please copy from .env.prod.example first:"
        echo "  cp infrastructure/.env.prod.example infrastructure/.env.prod"
        exit 1
    fi

    # Generate secret
    log_info "Generating JWT secret key..."
    jwt_secret=$(generate_jwt_secret)

    # Update .env.prod
    update_env_file "$jwt_secret"

    echo ""
    log_info "This key is used by card-renderer to sign Mini App JWT tokens."
    log_info "Make sure to redeploy after updating: make deploy"
}

main "$@"
