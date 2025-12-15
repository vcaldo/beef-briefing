"""
Activity page for group statistics.

Time-based analytics:
- Message timeline (area chart)
- Hourly distribution (bar chart)
- Day of week distribution (bar chart)
- Activity heatmap (hour x day matrix)
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates

# Day labels for heatmap
DAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]


def create_activity_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
) -> html.Div:
    """
    Create the activity page for a group.

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
    colors = get_chart_colors(theme_name)

    # Get date range
    start_date, end_date = get_period_dates(period)

    # Fetch data
    daily_df = queries.get_daily_activity(chat_id, start_date, end_date)
    hourly_df = queries.get_hourly_activity_pattern(chat_id, start_date, end_date)
    dow_df = queries.get_day_of_week_pattern(chat_id, start_date, end_date)
    heatmap_df = queries.get_hourly_heatmap_data(chat_id, start_date, end_date)

    # Prepare timeline data
    timeline_data = []
    if not daily_df.empty:
        for _, row in daily_df.iterrows():
            timeline_data.append(
                {
                    "date": row["date"].strftime("%b %d"),
                    "messages": int(row["message_count"]),
                    "users": int(row["unique_users"]),
                }
            )

    # Prepare hourly data
    hourly_data = []
    for hour in range(24):
        matching = hourly_df[hourly_df["hour"] == hour] if not hourly_df.empty else None
        count = int(matching["message_count"].iloc[0]) if matching is not None and len(matching) > 0 else 0
        hourly_data.append({"hour": f"{hour:02d}", "messages": count})

    # Prepare day of week data
    dow_data = []
    for day in range(7):
        matching = dow_df[dow_df["day_of_week"] == day] if not dow_df.empty else None
        count = int(matching["message_count"].iloc[0]) if matching is not None and len(matching) > 0 else 0
        dow_data.append({"day": DAY_LABELS[day], "messages": count})

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="activity",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
            ),
            dmc.Space(h="xl"),
            # Message timeline
            _create_timeline_card(timeline_data, colors),
            dmc.Space(h="lg"),
            # Distribution charts
            dbc.Row(
                [
                    dbc.Col(
                        _create_hourly_card(hourly_data, colors),
                        md=6,
                    ),
                    dbc.Col(
                        _create_dow_card(dow_data, colors),
                        md=6,
                    ),
                ],
                className="g-3",
            ),
            dmc.Space(h="lg"),
            # Heatmap
            _create_heatmap_card(heatmap_df, colors),
        ],
        fluid=True,
        className="py-4",
    )


