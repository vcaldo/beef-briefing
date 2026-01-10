"""
Card generator for aggregating ML results into weekly user cards.

Design Principles:
1. Uses pluggable calculators from calculators.py
2. 30-day rolling window for stable personality traits
3. Weekly cards with trend comparisons
4. Idempotent upserts (safe to re-run)
"""

import json
import logging
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

from sqlalchemy import text
from sqlalchemy.engine import Engine

from src.cards.calculators import CALCULATORS, _clamp
from src.instrumentation import function_trace

logger = logging.getLogger(__name__)


# Badge rules for modifier calculation (copied from template_loader.py)
# Each badge contributes to overall score: legendary +2, epic +1, negative -2
BADGE_RULES = [
    # Aura badges
    {"condition": lambda s: s.get("aura", {}).get("score", 0) >= 80, "rarity": "legendary"},
    {"condition": lambda s: s.get("aura", {}).get("score", 100) < 30, "rarity": "negative"},
    # Presence badges
    {"condition": lambda s: s.get("presence", {}).get("score", 0) >= 80, "rarity": "epic"},
    {
        "condition": lambda s: (
            s.get("presence", {}).get("score", 100) < 20
            and s.get("activity", {}).get("messages", 0) >= 10
        ),
        "rarity": "negative",
    },
    # Humor badges
    {"condition": lambda s: s.get("humor", {}).get("score", 0) >= 70, "rarity": "legendary"},
    {"condition": lambda s: s.get("humor", {}).get("score", 100) < 10, "rarity": "negative"},
    # Toxicity badges
    {"condition": lambda s: s.get("toxicity", {}).get("pct", 100) < 2, "rarity": "legendary"},
    {"condition": lambda s: s.get("toxicity", {}).get("pct", 0) > 20, "rarity": "negative"},
    # Activity badges
    {"condition": lambda s: s.get("activity", {}).get("messages", 0) >= 500, "rarity": "legendary"},
    {"condition": lambda s: s.get("activity", {}).get("messages", 0) < 20, "rarity": "negative"},
    # Popularity badges
    {"condition": lambda s: s.get("popularity", {}).get("unique_reactors", 0) >= 10, "rarity": "legendary"},
    {
        "condition": lambda s: (
            s.get("popularity", {}).get("total_reactions", 0) == 0
            and s.get("activity", {}).get("messages", 0) >= 30
        ),
        "rarity": "negative",
    },
]

BADGE_MODIFIERS = {
    "legendary": 10,
    "epic": 7,
    "negative": -5,
    "rare": 7,
    "common": 3,
}


