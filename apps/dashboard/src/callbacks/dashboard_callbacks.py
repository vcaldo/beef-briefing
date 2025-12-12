"""
Dashboard callbacks for Dash interactivity.
"""

import logging
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Tuple

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
from dash import Input, Output, State, callback, html, no_update
from flask import request

logger = logging.getLogger(__name__)

# Color scheme matching UX guidelines
COLORS = {
    "primary": "#00d9ff",
    "secondary": "#ff6b6b",
    "success": "#34d399",
    "warning": "#fbbf24",
    "bg": "#0a0e1a",
    "card_bg": "rgba(20, 24, 34, 0.7)",
    "text": "#e8eaed",
    "text_secondary": "#9aa0a6",
    "grid": "rgba(255, 255, 255, 0.05)",
}

# Chart layout defaults
CHART_LAYOUT = {
    "paper_bgcolor": "rgba(0,0,0,0)",
    "plot_bgcolor": "rgba(0,0,0,0)",
    "font": {"color": COLORS["text"], "family": "Space Grotesk, sans-serif"},
    "margin": {"l": 40, "r": 20, "t": 30, "b": 40},
    "xaxis": {
        "gridcolor": COLORS["grid"],
        "zerolinecolor": COLORS["grid"],
    },
    "yaxis": {
        "gridcolor": COLORS["grid"],
        "zerolinecolor": COLORS["grid"],
    },
}


