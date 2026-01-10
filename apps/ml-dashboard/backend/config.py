"""
Configuration for ML Dashboard backend.
Uses Pydantic for environment variable parsing.
"""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Configuration loaded from environment variables."""

    # Database Configuration
    db_host: str = "postgres"
    db_port: int = 5432
    db_user: str = "postgres"
    db_password: str = ""
    db_name: str = "beef_briefing"
    db_ssl_mode: str = "disable"

    # Qdrant Configuration (dev only)
    qdrant_host: str = "qdrant"
    qdrant_port: int = 6333
    qdrant_enabled: bool = True

    # Embedding model (same as ml-processor for consistency)
    embedding_model: str = "sentence-transformers/paraphrase-multilingual-mpnet-base-v2"

    # Application Settings
    environment: str = "development"
    log_level: str = "debug"

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


settings = Settings()
