"""
Cards Gallery page for admin users.

Displays generated card images in a carousel with:
- Week selector dropdown
- Prev/Next navigation
- Click to open full-size modal
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html
from dash_iconify import DashIconify

from src.charts import get_chart_colors
from src.components import create_group_nav


def create_gallery_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
    photo_client,
    gallery_client=None,
    week: str | None = None,
) -> html.Div:
    """
    Create the cards gallery page (admin only).

    Args:
        chat_id: Chat ID
        chat_info: Chat metadata
        user: Current user session data
        period: Selected time period (unused, kept for consistency)
        base_url: Base URL path
        theme_name: Current theme name
        queries: DashboardQueries instance (unused, kept for consistency)
        photo_client: PhotoClient for fetching profile photos (unused)
        gallery_client: GalleryClient for fetching card images
        week: Selected week from query param (YYYY-MM-DD)

    Returns:
        Page layout as html.Div
    """
    chat_title = chat_info.get("title", f"Chat {chat_id}")
    is_admin = user.get("is_admin", False)
    colors = get_chart_colors(theme_name)

    # Fetch available weeks
    available_weeks = []
    if gallery_client:
        available_weeks = gallery_client.get_available_weeks(chat_id) or []

    # Use provided week or default to latest
    selected_week = week if week in available_weeks else (available_weeks[0] if available_weeks else None)

    # Fetch images for selected week
    images = []
    if gallery_client and selected_week:
        images = gallery_client.get_gallery_images(chat_id, selected_week) or []

    # Build page content
    content = []

    # Week selector
    if available_weeks:
        content.append(
            _create_week_selector(
                weeks=available_weeks,
                selected=selected_week,
                chat_id=chat_id,
                base_url=base_url,
                period=period,
                colors=colors,
            )
        )
        content.append(dmc.Space(h="lg"))

    # Carousel or empty state
    if images:
        content.append(_create_carousel(images, colors))
    else:
        content.append(_create_empty_state(colors))

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="gallery",
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


def _create_week_selector(
    weeks: list[str],
    selected: str | None,
    chat_id: int,
    base_url: str,
    period: str,
    colors: dict,
) -> dmc.Card:
    """Create week selection as anchor links."""
    # Build week options as buttons/links
    week_links = []
    for week in weeks:
        is_active = week == selected
        href = f"{base_url}/group/{chat_id}/gallery?period={period}&week={week}"
        week_links.append(
            dmc.Anchor(
                dmc.Button(
                    _format_week(week),
                    variant="filled" if is_active else "outline",
                    size="sm",
                ),
                href=href,
                underline="never",
            )
        )

    return dmc.Card(
        dmc.Group(
            [
                dmc.ThemeIcon(
                    DashIconify(icon="mdi:calendar-week", width=20),
                    size="lg",
                    variant="light",
                    radius="md",
                ),
                dmc.Text("Select week:", size="sm", c="dimmed"),
                dmc.Group(week_links, gap="xs"),
            ],
            gap="md",
        ),
        withBorder=True,
        shadow="sm",
        radius="md",
        p="md",
    )


def _create_carousel(images: list[dict], colors: dict) -> dmc.Card:
    """Create image carousel with prev/next navigation."""
    slides = [
        dmc.CarouselSlide(_create_card_slide(img, colors))
        for img in images
    ]

    return dmc.Card(
        [
            dmc.Carousel(
                slides,
                withIndicators=True,
                withControls=True,
                type="loop",
                slideSize="100%",
                slideGap="md",
                controlsOffset="xs",
                controlSize=32,
            ),
            dmc.Space(h="md"),
            dmc.Text(
                f"Showing {len(images)} cards",
                size="sm",
                c="dimmed",
                ta="center",
            ),
        ],
        withBorder=True,
        shadow="sm",
        radius="md",
        p="lg",
    )


def _create_card_slide(img: dict, colors: dict) -> dmc.Stack:
    """Create a single carousel slide with card image."""
    first_name = img.get("first_name", "")
    last_name = img.get("last_name", "")
    user_name = f"{first_name} {last_name}".strip() or "Unknown"
    username = img.get("username", "")
    url = img.get("url", "")

    return dmc.Stack(
        [
            # User info header
            dmc.Group(
                [
                    dmc.Text(user_name, fw=500, size="lg"),
                    dmc.Text(f"@{username}", size="sm", c="dimmed") if username else None,
                ],
                justify="center",
                gap="xs",
            ),
            # Card image - opens in new tab on click
            dmc.Anchor(
                html.Img(
                    src=url,
                    style={
                        "width": "100%",
                        "maxWidth": "400px",
                        "height": "auto",
                        "cursor": "pointer",
                        "borderRadius": "8px",
                        "boxShadow": "0 4px 12px rgba(0,0,0,0.15)",
                    },
                ),
                href=url,
                target="_blank",
            ),
        ],
        align="center",
        gap="md",
    )


def _create_empty_state(colors: dict) -> dmc.Card:
    """Create empty state when no images available."""
    return dmc.Card(
        dmc.Center(
            dmc.Stack(
                [
                    dmc.ThemeIcon(
                        DashIconify(icon="mdi:image-off-outline", width=48),
                        size=80,
                        variant="light",
                        radius="xl",
                        color="gray",
                    ),
                    dmc.Title("No Cards Available", order=4),
                    dmc.Text(
                        "No card images have been generated for this chat yet.",
                        c="dimmed",
                        ta="center",
                    ),
                    dmc.Text(
                        "Run the card image generator to create cards.",
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


def _format_week(week_str: str) -> str:
    """Format week string for display."""
    from datetime import datetime

    try:
        dt = datetime.strptime(week_str, "%Y-%m-%d")
        return dt.strftime("%b %d")
    except ValueError:
        return week_str
