"""
Card generation module for aggregating ML results into user cards.

This module provides:
- CardGenerator: Main orchestrator for generating weekly user cards
- Pluggable stat calculators: Easy to add/remove stat calculations
"""

from src.cards.generator import CardGenerator

__all__ = ["CardGenerator"]
