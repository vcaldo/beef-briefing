"""
Card page for group statistics.

Displays weekly user personality cards with ML-analyzed stats:
- Mood, Comedy, Activity, Influence, Volatility, Toxicity, Chronotype
- Trend comparisons vs previous week
- Admin dropdown to view any user's card
"""

import math

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav

# Stat configuration: icon, color scheme, and display details
STAT_CONFIG = {
    "mood": {
        "icon": "mdi:emoticon-outline",
        "title": "Mood",
        "color_positive": "#22c55e",
        "color_negative": "#ef4444",
        "color_neutral": "#94a3b8",
    },
    "comedy": {
        "icon": "mdi:emoticon-lol-outline",
        "title": "Comedy",
    },
    "activity": {
        "icon": "mdi:chart-line",
        "title": "Activity",
    },
    "influence": {
        "icon": "mdi:account-group-outline",
        "title": "Influence",
    },
    "volatility": {
        "icon": "mdi:chart-bell-curve",
        "title": "Volatility",
    },
    "toxicity": {
        "icon": "mdi:shield-check-outline",
        "title": "Toxicity",
    },
    "chronotype": {
        "icon": "mdi:clock-outline",
        "title": "Chronotype",
    },
}


def create_card_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
    photo_client,
    card_client=None,
    target_user_id: int | None = None,
) -> html.Div:
    """
    Create the user card page.

    Args:
        chat_id: Chat ID
        chat_info: Chat metadata
        user: Current user session data
        period: Selected time period (unused, kept for consistency)
        base_url: Base URL path
        theme_name: Current theme name
        queries: DashboardQueries instance (unused, kept for consistency)
        photo_client: PhotoClient for fetching profile photos
        card_client: CardClient for fetching card data
        target_user_id: User ID to display card for (admin feature)

    Returns:
        Page layout as html.Div
    """
    chat_title = chat_info.get("title", f"Chat {chat_id}")
    current_user_id = user.get("user_id")
    is_admin = user.get("is_admin", False)
    colors = get_chart_colors(theme_name)

    # Determine which user's card to show
    display_user_id = target_user_id if target_user_id else current_user_id

    # For admins, fetch all users with cards for the dropdown
    all_users = []
    if is_admin and card_client:
        cards_response = card_client.get_chat_cards(chat_id, limit=100)
        if cards_response and "cards" in cards_response:
            all_users = [
                {
                    "user_id": c.get("user_id"),
                    "first_name": c.get("user", {}).get("first_name", "Unknown"),
                    "last_name": c.get("user", {}).get("last_name", ""),
                    "username": c.get("user", {}).get("username", ""),
                }
                for c in cards_response["cards"]
            ]

    # Fetch the card for the target user
    card_data = None
    card_user = None
    if card_client:
        response = card_client.get_user_card(display_user_id, chat_id)
        if response:
            card_data = response.get("card")
            card_user = response.get("user")

    # Get user photo
    photo_url = None
    if photo_client and display_user_id:
        photo_url = photo_client.get_user_photo(display_user_id, size="small")

    # Build page content
    content = []

    # Admin user selector
    if is_admin and all_users:
        content.append(
            _create_user_selector(
                users=all_users,
                selected_user_id=display_user_id,
                chat_id=chat_id,
                base_url=base_url,
                colors=colors,
            )
        )
        content.append(dmc.Space(h="md"))

    # Card content or empty state
    if card_data and card_user:
        content.append(
            _create_profile_header(
                user_data=card_user,
                card_data=card_data,
                photo_url=photo_url,
                colors=colors,
            )
        )
        content.append(dmc.Space(h="xl"))
        content.append(
            _create_stats_grid(
                stats=card_data.get("stats", {}),
                trends=card_data.get("trends", {}),
                colors=colors,
            )
        )
    else:
        # No card available
        user_name = card_user.get("first_name", "User") if card_user else "User"
        content.append(_create_no_card_message(user_name, colors))

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="card",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            *content,
        ],
        fluid=True,
        className="py-4",
    )


