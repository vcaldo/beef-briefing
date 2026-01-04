"""
Leaderboard application entry point.
Flask + Dash with New Relic APM instrumentation.
"""

import logging
import os
import sys

from config import load_config

# Load config first (before New Relic import)
cfg = load_config()

# Initialize New Relic APM if configured (must be done before other imports)
if cfg.new_relic_enabled():
    os.environ["NEW_RELIC_APP_NAME"] = cfg.new_relic_full_app_name
    os.environ["NEW_RELIC_LICENSE_KEY"] = cfg.new_relic_license_key
    import newrelic.agent

    newrelic.agent.initialize()

from flask import Flask, g, redirect, request
from dash import Dash, html, dcc, callback, Output, Input, clientside_callback, ClientsideFunction
import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash_iconify import DashIconify
from sqlalchemy import create_engine

from src.database import DashboardQueries, SessionQueries
from src.auth import TelegramAuthService
from src.api import PhotoClient, GalleryClient
from src.routes import auth_bp
from src.routes.auth import init_auth_routes
from src.pages import (
    create_landing_page,
    create_overview_page,
    create_activity_page,
    create_reactions_page,
    create_leaderboard_page,
    create_my_stats_page,
    create_sentiment_page,
    create_topics_page,
    create_insights_page,
    create_comedy_page,
    create_gallery_page,
)
from src.utils import filter_chats_for_user
from src.components import DEFAULT_PERIOD
from src.themes import (
    GOOGLE_FONTS_URL,
    THEME_STORAGE_KEY,
    THEMES,
    DEFAULT_LIGHT_THEME,
    DEFAULT_DARK_THEME,
    get_theme,
)
from src.components import THEME_SWITCHER_ID


def setup_logging(config):
    """Configure logging based on environment."""
    level_map = {
        "debug": logging.DEBUG,
        "info": logging.INFO,
        "warn": logging.WARNING,
        "warning": logging.WARNING,
        "error": logging.ERROR,
    }
    level = level_map.get(config.log_level.lower(), logging.INFO)

    if config.is_production():
        # JSON-like structured logging for production
        format_str = (
            '{"time":"%(asctime)s","level":"%(levelname)s",'
            '"logger":"%(name)s","message":"%(message)s"}'
        )
    else:
        # Human-readable for development
        format_str = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"

    logging.basicConfig(level=level, format=format_str, stream=sys.stdout)
    return logging.getLogger(__name__)


logger = setup_logging(cfg)

# Initialize Flask server
server = Flask(__name__)

# Determine URL prefixes for Dash
# - routes_pathname_prefix: where the app actually responds (internal routing)
# - requests_pathname_prefix: what URL prefix to use for assets/callbacks (browser-facing)
#
# In production behind Traefik:
#   - Traefik strips /leaderboard prefix before forwarding, so app sees /
#   - But generated URLs must include /leaderboard for the browser
#
# In development:
#   - No reverse proxy, so app must respond on /leaderboard directly
#   - Generated URLs also use /leaderboard
requests_prefix = cfg.leaderboard_path
if not requests_prefix.endswith("/"):
    requests_prefix = requests_prefix + "/"

# In production, Traefik strips the path prefix; in dev, we handle it directly
routes_prefix = "/" if cfg.is_production() else requests_prefix

# Initialize Dash app with Bootstrap theme and custom fonts
app = Dash(
    __name__,
    server=server,
    external_stylesheets=[dbc.themes.BOOTSTRAP, GOOGLE_FONTS_URL],
    title="Beef Briefing Leaderboard",
    routes_pathname_prefix=routes_prefix,
    requests_pathname_prefix=requests_prefix,
    suppress_callback_exceptions=True,
)

# Set emoji favicon using SVG data URL
app.index_string = """<!DOCTYPE html>
<html>
    <head>
        {%metas%}
        <title>{%title%}</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        {%css%}
        <style>
            /* Animation keyframes for card entrance */
            @keyframes fadeSlideIn {
                from {
                    opacity: 0;
                    transform: translateY(20px);
                }
                to {
                    opacity: 1;
                    transform: translateY(0);
                }
            }
        </style>
    </head>
    <body>
        {%app_entry%}
        <footer>
            {%config%}
            {%scripts%}
            {%renderer%}
        </footer>
    </body>
</html>"""

# Database engine (lazy initialization)
_engine = None


def get_engine():
    """Get or create SQLAlchemy engine."""
    global _engine
    if _engine is None:
        _engine = create_engine(cfg.dsn(), pool_pre_ping=True)
        logger.info("Database connection established")
    return _engine


# Query executor (lazy initialization)
_queries = None


