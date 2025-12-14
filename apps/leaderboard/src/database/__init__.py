"""Database access layer for the leaderboard."""

from .queries import DashboardQueries
from .sessions import SessionQueries

__all__ = ["DashboardQueries", "SessionQueries"]
