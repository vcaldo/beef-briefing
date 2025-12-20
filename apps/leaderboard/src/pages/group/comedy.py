"""
Comedy page for group statistics.

Hybrid funniness score combining ML humor detection and community laugh reactions:
- Summary cards (laugh reactions, funniest user, top message)
- Comedy leaderboard with hybrid score
- Humor type distribution (donut chart)
- Top funny messages showcase
- Comedy timeline (laugh reactions + humorous messages over time)
- Personal comedy stats vs group average
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Color scheme for comedy theme
COMEDY_COLORS = {
    "primary": "#f59e0b",      # Amber
    "laugh": "#fcd34d",        # Yellow
    "joke": "#f59e0b",         # Amber
    "sarcasm": "#8b5cf6",      # Purple
    "wordplay": "#3b82f6",     # Blue
    "irony": "#ec4899",        # Pink
}


def create_comedy_page(
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
    Create the comedy analysis page for a group.

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
    colors = get_chart_colors(theme_name)

    # Get date range
    start_date, end_date = get_period_dates(period)

    # Fetch data
    comedy_stats = queries.get_comedy_stats(chat_id, start_date, end_date)
    comedy_leaderboard = queries.get_comedy_leaderboard(chat_id, 20, start_date, end_date)
    top_messages = queries.get_top_funny_messages(chat_id, 5, start_date, end_date)
    comedy_timeline_df = queries.get_comedy_timeline(chat_id, start_date, end_date)
    humor_dist_df = queries.get_humor_type_distribution(chat_id, start_date, end_date)

    # Get user's comedy stats
    user_id = user.get("user_id")
    user_comedy_stats = {}
    if user_id:
        user_comedy_stats = queries.get_user_comedy_stats(chat_id, user_id, start_date, end_date)

    # Prepare humor distribution data for donut chart
    humor_dist_data = []
    if not humor_dist_df.empty:
        for _, row in humor_dist_df.iterrows():
            humor_type = row["humor_type"]
            humor_dist_data.append({
                "name": humor_type.capitalize() if humor_type else "Unknown",
                "value": int(row["count"]),
                "color": COMEDY_COLORS.get(humor_type, colors["muted"]),
            })

    # Prepare timeline data
    timeline_data = []
    if not comedy_timeline_df.empty:
        for _, row in comedy_timeline_df.iterrows():
            timeline_data.append({
                "date": row["date"].strftime("%b %d"),
                "laughs": int(row["laugh_reactions"]),
                "humor": int(row["humorous_count"]),
            })

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="comedy",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Summary cards
            _create_summary_cards(comedy_stats, comedy_leaderboard, top_messages, colors),
            dmc.Space(h="lg"),
            # Comedy leaderboard and humor distribution
            dbc.Row(
                [
                    dbc.Col(
                        _create_comedy_leaderboard_card(comedy_leaderboard, colors),
                        lg=8,
                    ),
                    dbc.Col(
                        _create_humor_distribution_card(humor_dist_data, colors),
                        lg=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Timeline and personal stats
            dbc.Row(
                [
                    dbc.Col(
                        _create_timeline_card(timeline_data, colors),
                        lg=8,
                    ),
                    dbc.Col(
                        _create_personal_comedy_card(user_comedy_stats, colors),
                        lg=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Top funny messages
            _create_top_messages_card(top_messages, colors),
        ],
        fluid=True,
        className="py-4",
    )


def _create_summary_cards(
    stats: dict,
    leaderboard: list[dict],
    top_messages: list[dict],
    colors: dict,
) -> dbc.Row:
    """Create summary stat cards row."""
    total_laughs = stats.get("total_laugh_reactions") or 0
    unique_reactors = stats.get("unique_reactors") or 0
    messages_with_laughs = stats.get("messages_with_laughs") or 0

    # Get funniest user
    funniest_user = leaderboard[0] if leaderboard else None
    funniest_name = funniest_user.get("first_name", "N/A") if funniest_user else "N/A"
    funniest_score = funniest_user.get("comedy_score", 0) if funniest_user else 0

    # Get top message preview
    top_message = top_messages[0] if top_messages else None
    top_message_preview = ""
    if top_message and top_message.get("text"):
        text = top_message["text"]
        top_message_preview = text[:40] + "..." if len(text) > 40 else text

    cards = [
        _create_stat_card(
            "Laugh Reactions",
            f"{total_laughs:,}",
            f"from {unique_reactors} users",
            "mdi:emoticon-lol-outline",
            colors,
            value_color=COMEDY_COLORS["primary"],
        ),
        _create_stat_card(
            "Messages with Laughs",
            f"{messages_with_laughs:,}",
            "got funny reactions",
            "mdi:message-text-outline",
            colors,
            value_color=COMEDY_COLORS["laugh"],
        ),
        _create_stat_card(
            "Funniest User",
            funniest_name,
            f"score: {funniest_score:.1f}",
            "mdi:crown-outline",
            colors,
            value_color=COMEDY_COLORS["primary"],
        ),
        _create_stat_card(
            "Top Message",
            top_message_preview or "N/A",
            f"by {top_message.get('author_name', 'Unknown')}" if top_message else "",
            "mdi:star-outline",
            colors,
            value_color=COMEDY_COLORS["primary"],
            small_value=True,
        ),
    ]

    return dbc.Row(
        [dbc.Col(card, sm=6, lg=3) for card in cards],
        className="g-3",
    )


def _create_stat_card(
    title: str,
    value: str,
    subtitle: str,
    icon: str,
    colors: dict,
    value_color: str | None = None,
    small_value: bool = False,
) -> dmc.Paper:
    """Create a single stat card."""
    if value_color:
        value_text = dmc.Text(
            value,
            size="md" if small_value else "xl",
            fw=700,
            c=value_color,
            lineClamp=1,
        )
    else:
        value_text = dmc.Text(
            value,
            size="md" if small_value else "xl",
            fw=700,
            lineClamp=1,
        )

    return dmc.Paper(
        dmc.Stack(
            [
                dmc.Group(
                    [
                        DashIconify(icon=icon, width=24, color=colors["muted"]),
                        dmc.Text(title, size="sm", c="dimmed"),
                    ],
                    gap="xs",
                ),
                value_text,
                dmc.Text(subtitle, size="xs", c="dimmed"),
            ],
            gap="xs",
        ),
        p="md",
        withBorder=True,
    )


def _create_comedy_leaderboard_card(users: list[dict], colors: dict) -> dmc.Paper:
    """Create comedy leaderboard ranking card."""
    if not users:
        content = dmc.Center(
            dmc.Text("No comedy data available", c="dimmed", size="sm"),
            h=400,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            comedy_score = user.get("comedy_score") or 0
            reaction_score = user.get("reaction_score") or 0
            ml_score = user.get("ml_score") or 0
            laugh_reactions = user.get("laugh_reactions") or 0
            distinct_reactors = user.get("distinct_reactors") or 0

            # Rank badge color
            if idx == 1:
                rank_color = "yellow"
            elif idx == 2:
                rank_color = "gray"
            elif idx == 3:
                rank_color = "orange"
            else:
                rank_color = "gray.3"

            rows.append(
                dmc.Stack(
                    [
                        dmc.Group(
                            [
                                dmc.Badge(
                                    f"#{idx}",
                                    color=rank_color,
                                    variant="filled" if idx <= 3 else "light",
                                    size="sm",
                                    w=40,
                                ),
                                dmc.Text(
                                    user.get("first_name", "Unknown"),
                                    size="sm",
                                    fw=500,
                                    style={"flex": 1},
                                ),
                                dmc.Tooltip(
                                    dmc.Text(
                                        f"{comedy_score:.1f}",
                                        size="sm",
                                        fw=700,
                                        c=COMEDY_COLORS["primary"],
                                    ),
                                    label=f"Reaction: {reaction_score:.1f} | ML: {ml_score:.1f}",
                                    position="left",
                                ),
                            ],
                            justify="space-between",
                            gap="xs",
                        ),
                        dmc.Group(
                            [
                                dmc.Text(
                                    f"{laugh_reactions} laughs",
                                    size="xs",
                                    c="dimmed",
                                ),
                                dmc.Text(
                                    f"{distinct_reactors} fans",
                                    size="xs",
                                    c="dimmed",
                                ),
                            ],
                            gap="md",
                        ),
                        dmc.Progress(
                            value=min(comedy_score * 10, 100),
                            color="yellow",
                            size="xs",
                        ),
                    ],
                    gap=4,
                )
            )
        content = dmc.ScrollArea(
            dmc.Stack(rows, gap="md"),
            h=400,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:trophy-outline",
                        width=20,
                        color=COMEDY_COLORS["primary"],
                    ),
                    dmc.Title("Comedy Leaderboard", order=4),
                ],
                gap="xs",
            ),
            dmc.Text(
                "Ranked by hybrid score (70% reactions, 30% ML)",
                size="xs",
                c="dimmed",
            ),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_humor_distribution_card(data: list, colors: dict) -> dmc.Paper:
    """Create humor type distribution donut chart card."""
    if data:
        chart = dmc.DonutChart(
            data=data,
            h=220,
            tooltipDataSource="segment",
            chartLabel="Humor",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No humor data", c="dimmed", size="sm"),
            h=220,
        )

    return dmc.Paper(
        [
            dmc.Title("Humor Types", order=4),
            dmc.Text("Distribution of humor styles", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            chart,
            dmc.Space(h="sm"),
            # Legend
            dmc.Group(
                [
                    dmc.Group(
                        [
                            dmc.Box(
                                w=12,
                                h=12,
                                style={
                                    "backgroundColor": color,
                                    "borderRadius": "2px",
                                },
                            ),
                            dmc.Text(label.capitalize(), size="xs"),
                        ],
                        gap="xs",
                    )
                    for label, color in [
                        ("joke", COMEDY_COLORS["joke"]),
                        ("sarcasm", COMEDY_COLORS["sarcasm"]),
                        ("wordplay", COMEDY_COLORS["wordplay"]),
                        ("irony", COMEDY_COLORS["irony"]),
                    ]
                ],
                gap="md",
                justify="center",
                wrap="wrap",
            ),
        ],
        p="md",
        withBorder=True,
    )


def _create_timeline_card(data: list, colors: dict) -> dmc.Paper:
    """Create comedy timeline area chart card."""
    if data:
        chart = dmc.AreaChart(
            data=data,
            dataKey="date",
            series=[
                {"name": "laughs", "color": COMEDY_COLORS["laugh"]},
                {"name": "humor", "color": COMEDY_COLORS["primary"]},
            ],
            h=300,
            curveType="natural",
            withLegend=True,
            legendProps={"verticalAlign": "bottom"},
            fillOpacity=0.3,
            strokeWidth=2,
            gridColor=colors["border"],
        )
    else:
        chart = dmc.Center(
            dmc.Text("No timeline data for this period", c="dimmed", size="sm"),
            h=300,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Comedy Activity", order=4),
                    dmc.Text("Laugh reactions and humorous messages over time", size="sm", c="dimmed"),
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


def _create_personal_comedy_card(stats: dict, colors: dict) -> dmc.Paper:
    """Create personal comedy stats comparison card."""
    if not stats or not stats.get("messages_analyzed"):
        content = dmc.Center(
            dmc.Text("No personal comedy data available", c="dimmed", size="sm"),
            h=300,
        )
    else:
        comedy_score = stats.get("comedy_score") or 0
        group_avg = stats.get("group_avg_comedy_score") or 0
        laugh_reactions = stats.get("laugh_reactions_received") or 0
        distinct_reactors = stats.get("distinct_reactors") or 0
        humor_rate = stats.get("humor_rate") or 0

        # Comparison indicator
        if comedy_score > group_avg:
            comparison = dmc.Badge("Above Average", color="green", size="sm")
        elif comedy_score < group_avg:
            comparison = dmc.Badge("Below Average", color="red", size="sm")
        else:
            comparison = dmc.Badge("Average", color="gray", size="sm")

        content = dmc.Stack(
            [
                dmc.Group(
                    [
                        dmc.Text("Your Comedy Score", size="sm", c="dimmed"),
                        comparison,
                    ],
                    justify="space-between",
                ),
                dmc.Text(f"{comedy_score:.1f}", size="xl", fw=700, c=COMEDY_COLORS["primary"]),
                dmc.Text(f"Group avg: {group_avg:.1f}", size="xs", c="dimmed"),
                dmc.Divider(),
                dmc.SimpleGrid(
                    [
                        dmc.Stack(
                            [
                                dmc.Text(f"{laugh_reactions}", size="lg", fw=600),
                                dmc.Text("Laughs Received", size="xs", c="dimmed"),
                            ],
                            gap=2,
                            align="center",
                        ),
                        dmc.Stack(
                            [
                                dmc.Text(f"{distinct_reactors}", size="lg", fw=600),
                                dmc.Text("Unique Fans", size="xs", c="dimmed"),
                            ],
                            gap=2,
                            align="center",
                        ),
                        dmc.Stack(
                            [
                                dmc.Text(f"{humor_rate:.1f}%", size="lg", fw=600),
                                dmc.Text("Humor Rate", size="xs", c="dimmed"),
                            ],
                            gap=2,
                            align="center",
                        ),
                    ],
                    cols=3,
                    spacing="xs",
                ),
            ],
            gap="sm",
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:account-outline",
                        width=20,
                        color=COMEDY_COLORS["primary"],
                    ),
                    dmc.Title("Your Comedy Stats", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("How you compare to the group", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_top_messages_card(messages: list[dict], colors: dict) -> dmc.Paper:
    """Create top funny messages showcase card."""
    if not messages:
        content = dmc.Center(
            dmc.Text("No funny messages found", c="dimmed", size="sm"),
            h=200,
        )
    else:
        items = []
        for idx, msg in enumerate(messages, 1):
            text = msg.get("text", "")
            preview = text[:150] + "..." if len(text) > 150 else text
            humor_type = msg.get("humor_type")
            laugh_count = msg.get("laugh_reactions") or 0
            author = msg.get("author_name", "Unknown")
            date = msg.get("date")

            items.append(
                dmc.Paper(
                    dmc.Stack(
                        [
                            dmc.Group(
                                [
                                    dmc.Badge(f"#{idx}", color="yellow", size="sm"),
                                    dmc.Text(author, size="sm", fw=500),
                                    dmc.Badge(
                                        humor_type.capitalize() if humor_type else "Humor",
                                        color=COMEDY_COLORS.get(humor_type, "gray"),
                                        variant="light",
                                        size="xs",
                                    ) if humor_type else None,
                                ],
                                gap="xs",
                            ),
                            dmc.Text(preview, size="sm", style={"whiteSpace": "pre-wrap"}),
                            dmc.Group(
                                [
                                    dmc.Group(
                                        [
                                            DashIconify(icon="mdi:emoticon-lol", width=16, color=COMEDY_COLORS["laugh"]),
                                            dmc.Text(f"{laugh_count}", size="xs", c="dimmed"),
                                        ],
                                        gap=4,
                                    ),
                                    dmc.Text(
                                        date.strftime("%b %d, %Y") if date else "",
                                        size="xs",
                                        c="dimmed",
                                    ),
                                ],
                                justify="space-between",
                            ),
                        ],
                        gap="xs",
                    ),
                    p="sm",
                    withBorder=True,
                    radius="sm",
                )
            )
        content = dmc.Stack(items, gap="sm")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:star-outline",
                        width=20,
                        color=COMEDY_COLORS["primary"],
                    ),
                    dmc.Title("Top Funny Messages", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Highest rated by combined ML + reaction score", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )
