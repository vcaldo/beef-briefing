"""
Dashboard callbacks for Dash interactivity.
"""

import calendar
import logging
import re
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

import plotly.graph_objects as go
from dash import Input, Output, State, ctx, dash_table, html, no_update
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

# Constants
TOP_REACTIONS_LIMIT = 20


def parse_date_range(
    start_date: Optional[str],
    end_date: Optional[str]
) -> Optional[tuple[datetime, datetime]]:
    """
    Parse date strings and return datetime range.
    End date is incremented by 1 day for inclusive range.
    Returns None if either date is missing or invalid.
    """
    if not start_date or not end_date:
        return None
    try:
        start = datetime.fromisoformat(start_date)
        end = datetime.fromisoformat(end_date) + timedelta(days=1)
        return start, end
    except (ValueError, TypeError):
        return None


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
            create_welcome_layout,
            create_login_required_layout,
            create_error_layout,
        )
        from src.auth.session import SessionData

        app_config = app.server.config.get("app_config")
        queries = app.server.config.get("queries")
        session_manager = app.server.config.get("session_manager")

        # Helper to get session (handles dev mode)
        def get_session():
            if app_config and app_config.is_development():
                logger.info("Development mode: bypassing authentication")
                return SessionData(
                    session_id="dev-session",
                    user_id=0,
                    username="dev_user",
                    first_name="Developer",
                    photo_url=None,
                    allowed_chat_ids="",  # Empty means all chats in dev
                    created_at=datetime.utcnow(),
                    expires_at=datetime.utcnow() + timedelta(days=365),
                    last_accessed_at=datetime.utcnow(),
                ), True  # is_dev

            session_id = request.cookies.get("dashboard_session")
            if not session_id or not session_manager:
                return None, False

            session_data = session_manager.get_session(session_id)
            return session_data, False

        # Route: Welcome page with chat cards
        if pathname in ["/beef-dashboard/", "/beef-dashboard"]:
            session_data, is_dev = get_session()

            if session_data is None:
                return create_login_required_layout()

            # Get chat card data
            chats = []
            if queries:
                if is_dev:
                    # Dev mode: show all chats
                    chats = queries.get_chat_card_data()
                else:
                    # Production: filter by allowed chat IDs
                    allowed_ids = session_data.get_allowed_chat_ids()
                    chats = queries.get_chat_card_data(chat_ids=allowed_ids)

            return create_welcome_layout(session=session_data, chats=chats)

        # Route: Analytics dashboard for specific chat
        chat_match = re.match(r"^/beef-dashboard/chat/(-?\d+)/?$", pathname or "")
        if chat_match:
            session_data, is_dev = get_session()

            if session_data is None:
                return create_login_required_layout()

            chat_id = int(chat_match.group(1))

            # Verify user has access to this chat (unless dev mode)
            if not is_dev:
                allowed_ids = session_data.get_allowed_chat_ids()
                if chat_id not in allowed_ids:
                    logger.warning(
                        "User attempted to access unauthorized chat",
                        extra={"user_id": session_data.user_id, "chat_id": chat_id}
                    )
                    return create_error_layout("Access denied")

            # Get chat info and available chats for the dropdown
            chats = []
            if queries:
                if is_dev:
                    chats = queries.get_available_chats()
                else:
                    allowed_ids = session_data.get_allowed_chat_ids()
                    chats = queries.get_available_chats(chat_ids=allowed_ids)

            # Verify the chat exists in the available chats
            chat_exists = any(c["id"] == chat_id for c in chats)
            if not chat_exists:
                return create_error_layout("Chat not found")

            return create_dashboard_layout(
                session=session_data,
                chats=chats,
                selected_chat_id=chat_id,
                show_back_link=True,
            )

        return create_error_layout("Page not found")

    @app.callback(
        [
            Output("stat-messages", "children"),
            Output("stat-users", "children"),
            Output("stat-reactions", "children"),
            Output("stat-media", "children"),
            Output("stat-msgday", "children"),
            Output("stat-topuser", "children"),
            Output("top-reactions-badges", "children"),
        ],
        [
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
        ],
    )
    def update_overview_stats(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
    ):
        """Update overview statistics cards and top reactions."""
        empty_result = ("--", "--", "--", "--", "--", "--", [])

        if not chat_id:
            return empty_result

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return empty_result

        queries = app.server.config.get("queries")
        if not queries:
            return empty_result

        try:
            start, end = date_range
            stats = queries.get_overview_stats(chat_id, start, end)
            top_reactions = queries.get_top_reactions(chat_id, start, end, limit=TOP_REACTIONS_LIMIT)

            # Build reaction badges
            badges = []
            for r in top_reactions:
                is_custom = r['emoji'] == '$'
                emoji_class = "reaction-emoji reaction-emoji--custom" if is_custom else "reaction-emoji"
                badges.append(
                    html.Span(
                        className="reaction-badge",
                        children=[
                            html.Span(r['emoji'], className=emoji_class),
                            html.Span(f"{r['count']:,}", className="reaction-count"),
                        ]
                    )
                )

            return (
                f"{stats['total_messages']:,}",
                f"{stats['active_users']:,}",
                f"{stats['total_reactions']:,}",
                f"{stats['media_count']:,}",
                f"{stats['msg_per_day']:.1f}",
                stats['top_user_name'],
                badges,
            )
        except Exception as e:
            logger.error(f"Error fetching overview stats: {e}")
            return empty_result

    @app.callback(
        Output("message-timeline-chart", "figure"),
        [
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
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

        if not chat_id:
            return fig

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start, end = date_range

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
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
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

        if not chat_id:
            return fig

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start, end = date_range
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
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
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

        if not chat_id:
            return fig

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start, end = date_range
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
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
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

        if not chat_id:
            return fig

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return fig

        queries = app.server.config.get("queries")
        if not queries:
            return fig

        try:
            start, end = date_range
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
        [
            Output("leaderboard-table", "children"),
            Output("pagination-info", "children"),
            Output("lb-prev", "disabled"),
            Output("lb-next", "disabled"),
            Output("leaderboard-page", "data"),
            Output("leaderboard-limit", "data"),
        ],
        [
            Input("selected-chat", "data"),
            Input("computed-start-date", "data"),
            Input("computed-end-date", "data"),
            Input("lb-10", "n_clicks"),
            Input("lb-25", "n_clicks"),
            Input("lb-50", "n_clicks"),
            Input("lb-prev", "n_clicks"),
            Input("lb-next", "n_clicks"),
        ],
        [
            State("leaderboard-page", "data"),
            State("leaderboard-limit", "data"),
        ],
    )
    def update_leaderboard(
        chat_id: Optional[int],
        start_date: Optional[str],
        end_date: Optional[str],
        clicks_10: int,
        clicks_25: int,
        clicks_50: int,
        clicks_prev: int,
        clicks_next: int,
        current_page: int,
        current_limit: int,
    ):
        """Update user leaderboard with DataTable (native sorting)."""

        # Initialize defaults
        page = current_page or 1
        limit = current_limit or 10

        # Determine what was triggered
        triggered = ctx.triggered_id

        # Handle limit button clicks - reset to page 1
        if triggered == "lb-10":
            limit = 10
            page = 1
        elif triggered == "lb-25":
            limit = 25
            page = 1
        elif triggered == "lb-50":
            limit = 50
            page = 1
        elif triggered == "lb-prev":
            page = max(1, page - 1)
        elif triggered == "lb-next":
            page = page + 1
        elif triggered in ["selected-chat", "computed-start-date", "computed-end-date"]:
            # Reset page on chat/date change
            page = 1

        empty_state = (
            html.Div("Select a chat and date range", className="empty-state"),
            "",
            True,
            True,
            1,
            limit,
        )

        if not chat_id:
            return empty_state

        date_range = parse_date_range(start_date, end_date)
        if not date_range:
            return empty_state

        queries = app.server.config.get("queries")
        if not queries:
            return (
                html.Div("Error loading data", className="error-state"),
                "",
                True,
                True,
                1,
                limit,
            )

        try:
            start, end = date_range

            # Get total count for pagination
            total = queries.get_user_rankings_total(chat_id, start, end)
            total_pages = max(1, (total + limit - 1) // limit)

            # Ensure page is valid
            page = min(page, total_pages)
            offset = (page - 1) * limit

            df = queries.get_user_rankings(chat_id, start, end, limit=limit, offset=offset)

            if df.empty:
                return (
                    html.Div("No data for selected period", className="empty-state"),
                    "",
                    True,
                    True,
                    1,
                    limit,
                )

            # Prepare data for DataTable
            records = []
            for idx, row in df.iterrows():
                rank = offset + idx + 1
                name = row["first_name"]
                if row["last_name"]:
                    name += f" {row['last_name']}"
                if row["username"]:
                    name += f" @{row['username']}"
                if row["is_premium"]:
                    name += " 💎"

                records.append({
                    "rank": rank,
                    "user": name,
                    "message_count": int(row["message_count"]),
                    "reactions_sent": int(row["reactions_sent"]),
                    "reactions_received": int(row["reactions_received"]),
                    "replies_sent": int(row["replies_sent"]),
                    "replies_received": int(row["replies_received"]),
                    "media_sent": int(row["media_sent"]),
                })

            # Dark theme styling for DataTable
            table = dash_table.DataTable(
                id="leaderboard-datatable",
                columns=[
                    {"name": "#", "id": "rank", "type": "numeric"},
                    {"name": "User", "id": "user", "type": "text"},
                    {"name": "Messages", "id": "message_count", "type": "numeric"},
                    {"name": "Reactions Given", "id": "reactions_sent", "type": "numeric"},
                    {"name": "Reactions Received", "id": "reactions_received", "type": "numeric"},
                    {"name": "Replies Sent", "id": "replies_sent", "type": "numeric"},
                    {"name": "Replies Received", "id": "replies_received", "type": "numeric"},
                    {"name": "Media Sent", "id": "media_sent", "type": "numeric"},
                ],
                data=records,
                sort_action="native",
                sort_mode="single",
                style_table={
                    "overflowX": "auto",
                },
                style_header={
                    "backgroundColor": "rgba(20, 24, 34, 0.9)",
                    "color": "#9aa0a6",
                    "fontWeight": "500",
                    "fontSize": "0.875rem",
                    "borderBottom": "1px solid rgba(255, 255, 255, 0.1)",
                    "textAlign": "left",
                    "padding": "12px 16px",
                },
                style_cell={
                    "backgroundColor": "transparent",
                    "color": "#e8eaed",
                    "fontFamily": "'JetBrains Mono', monospace",
                    "fontSize": "0.875rem",
                    "borderBottom": "1px solid rgba(255, 255, 255, 0.1)",
                    "padding": "12px 16px",
                    "textAlign": "right",
                },
                style_cell_conditional=[
                    {"if": {"column_id": "rank"}, "width": "60px", "textAlign": "center"},
                    {"if": {"column_id": "user"}, "textAlign": "left", "minWidth": "180px"},
                ],
                style_data_conditional=[
                    # Gold for rank 1
                    {
                        "if": {"filter_query": "{rank} = 1", "column_id": "rank"},
                        "color": "#ffd700",
                        "fontWeight": "600",
                    },
                    # Silver for rank 2
                    {
                        "if": {"filter_query": "{rank} = 2", "column_id": "rank"},
                        "color": "#c0c0c0",
                        "fontWeight": "600",
                    },
                    # Bronze for rank 3
                    {
                        "if": {"filter_query": "{rank} = 3", "column_id": "rank"},
                        "color": "#cd7f32",
                        "fontWeight": "600",
                    },
                    # Hover effect
                    {
                        "if": {"state": "active"},
                        "backgroundColor": "rgba(0, 217, 255, 0.1)",
                        "border": "none",
                    },
                ],
                style_as_list_view=True,
            )

            # Pagination info
            pagination_text = f"Page {page} of {total_pages} ({total} users)"

            return (
                table,
                pagination_text,
                page <= 1,  # Disable prev if on first page
                page >= total_pages,  # Disable next if on last page
                page,
                limit,
            )

        except Exception as e:
            logger.error(f"Error creating leaderboard: {e}")
            return (
                html.Div("Error loading leaderboard", className="error-state"),
                "",
                True,
                True,
                1,
                limit,
            )

    # Callback: Update dates when month/year dropdowns change
    @app.callback(
        [
            Output("computed-start-date", "data", allow_duplicate=True),
            Output("computed-end-date", "data", allow_duplicate=True),
            Output("selection-mode", "data", allow_duplicate=True),
        ],
        [
            Input("month-selector", "value"),
            Input("year-selector", "value"),
        ],
        prevent_initial_call=True,
    )
    def update_dates_from_month_year(month: int, year: int):
        """Compute date range from month/year selection."""
        if not month or not year:
            return no_update, no_update, no_update

        # First day of month
        start_date = datetime(year, month, 1).date()

        # Last day of month
        _, last_day = calendar.monthrange(year, month)
        end_date = datetime(year, month, last_day).date()

        return start_date.isoformat(), end_date.isoformat(), "month"

    # Callback: Update dates when period buttons are clicked
    @app.callback(
        [
            Output("computed-start-date", "data", allow_duplicate=True),
            Output("computed-end-date", "data", allow_duplicate=True),
            Output("selection-mode", "data", allow_duplicate=True),
            Output("period-24h", "className"),
            Output("period-7d", "className"),
            Output("period-30d", "className"),
            Output("period-90d", "className"),
            Output("period-180d", "className"),
            Output("period-365d", "className"),
            Output("period-ytd", "className"),
        ],
        [
            Input("period-24h", "n_clicks"),
            Input("period-7d", "n_clicks"),
            Input("period-30d", "n_clicks"),
            Input("period-90d", "n_clicks"),
            Input("period-180d", "n_clicks"),
            Input("period-365d", "n_clicks"),
            Input("period-ytd", "n_clicks"),
        ],
        prevent_initial_call=True,
    )
    def update_period(
        clicks_24h: int,
        clicks_7d: int,
        clicks_30d: int,
        clicks_90d: int,
        clicks_180d: int,
        clicks_365d: int,
        clicks_ytd: int,
    ):
        """Update date range based on period button selection."""
        triggered = ctx.triggered_id
        today = datetime.now().date()

        # Default classes (7 buttons)
        classes = ["period-tab"] * 7

        # Period mapping: (days_back, class_index)
        periods = {
            "period-24h": (1, 0),
            "period-7d": (7, 1),
            "period-30d": (30, 2),
            "period-90d": (90, 3),
            "period-180d": (180, 4),
            "period-365d": (365, 5),
            "period-ytd": ("ytd", 6),
        }

        if triggered in periods:
            days_or_ytd, idx = periods[triggered]
            classes[idx] = "period-tab active"

            if days_or_ytd == "ytd":
                # Year to date: Jan 1 of current year to today
                start = datetime(today.year, 1, 1).date()
            else:
                start = today - timedelta(days=days_or_ytd)
        else:
            # Default to 7d
            start = today - timedelta(days=7)
            classes[1] = "period-tab active"

        return start.isoformat(), today.isoformat(), "preset", *classes

    # Callback: Update visual styling based on selection mode
    @app.callback(
        Output("month-year-selector", "className"),
        Input("selection-mode", "data"),
    )
    def update_selector_styling(mode: str):
        """Update month/year selector styling based on selection mode."""
        if mode == "month":
            return "month-year-selector active-selector"
        return "month-year-selector dimmed"

    # Callback: Populate year dropdown with available years from database
    @app.callback(
        [
            Output("year-selector", "options"),
            Output("year-selector", "value"),
        ],
        Input("selected-chat", "data"),
    )
    def populate_year_options(chat_id: Optional[int]):
        """Populate year dropdown with years that have data for the selected chat."""
        if not chat_id:
            return [], None

        # Get queries from app config
        queries = app.server.config.get("queries")
        if not queries:
            current_year = datetime.now().year
            return [{"label": str(current_year), "value": current_year}], current_year

        # Ensure chat_id is int (might come as string from JSON store)
        chat_id = int(chat_id)

        years = queries.get_available_years(chat_id)
        if not years:
            # Fallback to current year if no data
            current_year = datetime.now().year
            return [{"label": str(current_year), "value": current_year}], current_year

        options = [{"label": str(y), "value": y} for y in years]
        # Default to most recent year
        return options, years[0]

    @app.callback(
        [
            Output("lb-10", "className"),
            Output("lb-25", "className"),
            Output("lb-50", "className"),
        ],
        [
            Input("lb-10", "n_clicks"),
            Input("lb-25", "n_clicks"),
            Input("lb-50", "n_clicks"),
        ],
        prevent_initial_call=True,
    )
    def update_leaderboard_buttons(clicks_10: int, clicks_25: int, clicks_50: int):
        """Update leaderboard button states."""
        triggered = ctx.triggered_id
        classes = ["lb-btn", "lb-btn", "lb-btn"]

        if triggered == "lb-10":
            classes[0] = "lb-btn active"
        elif triggered == "lb-25":
            classes[1] = "lb-btn active"
        elif triggered == "lb-50":
            classes[2] = "lb-btn active"
        else:
            classes[0] = "lb-btn active"

        return classes
