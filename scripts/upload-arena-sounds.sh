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

# Target sample rate for browser compatibility (44100 Hz is universally supported)
TARGET_SAMPLE_RATE=44100

# Manifest file name for version tracking
MANIFEST_FILE=".manifest.json"

# =============================================================================
# VERSION MANAGEMENT FUNCTIONS
# =============================================================================

# Generate content hash of all sound files
# Creates a deterministic hash that changes only when file contents change
generate_content_hash() {
    if [[ ! -d "$SOUNDS_DIR" ]]; then
        echo ""
        return
    fi

    # Hash all .ogg files, sort for determinism, then hash the result
    find "$SOUNDS_DIR" -name "*.ogg" -exec sha256sum {} \; 2>/dev/null | sort | sha256sum | cut -d' ' -f1
}

# Get deployed manifest from object storage
# Returns JSON or empty object if not found
get_deployed_manifest() {
    local bucket="$MINIO_BUCKET"
    local manifest_path="sounds/arena/${MANIFEST_FILE}"

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
    local manifest_path="sounds/arena/${MANIFEST_FILE}"
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
    local var_name="SOUND_VERSION"

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
        echo "$deployed_version" > /tmp/arena_sound_version
        echo "$local_hash" > /tmp/arena_sound_hash
        # Return 2 to signal "no upload needed"
        return 2
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

    echo "$new_version" > /tmp/arena_sound_version
    echo "$local_hash" > /tmp/arena_sound_hash
    return 0
}

# =============================================================================
# FUNCTIONS
# =============================================================================

# Normalize audio files for browser compatibility
# Re-encodes files with incompatible sample rates (e.g., 96000 Hz) to 44100 Hz
normalize_sounds() {
    if ! command -v ffprobe &> /dev/null || ! command -v ffmpeg &> /dev/null; then
        log_warn "ffmpeg/ffprobe not found, skipping audio normalization"
        return 0
    fi

    log_info "Checking audio files for browser compatibility..."
    local normalized=0

    for sound_file in "$SOUNDS_DIR"/*.ogg; do
        [[ ! -f "$sound_file" ]] && continue

        local filename
        filename=$(basename "$sound_file")

        # Get sample rate using ffprobe
        local sample_rate
        sample_rate=$(ffprobe -v quiet -select_streams a:0 -show_entries stream=sample_rate -of csv=p=0 "$sound_file" 2>/dev/null)

        # Check if sample rate is browser-compatible (44100 or 48000)
        if [[ -n "$sample_rate" ]] && [[ "$sample_rate" -ne 44100 ]] && [[ "$sample_rate" -ne 48000 ]]; then
            echo -n "  Normalizing $filename (${sample_rate}Hz -> ${TARGET_SAMPLE_RATE}Hz)... "

            local temp_file="${sound_file}.tmp.ogg"

            if ffmpeg -y -i "$sound_file" -ar "$TARGET_SAMPLE_RATE" -c:a libvorbis -q:a 6 "$temp_file" &>/dev/null; then
                mv "$temp_file" "$sound_file"
                echo -e "${GREEN}done${NC}"
                ((++normalized))
            else
                rm -f "$temp_file"
                echo -e "${RED}failed${NC}"
                log_warn "Could not normalize $filename, uploading as-is"
            fi
        fi
    done

    if [[ "$normalized" -gt 0 ]]; then
        log_success "Normalized $normalized file(s) to ${TARGET_SAMPLE_RATE}Hz"
    else
        log_info "All files have compatible sample rates"
    fi
    echo ""
}

show_usage() {
    echo "Usage: $0 [--dev|--prod] [--list|--clean]"
    echo ""
    echo "Options:"
    echo "  --dev   Upload to development MinIO (default)"
    echo "  --prod  Upload to production Linode Object Storage"
    echo "  --list  List uploaded sounds"
    echo "  --clean Delete all sounds from storage"
    echo ""
    echo "Examples:"
    echo "  $0 --dev      # Upload to local MinIO"
    echo "  $0 --prod     # Upload to production"
    echo "  $0 --clean    # Delete all sounds from dev storage"
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

# Delete all sounds from storage
clean_sounds() {
    local bucket="$MINIO_BUCKET"
    local target_path="sounds/arena"

    log_warn "This will delete ALL sounds from ${MC_ALIAS}/${bucket}/${target_path}/"
    echo ""

    # List what will be deleted
    local file_count
    file_count=$(mc ls "${MC_ALIAS}/${bucket}/${target_path}/" 2>/dev/null | wc -l)

    if [[ "$file_count" -eq 0 ]]; then
        log_info "No files to delete"
        return 0
    fi

    log_info "Found $file_count file(s) to delete"
    echo ""

    # Delete all files in the path
    if mc rm --recursive --force "${MC_ALIAS}/${bucket}/${target_path}/" &>/dev/null; then
        log_success "Deleted all sounds from storage"
    else
        log_error "Failed to delete sounds"
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
STEP_TITLE="Arena Sounds"
case "$ACTION" in
    upload) STEP_TITLE="Arena Sounds Upload" ;;
    list)   STEP_TITLE="Arena Sounds List" ;;
    clean)  STEP_TITLE="Arena Sounds Clean" ;;
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
        list_sounds
        ;;
    clean)
        clean_sounds
        ;;
    upload)
        # Determine which env file to update
        ENV_FILE_TO_UPDATE="$DEV_ENV_FILE"
        if [[ "$ENV_MODE" == "prod" ]]; then
            ENV_FILE_TO_UPDATE="$PROD_ENV_FILE"
        fi

        # Check for content changes and auto-bump version if needed
        # Returns 0 if content changed (proceed with upload)
        # Returns 2 if content unchanged (skip upload)
        check_and_bump_version "$ENV_FILE_TO_UPDATE"
        needs_upload=$?

        if [[ $needs_upload -eq 2 ]]; then
            log_success "Assets are up to date, skipping upload"
            # Cleanup temp files
            rm -f /tmp/arena_sound_version /tmp/arena_sound_hash
            exit 0
        fi

        # Normalize audio files for browser compatibility before uploading
        normalize_sounds

        # Upload sound files
        upload_sounds

        # Upload manifest with version info
        FINAL_VERSION=$(cat /tmp/arena_sound_version 2>/dev/null || echo "1")
        FINAL_HASH=$(cat /tmp/arena_sound_hash 2>/dev/null || echo "")
        FILE_COUNT=$(find "$SOUNDS_DIR" -name "*.ogg" | wc -l)

        upload_manifest "$FINAL_VERSION" "$FINAL_HASH" "$FILE_COUNT"

        # Cleanup temp files
        rm -f /tmp/arena_sound_version /tmp/arena_sound_hash

        echo ""
        log_success "Sounds uploaded successfully!"
        echo ""
        log_info "Files are available at: sounds/arena/{filename}.ogg"
        log_info "Current SOUND_VERSION: ${FINAL_VERSION}"
        ;;
esac
