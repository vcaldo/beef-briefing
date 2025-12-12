"""
Main dashboard layout component.
"""

from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

from dash import dcc, html

from src.auth.session import SessionData


def create_header(session: Optional[SessionData] = None) -> html.Div:
    """Create the dashboard header."""
    user_info = []
    if session:
        user_info = [
            html.Div(
                className="user-info",
                children=[
                    html.Img(
                        src=session.photo_url or "/beef-dashboard/assets/default-avatar.svg",
                        className="user-avatar",
                    ) if session.photo_url else html.Div(
                        className="user-avatar-placeholder",
                        children=session.first_name[0].upper(),
                    ),
                    html.Span(session.first_name, className="user-name"),
                    html.A("Logout", href="/beef-dashboard/logout", className="logout-link"),
                ],
            )
        ]

    return html.Header(
        className="dashboard-header",
        children=[
            html.Div(
                className="header-content",
                children=[
                    html.Div(
                        className="logo-section",
                        children=[
                            html.H1("Beef Dashboard", className="logo-text"),
                            html.Span("Analytics", className="logo-subtitle"),
                        ],
                    ),
                    html.Div(className="header-right", children=user_info),
                ],
            ),
        ],
    )


def create_period_selector() -> html.Div:
    """Create the time period selector."""
    return html.Div(
        className="period-selector",
        children=[
            html.Div(
                className="period-tabs",
                children=[
                    html.Button(
                        "Today",
                        id="period-today",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "Week",
                        id="period-week",
                        className="period-tab active",
                        n_clicks=0,
                    ),
                    html.Button(
                        "Month",
                        id="period-month",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "Quarter",
                        id="period-quarter",
                        className="period-tab",
                        n_clicks=0,
                    ),
                ],
            ),
            html.Div(
                className="date-picker-container",
                children=[
                    dcc.DatePickerRange(
                        id="date-range-picker",
                        start_date=(datetime.now() - timedelta(days=7)).date(),
                        end_date=datetime.now().date(),
                        display_format="MMM D, YYYY",
                        className="date-picker",
                    ),
                ],
            ),
        ],
    )


def truncate_title(title: str, max_length: int = 30) -> str:
    """Truncate title to max length with ellipsis."""
    if len(title) <= max_length:
        return title
    return title[:max_length - 1].rstrip() + "…"


def create_chat_selector(chats: List[Dict[str, Any]]) -> html.Div:
    """Create the chat/group selector dropdown."""
    options = [
        {
            "label": f"{truncate_title(chat['title'])} ({chat['message_count']:,} msgs)",
            "value": chat["id"],
            "title": chat["title"],  # Full title shown on hover
        }
        for chat in chats
    ]

    return html.Div(
        className="chat-selector",
        children=[
            html.Label("Group:", className="selector-label"),
            dcc.Dropdown(
                id="chat-selector",
                options=options,
                value=chats[0]["id"] if chats else None,
                clearable=False,
                className="chat-dropdown",
            ),
        ],
    )


def create_overview_cards() -> html.Div:
    """Create the overview statistics cards section."""
    return html.Div(
        className="overview-cards",
        children=[
            html.Div(
                className="stat-card",
                id="card-messages",
                children=[
                    html.Div(className="stat-icon messages-icon", children="💬"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Total Messages", className="stat-label"),
                            html.Span("--", id="stat-messages", className="stat-value"),
                        ],
                    ),
                ],
            ),
            html.Div(
                className="stat-card",
                id="card-users",
                children=[
                    html.Div(className="stat-icon users-icon", children="👥"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Active Users", className="stat-label"),
                            html.Span("--", id="stat-users", className="stat-value"),
                        ],
                    ),
                ],
            ),
            html.Div(
                className="stat-card",
                id="card-reactions",
                children=[
                    html.Div(className="stat-icon reactions-icon", children="❤️"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Reactions", className="stat-label"),
                            html.Span("--", id="stat-reactions", className="stat-value"),
                        ],
                    ),
                ],
            ),
            html.Div(
                className="stat-card",
                id="card-media",
                children=[
                    html.Div(className="stat-icon media-icon", children="📷"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Media Shared", className="stat-label"),
                            html.Span("--", id="stat-media", className="stat-value"),
                        ],
                    ),
                ],
            ),
        ],
    )


