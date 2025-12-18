# ML Processor: Hybrid Implementation Guide

## Overview

This document describes the architecture for a Python-based ML processing service that supports **both local models and paid APIs** via environment configuration. The design follows the **Strategy Pattern** with lazy-loading to minimize resource usage.

### Key Principles

1. **Single codebase** - One service, configurable providers
2. **Lazy loading** - Only load dependencies when needed (no torch if using APIs)
3. **Extensible** - Easy to add new analysis types or providers
4. **Fallback support** - Graceful degradation when providers fail
5. **Config-driven** - Switch providers via environment variables

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ml-processor                             │
│                                                                 │
│  ┌──────────────┐      ┌────────────────────────────────────┐  │
│  │              │      │         AnalyzerRegistry            │  │
│  │   Scheduler  │─────▶│                                    │  │
│  │  (APScheduler)      │  sentiment: provider from config   │  │
│  │              │      │  toxicity:  provider from config   │  │
│  └──────────────┘      │  topics:    provider from config   │  │
│                        └────────────────────────────────────┘  │
│                                       │                         │
│                    ┌──────────────────┼──────────────────┐     │
│                    ▼                  ▼                  ▼     │
│            ┌─────────────┐    ┌─────────────┐    ┌───────────┐ │
│            │   Local     │    │  Perspective │    │  OpenAI   │ │
│            │  (HuggingFace)   │    API      │    │   API     │ │
│            └─────────────┘    └─────────────┘    └───────────┘ │
│                                                                 │
│                    │ unified results                            │
│                    ▼                                            │
│             ┌─────────────┐                                    │
│             │  PostgreSQL │                                    │
│             └─────────────┘                                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
apps/ml-processor/
├── src/
│   ├── __init__.py
│   ├── main.py                    # Entry point, scheduler setup
│   ├── config.py                  # Pydantic settings
│   ├── database.py                # SQLAlchemy connection
│   │
│   ├── analyzers/                 # Analysis implementations
│   │   ├── __init__.py
│   │   ├── base.py                # Abstract base + registry
│   │   ├── sentiment.py           # Sentiment analyzers
│   │   ├── toxicity.py            # Toxicity analyzers
│   │   ├── topics.py              # Topic clustering
│   │   ├── humor.py               # Humor detection
│   │   ├── questions.py           # Question classification
│   │   └── ner.py                 # Named entity recognition
│   │
│   ├── providers/                 # API client wrappers
│   │   ├── __init__.py
│   │   ├── openai_client.py
│   │   ├── perspective_client.py
│   │   └── anthropic_client.py
│   │
│   ├── processor.py               # Main batch processor
│   └── models.py                  # Pydantic models for results
│
├── requirements.txt
├── requirements-local.txt         # torch, transformers (optional)
├── Dockerfile
└── README.md
```

---

## Configuration

### Environment Variables

```bash
# Provider selection per analysis type
ML_SENTIMENT_PROVIDER=local        # local | openai | anthropic
ML_TOXICITY_PROVIDER=perspective   # local | perspective | openai
ML_TOPICS_PROVIDER=openai          # local | openai | cohere
ML_HUMOR_PROVIDER=local            # local | openai
ML_QUESTIONS_PROVIDER=local        # local | openai
ML_NER_PROVIDER=local              # local | openai

# API keys (only needed for non-local providers)
ML_OPENAI_API_KEY=sk-...
ML_PERSPECTIVE_API_KEY=...
ML_ANTHROPIC_API_KEY=...
ML_COHERE_API_KEY=...

# Database
ML_DATABASE_URL=postgresql://user:pass@localhost:5432/beef

# API Service (for fetching messages, storing results)
ML_API_URL=http://localhost:8080
ML_API_KEY=your-api-key

# Processing settings
ML_BATCH_SIZE=100
ML_SCHEDULE_INTERVAL_MINUTES=30
```

### Targeting Production from Local

The ML processor can run locally while targeting production database and API. This is useful for:
- Running heavy ML processing on a local GPU without affecting production servers
- Testing changes against real data before deploying
- One-off batch processing jobs

**Example: Local-to-Production Config**

Create `ml-processor/.env.prod` for targeting production:

```bash
# .env.prod - Target production from local machine

# Database: Connect directly to production PostgreSQL
# Requires: SSH tunnel or direct access to production DB
ML_DATABASE_URL=postgresql://ml_processor:password@prod-db.example.com:5432/beef

# API Service: Point to production API
ML_API_URL=https://api.yourdomain.com
ML_API_KEY=your-production-api-key

# Provider configuration (same as production or use local GPU)
ML_SENTIMENT_PROVIDER=local     # Use local GPU for heavy lifting
ML_TOXICITY_PROVIDER=local
ML_TOPICS_PROVIDER=openai       # Use API for complex tasks
ML_OPENAI_API_KEY=sk-...

# Processing settings
ML_BATCH_SIZE=500
```

**Usage:**

```bash
# Run with production config
ML_ENV_FILE=.env.prod python -m src.main process --chat-id -1003280306634

