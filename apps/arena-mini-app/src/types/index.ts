// Re-export auth types from shared
export type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

/**
 * Arena Mini App Types
 *
 * These types will be fully defined in a subsequent task.
 * For now, placeholder types for the basic structure.
 */

// Game phases
export type GamePhase = 'lobby' | 'shop' | 'battle' | 'stats'

// Match types
export type MatchType = 'casual' | 'ranked'

// Match status
export type MatchStatus = 'waiting' | 'ready' | 'in_progress' | 'completed'

// Basic match structure (to be expanded)
export interface Match {
  id: string
  type: MatchType
  status: MatchStatus
  created_at: string
  participants: Participant[]
}

export interface Participant {
  user_id: number
  username: string
  display_name: string
}

// Card structure (to be expanded)
export interface Card {
  id: string
  user_id: number
  name: string
  image_url: string
  compact_image_url?: string
  base_atk: number
  base_hp: number
  current_atk?: number
  current_hp?: number
  upgrades?: number
}

// Shop state (to be expanded)
export interface ShopState {
  coins: number
  shop_cards: Card[]
  team_cards: Card[]
  can_reroll: boolean
  has_submitted: boolean
}

// Battle state (to be expanded)
export interface BattleState {
  match_id: string
  player_team: Card[]
  opponent_team: Card[]
  events: BattleEvent[]
  winner_id?: number
}

export interface BattleEvent {
  type: 'attack' | 'death' | 'summary' | 'victory'
  attacker_id?: string
  defender_id?: string
  damage?: number
  message?: string
}
