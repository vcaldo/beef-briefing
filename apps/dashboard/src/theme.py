"""
Centralized theme configuration for the Beef Dashboard.

This module serves as the single source of truth for all styling values.
CSS variables in styles.css should mirror these values.
"""

import plotly.graph_objects as go
import plotly.io as pio

# Theme configuration - single source of truth
THEME = {
    "colors": {
        "bg_primary": "#0a0e1a",
        "bg_secondary": "#141822",
        "bg_card": "rgba(20, 24, 34, 0.7)",
        "accent_primary": "#00d9ff",
        "accent_secondary": "#ff6b6b",
        "success": "#34d399",
        "warning": "#fbbf24",
        "danger": "#ef4444",
        "text_primary": "#e8eaed",
        "text_secondary": "#9aa0a6",
        "border": "rgba(255, 255, 255, 0.1)",
        "grid": "rgba(255, 255, 255, 0.05)",
    },
    "fonts": {
        "display": "'JetBrains Mono', monospace",
        "body": "'Space Grotesk', sans-serif",
    },
    "chart_colorway": [
        "#00d9ff",  # cyan (primary)
        "#ff6b6b",  # coral (secondary)
        "#34d399",  # green (success)
        "#fbbf24",  # yellow (warning)
        "#a78bfa",  # purple
        "#f472b6",  # pink
        "#60a5fa",  # blue
        "#c084fc",  # violet
    ],
}

# Convenience aliases for backwards compatibility
COLORS = {
    "primary": THEME["colors"]["accent_primary"],
    "secondary": THEME["colors"]["accent_secondary"],
    "success": THEME["colors"]["success"],
    "warning": THEME["colors"]["warning"],
    "bg": THEME["colors"]["bg_primary"],
    "card_bg": THEME["colors"]["bg_card"],
    "text": THEME["colors"]["text_primary"],
    "text_secondary": THEME["colors"]["text_secondary"],
    "grid": THEME["colors"]["grid"],
}

# Media type color mapping
MEDIA_COLORS = {
    "photo": THEME["colors"]["accent_primary"],
    "video": THEME["colors"]["accent_secondary"],
    "audio": THEME["colors"]["success"],
    "voice": THEME["colors"]["warning"],
    "document": "#a78bfa",
    "animation": "#f472b6",
    "video_note": "#60a5fa",
    "sticker": "#c084fc",
}


def create_plotly_template() -> go.layout.Template:
    """Create custom Plotly template matching app theme."""
    return go.layout.Template(
        layout=go.Layout(
            paper_bgcolor="rgba(0,0,0,0)",
            plot_bgcolor="rgba(0,0,0,0)",
            font=dict(
                family=THEME["fonts"]["body"],
                color=THEME["colors"]["text_primary"],
                size=12,
            ),
            colorway=THEME["chart_colorway"],
            margin=dict(l=40, r=20, t=30, b=40),
            xaxis=dict(
                gridcolor=THEME["colors"]["grid"],
                zerolinecolor=THEME["colors"]["grid"],
                showgrid=True,
            ),
            yaxis=dict(
                gridcolor=THEME["colors"]["grid"],
                zerolinecolor=THEME["colors"]["grid"],
                showgrid=True,
            ),
            hoverlabel=dict(
                bgcolor=THEME["colors"]["bg_secondary"],
                bordercolor=THEME["colors"]["border"],
                font=dict(
                    family=THEME["fonts"]["body"],
                    color=THEME["colors"]["text_primary"],
                ),
            ),
            legend=dict(
                bgcolor="rgba(0,0,0,0)",
                font=dict(color=THEME["colors"]["text_secondary"]),
            ),
        )
    )


def init_plotly_theme() -> None:
    """Initialize and register the custom Plotly template."""
    pio.templates["beef_dark"] = create_plotly_template()
    pio.templates.default = "beef_dark"


# Auto-initialize when module is imported
init_plotly_theme()
