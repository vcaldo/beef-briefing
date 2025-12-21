"""Theme configuration dataclasses and loader."""

import json
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)


@dataclass
class ThemeColors:
    """Color configuration for a theme."""

    background_gradient: list[str]  # [start, mid, end]
    primary_accent: str
    secondary_accent: str
    stat_colors: dict[str, str]  # vibe, activity, presence, etc.
    badge_rarity_colors: dict[str, dict[str, str]]  # common, rare, epic, legendary
    tier_colors: dict[str, list[str]]  # legendary, elite, etc. [start, end]
    text_primary: str = "#ffffff"
    text_secondary: str = "rgba(255, 255, 255, 0.6)"
    border_color: str = "rgba(255, 255, 255, 0.1)"

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ThemeColors":
        """Create ThemeColors from dictionary."""
        return cls(
            background_gradient=data.get("background_gradient", ["#1a1a2e", "#16213e", "#0f3460"]),
            primary_accent=data.get("primary_accent", "#00d9ff"),
            secondary_accent=data.get("secondary_accent", "#e94560"),
            stat_colors=data.get("stat_colors", {}),
            badge_rarity_colors=data.get("badge_rarity_colors", {}),
            tier_colors=data.get("tier_colors", {}),
            text_primary=data.get("text_primary", "#ffffff"),
            text_secondary=data.get("text_secondary", "rgba(255, 255, 255, 0.6)"),
            border_color=data.get("border_color", "rgba(255, 255, 255, 0.1)"),
        )


@dataclass
class ThemeTypography:
    """Typography configuration for a theme."""

    header_font: str
    body_font: str
    header_weights: list[int] = field(default_factory=lambda: [400, 700, 900])
    body_weights: list[int] = field(default_factory=lambda: [400, 500, 700])

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ThemeTypography":
        """Create ThemeTypography from dictionary."""
        return cls(
            header_font=data.get("header_font", "Orbitron"),
            body_font=data.get("body_font", "Rajdhani"),
            header_weights=data.get("header_weights", [400, 700, 900]),
            body_weights=data.get("body_weights", [400, 500, 700]),
        )

    def get_google_fonts_import(self) -> str:
        """Generate Google Fonts import URL."""
        fonts = []

        # Header font
        header_weights = ",".join(str(w) for w in self.header_weights)
        fonts.append(f"family={self.header_font.replace(' ', '+')}:wght@{header_weights}")

        # Body font (if different)
        if self.body_font != self.header_font:
            body_weights = ",".join(str(w) for w in self.body_weights)
            fonts.append(f"family={self.body_font.replace(' ', '+')}:wght@{body_weights}")

        return f"https://fonts.googleapis.com/css2?{'&'.join(fonts)}&display=swap"


@dataclass
class ThemeConfig:
    """Complete theme configuration."""

    name: str
    colors: ThemeColors
    typography: ThemeTypography

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ThemeConfig":
        """Create ThemeConfig from dictionary."""
        return cls(
            name=data.get("name", "unknown"),
            colors=ThemeColors.from_dict(data.get("colors", {})),
            typography=ThemeTypography.from_dict(data.get("typography", {})),
        )

    def to_template_context(self) -> dict[str, Any]:
        """Convert theme config to template context dictionary."""
        return {
            "name": self.name,
            "colors": {
                "bg_start": self.colors.background_gradient[0],
                "bg_mid": self.colors.background_gradient[1] if len(self.colors.background_gradient) > 1 else self.colors.background_gradient[0],
                "bg_end": self.colors.background_gradient[2] if len(self.colors.background_gradient) > 2 else self.colors.background_gradient[-1],
                "primary": self.colors.primary_accent,
                "secondary": self.colors.secondary_accent,
                "text_primary": self.colors.text_primary,
                "text_secondary": self.colors.text_secondary,
                "border": self.colors.border_color,
                "stat": self.colors.stat_colors,
                "badge": self.colors.badge_rarity_colors,
                "tier": self.colors.tier_colors,
            },
            "typography": {
                "header_font": self.typography.header_font,
                "body_font": self.typography.body_font,
                "google_fonts_import": self.typography.get_google_fonts_import(),
            },
        }


class ThemeLoader:
    """Loads theme configurations from JSON files."""

    def __init__(self, themes_dir: str | Path):
        self.themes_dir = Path(themes_dir)
        self._cache: dict[str, ThemeConfig] = {}

    def theme_exists(self, theme_name: str) -> bool:
        """Check if a theme exists."""
        theme_path = self.themes_dir / "themes" / theme_name / "theme.json"
        return theme_path.exists()

    def load_theme(self, theme_name: str) -> ThemeConfig:
        """Load a theme configuration by name."""
        if theme_name in self._cache:
            return self._cache[theme_name]

        theme_path = self.themes_dir / "themes" / theme_name / "theme.json"

        if not theme_path.exists():
            logger.warning(f"Theme config not found: {theme_path}, using defaults")
            return self._get_default_theme(theme_name)

        try:
            with open(theme_path) as f:
                data = json.load(f)
            theme = ThemeConfig.from_dict(data)
            self._cache[theme_name] = theme
            return theme
        except (json.JSONDecodeError, KeyError) as e:
            logger.error(f"Failed to load theme {theme_name}: {e}")
            return self._get_default_theme(theme_name)

    def _get_default_theme(self, theme_name: str) -> ThemeConfig:
        """Return default gaming theme configuration."""
        return ThemeConfig.from_dict({
            "name": theme_name,
            "colors": {
                "background_gradient": ["#1a1a2e", "#16213e", "#0f3460"],
                "primary_accent": "#00d9ff",
                "secondary_accent": "#e94560",
                "stat_colors": {
                    "vibe": "#fbbf24",
                    "activity": "#00d9ff",
                    "presence": "#14b8a6",
                    "humor": "#a855f7",
                    "toxicity": "#ef4444",
                    "popularity": "#e94560",
                },
                "badge_rarity_colors": {
                    "common": {"bg_start": "#374151", "bg_end": "#4b5563", "border": "#6b7280", "text": "#d1d5db"},
                    "rare": {"bg_start": "#1e3a8a", "bg_end": "#2563eb", "border": "#3b82f6", "text": "#93c5fd"},
                    "epic": {"bg_start": "#6b21a8", "bg_end": "#9333ea", "border": "#a855f7", "text": "#d8b4fe"},
                    "legendary": {"bg_start": "#92400e", "bg_end": "#f59e0b", "border": "#fbbf24", "text": "#fef3c7"},
                },
                "tier_colors": {
                    "legendary": ["#fbbf24", "#f59e0b"],
                    "elite": ["#00d9ff", "#0ea5e9"],
                    "outstanding": ["#a855f7", "#7c3aed"],
                    "regular": ["#14b8a6", "#0d9488"],
                    "beginner": ["#9ca3af", "#6b7280"],
                    "rookie": ["#f472b6", "#ec4899"],
                },
                "text_primary": "#ffffff",
                "text_secondary": "rgba(255, 255, 255, 0.6)",
                "border_color": "rgba(255, 255, 255, 0.1)",
            },
            "typography": {
                "header_font": "Orbitron",
                "body_font": "Rajdhani",
            },
        })

    def list_themes(self) -> list[str]:
        """List all available themes."""
        themes_path = self.themes_dir / "themes"
        if not themes_path.exists():
            return []
        return [
            d.name
            for d in themes_path.iterdir()
            if d.is_dir() and (d / "theme.json").exists()
        ]