def get_queries() -> DashboardQueries:
    """Get or create DashboardQueries instance."""
    global _queries
    if _queries is None:
        _queries = DashboardQueries(get_engine())
        logger.info("DashboardQueries initialized")
    return _queries


# Session queries (lazy initialization)
_session_queries = None


def get_session_queries() -> SessionQueries:
    """Get or create SessionQueries instance."""
    global _session_queries
    if _session_queries is None:
        _session_queries = SessionQueries(get_engine())
        logger.info("SessionQueries initialized")
    return _session_queries


# Auth service (lazy initialization)
_auth_service = None


def get_auth_service() -> TelegramAuthService:
    """Get or create TelegramAuthService instance."""
    global _auth_service
    if _auth_service is None:
        _auth_service = TelegramAuthService(
            config=cfg,
            session_queries=get_session_queries(),
            dashboard_queries=get_queries(),
        )
        logger.info("TelegramAuthService initialized")
    return _auth_service


# Photo client (lazy initialization)
_photo_client = None


def get_photo_client() -> PhotoClient:
    """Get or create PhotoClient instance."""
    global _photo_client
    if _photo_client is None:
        if cfg.api_service_url and cfg.api_key:
            _photo_client = PhotoClient(cfg.api_service_url, cfg.api_key)
            logger.info("PhotoClient initialized")
        else:
            # Return a stub client that always returns None
            _photo_client = PhotoClient("", "")
            logger.warning(
                "PhotoClient not configured (missing API_SERVICE_URL or API_KEY_FILE)"
            )
    return _photo_client


# Gallery client (lazy initialization)
_gallery_client = None


def get_gallery_client() -> GalleryClient:
    """Get or create GalleryClient instance."""
    global _gallery_client
    if _gallery_client is None:
        if cfg.card_renderer_url and cfg.card_renderer_api_key:
            _gallery_client = GalleryClient(
                cfg.card_renderer_url, cfg.card_renderer_api_key
            )
            logger.info("GalleryClient initialized")
        else:
            # Return a stub client that always returns None
            _gallery_client = GalleryClient("", "")
            logger.warning(
                "GalleryClient not configured (missing CARD_RENDERER_URL or API key)"
            )
    return _gallery_client


# Dynamic layout function
def serve_layout():
    """
    Serve layout dynamically based on route.

    This function is called on every page load, allowing us to
    access Flask's g.user for authenticated layouts.
    """
    return html.Div(
        [
            dcc.Location(id="url", refresh=False),
            # Theme store persisted to localStorage
            dcc.Store(id="theme-store", storage_type="local"),
            # Trigger for initial theme detection
            dcc.Store(id="theme-initialized", data=False),
            # Main content wrapped in themed MantineProvider
            html.Div(id="themed-app-container"),
        ]
    )


app.layout = serve_layout


# Clientside callback to detect OS theme preference on initial load
app.clientside_callback(
    """
    function(initialized, currentTheme) {
        // If theme is already set, don't override
        if (currentTheme) {
            return window.dash_clientside.no_update;
        }

        // Detect OS preference
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        return prefersDark ? 'smokehouse' : 'butcher-paper';
    }
    """,
    Output("theme-store", "data", allow_duplicate=True),
    Input("theme-initialized", "data"),
    Input("theme-store", "data"),
    prevent_initial_call="initial_duplicate",
)


# Callback to sync theme switcher with store
@callback(
    Output("theme-store", "data"),
    Input(THEME_SWITCHER_ID, "value"),
    prevent_initial_call=True,
)
def update_theme_store(theme_value):
    """Update theme store when user changes theme via switcher."""
    if theme_value:
        return theme_value
    return DEFAULT_LIGHT_THEME


def _create_404_page(theme: dict, background: dict, base_url: str):
    """Create a 404 page with themed styling."""
    return dmc.MantineProvider(
        html.Div(
            dmc.Center(
                dmc.Stack(
                    [
                        DashIconify(icon="mdi:alert-circle-outline", width=64, color="gray"),
                        dmc.Title("404 - Page Not Found", order=2),
                        dmc.Text("The page you're looking for doesn't exist.", c="dimmed"),
                        dmc.Anchor(
                            dmc.Button("Go Home", variant="light"),
                            href=base_url,
                            refresh=True,
                        ),
                    ],
                    align="center",
                    gap="md",
                ),
                mih="100vh",
            ),
            style=background,
        ),
        theme=theme,
    )


