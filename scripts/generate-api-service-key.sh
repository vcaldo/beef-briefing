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

# Save to api-service/app_keys/{app} (for api-service to validate incoming requests)
API_SERVICE_DIR="$SECRETS_DIR/apps/api-service/app_keys"
mkdir -p "$API_SERVICE_DIR"
echo -n "$KEY" > "$API_SERVICE_DIR/$APP_NAME"
chmod 600 "$API_SERVICE_DIR/$APP_NAME"

# Save to {app}/api_key (for the app to use when making requests)
APP_DIR="$SECRETS_DIR/apps/$APP_NAME"
mkdir -p "$APP_DIR"
echo -n "$KEY" > "$APP_DIR/api_key"
chmod 600 "$APP_DIR/api_key"

echo "API key generated for: $APP_NAME"
echo "  api-service reads from: $API_SERVICE_DIR/$APP_NAME"
echo "  $APP_NAME reads from:   $APP_DIR/api_key"
