"""
Topics page for group statistics.

Topic and NER visualizations:
- Summary cards (topics, messages categorized, entities, top entity type)
- Topic distribution (horizontal bar chart)
- Entity type distribution (donut chart)
- Topic timeline (stacked area chart)
- Top entities table
- User topic interests table
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Entity type colors
ENTITY_COLORS = {
    "PERSON": "#3b82f6",  # Blue
    "ORG": "#8b5cf6",     # Purple
    "LOC": "#22c55e",     # Green
    "MISC": "#f59e0b",    # Amber
}

# Topic colors for timeline (reuse chart series colors)
TOPIC_SERIES_COLORS = [
    "#3b82f6",  # Blue
    "#8b5cf6",  # Purple
    "#22c55e",  # Green
    "#f59e0b",  # Amber
    "#ec4899",  # Pink
]


def create_topics_page(
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
    Create the topics analysis page for a group.

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
    topics_overview = queries.get_topics_overview(chat_id, start_date, end_date)
    ner_overview = queries.get_ner_overview(chat_id, start_date, end_date)
    topic_dist_df = queries.get_topic_distribution(chat_id, 10, start_date, end_date)
    ner_dist_df = queries.get_ner_distribution(chat_id, start_date, end_date)
    topic_timeline_df = queries.get_topic_timeline(chat_id, start_date, end_date, 5)
    top_entities = queries.get_top_entities(chat_id, None, 15, start_date, end_date)
    user_interests = queries.get_user_topic_interests(chat_id, 10, start_date, end_date)

    # Prepare topic distribution data for bar chart
    topic_dist_data = []
    if not topic_dist_df.empty:
        for _, row in topic_dist_df.iterrows():
            keywords = row["keywords"]
            # Handle both list and string formats
            if isinstance(keywords, list):
                label = ", ".join(keywords[:3])
            else:
                label = str(keywords)[:30]
            topic_dist_data.append({
                "topic": label,
                "messages": int(row["message_count"]),
            })

    # Prepare NER distribution data for donut chart
    ner_dist_data = []
    if not ner_dist_df.empty:
        for _, row in ner_dist_df.iterrows():
            entity_type = row["entity_type"]
            ner_dist_data.append({
                "name": entity_type,
                "value": int(row["count"]),
                "color": ENTITY_COLORS.get(entity_type, colors["muted"]),
            })

    # Prepare topic timeline data
    topic_timeline_data = []
    topic_series = []
    if not topic_timeline_df.empty:
        # Get unique topics and assign colors (dedupe by topic_id only since keywords is a list)
        seen_topics = set()
        topic_map = {}
        idx = 0
        for _, row in topic_timeline_df.iterrows():
            if row["topic_id"] in seen_topics:
                continue
            seen_topics.add(row["topic_id"])
            keywords = row["keywords"]
            if isinstance(keywords, list):
                label = ", ".join(keywords[:2])
            else:
                label = str(keywords)[:20]
            topic_map[row["topic_id"]] = label
            topic_series.append({
                "name": label,
                "color": TOPIC_SERIES_COLORS[idx % len(TOPIC_SERIES_COLORS)],
            })
            idx += 1

        # Pivot data by date
        dates = topic_timeline_df["date"].unique()
        for d in sorted(dates):
            day_data = {"date": d.strftime("%b %d")}
            day_rows = topic_timeline_df[topic_timeline_df["date"] == d]
            for _, row in day_rows.iterrows():
                label = topic_map.get(row["topic_id"], f"Topic {row['topic_id']}")
                day_data[label] = int(row["count"])
            topic_timeline_data.append(day_data)

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="topics",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Summary cards
            _create_summary_cards(topics_overview, ner_overview, colors),
            dmc.Space(h="lg"),
            # Topic distribution and NER distribution
            dbc.Row(
                [
                    dbc.Col(
                        _create_topic_distribution_card(topic_dist_data, colors),
                        md=8,
                    ),
                    dbc.Col(
                        _create_ner_distribution_card(ner_dist_data, colors),
                        md=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Topic timeline
            _create_topic_timeline_card(topic_timeline_data, topic_series, colors),
            dmc.Space(h="lg"),
            # Top entities and user interests
            dbc.Row(
                [
                    dbc.Col(
                        _create_top_entities_card(top_entities, colors),
                        md=6,
                    ),
                    dbc.Col(
                        _create_user_interests_card(user_interests, colors),
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
    topics_overview: dict,
    ner_overview: dict,
    colors: dict,
) -> dbc.Row:
    """Create summary stat cards row."""
    cards = [
        _create_stat_card(
            "Topics",
            f"{topics_overview.get('total_topics') or 0:,}",
            "topic clusters discovered",
            "mdi:tag-multiple-outline",
            colors,
        ),
        _create_stat_card(
            "Categorized",
            f"{topics_overview.get('total_messages_with_topics') or 0:,}",
            f"{topics_overview.get('coverage_rate') or 0}% coverage",
            "mdi:message-text-outline",
            colors,
        ),
        _create_stat_card(
            "Entities",
            f"{ner_overview.get('total_entities') or 0:,}",
            f"{ner_overview.get('unique_entities') or 0:,} unique",
            "mdi:account-box-outline",
            colors,
        ),
        _create_stat_card(
            "Top Type",
            ner_overview.get("top_entity_type") or "N/A",
            "most mentioned type",
            "mdi:trending-up",
            colors,
            value_color=ENTITY_COLORS.get(ner_overview.get("top_entity_type")),
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


def _create_topic_distribution_card(data: list, colors: dict) -> dmc.Paper:
    """Create topic distribution horizontal bar chart card."""
    if data:
        chart = dmc.BarChart(
            data=data,
            dataKey="topic",
            series=[{"name": "messages", "color": colors["primary"]}],
            h=300,
            orientation="vertical",
            gridColor=colors["border"],
            tickLine="none",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No topic data available", c="dimmed", size="sm"),
            h=300,
        )

    return dmc.Paper(
        [
            dmc.Title("Topic Distribution", order=4),
            dmc.Text("Messages per topic cluster", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            chart,
        ],
        p="md",
        withBorder=True,
    )


def _create_ner_distribution_card(data: list, colors: dict) -> dmc.Paper:
    """Create NER type distribution donut chart card."""
    if data:
        chart = dmc.DonutChart(
            data=data,
            h=220,
            tooltipDataSource="segment",
            chartLabel="Entities",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No entity data", c="dimmed", size="sm"),
            h=220,
        )

    return dmc.Paper(
        [
            dmc.Title("Entity Types", order=4),
            dmc.Text("Named entity distribution", size="xs", c="dimmed"),
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
                            dmc.Text(label, size="xs"),
                        ],
                        gap="xs",
                    )
                    for label, color in ENTITY_COLORS.items()
                ],
                gap="md",
                justify="center",
                wrap="wrap",
            ),
        ],
        p="md",
        withBorder=True,
    )


def _create_topic_timeline_card(
    data: list,
    series: list,
    colors: dict,
) -> dmc.Paper:
    """Create topic timeline stacked area chart card."""
    if data and series:
        chart = dmc.AreaChart(
            data=data,
            dataKey="date",
            series=series,
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
                    dmc.Title("Topic Trends", order=4),
                    dmc.Text("Top 5 topics over time", size="sm", c="dimmed"),
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


def _create_top_entities_card(entities: list[dict], colors: dict) -> dmc.Paper:
    """Create top entities table card."""
    if not entities:
        content = dmc.Center(
            dmc.Text("No entity data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, entity in enumerate(entities, 1):
            entity_type = entity.get("entity_type", "MISC")
            rows.append(
                dmc.Group(
                    [
                        dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                        dmc.Badge(
                            entity_type,
                            color=_get_mantine_color(entity_type),
                            size="sm",
                            variant="light",
                            w=60,
                        ),
                        dmc.Text(
                            entity.get("entity_text", "Unknown"),
                            size="sm",
                            flex=1,
                            truncate=True,
                        ),
                        dmc.Text(
                            f"{entity.get('count', 0):,}",
                            size="sm",
                            fw=600,
                        ),
                    ],
                    justify="space-between",
                    gap="xs",
                )
            )
        content = dmc.Stack(rows, gap="xs")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:account-box-multiple-outline",
                        width=20,
                        color=colors["primary"],
                    ),
                    dmc.Title("Top Entities", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Most mentioned people, organizations, and places", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _create_user_interests_card(users: list[dict], colors: dict) -> dmc.Paper:
    """Create user topic interests table card."""
    if not users:
        content = dmc.Center(
            dmc.Text("No user data available", c="dimmed", size="sm"),
            h=200,
        )
    else:
        rows = []
        for idx, user in enumerate(users, 1):
            keywords = user.get("top_topic_keywords", [])
            if isinstance(keywords, list):
                topic_label = ", ".join(keywords[:2])
            else:
                topic_label = str(keywords)[:25]

            rows.append(
                dmc.Group(
                    [
                        dmc.Text(f"{idx}.", size="sm", c="dimmed", w=24),
                        dmc.Text(
                            user.get("first_name", "Unknown"),
                            size="sm",
                            flex=1,
                        ),
                        dmc.Badge(
                            topic_label,
                            color="blue",
                            size="sm",
                            variant="light",
                        ),
                        dmc.Text(
                            f"{user.get('topic_message_count', 0):,}",
                            size="sm",
                            fw=600,
                        ),
                    ],
                    justify="space-between",
                    gap="xs",
                )
            )
        content = dmc.Stack(rows, gap="xs")

    return dmc.Paper(
        [
            dmc.Group(
                [
                    DashIconify(
                        icon="mdi:account-star-outline",
                        width=20,
                        color=colors["primary"],
                    ),
                    dmc.Title("User Interests", order=4),
                ],
                gap="xs",
            ),
            dmc.Text("Top topic per user", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            content,
        ],
        p="md",
        withBorder=True,
    )


def _get_mantine_color(entity_type: str) -> str:
    """Map entity type to Mantine color name."""
    color_map = {
        "PERSON": "blue",
        "ORG": "violet",
        "LOC": "green",
        "MISC": "orange",
    }
    return color_map.get(entity_type, "gray")
