#!/bin/bash
# Layer-aware Docker image transfer using OCI directory format + rsync
# Usage: source this file, then call transfer_images
#
# This script transfers Docker images to a remote server by:
# 1. Exporting images to OCI directory format (content-addressed blobs)
# 2. Using rsync to transfer only changed blobs
# 3. Loading images from the OCI directory on the remote server
#
# This is much faster than the traditional docker save | gzip | scp approach
# when only a few layers have changed (e.g., code-only changes).

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
LAYER_CACHE_DIR="${LAYER_CACHE_DIR:-/tmp/beef-briefing-oci-cache}"
REMOTE_LAYER_CACHE_DIR="${REMOTE_LAYER_CACHE_DIR:-~/beef-briefing/.oci-cache}"

# =============================================================================
# OCI DIRECTORY FUNCTIONS
# =============================================================================

# Extract images to OCI directory format
# Args: $1 = tag
# Returns: path to OCI directory (via stdout)
prepare_oci_directory() {
    local tag="$1"
    local oci_dir="${LAYER_CACHE_DIR}/${tag}"

    # Clean up any existing directory for this tag
    rm -rf "$oci_dir"
    mkdir -p "$oci_dir"

    # Build image list from IMAGES array (defined in common.sh)
    local image_args=""
    for image in "${IMAGES[@]}"; do
        image_args="$image_args $image:$tag"
    done

    # Redirect log output to stderr so it doesn't get captured with the return value
    log_info "Extracting images to OCI directory..." >&2
    # docker save produces OCI-layout format, extract to directory
    docker save $image_args | tar -xf - -C "$oci_dir"

    # Count blobs and calculate size
    local blob_count=0
    local total_size="0"
    if [[ -d "$oci_dir/blobs/sha256" ]]; then
        blob_count=$(ls -1 "$oci_dir/blobs/sha256/" 2>/dev/null | wc -l)
        total_size=$(du -sh "$oci_dir" 2>/dev/null | cut -f1)
    fi

    log_success "OCI directory ready: $blob_count blobs, $total_size total" >&2
    # Return the directory path (only this goes to stdout)
    echo "$oci_dir"
}

# Transfer OCI directory to remote using rsync
# Args: $1 = oci_dir, $2 = ssh_host, $3 = tag
transfer_oci_directory() {
    local oci_dir="$1"
    local ssh_host="$2"
    local tag="$3"
    local remote_dir="${REMOTE_LAYER_CACHE_DIR}/${tag}"

    log_info "Creating remote directory structure..."
    remote_exec "$ssh_host" "mkdir -p $remote_dir/blobs/sha256"

    log_info "Transferring layers with rsync (only changed blobs)..."

    # rsync options:
    # -a: archive mode (preserves permissions, timestamps)
    # -v: verbose output
    # -z: compress during transfer
    # --progress: show transfer progress
    # --checksum: compare by checksum instead of mod-time/size
    #             This ensures unchanged blobs are skipped even if timestamps differ
    # -e "ssh -o Compression=no": use SSH without compression (rsync -z handles it)
    rsync -avz --progress --checksum \
        -e "ssh -o Compression=no" \
        "$oci_dir/" \
        "${ssh_host}:${remote_dir}/"

    log_success "Transfer complete"
}

# Load images on remote from OCI directory
# Args: $1 = ssh_host, $2 = tag
load_images_remote() {
    local ssh_host="$1"
    local tag="$2"
    local remote_dir="${REMOTE_LAYER_CACHE_DIR}/${tag}"

    log_info "Loading images on remote from OCI directory..."
    remote_exec "$ssh_host" "
        set -e
        cd $remote_dir

        # docker load expects a tar stream, create it from the directory
        tar -cf - . | docker load

        echo 'Images loaded successfully'
    "
    log_success "Images loaded on remote"
}

# Main transfer function - orchestrates the full transfer process
# Args: $1 = tag (commit hash), $2 = ssh_host
transfer_images() {
    local tag="$1"
    local ssh_host="$2"

    log_step "Preparing OCI directory..."
    local oci_dir
    oci_dir=$(prepare_oci_directory "$tag")

    log_step "Transferring to remote (rsync)..."
    transfer_oci_directory "$oci_dir" "$ssh_host" "$tag"

    log_step "Loading images on remote..."
    load_images_remote "$ssh_host" "$tag"
}

