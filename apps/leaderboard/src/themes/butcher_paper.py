"""
Butcher Paper Theme - Light

Inspiration: Artisan butcher shops, craft meat markets, hand-stamped packaging.
A warm, sophisticated light theme with oxblood reds and aged brass accents.
"""

from .base import DEFAULT_COMPONENT_PROPS, generate_shades

# Color palette
COLORS = {
    "background": "#F5F0E8",  # Warm cream - like actual butcher paper
    "surface": "#FFFDF8",  # Slightly lighter for cards
    "primary": "#8B2635",  # Oxblood - aged beef, rich and sophisticated
    "accent": "#C4A747",  # Aged brass - butcher hooks, vintage scales
    "text": "#2D2420",  # Espresso brown - warmer than black
    "muted": "#7D7068",  # Warm gray
    "border": "#E5DED3",  # Subtle warm border
}

# Generate Mantine-compatible 10-shade palettes
MANTINE_COLORS = {
    "oxblood": generate_shades(COLORS["primary"], base_index=6),
    "brass": generate_shades(COLORS["accent"], base_index=5),
    "paper": [
        "#FFFDFB",
        "#FDF9F4",
        "#FAF5ED",
        "#F7F1E6",
        "#F5F0E8",
        "#E8E1D6",
        "#D4CCC0",
        "#B8AFA2",
        "#9C9285",
        "#807568",
    ],
}

# Background style with subtle paper texture
BACKGROUND_STYLE = {
    "background": COLORS["background"],
    "minHeight": "100vh",
}

# Mantine theme configuration
THEME = {
    "colorScheme": "light",
    "primaryColor": "oxblood",
    "colors": MANTINE_COLORS,
    "fontFamily": "'Atkinson Hyperlegible', sans-serif",
    "fontFamilyMonospace": "'JetBrains Mono', monospace",
    "headings": {
        "fontFamily": "'Fraunces', serif",
        "fontWeight": "600",
    },
    "components": {
        **DEFAULT_COMPONENT_PROPS,
        "Card": {
            "defaultProps": {
                "shadow": "sm",
                "radius": "md",
                "withBorder": True,
            },
            "styles": {
                "root": {
                    "backgroundColor": COLORS["surface"],
                    "borderColor": COLORS["border"],
                }
            },
        },
        "Paper": {
            "styles": {
                "root": {
                    "backgroundColor": COLORS["surface"],
                }
            },
        },
        "Button": {
            "defaultProps": {"radius": "md"},
        },
        "Title": {
            "styles": {
                "root": {
                    "color": COLORS["text"],
                }
            },
        },
        "Text": {
            "styles": {
                "root": {
                    "color": COLORS["text"],
                }
            },
        },
    },
    "other": {
        "backgroundColor": COLORS["background"],
        "surfaceColor": COLORS["surface"],
        "textColor": COLORS["text"],
        "mutedColor": COLORS["muted"],
        "accentColor": COLORS["accent"],
        "borderColor": COLORS["border"],
    },
}
