"""
User behavior quadrant visualization.

Plots users on a 2D space:
- X-axis: Sentiment (-1 to +1)
- Y-axis: Toxicity rate (0% to 100%)
"""

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go


def create_behavior_quadrant(
    df: pd.DataFrame,
    selected_user_id: int | None = None,
) -> go.Figure:
    """
    Create a scatter plot showing user behavior in sentiment-toxicity space.

    Quadrants:
    - Top-left: Negative sentiment + High toxicity (HOSTILE)
    - Top-right: Positive sentiment + High toxicity (PASSIONATE)
    - Bottom-left: Negative sentiment + Low toxicity (CRITICAL)
    - Bottom-right: Positive sentiment + Low toxicity (FRIENDLY)

    Args:
        df: DataFrame with columns: user_id, first_name, avg_sentiment,
            toxicity_rate, messages_analyzed
        selected_user_id: Optional user ID to highlight

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
        fig.update_layout(
            template="plotly_dark",
            xaxis=dict(visible=False),
            yaxis=dict(visible=False),
        )
        return fig

    # Create display name
    df = df.copy()
    df["display_name"] = df["first_name"].fillna("Unknown")

    # Create the scatter plot
    fig = px.scatter(
        df,
        x="avg_sentiment",
        y="toxicity_rate",
        size="messages_analyzed",
        color="avg_sentiment",
        color_continuous_scale="RdYlGn",
        hover_name="display_name",
        hover_data={
            "avg_sentiment": ":.2f",
            "toxicity_rate": ":.1%",
            "messages_analyzed": True,
            "username": True,
        },
        labels={
            "avg_sentiment": "Sentiment Score",
            "toxicity_rate": "Toxicity Rate",
            "messages_analyzed": "Messages",
        },
    )

    # Add quadrant divider lines
    fig.add_hline(
        y=0.1,
        line_dash="dash",
        line_color="rgba(255, 255, 255, 0.3)",
        annotation_text="",
    )
    fig.add_vline(
        x=0,
        line_dash="dash",
        line_color="rgba(255, 255, 255, 0.3)",
        annotation_text="",
    )

    # Add quadrant labels
    annotations = [
        dict(
            x=-0.7,
            y=0.85,
            text="HOSTILE",
            showarrow=False,
            font=dict(size=12, color="rgba(255, 100, 100, 0.6)"),
        ),
        dict(
            x=0.7,
            y=0.85,
            text="PASSIONATE",
            showarrow=False,
            font=dict(size=12, color="rgba(255, 200, 100, 0.6)"),
        ),
        dict(
            x=-0.7,
            y=0.02,
            text="CRITICAL",
            showarrow=False,
            font=dict(size=12, color="rgba(100, 150, 255, 0.6)"),
        ),
        dict(
            x=0.7,
            y=0.02,
            text="FRIENDLY",
            showarrow=False,
            font=dict(size=12, color="rgba(100, 255, 150, 0.6)"),
        ),
    ]

    # Highlight selected user if any
    if selected_user_id is not None:
        selected = df[df["user_id"] == selected_user_id]
        if not selected.empty:
            fig.add_trace(
                go.Scatter(
                    x=selected["avg_sentiment"],
                    y=selected["toxicity_rate"],
                    mode="markers",
                    marker=dict(
                        size=20,
                        color="white",
                        line=dict(color="white", width=3),
                        symbol="circle-open",
                    ),
                    name="Selected",
                    hoverinfo="skip",
                )
            )

    fig.update_layout(
        template="plotly_dark",
        xaxis=dict(
            range=[-1.1, 1.1],
            title="Sentiment Score",
            tickvals=[-1, -0.5, 0, 0.5, 1],
            ticktext=["Negative", "", "Neutral", "", "Positive"],
            gridcolor="rgba(255, 255, 255, 0.1)",
        ),
        yaxis=dict(
            range=[-0.05, 1.0],
            title="Toxicity Rate",
            tickformat=".0%",
            gridcolor="rgba(255, 255, 255, 0.1)",
        ),
        coloraxis_showscale=False,
        annotations=annotations,
        margin=dict(l=60, r=20, t=40, b=60),
        height=500,
    )

    return fig