# Or export inline
ML_DATABASE_URL="postgresql://..." ML_API_URL="https://..." python -m src.main process --chat-id -1003280306634
```

**SSH Tunnel for Database Access:**

If production DB isn't directly accessible:

```bash
# Terminal 1: Create SSH tunnel to production DB
ssh -L 5433:localhost:5432 user@prod-server

# Terminal 2: Run processor with tunneled connection
ML_DATABASE_URL=postgresql://user:pass@localhost:5433/beef python -m src.main process --chat-id -1003280306634
```

| Target | Database URL | API URL |
|--------|--------------|---------|
| Local dev | `postgresql://...@localhost:5432/beef` | `http://localhost:8080` |
| Production (direct) | `postgresql://...@prod-db:5432/beef` | `https://api.yourdomain.com` |
| Production (tunnel) | `postgresql://...@localhost:5433/beef` | `https://api.yourdomain.com` |

### Config Class

```python
# src/config.py
from typing import Literal
from pydantic_settings import BaseSettings

ProviderType = Literal["local", "openai", "anthropic", "perspective", "cohere"]

class MLSettings(BaseSettings):
    # Provider selection
    sentiment_provider: ProviderType = "local"
    toxicity_provider: ProviderType = "local"
    topics_provider: ProviderType = "local"
    humor_provider: ProviderType = "local"
    questions_provider: ProviderType = "local"
    ner_provider: ProviderType = "local"

    # ML provider API keys (optional based on provider selection)
    openai_api_key: str | None = None
    perspective_api_key: str | None = None
    anthropic_api_key: str | None = None
    cohere_api_key: str | None = None

    # Database (can target local or production)
    database_url: str

    # API Service (can target local or production)
    api_url: str = "http://localhost:8080"
    api_key: str | None = None

    # Processing
    batch_size: int = 100
    schedule_interval_minutes: int = 30

    # Environment file override
    env_file: str | None = None

    class Config:
        env_prefix = "ML_"
        env_file = ".env"

    def __init__(self, **kwargs):
        # Support ML_ENV_FILE to load different config files
        import os
        env_file = os.getenv("ML_ENV_FILE", ".env")
        super().__init__(_env_file=env_file, **kwargs)

    def validate_api_keys(self) -> list[str]:
        """Return list of missing API keys for configured providers."""
        errors = []

        api_providers = {
            "openai": self.openai_api_key,
            "anthropic": self.anthropic_api_key,
            "perspective": self.perspective_api_key,
            "cohere": self.cohere_api_key,
        }

        for attr in ["sentiment_provider", "toxicity_provider", "topics_provider",
                     "humor_provider", "questions_provider", "ner_provider"]:
            provider = getattr(self, attr)
            if provider != "local" and not api_providers.get(provider):
                errors.append(f"{attr}={provider} requires {provider.upper()}_API_KEY")

        return errors
```

---

## Base Analyzer Interface