def _create_user_selector(
    users: list[dict],
    selected_user_id: int,
    chat_id: int,
    base_url: str,
    colors: dict,
) -> dmc.Card:
    """Create admin user selector dropdown."""
    # Build options for select
    options = [
        {
            "value": str(u["user_id"]),
            "label": _format_user_name(u),
        }
        for u in users
    ]

    return dmc.Card(
        dmc.Group(
            [
                dmc.ThemeIcon(
                    DashIconify(icon="mdi:account-search-outline", width=20),
                    size="lg",
                    variant="light",
                    radius="md",
                ),
                dmc.Text("View card for:", size="sm", c="dimmed"),
                dmc.Select(
                    id="card-user-select",
                    data=options,
                    value=str(selected_user_id),
                    searchable=True,
                    placeholder="Select user...",
                    style={"minWidth": "200px"},
                ),
            ],
            gap="md",
        ),
        withBorder=True,
        shadow="sm",
        radius="md",
        p="md",
    )


def _create_profile_header(
    user_data: dict,
    card_data: dict,
    photo_url: str | None,
    colors: dict,
) -> dmc.Card:
    """Create the profile header card with avatar and week info."""
    first_name = user_data.get("first_name", "Unknown")
    last_name = user_data.get("last_name", "")
    username = user_data.get("username", "")
    full_name = f"{first_name} {last_name}".strip()
    avatar_letter = first_name[0].upper() if first_name else "?"

    week_start = card_data.get("week_start", "")
    week_end = card_data.get("week_end", "")
    messages_analyzed = card_data.get("messages_analyzed", 0)

    # Format week display
    if week_start and week_end:
        week_display = f"Week of {_format_date(week_start)} - {_format_date(week_end)}"
    else:
        week_display = "Current Week"

    return dmc.Card(
        dmc.Group(
            [
                dmc.Avatar(
                    src=photo_url,
                    children=avatar_letter,
                    size="xl",
                    radius="xl",
                ),
                dmc.Stack(
                    [
                        dmc.Title(full_name, order=3),
                        dmc.Text(f"@{username}" if username else "", size="sm", c="dimmed")
                        if username
                        else None,
                        dmc.Group(
                            [
                                dmc.Badge(
                                    week_display,
                                    variant="light",
                                    size="lg",
                                    leftSection=DashIconify(
                                        icon="mdi:calendar-outline", width=14
                                    ),
                                ),
                                dmc.Badge(
                                    f"{messages_analyzed} messages",
                                    variant="outline",
                                    size="lg",
                                    leftSection=DashIconify(
                                        icon="mdi:message-text-outline", width=14
                                    ),
                                ),
                            ],
                            gap="sm",
                        ),
                    ],
                    gap="xs",
                ),
            ],
            gap="lg",
        ),
        withBorder=True,
        shadow="sm",
        radius="md",
        p="lg",
    )


def _create_stats_grid(
    stats: dict,
    trends: dict,
    colors: dict,
) -> dmc.SimpleGrid:
    """Create the grid of stat cards."""
    stat_cards = []

    for stat_key, config in STAT_CONFIG.items():
        # Special handling for influence - derive from reactions_received
        if stat_key == "influence":
            reactions = stats.get("reactions_received", 0)
            if isinstance(reactions, dict):
                reactions = reactions.get("count", 0)
            # Create synthetic stat_data for influence
            # Use log scale: 0 reactions = 0, 10 = ~25, 100 = ~50, 1000 = ~75
            if reactions > 0:
                score = min(100, int(math.log10(max(1, reactions)) * 33))
            else:
                score = 0
            stat_data = {"score": score, "label": f"{reactions} reactions", "count": reactions}
        else:
            stat_data = stats.get(stat_key, {})
            # Handle case where stat_data might be a number (not dict)
            if not isinstance(stat_data, dict):
                stat_data = {"score": stat_data}

        trend_data = trends.get(stat_key) if trends else None

        stat_cards.append(
            _create_stat_card(
                stat_key=stat_key,
                stat_data=stat_data,
                trend_data=trend_data,
                config=config,
                colors=colors,
            )
        )

    return dmc.SimpleGrid(
        stat_cards,
        cols={"base": 1, "sm": 2, "lg": 3, "xl": 4},
        spacing="md",
    )


