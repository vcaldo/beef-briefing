"""FastAPI routes for card image generator."""

import logging
from datetime import date
from typing import Annotated

import jwt
from fastapi import APIRouter, Depends, HTTPException, Query, Request, Security
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

from ..database.mini_app_queries import MiniAppQueries, get_period_dates
from ..generator import CardGenerator
from .mini_app_auth import (
    InitDataValidation,
    create_jwt_token,
    validate_init_data,
    verify_jwt_token,
)

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


def get_jwt_secret(request: Request) -> str:
    """Get JWT secret from app state."""
    return getattr(request.app.state, "jwt_secret_key", "")


def get_bot_token(request: Request) -> str:
    """Get Telegram bot token from app state."""
    return getattr(request.app.state, "telegram_bot_token", "")


def get_mini_app_queries(request: Request) -> MiniAppQueries:
    """Get MiniAppQueries from app state."""
    queries = getattr(request.app.state, "mini_app_queries", None)
    if queries is None:
        raise HTTPException(status_code=503, detail="Mini app queries not initialized")
    return queries


async def verify_api_key_or_jwt(
    request: Request,
    authorization: str = Security(api_key_header),
) -> dict:
    """
    Verify API key or JWT token from Authorization header.

    Returns:
        dict with auth info:
        - For API key: {"type": "api_key", "key": "..."}
        - For JWT: {"type": "jwt", "user_id": ..., "chat_id": ...}
    """
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing authorization header")

    # Expected format: "Bearer <key>"
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(status_code=401, detail="Invalid authorization format")

    token = parts[1]

    # First try API key authentication
    api_keys = get_api_keys(request)
    if token in api_keys.values():
        return {"type": "api_key", "key": token}

    # Then try JWT authentication
    jwt_secret = get_jwt_secret(request)
    if jwt_secret:
        try:
            payload = verify_jwt_token(token, jwt_secret)
            if payload.get("type") == "mini_app":
                return {
                    "type": "jwt",
                    "user_id": payload.get("user_id"),
                    "chat_id": payload.get("chat_id"),
                }
        except jwt.InvalidTokenError:
            pass

    raise HTTPException(status_code=401, detail="Invalid API key or token")


# Keep the old function for backward compatibility
async def verify_api_key(
    request: Request,
    authorization: str = Security(api_key_header),
) -> str:
    """Verify API key from Authorization header (legacy, API key only)."""
    auth_info = await verify_api_key_or_jwt(request, authorization)
    if auth_info["type"] == "api_key":
        return auth_info["key"]
    # JWT is also valid for existing endpoints
    return f"jwt:{auth_info.get('user_id', 'unknown')}"


# Request/Response models
class RenderRequest(BaseModel):
    """Request to render card images."""

    chat_id: int = Field(..., description="Chat ID")
    week_start: str = Field(..., description="Week start date (YYYY-MM-DD)")
    user_ids: list[int] | None = Field(
        None, description="Optional list of specific user IDs"
    )
    theme: str | None = Field(None, description="Template theme name (uses DEFAULT_CARD_THEME if not specified)")
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
    generator: CardGenerator = Depends(get_generator),
    api_key: str = Depends(verify_api_key),
):
    """
    Get list of weeks with generated card images for a chat.

    Returns list of week_start dates (YYYY-MM-DD) in descending order.
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

    result = await generator.render_cards(
        chat_id=render_request.chat_id,
        week_start=week_start,
        user_ids=render_request.user_ids,
        theme=theme,
        force_regenerate=render_request.force_regenerate,
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
    generator: CardGenerator = Depends(get_generator),
    api_key: str = Depends(verify_api_key),
):
    """
    Get card images for a chat/week.

    Returns list of image references with metadata.
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
    generator: CardGenerator = Depends(get_generator),
    api_key: str = Depends(verify_api_key),
):
    """
    Get presigned URL for a specific card image.

    The URL expires after the specified duration (default 1 hour).
    """
    url = generator.get_image_url(image_id, expires_seconds=expires)
    if url is None:
        raise HTTPException(status_code=404, detail="Image not found")

    return ImageUrlResponse(
        image_id=image_id,
        url=url,
        expires_in=expires,
    )


# Mini App Authentication Models
class MiniAppAuthRequest(BaseModel):
    """Request to authenticate Mini App user."""

    init_data: str = Field(..., description="Telegram init data string")


class MiniAppAuthResponse(BaseModel):
    """Response with JWT token for Mini App authentication."""

    token: str
    user_id: int
    chat_id: int | None
    first_name: str
    username: str | None


@router.post("/api/v1/mini-app/auth", response_model=MiniAppAuthResponse)
async def authenticate_mini_app(
    request: Request,
    auth_request: MiniAppAuthRequest,
):
    """
    Authenticate Telegram Mini App user via init data validation.

    Validates the init data signature using the bot token and returns
    a JWT token for subsequent authenticated API calls.

    This endpoint is unauthenticated - it validates the Telegram-signed init data.
    """
    bot_token = get_bot_token(request)
    if not bot_token:
        logger.error("TELEGRAM_BOT_TOKEN not configured")
        raise HTTPException(
            status_code=503,
            detail="Mini App authentication not configured",
        )

    jwt_secret = get_jwt_secret(request)
    if not jwt_secret:
        logger.error("JWT_SECRET_KEY not configured")
        raise HTTPException(
            status_code=503,
            detail="Mini App authentication not configured",
        )

    try:
        validated = validate_init_data(auth_request.init_data, bot_token)
        token = create_jwt_token(validated, jwt_secret)

        logger.info(
            f"Mini App auth successful for user {validated.user_id}, chat {validated.chat_id}"
        )

        return MiniAppAuthResponse(
            token=token,
            user_id=validated.user_id,
            chat_id=validated.chat_id,
            first_name=validated.first_name,
            username=validated.username,
        )
    except ValueError as e:
        logger.warning(f"Mini App auth failed: {e}")
        raise HTTPException(status_code=401, detail=str(e))


