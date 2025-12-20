"""
Card generation module for aggregating ML results into user cards.

This module provides:
- CardGenerator: Main orchestrator for generating weekly user cards
- CardImageClient: Client for card-image-generator service
- Pluggable stat calculators: Easy to add/remove stat calculations
"""

from src.cards.generator import CardGenerator
from src.cards.image_client import CardImageClient

__all__ = ["CardGenerator", "CardImageClient"]
