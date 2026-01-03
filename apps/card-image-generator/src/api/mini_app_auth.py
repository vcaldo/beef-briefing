"""Mini App authentication module.

Validates Telegram Mini App init data and issues JWT tokens for authenticated sessions.
"""

import hashlib
import hmac
import json
import time
import urllib.parse
from datetime import datetime, timedelta, timezone
from typing import Any

import jwt
from pydantic import BaseModel

# JWT settings
JWT_ALGORITHM = "HS256"
JWT_EXPIRATION_HOURS = 24


class InitDataValidation(BaseModel):
    """Validated init data from Telegram Mini App."""

    user_id: int
    chat_id: int | None = None
    first_name: str
    last_name: str | None = None
    username: str | None = None
    auth_date: int


def validate_init_data(
    init_data: str, bot_token: str, max_age_seconds: int = 86400
) -> InitDataValidation:
    """
    Validate Telegram Mini App init data.

    Steps per Telegram documentation:
    1. Parse query string
    2. Extract and remove hash
    3. Sort remaining params alphabetically
    4. Create data-check-string (key=value joined by newlines)
    5. Calculate secret_key = HMAC_SHA256("WebAppData", bot_token)
    6. Calculate hash = HMAC_SHA256(secret_key, data_check_string)
    7. Compare hashes
    8. Validate auth_date freshness

    Args:
        init_data: Raw init data query string from Telegram
        bot_token: Bot token for HMAC validation
        max_age_seconds: Maximum age of init data (default 24 hours)

    Returns:
        Validated init data with user info

    Raises:
        ValueError: If validation fails
    """
    # Parse query string
    params = dict(urllib.parse.parse_qsl(init_data, keep_blank_values=True))

    received_hash = params.pop("hash", None)
    if not received_hash:
        raise ValueError("Missing hash in init data")

    # Sort and create data-check-string
    data_check_string = "\n".join(f"{k}={v}" for k, v in sorted(params.items()))

    # Calculate secret key: HMAC_SHA256("WebAppData", bot_token)
    secret_key = hmac.new(
        b"WebAppData",
        bot_token.encode(),
        hashlib.sha256,
    ).digest()

    # Calculate expected hash
    calculated_hash = hmac.new(
        secret_key,
        data_check_string.encode(),
        hashlib.sha256,
    ).hexdigest()

    # Constant-time comparison
    if not hmac.compare_digest(calculated_hash, received_hash):
        raise ValueError("Invalid hash")

    # Validate auth_date
    auth_date = int(params.get("auth_date", 0))
    if time.time() - auth_date > max_age_seconds:
        raise ValueError("Init data expired")

    # Parse user object
    user_str = params.get("user", "{}")
    try:
        user_data = json.loads(user_str)
    except json.JSONDecodeError:
        raise ValueError("Invalid user data format")

    if not user_data.get("id"):
        raise ValueError("Missing user ID in init data")

    # Extract chat info if present
    chat_id = None
    if "chat" in params:
        try:
            chat_data = json.loads(params["chat"])
            chat_id = chat_data.get("id")
        except json.JSONDecodeError:
            pass

    # Chat ID might also be in start_param (passed via URL)
    if chat_id is None and "start_param" in params:
        try:
            chat_id = int(params["start_param"])
        except (ValueError, TypeError):
            pass

    return InitDataValidation(
        user_id=user_data["id"],
        chat_id=chat_id,
        first_name=user_data.get("first_name", ""),
        last_name=user_data.get("last_name"),
        username=user_data.get("username"),
        auth_date=auth_date,
    )


def create_jwt_token(user_data: InitDataValidation, secret_key: str) -> str:
    """
    Create JWT token for authenticated Mini App user.

    Args:
        user_data: Validated user data from init data
        secret_key: Secret key for JWT signing

    Returns:
        Encoded JWT token
    """
    payload = {
        "user_id": user_data.user_id,
        "chat_id": user_data.chat_id,
        "username": user_data.username,
        "first_name": user_data.first_name,
        "exp": datetime.now(timezone.utc) + timedelta(hours=JWT_EXPIRATION_HOURS),
        "iat": datetime.now(timezone.utc),
        "type": "mini_app",
    }
    return jwt.encode(payload, secret_key, algorithm=JWT_ALGORITHM)


def verify_jwt_token(token: str, secret_key: str) -> dict[str, Any]:
    """
    Verify and decode JWT token.

    Args:
        token: JWT token to verify
        secret_key: Secret key for verification

    Returns:
        Decoded token payload

    Raises:
        jwt.InvalidTokenError: If token is invalid or expired
    """
    return jwt.decode(token, secret_key, algorithms=[JWT_ALGORITHM])
