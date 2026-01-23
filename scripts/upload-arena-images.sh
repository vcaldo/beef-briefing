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

# Manifest file name for version tracking
MANIFEST_FILE=".manifest.json"

# =============================================================================
# VERSION MANAGEMENT FUNCTIONS
# =============================================================================

# Generate content hash of all image files
# Creates a deterministic hash that changes only when file contents change
generate_content_hash() {
    if [[ ! -d "$IMAGES_DIR" ]]; then
        echo ""
        return
    fi

    # Hash all .png files recursively, sort for determinism, then hash the result
    find "$IMAGES_DIR" -name "*.png" -exec sha256sum {} \; 2>/dev/null | sort | sha256sum | cut -d' ' -f1
}

# Get deployed manifest from object storage
# Returns JSON or empty object if not found
get_deployed_manifest() {
    local bucket="$MINIO_BUCKET"
    local manifest_path="images/arena/${MANIFEST_FILE}"

    mc cat "${MC_ALIAS}/${bucket}/${manifest_path}" 2>/dev/null || echo '{}'
}

# Extract value from JSON (simple parser without jq dependency)
json_get() {
    local json="$1"
    local key="$2"
    echo "$json" | grep -o "\"${key}\"[[:space:]]*:[[:space:]]*[^,}]*" | sed 's/.*:[[:space:]]*"\?\([^",}]*\)"\?.*/\1/'
}

# Update version in environment file
bump_env_version() {
    local env_file="$1"
    local var_name="$2"
    local new_version="$3"

    if [[ -f "$env_file" ]]; then
        sed -i "s/^${var_name}=.*/${var_name}=${new_version}/" "$env_file"
        log_success "Updated ${var_name}=${new_version} in $(basename "$env_file")"
    else
        log_warn "Environment file not found: $env_file"
    fi
}

# Upload manifest file to object storage
upload_manifest() {
    local bucket="$MINIO_BUCKET"
    local manifest_path="images/arena/${MANIFEST_FILE}"
    local version="$1"
    local hash="$2"
    local file_count="$3"
    local uploaded_at
    uploaded_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Create temporary manifest file
    local temp_manifest
    temp_manifest=$(mktemp)
    cat > "$temp_manifest" << EOF
{
  "version": ${version},
  "hash": "${hash}",
  "uploaded_at": "${uploaded_at}",
  "file_count": ${file_count}
}
EOF

    echo -n "  Uploading manifest... "
    if mc cp "$temp_manifest" "${MC_ALIAS}/${bucket}/${manifest_path}" \
        --attr "Content-Type=application/json;Cache-Control=no-cache" \
        &>/dev/null; then
        echo -e "${GREEN}done${NC}"
    else
        echo -e "${RED}failed${NC}"
        log_warn "Failed to upload manifest"
    fi

    rm -f "$temp_manifest"
}

# Check and auto-bump version if content changed
check_and_bump_version() {
    local env_file="$1"
    local var_name="IMAGE_VERSION"

    log_info "Checking for content changes..."

    # Generate local content hash
    local local_hash
    local_hash=$(generate_content_hash)

    if [[ -z "$local_hash" ]]; then
        log_warn "Could not generate content hash"
        return 1
    fi

    # Get deployed manifest
    local manifest
    manifest=$(get_deployed_manifest)

    local deployed_hash
    local deployed_version
    deployed_hash=$(json_get "$manifest" "hash")
    deployed_version=$(json_get "$manifest" "version")

    # Default version to 0 if not found
    if [[ -z "$deployed_version" ]] || [[ "$deployed_version" == "null" ]]; then
        deployed_version=0
    fi

    # Compare hashes
    if [[ "$local_hash" == "$deployed_hash" ]]; then
        log_info "Content unchanged (hash: ${local_hash:0:12}...)"
        log_info "${var_name} stays at ${deployed_version}"
        echo "$deployed_version" > /tmp/arena_image_version
        echo "$local_hash" > /tmp/arena_image_hash
        return 0
    fi

    # Content changed - bump version
    local new_version=$((deployed_version + 1))

    if [[ -n "$deployed_hash" ]] && [[ "$deployed_hash" != "null" ]]; then
        log_info "Content hash changed: ${deployed_hash:0:12}... -> ${local_hash:0:12}..."
    else
        log_info "No previous manifest found, initializing version"
    fi

    log_info "Bumping ${var_name}: ${deployed_version} -> ${new_version}"
    bump_env_version "$env_file" "$var_name" "$new_version"

    echo "$new_version" > /tmp/arena_image_version
    echo "$local_hash" > /tmp/arena_image_hash
    return 0
}

