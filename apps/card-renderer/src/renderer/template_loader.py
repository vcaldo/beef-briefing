"""Template loading and context transformation."""

import json
import logging
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
from typing import Any, Callable

from jinja2 import Environment, FileSystemLoader, select_autoescape

from ..themes import ThemeLoader

logger = logging.getLogger(__name__)

# Constants
MAX_BADGES = 8  # Maximum badges to display on card
TOP_RANK_COUNT = 3  # Number of top ranks for medals

# Stat keys in display order
STAT_KEYS = ["aura", "activity", "presence", "humor", "toxicity", "popularity"]


@dataclass
class TrendContext:
    """Trend information for a stat."""

    direction: str  # "up", "down", "stable"
    icon: str  # Arrow emoji
    pct_change: str  # "+15%" or "-8%"


@dataclass
class StatContext:
    """Normalized stat for template rendering."""

    key: str  # "aura", "activity", etc.
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

    key: str  # "radiant", "comedian"
    name: str  # "Radiant"
    icon: str  # Emoji
    rarity: str  # "common", "rare", "epic", "legendary", "negative"


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
    reactions_sent: int = 0
    replies_sent: int = 0


@dataclass
class CombatContext:
    """Combat stats for template."""

    atk: int
    def_: int  # Use def_ to avoid Python keyword conflict
    hp: int


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

    # Raw stats for direct access (new 6 metrics + overall)
    aura: dict = field(default_factory=dict)
    activity_stats: dict = field(default_factory=dict)
    presence: dict = field(default_factory=dict)
    humor: dict = field(default_factory=dict)
    toxicity: dict = field(default_factory=dict)
    popularity: dict = field(default_factory=dict)
    overall: dict = field(default_factory=dict)

    # Ranking
    rank: int | None = None

    # Combat stats
    combat: CombatContext | None = None

    # Theme
    theme: str = "gaming"


# Badge derivation rules - new 6 metrics system
BADGE_RULES = [
    # Aura badges
    {
        "condition": lambda s: s.get("aura", {}).get("score", 0) >= 80,
        "badge": BadgeContext("radiant", "Radiant", "\U0001F31F", "legendary"),
    },
    {
        "condition": lambda s: s.get("aura", {}).get("score", 100) < 30,
        "badge": BadgeContext("gloomy", "Gloomy", "\U0001F327", "negative"),
    },
    # Presence badges
    {
        "condition": lambda s: s.get("presence", {}).get("score", 0) >= 80,
        "badge": BadgeContext("regular", "Regular", "\U0001F4C5", "epic"),
    },
    {
        "condition": lambda s: (
            s.get("presence", {}).get("score", 100) < 20
            and s.get("activity", {}).get("messages", 0) >= 10
        ),
        "badge": BadgeContext("tourist", "Tourist", "\U0001F30D", "negative"),
    },
    # Humor badges
    {
        "condition": lambda s: s.get("humor", {}).get("score", 0) >= 70,
        "badge": BadgeContext("comedian", "Comedian", "\U0001F3AD", "legendary"),
    },
    {
        "condition": lambda s: s.get("humor", {}).get("score", 100) < 10,
        "badge": BadgeContext("deadpan", "Deadpan", "\U0001F610", "negative"),
    },
    # Toxicity badges
    {
        "condition": lambda s: s.get("toxicity", {}).get("pct", 100) < 2,
        "badge": BadgeContext("zen", "Zen", "\u262F\uFE0F", "legendary"),
    },
    {
        "condition": lambda s: s.get("toxicity", {}).get("pct", 0) > 20,
        "badge": BadgeContext("toxic", "Toxic", "\u2620\uFE0F", "negative"),
    },
    # Activity badges (based on message count thresholds)
    {
        "condition": lambda s: s.get("activity", {}).get("messages", 0) >= 500,
        "badge": BadgeContext("hyperactive", "Hyperactive", "\U0001F525", "legendary"),
    },
    {
        "condition": lambda s: s.get("activity", {}).get("messages", 0) < 20,
        "badge": BadgeContext("ghost", "Ghost", "\U0001F47B", "negative"),
    },
    # Popularity badges (based on unique reactors/repliers)
    {
        "condition": lambda s: s.get("popularity", {}).get("unique_reactors", 0) >= 10,
        "badge": BadgeContext("star", "Star", "\u2B50", "legendary"),
    },
    {
        "condition": lambda s: (
            s.get("popularity", {}).get("total_reactions", 0) == 0
            and s.get("activity", {}).get("messages", 0) >= 30
        ),
        "badge": BadgeContext("cricket", "Cricket", "\U0001F997", "negative"),
    },
    # Streak badge (dynamic name based on actual streak count)
    {
        "condition": lambda s: s.get("presence", {}).get("streak", 0) >= 7,
        "badge_factory": lambda s: BadgeContext(
            "streak",
            f"{s['presence']['streak']}-day streak",
            "\U0001F4C8",  # 📈
            "common",
        ),
    },
]

# Stat display configuration - new 6 metrics
STAT_CONFIG = {
    "aura": {
        "label": "Aura",
        "icon": "\U0001F31F",  # 🌟
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
    "activity": {
        "label": "Activity",
        "icon": "\U0001F525",  # 🔥
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
    "presence": {
        "label": "Presence",
        "icon": "\U0001F4C5",  # 📅
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
    "humor": {
        "label": "Humor",
        "icon": "\U0001F3AD",  # 🎭
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
    "toxicity": {
        "label": "Toxicity",
        "icon": "\u2620\uFE0F",  # ☠️
        "format": lambda v: f"{v:.1f}%",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "pct",
    },
    "popularity": {
        "label": "Popularity",
        "icon": "\u2B50",  # ⭐
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
    "overall": {
        "label": "Overall",
        "icon": "\U0001F3C6",  # 🏆
        "format": lambda v: f"{v:.0f}",
        "to_pct": lambda v: min(100, max(0, v)),
        "value_key": "score",
    },
}


class TemplateLoader:
    """Loads and renders card templates with Jinja2."""

    def __init__(
        self,
        templates_dir: str,
        tier_class_fn: Callable[[str], str] | None = None,
    ):
        """
        Initialize template loader.

        Args:
            templates_dir: Path to templates directory
            tier_class_fn: Optional function to map tier label to CSS class.
                          If not provided, uses label.lower() as class name.
        """
        self.templates_dir = Path(templates_dir)
        self.env = Environment(
            loader=FileSystemLoader(templates_dir),
            autoescape=select_autoescape(["html", "xml"]),
        )
        self.theme_loader = ThemeLoader(templates_dir)
        self._tier_class_fn = tier_class_fn or (lambda label: label.lower())

    def get_template_path(self, theme: str) -> str:
        """Get template path for a theme."""
        return f"themes/{theme}/card.html"

    def get_compact_template_path(self, theme: str) -> str:
        """Get compact template path for a theme."""
        return f"themes/{theme}/compact_card.html"

    def compact_template_exists(self, theme: str) -> bool:
        """Check if a compact template exists for a theme."""
        template_path = self.templates_dir / "themes" / theme / "compact_card.html"
        return template_path.exists()

    def theme_exists(self, theme: str) -> bool:
        """Check if a theme template exists."""
        template_path = self.templates_dir / "themes" / theme / "card.html"
        return template_path.exists()

    def _build_stat_context(
        self,
        stat_key: str,
        stat_data: dict,
        category_rankings: dict[str, dict[int, int]],
        user_id: int,
        trends_raw: dict,
    ) -> StatContext:
        """Build a StatContext for a single stat."""
        config = STAT_CONFIG[stat_key]
        value_key = config["value_key"]
        value = stat_data.get(value_key, 0) if stat_data else 0

        # Get category rank info
        cat_ranks = category_rankings.get(stat_key, {})
        cat_rank = cat_ranks.get(user_id)
        medal = RANK_MEDALS.get(cat_rank, "") if cat_rank else ""

        return StatContext(
            key=stat_key,
            label=config["label"],
            icon=config["icon"],
            value=value,
            percentage=config["to_pct"](value),
            display_value=config["format"](value),
            trend=self._make_trend(trends_raw.get(stat_key)),
            category_rank=cat_rank,
            category_rank_medal=medal,
        )

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

        # Extract raw stats for the 6 metrics + overall
        aura = stats_raw.get("aura", {})
        activity = stats_raw.get("activity", {})
        presence = stats_raw.get("presence", {})
        humor = stats_raw.get("humor", {})
        toxicity = stats_raw.get("toxicity", {})
        popularity = stats_raw.get("popularity", {})
        overall = stats_raw.get("overall", {})

        # Add tier_class for CSS styling (maps tier label to position-based class)
        tier_label = overall.get("label", "")
        overall["tier_class"] = self._tier_class_fn(tier_label) if tier_label else "tier-6"

        # Extract combat stats
        combat_raw = stats_raw.get("combat", {})
        combat = (
            CombatContext(
                atk=combat_raw.get("atk", 1),
                def_=combat_raw.get("def", 1),
                hp=combat_raw.get("hp", 3),
            )
            if combat_raw
            else None
        )

        # Build stats list using refactored method
        stats_list = [
            self._build_stat_context(
                stat_key,
                stats_raw.get(stat_key, {}),
                category_rankings,
                user_id,
                trends_raw,
            )
            for stat_key in STAT_KEYS
        ]

        # Derive badges
        badges = self._derive_badges(stats_raw)

        # Activity context
        activity_ctx = ActivityContext(
            messages=activity.get("messages", 0),
            active_days=presence.get("active_days", 0),
            avg_length=activity.get("avg_length", 0),
            reactions_sent=activity.get("reactions_sent", 0),
            replies_sent=activity.get("replies_sent", 0),
        )

        return TemplateContext(
            user=user,
            week_start=week_start.isoformat(),
            week_end=week_end.isoformat(),
            week_number=week_number,
            period_display=period_display,
            stats=stats_list,
            badges=badges[:MAX_BADGES],
            activity=activity_ctx,
            aura=aura,
            activity_stats=activity,
            presence=presence,
            humor=humor,
            toxicity=toxicity,
            popularity=popularity,
            overall=overall,
            rank=rank,
            combat=combat,
            theme=theme,
        )

    def _make_trend(self, trend_data: dict | None) -> TrendContext | None:
        """Convert trend data to TrendContext."""
        if not trend_data:
            return None

        direction = trend_data.get("direction", "stable")
        pct = trend_data.get("pct_change", 0)

        # Round first to check if it's effectively zero
        rounded_pct = round(pct)

        if rounded_pct == 0:
            # Neutral: no sign, no color
            direction = "stable"
            icon = "\u23F9\uFE0F"  # ⏹️
            pct_str = "0%"
        elif direction == "up":
            icon = "\u2B06\uFE0F"
            pct_str = f"+{rounded_pct}%"
        elif direction == "down":
            icon = "\u2B07\uFE0F"
            pct_str = f"{rounded_pct}%"
        else:
            icon = "\u23F9\uFE0F"  # ⏹️
            pct_str = "0%"

        return TrendContext(direction=direction, icon=icon, pct_change=pct_str)

    def _derive_badges(self, stats: dict) -> list[BadgeContext]:
        """Derive badges from stats using badge rules."""
        badges = []
        for rule in BADGE_RULES:
            try:
                if rule["condition"](stats):
                    # Support both static badge and dynamic badge_factory
                    if "badge_factory" in rule:
                        badges.append(rule["badge_factory"](stats))
                    else:
                        badges.append(rule["badge"])
            except (KeyError, TypeError):
                continue
        return badges

    def render(self, theme: str, context: TemplateContext, card_type: str = "regular") -> str:
        """
        Render a card template to HTML.

        Args:
            theme: Template theme name
            context: TemplateContext with all data
            card_type: Card type ('regular' or 'compact')

        Returns:
            Rendered HTML string
        """
        if card_type == "compact":
            if not self.compact_template_exists(theme):
                raise ValueError(f"Compact template not found for theme: {theme}")
            template_path = self.get_compact_template_path(theme)
        else:
            template_path = self.get_template_path(theme)

        template = self.env.get_template(template_path)

        # Load theme configuration
        theme_config = self.theme_loader.load_theme(theme)
        theme_context = theme_config.to_template_context()

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
            "aura": context.aura,
            "activity_stats": context.activity_stats,
            "presence": context.presence,
            "humor": context.humor,
            "toxicity": context.toxicity,
            "popularity": context.popularity,
            "overall": context.overall,
            "rank": context.rank,
            "combat": context.combat,
            "theme": theme_context,  # Inject theme configuration
        }

        return template.render(**ctx_dict)