def _create_stat_card(
    stat_key: str,
    stat_data: dict,
    trend_data: dict | None,
    config: dict,
    colors: dict,
) -> dmc.Card:
    """Create an individual stat card with RingProgress visualization."""
    icon = config.get("icon", "mdi:help-circle-outline")
    title = config.get("title", stat_key.capitalize())

    # Extract score and label based on stat type
    if stat_key == "activity":
        # Activity uses message count with logarithmic scale for ring progress
        messages = stat_data.get("messages", 0)
        active_days = stat_data.get("active_days", 0)
        # Log scale: 1 msg = ~0%, 100 msgs = ~50%, 10000 msgs = ~100%
        score = min(100, int(math.log10(max(1, messages)) * 25)) if messages > 0 else 0
        label = stat_data.get("label", f"{messages} msgs, {active_days} days")
        display_value = str(messages)
    elif stat_key == "chronotype":
        # Chronotype uses "type" field, not "label"
        label = stat_data.get("type", stat_data.get("label", "Unknown"))
        peak_hour = stat_data.get("peak_hour")
        score = None  # No ring progress for chronotype
        display_value = label
        if peak_hour is not None:
            display_value = f"{label} ({_format_hour(peak_hour)})"
    elif stat_key == "toxicity":
        # Toxicity uses "pct" field, not "score"
        score = stat_data.get("pct", stat_data.get("score", 0))
        if isinstance(score, (int, float)):
            score = float(score)
        else:
            score = 0
        label = stat_data.get("label", "")
        display_value = f"{int(score)}"
    elif stat_key in ("comedy", "volatility"):
        # Comedy and volatility scores are 0-1 scale, convert to 0-100
        score = stat_data.get("score", 0)
        if isinstance(score, (int, float)):
            score = float(score)
            # If score is <= 1, assume it's 0-1 scale and convert to percentage
            if score <= 1:
                score = score * 100
        else:
            score = 0
        label = stat_data.get("label", "")
        display_value = f"{int(score)}"
    elif stat_key == "influence":
        # Influence is derived from reactions_received (stored at top level of stats)
        # We need to get this from the parent stats dict, passed via stat_data
        score = stat_data.get("score", 0)
        if isinstance(score, (int, float)):
            score = float(score)
        else:
            score = 0
        label = stat_data.get("label", "")
        display_value = f"{int(score)}" if score > 0 else "N/A"
    else:
        # Standard scored stats (mood)
        score = stat_data.get("score", 0)
        if isinstance(score, (int, float)):
            score = float(score)
        else:
            score = 0
        label = stat_data.get("label", "")
        display_value = f"{int(score)}" if score is not None else "N/A"

    # Determine color based on stat type and score
    ring_color = colors["primary"]
    if stat_key == "mood" and score is not None:
        if score >= 60:
            ring_color = config.get("color_positive", colors["primary"])
        elif score <= 40:
            ring_color = config.get("color_negative", "#ef4444")
        else:
            ring_color = config.get("color_neutral", colors["muted"])
    elif stat_key == "toxicity" and score is not None:
        # Invert toxicity display - lower is better
        if score <= 30:
            ring_color = "#22c55e"  # Green for low toxicity
        elif score >= 70:
            ring_color = "#ef4444"  # Red for high toxicity
        else:
            ring_color = colors["muted"]

    # Build trend badge
    trend_badge = None
    if trend_data:
        direction = trend_data.get("direction", "stable")
        change = trend_data.get("change", 0)
        if direction == "up":
            trend_badge = dmc.Badge(
                f"+{change:.0f}%",
                color="green",
                variant="light",
                size="sm",
                leftSection=DashIconify(icon="mdi:arrow-up", width=12),
            )
        elif direction == "down":
            trend_badge = dmc.Badge(
                f"{change:.0f}%",
                color="red",
                variant="light",
                size="sm",
                leftSection=DashIconify(icon="mdi:arrow-down", width=12),
            )
        else:
            trend_badge = dmc.Badge(
                "Stable",
                color="gray",
                variant="light",
                size="sm",
                leftSection=DashIconify(icon="mdi:minus", width=12),
            )

    # Build card content
    card_content = [
        # Header with icon and title
        dmc.Group(
            [
                dmc.ThemeIcon(
                    DashIconify(icon=icon, width=20),
                    size="lg",
                    variant="light",
                    radius="md",
                    color=colors["primary"],
                ),
                dmc.Text(title, fw=500, size="sm"),
            ],
            gap="sm",
        ),
        dmc.Space(h="md"),
    ]

    # Ring progress or text display
    if score is not None and stat_key != "chronotype":
        card_content.append(
            dmc.Center(
                dmc.RingProgress(
                    sections=[{"value": min(100, max(0, score)), "color": ring_color}],
                    size=100,
                    thickness=8,
                    label=dmc.Center(
                        dmc.Text(display_value, size="lg", fw=700),
                    ),
                ),
            )
        )
    else:
        # For chronotype, just show the label prominently
        card_content.append(
            dmc.Center(
                dmc.Stack(
                    [
                        dmc.Text(display_value, size="xl", fw=700, ta="center"),
                    ],
                    gap="xs",
                    align="center",
                ),
                mih=100,
            )
        )

    card_content.append(dmc.Space(h="sm"))

    # Label and trend
    card_content.append(
        dmc.Stack(
            [
                dmc.Text(label, size="sm", c="dimmed", ta="center")
                if label and stat_key != "chronotype"
                else None,
                dmc.Center(trend_badge) if trend_badge else None,
            ],
            gap="xs",
        )
    )

    return dmc.Card(
        card_content,
        withBorder=True,
        shadow="sm",
        radius="md",
        p="lg",
    )


