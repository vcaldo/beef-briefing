"""
Base classes for ML analyzers.

Implements the Strategy pattern with lazy loading to minimize
resource usage when certain providers are not needed.
"""

import logging
from abc import ABC, abstractmethod
from enum import Enum
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from config import Config

logger = logging.getLogger(__name__)


class AnalysisType(str, Enum):
    """Types of ML analysis available."""

    SENTIMENT = "sentiment"
    TOXICITY = "toxicity"
    TOPICS = "topics"
    NER = "ner"
    EMBEDDINGS = "embeddings"
    HUMOR = "humor"
    QUESTIONS = "questions"


class Analyzer(ABC):
    """
    Base class for all analyzers.

    Analyzers implement the Strategy pattern, allowing different
    providers (local, OpenAI, etc.) to be swapped based on configuration.
    """

    @property
    @abstractmethod
    def analysis_type(self) -> AnalysisType:
        """Return the type of analysis this analyzer performs."""
        pass

    @abstractmethod
    def analyze(self, texts: list[str], **kwargs) -> list[Any]:
        """
        Analyze a batch of texts.

        Args:
            texts: List of text strings to analyze
            **kwargs: Additional arguments specific to the analyzer

        Returns:
            List of results, one per input text.
            The exact structure depends on the analysis type.
        """
        pass

    @abstractmethod
    def is_available(self) -> bool:
        """
        Check if this analyzer can run.

        Returns True if all required dependencies and API keys are available.
        """
        pass

    def get_provider_name(self) -> str:
        """Return the provider name for logging."""
        return self.__class__.__name__

    def cleanup(self) -> None:
        """
        Release any resources held by this analyzer.

        Override in subclasses that need cleanup.
        """
        pass


class FallbackAnalyzer(Analyzer):
    """
    Wrapper that tries a primary analyzer and falls back to secondary on failure.

    Useful for providing graceful degradation when API providers fail.
    """

    def __init__(self, primary: Analyzer, fallback: Analyzer):
        """
        Initialize with primary and fallback analyzers.

        Args:
            primary: First analyzer to try
            fallback: Analyzer to use if primary fails
        """
        self.primary = primary
        self.fallback = fallback

    @property
    def analysis_type(self) -> AnalysisType:
        return self.primary.analysis_type

    def analyze(self, texts: list[str], **kwargs) -> list[Any]:
        try:
            return self.primary.analyze(texts, **kwargs)
        except Exception as e:
            logger.warning(
                f"Primary analyzer {self.primary.get_provider_name()} failed: {e}. "
                f"Falling back to {self.fallback.get_provider_name()}"
            )
            return self.fallback.analyze(texts, **kwargs)

    def is_available(self) -> bool:
        return self.primary.is_available() or self.fallback.is_available()

    def get_provider_name(self) -> str:
        return f"{self.primary.get_provider_name()}+{self.fallback.get_provider_name()}"

    def cleanup(self) -> None:
        self.primary.cleanup()
        self.fallback.cleanup()


class AnalyzerRegistry:
    """
    Factory and registry for analyzers.

    Lazy-loads analyzers based on configuration, only importing
    and initializing the required providers.
    """

    def __init__(self, config: "Config"):
        """
        Initialize the registry with configuration.

        Args:
            config: Application configuration with provider selection
        """
        self.config = config
        self._analyzers: dict[AnalysisType, Analyzer] = {}

    def _get_provider_for_type(self, analysis_type: AnalysisType) -> str:
        """Get the configured provider for an analysis type."""
        mapping = {
            AnalysisType.SENTIMENT: self.config.sentiment_provider,
            AnalysisType.TOXICITY: self.config.toxicity_provider,
            AnalysisType.TOPICS: self.config.topics_provider,
            AnalysisType.NER: self.config.ner_provider,
            AnalysisType.EMBEDDINGS: self.config.embeddings_provider,
            AnalysisType.HUMOR: self.config.humor_provider,
            AnalysisType.QUESTIONS: self.config.questions_provider,
        }
        return mapping.get(analysis_type, "local")

    def _create_analyzer(self, analysis_type: AnalysisType) -> Analyzer:
        """
        Create an analyzer instance based on configuration.

        Imports are done lazily to avoid loading unused dependencies.
        """
        provider = self._get_provider_for_type(analysis_type)

        # ===== SENTIMENT =====
        if analysis_type == AnalysisType.SENTIMENT:
            if provider == "local":
                from src.analyzers.sentiment import LocalSentimentAnalyzer

                return LocalSentimentAnalyzer(
                    model_name=self.config.sentiment_model,
                    device=self.config.device,
                )
            elif provider == "openai":
                from src.analyzers.sentiment import OpenAISentimentAnalyzer

                return OpenAISentimentAnalyzer(api_key=self.config.openai_api_key)
            elif provider == "anthropic":
                from src.analyzers.sentiment import AnthropicSentimentAnalyzer

                return AnthropicSentimentAnalyzer(api_key=self.config.anthropic_api_key)

        # ===== TOXICITY =====
        elif analysis_type == AnalysisType.TOXICITY:
            if provider == "local":
                from src.analyzers.toxicity import LocalToxicityAnalyzer

                return LocalToxicityAnalyzer(
                    model_name=self.config.toxicity_model,
                    device=self.config.device,
                )
            elif provider == "perspective":
                from src.analyzers.toxicity import PerspectiveToxicityAnalyzer

                return PerspectiveToxicityAnalyzer(
                    api_key=self.config.perspective_api_key
                )
            elif provider == "openai":
                from src.analyzers.toxicity import OpenAIModerationAnalyzer

                return OpenAIModerationAnalyzer(api_key=self.config.openai_api_key)

        # ===== TOPICS =====
        elif analysis_type == AnalysisType.TOPICS:
            if provider == "local":
                from src.analyzers.topics import LocalTopicClusterer

                return LocalTopicClusterer()
            elif provider == "openai":
                from src.analyzers.topics import OpenAITopicClusterer

                return OpenAITopicClusterer(api_key=self.config.openai_api_key)

        # ===== NER =====
        elif analysis_type == AnalysisType.NER:
            if provider == "local":
                from src.analyzers.ner import LocalNERExtractor

                return LocalNERExtractor(model_name=self.config.ner_model)
            elif provider == "openai":
                from src.analyzers.ner import OpenAINERExtractor

                return OpenAINERExtractor(api_key=self.config.openai_api_key)

        # ===== EMBEDDINGS =====
        elif analysis_type == AnalysisType.EMBEDDINGS:
            if provider == "local":
                from src.analyzers.embeddings import LocalEmbeddingEncoder

                return LocalEmbeddingEncoder(
                    model_name=self.config.embedding_model,
                    device=self.config.device,
                )
            elif provider == "openai":
                from src.analyzers.embeddings import OpenAIEmbeddingEncoder

                return OpenAIEmbeddingEncoder(api_key=self.config.openai_api_key)

        # ===== HUMOR =====
        elif analysis_type == AnalysisType.HUMOR:
            if provider == "local":
                from src.analyzers.humor import LocalHumorDetector

                return LocalHumorDetector()
            elif provider == "openai":
                from src.analyzers.humor import OpenAIHumorDetector

                return OpenAIHumorDetector(api_key=self.config.openai_api_key)

        # ===== QUESTIONS =====
        elif analysis_type == AnalysisType.QUESTIONS:
            if provider == "local":
                from src.analyzers.questions import LocalQuestionClassifier

                return LocalQuestionClassifier(
                    model_name=self.config.questions_model,
                    device=self.config.device,
                )
            elif provider == "openai":
                from src.analyzers.questions import OpenAIQuestionClassifier

                return OpenAIQuestionClassifier(api_key=self.config.openai_api_key)

        raise ValueError(f"Unknown provider '{provider}' for {analysis_type}")

    def get(self, analysis_type: AnalysisType) -> Analyzer:
        """
        Get or create an analyzer for the given type.

        The analyzer is cached after first creation.
        """
        if analysis_type not in self._analyzers:
            self._analyzers[analysis_type] = self._create_analyzer(analysis_type)
            logger.info(
                f"Loaded {analysis_type.value} analyzer: "
                f"{self._analyzers[analysis_type].get_provider_name()}"
            )
        return self._analyzers[analysis_type]

    def get_if_available(self, analysis_type: AnalysisType) -> Analyzer | None:
        """
        Get an analyzer only if it's available.

        Returns None if the analyzer cannot run (missing deps/keys).
        """
        try:
            analyzer = self.get(analysis_type)
            if analyzer.is_available():
                return analyzer
            logger.warning(f"{analysis_type.value} analyzer not available")
            return None
        except Exception as e:
            logger.error(f"Failed to load {analysis_type.value} analyzer: {e}")
            return None

    def get_all_available(self) -> list[Analyzer]:
        """Get all analyzers that are configured and available."""
        analyzers = []
        for analysis_type in AnalysisType:
            analyzer = self.get_if_available(analysis_type)
            if analyzer:
                analyzers.append(analyzer)
        return analyzers

    def cleanup_all(self) -> None:
        """Release resources from all loaded analyzers."""
        for analyzer in self._analyzers.values():
            try:
                analyzer.cleanup()
            except Exception as e:
                logger.warning(f"Error cleaning up {analyzer.get_provider_name()}: {e}")
        self._analyzers.clear()
