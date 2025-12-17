"""
Main ML processing pipeline.

Coordinates analyzers and handles database operations.
"""

import logging
import time
from typing import TYPE_CHECKING

import numpy as np

from src.analyzers.base import AnalysisType, AnalyzerRegistry
from src.database import MLQueries
from src.vector.qdrant import QdrantWrapper

if TYPE_CHECKING:
    from config import Config

logger = logging.getLogger(__name__)

PROCESSOR_VERSION = "v2.0.0"


class MLProcessor:
    """
    Main ML processing pipeline.

    Orchestrates:
    1. Fetching unprocessed messages from database
    2. Running configured analyzers (sentiment, toxicity, embeddings, topics, NER)
    3. Storing embeddings in Qdrant
    4. Saving results to ml_* tables
    5. Marking messages as processed
    """

    def __init__(
        self,
        config: "Config",
        queries: MLQueries,
        registry: AnalyzerRegistry,
    ):
        """
        Initialize the ML processor.

        Args:
            config: Application configuration
            queries: Database query helper
            registry: Analyzer registry for lazy-loading analyzers
        """
        self.config = config
        self.queries = queries
        self.registry = registry
        self.qdrant: QdrantWrapper | None = None

    def setup(self):
        """Initialize Qdrant connection."""
        logger.info("Setting up ML processor...")

        # Initialize Qdrant client
        self.qdrant = QdrantWrapper(
            host=self.config.qdrant_host,
            port=self.config.qdrant_port,
        )
        qdrant_info = self.qdrant.get_collection_info()
        logger.info(f"Qdrant connected: {qdrant_info}")

        logger.info("ML processor setup complete")

    def cleanup(self):
        """Release resources."""
        self.registry.cleanup_all()
        self.qdrant = None
        logger.info("ML processor cleaned up")

    def process_batch(
        self,
        chat_id: int,
        limit: int | None = None,
    ) -> int:
        """
        Process a batch of unprocessed messages.

        Args:
            chat_id: Target chat ID
            limit: Maximum messages to process (defaults to config.batch_size)

        Returns:
            Number of messages processed
        """
        limit = limit or self.config.batch_size

        # Fetch unprocessed messages
        messages = self.queries.get_unprocessed_messages(chat_id, limit)

        if not messages:
            logger.debug(f"No unprocessed messages for chat {chat_id}")
            return 0

        logger.info(f"Processing {len(messages)} messages for chat {chat_id}")

        # Extract texts and IDs
        texts = [m["text"] for m in messages]
        message_ids = [m["id"] for m in messages]
        user_ids = [m["user_id"] for m in messages]

        # Run analyzers
        sentiment_results = self._run_sentiment(texts)
        toxicity_results = self._run_toxicity(texts)
        embeddings = self._run_embeddings(texts)
        ner_results = self._run_ner(texts)

        # Store embeddings in Qdrant
        if embeddings is not None and self.qdrant:
            self._store_embeddings(
                message_ids=message_ids,
                chat_ids=[chat_id] * len(messages),
                user_ids=user_ids,
                texts=texts,
                embeddings=embeddings,
            )

        # Run topic clustering (requires embeddings)
        topic_results, cluster_keywords, cluster_counts = self._run_topics(
            texts, embeddings
        )

        # Save results to database
        self._save_results(
            chat_id=chat_id,
            messages=messages,
            sentiment_results=sentiment_results,
            toxicity_results=toxicity_results,
            ner_results=ner_results,
            topic_results=topic_results,
            cluster_keywords=cluster_keywords,
            cluster_counts=cluster_counts,
        )

        # Mark messages as processed
        self.queries.mark_processed(
            message_ids=message_ids,
            chat_id=chat_id,
            processor_version=PROCESSOR_VERSION,
        )

        logger.info(f"Processed {len(messages)} messages")
        return len(messages)

    def _run_sentiment(self, texts: list[str]) -> list[dict] | None:
        """Run sentiment analysis if available."""
        analyzer = self.registry.get_if_available(AnalysisType.SENTIMENT)
        if not analyzer:
            return None

        try:
            return analyzer.analyze(texts)
        except Exception as e:
            logger.error(f"Sentiment analysis failed: {e}")
            return None

    def _run_toxicity(self, texts: list[str]) -> list[dict] | None:
        """Run toxicity detection if available."""
        analyzer = self.registry.get_if_available(AnalysisType.TOXICITY)
        if not analyzer:
            return None

        try:
            return analyzer.analyze(texts)
        except Exception as e:
            logger.error(f"Toxicity detection failed: {e}")
            return None

    def _run_embeddings(self, texts: list[str]) -> np.ndarray | None:
        """Run embedding generation if available."""
        analyzer = self.registry.get_if_available(AnalysisType.EMBEDDINGS)
        if not analyzer:
            return None

        try:
            # Use the encode() method directly for efficiency
            from src.analyzers.embeddings import LocalEmbeddingEncoder

            if isinstance(analyzer, LocalEmbeddingEncoder):
                return analyzer.encode(texts)
            else:
                # Fallback for other implementations
                results = analyzer.analyze(texts)
                return np.array([r["embedding"] for r in results])
        except Exception as e:
            logger.error(f"Embedding generation failed: {e}")
            return None

    def _run_ner(self, texts: list[str]) -> list[list[dict]] | None:
        """Run NER extraction if available."""
        analyzer = self.registry.get_if_available(AnalysisType.NER)
        if not analyzer:
            return None

        try:
            return analyzer.analyze(texts)
        except Exception as e:
            logger.error(f"NER extraction failed: {e}")
            return None

    def _run_topics(
        self,
        texts: list[str],
        embeddings: np.ndarray | None,
    ) -> tuple[list[dict] | None, dict[int, list[str]], dict[int, int]]:
        """
        Run topic clustering if available.

        Returns:
            (topic_results, cluster_keywords, cluster_counts)
        """
        if embeddings is None:
            return None, {}, {}

        analyzer = self.registry.get_if_available(AnalysisType.TOPICS)
        if not analyzer:
            return None, {}, {}

        try:
            # Run clustering
            results = analyzer.analyze(texts, embeddings=embeddings)

            # Extract keywords and counts
            from src.analyzers.topics import LocalTopicClusterer

            if isinstance(analyzer, LocalTopicClusterer):
                labels = [r["topic_id"] for r in results]
                keywords = analyzer.extract_keywords(texts, labels)
                counts = analyzer.get_cluster_stats(labels)
            else:
                keywords = {}
                counts = {}

            return results, keywords, counts

        except Exception as e:
            logger.error(f"Topic clustering failed: {e}")
            return None, {}, {}

    def _store_embeddings(
        self,
        message_ids: list[int],
        chat_ids: list[int],
        user_ids: list[int | None],
        texts: list[str],
        embeddings: np.ndarray,
    ):
        """Store embeddings in Qdrant."""
        try:
            self.qdrant.upsert_embeddings(
                message_ids=message_ids,
                chat_ids=chat_ids,
                user_ids=user_ids,
                texts=texts,
                embeddings=embeddings,
            )
            logger.debug(f"Stored {len(message_ids)} embeddings in Qdrant")
        except Exception as e:
            logger.error(f"Failed to store embeddings in Qdrant: {e}")

    def _save_results(
        self,
        chat_id: int,
        messages: list[dict],
        sentiment_results: list[dict] | None,
        toxicity_results: list[dict] | None,
        ner_results: list[list[dict]] | None,
        topic_results: list[dict] | None,
        cluster_keywords: dict[int, list[str]],
        cluster_counts: dict[int, int],
    ):
        """Save all analysis results to database."""
        # Save sentiment results
        if sentiment_results:
            db_results = []
            for i, msg in enumerate(messages):
                if i < len(sentiment_results):
                    result = sentiment_results[i]
                    db_results.append(
                        {
                            "message_id": msg["id"],
                            "chat_id": chat_id,
                            "label": result["label"],
                            "score_positive": result["score_positive"],
                            "score_neutral": result["score_neutral"],
                            "score_negative": result["score_negative"],
                            "confidence": result["confidence"],
                        }
                    )
            count = self.queries.save_sentiment_results(db_results)
            logger.debug(f"Saved {count} sentiment results")

        # Save toxicity results
        if toxicity_results:
            db_results = []
            for i, msg in enumerate(messages):
                if i < len(toxicity_results):
                    result = toxicity_results[i]
                    db_results.append(
                        {
                            "message_id": msg["id"],
                            "chat_id": chat_id,
                            "is_toxic": result["is_toxic"],
                            "label": result["label"],
                            "score": result["score"],
                        }
                    )
            count = self.queries.save_toxicity_results(db_results)
            logger.debug(f"Saved {count} toxicity results")

        # Save NER results
        if ner_results:
            db_results = []
            for i, msg in enumerate(messages):
                if i < len(ner_results):
                    for entity in ner_results[i]:
                        db_results.append(
                            {
                                "message_id": msg["id"],
                                "chat_id": chat_id,
                                "entity_type": entity["entity_type"],
                                "entity_text": entity["entity_text"],
                                "start_pos": entity.get("start_pos"),
                                "end_pos": entity.get("end_pos"),
                                "confidence": entity.get("confidence", 1.0),
                            }
                        )
            if db_results:
                count = self.queries.save_ner_results(db_results)
                logger.debug(f"Saved {count} NER entities")

        # Save topic clusters
        if cluster_keywords:
            count = self.queries.save_topic_clusters(
                chat_id=chat_id,
                clusters=cluster_keywords,
                message_counts=cluster_counts,
            )
            logger.debug(f"Saved {count} topic clusters")

        # Save message-topic assignments
        if topic_results:
            db_results = []
            for i, msg in enumerate(messages):
                if i < len(topic_results):
                    result = topic_results[i]
                    if result["topic_id"] >= 0:  # Skip outliers
                        db_results.append(
                            {
                                "message_id": msg["id"],
                                "chat_id": chat_id,
                                "topic_id": result["topic_id"],
                                "similarity": result["similarity"],
                            }
                        )
            if db_results:
                count = self.queries.save_message_topics(db_results)
                logger.debug(f"Saved {count} topic assignments")

    def run_continuous(self, chat_id: int):
        """
        Run continuous processing loop.

        Processes messages in batches, sleeping when no messages are available.

        Args:
            chat_id: Target chat ID
        """
        logger.info(f"Starting continuous processing for chat {chat_id}...")

        try:
            while True:
                processed = self.process_batch(chat_id)

                if processed == 0:
                    # No messages to process, wait longer
                    logger.debug(
                        f"Sleeping for {self.config.sleep_seconds}s (no messages)"
                    )
                    time.sleep(self.config.sleep_seconds)
                else:
                    # Brief pause between batches
                    time.sleep(1)

        except KeyboardInterrupt:
            logger.info("Received interrupt, shutting down...")

    def print_status(self, chat_id: int):
        """
        Print current processing status.

        Args:
            chat_id: Target chat ID
        """
        status = self.queries.get_processing_status(chat_id)
        qdrant_info = self.qdrant.get_collection_info() if self.qdrant else {}

        print("\n=== ML Processing Status ===")
        print(f"Chat ID:                  {chat_id}")
        print(f"Total messages with text: {status.get('total_with_text', 0):,}")
        print(f"Processed:                {status.get('processed', 0):,}")
        print(f"Unprocessed:              {status.get('unprocessed', 0):,}")
        print(f"Sentiment analyzed:       {status.get('sentiment_analyzed', 0):,}")
        print(f"Toxicity analyzed:        {status.get('toxicity_analyzed', 0):,}")
        print(f"  - Toxic messages:       {status.get('toxic_count', 0):,}")
        print(f"Topics clustered:         {status.get('topics_clustered', 0):,}")
        print(f"NER entities extracted:   {status.get('ner_extracted', 0):,}")
        print(f"  - Unique entities:      {status.get('unique_entities', 0):,}")
        print(f"\n=== Qdrant Status ===")
        points = qdrant_info.get("points_count", 0) or 0
        print(f"Embeddings stored:        {points:,}")
        print(f"Collection status:        {qdrant_info.get('status', 'unknown')}")
        print()
