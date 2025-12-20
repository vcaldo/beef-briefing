"""
Leaderboard page for group statistics.

User rankings:
- Metric selector (Messages, Reactions Sent, Reactions Received, Active Days)
- Paginated table (20 per page)
- Rank, avatar, name, score columns
"""

import math

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html

from src.charts import get_chart_colors
from src.components import create_group_nav, get_period_dates
from src.utils import format_number

# Metric options for the selector
METRICS = [
    {"value": "message_count", "label": "Messages"},
    {"value": "reactions_sent", "label": "Reactions Sent"},
    {"value": "reactions_received", "label": "Reactions Received"},
    {"value": "active_days", "label": "Active Days"},
]

ITEMS_PER_PAGE = 20


def create_leaderboard_page(
    chat_id: int,
    chat_info: dict,
    user: dict,
    period: str,
    base_url: str,
    theme_name: str | None,
    queries,
    photo_client,
    metric: str = "message_count",
    page: int = 1,
) -> html.Div:
    """
    Create the leaderboard page for a group.

    Args:
        chat_id: Chat ID
        chat_info: Chat metadata
        user: Current user session data
        period: Selected time period
        base_url: Base URL path
        theme_name: Current theme name
        queries: DashboardQueries instance
        photo_client: PhotoClient for fetching profile photos
        metric: Metric to rank by
        page: Current page number

    Returns:
        Page layout as html.Div
    """
    chat_title = chat_info.get("title", f"Chat {chat_id}")
    is_admin = user.get("is_admin", False)
    colors = get_chart_colors(theme_name)

    # Get date range for period
    start_date, end_date = get_period_dates(period)

    # Validate metric
    valid_metrics = [m["value"] for m in METRICS]
    if metric not in valid_metrics:
        metric = "message_count"

    # Calculate pagination
    total_users = queries.get_user_rankings_total(chat_id, start_date, end_date)
    total_pages = max(1, math.ceil(total_users / ITEMS_PER_PAGE))
    page = max(1, min(page, total_pages))
    offset = (page - 1) * ITEMS_PER_PAGE

    # Fetch users for current page
    users = queries.get_user_rankings(
        chat_id,
        metric=metric,
        limit=ITEMS_PER_PAGE,
        offset=offset,
        start_date=start_date,
        end_date=end_date,
    )

    # Fetch profile photos for all users on this page
    user_ids = [u["user_id"] for u in users]
    photo_urls = photo_client.get_user_photos_batch(user_ids, size="small")

    # Get metric label
    metric_label = next(
        (m["label"] for m in METRICS if m["value"] == metric), "Score"
    )

    # Build base URL for this page
    page_base_url = f"{base_url}/group/{chat_id}/leaderboard"

    return dbc.Container(
        [
            create_group_nav(
                chat_id=chat_id,
                chat_title=chat_title,
                current_tab="leaderboard",
                current_period=period,
                base_url=base_url,
                theme_name=theme_name,
                is_admin=is_admin,
            ),
            dmc.Space(h="xl"),
            # Header with metric selector
            dmc.Group(
                [
                    dmc.Group(
                        [
                            dmc.Text("Rank by:", size="sm", c="dimmed"),
                            _create_metric_selector(
                                page_base_url, metric, period, colors
                            ),
                        ],
                        gap="md",
                    ),
                    dmc.Text(
                        f"{total_users} users",
                        size="sm",
                        c="dimmed",
                    ),
                ],
                justify="space-between",
                align="center",
            ),
            dmc.Space(h="lg"),
            # Leaderboard table
            dmc.Paper(
                [
                    _create_leaderboard_table(users, metric_label, colors, photo_urls),
                    dmc.Space(h="md"),
                    _create_pagination(
                        page_base_url, metric, period, page, total_pages
                    ),
                ],
                p="md",
                withBorder=True,
            ),
        ],
        fluid=True,
        className="py-4",
    )


def _create_metric_selector(
    base_url: str, current_metric: str, period: str, colors: dict
) -> dmc.Group:
    """Create metric selector as link buttons."""
    buttons = []
    for m in METRICS:
        is_active = m["value"] == current_metric
        href = f"{base_url}?metric={m['value']}&period={period}"
        buttons.append(
            dmc.Anchor(
                dmc.Button(
                    m["label"],
                    variant="filled" if is_active else "light",
                    color=colors["primary"] if is_active else "gray",
                    size="xs",
                ),
                href=href,
                underline="never",
            )
        )
    return dmc.Group(buttons, gap="xs")


