"""
Database connection management for the ML Dashboard.
"""

from sqlalchemy import create_engine
from sqlalchemy.engine import Engine

from config import Config


def get_engine(config: Config) -> Engine:
    """
    Create a SQLAlchemy engine with connection pooling.

    Args:
        config: Application configuration

    Returns:
        SQLAlchemy Engine instance
    """
    return create_engine(
        config.dsn(),
        pool_pre_ping=True,
        pool_size=5,
        max_overflow=10,
    )
