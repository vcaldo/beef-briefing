#!/bin/bash
# Setup Terraform configuration from environment file
# Usage: ./scripts/tf-setup.sh [ENV_FILE]
#
# Copies terraform.tfvars.example to terraform.tfvars and populates values
# from the environment file (.env.prod or .env.dev)

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================
TFVARS_FILE="$TERRAFORM_DIR/terraform.tfvars"
TFVARS_EXAMPLE="$TERRAFORM_DIR/terraform.tfvars.example"

# Determine which env file to use
if [[ -n "$1" ]]; then
    ENV_FILE="$1"
elif [[ -f "$PROD_ENV_FILE" ]]; then
    ENV_FILE="$PROD_ENV_FILE"
elif [[ -f "$PROJECT_ROOT/infrastructure/.env.dev" ]]; then
    ENV_FILE="$PROJECT_ROOT/infrastructure/.env.dev"
else
    log_error "No environment file found. Please create .env.prod or .env.dev"
    exit 1
fi

# =============================================================================
# FUNCTIONS
# =============================================================================

# Get value from env file
get_env() {
    local key="$1"
    grep "^${key}=" "$ENV_FILE" 2>/dev/null | cut -d'=' -f2 | tr -d '\n\r'
}

# Update terraform.tfvars with a value
update_tfvar() {
    local pattern="$1"
    local replacement="$2"
    local description="$3"
    local value="$4"

    if [[ -n "$value" ]]; then
        sed -i "s|$pattern|$replacement|" "$TFVARS_FILE"
        log_success "Populated $description"
    fi
}

# =============================================================================
# MAIN
# =============================================================================

log_step "Setting up Terraform configuration..."

# Check if tfvars already exists
if [[ -f "$TFVARS_FILE" ]]; then
    log_warn "terraform.tfvars already exists at $TFVARS_FILE"
    log_warn "Delete it first to recreate, or edit it manually."
    exit 0
fi

# Check example file exists
if [[ ! -f "$TFVARS_EXAMPLE" ]]; then
    log_error "terraform.tfvars.example not found at $TFVARS_EXAMPLE"
    exit 1
fi

# Copy example file
log_info "Creating terraform.tfvars from example..."
cp "$TFVARS_EXAMPLE" "$TFVARS_FILE"
log_success "Created terraform.tfvars"

log_info "Using environment file: $ENV_FILE"

# Read values from env file
LINODE_TOKEN=$(get_env "LINODE_TOKEN")
LINODE_REGION=$(get_env "LINODE_REGION")
LINODE_INSTANCE_TYPE=$(get_env "LINODE_INSTANCE_TYPE")
LINODE_HOSTNAME=$(get_env "LINODE_HOSTNAME")
DOMAIN_NAME=$(get_env "DOMAIN_NAME")
NEW_RELIC_KEY=$(get_env "NEW_RELIC_API_KEY")
NEW_RELIC_ACCOUNT=$(get_env "NEW_RELIC_ACCOUNT_ID")
NEW_RELIC_REGION=$(get_env "NEW_RELIC_REGION")
SSH_KEY_PATH=$(get_env "SSH_PUBLIC_KEY_PATH")
POSTGRES_VOLUME_SIZE=$(get_env "POSTGRES_VOLUME_SIZE")
POSTGRES_DATA_PATH=$(get_env "POSTGRES_DATA_PATH")
OBJECT_STORAGE_REGION=$(get_env "OBJECT_STORAGE_REGION")
OBJECT_STORAGE_BUCKET_SUFFIX=$(get_env "OBJECT_STORAGE_BUCKET_SUFFIX")
OBJECT_STORAGE_VERSIONING=$(get_env "OBJECT_STORAGE_VERSIONING")
OBJECT_STORAGE_LIFECYCLE_DAYS=$(get_env "OBJECT_STORAGE_LIFECYCLE_DAYS")
OBJECT_STORAGE_VERSION_RETENTION_DAYS=$(get_env "OBJECT_STORAGE_VERSION_RETENTION_DAYS")

# Update tfvars file
log_step "Populating terraform.tfvars..."

# Required: Linode token
if [[ -n "$LINODE_TOKEN" ]]; then
    sed -i "s|linode_token = \".*\"|linode_token = \"$LINODE_TOKEN\"|" "$TFVARS_FILE"
    log_success "Populated linode_token"
