"""
Sentiment page for group statistics.

Sentiment analysis visualizations:
- Summary cards (analyzed, positive %, toxicity %, confidence)
- Sentiment distribution (donut chart)
- Sentiment timeline (stacked area chart)
- Sentiment heatmap (hour x day matrix)
- User rankings (most positive/negative)
- Toxicity timeline (bar chart)
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Day labels for heatmap
DAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]

# Sentiment colors
SENTIMENT_COLORS = {
    "positive": "#22c55e",  # Green
    "neutral": "#94a3b8",   # Gray
    "negative": "#ef4444",  # Red
}


def create_sentiment_page(
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
    Create the sentiment analysis page for a group.

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
    sentiment_stats = queries.get_sentiment_stats(chat_id, start_date, end_date)
    toxicity_stats = queries.get_toxicity_stats(chat_id, start_date, end_date)
    distribution_df = queries.get_sentiment_distribution(chat_id, start_date, end_date)
    timeline_df = queries.get_sentiment_timeline(chat_id, start_date, end_date)
    heatmap_df = queries.get_hourly_sentiment_heatmap(chat_id, start_date, end_date)
    positive_users = queries.get_user_sentiment_rankings(
        chat_id, start_date, end_date, limit=5, ascending=False
    )
    negative_users = queries.get_user_sentiment_rankings(
        chat_id, start_date, end_date, limit=5, ascending=True
    )
    toxic_users = queries.get_user_toxicity_rankings(
        chat_id, limit=5, start_date=start_date, end_date=end_date
    )

    # Prepare timeline data
    timeline_data = []
    if not timeline_df.empty:
        for _, row in timeline_df.iterrows():
            timeline_data.append(
                {
                    "date": row["date"].strftime("%b %d"),
                    "positive": int(row["positive"]),
                    "neutral": int(row["neutral"]),
                    "negative": int(row["negative"]),
                }
            )

    # Prepare distribution data for donut chart
    distribution_data = []
    if not distribution_df.empty:
        for _, row in distribution_df.iterrows():
            distribution_data.append(
                {
                    "name": row["label"].capitalize(),
                    "value": int(row["count"]),
                    "color": SENTIMENT_COLORS.get(row["label"], colors["muted"]),
                }
            )

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="sentiment",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Summary cards
            _create_summary_cards(sentiment_stats, toxicity_stats, colors),
            dmc.Space(h="lg"),
            # Distribution and timeline
            dbc.Row(
                [
                    dbc.Col(
                        _create_distribution_card(distribution_data, colors),
                        md=4,
                    ),
                    dbc.Col(
                        _create_timeline_card(timeline_data, colors),
                        md=8,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Sentiment heatmap
            _create_heatmap_card(heatmap_df, colors),
            dmc.Space(h="lg"),
            # User rankings
            dbc.Row(
                [
                    dbc.Col(
                        _create_user_ranking_card(
                            positive_users,
                            "Most Positive Users",
                            "avg_sentiment",
                            colors,
                            is_positive=True,
                        ),
                        md=6,
                    ),
                    dbc.Col(
                        _create_user_ranking_card(
                            negative_users,
                            "Most Negative Users",
                            "avg_sentiment",
                            colors,
                            is_positive=False,
                        ),
                        md=6,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Toxicity user rankings
            dbc.Row(
                [
                    dbc.Col(
                        _create_toxicity_ranking_card(toxic_users, colors),
                        md=6,
                    ),
                ],
                className="g-3",
            ),
        ],
        fluid=True,
        className="py-4",
    )


def _create_summary_cards(sentiment_stats: dict, toxicity_stats: dict, colors: dict) -> dbc.Row:
    """Create summary stat cards row."""
    total_analyzed = sentiment_stats.get("total_analyzed", 0)
    total_messages = sentiment_stats.get("total_messages", 0)
    coverage = round(total_analyzed / total_messages * 100, 1) if total_messages > 0 else 0

    cards = [
        _create_stat_card(
            "Analyzed",
            f"{total_analyzed:,}",
            f"{coverage}% coverage",
            "mdi:chart-bar",
            colors,
        ),
        _create_stat_card(
            "Positive",
            f"{sentiment_stats.get('positive_rate', 0)}%",
            f"{sentiment_stats.get('positive_count', 0):,} messages",
            "mdi:emoticon-happy-outline",
            colors,
            value_color=SENTIMENT_COLORS["positive"],
        ),
        _create_stat_card(
            "Toxicity",
            f"{toxicity_stats.get('toxic_rate', 0)}%",
            f"{toxicity_stats.get('toxic_count', 0):,} toxic",
            "mdi:alert-circle-outline",
            colors,
            value_color=SENTIMENT_COLORS["negative"],
        ),
        _create_stat_card(
            "Confidence",
            f"{sentiment_stats.get('avg_confidence', 0):.2f}",
            "avg model confidence",
            "mdi:shield-check-outline",
            colors,
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
    from dash_iconify import DashIconify

    # Build value text with optional color
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


def _create_distribution_card(data: list, colors: dict) -> dmc.Paper:
    """Create sentiment distribution donut chart card."""
    if data:
        chart = dmc.DonutChart(
            data=data,
            h=220,
            tooltipDataSource="segment",
            chartLabel="Sentiment",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No sentiment data", c="dimmed", size="sm"),
            h=220,
        )

    return dmc.Paper(
        [
            dmc.Title("Sentiment Distribution", order=4),
            dmc.Text("Message sentiment breakdown", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            chart,
            dmc.Space(h="sm"),
            # Legend
            dmc.Group(
                [
                    dmc.Group(
                        [
                            dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["positive"], "borderRadius": "2px"}),
                            dmc.Text("Positive", size="xs"),
                        ],
                        gap="xs",
                    ),
                    dmc.Group(
                        [
                            dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["neutral"], "borderRadius": "2px"}),
                            dmc.Text("Neutral", size="xs"),
                        ],
                        gap="xs",
                    ),
                    dmc.Group(
                        [
                            dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["negative"], "borderRadius": "2px"}),
                            dmc.Text("Negative", size="xs"),
                        ],
                        gap="xs",
                    ),
                ],
                gap="md",
                justify="center",
            ),
        ],
        p="md",
        withBorder=True,
    )


def _create_timeline_card(data: list, colors: dict) -> dmc.Paper:
    """Create sentiment timeline stacked area chart card."""
    if data:
        chart = dmc.AreaChart(
            data=data,
            dataKey="date",
            series=[
                {"name": "positive", "color": SENTIMENT_COLORS["positive"]},
                {"name": "neutral", "color": SENTIMENT_COLORS["neutral"]},
                {"name": "negative", "color": SENTIMENT_COLORS["negative"]},
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
            dmc.Text("No sentiment data for this period", c="dimmed", size="sm"),
            h=250,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Sentiment Over Time", order=4),
                    dmc.Text("Daily sentiment distribution", size="sm", c="dimmed"),
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


def _create_heatmap_card(heatmap_df, colors: dict) -> dmc.Paper:
    """Create sentiment heatmap card using custom grid."""
    if heatmap_df.empty:
        content = dmc.Center(
            dmc.Text("No heatmap data", c="dimmed", size="sm"),
            h=200,
        )
    else:
        # Create matrix: 7 days x 24 hours
        matrix = [[None for _ in range(24)] for _ in range(7)]

        for _, row in heatmap_df.iterrows():
            day = int(row["day_of_week"])
            hour = int(row["hour"])
            sentiment = float(row["avg_sentiment"])
            if 0 <= day < 7 and 0 <= hour < 24:
                matrix[day][hour] = sentiment

        # Build grid cells
        rows = []

        # Header row with hour labels
        header_cells = [dmc.Text("", size="xs", w=40, ta="center")]
        for hour in range(0, 24, 3):
            header_cells.append(
                dmc.Text(f"{hour:02d}", size="xs", c="dimmed", w=36, ta="center")
            )
        rows.append(dmc.Group(header_cells, gap=2))

        # Data rows
        for day_idx, day_label in enumerate(DAY_LABELS):
            row_cells = [
                dmc.Text(day_label, size="xs", c="dimmed", w=40, ta="right", pr=4)
            ]

            for hour in range(24):
                sentiment = matrix[day_idx][hour]

                if sentiment is None:
                    bg_color = colors["surface"]
                    tooltip_text = f"{day_label} {hour:02d}:00 - No data"
                else:
                    # Map sentiment (-1 to 1) to color
                    if sentiment > 0.2:
                        bg_color = SENTIMENT_COLORS["positive"]
                    elif sentiment < -0.2:
                        bg_color = SENTIMENT_COLORS["negative"]
                    else:
                        bg_color = SENTIMENT_COLORS["neutral"]
                    tooltip_text = f"{day_label} {hour:02d}:00 - {sentiment:.2f}"

                row_cells.append(
                    dmc.Tooltip(
                        dmc.Box(
                            w=12,
                            h=12,
                            style={"backgroundColor": bg_color, "borderRadius": "2px"},
                        ),
                        label=tooltip_text,
                        position="top",
                    )
                )

            rows.append(dmc.Group(row_cells, gap=2))

        content = dmc.Stack(rows, gap=2)

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Sentiment Heatmap", order=4),
                    dmc.Text("Average sentiment by hour and day", size="sm", c="dimmed"),
                ],
                justify="space-between",
                align="center",
            ),
            dmc.Space(h="md"),
            content,
            dmc.Space(h="sm"),
            # Legend
            dmc.Group(
                [
                    dmc.Text("Negative", size="xs", c="dimmed"),
                    dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["negative"], "borderRadius": "2px"}),
                    dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["neutral"], "borderRadius": "2px"}),
                    dmc.Box(w=12, h=12, style={"backgroundColor": SENTIMENT_COLORS["positive"], "borderRadius": "2px"}),
                    dmc.Text("Positive", size="xs", c="dimmed"),
                ],
                gap="xs",
            ),
        ],
        p="md",
        withBorder=True,
    )


def _create_user_ranking_card(
    users: list[dict],
    title: str,
    metric_key: str,
    colors: dict,
    is_positive: bool = True,
) -> dmc.Paper:
    """Create user ranking table card with confidence indicators."""
    from dash_iconify import DashIconify

    # Confidence color thresholds
    CONFIDENCE_HIGH = 0.8  # Green - reliable
    CONFIDENCE_MED = 0.5   # Amber - moderate
    AMBER_COLOR = "#f59e0b"

    if not users:
        content = dmc.Center(
            dmc.Text("No user data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            # Get smoothed sentiment (primary) and raw sentiment (secondary)
            smoothed = user.get("smoothed_sentiment", 0) or 0
            raw = user.get("raw_sentiment", 0) or 0
            confidence = user.get("confidence", 0) or 0
            msg_count = user.get("messages_analyzed", 0) or 0

            # Display value is smoothed sentiment
            display_value = f"{smoothed:+.2f}" if smoothed != 0 else "0.00"
            raw_display = f"Raw: {raw:+.2f}" if raw != 0 else "Raw: 0.00"

            # Determine color based on smoothed sentiment value
            if smoothed > 0:
                value_color = SENTIMENT_COLORS["positive"]
            elif smoothed < 0:
                value_color = SENTIMENT_COLORS["negative"]
            else:
                value_color = SENTIMENT_COLORS["neutral"]

            # Confidence indicator color
            if confidence >= CONFIDENCE_HIGH:
                conf_color = SENTIMENT_COLORS["positive"]
            elif confidence >= CONFIDENCE_MED:
                conf_color = AMBER_COLOR
            else:
                conf_color = SENTIMENT_COLORS["negative"]

            rows.append(
                dmc.Stack(
                    [
                        # Main row: rank, name, score, confidence ring
                        dmc.Group(
                            [
                                dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                                dmc.Text(
                                    user.get("first_name", "Unknown"),
                                    size="sm",
                                    style={"flex": 1},
                                ),
                                dmc.Text(
                                    display_value,
                                    size="sm",
                                    fw=600,
                                    c=value_color,
                                ),
                                dmc.Tooltip(
                                    dmc.RingProgress(
                                        sections=[{"value": confidence * 100, "color": conf_color}],
                                        size=32,
                                        thickness=3,
                                    ),
                                    label=f"{msg_count} messages analyzed",
                                    position="left",
                                ),
                            ],
                            justify="space-between",
                            gap="xs",
                        ),
                        # Secondary row: raw score
                        dmc.Group(
                            [
                                dmc.Text("", w=24),  # spacer for alignment
                                dmc.Text(raw_display, size="xs", c="dimmed"),
                            ],
                            gap="xs",
                        ),
                    ],
                    gap=2,
                )
            )

        content = dmc.Stack(rows, gap="sm")

    icon = "mdi:emoticon-happy-outline" if is_positive else "mdi:emoticon-sad-outline"
    icon_color = SENTIMENT_COLORS["positive"] if is_positive else SENTIMENT_COLORS["negative"]

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(icon=icon, width=20, color=icon_color),
                    dmc.Title(title, order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Sample-size adjusted sentiment score", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_toxicity_ranking_card(users: list[dict], colors: dict) -> dmc.Paper:
    """Create toxicity user ranking table card."""
    from dash_iconify import DashIconify

    if not users:
        content = dmc.Center(
            dmc.Text("No toxicity data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            toxicity_rate = user.get("toxicity_rate", 0) or 0
            msg_count = user.get("messages_analyzed", 0) or 0

            rows.append(
                dmc.Group(
                    [
                        dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                        dmc.Text(
                            user.get("first_name", "Unknown"),
                            size="sm",
                            style={"flex": 1},
                        ),
                        dmc.Text(
                            f"{toxicity_rate:.1f}%",
                            size="sm",
                            fw=600,
                            c=SENTIMENT_COLORS["negative"],
                        ),
                        dmc.Text(
                            f"{msg_count} msgs",
                            size="xs",
                            c="dimmed",
                        ),
                    ],
                    justify="space-between",
                    gap="xs",
                )
            )

        content = dmc.Stack(rows, gap="sm")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:alert-circle-outline",
                        width=20,
                        color=SENTIMENT_COLORS["negative"],
                    ),
                    dmc.Title("Most Toxic Users", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Users with highest toxicity rate", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )
