"""FastAPI routes for card image generator."""

import logging
from datetime import date
from typing import Annotated, Any, Optional

from fastapi import APIRouter, Depends, HTTPException, Query, Request, Security
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

from ..generator import CardGenerator
from ..instrumentation import add_custom_attributes, notice_error
from ..renderer.position_loader import PositionLoader

logger = logging.getLogger(__name__)
router = APIRouter()


# Security
api_key_header = APIKeyHeader(name="Authorization", auto_error=False)


def get_generator(request: Request) -> CardGenerator:
    """Get generator from app state."""
    generator = getattr(request.app.state, "generator", None)
    if generator is None:
        raise HTTPException(status_code=503, detail="Generator not initialized")
    return generator


def get_api_keys(request: Request) -> dict[str, str]:
    """Get API keys from app state."""
    return getattr(request.app.state, "api_keys", {})


def get_default_theme(request: Request) -> str:
    """Get default theme from app state."""
    return getattr(request.app.state, "default_theme", "gaming")


def get_position_loader(request: Request) -> Optional[PositionLoader]:
    """Get position loader from app state."""
    return getattr(request.app.state, "position_loader", None)


async def verify_api_key(
    request: Request,
    authorization: str = Security(api_key_header),
) -> str:
    """Verify API key from Authorization header."""
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing authorization header")

    # Expected format: "Bearer <key>"
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(status_code=401, detail="Invalid authorization format")

    token = parts[1]

    # Verify API key
    api_keys = get_api_keys(request)
    if token in api_keys.values():
        return token

    raise HTTPException(status_code=401, detail="Invalid API key")


# Request/Response models
class RenderRequest(BaseModel):
    """Request to render card images."""

    chat_id: int = Field(..., description="Chat ID")
    week_start: str = Field(..., description="Week start date (YYYY-MM-DD)")
    user_ids: list[int] | None = Field(
        None, description="Optional list of specific user IDs"
    )
    theme: str | None = Field(None, description="Template theme name (uses DEFAULT_CARD_THEME if not specified)")
    card_type: str = Field("regular", description="Card type: 'regular' or 'compact'")
    force_regenerate: bool = Field(False, description="Regenerate even if exists")


class RenderResultItem(BaseModel):
    """Single card render result."""

    user_id: int
    status: str
    image_id: int | None = None
    storage_path: str | None = None
    error: str | None = None


class RenderResponse(BaseModel):
    """Response from render endpoint."""

    generated: int
    skipped: int
    failed: int
    results: list[RenderResultItem]


class ImageInfo(BaseModel):
    """Card image information."""

    id: int
    user_id: int
    chat_id: int
    week_start: str
    storage_path: str
    theme: str
    generated_at: str
    first_name: str | None = None
    last_name: str | None = None
    username: str | None = None
    placeholder_positions: Optional[dict[str, Any]] = None


class ImageListResponse(BaseModel):
    """Response with list of images."""

    images: list[ImageInfo]


class ImageUrlResponse(BaseModel):
    """Response with presigned URL."""

    image_id: int
    url: str
    expires_in: int


class WeekListResponse(BaseModel):
    """Response with list of available weeks."""

    weeks: list[str]


@router.get("/api/v1/weeks", response_model=WeekListResponse)
async def get_available_weeks(
    chat_id: Annotated[int, Query(description="Chat ID")],
    generator: CardGenerator = Depends(get_generator),
):
    """
    Get list of weeks with generated card images for a chat.

    Returns list of week_start dates (YYYY-MM-DD) in descending order.
    Public endpoint for Mini App gallery.
    """
    weeks = generator.queries.get_available_weeks(chat_id)
    return WeekListResponse(weeks=[w.isoformat() for w in weeks])


