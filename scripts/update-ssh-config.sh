#!/bin/bash
# Update SSH config with server connection details
# Usage: ./scripts/update-ssh-config.sh
#
# Reads configuration from environment variables or .env file:
#   - LINODE_HOSTNAME: Host alias name (default: beef-briefing)
#   - SSH_PUBLIC_KEY_PATH: Path to SSH public key (default: ~/.ssh/id_rsa.pub)
#
# Gets IP address from Terraform output

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================
SSH_CONFIG_FILE="${HOME}/.ssh/config"
ENV_FILE="${ENV_FILE:-$PROJECT_ROOT/infrastructure/.env.prod}"

# =============================================================================
# FUNCTIONS
# =============================================================================

# Get value from env file
get_env_value() {
    local key="$1"
    local default="$2"
    local value

    if [[ -f "$ENV_FILE" ]]; then
        value=$(grep "^${key}=" "$ENV_FILE" 2>/dev/null | cut -d'=' -f2- | tr -d '\n\r' | sed 's/^"//' | sed 's/"$//')
    fi

    echo "${value:-$default}"
}

# Get IP from Terraform
get_terraform_ip() {
    cd "$TERRAFORM_DIR"
    terraform output -raw instance_ip 2>/dev/null || {
        log_error "Failed to get IP from Terraform. Is infrastructure deployed?"
        exit 1
    }
}

# Derive private key path from public key path
get_private_key_path() {
    local public_key_path="$1"

    # Expand ~ to home directory
    public_key_path="${public_key_path/#\~/$HOME}"

    # Remove .pub extension if present
    if [[ "$public_key_path" == *.pub ]]; then
        echo "${public_key_path%.pub}"
    else
        echo "$public_key_path"
    fi
}

# Check if SSH config block exists for host
ssh_config_exists() {
    local host_alias="$1"

    if [[ ! -f "$SSH_CONFIG_FILE" ]]; then
        return 1
    fi

    grep -qE "^Host\s+${host_alias}\s*$" "$SSH_CONFIG_FILE"
}

# Remove existing SSH config block for host
remove_ssh_config_block() {
    local host_alias="$1"

    if [[ ! -f "$SSH_CONFIG_FILE" ]]; then
        return 0
    fi

    # Create temp file
    local temp_file
    temp_file=$(mktemp)

    # Use awk to remove the block
    awk -v host="$host_alias" '
        /^Host / {
            if ($2 == host) {
                skip = 1
                next
            } else {
                skip = 0
            }
        }
        /^[^ \t]/ && !/^Host / {
            skip = 0
        }
        !skip { print }
    ' "$SSH_CONFIG_FILE" > "$temp_file"

    # Replace original file
    mv "$temp_file" "$SSH_CONFIG_FILE"
}

# Add SSH config block
add_ssh_config_block() {
    local host_alias="$1"
    local hostname="$2"
    local user="$3"
    local identity_file="$4"

    # Ensure SSH directory exists
    mkdir -p "$(dirname "$SSH_CONFIG_FILE")"

    # Add newline if file exists and doesn't end with newline
    if [[ -f "$SSH_CONFIG_FILE" ]] && [[ -s "$SSH_CONFIG_FILE" ]]; then
        # Check if file ends with newline
        if [[ $(tail -c1 "$SSH_CONFIG_FILE" | wc -l) -eq 0 ]]; then
            echo "" >> "$SSH_CONFIG_FILE"
        fi
        echo "" >> "$SSH_CONFIG_FILE"
    fi

    # Append the new config block
    cat >> "$SSH_CONFIG_FILE" << EOF
Host ${host_alias}
    HostName ${hostname}
    User ${user}
    IdentityFile ${identity_file}
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR
EOF

    # Set proper permissions
    chmod 600 "$SSH_CONFIG_FILE"
}

# =============================================================================
# MAIN
# =============================================================================

log_step "Updating SSH config..."

# Get configuration
HOST_ALIAS=$(get_env_value "LINODE_HOSTNAME" "beef-briefing")
SSH_PUBLIC_KEY=$(get_env_value "SSH_PUBLIC_KEY_PATH" "~/.ssh/id_rsa.pub")
SSH_USER="admin"

log_info "Host alias: $HOST_ALIAS"

# Get IP from Terraform
log_info "Getting IP from Terraform..."
IP_ADDRESS=$(get_terraform_ip)
log_success "IP address: $IP_ADDRESS"

# Derive private key path
IDENTITY_FILE=$(get_private_key_path "$SSH_PUBLIC_KEY")
log_info "Identity file: $IDENTITY_FILE"

# Check if private key exists
IDENTITY_FILE_EXPANDED="${IDENTITY_FILE/#\~/$HOME}"
if [[ ! -f "$IDENTITY_FILE_EXPANDED" ]]; then
    log_warn "Private key not found at $IDENTITY_FILE"
    log_warn "Make sure the key exists before using SSH"
fi

# Update SSH config
if ssh_config_exists "$HOST_ALIAS"; then
    log_info "Updating existing SSH config for '$HOST_ALIAS'..."
    remove_ssh_config_block "$HOST_ALIAS"
else
    log_info "Adding new SSH config for '$HOST_ALIAS'..."
fi

add_ssh_config_block "$HOST_ALIAS" "$IP_ADDRESS" "$SSH_USER" "$IDENTITY_FILE"

log_success "SSH config updated successfully!"
echo ""
echo "  You can now connect using:"
echo "    ssh $HOST_ALIAS"
echo ""
echo "  Config added to: $SSH_CONFIG_FILE"
echo ""
