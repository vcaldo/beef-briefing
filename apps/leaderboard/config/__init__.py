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


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
