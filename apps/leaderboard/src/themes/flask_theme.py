"""
Flask theme utilities for server-rendered pages.

Provides shared CSS and HTML utilities for login/error pages
that are rendered directly by Flask (not Dash).
"""

from src.themes.base import GOOGLE_FONTS_URL

# Shared theme CSS variables for all Flask-rendered pages
THEME_CSS = """
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
"""

# Theme detection script to prevent flash of unstyled content
THEME_DETECTION_SCRIPT = """
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
"""

# Shared card styles
CARD_CSS = """
.page-card {
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
.page-title {
    margin-top: 16px;
    font-weight: 600;
    font-family: var(--font-heading);
    color: var(--text);
    font-size: 1.75rem;
}
.page-subtitle {
    color: var(--muted);
    font-family: var(--font-heading);
    font-size: 1.25rem;
    margin-top: 4px;
}
.page-text {
    color: var(--muted);
    font-size: 14px;
    margin-top: 16px;
}
.page-btn {
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
.page-btn:hover {
    opacity: 0.9;
    color: white;
}
"""

# Pulse animation for emoji
ANIMATION_CSS = """
@keyframes pulse {
    0% { transform: scale(1); }
    50% { transform: scale(1.15); }
    100% { transform: scale(1); }
}
.pulse-emoji {
    animation: pulse 2s ease-in-out infinite;
    display: inline-block;
}
"""


def get_html_head(title: str, extra_css: str = "") -> str:
    """
    Generate HTML head section with theme support.

    Args:
        title: Page title
        extra_css: Additional page-specific CSS

    Returns:
        HTML string for the head section
    """
    return f"""
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title>{title}</title>
        <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🥩</text></svg>">
        <link rel="preconnect" href="https://fonts.googleapis.com">
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
        <link href="{GOOGLE_FONTS_URL}" rel="stylesheet">
        <style>
            {THEME_CSS}
            {CARD_CSS}
            {ANIMATION_CSS}
            {extra_css}
        </style>
        {THEME_DETECTION_SCRIPT}
    </head>
    """


__all__ = [
    "THEME_CSS",
    "THEME_DETECTION_SCRIPT",
    "CARD_CSS",
    "ANIMATION_CSS",
    "get_html_head",
    "GOOGLE_FONTS_URL",
]
