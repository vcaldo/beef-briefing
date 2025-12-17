"""
ML Analytics Dashboard - Streamlit Application

A visualization tool for exploring ML-processed Telegram chat data,
including sentiment analysis, toxicity detection, and message embeddings.
"""

import streamlit as st

from config import load_config
from src.database.connection import get_engine
from src.database.queries import MLDashboardQueries
from src.vector.qdrant_client import EmbeddingsClient
from src.embeddings.reducer import EmbeddingReducer
from src.components.sidebar import render_sidebar
from src.pages.group_overview import render_group_overview
from src.pages.user_drilldown import render_user_drilldown
from src.pages.embedding_explorer import render_embedding_explorer


# Page configuration
st.set_page_config(
    page_title="ML Analytics Dashboard",
    page_icon="🧠",
    layout="wide",
    initial_sidebar_state="expanded",
)

# Custom CSS for dark theme consistency
st.markdown(
    """
    <style>
    .stMetric {
        background-color: rgba(255, 255, 255, 0.05);
        padding: 1rem;
        border-radius: 0.5rem;
    }
    </style>
    """,
    unsafe_allow_html=True,
)


@st.cache_resource
def get_resources():
    """Initialize and cache application resources."""
    config = load_config()
    engine = get_engine(config)
    queries = MLDashboardQueries(engine)
    qdrant = EmbeddingsClient(config.qdrant_host, config.qdrant_port)
    reducer = EmbeddingReducer(config.cache_dir)
    return config, queries, qdrant, reducer


def main():
    """Main application entry point."""
    # Initialize resources
    config, queries, qdrant, reducer = get_resources()

    # Render sidebar and get selections
    chat_id, user_id, page = render_sidebar(queries)

    if chat_id is None:
        st.warning("No chats with ML data available. Please run the ML processor first.")
        return

    # Route to appropriate page
    if page == "Group Overview":
        render_group_overview(queries, chat_id)

    elif page == "User Analysis":
        if user_id is None:
            st.info("Select a user from the sidebar to view their analysis.")
        else:
            render_user_drilldown(
                queries, chat_id, user_id, qdrant=qdrant, reducer=reducer
            )

    elif page == "Embedding Explorer":
        render_embedding_explorer(queries, chat_id, qdrant, reducer)


if __name__ == "__main__":
    main()
