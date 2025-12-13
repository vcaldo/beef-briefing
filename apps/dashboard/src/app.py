"""
Dash application factory for Beef Dashboard.
"""

import logging
from typing import Optional

import dash
from dash import dcc, html
from flask import Flask, redirect, request, make_response
from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

from src.config import Config
from src.database.connection import DatabaseConnection
from src.database.queries import DashboardQueries
from src.auth.telegram_oauth import (
    validate_telegram_auth,
    generate_telegram_widget_html,
    TelegramAuthData,
)
from src.auth.session import SessionManager
from src.api.api_client import ApiServiceClient

logger = logging.getLogger(__name__)

# User activity lookback period for determining accessible chats (in days)
ACTIVITY_LOOKBACK_DAYS = 15


def create_app(config: Config) -> dash.Dash:
    """
    Create and configure the Dash application.

    Args:
        config: Application configuration

    Returns:
        Configured Dash application
    """
    # Initialize database
    db = DatabaseConnection(config)
    db.initialize()

    # Initialize session manager
    session_manager = SessionManager(db.engine, config.session_lifetime_days)

    # Initialize API client for api-service
    api_client = ApiServiceClient(
        base_url=config.api_service_url,
        api_key=config.analytics_api_key if config.analytics_api_key else None,
    )

    # Initialize queries
    queries = DashboardQueries(db.engine)

    # Create Flask server
    server = Flask(__name__)
    server.secret_key = config.flask_secret_key
    server.config["SESSION_COOKIE_SECURE"] = config.is_production()
    server.config["SESSION_COOKIE_HTTPONLY"] = True
    server.config["SESSION_COOKIE_SAMESITE"] = "Lax"

    # Initialize rate limiter
    limiter = Limiter(
        key_func=get_remote_address,
        app=server,
        default_limits=["200 per day", "50 per hour"],
        storage_uri="memory://",
    )

    # Create Dash app
    app = dash.Dash(
        __name__,
        server=server,
        url_base_pathname="/beef-dashboard/",
        suppress_callback_exceptions=True,
        title="Beef Dashboard",
        update_title=None,
        assets_folder="assets",
        external_stylesheets=[
            # Google Fonts - JetBrains Mono and Space Grotesk
            "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;600;700&family=Space+Grotesk:wght@300;400;500;600;700&display=swap",
        ],
    )

    # Set custom favicon
    app._favicon = "favicon.svg"

    # Store references for use in callbacks (on Flask server config, not Dash config)
    server.config["db"] = db
    server.config["queries"] = queries
    server.config["session_manager"] = session_manager
    server.config["api_client"] = api_client
    server.config["app_config"] = config

    # Define the app layout
    app.layout = html.Div(
        id="app-container",
        children=[
            dcc.Location(id="url", refresh=False),
            dcc.Store(id="session-store", storage_type="session"),
            html.Div(id="page-content"),
        ],
    )

    # Register Flask routes for authentication
    register_auth_routes(server, config, queries, session_manager, api_client, limiter)

    # Register health check route
    @server.route("/beef-dashboard/health")
    def health_check():
        return {"status": "healthy"}

    # Register callbacks
    from src.callbacks.dashboard_callbacks import register_callbacks
    register_callbacks(app)

    return app


