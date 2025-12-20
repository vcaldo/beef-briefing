"""Formatting utility functions for the leaderboard application."""

from datetime import datetime, timezone


def format_number(n: int | float | None) -> str:
    """
    Format number with K/M suffix for large values.

    Args:
        n: Number to format (can be int, float, or None)

    Returns:
        Formatted string (e.g., "1.2K", "3.5M", "42")
    """
    if n is None:
        return "0"
    n = int(n)
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    elif n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)


def format_relative_time(dt: datetime | None) -> str:
    """
    Format datetime as relative time string.

    Args:
        dt: Datetime to format (can be None)

    Returns:
        Relative time string (e.g., "2d ago", "3h ago", "Just now")
    """
    if dt is None:
        return "Never"

    now = datetime.now(timezone.utc)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)

    diff = now - dt

    if diff.days > 365:
        years = diff.days // 365
        return f"{years}y ago"
    elif diff.days > 30:
        months = diff.days // 30
        return f"{months}mo ago"
    elif diff.days > 0:
        return f"{diff.days}d ago"
    elif diff.seconds > 3600:
        hours = diff.seconds // 3600
        return f"{hours}h ago"
    elif diff.seconds > 60:
        minutes = diff.seconds // 60
        return f"{minutes}m ago"
    else:
        return "Just now"
