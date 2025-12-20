"""Database queries for reading card data."""

from datetime import date
from typing import Any

from sqlalchemy import text
from sqlalchemy.engine import Engine


class CardQueries:
    """Read-only queries for card data from ml_user_cards."""

    def __init__(self, engine: Engine):
        self.engine = engine

    def get_cards_for_chat_week(
        self,
        chat_id: int,
        week_start: date,
        user_ids: list[int] | None = None,
    ) -> list[dict[str, Any]]:
        """
        Get card data for a chat and week.

        Returns list of dicts with:
        - card_id, user_id, chat_id, week_start, week_end
        - stats (JSON), trends (JSON)
        - card_version, messages_analyzed
        - user info (first_name, last_name, username)
        - profile_photo_path (if available)
        """
        query = text("""
            SELECT
                c.id AS card_id,
                c.user_id,
                c.chat_id,
                c.week_start,
                c.week_end,
                c.stats,
                c.trends,
                c.card_version,
                c.messages_analyzed,
                u.first_name,
                u.last_name,
                u.username,
                upp.minio_object_key AS profile_photo_path
            FROM ml_user_cards c
            JOIN users u ON c.user_id = u.id
            LEFT JOIN user_profile_photos upp ON u.id = upp.user_id
                AND upp.id = (
                    SELECT id FROM user_profile_photos
                    WHERE user_id = u.id
                    ORDER BY created_at DESC
                    LIMIT 1
                )
            WHERE c.chat_id = :chat_id
              AND c.week_start = :week_start
              AND (:user_ids IS NULL OR c.user_id = ANY(:user_ids))
            ORDER BY (c.stats->'activity'->>'messages')::numeric DESC NULLS LAST
        """)

        with self.engine.connect() as conn:
            result = conn.execute(
                query,
                {
                    "chat_id": chat_id,
                    "week_start": week_start,
                    "user_ids": user_ids,
                },
            )
            return [dict(row._mapping) for row in result]

    def get_latest_week_for_chat(self, chat_id: int) -> date | None:
        """Get the most recent week_start for a chat."""
        query = text("""
            SELECT week_start
            FROM ml_user_cards
            WHERE chat_id = :chat_id
            ORDER BY week_start DESC
            LIMIT 1
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"chat_id": chat_id}).scalar()
            return result

    def get_available_weeks(self, chat_id: int) -> list[date]:
        """
        Get all distinct weeks with generated card images for a chat.

        Returns list of week_start dates in descending order (most recent first).
        """
        query = text("""
            SELECT DISTINCT week_start
            FROM ml_user_card_images
            WHERE chat_id = :chat_id
            ORDER BY week_start DESC
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"chat_id": chat_id})
            return [row.week_start for row in result]

    def get_existing_images(
        self,
        chat_id: int,
        week_start: date,
        theme: str,
    ) -> dict[int, dict[str, Any]]:
        """
        Get existing card images for a chat/week/theme.

        Returns dict mapping user_id to image info.
        """
        query = text("""
            SELECT
                user_id,
                id AS image_id,
                storage_path,
                card_data_version,
                file_hash
            FROM ml_user_card_images
            WHERE chat_id = :chat_id
              AND week_start = :week_start
              AND theme = :theme
        """)

        with self.engine.connect() as conn:
            result = conn.execute(
                query,
                {"chat_id": chat_id, "week_start": week_start, "theme": theme},
            )
            return {row.user_id: dict(row._mapping) for row in result}

    def get_category_rankings(
        self,
        chat_id: int,
        week_start: date,
    ) -> dict[str, dict[int, int]]:
        """
        Get top 3 rankings per category for a chat/week.

        Returns dict mapping category -> {user_id: rank} (only top 3 users)
        Example: {"mood": {123: 1, 456: 2, 789: 3}, "comedy": {...}, ...}
        """
        query = text("""
            WITH category_ranks AS (
                SELECT
                    user_id,
                    ROW_NUMBER() OVER (ORDER BY (stats->'mood'->>'score')::float DESC NULLS LAST) AS mood_rank,
                    ROW_NUMBER() OVER (ORDER BY (stats->'comedy'->>'score')::float DESC NULLS LAST) AS comedy_rank,
                    ROW_NUMBER() OVER (ORDER BY (stats->'volatility'->>'score')::float DESC NULLS LAST) AS volatility_rank,
                    ROW_NUMBER() OVER (ORDER BY (stats->'toxicity'->>'pct')::float DESC NULLS LAST) AS toxicity_rank,
                    ROW_NUMBER() OVER (ORDER BY (stats->'activity'->>'messages')::int DESC NULLS LAST) AS activity_rank,
                    ROW_NUMBER() OVER (ORDER BY (stats->>'reactions_received')::int DESC NULLS LAST) AS reactions_rank
                FROM ml_user_cards
                WHERE chat_id = :chat_id
                  AND week_start = :week_start
            )
            SELECT user_id, mood_rank, comedy_rank, volatility_rank, toxicity_rank, activity_rank, reactions_rank
            FROM category_ranks
            WHERE mood_rank <= 3
               OR comedy_rank <= 3
               OR volatility_rank <= 3
               OR toxicity_rank <= 3
               OR activity_rank <= 3
               OR reactions_rank <= 3
        """)

        with self.engine.connect() as conn:
            result = conn.execute(
                query,
                {"chat_id": chat_id, "week_start": week_start},
            )

            rankings: dict[str, dict[int, int]] = {
                "mood": {},
                "comedy": {},
                "volatility": {},
                "toxicity": {},
                "activity": {},
                "reactions": {},
            }

            for row in result:
                user_id = row.user_id
                if row.mood_rank <= 3:
                    rankings["mood"][user_id] = row.mood_rank
                if row.comedy_rank <= 3:
                    rankings["comedy"][user_id] = row.comedy_rank
                if row.volatility_rank <= 3:
                    rankings["volatility"][user_id] = row.volatility_rank
                if row.toxicity_rank <= 3:
                    rankings["toxicity"][user_id] = row.toxicity_rank
                if row.activity_rank <= 3:
                    rankings["activity"][user_id] = row.activity_rank
                if row.reactions_rank <= 3:
                    rankings["reactions"][user_id] = row.reactions_rank

            return rankings
