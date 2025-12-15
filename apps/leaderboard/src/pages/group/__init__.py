"""
Group statistics pages for the leaderboard dashboard.

Pages:
- Overview: High-level snapshot with trend comparisons
- Activity: Time-based charts and patterns
- Reactions: Emoji analytics and engagement
- Leaderboard: User rankings with multiple metrics
- My Stats: Personal stats with group comparison
"""

from .activity import create_activity_page
from .leaderboard import create_leaderboard_page
from .my_stats import create_my_stats_page
from .overview import create_overview_page
from .reactions import create_reactions_page
from .sentiment import create_sentiment_page

__all__ = [
    "create_overview_page",
    "create_activity_page",
    "create_reactions_page",
    "create_leaderboard_page",
    "create_my_stats_page",
    "create_sentiment_page",
]
