"""Landing page with group cards."""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.components import create_group_card


def create_landing_page(chats: list[dict], user: dict, base_url: str) -> html.Div:
    """
    Create landing page layout with group cards.

    Args:
        chats: List of chat dicts with stats
        user: Session dict with user info
        base_url: Base URL for navigation links

    Returns:
        Dash layout component
    """
    # Create greeting based on user info
    greeting = f"Welcome, {user.get('first_name', 'User')}"

    # Create group cards
    cards = []
    for chat in chats:
        card = create_group_card(
            chat_id=chat["id"],
            title=chat["title"],
            chat_type=chat["type"],
            message_count=chat["message_count"],
            user_count=chat["user_count"],
            last_activity=chat["last_activity"],
            avg_messages_per_day=float(chat.get("avg_messages_per_day", 0)),
        )
        cards.append(dbc.Col(card, xs=12, sm=6, lg=4, xl=3))

    # Empty state if no chats
    if not cards:
        content = _create_empty_state(user)
    else:
        content = dbc.Row(cards, className="g-3")

    return html.Div(
        [
            dmc.MantineProvider(
                dbc.Container(
                    [
                        # Header
                        dmc.Group(
                            [
                                dmc.Stack(
                                    [
                                        dmc.Title("Beef Briefing", order=1),
                                        dmc.Text(greeting, c="dimmed", size="lg"),
                                    ],
                                    gap=4,
                                ),
                                # User menu / logout
                                dmc.Group(
                                    [
                                        dmc.Avatar(
                                            src=user.get("photo_url"),
                                            children=user.get("first_name", "U")[0],
                                            radius="xl",
                                            size="md",
                                        ),
                                        dmc.Anchor(
                                            dmc.Button(
                                                "Logout",
                                                variant="subtle",
                                                color="gray",
                                                size="sm",
                                                leftSection=DashIconify(
                                                    icon="mdi:logout", width=16
                                                ),
                                            ),
                                            href=f"{base_url}/logout",
                                        ),
                                    ],
                                    gap="sm",
                                ),
                            ],
                            justify="space-between",
                            align="center",
                            mb="xl",
                            mt="lg",
                        ),
                        # Section title
                        dmc.Group(
                            [
                                DashIconify(icon="mdi:forum", width=24, color="blue"),
                                dmc.Title("Your Groups", order=2),
                            ],
                            gap="sm",
                            mb="lg",
                        ),
                        # Cards grid or empty state
                        content,
                    ],
                    fluid=True,
                    className="py-3",
                ),
            ),
        ]
    )


def _create_empty_state(user: dict) -> html.Div:
    """Create empty state when user has no accessible groups."""
    if user.get("is_admin"):
        message = "No groups found in the database."
        sub_message = "Groups will appear here once they start receiving messages."
    else:
        message = "You don't have access to any groups yet."
        sub_message = "Send some messages in a group to gain access."

    return dmc.Center(
        dmc.Stack(
            [
                DashIconify(
                    icon="mdi:forum-outline",
                    width=64,
                    color="gray",
                ),
                dmc.Text(message, size="lg", c="dimmed", ta="center"),
                dmc.Text(sub_message, size="sm", c="dimmed", ta="center"),
            ],
            align="center",
            gap="md",
        ),
        style={"minHeight": "400px"},
    )
