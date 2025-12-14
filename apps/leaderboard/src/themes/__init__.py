"""
Leaderboard Theme System

Three distinctive themes inspired by the Beef Briefing context:
- Butcher Paper (Light): Artisan butcher shop aesthetic
- Smokehouse (Dark): Texas BBQ joint aesthetic
- Neon Diner (Vibrant): 80s diner neon aesthetic
"""

from .base import (
    GOOGLE_FONTS_URL,
    THEME_BUTCHER_PAPER,
    THEME_NEON_DINER,
    THEME_SMOKEHOUSE,
    THEME_STORAGE_KEY,
)
from .butcher_paper import BACKGROUND_STYLE as BUTCHER_PAPER_BG
from .butcher_paper import COLORS as BUTCHER_PAPER_COLORS
from .butcher_paper import THEME as BUTCHER_PAPER_THEME
from .neon_diner import BACKGROUND_STYLE as NEON_DINER_BG
from .neon_diner import COLORS as NEON_DINER_COLORS
from .neon_diner import THEME as NEON_DINER_THEME
from .smokehouse import BACKGROUND_STYLE as SMOKEHOUSE_BG
from .smokehouse import COLORS as SMOKEHOUSE_COLORS
from .smokehouse import THEME as SMOKEHOUSE_THEME

# Theme registry
THEMES = {
    THEME_BUTCHER_PAPER: {
        "theme": BUTCHER_PAPER_THEME,
        "background": BUTCHER_PAPER_BG,
        "colors": BUTCHER_PAPER_COLORS,
        "label": "Butcher Paper",
        "icon": "tabler:sun",
    },
    THEME_SMOKEHOUSE: {
        "theme": SMOKEHOUSE_THEME,
        "background": SMOKEHOUSE_BG,
        "colors": SMOKEHOUSE_COLORS,
        "label": "Smokehouse",
        "icon": "tabler:moon",
    },
    THEME_NEON_DINER: {
        "theme": NEON_DINER_THEME,
        "background": NEON_DINER_BG,
        "colors": NEON_DINER_COLORS,
        "label": "Neon Diner",
        "icon": "tabler:bolt",
    },
}

# Default theme for OS preference fallback
DEFAULT_LIGHT_THEME = THEME_BUTCHER_PAPER
DEFAULT_DARK_THEME = THEME_SMOKEHOUSE


def get_theme(theme_name: str | None) -> dict:
    """
    Get theme configuration by name.

    Args:
        theme_name: Theme identifier or None for default

    Returns:
        Theme configuration dict with 'theme', 'background', 'colors' keys
    """
    if theme_name is None:
        theme_name = DEFAULT_LIGHT_THEME

    return THEMES.get(theme_name, THEMES[DEFAULT_LIGHT_THEME])


def get_theme_names() -> list[str]:
    """Get list of available theme names."""
    return list(THEMES.keys())


__all__ = [
    # Theme identifiers
    "THEME_BUTCHER_PAPER",
    "THEME_SMOKEHOUSE",
    "THEME_NEON_DINER",
    # Theme data
    "THEMES",
    "BUTCHER_PAPER_THEME",
    "SMOKEHOUSE_THEME",
    "NEON_DINER_THEME",
    "BUTCHER_PAPER_COLORS",
    "SMOKEHOUSE_COLORS",
    "NEON_DINER_COLORS",
    "BUTCHER_PAPER_BG",
    "SMOKEHOUSE_BG",
    "NEON_DINER_BG",
    # Helpers
    "get_theme",
    "get_theme_names",
    # Config
    "GOOGLE_FONTS_URL",
    "THEME_STORAGE_KEY",
    "DEFAULT_LIGHT_THEME",
    "DEFAULT_DARK_THEME",
]