def parse_group_path(route_path: str) -> tuple[int | None, str | None]:
    """
    Parse a group page path to extract chat_id and page name.

    Args:
        route_path: Route path like /group/123/overview

    Returns:
        Tuple of (chat_id, page_name) or (None, None) if not a group path
    """
    import re

    match = re.match(r"^/group/(-?\d+)(?:/(\w+(?:-\w+)?))?$", route_path)
    if not match:
        return None, None

    chat_id = int(match.group(1))
    page_name = match.group(2) or "overview"
    return chat_id, page_name


def parse_query_params(search: str | None) -> dict:
    """
    Parse URL query string into a dict.

    Args:
        search: URL search string like ?period=7d&metric=message_count

    Returns:
        Dict of query parameters
    """
    if not search:
        return {}

    from urllib.parse import parse_qs

    params = parse_qs(search.lstrip("?"))
    # Convert single-value lists to scalars
    return {k: v[0] if len(v) == 1 else v for k, v in params.items()}


def get_period_from_search(search: str | None) -> str:
    """
    Extract period parameter from URL query string.

    Args:
        search: URL search string like ?period=7d

    Returns:
        Period value or default
    """
    params = parse_query_params(search)
    return params.get("period", DEFAULT_PERIOD)


# Main routing callback with theming
@callback(
    Output("themed-app-container", "children"),
    Input("url", "pathname"),
    Input("url", "search"),
    Input("theme-store", "data"),
)
def display_page(pathname, search, theme_name):
    """Route to appropriate page based on URL with theme applied."""
    # Get theme configuration
    theme_config = get_theme(theme_name)
    theme = theme_config["theme"]
    background = theme_config["background"]

    # Get user from session cookie (Flask g is not available in callbacks)
    session_id = request.cookies.get("session_id")
    user = get_auth_service().get_session(session_id) if session_id else None

    if not user:
        # Redirect to login if not authenticated
        return dmc.MantineProvider(
            html.Div(
                dmc.Center(
                    dmc.Stack(
                        [
                            dmc.Text("Session expired", c="dimmed"),
                            dmc.Anchor(
                                dmc.Button("Login", variant="light"),
                                href=f"{cfg.leaderboard_path}/login",
                                refresh=True,
                            ),
                        ],
                        align="center",
                        gap="md",
                    ),
                    mih="100vh",
                ),
                style=background,
            ),
            theme=theme,
        )

    # Strip prefix for route matching
    # Note: pathname comes from dcc.Location which reflects the browser URL,
    # so it always includes the /leaderboard prefix regardless of environment
    route_path = pathname or "/"
    prefix = cfg.leaderboard_path
    if route_path.startswith(prefix):
        route_path = route_path[len(prefix):] or "/"

    # Route to landing page for root
    if route_path in ["/", ""]:
        all_chats = get_queries().get_chats_with_stats()
        visible_chats = filter_chats_for_user(all_chats, user)
        return dmc.MantineProvider(
            html.Div(
                create_landing_page(
                    visible_chats, user, cfg.leaderboard_path, theme_name, get_photo_client()
                ),
                style=background,
            ),
            theme=theme,
        )

    # Route to group pages
    chat_id, page_name = parse_group_path(route_path)
    if chat_id is not None:
        # Get chat info and verify access
        chat_info = get_queries().get_chat_info(chat_id)
        if not chat_info:
            # Chat not found - show 404
            return _create_404_page(theme, background, cfg.leaderboard_path)

        # Check if user has access to this chat
        all_chats = get_queries().get_chats_with_stats()
        visible_chats = filter_chats_for_user(all_chats, user)
        visible_chat_ids = [c["id"] for c in visible_chats]
        if chat_id not in visible_chat_ids:
            # Access denied - show 403
            return dmc.MantineProvider(
                html.Div(
                    dmc.Center(
                        dmc.Stack(
                            [
                                DashIconify(icon="mdi:lock-outline", width=64, color="gray"),
                                dmc.Title("Access Denied", order=2),
                                dmc.Text("You don't have access to this group.", c="dimmed"),
                                dmc.Anchor(
                                    dmc.Button("Go Home", variant="light"),
                                    href=cfg.leaderboard_path,
                                    refresh=True,
                                ),
                            ],
                            align="center",
                            gap="md",
                        ),
                        mih="100vh",
                    ),
                    style=background,
                ),
                theme=theme,
            )

        # Get query params from URL
        query_params = parse_query_params(search)
        period = query_params.get("period", DEFAULT_PERIOD)
        queries = get_queries()
        photo_client = get_photo_client()

        # Route to specific group page
        page_creators = {
            "overview": create_overview_page,
            "activity": create_activity_page,
            "reactions": create_reactions_page,
            "leaderboard": create_leaderboard_page,
            "my-stats": create_my_stats_page,
            "sentiment": create_sentiment_page,
            "topics": create_topics_page,
            "insights": create_insights_page,
            "comedy": create_comedy_page,
            "gallery": create_gallery_page,
        }

        page_creator = page_creators.get(page_name)
        if page_creator:
            # Build base kwargs for all pages
            page_kwargs = {
                "chat_id": chat_id,
                "chat_info": chat_info,
                "user": user,
                "period": period,
                "base_url": cfg.leaderboard_path,
                "theme_name": theme_name,
                "queries": queries,
                "photo_client": photo_client,
            }

            # Add page-specific kwargs from query params
            if page_name == "leaderboard":
                page_kwargs["metric"] = query_params.get("metric", "message_count")
                try:
                    page_kwargs["page"] = int(query_params.get("page", 1))
                except (ValueError, TypeError):
                    page_kwargs["page"] = 1
            elif page_name == "gallery":
                # Gallery page needs gallery_client and week param (admin only)
                page_kwargs["gallery_client"] = get_gallery_client()
                page_kwargs["week"] = query_params.get("week")

            try:
                page_content = page_creator(**page_kwargs)
            except Exception as e:
                logger.error(f"Error rendering {page_name} page: {e}")
                page_content = dmc.Center(
                    dmc.Stack(
                        [
                            DashIconify(
                                icon="mdi:alert-circle-outline", width=64, color="red"
                            ),
                            dmc.Title("Something went wrong", order=2),
                            dmc.Text(
                                "An error occurred while loading this page.",
                                c="dimmed",
                            ),
                            dmc.Anchor(
                                dmc.Button("Try Again", variant="light"),
                                href=request.url,
                                refresh=True,
                            ),
                        ],
                        align="center",
                        gap="md",
                    ),
                    mih="100vh",
                )

            return dmc.MantineProvider(
                html.Div(
                    page_content,
                    style=background,
                ),
                theme=theme,
            )

    # 404 fallback
    return _create_404_page(theme, background, cfg.leaderboard_path)


