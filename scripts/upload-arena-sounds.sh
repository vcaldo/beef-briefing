#!/bin/bash
# Upload arena sounds to object storage
# Usage: ./scripts/upload-arena-sounds.sh [--dev|--prod]
#
# Uploads sound files from apps/arena-mini-app/assets/sounds/ to
# {bucket}/sounds/arena/{filename}.ogg with appropriate headers.

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

DEV_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.dev"
SOUNDS_DIR="$PROJECT_ROOT/apps/arena-mini-app/assets/sounds"
MC_ALIAS="arena-sounds-upload"

# Cache headers: 1 year (31536000 seconds)
CACHE_CONTROL="public,max-age=31536000"

# =============================================================================
# FUNCTIONS
# =============================================================================

show_usage() {
    echo "Usage: $0 [--dev|--prod]"
    echo ""
    echo "Options:"
    echo "  --dev   Upload to development MinIO (default)"
    echo "  --prod  Upload to production Linode Object Storage"
    echo ""
    echo "Examples:"
    echo "  $0 --dev     # Upload to local MinIO"
    echo "  $0 --prod    # Upload to production"
    echo "  $0           # Same as --dev"
}

# Load environment variables from file
load_env_file() {
    local env_file="$1"

    if [[ ! -f "$env_file" ]]; then
        log_error "Environment file not found: $env_file"
        exit 1
    fi

    log_info "Loading environment from: $env_file"

    # Export variables from env file (skip comments and empty lines)
    set -a
    while IFS='=' read -r key value; do
        # Skip comments and empty lines
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        # Remove leading/trailing whitespace from key
        key=$(echo "$key" | xargs)
        # Skip if no key
        [[ -z "$key" ]] && continue
        # Remove quotes from value
        value=$(echo "$value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
        export "$key=$value"
    done < "$env_file"
    set +a
}

# Configure mc alias based on environment
configure_mc_alias() {
    local use_ssl="$1"
    local protocol="http"

    if [[ "$use_ssl" == "true" ]]; then
        protocol="https"
    fi

    log_info "Configuring mc alias '$MC_ALIAS'..."
    log_info "Endpoint: ${protocol}://${MINIO_ENDPOINT}"

    mc alias set "$MC_ALIAS" "${protocol}://${MINIO_ENDPOINT}" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --quiet

    log_success "Alias configured"
}

# Upload all sound files
upload_sounds() {
    local bucket="$MINIO_BUCKET"
    local target_path="sounds/arena"
    local uploaded=0
    local failed=0

    if [[ ! -d "$SOUNDS_DIR" ]]; then
        log_error "Sounds directory not found: $SOUNDS_DIR"
        exit 1
    fi

    # Count files
    local total_files
    total_files=$(find "$SOUNDS_DIR" -name "*.ogg" | wc -l)

    if [[ "$total_files" -eq 0 ]]; then
        log_warn "No OGG files found in $SOUNDS_DIR"
        exit 0
    fi

    log_info "Found $total_files OGG files to upload"
    log_info "Target: ${MC_ALIAS}/${bucket}/${target_path}/"

    echo ""

    for sound_file in "$SOUNDS_DIR"/*.ogg; do
        local filename
        filename=$(basename "$sound_file")

        echo -n "  Uploading $filename... "

        if mc cp "$sound_file" "${MC_ALIAS}/${bucket}/${target_path}/${filename}" \
            --attr "Content-Type=audio/ogg;Cache-Control=${CACHE_CONTROL};x-amz-acl=public-read" \
            &>/dev/null; then
            echo -e "${GREEN}done${NC}"
            ((++uploaded))
        else
            echo -e "${RED}failed${NC}"
            ((++failed))
        fi
    done

    echo ""
    log_info "Upload complete: $uploaded succeeded, $failed failed"

    if [[ "$failed" -gt 0 ]]; then
        exit 1
    fi
}

# List uploaded sounds
list_sounds() {
    local bucket="$MINIO_BUCKET"
    local target_path="sounds/arena"

    log_info "Listing sounds at ${MC_ALIAS}/${bucket}/${target_path}/"
    echo ""
    mc ls "${MC_ALIAS}/${bucket}/${target_path}/" 2>/dev/null || {
        log_warn "No sounds found or path doesn't exist"
    }
}

# =============================================================================
# MAIN
# =============================================================================

# Parse arguments
ENV_MODE="dev"
ACTION="upload"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dev)
            ENV_MODE="dev"
            shift
            ;;
        --prod)
            ENV_MODE="prod"
            shift
            ;;
        --list)
            ACTION="list"
            shift
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

log_step "Arena Sounds Upload ($ENV_MODE)"

# Check mc is installed
if ! command -v mc &> /dev/null; then
    log_error "MinIO Client (mc) is not installed"
    log_info "Install with: brew install minio-mc (macOS) or see https://min.io/docs/minio/linux/reference/minio-mc.html"
    exit 1
fi

# Load environment and configure
if [[ "$ENV_MODE" == "prod" ]]; then
    load_env_file "$PROD_ENV_FILE"
    configure_mc_alias "${MINIO_USE_SSL:-true}"
else
    load_env_file "$DEV_ENV_FILE"
    configure_mc_alias "${MINIO_USE_SSL:-false}"
fi

# Execute action
if [[ "$ACTION" == "list" ]]; then
    list_sounds
else
    upload_sounds
    echo ""
    log_success "Sounds uploaded successfully!"
    echo ""
    log_info "Files are available at: sounds/arena/{filename}.ogg"
fi
