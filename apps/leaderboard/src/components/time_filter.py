"""
Time filter component for group statistics pages.

A segmented control for selecting time periods:
24h, 7d, 30d, 90d, 180d, 365d, YTD, MAX
"""

from datetime import date, datetime, timedelta

import dash_mantine_components as dmc

# Component ID for callbacks
TIME_FILTER_ID = "time-filter"

# Period definitions
PERIODS = [
    {"value": "24h", "label": "24h", "days": 1},
    {"value": "7d", "label": "7d", "days": 7},
    {"value": "30d", "label": "30d", "days": 30},
    {"value": "90d", "label": "90d", "days": 90},
    {"value": "180d", "label": "180d", "days": 180},
    {"value": "365d", "label": "365d", "days": 365},
    {"value": "ytd", "label": "YTD", "days": None},  # Calculated dynamically
    {"value": "max", "label": "MAX", "days": None},  # All time
]

DEFAULT_PERIOD = "30d"


def get_period_dates(period: str) -> tuple[date | None, date | None]:
    """
    Get start and end dates for a period.

    Args:
        period: Period value (e.g., "7d", "30d", "ytd", "max")

    Returns:
        Tuple of (start_date, end_date) where None means no limit
    """
    today = date.today()
    end_date = today + timedelta(days=1)  # Include today

    if period == "max":
        return None, None

    if period == "ytd":
        start_date = date(today.year, 1, 1)
        return start_date, end_date

    # Find period definition
    for p in PERIODS:
        if p["value"] == period and p["days"] is not None:
            start_date = today - timedelta(days=p["days"])
            return start_date, end_date

    # Default to 30 days if unknown period
    return today - timedelta(days=30), end_date


def get_comparison_dates(
    period: str,
) -> tuple[date | None, date | None, date | None, date | None]:
    """
    Get current and previous period dates for trend comparison.

    Args:
        period: Period value (e.g., "7d", "30d")

    Returns:
        Tuple of (current_start, current_end, previous_start, previous_end)
        Returns (None, None, None, None) for MAX period (no comparison)
    """
    current_start, current_end = get_period_dates(period)

    if current_start is None:
        # MAX period - no comparison
        return None, None, None, None

    # Calculate previous period of same length
    period_length = (current_end - current_start).days
    previous_end = current_start
    previous_start = previous_end - timedelta(days=period_length)

    return current_start, current_end, previous_start, previous_end


def create_time_filter(current_period: str | None = None) -> dmc.SegmentedControl:
    """
    Create a time filter component.

    Args:
        current_period: Currently active period, or None for default

    Returns:
        DMC SegmentedControl component
    """
    if current_period is None:
        current_period = DEFAULT_PERIOD

    data = [{"value": p["value"], "label": p["label"]} for p in PERIODS]

    return dmc.SegmentedControl(
        id=TIME_FILTER_ID,
        value=current_period,
        data=data,
        size="xs",
        radius="md",
        transitionDuration=200,
    )


def create_time_filter_with_label(current_period: str | None = None) -> dmc.Group:
    """
    Create a time filter with a label.

    Args:
        current_period: Currently active period

    Returns:
        DMC Group containing label and filter
    """
    return dmc.Group(
        [
            dmc.Text("Period:", size="sm", c="dimmed"),
            create_time_filter(current_period),
        ],
        gap="xs",
    )


def format_period_label(period: str) -> str:
    """
    Get a human-readable label for a period.

    Args:
        period: Period value (e.g., "7d", "ytd")

    Returns:
        Human-readable string (e.g., "Last 7 days", "Year to date")
    """
    labels = {
        "24h": "Last 24 hours",
        "7d": "Last 7 days",
        "30d": "Last 30 days",
        "90d": "Last 90 days",
        "180d": "Last 6 months",
        "365d": "Last year",
        "ytd": "Year to date",
        "max": "All time",
    }
    return labels.get(period, period)
