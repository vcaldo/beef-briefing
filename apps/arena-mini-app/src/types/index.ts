// Re-export auth types from shared
export type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

// ============================================================================
// Game Phase & Match Types
// ============================================================================

/** Main navigation tabs */
export type GamePhase = 'lobby' | 'shop' | 'battle' | 'stats'

/** Match type (ranked tournaments vs regular casual matches) */
export type MatchType = 'ranked' | 'regular'

/** Match format (1v1 or larger arena) */
export type MatchFormat = '1v1' | 'arena'

/** Match lifecycle status */
export type MatchStatus = 'open' | 'shop_phase' | 'battle_phase' | 'completed' | 'cancelled'

/** Participant status within a match */
export type ParticipantStatus = 'joined' | 'ready' | 'eliminated' | 'winner'

// ============================================================================
// Card Types
// ============================================================================

/** Base card stats from user cards */
export interface Card {
  card_id: number
  user_id: number
  name: string
  username?: string
  photo_url?: string
  atk: number
  def: number
  hp: number
  max_hp: number
  atk_upgrades: number
  hp_upgrades: number
  position?: number
}

/** Card available for purchase in shop */
export interface ShopCard {
  card_id: number
  user_id: number
  name: string
  username?: string
  photo_url?: string
  card_image_url?: string
  atk: number
  def: number
  hp: number
  is_purchased: boolean
  index: number
}

/** Enhanced shop card with affordability info */
export interface EnhancedShopCard extends ShopCard {
  can_buy: boolean
  buy_disabled_reason?: string
}

/** Enhanced team card with upgrade preview */
export interface EnhancedTeamCard extends Card {
  can_upgrade_atk: boolean
  can_upgrade_hp: boolean
  upgrade_atk_disabled_reason?: string
  upgrade_hp_disabled_reason?: string
  atk_if_upgraded: number
  hp_if_upgraded: number
  max_hp_if_upgraded: number
}

// ============================================================================
// Participant Types
// ============================================================================

/** Participant info without user details */
export interface Participant {
  id: number
  match_id: string
  user_id: number
  status: ParticipantStatus
  joined_at: string
  coins_remaining: number
  team_order?: number[]
  placement?: number
  wins: number
  losses: number
  total_damage_dealt: number
}

/** Participant with user display info */
export interface ParticipantWithUser extends Participant {
  first_name: string
  username?: string
}

// ============================================================================
// Match Types
// ============================================================================

/** Full match details */
export interface Match {
  id: string
  chat_id: number
  match_type: MatchType
  format?: MatchFormat
  status: MatchStatus
  created_at: string
  join_deadline?: string
  shop_phase_started_at?: string
  shop_phase_deadline?: string
  battle_started_at?: string
  completed_at?: string
  tournament_date?: string
  creator_user_id?: number
  current_round: number
  winner_user_id?: number
}

/** Match response with participants */
export interface MatchResponse extends Match {
  participants: ParticipantWithUser[]
  card_count: number
}

// ============================================================================
// Shop Types
// ============================================================================

/** What actions a player can afford */
export interface ShopAffordability {
  can_buy: boolean
  can_reroll: boolean
  can_upgrade: boolean
  can_submit: boolean
  buy_disabled_reason?: string
  reroll_disabled_reason?: string
  upgrade_disabled_reason?: string
  submit_disabled_reason?: string
}

/** Full shop state response */
export interface ShopResponse {
  match_id: string
  status: string
  coins: number
  cards: EnhancedShopCard[]
  team: EnhancedTeamCard[]
  team_order: number[]
  is_ready: boolean
  team_submitted: boolean
  deadline?: string
  time_remaining_seconds: number
  affordability: ShopAffordability
}

// ============================================================================
// Battle Types
// ============================================================================

/** Battle event types */
export type BattleEventType = 'attack' | 'damage' | 'death' | 'advance' | 'victory' | 'summary'