# Leaderboard Mini App Models
class StatsResponse(BaseModel):
    """Overview statistics response."""

    total_messages: int
    total_users: int
    total_reactions: int
    total_media: int
    messages_per_day: float


class ActivityDataPoint(BaseModel):
    """Single activity data point."""

    date: str
    messages: int
    users: int


class ActivityResponse(BaseModel):
    """Activity timeline response."""

    data: list[ActivityDataPoint]


class LeaderboardUser(BaseModel):
    """User in leaderboard."""

    rank: int
    user_id: int
    first_name: str
    last_name: str | None
    username: str | None
    score: int


class LeaderboardResponse(BaseModel):
    """Leaderboard response with pagination."""

    users: list[LeaderboardUser]
    total: int
    page: int
    limit: int


async def verify_mini_app_jwt(
    request: Request,
    authorization: str = Security(api_key_header),
) -> dict:
    """
    Verify JWT token for mini-app endpoints.

    Returns auth info with user_id and chat_id.
    Raises 401 if not authenticated or not a mini-app token.
    """
    auth_info = await verify_api_key_or_jwt(request, authorization)
    if auth_info.get("type") != "jwt":
        raise HTTPException(
            status_code=401, detail="Mini app authentication required"
        )
    return auth_info


def verify_chat_access(auth_info: dict, chat_id: int) -> None:
    """
    Verify that the authenticated user has access to the chat.

    For now, we allow access if the user authenticated from this chat.
    """
    token_chat_id = auth_info.get("chat_id")
    if token_chat_id is not None and token_chat_id != chat_id:
        raise HTTPException(
            status_code=403, detail="Access denied to this chat"
        )


@router.get("/api/v1/mini-app/stats", response_model=StatsResponse)
async def get_mini_app_stats(
    chat_id: Annotated[int, Query(description="Chat ID")],
    period: Annotated[str, Query(description="Time period")] = "30d",
    request: Request = None,
    auth_info: dict = Depends(verify_mini_app_jwt),
):
    """
    Get overview statistics for a chat.

    Requires mini-app JWT authentication.
    """
    verify_chat_access(auth_info, chat_id)

    queries = get_mini_app_queries(request)
    start_date, end_date = get_period_dates(period)
    stats = queries.get_overview_stats(chat_id, start_date, end_date)

    return StatsResponse(
        total_messages=stats["total_messages"],
        total_users=stats["total_users"],
        total_reactions=stats["total_reactions"],
        total_media=stats["total_media"],
        messages_per_day=stats["messages_per_day"],
    )


@router.get("/api/v1/mini-app/activity", response_model=ActivityResponse)
async def get_mini_app_activity(
    chat_id: Annotated[int, Query(description="Chat ID")],
    period: Annotated[str, Query(description="Time period")] = "30d",
    request: Request = None,
    auth_info: dict = Depends(verify_mini_app_jwt),
):
    """
    Get daily activity timeline for a chat.

    Requires mini-app JWT authentication.
    """
    verify_chat_access(auth_info, chat_id)

    queries = get_mini_app_queries(request)
    start_date, end_date = get_period_dates(period)
    activity = queries.get_daily_activity(chat_id, start_date, end_date)

    return ActivityResponse(
        data=[
            ActivityDataPoint(
                date=row["date"],
                messages=row["messages"],
                users=row["users"],
            )
            for row in activity
        ]
    )


@router.get("/api/v1/mini-app/leaderboard", response_model=LeaderboardResponse)
async def get_mini_app_leaderboard(
    chat_id: Annotated[int, Query(description="Chat ID")],
    period: Annotated[str, Query(description="Time period")] = "30d",
    metric: Annotated[
        str, Query(description="Ranking metric")
    ] = "message_count",
    page: Annotated[int, Query(ge=1, description="Page number")] = 1,
    limit: Annotated[int, Query(ge=1, le=100, description="Items per page")] = 20,
    request: Request = None,
    auth_info: dict = Depends(verify_mini_app_jwt),
):
    """
    Get user leaderboard for a chat.

    Requires mini-app JWT authentication.
    """
    verify_chat_access(auth_info, chat_id)

    # Validate metric
    valid_metrics = ["message_count", "reactions_sent", "reactions_received", "active_days"]
    if metric not in valid_metrics:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid metric. Must be one of: {', '.join(valid_metrics)}",
        )

    queries = get_mini_app_queries(request)
    start_date, end_date = get_period_dates(period)
    offset = (page - 1) * limit

    users = queries.get_user_rankings(
        chat_id=chat_id,
        metric=metric,
        limit=limit,
        offset=offset,
        start_date=start_date,
        end_date=end_date,
    )
    total = queries.get_user_rankings_total(chat_id, start_date, end_date)

    return LeaderboardResponse(
        users=[
            LeaderboardUser(
                rank=user["rank"],
                user_id=user["user_id"],
                first_name=user["first_name"],
                last_name=user.get("last_name"),
                username=user.get("username"),
                score=user["score"],
            )
            for user in users
        ],
        total=total,
        page=page,
        limit=limit,
    )


@router.get("/health")
async def health_check():
    """Health check endpoint (unauthenticated)."""
    return {"status": "ok"}
