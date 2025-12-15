"""
Configuration module for the leaderboard service.
Mirrors the Go pkg/config pattern using Pydantic for environment parsing.
"""

import os
from functools import cached_property
from typing import Optional

from pydantic_settings import BaseSettings


class Config(BaseSettings):
    """Configuration loaded from environment variables."""

    # Database Configuration
    db_host: str = "localhost"
    db_port: int = 5432
    db_user: str = "postgres"
    db_password: str = ""
    db_name: str = "beef_briefing"
    db_ssl_mode: str = "disable"

    # Leaderboard Service Configuration
    leaderboard_port: int = 8050
    leaderboard_path: str = "/leaderboard"

    # Application Settings
    environment: str = "development"
    log_level: str = "info"

    # Telegram OAuth Configuration
    telegram_bot_token: str = ""
    telegram_bot_username: str = ""

    # Admin Configuration (comma-separated user IDs)
    admin_user_ids: str = ""

    # API Service Configuration (for fetching profile photos)
    api_service_url: str = ""
    api_key_file: str = ""

    # New Relic APM Configuration (optional)
    new_relic_app_name: Optional[str] = None
    new_relic_license_key: Optional[str] = None

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"

    def dsn(self) -> str:
        """Return PostgreSQL connection string (SQLAlchemy format)."""
        return (
            f"postgresql://{self.db_user}:{self.db_password}"
            f"@{self.db_host}:{self.db_port}/{self.db_name}"
            f"?sslmode={self.db_ssl_mode}"
        )

    def is_production(self) -> bool:
        """Return True if running in production environment."""
        return self.environment == "production"

    def new_relic_enabled(self) -> bool:
        """Return True if New Relic APM is configured."""
        return bool(self.new_relic_app_name and self.new_relic_license_key)

    @cached_property
    def new_relic_full_app_name(self) -> str:
        """Return full New Relic app name: {base-name}-leaderboard-{environment}."""
        if self.new_relic_app_name:
            return f"{self.new_relic_app_name}-leaderboard-{self.environment}"
        return ""

    def get_admin_user_ids(self) -> list[int]:
        """Parse admin_user_ids string into list of integers."""
        if not self.admin_user_ids:
            return []
        return [int(uid.strip()) for uid in self.admin_user_ids.split(",") if uid.strip()]

    def is_admin(self, user_id: int) -> bool:
        """Check if user_id is in the admin list."""
        return user_id in self.get_admin_user_ids()

    def get_first_admin_id(self) -> int | None:
        """Get first admin user ID (used for dev mode auto-login)."""
        admin_ids = self.get_admin_user_ids()
        return admin_ids[0] if admin_ids else None

    @cached_property
    def api_key(self) -> str:
        """Load API key from file if api_key_file is set."""
        if not self.api_key_file:
            return ""
        try:
            with open(self.api_key_file) as f:
                return f.read().strip()
        except FileNotFoundError:
            return ""


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
