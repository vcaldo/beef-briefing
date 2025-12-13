"""
Main dashboard layout component.
"""

from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List, Optional

from dash import dcc, html

from src.auth.session import SessionData


def create_header(
    session: Optional[SessionData] = None,
    show_back_link: bool = False,
) -> html.Div:
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

    # Back link for analytics pages
    back_link = []
    if show_back_link:
        back_link = [
            html.A(
                "← Back to Groups",
                href="/beef-dashboard/",
                className="back-link"
            ),
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
                            *back_link,
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
    """Create the time period selector with preset buttons and month/year dropdowns."""
    today = datetime.now()
    current_month = today.month

    # Month options (all 12 months, always available)
    month_options = [
        {"label": "Jan", "value": 1},
        {"label": "Feb", "value": 2},
        {"label": "Mar", "value": 3},
        {"label": "Apr", "value": 4},
        {"label": "May", "value": 5},
        {"label": "Jun", "value": 6},
        {"label": "Jul", "value": 7},
        {"label": "Aug", "value": 8},
        {"label": "Sep", "value": 9},
        {"label": "Oct", "value": 10},
        {"label": "Nov", "value": 11},
        {"label": "Dec", "value": 12},
    ]

    # Year options: populated dynamically by callback based on available data

    return html.Div(
        className="period-selector",
        children=[
            html.Div(
                className="period-tabs",
                children=[
                    html.Button(
                        "24h",
                        id="period-24h",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "7d",
                        id="period-7d",
                        className="period-tab active",
                        n_clicks=0,
                    ),
                    html.Button(
                        "30d",
                        id="period-30d",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "90d",
                        id="period-90d",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "180d",
                        id="period-180d",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "365d",
                        id="period-365d",
                        className="period-tab",
                        n_clicks=0,
                    ),
                    html.Button(
                        "YTD",
                        id="period-ytd",
                        className="period-tab",
                        n_clicks=0,
                    ),
                ],
            ),
            html.Div(
                id="month-year-selector",
                className="month-year-selector",
                children=[
                    dcc.Dropdown(
                        id="month-selector",
                        options=month_options,
                        value=current_month,
                        clearable=False,
                        searchable=False,
                        className="month-dropdown",
                    ),
                    dcc.Dropdown(
                        id="year-selector",
                        options=[],  # Populated dynamically by callback
                        value=None,
                        clearable=False,
                        searchable=False,
                        className="year-dropdown",
                    ),
                ],
            ),
        ],
    )


def truncate_title(title: str, max_length: int = 30) -> str:
    if len(title) <= max_length:
        return title
    return title[:max_length - 1].rstrip() + "…"


def create_chat_selector(
    chats: List[Dict[str, Any]],
    selected_id: Optional[int] = None,
) -> html.Div:
    """Create the chat/group selector dropdown."""
    options = [
        {
            "label": f"{truncate_title(chat['title'])} ({chat['message_count']:,} msgs)",
            "value": chat["id"],
            "title": chat["title"],  # Full title shown on hover
        }
        for chat in chats
    ]

    # Determine default value
    default_value = selected_id
    if default_value is None and chats:
        default_value = chats[0]["id"]

    return html.Div(
        className="chat-selector",
        children=[
            html.Label("Group:", className="selector-label"),
            dcc.Dropdown(
                id="chat-selector",
                options=options,
                value=default_value,
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
            # 1. Top User
            html.Div(
                className="stat-card",
                id="card-topuser",
                children=[
                    html.Div(className="stat-icon topuser-icon", children="🏆"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Top User", className="stat-label"),
                            html.Span("--", id="stat-topuser", className="stat-value"),
                        ],
                    ),
                ],
            ),
            # 2. Total Messages
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
            # 3. Active Users
            html.Div(
                className="stat-card",
                id="card-users",
                children=[
                    html.Div(className="stat-icon users-icon", children="👪"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Active Users", className="stat-label"),
                            html.Span("--", id="stat-users", className="stat-value"),
                        ],
                    ),
                ],
            ),
            # 4. Reactions
            html.Div(
                className="stat-card",
                id="card-reactions",
                children=[
                    html.Div(className="stat-icon reactions-icon", children="🔥"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Reactions", className="stat-label"),
                            html.Span("--", id="stat-reactions", className="stat-value"),
                        ],
                    ),
                ],
            ),
            # 5. Msg/Day
            html.Div(
                className="stat-card",
                id="card-msgday",
                children=[
                    html.Div(className="stat-icon msgday-icon", children="📈"),
                    html.Div(
                        className="stat-content",
                        children=[
                            html.Span("Msg/Day", className="stat-label"),
                            html.Span("--", id="stat-msgday", className="stat-value"),
                        ],
                    ),
                ],
            ),
            # 6. Media Shared
            html.Div(
                className="stat-card",
                id="card-media",
                children=[
                    html.Div(className="stat-icon media-icon", children="📎"),
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


def create_top_reactions_row() -> html.Div:
    """Create the top reactions row with emoji badges."""
    return html.Div(
        className="top-reactions-row",
        children=[
            html.Span("Top Reactions:", className="reactions-label"),
            html.Div(id="top-reactions-badges", className="reactions-badges"),
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
        ],
    )


def create_top_users_section() -> html.Div:
    """Create the top users leaderboard section."""
    return html.Div(
        className="top-users-section",
        children=[
            html.Div(
                className="chart-container full-width",
                children=[
                    html.H3("Top Users", className="chart-title"),
                    html.Div(
                        className="leaderboard-controls",
                        children=[
                            html.Div(
                                className="limit-controls",
                                children=[
                                    html.Button("Top 10", id="lb-10", className="lb-btn active", n_clicks=0),
                                    html.Button("Top 25", id="lb-25", className="lb-btn", n_clicks=0),
                                    html.Button("Top 50", id="lb-50", className="lb-btn", n_clicks=0),
                                ],
                            ),
                            html.Div(
                                className="pagination-controls",
                                children=[
                                    html.Button("← Prev", id="lb-prev", className="lb-btn pagination-btn", n_clicks=0),
                                    html.Span(id="pagination-info", className="pagination-info"),
                                    html.Button("Next →", id="lb-next", className="lb-btn pagination-btn", n_clicks=0),
                                ],
                            ),
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
    selected_chat_id: Optional[int] = None,
    show_back_link: bool = False,
) -> html.Div:
    """
    Create the complete dashboard layout.

    Args:
        session: Current user session data
        chats: List of available chats
        selected_chat_id: ID of the currently selected chat
        show_back_link: Whether to show back navigation link

    Returns:
        Dash HTML Div with complete layout
    """
    if chats is None:
        chats = []

    # Determine which chat to select
    default_chat_id = selected_chat_id
    if default_chat_id is None and chats:
        default_chat_id = chats[0]["id"]

    # Calculate default dates (last 7 days)
    today = datetime.now().date()
    default_start = (today - timedelta(days=7)).isoformat()
    default_end = today.isoformat()

    return html.Div(
        className="dashboard-wrapper",
        children=[
            create_header(session, show_back_link=show_back_link),
            html.Main(
                className="dashboard-main",
                children=[
                    html.Div(
                        className="controls-bar",
                        children=[
                            create_period_selector(),
                        ],
                    ),
                    create_overview_cards(),
                    create_top_reactions_row(),
                    create_top_users_section(),
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
            dcc.Store(id="selected-period", data="7d"),
            dcc.Store(id="selected-chat", data=default_chat_id),
            dcc.Store(id="leaderboard-limit", data=10),
            dcc.Store(id="leaderboard-page", data=1),
            # Date computation stores
            dcc.Store(id="computed-start-date", data=default_start),
            dcc.Store(id="computed-end-date", data=default_end),
            dcc.Store(id="selection-mode", data="preset"),  # "preset" or "month"
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
                    html.Div("🥩", className="login-logo"),
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


def format_last_activity(last_activity: Optional[datetime]) -> str:
    """Format last activity timestamp for display."""
    if not last_activity:
        return "No activity"

    now = datetime.now()
    if last_activity.tzinfo:
        now = datetime.now(timezone.utc)

    diff = now - last_activity

    if diff.days == 0:
        hours = diff.seconds // 3600
        if hours == 0:
            minutes = diff.seconds // 60
            return f"{minutes}m ago" if minutes > 0 else "Just now"
        return f"{hours}h ago"
    elif diff.days == 1:
        return "Yesterday"
    elif diff.days < 7:
        return f"{diff.days}d ago"
    else:
        return last_activity.strftime("%b %d, %Y")


def create_chat_card(chat: Dict[str, Any]) -> html.A:
    """
    Create a single chat card for the welcome page.

    Args:
        chat: Chat data dictionary with id, type, title, message_count,
              user_count, last_activity, avg_messages_per_day
    """
    chat_type = chat.get("type", "group")
    is_supergroup = chat_type == "supergroup"

    # Icon based on chat type (building for supergroup, users for group)
    icon = "🏛" if is_supergroup else "👥"

    # Type badge
    type_badge = html.Span(
        chat_type,
        className=f"chat-type-badge {'supergroup' if is_supergroup else 'group'}"
    )

    # Format last activity
    last_activity_str = format_last_activity(chat.get("last_activity"))

    return html.A(
        href=f"/beef-dashboard/chat/{chat['id']}",
        className="chat-card",
        children=[
            html.Div(
                className="chat-card-header",
                children=[
                    html.Span(icon, className="chat-icon"),
                    html.H3(
                        truncate_title(chat.get("title", "Unknown Chat"), 35),
                        className="chat-title",
                        title=chat.get("title", ""),  # Full title on hover
                    ),
                    type_badge,
                ],
            ),
            html.Div(className="chat-card-divider"),
            html.Div(
                className="chat-card-stats",
                children=[
                    html.Div(
                        className="chat-stat",
                        children=[
                            html.Span("Messages", className="chat-stat-label"),
                            html.Span(
                                f"{chat.get('message_count', 0):,}",
                                className="chat-stat-value"
                            ),
                        ],
                    ),
                    html.Div(
                        className="chat-stat",
                        children=[
                            html.Span("Users", className="chat-stat-label"),
                            html.Span(
                                str(chat.get("user_count", 0)),
                                className="chat-stat-value"
                            ),
                        ],
                    ),
                ],
            ),
            html.Div(
                className="chat-card-stats",
                children=[
                    html.Div(
                        className="chat-stat",
                        children=[
                            html.Span("Avg/day", className="chat-stat-label"),
                            html.Span(
                                f"{chat.get('avg_messages_per_day', 0):.1f}",
                                className="chat-stat-value"
                            ),
                        ],
                    ),
                ],
            ),
            html.Div(
                className="chat-card-footer",
                children=[
                    html.Span(
                        f"Last activity: {last_activity_str}",
                        className="chat-last-activity"
                    ),
                ],
            ),
        ],
    )


def create_welcome_layout(
    session: Optional[SessionData] = None,
    chats: Optional[List[Dict[str, Any]]] = None,
) -> html.Div:
    """
    Create the welcome page layout with chat cards grid.

    Args:
        session: Current user session data
        chats: List of chat data for cards

    Returns:
        Dash HTML Div with welcome page layout
    """
    if chats is None:
        chats = []

    # Create chat cards
    chat_cards = [create_chat_card(chat) for chat in chats]

    return html.Div(
        className="welcome-wrapper",
        children=[
            create_header(session),
            html.Main(
                className="welcome-main",
                children=[
                    html.Div(
                        className="welcome-header",
                        children=[
                            html.H2("Chats", className="welcome-title"),
                            html.Span(
                                f"{len(chats)} chats",
                                className="chat-count-badge"
                            ),
                        ],
                    ),
                    html.Div(
                        className="chat-cards-grid",
                        children=chat_cards if chat_cards else [
                            html.Div(
                                className="no-chats-message",
                                children=[
                                    html.P("No chats available."),
                                    html.P(
                                        "You need to have recent activity in a chat to see it here.",
                                        className="no-chats-hint"
                                    ),
                                ],
                            )
                        ],
                    ),
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
        ],
    )