def register_callbacks(app) -> None:
    """Register all dashboard callbacks."""

    @app.callback(
        Output("page-content", "children"),
        Input("url", "pathname"),
    )
    def display_page(pathname: str):
        """Route to appropriate page based on URL."""
        from src.components.layout import (
            create_dashboard_layout,
            create_login_required_layout,
            create_error_layout,
        )

        if pathname in ["/beef-dashboard/", "/beef-dashboard"]:
            # Check authentication
            session_id = request.cookies.get("dashboard_session")
            if not session_id:
                return create_login_required_layout()

            # Get session data
            session_manager = app.server.config.get("session_manager")
            queries = app.server.config.get("queries")

            if session_manager:
                session_data = session_manager.get_session(session_id)
                if not session_data:
                    return create_login_required_layout()

                # Get available chats
                chats = []
                if queries:
                    all_chats = queries.get_available_chats()
                    allowed_ids = session_data.get_allowed_chat_ids()
                    chats = [c for c in all_chats if c["id"] in allowed_ids]

                return create_dashboard_layout(session=session_data, chats=chats)

        return create_error_layout("Page not found")

    @app.callback(
        [
            Output("stat-messages", "children"),
            Output("stat-users", "children"),
            Output("stat-reactions", "children"),
            Output("stat-media", "children"),
        ],
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
        ],
    )
    def update_overview_stats(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ) -> Tuple[str, str, str, str]:
        """Update overview statistics cards."""
        if not chat_id or not start_date or not end_date:
            return "--", "--", "--", "--"

        queries = app.server.config.get("queries")
        if not queries:
            return "--", "--", "--", "--"

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            stats = queries.get_overview_stats(chat_id, start, end)

            return (
                f"{stats['total_messages']:,}",
                f"{stats['active_users']:,}",
                f"{stats['total_reactions']:,}",
                f"{stats['media_count']:,}",
            )
        except Exception as e:
            logger.error(f"Error fetching overview stats: {e}")
            return "--", "--", "--", "--"

    @app.callback(
        Output("message-timeline-chart", "figure"),
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
        ],
    )
    def update_timeline_chart(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ) -> go.Figure:
        """Update message timeline chart."""
        fig = go.Figure()
        fig.update_layout(**CHART_LAYOUT)

        if not chat_id or not start_date or not end_date:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            # Determine granularity based on date range
            days_diff = (end - start).days
            if days_diff <= 2:
                granularity = "hour"
            elif days_diff <= 31:
                granularity = "day"
            elif days_diff <= 90:
                granularity = "week"
            else:
                granularity = "month"

            df = queries.get_message_timeline(chat_id, start, end, granularity)

            if df.empty:
                return fig

            fig = go.Figure()

            # Messages line
            fig.add_trace(
                go.Scatter(
                    x=df["period"],
                    y=df["message_count"],
                    mode="lines+markers",
                    name="Messages",
                    line={"color": COLORS["primary"], "width": 2},
                    marker={"size": 6},
                    fill="tozeroy",
                    fillcolor="rgba(0, 217, 255, 0.1)",
                )
            )

            # Active users line
            fig.add_trace(
                go.Scatter(
                    x=df["period"],
                    y=df["user_count"],
                    mode="lines+markers",
                    name="Active Users",
                    line={"color": COLORS["secondary"], "width": 2},
                    marker={"size": 6},
                    yaxis="y2",
                )
            )

            fig.update_layout(
                **CHART_LAYOUT,
                showlegend=True,
                legend={"orientation": "h", "y": 1.1},
                yaxis2={
                    "overlaying": "y",
                    "side": "right",
                    "gridcolor": COLORS["grid"],
                    "title": "Users",
                },
                yaxis={"title": "Messages"},
                hovermode="x unified",
            )

            return fig

        except Exception as e:
            logger.error(f"Error creating timeline chart: {e}")
            return fig

    @app.callback(
        Output("activity-heatmap-chart", "figure"),
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
        ],
    )
    def update_heatmap_chart(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ) -> go.Figure:
        """Update activity heatmap chart."""
        fig = go.Figure()
        fig.update_layout(**CHART_LAYOUT)

        if not chat_id or not start_date or not end_date:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            df = queries.get_hourly_heatmap_data(chat_id, start, end)

            if df.empty:
                return fig

            # Pivot for heatmap
            pivot = df.pivot(
                index="day_of_week",
                columns="hour",
                values="message_count",
            ).fillna(0)

            # Day names
            day_names = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]

            fig = go.Figure(
                data=go.Heatmap(
                    z=pivot.values,
                    x=[f"{h:02d}:00" for h in range(24)],
                    y=[day_names[int(i)] for i in pivot.index],
                    colorscale=[
                        [0, "rgba(0, 217, 255, 0.1)"],
                        [0.5, "rgba(0, 217, 255, 0.5)"],
                        [1, COLORS["primary"]],
                    ],
                    showscale=False,
                    hovertemplate="<b>%{y} %{x}</b><br>Messages: %{z}<extra></extra>",
                )
            )

            fig.update_layout(
                **CHART_LAYOUT,
                xaxis={"dtick": 4, **CHART_LAYOUT["xaxis"]},
            )

            return fig

        except Exception as e:
            logger.error(f"Error creating heatmap: {e}")
            return fig

    @app.callback(
        Output("reaction-chart", "figure"),
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
        ],
    )
    def update_reaction_chart(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ) -> go.Figure:
        """Update reaction distribution chart."""
        fig = go.Figure()
        fig.update_layout(**CHART_LAYOUT)

        if not chat_id or not start_date or not end_date:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            df = queries.get_reaction_distribution(chat_id, start, end, limit=10)

            if df.empty:
                return fig

            fig = go.Figure(
                data=go.Bar(
                    x=df["count"],
                    y=df["emoji_value"],
                    orientation="h",
                    marker={"color": COLORS["primary"]},
                    hovertemplate="<b>%{y}</b><br>Count: %{x}<extra></extra>",
                )
            )

            fig.update_layout(
                **CHART_LAYOUT,
                yaxis={"categoryorder": "total ascending"},
            )

            return fig

        except Exception as e:
            logger.error(f"Error creating reaction chart: {e}")
            return fig

    @app.callback(
        Output("media-chart", "figure"),
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
        ],
    )
    def update_media_chart(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ) -> go.Figure:
        """Update media distribution chart."""
        fig = go.Figure()
        fig.update_layout(**CHART_LAYOUT)

        if not chat_id or not start_date or not end_date:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            df = queries.get_media_distribution(chat_id, start, end)

            if df.empty:
                return fig

            # Define colors for media types
            media_colors = {
                "photo": "#00d9ff",
                "video": "#ff6b6b",
                "audio": "#34d399",
                "voice": "#fbbf24",
                "document": "#a78bfa",
                "animation": "#f472b6",
                "video_note": "#60a5fa",
                "sticker": "#c084fc",
            }

            colors = [media_colors.get(mt, COLORS["text_secondary"]) for mt in df["media_type"]]

            fig = go.Figure(
                data=go.Pie(
                    labels=df["media_type"],
                    values=df["count"],
                    marker={"colors": colors},
                    hole=0.4,
                    hovertemplate="<b>%{label}</b><br>Count: %{value}<br>%{percent}<extra></extra>",
                )
            )

            fig.update_layout(
                **CHART_LAYOUT,
                showlegend=True,
                legend={"orientation": "h", "y": -0.1},
            )

            return fig

        except Exception as e:
            logger.error(f"Error creating media chart: {e}")
            return fig

    @app.callback(
        Output("leaderboard-table", "children"),
        [
            Input("chat-selector", "value"),
            Input("date-range-picker", "start_date"),
            Input("date-range-picker", "end_date"),
            Input("lb-10", "n_clicks"),
            Input("lb-20", "n_clicks"),
            Input("lb-50", "n_clicks"),
        ],
    )
    def update_leaderboard(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
        clicks_10: int,
        clicks_20: int,
        clicks_50: int,
    ) -> html.Div:
        """Update user leaderboard."""
        from dash import ctx

        # Determine limit based on which button was clicked
        triggered = ctx.triggered_id
        if triggered == "lb-20":
            limit = 20
        elif triggered == "lb-50":
            limit = 50
        else:
            limit = 10

        if not chat_id or not start_date or not end_date:
            return html.Div("Select a chat and date range", className="empty-state")

        queries = app.server.config.get("queries")
        if not queries:
            return html.Div("Error loading data", className="error-state")

        try:
            start = datetime.fromisoformat(start_date)
            end = datetime.fromisoformat(end_date) + timedelta(days=1)

            df = queries.get_user_rankings(chat_id, start, end, limit=limit)

            if df.empty:
                return html.Div("No data for selected period", className="empty-state")

            # Build leaderboard rows
            rows = []
            for idx, row in df.iterrows():
                rank = idx + 1
                rank_class = "rank-gold" if rank == 1 else ("rank-silver" if rank == 2 else ("rank-bronze" if rank == 3 else ""))

                name = row["first_name"]
                if row["last_name"]:
                    name += f" {row['last_name']}"

                username = f"@{row['username']}" if row['username'] else ""

                rows.append(
                    html.Tr(
                        className=f"leaderboard-row {rank_class}",
                        children=[
                            html.Td(f"#{rank}", className="rank-cell"),
                            html.Td(
                                children=[
                                    html.Span(name, className="user-name"),
                                    html.Span(username, className="username") if username else None,
                                    html.Span("⭐", className="premium-badge") if row["is_premium"] else None,
                                ],
                                className="name-cell",
                            ),
                            html.Td(f"{int(row['message_count']):,}", className="stat-cell"),
                            html.Td(f"{int(row['active_days'])}", className="stat-cell"),
                            html.Td(f"{int(row['reactions_received']):,}", className="stat-cell"),
                        ],
                    )
                )

            return html.Table(
                className="leaderboard-table",
                children=[
                    html.Thead(
                        html.Tr([
                            html.Th("Rank"),
                            html.Th("User"),
                            html.Th("Messages"),
                            html.Th("Active Days"),
                            html.Th("Reactions"),
                        ])
                    ),
                    html.Tbody(rows),
                ],
            )

        except Exception as e:
            logger.error(f"Error creating leaderboard: {e}")
            return html.Div("Error loading leaderboard", className="error-state")

    @app.callback(
        [
            Output("date-range-picker", "start_date"),
            Output("date-range-picker", "end_date"),
            Output("period-today", "className"),
            Output("period-week", "className"),
            Output("period-month", "className"),
            Output("period-quarter", "className"),
        ],
        [
            Input("period-today", "n_clicks"),
            Input("period-week", "n_clicks"),
            Input("period-month", "n_clicks"),
            Input("period-quarter", "n_clicks"),
        ],
        prevent_initial_call=True,
    )
    def update_period(
        today_clicks: int,
        week_clicks: int,
        month_clicks: int,
        quarter_clicks: int,
    ):
        """Update date range based on period selection."""
        from dash import ctx

        triggered = ctx.triggered_id
        today = datetime.now().date()

        # Default classes
        classes = ["period-tab", "period-tab", "period-tab", "period-tab"]

        if triggered == "period-today":
            start = today
            classes[0] = "period-tab active"
        elif triggered == "period-week":
            start = today - timedelta(days=7)
            classes[1] = "period-tab active"
        elif triggered == "period-month":
            start = today - timedelta(days=30)
            classes[2] = "period-tab active"
        elif triggered == "period-quarter":
            start = today - timedelta(days=90)
            classes[3] = "period-tab active"
        else:
            start = today - timedelta(days=7)
            classes[1] = "period-tab active"

        return start, today, *classes

    @app.callback(
        [
            Output("lb-10", "className"),
            Output("lb-20", "className"),
            Output("lb-50", "className"),
        ],
        [
            Input("lb-10", "n_clicks"),
            Input("lb-20", "n_clicks"),
            Input("lb-50", "n_clicks"),
        ],
        prevent_initial_call=True,
    )
    def update_leaderboard_buttons(clicks_10: int, clicks_20: int, clicks_50: int):
        """Update leaderboard button states."""
        from dash import ctx

        triggered = ctx.triggered_id
        classes = ["lb-btn", "lb-btn", "lb-btn"]

        if triggered == "lb-10":
            classes[0] = "lb-btn active"
        elif triggered == "lb-20":
            classes[1] = "lb-btn active"
        elif triggered == "lb-50":
            classes[2] = "lb-btn active"
        else:
            classes[0] = "lb-btn active"

        return classes
