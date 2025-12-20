#!/bin/bash
# Common functions for deployment scripts

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TERRAFORM_DIR="$PROJECT_ROOT/infrastructure/terraform"
PROD_COMPOSE_FILE="$PROJECT_ROOT/infrastructure/docker-compose.prod.yml"
PROD_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.prod"
SECRETS_DIR="$PROJECT_ROOT/infrastructure/secrets/apps"

# Remote paths
REMOTE_APP_DIR="~/beef-briefing"
REMOTE_PREVIOUS_TAG_FILE="$REMOTE_APP_DIR/.previous_tag"

# Images to manage
IMAGES=(
    "beef-briefing/api-service"
    "beef-briefing/telegram-bot"
    "beef-briefing/leaderboard"
    "beef-briefing/card-image-generator"
)

# Required environment variables for deployment
REQUIRED_ENV_VARS=(
    "DB_PASSWORD"
    "TELEGRAM_BOT_TOKEN"
    "MINIO_ACCESS_KEY"
    "MINIO_SECRET_KEY"
    "MINIO_BUCKET"
    "DOMAIN_NAME"
    "LETSENCRYPT_EMAIL"
    "TRAEFIK_DASHBOARD_USERS"
)

# =============================================================================
# LOGGING FUNCTIONS
# =============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}ℹ${NC}  $1"
}

log_success() {
    echo -e "${GREEN}✓${NC}  $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

log_error() {
    echo -e "${RED}✗${NC}  $1" >&2
}

log_step() {
    echo -e "\n${BLUE}▶${NC}  ${YELLOW}$1${NC}"
}

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================

# Get SSH user@host from Terraform output
get_ssh_host() {
    cd "$TERRAFORM_DIR"
    terraform output -raw ssh_user_host 2>/dev/null || {
        log_error "Failed to get SSH host from Terraform. Is infrastructure deployed?"
        exit 1
    }
}

# Get remote server CPU architecture (returns GOARCH-compatible value: amd64, arm64, arm)
get_remote_arch() {
    local ssh_host="$1"
    local arch_raw
    arch_raw=$(ssh "$ssh_host" 'uname -m' 2>/dev/null) || {
        log_warn "Failed to detect remote architecture, defaulting to amd64"
        echo "amd64"
        return
    }

    case "$arch_raw" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        armv7l)  echo "arm" ;;
        *)       echo "amd64" ;;
    esac
}

# Get current git commit hash (short)
get_commit_hash() {
    git -C "$PROJECT_ROOT" rev-parse --short HEAD
}

# Check if a file exists
require_file() {
    local file="$1"
    local description="$2"
    if [[ ! -f "$file" ]]; then
        log_error "$description not found: $file"
        exit 1
    fi
}

# Check if a directory exists
require_dir() {
    local dir="$1"
    local description="$2"
    if [[ ! -d "$dir" ]]; then
        log_error "$description not found: $dir"
        exit 1
    fi
}

# Check if required environment variables are set in .env.prod
validate_env_vars() {
    local env_file="$1"
    local missing=()

    for var in "${REQUIRED_ENV_VARS[@]}"; do
        # Check if variable exists and has a non-placeholder value
        local value
        value=$(grep "^${var}=" "$env_file" | cut -d'=' -f2- | tr -d '\n\r')

        if [[ -z "$value" ]]; then
            missing+=("$var (not set)")
        elif [[ "$value" == *"your_"* ]] || [[ "$value" == *"placeholder"* ]]; then
            missing+=("$var (placeholder value)")
        fi
    done

    # Special validation for Traefik dashboard users (htpasswd bcrypt format)
    if grep -q '^TRAEFIK_DASHBOARD_USERS=' "$env_file"; then
        traefik_users=$(grep '^TRAEFIK_DASHBOARD_USERS=' "$env_file" | cut -d'=' -f2-)

        # MUST use $$2y$$ or $$2a$$ format (double $$ for docker-compose escaping)
        # Single $ will be interpreted as variable substitution and corrupt the hash
        if [[ "$traefik_users" != *'$$2y$$'* ]] && [[ "$traefik_users" != *'$$2a$$'* ]]; then
            log_error "TRAEFIK_DASHBOARD_USERS must use \$\$ escaped format for docker-compose"
            log_error "Current value uses single \$ which will be corrupted by variable expansion"
            log_error "Run 'make generate-traefik-password ENV_FILE=infrastructure/.env.prod' to fix"
            missing+=("TRAEFIK_DASHBOARD_USERS (invalid format)")
        fi
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing or invalid environment variables in $env_file:"
        for var in "${missing[@]}"; do
            echo "  - $var"
        done
        exit 1
    fi
}

# Get current IMAGE_TAG from env file
get_env_image_tag() {
    local env_file="$1"
    grep "^IMAGE_TAG=" "$env_file" | cut -d'=' -f2 | tr -d '\n\r'
}

# Update IMAGE_TAG in env file
set_env_image_tag() {
    local env_file="$1"
    local new_tag="$2"
    sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=$new_tag/" "$env_file"
}

# Run command on remote server via SSH
remote_exec() {
    local ssh_host="$1"
    shift
    ssh "$ssh_host" "$@"
}

# Copy file to remote server
remote_copy() {
    local src="$1"
    local ssh_host="$2"
    local dest="$3"
    scp -r "$src" "${ssh_host}:${dest}"
}

# Get value from environment file
get_env_value() {
    local env_file="$1"
    local key="$2"
    local default="${3:-}"

    local value
    if [[ -f "$env_file" ]]; then
        value=$(grep "^${key}=" "$env_file" 2>/dev/null | cut -d'=' -f2- | tr -d '\n\r' | sed 's/^"//' | sed 's/"$//')
    fi

    echo "${value:-$default}"
}

# Get domain name from env file
get_domain_from_env() {
    local env_file="${1:-$PROD_ENV_FILE}"
    get_env_value "$env_file" "DOMAIN_NAME" "example.com"
}
