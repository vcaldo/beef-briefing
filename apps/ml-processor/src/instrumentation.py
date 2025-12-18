"""
New Relic APM instrumentation helpers.

Provides decorators that work whether New Relic is enabled or not,
avoiding the need for conditional imports throughout the codebase.
"""

import functools
import logging
from typing import Any, Callable, TypeVar

logger = logging.getLogger(__name__)

# Type variable for generic function signatures
F = TypeVar("F", bound=Callable[..., Any])

# Try to import newrelic, but don't fail if not available/configured
try:
    import newrelic.agent

    _newrelic_available = True
except ImportError:
    _newrelic_available = False
    newrelic = None  # type: ignore


def is_newrelic_active() -> bool:
    """Check if New Relic agent is active and recording."""
    if not _newrelic_available:
        return False
    try:
        app = newrelic.agent.application()
        return app is not None and app.active
    except Exception:
        return False


def background_task(name: str | None = None, group: str = "Task") -> Callable[[F], F]:
    """
    Decorator for background tasks (CLI commands, batch jobs).

    If New Relic is not available/active, the function runs normally.

    Args:
        name: Transaction name (defaults to function name)
        group: Transaction group (default: "Task")
    """

    def decorator(func: F) -> F:
        if not _newrelic_available:
            return func

        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            task_name = name or func.__name__
            with newrelic.agent.BackgroundTask(
                application=newrelic.agent.application(),
                name=task_name,
                group=group,
            ):
                return func(*args, **kwargs)

        return wrapper  # type: ignore

    return decorator


def function_trace(name: str | None = None, group: str | None = None) -> Callable[[F], F]:
    """
    Decorator to trace function execution time.

    If New Relic is not available/active, the function runs normally.

    Args:
        name: Trace name (defaults to function name)
        group: Trace group for categorization
    """

    def decorator(func: F) -> F:
        if not _newrelic_available:
            return func

        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            trace_name = name or func.__name__
            with newrelic.agent.FunctionTrace(
                name=trace_name,
                group=group,
            ):
                return func(*args, **kwargs)

        return wrapper  # type: ignore

    return decorator


def record_custom_metric(name: str, value: float) -> None:
    """
    Record a custom metric value.

    Args:
        name: Metric name (e.g., "Custom/Messages/Processed")
        value: Metric value
    """
    if not is_newrelic_active():
        return

    try:
        newrelic.agent.record_custom_metric(name, value)
    except Exception as e:
        logger.debug(f"Failed to record metric {name}: {e}")


def record_custom_metrics(metrics: list[tuple[str, float]]) -> None:
    """
    Record multiple custom metrics at once.

    Args:
        metrics: List of (name, value) tuples
    """
    if not is_newrelic_active():
        return

    try:
        for name, value in metrics:
            newrelic.agent.record_custom_metric(name, value)
    except Exception as e:
        logger.debug(f"Failed to record metrics: {e}")


def add_custom_attribute(key: str, value: Any) -> None:
    """
    Add a custom attribute to the current transaction.

    Args:
        key: Attribute name
        value: Attribute value
    """
    if not is_newrelic_active():
        return

    try:
        newrelic.agent.add_custom_attribute(key, value)
    except Exception as e:
        logger.debug(f"Failed to add attribute {key}: {e}")


def add_custom_attributes(attributes: dict[str, Any]) -> None:
    """
    Add multiple custom attributes to the current transaction.

    Args:
        attributes: Dictionary of attribute key-value pairs
    """
    if not is_newrelic_active():
        return

    try:
        for key, value in attributes.items():
            newrelic.agent.add_custom_attribute(key, value)
    except Exception as e:
        logger.debug(f"Failed to add attributes: {e}")


def notice_error(error: Exception | None = None) -> None:
    """
    Record an error in New Relic.

    Args:
        error: Exception to record (uses current exception if None)
    """
    if not is_newrelic_active():
        return

    try:
        newrelic.agent.notice_error(error=error)
    except Exception as e:
        logger.debug(f"Failed to notice error: {e}")
