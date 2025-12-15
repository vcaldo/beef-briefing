"""
Group navigation component for group statistics pages.

Provides a header with:
- Back button to landing page
- Group title
- Tab navigation (Overview, Activity, Reactions, Leaderboard, My Stats)
- Time filter
- Theme switcher

All navigation is URL-based using anchor links for server-side rendering.
"""

import dash_mantine_components as dmc
from dash_iconify import DashIconify

from src.components.theme_switcher import create_theme_switcher
from src.components.time_filter import create_time_filter_links

# Tab definitions
TABS = [
    {"value": "overview", "label": "Overview", "icon": "mdi:view-dashboard-outline"},
    {"value": "activity", "label": "Activity", "icon": "mdi:chart-timeline-variant"},
    {"value": "reactions", "label": "Reactions", "icon": "mdi:emoticon-outline"},
    {"value": "leaderboard", "label": "Leaderboard", "icon": "mdi:trophy-outline"},
    {"value": "my-stats", "label": "My Stats", "icon": "mdi:account-outline"},
]


def create_group_nav(
    chat_id: int,
    chat_title: str,
    current_tab: str,
    current_period: str,
    base_url: str,
    theme_name: str | None = None,
) -> dmc.Stack:
    """
    Create the group navigation header.

    Args:
        chat_id: Chat ID for URL generation
        chat_title: Display title of the group
        current_tab: Currently active tab (overview, activity, etc.)
        current_period: Currently selected time period
        base_url: Base URL path (e.g., "/leaderboard")
        theme_name: Current theme name

    Returns:
        DMC Stack containing the full navigation header
    """
    return dmc.Stack(
        [
            # Top row: back button, title, theme switcher
            dmc.Group(
                [
                    dmc.Anchor(
                        dmc.ActionIcon(
                            DashIconify(icon="mdi:arrow-left", width=20),
                            variant="subtle",
                            size="lg",
                        ),
                        href=base_url,
                    ),
                    dmc.Box(dmc.Title(chat_title, order=2), style={"flex": 1}),
                    create_theme_switcher(theme_name),
                ],
                justify="space-between",
                align="center",
            ),
            # Tab row with time filter
            dmc.Group(
                [
                    _create_tab_links(base_url, chat_id, current_tab, current_period),
                    create_time_filter_links(
                        base_url, chat_id, current_tab, current_period
                    ),
                ],
                justify="space-between",
                align="center",
                wrap="wrap",
                gap="md",
            ),
        ],
        gap="md",
    )


def _create_tab_links(
    base_url: str, chat_id: int, current_tab: str, period: str
) -> dmc.Group:
    """Create tab navigation as anchor links."""
    tabs = []
    for tab in TABS:
        is_active = tab["value"] == current_tab
        href = f"{base_url}/group/{chat_id}/{tab['value']}?period={period}"
        tabs.append(
            dmc.Anchor(
                dmc.Button(
                    dmc.Group(
                        [
                            DashIconify(icon=tab["icon"], width=16),
                            dmc.Text(tab["label"], size="sm"),
                        ],
                        gap="xs",
                    ),
                    variant="filled" if is_active else "subtle",
                    size="sm",
                ),
                href=href,
                underline="never",
            )
        )
    return dmc.Group(tabs, gap="xs")


def create_group_header_simple(
    chat_title: str,
    base_url: str,
    theme_name: str | None = None,
) -> dmc.Group:
    """
    Create a simple group header without tabs.

    Args:
        chat_title: Display title of the group
        base_url: Base URL path
        theme_name: Current theme name

    Returns:
        DMC Group with back button, title, and theme switcher
    """
    return dmc.Group(
        [
            dmc.Anchor(
                dmc.ActionIcon(
                    DashIconify(icon="mdi:arrow-left", width=20),
                    variant="subtle",
                    size="lg",
                ),
                href=base_url,
            ),
            dmc.Box(dmc.Title(chat_title, order=2), style={"flex": 1}),
            create_theme_switcher(theme_name),
        ],
        justify="space-between",
        align="center",
    )


def get_tab_url(base_url: str, chat_id: int, tab: str, period: str | None = None) -> str:
    """
    Generate URL for a group tab.

    Args:
        base_url: Base URL path
        chat_id: Chat ID
        tab: Tab name
        period: Optional period to include in query string

    Returns:
        Full URL path for the tab
    """
    url = f"{base_url}/group/{chat_id}/{tab}"
    if period:
        url += f"?period={period}"
    return url