def _create_timeline_card(data: list, colors: dict) -> dmc.Paper:
    """Create message timeline chart card."""
    if data:
        chart = dmc.AreaChart(
            data=data,
            dataKey="date",
            series=[
                {"name": "messages", "color": colors["primary"]},
                {"name": "users", "color": colors["accent"]},
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
            dmc.Text("No activity data for this period", c="dimmed", size="sm"),
            h=250,
        )

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Message Timeline", order=4),
                    dmc.Text("Daily messages and unique users", size="sm", c="dimmed"),
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


def _create_hourly_card(data: list, colors: dict) -> dmc.Paper:
    """Create hourly distribution chart card."""
    if data and any(d["messages"] > 0 for d in data):
        chart = dmc.BarChart(
            data=data,
            dataKey="hour",
            series=[{"name": "messages", "color": colors["primary"]}],
            h=200,
            gridColor=colors["border"],
            tickLine="none",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No hourly data", c="dimmed", size="sm"),
            h=200,
        )

    return dmc.Paper(
        [
            dmc.Title("Hourly Distribution", order=4),
            dmc.Text("Messages by hour of day (0-23)", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            chart,
        ],
        p="md",
        withBorder=True,
    )


def _create_dow_card(data: list, colors: dict) -> dmc.Paper:
    """Create day of week distribution chart card."""
    if data and any(d["messages"] > 0 for d in data):
        chart = dmc.BarChart(
            data=data,
            dataKey="day",
            series=[{"name": "messages", "color": colors["accent"]}],
            h=200,
            gridColor=colors["border"],
            tickLine="none",
        )
    else:
        chart = dmc.Center(
            dmc.Text("No weekly data", c="dimmed", size="sm"),
            h=200,
        )

    return dmc.Paper(
        [
            dmc.Title("Day of Week", order=4),
            dmc.Text("Messages by day of week", size="xs", c="dimmed"),
            dmc.Space(h="md"),
            chart,
        ],
        p="md",
        withBorder=True,
    )


def _create_heatmap_card(heatmap_df, colors: dict) -> dmc.Paper:
    """Create activity heatmap card using custom grid."""
    # Build heatmap grid data
    # heatmap_df has columns: day_of_week (0-6), hour (0-23), message_count

    if heatmap_df.empty:
        content = dmc.Center(
            dmc.Text("No heatmap data", c="dimmed", size="sm"),
            h=200,
        )
    else:
        # Create matrix: 7 days x 24 hours
        matrix = [[0 for _ in range(24)] for _ in range(7)]
        max_count = 1

        for _, row in heatmap_df.iterrows():
            day = int(row["day_of_week"])
            hour = int(row["hour"])
            count = int(row["message_count"])
            if 0 <= day < 7 and 0 <= hour < 24:
                matrix[day][hour] = count
                max_count = max(max_count, count)

        # Build grid cells
        rows = []

        # Header row with hour labels
        header_cells = [
            dmc.Text("", size="xs", w=40, ta="center")
        ]
        for hour in range(0, 24, 3):  # Show every 3 hours
            header_cells.append(
                dmc.Text(
                    f"{hour:02d}",
                    size="xs",
                    c="dimmed",
                    w=36,
                    ta="center",
                )
            )
        rows.append(dmc.Group(header_cells, gap=2))

        # Data rows
        for day_idx, day_label in enumerate(DAY_LABELS):
            row_cells = [
                dmc.Text(
                    day_label,
                    size="xs",
                    c="dimmed",
                    w=40,
                    ta="right",
                    pr=4,
                )
            ]

            for hour in range(24):
                count = matrix[day_idx][hour]
                # Calculate intensity (0-1)
                intensity = count / max_count if max_count > 0 else 0

                # Interpolate color from surface (0) to primary (1)
                if intensity == 0:
                    bg_color = colors["surface"]
                elif intensity < 0.33:
                    bg_color = colors["muted"]
                elif intensity < 0.66:
                    bg_color = colors["accent"]
                else:
                    bg_color = colors["primary"]

                row_cells.append(
                    dmc.Tooltip(
                        dmc.Box(
                            w=12,
                            h=12,
                            bg=bg_color,
                            style={"borderRadius": "2px"},
                        ),
                        label=f"{day_label} {hour:02d}:00 - {count} messages",
                        position="top",
                    )
                )

            rows.append(dmc.Group(row_cells, gap=2))

        content = dmc.Stack(rows, gap=2)

    return dmc.Paper(
        [
            dmc.Group(
                [
                    dmc.Title("Activity Heatmap", order=4),
                    dmc.Text("Messages by hour and day of week", size="sm", c="dimmed"),
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
                    dmc.Text("Less", size="xs", c="dimmed"),
                    dmc.Box(
                        w=12,
                        h=12,
                        bg=colors["surface"],
                        style={"borderRadius": "2px", "border": f"1px solid {colors['border']}"},
                    ),
                    dmc.Box(
                        w=12,
                        h=12,
                        bg=colors["muted"],
                        style={"borderRadius": "2px"},
                    ),
                    dmc.Box(
                        w=12,
                        h=12,
                        bg=colors["accent"],
                        style={"borderRadius": "2px"},
                    ),
                    dmc.Box(
                        w=12,
                        h=12,
                        bg=colors["primary"],
                        style={"borderRadius": "2px"},
                    ),
                    dmc.Text("More", size="xs", c="dimmed"),
                ],
                gap="xs",
            ),
        ],
        p="md",
        withBorder=True,
    )