def register_auth_routes(
    server: Flask,
    config: Config,
    queries: DashboardQueries,
    session_manager: SessionManager,
    api_client: ApiServiceClient,
    limiter: Limiter,
) -> None:
    """Register authentication routes on the Flask server."""

    @server.route("/beef-dashboard/login")
    @limiter.limit("30 per minute")
    def login_page():
        """Display Telegram login widget."""
        session_id = request.cookies.get("dashboard_session")
        if session_id:
            session_data = session_manager.get_session(session_id)
            if session_data:
                return redirect("/beef-dashboard/")

        callback_url = f"https://{config.dashboard_domain}/beef-dashboard/auth/callback"
        if config.is_development():
            callback_url = f"http://localhost:{config.dashboard_port}/beef-dashboard/auth/callback"

        widget_html = generate_telegram_widget_html(
            bot_username=config.telegram_bot_username,
            callback_url=callback_url,
        )

        return f"""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Login - Beef Dashboard</title>
            <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&family=Space+Grotesk:wght@400;600;700&display=swap" rel="stylesheet">
            <style>
                :root {{
                    --color-bg-primary: #0a0e1a;
                    --color-bg-secondary: #141822;
                    --color-accent-primary: #00d9ff;
                    --color-text-primary: #e8eaed;
                    --color-text-secondary: #9aa0a6;
                }}
                * {{
                    margin: 0;
                    padding: 0;
                    box-sizing: border-box;
                }}
                body {{
                    font-family: 'Space Grotesk', sans-serif;
                    background: radial-gradient(circle at 20% 80%, rgba(0,217,255,0.08) 0%, transparent 50%),
                                radial-gradient(circle at 80% 20%, rgba(255,107,107,0.06) 0%, transparent 50%),
                                linear-gradient(180deg, #0a0e1a 0%, #0d1117 100%);
                    min-height: 100vh;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    color: var(--color-text-primary);
                }}
                .login-container {{
                    background: rgba(20, 24, 34, 0.7);
                    backdrop-filter: blur(12px) saturate(180%);
                    border: 1px solid rgba(255,255,255,0.1);
                    border-radius: 16px;
                    padding: 48px;
                    text-align: center;
                    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
                    animation: fadeInScale 0.6s ease-out;
                }}
                @keyframes fadeInScale {{
                    from {{
                        opacity: 0;
                        transform: scale(0.95) translateY(10px);
                    }}
                    to {{
                        opacity: 1;
                        transform: scale(1) translateY(0);
                    }}
                }}
                h1 {{
                    font-family: 'JetBrains Mono', monospace;
                    font-size: 2rem;
                    font-weight: 700;
                    margin-bottom: 8px;
                    background: linear-gradient(135deg, var(--color-accent-primary), #ff6b6b);
                    -webkit-background-clip: text;
                    -webkit-text-fill-color: transparent;
                    background-clip: text;
                }}
                p {{
                    color: var(--color-text-secondary);
                    margin-bottom: 32px;
                    font-size: 1rem;
                }}
                .widget-container {{
                    display: flex;
                    justify-content: center;
                }}
            </style>
        </head>
        <body>
            <div class="login-container">
                <h1>Beef Dashboard</h1>
                <p>Sign in with Telegram to access analytics</p>
                <div class="widget-container">
                    {widget_html}
                </div>
            </div>
        </body>
        </html>
        """

    @server.route("/beef-dashboard/auth/callback")
    @limiter.limit("10 per minute")
    def auth_callback():
        """Handle Telegram OAuth callback."""
        auth_data = request.args.to_dict()
        validated = validate_telegram_auth(auth_data, config.telegram_bot_token)
        if validated is None:
            logger.warning("Invalid Telegram auth data received")
            return redirect("/beef-dashboard/login?error=invalid_auth")

        # Determine accessible chats based on user role
        is_admin = config.is_admin(validated.id)

        if is_admin:
            # Admins get access to all chats in the database
            all_chats = queries.get_available_chats()
            accessible_chat_ids = [c["id"] for c in all_chats]
            logger.info(
                "Admin user authenticated, granting access to all chats",
                extra={"user_id": validated.id, "chat_count": len(accessible_chat_ids)}
            )
        else:
            # Regular users: fetch chats where they've been active
            accessible_chat_ids = api_client.get_user_active_chats(
                user_id=validated.id,
                days=ACTIVITY_LOOKBACK_DAYS
            )
            logger.info(
                "User authenticated, fetched active chats",
                extra={"user_id": validated.id, "chat_count": len(accessible_chat_ids)}
            )

        if not accessible_chat_ids:
            logger.warning(
                "User has no active chats in the configured lookback period",
                extra={"user_id": validated.id, "days": ACTIVITY_LOOKBACK_DAYS}
            )
            return redirect("/beef-dashboard/login?error=no_activity")

        # Create session with accessible chats
        session_id = session_manager.create_session(
            user_id=validated.id,
            username=validated.username,
            first_name=validated.first_name,
            photo_url=validated.photo_url,
            allowed_chat_ids=accessible_chat_ids,
        )

        # Set cookie and redirect
        response = make_response(redirect("/beef-dashboard/"))
        response.set_cookie(
            "dashboard_session",
            session_id,
            max_age=config.session_lifetime_seconds,
            secure=config.is_production(),
            httponly=True,
            samesite="Lax",
        )

        logger.info(
            "User authenticated successfully",
            extra={"user_id": validated.id, "username": validated.username}
        )

        return response

    @server.route("/beef-dashboard/logout")
    def logout():
        """Log out the user."""
        session_id = request.cookies.get("dashboard_session")
        if session_id:
            session_manager.delete_session(session_id)

        response = make_response(redirect("/beef-dashboard/login"))
        response.delete_cookie("dashboard_session")
        return response