# =============================================================================
# FUNCTIONS
# =============================================================================

show_usage() {
    echo "Usage: $0 [--dev|--prod] [--list|--clean]"
    echo ""
    echo "Options:"
    echo "  --dev   Upload to development MinIO (default)"
    echo "  --prod  Upload to production Linode Object Storage"
    echo "  --list  List uploaded images"
    echo "  --clean Delete all images from storage"
    echo ""
    echo "Examples:"
    echo "  $0 --dev      # Upload to local MinIO"
    echo "  $0 --prod     # Upload to production"
    echo "  $0 --list     # List uploaded images"
    echo "  $0 --clean    # Delete all images from dev storage"
    echo "  $0            # Same as --dev"
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

# Delete all images from storage
clean_images() {
    local bucket="$MINIO_BUCKET"
    local base_target_path="images/arena"

    log_warn "This will delete ALL images from ${MC_ALIAS}/${bucket}/${base_target_path}/"
    echo ""

    # Count total files
    local total_count=0
    for category in "${IMAGE_CATEGORIES[@]}"; do
        local target_path="$base_target_path/$category"
        local count
        count=$(mc ls "${MC_ALIAS}/${bucket}/${target_path}/" 2>/dev/null | wc -l)
        total_count=$((total_count + count))
    done

    if [[ "$total_count" -eq 0 ]]; then
        log_info "No files to delete"
        return 0
    fi

    log_info "Found $total_count file(s) to delete across all categories"
    echo ""

    # Delete all files in the path (recursively)
    if mc rm --recursive --force "${MC_ALIAS}/${bucket}/${base_target_path}/" &>/dev/null; then
        log_success "Deleted all images from storage"
    else
        log_error "Failed to delete images"
        exit 1
    fi
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
        --clean)
            ACTION="clean"
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

# Set step title based on action
STEP_TITLE="Arena Images"
case "$ACTION" in
    upload) STEP_TITLE="Arena Images Upload" ;;
    list)   STEP_TITLE="Arena Images List" ;;
    clean)  STEP_TITLE="Arena Images Clean" ;;
esac
log_step "$STEP_TITLE ($ENV_MODE)"

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
case "$ACTION" in
    list)
        list_images
        ;;
    clean)
        clean_images
        ;;
    upload)
        # Determine which env file to update
        ENV_FILE_TO_UPDATE="$DEV_ENV_FILE"
        if [[ "$ENV_MODE" == "prod" ]]; then
            ENV_FILE_TO_UPDATE="$PROD_ENV_FILE"
        fi

        # Check for content changes and auto-bump version if needed
        check_and_bump_version "$ENV_FILE_TO_UPDATE"

        # Upload image files
        upload_images

        # Upload manifest with version info
        FINAL_VERSION=$(cat /tmp/arena_image_version 2>/dev/null || echo "1")
        FINAL_HASH=$(cat /tmp/arena_image_hash 2>/dev/null || echo "")
        FILE_COUNT=$(find "$IMAGES_DIR" -name "*.png" | wc -l)

        upload_manifest "$FINAL_VERSION" "$FINAL_HASH" "$FILE_COUNT"

        # Cleanup temp files
        rm -f /tmp/arena_image_version /tmp/arena_image_hash

        echo ""
        log_success "Images uploaded successfully!"
        echo ""
        log_info "Files are available at: images/arena/{category}/{filename}.png"
        log_info "Current IMAGE_VERSION: ${FINAL_VERSION}"
        ;;
esac
