#!/bin/bash
# Sync Object Storage credentials from Terraform to .env.prod
# Usage: ./scripts/tf-sync-object-storage.sh [ENV_FILE]
#
# Reads Terraform outputs and updates the environment file with
# Linode Object Storage credentials (MINIO_* variables)

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================
TARGET_ENV_FILE="${1:-$PROD_ENV_FILE}"

# =============================================================================
# FUNCTIONS
# =============================================================================

# Get Terraform output value
get_tf_output() {
    local key="$1"
    cd "$TERRAFORM_DIR"
    terraform output -raw "$key" 2>/dev/null
}

# Update or append env variable
set_env_var() {
    local file="$1"
    local key="$2"
    local value="$3"

    # Escape special characters for sed
    local escaped_value
    escaped_value=$(printf '%s\n' "$value" | sed 's/[&/\]/\\&/g')

    if grep -q "^${key}=" "$file"; then
        sed -i "s|^${key}=.*|${key}=${escaped_value}|" "$file"
    else
        echo "${key}=${value}" >> "$file"
    fi
}

# =============================================================================
# MAIN
# =============================================================================

log_step "Syncing Object Storage credentials..."

# Check Terraform state exists
if [[ ! -f "$TERRAFORM_DIR/terraform.tfstate" ]]; then
    log_error "No Terraform state found. Run 'make tf-apply' first."
    exit 1
fi

# Check target env file exists
if [[ ! -f "$TARGET_ENV_FILE" ]]; then
    log_error "Environment file not found: $TARGET_ENV_FILE"
    exit 1
fi

log_info "Reading Terraform outputs..."

# Get values from Terraform
ENDPOINT=$(get_tf_output "object_storage_endpoint")
ACCESS_KEY=$(get_tf_output "object_storage_access_key_id")
SECRET_KEY=$(get_tf_output "object_storage_secret_access_key")
BUCKET_NAME=$(get_tf_output "object_storage_bucket_name")

if [[ -z "$ENDPOINT" ]]; then
    log_error "Failed to get object_storage_endpoint from Terraform"
    exit 1
fi

log_info "Updating $TARGET_ENV_FILE..."

# Update env file
set_env_var "$TARGET_ENV_FILE" "MINIO_ENDPOINT" "$ENDPOINT"
set_env_var "$TARGET_ENV_FILE" "MINIO_ACCESS_KEY" "$ACCESS_KEY"
set_env_var "$TARGET_ENV_FILE" "MINIO_SECRET_KEY" "$SECRET_KEY"
set_env_var "$TARGET_ENV_FILE" "MINIO_USE_SSL" "true"
set_env_var "$TARGET_ENV_FILE" "MINIO_BUCKET" "$BUCKET_NAME"

log_success "Updated MINIO_ENDPOINT=$ENDPOINT"
log_success "Updated MINIO_ACCESS_KEY"
log_success "Updated MINIO_SECRET_KEY"
log_success "Updated MINIO_USE_SSL=true"
log_success "Updated MINIO_BUCKET=$BUCKET_NAME"

echo ""
log_success "Sync complete!"
echo ""
