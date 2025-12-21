"""Database utility functions."""

from typing import Any


def row_to_dict(row) -> dict[str, Any]:
    """Convert a SQLAlchemy row to a dictionary."""
    return dict(row._mapping)


def rows_to_dicts(result) -> list[dict[str, Any]]:
    """Convert SQLAlchemy result rows to a list of dictionaries."""
    return [row_to_dict(row) for row in result]
