"""
Login page layout with Telegram Login Widget.

Uses Dash Mantine Components (DMC) for UI elements and
Dash Bootstrap Components (DBC) for layout grid.
"""

import dash_bootstrap_components as dbc
import dash_mantine_components as dmc
from dash import html


def create_login_page(bot_username: str, callback_url: str) -> html.Div:
    """
    Create login page layout.

    Args:
        bot_username: Telegram bot username (without @)
        callback_url: URL for Telegram OAuth callback

    Returns:
        Dash layout component for login page
    """
    # Minimal CSS for pulsating animation only
    pulse_css = """
    @keyframes pulse {
        0% { transform: scale(1); }
        50% { transform: scale(1.15); }
        100% { transform: scale(1); }
    }
    .pulse-emoji {
        animation: pulse 2s ease-in-out infinite;
        display: inline-block;
    }
    """

    return html.Div(
        [
            html.Style(pulse_css),
            dmc.MantineProvider(
                dbc.Container(
                    [
                        dbc.Row(
                            [
                                dbc.Col(
                                    [
                                        dmc.Paper(
                                            [
                                                # Pulsating beef emoji
                                                dmc.Center(
                                                    html.Span(
                                                        "\U0001F969",  # Beef emoji
                                                        className="pulse-emoji",
                                                        style={"fontSize": "72px"},
                                                    )
                                                ),
                                                # Title
                                                dmc.Title(
                                                    "Beef Briefing",
                                                    order=2,
                                                    ta="center",
                                                    mt="md",
                                                ),
                                                dmc.Title(
                                                    "Leaderboard",
                                                    order=3,
                                                    ta="center",
                                                    c="dimmed",
                                                ),
                                                # Subtitle
                                                dmc.Text(
                                                    "Sign in with Telegram to continue",
                                                    c="dimmed",
                                                    ta="center",
                                                    size="sm",
                                                    mt="md",
                                                ),
                                                # Telegram Login Widget container
                                                dmc.Center(
                                                    html.Div(
                                                        id="telegram-login-widget",
                                                        style={"marginTop": "24px"},
                                                    ),
                                                    mt="xl",
                                                ),
                                                # Script to load Telegram widget
                                                html.Script(
                                                    f"""
                                                    (function() {{
                                                        var container = document.getElementById('telegram-login-widget');
                                                        if (container && !container.querySelector('script')) {{
                                                            var script = document.createElement('script');
                                                            script.async = true;
                                                            script.src = 'https://telegram.org/js/telegram-widget.js?22';
                                                            script.setAttribute('data-telegram-login', '{bot_username}');
                                                            script.setAttribute('data-size', 'large');
                                                            script.setAttribute('data-radius', '8');
                                                            script.setAttribute('data-auth-url', '{callback_url}');
                                                            script.setAttribute('data-request-access', 'write');
                                                            container.appendChild(script);
                                                        }}
                                                    }})();
                                                    """
                                                ),
                                            ],
                                            shadow="md",
                                            radius="md",
                                            p="xl",
                                            withBorder=True,
                                            style={
                                                "backgroundColor": "white",
                                                "maxWidth": "400px",
                                                "margin": "0 auto",
                                            },
                                        )
                                    ],
                                    md=6,
                                    lg=4,
                                    className="mx-auto",
                                )
                            ],
                            justify="center",
                            align="center",
                            style={"minHeight": "100vh"},
                        )
                    ],
                    fluid=True,
                    style={"backgroundColor": "#f8f9fa"},
                )
            ),
        ]
    )
