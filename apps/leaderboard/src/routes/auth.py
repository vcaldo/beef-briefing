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
    """Render login page HTML with theme support."""
    # Google Fonts for all themes
    google_fonts = (
        "https://fonts.googleapis.com/css2?"
        "family=Fraunces:opsz,wght@9..144,400;9..144,600;9..144,700"
        "&family=Atkinson+Hyperlegible:wght@400;700"
        "&family=Bebas+Neue"
        "&family=Source+Sans+3:wght@400;600"
        "&family=Righteous"
        "&family=DM+Sans:wght@400;500;600"
        "&display=swap"
    )

    html_template = """
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title>Beef Briefing Leaderboard - Login</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        <link rel="preconnect" href="https://fonts.googleapis.com">
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
        <link href="{{ google_fonts }}" rel="stylesheet">
        <style>
            /* Theme CSS Variables */
            :root, :root[data-theme="butcher-paper"] {
                --bg: #F5F0E8;
                --surface: #FFFDF8;
                --primary: #8B2635;
                --text: #2D2420;
                --muted: #7D7068;
                --border: #E5DED3;
                --font-heading: 'Fraunces', serif;
                --font-body: 'Atkinson Hyperlegible', sans-serif;
                --shadow: 0 2px 8px rgba(45, 36, 32, 0.1);
            }
            :root[data-theme="smokehouse"] {
                --bg: #1A1614;
                --surface: #252120;
                --primary: #E8A849;
                --text: #F0E6DC;
                --muted: #8B7D73;
                --border: #3D352F;
                --font-heading: 'Bebas Neue', sans-serif;
                --font-body: 'Source Sans 3', sans-serif;
                --shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
            }
            :root[data-theme="neon-diner"] {
                --bg: #0D0D12;
                --surface: #1A1A24;
                --primary: #FF2D6A;
                --text: #EEEEF0;
                --muted: #6B6B7B;
                --border: #2D2D3A;
                --font-heading: 'Righteous', cursive;
                --font-body: 'DM Sans', sans-serif;
                --shadow: 0 4px 24px rgba(255, 45, 106, 0.15);
            }

            * { box-sizing: border-box; }
            body {
                margin: 0;
                padding: 0;
                min-height: 100vh;
                background-color: var(--bg);
                color: var(--text);
                font-family: var(--font-body);
                display: flex;
                align-items: center;
                justify-content: center;
                transition: background-color 0.3s ease, color 0.3s ease;
            }
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
                background: var(--surface);
                border-radius: 12px;
                box-shadow: var(--shadow);
                border: 1px solid var(--border);
                padding: 48px;
                max-width: 400px;
                width: 90%;
                margin: 0 auto;
                text-align: center;
                transition: background-color 0.3s ease, border-color 0.3s ease;
            }
            .login-title {
                margin-top: 16px;
                font-weight: 600;
                font-family: var(--font-heading);
                color: var(--text);
                font-size: 1.75rem;
            }
            .login-subtitle {
                color: var(--muted);
                font-family: var(--font-heading);
                font-size: 1.25rem;
                margin-top: 4px;
            }
            .login-text {
                color: var(--muted);
                font-size: 14px;
                margin-top: 16px;
            }
            #telegram-login-widget {
                margin-top: 24px;
                display: flex;
                justify-content: center;
            }
        </style>
        <script>
            // Apply theme before page renders to prevent flash
            (function() {
                var stored = localStorage.getItem('beef-theme');
                if (stored) {
                    document.documentElement.setAttribute('data-theme', stored);
                } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
                    document.documentElement.setAttribute('data-theme', 'smokehouse');
                } else {
                    document.documentElement.setAttribute('data-theme', 'butcher-paper');
                }
            })();
        </script>
    </head>
    <body>
        <div class="login-card">
            <div class="pulse-emoji" style="font-size: 72px;">🥩</div>
            <h2 class="login-title">Beef Briefing</h2>
            <h4 class="login-subtitle">Leaderboard</h4>
            <p class="login-text">Sign in with Telegram to continue</p>
            <div id="telegram-login-widget"></div>
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

    return render_template_string(html_template, telegram_script=telegram_script, google_fonts=google_fonts)


def _render_error_page(message: str) -> str:
    """Render error page HTML with theme support."""
    # Google Fonts for all themes
    google_fonts = (
        "https://fonts.googleapis.com/css2?"
        "family=Fraunces:opsz,wght@9..144,400;9..144,600;9..144,700"
        "&family=Atkinson+Hyperlegible:wght@400;700"
        "&family=Bebas+Neue"
        "&family=Source+Sans+3:wght@400;600"
        "&family=Righteous"
        "&family=DM+Sans:wght@400;500;600"
        "&display=swap"
    )

    html_template = """
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title>Beef Briefing Leaderboard - Error</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        <link rel="preconnect" href="https://fonts.googleapis.com">
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
        <link href="{{ google_fonts }}" rel="stylesheet">
        <style>
            /* Theme CSS Variables */
            :root, :root[data-theme="butcher-paper"] {
                --bg: #F5F0E8;
                --surface: #FFFDF8;
                --primary: #8B2635;
                --text: #2D2420;
                --muted: #7D7068;
                --border: #E5DED3;
                --font-heading: 'Fraunces', serif;
                --font-body: 'Atkinson Hyperlegible', sans-serif;
                --shadow: 0 2px 8px rgba(45, 36, 32, 0.1);
            }
            :root[data-theme="smokehouse"] {
                --bg: #1A1614;
                --surface: #252120;
                --primary: #E8A849;
                --text: #F0E6DC;
                --muted: #8B7D73;
                --border: #3D352F;
                --font-heading: 'Bebas Neue', sans-serif;
                --font-body: 'Source Sans 3', sans-serif;
                --shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
            }
            :root[data-theme="neon-diner"] {
                --bg: #0D0D12;
                --surface: #1A1A24;
                --primary: #FF2D6A;
                --text: #EEEEF0;
                --muted: #6B6B7B;
                --border: #2D2D3A;
                --font-heading: 'Righteous', cursive;
                --font-body: 'DM Sans', sans-serif;
                --shadow: 0 4px 24px rgba(255, 45, 106, 0.15);
            }

            * { box-sizing: border-box; }
            body {
                margin: 0;
                padding: 0;
                min-height: 100vh;
                background-color: var(--bg);
                color: var(--text);
                font-family: var(--font-body);
                display: flex;
                align-items: center;
                justify-content: center;
            }
            .error-card {
                background: var(--surface);
                border-radius: 12px;
                box-shadow: var(--shadow);
                border: 1px solid var(--border);
                padding: 48px;
                max-width: 400px;
                width: 90%;
                margin: 0 auto;
                text-align: center;
            }
            .error-title {
                margin-top: 16px;
                font-family: var(--font-heading);
                color: var(--text);
                font-size: 1.5rem;
            }
            .error-message {
                color: var(--muted);
                margin-top: 8px;
            }
            .retry-btn {
                display: inline-block;
                margin-top: 24px;
                padding: 12px 24px;
                background-color: var(--primary);
                color: white;
                text-decoration: none;
                border-radius: 8px;
                font-weight: 500;
                transition: opacity 0.2s ease;
            }
            .retry-btn:hover {
                opacity: 0.9;
                color: white;
            }
        </style>
        <script>
            // Apply theme before page renders to prevent flash
            (function() {
                var stored = localStorage.getItem('beef-theme');
                if (stored) {
                    document.documentElement.setAttribute('data-theme', stored);
                } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
                    document.documentElement.setAttribute('data-theme', 'smokehouse');
                } else {
                    document.documentElement.setAttribute('data-theme', 'butcher-paper');
                }
            })();
        </script>
    </head>
    <body>
        <div class="error-card">
            <div style="font-size: 72px;">😕</div>
            <h2 class="error-title">Oops!</h2>
            <p class="error-message">{{ message }}</p>
            <a href="{{ login_url }}" class="retry-btn">Try Again</a>
        </div>
    </body>
    </html>
    """

    return render_template_string(html_template, message=message, login_url=_get_login_url(), google_fonts=google_fonts)
