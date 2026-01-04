#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${1:-}"
SECRETS_DIR="${2:-infrastructure/secrets}"

if [ -z "$APP_NAME" ]; then
    echo "Error: APP_NAME required"
    echo "Usage: $0 <app-name> [secrets-dir]"
    exit 1
fi

# Generate key
KEY=$(openssl rand -base64 32)

# Save to card-renderer/app_keys/{app} (for card-renderer to validate incoming requests)
CARD_GEN_DIR="$SECRETS_DIR/apps/card-renderer/app_keys"
mkdir -p "$CARD_GEN_DIR"
echo -n "$KEY" > "$CARD_GEN_DIR/$APP_NAME"
chmod 600 "$CARD_GEN_DIR/$APP_NAME"

# Save to {app}/card_renderer_api_key (for the app to use when making requests)
APP_DIR="$SECRETS_DIR/apps/$APP_NAME"
mkdir -p "$APP_DIR"
echo -n "$KEY" > "$APP_DIR/card_renderer_api_key"
chmod 600 "$APP_DIR/card_renderer_api_key"

echo "API key generated for: $APP_NAME"
echo "  card-renderer reads from: $CARD_GEN_DIR/$APP_NAME"
echo "  $APP_NAME reads from:            $APP_DIR/card_renderer_api_key"