class CardGenerator:
    """
    Generates weekly user cards using aggregated ML results.

    Cards are generated for a specific week using a 30-day rolling window
    ending on the week's end date. Trends compare current week to previous week.
    """

    def __init__(
        self,
        engine: Engine,
        timezone: str,
        tiers: list[tuple[str, int]],
        window_days: int = 30,
    ):
        """
        Initialize the card generator.

        Args:
            engine: SQLAlchemy engine
            timezone: IANA timezone identifier (e.g., 'America/Sao_Paulo')
            tiers: List of (name, min_score) tuples for tier classification
            window_days: Rolling window size for stats (default: 30)
        """
        self._engine = engine
        self._timezone = timezone
        self._tz = ZoneInfo(timezone)
        self._tiers = tiers
        self._window_days = window_days

    def _get_week_bounds(self, week_start: datetime) -> tuple[datetime, datetime]:
        """Get week start (Monday 00:00) and end (Sunday 23:59:59) in configured timezone."""
        # If naive datetime, localize to configured timezone
        if week_start.tzinfo is None:
            week_start = week_start.replace(tzinfo=self._tz)

        # Ensure week_start is a Monday
        days_since_monday = week_start.weekday()
        if days_since_monday != 0:
            week_start = week_start - timedelta(days=days_since_monday)

        # Set to Monday 00:00:00 local time
        week_start = week_start.replace(hour=0, minute=0, second=0, microsecond=0)

        # Sunday 23:59:59 local time
        week_end = week_start + timedelta(days=6, hours=23, minutes=59, seconds=59)

        return week_start, week_end

    def _get_window_bounds(self, week_end: datetime) -> tuple[datetime, datetime]:
        """Get rolling window bounds ending on week_end."""
        window_end = week_end
        window_start = week_end - timedelta(days=self._window_days - 1)
        return window_start, window_end

    @function_trace(name="get_active_users", group="CardGenerator")
    def _get_active_users(
        self, chat_id: int, window_start: datetime, window_end: datetime
    ) -> list[int]:
        """Get users with messages in the time window."""
        query = """
            SELECT DISTINCT m.user_id
            FROM messages m
            WHERE m.chat_id = :chat_id
              AND m.date >= :window_start
              AND m.date <= :window_end
              AND m.user_id IS NOT NULL
        """
        with self._engine.connect() as conn:
            result = conn.execute(
                text(query),
                {
                    "chat_id": chat_id,
                    "window_start": window_start,
                    "window_end": window_end,
                },
            )
            return [row[0] for row in result.fetchall()]

    @function_trace(name="get_user_message_count", group="CardGenerator")
    def _get_user_message_count(
        self, user_id: int, chat_id: int, window_start: datetime, window_end: datetime
    ) -> int:
        """Get message count for a user in window."""
        query = """
            SELECT COUNT(*) as count
            FROM messages m
            WHERE m.user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.date >= :window_start
              AND m.date <= :window_end
        """
        with self._engine.connect() as conn:
            result = conn.execute(
                text(query),
                {
                    "user_id": user_id,
                    "chat_id": chat_id,
                    "window_start": window_start,
                    "window_end": window_end,
                },
            )
            row = result.fetchone()
            return row[0] if row else 0

    @function_trace(name="compute_stats", group="CardGenerator")
    def _compute_stats(
        self,
        user_id: int,
        chat_id: int,
        window_start: datetime,
        window_end: datetime,
    ) -> dict:
        """
        Compute all stats for a user using registered calculators.

        Two-phase computation:
        1. Compute all base metrics (vibe, activity, presence, humor, toxicity, popularity)
        2. Compute overall score using existing stats to avoid re-querying

        Stats that return None (due to insufficient data) are skipped.
        """
        stats = {}

        # Phase 1: Compute base metrics (everything except overall)
        for stat_name, calculator in CALCULATORS.items():
            if stat_name == "overall":
                continue  # Computed in phase 2

            try:
                result = calculator(
                    self._engine, user_id, chat_id, window_start, window_end,
                    timezone=self._timezone,
                )
                if result is not None:
                    stats[stat_name] = result.value
            except Exception as e:
                logger.warning(
                    f"Calculator {stat_name} failed for user {user_id}: {e}"
                )

        # Phase 2: Compute overall score with existing stats
        if "overall" in CALCULATORS and stats:
            try:
                result = CALCULATORS["overall"](
                    self._engine, user_id, chat_id, window_start, window_end,
                    timezone=self._timezone,
                    existing_stats=stats,
                    tiers=self._tiers,
                )
                if result is not None:
                    stats["overall"] = result.value
            except Exception as e:
                logger.warning(
                    f"Calculator overall failed for user {user_id}: {e}"
                )

        # Phase 3: Compute combat stats with existing stats
        if "combat" in CALCULATORS and stats:
            try:
                result = CALCULATORS["combat"](
                    self._engine, user_id, chat_id, window_start, window_end,
                    existing_stats=stats,
                )
                if result is not None:
                    stats["combat"] = result.value
            except Exception as e:
                logger.warning(
                    f"Calculator combat failed for user {user_id}: {e}"
                )

        return stats

    def _apply_trend_modifier(self, trends: dict | None) -> float:
        """
        Calculate trend modifier for overall score.

        For each metric:
        - Upward trend >10% change: +5 points
        - Upward trend ≤10% change: +3 points
        - Downward trend ≤10% change: -3 points
        - Downward trend >10% change: -5 points

        For toxicity, direction is inverted (decreasing = good).

        Returns:
            Total modifier value (can be positive or negative)
        """
        if not trends:
            return 0.0

        modifier = 0.0

        # Positive metrics: up = good, down = bad
        positive_metrics = ["popularity", "presence", "aura", "humor", "activity"]
        for metric in positive_metrics:
            if metric in trends:
                pct = abs(trends[metric].get("pct_change", 0))
                direction = trends[metric].get("direction", "stable")
                if direction == "up":
                    modifier += 5 if pct > 10 else 3
                elif direction == "down":
                    modifier -= 5 if pct > 10 else 3

        # Toxicity: down = good (decreasing toxicity), up = bad
        if "toxicity" in trends:
            pct = abs(trends["toxicity"].get("pct_change", 0))
            direction = trends["toxicity"].get("direction", "stable")
            if direction == "down":  # Decreasing toxicity is GOOD
                modifier += 5 if pct > 10 else 3
            elif direction == "up":  # Increasing toxicity is BAD
                modifier -= 5 if pct > 10 else 3

        return modifier

    def _apply_badge_modifier(self, stats: dict) -> float:
        """
        Calculate badge modifier for overall score.

        Sums modifiers for all earned badges:
        - Legendary: +5
        - Epic: +3
        - Negative: -5

        Returns:
            Total modifier value (can be positive or negative)
        """
        modifier = 0.0

        for rule in BADGE_RULES:
            try:
                if rule["condition"](stats):
                    rarity = rule["rarity"]
                    modifier += BADGE_MODIFIERS.get(rarity, 0)
            except (KeyError, TypeError):
                continue

        return modifier

    def _apply_overall_modifiers(
        self, stats: dict, trends: dict | None
    ) -> None:
        """
        Apply trend and badge modifiers to overall score in place.

        Modifies the stats dict's overall.score value with applied modifiers,
        clamped to 0-100 range.
        """
        if "overall" not in stats:
            return

        base_score = stats["overall"].get("score", 0)
        trend_modifier = self._apply_trend_modifier(trends)
        badge_modifier = self._apply_badge_modifier(stats)

        # Apply modifiers and clamp
        final_score = _clamp(base_score + trend_modifier + badge_modifier, 0, 100)

        # Update stats with final score and modifier info
        stats["overall"]["score"] = round(final_score, 1)
        stats["overall"]["trend_modifier"] = round(trend_modifier, 1)
        stats["overall"]["badge_modifier"] = round(badge_modifier, 1)

        # Update label based on new score
        from src.cards.calculators import _overall_label
        stats["overall"]["label"] = _overall_label(final_score, self._tiers)

    def _calc_pct_change(self, current: float, previous: float) -> float:
        """Calculate percentage change, handling division by zero."""
        if previous == 0:
            return 100.0 if current > 0 else 0.0
        return round((current - previous) / abs(previous) * 100, 1)

    @function_trace(name="compute_trends", group="CardGenerator")
    def _compute_trends(
        self,
        user_id: int,
        chat_id: int,
        current_stats: dict,
        prev_week_start: datetime,
    ) -> dict | None:
        """
        Compute trends by comparing current week to previous week.

        Computes trends for all 6 metrics: vibe, activity, presence,
        humor, toxicity, and popularity.
        """
        # Get previous week's card
        prev_card = self._get_previous_card(user_id, chat_id, prev_week_start)
        if not prev_card:
            return None

        prev_stats = prev_card.get("stats", {})
        trends = {}

        # Metrics with "score" field (0-100 scale)
        score_metrics = ["aura", "activity", "presence", "humor", "popularity"]
        for metric in score_metrics:
            if metric in current_stats and metric in prev_stats:
                curr_val = current_stats[metric].get("score", 0)
                prev_val = prev_stats[metric].get("score", 0)
                delta = round(curr_val - prev_val, 1)
                direction = "up" if delta > 0 else "down" if delta < 0 else "stable"
                pct_change = self._calc_pct_change(curr_val, prev_val)
                trends[metric] = {
                    "delta": delta,
                    "direction": direction,
                    "pct_change": pct_change,
                }

        # Toxicity uses "pct" field (percentage)
        if "toxicity" in current_stats and "toxicity" in prev_stats:
            curr_val = current_stats["toxicity"].get("pct", 0)
            prev_val = prev_stats["toxicity"].get("pct", 0)
            delta = round(curr_val - prev_val, 2)
            direction = "up" if delta > 0 else "down" if delta < 0 else "stable"
            pct_change = self._calc_pct_change(curr_val, prev_val)
            trends["toxicity"] = {
                "delta": delta,
                "direction": direction,
                "pct_change": pct_change,
            }

        return trends if trends else None

    def _get_previous_card(
        self, user_id: int, chat_id: int, week_start: datetime
    ) -> dict | None:
        """Get the card from the previous week."""
        query = """
            SELECT stats
            FROM ml_user_cards
            WHERE user_id = :user_id
              AND chat_id = :chat_id
              AND week_start = :week_start
        """
        with self._engine.connect() as conn:
            result = conn.execute(
                text(query),
                {"user_id": user_id, "chat_id": chat_id, "week_start": week_start.date()},
            )
            row = result.mappings().fetchone()
            if row:
                stats = row["stats"]
                if isinstance(stats, str):
                    stats = json.loads(stats)
                return {"stats": stats}
            return None

    @function_trace(name="upsert_card", group="CardGenerator")
    def _upsert_card(
        self,
        user_id: int,
        chat_id: int,
        week_start: datetime,
        week_end: datetime,
        stats_window_start: datetime,
        stats_window_end: datetime,
        stats: dict,
        trends: dict | None,
        messages_analyzed: int,
    ) -> bool:
        """Upsert a user card into the database."""
        query = """
            INSERT INTO ml_user_cards (
                user_id, chat_id, week_start, week_end,
                stats_window_start, stats_window_end,
                stats, trends, messages_analyzed,
                timezone, card_version, generated_at
            ) VALUES (
                :user_id, :chat_id, :week_start, :week_end,
                :stats_window_start, :stats_window_end,
                :stats, :trends, :messages_analyzed,
                :timezone, 1, NOW()
            )
            ON CONFLICT (user_id, chat_id, week_start)
            DO UPDATE SET
                week_end = EXCLUDED.week_end,
                stats_window_start = EXCLUDED.stats_window_start,
                stats_window_end = EXCLUDED.stats_window_end,
                stats = EXCLUDED.stats,
                trends = EXCLUDED.trends,
                messages_analyzed = EXCLUDED.messages_analyzed,
                timezone = EXCLUDED.timezone,
                card_version = ml_user_cards.card_version + 1,
                generated_at = NOW()
        """
        try:
            with self._engine.connect() as conn:
                conn.execute(
                    text(query),
                    {
                        "user_id": user_id,
                        "chat_id": chat_id,
                        "week_start": week_start.date(),
                        "week_end": week_end.date(),
                        "stats_window_start": stats_window_start.date(),
                        "stats_window_end": stats_window_end.date(),
                        "stats": json.dumps(stats),
                        "trends": json.dumps(trends) if trends else None,
                        "messages_analyzed": messages_analyzed,
                        "timezone": self._timezone,
                    },
                )
                conn.commit()
            return True
        except Exception as e:
            logger.error(f"Failed to upsert card for user {user_id}: {e}")
            return False

    @function_trace(name="generate_cards", group="CardGenerator")
    def generate_cards(
        self,
        chat_id: int,
        week_start: datetime | None = None,
        min_messages: int = 10,
        from_date: datetime | None = None,
        to_date: datetime | None = None,
    ) -> dict:
        """
        Generate cards for all active users in a chat.

        Args:
            chat_id: Target chat ID
            week_start: Week to generate cards for (default: current week)
            min_messages: Minimum messages required for card generation
            from_date: Explicit stats window start (overrides window_days)
            to_date: Explicit stats window end (overrides window_days)

        Returns:
            Dict with generation stats
        """
        # Calculate week bounds
        if week_start is None:
            # Use current week (Monday) in configured timezone
            today = datetime.now(self._tz)
            week_start = today - timedelta(days=today.weekday())

        week_start, week_end = self._get_week_bounds(week_start)

        # Use explicit dates if provided, otherwise calculate from window_days
        if from_date is not None and to_date is not None:
            window_start = from_date
            window_end = to_date
        elif from_date is not None:
            # from_date provided, use week_end as to_date
            window_start = from_date
            window_end = week_end
        elif to_date is not None:
            # to_date provided, calculate window_start from window_days
            window_end = to_date
            window_start = to_date - timedelta(days=self._window_days - 1)
        else:
            # Default: use window_days from week_end
            window_start, window_end = self._get_window_bounds(week_end)

        logger.info(f"Generating cards for chat {chat_id}")
        logger.info(f"Week: {week_start.date()} - {week_end.date()}")
        logger.info(f"Stats window: {window_start.date()} - {window_end.date()}")

        # Get active users
        active_users = self._get_active_users(chat_id, window_start, window_end)
        logger.info(f"Found {len(active_users)} active users")

        # Calculate previous week for trends
        prev_week_start = week_start - timedelta(days=7)

        # Generate cards
        generated = 0
        skipped = 0

        for user_id in active_users:
            # Check message count
            msg_count = self._get_user_message_count(
                user_id, chat_id, window_start, window_end
            )
            if msg_count < min_messages:
                logger.debug(
                    f"Skipping user {user_id}: only {msg_count} messages "
                    f"(min: {min_messages})"
                )
                skipped += 1
                continue

            # Compute stats
            stats = self._compute_stats(user_id, chat_id, window_start, window_end)
            if not stats:
                logger.debug(f"Skipping user {user_id}: no stats computed")
                skipped += 1
                continue

            # Compute trends
            trends = self._compute_trends(user_id, chat_id, stats, prev_week_start)

            # Apply modifiers to overall score (in place)
            self._apply_overall_modifiers(stats, trends)

            # Upsert card
            if self._upsert_card(
                user_id=user_id,
                chat_id=chat_id,
                week_start=week_start,
                week_end=week_end,
                stats_window_start=window_start,
                stats_window_end=window_end,
                stats=stats,
                trends=trends,
                messages_analyzed=msg_count,
            ):
                generated += 1
                logger.debug(f"Generated card for user {user_id}")
            else:
                skipped += 1

        logger.info(f"Card generation complete: {generated} generated, {skipped} skipped")

        return {
            "week_start": week_start.date().isoformat(),
            "week_end": week_end.date().isoformat(),
            "window_start": window_start.date().isoformat(),
            "window_end": window_end.date().isoformat(),
            "active_users": len(active_users),
            "generated": generated,
            "skipped": skipped,
        }