def _create_pagination(
    base_url: str, metric: str, period: str, current_page: int, total_pages: int
) -> dmc.Group:
    """Create pagination as link buttons."""
    if total_pages <= 1:
        return dmc.Group([], justify="center")

    buttons = []

    # Previous button
    if current_page > 1:
        buttons.append(
            dmc.Anchor(
                dmc.Button("←", variant="light", size="xs"),
                href=f"{base_url}?metric={metric}&period={period}&page={current_page - 1}",
                underline="never",
            )
        )

    # Page numbers (show 5 pages around current)
    start_page = max(1, current_page - 2)
    end_page = min(total_pages, current_page + 2)

    # Ensure we always show 5 pages if possible
    if end_page - start_page < 4:
        if start_page == 1:
            end_page = min(total_pages, start_page + 4)
        else:
            start_page = max(1, end_page - 4)

    # First page and ellipsis
    if start_page > 1:
        buttons.append(
            dmc.Anchor(
                dmc.Button("1", variant="light", size="xs"),
                href=f"{base_url}?metric={metric}&period={period}&page=1",
                underline="never",
            )
        )
        if start_page > 2:
            buttons.append(dmc.Text("...", size="sm", c="dimmed"))

    # Page numbers
    for p in range(start_page, end_page + 1):
        is_current = p == current_page
        buttons.append(
            dmc.Anchor(
                dmc.Button(
                    str(p),
                    variant="filled" if is_current else "light",
                    size="xs",
                ),
                href=f"{base_url}?metric={metric}&period={period}&page={p}",
                underline="never",
            )
        )

    # Last page and ellipsis
    if end_page < total_pages:
        if end_page < total_pages - 1:
            buttons.append(dmc.Text("...", size="sm", c="dimmed"))
        buttons.append(
            dmc.Anchor(
                dmc.Button(str(total_pages), variant="light", size="xs"),
                href=f"{base_url}?metric={metric}&period={period}&page={total_pages}",
                underline="never",
            )
        )

    # Next button
    if current_page < total_pages:
        buttons.append(
            dmc.Anchor(
                dmc.Button("→", variant="light", size="xs"),
                href=f"{base_url}?metric={metric}&period={period}&page={current_page + 1}",
                underline="never",
            )
        )

    return dmc.Group(buttons, justify="center", gap="xs")


def _create_leaderboard_table(
    users: list, metric_label: str, colors: dict, photo_urls: dict
) -> dmc.Table:
    """Create the leaderboard table."""
    if not users:
        return dmc.Table(
            striped=True,
            highlightOnHover=True,
            children=[
                dmc.TableThead(
                    dmc.TableTr(
                        [
                            dmc.TableTh("#", w="60px"),
                            dmc.TableTh("User"),
                            dmc.TableTh(metric_label, ta="right"),
                        ]
                    )
                ),
                dmc.TableTbody(
                    [
                        dmc.TableTr(
                            dmc.TableTd(
                                dmc.Text("No users yet", c="dimmed", size="sm"),
                                colSpan=3,
                                ta="center",
                            )
                        )
                    ]
                ),
            ],
        )

    rows = []
    for user in users:
        rank = user["rank"]

        # Rank badge styling
        if rank == 1:
            rank_badge = dmc.Badge(
                "#1",
                color="yellow",
                variant="filled",
                size="lg",
            )
        elif rank == 2:
            rank_badge = dmc.Badge(
                "#2",
                color="gray",
                variant="filled",
                size="md",
            )
        elif rank == 3:
            rank_badge = dmc.Badge(
                "#3",
                color="orange",
                variant="filled",
                size="md",
            )
        else:
            rank_badge = dmc.Badge(
                f"#{rank}",
                variant="light",
                size="sm",
            )

        # Premium indicator
        premium_badge = None
        if user.get("is_premium"):
            premium_badge = dmc.Badge(
                "Premium",
                color="violet",
                variant="light",
                size="xs",
            )

        rows.append(
            dmc.TableTr(
                [
                    dmc.TableTd(rank_badge, w="60px"),
                    dmc.TableTd(
                        dmc.Group(
                            [
                                dmc.Avatar(
                                    src=photo_urls.get(user["user_id"]),
                                    children=user["first_name"][0].upper()
                                    if user["first_name"]
                                    else "?",
                                    size="md",
                                    radius="xl",
                                ),
                                dmc.Stack(
                                    [
                                        dmc.Group(
                                            [
                                                dmc.Text(
                                                    user["first_name"] or "Unknown",
                                                    size="sm",
                                                    fw=500,
                                                ),
                                                premium_badge,
                                            ]
                                            if premium_badge
                                            else [
                                                dmc.Text(
                                                    user["first_name"] or "Unknown",
                                                    size="sm",
                                                    fw=500,
                                                )
                                            ],
                                            gap="xs",
                                        ),
                                        dmc.Text(
                                            f"@{user['username']}"
                                            if user.get("username")
                                            else "",
                                            size="xs",
                                            c="dimmed",
                                        )
                                        if user.get("username")
                                        else None,
                                    ],
                                    gap=0,
                                ),
                            ],
                            gap="sm",
                        )
                    ),
                    dmc.TableTd(
                        dmc.Text(
                            format_number(user["score"]),
                            size="sm",
                            fw=500,
                        ),
                        ta="right",
                    ),
                ]
            )
        )

    return dmc.Table(
        striped=True,
        highlightOnHover=True,
        children=[
            dmc.TableThead(
                dmc.TableTr(
                    [
                        dmc.TableTh("#", w="60px"),
                        dmc.TableTh("User"),
                        dmc.TableTh(metric_label, ta="right"),
                    ]
                )
            ),
            dmc.TableTbody(rows),
        ],
    )
