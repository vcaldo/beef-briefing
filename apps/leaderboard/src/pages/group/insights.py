"""
Insights page for group statistics.

Humor and questions visualizations:
- Summary cards (humorous messages, questions, top types)
- Humor type distribution (donut chart)
- Question type distribution (donut chart)
- Combined timeline (humor + questions over time)
- Funniest users ranking
- Most inquisitive users ranking
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Humor type colors
HUMOR_COLORS = {
    "joke": "#f59e0b",      # Amber
    "sarcasm": "#8b5cf6",   # Purple
    "wordplay": "#3b82f6",  # Blue
    "irony": "#ec4899",     # Pink
}

# Question type colors
QUESTION_COLORS = {
    "factual": "#3b82f6",       # Blue
    "opinion": "#8b5cf6",       # Purple
    "rhetorical": "#f59e0b",    # Amber
    "clarification": "#22c55e", # Green
}


def create_insights_page(
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
    Create the insights analysis page for a group.

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
    humor_stats = queries.get_humor_stats(chat_id, start_date, end_date)
    questions_stats = queries.get_questions_stats(chat_id, start_date, end_date)
    humor_dist_df = queries.get_humor_type_distribution(chat_id, start_date, end_date)
    question_dist_df = queries.get_question_type_distribution(chat_id, start_date, end_date)
    humor_timeline_df = queries.get_humor_timeline(chat_id, start_date, end_date)
    questions_timeline_df = queries.get_questions_timeline(chat_id, start_date, end_date)
    funniest_users = queries.get_funniest_users(chat_id, 10, start_date, end_date)
    inquisitive_users = queries.get_most_inquisitive_users(chat_id, 10, start_date, end_date)

    # Prepare humor distribution data for donut chart
    humor_dist_data = []
    if not humor_dist_df.empty:
        for _, row in humor_dist_df.iterrows():
            humor_type = row["humor_type"]
            humor_dist_data.append({
                "name": humor_type.capitalize() if humor_type else "Unknown",
                "value": int(row["count"]),
                "color": HUMOR_COLORS.get(humor_type, colors["muted"]),
            })

    # Prepare question distribution data for donut chart
    question_dist_data = []
    if not question_dist_df.empty:
        for _, row in question_dist_df.iterrows():
            question_type = row["question_type"]
            question_dist_data.append({
                "name": question_type.capitalize() if question_type else "Unknown",
                "value": int(row["count"]),
                "color": QUESTION_COLORS.get(question_type, colors["muted"]),
            })

    # Prepare combined timeline data
    timeline_data = []
    if not humor_timeline_df.empty or not questions_timeline_df.empty:
        # Merge timelines by date
        all_dates = set()
        humor_by_date = {}
        questions_by_date = {}

        if not humor_timeline_df.empty:
            for _, row in humor_timeline_df.iterrows():
                d = row["date"]
                all_dates.add(d)
                humor_by_date[d] = int(row["humorous_count"])

        if not questions_timeline_df.empty:
            for _, row in questions_timeline_df.iterrows():
                d = row["date"]
                all_dates.add(d)
                questions_by_date[d] = int(row["question_count"])

        for d in sorted(all_dates):
            timeline_data.append({
                "date": d.strftime("%b %d"),
                "humor": humor_by_date.get(d, 0),
                "questions": questions_by_date.get(d, 0),
            })

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="insights",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Summary cards
            _create_summary_cards(humor_stats, questions_stats, colors),
            dmc.Space(h="lg"),
            # Humor and question distributions
            dbc.Row(
                [
                    dbc.Col(
                        _create_humor_distribution_card(humor_dist_data, colors),
                        md=6,
                    ),
                    dbc.Col(
                        _create_question_distribution_card(question_dist_data, colors),
                        md=6,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Combined timeline
            _create_timeline_card(timeline_data, colors),
            dmc.Space(h="lg"),
            # User rankings
            dbc.Row(
                [
                    dbc.Col(
                        _create_funniest_users_card(funniest_users, colors),
                        md=6,
                    ),
                    dbc.Col(
                        _create_inquisitive_users_card(inquisitive_users, colors),
                        md=6,
                    ),
                ],
                className="g-3",
            ),
        ],
        fluid=True,
        className="py-4",
    )


def _create_summary_cards(
    humor_stats: dict,
    questions_stats: dict,
    colors: dict,
) -> dbc.Row:
    """Create summary stat cards row."""
    top_humor_type = humor_stats.get("top_humor_type")
    top_question_type = questions_stats.get("top_question_type")

    cards = [
        _create_stat_card(
            "Humorous",
            f"{humor_stats.get('humorous_count') or 0:,}",
            f"{humor_stats.get('humor_rate') or 0}% of messages",
            "mdi:emoticon-lol-outline",
            colors,
            value_color="#f59e0b",
        ),
        _create_stat_card(
            "Questions",
            f"{questions_stats.get('question_count') or 0:,}",
            f"{questions_stats.get('question_rate') or 0}% of messages",
            "mdi:help-circle-outline",
            colors,
            value_color="#3b82f6",
        ),
        _create_stat_card(
            "Top Humor",
            top_humor_type.capitalize() if top_humor_type else "N/A",
            "most common style",
            "mdi:theater",
            colors,
            value_color=HUMOR_COLORS.get(top_humor_type),
        ),
        _create_stat_card(
            "Top Question",
            top_question_type.capitalize() if top_question_type else "N/A",
            "most common type",
            "mdi:comment-question-outline",
            colors,
            value_color=QUESTION_COLORS.get(top_question_type),
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
) -> dmc.Paper:
    """Create a single stat card."""
    if value_color:
        value_text = dmc.Text(value, size="xl", fw=700, c=value_color)
    else:
        value_text = dmc.Text(value, size="xl", fw=700)

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
                    for label, color in HUMOR_COLORS.items()
                ],
                gap="md",
                justify="center",
                wrap="wrap",
            ),
        ],
        p="md",
        withBorder=True,
    )


def _create_question_distribution_card(data: list, colors: dict) -> dmc.Paper:
    """Create question type distribution donut chart card."""
    if data:
        chart = dmc.DonutChart(
            data=data,
            h=220,
            tooltipDataSource="segment",
            chartLabel="Questions",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No question data", c="dimmed", size="sm"),
            h=220,
        )

    return dmc.Paper(
        [
            dmc.Title("Question Types", order=4),
            dmc.Text("Distribution of question styles", size="xs", c="dimmed"),
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
                    for label, color in QUESTION_COLORS.items()
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
    """Create combined humor + questions timeline area chart card."""
    if data:
        chart = dmc.AreaChart(
            data=data,
            dataKey="date",
            series=[
                {"name": "humor", "color": "#f59e0b"},
                {"name": "questions", "color": "#3b82f6"},
            ],
            h=250,
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
            h=250,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Activity Over Time", order=4),
                    dmc.Text("Daily humor and questions", size="sm", c="dimmed"),
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


def _create_funniest_users_card(users: list[dict], colors: dict) -> dmc.Paper:
    """Create funniest users ranking card."""
    if not users:
        content = dmc.Center(
            dmc.Text("No humor data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            humor_rate = user.get("humor_rate", 0) or 0
            humorous_count = user.get("humorous_count", 0) or 0

            rows.append(
                dmc.Stack(
                    [
                        dmc.Group(
                            [
                                dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                                dmc.Text(
                                    user.get("first_name", "Unknown"),
                                    size="sm",
                                    style={"flex": 1},
                                ),
                                dmc.Text(
                                    f"{humor_rate:.1f}%",
                                    size="sm",
                                    fw=600,
                                    c="#f59e0b",
                                ),
                            ],
                            justify="space-between",
                            gap="xs",
                        ),
                        dmc.Progress(
                            value=min(humor_rate, 100),
                            color="yellow",
                            size="xs",
                        ),
                        dmc.Text(
                            f"{humorous_count} funny messages",
                            size="xs",
                            c="dimmed",
                        ),
                    ],
                    gap=4,
                )
            )
        content = dmc.Stack(rows, gap="md")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:emoticon-lol-outline",
                        width=20,
                        color="#f59e0b",
                    ),
                    dmc.Title("Funniest Users", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Ranked by humor rate (min 10 messages)", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_inquisitive_users_card(users: list[dict], colors: dict) -> dmc.Paper:
    """Create most inquisitive users ranking card."""
    if not users:
        content = dmc.Center(
            dmc.Text("No question data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            question_rate = user.get("question_rate", 0) or 0
            question_count = user.get("question_count", 0) or 0

            rows.append(
                dmc.Stack(
                    [
                        dmc.Group(
                            [
                                dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                                dmc.Text(
                                    user.get("first_name", "Unknown"),
                                    size="sm",
                                    style={"flex": 1},
                                ),
                                dmc.Text(
                                    f"{question_rate:.1f}%",
                                    size="sm",
                                    fw=600,
                                    c="#3b82f6",
                                ),
                            ],
                            justify="space-between",
                            gap="xs",
                        ),
                        dmc.Progress(
                            value=min(question_rate, 100),
                            color="blue",
                            size="xs",
                        ),
                        dmc.Text(
                            f"{question_count} questions asked",
                            size="xs",
                            c="dimmed",
                        ),
                    ],
                    gap=4,
                )
            )
        content = dmc.Stack(rows, gap="md")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:help-circle-outline",
                        width=20,
                        color="#3b82f6",
                    ),
                    dmc.Title("Most Inquisitive", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Ranked by question rate (min 10 messages)", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )
