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

from src.cards.calculators import CALCULATORS
from src.instrumentation import function_trace

logger = logging.getLogger(__name__)


class CardGenerator:
    """
    Generates weekly user cards using aggregated ML results.

    Cards are generated for a specific week using a 30-day rolling window
    ending on the week's end date. Trends compare current week to previous week.
    """

    def __init__(self, engine: Engine, timezone: str, window_days: int = 30):
        """
        Initialize the card generator.

        Args:
            engine: SQLAlchemy engine
            timezone: IANA timezone identifier (e.g., 'America/Sao_Paulo')
            window_days: Rolling window size for stats (default: 30)
        """
        self._engine = engine
        self._timezone = timezone
        self._tz = ZoneInfo(timezone)
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

        Runs each calculator and collects results. Stats that return None
        (due to insufficient data) are skipped.
        """
        stats = {}

        for stat_name, calculator in CALCULATORS.items():
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

        return stats

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
        score_metrics = ["vibe", "activity", "presence", "humor", "popularity"]
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
