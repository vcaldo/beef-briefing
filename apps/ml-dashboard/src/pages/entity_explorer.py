"""
Entity Explorer page - Named Entity Recognition visualization.

Shows:
- Entity statistics
- Entity type distribution
- Top entities (filterable by type)
- Entity timeline
- Co-occurrence analysis
"""

import streamlit as st
import pandas as pd

from src.charts.entities import (
    create_entity_type_bar,
    create_top_entities_bar,
    create_entity_timeline,
    create_cooccurrence_heatmap,
)
from src.database.queries import MLDashboardQueries


def render_entity_explorer(queries: MLDashboardQueries, chat_id: int) -> None:
    """
    Render the entity explorer page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
    """
    st.header("Entity Explorer")
    st.caption("Discover people, organizations, and locations mentioned in the chat")

    # Key metrics
    ner_stats = queries.get_ner_stats(chat_id)

    col1, col2, col3, col4 = st.columns(4)
    with col1:
        st.metric("Total Mentions", f"{ner_stats.get('total_entities', 0):,}")
    with col2:
        st.metric("Unique Entities", f"{ner_stats.get('unique_entities', 0):,}")
    with col3:
        st.metric(
            "Messages with Entities", f"{ner_stats.get('messages_with_entities', 0):,}"
        )
    with col4:
        avg_conf = ner_stats.get("avg_confidence") or 0
        st.metric("Avg Confidence", f"{avg_conf:.2f}")

    st.divider()

    # Entity type distribution
    st.subheader("Entity Types")
    type_df = queries.get_entity_type_distribution(chat_id)
    if not type_df.empty:
        fig = create_entity_type_bar(type_df)
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No entity type data available")

    st.divider()

    # Top entities with filter
    col1, col2 = st.columns([3, 1])
    with col1:
        st.subheader("Top Entities")
    with col2:
        entity_filter = st.selectbox(
            "Filter by Type",
            options=["All", "PERSON", "ORG", "LOC", "MISC"],
            index=0,
            label_visibility="collapsed",
        )

    selected_type = None if entity_filter == "All" else entity_filter
    top_entities = queries.get_top_entities(
        chat_id, entity_type=selected_type, limit=20
    )
    if top_entities:
        fig = create_top_entities_bar(top_entities, entity_type=selected_type)
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No entities found")

    st.divider()

    # Timeline
    st.subheader("Entity Mentions Over Time")
    timeline_df = queries.get_entity_timeline(chat_id, entity_type=selected_type)
    if not timeline_df.empty:
        fig = create_entity_timeline(timeline_df)
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No timeline data available")

    st.divider()

    # Co-occurrence analysis
    with st.expander("Entity Co-occurrence", expanded=False):
        st.caption("Entities that frequently appear together in the same message")
        cooccur_df = queries.get_entity_cooccurrence(chat_id, limit=50)
        if not cooccur_df.empty:
            fig = create_cooccurrence_heatmap(cooccur_df)
            st.plotly_chart(fig, use_container_width=True)

            # Also show as table
            st.dataframe(
                cooccur_df.rename(
                    columns={
                        "entity1": "Entity 1",
                        "entity2": "Entity 2",
                        "cooccurrence_count": "Co-occurrences",
                    }
                ),
                use_container_width=True,
                hide_index=True,
            )
        else:
            st.info("Not enough co-occurrence data")

    # User entity mentions
    st.divider()
    st.subheader("Users Who Mention Most Entities")
    user_mentions = queries.get_user_entity_mentions(chat_id, limit=10)
    if user_mentions:
        df = pd.DataFrame(user_mentions)
        df["user"] = df.apply(
            lambda r: f"{r['first_name']} (@{r['username']})"
            if r.get("username")
            else r["first_name"],
            axis=1,
        )
        display_df = df[["user", "entity_mentions", "unique_entities"]].copy()
        display_df.columns = ["User", "Total Mentions", "Unique Entities"]
        st.dataframe(display_df, use_container_width=True, hide_index=True)
    else:
        st.info("No user entity data available")
