"""
User Cards page - Weekly aggregated user statistics viewer.

Shows:
- Week selector
- User card gallery for selected week
- Individual user card detail view
- User stat history over time
- Weekly leaderboards
"""

import streamlit as st
import pandas as pd

from src.charts.user_cards import (
    create_user_card_radar,
    create_stat_trend_line,
    create_weekly_leaderboard_bar,
    create_multi_stat_comparison,
    STAT_LABELS,
)
from src.database.queries import MLDashboardQueries


def render_user_cards(queries: MLDashboardQueries, chat_id: int) -> None:
    """
    Render the user cards page.

    Args:
        queries: Database query object
        chat_id: Selected chat ID
    """
    st.header("User Cards")
    st.caption("Weekly user behavior summaries")

    # Get available weeks
    weeks = queries.get_user_cards_weeks(chat_id)

    if not weeks:
        st.warning(
            "No user cards data available. Run the ML processor to generate weekly cards."
        )
        return

    # Week selector
    week_options = {
        f"{w['week_start']} to {w['week_end']} ({w['user_count']} users)": w[
            "week_start"
        ]
        for w in weeks
    }
    selected_week_label = st.selectbox(
        "Select Week",
        options=list(week_options.keys()),
        index=0,
    )
    selected_week = week_options[selected_week_label]

    st.divider()

    # Get all cards for this week
    weekly_cards = queries.get_weekly_user_cards(chat_id, selected_week)

    if not weekly_cards:
        st.warning("No user cards for this week")
        return

    # View mode selector
    view_mode = st.radio(
        "View Mode",
        options=["Gallery", "Leaderboards", "Compare Users"],
        horizontal=True,
        label_visibility="collapsed",
    )

    st.divider()

    if view_mode == "Gallery":
        _render_gallery_view(queries, chat_id, weekly_cards, selected_week)
    elif view_mode == "Leaderboards":
        _render_leaderboard_view(queries, chat_id, selected_week)
    else:
        _render_comparison_view(weekly_cards)


def _render_gallery_view(
    queries: MLDashboardQueries,
    chat_id: int,
    cards: list[dict],
    week_start,
) -> None:
    """Render the gallery view of user cards."""
    # User selector
    user_options = {
        f"{c['first_name']} (@{c['username']})"
        if c.get("username")
        else c["first_name"]: c
        for c in cards
    }
    selected_user_label = st.selectbox(
        "Select User",
        options=list(user_options.keys()),
        index=0,
    )
    selected_card = user_options[selected_user_label]

    st.divider()

    # Display the card
    col1, col2 = st.columns([1, 1])

    with col1:
        st.subheader(selected_user_label)
        st.caption(f"Messages analyzed: {selected_card.get('messages_analyzed', 0):,}")

        # Radar chart
        stats = selected_card.get("stats", {})
        if stats:
            fig = create_user_card_radar(stats, selected_user_label.split()[0])
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No stats available")

    with col2:
        # Stats breakdown
        st.subheader("Stats Breakdown")
        stats = selected_card.get("stats", {})
        if stats:
            for key, value in stats.items():
                if key in STAT_LABELS:
                    label = STAT_LABELS.get(key, key)
                    st.write(f"**{label}**")
                    st.progress(min(float(value), 1.0), text=f"{float(value):.2f}")
        else:
            st.info("No stats breakdown available")

    st.divider()

    # User history
    st.subheader("Stat History")
    user_id = selected_card.get("user_id")
    if user_id:
        history = queries.get_user_card_history(user_id, chat_id, limit=12)
        if history and len(history) > 1:
            # Stat selector for trend
            stat_options = list(STAT_LABELS.keys())
            cols = st.columns(4)
            for i, stat_key in enumerate(stat_options[:4]):
                with cols[i]:
                    fig = create_stat_trend_line(history, stat_key)
                    st.plotly_chart(fig, use_container_width=True)

            # Second row
            if len(stat_options) > 4:
                cols = st.columns(4)
                for i, stat_key in enumerate(stat_options[4:8]):
                    with cols[i]:
                        fig = create_stat_trend_line(history, stat_key)
                        st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("Not enough history data for trends")


def _render_leaderboard_view(
    queries: MLDashboardQueries,
    chat_id: int,
    week_start,
) -> None:
    """Render the leaderboard view."""
    st.subheader("Weekly Leaderboards")

    # Create columns for leaderboards
    col1, col2 = st.columns(2)

    stat_keys = list(STAT_LABELS.keys())
    half = len(stat_keys) // 2

    with col1:
        for stat_key in stat_keys[:half]:
            with st.expander(f"{STAT_LABELS[stat_key]} Leaders", expanded=True):
                leaders = queries.get_weekly_leaderboard(chat_id, week_start, stat_key)
                if leaders:
                    fig = create_weekly_leaderboard_bar(leaders, stat_key)
                    st.plotly_chart(fig, use_container_width=True)
                else:
                    st.info("No data")

    with col2:
        for stat_key in stat_keys[half:]:
            with st.expander(f"{STAT_LABELS[stat_key]} Leaders", expanded=True):
                leaders = queries.get_weekly_leaderboard(chat_id, week_start, stat_key)
                if leaders:
                    fig = create_weekly_leaderboard_bar(leaders, stat_key)
                    st.plotly_chart(fig, use_container_width=True)
                else:
                    st.info("No data")


def _render_comparison_view(cards: list[dict]) -> None:
    """Render the user comparison view."""
    st.subheader("Compare Users")

    # Multi-select users
    user_options = {
        f"{c['first_name']} (@{c['username']})"
        if c.get("username")
        else c["first_name"]: c
        for c in cards
    }

    selected_users = st.multiselect(
        "Select users to compare",
        options=list(user_options.keys()),
        default=list(user_options.keys())[:5],
        max_selections=10,
    )

    if len(selected_users) < 2:
        st.info("Select at least 2 users to compare")
        return

    # Stat selector
    stat_keys = st.multiselect(
        "Select stats to compare",
        options=list(STAT_LABELS.keys()),
        default=["mood", "toxicity", "activity", "humor"],
        format_func=lambda x: STAT_LABELS.get(x, x),
    )

    if not stat_keys:
        st.info("Select at least one stat to compare")
        return

    # Get selected cards
    selected_cards = [user_options[name] for name in selected_users]

    # Create comparison chart
    fig = create_multi_stat_comparison(selected_cards, stat_keys)
    st.plotly_chart(fig, use_container_width=True)

    # Also show as table
    st.divider()
    st.subheader("Comparison Table")

    table_data = []
    for card in selected_cards:
        row = {"User": card["first_name"]}
        stats = card.get("stats", {})
        for key in stat_keys:
            row[STAT_LABELS.get(key, key)] = f"{float(stats.get(key, 0)):.2f}"
        table_data.append(row)

    st.dataframe(pd.DataFrame(table_data), use_container_width=True, hide_index=True)
