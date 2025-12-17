"""
Group Overview page - main dashboard view.

Shows:
- Key metrics (messages analyzed, sentiment breakdown, toxicity rate)
- User behavior quadrant (sentiment x toxicity scatter)
- Top toxic users
- Toxicity timeline
"""

import streamlit as st

from src.charts.quadrant import create_behavior_quadrant
from src.charts.toxicity import create_toxicity_timeline, create_user_toxicity_bar
from src.components.metrics import render_metric_cards
from src.database.queries import MLDashboardQueries


def render_group_overview(queries: MLDashboardQueries, chat_id: int) -> None:
    """
    Render the group overview page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
    """
    # Get chat info
    chat_info = queries.get_chat_info(chat_id)
    if chat_info:
        st.header(f"{chat_info['title']}")
    else:
        st.header("Group Overview")

    # Key metrics
    sentiment_stats = queries.get_sentiment_stats(chat_id)
    toxicity_stats = queries.get_toxicity_stats(chat_id)
    render_metric_cards(sentiment_stats, toxicity_stats)

    st.divider()

    # Main content in two columns
    col1, col2 = st.columns([2, 1])

    with col1:
        st.subheader("User Behavior Quadrant")
        st.caption(
            "Each dot is a user. Position shows sentiment (x) vs toxicity (y). "
            "Size = messages analyzed."
        )

        # Date range filter
        date_col1, date_col2 = st.columns(2)
        with date_col1:
            start_date = st.date_input("Start Date", value=None)
        with date_col2:
            end_date = st.date_input("End Date", value=None)

        # Get quadrant data with optional date filtering
        quadrant_df = queries.get_user_behavior_quadrant(chat_id, start_date, end_date)

        if not quadrant_df.empty:
            fig = create_behavior_quadrant(quadrant_df)
            st.plotly_chart(fig, use_container_width=True)

            # Show user table below
            with st.expander("User Details", expanded=False):
                display_df = quadrant_df[
                    ["first_name", "username", "avg_sentiment", "toxicity_rate", "messages_analyzed"]
                ].copy()
                display_df.columns = ["Name", "Username", "Sentiment", "Toxicity", "Messages"]
                display_df["Username"] = display_df["Username"].fillna("").apply(
                    lambda x: f"@{x}" if x else ""
                )
                display_df["Sentiment"] = display_df["Sentiment"].round(2)
                display_df["Toxicity"] = (display_df["Toxicity"] * 100).round(1).astype(str) + "%"
                st.dataframe(
                    display_df.sort_values("Messages", ascending=False),
                    use_container_width=True,
                    hide_index=True,
                )
        else:
            st.info("No user behavior data available. Run the ML processor first.")

    with col2:
        st.subheader("Most Toxic Users")

        toxic_users = queries.get_user_toxicity_rankings(chat_id, limit=10)
        if toxic_users:
            fig = create_user_toxicity_bar(toxic_users)
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No toxic messages detected")

    st.divider()

    # Toxicity timeline
    st.subheader("Toxicity Over Time")

    timeline_df = queries.get_toxicity_timeline(chat_id)
    if not timeline_df.empty:
        fig = create_toxicity_timeline(timeline_df)
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("No toxicity timeline data available")

    st.divider()

    # Top toxic messages
    st.subheader("Top Toxic Messages")

    toxic_messages = queries.get_top_toxic_messages(chat_id, limit=15)
    if toxic_messages:
        import pandas as pd

        messages_df = pd.DataFrame(toxic_messages)
        messages_df["date"] = pd.to_datetime(messages_df["date"]).dt.strftime(
            "%Y-%m-%d %H:%M"
        )
        # Format user display
        messages_df["user"] = messages_df.apply(
            lambda r: f"{r['first_name']} (@{r['username']})"
            if r.get("username")
            else r["first_name"],
            axis=1,
        )
        # Truncate long messages
        messages_df["text"] = messages_df["text"].apply(
            lambda x: x[:150] + "..." if len(x) > 150 else x
        )
        # Format toxicity score as percentage
        messages_df["score"] = (messages_df["toxicity_score"] * 100).round(1)

        display_df = messages_df[["date", "user", "text", "score"]].copy()
        display_df.columns = ["Date", "User", "Message", "Toxicity %"]

        st.dataframe(
            display_df,
            use_container_width=True,
            hide_index=True,
            column_config={
                "Message": st.column_config.TextColumn(width="large"),
            },
        )
    else:
        st.info("No toxic messages detected")
