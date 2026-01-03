/**
 * Types for the Leaderboard Mini App.
 */

export interface AuthResponse {
  token: string
  user_id: number
  chat_id: number | null
  first_name: string
  username: string | null
}

export interface StatsResponse {
  total_messages: number
  total_users: number
  total_reactions: number
  total_media: number
  messages_per_day: number
}

export interface ActivityDataPoint {
  date: string
  messages: number
  users: number
}

export interface ActivityResponse {
  data: ActivityDataPoint[]
}

export interface LeaderboardUser {
  rank: number
  user_id: number
  first_name: string
  last_name: string | null
  username: string | null
  score: number
}

export interface LeaderboardResponse {
  users: LeaderboardUser[]
  total: number
  page: number
  limit: number
}

export type Period = '24h' | '7d' | '30d' | '90d' | '180d' | '365d' | 'ytd' | 'max'

export type LeaderboardMetric = 'message_count' | 'reactions_sent' | 'reactions_received' | 'active_days'
