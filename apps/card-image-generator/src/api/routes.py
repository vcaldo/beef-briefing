"""FastAPI routes for card image generator."""

from datetime import date
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Security
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

router = APIRouter()

# Global references set by main.py
_generator = None
_api_keys: dict[str, str] = {}


def set_generator(generator):
    """Set the global generator instance."""
    global _generator
    _generator = generator


def set_api_keys(keys: dict[str, str]):
    """Set valid API keys."""
    global _api_keys
    _api_keys = keys


# Security
api_key_header = APIKeyHeader(name="Authorization", auto_error=False)


async def verify_api_key(
    authorization: str = Security(api_key_header),
) -> str:
    """Verify API key from Authorization header."""
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing authorization header")

    # Expected format: "Bearer <key>"
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(status_code=401, detail="Invalid authorization format")

    key = parts[1]
    if key not in _api_keys.values():
        raise HTTPException(status_code=401, detail="Invalid API key")

    return key


# Request/Response models
class RenderRequest(BaseModel):
    """Request to render card images."""

    chat_id: int = Field(..., description="Chat ID")
    week_start: str = Field(..., description="Week start date (YYYY-MM-DD)")
    user_ids: list[int] | None = Field(
        None, description="Optional list of specific user IDs"
    )
    theme: str = Field("gaming", description="Template theme name")
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
    api_key: str = Depends(verify_api_key),
):
    """
    Get list of weeks with generated card images for a chat.

    Returns list of week_start dates (YYYY-MM-DD) in descending order.
    """
    if _generator is None:
        raise HTTPException(status_code=503, detail="Generator not initialized")

    weeks = _generator.queries.get_available_weeks(chat_id)
    return WeekListResponse(weeks=[w.isoformat() for w in weeks])


@router.post("/api/v1/render", response_model=RenderResponse)
async def render_cards(
    request: RenderRequest,
    api_key: str = Depends(verify_api_key),
):
    """
    Trigger card image generation for a chat/week.

    Renders card images for all users (or specified users) in a chat
    for a given week, stores them in object storage, and saves
    references in the database.
    """
    if _generator is None:
        raise HTTPException(status_code=503, detail="Generator not initialized")

    try:
        week_start = date.fromisoformat(request.week_start)
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="Invalid week_start format. Use YYYY-MM-DD",
        )

    result = await _generator.render_cards(
        chat_id=request.chat_id,
        week_start=week_start,
        user_ids=request.user_ids,
        theme=request.theme,
        force_regenerate=request.force_regenerate,
    )

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


@router.get("/api/v1/images", response_model=ImageListResponse)
async def get_images(
    chat_id: Annotated[int, Query(description="Chat ID")],
    week_start: Annotated[str | None, Query(description="Week start (YYYY-MM-DD)")] = None,
    user_id: Annotated[int | None, Query(description="Filter by user ID")] = None,
    theme: Annotated[str | None, Query(description="Filter by theme")] = None,
    api_key: str = Depends(verify_api_key),
):
    """
    Get card images for a chat/week.

    Returns list of image references with metadata.
    """
    if _generator is None:
        raise HTTPException(status_code=503, detail="Generator not initialized")

    # Get latest week if not specified
    if week_start is None:
        latest = _generator.queries.get_latest_week_for_chat(chat_id)
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

    images = _generator.repository.get_images_for_chat_week(
        chat_id=chat_id,
        week_start=week_start_date,
        user_id=user_id,
        theme=theme,
    )

    return ImageListResponse(
        images=[
            ImageInfo(
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
            for img in images
        ]
    )


@router.get("/api/v1/image/{image_id}", response_model=ImageUrlResponse)
async def get_image_url(
    image_id: int,
    expires: Annotated[int, Query(ge=60, le=86400)] = 3600,
    api_key: str = Depends(verify_api_key),
):
    """
    Get presigned URL for a specific card image.

    The URL expires after the specified duration (default 1 hour).
    """
    if _generator is None:
        raise HTTPException(status_code=503, detail="Generator not initialized")

    url = _generator.get_image_url(image_id, expires_seconds=expires)
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