# =============================================================================
# CACHE CLEANUP FUNCTIONS
# =============================================================================

# Cleanup local OCI cache, keeping the N most recent tags
# Args: $1 = number of tags to keep (default: 2)
cleanup_local_cache() {
    local keep_tags="${1:-2}"

    if [[ ! -d "$LAYER_CACHE_DIR" ]]; then
        return 0
    fi

    log_info "Cleaning local OCI cache (keeping last $keep_tags)..."

    # List directories sorted by modification time, remove oldest
    local dir_count
    dir_count=$(ls -1 "$LAYER_CACHE_DIR" 2>/dev/null | wc -l)

    if [[ "$dir_count" -gt "$keep_tags" ]]; then
        ls -t "$LAYER_CACHE_DIR" | tail -n +$((keep_tags + 1)) | while read -r dir; do
            log_info "  Removing old cache: $dir"
            rm -rf "${LAYER_CACHE_DIR}/${dir}"
        done
    fi
}

# Cleanup remote OCI cache, keeping the N most recent tags
# Args: $1 = ssh_host, $2 = number of tags to keep (default: 2)
cleanup_remote_cache() {
    local ssh_host="$1"
    local keep_tags="${2:-2}"

    log_info "Cleaning remote OCI cache (keeping last $keep_tags)..."
    remote_exec "$ssh_host" "
        if [[ -d $REMOTE_LAYER_CACHE_DIR ]]; then
            dir_count=\$(ls -1 $REMOTE_LAYER_CACHE_DIR 2>/dev/null | wc -l)
            if [[ \"\$dir_count\" -gt $keep_tags ]]; then
                ls -t $REMOTE_LAYER_CACHE_DIR | tail -n +$((keep_tags + 1)) | while read dir; do
                    echo \"  Removing old cache: \$dir\"
                    rm -rf \"${REMOTE_LAYER_CACHE_DIR}/\$dir\"
                done
            fi
        fi
    " || log_warn "Remote cache cleanup had warnings (non-fatal)"
}

# =============================================================================
# STATISTICS FUNCTIONS
# =============================================================================

# Show local and remote cache statistics
# Args: $1 = ssh_host
show_cache_stats() {
    local ssh_host="$1"

    echo ""
    echo "Layer Cache Statistics"
    echo "======================"
    echo ""
    echo "Local cache ($LAYER_CACHE_DIR):"
    if [[ -d "$LAYER_CACHE_DIR" ]]; then
        ls -1 "$LAYER_CACHE_DIR" 2>/dev/null | while read -r dir; do
            local size
            size=$(du -sh "${LAYER_CACHE_DIR}/${dir}" 2>/dev/null | cut -f1)
            local blobs=0
            if [[ -d "${LAYER_CACHE_DIR}/${dir}/blobs/sha256" ]]; then
                blobs=$(ls -1 "${LAYER_CACHE_DIR}/${dir}/blobs/sha256" 2>/dev/null | wc -l)
            fi
            echo "  $dir: $size ($blobs blobs)"
        done
    else
        echo "  (no cache)"
    fi

    echo ""
    echo "Remote cache ($REMOTE_LAYER_CACHE_DIR):"
    remote_exec "$ssh_host" "
        if [[ -d $REMOTE_LAYER_CACHE_DIR ]]; then
            ls -1 $REMOTE_LAYER_CACHE_DIR 2>/dev/null | while read dir; do
                size=\$(du -sh \"${REMOTE_LAYER_CACHE_DIR}/\$dir\" 2>/dev/null | cut -f1)
                blobs=0
                if [[ -d \"${REMOTE_LAYER_CACHE_DIR}/\$dir/blobs/sha256\" ]]; then
                    blobs=\$(ls -1 \"${REMOTE_LAYER_CACHE_DIR}/\$dir/blobs/sha256\" 2>/dev/null | wc -l)
                fi
                echo \"  \$dir: \$size (\$blobs blobs)\"
            done
        else
            echo '  (no cache)'
        fi
    " 2>/dev/null || echo "  (unable to connect)"
    echo ""
}
