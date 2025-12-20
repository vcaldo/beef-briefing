"""Template loading and context transformation."""

import json
import logging
from dataclasses import dataclass, field
from datetime import date, datetime
from pathlib import Path
from typing import Any

from jinja2 import Environment, FileSystemLoader, select_autoescape

logger = logging.getLogger(__name__)


@dataclass
class TrendContext:
    """Trend information for a stat."""

    direction: str  # "up", "down", "stable"
    icon: str  # Arrow emoji
    pct_change: str  # "+15%" or "-8%"


@dataclass
class StatContext:
    """Normalized stat for template rendering."""

    key: str  # "mood", "comedy", etc.
    label: str  # Display label
    icon: str  # Emoji or icon class
    value: float  # Raw value
    percentage: float  # 0-100 for progress bars
    display_value: str  # Formatted for display
    trend: TrendContext | None = None
    category_rank: int | None = None  # 1, 2, or 3 if top 3 in category
    category_rank_medal: str = ""  # Medal emoji if top 3


# Medal emoji mapping for top 3 ranks
RANK_MEDALS = {
    1: "\U0001F947",  # 🥇
    2: "\U0001F948",  # 🥈
    3: "\U0001F949",  # 🥉
}


@dataclass
class BadgeContext:
    """Badge derived from stats."""

    key: str  # "night_owl", "comedian"
    name: str  # "Night Owl"
    icon: str  # Emoji
    rarity: str  # "common", "rare", "epic", "legendary"


@dataclass
class UserContext:
    """User information for template."""

    user_id: int
    first_name: str
    last_name: str
    username: str
    photo_url: str | None = None
    initials: str = ""

    def __post_init__(self):
        if not self.initials:
            first = self.first_name[0].upper() if self.first_name else ""
            last = self.last_name[0].upper() if self.last_name else ""
            self.initials = first + last or "?"


@dataclass
class ActivityContext:
    """Activity summary for template."""

    messages: int
    active_days: int
    avg_length: float


@dataclass
class TemplateContext:
    """Full context for card template rendering."""

    # User info
    user: UserContext

    # Week info
    week_start: str
    week_end: str
    week_number: int
    period_display: str

    # Stats (normalized for template)
    stats: list[StatContext]

    # Derived badges
    badges: list[BadgeContext]

    # Activity summary
    activity: ActivityContext

    # Raw stats for direct access
    mood: dict = field(default_factory=dict)
    comedy: dict = field(default_factory=dict)
    volatility: dict = field(default_factory=dict)
    toxicity: dict = field(default_factory=dict)
    chronotype: dict = field(default_factory=dict)
    reactions_received: int = 0

    # Ranking
    rank: int | None = None

    # Theme
    theme: str = "gaming"


# Badge derivation rules
BADGE_RULES = [
    # Chronotype badges
    {
        "condition": lambda s: s.get("chronotype", {}).get("type") == "Coruja",
        "badge": BadgeContext("night_owl", "Night Owl", "\U0001F989", "rare"),
    },
    {
        "condition": lambda s: s.get("chronotype", {}).get("type") == "Madrugador",
        "badge": BadgeContext("early_bird", "Early Bird", "\U0001F305", "rare"),
    },
    # Mood badges
    {
        "condition": lambda s: s.get("mood", {}).get("score", 0) >= 90,
        "badge": BadgeContext("sunshine", "Ray of Sunshine", "\u2B50", "legendary"),
    },
    {
        "condition": lambda s: s.get("mood", {}).get("score", 0) >= 75,
        "badge": BadgeContext("optimist", "Optimist", "\U0001F60A", "epic"),
    },
    # Comedy badges
    {
        "condition": lambda s: s.get("comedy", {}).get("score", 0) >= 0.7,
        "badge": BadgeContext("comedian", "Stand-Up King", "\U0001F3AD", "legendary"),
    },
    {
        "condition": lambda s: s.get("comedy", {}).get("score", 0) >= 0.5,
        "badge": BadgeContext("funny", "Class Clown", "\U0001F602", "epic"),
    },
    # Activity badges
    {
        "condition": lambda s: s.get("activity", {}).get("messages", 0) >= 500,
        "badge": BadgeContext("chatterbox", "Chatterbox", "\U0001F4AC", "legendary"),
    },
    {
        "condition": lambda s: s.get("activity", {}).get("messages", 0) >= 200,
        "badge": BadgeContext("active", "Active Voice", "\U0001F525", "epic"),
    },
    {
        "condition": lambda s: s.get("activity", {}).get("messages", 0) >= 100,
        "badge": BadgeContext("regular", "Regular", "\U0001F4DD", "rare"),
    },
    # Toxicity badges (inverted - low toxicity is good)
    {
        "condition": lambda s: s.get("toxicity", {}).get("pct", 100) < 1,
        "badge": BadgeContext("zen_master", "Zen Master", "\u262F\uFE0F", "legendary"),
    },
    {
        "condition": lambda s: s.get("toxicity", {}).get("pct", 100) < 5,
        "badge": BadgeContext("peaceful", "Peacekeeper", "\U0001F54A\uFE0F", "epic"),
    },
    # Reactions badges
    {
        "condition": lambda s: s.get("reactions_received", 0) >= 100,
        "badge": BadgeContext("beloved", "Beloved", "\u2764\uFE0F", "legendary"),
    },
    {
        "condition": lambda s: s.get("reactions_received", 0) >= 50,
        "badge": BadgeContext("popular", "Popular", "\U0001F44D", "epic"),
    },
]