```python
# src/analyzers/base.py
from abc import ABC, abstractmethod
from enum import Enum
from typing import TypeVar, Generic
from functools import lru_cache
import logging

from src.config import MLSettings, ProviderType
from src.models import AnalysisResult

logger = logging.getLogger(__name__)

class AnalysisType(str, Enum):
    SENTIMENT = "sentiment"
    TOXICITY = "toxicity"
    TOPICS = "topics"
    HUMOR = "humor"
    QUESTIONS = "questions"
    NER = "ner"


class Analyzer(ABC):
    """Base class for all analyzers."""

    @property
    @abstractmethod
    def analysis_type(self) -> AnalysisType:
        """Return the type of analysis this analyzer performs."""
        pass

    @abstractmethod
    async def analyze(self, texts: list[str]) -> list[dict]:
        """
        Analyze a batch of texts.

        Args:
            texts: List of text strings to analyze

        Returns:
            List of result dicts, one per input text
        """
        pass

    @abstractmethod
    def is_available(self) -> bool:
        """Check if this analyzer can run (has required deps/keys)."""
        pass

    def get_provider_name(self) -> str:
        """Return the provider name for logging."""
        return self.__class__.__name__


class FallbackAnalyzer(Analyzer):
    """Wrapper that tries primary analyzer, falls back to secondary on failure."""

    def __init__(self, primary: Analyzer, fallback: Analyzer):
        self.primary = primary
        self.fallback = fallback

    @property
    def analysis_type(self) -> AnalysisType:
        return self.primary.analysis_type

    async def analyze(self, texts: list[str]) -> list[dict]:
        try:
            return await self.primary.analyze(texts)
        except Exception as e:
            logger.warning(
                f"Primary analyzer {self.primary.get_provider_name()} failed: {e}. "
                f"Falling back to {self.fallback.get_provider_name()}"
            )
            return await self.fallback.analyze(texts)

    def is_available(self) -> bool:
        return self.primary.is_available() or self.fallback.is_available()


class AnalyzerRegistry:
    """
    Factory and registry for analyzers.
    Lazy-loads analyzers based on configuration.
    """

    def __init__(self, config: MLSettings):
        self.config = config
        self._analyzers: dict[AnalysisType, Analyzer] = {}
        self._loaded = False

    def _get_provider_for_type(self, analysis_type: AnalysisType) -> ProviderType:
        """Get configured provider for an analysis type."""
        mapping = {
            AnalysisType.SENTIMENT: self.config.sentiment_provider,
            AnalysisType.TOXICITY: self.config.toxicity_provider,
            AnalysisType.TOPICS: self.config.topics_provider,
            AnalysisType.HUMOR: self.config.humor_provider,
            AnalysisType.QUESTIONS: self.config.questions_provider,
            AnalysisType.NER: self.config.ner_provider,
        }
        return mapping[analysis_type]

    def _create_analyzer(self, analysis_type: AnalysisType) -> Analyzer:
        """
        Create an analyzer instance based on config.
        Imports are done lazily to avoid loading unused dependencies.
        """
        provider = self._get_provider_for_type(analysis_type)

        # ===== SENTIMENT =====
        if analysis_type == AnalysisType.SENTIMENT:
            if provider == "local":
                from src.analyzers.sentiment import LocalSentimentAnalyzer
                return LocalSentimentAnalyzer()
            elif provider == "openai":
                from src.analyzers.sentiment import OpenAISentimentAnalyzer
                return OpenAISentimentAnalyzer(self.config.openai_api_key)
            elif provider == "anthropic":
                from src.analyzers.sentiment import AnthropicSentimentAnalyzer
                return AnthropicSentimentAnalyzer(self.config.anthropic_api_key)

        # ===== TOXICITY =====
        elif analysis_type == AnalysisType.TOXICITY:
            if provider == "local":
                from src.analyzers.toxicity import LocalToxicityAnalyzer
                return LocalToxicityAnalyzer()
            elif provider == "perspective":
                from src.analyzers.toxicity import PerspectiveToxicityAnalyzer
                return PerspectiveToxicityAnalyzer(self.config.perspective_api_key)
            elif provider == "openai":
                from src.analyzers.toxicity import OpenAIModerationAnalyzer
                return OpenAIModerationAnalyzer(self.config.openai_api_key)

        # ===== TOPICS =====
        elif analysis_type == AnalysisType.TOPICS:
            if provider == "local":
                from src.analyzers.topics import LocalTopicClusterer
                return LocalTopicClusterer()
            elif provider == "openai":
                from src.analyzers.topics import OpenAITopicClusterer
                return OpenAITopicClusterer(self.config.openai_api_key)
            elif provider == "cohere":
                from src.analyzers.topics import CohereTopicClusterer
                return CohereTopicClusterer(self.config.cohere_api_key)

        # ===== HUMOR =====
        elif analysis_type == AnalysisType.HUMOR:
            if provider == "local":
                from src.analyzers.humor import LocalHumorDetector
                return LocalHumorDetector()
            elif provider == "openai":
                from src.analyzers.humor import OpenAIHumorDetector
                return OpenAIHumorDetector(self.config.openai_api_key)

        # ===== QUESTIONS =====
        elif analysis_type == AnalysisType.QUESTIONS:
            if provider == "local":
                from src.analyzers.questions import LocalQuestionClassifier
                return LocalQuestionClassifier()
            elif provider == "openai":
                from src.analyzers.questions import OpenAIQuestionClassifier
                return OpenAIQuestionClassifier(self.config.openai_api_key)

        # ===== NER =====
        elif analysis_type == AnalysisType.NER:
            if provider == "local":
                from src.analyzers.ner import LocalNERExtractor
                return LocalNERExtractor()
            elif provider == "openai":
                from src.analyzers.ner import OpenAINERExtractor
                return OpenAINERExtractor(self.config.openai_api_key)

        raise ValueError(f"Unknown provider '{provider}' for {analysis_type}")

    def get(self, analysis_type: AnalysisType) -> Analyzer:
        """Get or create an analyzer for the given type."""
        if analysis_type not in self._analyzers:
            self._analyzers[analysis_type] = self._create_analyzer(analysis_type)
            logger.info(
                f"Loaded {analysis_type.value} analyzer: "
                f"{self._analyzers[analysis_type].get_provider_name()}"
            )
        return self._analyzers[analysis_type]

    def get_all_enabled(self) -> list[Analyzer]:
        """Get all analyzers that are configured and available."""
        analyzers = []
        for analysis_type in AnalysisType:
            try:
                analyzer = self.get(analysis_type)
                if analyzer.is_available():
                    analyzers.append(analyzer)
                else:
                    logger.warning(f"{analysis_type.value} analyzer not available")
            except Exception as e:
                logger.error(f"Failed to load {analysis_type.value} analyzer: {e}")
        return analyzers
```

---

## Analyzer Implementations

### Sentiment Analyzers

