"""
Neon Diner Theme - Vibrant

Inspiration: 80s diners, neon signs, late-night eateries, vaporwave.
An electric theme with hot pink, cyan, and yellow neon on deep space black.
"""

from .base import DEFAULT_COMPONENT_PROPS, generate_shades

# Color palette
COLORS = {
    "background": "#0D0D12",  # Deep space blue-black
    "surface": "#1A1A24",  # Slightly lighter for cards
    "primary": "#FF2D6A",  # Hot pink neon - electric, memorable
    "secondary": "#00F0FF",  # Cyan neon - classic pairing
    "accent": "#FFE156",  # Yellow neon - third tube color
    "text": "#EEEEF0",  # Cool white
    "muted": "#6B6B7B",  # Cool gray
    "border": "#2D2D3A",  # Subtle cool border
}

# Generate Mantine-compatible 10-shade palettes
MANTINE_COLORS = {
    "neonPink": generate_shades(COLORS["primary"], base_index=5),
    "neonCyan": generate_shades(COLORS["secondary"], base_index=4),
    "neonYellow": generate_shades(COLORS["accent"], base_index=4),
    "space": [
        "#EEEEF0",
        "#C4C4CC",
        "#9A9AA8",
        "#707084",
        "#4D4D60",
        "#3A3A4C",
        "#2D2D3A",
        "#1F1F28",
        "#1A1A24",
        "#0D0D12",
    ],
}

# Background style with subtle grid pattern
BACKGROUND_STYLE = {
    "background": COLORS["background"],
    "backgroundImage": (
        "linear-gradient(rgba(255,255,255,0.02) 1px, transparent 1px), "
        "linear-gradient(90deg, rgba(255,255,255,0.02) 1px, transparent 1px)"
    ),
    "backgroundSize": "50px 50px",
    "minHeight": "100vh",
}

# Mantine theme configuration
THEME = {
    "colorScheme": "dark",
    "primaryColor": "neonPink",
    "colors": MANTINE_COLORS,
    "fontFamily": "'DM Sans', sans-serif",
    "fontFamilyMonospace": "'IBM Plex Mono', monospace",
    "headings": {
        "fontFamily": "'Righteous', cursive",
        "fontWeight": "400",
        "sizes": {
            "h1": {"fontSize": "2.75rem", "lineHeight": "1.15"},
            "h2": {"fontSize": "2rem", "lineHeight": "1.2"},
            "h3": {"fontSize": "1.5rem", "lineHeight": "1.25"},
            "h4": {"fontSize": "1.25rem", "lineHeight": "1.3"},
        },
    },
    "components": {
        **DEFAULT_COMPONENT_PROPS,
        "Card": {
            "defaultProps": {
                "shadow": "lg",
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
            "styles": {
                "root": {
                    "fontWeight": "500",
                }
            },
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
        "secondaryColor": COLORS["secondary"],
        "borderColor": COLORS["border"],
        "gridPattern": (
            "linear-gradient(rgba(255,255,255,0.02) 1px, transparent 1px), "
            "linear-gradient(90deg, rgba(255,255,255,0.02) 1px, transparent 1px)"
        ),
    },
}
