"""
Content analysis visualizations (humor and questions).
"""

import pandas as pd
import plotly.graph_objects as go


HUMOR_TYPE_COLORS = {
    "joke": "#f59e0b",
    "sarcasm": "#8b5cf6",
    "wordplay": "#06b6d4",
    "irony": "#ec4899",
    "unknown": "#6b7280",
}

QUESTION_TYPE_COLORS = {
    "factual": "#3b82f6",
    "opinion": "#22c55e",
    "rhetorical": "#f59e0b",
    "clarification": "#8b5cf6",
    "unknown": "#6b7280",
}


def create_type_donut(
    df: pd.DataFrame,
    type_col: str,
    colors: dict,
    title: str,
) -> go.Figure:
    """
    Create a donut chart for type distribution.

    Args:
        df: DataFrame with type_col, count columns
        type_col: Column name for the type
        colors: Dict mapping type to color
        title: Chart title

    Returns:
        Plotly Figure object
    """
    if df.empty:
        fig = go.Figure()
        fig.add_annotation(
            text="No data available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    fig = go.Figure(
        data=[
            go.Pie(
                labels=df[type_col],
                values=df["count"],
                hole=0.5,
                marker=dict(colors=[colors.get(t, "#6b7280") for t in df[type_col]]),
                textinfo="label+percent",
                hovertemplate="<b>%{label}</b><br>Count: %{value}<br>%{percent}<extra></extra>",
            )
        ]
    )

    fig.update_layout(
        template="plotly_dark",
        title=title,
        showlegend=False,
        margin=dict(l=20, r=20, t=60, b=20),
        height=300,
    )

    return fig


def create_content_timeline(
    df: pd.DataFrame,
    count_col: str,
    rate_col: str,
    color: str,
    label: str,
) -> go.Figure:
    """
    Create a timeline chart for content analysis (humor or questions).

    Args:
        df: DataFrame with date, count_col, rate_col columns
        count_col: Column name for absolute counts
        rate_col: Column name for rate percentage
        color: Primary color for the chart
        label: Label for the metric (e.g., "Humorous" or "Questions")

    Returns:
        Plotly Figure object
    """
    if df.empty:
        fig = go.Figure()
        fig.add_annotation(
            text="No timeline data available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    fig = go.Figure()

    # Bar chart for count
    fig.add_trace(
        go.Bar(
            x=df["date"],
            y=df[count_col],
            name=f"{label} Messages",
            marker_color=color,
            opacity=0.7,
            yaxis="y",
        )
    )

    # Line chart for rate
    fig.add_trace(
        go.Scatter(
            x=df["date"],
            y=df[rate_col],
            name=f"{label} Rate",
            mode="lines+markers",
            line=dict(color="#ffffff", width=2),
            marker=dict(size=4),
            yaxis="y2",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title="Date", gridcolor="rgba(255, 255, 255, 0.1)"),
        yaxis=dict(
            title=f"{label} Messages", gridcolor="rgba(255, 255, 255, 0.1)", side="left"
        ),
        yaxis2=dict(
            title=f"{label} Rate (%)",
            overlaying="y",
            side="right",
            ticksuffix="%",
            range=[0, max(float(df[rate_col].max()) * 1.2, 10)],
        ),
        legend=dict(
            orientation="h", yanchor="bottom", y=1.02, xanchor="center", x=0.5
        ),
        margin=dict(l=60, r=60, t=40, b=60),
        height=400,
        hovermode="x unified",
    )

    return fig


def create_user_ranking_bar(
    users: list[dict],
    rate_col: str,
    color: str,
    label: str,
    limit: int = 10,
) -> go.Figure:
    """
    Create a horizontal bar chart for user rankings.

    Args:
        users: List of dicts with first_name, rate_col
        rate_col: Column name for the rate metric
        color: Bar color
        label: Metric label
        limit: Max users to show

    Returns:
        Plotly Figure object
    """
    if not users:
        fig = go.Figure()
        fig.add_annotation(
            text=f"No {label.lower()} users found",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    df = pd.DataFrame(users[:limit])
    df = df.sort_values(rate_col, ascending=True)

    fig = go.Figure(
        go.Bar(
            x=df[rate_col],
            y=df["first_name"],
            orientation="h",
            marker_color=color,
            text=[f"{r:.1f}%" for r in df[rate_col]],
            textposition="outside",
            hovertemplate=f"<b>%{{y}}</b><br>{label}: %{{x:.1f}}%<extra></extra>",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title=f"{label} Rate (%)", range=[0, float(df[rate_col].max()) * 1.3]),
        yaxis=dict(title=""),
        margin=dict(l=100, r=60, t=20, b=40),
        height=max(200, len(df) * 35 + 60),
    )

    return fig