def _create_no_card_message(user_name: str, colors: dict) -> dmc.Card:
    """Create message when no card is available."""
    return dmc.Card(
        dmc.Center(
            dmc.Stack(
                [
                    dmc.ThemeIcon(
                        DashIconify(icon="mdi:card-off-outline", width=48),
                        size=80,
                        variant="light",
                        radius="xl",
                        color="gray",
                    ),
                    dmc.Title("No Card Available", order=4),
                    dmc.Text(
                        f"No card has been generated for {user_name} this week.",
                        c="dimmed",
                        ta="center",
                    ),
                    dmc.Text(
                        "Cards are generated weekly based on chat activity.",
                        size="sm",
                        c="dimmed",
                        ta="center",
                    ),
                ],
                align="center",
                gap="md",
            ),
            mih=300,
        ),
        withBorder=True,
        shadow="sm",
        radius="md",
        p="xl",
    )


def _format_user_name(user: dict) -> str:
    """Format user name for display."""
    first_name = user.get("first_name", "Unknown")
    last_name = user.get("last_name", "")
    username = user.get("username", "")

    name = f"{first_name} {last_name}".strip()
    if username:
        name = f"{name} (@{username})"
    return name


def _format_date(date_str: str) -> str:
    """Format date string for display (YYYY-MM-DD or ISO datetime -> Mon DD)."""
    try:
        from datetime import datetime

        # Handle ISO datetime format (2025-12-15T00:00:00Z)
        if "T" in date_str:
            dt = datetime.fromisoformat(date_str.replace("Z", "+00:00"))
        else:
            dt = datetime.strptime(date_str, "%Y-%m-%d")
        return dt.strftime("%b %d")
    except (ValueError, TypeError):
        return date_str


def _format_hour(hour: int) -> str:
    """Format hour as 12-hour time."""
    if hour == 0:
        return "12am"
    elif hour < 12:
        return f"{hour}am"
    elif hour == 12:
        return "12pm"
    else:
        return f"{hour - 12}pm"
