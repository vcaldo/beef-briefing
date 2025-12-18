"""
Database query layer for ML processor.

Design Principles:
1. Raw SQL with SQLAlchemy text() - no ORM
2. Parameterized queries with named parameters (:param)
3. Connection pooling via SQLAlchemy engine
4. Batch operations with ON CONFLICT for idempotency
"""

import logging
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.engine import Engine

from src.instrumentation import function_trace

logger = logging.getLogger(__name__)


class MLQueries:
    """
    Database operations for ML processing.

    Handles reading unprocessed messages and writing results
    to the ml_* tables.
    """

    def __init__(self, engine: Engine):
        """
        Initialize with SQLAlchemy engine.

        Args:
            engine: SQLAlchemy engine with connection pooling
        """
        self._engine = engine

    # =========================================
    # PRIVATE HELPERS
    # =========================================

    def _execute_single(self, query: str, params: dict) -> dict | None:
        """Execute query expecting 0 or 1 row, return dict or None."""
        with self._engine.connect() as conn:
            result = conn.execute(text(query), params)
            row = result.mappings().fetchone()
            return dict(row) if row else None

    def _execute_many(self, query: str, params: dict) -> list[dict]:
        """Execute query expecting multiple rows, return list of dicts."""
        with self._engine.connect() as conn:
            result = conn.execute(text(query), params)
            return [dict(row) for row in result.mappings()]

    def _execute_write(self, query: str, params: dict) -> int:
        """Execute write query and return rowcount."""
        with self._engine.connect() as conn:
            result = conn.execute(text(query), params)
            conn.commit()
            return result.rowcount

    def _execute_batch(self, query: str, params_list: list[dict]) -> int:
        """Execute batch write query and return total rowcount."""
        if not params_list:
            return 0

        with self._engine.connect() as conn:
            total = 0
            for params in params_list:
                result = conn.execute(text(query), params)
                total += result.rowcount
            conn.commit()
            return total

    # =========================================
    # READ OPERATIONS
    # =========================================

    @function_trace(name="get_unprocessed_messages", group="Database")
    def get_unprocessed_messages(
        self,
        chat_id: int,
        limit: int = 500,
        from_date: datetime | None = None,
        to_date: datetime | None = None,
    ) -> list[dict]:
        """
        Fetch messages that haven't been ML-processed.

        Args:
            chat_id: Target chat ID
            limit: Maximum messages to fetch
            from_date: Only fetch messages from this date (inclusive)
            to_date: Only fetch messages until this date (exclusive)

        Returns:
            List of message dicts with id, message_id, chat_id, user_id, text, date
        """
        # Build dynamic WHERE clause for date filtering
        date_filters = ""
        if from_date:
            date_filters += " AND m.date >= :from_date"
        if to_date:
            date_filters += " AND m.date < :to_date"

        query = f"""
            SELECT
                m.id,
                m.message_id,
                m.chat_id,
                m.user_id,
                COALESCE(m.text, m.caption) as text,
                m.date
            FROM messages m
            LEFT JOIN ml_processing_state mps ON mps.message_id = m.id
            WHERE m.chat_id = :chat_id
                AND mps.message_id IS NULL
                AND (m.text IS NOT NULL OR m.caption IS NOT NULL)
                {date_filters}
            ORDER BY m.date ASC
            LIMIT :limit
        """
        params: dict[str, Any] = {"chat_id": chat_id, "limit": limit}
        if from_date:
            params["from_date"] = from_date
        if to_date:
            params["to_date"] = to_date

        return self._execute_many(query, params)

    @function_trace(name="get_processing_status", group="Database")
    def get_processing_status(self, chat_id: int) -> dict:
        """
        Get processing statistics for a chat.

        Returns:
            {
                'total_with_text': int,
                'processed': int,
                'unprocessed': int,
                'sentiment_analyzed': int,
                'toxicity_analyzed': int,
                'toxic_count': int,
                'topics_clustered': int,
                'ner_extracted': int,
                'unique_entities': int,
                'humor_analyzed': int,
                'humorous_count': int,
                'questions_analyzed': int,
                'question_count': int
            }
        """
        # Total messages with text
        total_query = """
            SELECT COUNT(*) as count
            FROM messages
            WHERE chat_id = :chat_id
                AND (text IS NOT NULL OR caption IS NOT NULL)
        """
        total = self._execute_single(total_query, {"chat_id": chat_id}) or {}

        # Processed count
        processed_query = """
            SELECT COUNT(*) as count
            FROM ml_processing_state
            WHERE chat_id = :chat_id
        """
        processed = self._execute_single(processed_query, {"chat_id": chat_id}) or {}

        # Sentiment count
        sentiment_query = """
            SELECT COUNT(*) as count
            FROM ml_sentiment
            WHERE chat_id = :chat_id
        """
        sentiment = self._execute_single(sentiment_query, {"chat_id": chat_id}) or {}

        # Toxicity count and toxic messages
        toxicity_query = """
            SELECT
                COUNT(*) as count,
                SUM(CASE WHEN is_toxic THEN 1 ELSE 0 END) as toxic_count
            FROM ml_toxicity
            WHERE chat_id = :chat_id
        """
        toxicity = self._execute_single(toxicity_query, {"chat_id": chat_id}) or {}

        # Topic assignments
        topics_query = """
            SELECT COUNT(*) as count
            FROM ml_message_topics
            WHERE chat_id = :chat_id
        """
        topics = self._execute_single(topics_query, {"chat_id": chat_id}) or {}

        # NER entities
        ner_query = """
            SELECT
                COUNT(*) as count,
                COUNT(DISTINCT entity_text) as unique_entities
            FROM ml_ner
            WHERE chat_id = :chat_id
        """
        ner = self._execute_single(ner_query, {"chat_id": chat_id}) or {}

        # Humor count
        humor_query = """
            SELECT
                COUNT(*) as count,
                SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) as humorous_count
            FROM ml_humor
            WHERE chat_id = :chat_id
        """
        humor = self._execute_single(humor_query, {"chat_id": chat_id}) or {}

        # Questions count
        questions_query = """
            SELECT
                COUNT(*) as count,
                SUM(CASE WHEN is_question THEN 1 ELSE 0 END) as question_count
            FROM ml_questions
            WHERE chat_id = :chat_id
        """
        questions = self._execute_single(questions_query, {"chat_id": chat_id}) or {}

        total_with_text = total.get("count", 0)
        processed_count = processed.get("count", 0)

        return {
            "total_with_text": total_with_text,
            "processed": processed_count,
            "unprocessed": total_with_text - processed_count,
            "sentiment_analyzed": sentiment.get("count", 0),
            "toxicity_analyzed": toxicity.get("count", 0),
            "toxic_count": toxicity.get("toxic_count", 0) or 0,
            "topics_clustered": topics.get("count", 0),
            "ner_extracted": ner.get("count", 0),
            "unique_entities": ner.get("unique_entities", 0) or 0,
            "humor_analyzed": humor.get("count", 0),
            "humorous_count": humor.get("humorous_count", 0) or 0,
            "questions_analyzed": questions.get("count", 0),
            "question_count": questions.get("question_count", 0) or 0,
        }

    # =========================================
    # WRITE OPERATIONS
    # =========================================

    @function_trace(name="save_sentiment_results", group="Database")
    def save_sentiment_results(self, results: list[dict]) -> int:
        """
        Save sentiment analysis results.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - label: str
                - score_positive: float
                - score_neutral: float
                - score_negative: float
                - confidence: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        query = """
            INSERT INTO ml_sentiment (
                message_id, chat_id, label,
                score_positive, score_neutral, score_negative, confidence
            )
            VALUES (
                :message_id, :chat_id, :label,
                :score_positive, :score_neutral, :score_negative, :confidence
            )
            ON CONFLICT (message_id) DO UPDATE SET
                label = EXCLUDED.label,
                score_positive = EXCLUDED.score_positive,
                score_neutral = EXCLUDED.score_neutral,
                score_negative = EXCLUDED.score_negative,
                confidence = EXCLUDED.confidence,
                created_at = NOW()
        """
        return self._execute_batch(query, results)

    @function_trace(name="save_toxicity_results", group="Database")
    def save_toxicity_results(self, results: list[dict]) -> int:
        """
        Save toxicity detection results.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - is_toxic: bool
                - label: str
                - score: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        query = """
            INSERT INTO ml_toxicity (message_id, chat_id, is_toxic, label, score)
            VALUES (:message_id, :chat_id, :is_toxic, :label, :score)
            ON CONFLICT (message_id) DO UPDATE SET
                is_toxic = EXCLUDED.is_toxic,
                label = EXCLUDED.label,
                score = EXCLUDED.score,
                created_at = NOW()
        """
        return self._execute_batch(query, results)

    @function_trace(name="save_humor_results", group="Database")
    def save_humor_results(self, results: list[dict]) -> int:
        """
        Save humor detection results.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - is_humorous: bool
                - humor_type: str | None
                - score: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        query = """
            INSERT INTO ml_humor (message_id, chat_id, is_humorous, humor_type, score)
            VALUES (:message_id, :chat_id, :is_humorous, :humor_type, :score)
            ON CONFLICT (message_id) DO UPDATE SET
                is_humorous = EXCLUDED.is_humorous,
                humor_type = EXCLUDED.humor_type,
                score = EXCLUDED.score,
                created_at = NOW()
        """
        return self._execute_batch(query, results)

    @function_trace(name="save_questions_results", group="Database")
    def save_questions_results(self, results: list[dict]) -> int:
        """
        Save question detection results.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - is_question: bool
                - question_type: str | None
                - score: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        query = """
            INSERT INTO ml_questions (message_id, chat_id, is_question, question_type, score)
            VALUES (:message_id, :chat_id, :is_question, :question_type, :score)
            ON CONFLICT (message_id) DO UPDATE SET
                is_question = EXCLUDED.is_question,
                question_type = EXCLUDED.question_type,
                score = EXCLUDED.score,
                created_at = NOW()
        """
        return self._execute_batch(query, results)

    @function_trace(name="save_ner_results", group="Database")
    def save_ner_results(self, results: list[dict]) -> int:
        """
        Save NER (Named Entity Recognition) results.

        Multiple entities per message are allowed.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - entity_type: str (e.g., 'PERSON', 'ORG', 'LOC')
                - entity_text: str
                - start_pos: int | None
                - end_pos: int | None
                - confidence: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        query = """
            INSERT INTO ml_ner (
                message_id, chat_id, entity_type, entity_text,
                start_pos, end_pos, confidence
            )
            VALUES (
                :message_id, :chat_id, :entity_type, :entity_text,
                :start_pos, :end_pos, :confidence
            )
            ON CONFLICT (message_id, entity_type, entity_text, COALESCE(start_pos, -1))
            DO NOTHING
        """
        return self._execute_batch(query, results)

    @function_trace(name="save_topic_clusters", group="Database")
    def save_topic_clusters(
        self,
        chat_id: int,
        clusters: dict[int, list[str]],
        message_counts: dict[int, int],
    ) -> int:
        """
        Save topic cluster definitions.

        Args:
            chat_id: Chat ID
            clusters: Dict mapping topic_id to list of keywords
            message_counts: Dict mapping topic_id to message count

        Returns:
            Number of rows affected
        """
        if not clusters:
            return 0

        query = """
            INSERT INTO ml_topics (chat_id, topic_id, keywords, message_count)
            VALUES (:chat_id, :topic_id, :keywords, :message_count)
            ON CONFLICT (chat_id, topic_id) DO UPDATE SET
                keywords = EXCLUDED.keywords,
                message_count = EXCLUDED.message_count,
                updated_at = NOW()
        """

        params_list = [
            {
                "chat_id": chat_id,
                "topic_id": topic_id,
                "keywords": keywords,
                "message_count": message_counts.get(topic_id, 0),
            }
            for topic_id, keywords in clusters.items()
            if topic_id >= 0  # Skip outliers (topic_id = -1)
        ]

        return self._execute_batch(query, params_list)

    @function_trace(name="save_message_topics", group="Database")
    def save_message_topics(self, results: list[dict]) -> int:
        """
        Save message-to-topic assignments.

        Args:
            results: List of dicts with:
                - message_id: int
                - chat_id: int
                - topic_id: int
                - similarity: float

        Returns:
            Number of rows affected
        """
        if not results:
            return 0

        # Filter out outliers (topic_id = -1)
        valid_results = [r for r in results if r.get("topic_id", -1) >= 0]
        if not valid_results:
            return 0

        query = """
            INSERT INTO ml_message_topics (message_id, chat_id, topic_id, similarity)
            VALUES (:message_id, :chat_id, :topic_id, :similarity)
            ON CONFLICT (message_id) DO UPDATE SET
                topic_id = EXCLUDED.topic_id,
                similarity = EXCLUDED.similarity,
                created_at = NOW()
        """
        return self._execute_batch(query, valid_results)

    @function_trace(name="mark_processed", group="Database")
    def mark_processed(
        self,
        message_ids: list[int],
        chat_id: int,
        processor_version: str,
    ) -> int:
        """
        Mark messages as processed.

        Args:
            message_ids: List of message IDs that were processed
            chat_id: Chat ID
            processor_version: Version string (e.g., 'v2.0.0')

        Returns:
            Number of rows affected
        """
        if not message_ids:
            return 0

        query = """
            INSERT INTO ml_processing_state (message_id, chat_id, processor_version)
            VALUES (:message_id, :chat_id, :processor_version)
            ON CONFLICT (message_id) DO UPDATE SET
                processor_version = EXCLUDED.processor_version,
                processed_at = NOW()
        """

        params_list = [
            {
                "message_id": msg_id,
                "chat_id": chat_id,
                "processor_version": processor_version,
            }
            for msg_id in message_ids
        ]

        return self._execute_batch(query, params_list)

    # =========================================
    # UTILITY METHODS
    # =========================================

    def get_existing_embeddings_count(self, chat_id: int) -> int:
        """Get count of messages that already have embeddings in Qdrant."""
        # This is informational only - actual embedding check is done via Qdrant
        query = """
            SELECT COUNT(*) as count
            FROM ml_processing_state
            WHERE chat_id = :chat_id
        """
        result = self._execute_single(query, {"chat_id": chat_id}) or {}
        return result.get("count", 0)

    def refresh_user_profiles(self, chat_id: int | None = None) -> None:
        """
        Refresh ML user profiles from aggregated results.

        Calls the refresh_ml_user_profiles() stored function.
        """
        if chat_id is not None:
            query = "SELECT refresh_ml_user_profiles(:chat_id)"
            params = {"chat_id": chat_id}
        else:
            query = "SELECT refresh_ml_user_profiles(NULL)"
            params = {}

        with self._engine.connect() as conn:
            conn.execute(text(query), params)
            conn.commit()

        logger.info(f"Refreshed ML user profiles for chat_id={chat_id or 'all'}")
