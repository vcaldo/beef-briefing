"""Reusable components for the leaderboard application."""

from .group_card import create_group_card
from .group_nav import (
    TABS,
    create_group_header_simple,
    create_group_nav,
    get_tab_url,
)
from .theme_switcher import (
    THEME_SWITCHER_ID,
    create_theme_switcher,
    create_theme_switcher_with_label,
)
from .time_filter import (
    DEFAULT_PERIOD,
    PERIODS,
    TIME_FILTER_ID,
    create_time_filter,
    create_time_filter_links,
    create_time_filter_with_label,
    format_period_label,
    get_comparison_dates,
    get_period_dates,
)

__all__ = [
    # Group card
    "create_group_card",
    # Group navigation
    "create_group_nav",
    "create_group_header_simple",
    "get_tab_url",
    "TABS",
    # Theme switcher
    "create_theme_switcher",
    "create_theme_switcher_with_label",
    "THEME_SWITCHER_ID",
    # Time filter
    "create_time_filter",
    "create_time_filter_links",
    "create_time_filter_with_label",
    "get_period_dates",
    "get_comparison_dates",
    "format_period_label",
    "TIME_FILTER_ID",
    "PERIODS",
    "DEFAULT_PERIOD",
]