# Stat display configuration
STAT_CONFIG = {
    "mood": {
        "label": "Mood",
        "icon": "\U0001F60A",
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
    },
    "comedy": {
        "label": "Comedy",
        "icon": "\U0001F3AD",
        "format": lambda v: f"{v * 100:.0f}%",
        "to_pct": lambda v: min(100, max(0, v * 100)),
    },
    "volatility": {
        "label": "Volatility",
        "icon": "\U0001F4C8",
        "format": lambda v: f"{v * 100:.0f}%",
        "to_pct": lambda v: min(100, max(0, v * 100)),
    },
    "toxicity": {
        "label": "Toxicity",
        "icon": "\u2620\uFE0F",
        "format": lambda v: f"{v:.1f}%",
        "to_pct": lambda v: min(100, max(0, v)),
    },
    "activity": {
        "label": "Activity",
        "icon": "\U0001F525",
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, min(v, 500) / 5)),  # 500 msgs = 100%
    },
    "reactions": {
        "label": "Reactions",
        "icon": "\u2764\uFE0F",
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, min(v, 100))),  # 100 reactions = 100%
    },
}


class TemplateLoader:
    """Loads and renders card templates with Jinja2."""

    def __init__(self, templates_dir: str):
        self.templates_dir = Path(templates_dir)
        self.env = Environment(
            loader=FileSystemLoader(templates_dir),
            autoescape=select_autoescape(["html", "xml"]),
        )

    def get_template_path(self, theme: str) -> str:
        """Get template path for a theme."""
        return f"themes/{theme}/card.html"

    def theme_exists(self, theme: str) -> bool:
        """Check if a theme template exists."""
        template_path = self.templates_dir / "themes" / theme / "card.html"
        return template_path.exists()

    def transform_card_data(
        self,
        card_data: dict[str, Any],
        theme: str = "gaming",
        rank: int | None = None,
        category_rankings: dict[str, dict[int, int]] | None = None,
    ) -> TemplateContext:
        """
        Transform raw card data from database into template context.

        Args:
            card_data: Row from ml_user_cards with user join
            theme: Template theme name
            rank: Optional ranking position
            category_rankings: Optional dict of category -> {user_id: rank} for top 3

        Returns:
            TemplateContext ready for template rendering
        """
        category_rankings = category_rankings or {}
        user_id = card_data["user_id"]
        stats_raw = card_data.get("stats") or {}
        if isinstance(stats_raw, str):
            stats_raw = json.loads(stats_raw)

        trends_raw = card_data.get("trends") or {}
        if isinstance(trends_raw, str):
            trends_raw = json.loads(trends_raw)

        # Build user context
        user = UserContext(
            user_id=card_data["user_id"],
            first_name=card_data.get("first_name", ""),
            last_name=card_data.get("last_name", ""),
            username=card_data.get("username", ""),
            photo_url=card_data.get("profile_photo_path"),
        )

        # Parse week dates
        week_start = card_data["week_start"]
        week_end = card_data["week_end"]

        if isinstance(week_start, str):
            week_start = date.fromisoformat(week_start)
        if isinstance(week_end, str):
            week_end = date.fromisoformat(week_end)

        week_number = week_start.isocalendar()[1]

        # Parse stats window dates (30-day period)
        stats_window_start = card_data.get("stats_window_start", week_start)
        stats_window_end = card_data.get("stats_window_end", week_end)

        if isinstance(stats_window_start, str):
            stats_window_start = date.fromisoformat(stats_window_start)
        if isinstance(stats_window_end, str):
            stats_window_end = date.fromisoformat(stats_window_end)

        # Display the 30-day stats window period
        period_display = f"{stats_window_start.strftime('%b %d')} - {stats_window_end.strftime('%b %d')}"

        # Extract and normalize stats
        stats_list = []

        def get_category_rank_info(category: str) -> tuple[int | None, str]:
            """Get rank and medal for a category."""
            cat_ranks = category_rankings.get(category, {})
            cat_rank = cat_ranks.get(user_id)
            medal = RANK_MEDALS.get(cat_rank, "") if cat_rank else ""
            return cat_rank, medal

        # Mood - always render
        mood = stats_raw.get("mood", {})
        score = mood.get("score", 0) if mood else 0
        config = STAT_CONFIG["mood"]
        cat_rank, medal = get_category_rank_info("mood")
        stats_list.append(
            StatContext(
                key="mood",
                label=config["label"],
                icon=config["icon"],
                value=score,
                percentage=config["to_pct"](score),
                display_value=config["format"](score),
                trend=self._make_trend(trends_raw.get("mood")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Comedy - always render
        comedy = stats_raw.get("comedy", {})
        score = comedy.get("score", 0) if comedy else 0
        config = STAT_CONFIG["comedy"]
        cat_rank, medal = get_category_rank_info("comedy")
        stats_list.append(
            StatContext(
                key="comedy",
                label=config["label"],
                icon=config["icon"],
                value=score,
                percentage=config["to_pct"](score),
                display_value=config["format"](score),
                trend=self._make_trend(trends_raw.get("comedy")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Volatility - always render
        volatility = stats_raw.get("volatility", {})
        score = volatility.get("score", 0) if volatility else 0
        config = STAT_CONFIG["volatility"]
        cat_rank, medal = get_category_rank_info("volatility")
        stats_list.append(
            StatContext(
                key="volatility",
                label=config["label"],
                icon=config["icon"],
                value=score,
                percentage=config["to_pct"](score),
                display_value=config["format"](score),
                trend=self._make_trend(trends_raw.get("volatility")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Toxicity - always render
        toxicity = stats_raw.get("toxicity", {})
        pct = toxicity.get("pct", 0) if toxicity else 0
        config = STAT_CONFIG["toxicity"]
        cat_rank, medal = get_category_rank_info("toxicity")
        stats_list.append(
            StatContext(
                key="toxicity",
                label=config["label"],
                icon=config["icon"],
                value=pct,
                percentage=config["to_pct"](pct),
                display_value=config["format"](pct),
                trend=self._make_trend(trends_raw.get("toxicity")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Activity - always render
        activity = stats_raw.get("activity", {})
        messages = activity.get("messages", 0) if activity else 0
        config = STAT_CONFIG["activity"]
        cat_rank, medal = get_category_rank_info("activity")
        stats_list.append(
            StatContext(
                key="activity",
                label=config["label"],
                icon=config["icon"],
                value=messages,
                percentage=config["to_pct"](messages),
                display_value=config["format"](messages),
                trend=self._make_trend(trends_raw.get("activity")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Reactions - always render
        reactions = stats_raw.get("reactions_received", 0)
        config = STAT_CONFIG["reactions"]
        cat_rank, medal = get_category_rank_info("reactions")
        stats_list.append(
            StatContext(
                key="reactions",
                label=config["label"],
                icon=config["icon"],
                value=reactions,
                percentage=config["to_pct"](reactions),
                display_value=config["format"](reactions),
                trend=self._make_trend(trends_raw.get("reactions")),
                category_rank=cat_rank,
                category_rank_medal=medal,
            )
        )

        # Derive badges
        badges = self._derive_badges(stats_raw)

        # Activity context
        activity_ctx = ActivityContext(
            messages=activity.get("messages", 0),
            active_days=activity.get("active_days", 0),
            avg_length=activity.get("avg_length", 0),
        )

        return TemplateContext(
            user=user,
            week_start=week_start.isoformat(),
            week_end=week_end.isoformat(),
            week_number=week_number,
            period_display=period_display,
            stats=stats_list,
            badges=badges[:8],  # Limit to 8 badges
            activity=activity_ctx,
            mood=mood,
            comedy=comedy,
            volatility=volatility,
            toxicity=toxicity,
            chronotype=stats_raw.get("chronotype", {}),
            reactions_received=reactions,
            rank=rank,
            theme=theme,
        )

    def _make_trend(self, trend_data: dict | None) -> TrendContext | None:
        """Convert trend data to TrendContext."""
        if not trend_data:
            return None

        direction = trend_data.get("direction", "stable")
        pct = trend_data.get("pct_change", 0)

        if direction == "up":
            icon = "\u2B06\uFE0F"
            pct_str = f"+{pct:.0f}%"
        elif direction == "down":
            icon = "\u2B07\uFE0F"
            pct_str = f"{pct:.0f}%"
        else:
            icon = "\u27A1\uFE0F"
            pct_str = "0%"

        return TrendContext(direction=direction, icon=icon, pct_change=pct_str)

    def _derive_badges(self, stats: dict) -> list[BadgeContext]:
        """Derive badges from stats using badge rules."""
        badges = []
        for rule in BADGE_RULES:
            try:
                if rule["condition"](stats):
                    badges.append(rule["badge"])
            except (KeyError, TypeError):
                continue
        return badges

    def render(self, theme: str, context: TemplateContext) -> str:
        """
        Render a card template to HTML.

        Args:
            theme: Template theme name
            context: TemplateContext with all data

        Returns:
            Rendered HTML string
        """
        template_path = self.get_template_path(theme)
        template = self.env.get_template(template_path)

        # Convert dataclass to dict for Jinja2
        ctx_dict = {
            "user": context.user,
            "week_start": context.week_start,
            "week_end": context.week_end,
            "week_number": context.week_number,
            "period_display": context.period_display,
            "stats": context.stats,
            "badges": context.badges,
            "activity": context.activity,
            "mood": context.mood,
            "comedy": context.comedy,
            "volatility": context.volatility,
            "toxicity": context.toxicity,
            "chronotype": context.chronotype,
            "reactions_received": context.reactions_received,
            "rank": context.rank,
            "theme": context.theme,
        }

        return template.render(**ctx_dict)