/** Snapshot of a card's state at a point in battle */
export interface CardSnapshot {
  card_id: number
  user_id: number
  name: string
  hp: number
  max_hp: number
  atk: number
  position: number
  is_alive: boolean
  is_attacking: boolean
  is_defending: boolean
}

/** Single battle event for replay */
export interface BattleEvent {
  type: BattleEventType
  round: number
  message?: string
  attacker_card_id?: number
  defender_card_id?: number
  attacker_team_owner_id?: number
  defender_team_owner_id?: number
  damage?: number
  hp_before?: number
  hp_after?: number
  is_summary?: boolean
  killer_card_id?: number
  total_damage_dealt?: number
  total_damage_taken?: number
  rounds_in_duel?: number
  kill_streak?: number
  card_states?: CardSnapshot[]
}

/** Battle round result */
export interface MatchRound {
  id: number
  match_id: string
  round_number: number
  player_a_id: number
  player_b_id: number
  player_a_team: Card[]
  player_b_team: Card[]
  winner_id?: number
  is_draw: boolean
  battle_log: BattleEvent[]
  player_a_damage: number
  player_b_damage: number
  total_rounds: number
  created_at: string
}

/** Battle state response */
export interface BattleResponse {
  match_id: string
  status: string
  rounds: MatchRound[]
  winner_id?: number
  is_complete: boolean
}

// ============================================================================
// Stats & Leaderboard Types
// ============================================================================

/** Leaderboard entry */
export interface LeaderboardEntry {
  user_id: number
  first_name: string
  username?: string
  rank: number
  wins: number
  losses: number
  matches_played: number
  tournaments_won?: number
  current_streak: number
  best_streak: number
}

/** Leaderboard response */
export interface LeaderboardResponse {
  entries: LeaderboardEntry[]
  type: 'ranked' | 'regular'
}

/** User profile stats */
export interface ProfileStats {
  user_id: number
  first_name: string
  username?: string
  ranked_wins: number
  ranked_losses: number
  ranked_tournaments_played: number
  ranked_tournaments_won: number
  ranked_current_streak: number
  ranked_best_streak: number
  ranked_rank: number
  regular_wins: number
  regular_losses: number
  regular_matches_played: number
  regular_current_streak: number
  regular_best_streak: number
  regular_rank: number
  first_match_at?: string
  last_match_at?: string
}

/** Opponent info for H2H */
export interface OpponentInfo {
  user_id: number
  first_name: string
  username?: string
  photo_url?: string
}

/** Match history entry */
export interface MatchHistoryEntry {
  match_id: string
  match_type: string
  your_photo_url?: string
  opponent: OpponentInfo
  result: 'win' | 'loss' | 'draw'
  your_team: Card[]
  opponent_team: Card[]
  completed_at: string
}

/** Match history response */
export interface MatchHistoryResponse {
  matches: MatchHistoryEntry[]
  total: number
  has_more: boolean
}

/** Profile response */
export interface ProfileResponse {
  profile: ProfileStats | null
  recent_matches: MatchHistoryEntry[]
}

/** H2H record */
export interface H2HRecord {
  opponent: OpponentInfo
  wins: number
  losses: number
  last_match_at?: string
}

/** H2H response */
export interface H2HResponse {
  record: H2HRecord | null
  recent_matches: MatchHistoryEntry[]
}

// ============================================================================
// Constants & Configuration
// ============================================================================

/** Game economy and timing constants */
export interface GameConstants {
  costs: {
    card: number
    reroll: number
    upgrade: number
  }
  sizes: {
    shop: number
    team: number
  }
  upgrades: {
    atk_amount: number
    hp_amount: number
  }
  timings: {
    shop_phase_duration: number
    join_window_duration: number
  }
}

// ============================================================================
// API Response Wrappers
// ============================================================================

/** Generic list matches response */
export interface MatchesListResponse {
  matches: MatchResponse[]
}

/** Share result response */
export interface ShareResultResponse {
  success: boolean
  match_id: string
  chat_id: number
}

/** Generic success response */
export interface SuccessResponse {
  ok: boolean
}
