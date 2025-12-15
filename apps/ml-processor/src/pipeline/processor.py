"""
Main ML processing pipeline.
"""

import logging
import time
from typing import Optional

from config import Config
from src.api.client import APIClient, MLResult, SentimentResult, ToxicityResult
from src.models import ModelManager
from src.vector.qdrant import QdrantWrapper

logger = logging.getLogger(__name__)

PROCESSOR_VERSION = "v1.0.0"


class MLProcessor:
    """Main ML processing pipeline."""

    def __init__(self, config: Config):
        """
        Initialize the ML processor.

        Args:
            config: Application configuration
        """
        self.config = config
        self.models = ModelManager(device=config.device)
        self.api_client: Optional[APIClient] = None
        self.qdrant: Optional[QdrantWrapper] = None

    def setup(self):
        """Initialize clients and load models."""
        logger.info("Setting up ML processor...")

        # Initialize API client
        self.api_client = APIClient(
            base_url=self.config.api_service_url,
            api_key=self.config.api_key,
        )
        logger.info(f"API client connected to: {self.config.api_service_url}")

        # Initialize Qdrant client
        self.qdrant = QdrantWrapper(
            host=self.config.qdrant_host,
            port=self.config.qdrant_port,
        )
        qdrant_info = self.qdrant.get_collection_info()
        logger.info(f"Qdrant connected: {qdrant_info}")

        # Load ML models
        self.models.load_all(
            sentiment_model=self.config.sentiment_model,
            toxicity_model=self.config.toxicity_model,
            embedding_model=self.config.embedding_model,
        )

    def cleanup(self):
        """Cleanup resources."""
        if self.api_client:
            self.api_client.close()
        self.models.unload_all()
        logger.info("ML processor cleaned up")

    def run_once(self) -> int:
        """
        Run a single processing cycle.

        Returns:
            Number of messages processed
        """
        # Fetch unprocessed messages
        response = self.api_client.get_messages(limit=self.config.batch_size)

        if not response.messages:
            logger.debug("No unprocessed messages")
            return 0

        messages = response.messages
        logger.info(f"Processing {len(messages)} messages (has_more={response.has_more})")

        # Extract texts
        texts = [m.text for m in messages]

        # Run sentiment analysis
        sentiment_results = self.models.sentiment_analyzer.analyze(texts)

        # Run toxicity detection
        toxicity_results = self.models.toxicity_detector.analyze(texts)

        # Generate embeddings
        embeddings = self.models.embedding_encoder.encode(texts)

        # Store embeddings in Qdrant
        message_ids = [m.id for m in messages]
        chat_ids = [m.chat_id for m in messages]
        user_ids = [m.user_id for m in messages]

        self.qdrant.upsert_embeddings(
            message_ids=message_ids,
            chat_ids=chat_ids,
            user_ids=user_ids,
            texts=texts,
            embeddings=embeddings,
        )

        # Prepare results for API
        ml_results = []
        for i, msg in enumerate(messages):
            sent = sentiment_results[i]
            tox = toxicity_results[i]

            ml_results.append(
                MLResult(
                    message_id=msg.id,
                    chat_id=msg.chat_id,
                    sentiment=SentimentResult(
                        label=sent.label,
                        scores={
                            "positive": sent.score_positive,
                            "neutral": sent.score_neutral,
                            "negative": sent.score_negative,
                        },
                    ),
                    toxicity=ToxicityResult(
                        is_toxic=tox.is_toxic,
                        label=tox.label,
                        score=tox.score,
                    ),
                )
            )

        # Post results to API
        result = self.api_client.post_results(ml_results, PROCESSOR_VERSION)
        logger.info(f"Posted results: {result}")

        return len(messages)

    def run_continuous(self):
        """Run continuous processing loop."""
        logger.info("Starting continuous processing loop...")

        try:
            while True:
                processed = self.run_once()

                if processed == 0:
                    # No messages to process, wait longer
                    logger.debug(f"Sleeping for {self.config.sleep_seconds}s (no messages)")
                    time.sleep(self.config.sleep_seconds)
                else:
                    # Brief pause between batches
                    time.sleep(1)

        except KeyboardInterrupt:
            logger.info("Received interrupt, shutting down...")

    def print_status(self):
        """Print current processing status."""
        status = self.api_client.get_status()
        qdrant_info = self.qdrant.get_collection_info()

        print("\n=== ML Processing Status ===")
        print(f"Total messages with text: {status.get('total_with_text', 0):,}")
        print(f"Processed:                {status.get('processed', 0):,}")
        print(f"Unprocessed:              {status.get('unprocessed', 0):,}")
        print(f"Sentiment analyzed:       {status.get('sentiment_analyzed', 0):,}")
        print(f"Toxicity analyzed:        {status.get('toxicity_analyzed', 0):,}")
        print(f"Toxic messages:           {status.get('toxic_messages', 0):,}")
        print(f"\n=== Qdrant Status ===")
        points = qdrant_info.get('points_count', 0) or 0
        print(f"Embeddings stored:        {points:,}")
        print(f"Collection status:        {qdrant_info.get('status', 'unknown')}")
        print()
