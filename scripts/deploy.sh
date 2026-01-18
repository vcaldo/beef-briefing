#!/bin/bash
# Deploy beef-briefing to production server
# Usage: ./scripts/deploy.sh [--skip-build] [--skip-cleanup]

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# ARGUMENT PARSING
# =============================================================================
SKIP_BUILD=false
SKIP_CLEANUP=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-build    Skip Docker image build (use existing images)"
            echo "  --skip-cleanup  Skip cleanup of old images on server"
            echo "  -h, --help      Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# =============================================================================
# PRE-FLIGHT CHECKS
# =============================================================================
log_step "Running pre-flight checks..."

# Check required files exist
require_file "$PROD_COMPOSE_FILE" "Production docker-compose file"
require_file "$PROD_ENV_FILE" "Production environment file"
require_dir "$SECRETS_DIR" "Secrets directory"

# Check for letsencrypt directory (optional - created on first run if missing)
LETSENCRYPT_DIR="$PROJECT_ROOT/infrastructure/letsencrypt"
if [[ -d "$LETSENCRYPT_DIR" ]]; then
    HAS_LETSENCRYPT=true
    log_info "Found letsencrypt directory (will transfer certificates)"
else
    HAS_LETSENCRYPT=false
    log_info "No letsencrypt directory found (will be created on server)"
fi

# Validate environment variables
log_info "Validating environment variables..."
validate_env_vars "$PROD_ENV_FILE"
log_success "Environment variables validated"

# Get SSH host
log_info "Getting SSH host from Terraform..."
SSH_HOST=$(get_ssh_host)
log_success "SSH host: $SSH_HOST"

# Get commit hash for tagging
COMMIT_HASH=$(get_commit_hash)
log_success "Commit hash: $COMMIT_HASH"

# Fetch deployer's IP for API allowlist
log_info "Fetching current IP address..."
ALLOWED_IP=$(curl -s whatismyip.akamai.com)
if [[ -z "$ALLOWED_IP" ]]; then
    log_error "Failed to fetch IP from whatismyip.akamai.com"
    exit 1
fi
log_success "Current IP: $ALLOWED_IP"

# Update ALLOWED_IP in .env.prod
if grep -q "^ALLOWED_IP=" "$PROD_ENV_FILE"; then
    sed -i "s/^ALLOWED_IP=.*/ALLOWED_IP=$ALLOWED_IP/" "$PROD_ENV_FILE"
else
    echo "ALLOWED_IP=$ALLOWED_IP" >> "$PROD_ENV_FILE"
fi
log_success "Updated ALLOWED_IP in .env.prod"

# Detect remote server architecture for cross-platform builds
log_info "Detecting remote server architecture..."
REMOTE_ARCH=$(get_remote_arch "$SSH_HOST")
HOST_ARCH=$(uname -m)
case "$HOST_ARCH" in
    x86_64)  HOST_GOARCH="amd64" ;;
    aarch64|arm64) HOST_GOARCH="arm64" ;;
    armv7l)  HOST_GOARCH="arm" ;;
    *)       HOST_GOARCH="amd64" ;;
esac

if [[ "$HOST_GOARCH" != "$REMOTE_ARCH" ]]; then
    log_warn "Cross-platform build: local $HOST_GOARCH -> remote $REMOTE_ARCH"
    DOCKER_PLATFORM="linux/$REMOTE_ARCH"
else
    log_success "Native build: $REMOTE_ARCH"
    DOCKER_PLATFORM="linux/$REMOTE_ARCH"
fi

# =============================================================================
# CHECK CACHE HEALTH
# =============================================================================
log_step "Checking remote cache health..."
check_cache_size "$SSH_HOST" 5  # Warn if cache > 5GB

# =============================================================================
# SAVE PREVIOUS TAG (for rollback)
# =============================================================================
log_step "Saving previous deployment tag for rollback..."

# Get current tag from remote server (if exists)
PREVIOUS_TAG=$(remote_exec "$SSH_HOST" "cat $REMOTE_APP_DIR/.env 2>/dev/null | grep '^IMAGE_TAG=' | cut -d'=' -f2" || echo "")

if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$COMMIT_HASH" ]]; then
    log_info "Previous tag: $PREVIOUS_TAG"
    remote_exec "$SSH_HOST" "mkdir -p ~/beef-briefing && echo '$PREVIOUS_TAG' > ~/beef-briefing/.previous_tag"
    log_success "Saved previous tag for rollback"
else
    log_warn "No previous tag to save (first deployment or same version)"
fi

# =============================================================================
# UPDATE LOCAL ENV FILE
# =============================================================================
log_step "Updating IMAGE_TAG in $PROD_ENV_FILE..."
set_env_image_tag "$PROD_ENV_FILE" "$COMMIT_HASH"
log_success "Updated IMAGE_TAG to $COMMIT_HASH"

# =============================================================================
# BUILD DOCKER IMAGES
# =============================================================================
if [[ "$SKIP_BUILD" == "false" ]]; then
    log_step "Building Docker images for platform $DOCKER_PLATFORM..."
    DOCKER_DEFAULT_PLATFORM="$DOCKER_PLATFORM" docker compose -f "$PROD_COMPOSE_FILE" --env-file "$PROD_ENV_FILE" build
    log_success "Docker images built for $DOCKER_PLATFORM"

    # Verify images exist
    log_info "Verifying built images..."
    for image in "${IMAGES[@]}"; do
        if ! docker images "$image:$COMMIT_HASH" --format "{{.ID}}" | grep -q .; then
            log_error "Image not found: $image:$COMMIT_HASH"
            exit 1
        fi
    done
    log_success "All images verified"
