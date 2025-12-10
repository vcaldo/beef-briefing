#!/bin/bash
# Generate Traefik dashboard basic auth password and update .env.prod

# =============================================================================
# CONFIGURATION
# =============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROD_ENV_FILE="$PROJECT_ROOT/infrastructure/.env.prod"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================
log_error() {
    echo -e "${RED}✗ Error: $1${NC}" >&2
}

log_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠ Warning: $1${NC}"
}

log_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# =============================================================================
# MAIN FUNCTIONS
# =============================================================================

# Check if htpasswd command is available
check_htpasswd() {
    if ! command -v htpasswd >/dev/null 2>&1; then
        log_error "htpasswd command not found"
        echo ""
        echo "Install apache2-utils (Debian/Ubuntu):"
        echo "  sudo apt-get install apache2-utils"
        echo ""
        echo "Install httpd-tools (RHEL/CentOS/Fedora):"
        echo "  sudo yum install httpd-tools"
        echo "  # or: sudo dnf install httpd-tools"
        echo ""
        echo "Install apache2-utils (macOS with Homebrew):"
        echo "  brew install httpd"
        echo ""
        exit 1
    fi
}

# Prompt for username with default value
prompt_username() {
    local username
    echo -n "Enter dashboard username [admin]: " >&2
    read username
    username="${username:-admin}"
    echo "$username"
}

# Prompt for password with hidden input
prompt_password() {
    local password
    echo "Enter dashboard password:" >&2
    if [ -t 0 ]; then
        # Terminal is available - use stty to hide input
        stty -echo
        read password
        stty echo
    else
        # Non-interactive input - just read normally
        read password
    fi
    echo "$password"
}

# Prompt for password confirmation with hidden input
prompt_password_confirm() {
    local password_confirm
    echo "Confirm password:" >&2
    if [ -t 0 ]; then
        # Terminal is available - use stty to hide input
        stty -echo
        read password_confirm
        stty echo
    else
        # Non-interactive input - just read normally
        read password_confirm
    fi
    echo "$password_confirm"
}

# Validate that passwords match
validate_passwords_match() {
    local password="$1"
    local password_confirm="$2"

    if [ "$password" != "$password_confirm" ]; then
        log_error "Passwords do not match"
        exit 1
    fi
}

# Validate that password is not empty
validate_password_not_empty() {
    local password="$1"

    if [ -z "$password" ]; then
        log_error "Password cannot be empty"
        exit 1
    fi
}

# Warn if password is weak (less than 8 characters)
validate_password_strength() {
    local password="$1"

    if [ ${#password} -lt 8 ]; then
        log_warning "Password is less than 8 characters"
    fi
}

# Generate htpasswd entry with bcrypt hash
generate_htpasswd_entry() {
    local username="$1"
    local password="$2"

    htpasswd -nbB "$username" "$password"
}

# Update or create TRAEFIK_DASHBOARD_USERS in .env.prod
update_env_file() {
    local htpasswd_entry="$1"

    # Escape $ to $$ for docker-compose .env file compatibility
    # Docker compose interprets single $ as variable substitution
    local escaped_entry
    escaped_entry=$(echo "$htpasswd_entry" | sed 's/\$/\$\$/g')

    if grep -q '^TRAEFIK_DASHBOARD_USERS=' "$PROD_ENV_FILE"; then
        # Update existing entry
        awk -v entry="$escaped_entry" '/^TRAEFIK_DASHBOARD_USERS=/ {print "TRAEFIK_DASHBOARD_USERS=" entry; next} {print}' "$PROD_ENV_FILE" > "$PROD_ENV_FILE.tmp"
        if [ $? -ne 0 ]; then
            log_error "Failed to update $PROD_ENV_FILE"
            rm -f "$PROD_ENV_FILE.tmp"
            exit 1
        fi
        mv "$PROD_ENV_FILE.tmp" "$PROD_ENV_FILE"
        log_success "Updated TRAEFIK_DASHBOARD_USERS in $PROD_ENV_FILE"
    else
        # Add new entry
        echo "" >> "$PROD_ENV_FILE" || { log_error "Failed to write to $PROD_ENV_FILE"; exit 1; }
        echo "# Traefik dashboard basic auth (generated $(date '+%Y-%m-%d %H:%M:%S'))" >> "$PROD_ENV_FILE" || { log_error "Failed to write to $PROD_ENV_FILE"; exit 1; }
        echo "TRAEFIK_DASHBOARD_USERS=$escaped_entry" >> "$PROD_ENV_FILE" || { log_error "Failed to write to $PROD_ENV_FILE"; exit 1; }
        log_success "Added TRAEFIK_DASHBOARD_USERS to $PROD_ENV_FILE"
    fi
}

# Get dashboard URL from .env.prod
get_dashboard_url() {
    local domain
    domain=$(grep '^DOMAIN_NAME=' "$PROD_ENV_FILE" | cut -d'=' -f2 | tr -d '\n\r"')

    if [ -z "$domain" ]; then
        echo "https://{DOMAIN_NAME}/traefik-dashboard"
    else
        echo "https://$domain/traefik-dashboard"
    fi
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

main() {
    echo "========================================="
    echo "Traefik Dashboard Password Generator"
    echo "========================================="
    echo ""

    # Check prerequisites
    check_htpasswd

    if [ ! -f "$PROD_ENV_FILE" ]; then
        log_error "$PROD_ENV_FILE not found"
        exit 1
    fi

    # Prompt for credentials
    username=$(prompt_username)
    password=$(prompt_password)
    password_confirm=$(prompt_password_confirm)

    echo ""

    # Validate inputs
    validate_passwords_match "$password" "$password_confirm"
    validate_password_not_empty "$password"
    validate_password_strength "$password"

    # Generate bcrypt hash
    log_info "Generating htpasswd entry..."
    htpasswd_entry=$(generate_htpasswd_entry "$username" "$password")

    if [ $? -ne 0 ]; then
        log_error "Failed to generate htpasswd entry"
        exit 1
    fi

    echo ""

    # Update .env.prod
    update_env_file "$htpasswd_entry"

    echo ""
    dashboard_url=$(get_dashboard_url)
    echo "Dashboard will be accessible at: $dashboard_url"
    echo "Username: $username"
}

main "$@"