@router.post("/api/v1/render", response_model=RenderResponse)
async def render_cards(
    render_request: RenderRequest,
    request: Request,
    generator: CardGenerator = Depends(get_generator),
    api_key: str = Depends(verify_api_key),
):
    """
    Trigger card image generation for a chat/week.

    Renders card images for all users (or specified users) in a chat
    for a given week, stores them in object storage, and saves
    references in the database.
    """
    try:
        week_start = date.fromisoformat(render_request.week_start)
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="Invalid week_start format. Use YYYY-MM-DD",
        )

    # Use default theme from config if not specified
    theme = render_request.theme or get_default_theme(request)

    # Validate card_type
    if render_request.card_type not in ("regular", "compact"):
        raise HTTPException(
            status_code=400,
            detail="card_type must be 'regular' or 'compact'",
        )

    # Add custom attributes for observability
    add_custom_attributes({
        "card.chat_id": render_request.chat_id,
        "card.week_start": render_request.week_start,
        "card.theme": theme,
        "card.card_type": render_request.card_type,
        "card.user_count": len(render_request.user_ids) if render_request.user_ids else 0,
        "card.force_regenerate": render_request.force_regenerate,
    })

    try:
        result = await generator.render_cards(
            chat_id=render_request.chat_id,
            week_start=week_start,
            user_ids=render_request.user_ids,
            theme=theme,
            card_type=render_request.card_type,
            force_regenerate=render_request.force_regenerate,
        )

        # Add result metrics as attributes
        add_custom_attributes({
            "card.result.generated": result.generated,
            "card.result.skipped": result.skipped,
            "card.result.failed": result.failed,
        })

        return RenderResponse(
            generated=result.generated,
            skipped=result.skipped,
            failed=result.failed,
            results=[
                RenderResultItem(
                    user_id=r.user_id,
                    status=r.status,
                    image_id=r.image_id,
                    storage_path=r.storage_path,
                    error=r.error,
                )
                for r in result.results
            ],
        )
    except Exception as e:
        notice_error(e)
        raise


@router.get("/api/v1/images", response_model=ImageListResponse)
async def get_images(
    chat_id: Annotated[int, Query(description="Chat ID")],
    week_start: Annotated[str | None, Query(description="Week start (YYYY-MM-DD)")] = None,
    user_id: Annotated[int | None, Query(description="Filter by user ID")] = None,
    theme: Annotated[str | None, Query(description="Filter by theme")] = None,
    include_positions: Annotated[bool, Query(description="Include placeholder position metadata for compact cards")] = True,
    request: Request = None,
    generator: CardGenerator = Depends(get_generator),
    position_loader: Optional[PositionLoader] = Depends(get_position_loader),
):
    """
    Get card images for a chat/week.

    Returns list of image references with metadata. For compact cards,
    optionally includes placeholder position coordinates for React overlays.

    Args:
        chat_id: Chat ID
        week_start: Week start date (YYYY-MM-DD), defaults to latest
        user_id: Optional filter by user ID
        theme: Optional filter by theme (e.g., "neon_arcade_compact")
        include_positions: Include placeholder positions (default: True)

    Returns:
        ImageListResponse with images and optional placeholder_positions field
    """
    # Get latest week if not specified
    if week_start is None:
        latest = generator.queries.get_latest_week_for_chat(chat_id)
        if latest is None:
            return ImageListResponse(images=[])
        week_start_date = latest
    else:
        try:
            week_start_date = date.fromisoformat(week_start)
        except ValueError:
            raise HTTPException(
                status_code=400,
                detail="Invalid week_start format. Use YYYY-MM-DD",
            )

    images = generator.repository.get_images_for_chat_week(
        chat_id=chat_id,
        week_start=week_start_date,
        user_id=user_id,
        theme=theme,
    )

    # Build image list with optional position metadata
    image_list = []
    for img in images:
        image_info = ImageInfo(
            id=img["id"],
            user_id=img["user_id"],
            chat_id=img["chat_id"],
            week_start=img["week_start"].isoformat(),
            storage_path=img["storage_path"],
            theme=img["theme"],
            generated_at=img["generated_at"].isoformat(),
            first_name=img.get("first_name"),
            last_name=img.get("last_name"),
            username=img.get("username"),
        )

        # Attach position metadata if requested and theme is compact
        if include_positions and position_loader and "_compact" in img["theme"]:
            base_theme = img["theme"].replace("_compact", "")
            # Load positions for this theme (without tier-specific overrides for now)
            # Tier-specific overrides would require additional database queries
            positions = position_loader.load_positions(base_theme)
            if positions:
                image_info.placeholder_positions = positions

        image_list.append(image_info)

    return ImageListResponse(images=image_list)


@router.get("/api/v1/image/{image_id}", response_model=ImageUrlResponse)
async def get_image_url(
    image_id: int,
    expires: Annotated[int, Query(ge=60, le=86400)] = 3600,
    generator: CardGenerator = Depends(get_generator),
):
    """
    Get presigned URL for a specific card image.

    The URL expires after the specified duration (default 1 hour).
    Public endpoint for Mini App gallery.
    """
    url = generator.get_image_url(image_id, expires_seconds=expires)
    if url is None:
        raise HTTPException(status_code=404, detail="Image not found")

    return ImageUrlResponse(
        image_id=image_id,
        url=url,
        expires_in=expires,
    )


@router.get("/health")
async def health_check():
    """Health check endpoint (unauthenticated)."""
    return {"status": "ok"}