# Register auth blueprint and initialize auth routes
# Note: Blueprint is registered on Flask server, not Dash app
# The URL prefix depends on whether Traefik strips the path or not
if cfg.is_production():
    # In production, Traefik strips /leaderboard, so routes are at root
    server.register_blueprint(auth_bp)
else:
    # In development, we need the full path prefix
    server.register_blueprint(auth_bp, url_prefix=cfg.leaderboard_path)

# Initialize auth routes with services (must be done after blueprint registration)
init_auth_routes(get_auth_service(), cfg)


# Route protection: check authentication before all requests
@server.before_request
def check_auth():
    """
    Protect routes that require authentication.

    Allows:
    - /login, /auth/callback, /logout (auth flow)
    - /health (health check)
    - Static assets (/_dash-*, /assets/*)

    All other routes require valid session.
    """
    path = request.path

    # In development, paths include the leaderboard prefix
    # In production, Traefik strips it
    if not cfg.is_production():
        # Remove the leaderboard prefix for checking
        prefix = cfg.leaderboard_path
        if path.startswith(prefix):
            path = path[len(prefix):] or "/"

    # Allow auth-related paths
    if path in ["/login", "/logout"] or path.startswith("/auth/"):
        return None

    # Allow health check
    if path == "/health":
        return None

    # Allow Dash static assets
    if path.startswith("/_dash") or path.startswith("/assets"):
        return None

    # Check for valid session
    session_id = request.cookies.get("session_id")
    if not session_id:
        return _redirect_to_login()

    session = get_auth_service().get_session(session_id)
    if not session:
        response = _redirect_to_login()
        response.delete_cookie("session_id")
        return response

    # Store session info in Flask g for use in request handlers
    g.user = session


def _redirect_to_login():
    """Redirect to login page with correct prefix."""
    if cfg.is_production():
        return redirect(cfg.leaderboard_path + "/login")
    return redirect(cfg.leaderboard_path + "/login")


# Health check endpoint on Flask server
@server.route("/health")
def health():
    """Health check endpoint."""
    return "OK", 200


if __name__ == "__main__":
    logger.info(f"Starting leaderboard service on port {cfg.leaderboard_port}")
    logger.info(f"Environment: {cfg.environment}")
    logger.info(f"Routes prefix: {routes_prefix}, Requests prefix: {requests_prefix}")

    if cfg.new_relic_enabled():
        logger.info(f"New Relic APM initialized: {cfg.new_relic_full_app_name}")
    else:
        logger.debug("New Relic APM not configured, skipping initialization")

    # Run the app
    app.run(host="0.0.0.0", port=cfg.leaderboard_port, debug=not cfg.is_production())
