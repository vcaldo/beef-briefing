"""
Embedding Explorer page - interactive UMAP visualization.

Shows:
- 2D/3D UMAP projection of message embeddings
- Color by sentiment or user
- Interactive hover to see message previews
"""

import streamlit as st
import pandas as pd

from src.charts.clusters import create_embedding_scatter, create_embedding_scatter_3d
from src.database.queries import MLDashboardQueries
from src.vector.qdrant_client import EmbeddingsClient
from src.embeddings.reducer import EmbeddingReducer


def render_embedding_explorer(
    queries: MLDashboardQueries,
    chat_id: int,
    qdrant: EmbeddingsClient,
    reducer: EmbeddingReducer,
) -> None:
    """
    Render the embedding explorer page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
        qdrant: Qdrant client for embeddings
        reducer: UMAP reducer
    """
    st.header("Embedding Explorer")

    # Check if Qdrant is available
    if not qdrant.is_available():
        st.error(
            "Qdrant vector database is not available. "
            "Make sure it's running and the collection exists."
        )
        return

    # Controls
    col1, col2, col3 = st.columns([1, 1, 2])

    with col1:
        n_components = st.radio(
            "Dimensions",
            options=[2, 3],
            horizontal=True,
            index=0,
        )

    with col2:
        color_by = st.radio(
            "Color by",
            options=["Sentiment", "User"],
            horizontal=True,
            index=0,
        )

    with col3:
        max_points = st.slider(
            "Max points",
            min_value=1000,
            max_value=10000,
            value=5000,
            step=1000,
            help="More points = slower but more complete view",
        )

    st.divider()

    # Load embeddings
    with st.spinner("Loading embeddings from Qdrant..."):
        embeddings, metadata = qdrant.get_embeddings_for_chat(chat_id, limit=max_points)

    if len(embeddings) == 0:
        st.warning(
            "No embeddings found for this chat. "
            "Run the ML processor to generate embeddings."
        )
        return

    st.info(f"Loaded {len(embeddings):,} message embeddings")

    # Compute UMAP
    with st.spinner(f"Computing {n_components}D UMAP projection (this may take a moment)..."):
        coords = reducer.reduce(embeddings, n_components=n_components)

    # Get sentiment labels for coloring
    sentiment_labels = None
    if color_by == "Sentiment":
        messages_df = queries.get_messages_with_sentiment(chat_id, limit=max_points)
        if not messages_df.empty:
            sentiment_labels = messages_df.set_index("message_id")["sentiment_label"]

    # Create visualization
    if n_components == 2:
        fig = create_embedding_scatter(
            coords,
            metadata,
            sentiment_labels=sentiment_labels,
            color_by=color_by.lower(),
        )
    else:
        fig = create_embedding_scatter_3d(
            coords,
            metadata,
            sentiment_labels=sentiment_labels,
        )

    st.plotly_chart(fig, use_container_width=True)

    # Stats
    st.divider()

    col1, col2, col3 = st.columns(3)

    with col1:
        collection_info = qdrant.get_collection_info()
        st.metric("Total Embeddings", f"{collection_info.get('points_count', 0):,}")

    with col2:
        st.metric("Displayed", f"{len(embeddings):,}")

    with col3:
        if sentiment_labels is not None:
            sentiment_counts = sentiment_labels.value_counts()
            dominant = sentiment_counts.index[0] if len(sentiment_counts) > 0 else "N/A"
            st.metric("Dominant Sentiment", dominant.title())

    # Optional: show sample messages
    with st.expander("Sample Messages", expanded=False):
        if metadata:
            sample_df = pd.DataFrame(metadata[:20])
            if sentiment_labels is not None:
                sample_df["sentiment"] = sample_df["message_id"].map(
                    dict(zip(sentiment_labels.index, sentiment_labels.values))
                )
            st.dataframe(sample_df, use_container_width=True, hide_index=True)
