#!/bin/bash
# Display summary tables for various operations
# Usage: ./scripts/show-summary.sh <mode> [options]
# Modes: dev, prod, build

set -e

# Load common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================
MODE="${1:-dev}"
shift || true

# Service configuration (name, port, url_path)
declare -A DEV_SERVICES=(
    ["api-service"]="8080|http://localhost:8080|/health"
    ["telegram-bot"]="-|-|"
    ["leaderboard"]="8050|http://localhost:8050|/health"
    ["ml-dashboard"]="8501|http://localhost:8501|"
    ["postgres"]="5432|localhost:5432|"
    ["minio"]="9000|http://localhost:9000|"
)

declare -A PROD_SERVICES=(
    ["api-service"]="-|https://api.\${DOMAIN}|"
    ["telegram-bot"]="-|-|"
    ["leaderboard"]="-|https://\${DOMAIN}\${LEADERBOARD_PATH}|"
    ["traefik"]="-|https://\${DOMAIN}/dashboard|"
)

# Image names for build summary
# Docker Compose uses directory name as project prefix
# In dev: infrastructure-{service}:latest
# In prod: beef-briefing/{service}:{tag}
DEV_IMAGES=(
    "infrastructure-api-service"
    "infrastructure-telegram-bot"
    "infrastructure-leaderboard"
    "infrastructure-ml-dashboard"
)

PROD_IMAGES=(
    "beef-briefing/api-service"
    "beef-briefing/telegram-bot"
    "beef-briefing/leaderboard"
    "beef-briefing/card-image-generator"
    "beef-briefing/deck-mini-app"
)

# =============================================================================
# TABLE DRAWING FUNCTIONS
# =============================================================================

# Box drawing characters
BOX_TL="+"  # Top-left
BOX_TR="+"  # Top-right
BOX_BL="+"  # Bottom-left
BOX_BR="+"  # Bottom-right
BOX_H="-"   # Horizontal
BOX_V="|"   # Vertical
BOX_TJ="+"  # Top junction
BOX_BJ="+"  # Bottom junction
BOX_LJ="+"  # Left junction
BOX_RJ="+"  # Right junction
BOX_CJ="+"  # Center junction

# Print horizontal line
print_line() {
    local widths=("$@")
    local first=true
    printf "%s" "$BOX_LJ"
    for w in "${widths[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            printf "%s" "$BOX_CJ"
        fi
        printf "%${w}s" | tr ' ' "$BOX_H"
    done
    printf "%s\n" "$BOX_RJ"
}

# Print top border
print_top() {
    local widths=("$@")
    local first=true
    printf "%s" "$BOX_TL"
    for w in "${widths[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            printf "%s" "$BOX_TJ"
        fi
        printf "%${w}s" | tr ' ' "$BOX_H"
    done
    printf "%s\n" "$BOX_TR"
}

# Print bottom border
print_bottom() {
    local widths=("$@")
    local first=true
    printf "%s" "$BOX_BL"
    for w in "${widths[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            printf "%s" "$BOX_BJ"
        fi
        printf "%${w}s" | tr ' ' "$BOX_H"
    done
    printf "%s\n" "$BOX_BR"
}

# Print row with values
print_row() {
    local widths_str="$1"
    shift
    local values=("$@")
    IFS=',' read -ra widths <<< "$widths_str"

    printf "%s" "$BOX_V"
    for i in "${!values[@]}"; do
        printf " %-$((${widths[$i]}-2))s %s" "${values[$i]}" "$BOX_V"
    done
    printf "\n"
}

