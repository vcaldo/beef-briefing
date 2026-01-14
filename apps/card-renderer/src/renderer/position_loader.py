"""
Position loader for compact card placeholders.

Loads and manages placeholder position configurations for compact cards,
enabling React apps to overlay values at precise coordinates.
"""

from pathlib import Path
from typing import Any, Dict, Optional
import json
from functools import lru_cache


class PositionLoader:
    """Loads and manages placeholder position configurations for compact cards."""

    def __init__(self, templates_dir: Path):
        """
        Initialize position loader.

        Args:
            templates_dir: Path to themes directory (e.g., apps/card-renderer/templates/themes)
        """
        self.templates_dir = templates_dir

    @lru_cache(maxsize=32)
    def load_positions(self, theme: str) -> Optional[Dict[str, Any]]:
        """
        Load position configuration for a theme.

        Args:
            theme: Theme name (without _compact suffix)

        Returns:
            Position configuration dict if file exists, None otherwise
        """
        position_file = self.templates_dir / theme / "compact_positions.json"

        if not position_file.exists():
            return None

        try:
            with open(position_file, 'r') as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            # Log error but don't crash - positions are optional
            print(f"Error loading positions for {theme}: {e}")
            return None

    def get_positions_for_tier(self, theme: str, tier_class: str) -> Dict[str, Any]:
        """
        Get position data with tier-specific overrides applied.

        Args:
            theme: Theme name (without _compact suffix)
            tier_class: Tier CSS class (e.g., "tier-1", "tier-6")

        Returns:
            Position configuration with tier overrides merged
        """
        base_positions = self.load_positions(theme)

        if not base_positions:
            return {}

        # Start with base positions
        result = {
            "version": base_positions.get("version"),
            "card_dimensions": base_positions.get("card_dimensions"),
            "placeholders": base_positions.get("placeholders", {}),
        }

        # Apply tier-specific overrides if they exist
        tier_variations = base_positions.get("tier_variations", {}).get(tier_class, {})
        if tier_variations:
            result["tier_overrides"] = tier_variations

        return result

    def has_positions(self, theme: str) -> bool:
        """
        Check if position configuration exists for theme.

        Args:
            theme: Theme name (without _compact suffix)

        Returns:
            True if positions.json file exists for theme
        """
        return self.load_positions(theme) is not None

    def clear_cache(self) -> None:
        """Clear the position cache. Useful for testing."""
        self.load_positions.cache_clear()
