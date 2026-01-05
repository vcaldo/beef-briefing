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

export type TabId = 'home' | 'leaderboard' | 'interactions' | 'profile'

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
  active_days: number
  current_streak: number
  avg_messages_per_day: number
  rank_by_messages: number
  rank_by_reactions_received: number
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
