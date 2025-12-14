"""
Authentication routes for Telegram OAuth.

Blueprint handles:
- GET /login - Display login page (or auto-login in dev mode)
- GET /auth/callback - Handle Telegram OAuth callback
- GET /logout - Clear session and redirect to login
"""

import logging

from dash import html
from flask import Blueprint, g, make_response, redirect, render_template_string, request

from src.pages import create_login_page

logger = logging.getLogger(__name__)

auth_bp = Blueprint("auth", __name__)

# These will be set by the app during initialization
_auth_service = None
_config = None


def init_auth_routes(auth_service, config):
    """
    Initialize auth routes with required services.

    Must be called before routes are used.

    Args:
        auth_service: TelegramAuthService instance
        config: Application Config instance
    """
    global _auth_service, _config
    _auth_service = auth_service
    _config = config


@auth_bp.route("/login")
def login():
    """
    Display login page or auto-login in development mode.

    In development (ENVIRONMENT != production):
        Auto-login as first admin user and redirect to dashboard.

    In production:
        Display Telegram Login Widget.
    """
    # Check if already logged in
    session_id = request.cookies.get("session_id")
    if session_id:
        session = _auth_service.get_session(session_id)
        if session:
            # Already logged in, redirect to dashboard
            return redirect(_get_dashboard_url())

    # Development mode: auto-login as first admin
    if not _config.is_production():
        first_admin = _config.get_first_admin_id()
        if first_admin:
            logger.info(
                "Dev mode auto-login",
                extra={"user_id": first_admin},
            )
            session_id = _auth_service.create_dev_session(first_admin)
            response = redirect(_get_dashboard_url())
            _set_session_cookie(response, session_id)
            return response
        else:
            logger.warning("Dev mode: No admin user IDs configured for auto-login")

    # Production mode: show login page
    callback_url = _get_callback_url()
    login_layout = create_login_page(_config.telegram_bot_username, callback_url)

    # Render the Dash component to HTML
    return _render_login_page(login_layout)


@auth_bp.route("/auth/callback")
def auth_callback():
    """
    Handle Telegram OAuth callback.

    Telegram redirects here with query parameters:
    - id: User ID
    - first_name: User's first name
    - last_name: (optional)
    - username: (optional)
    - photo_url: (optional)
    - auth_date: Unix timestamp
    - hash: HMAC-SHA256 hash

    Process:
    1. Validate HMAC-SHA256 hash
    2. Verify auth_date freshness
    3. Create session in database
    4. Set session cookie
    5. Redirect to dashboard
    """
    # Collect auth data from query parameters
    auth_data = {
        "id": request.args.get("id"),
        "first_name": request.args.get("first_name"),
        "auth_date": request.args.get("auth_date"),
        "hash": request.args.get("hash"),
    }

    # Add optional fields if present
    for field in ["last_name", "username", "photo_url"]:
        value = request.args.get(field)
        if value:
            auth_data[field] = value

    # Validate auth data
    if not _auth_service.validate_telegram_auth(auth_data):
        logger.warning(
            "Auth callback: validation failed",
            extra={"user_id": auth_data.get("id")},
        )
        return _render_error_page("Authentication failed. Please try again.")

    # Create session
    try:
        session_id = _auth_service.create_session(auth_data)
    except Exception as e:
        logger.exception("Auth callback: session creation failed")
        return _render_error_page("Session creation failed. Please try again.")

    # Redirect to dashboard with session cookie
    response = redirect(_get_dashboard_url())
    _set_session_cookie(response, session_id)

    logger.info(
        "Auth callback: login successful",
        extra={
            "user_id": auth_data.get("id"),
            "username": auth_data.get("username"),
        },
    )

    return response


@auth_bp.route("/logout")
def logout():
    """
    Clear session and redirect to login.
    """
    session_id = request.cookies.get("session_id")
    if session_id:
        _auth_service.logout(session_id)

    response = redirect(_get_login_url())
    response.delete_cookie("session_id")

    return response


def _get_dashboard_url() -> str:
    """Get URL for main dashboard."""
    # In production, Traefik handles the path prefix
    if _config.is_production():
        return _config.leaderboard_path + "/"
    return _config.leaderboard_path + "/"


