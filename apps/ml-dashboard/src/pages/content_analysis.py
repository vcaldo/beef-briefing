"""
Content Analysis page - Humor and Questions exploration.

Shows:
- Key metrics (humor rate, question rate)
- Type distributions (donut charts)
- User rankings (funniest, most curious)
- Timelines
- Top messages
"""

import streamlit as st
import pandas as pd

from src.charts.content import (
    create_type_donut,
    create_content_timeline,
    create_user_ranking_bar,
    HUMOR_TYPE_COLORS,
    QUESTION_TYPE_COLORS,
)
from src.database.queries import MLDashboardQueries


def render_content_analysis(queries: MLDashboardQueries, chat_id: int) -> None:
    """
    Render the content analysis page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
    """
    st.header("Content Analysis")
    st.caption("Explore humor and questions in the conversation")

    # Tabs for Humor vs Questions
    tab_humor, tab_questions = st.tabs(["Humor", "Questions"])

    with tab_humor:
        _render_humor_section(queries, chat_id)

    with tab_questions:
        _render_questions_section(queries, chat_id)


def _render_humor_section(queries: MLDashboardQueries, chat_id: int) -> None:
    """Render the humor analysis section."""
    # Key metrics
    humor_stats = queries.get_humor_stats(chat_id)

    col1, col2, col3, col4 = st.columns(4)
    with col1:
        st.metric("Messages Analyzed", f"{humor_stats.get('total_analyzed', 0):,}")
    with col2:
        st.metric("Humorous Messages", f"{humor_stats.get('humorous_count', 0):,}")
    with col3:
        st.metric("Humor Rate", f"{humor_stats.get('humor_rate', 0):.1f}%")
    with col4:
        avg_score = humor_stats.get("avg_score") or 0
        st.metric("Avg Confidence", f"{avg_score:.2f}")

    st.divider()

    # Two columns: Type distribution + Top users
    col1, col2 = st.columns([1, 1])

    with col1:
        st.subheader("Humor Types")
        type_df = queries.get_humor_type_distribution(chat_id)
        if not type_df.empty:
            fig = create_type_donut(type_df, "humor_type", HUMOR_TYPE_COLORS, "")
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No humor type data available")

    with col2:
        st.subheader("Funniest Users")
        funny_users = queries.get_user_humor_rankings(chat_id, limit=10)
        if funny_users:
            fig = create_user_ranking_bar(funny_users, "humor_rate", "#f59e0b", "Humor")
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No user humor data available")

    st.divider()

    # Timeline
    st.subheader("Humor Over Time")
    timeline_df = queries.get_humor_timeline(chat_id)
    if not timeline_df.empty:
        fig = create_content_timeline(
            timeline_df, "humorous_count", "humor_rate", "#f59e0b", "Humorous"
        )
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No humor timeline data available")

    st.divider()

    # Top humorous messages
    st.subheader("Top Humorous Messages")
    top_messages = queries.get_top_humorous_messages(chat_id, limit=15)
    if top_messages:
        messages_df = pd.DataFrame(top_messages)
        messages_df["date"] = pd.to_datetime(messages_df["date"]).dt.strftime(
            "%Y-%m-%d %H:%M"
        )
        messages_df["user"] = messages_df.apply(
            lambda r: f"{r['first_name']} (@{r['username']})"
            if r.get("username")
            else r["first_name"],
            axis=1,
        )
        messages_df["text"] = messages_df["text"].apply(
            lambda x: x[:150] + "..." if len(x) > 150 else x
        )
        messages_df["score"] = (messages_df["score"] * 100).round(1)

        display_df = messages_df[["date", "user", "text", "humor_type", "score"]].copy()
        display_df.columns = ["Date", "User", "Message", "Type", "Score %"]

        st.dataframe(
            display_df,
            use_container_width=True,
            hide_index=True,
            column_config={"Message": st.column_config.TextColumn(width="large")},
        )
    else:
        st.info("No humorous messages found")


def _render_questions_section(queries: MLDashboardQueries, chat_id: int) -> None:
    """Render the questions analysis section."""
    # Key metrics
    question_stats = queries.get_question_stats(chat_id)

    col1, col2, col3, col4 = st.columns(4)
    with col1:
        st.metric("Messages Analyzed", f"{question_stats.get('total_analyzed', 0):,}")
    with col2:
        st.metric("Questions Found", f"{question_stats.get('question_count', 0):,}")
    with col3:
        st.metric("Question Rate", f"{question_stats.get('question_rate', 0):.1f}%")
    with col4:
        avg_score = question_stats.get("avg_score") or 0
        st.metric("Avg Confidence", f"{avg_score:.2f}")

    st.divider()

    # Two columns: Type distribution + Top users
    col1, col2 = st.columns([1, 1])

    with col1:
        st.subheader("Question Types")
        type_df = queries.get_question_type_distribution(chat_id)
        if not type_df.empty:
            fig = create_type_donut(
                type_df, "question_type", QUESTION_TYPE_COLORS, ""
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No question type data available")

    with col2:
        st.subheader("Most Curious Users")
        curious_users = queries.get_user_question_rankings(chat_id, limit=10)
        if curious_users:
            fig = create_user_ranking_bar(
                curious_users, "question_rate", "#3b82f6", "Questions"
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No user question data available")

    st.divider()

    # Timeline
    st.subheader("Questions Over Time")
    timeline_df = queries.get_question_timeline(chat_id)
    if not timeline_df.empty:
        fig = create_content_timeline(
            timeline_df, "question_count", "question_rate", "#3b82f6", "Questions"
        )
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No question timeline data available")
