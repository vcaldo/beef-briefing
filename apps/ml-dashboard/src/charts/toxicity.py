"""
Toxicity visualization charts.
"""

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go


def create_toxicity_timeline(df: pd.DataFrame) -> go.Figure:
    """
    Create a timeline chart showing toxicity rate over time.

    Args:
        df: DataFrame with columns: date, toxic_count, total_count, toxic_rate

    Returns:
        Plotly Figure object
    """
    if df.empty:
        fig = go.Figure()
        fig.add_annotation(
            text="No toxicity data available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    # Create dual-axis chart
    fig = go.Figure()

    # Bar chart for toxic count
    fig.add_trace(
        go.Bar(
            x=df["date"],
            y=df["toxic_count"],
            name="Toxic Messages",
            marker_color="#ef4444",
            opacity=0.7,
            yaxis="y",
        )
    )

    # Line chart for toxicity rate
    fig.add_trace(
        go.Scatter(
            x=df["date"],
            y=df["toxic_rate"],
            name="Toxicity Rate",
            mode="lines+markers",
            line=dict(color="#f59e0b", width=2),
            marker=dict(size=4),
            yaxis="y2",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(
            title="Date",
            gridcolor="rgba(255, 255, 255, 0.1)",
        ),
        yaxis=dict(
            title="Toxic Messages",
            gridcolor="rgba(255, 255, 255, 0.1)",
            side="left",
        ),
        yaxis2=dict(
            title="Toxicity Rate (%)",
            overlaying="y",
            side="right",
            ticksuffix="%",
            range=[0, max(df["toxic_rate"].max() * 1.2, 10)],
        ),
        legend=dict(
            orientation="h",
            yanchor="bottom",
            y=1.02,
            xanchor="center",
            x=0.5,
        ),
        margin=dict(l=60, r=60, t=40, b=60),
        height=400,
        hovermode="x unified",
    )

    return fig


def create_toxicity_heatmap(df: pd.DataFrame) -> go.Figure:
    """
    Create a heatmap showing toxicity patterns by day and hour.

    Args:
        df: DataFrame with columns: day_of_week (0-6), hour (0-23), toxic_rate

    Returns:
        Plotly Figure object
    """
    if df.empty:
        fig = go.Figure()
        fig.add_annotation(
            text="No toxicity pattern data available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    # Pivot for heatmap
    pivot = df.pivot(index="day_of_week", columns="hour", values="toxic_rate")
    pivot = pivot.fillna(0)

    days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
    hours = [f"{h:02d}:00" for h in range(24)]

    fig = go.Figure(
        data=go.Heatmap(
            z=pivot.values,
            x=hours,
            y=[days[i] for i in pivot.index],
            colorscale=[
                [0, "#1e293b"],
                [0.5, "#f59e0b"],
                [1, "#ef4444"],
            ],
            colorbar=dict(title="Toxicity %"),
            hovertemplate="Day: %{y}<br>Hour: %{x}<br>Toxicity: %{z:.1f}%<extra></extra>",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(
            title="Hour of Day",
            tickangle=-45,
        ),
        yaxis=dict(
            title="Day of Week",
        ),
        margin=dict(l=60, r=20, t=40, b=80),
        height=350,
    )

    return fig


def create_user_toxicity_bar(users: list[dict], limit: int = 10) -> go.Figure:
    """
    Create a horizontal bar chart of most toxic users.

    Args:
        users: List of dicts with first_name, toxicity_rate, messages_analyzed
        limit: Max number of users to show

    Returns:
        Plotly Figure object
    """
    if not users:
        fig = go.Figure()
        fig.add_annotation(
            text="No toxic users found",
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
    df = df.sort_values("toxicity_rate", ascending=True)

    fig = go.Figure(
        go.Bar(
            x=df["toxicity_rate"],
            y=df["first_name"],
            orientation="h",
            marker=dict(
                color=df["toxicity_rate"],
                colorscale=[[0, "#f59e0b"], [1, "#ef4444"]],
            ),
            text=[f"{r:.1f}%" for r in df["toxicity_rate"]],
            textposition="outside",
            hovertemplate=(
                "<b>%{y}</b><br>"
                "Toxicity: %{x:.1f}%<br>"
                "<extra></extra>"
            ),
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(
            title="Toxicity Rate (%)",
            range=[0, df["toxicity_rate"].max() * 1.3],
        ),
        yaxis=dict(title=""),
        margin=dict(l=100, r=60, t=20, b=40),
        height=max(200, len(df) * 35 + 60),
    )

    return fig
