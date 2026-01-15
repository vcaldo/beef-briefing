/**
 * Type definitions for arena-mini-app.
 * These types mirror the backend Go structs from the Arena API.
 */

// Re-export shared auth types
export type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

// =============================================================================
// ENUMS
// =============================================================================

/** Match status enum matching backend MatchStatus */
export type MatchStatus =
  | 'open'
  | 'shop_phase'
  | 'battle_phase'
  | 'completed'
  | 'cancelled'

/** Match type enum */
export type MatchType = 'ranked' | 'regular'

/** Match format enum */
export type MatchFormat = '1v1' | 'arena'

/** Participant status enum */
export type ParticipantStatus = 'joined' | 'ready' | 'eliminated' | 'winner'

/** Battle event type enum */
export type EventType =
  | 'attack'
  | 'damage'
  | 'death'
  | 'advance'
  | 'victory'
  | 'summary'

// =============================================================================
// MATCH & PARTICIPANT TYPES
// =============================================================================

/** Participant info with user details */
export interface Participant {
  id: number
  match_id: string
  user_id: number
  status: ParticipantStatus
  joined_at: string
  coins_remaining: number
  shop_cards?: ShopCard[] | null
  team?: Card[] | null
  team_order?: number[]
  team_submitted_at?: string | null
  placement?: number | null
  wins: number
  losses: number
  total_damage_dealt: number
  // User info from join
  first_name: string
  username?: string
}

/** Match data with participants */
export interface Match {
  id: string
  chat_id: number
  match_type: MatchType
  format?: MatchFormat | null
  status: MatchStatus
  created_at: string
  join_deadline?: string | null
  shop_phase_started_at?: string | null
  shop_phase_deadline?: string | null
  battle_started_at?: string | null
  completed_at?: string | null
  tournament_date?: string | null
  creator_user_id?: number | null
  current_round: number
  winner_user_id?: number | null
  // Extended fields from MatchResponse
  participants: Participant[]
  card_count: number
}

/** Response from getMatches endpoint */
export interface MatchesResponse {
  matches: Match[]
}

// =============================================================================
// CARD TYPES
// =============================================================================

/** Base card stats */
export interface CardStats {
  atk: number
  def: number
  hp: number
}

/** Shop card available for purchase */
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
  stats?: Record<string, unknown>
  is_purchased: boolean
  index: number
}

/** Card in team or battle with full stats */
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
  position: number
}

/** Team card with slot info */
export interface TeamCard extends Card {
  slot_index: number
}

/** Card with affordability flag for shop display */
export interface EnhancedShopCard extends ShopCard {
  can_afford: boolean
}

/** Team card with upgrade info and image URL for display */
export interface EnhancedTeamCard extends Card {
  card_image_url?: string
  placeholder_positions?: PlaceholderPositions
}

// =============================================================================
// SHOP TYPES
// =============================================================================

/** Shop state response from getShop endpoint */
export interface ShopResponse {
  match_id: string
  status: MatchStatus
  coins: number
  cards: ShopCard[]
  team: Card[]
  team_order: number[]
  is_ready: boolean
  deadline?: string | null
  time_remaining_seconds: number
}

/** Enhanced shop response with affordability and image URLs */
export interface EnhancedShopResponse {
  match_id: string
  status: MatchStatus
  coins: number
  cards: EnhancedShopCard[]
  team: EnhancedTeamCard[]
  team_order: number[]
  is_ready: boolean
  deadline?: string | null
  time_remaining_seconds: number
  can_reroll: boolean
}

// =============================================================================
// BATTLE TYPES
// =============================================================================

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
  type: EventType
  round: number
  message?: string
  // Card identifiers
  attacker_card_id?: number
  defender_card_id?: number
  attacker_team_owner_id?: number
  defender_team_owner_id?: number
  // Combat stats
  damage?: number
  hp_before?: number
  hp_after?: number
  // Summary event fields
  is_summary?: boolean
  killer_card_id?: number | null
  total_damage_dealt?: number
  total_damage_taken?: number
  rounds_in_duel?: number
  kill_streak?: number
  // Card states after this event
  card_states?: CardSnapshot[]
}

/** Team state in battle */
export interface BattleTeam {
  owner_id: number
  owner_name: string
  cards: Card[]
}

/** Battle result from getBattle endpoint */
export interface BattleResult {
  match_id: string
  winner_id?: number | null
  is_draw: boolean
  events: BattleEvent[]
  num_rounds: number
  team_a_damage: number
  team_b_damage: number
  team_a_final: BattleTeam
  team_b_final: BattleTeam
  // Additional match context
  player_a_id: number
  player_b_id: number
  player_a_name: string
  player_b_name: string
}

// =============================================================================
// LEADERBOARD & STATS TYPES
// =============================================================================

