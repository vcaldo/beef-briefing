#!/bin/bash
# Build import-cli for remote architecture and deploy to production server
# Usage: ./scripts/go-build-import-cli-prod.sh
#
# Detects the remote server architecture, cross-compiles the import-cli
# binary, and deploys it to the production server.

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================
IMPORT_CLI_DIR="$PROJECT_ROOT/apps/import-cli"
REMOTE_BIN_DIR="~/beef-briefing/apps/import-cli/bin"

# =============================================================================
# MAIN
# =============================================================================

log_step "Building import-cli for production server..."

# Get SSH host
log_info "Getting SSH host from Terraform..."
SSH_HOST=$(get_ssh_host)
log_success "SSH host: $SSH_HOST"

# Detect remote architecture
log_info "Detecting remote server architecture..."
REMOTE_ARCH_RAW=$(ssh "$SSH_HOST" 'uname -m' 2>/dev/null || echo "x86_64")

case "$REMOTE_ARCH_RAW" in
    x86_64)  REMOTE_ARCH="amd64" ;;
    aarch64) REMOTE_ARCH="arm64" ;;
    armv7l)  REMOTE_ARCH="arm" ;;
    *)       REMOTE_ARCH="amd64" ;;
esac

log_success "Remote architecture: $REMOTE_ARCH_RAW (GOARCH: $REMOTE_ARCH)"

# Check local architecture
HOST_ARCH=$(go env GOHOSTARCH)
if [[ "$HOST_ARCH" != "$REMOTE_ARCH" ]]; then
    log_warn "Cross-compiling from $HOST_ARCH to $REMOTE_ARCH"
else
    log_info "Native build: $REMOTE_ARCH"
fi

# Build binary
log_step "Building import-cli binary..."
cd "$IMPORT_CLI_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH="$REMOTE_ARCH" go build -a -installsuffix cgo -o bin/import-cli ./cmd
log_success "Binary built: $IMPORT_CLI_DIR/bin/import-cli"

# Create remote directory
log_step "Deploying to production server..."
log_info "Creating remote directory..."
ssh "$SSH_HOST" "mkdir -p $REMOTE_BIN_DIR"

# Copy binary
log_info "Copying binary..."
scp "$IMPORT_CLI_DIR/bin/import-cli" "${SSH_HOST}:${REMOTE_BIN_DIR}/"

# Make executable
ssh "$SSH_HOST" "chmod +x ${REMOTE_BIN_DIR}/import-cli"

log_success "Binary deployed to ${SSH_HOST}:${REMOTE_BIN_DIR}/import-cli"

echo ""
echo "  To use the import-cli on the server:"
echo "    ssh $SSH_HOST"
echo "    cd ~/beef-briefing/apps/import-cli"
echo "    ./bin/import-cli --help"
echo ""
