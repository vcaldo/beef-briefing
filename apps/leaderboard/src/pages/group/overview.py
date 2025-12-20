"""
Overview page for group statistics.

High-level snapshot with:
- 4 stat cards (Messages, Active Users, Reactions, Media) with % change
- Activity sparkline
- Mini leaderboard (top 5 users)
- Top reactions
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav, get_comparison_dates, get_period_dates


def create_overview_page(
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
    Create the overview page for a group.

    Args:
        chat_id: Chat ID
        chat_info: Chat metadata (title, type, etc.)
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
    colors = get_chart_colors(theme_name)

    # Get date ranges for current and previous periods
    current_start, current_end, previous_start, previous_end = get_comparison_dates(
        period
    )

    # Fetch stats with comparison
    if current_start is not None:
        stats_comparison = queries.get_overview_stats_comparison(
            chat_id, current_start, current_end, previous_start, previous_end
        )
        stats = stats_comparison["current"]
        changes = stats_comparison["changes"]
    else:
        # MAX period - no comparison
        stats = queries.get_overview_stats(chat_id)
        changes = {
            "total_messages": None,
            "total_users": None,
            "total_reactions": None,
            "total_media": None,
        }

    # Fetch activity data for sparkline
    start_date, end_date = get_period_dates(period)
    activity_df = queries.get_daily_activity(chat_id, start_date, end_date)

    # Fetch top users for mini leaderboard
    top_users = queries.get_user_rankings(chat_id, metric="message_count", limit=5)

    # Fetch photos for top users
    user_ids = [u["user_id"] for u in top_users]
    photo_urls = photo_client.get_user_photos_batch(user_ids, size="small")

    # Fetch top reactions
    top_reactions = queries.get_top_reactions(chat_id, limit=5)

    # Prepare sparkline data
    sparkline_data = []
    if not activity_df.empty:
        sparkline_data = activity_df["message_count"].tolist()[-30:]  # Last 30 points

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="overview",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Stat cards row
            dmc.SimpleGrid(
                [
                    _create_stat_card(
                        "Messages",
                        _format_number(stats["total_messages"]),
                        changes["total_messages"],
                        "mdi:message-outline",
                        colors,
                    ),
                    _create_stat_card(
                        "Active Users",
                        _format_number(stats["total_users"]),
                        changes["total_users"],
                        "mdi:account-group-outline",
                        colors,
                    ),
                    _create_stat_card(
                        "Reactions",
                        _format_number(stats["total_reactions"]),
                        changes["total_reactions"],
                        "mdi:emoticon-outline",
                        colors,
                    ),
                    _create_stat_card(
                        "Media",
                        _format_number(stats["total_media"]),
                        changes["total_media"],
                        "mdi:image-outline",
                        colors,
                    ),
                ],
                cols={"base": 1, "sm": 2, "lg": 4},
                spacing="md",
            ),
            dmc.Space(h="xl"),
            # Charts row
            dbc.Row(
                [
                    dbc.Col(
                        _create_activity_card(sparkline_data, colors),
                        lg=8,
                    ),
                    dbc.Col(
                        _create_leaderboard_card(
                            top_users, base_url, chat_id, colors, photo_urls
                        ),
                        lg=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Additional info row
            dbc.Row(
                [
                    dbc.Col(
                        _create_reactions_card(top_reactions, colors),
                        lg=6,
                    ),
                    dbc.Col(
                        _create_quick_stats_card(stats, chat_info, colors),
                        lg=6,
                    ),
                ],
                className="g-3",
            ),
        ],
        fluid=True,
        className="py-4",
    )


def _create_stat_card(
    label: str,
    value: str,
    change: float | None,
    icon: str,
    colors: dict,
) -> dmc.Paper:
    """Create a stat card with value and change indicator."""
    if change is None:
        change_text = "N/A"
        change_color = "dimmed"
    elif change > 0:
        change_text = f"+{change:.0f}%"
        change_color = "green"
    elif change < 0:
        change_text = f"{change:.0f}%"
        change_color = "red"
    else:
        change_text = "0%"
        change_color = "dimmed"

    return dmc.Paper(
        dmc.Stack(
            [
                dmc.Group(
                    [
                        DashIconify(icon=icon, width=24, color=colors["muted"]),
                        dmc.Text(label, size="sm", c="dimmed"),
                    ],
                    gap="xs",
                ),
                dmc.Group(
                    [
                        dmc.Title(value, order=2),
                        dmc.Badge(
                            change_text, color=change_color, variant="light", size="sm"
                        ),
                    ],
                    justify="space-between",
                    align="flex-end",
                ),
            ],
            gap="xs",
        ),
        p="md",
        withBorder=True,
    )


def _create_activity_card(sparkline_data: list, colors: dict) -> dmc.Paper:
    """Create activity chart card with sparkline."""
    if sparkline_data:
        chart = dmc.AreaChart(
            data=[{"index": i, "messages": v} for i, v in enumerate(sparkline_data)],
            dataKey="index",
            series=[{"name": "messages", "color": colors["primary"]}],
            h=200,
            curveType="natural",
            withXAxis=False,
            withYAxis=False,
            withDots=False,
            fillOpacity=0.3,
            strokeWidth=2,
            gridColor=colors["border"],
        )
    else:
        chart = dmc.Center(
            dmc.Text("No activity data", c="dimmed", size="sm"),
            h=200,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Activity", order=4),
                    dmc.Text("Messages over time", size="sm", c="dimmed"),
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


def _create_leaderboard_card(
    top_users: list, base_url: str, chat_id: int, colors: dict, photo_urls: dict
) -> dmc.Paper:
    """Create mini leaderboard card."""
    if top_users:
        rows = []
        for user in top_users:
            rows.append(
                dmc.Group(
                    [
                        dmc.Badge(
                            f"#{user['rank']}",
                            variant="light",
                            size="sm",
                            w=40,
                        ),
                        dmc.Avatar(
                            src=photo_urls.get(user["user_id"]),
                            children=user["first_name"][0].upper()
                            if user["first_name"]
                            else "?",
                            size="sm",
                            radius="xl",
                        ),
                        dmc.Box(
                            dmc.Text(
                                user["first_name"] or "Unknown",
                                size="sm",
                                lineClamp=1,
                            ),
                            style={"flex": 1},
                        ),
                        dmc.Text(
                            _format_number(user["score"]),
                            size="sm",
                            fw=500,
                        ),
                    ],
                    gap="xs",
                )
            )
        content = dmc.Stack(rows, gap="xs")
    else:
        content = dmc.Center(
            dmc.Text("No users yet", c="dimmed", size="sm"),
            h=150,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Top Contributors", order=4),
                    dmc.Anchor(
                        dmc.Text("View all", size="xs", c="dimmed"),
                        href=f"{base_url}/group/{chat_id}/leaderboard",
                    ),
                ],
                justify="space-between",
                align="center",
            ),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_reactions_card(top_reactions: list, colors: dict) -> dmc.Paper:
    """Create top reactions card."""
    if top_reactions:
        rows = []
        max_count = top_reactions[0]["count"] if top_reactions else 1
        for reaction in top_reactions:
            pct = (reaction["count"] / max_count) * 100 if max_count > 0 else 0
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
                            _format_number(reaction["count"]),
                            size="sm",
                            c="dimmed",
                            miw=50,
                            ta="right",
                        ),
                    ],
                    gap="sm",
                )
            )
        content = dmc.Stack(rows, gap="xs")
    else:
        content = dmc.Center(
            dmc.Text("No reactions yet", c="dimmed", size="sm"),
            h=100,
        )

    return dmc.Paper(
        [
            dmc.Title("Top Reactions", order=4),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_quick_stats_card(stats: dict, chat_info: dict, colors: dict) -> dmc.Paper:
    """Create quick stats card with additional info."""
    first_msg = chat_info.get("first_message")
    last_msg = chat_info.get("last_message")

    first_msg_str = first_msg.strftime("%b %d, %Y") if first_msg else "N/A"
    last_msg_str = last_msg.strftime("%b %d, %Y") if last_msg else "N/A"

    return dmc.Paper(
        [
            dmc.Title("Quick Stats", order=4),
            dmc.Space(h="md"),
            dmc.SimpleGrid(
                [
                    dmc.Stack(
                        [
                            dmc.Text("Messages/Day", size="xs", c="dimmed"),
                            dmc.Text(
                                f"{stats.get('messages_per_day', 0):.1f}",
                                size="lg",
                                fw=500,
                            ),
                        ],
                        gap=0,
                    ),
                    dmc.Stack(
                        [
                            dmc.Text("First Message", size="xs", c="dimmed"),
                            dmc.Text(first_msg_str, size="sm"),
                        ],
                        gap=0,
                    ),
                    dmc.Stack(
                        [
                            dmc.Text("Last Message", size="xs", c="dimmed"),
                            dmc.Text(last_msg_str, size="sm"),
                        ],
                        gap=0,
                    ),
                    dmc.Stack(
                        [
                            dmc.Text("Group Type", size="xs", c="dimmed"),
                            dmc.Text(
                                chat_info.get("type", "unknown").capitalize(), size="sm"
                            ),
                        ],
                        gap=0,
                    ),
                ],
                cols=2,
                spacing="md",
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
