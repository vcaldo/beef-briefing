#!/bin/bash
# Upload arena images to object storage
# Usage: ./scripts/upload-arena-images.sh [--dev|--prod]
#
# Uploads image files from apps/arena-mini-app/assets/images/ to
# {bucket}/images/arena/{category}/{filename}.png with appropriate headers.

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

DEV_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.dev"
IMAGES_DIR="$PROJECT_ROOT/apps/arena-mini-app/assets/images"
MC_ALIAS="arena-images-upload"

# Cache headers: 1 year (31536000 seconds)
CACHE_CONTROL="public,max-age=31536000"

# Image categories (subdirectories in assets/images/)
IMAGE_CATEGORIES=(
    "buttons"
    "panels"
    "bars"
    "icons"
    "effects"
    "effects/explosion"
)

# =============================================================================
# FUNCTIONS
# =============================================================================

show_usage() {
    echo "Usage: $0 [--dev|--prod|--list]"
    echo ""
    echo "Options:"
    echo "  --dev   Upload to development MinIO (default)"
    echo "  --prod  Upload to production Linode Object Storage"
    echo "  --list  List uploaded images"
    echo ""
    echo "Examples:"
    echo "  $0 --dev     # Upload to local MinIO"
    echo "  $0 --prod    # Upload to production"
    echo "  $0 --list    # List uploaded images"
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

# Upload all image files
upload_images() {
    local bucket="$MINIO_BUCKET"
    local base_target_path="images/arena"
    local uploaded=0
    local failed=0
    local total_files=0

    if [[ ! -d "$IMAGES_DIR" ]]; then
        log_error "Images directory not found: $IMAGES_DIR"
        exit 1
    fi

    # Count total files across all categories
    for category in "${IMAGE_CATEGORIES[@]}"; do
        local category_dir="$IMAGES_DIR/$category"
        if [[ -d "$category_dir" ]]; then
            local count
            count=$(find "$category_dir" -maxdepth 1 -name "*.png" | wc -l)
            total_files=$((total_files + count))
        fi
    done

    if [[ "$total_files" -eq 0 ]]; then
        log_warn "No PNG files found in $IMAGES_DIR"
        exit 0
    fi

    log_info "Found $total_files PNG files to upload across ${#IMAGE_CATEGORIES[@]} categories"
    log_info "Target: ${MC_ALIAS}/${bucket}/${base_target_path}/"

    echo ""

    # Upload files by category
    for category in "${IMAGE_CATEGORIES[@]}"; do
        local category_dir="$IMAGES_DIR/$category"

        if [[ ! -d "$category_dir" ]]; then
            log_warn "Category directory not found, skipping: $category"
            continue
        fi

        # Count files in this category
        local category_count
        category_count=$(find "$category_dir" -maxdepth 1 -name "*.png" | wc -l)

        if [[ "$category_count" -eq 0 ]]; then
            continue
        fi

        echo -e "${BLUE}Category: $category ($category_count files)${NC}"

        # Upload each PNG file in this category
        for image_file in "$category_dir"/*.png; do
            [[ ! -f "$image_file" ]] && continue

            local filename
            filename=$(basename "$image_file")

            # Target path: images/arena/{category}/{filename}.png
            local target_path="$base_target_path/$category/$filename"

            echo -n "  Uploading $filename... "

            if mc cp "$image_file" "${MC_ALIAS}/${bucket}/${target_path}" \
                --attr "Content-Type=image/png;Cache-Control=${CACHE_CONTROL};x-amz-acl=public-read" \
                &>/dev/null; then
                echo -e "${GREEN}done${NC}"
                ((++uploaded))
            else
                echo -e "${RED}failed${NC}"
                ((++failed))
            fi
        done

        echo ""
    done

    log_info "Upload complete: $uploaded succeeded, $failed failed"

    if [[ "$failed" -gt 0 ]]; then
        exit 1
    fi
}

# List uploaded images
list_images() {
    local bucket="$MINIO_BUCKET"
    local base_target_path="images/arena"

    log_info "Listing images at ${MC_ALIAS}/${bucket}/${base_target_path}/"
    echo ""

    for category in "${IMAGE_CATEGORIES[@]}"; do
        local target_path="$base_target_path/$category"
        echo -e "${BLUE}Category: $category${NC}"
        mc ls "${MC_ALIAS}/${bucket}/${target_path}/" 2>/dev/null || {
            log_warn "  No images found or path doesn't exist"
        }
        echo ""
    done
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

log_step "Arena Images Upload ($ENV_MODE)"

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
    list_images
else
    upload_images
    echo ""
    log_success "Images uploaded successfully!"
    echo ""
    log_info "Files are available at: images/arena/{category}/{filename}.png"
fi
