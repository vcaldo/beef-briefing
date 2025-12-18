"""
New Relic APM instrumentation helpers.

Provides decorators that work whether New Relic is enabled or not,
avoiding the need for conditional imports throughout the codebase.
"""

import functools
import logging
from typing import Any, Callable, TypeVar

logger = logging.getLogger(__name__)

# Model pricing per token (in USD)
# Based on 2024 pricing: https://openai.com/pricing
MODEL_PRICING: dict[str, dict[str, float]] = {
    # OpenAI Chat Models
    "gpt-4o-mini": {"input": 0.15e-6, "output": 0.60e-6},
    "gpt-4o": {"input": 2.50e-6, "output": 10.0e-6},
    "gpt-4-turbo": {"input": 10.0e-6, "output": 30.0e-6},
    # OpenAI Embedding Models
    "text-embedding-3-small": {"input": 0.02e-6, "output": 0.0},
    "text-embedding-3-large": {"input": 0.13e-6, "output": 0.0},
    "text-embedding-ada-002": {"input": 0.10e-6, "output": 0.0},
    # OpenAI Moderation (free)
    "omni-moderation-latest": {"input": 0.0, "output": 0.0},
    # Anthropic Models
    "claude-3-haiku-20240307": {"input": 0.25e-6, "output": 1.25e-6},
    "claude-3-5-haiku-20241022": {"input": 1.0e-6, "output": 5.0e-6},
    "claude-3-5-sonnet-20241022": {"input": 3.0e-6, "output": 15.0e-6},
}

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


def calculate_cost(
    model: str,
    prompt_tokens: int,
    completion_tokens: int = 0,
) -> float:
    """
    Calculate the estimated cost for API usage.

    Args:
        model: Model name (e.g., "gpt-4o-mini")
        prompt_tokens: Number of input/prompt tokens
        completion_tokens: Number of output/completion tokens

    Returns:
        Estimated cost in USD
    """
    pricing = MODEL_PRICING.get(model, {"input": 0.0, "output": 0.0})
    input_cost = prompt_tokens * pricing["input"]
    output_cost = completion_tokens * pricing["output"]
    return input_cost + output_cost


def add_analyzer_attributes(
    analysis_type: str,
    provider: str,
    model: str,
    is_local: bool,
    batch_size: int,
    usage: dict | None = None,
) -> None:
    """
    Add analyzer-specific attributes to the current transaction.

    Args:
        analysis_type: Type of analysis (sentiment, toxicity, etc.)
        provider: Provider name (local, openai, anthropic, etc.)
        model: Model name used
        is_local: Whether this is a local provider
        batch_size: Number of texts in the batch
        usage: Token usage dict with prompt_tokens, completion_tokens, total_tokens
    """
    prefix = analysis_type.lower()

    attributes = {
        f"{prefix}.provider": provider,
        f"{prefix}.model": model,
        f"{prefix}.is_local": is_local,
        f"{prefix}.batch_size": batch_size,
    }

    if usage:
        prompt_tokens = usage.get("prompt_tokens", 0)
        completion_tokens = usage.get("completion_tokens", 0)
        total_tokens = usage.get("total_tokens", 0)
        model_name = usage.get("model", model)

        attributes[f"{prefix}.tokens.prompt"] = prompt_tokens
        attributes[f"{prefix}.tokens.completion"] = completion_tokens
        attributes[f"{prefix}.tokens.total"] = total_tokens

        # Calculate and add cost
        cost = calculate_cost(model_name, prompt_tokens, completion_tokens)
        if cost > 0:
            attributes[f"{prefix}.cost.estimated"] = cost

    add_custom_attributes(attributes)


def record_batch_ai_metrics(
    total_tokens: int,
    total_cost: float,
    api_calls: int,
    local_inferences: int,
) -> None:
    """
    Record aggregate AI metrics for a batch.

    Args:
        total_tokens: Total tokens used across all API calls
        total_cost: Total estimated cost in USD
        api_calls: Number of API calls made
        local_inferences: Number of local model inferences
    """
    record_custom_metric("Custom/AI/TotalTokens", total_tokens)
    record_custom_metric("Custom/AI/EstimatedCost", total_cost)
    record_custom_metric("Custom/AI/APICalls", api_calls)
    record_custom_metric("Custom/AI/LocalInferences", local_inferences)

    add_custom_attributes({
        "ai.batch.total_tokens": total_tokens,
        "ai.batch.estimated_cost": total_cost,
        "ai.batch.api_calls": api_calls,
        "ai.batch.local_inferences": local_inferences,
    })
