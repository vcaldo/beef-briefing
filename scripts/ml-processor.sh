#!/usr/bin/env bash
# scripts/ml-processor.sh - Execute ml-processor commands in Docker container
#
# Usage:
#   ./scripts/ml-processor.sh [--prod] <command> [options]
#
# Examples:
#   ./scripts/ml-processor.sh process --limit 100       # Dev database
#   ./scripts/ml-processor.sh --prod process            # Prod database (SSH tunnel)

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Configuration
CONTAINER="beef-ml-processor-dev"
DEFAULT_CHAT_ID="-1002572302334"

# Production database (via SSH tunnel: make pg-tunnel)
PROD_DB_HOST="host.docker.internal"
PROD_DB_PORT="5433"

# Production card-renderer (external URL via Traefik)
PROD_CARD_RENDERER_URL="https://cards-api.barra-pesada.online"

# Environment overrides for docker exec
ENV_OVERRIDES=""

usage() {
    cat <<EOF
Usage: $0 [--prod] <command> [options]

Environment:
  --prod          Target production database (via SSH tunnel)
                  Requires: make pg-tunnel running in another terminal
                  Also uses external card-renderer URL for render command

Commands:
  process      Run batch processing
  status       Show processing status
  continuous   Run continuous processing (daemon mode)
  cards        Generate weekly user cards
  render       Render card images (requires card-renderer service)
  clean-cards  Delete user cards for a chat
  shell        Open interactive shell in container

Options (passed through to main.py):
  --chat-id ID           Chat ID (default: $DEFAULT_CHAT_ID)
  --source-chat-id ID    Source chat ID for messages (enables impersonation mode)
  --limit N              Limit messages to process
  --batch-size N         Messages per batch
  --from-date D          Process from date (YYYY-MM-DD)
  --to-date D            Process until date (YYYY-MM-DD)
  --week D               Week start for cards/render (YYYY-MM-DD)
  --window-days N        Rolling window for cards (default: 30)
  --min-messages N       Min messages for cards (default: 10)
  --theme T              Theme for card images (default: gaming)
  --card-type T          Card type for render (regular or compact, default: regular)
  --force                Force regenerate card images

Examples:
  $0 process --limit 100                    # Dev: local postgres
  $0 --prod process --limit 100             # Prod: via SSH tunnel
  $0 status                                 # Dev status
  $0 --prod status                          # Prod status
  $0 cards --week 2025-01-06                # Generate cards (dev)
  $0 render --week 2025-01-06               # Render regular card images (dev)
  $0 render --card-type compact             # Render compact card images
  $0 render --force                         # Force re-render images
  $0 cards --chat-id -1009876543 --source-chat-id -1001234567 --timezone UTC  # Impersonation mode
  $0 shell                                  # Open shell
EOF
    exit 1
}

# Check if container is running
check_container() {
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
        log_error "Container '$CONTAINER' is not running"
        log_info "Start it with: make up-build"
        exit 1
    fi
}

# Run docker exec with environment overrides
docker_exec() {
    # Check if TTY is available
    local docker_flags="-i"
    if [[ -t 0 ]]; then
        docker_flags="-it"
    fi

    if [[ -n "$ENV_OVERRIDES" ]]; then
        # shellcheck disable=SC2086
        docker exec $docker_flags $ENV_OVERRIDES "$CONTAINER" "$@"
    else
        docker exec $docker_flags "$CONTAINER" "$@"
    fi
}

# Check if --chat-id is in args
has_chat_id() {
    for arg in "$@"; do
        if [[ "$arg" == "--chat-id" ]]; then
            return 0
        fi
    done
    return 1
}

# Main
main() {
    if [[ $# -lt 1 ]]; then
        usage
    fi

    # Parse --prod flag
    if [[ "$1" == "--prod" ]]; then
        shift
        ENV_OVERRIDES="-e DB_HOST=$PROD_DB_HOST -e DB_PORT=$PROD_DB_PORT -e CARD_RENDERER_URL=$PROD_CARD_RENDERER_URL"
        log_info "Targeting PRODUCTION database (via SSH tunnel on port $PROD_DB_PORT)"
        log_info "Card renderer: $PROD_CARD_RENDERER_URL"
    fi

    if [[ $# -lt 1 ]]; then
        usage
    fi

    local command="$1"
    shift

    check_container

    case "$command" in
        process)
            if ! has_chat_id "$@"; then
                log_info "Using default chat-id: $DEFAULT_CHAT_ID"
                docker_exec python main.py process --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py process "$@"
            fi
            ;;

        status)
            if ! has_chat_id "$@"; then
                docker_exec python main.py status --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py status "$@"
            fi
            ;;

        continuous)
            if ! has_chat_id "$@"; then
                log_info "Starting continuous processing for chat: $DEFAULT_CHAT_ID"
                docker_exec python main.py continuous --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py continuous "$@"
            fi
            ;;

        cards)
            if ! has_chat_id "$@"; then
                docker_exec python main.py generate-cards --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py generate-cards "$@"
            fi
            ;;

        render)
            if ! has_chat_id "$@"; then
                docker_exec python main.py render-cards --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py render-cards "$@"
            fi
            ;;

        clean-cards)
            if ! has_chat_id "$@"; then
                docker_exec python main.py clean-cards --chat-id "$DEFAULT_CHAT_ID" "$@"
            else
                docker_exec python main.py clean-cards "$@"
            fi
            ;;

        shell)
            log_info "Opening shell in $CONTAINER..."
            docker_exec /bin/bash
            ;;

        help|--help|-h)
            usage
            ;;

        *)
            log_error "Unknown command: $command"
            usage
            ;;
    esac
}

main "$@"
