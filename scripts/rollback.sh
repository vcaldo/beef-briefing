#!/bin/bash
# Rollback beef-briefing to previous deployment
# Usage: ./scripts/rollback.sh [--force]

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# ARGUMENT PARSING
# =============================================================================
FORCE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --force|-f)
            FORCE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -f, --force  Skip confirmation prompt"
            echo "  -h, --help   Show this help message"
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

# Get SSH host
log_info "Getting SSH host from Terraform..."
SSH_HOST=$(get_ssh_host)
log_success "SSH host: $SSH_HOST"

# =============================================================================
# GET PREVIOUS TAG
# =============================================================================
log_step "Checking for previous deployment..."

# Get previous tag from server
PREVIOUS_TAG=$(remote_exec "$SSH_HOST" "cat $REMOTE_PREVIOUS_TAG_FILE 2>/dev/null" || echo "")

if [[ -z "$PREVIOUS_TAG" ]]; then
    log_error "No previous deployment found. Cannot rollback."
    log_info "The .previous_tag file does not exist on the server."
    log_info "This could mean:"
    log_info "  - This is the first deployment"
    log_info "  - The previous tag file was deleted"
    log_info "  - A rollback was already performed"
    exit 1
fi

# Get current tag
CURRENT_TAG=$(remote_exec "$SSH_HOST" "grep '^IMAGE_TAG=' ~/beef-briefing/.env | cut -d'=' -f2" || echo "unknown")

log_success "Previous tag found: $PREVIOUS_TAG"
log_info "Current tag: $CURRENT_TAG"

# =============================================================================
# VERIFY PREVIOUS IMAGES EXIST
# =============================================================================
log_step "Verifying previous images exist on server..."

MISSING_IMAGES=()
for image in "${IMAGES[@]}"; do
    if ! remote_exec "$SSH_HOST" "docker images '$image:$PREVIOUS_TAG' --format '{{.ID}}' | grep -q ."; then
        MISSING_IMAGES+=("$image:$PREVIOUS_TAG")
    fi
done

if [[ ${#MISSING_IMAGES[@]} -gt 0 ]]; then
    log_error "Previous images not found on server:"
    for img in "${MISSING_IMAGES[@]}"; do
        echo "  - $img"
    done
    log_error "Cannot rollback without the previous images."
    log_info "You may need to redeploy from the commit: $PREVIOUS_TAG"
    exit 1
fi

log_success "All previous images verified"

# =============================================================================
# CONFIRMATION
# =============================================================================
if [[ "$FORCE" == "false" ]]; then
    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  ROLLBACK CONFIRMATION${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  Current version:  $CURRENT_TAG"
    echo "  Rollback to:      $PREVIOUS_TAG"
    echo "  Server:           $SSH_HOST"
    echo ""
    read -p "  Are you sure you want to rollback? [y/N] " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Rollback cancelled"
        exit 0
    fi
fi

# =============================================================================
# PERFORM ROLLBACK
# =============================================================================
log_step "Performing rollback to $PREVIOUS_TAG..."

remote_exec "$SSH_HOST" "
    set -e
    cd ~/beef-briefing

    # Update IMAGE_TAG in .env
    sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=$PREVIOUS_TAG/' .env

    # Restart services with previous images
    docker compose up -d

    # Clear the previous tag file (prevent double rollback)
    rm -f .previous_tag

    echo 'Rollback complete!'
"

log_success "Rollback completed"

# =============================================================================
# UPDATE LOCAL ENV FILE
# =============================================================================
log_step "Updating local .env.prod..."
if [[ -f "$PROD_ENV_FILE" ]]; then
    set_env_image_tag "$PROD_ENV_FILE" "$PREVIOUS_TAG"
    log_success "Updated local IMAGE_TAG to $PREVIOUS_TAG"
fi

# =============================================================================
# SUMMARY
# =============================================================================
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ROLLBACK SUCCESSFUL${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Rolled back to:  $PREVIOUS_TAG"
echo "  Previous:        $CURRENT_TAG"
echo "  Server:          $SSH_HOST"
echo ""
echo -e "${YELLOW}  NOTE: The .previous_tag file has been cleared.${NC}"
echo -e "${YELLOW}  Another rollback is not possible until a new deployment.${NC}"
echo ""
