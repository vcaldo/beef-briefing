"""
Configuration module for Beef Dashboard.
Mirrors the pattern from pkg/config/config.go in the Go services.
"""

import os
import logging
from dataclasses import dataclass, field
from typing import List, Optional
from pathlib import Path

from dotenv import load_dotenv


def _read_secret_from_file(filepath: str) -> str:
    """Read a secret from a file and strip whitespace."""
    try:
        return Path(filepath).read_text().strip()
    except Exception as e:
        raise ValueError(f"Failed to read secret from {filepath}: {e}")


def _parse_chat_ids(value: str) -> List[int]:
    """Parse comma-separated chat IDs into a list of integers."""
    if not value:
        return []
    try:
        return [int(cid.strip()) for cid in value.split(',') if cid.strip()]
    except ValueError as e:
        raise ValueError(f"Invalid chat ID format: {e}")


@dataclass
class Config:
    """Dashboard configuration loaded from environment variables."""

    # Database Configuration
    db_host: str = field(default_factory=lambda: os.getenv('DB_HOST', 'localhost'))
    db_port: int = field(default_factory=lambda: int(os.getenv('DB_PORT', '5432')))
    db_user: str = field(default_factory=lambda: os.getenv('DB_USER', 'postgres'))
    db_password: str = field(default_factory=lambda: os.getenv('DB_PASSWORD', ''))
    db_name: str = field(default_factory=lambda: os.getenv('DB_NAME', 'beef_briefing'))
    db_ssl_mode: str = field(default_factory=lambda: os.getenv('DB_SSL_MODE', 'disable'))

    # Dashboard Configuration
    dashboard_port: int = field(default_factory=lambda: int(os.getenv('DASHBOARD_PORT', '8050')))
    dashboard_domain: str = field(default_factory=lambda: os.getenv('DASHBOARD_DOMAIN', 'localhost'))

    # Telegram Configuration
    telegram_bot_token: str = field(default_factory=lambda: os.getenv('TELEGRAM_BOT_TOKEN', ''))
    telegram_bot_username: str = field(default_factory=lambda: os.getenv('TELEGRAM_BOT_USERNAME', ''))

    # Access Control
    admin_user_ids: List[int] = field(default_factory=lambda: _parse_chat_ids(os.getenv('ADMIN_USER_IDS', '')))

    # API Service Configuration
    api_service_url: str = field(default_factory=lambda: os.getenv('API_SERVICE_URL', 'http://api-service:8080'))
    analytics_api_key: str = field(default_factory=lambda: os.getenv('ANALYTICS_API_KEY', ''))

    # Session Configuration
    session_lifetime_days: int = field(default_factory=lambda: int(os.getenv('SESSION_LIFETIME_DAYS', '7')))
    flask_secret_key: str = field(default='')
    flask_secret_key_file: str = field(default_factory=lambda: os.getenv('FLASK_SECRET_KEY_FILE', ''))

    # Application Settings
    environment: str = field(default_factory=lambda: os.getenv('ENVIRONMENT', 'development'))
    log_level: str = field(default_factory=lambda: os.getenv('LOG_LEVEL', 'info'))

    # New Relic APM Configuration (optional)
    new_relic_app_name: str = field(default_factory=lambda: os.getenv('NEW_RELIC_APP_NAME', ''))
    new_relic_license_key: str = field(default_factory=lambda: os.getenv('NEW_RELIC_LICENSE_KEY', ''))

    def __post_init__(self):
        """Load secrets from files after initialization."""
        # Load Flask secret key from file if specified
        if self.flask_secret_key_file:
            self.flask_secret_key = _read_secret_from_file(self.flask_secret_key_file)
        elif not self.flask_secret_key:
            # Default for development only
            if self.is_production():
                raise ValueError("FLASK_SECRET_KEY or FLASK_SECRET_KEY_FILE must be set in production")
            self.flask_secret_key = 'dev-secret-key-change-in-production'

        # Validate required fields
        if not self.telegram_bot_token:
            raise ValueError("TELEGRAM_BOT_TOKEN is required")
        if not self.telegram_bot_username and self.is_production():
            raise ValueError("TELEGRAM_BOT_USERNAME is required in production")

    @property
    def database_url(self) -> str:
        """PostgreSQL connection URL for SQLAlchemy."""
        return f"postgresql://{self.db_user}:{self.db_password}@{self.db_host}:{self.db_port}/{self.db_name}"

    @property
    def dsn(self) -> str:
        """PostgreSQL DSN string (matches Go DSN format)."""
        return f"host={self.db_host} port={self.db_port} user={self.db_user} password={self.db_password} dbname={self.db_name} sslmode={self.db_ssl_mode}"

    def is_production(self) -> bool:
        """Check if running in production environment."""
        return self.environment == 'production'

    def is_development(self) -> bool:
        """Check if running in development environment."""
        return self.environment == 'development'

    def is_admin(self, user_id: int) -> bool:
        """Check if user is an admin (can see all chats)."""
        return user_id in self.admin_user_ids

    def new_relic_enabled(self) -> bool:
        """Check if New Relic APM is configured."""
        return bool(self.new_relic_app_name and self.new_relic_license_key)

    @property
    def session_lifetime_seconds(self) -> int:
        """Session lifetime in seconds."""
        return self.session_lifetime_days * 24 * 60 * 60


def load_config() -> Config:
    """
    Load configuration from environment variables.
    Attempts to load .env file if present.
    """
    # Load .env file (ignore if not found)
    load_dotenv()

    return Config()


def setup_logging(config: Config) -> None:
    """
    Configure logging based on environment.
    Production: JSON format, INFO level
    Development: Text format, DEBUG level
    """
    level = getattr(logging, config.log_level.upper(), logging.INFO)

    if config.is_production():
        # JSON format for production (compatible with log aggregators)
        logging.basicConfig(
            level=level,
            format='{"timestamp": "%(asctime)s", "level": "%(levelname)s", "logger": "%(name)s", "message": "%(message)s"}',
            datefmt='%Y-%m-%dT%H:%M:%S%z'
        )
    else:
        # Human-readable format for development
        logging.basicConfig(
            level=logging.DEBUG,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'
        )