def create_main_charts() -> html.Div:
    """Create the main charts section."""
    return html.Div(
        className="charts-grid",
        children=[
            # Message Timeline (full width)
            html.Div(
                className="chart-container full-width",
                children=[
                    html.H3("Message Activity", className="chart-title"),
                    dcc.Loading(
                        id="loading-timeline",
                        type="circle",
                        color="#00d9ff",
                        children=[
                            dcc.Graph(
                                id="message-timeline-chart",
                                config={"displayModeBar": False},
                                className="chart",
                            ),
                        ],
                    ),
                ],
            ),
            # Activity Heatmap
            html.Div(
                className="chart-container",
                children=[
                    html.H3("Activity Patterns", className="chart-title"),
                    dcc.Loading(
                        id="loading-heatmap",
                        type="circle",
                        color="#00d9ff",
                        children=[
                            dcc.Graph(
                                id="activity-heatmap-chart",
                                config={"displayModeBar": False},
                                className="chart",
                            ),
                        ],
                    ),
                ],
            ),
            # Reaction Distribution
            html.Div(
                className="chart-container",
                children=[
                    html.H3("Top Reactions", className="chart-title"),
                    dcc.Loading(
                        id="loading-reactions",
                        type="circle",
                        color="#00d9ff",
                        children=[
                            dcc.Graph(
                                id="reaction-chart",
                                config={"displayModeBar": False},
                                className="chart",
                            ),
                        ],
                    ),
                ],
            ),
            # Media Distribution
            html.Div(
                className="chart-container",
                children=[
                    html.H3("Media Types", className="chart-title"),
                    dcc.Loading(
                        id="loading-media",
                        type="circle",
                        color="#00d9ff",
                        children=[
                            dcc.Graph(
                                id="media-chart",
                                config={"displayModeBar": False},
                                className="chart",
                            ),
                        ],
                    ),
                ],
            ),
            # User Leaderboard (full width)
            html.Div(
                className="chart-container full-width",
                children=[
                    html.H3("Top Contributors", className="chart-title"),
                    html.Div(
                        className="leaderboard-controls",
                        children=[
                            html.Button("Top 10", id="lb-10", className="lb-btn active", n_clicks=0),
                            html.Button("Top 20", id="lb-20", className="lb-btn", n_clicks=0),
                            html.Button("Top 50", id="lb-50", className="lb-btn", n_clicks=0),
                        ],
                    ),
                    dcc.Loading(
                        id="loading-leaderboard",
                        type="circle",
                        color="#00d9ff",
                        children=[
                            html.Div(id="leaderboard-table", className="leaderboard"),
                        ],
                    ),
                ],
            ),
        ],
    )


def create_dashboard_layout(
    session: Optional[SessionData] = None,
    chats: Optional[List[Dict[str, Any]]] = None,
) -> html.Div:
    """
    Create the complete dashboard layout.

    Args:
        session: Current user session data
        chats: List of available chats

    Returns:
        Dash HTML Div with complete layout
    """
    if chats is None:
        chats = []

    return html.Div(
        className="dashboard-wrapper",
        children=[
            create_header(session),
            html.Main(
                className="dashboard-main",
                children=[
                    html.Div(
                        className="controls-bar",
                        children=[
                            create_chat_selector(chats) if chats else html.Div(),
                            create_period_selector(),
                        ],
                    ),
                    create_overview_cards(),
                    create_main_charts(),
                ],
            ),
            html.Footer(
                className="dashboard-footer",
                children=[
                    html.Span("Beef Dashboard"),
                    html.Span(" · "),
                    html.Span("Powered by Plotly Dash"),
                ],
            ),
            # Hidden stores for state management
            dcc.Store(id="selected-period", data="week"),
            dcc.Store(id="selected-chat", data=chats[0]["id"] if chats else None),
            dcc.Store(id="leaderboard-limit", data=10),
        ],
    )


def create_login_required_layout() -> html.Div:
    """Create layout shown when user is not authenticated."""
    return html.Div(
        className="login-required",
        children=[
            html.Div(
                className="login-card",
                children=[
                    html.H1("Beef Dashboard"),
                    html.P("Please log in to access the dashboard."),
                    html.A(
                        "Login with Telegram",
                        href="/beef-dashboard/login",
                        className="login-button",
                    ),
                ],
            ),
        ],
    )


def create_error_layout(message: str) -> html.Div:
    """Create error layout."""
    return html.Div(
        className="error-container",
        children=[
            html.Div(
                className="error-card",
                children=[
                    html.H2("Oops!"),
                    html.P(message),
                    html.A("Go Back", href="/beef-dashboard/", className="back-link"),
                ],
            ),
        ],
    )
