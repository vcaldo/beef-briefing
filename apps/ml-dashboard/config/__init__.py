"""
Configuration module for the ML Dashboard service.
Mirrors the Go pkg/config pattern using Pydantic for environment parsing.
"""

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

    # Qdrant Configuration
    qdrant_host: str = "localhost"
    qdrant_port: int = 6333

    # ML Dashboard Configuration
    ml_dashboard_port: int = 8501

    # Application Settings
    environment: str = "development"
    log_level: str = "info"

    # Cache Configuration
    cache_dir: str = "/app/cache"

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


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
