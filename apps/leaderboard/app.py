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

from flask import Flask
from dash import Dash, html
import dash_bootstrap_components as dbc
from sqlalchemy import create_engine

from src.database import DashboardQueries


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

# Determine URL prefix for Dash
# In production behind Traefik, the path prefix is stripped, so Dash sees /
# In development, we access directly on port 8050, so no prefix needed
# The requests_pathname_prefix tells Dash what URL prefix to use for assets/callbacks
url_prefix = cfg.leaderboard_path if cfg.is_production() else "/"
if not url_prefix.endswith("/"):
    url_prefix = url_prefix + "/"

# Initialize Dash app with Bootstrap theme
app = Dash(
    __name__,
    server=server,
    external_stylesheets=[dbc.themes.BOOTSTRAP],
    title="Beef Briefing Leaderboard",
    requests_pathname_prefix=url_prefix,
)

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


# Health check endpoint on Flask server
@server.route("/health")
def health():
    """Health check endpoint."""
    return "OK", 200


if __name__ == "__main__":
    logger.info(f"Starting leaderboard service on port {cfg.leaderboard_port}")
    logger.info(f"Environment: {cfg.environment}")
    logger.info(f"URL prefix: {url_prefix}")

    if cfg.new_relic_enabled():
        logger.info(f"New Relic APM initialized: {cfg.new_relic_full_app_name}")
    else:
        logger.debug("New Relic APM not configured, skipping initialization")

    # Run the app
    app.run(host="0.0.0.0", port=cfg.leaderboard_port, debug=not cfg.is_production())
