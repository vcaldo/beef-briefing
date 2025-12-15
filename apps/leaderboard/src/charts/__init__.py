"""
Charts module for leaderboard dashboard.

Provides theme-aware chart utilities and custom chart components.
"""

from src.themes import THEMES, THEME_BUTCHER_PAPER


def get_chart_colors(theme_name: str | None) -> dict:
    """
    Get color palette for charts based on current theme.

    Args:
        theme_name: Theme identifier or None for default

    Returns:
        Dictionary with chart color values
    """
    if theme_name is None:
        theme_name = THEME_BUTCHER_PAPER

    theme_data = THEMES.get(theme_name, THEMES[THEME_BUTCHER_PAPER])
    colors = theme_data["colors"]

    return {
        "primary": colors["primary"],
        "accent": colors["accent"],
        "muted": colors["muted"],
        "text": colors["text"],
        "background": colors["background"],
        "surface": colors["surface"],
        "border": colors["border"],
    }


def get_series_colors(theme_name: str | None) -> list[str]:
    """
    Get ordered list of colors for multi-series charts.

    Args:
        theme_name: Theme identifier or None for default

    Returns:
        List of hex colors in order: primary, accent, muted
    """
    colors = get_chart_colors(theme_name)
    return [colors["primary"], colors["accent"], colors["muted"]]


def get_gradient_colors(theme_name: str | None, steps: int = 5) -> list[str]:
    """
    Get a gradient of colors from primary to accent for heatmaps.

    Args:
        theme_name: Theme identifier or None for default
        steps: Number of color steps in the gradient

    Returns:
        List of hex colors from low (muted) to high (primary)
    """
    colors = get_chart_colors(theme_name)

    # For heatmaps: surface (zero) -> muted (low) -> accent (mid) -> primary (high)
    if steps <= 2:
        return [colors["surface"], colors["primary"]]
    elif steps <= 3:
        return [colors["surface"], colors["accent"], colors["primary"]]
    else:
        # Interpolate between surface -> muted -> accent -> primary
        return [
            colors["surface"],
            colors["muted"],
            colors["accent"],
            colors["primary"],
        ]


__all__ = [
    "get_chart_colors",
    "get_series_colors",
    "get_gradient_colors",
]
