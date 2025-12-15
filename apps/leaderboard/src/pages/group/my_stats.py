"""
My Stats page for group statistics.

Personal stats with group comparison:
- Profile header (avatar, name, rank, member since)
- Stat comparisons (6 metrics vs group average)
- Personal timeline (line chart: user vs group average)
- Top reactions you send (horizontal bar chart)
- Reply stats (replies sent/received, top partners)
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates


def create_my_stats_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
) -> html.Div:
    """
    Create the personal stats page for the current user.

    Args:
        chat_id: Chat ID
        chat_info: Chat metadata
        user: Current user session data
        period: Selected time period
        base_url: Base URL path
        theme_name: Current theme name
        queries: DashboardQueries instance

    Returns:
        Page layout as html.Div
    """
    chat_title = chat_info.get("title", f"Chat {chat_id}")
    user_id = user.get("user_id")
    user_name = user.get("first_name", "User")
    colors = get_chart_colors(theme_name)

    # Get date range for period
    start_date, end_date = get_period_dates(period)

    # Fetch user stats
    user_stats = queries.get_user_stats(chat_id, user_id, start_date, end_date)
    if user_stats is None:
        user_stats = {
            "message_count": 0,
            "reactions_sent": 0,
            "reactions_received": 0,
            "active_days": 0,
        }

    # Fetch user rank (for messages)
    user_rank = queries.get_user_rank(
        chat_id, user_id, "message_count", start_date, end_date
    )

    # Fetch group averages
    group_avgs = queries.get_group_averages(chat_id, start_date, end_date)

    # Fetch first message date
    first_msg_date = queries.get_user_first_message_date(chat_id, user_id)
    member_since = first_msg_date.strftime("%b %d, %Y") if first_msg_date else "Unknown"

    # Fetch reply stats
    reply_stats = queries.get_user_reply_stats(
        chat_id, user_id, start_date=start_date, end_date=end_date
    )

    # Fetch user's reaction distribution
    user_reactions = queries.get_user_reaction_distribution(
        chat_id, user_id, limit=5, start_date=start_date, end_date=end_date
    )

    # Fetch user's daily activity
    user_activity_df = queries.get_user_daily_activity(
        chat_id, user_id, start_date, end_date
    )

    # Fetch group daily activity for comparison
    group_activity_df = queries.get_daily_activity(chat_id, start_date, end_date)

    # Total users for context
    total_users = group_avgs.get("total_users", 0)

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="my-stats",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
            ),
            dmc.Space(h="xl"),
            # Profile header
            _create_profile_header(
                user_name=user_name,
                photo_url=user.get("photo_url"),
                rank=user_rank,
                total_users=total_users,
                member_since=member_since,
                is_premium=user.get("is_premium", False),
            ),
            dmc.Space(h="xl"),
            # Stat comparisons
            dmc.Title("Your Stats vs Group Average", order=4),
            dmc.Space(h="md"),
            dmc.SimpleGrid(
                [
                    _create_comparison_card(
                        "Messages",
                        user_stats.get("message_count", 0),
                        group_avgs.get("avg_messages", 0),
                        colors,
                    ),
                    _create_comparison_card(
                        "Reactions Sent",
                        user_stats.get("reactions_sent", 0),
                        group_avgs.get("avg_reactions_sent", 0),
                        colors,
                    ),
                    _create_comparison_card(
                        "Reactions Received",
                        user_stats.get("reactions_received", 0),
                        group_avgs.get("avg_reactions_received", 0),
                        colors,
                    ),
                    _create_comparison_card(
                        "Replies Sent",
                        reply_stats.get("replies_sent", 0),
                        None,  # No group average for replies yet
                        colors,
                    ),
                    _create_comparison_card(
                        "Replies Received",
                        reply_stats.get("replies_received", 0),
                        None,
                        colors,
                    ),
                    _create_comparison_card(
                        "Active Days",
                        user_stats.get("active_days", 0),
                        group_avgs.get("avg_active_days", 0),
                        colors,
                    ),
                ],
                cols={"base": 1, "sm": 2, "lg": 3},
                spacing="md",
            ),
            dmc.Space(h="xl"),
            # Charts row
            dbc.Row(
                [
                    dbc.Col(
                        _create_activity_chart(
                            user_activity_df, group_activity_df, colors
                        ),
                        lg=8,
                    ),
                    dbc.Col(
                        _create_reactions_card(user_reactions, colors),
                        lg=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="xl"),
            # Reply stats
            _create_reply_stats_card(reply_stats, colors),
        ],
        fluid=True,
        className="py-4",
    )


def _create_profile_header(
    user_name: str,
    photo_url: str | None,
    rank: int | None,
    total_users: int,
    member_since: str,
    is_premium: bool,
) -> dmc.Paper:
    """Create the profile header card."""
    rank_text = f"Rank #{rank}" if rank else "Unranked"
    rank_context = f" of {total_users}" if total_users > 0 else ""

    badges = []
    if rank and rank <= 3:
        badge_colors = {1: "yellow", 2: "gray", 3: "orange"}
        badges.append(
            dmc.Badge(
                rank_text,
                color=badge_colors.get(rank, "blue"),
                variant="filled",
                size="lg",
            )
        )
    else:
        badges.append(
            dmc.Badge(
                rank_text + rank_context,
                variant="light",
                size="lg",
            )
        )

    if is_premium:
        badges.append(
            dmc.Badge(
                "Premium",
                color="violet",
                variant="light",
                size="sm",
            )
        )

    return dmc.Paper(
        dmc.Group(
            [
                dmc.Avatar(
                    src=photo_url,
                    alt=user_name,
                    size="xl",
                    radius="xl",
                    children=user_name[0].upper() if user_name else "?",
                ),
                dmc.Stack(
                    [
                        dmc.Title(user_name, order=3),
                        dmc.Group(
                            badges
                            + [
                                dmc.Text(
                                    f"Member since {member_since}",
                                    size="sm",
                                    c="dimmed",
                                ),
                            ],
                            gap="md",
                        ),
                    ],
                    gap="xs",
                ),
            ],
            gap="lg",
        ),
        p="lg",
        withBorder=True,
    )


def _create_comparison_card(
    label: str,
    your_value: int | float,
    group_avg: float | None,
    colors: dict,
) -> dmc.Paper:
    """Create a stat comparison card."""
    # Convert to float to handle Decimal values from database
    your_value = float(your_value) if your_value else 0.0
    group_avg = float(group_avg) if group_avg else None
    your_display = _format_number(your_value)

    if group_avg is not None and group_avg > 0:
        pct = float((your_value - group_avg) / group_avg) * 100
        diff_text = f"{pct:+.0f}% vs avg"
        diff_color = "green" if pct > 0 else "red" if pct < 0 else "dimmed"
        # Calculate progress bar percentage (capped at 200%)
        progress_pct = float(min(100, (your_value / (group_avg * 2)) * 100)) if group_avg else 50.0
    else:
        diff_text = "N/A"
        diff_color = "dimmed"
        progress_pct = 50.0

    return dmc.Paper(
        dmc.Stack(
            [
                dmc.Text(label, size="sm", c="dimmed"),
                dmc.Group(
                    [
                        dmc.Title(your_display, order=3),
                        dmc.Text(diff_text, size="sm", c=diff_color),
                    ],
                    justify="space-between",
                    align="flex-end",
                ),
                dmc.Progress(
                    value=progress_pct,
                    size="sm",
                    radius="xl",
                    color=colors["primary"],
                ),
            ],
            gap="xs",
        ),
        p="md",
        withBorder=True,
    )


def _create_activity_chart(
    user_df,
    group_df,
    colors: dict,
) -> dmc.Paper:
    """Create activity chart comparing user to group."""
    # Merge user and group data
    if user_df.empty and group_df.empty:
        chart = dmc.Center(
            dmc.Text("No activity data available", c="dimmed", size="sm"),
            h=250,
        )
    else:
        # Build chart data
        chart_data = []

        # Create a date-indexed dict for user data
        user_by_date = {}
        if not user_df.empty:
            for _, row in user_df.iterrows():
                date_str = str(row["date"])
                user_by_date[date_str] = float(row["message_count"]) if row["message_count"] else 0.0

        # Use group data as base (has all dates)
        if not group_df.empty:
            for _, row in group_df.iterrows():
                date_str = str(row["date"])
                msg_count = float(row["message_count"]) if row["message_count"] else 0.0
                unique_users = float(row.get("unique_users", 1)) if row.get("unique_users") else 1.0
                chart_data.append(
                    {
                        "date": date_str,
                        "you": float(user_by_date.get(date_str, 0)),
                        "group_avg": msg_count / max(1.0, unique_users),
                    }
                )
        elif not user_df.empty:
            # Only user data available
            for _, row in user_df.iterrows():
                chart_data.append(
                    {
                        "date": str(row["date"]),
                        "you": float(row["message_count"]) if row["message_count"] else 0.0,
                        "group_avg": 0.0,
                    }
                )

        # Limit to last 30 points for readability
        chart_data = chart_data[-30:]

        if chart_data:
            chart = dmc.LineChart(
                data=chart_data,
                dataKey="date",
                series=[
                    {"name": "you", "color": colors["primary"], "strokeWidth": 2},
                    {
                        "name": "group_avg",
                        "color": colors["muted"],
                        "strokeDasharray": "5 5",
                    },
                ],
                h=250,
                curveType="natural",
                withXAxis=True,
                withYAxis=True,
                withDots=False,
                gridColor=colors["border"],
            )
        else:
            chart = dmc.Center(
                dmc.Text("No activity data available", c="dimmed", size="sm"),
                h=250,
            )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Your Activity", order=4),
                    dmc.Group(
                        [
                            dmc.Badge("You", color=colors["primary"], size="sm"),
                            dmc.Badge(
                                "Group Avg",
                                color=colors["muted"],
                                variant="outline",
                                size="sm",
                            ),
                        ],
                        gap="xs",
                    ),
                ],
                justify="space-between",
                align="center",
            ),
            dmc.Space(h="md"),
            chart,
        ],
        p="md",
        withBorder=True,
    )


def _create_reactions_card(reactions: list, colors: dict) -> dmc.Paper:
    """Create top reactions card."""
    if reactions:
        rows = []
        max_count = float(reactions[0]["count"]) if reactions else 1.0
        for reaction in reactions:
            count = float(reaction["count"])
            pct = (count / max_count) * 100 if max_count > 0 else 0.0
            rows.append(
                dmc.Group(
                    [
                        dmc.Text(
                            reaction["emoji"], size="lg", miw=30
                        ),
                        dmc.Box(
                            dmc.Progress(
                                value=pct,
                                size="sm",
                                radius="xl",
                                color=colors["primary"],
                            ),
                            style={"flex": 1},
                        ),
                        dmc.Text(
                            _format_number(count),
                            size="sm",
                            c="dimmed",
                            miw=40,
                            ta="right",
                        ),
                    ],
                    gap="sm",
                )
            )
        content = dmc.Stack(rows, gap="xs")
    else:
        content = dmc.Center(
            dmc.Text("No reactions sent yet", c="dimmed", size="sm"),
            h=150,
        )

    return dmc.Paper(
        [
            dmc.Title("Your Top Reactions", order=4),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_reply_stats_card(reply_stats: dict, colors: dict) -> dmc.Paper:
    """Create reply statistics card."""
    replies_sent = reply_stats.get("replies_sent", 0)
    replies_received = reply_stats.get("replies_received", 0)
    top_replied_to = reply_stats.get("top_replied_to", [])
    top_repliers = reply_stats.get("top_repliers", [])

    def create_partner_list(partners: list, label: str) -> dmc.Stack:
        if partners:
            items = [
                dmc.Group(
                    [
                        dmc.Avatar(
                            children=p["first_name"][0].upper()
                            if p.get("first_name")
                            else "?",
                            size="sm",
                            radius="xl",
                        ),
                        dmc.Box(
                            dmc.Text(
                                p.get("first_name", "Unknown"),
                                size="sm",
                                lineClamp=1,
                            ),
                            style={"flex": 1},
                        ),
                        dmc.Badge(
                            str(p["count"]),
                            variant="light",
                            size="sm",
                        ),
                    ],
                    gap="xs",
                )
                for p in partners[:5]
            ]
            return dmc.Stack(
                [dmc.Text(label, size="xs", c="dimmed")] + items,
                gap="xs",
            )
        else:
            return dmc.Stack(
                [
                    dmc.Text(label, size="xs", c="dimmed"),
                    dmc.Text("No data yet", size="sm", c="dimmed"),
                ],
                gap="xs",
            )

    return dmc.Paper(
        [
            dmc.Title("Reply Activity", order=4),
            dmc.Space(h="md"),
            dbc.Row(
                [
                    dbc.Col(
                        dmc.Stack(
                            [
                                dmc.Text("Replies Sent", size="sm", c="dimmed"),
                                dmc.Title(_format_number(replies_sent), order=3),
                                dmc.Space(h="sm"),
                                create_partner_list(top_replied_to, "Top replied to"),
                            ],
                            gap="xs",
                        ),
                        md=6,
                    ),
                    dbc.Col(
                        dmc.Stack(
                            [
                                dmc.Text("Replies Received", size="sm", c="dimmed"),
                                dmc.Title(_format_number(replies_received), order=3),
                                dmc.Space(h="sm"),
                                create_partner_list(top_repliers, "Top repliers"),
                            ],
                            gap="xs",
                        ),
                        md=6,
                    ),
                ],
            ),
        ],
        p="md",
        withBorder=True,
    )


def _format_number(n: int | float) -> str:
    """Format number with K/M suffix for large values."""
    if n is None:
        return "0"
    n = int(n)
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    elif n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)
