"""
User Drilldown page - individual user analysis.

Shows:
- User comparison to group averages
- User position on quadrant (highlighted)
- User sentiment timeline
- User's messages in embedding space
"""

import streamlit as st

from src.charts.quadrant import create_behavior_quadrant
from src.components.metrics import render_user_comparison_metrics
from src.database.queries import MLDashboardQueries
from src.vector.qdrant_client import EmbeddingsClient
from src.embeddings.reducer import EmbeddingReducer
from src.charts.clusters import create_embedding_scatter


def render_user_drilldown(
    queries: MLDashboardQueries,
    chat_id: int,
    user_id: int,
    qdrant: EmbeddingsClient | None = None,
    reducer: EmbeddingReducer | None = None,
) -> None:
    """
    Render the user drilldown page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
        user_id: Selected user ID
        qdrant: Optional Qdrant client for embeddings
        reducer: Optional UMAP reducer
    """
    # Get user details
    user = queries.get_user_details(user_id, chat_id)

    if not user:
        st.warning("User not found or has no analyzed messages")
        return

    # Header
    username = f"@{user['username']}" if user.get("username") else ""
    st.header(f"{user['first_name']} {username}")

    st.caption(
        f"Analyzed: {user['messages_analyzed']:,} / "
        f"{user['total_messages']:,} messages"
    )

    # Comparison metrics
    comparison = queries.get_user_vs_group_comparison(user_id, chat_id)
    render_user_comparison_metrics(comparison)

    st.divider()

    # Two column layout
    col1, col2 = st.columns(2)

    with col1:
        st.subheader("Position in Group")

        # Get all users for quadrant and highlight this one
        quadrant_df = queries.get_user_behavior_quadrant(chat_id)
        if not quadrant_df.empty:
            fig = create_behavior_quadrant(quadrant_df, selected_user_id=user_id)
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No quadrant data available")

    with col2:
        st.subheader("Sentiment Breakdown")

        # Show user's sentiment distribution
        avg_sentiment = user.get("avg_sentiment", 0) or 0
        toxicity_rate = user.get("toxicity_rate", 0) or 0
        variance = user.get("sentiment_variance", 0) or 0

        col2a, col2b = st.columns(2)
        with col2a:
            st.metric("Avg Sentiment", f"{avg_sentiment:.2f}")
            st.metric("Variance", f"{variance:.3f}")

        with col2b:
            st.metric("Toxicity Rate", f"{toxicity_rate:.1%}")

            # Interpret sentiment
            if avg_sentiment > 0.3:
                st.success("Generally Positive")
            elif avg_sentiment < -0.3:
                st.error("Generally Negative")
            else:
                st.info("Mostly Neutral")

    st.divider()

    # Sentiment timeline
    st.subheader("Sentiment Over Time")

    timeline_df = queries.get_user_sentiment_timeline(user_id, chat_id)
    if not timeline_df.empty:
        import plotly.graph_objects as go

        fig = go.Figure()

        # Stacked area for sentiment counts
        fig.add_trace(
            go.Scatter(
                x=timeline_df["date"],
                y=timeline_df["positive"],
                name="Positive",
                mode="lines",
                fill="tonexty",
                line=dict(color="#22c55e"),
                stackgroup="sentiment",
            )
        )
        fig.add_trace(
            go.Scatter(
                x=timeline_df["date"],
                y=timeline_df["neutral"],
                name="Neutral",
                mode="lines",
                fill="tonexty",
                line=dict(color="#94a3b8"),
                stackgroup="sentiment",
            )
        )
        fig.add_trace(
            go.Scatter(
                x=timeline_df["date"],
                y=timeline_df["negative"],
                name="Negative",
                mode="lines",
                fill="tonexty",
                line=dict(color="#ef4444"),
                stackgroup="sentiment",
            )
        )

        fig.update_layout(
            template="plotly_dark",
            xaxis_title="Date",
            yaxis_title="Messages",
            legend=dict(
                orientation="h",
                yanchor="bottom",
                y=1.02,
                xanchor="center",
                x=0.5,
            ),
            margin=dict(l=60, r=20, t=40, b=60),
            height=350,
        )

        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No sentiment timeline data available")

    st.divider()

    # Sample messages
    st.subheader("Sample Messages")

    sample_messages = queries.get_user_sample_messages(user_id, chat_id, limit=20)
    if sample_messages:
        import pandas as pd

        messages_df = pd.DataFrame(sample_messages)
        messages_df["date"] = pd.to_datetime(messages_df["date"]).dt.strftime(
            "%Y-%m-%d %H:%M"
        )
        # Truncate long messages for display
        messages_df["text"] = messages_df["text"].apply(
            lambda x: x[:150] + "..." if len(x) > 150 else x
        )
        # Format sentiment with emoji
        sentiment_emoji = {"positive": "😊", "neutral": "😐", "negative": "😠"}
        messages_df["sentiment"] = messages_df["sentiment_label"].map(
            lambda x: f"{sentiment_emoji.get(x, '')} {x}"
        )
        # Format negativity score as percentage
        messages_df["negativity"] = (messages_df["score_negative"] * 100).round(1)

        # Format toxicity
        messages_df["toxic"] = messages_df["is_toxic"].map(
            lambda x: "⚠️ Yes" if x else "No"
        )

        display_df = messages_df[["date", "text", "sentiment", "negativity", "toxic"]].copy()
        display_df.columns = ["Date", "Message", "Sentiment", "Negativity %", "Toxic"]

        st.dataframe(
            display_df,
            use_container_width=True,
            hide_index=True,
            column_config={
                "Message": st.column_config.TextColumn(width="large"),
            },
        )
    else:
        st.info("No messages available for this user")

    # Embedding visualization (if available)
    if qdrant and reducer and qdrant.is_available():
        st.divider()
        st.subheader("Message Embeddings")
        st.caption("This user's messages highlighted in the group's semantic space")

        with st.spinner("Loading embeddings..."):
            # Get all embeddings for the chat
            embeddings, metadata = qdrant.get_embeddings_for_chat(chat_id, limit=5000)

            if len(embeddings) > 0:
                # Reduce to 2D
                coords = reducer.reduce(embeddings, n_components=2)

                # Get sentiment labels
                messages_df = queries.get_messages_with_sentiment(chat_id, limit=10000)
                sentiment_map = dict(
                    zip(messages_df["message_id"], messages_df["sentiment_label"])
                )
                sentiment_labels = messages_df.set_index("message_id")["sentiment_label"]

                fig = create_embedding_scatter(
                    coords,
                    metadata,
                    sentiment_labels=sentiment_labels,
                    color_by="user",
                    selected_user_id=user_id,
                )
                st.plotly_chart(fig, use_container_width=True)
            else:
                st.info("No embeddings available for this chat")
