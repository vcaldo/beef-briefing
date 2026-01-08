"""Main card generation orchestrator."""

import logging
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from typing import Any, Callable

from minio.error import S3Error
from sqlalchemy import create_engine
from sqlalchemy.engine import Engine

from .database import CardQueries, CardImageRepository
from .instrumentation import (
    add_custom_attributes,
    function_trace_async,
    record_custom_metrics,
)
from .renderer import PlaywrightRenderer, TemplateLoader
from .storage import CardStorageClient

logger = logging.getLogger(__name__)


@dataclass
class RenderResult:
    """Result of rendering a single card."""

    user_id: int
    status: str  # "generated", "skipped", "failed"
    image_id: int | None = None
    storage_path: str | None = None
    error: str | None = None


@dataclass
class BatchRenderResult:
    """Result of rendering a batch of cards."""

    generated: int
    skipped: int
    failed: int
    results: list[RenderResult]


class CardGenerator:
    """Orchestrates card image generation."""

    def __init__(
        self,
        engine: Engine,
        storage: CardStorageClient,
        templates_dir: str,
        card_width: int = 400,
        card_height: int = 600,
        card_scale: int = 2,
        tier_class_fn: Callable[[str], str] | None = None,
    ):
        self.engine = engine
        self.storage = storage
        self.queries = CardQueries(engine)
        self.repository = CardImageRepository(engine)
        self.template_loader = TemplateLoader(templates_dir, tier_class_fn=tier_class_fn)
        self.renderer = PlaywrightRenderer(
            width=card_width,
            height=card_height,
            scale=card_scale,
        )
        self.card_width = card_width
        self.card_height = card_height
        self.templates_dir = Path(templates_dir)

    async def start(self) -> None:
        """Initialize renderer."""
        await self.renderer.start()

    async def stop(self) -> None:
        """Cleanup resources."""
        await self.renderer.stop()

    @function_trace_async(name="render_cards", group="CardGenerator")
    async def render_cards(
        self,
        chat_id: int,
        week_start: date | str,
        user_ids: list[int] | None = None,
        theme: str = "gaming",
        force_regenerate: bool = False,
        template_version: int = 1,
    ) -> BatchRenderResult:
        """
        Render card images for a chat/week.

        Args:
            chat_id: Chat ID
            week_start: Week start date
            user_ids: Optional list of specific user IDs to render
            theme: Template theme name
            force_regenerate: If True, regenerate even if images exist
            template_version: Template version number

        Returns:
            BatchRenderResult with counts and individual results
        """
        if isinstance(week_start, str):
            week_start = date.fromisoformat(week_start)

        # Validate theme exists
        if not self.template_loader.theme_exists(theme):
            raise ValueError(f"Theme not found: {theme}")

        # Get card data from database
        cards = self.queries.get_cards_for_chat_week(chat_id, week_start, user_ids)

        # Get category rankings for top 3 badges
        category_rankings = self.queries.get_category_rankings(chat_id, week_start)

        if not cards:
            logger.warning(f"No cards found for chat {chat_id} week {week_start}")
            return BatchRenderResult(
                generated=0,
                skipped=0,
                failed=0,
                results=[],
            )

        # Get existing images (for skip logic)
        existing_images = {}
        if not force_regenerate:
            existing_images = self.queries.get_existing_images(
                chat_id, week_start, theme
            )

        results = []
        generated = 0
        skipped = 0
        failed = 0

        # Compute base URL for template assets
        base_url = f"file://{self.templates_dir}/themes/{theme}/"

        for rank, card_data in enumerate(cards, 1):
            user_id = card_data["user_id"]
            card_version = card_data["card_version"]

            # Check if we should skip
            if not force_regenerate and user_id in existing_images:
                existing = existing_images[user_id]
                if existing["card_data_version"] >= card_version:
                    logger.debug(f"Skipping user {user_id}: image up to date")
                    results.append(
                        RenderResult(
                            user_id=user_id,
                            status="skipped",
                            image_id=existing["image_id"],
                            storage_path=existing["storage_path"],
                        )
                    )
                    skipped += 1
                    continue

            try:
                result = await self._render_single_card(
                    card_data=card_data,
                    chat_id=chat_id,
                    week_start=week_start,
                    theme=theme,
                    template_version=template_version,
                    rank=rank,
                    base_url=base_url,
                    category_rankings=category_rankings,
                )
                results.append(result)
                if result.status == "generated":
                    generated += 1
                else:
                    failed += 1

            except (ValueError, KeyError, OSError) as e:
                logger.exception(f"Failed to render card for user {user_id}")
                results.append(
                    RenderResult(
                        user_id=user_id,
                        status="failed",
                        error=str(e),
                    )
                )
                failed += 1

        # Record custom metrics
        record_custom_metrics([
            ("Custom/Cards/Generated", generated),
            ("Custom/Cards/Skipped", skipped),
            ("Custom/Cards/Failed", failed),
            ("Custom/Cards/Total", len(results)),
        ])

        return BatchRenderResult(
            generated=generated,
            skipped=skipped,
            failed=failed,
            results=results,
        )

    @function_trace_async(name="render_single_card", group="CardGenerator")
    async def _render_single_card(
        self,
        card_data: dict[str, Any],
        chat_id: int,
        week_start: date,
        theme: str,
        template_version: int,
        rank: int,
        base_url: str,
        category_rankings: dict[str, dict[int, int]] | None = None,
    ) -> RenderResult:
        """Render a single card image."""
        user_id = card_data["user_id"]
        card_id = card_data["card_id"]
        card_version = card_data["card_version"]

        # Add per-card attributes for detailed tracing
        add_custom_attributes({
            "card.current.user_id": user_id,
            "card.current.rank": rank,
            "card.current.card_id": card_id,
        })

        # Convert profile photo path to presigned URL for Playwright
        profile_photo_path = card_data.get("profile_photo_path")
        if profile_photo_path:
            try:
                card_data["profile_photo_path"] = self.storage.get_presigned_url(
                    profile_photo_path,
                    expires_seconds=300,  # 5 minutes is enough for rendering
                )
            except S3Error as e:
                logger.warning(f"Failed to get presigned URL for profile photo: {e}")
                card_data["profile_photo_path"] = None

        # Transform data to template context
        context = self.template_loader.transform_card_data(
            card_data, theme=theme, rank=rank, category_rankings=category_rankings
        )

        # Render HTML
        html_content = self.template_loader.render(theme, context)

        # Render to PNG
        image_data = await self.renderer.render_html(html_content, base_url)

        # Upload to storage
        storage_path, file_hash, file_size = self.storage.upload_card_image(
            chat_id=chat_id,
            week_start=week_start.isoformat(),
            user_id=user_id,
            image_data=image_data,
            theme=theme,
        )

        # Save reference to database
        image_id = self.repository.upsert_card_image(
            card_id=card_id,
            user_id=user_id,
            chat_id=chat_id,
            week_start=week_start,
            storage_path=storage_path,
            file_hash=file_hash,
            file_size=file_size,
            width=self.card_width * self.renderer.scale,
            height=self.card_height * self.renderer.scale,
            theme=theme,
            template_version=template_version,
            card_data_version=card_version,
        )

        logger.info(
            f"Generated card image for user {user_id}: {storage_path} "
            f"({file_size} bytes)"
        )

        return RenderResult(
            user_id=user_id,
            status="generated",
            image_id=image_id,
            storage_path=storage_path,
        )

    def get_image_url(
        self,
        image_id: int,
        expires_seconds: int = 3600,
    ) -> str | None:
        """Get presigned URL for a card image."""
        image = self.repository.get_image_by_id(image_id)
        if not image:
            return None

        return self.storage.get_presigned_url(
            image["storage_path"],
            expires_seconds=expires_seconds,
        )

    async def __aenter__(self):
        await self.start()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.stop()
