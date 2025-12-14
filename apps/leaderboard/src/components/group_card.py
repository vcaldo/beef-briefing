"""Group card component for the landing page."""

from datetime import datetime, timezone

import dash_mantine_components as dmc
from dash_iconify import DashIconify


def create_group_card(
    chat_id: int,
    title: str,
    chat_type: str,
    message_count: int,
    user_count: int,
    last_activity: datetime,
    avg_messages_per_day: float,
    base_url: str = "",
) -> dmc.Card:
    """
    Create a card component for displaying group information.

    Args:
        chat_id: Unique chat identifier
        title: Group/supergroup title
        chat_type: 'group' or 'supergroup'
        message_count: Total message count
        user_count: Number of unique users
        last_activity: Timestamp of last message
        avg_messages_per_day: Average messages per day
        base_url: Base URL path for links

    Returns:
        dmc.Card component
    """
    # Format last activity as relative time
    last_activity_str = _format_relative_time(last_activity)

    # Format numbers
    message_count_str = _format_number(message_count)
    user_count_str = _format_number(user_count)
    avg_msg_str = f"{avg_messages_per_day:.1f}"

    # Truncate title if too long
    display_title = title[:30] + "..." if len(title) > 30 else title

    # Get first letter for avatar placeholder
    avatar_letter = title[0].upper() if title else "G"

    return dmc.Card(
        children=[
            # Header with avatar and badge
            dmc.Group(
                [
                    # Placeholder avatar for group photo
                    dmc.Avatar(
                        children=avatar_letter,
                        size="lg",
                        radius="md",
                    ),
                    dmc.Text(display_title, fw=500, size="md", lineClamp=1),
                ],
                gap="sm",
            ),
            # Stats section
            dmc.Space(h="md"),
            dmc.SimpleGrid(
                cols=2,
                spacing="xs",
                children=[
                    _create_stat_item(
                        icon="mdi:message-text-outline",
                        label="Messages",
                        value=message_count_str,
                    ),
                    _create_stat_item(
                        icon="mdi:account-group-outline",
                        label="Members",
                        value=user_count_str,
                    ),
                    _create_stat_item(
                        icon="mdi:chart-line",
                        label="Avg/Day",
                        value=avg_msg_str,
                    ),
                    _create_stat_item(
                        icon="mdi:clock-outline",
                        label="Last Active",
                        value=last_activity_str,
                    ),
                ],
            ),
            # View details button
            dmc.Space(h="md"),
            dmc.Anchor(
                dmc.Button(
                    "View Details",
                    leftSection=DashIconify(icon="mdi:arrow-right", width=16),
                    variant="light",
                    fullWidth=True,
                ),
                href=f"{base_url}/group/{chat_id}/overview",
                underline="never",
                style={"width": "100%"},
            ),
        ],
        withBorder=True,
        shadow="sm",
        radius="md",
        p="lg",
    )


def _create_stat_item(icon: str, label: str, value: str) -> dmc.Group:
    """Create a stat item with icon, label, and value."""
    return dmc.Group(
        [
            DashIconify(icon=icon, width=16, color="gray"),
            dmc.Stack(
                [
                    dmc.Text(value, size="sm", fw=500),
                    dmc.Text(label, size="xs", c="dimmed"),
                ],
                gap=0,
            ),
        ],
        gap="xs",
    )


def _format_relative_time(dt: datetime) -> str:
    """Format datetime as relative time string."""
    if dt is None:
        return "Never"

    # Ensure timezone-aware comparison
    now = datetime.now(timezone.utc)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)

    diff = now - dt

    if diff.days > 365:
        years = diff.days // 365
        return f"{years}y ago"
    elif diff.days > 30:
        months = diff.days // 30
        return f"{months}mo ago"
    elif diff.days > 0:
        return f"{diff.days}d ago"
    elif diff.seconds > 3600:
        hours = diff.seconds // 3600
        return f"{hours}h ago"
    elif diff.seconds > 60:
        minutes = diff.seconds // 60
        return f"{minutes}m ago"
    else:
        return "Just now"


def _format_number(n: int) -> str:
    """Format number with K/M suffix for large values."""
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    elif n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)
