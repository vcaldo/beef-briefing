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
from dash import Dash, html
import dash_bootstrap_components as dbc
from sqlalchemy import create_engine

from src.database import DashboardQueries, SessionQueries
from src.auth import TelegramAuthService
from src.routes import auth_bp
from src.routes.auth import init_auth_routes


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
    requests_pathname_prefix=requests_prefix
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


# Application layout - skeleton with "Coming Soon" message
app.layout = dbc.Container(
    [
        dbc.Row(
            [
                dbc.Col(
                    [
                        html.H1("Leaderboard", className="text-center my-4"),
                        html.P(
                            "Leaderboard coming soon!",
                            className="text-center lead text-muted",
                        ),
                    ],
                    width=12,
                )
            ]
        )
    ],
    fluid=True,
    className="py-5",
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
