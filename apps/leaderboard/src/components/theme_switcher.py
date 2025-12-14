"""
Theme switcher component.

A three-way segmented control for switching between themes:
- Butcher Paper (Light) - Sun icon
- Smokehouse (Dark) - Moon icon
- Neon Diner (Vibrant) - Bolt icon
"""

import dash_mantine_components as dmc
from dash_iconify import DashIconify

from src.themes import (
    THEME_BUTCHER_PAPER,
    THEME_NEON_DINER,
    THEME_SMOKEHOUSE,
    THEMES,
)

# Component ID for callbacks
THEME_SWITCHER_ID = "theme-switcher"


def create_theme_switcher(current_theme: str | None = None) -> dmc.SegmentedControl:
    """
    Create a theme switcher component.

    Args:
        current_theme: Currently active theme name, or None for default

    Returns:
        DMC SegmentedControl component
    """
    if current_theme is None:
        current_theme = THEME_BUTCHER_PAPER

    data = []
    for theme_name in [THEME_BUTCHER_PAPER, THEME_SMOKEHOUSE, THEME_NEON_DINER]:
        theme_info = THEMES[theme_name]
        data.append(
            {
                "value": theme_name,
                "label": dmc.Center(
                    DashIconify(
                        icon=theme_info["icon"],
                        width=18,
                    ),
                    style={"width": "100%"},
                ),
            }
        )

    return dmc.SegmentedControl(
        id=THEME_SWITCHER_ID,
        value=current_theme,
        data=data,
        size="sm",
        radius="md",
        transitionDuration=200,
    )


def create_theme_switcher_with_label(
    current_theme: str | None = None,
) -> dmc.Group:
    """
    Create a theme switcher with a label.

    Args:
        current_theme: Currently active theme name

    Returns:
        DMC Group containing label and switcher
    """
    return dmc.Group(
        [
            dmc.Text("Theme:", size="sm", c="dimmed"),
            create_theme_switcher(current_theme),
        ],
        gap="xs",
    )