```python
# src/analyzers/sentiment.py
import asyncio
from src.analyzers.base import Analyzer, AnalysisType

class LocalSentimentAnalyzer(Analyzer):
    """Uses HuggingFace cardiffnlp/twitter-roberta-base-sentiment-latest"""

    def __init__(self):
        self._pipeline = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _load_model(self):
        if self._pipeline is None:
            from transformers import pipeline
            self._pipeline = pipeline(
                "sentiment-analysis",
                model="cardiffnlp/twitter-roberta-base-sentiment-latest",
                top_k=None,
                truncation=True,
                max_length=512
            )

    async def analyze(self, texts: list[str]) -> list[dict]:
        self._load_model()

        # Run in thread pool to avoid blocking
        loop = asyncio.get_event_loop()
        results = await loop.run_in_executor(None, self._pipeline, texts)

        normalized = []
        for result in results:
            # Convert to standardized format
            scores = {r["label"].lower(): r["score"] for r in result}
            normalized.append({
                "positive": scores.get("positive", 0),
                "negative": scores.get("negative", 0),
                "neutral": scores.get("neutral", 0),
                "label": max(scores, key=scores.get),
                "confidence": max(scores.values()),
            })
        return normalized

    def is_available(self) -> bool:
        try:
            import transformers
            return True
        except ImportError:
            return False


class OpenAISentimentAnalyzer(Analyzer):
    """Uses OpenAI API for sentiment analysis."""

    def __init__(self, api_key: str):
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _get_client(self):
        if self._client is None:
            from openai import AsyncOpenAI
            self._client = AsyncOpenAI(api_key=self.api_key)
        return self._client

    async def analyze(self, texts: list[str]) -> list[dict]:
        client = self._get_client()

        # Batch texts into a single prompt for efficiency
        prompt = """Analyze the sentiment of each message. Return JSON array with objects containing:
- positive: float 0-1
- negative: float 0-1
- neutral: float 0-1
- label: "positive" | "negative" | "neutral"

Messages:
"""
        for i, text in enumerate(texts):
            prompt += f"{i+1}. {text[:500]}\n"  # Truncate long texts

        response = await client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"},
            temperature=0
        )

        import json
        results = json.loads(response.choices[0].message.content)
        return results.get("sentiments", results.get("results", []))

    def is_available(self) -> bool:
        return bool(self.api_key)


class AnthropicSentimentAnalyzer(Analyzer):
    """Uses Anthropic Claude for sentiment analysis."""

    def __init__(self, api_key: str):
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _get_client(self):
        if self._client is None:
            from anthropic import AsyncAnthropic
            self._client = AsyncAnthropic(api_key=self.api_key)
        return self._client

    async def analyze(self, texts: list[str]) -> list[dict]:
        client = self._get_client()

        prompt = """Analyze the sentiment of each message. Return a JSON array where each object has:
- positive: float between 0 and 1
- negative: float between 0 and 1
- neutral: float between 0 and 1
- label: the dominant sentiment ("positive", "negative", or "neutral")

Messages to analyze:
"""
        for i, text in enumerate(texts):
            prompt += f"{i+1}. {text[:500]}\n"

        response = await client.messages.create(
            model="claude-3-haiku-20240307",
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}]
        )

        import json
        # Extract JSON from response
        content = response.content[0].text
        # Find JSON array in response
        start = content.find("[")
        end = content.rfind("]") + 1
        if start >= 0 and end > start:
            return json.loads(content[start:end])
        return []

    def is_available(self) -> bool:
        return bool(self.api_key)
```

### Toxicity Analyzers

