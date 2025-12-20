"""
Reactions page for group statistics.

Reaction analytics:
- Summary cards (total reactions, unique emoji, reactions/message)
- Emoji distribution (horizontal bar chart)
- Type breakdown (donut: emoji/custom_emoji/paid)
- Top reactors/receivers tables
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates
from src.utils import format_number


def create_reactions_page(
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
    Create the reactions page for a group.

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

    # Fetch stats
    stats = queries.get_overview_stats(chat_id, start_date, end_date)
    total_reactions = stats.get("total_reactions", 0)
    total_messages = stats.get("total_messages", 0)

    # Fetch reaction distribution
    reaction_df = queries.get_reaction_distribution(chat_id, start_date, end_date)

    # Calculate summary stats
    unique_emoji = len(reaction_df) if not reaction_df.empty else 0
    reactions_per_msg = (
        round(total_reactions / total_messages, 2) if total_messages > 0 else 0
    )

    # Prepare reaction bar data (top 10)
    reaction_bar_data = []
    if not reaction_df.empty:
        top_reactions = reaction_df.head(10)
        for _, row in top_reactions.iterrows():
            reaction_bar_data.append(
                {"emoji": row["emoji"], "count": int(row["count"])}
            )

    # Prepare type breakdown data
    type_data = []
    if not reaction_df.empty:
        type_counts = reaction_df.groupby("reaction_type")["count"].sum()
        for rtype, count in type_counts.items():
            label = {
                "emoji": "Standard Emoji",
                "custom_emoji": "Custom Emoji",
                "paid": "Paid Reactions",
            }.get(rtype, rtype)
            type_data.append({"name": label, "value": int(count), "color": colors["primary"]})

    # Fetch top reactors and receivers
    top_reactors = queries.get_user_rankings(
        chat_id,
        metric="reactions_sent",
        limit=10,
        start_date=start_date,
        end_date=end_date,
    )
    top_receivers = queries.get_user_rankings(
        chat_id,
        metric="reactions_received",
        limit=10,
        start_date=start_date,
        end_date=end_date,
    )

    # Fetch profile photos for all users
    all_user_ids = set()
    for u in top_reactors:
        all_user_ids.add(u["user_id"])
    for u in top_receivers:
        all_user_ids.add(u["user_id"])
    photo_urls = photo_client.get_user_photos_batch(list(all_user_ids), size="small")

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="reactions",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Summary cards
            dmc.SimpleGrid(
                [
                    _create_summary_card("Total Reactions", format_number(total_reactions)),
                    _create_summary_card("Unique Emoji", str(unique_emoji)),
                    _create_summary_card("Reactions/Message", f"{reactions_per_msg:.2f}"),
                ],
                cols={"base": 1, "sm": 3},
                spacing="md",
            ),
            dmc.Space(h="xl"),
            # Charts row
            dbc.Row(
                [
                    dbc.Col(
                        _create_reactions_bar_card(reaction_bar_data, colors),
                        lg=8,
                    ),
                    dbc.Col(
                        _create_type_breakdown_card(type_data, colors),
                        lg=4,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="xl"),
            # Top reactors/receivers
            _create_user_tables_card(top_reactors, top_receivers, colors, photo_urls),
        ],
        fluid=True,
        className="py-4",
    )


def _create_summary_card(label: str, value: str) -> dmc.Paper:
    """Create a summary stat card."""
    return dmc.Paper(
        dmc.Stack(
            [
                dmc.Text(label, size="sm", c="dimmed"),
                dmc.Title(value, order=2),
            ],
            gap="xs",
        ),
        p="md",
        withBorder=True,
    )


def _create_reactions_bar_card(data: list, colors: dict) -> dmc.Paper:
    """Create horizontal bar chart for top reactions."""
    if data:
        # Build horizontal bars manually since DMC BarChart is vertical
        max_count = data[0]["count"] if data else 1
        rows = []
        for item in data:
            pct = (item["count"] / max_count) * 100 if max_count > 0 else 0
            rows.append(
                dmc.Group(
                    [
                        dmc.Text(
                            item["emoji"], size="xl", miw=40
                        ),
                        dmc.Box(
                            dmc.Progress(
                                value=pct,
                                size="lg",
                                radius="xl",
                                color=colors["primary"],
                            ),
                            style={"flex": 1},
                        ),
                        dmc.Text(
                            format_number(item["count"]),
                            size="sm",
                            fw=500,
                            miw=60,
                            ta="right",
                        ),
                    ],
                    gap="sm",
                )
            )
        content = dmc.Stack(rows, gap="sm")
    else:
        content = dmc.Center(
            dmc.Text("No reaction data", c="dimmed", size="sm"),
            h=200,
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


def _create_type_breakdown_card(data: list, colors: dict) -> dmc.Paper:
    """Create donut chart for reaction type breakdown."""
    if data:
        chart = dmc.DonutChart(
            data=data,
            h=200,
            tooltipDataSource="segment",
            chartLabel="Types",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No type data", c="dimmed", size="sm"),
            h=200,
        )

    return dmc.Paper(
        [
            dmc.Title("Type Breakdown", order=4),
            dmc.Space(h="md"),
            chart,
        ],
        p="md",
        withBorder=True,
    )


def _create_user_tables_card(
    top_reactors: list, top_receivers: list, colors: dict, photo_urls: dict
) -> dmc.Paper:
    """Create tabbed tables for top reactors and receivers."""

    def create_user_rows(users: list, score_label: str) -> list:
        if not users:
            return [
                dmc.TableTr(
                    dmc.TableTd(
                        dmc.Text("No data", c="dimmed", size="sm"),
                        colSpan=3,
                        style={"textAlign": "center"},
                    )
                )
            ]

        rows = []
        for user in users:
            rows.append(
                dmc.TableTr(
                    [
                        dmc.TableTd(
                            dmc.Badge(f"#{user['rank']}", variant="light", size="sm")
                        ),
                        dmc.TableTd(
                            dmc.Group(
                                [
                                    dmc.Avatar(
                                        src=photo_urls.get(user["user_id"]),
                                        children=user["first_name"][0].upper()
                                        if user["first_name"]
                                        else "?",
                                        size="sm",
                                        radius="xl",
                                    ),
                                    dmc.Text(
                                        user["first_name"] or "Unknown",
                                        size="sm",
                                    ),
                                ],
                                gap="xs",
                            )
                        ),
                        dmc.TableTd(
                            dmc.Text(
                                format_number(user["score"]),
                                size="sm",
                                fw=500,
                            ),
                            style={"textAlign": "right"},
                        ),
                    ]
                )
            )
        return rows

    reactors_table = dmc.Table(
        striped=True,
        highlightOnHover=True,
        children=[
            dmc.TableThead(
                dmc.TableTr(
                    [
                        dmc.TableTh("#", style={"width": "50px"}),
                        dmc.TableTh("User"),
                        dmc.TableTh("Sent", style={"textAlign": "right"}),
                    ]
                )
            ),
            dmc.TableTbody(create_user_rows(top_reactors, "Sent")),
        ],
    )

    receivers_table = dmc.Table(
        striped=True,
        highlightOnHover=True,
        children=[
            dmc.TableThead(
                dmc.TableTr(
                    [
                        dmc.TableTh("#", style={"width": "50px"}),
                        dmc.TableTh("User"),
                        dmc.TableTh("Received", style={"textAlign": "right"}),
                    ]
                )
            ),
            dmc.TableTbody(create_user_rows(top_receivers, "Received")),
        ],
    )

    return dmc.Paper(
        [
            dmc.Tabs(
                [
                    dmc.TabsList(
                        [
                            dmc.TabsTab("Top Reactors", value="reactors"),
                            dmc.TabsTab("Top Receivers", value="receivers"),
                        ]
                    ),
                    dmc.TabsPanel(reactors_table, value="reactors", pt="md"),
                    dmc.TabsPanel(receivers_table, value="receivers", pt="md"),
                ],
                value="reactors",
            ),
        ],
        p="md",
        withBorder=True,
    )
