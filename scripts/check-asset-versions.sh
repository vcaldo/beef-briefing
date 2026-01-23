#!/bin/bash
# Check arena asset versions (read-only)
# Usage: ./scripts/check-asset-versions.sh [--dev|--prod]
#
# Compares local asset content hashes with deployed manifests to detect changes.
# Does not upload or modify any files.

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

DEV_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.dev"
SOUNDS_DIR="$PROJECT_ROOT/apps/arena-mini-app/assets/sounds"
IMAGES_DIR="$PROJECT_ROOT/apps/arena-mini-app/assets/images"
MC_ALIAS="arena-version-check"

# Manifest file name
MANIFEST_FILE=".manifest.json"

# =============================================================================
# FUNCTIONS
# =============================================================================

show_usage() {
    echo "Usage: $0 [--dev|--prod]"
    echo ""
    echo "Options:"
    echo "  --dev   Check development storage (MinIO) - default"
    echo "  --prod  Check production storage (Linode Object Storage)"
    echo ""
    echo "Examples:"
    echo "  $0 --dev     # Check local MinIO"
    echo "  $0 --prod    # Check production"
    echo "  $0           # Same as --dev"
}

# Load environment variables from file
load_env_file() {
    local env_file="$1"

    if [[ ! -f "$env_file" ]]; then
        log_error "Environment file not found: $env_file"
        exit 1
    fi

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

    mc alias set "$MC_ALIAS" "${protocol}://${MINIO_ENDPOINT}" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --quiet 2>/dev/null || {
        log_error "Failed to configure storage access"
        exit 1
    }
}

# Generate content hash of sound files
generate_sounds_hash() {
    if [[ ! -d "$SOUNDS_DIR" ]]; then
        echo ""
        return
    fi
    find "$SOUNDS_DIR" -name "*.ogg" -exec sha256sum {} \; 2>/dev/null | sort | sha256sum | cut -d' ' -f1
}

# Generate content hash of image files
generate_images_hash() {
    if [[ ! -d "$IMAGES_DIR" ]]; then
        echo ""
        return
    fi
    find "$IMAGES_DIR" -name "*.png" -exec sha256sum {} \; 2>/dev/null | sort | sha256sum | cut -d' ' -f1
}

# Get deployed manifest from object storage
get_manifest() {
    local asset_type="$1"  # "sounds" or "images"
    local bucket="$MINIO_BUCKET"
    local manifest_path="${asset_type}/arena/${MANIFEST_FILE}"

    mc cat "${MC_ALIAS}/${bucket}/${manifest_path}" 2>/dev/null || echo '{}'
}

# Extract value from JSON (simple parser without jq dependency)
json_get() {
    local json="$1"
    local key="$2"
    echo "$json" | grep -o "\"${key}\"[[:space:]]*:[[:space:]]*[^,}]*" | sed 's/.*:[[:space:]]*"\?\([^",}]*\)"\?.*/\1/'
}

# Check and display status for an asset type
check_asset_type() {
    local asset_type="$1"
    local local_hash="$2"
    local version_var="$3"
    local env_file="$4"

    # Get deployed manifest
    local manifest
    manifest=$(get_manifest "$asset_type")

    local deployed_hash
    local deployed_version
    local deployed_at
    local file_count
    deployed_hash=$(json_get "$manifest" "hash")
    deployed_version=$(json_get "$manifest" "version")
    deployed_at=$(json_get "$manifest" "uploaded_at")
    file_count=$(json_get "$manifest" "file_count")

    # Get env file version
    local env_version
    env_version=$(grep "^${version_var}=" "$env_file" 2>/dev/null | cut -d'=' -f2 | tr -d '\n\r')

    # Determine status
    local status
    local status_color
    if [[ -z "$deployed_hash" ]] || [[ "$deployed_hash" == "null" ]]; then
        status="NOT DEPLOYED"
        status_color="${YELLOW}"
        deployed_version="n/a"
        deployed_hash="n/a"
    elif [[ "$local_hash" == "$deployed_hash" ]]; then
        status="UP TO DATE"
        status_color="${GREEN}"
    else
        status="NEEDS UPDATE"
        status_color="${RED}"
    fi

    # Display results
    echo -e "${BLUE}${asset_type^}:${NC}"
    echo -e "  Deployed version:  ${deployed_version:-n/a}"
    echo -e "  Env file version:  ${env_version:-not set} (${version_var})"
    echo -e "  Deployed hash:     ${deployed_hash:0:16}${deployed_hash:+...}"
    echo -e "  Local hash:        ${local_hash:0:16}..."
    if [[ -n "$deployed_at" ]] && [[ "$deployed_at" != "null" ]]; then
        echo -e "  Last uploaded:     ${deployed_at}"
    fi
    if [[ -n "$file_count" ]] && [[ "$file_count" != "null" ]]; then
        echo -e "  File count:        ${file_count}"
    fi
    echo -e "  Status:            ${status_color}${status}${NC}"
    echo ""

    # Return status for summary
    if [[ "$status" == "NEEDS UPDATE" ]]; then
        return 1
    fi
    return 0
}

# =============================================================================
# MAIN
# =============================================================================

# Parse arguments
ENV_MODE="dev"

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

log_step "Arena Asset Version Check ($ENV_MODE)"

# Check mc is installed
if ! command -v mc &> /dev/null; then
    log_error "MinIO Client (mc) is not installed"
    log_info "Install with: brew install minio-mc (macOS) or see https://min.io/docs/minio/linux/reference/minio-mc.html"
    exit 1
fi

# Load environment and configure
ENV_FILE_TO_CHECK="$DEV_ENV_FILE"
if [[ "$ENV_MODE" == "prod" ]]; then
    load_env_file "$PROD_ENV_FILE"
    ENV_FILE_TO_CHECK="$PROD_ENV_FILE"
    configure_mc_alias "${MINIO_USE_SSL:-true}"
else
    load_env_file "$DEV_ENV_FILE"
    configure_mc_alias "${MINIO_USE_SSL:-false}"
fi

echo ""

# Generate local hashes
log_info "Generating local content hashes..."
SOUNDS_HASH=$(generate_sounds_hash)
IMAGES_HASH=$(generate_images_hash)
echo ""

# Check each asset type
SOUNDS_OK=true
IMAGES_OK=true

check_asset_type "sounds" "$SOUNDS_HASH" "SOUND_VERSION" "$ENV_FILE_TO_CHECK" || SOUNDS_OK=false
check_asset_type "images" "$IMAGES_HASH" "IMAGE_VERSION" "$ENV_FILE_TO_CHECK" || IMAGES_OK=false

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [[ "$SOUNDS_OK" == "true" ]] && [[ "$IMAGES_OK" == "true" ]]; then
    log_success "All assets are up to date!"
else
    log_warn "Some assets need updating. Run:"
    if [[ "$SOUNDS_OK" == "false" ]]; then
        if [[ "$ENV_MODE" == "prod" ]]; then
            echo "  make arena-sounds-upload-prod"
        else
            echo "  make arena-sounds-upload"
        fi
    fi
    if [[ "$IMAGES_OK" == "false" ]]; then
        if [[ "$ENV_MODE" == "prod" ]]; then
            echo "  make arena-images-upload-prod"
        else
            echo "  make arena-images-upload"
        fi
    fi
fi