```python
# src/analyzers/toxicity.py
import asyncio
import httpx
from src.analyzers.base import Analyzer, AnalysisType

class LocalToxicityAnalyzer(Analyzer):
    """Uses unitary/toxic-bert for toxicity detection."""

    def __init__(self):
        self._pipeline = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    def _load_model(self):
        if self._pipeline is None:
            from transformers import pipeline
            self._pipeline = pipeline(
                "text-classification",
                model="unitary/toxic-bert",
                top_k=None,
                truncation=True,
                max_length=512
            )

    async def analyze(self, texts: list[str]) -> list[dict]:
        self._load_model()

        loop = asyncio.get_event_loop()
        results = await loop.run_in_executor(None, self._pipeline, texts)

        normalized = []
        for result in results:
            scores = {r["label"].lower(): r["score"] for r in result}
            toxicity = scores.get("toxic", 0)
            normalized.append({
                "toxicity_score": toxicity,
                "is_toxic": toxicity > 0.5,
                "categories": {
                    "toxic": scores.get("toxic", 0),
                    "severe_toxic": scores.get("severe_toxic", 0),
                    "obscene": scores.get("obscene", 0),
                    "threat": scores.get("threat", 0),
                    "insult": scores.get("insult", 0),
                    "identity_hate": scores.get("identity_hate", 0),
                }
            })
        return normalized

    def is_available(self) -> bool:
        try:
            import transformers
            return True
        except ImportError:
            return False


class PerspectiveToxicityAnalyzer(Analyzer):
    """Uses Google Perspective API for toxicity detection."""

    ENDPOINT = "https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze"

    def __init__(self, api_key: str):
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    async def analyze(self, texts: list[str]) -> list[dict]:
        async with httpx.AsyncClient() as client:
            results = []
            for text in texts:
                response = await client.post(
                    self.ENDPOINT,
                    params={"key": self.api_key},
                    json={
                        "comment": {"text": text[:3000]},  # API limit
                        "languages": ["pt", "en"],
                        "requestedAttributes": {
                            "TOXICITY": {},
                            "SEVERE_TOXICITY": {},
                            "INSULT": {},
                            "THREAT": {},
                            "IDENTITY_ATTACK": {},
                        }
                    },
                    timeout=30.0
                )

                if response.status_code == 200:
                    data = response.json()
                    scores = data.get("attributeScores", {})

                    toxicity = scores.get("TOXICITY", {}).get("summaryScore", {}).get("value", 0)
                    results.append({
                        "toxicity_score": toxicity,
                        "is_toxic": toxicity > 0.5,
                        "categories": {
                            "toxicity": toxicity,
                            "severe_toxicity": scores.get("SEVERE_TOXICITY", {}).get("summaryScore", {}).get("value", 0),
                            "insult": scores.get("INSULT", {}).get("summaryScore", {}).get("value", 0),
                            "threat": scores.get("THREAT", {}).get("summaryScore", {}).get("value", 0),
                            "identity_attack": scores.get("IDENTITY_ATTACK", {}).get("summaryScore", {}).get("value", 0),
                        }
                    })
                else:
                    # Return neutral on API error
                    results.append({
                        "toxicity_score": 0,
                        "is_toxic": False,
                        "categories": {},
                        "error": f"API error: {response.status_code}"
                    })

            return results

    def is_available(self) -> bool:
        return bool(self.api_key)


class OpenAIModerationAnalyzer(Analyzer):
    """Uses OpenAI Moderation API (free with API key)."""

    def __init__(self, api_key: str):
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    def _get_client(self):
        if self._client is None:
            from openai import AsyncOpenAI
            self._client = AsyncOpenAI(api_key=self.api_key)
        return self._client

    async def analyze(self, texts: list[str]) -> list[dict]:
        client = self._get_client()

        # OpenAI moderation supports batching
        response = await client.moderations.create(input=texts)

        results = []
        for result in response.results:
            # Aggregate categories into toxicity score
            scores = result.category_scores
            toxicity = max(
                scores.hate,
                scores.harassment,
                scores.violence,
                scores.self_harm,
            )

            results.append({
                "toxicity_score": toxicity,
                "is_toxic": result.flagged,
                "categories": {
                    "hate": scores.hate,
                    "harassment": scores.harassment,
                    "violence": scores.violence,
                    "self_harm": scores.self_harm,
                    "sexual": scores.sexual,
                }
            })

        return results

    def is_available(self) -> bool:
        return bool(self.api_key)
```

### Topic Clustering

```python
# src/analyzers/topics.py
import asyncio
import numpy as np
from src.analyzers.base import Analyzer, AnalysisType

class LocalTopicClusterer(Analyzer):
    """Uses sentence-transformers + HDBSCAN for topic clustering."""

    def __init__(self):
        self._model = None
        self._clusterer = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOPICS

    def _load_model(self):
        if self._model is None:
            from sentence_transformers import SentenceTransformer
            # Multilingual model for Portuguese support
            self._model = SentenceTransformer("paraphrase-multilingual-MiniLM-L12-v2")

    async def analyze(self, texts: list[str]) -> list[dict]:
        self._load_model()

        loop = asyncio.get_event_loop()

        # Get embeddings
        embeddings = await loop.run_in_executor(
            None,
            lambda: self._model.encode(texts, show_progress_bar=False)
        )

        # Cluster
        from sklearn.cluster import HDBSCAN
        clusterer = HDBSCAN(min_cluster_size=3, min_samples=2)
        labels = clusterer.fit_predict(embeddings)

        results = []
        for i, label in enumerate(labels):
            results.append({
                "cluster_id": int(label),  # -1 means noise/outlier
                "is_outlier": label == -1,
                "embedding": embeddings[i].tolist()  # Optional: store for later
            })

        return results

    def is_available(self) -> bool:
        try:
            import sentence_transformers
            import sklearn
            return True
        except ImportError:
            return False


class OpenAITopicClusterer(Analyzer):
    """Uses OpenAI embeddings + local clustering."""

    def __init__(self, api_key: str):
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOPICS

    def _get_client(self):
        if self._client is None:
            from openai import AsyncOpenAI
            self._client = AsyncOpenAI(api_key=self.api_key)
        return self._client

    async def analyze(self, texts: list[str]) -> list[dict]:
        client = self._get_client()

        # Get embeddings from OpenAI
        response = await client.embeddings.create(
            model="text-embedding-3-small",
            input=texts
        )

        embeddings = np.array([e.embedding for e in response.data])

        # Cluster locally (cheap, no additional API cost)
        from sklearn.cluster import HDBSCAN
        clusterer = HDBSCAN(min_cluster_size=3, min_samples=2)
        labels = clusterer.fit_predict(embeddings)

        results = []
        for i, label in enumerate(labels):
            results.append({
                "cluster_id": int(label),
                "is_outlier": label == -1,
            })

        return results

    def is_available(self) -> bool:
        return bool(self.api_key)
```