/** Leaderboard entry for a user */
export interface LeaderboardEntry {
  rank: number
  user_id: number
  first_name: string
  username?: string
  photo_url?: string
  // Ranked stats
  ranked_wins: number
  ranked_losses: number
  ranked_draws: number
  ranked_tournaments_played: number
  ranked_tournaments_won: number
  ranked_current_streak: number
  ranked_best_streak: number
  // Regular/casual stats
  regular_wins: number
  regular_losses: number
  regular_draws: number
  regular_matches_played: number
  regular_current_streak: number
  regular_best_streak: number
  // Computed fields
  score: number
  tier?: string
}

/** Leaderboard response with pagination */
export interface LeaderboardResponse {
  entries: LeaderboardEntry[]
  total: number
  page: number
  limit: number
  type: 'ranked' | 'casual'
}

/** Match history entry */
export interface MatchHistoryEntry {
  match_id: string
  match_type: MatchType
  opponent_id: number
  opponent_name: string
  opponent_username?: string
  result: 'win' | 'loss' | 'draw'
  damage_dealt: number
  damage_received: number
  rating_change?: number
  played_at: string
}

/** Match history response with pagination */
export interface HistoryResponse {
  matches: MatchHistoryEntry[]
  total: number
  page: number
  limit: number
}

/** Head-to-head record against an opponent */
export interface H2HRecord {
  wins: number
  losses: number
  draws: number
  total_matches: number
  win_rate: number
  last_played?: string
}

/** Head-to-head response */
export interface H2HResponse {
  opponent_id: number
  opponent_name: string
  opponent_username?: string
  record: H2HRecord
}

/** User profile stats */
export interface ProfileStats {
  // Ranked stats
  ranked_wins: number
  ranked_losses: number
  ranked_draws: number
  ranked_matches_played: number
  ranked_win_rate: number
  ranked_current_streak: number
  ranked_best_streak: number
  ranked_tournaments_played: number
  ranked_tournaments_won: number
  // Regular/casual stats
  regular_wins: number
  regular_losses: number
  regular_draws: number
  regular_matches_played: number
  regular_win_rate: number
  regular_current_streak: number
  regular_best_streak: number
  // Overall
  total_matches: number
  total_wins: number
  total_damage_dealt: number
  tier?: string
  rank?: number
}

/** User profile response */
export interface ProfileResponse {
  user_id: number
  first_name: string
  username?: string
  photo_url?: string
  stats: ProfileStats
}

// =============================================================================
// GAME CONSTANTS
// =============================================================================

/** Game constants from getConstants endpoint */
export interface GameConstants {
  starting_coins: number
  card_cost: number
  reroll_cost: number
  upgrade_cost: number
  upgrade_amount: number
  team_size: number
  shop_size: number
  join_window_seconds: number
  shop_phase_seconds: number
  minimum_cards_required: number
}

// =============================================================================
// CARD IMAGE & COMPACT CARD TYPES
// =============================================================================

/** Card image from card-renderer API */
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

/** Card image with presigned URL */
export interface CardImageResponse extends CardImage {
  url: string
  placeholder_positions?: PlaceholderPositions
}

/** Position metadata for a single stat overlay */
export interface StatPosition {
  x: number
  y: number
  width: number
  height: number
  anchor?: string
  font_size: number
  font_weight?: string
  color: string
  text_align?: string
  container_x?: number
  container_y?: number
  container_width?: number
  container_height?: number
  note?: string
  text_shadow?: string
}

/** Combat stats positions */
export interface CombatStatsPositions {
  atk: StatPosition
  def: StatPosition
  hp: StatPosition
}

/** HP bar container position */
export interface HpBarContainerPosition {
  x: number
  y: number
  width: number
  height: number
  border_radius: number
  note?: string
}

/** HP bar fill position */
export interface HpBarFillPosition {
  x: number
  y: number
  max_width: number
  height: number
  border_radius: number
  color: string
  background_color: string
  note?: string
}

/** HP bar positions */
export interface HpBarPositions {
  container: HpBarContainerPosition
  fill: HpBarFillPosition
}

/** Placeholders structure */
export interface Placeholders {
  combat_stats: CombatStatsPositions
  hp_bar: HpBarPositions
}

/** Card dimensions metadata */
export interface CardDimensions {
  width: number
  height: number
  scale: number
  unit: string
  note?: string
}

/** Tier-specific style variations */
export interface TierVariation {
  combat_stats?: {
    atk?: Partial<StatPosition>
    def?: Partial<StatPosition>
    hp?: Partial<StatPosition>
  }
}

/** Complete placeholder positions metadata for compact cards */
export interface PlaceholderPositions {
  version: string
  card_dimensions: CardDimensions
  placeholders: Placeholders
  tier_variations?: Record<string, TierVariation>
  usage_notes?: Record<string, string>
}

// =============================================================================
// UI TYPES
// =============================================================================

/** Tab identifiers for main navigation */
export type TabId = 'lobby' | 'shop' | 'battle' | 'stats'

/** Stats sub-tab identifiers */
export type StatsSubTab = 'leaderboard' | 'profile' | 'history' | 'h2h'

/** App state enum */
export type AppState = 'loading' | 'authenticated' | 'error'

/** Upgrade type for card upgrades */
export type UpgradeType = 'atk' | 'hp'
