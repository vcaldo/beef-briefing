/**
 * Types for the Leaderboard Mini App.
 */

export interface AuthResponse {
  token: string
  user_id: number
  chat_id: number | null
  chat_title: string | null
  first_name: string
  username: string | null
  is_admin: boolean
}

export interface StatsResponse {
  total_messages: number
  total_users: number
  total_reactions: number
  total_media: number
  messages_per_day: number
  // Trend fields (percentage change vs previous period, null if unavailable)
  total_messages_trend?: number | null
  total_users_trend?: number | null
  total_reactions_trend?: number | null
  total_media_trend?: number | null
  messages_per_day_trend?: number | null
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
  photo_url?: string | null
}

export interface LeaderboardResponse {
  users: LeaderboardUser[]
  total: number
  page: number
  limit: number
}

export type Period = '24h' | '7d' | '30d' | '90d' | '180d' | '365d' | 'ytd' | 'max'

export type LeaderboardMetric =
  | 'message_count'
  | 'reactions_sent'
  | 'reactions_received'
  | 'replies_sent'
  | 'replies_received'
  | 'active_days'
  | 'current_streak'

export type TabId = 'home' | 'leaderboard' | 'interactions' | 'profile' | 'card'

// Card Gallery Types
export interface CardImage {
  id: number
  user_id: number
  chat_id: number
  week_start: string
  storage_path: string
  theme: string
  generated_at: string
  first_name: string | null
  last_name: string | null
  username: string | null
}

export interface CardImageWithUrl extends CardImage {
  url: string
}

// Reactions Overview Types
export interface TopReaction {
  emoji: string
  reaction_type: 'emoji' | 'custom_emoji' | 'paid'
  count: number
}

export interface ReactionUser {
  rank: number
  user_id: number
  first_name: string
  last_name: string | null
  username: string | null
  score: number
  photo_url?: string | null
}

export interface ReactionsOverviewResponse {
  top_reactions: TopReaction[]
  top_givers: ReactionUser[]
  top_receivers: ReactionUser[]
}

// Replies Overview Types
export interface RepliesOverviewResponse {
  top_senders: ReactionUser[]
  top_receivers: ReactionUser[]
}

// Profile Types
export interface ProfileStats {
  message_count: number
  reactions_sent: number
  reactions_received: number
  replies_sent: number
  replies_received: number
  active_days: number
  current_streak: number
  avg_messages_per_day: number
  rank_by_messages: number
  rank_by_reactions_received: number
  // Trend fields (percentage change vs previous period, null if unavailable)
  message_count_trend?: number | null
  reactions_sent_trend?: number | null
  reactions_received_trend?: number | null
  replies_sent_trend?: number | null
  replies_received_trend?: number | null
  active_days_trend?: number | null
  avg_messages_per_day_trend?: number | null
}

export interface TopInteractor {
  rank: number
  user_id: number
  first_name: string
  last_name: string | null
  username: string | null
  score: number
  top_emoji?: string
  photo_url?: string | null
}

export interface ProfileResponse {
  photo_url?: string | null
  stats: ProfileStats
  top_reactors: TopInteractor[]
  top_reacted_to: TopInteractor[]
  top_repliers: TopInteractor[]
  top_replied_to: TopInteractor[]
  heatmap: HeatmapData
  activity?: ActivityDataPoint[]
  // Fields populated when admin is impersonating another user
  user_id?: number
  first_name?: string
  last_name?: string | null
  username?: string | null
}

// Admin user list types
export interface ChatUser {
  user_id: number
  first_name: string
  last_name: string | null
  username: string | null
  photo_url?: string | null
}

export interface ChatUsersResponse {
  users: ChatUser[]
}

// Heatmap Types
export interface HeatmapCell {
  day_of_week: number // 0-6 (Sunday=0)
  hour: number // 0-23
  message_count: number
  unique_users?: number
}

export interface HeatmapData {
  data: HeatmapCell[]
  max_count: number
  total_messages: number
}

export interface HeatmapResponse {
  group: HeatmapData
  user?: HeatmapData
}
