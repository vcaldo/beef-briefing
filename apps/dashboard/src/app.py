"""
Dash application factory for Beef Dashboard.
"""

import logging
from functools import wraps
from typing import Any, Callable, Optional

import dash
from dash import dcc, html
from flask import Flask, redirect, request, session, url_for, make_response

from src.config import Config
from src.database.connection import DatabaseConnection
from src.database.queries import DashboardQueries
from src.auth.telegram_oauth import (
    validate_telegram_auth,
    generate_telegram_widget_html,
    TelegramAuthData,
)
from src.auth.membership import verify_membership_any_chat, MembershipCache
from src.auth.session import SessionManager

logger = logging.getLogger(__name__)


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

    # Initialize membership cache
    membership_cache = MembershipCache()

    # Initialize queries
    queries = DashboardQueries(db.engine)

    # Create Flask server
    server = Flask(__name__)
    server.secret_key = config.flask_secret_key
    server.config["SESSION_COOKIE_SECURE"] = config.is_production()
    server.config["SESSION_COOKIE_HTTPONLY"] = True
    server.config["SESSION_COOKIE_SAMESITE"] = "Lax"

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

    # Store references for use in callbacks
    app.config["db"] = db
    app.config["queries"] = queries
    app.config["session_manager"] = session_manager
    app.config["membership_cache"] = membership_cache
    app.config["app_config"] = config

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
    register_auth_routes(server, config, db, session_manager, membership_cache)

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
    db: DatabaseConnection,
    session_manager: SessionManager,
    membership_cache: MembershipCache,
) -> None:
    """Register authentication routes on the Flask server."""

    @server.route("/beef-dashboard/login")
    def login_page():
        """Display Telegram login widget."""
        # Check if already authenticated
        session_id = request.cookies.get("dashboard_session")
        if session_id:
            session_data = session_manager.get_session(session_id)
            if session_data:
                return redirect("/beef-dashboard/")

        # Get bot username from token (format: 123456:ABC-DEF)
        bot_username = config.telegram_bot_token.split(":")[0]
        # Note: You'll need to set the actual bot username in config

        callback_url = f"https://{config.dashboard_domain}/beef-dashboard/auth/callback"
        if config.is_development():
            callback_url = f"http://localhost:{config.dashboard_port}/beef-dashboard/auth/callback"

        widget_html = generate_telegram_widget_html(
            bot_username=bot_username,  # This should be the bot's @username
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
    def auth_callback():
        """Handle Telegram OAuth callback."""
        # Get auth data from query parameters
        auth_data = request.args.to_dict()

        # Validate the authentication
        validated = validate_telegram_auth(auth_data, config.telegram_bot_token)
        if validated is None:
            logger.warning("Invalid Telegram auth data received")
            return redirect("/beef-dashboard/login?error=invalid_auth")

        # Verify group membership
        membership = verify_membership_any_chat(
            user_id=validated.id,
            allowed_chat_ids=config.allowed_chat_ids,
            bot_token=config.telegram_bot_token,
            cache=membership_cache,
        )

        if not membership.is_member:
            logger.warning(
                "User not a member of any allowed group",
                extra={"user_id": validated.id, "error": membership.error}
            )
            return redirect("/beef-dashboard/login?error=not_member")

        # Create session
        session_id = session_manager.create_session(
            user_id=validated.id,
            username=validated.username,
            first_name=validated.first_name,
            photo_url=validated.photo_url,
            allowed_chat_ids=config.allowed_chat_ids,
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


def require_auth(f: Callable) -> Callable:
    """
    Decorator to require authentication for Dash callbacks.
    Use with Flask request context.
    """
    @wraps(f)
    def decorated_function(*args: Any, **kwargs: Any) -> Any:
        from flask import current_app

        session_id = request.cookies.get("dashboard_session")
        if not session_id:
            return redirect("/beef-dashboard/login")

        session_manager = current_app.config.get("session_manager")
        if session_manager:
            session_data = session_manager.get_session(session_id)
            if not session_data:
                return redirect("/beef-dashboard/login")

        return f(*args, **kwargs)

    return decorated_function
