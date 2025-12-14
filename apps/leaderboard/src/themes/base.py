"""
Base utilities for theme generation.

Provides color shade generation and shared theme configuration.
"""


def hex_to_rgb(hex_color: str) -> tuple[int, int, int]:
    """Convert hex color to RGB tuple."""
    hex_color = hex_color.lstrip("#")
    return tuple(int(hex_color[i : i + 2], 16) for i in (0, 2, 4))


def rgb_to_hex(r: int, g: int, b: int) -> str:
    """Convert RGB values to hex color."""
    return f"#{r:02x}{g:02x}{b:02x}".upper()


def lerp_color(
    color1: tuple[int, int, int], color2: tuple[int, int, int], t: float
) -> tuple[int, int, int]:
    """Linear interpolation between two colors."""
    return tuple(int(c1 + (c2 - c1) * t) for c1, c2 in zip(color1, color2))


def generate_shades(base_hex: str, base_index: int = 6) -> list[str]:
    """
    Generate 10 shades from a base color for Mantine color palette.

    Args:
        base_hex: The base color in hex format (e.g., "#8B2635")
        base_index: Which index (0-9) the base color should occupy.
                   Lower = lighter shades, Higher = darker shades.
                   Default 6 works well for most primary colors.

    Returns:
        List of 10 hex colors from lightest (0) to darkest (9)
    """
    base_rgb = hex_to_rgb(base_hex)

    # Light end (for generating lighter shades)
    light_rgb = (255, 252, 250)  # Warm white

    # Dark end (for generating darker shades)
    dark_rgb = (15, 10, 8)  # Warm black

    shades = []

    for i in range(10):
        if i < base_index:
            # Lighter shades - interpolate from light to base
            t = i / base_index
            rgb = lerp_color(light_rgb, base_rgb, t)
        elif i == base_index:
            rgb = base_rgb
        else:
            # Darker shades - interpolate from base to dark
            t = (i - base_index) / (9 - base_index)
            rgb = lerp_color(base_rgb, dark_rgb, t)

        shades.append(rgb_to_hex(*rgb))

    return shades


# Google Fonts URL with all theme fonts
GOOGLE_FONTS_URL = (
    "https://fonts.googleapis.com/css2?"
    "family=Fraunces:opsz,wght@9..144,400;9..144,600;9..144,700"
    "&family=Atkinson+Hyperlegible:wght@400;700"
    "&family=Bebas+Neue"
    "&family=Source+Sans+3:wght@400;600"
    "&family=Righteous"
    "&family=DM+Sans:wght@400;500;600"
    "&family=JetBrains+Mono:wght@400;500"
    "&family=Fira+Code:wght@400;500"
    "&family=IBM+Plex+Mono:wght@400;500"
    "&display=swap"
)

# localStorage key for theme persistence (shared with Flask pages)
THEME_STORAGE_KEY = "beef-theme"

# Theme identifiers
THEME_BUTCHER_PAPER = "butcher-paper"
THEME_SMOKEHOUSE = "smokehouse"
THEME_NEON_DINER = "neon-diner"

# Default component props shared across themes
DEFAULT_COMPONENT_PROPS = {
    "Card": {"defaultProps": {"shadow": "sm", "radius": "md", "withBorder": False}},
    "Button": {"defaultProps": {"radius": "md"}},
    "Paper": {"defaultProps": {"radius": "md"}},
    "TextInput": {"defaultProps": {"radius": "md"}},
    "Select": {"defaultProps": {"radius": "md"}},
}
