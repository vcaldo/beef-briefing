/**
 * Shared authentication types for all Telegram Mini Apps.
 * These types are returned by the /api/v1/mini-app/auth endpoint.
 */

export interface AuthResponse {
  token: string;
  user_id: number;
  chat_id: number | null;
  chat_title?: string | null;
  first_name: string;
  username: string | null;
  is_admin?: boolean;
  chat_timezone?: string | null;
}

export interface User {
  user_id: number;
  first_name: string;
  last_name?: string | null;
  username?: string | null;
  photo_url?: string | null;
}
