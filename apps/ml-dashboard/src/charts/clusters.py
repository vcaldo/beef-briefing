"""
Embedding cluster visualization using UMAP projections.
"""

import numpy as np
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go


SENTIMENT_COLORS = {
    "positive": "#22c55e",  # Green
    "neutral": "#94a3b8",   # Slate
    "negative": "#ef4444",  # Red
}


def create_embedding_scatter(
    coords: np.ndarray,
    metadata: list[dict],
    sentiment_labels: pd.Series | None = None,
    color_by: str = "sentiment",
    selected_user_id: int | None = None,
) -> go.Figure:
    """
    Create a 2D scatter plot of message embeddings.

    Args:
        coords: (N, 2) array of UMAP coordinates
        metadata: List of dicts with message_id, user_id, text_preview
        sentiment_labels: Optional series mapping message_id to sentiment
        color_by: "sentiment" or "user"
        selected_user_id: Optional user ID to highlight

    Returns:
        Plotly Figure object
    """
    if len(coords) == 0:
        fig = go.Figure()
        fig.add_annotation(
            text="No embeddings available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        fig.update_layout(
            template="plotly_dark",
            xaxis=dict(visible=False),
            yaxis=dict(visible=False),
        )
        return fig

    # Build DataFrame for plotting
    df = pd.DataFrame({
        "x": coords[:, 0],
        "y": coords[:, 1],
        "message_id": [m["message_id"] for m in metadata],
        "user_id": [m.get("user_id") for m in metadata],
        "text": [m.get("text_preview", "")[:50] + "..." for m in metadata],
    })

    # Add sentiment labels if provided
    if sentiment_labels is not None and color_by == "sentiment":
        df["sentiment"] = df["message_id"].map(
            dict(zip(sentiment_labels.index, sentiment_labels.values))
        ).fillna("unknown")

        fig = px.scatter(
            df,
            x="x",
            y="y",
            color="sentiment",
            color_discrete_map={
                **SENTIMENT_COLORS,
                "unknown": "#666666",
            },
            hover_data=["text"],
            opacity=0.6,
        )
    elif color_by == "user" and selected_user_id is not None:
        # Highlight selected user vs others
        df["is_selected"] = df["user_id"] == selected_user_id
        df["group"] = df["is_selected"].map({True: "Selected User", False: "Others"})

        fig = px.scatter(
            df,
            x="x",
            y="y",
            color="group",
            color_discrete_map={
                "Selected User": "#f59e0b",
                "Others": "#475569",
            },
            hover_data=["text"],
            opacity=0.7,
        )
    else:
        # Default: color by user_id
        fig = px.scatter(
            df,
            x="x",
            y="y",
            color="user_id",
            hover_data=["text"],
            opacity=0.6,
        )
        fig.update_layout(coloraxis_showscale=False)

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(
            showticklabels=False,
            title="",
            showgrid=False,
            zeroline=False,
        ),
        yaxis=dict(
            showticklabels=False,
            title="",
            showgrid=False,
            zeroline=False,
        ),
        margin=dict(l=20, r=20, t=40, b=20),
        height=600,
        legend=dict(
            orientation="h",
            yanchor="bottom",
            y=1.02,
            xanchor="center",
            x=0.5,
        ),
    )

    fig.update_traces(marker=dict(size=6))

    return fig


def create_embedding_scatter_3d(
    coords: np.ndarray,
    metadata: list[dict],
    sentiment_labels: pd.Series | None = None,
) -> go.Figure:
    """
    Create a 3D scatter plot of message embeddings.

    Args:
        coords: (N, 3) array of UMAP coordinates
        metadata: List of dicts with message_id, user_id, text_preview
        sentiment_labels: Optional series mapping message_id to sentiment

    Returns:
        Plotly Figure object
    """
    if len(coords) == 0 or coords.shape[1] < 3:
        fig = go.Figure()
        fig.add_annotation(
            text="No 3D embeddings available",
            xref="paper",
            yref="paper",
            x=0.5,
            y=0.5,
            showarrow=False,
            font=dict(size=16, color="#888"),
        )
        return fig

    df = pd.DataFrame({
        "x": coords[:, 0],
        "y": coords[:, 1],
        "z": coords[:, 2],
        "message_id": [m["message_id"] for m in metadata],
        "text": [m.get("text_preview", "")[:50] + "..." for m in metadata],
    })

    if sentiment_labels is not None:
        df["sentiment"] = df["message_id"].map(
            dict(zip(sentiment_labels.index, sentiment_labels.values))
        ).fillna("unknown")

        fig = px.scatter_3d(
            df,
            x="x",
            y="y",
            z="z",
            color="sentiment",
            color_discrete_map={
                **SENTIMENT_COLORS,
                "unknown": "#666666",
            },
            hover_data=["text"],
            opacity=0.6,
        )
    else:
        fig = px.scatter_3d(
            df,
            x="x",
            y="y",
            z="z",
            hover_data=["text"],
            opacity=0.6,
        )

    fig.update_layout(
        template="plotly_dark",
        scene=dict(
            xaxis=dict(showticklabels=False, title=""),
            yaxis=dict(showticklabels=False, title=""),
            zaxis=dict(showticklabels=False, title=""),
        ),
        margin=dict(l=0, r=0, t=40, b=0),
        height=600,
    )

    fig.update_traces(marker=dict(size=3))

    return fig
