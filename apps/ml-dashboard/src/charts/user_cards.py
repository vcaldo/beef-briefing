"""
User Cards visualizations.
"""

import pandas as pd
import plotly.graph_objects as go


STAT_COLORS = {
    "mood": "#22c55e",
    "volatility": "#f59e0b",
    "toxicity": "#ef4444",
    "activity": "#3b82f6",
    "reactions_received": "#8b5cf6",
    "chronotype": "#06b6d4",
    "influence": "#ec4899",
    "humor": "#fbbf24",
}

STAT_LABELS = {
    "mood": "Mood",
    "volatility": "Volatility",
    "toxicity": "Toxicity",
    "activity": "Activity",
    "reactions_received": "Reactions",
    "chronotype": "Chronotype",
    "influence": "Influence",
    "humor": "Humor",
}


def create_user_card_radar(stats: dict, user_name: str) -> go.Figure:
    """
    Create a radar chart from user card stats.

    Args:
        stats: Dict with stat keys and values (0-1 normalized)
        user_name: User name for title

    Returns:
        Plotly Figure object
    """
    if not stats:
        fig = go.Figure()
        fig.add_annotation(
            text="No stats available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    # Filter to only include known stats
    filtered_stats = {k: v for k, v in stats.items() if k in STAT_LABELS}
    if not filtered_stats:
        fig = go.Figure()
        fig.add_annotation(
            text="No recognized stats",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    categories = list(filtered_stats.keys())
    values = list(filtered_stats.values())

    # Close the radar chart
    categories_closed = categories + [categories[0]]
    values_closed = values + [values[0]]

    fig = go.Figure()

    fig.add_trace(
        go.Scatterpolar(
            r=values_closed,
            theta=[STAT_LABELS.get(c, c) for c in categories_closed],
            fill="toself",
            fillcolor="rgba(59, 130, 246, 0.3)",
            line=dict(color="#3b82f6", width=2),
            name=user_name,
        )
    )

    fig.update_layout(
        template="plotly_dark",
        polar=dict(
            radialaxis=dict(
                visible=True,
                range=[0, 1],
                tickvals=[0.25, 0.5, 0.75, 1.0],
                gridcolor="rgba(255, 255, 255, 0.2)",
            ),
            angularaxis=dict(gridcolor="rgba(255, 255, 255, 0.2)"),
            bgcolor="rgba(0, 0, 0, 0)",
        ),
        showlegend=False,
        margin=dict(l=60, r=60, t=40, b=40),
        height=400,
    )

    return fig


def create_stat_trend_line(history: list[dict], stat_key: str) -> go.Figure:
    """
    Create a line chart showing a stat's trend over weeks.

    Args:
        history: List of dicts with week_start, stats
        stat_key: Which stat to plot

    Returns:
        Plotly Figure object
    """
    if not history:
        fig = go.Figure()
        fig.add_annotation(
            text="No history available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    weeks = [h["week_start"] for h in history]
    values = [h["stats"].get(stat_key, 0) if h.get("stats") else 0 for h in history]

    color = STAT_COLORS.get(stat_key, "#3b82f6")
    label = STAT_LABELS.get(stat_key, stat_key)

    # Parse color for rgba
    r = int(color[1:3], 16)
    g = int(color[3:5], 16)
    b = int(color[5:7], 16)

    fig = go.Figure()

    fig.add_trace(
        go.Scatter(
            x=weeks,
            y=values,
            mode="lines+markers",
            line=dict(color=color, width=3),
            marker=dict(size=8),
            fill="tozeroy",
            fillcolor=f"rgba({r}, {g}, {b}, 0.2)",
            hovertemplate=f"Week: %{{x}}<br>{label}: %{{y:.2f}}<extra></extra>",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title="Week", gridcolor="rgba(255, 255, 255, 0.1)"),
        yaxis=dict(
            title=label, range=[0, 1], gridcolor="rgba(255, 255, 255, 0.1)"
        ),
        margin=dict(l=60, r=20, t=20, b=60),
        height=250,
    )

    return fig


def create_weekly_leaderboard_bar(
    users: list[dict],
    stat_key: str,
) -> go.Figure:
    """
    Create a horizontal bar chart for weekly stat leaderboard.

    Args:
        users: List of dicts with first_name, value
        stat_key: Stat being displayed

    Returns:
        Plotly Figure object
    """
    if not users:
        fig = go.Figure()
        fig.add_annotation(
            text="No leaderboard data",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    df = pd.DataFrame(users)
    df = df.sort_values("value", ascending=True)

    color = STAT_COLORS.get(stat_key, "#3b82f6")
    label = STAT_LABELS.get(stat_key, stat_key)

    fig = go.Figure(
        go.Bar(
            x=df["value"],
            y=df["first_name"],
            orientation="h",
            marker_color=color,
            text=[f"{v:.2f}" for v in df["value"]],
            textposition="outside",
            hovertemplate=f"<b>%{{y}}</b><br>{label}: %{{x:.2f}}<extra></extra>",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title=label, range=[0, min(float(df["value"].max()) * 1.3, 1.0)]),
        yaxis=dict(title=""),
        margin=dict(l=100, r=60, t=20, b=40),
        height=max(200, len(df) * 35 + 60),
    )

    return fig


def create_multi_stat_comparison(
    users: list[dict],
    stat_keys: list[str],
) -> go.Figure:
    """
    Create a grouped bar chart comparing multiple stats across users.

    Args:
        users: List of dicts with first_name, stats dict
        stat_keys: List of stat keys to compare

    Returns:
        Plotly Figure object
    """
    if not users or not stat_keys:
        fig = go.Figure()
        fig.add_annotation(
            text="No comparison data",
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

    names = [u["first_name"] for u in users]

    for stat_key in stat_keys:
        values = [
            u["stats"].get(stat_key, 0) if u.get("stats") else 0 for u in users
        ]
        fig.add_trace(
            go.Bar(
                name=STAT_LABELS.get(stat_key, stat_key),
                x=names,
                y=values,
                marker_color=STAT_COLORS.get(stat_key, "#6b7280"),
            )
        )

    fig.update_layout(
        template="plotly_dark",
        barmode="group",
        xaxis=dict(title=""),
        yaxis=dict(
            title="Value", range=[0, 1], gridcolor="rgba(255, 255, 255, 0.1)"
        ),
        legend=dict(
            orientation="h", yanchor="bottom", y=1.02, xanchor="center", x=0.5
        ),
        margin=dict(l=60, r=20, t=60, b=60),
        height=400,
    )

    return fig
