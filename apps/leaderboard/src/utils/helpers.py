"""Helper functions for role-based rendering and data filtering."""


def admin_only(component, user: dict):
    """
    Render component only if user is admin.

    Args:
        component: Dash component to render
        user: Session dict with 'is_admin' field

    Returns:
        Component if admin, None otherwise
    """
    return component if user.get("is_admin") else None


def filter_chats_for_user(chats: list[dict], user: dict) -> list[dict]:
    """
    Filter chats based on user permissions.

    Admins see all chats.
    Regular users see only chats in their allowed_chat_ids.

    Args:
        chats: List of chat dicts from get_chats_with_stats()
        user: Session dict with 'is_admin' and 'allowed_chat_ids'

    Returns:
        Filtered list of chats
    """
    if user.get("is_admin"):
        return chats

    allowed_ids = set(user.get("allowed_chat_ids", []))
    return [chat for chat in chats if chat["id"] in allowed_ids]