else
    log_warn "Skipping build (--skip-build)"
fi

# =============================================================================
# LAYER-AWARE IMAGE TRANSFER
# =============================================================================
# Source layer transfer functions
source "$SCRIPT_DIR/layer-transfer.sh"

# Transfer images using OCI directory format (only changed layers)
transfer_images "$COMMIT_HASH" "$SSH_HOST"

log_step "Transferring config files to server..."

# Transfer compose file
log_info "Transferring docker-compose.yml..."
remote_copy "$PROD_COMPOSE_FILE" "$SSH_HOST" "/tmp/docker-compose.yml"

# Transfer env file
log_info "Transferring .env..."
remote_copy "$PROD_ENV_FILE" "$SSH_HOST" "/tmp/.env"

# Transfer secrets
log_info "Transferring secrets..."
remote_copy "$SECRETS_DIR" "$SSH_HOST" "/tmp/"

# Transfer letsencrypt directory if it exists
if [[ "$HAS_LETSENCRYPT" == "true" ]]; then
    log_info "Transferring letsencrypt certificates..."
    remote_copy "$LETSENCRYPT_DIR" "$SSH_HOST" "/tmp/"
fi

log_success "All config files transferred"

# =============================================================================
# DEPLOY ON SERVER
# =============================================================================
log_step "Deploying on server..."

remote_exec "$SSH_HOST" "
    set -e

    # Create directories
    mkdir -p ~/beef-briefing

    # Move config files (can be done while containers are running)
    mv /tmp/docker-compose.yml ~/beef-briefing/
    mv /tmp/.env ~/beef-briefing/

    # Setup letsencrypt directory if transferred (can be done while containers are running)
    if [ -d /tmp/letsencrypt ]; then
        echo 'Setting up letsencrypt directory...'
        mkdir -p ~/beef-briefing/letsencrypt
        mv /tmp/letsencrypt/* ~/beef-briefing/letsencrypt/ 2>/dev/null || true
        rmdir /tmp/letsencrypt
    fi

    # Ensure letsencrypt directory exists with proper permissions
    mkdir -p ~/beef-briefing/letsencrypt
    chmod 700 ~/beef-briefing/letsencrypt

    # Create acme.json if it doesn't exist
    if [ ! -f ~/beef-briefing/letsencrypt/acme.json ]; then
        touch ~/beef-briefing/letsencrypt/acme.json
        chmod 600 ~/beef-briefing/letsencrypt/acme.json
        echo 'Created acme.json for Let'\''s Encrypt'
    fi

    # Images already loaded by layer-aware transfer (transfer_images function)
    # NOW stop containers (just before replacing secrets - minimizes downtime)
    echo 'Stopping containers for secrets update...'
    cd ~/beef-briefing && docker compose down 2>/dev/null || true

    # Remove old secrets with fallback for permission issues
    rm -rf ~/beef-briefing/secrets 2>/dev/null || sudo rm -rf ~/beef-briefing/secrets
    mkdir -p ~/beef-briefing/secrets
    mv /tmp/apps ~/beef-briefing/secrets/

    # Start services immediately after secrets are in place
    # Use --no-build since images are pre-loaded (build context doesn't exist on server)
    echo 'Starting services...'
    cd ~/beef-briefing && docker compose up -d --no-build

    echo 'Deployment complete!'
"

log_success "Services deployed with tag: $COMMIT_HASH"

# =============================================================================
# CLEANUP OLD IMAGES AND LAYER CACHE
# =============================================================================
if [[ "$SKIP_CLEANUP" == "false" ]]; then
    log_step "Cleaning up old images and layer cache..."

    # Cleanup Docker images (keep current and previous)
    if [[ -n "$PREVIOUS_TAG" ]]; then
        remote_exec "$SSH_HOST" "
            # Get all tags for beef-briefing images
            CURRENT_TAG='$COMMIT_HASH'
            PREVIOUS_TAG='$PREVIOUS_TAG'

            # Find and remove old images (keep only current and previous)
            for repo in beef-briefing/api-service beef-briefing/telegram-bot beef-briefing/card-renderer; do
                docker images \"\$repo\" --format '{{.Tag}}' | while read tag; do
                    if [[ \"\$tag\" != \"\$CURRENT_TAG\" && \"\$tag\" != \"\$PREVIOUS_TAG\" && \"\$tag\" != '<none>' ]]; then
                        echo \"Removing \$repo:\$tag\"
                        docker rmi \"\$repo:\$tag\" 2>/dev/null || true
                    fi
                done
            done

            # Prune dangling images
            docker image prune -f
        " || log_warn "Image cleanup had some warnings (non-fatal)"
    fi

    # Cleanup OCI layer cache (keep last 2 versions)
    cleanup_local_cache 2
    cleanup_remote_cache "$SSH_HOST" 2

    log_success "Cleanup complete"
else
    log_warn "Skipping cleanup (--skip-cleanup flag used)"
    log_warn "Cache will continue to grow - run 'make layer-cache-clean-remote' periodically"
fi

# =============================================================================
# SUMMARY
# =============================================================================
DOMAIN=$(get_domain_from_env "$PROD_ENV_FILE")
DOMAIN="$DOMAIN" IMAGE_TAG="$COMMIT_HASH" SSH_HOST="$SSH_HOST" PREVIOUS_TAG="${PREVIOUS_TAG:-N/A}" ALLOWED_IP="$ALLOWED_IP" \
    "$SCRIPT_DIR/show-summary.sh" prod
