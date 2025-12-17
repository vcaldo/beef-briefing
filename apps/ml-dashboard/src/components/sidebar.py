"""
Sidebar navigation and selection components.
"""

import streamlit as st

from src.database.queries import MLDashboardQueries


def render_sidebar(queries: MLDashboardQueries) -> tuple[int | None, int | None, str]:
    """
    Render the sidebar with chat and user selection.

    Args:
        queries: Database query object

    Returns:
        Tuple of (selected_chat_id, selected_user_id, selected_page)
    """
    st.sidebar.title("ML Analytics")

    # Page selection
    page = st.sidebar.radio(
        "View",
        ["Group Overview", "User Analysis", "Embedding Explorer"],
        label_visibility="collapsed",
    )

    st.sidebar.divider()

    # Chat selection
    chats = queries.get_available_chats()

    if not chats:
        st.sidebar.warning("No chats with ML data found")
        return None, None, page

    chat_options = {c["title"]: c["id"] for c in chats}
    selected_chat_title = st.sidebar.selectbox(
        "Select Group",
        options=list(chat_options.keys()),
        index=0,
    )
    chat_id = chat_options[selected_chat_title]

    # Show chat stats
    selected_chat = next((c for c in chats if c["id"] == chat_id), None)
    if selected_chat:
        st.sidebar.caption(
            f"Analyzed: {selected_chat['analyzed_count']:,} / "
            f"{selected_chat['message_count']:,} messages"
        )

    st.sidebar.divider()

    # User selection (for User Analysis page)
    user_id = None
    if page == "User Analysis":
        users = queries.get_group_users(chat_id)
        if users:
            user_options = {
                f"{u['first_name']} (@{u['username']})"
                if u.get("username")
                else u["first_name"]: u["user_id"]
                for u in users
            }
            selected_user_name = st.sidebar.selectbox(
                "Select User",
                options=list(user_options.keys()),
                index=0,
            )
            user_id = user_options[selected_user_name]
        else:
            st.sidebar.warning("No users with ML data in this group")

    return chat_id, user_id, page
