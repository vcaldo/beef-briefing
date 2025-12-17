"""
Metric card components for the dashboard.
"""

import streamlit as st


def render_metric_cards(
    sentiment_stats: dict,
    toxicity_stats: dict,
) -> None:
    """
    Render a row of metric cards showing key stats.

    Args:
        sentiment_stats: Dict with total_analyzed, positive_count, etc.
        toxicity_stats: Dict with total_analyzed, toxic_count, toxic_rate
    """
    col1, col2, col3, col4 = st.columns(4)

    total_analyzed = sentiment_stats.get("total_analyzed", 0)
    positive_count = sentiment_stats.get("positive_count", 0)
    negative_count = sentiment_stats.get("negative_count", 0)
    toxic_rate = toxicity_stats.get("toxic_rate", 0)

    # Calculate rates
    positive_rate = (
        (positive_count / total_analyzed * 100) if total_analyzed > 0 else 0
    )
    negative_rate = (
        (negative_count / total_analyzed * 100) if total_analyzed > 0 else 0
    )

    with col1:
        st.metric(
            label="Messages Analyzed",
            value=f"{total_analyzed:,}",
        )

    with col2:
        st.metric(
            label="Positive",
            value=f"{positive_rate:.1f}%",
            delta=None,
        )

    with col3:
        st.metric(
            label="Negative",
            value=f"{negative_rate:.1f}%",
            delta=None,
        )

    with col4:
        st.metric(
            label="Toxic",
            value=f"{toxic_rate:.1f}%",
            delta=None,
        )


def render_user_comparison_metrics(comparison: dict) -> None:
    """
    Render metrics comparing user to group average.

    Args:
        comparison: Dict with user_avg_sentiment, group_avg_sentiment, etc.
    """
    col1, col2 = st.columns(2)

    user_sentiment = comparison.get("user_avg_sentiment", 0) or 0
    group_sentiment = comparison.get("group_avg_sentiment", 0) or 0
    user_toxicity = comparison.get("user_toxicity_rate", 0) or 0
    group_toxicity = comparison.get("group_toxicity_rate", 0) or 0

    sentiment_diff = user_sentiment - group_sentiment
    toxicity_diff = user_toxicity - group_toxicity

    with col1:
        st.metric(
            label="Sentiment vs Group",
            value=f"{user_sentiment:.2f}",
            delta=f"{sentiment_diff:+.2f}",
            delta_color="normal",
        )

    with col2:
        st.metric(
            label="Toxicity vs Group",
            value=f"{user_toxicity:.1f}%",
            delta=f"{toxicity_diff:+.1f}%",
            delta_color="inverse",
        )

    # Percentile info
    sentiment_pct = comparison.get("sentiment_percentile", 0) or 0
    toxicity_pct = comparison.get("toxicity_percentile", 0) or 0

    st.caption(
        f"Sentiment: Top {100 - sentiment_pct:.0f}% | "
        f"Toxicity: {'Top' if toxicity_pct < 50 else 'Bottom'} {min(toxicity_pct, 100-toxicity_pct):.0f}%"
    )
