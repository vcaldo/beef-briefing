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
from dash import Dash, html, dcc, callback, Output, Input
import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash_iconify import DashIconify
from sqlalchemy import create_engine

from src.database import DashboardQueries, SessionQueries
from src.auth import TelegramAuthService
from src.routes import auth_bp
from src.routes.auth import init_auth_routes
from src.pages import create_landing_page
from src.utils import filter_chats_for_user


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

# Initialize Dash app with Bootstrap theme
app = Dash(
    __name__,
    server=server,
    external_stylesheets=[dbc.themes.BOOTSTRAP],
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
            html.Div(id="page-content"),
        ]
    )


app.layout = serve_layout


# Routing callback for dynamic page content
@callback(Output("page-content", "children"), Input("url", "pathname"))
def display_page(pathname):
    """Route to appropriate page based on URL."""
    # Get user from session cookie (Flask g is not available in callbacks)
    session_id = request.cookies.get("session_id")
    user = get_auth_service().get_session(session_id) if session_id else None

    if not user:
        # Redirect to login if not authenticated
        return dmc.MantineProvider(
            dmc.Center(
                dmc.Stack(
                    [
                        dmc.Text("Session expired", c="dimmed"),
                        dmc.Anchor(
                            dmc.Button("Login", variant="light"),
                            href=f"{cfg.leaderboard_path}/login",
                        ),
                    ],
                    align="center",
                    gap="md",
                ),
                style={"minHeight": "100vh"},
            )
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
        return create_landing_page(visible_chats, user, cfg.leaderboard_path)

    # 404 fallback
    return dmc.MantineProvider(
        dmc.Center(
            dmc.Stack(
                [
                    DashIconify(icon="mdi:alert-circle-outline", width=64, color="gray"),
                    dmc.Title("404 - Page Not Found", order=2),
                    dmc.Text("The page you're looking for doesn't exist.", c="dimmed"),
                    dmc.Anchor(
                        dmc.Button("Go Home", variant="light"),
                        href=cfg.leaderboard_path,
                    ),
                ],
                align="center",
                gap="md",
            ),
            style={"minHeight": "100vh"},
        )
    )


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
