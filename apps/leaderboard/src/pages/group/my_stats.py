"""
My Stats page for group statistics.

Personal stats with group comparison:
- Profile header (avatar, name, rank, member since)
- Stat comparisons (6 metrics vs group average)
- Sentiment analysis (score, distribution, rank)
- Personal timeline (line chart: user vs group average)
- Top reactions you send (horizontal bar chart)
- Reply stats (replies sent/received, top partners)
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Sentiment colors (matching sentiment.py)
SENTIMENT_COLORS = {
    "positive": "#22c55e",  # Green
    "neutral": "#94a3b8",   # Gray
    "negative": "#ef4444",  # Red
}


def create_my_stats_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
    photo_client,
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
        photo_client: PhotoClient for fetching profile photos

    Returns:
        Page layout as html.Div
    """
    chat_title = chat_info.get("title", f"Chat {chat_id}")
    is_admin = user.get("is_admin", False)
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

    # Fetch photos for reply partners
    partner_ids = set()
    for p in reply_stats.get("top_replied_to", []):
        partner_ids.add(p["user_id"])
    for p in reply_stats.get("top_repliers", []):
        partner_ids.add(p["user_id"])
    partner_photo_urls = photo_client.get_user_photos_batch(
        list(partner_ids), size="small"
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

    # Fetch user sentiment stats
    sentiment_stats = queries.get_user_sentiment_stats(
        chat_id, user_id, start_date, end_date
    )

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
                is_admin=is_admin,
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
            # Sentiment section
            _create_sentiment_card(sentiment_stats, colors),
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
            _create_reply_stats_card(reply_stats, colors, partner_photo_urls),
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


def _create_reply_stats_card(
    reply_stats: dict, colors: dict, photo_urls: dict
) -> dmc.Paper:
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
                            src=photo_urls.get(p["user_id"]),
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


def _create_sentiment_card(sentiment_stats: dict, colors: dict) -> dmc.Paper:
    """Create sentiment analysis card showing user's sentiment profile."""
    from dash_iconify import DashIconify

    avg_sentiment = sentiment_stats.get("avg_sentiment")
    positive_count = sentiment_stats.get("positive_count", 0) or 0
    neutral_count = sentiment_stats.get("neutral_count", 0) or 0
    negative_count = sentiment_stats.get("negative_count", 0) or 0
    messages_analyzed = sentiment_stats.get("messages_analyzed", 0) or 0
    group_avg = sentiment_stats.get("group_avg_sentiment")
    sentiment_rank = sentiment_stats.get("sentiment_rank")
    total_ranked = sentiment_stats.get("total_ranked_users", 0) or 0

    # No sentiment data
    if messages_analyzed == 0 or avg_sentiment is None:
        return dmc.Paper(
            [
                dmc.Group(
                    [
                        DashIconify(icon="mdi:brain", width=24, color=colors["muted"]),
                        dmc.Title("Your Sentiment", order=4),
                    ],
                    gap="xs",
                ),
                dmc.Space(h="md"),
                dmc.Center(
                    dmc.Text("No sentiment data available yet", c="dimmed", size="sm"),
                    h=100,
                ),
            ],
            p="md",
            withBorder=True,
        )

    # Determine sentiment label and color
    if avg_sentiment > 0.1:
        sentiment_label = "Positive"
        sentiment_color = SENTIMENT_COLORS["positive"]
        sentiment_icon = "mdi:emoticon-happy-outline"
    elif avg_sentiment < -0.1:
        sentiment_label = "Negative"
        sentiment_color = SENTIMENT_COLORS["negative"]
        sentiment_icon = "mdi:emoticon-sad-outline"
    else:
        sentiment_label = "Neutral"
        sentiment_color = SENTIMENT_COLORS["neutral"]
        sentiment_icon = "mdi:emoticon-neutral-outline"

    # Calculate percentages for distribution
    total = positive_count + neutral_count + negative_count
    pos_pct = (positive_count / total * 100) if total > 0 else 0
    neu_pct = (neutral_count / total * 100) if total > 0 else 0
    neg_pct = (negative_count / total * 100) if total > 0 else 0

    # Group comparison
    if group_avg is not None:
        diff = float(avg_sentiment) - float(group_avg)
        if diff > 0.05:
            diff_text = f"+{diff:.2f} vs group"
            diff_color = "green"
        elif diff < -0.05:
            diff_text = f"{diff:.2f} vs group"
            diff_color = "red"
        else:
            diff_text = "Same as group"
            diff_color = "dimmed"
    else:
        diff_text = ""
        diff_color = "dimmed"

    # Rank display
    if sentiment_rank and total_ranked > 0:
        rank_text = f"#{sentiment_rank} of {total_ranked}"
    else:
        rank_text = "Not ranked (need 5+ messages)"

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(icon="mdi:brain", width=24, color=colors["muted"]),
                    dmc.Title("Your Sentiment", order=4),
                ],
                gap="xs",
            ),
            dmc.Space(h="md"),
            dbc.Row(
                [
                    # Score column
                    dbc.Col(
                        dmc.Stack(
                            [
                                dmc.Group(
                                    [
                                        DashIconify(
                                            icon=sentiment_icon,
                                            width=32,
                                            color=sentiment_color,
                                        ),
                                        dmc.Stack(
                                            [
                                                dmc.Text(
                                                    f"{avg_sentiment:+.2f}",
                                                    size="xl",
                                                    fw=700,
                                                    c=sentiment_color,
                                                ),
                                                dmc.Text(
                                                    sentiment_label,
                                                    size="sm",
                                                    c="dimmed",
                                                ),
                                            ],
                                            gap=0,
                                        ),
                                    ],
                                    gap="sm",
                                ),
                                dmc.Text(diff_text, size="sm", c=diff_color)
                                if diff_text
                                else None,
                                dmc.Text(
                                    f"Positivity Rank: {rank_text}",
                                    size="xs",
                                    c="dimmed",
                                ),
                                dmc.Text(
                                    f"{messages_analyzed} messages analyzed",
                                    size="xs",
                                    c="dimmed",
                                ),
                            ],
                            gap="xs",
                        ),
                        md=4,
                    ),
                    # Distribution column
                    dbc.Col(
                        dmc.Stack(
                            [
                                dmc.Text("Your Message Breakdown", size="sm", c="dimmed"),
                                # Stacked progress bar
                                dmc.Group(
                                    [
                                        dmc.Box(
                                            w=f"{pos_pct}%",
                                            h=8,
                                            style={
                                                "backgroundColor": SENTIMENT_COLORS["positive"],
                                                "borderRadius": "4px 0 0 4px" if pos_pct > 0 else "0",
                                                "minWidth": "4px" if pos_pct > 0 else "0",
                                            },
                                        )
                                        if pos_pct > 0
                                        else None,
                                        dmc.Box(
                                            w=f"{neu_pct}%",
                                            h=8,
                                            style={
                                                "backgroundColor": SENTIMENT_COLORS["neutral"],
                                                "minWidth": "4px" if neu_pct > 0 else "0",
                                            },
                                        )
                                        if neu_pct > 0
                                        else None,
                                        dmc.Box(
                                            w=f"{neg_pct}%",
                                            h=8,
                                            style={
                                                "backgroundColor": SENTIMENT_COLORS["negative"],
                                                "borderRadius": "0 4px 4px 0" if neg_pct > 0 else "0",
                                                "minWidth": "4px" if neg_pct > 0 else "0",
                                            },
                                        )
                                        if neg_pct > 0
                                        else None,
                                    ],
                                    gap=0,
                                    style={"width": "100%"},
                                ),
                                # Legend with counts
                                dmc.Group(
                                    [
                                        dmc.Group(
                                            [
                                                dmc.Box(
                                                    w=12,
                                                    h=12,
                                                    style={
                                                        "backgroundColor": SENTIMENT_COLORS["positive"],
                                                        "borderRadius": "2px",
                                                    },
                                                ),
                                                dmc.Text(
                                                    f"Positive: {positive_count} ({pos_pct:.0f}%)",
                                                    size="xs",
                                                ),
                                            ],
                                            gap="xs",
                                        ),
                                        dmc.Group(
                                            [
                                                dmc.Box(
                                                    w=12,
                                                    h=12,
                                                    style={
                                                        "backgroundColor": SENTIMENT_COLORS["neutral"],
                                                        "borderRadius": "2px",
                                                    },
                                                ),
                                                dmc.Text(
                                                    f"Neutral: {neutral_count} ({neu_pct:.0f}%)",
                                                    size="xs",
                                                ),
                                            ],
                                            gap="xs",
                                        ),
                                        dmc.Group(
                                            [
                                                dmc.Box(
                                                    w=12,
                                                    h=12,
                                                    style={
                                                        "backgroundColor": SENTIMENT_COLORS["negative"],
                                                        "borderRadius": "2px",
                                                    },
                                                ),
                                                dmc.Text(
                                                    f"Negative: {negative_count} ({neg_pct:.0f}%)",
                                                    size="xs",
                                                ),
                                            ],
                                            gap="xs",
                                        ),
                                    ],
                                    gap="md",
                                ),
                            ],
                            gap="sm",
                        ),
                        md=8,
                    ),
                ],
            ),
        ],
        p="md",
        withBorder=True,
    )