# Print centered title row
print_title() {
    local total_width="$1"
    local title="$2"
    local padding=$(( (total_width - ${#title} - 2) / 2 ))
    printf "%s%*s%s%*s%s\n" "$BOX_V" "$padding" "" "$title" "$((total_width - padding - ${#title} - 2))" "" "$BOX_V"
}

# =============================================================================
# SUMMARY FUNCTIONS
# =============================================================================

# Get container status
get_container_status() {
    local service="$1"
    local compose_file="${2:-$COMPOSE_FILE}"
    local env_file="${3:-$ENV_FILE}"

    local status
    status=$(docker compose -f "$compose_file" --env-file "$env_file" ps --format "{{.Status}}" "$service" 2>/dev/null | head -1)

    if [[ -z "$status" ]]; then
        echo "not started"
    elif [[ "$status" == *"Up"* ]]; then
        if [[ "$status" == *"healthy"* ]]; then
            echo "healthy"
        elif [[ "$status" == *"unhealthy"* ]]; then
            echo "unhealthy"
        else
            echo "running"
        fi
    elif [[ "$status" == *"Exited"* ]]; then
        echo "exited"
    else
        echo "unknown"
    fi
}

# Color status text
color_status() {
    local status="$1"
    case "$status" in
        running|healthy|deployed)
            echo -e "${GREEN}$status${NC}"
            ;;
        unhealthy|exited|failed)
            echo -e "${RED}$status${NC}"
            ;;
        "not started")
            echo -e "${YELLOW}$status${NC}"
            ;;
        *)
            echo "$status"
            ;;
    esac
}

# Show development environment summary
show_dev_summary() {
    local compose_file="${1:-infrastructure/docker-compose.dev.yml}"
    local env_file="${2:-infrastructure/.env.dev}"

    echo ""
    echo -e "${GREEN}+================================================================+${NC}"
    echo -e "${GREEN}|              DEVELOPMENT ENVIRONMENT READY                    |${NC}"
    echo -e "${GREEN}+==================+===========+=======+========================+${NC}"
    echo -e "${GREEN}| Service          | Status    | Port  | URL                    |${NC}"
    echo -e "${GREEN}+==================+===========+=======+========================+${NC}"

    for service in api-service telegram-bot leaderboard ml-dashboard postgres minio; do
        local info="${DEV_SERVICES[$service]}"
        IFS='|' read -r port url _ <<< "$info"

        local status
        status=$(get_container_status "$service" "$compose_file" "$env_file")
        local status_colored
        status_colored=$(color_status "$status")

        printf "${GREEN}|${NC} %-16s ${GREEN}|${NC} %-9s ${GREEN}|${NC} %-5s ${GREEN}|${NC} %-22s ${GREEN}|${NC}\n" \
            "$service" "$status_colored" "$port" "$url"
    done

    echo -e "${GREEN}+==================+===========+=======+========================+${NC}"
    echo ""
}

# Show production deployment summary
show_prod_summary() {
    local domain="$1"
    local image_tag="$2"
    local ssh_host="$3"
    local previous_tag="${4:-N/A}"
    local allowed_ip="${5:-N/A}"

    echo ""
    echo -e "${GREEN}+====================================================================+${NC}"
    echo -e "${GREEN}|                    PRODUCTION DEPLOYMENT COMPLETE                  |${NC}"
    echo -e "${GREEN}+==================+==========+======================================+${NC}"
    echo -e "${GREEN}| Service          | Status   | URL                                  |${NC}"
    echo -e "${GREEN}+==================+==========+======================================+${NC}"

    printf "${GREEN}|${NC} %-16s ${GREEN}|${NC} ${GREEN}%-8s${NC} ${GREEN}|${NC} %-36s ${GREEN}|${NC}\n" \
        "api-service" "deployed" "https://api.${domain}"
    printf "${GREEN}|${NC} %-16s ${GREEN}|${NC} ${GREEN}%-8s${NC} ${GREEN}|${NC} %-36s ${GREEN}|${NC}\n" \
        "telegram-bot" "deployed" "-"
    printf "${GREEN}|${NC} %-16s ${GREEN}|${NC} ${GREEN}%-8s${NC} ${GREEN}|${NC} %-36s ${GREEN}|${NC}\n" \
        "leaderboard" "deployed" "https://${domain}${LEADERBOARD_PATH:-/leaderboard}"
    printf "${GREEN}|${NC} %-16s ${GREEN}|${NC} ${GREEN}%-8s${NC} ${GREEN}|${NC} %-36s ${GREEN}|${NC}\n" \
        "traefik" "deployed" "https://${domain}/dashboard"

    echo -e "${GREEN}+==================+==========+======================================+${NC}"
    echo -e "${GREEN}| Image Tag: ${NC}${BLUE}${image_tag}${NC}${GREEN} | Server: ${NC}${BLUE}${ssh_host}${NC}"
    echo -e "${GREEN}| Previous:  ${NC}${YELLOW}${previous_tag}${NC}${GREEN} | Rollback: ${NC}make prod-rollback"
    echo -e "${GREEN}| API IP Allowlist: ${NC}${BLUE}${allowed_ip}${NC}"
    echo -e "${GREEN}+====================================================================+${NC}"
    echo ""
}