else
    log_warn "LINODE_TOKEN not found in $ENV_FILE"
fi

# Optional values (uncomment and set)
update_tfvar '# region = ".*"' "region = \"$LINODE_REGION\"" "region" "$LINODE_REGION"
update_tfvar '# instance_type = ".*"' "instance_type = \"$LINODE_INSTANCE_TYPE\"" "instance_type" "$LINODE_INSTANCE_TYPE"
update_tfvar '# hostname = ".*"' "hostname = \"$LINODE_HOSTNAME\"" "hostname" "$LINODE_HOSTNAME"
update_tfvar '# domain_name = ".*"' "domain_name = \"$DOMAIN_NAME\"" "domain_name" "$DOMAIN_NAME"

# Domain email derived from domain name
if [[ -n "$DOMAIN_NAME" ]]; then
    sed -i "s|# domain_email = \".*\"|domain_email = \"admin@$DOMAIN_NAME\"|" "$TFVARS_FILE"
    log_success "Populated domain_email as admin@$DOMAIN_NAME"
fi

update_tfvar '# new_relic_license_key = ".*"' "new_relic_license_key = \"$NEW_RELIC_KEY\"" "new_relic_license_key" "$NEW_RELIC_KEY"
update_tfvar '# new_relic_account_id = ".*"' "new_relic_account_id = \"$NEW_RELIC_ACCOUNT\"" "new_relic_account_id" "$NEW_RELIC_ACCOUNT"
update_tfvar '# new_relic_region = ".*"' "new_relic_region = \"$NEW_RELIC_REGION\"" "new_relic_region" "$NEW_RELIC_REGION"
update_tfvar '# ssh_public_key_path = ".*"' "ssh_public_key_path = \"$SSH_KEY_PATH\"" "ssh_public_key_path" "$SSH_KEY_PATH"

# Numeric values (no quotes)
if [[ -n "$POSTGRES_VOLUME_SIZE" ]]; then
    sed -i "s|# postgres_volume_size = .*|postgres_volume_size = $POSTGRES_VOLUME_SIZE|" "$TFVARS_FILE"
    log_success "Populated postgres_volume_size"
fi

update_tfvar '# postgres_volume_mount_path = ".*"' "postgres_volume_mount_path = \"$POSTGRES_DATA_PATH\"" "postgres_volume_mount_path" "$POSTGRES_DATA_PATH"
update_tfvar '# object_storage_region = ".*"' "object_storage_region = \"$OBJECT_STORAGE_REGION\"" "object_storage_region" "$OBJECT_STORAGE_REGION"
update_tfvar '# object_storage_bucket_suffix = ".*"' "object_storage_bucket_suffix = \"$OBJECT_STORAGE_BUCKET_SUFFIX\"" "object_storage_bucket_suffix" "$OBJECT_STORAGE_BUCKET_SUFFIX"

# Boolean/numeric values (no quotes)
if [[ -n "$OBJECT_STORAGE_VERSIONING" ]]; then
    sed -i "s|# object_storage_versioning = .*|object_storage_versioning = $OBJECT_STORAGE_VERSIONING|" "$TFVARS_FILE"
    log_success "Populated object_storage_versioning"
fi

if [[ -n "$OBJECT_STORAGE_LIFECYCLE_DAYS" ]]; then
    sed -i "s|# object_storage_lifecycle_expiration_days = .*|object_storage_lifecycle_expiration_days = $OBJECT_STORAGE_LIFECYCLE_DAYS|" "$TFVARS_FILE"
    log_success "Populated object_storage_lifecycle_expiration_days"
fi

if [[ -n "$OBJECT_STORAGE_VERSION_RETENTION_DAYS" ]]; then
    sed -i "s|# object_storage_noncurrent_version_expiration_days = .*|object_storage_noncurrent_version_expiration_days = $OBJECT_STORAGE_VERSION_RETENTION_DAYS|" "$TFVARS_FILE"
    log_success "Populated object_storage_noncurrent_version_expiration_days"
fi

echo ""
log_success "Setup complete!"
echo ""
echo "  Review the configuration at:"
echo "    $TFVARS_FILE"
echo ""
echo "  Next steps:"
echo "    make tf-init    # Initialize Terraform"
echo "    make tf-plan    # Review changes"
echo "    make tf-apply   # Apply infrastructure"
echo ""