---

## Main Processor

```python
# src/processor.py
import logging
from datetime import datetime, timedelta
from typing import Optional
from sqlalchemy.ext.asyncio import AsyncSession

from src.config import MLSettings
from src.analyzers.base import AnalyzerRegistry, AnalysisType
from src.database import get_session
from src.models import Message, AnalysisResult

logger = logging.getLogger(__name__)


class MessageProcessor:
    """
    Main processor that coordinates analysis across all configured analyzers.
    """

    def __init__(self, config: MLSettings, registry: AnalyzerRegistry):
        self.config = config
        self.registry = registry

    async def process_unprocessed_messages(
        self,
        chat_id: int,
        limit: Optional[int] = None
    ) -> int:
        """
        Process messages that haven't been analyzed yet.

        Returns:
            Number of messages processed
        """
        limit = limit or self.config.batch_size

        async with get_session() as session:
            # Fetch unprocessed messages
            messages = await self._get_unprocessed_messages(session, chat_id, limit)

            if not messages:
                logger.info(f"No unprocessed messages for chat {chat_id}")
                return 0

            logger.info(f"Processing {len(messages)} messages for chat {chat_id}")

            # Extract texts
            texts = [m.text for m in messages if m.text]
            message_ids = [m.id for m in messages if m.text]

            if not texts:
                return 0

            # Run all configured analyzers
            all_results = {}
            for analysis_type in AnalysisType:
                try:
                    analyzer = self.registry.get(analysis_type)
                    if analyzer.is_available():
                        results = await analyzer.analyze(texts)
                        all_results[analysis_type] = results
                        logger.debug(f"Completed {analysis_type.value} analysis")
                except Exception as e:
                    logger.error(f"Failed {analysis_type.value} analysis: {e}")

            # Save results
            await self._save_results(session, message_ids, all_results)

            return len(messages)

    async def _get_unprocessed_messages(
        self,
        session: AsyncSession,
        chat_id: int,
        limit: int
    ) -> list[Message]:
        """Fetch messages that haven't been analyzed."""
        from sqlalchemy import select, and_

        # Messages without analysis results
        stmt = (
            select(Message)
            .where(
                and_(
                    Message.chat_id == chat_id,
                    Message.text.isnot(None),
                    Message.ml_processed_at.is_(None)  # Not yet processed
                )
            )
            .order_by(Message.date.asc())
            .limit(limit)
        )

        result = await session.execute(stmt)
        return result.scalars().all()

    async def _save_results(
        self,
        session: AsyncSession,
        message_ids: list[int],
        results: dict[AnalysisType, list[dict]]
    ):
        """Save analysis results to database."""
        from sqlalchemy import update

        for i, message_id in enumerate(message_ids):
            # Build combined result
            combined = {
                "analyzed_at": datetime.utcnow().isoformat(),
            }

            for analysis_type, analysis_results in results.items():
                if i < len(analysis_results):
                    combined[analysis_type.value] = analysis_results[i]

            # Update message with results
            stmt = (
                update(Message)
                .where(Message.id == message_id)
                .values(
                    ml_analysis=combined,
                    ml_processed_at=datetime.utcnow()
                )
            )
            await session.execute(stmt)

        await session.commit()
        logger.info(f"Saved analysis results for {len(message_ids)} messages")
```

---

## Entry Point (CLI Job)

**The ML processor runs as a batch job, not a daemon.**

