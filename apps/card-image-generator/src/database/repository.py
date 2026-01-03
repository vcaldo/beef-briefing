"""Repository for saving card image references."""

from datetime import date
from typing import Any

from sqlalchemy import text
from sqlalchemy.engine import Engine

from .utils import row_to_dict, rows_to_dicts


class CardImageRepository:
    """Write operations for card images."""

    def __init__(self, engine: Engine):
        self.engine = engine

    def upsert_card_image(
        self,
        card_id: int,
        user_id: int,
        chat_id: int,
        week_start: date,
        storage_path: str,
        file_hash: str,
        file_size: int,
        width: int,
        height: int,
        theme: str,
        template_version: int,
        card_data_version: int,
    ) -> int:
        """
        Insert or update a card image reference.

        Returns the image ID.
        """
        query = text("""
            INSERT INTO ml_user_card_images (
                card_id, user_id, chat_id, week_start,
                storage_path, file_hash, file_size,
                width, height, theme, template_version, card_data_version
            ) VALUES (
                :card_id, :user_id, :chat_id, :week_start,
                :storage_path, :file_hash, :file_size,
                :width, :height, :theme, :template_version, :card_data_version
            )
            ON CONFLICT (card_id, theme) DO UPDATE SET
                storage_path = EXCLUDED.storage_path,
                file_hash = EXCLUDED.file_hash,
                file_size = EXCLUDED.file_size,
                width = EXCLUDED.width,
                height = EXCLUDED.height,
                template_version = EXCLUDED.template_version,
                card_data_version = EXCLUDED.card_data_version,
                generated_at = NOW()
            RETURNING id
        """)

        with self.engine.begin() as conn:
            result = conn.execute(
                query,
                {
                    "card_id": card_id,
                    "user_id": user_id,
                    "chat_id": chat_id,
                    "week_start": week_start,
                    "storage_path": storage_path,
                    "file_hash": file_hash,
                    "file_size": file_size,
                    "width": width,
                    "height": height,
                    "theme": theme,
                    "template_version": template_version,
                    "card_data_version": card_data_version,
                },
            )
            return result.scalar()

    def get_image_by_id(self, image_id: int) -> dict[str, Any] | None:
        """Get card image by ID."""
        query = text("""
            SELECT
                id, card_id, user_id, chat_id, week_start,
                storage_path, file_hash, file_size, mime_type,
                width, height, theme, template_version,
                card_data_version, generated_at
            FROM ml_user_card_images
            WHERE id = :image_id
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"image_id": image_id}).fetchone()
            return row_to_dict(result) if result else None

    def get_images_for_chat_week(
        self,
        chat_id: int,
        week_start: date,
        user_id: int | None = None,
        theme: str | None = None,
    ) -> list[dict[str, Any]]:
        """Get all card images for a chat/week."""
        query = text("""
            SELECT
                i.id, i.card_id, i.user_id, i.chat_id, i.week_start,
                i.storage_path, i.file_hash, i.file_size, i.mime_type,
                i.width, i.height, i.theme, i.template_version,
                i.card_data_version, i.generated_at,
                u.first_name, u.last_name, u.username
            FROM ml_user_card_images i
            JOIN users u ON i.user_id = u.id
            JOIN ml_user_cards c ON i.card_id = c.id
            WHERE i.chat_id = :chat_id
              AND i.week_start = :week_start
              AND (:user_id IS NULL OR i.user_id = :user_id)
              AND (:theme IS NULL OR i.theme = :theme)
            ORDER BY (c.stats->'overall'->>'score')::float DESC NULLS LAST
        """)

        with self.engine.connect() as conn:
            result = conn.execute(
                query,
                {
                    "chat_id": chat_id,
                    "week_start": week_start,
                    "user_id": user_id,
                    "theme": theme,
                },
            )
            return rows_to_dicts(result)