def _get_login_url() -> str:
    """Get URL for login page."""
    if _config.is_production():
        return _config.leaderboard_path + "/login"
    return _config.leaderboard_path + "/login"


def _get_callback_url() -> str:
    """Get full URL for OAuth callback."""
    # The callback URL needs to be absolute for Telegram
    # In production, use the DOMAIN from headers or config
    # For now, use a relative path that Telegram will resolve
    if _config.is_production():
        return _config.leaderboard_path + "/auth/callback"
    return _config.leaderboard_path + "/auth/callback"


def _set_session_cookie(response, session_id: str) -> None:
    """Set session cookie on response."""
    response.set_cookie(
        "session_id",
        session_id,
        httponly=True,
        secure=_config.is_production(),
        samesite="Lax",
        max_age=7 * 24 * 60 * 60,  # 7 days
    )


def _render_login_page(layout) -> str:
    """Render login page HTML."""
    # Create a minimal HTML template with the login page
    html_template = """
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title>Beef Briefing Leaderboard - Login</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
        <style>
            body { margin: 0; padding: 0; min-height: 100vh; background-color: #f8f9fa; }
            @keyframes pulse {
                0% { transform: scale(1); }
                50% { transform: scale(1.15); }
                100% { transform: scale(1); }
            }
            .pulse-emoji {
                animation: pulse 2s ease-in-out infinite;
                display: inline-block;
            }
            .login-card {
                background: white;
                border-radius: 8px;
                box-shadow: 0 2px 8px rgba(0,0,0,0.1);
                padding: 48px;
                max-width: 400px;
                margin: 0 auto;
                text-align: center;
            }
            .login-title { margin-top: 16px; font-weight: 600; }
            .login-subtitle { color: #868e96; }
            .login-text { color: #868e96; font-size: 14px; margin-top: 16px; }
            #telegram-login-widget { margin-top: 24px; display: flex; justify-content: center; }
        </style>
    </head>
    <body>
        <div class="container-fluid d-flex align-items-center justify-content-center" style="min-height: 100vh;">
            <div class="login-card">
                <div class="pulse-emoji" style="font-size: 72px;">🥩</div>
                <h2 class="login-title">Beef Briefing</h2>
                <h4 class="login-subtitle">Leaderboard</h4>
                <p class="login-text">Sign in with Telegram to continue</p>
                <div id="telegram-login-widget"></div>
            </div>
        </div>
        {{ telegram_script | safe }}
    </body>
    </html>
    """

    telegram_script = f"""
    <script>
        (function() {{
            var container = document.getElementById('telegram-login-widget');
            if (container) {{
                var script = document.createElement('script');
                script.async = true;
                script.src = 'https://telegram.org/js/telegram-widget.js?22';
                script.setAttribute('data-telegram-login', '{_config.telegram_bot_username}');
                script.setAttribute('data-size', 'large');
                script.setAttribute('data-radius', '8');
                script.setAttribute('data-auth-url', '{_get_callback_url()}');
                script.setAttribute('data-request-access', 'write');
                container.appendChild(script);
            }}
        }})();
    </script>
    """

    return render_template_string(html_template, telegram_script=telegram_script)


def _render_error_page(message: str) -> str:
    """Render error page HTML."""
    html_template = """
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title>Beef Briefing Leaderboard - Error</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
        <style>
            body { margin: 0; padding: 0; min-height: 100vh; background-color: #f8f9fa; }
            .error-card {
                background: white;
                border-radius: 8px;
                box-shadow: 0 2px 8px rgba(0,0,0,0.1);
                padding: 48px;
                max-width: 400px;
                margin: 0 auto;
                text-align: center;
            }
        </style>
    </head>
    <body>
        <div class="container-fluid d-flex align-items-center justify-content-center" style="min-height: 100vh;">
            <div class="error-card">
                <div style="font-size: 72px;">😕</div>
                <h2 style="margin-top: 16px;">Oops!</h2>
                <p style="color: #868e96;">{{ message }}</p>
                <a href="{{ login_url }}" class="btn btn-primary mt-3">Try Again</a>
            </div>
        </div>
    </body>
    </html>
    """

    return render_template_string(html_template, message=message, login_url=_get_login_url())