```python
# src/main.py
import asyncio
import argparse
import logging
import sys

from src.config import MLSettings
from src.analyzers.base import AnalyzerRegistry
from src.processor import MessageProcessor
from src.database import init_db

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


async def run_processing(chat_id: int, batch_size: int, limit: int | None) -> int:
    """
    Run ML processing as a batch job.

    Args:
        chat_id: Target chat to process
        batch_size: Messages per batch
        limit: Max total messages to process (None = unlimited)

    Returns:
        Total messages processed
    """
    # Load config
    config = MLSettings()

    # Validate API keys
    errors = config.validate_api_keys()
    if errors:
        for error in errors:
            logger.error(f"Configuration error: {error}")
        raise SystemExit(1)

    # Initialize database
    await init_db(config.database_url)

    # Create registry and processor
    registry = AnalyzerRegistry(config)
    processor = MessageProcessor(config, registry)

    # Log configured providers
    for analysis_type in ["sentiment", "toxicity", "topics", "humor", "questions", "ner"]:
        provider = getattr(config, f"{analysis_type}_provider")
        logger.info(f"{analysis_type}: {provider}")

    # Process messages in batches
    total_processed = 0
    remaining = limit

    while True:
        # Determine batch size for this iteration
        current_batch = batch_size
        if remaining is not None:
            current_batch = min(batch_size, remaining)
            if current_batch <= 0:
                break

        # Process batch
        count = await processor.process_unprocessed_messages(chat_id, limit=current_batch)
        total_processed += count

        logger.info(f"Batch complete: {count} messages (total: {total_processed})")

        # Update remaining
        if remaining is not None:
            remaining -= count

        # No more messages to process
        if count < current_batch:
            break

    logger.info(f"Processing complete: {total_processed} total messages processed")
    return total_processed


def main():
    parser = argparse.ArgumentParser(
        description="ML Processor - Batch job for analyzing messages"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    # Process command
    process_parser = subparsers.add_parser("process", help="Process unanalyzed messages")
    process_parser.add_argument(
        "--chat-id", type=int, required=True,
        help="Target chat ID to process"
    )
    process_parser.add_argument(
        "--batch-size", type=int, default=500,
        help="Messages per batch (default: 500)"
    )
    process_parser.add_argument(
        "--limit", type=int, default=None,
        help="Max messages to process (default: unlimited)"
    )

    # Status command
    status_parser = subparsers.add_parser("status", help="Show processing status")
    status_parser.add_argument(
        "--chat-id", type=int, required=True,
        help="Target chat ID"
    )

    args = parser.parse_args()

    if args.command == "process":
        result = asyncio.run(run_processing(
            chat_id=args.chat_id,
            batch_size=args.batch_size,
            limit=args.limit
        ))
        sys.exit(0 if result >= 0 else 1)

    elif args.command == "status":
        # TODO: Implement status command
        logger.info(f"Status for chat {args.chat_id}: Not implemented yet")
        sys.exit(0)


if __name__ == "__main__":
    main()
```

### Usage Examples

```bash
# Process all unprocessed messages (runs until done)
python -m src.main process --chat-id -1003280306634

# Process with batch size control
python -m src.main process --chat-id -1003280306634 --batch-size 100

# Process limited number (for testing or incremental runs)
python -m src.main process --chat-id -1003280306634 --limit 1000

# Check processing status
python -m src.main status --chat-id -1003280306634
```

---

## Database Schema Additions

```sql
-- Add ML analysis columns to messages table
ALTER TABLE messages ADD COLUMN IF NOT EXISTS ml_analysis JSONB;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS ml_processed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_messages_ml_unprocessed
    ON messages(chat_id, date)
    WHERE ml_processed_at IS NULL AND text IS NOT NULL;

-- Aggregated user stats (computed periodically)
CREATE TABLE IF NOT EXISTS ml_user_stats (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,

    -- Sentiment aggregates
    avg_sentiment_positive NUMERIC(5,4),
    avg_sentiment_negative NUMERIC(5,4),
    avg_sentiment_neutral NUMERIC(5,4),
    dominant_sentiment VARCHAR(20),

    -- Toxicity aggregates
    avg_toxicity_score NUMERIC(5,4),
    max_toxicity_score NUMERIC(5,4),
    toxic_message_count INTEGER DEFAULT 0,
    toxicity_label VARCHAR(30),

    -- Topic aggregates
    primary_topic_cluster INTEGER,
    topic_diversity_score NUMERIC(5,4),

    -- Humor aggregates
    avg_humor_score NUMERIC(5,4),
    humor_label VARCHAR(30),

    -- Timestamps
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    computed_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(user_id, chat_id, period_start, period_end)
);

CREATE INDEX idx_ml_user_stats_lookup ON ml_user_stats(chat_id, user_id);
```

---

## Docker Configuration

### Dockerfile

```dockerfile
# apps/ml-processor/Dockerfile

# Base image with Python
FROM python:3.12-slim as base

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy source
COPY src/ src/

# Default command
CMD ["python", "-m", "src.main"]


# Variant with local models (larger image, includes torch)
FROM base as local

COPY requirements-local.txt .
RUN pip install --no-cache-dir -r requirements-local.txt

# Download models at build time (optional, for faster startup)
RUN python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='cardiffnlp/twitter-roberta-base-sentiment-latest')"
```

### requirements.txt (API-only)

```txt
pydantic>=2.0
pydantic-settings>=2.0
sqlalchemy[asyncio]>=2.0
asyncpg>=0.29
apscheduler>=3.10
httpx>=0.25
openai>=1.0
anthropic>=0.18
numpy>=1.24
scikit-learn>=1.3
```

### requirements-local.txt (with HuggingFace)

```txt
-r requirements.txt
torch>=2.0
transformers>=4.35
sentence-transformers>=2.2
spacy>=3.7
```

### docker-compose.dev.yml addition