# Show build summary
show_build_summary() {
    local image_tag="${1:-latest}"
    local start_time="$2"
    local end_time="$3"

    echo ""
    echo -e "${GREEN}+====================================================================+${NC}"
    echo -e "${GREEN}|                         BUILD SUMMARY                              |${NC}"
    echo -e "${GREEN}+================================+============+======================+${NC}"
    echo -e "${GREEN}| Image                          | Size       | Tag                  |${NC}"
    echo -e "${GREEN}+================================+============+======================+${NC}"

    local total_size=0
    local total_size_raw=0

    # Use dev images (docker compose naming: infrastructure-{service})
    for image in "${DEV_IMAGES[@]}"; do
        local size
        local display_tag="latest"

        # Try with latest tag first (docker compose default)
        size=$(docker images "$image:latest" --format "{{.Size}}" 2>/dev/null | head -1)

        # If not found with latest, try with provided tag
        if [[ -z "$size" ]] && [[ "$image_tag" != "latest" ]]; then
            size=$(docker images "$image:$image_tag" --format "{{.Size}}" 2>/dev/null | head -1)
            display_tag="$image_tag"
        fi

        if [[ -z "$size" ]]; then
            size="not found"
        else
            # Extract numeric value for total
            local num
            num=$(echo "$size" | grep -oE '[0-9.]+' | head -1)
            local unit
            unit=$(echo "$size" | grep -oE '[A-Z]+' | head -1)
            if [[ -n "$num" ]]; then
                case "$unit" in
                    GB) total_size_raw=$(awk "BEGIN {print $total_size_raw + $num * 1024}") ;;
                    MB) total_size_raw=$(awk "BEGIN {print $total_size_raw + $num}") ;;
                    KB) total_size_raw=$(awk "BEGIN {print $total_size_raw + $num / 1024}") ;;
                esac
            fi
        fi

        printf "${GREEN}|${NC} %-30s ${GREEN}|${NC} %-10s ${GREEN}|${NC} %-20s ${GREEN}|${NC}\n" \
            "$image" "$size" "$display_tag"
    done

    echo -e "${GREEN}+================================+============+======================+${NC}"

    # Format total size
    total_size=$(printf "%.1f" "$total_size_raw")

    # Calculate duration if times provided
    if [[ -n "$start_time" && -n "$end_time" ]]; then
        local duration=$((end_time - start_time))
        echo -e "${GREEN}| Total Size: ~${total_size}MB | Duration: ${duration}s${NC}"
    else
        echo -e "${GREEN}| Total Size: ~${total_size}MB |${NC}"
    fi

    echo -e "${GREEN}+====================================================================+${NC}"
    echo ""
}

# =============================================================================
# MAIN
# =============================================================================

case "$MODE" in
    dev)
        COMPOSE_FILE="${COMPOSE_FILE:-infrastructure/docker-compose.dev.yml}"
        ENV_FILE="${ENV_FILE:-infrastructure/.env.dev}"
        show_dev_summary "$COMPOSE_FILE" "$ENV_FILE"
        ;;
    prod)
        DOMAIN="${DOMAIN:-example.com}"
        IMAGE_TAG="${IMAGE_TAG:-latest}"
        SSH_HOST="${SSH_HOST:-unknown}"
        PREVIOUS_TAG="${PREVIOUS_TAG:-N/A}"
        ALLOWED_IP="${ALLOWED_IP:-N/A}"
        show_prod_summary "$DOMAIN" "$IMAGE_TAG" "$SSH_HOST" "$PREVIOUS_TAG" "$ALLOWED_IP"
        ;;
    build)
        IMAGE_TAG="${IMAGE_TAG:-latest}"
        START_TIME="${START_TIME:-}"
        END_TIME="${END_TIME:-}"
        show_build_summary "$IMAGE_TAG" "$START_TIME" "$END_TIME"
        ;;
    *)
        echo "Usage: $0 <mode> [options]"
        echo "Modes:"
        echo "  dev   - Show development environment status"
        echo "  prod  - Show production deployment summary"
        echo "  build - Show build summary with image sizes"
        exit 1
        ;;
esac
