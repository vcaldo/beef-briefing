"""
Named Entity Recognition visualizations.
"""

import pandas as pd
import plotly.graph_objects as go


ENTITY_TYPE_COLORS = {
    "PERSON": "#3b82f6",
    "ORG": "#22c55e",
    "LOC": "#f59e0b",
    "MISC": "#8b5cf6",
}


def create_entity_type_bar(df: pd.DataFrame) -> go.Figure:
    """
    Create a bar chart showing entity type distribution.

    Args:
        df: DataFrame with entity_type, count, unique_count columns

    Returns:
        Plotly Figure object
    """
    if df.empty:
        fig = go.Figure()
        fig.add_annotation(
            text="No entities found",
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

    fig.add_trace(
        go.Bar(
            x=df["entity_type"],
            y=df["count"],
            name="Total Mentions",
            marker_color=[
                ENTITY_TYPE_COLORS.get(t, "#6b7280") for t in df["entity_type"]
            ],
            text=df["count"],
            textposition="auto",
        )
    )

    fig.add_trace(
        go.Scatter(
            x=df["entity_type"],
            y=df["unique_count"],
            name="Unique Entities",
            mode="lines+markers",
            marker=dict(color="#ffffff", size=10),
            line=dict(color="#ffffff", width=2),
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title="Entity Type"),
        yaxis=dict(title="Count", gridcolor="rgba(255, 255, 255, 0.1)"),
        legend=dict(
            orientation="h", yanchor="bottom", y=1.02, xanchor="center", x=0.5
        ),
        margin=dict(l=60, r=20, t=60, b=60),
        height=350,
        barmode="group",
    )

    return fig


def create_top_entities_bar(
    entities: list[dict],
    entity_type: str | None = None,
) -> go.Figure:
    """
    Create a horizontal bar chart of top entities.

    Args:
        entities: List of dicts with entity_text, entity_type, mention_count
        entity_type: Optional type filter for consistent coloring

    Returns:
        Plotly Figure object
    """
    if not entities:
        fig = go.Figure()
        fig.add_annotation(
            text="No entities found",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    df = pd.DataFrame(entities)
    df = df.sort_values("mention_count", ascending=True).tail(15)

    colors = [ENTITY_TYPE_COLORS.get(t, "#6b7280") for t in df["entity_type"]]

    fig = go.Figure(
        go.Bar(
            x=df["mention_count"],
            y=df["entity_text"],
            orientation="h",
            marker_color=colors,
            text=df["mention_count"],
            textposition="outside",
            hovertemplate="<b>%{y}</b><br>Mentions: %{x}<br>Type: %{customdata}<extra></extra>",
            customdata=df["entity_type"],
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title="Mention Count"),
        yaxis=dict(title=""),
        margin=dict(l=150, r=60, t=20, b=40),
        height=max(300, len(df) * 30 + 60),
    )

    return fig


def create_entity_timeline(df: pd.DataFrame) -> go.Figure:
    """
    Create a timeline of entity mentions.

    Args:
        df: DataFrame with date, mention_count, unique_entities columns

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

    fig.add_trace(
        go.Bar(
            x=df["date"],
            y=df["mention_count"],
            name="Mentions",
            marker_color="#3b82f6",
            opacity=0.7,
        )
    )

    fig.add_trace(
        go.Scatter(
            x=df["date"],
            y=df["unique_entities"],
            name="Unique Entities",
            mode="lines+markers",
            line=dict(color="#f59e0b", width=2),
            marker=dict(size=4),
            yaxis="y2",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(title="Date", gridcolor="rgba(255, 255, 255, 0.1)"),
        yaxis=dict(title="Total Mentions", gridcolor="rgba(255, 255, 255, 0.1)"),
        yaxis2=dict(title="Unique Entities", overlaying="y", side="right"),
        legend=dict(
            orientation="h", yanchor="bottom", y=1.02, xanchor="center", x=0.5
        ),
        margin=dict(l=60, r=60, t=40, b=60),
        height=350,
        hovermode="x unified",
    )

    return fig


def create_cooccurrence_heatmap(df: pd.DataFrame) -> go.Figure:
    """
    Create a heatmap of entity co-occurrences.

    Args:
        df: DataFrame with entity1, entity2, cooccurrence_count columns

    Returns:
        Plotly Figure object
    """
    if df.empty or len(df) < 3:
        fig = go.Figure()
        fig.add_annotation(
            text="Not enough co-occurrence data",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(template="plotly_dark")
        return fig

    # Create a pivot table for heatmap
    pivot = df.pivot(index="entity1", columns="entity2", values="cooccurrence_count")
    pivot = pivot.fillna(0)

    fig = go.Figure(
        data=go.Heatmap(
            z=pivot.values,
            x=pivot.columns.tolist(),
            y=pivot.index.tolist(),
            colorscale=[[0, "#1e293b"], [0.5, "#3b82f6"], [1, "#22c55e"]],
            hovertemplate="<b>%{y}</b> + <b>%{x}</b><br>Co-occurrences: %{z}<extra></extra>",
        )
    )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(tickangle=-45),
        margin=dict(l=100, r=20, t=40, b=100),
        height=500,
    )

    return fig
