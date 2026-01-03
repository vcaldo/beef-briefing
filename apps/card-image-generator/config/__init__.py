"""
Configuration module for the card image generator service.
Uses Pydantic for environment variable parsing.
"""

import os
from functools import cached_property

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

    # MinIO/Object Storage Configuration
    minio_endpoint: str = "localhost:9000"
    minio_access_key: str = "minioadmin"
    minio_secret_key: str = "minioadmin"
    minio_bucket: str = "telegram-media"
    minio_use_ssl: bool = False
    minio_region: str = ""

    # Service Configuration
    card_generator_port: int = 8051
    card_generator_host: str = "0.0.0.0"

    # Template Configuration
    templates_dir: str = "/app/templates"
    default_card_theme: str = "gaming"

    # Card Dimensions
    card_width: int = 400
    card_height: int = 600
    card_scale: int = 2  # Render at 2x for retina

    # Application Settings
    environment: str = "development"
    log_level: str = "info"

    # API Authentication
    app_keys_dir: str = "/app/secrets/app_keys"

    # Mini App JWT Authentication
    jwt_secret_key: str = ""  # Required for Mini App auth
    telegram_bot_token: str = ""  # Required for init data validation

    # CORS Configuration
    cors_origins: str = ""  # Comma-separated list of allowed origins

    # New Relic APM Configuration (optional)
    new_relic_app_name: str | None = None
    new_relic_license_key: str | None = None

    # Tier Configuration (same format as ml-processor: "Name:MinScore")
    tier_1: str = "Legendary:81"
    tier_2: str = "Elite:77"
    tier_3: str = "Outstanding:72"
    tier_4: str = "Regular:55"
    tier_5: str = "Beginner:32"
    tier_6: str = "Rookie:10"

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"

    def dsn(self) -> str:
        """Return PostgreSQL connection string."""
        return (
            f"postgresql://{self.db_user}:{self.db_password}"
            f"@{self.db_host}:{self.db_port}/{self.db_name}"
            f"?sslmode={self.db_ssl_mode}"
        )

    def is_production(self) -> bool:
        """Check if running in production environment."""
        return self.environment.lower() == "production"

    def new_relic_enabled(self) -> bool:
        """Return True if New Relic APM is configured."""
        return bool(self.new_relic_app_name and self.new_relic_license_key)

    @cached_property
    def new_relic_full_app_name(self) -> str:
        """Return full New Relic app name."""
        if self.new_relic_app_name:
            return f"{self.new_relic_app_name}-card-image-generator-{self.environment}"
        return ""

    @cached_property
    def tiers(self) -> list[tuple[str, int]]:
        """Parse tier configuration into list of (name, min_score) tuples, ordered tier_1 to tier_6."""
        result = []
        for tier_str in [self.tier_1, self.tier_2, self.tier_3, self.tier_4, self.tier_5, self.tier_6]:
            if tier_str:
                name, score = tier_str.split(":")
                result.append((name.strip(), int(score.strip())))
        return result

    @cached_property
    def tier_name_to_class(self) -> dict[str, str]:
        """Map tier names (case-insensitive) to CSS class names (tier-1 through tier-6)."""
        return {name.lower(): f"tier-{i+1}" for i, (name, _) in enumerate(self.tiers)}

    def get_tier_class(self, label: str) -> str:
        """Get CSS class for a tier label. Returns 'tier-6' as fallback."""
        return self.tier_name_to_class.get(label.lower(), "tier-6")

    def get_app_keys(self) -> dict[str, str]:
        """Load API keys from app_keys_dir."""
        keys = {}
        if not os.path.isdir(self.app_keys_dir):
            return keys

        for filename in os.listdir(self.app_keys_dir):
            filepath = os.path.join(self.app_keys_dir, filename)
            if os.path.isfile(filepath):
                with open(filepath) as f:
                    keys[filename] = f.read().strip()
        return keys


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
