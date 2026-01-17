/**
 * Type definitions for deck-mini-app.
 */

export type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

export interface CardImage {
  id: number
  user_id: number
  chat_id: number
  week_start: string
  storage_path: string
  generated_at: string
  first_name: string | null
  last_name: string | null
  username: string | null
}

export interface CardImageWithUrl extends CardImage {
  url: string
}
