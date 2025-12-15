"""
Configuration module for the ML processor service.
Uses Pydantic for environment variable parsing.
"""

from functools import cached_property
from pathlib import Path

from pydantic_settings import BaseSettings


class Config(BaseSettings):
    """Configuration loaded from environment variables."""

    # API Service Configuration
    api_service_url: str = "http://localhost:8080"
    api_key_file: str = "../../infrastructure/secrets/apps/ml-processor/api_key"

    # Qdrant Configuration
    qdrant_host: str = "localhost"
    qdrant_port: int = 6333

    # Processing Configuration
    batch_size: int = 500
    sleep_seconds: int = 60

    # Model Configuration
    device: str = "cuda"
    sentiment_model: str = "lxyuan/distilbert-base-multilingual-cased-sentiments-student"
    toxicity_model: str = "ruanchaves/bert-base-portuguese-cased-hatebr"
    embedding_model: str = "sentence-transformers/paraphrase-multilingual-mpnet-base-v2"

    # Application Settings
    environment: str = "development"
    log_level: str = "info"

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"

    def is_production(self) -> bool:
        """Check if running in production environment."""
        return self.environment.lower() == "production"

    @cached_property
    def api_key(self) -> str:
        """Load API key from file."""
        key_path = Path(self.api_key_file)
        if not key_path.exists():
            raise FileNotFoundError(f"API key file not found: {key_path}")
        return key_path.read_text().strip()

    @property
    def qdrant_url(self) -> str:
        """Return Qdrant connection URL."""
        return f"http://{self.qdrant_host}:{self.qdrant_port}"


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