```yaml
services:
  ml-processor:
    build:
      context: ./apps/ml-processor
      target: local  # Use 'base' for API-only
    environment:
      ML_DATABASE_URL: postgresql://user:password@postgres:5432/beef
      ML_SENTIMENT_PROVIDER: local
      ML_TOXICITY_PROVIDER: perspective
      ML_TOPICS_PROVIDER: openai
      ML_OPENAI_API_KEY: ${OPENAI_API_KEY}
      ML_PERSPECTIVE_API_KEY: ${PERSPECTIVE_API_KEY}
    depends_on:
      - postgres
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

---

## Adding New Analysis Types

To add a new analysis type (e.g., "emotion detection"):

### 1. Add to AnalysisType enum

```python
# src/analyzers/base.py
class AnalysisType(str, Enum):
    # ... existing types
    EMOTION = "emotion"
```

### 2. Add config option

```python
# src/config.py
class MLSettings(BaseSettings):
    # ... existing
    emotion_provider: ProviderType = "local"
```

### 3. Create analyzer file

```python
# src/analyzers/emotion.py
from src.analyzers.base import Analyzer, AnalysisType

class LocalEmotionAnalyzer(Analyzer):
    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.EMOTION

    async def analyze(self, texts: list[str]) -> list[dict]:
        # Implementation
        pass

    def is_available(self) -> bool:
        return True

class OpenAIEmotionAnalyzer(Analyzer):
    # ... API implementation
    pass
```

### 4. Register in factory

```python
# src/analyzers/base.py - in AnalyzerRegistry._create_analyzer()
elif analysis_type == AnalysisType.EMOTION:
    if provider == "local":
        from src.analyzers.emotion import LocalEmotionAnalyzer
        return LocalEmotionAnalyzer()
    elif provider == "openai":
        from src.analyzers.emotion import OpenAIEmotionAnalyzer
        return OpenAIEmotionAnalyzer(self.config.openai_api_key)
```

---

## Cost Comparison

| Analysis | Local | OpenAI | Perspective | Anthropic |
|----------|-------|--------|-------------|-----------|
| Sentiment | GPU time | ~$0.02/1K | N/A | ~$0.01/1K |
| Toxicity | GPU time | Free (moderation) | Free tier | N/A |
| Topics (embeddings) | GPU time | ~$0.02/1K | N/A | N/A |
| Humor | GPU time | ~$0.10/1K | N/A | ~$0.05/1K |
| NER | GPU time | ~$0.10/1K | N/A | ~$0.05/1K |

**Recommended hybrid setup:**
- **Sentiment**: Local (cardiffnlp model is excellent)
- **Toxicity**: Perspective API (purpose-built, free tier)
- **Topics**: OpenAI embeddings (best quality) + local clustering
- **Humor/Questions/NER**: Local (good enough, saves costs)

---

## Testing

```python
# tests/test_analyzers.py
import pytest
from src.config import MLSettings
from src.analyzers.base import AnalyzerRegistry, AnalysisType

@pytest.fixture
def config():
    return MLSettings(
        database_url="postgresql://test:test@localhost/test",
        sentiment_provider="local",
        toxicity_provider="local",
    )

@pytest.fixture
def registry(config):
    return AnalyzerRegistry(config)

@pytest.mark.asyncio
async def test_sentiment_local(registry):
    analyzer = registry.get(AnalysisType.SENTIMENT)

    results = await analyzer.analyze([
        "I love this!",
        "This is terrible.",
        "The weather is nice today."
    ])

    assert len(results) == 3
    assert results[0]["label"] == "positive"
    assert results[1]["label"] == "negative"
    assert "confidence" in results[0]

@pytest.mark.asyncio
async def test_provider_switching(config):
    # Test with OpenAI
    config.sentiment_provider = "openai"
    config.openai_api_key = "test-key"

    registry = AnalyzerRegistry(config)
    analyzer = registry.get(AnalysisType.SENTIMENT)

    assert "OpenAI" in analyzer.get_provider_name()
```

---

## Related Documentation

- `local_docs/CARD_GENERATION_IMPLEMENTATION.md` - Weekly user card generation
- `local_docs/USER_CARDS_API_IMPLEMENTATION.md` - REST API for serving cards
- `docs/plans/user-cards-green-stats.md` - Green tier stats (no new models)
- `docs/plans/user-cards-yellow-stats.md` - Yellow tier stats (aggregations + embeddings)
- `docs/plans/user-cards-red-stats.md` - Red tier stats (new ML models)

---

## Summary

This architecture provides:

1. **Flexibility**: Switch between local and API providers via config
2. **Extensibility**: Add new analysis types with minimal code changes
3. **Efficiency**: Lazy loading prevents unnecessary resource usage
4. **Reliability**: Fallback support for graceful degradation
5. **Cost control**: Mix expensive and free providers per analysis type

The same codebase runs with local GPU models or paid APIs, determined entirely by environment variables.

---

**Last Updated**: 2025-12-17
**Status**: Implementation Guide
