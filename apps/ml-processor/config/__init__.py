"""
Configuration module for the ML processor service.
Uses Pydantic for environment variable parsing.
"""

from functools import cached_property
from typing import Literal, Optional

from pydantic_settings import BaseSettings

ProviderType = Literal["local", "openai", "anthropic", "perspective"]


class Config(BaseSettings):
    """Configuration loaded from environment variables."""

    # Database Configuration (direct PostgreSQL access)
    db_host: str = "localhost"
    db_port: int = 5432
    db_user: str = "postgres"
    db_password: str = ""
    db_name: str = "beef_briefing"
    db_ssl_mode: str = "disable"

    # Provider Selection (per analysis type)
    sentiment_provider: ProviderType = "local"
    toxicity_provider: ProviderType = "local"
    topics_provider: ProviderType = "local"
    ner_provider: ProviderType = "local"
    embeddings_provider: ProviderType = "local"
    humor_provider: ProviderType = "local"
    questions_provider: ProviderType = "local"

    # Optional API Keys (only needed for non-local providers)
    openai_api_key: Optional[str] = None
    perspective_api_key: Optional[str] = None
    anthropic_api_key: Optional[str] = None

    # New Relic APM Configuration (optional)
    new_relic_app_name: Optional[str] = None
    new_relic_license_key: Optional[str] = None

    # Qdrant Configuration
    qdrant_host: str = "localhost"
    qdrant_port: int = 6333

    # Processing Configuration
    batch_size: int = 500
    sleep_seconds: int = 60

    # Model Configuration (for local providers)
    device: str = "cuda"
    sentiment_model: str = "lxyuan/distilbert-base-multilingual-cased-sentiments-student"
    toxicity_model: str = "ruanchaves/bert-base-portuguese-cased-hatebr"
    embedding_model: str = "sentence-transformers/paraphrase-multilingual-mpnet-base-v2"
    ner_model: str = "pt_core_news_lg"
    questions_model: str = "facebook/bart-large-mnli"  # Zero-shot classification model

    # Application Settings
    environment: str = "development"
    log_level: str = "info"

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
        """Return full New Relic app name: {base-name}-ml-processor-{environment}."""
        if self.new_relic_app_name:
            return f"{self.new_relic_app_name}-ml-processor-{self.environment}"
        return ""

    @property
    def qdrant_url(self) -> str:
        """Return Qdrant connection URL."""
        return f"http://{self.qdrant_host}:{self.qdrant_port}"

    def validate_api_keys(self) -> list[str]:
        """Return list of missing API keys for configured providers."""
        errors = []

        api_providers = {
            "openai": self.openai_api_key,
            "anthropic": self.anthropic_api_key,
            "perspective": self.perspective_api_key,
        }

        provider_attrs = [
            ("sentiment_provider", self.sentiment_provider),
            ("toxicity_provider", self.toxicity_provider),
            ("topics_provider", self.topics_provider),
            ("ner_provider", self.ner_provider),
            ("embeddings_provider", self.embeddings_provider),
            ("humor_provider", self.humor_provider),
            ("questions_provider", self.questions_provider),
        ]

        for attr_name, provider in provider_attrs:
            if provider != "local" and not api_providers.get(provider):
                errors.append(
                    f"{attr_name}={provider} requires {provider.upper()}_API_KEY"
                )

        return errors


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config()
