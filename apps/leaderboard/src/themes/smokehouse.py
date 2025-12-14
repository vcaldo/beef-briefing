"""
Smokehouse Theme - Dark

Inspiration: Texas BBQ joints, ember glow, cast iron, oak smoke.
A warm dark theme with amber flames and smoked paprika accents.
"""

from .base import DEFAULT_COMPONENT_PROPS, generate_shades

# Color palette
COLORS = {
    "background": "#1A1614",  # Charred wood - deep warm black
    "surface": "#252120",  # Slightly lighter for cards
    "primary": "#E8A849",  # Amber/flame - embers, honey glaze
    "accent": "#C75D3A",  # Smoked paprika - spice, heat
    "text": "#F0E6DC",  # Warm white - smoke-filtered light
    "muted": "#8B7D73",  # Ash gray
    "border": "#3D352F",  # Subtle warm border
}

# Generate Mantine-compatible 10-shade palettes
MANTINE_COLORS = {
    "amber": generate_shades(COLORS["primary"], base_index=5),
    "paprika": generate_shades(COLORS["accent"], base_index=5),
    "charcoal": [
        "#F0E6DC",
        "#D4C9BC",
        "#B8AA9C",
        "#9C8B7D",
        "#7D6D5E",
        "#5E4F42",
        "#3D352F",
        "#2D2622",
        "#252120",
        "#1A1614",
    ],
}

# Background style with radial ember glow
BACKGROUND_STYLE = {
    "background": f"radial-gradient(ellipse at 50% 120%, #3D2A20 0%, {COLORS['background']} 70%)",
    "minHeight": "100vh",
}

# Mantine theme configuration
THEME = {
    "colorScheme": "dark",
    "primaryColor": "amber",
    "colors": MANTINE_COLORS,
    "fontFamily": "'Source Sans 3', sans-serif",
    "fontFamilyMonospace": "'Fira Code', monospace",
    "headings": {
        "fontFamily": "'Bebas Neue', sans-serif",
        "fontWeight": "400",
        "sizes": {
            "h1": {"fontSize": "3rem", "lineHeight": "1.1"},
            "h2": {"fontSize": "2.25rem", "lineHeight": "1.15"},
            "h3": {"fontSize": "1.75rem", "lineHeight": "1.2"},
            "h4": {"fontSize": "1.5rem", "lineHeight": "1.25"},
        },
    },
    "components": {
        **DEFAULT_COMPONENT_PROPS,
        "Card": {
            "defaultProps": {
                "shadow": "md",
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
                    "letterSpacing": "0.05em",
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
        "backgroundGradient": f"radial-gradient(ellipse at 50% 120%, #3D2A20 0%, {COLORS['background']} 70%)",
    },
}
