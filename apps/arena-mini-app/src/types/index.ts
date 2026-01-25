/**
 * @fileoverview Type definitions for the Arena Mini-App.
 *
 * This module contains all TypeScript interfaces and type aliases used throughout
 * the Arena Mini-App. Types are organized into categories:
 * - Enums: Status and type enumerations
 * - Match & Participant: Match lobby and player data
 * - Card: Card stats, shop cards, team cards
 * - Shop: Shop phase state and responses
 * - Battle: Battle events, snapshots, and results
 * - Leaderboard & Stats: Rankings, history, profiles
 * - Game Constants: Economy and timing values
 * - Card Image: Compact card rendering metadata
 * - UI: Navigation and state types
 *
 * These types mirror the backend Go structs from the Arena API for type safety.
 *
 * @module types
 * @see {@link https://github.com/your-repo/arena-api Arena API Documentation}
 */

// Re-export shared auth types
export type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

// Re-export animation types
export type {
  CardAnimationState,
  EventAnimationPhase,
  AnimationDurationKey,
} from './animation'
export { ANIMATION_DURATIONS } from './animation'

// =============================================================================
// ENUMS
// =============================================================================

/**
 * Match lifecycle status.
 *
 * Represents the current phase of an arena match:
 * - `open`: Match is accepting participants (lobby phase)
 * - `shop_phase`: Players are building teams from the shop
 * - `battle_phase`: Battle is in progress or complete
 * - `completed`: Match has finished with a result
 * - `cancelled`: Match was cancelled (not enough players, etc.)
 *
 * @example
 * if (match.status === 'shop_phase') {
 *   navigateToShop();
 * }
 */
export type MatchStatus =
  | 'open'
  | 'shop_phase'
  | 'battle_phase'
  | 'completed'
  | 'cancelled'

/**
 * Match type distinguishing ranked from casual matches.
 *
 * - `ranked`: Affects player rankings and tournament standings
 * - `regular`: Casual match with no ranking impact
 *
 * @example
 * const isRanked = match.match_type === 'ranked';
 */
export type MatchType = 'ranked' | 'regular'

/**
 * Match format defining the battle structure.
 *
 * - `1v1`: Direct head-to-head battle between two players
 * - `arena`: Multi-player arena format (future support)
 */
export type MatchFormat = '1v1' | 'arena'

/**
 * Participant status within a match.
 *
 * - `joined`: Player has joined but not submitted team
 * - `ready`: Player has submitted their team
 * - `eliminated`: Player was eliminated from the match
 * - `winner`: Player won the match
 */
export type ParticipantStatus = 'joined' | 'ready' | 'eliminated' | 'winner'

/**
 * Battle event type for replay playback.
 *
 * Each event represents a discrete action in the battle:
 * - `attack`: A card attacks another card
 * - `damage`: Damage is dealt (may be separate from attack)
 * - `death`: A card's HP reaches zero
 * - `advance`: A card advances to the next position
 * - `victory`: Match ends with a winner
 * - `summary`: Round or match summary information
 *
 * @example
 * switch (event.type) {
 *   case 'attack':
 *     highlightAttacker(event.attacker_card_id);
 *     break;
 *   case 'death':
 *     markCardAsDead(event.defender_card_id);
 *     break;
 * }
 */
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

/**
 * A participant in an arena match.
 *
 * Contains user identity, match progress, and team composition data.
 * Participants are created when users join a match and updated throughout
 * the match lifecycle.
 *
 * @example
 * // Check if participant is ready
 * const isReady = participant.status === 'ready';
 *
 * // Calculate remaining coins after purchases
 * const canAffordCard = participant.coins_remaining >= CARD_COST;
 *
 * @see {@link Match} Parent match containing participants
 * @see {@link ParticipantStatus} Possible status values
 */
export interface Participant {
  /** Unique participant record ID */
  id: number
  /** Match this participant belongs to */
  match_id: string
  /** Telegram user ID of the participant */
  user_id: number
  /** Current status in the match */
  status: ParticipantStatus
  /** ISO timestamp when participant joined */
  joined_at: string
  /** Remaining coins for purchases (starts at 10) */
  coins_remaining: number
  /** Cards available in participant's shop (shop phase only) */
  shop_cards?: ShopCard[] | null
  /** Cards in participant's team (max 3) */
  team?: Card[] | null
  /** Battle order indices for team cards */
  team_order?: number[]
  /** ISO timestamp when team was submitted */
  team_submitted_at?: string | null
  /** Final placement in match (1 = winner) */
  placement?: number | null
  /** Number of wins in this match */
  wins: number
  /** Number of losses in this match */
  losses: number
  /** Total damage dealt across all battles */
  total_damage_dealt: number
  /** User's display name from Telegram */
  first_name: string
  /** User's Telegram username (optional) */
  username?: string
  /** Presigned URL for user's profile photo */
  photo_url?: string
}

/**
 * An arena match instance.
 *
 * Represents a complete match from creation through completion, including
 * all participants and timing information. Matches progress through phases:
 * open → shop_phase → battle_phase → completed.
 *
 * @example
 * // Check if match can be started
 * const canStart = match.status === 'open' &&
 *                  match.participants.length >= 2 &&
 *                  match.creator_user_id === currentUserId;
 *
 * // Calculate time remaining to join
 * if (match.join_deadline) {
 *   const deadline = new Date(match.join_deadline);
 *   const remaining = deadline.getTime() - Date.now();
 * }
 *
 * @see {@link Participant} Players in this match
 * @see {@link MatchStatus} Possible status values
 */
export interface Match {
  /** Unique match identifier (UUID) */
  id: string
  /** Telegram chat ID where match was created */
  chat_id: number
  /** Whether match affects rankings */
  match_type: MatchType
  /** Battle format (1v1, arena) */
  format?: MatchFormat | null
  /** Current match phase */
  status: MatchStatus
  /** ISO timestamp when match was created */
  created_at: string
  /** ISO deadline for joining (open phase) */
  join_deadline?: string | null
  /** ISO timestamp when shop phase started */
  shop_phase_started_at?: string | null
  /** ISO deadline for shop phase */
  shop_phase_deadline?: string | null
  /** ISO timestamp when battle started */
  battle_started_at?: string | null
  /** ISO timestamp when match completed */
  completed_at?: string | null
  /** Date of ranked tournament (for daily tournaments) */
  tournament_date?: string | null
  /** User ID of match creator */
  creator_user_id?: number | null
  /** Current battle round number */
  current_round: number
  /** User ID of match winner (if completed) */
  winner_user_id?: number | null
  /** All participants in this match */
  participants: Participant[]
  /** Total number of cards in play */
  card_count: number
}

/**
 * Response from the getMatches API endpoint.
 *
 * Contains all active matches for a chat that the user can join or view.
 *
 * @see {@link Match} Individual match structure
 */
export interface MatchesResponse {
  /** List of active matches */
  matches: Match[]
}

// =============================================================================
// CARD TYPES
// =============================================================================

/**
 * Base combat statistics for a card.
 *
 * @example
 * const totalPower = stats.atk + stats.def + stats.hp;
 */
export interface CardStats {
  /** Attack power - damage dealt per hit */
  atk: number
  /** Defense value - damage reduction */
  def: number
  /** Hit points - health remaining */
  hp: number
}

/**
 * A card available for purchase in the shop.
 *
 * Shop cards represent user stats as game cards. Each card costs 3 coins
 * and can only be purchased once. Cards are generated from weekly user
 * activity metrics.
 *
 * @example
 * // Buy a card from the shop
 * if (!card.is_purchased && coins >= CARD_COST) {
 *   await apiClient.buyCard(matchId, card.index);
 * }
 *
 * @see {@link EnhancedShopCard} Extended version with affordability flag
 */
export interface ShopCard {
  /** Unique card identifier */
  card_id: number
  /** Telegram user ID this card represents */
  user_id: number
  /** Display name on the card */
  name: string
  /** Telegram username (optional) */
  username?: string
  /** User's profile photo URL */
  photo_url?: string
  /** Pre-rendered card image URL (400x600) - fallback */
  card_image_url?: string
  /** Compact card image URL (300x450) */
  compact_card_image_url?: string
  /** Metadata for stat overlay positioning */
  placeholder_positions?: PlaceholderPositions
  /** Attack stat value */
  atk: number
  /** Defense stat value */
  def: number
  /** Health points value */
  hp: number
  /** Additional stats metadata */
  stats?: Record<string, unknown>
  /** Whether this card has been purchased */
  is_purchased: boolean
  /** Position index in the shop (0-5) */
  index: number
}

/**
 * A card in a player's team or battle.
 *
 * Extends basic card data with battle-specific fields like max HP,
 * upgrade counts, and battle position. Used during shop phase for
 * team building and during battle for combat.
 *
 * @example
 * // Check if card can receive more upgrades
 * const canUpgradeHp = card.hp_upgrades < MAX_UPGRADES;
 *
 * // Calculate upgrade bonus
 * const totalHpBonus = card.hp_upgrades * UPGRADE_AMOUNT;
 *
 * @see {@link TeamCard} Version with slot index
 * @see {@link EnhancedTeamCard} Version with image data
 */
export interface Card {
  /** Unique card identifier */
  card_id: number
  /** Telegram user ID this card represents */
  user_id: number
  /** Display name on the card */
  name: string
  /** Telegram username (optional) */
  username?: string
  /** User's profile photo URL */
  photo_url?: string
  /** Current attack stat (base + upgrades) */
  atk: number
  /** Defense stat value */
  def: number
  /** Current hit points */
  hp: number
  /** Maximum HP (for HP bar percentage) */
  max_hp: number
  /** Number of ATK upgrades applied (+3 each) */
  atk_upgrades: number
  /** Number of HP upgrades applied (+3 each) */
  hp_upgrades: number
  /** Battle order position (0-2) */
  position: number
}

/**
 * A card in a team slot with slot index.
 *
 * @extends Card
 * @see {@link Card} Base card interface
 */
export interface TeamCard extends Card {
  /** Slot index in team display (0-2) */
  slot_index: number
}

/**
 * Shop card with affordability information.
 *
 * Used in the shop UI to show whether the player has enough coins
 * to purchase each card. The `can_afford` flag enables/disables
 * buy buttons.
 *
 * @extends ShopCard
 * @example
 * <button disabled={!card.can_afford}>Buy (3 coins)</button>
 */
export interface EnhancedShopCard extends ShopCard {
  /** Whether player has enough coins to buy */
  can_afford: boolean
}

/**
 * Team card with image URL and position metadata.
 *
 * Used for rendering compact cards (300x450) with stat overlays.
 * The `placeholder_positions` metadata enables precise positioning
 * of ATK/DEF/HP values and HP bar on the card image.
 *
 * @extends Card
 * @see {@link PlaceholderPositions} Position metadata structure
 * @see {@link CompactCard} Component that renders these cards
 */
export interface EnhancedTeamCard extends Card {
  /** Compact card image URL (300x450) */
  card_image_url?: string
  /** Metadata for stat overlay positioning */
  placeholder_positions?: PlaceholderPositions
}

// =============================================================================
// SHOP TYPES
// =============================================================================

/**
 * Shop state response from the getShop API endpoint.
 *
 * Contains all data needed to render the shop phase UI, including
 * available cards, current team, coins, and timing information.
 *
 * @see {@link EnhancedShopResponse} Version with affordability data
 */
export interface ShopResponse {
  /** Match this shop belongs to */
  match_id: string
  /** Current match status (should be 'shop_phase') */
  status: MatchStatus
  /** Player's remaining coins */
  coins: number
  /** Available cards in the shop */
  cards: ShopCard[]
  /** Player's current team */
  team: Card[]
  /** Battle order of team cards */
  team_order: number[]
  /** Whether team has been submitted */
  is_ready: boolean
  /** ISO deadline for shop phase */
  deadline?: string | null
  /** Seconds remaining in shop phase */
  time_remaining_seconds: number
}

/**
 * Enhanced shop response with UI-ready data.
 *
 * Extends ShopResponse with computed fields for affordability and
 * card image URLs. Used directly by ShopPage component.
 *
 * @example
 * // Check if player can still reroll
 * if (shopData.can_reroll && coins >= REROLL_COST) {
 *   showRerollButton();
 * }
 *
 * @see {@link ShopResponse} Base response structure
 */
export interface EnhancedShopResponse {
  /** Match this shop belongs to */
  match_id: string
  /** Current match status */
  status: MatchStatus
  /** Player's remaining coins */
  coins: number
  /** Shop cards with affordability flags */
  cards: EnhancedShopCard[]
  /** Team cards with image URLs */
  team: EnhancedTeamCard[]
  /** Battle order of team cards */
  team_order: number[]
  /** Whether team has been submitted */
  is_ready: boolean
  /** ISO deadline for shop phase */
  deadline?: string | null
  /** Seconds remaining in shop phase */
  time_remaining_seconds: number
  /** Whether reroll is available (disabled after first purchase) */
  can_reroll: boolean
}

// =============================================================================
// BATTLE TYPES
// =============================================================================

/**
 * Snapshot of a card's state at a specific point in battle.
 *
 * Used to track card state changes during battle replay. Each event
 * may include card_states showing all cards after the event resolves.
 *
 * @example
 * // Update card display based on snapshot
 * cardStates.forEach(snapshot => {
 *   setCardState(snapshot.card_id, {
 *     hp: snapshot.hp,
 *     isAlive: snapshot.is_alive,
 *     isAttacking: snapshot.is_attacking
 *   });
 * });
 *
 * @see {@link BattleEvent} Events that contain snapshots
 */
export interface CardSnapshot {
  /** Unique card identifier */
  card_id: number
  /** Telegram user ID this card represents */
  user_id: number
  /** Display name on the card */
  name: string
  /** Current HP at this point */
  hp: number
  /** Maximum HP (for percentage calc) */
  max_hp: number
  /** Current ATK value */
  atk: number
  /** Battle position (0-2) */
  position: number
  /** Whether card is still alive */
  is_alive: boolean
  /** Whether card is currently attacking */
  is_attacking: boolean
  /** Whether card is being attacked */
  is_defending: boolean
}

/**
 * A single event in the battle replay sequence.
 *
 * Events represent discrete actions during combat. The BattlePage
 * component plays through events sequentially, updating card states
 * and UI after each event.
 *
 * @example
 * // Process an attack event
 * if (event.type === 'attack') {
 *   highlightCards(event.attacker_card_id, event.defender_card_id);
 *   showDamageNumber(event.damage);
 *   updateHp(event.defender_card_id, event.hp_after);
 * }
 *
 * @see {@link EventType} Possible event types
 * @see {@link CardSnapshot} Card state after events
 */
export interface BattleEvent {
  /** Type of event (attack, death, victory, etc.) */
  type: EventType
  /** Round number when event occurred */
  round: number
  /** Human-readable event description */
  message?: string
  /** Card ID of the attacker (attack events) */
  attacker_card_id?: number
  /** Card ID of the defender (attack events) */
  defender_card_id?: number
  /** User ID who owns the attacking card */
  attacker_team_owner_id?: number
  /** User ID who owns the defending card */
  defender_team_owner_id?: number
  /** Damage dealt in this event */
  damage?: number
  /** Defender's HP before damage */
  hp_before?: number
  /** Defender's HP after damage */
  hp_after?: number
  /** Whether this is a summary event */
  is_summary?: boolean
  /** Card that dealt the killing blow (death events) */
  killer_card_id?: number | null
  /** Total damage dealt by this card (summary) */
  total_damage_dealt?: number
  /** Total damage taken by this card (summary) */
  total_damage_taken?: number
  /** Rounds this duel lasted (summary) */
  rounds_in_duel?: number
  /** Consecutive kills by this card */
  kill_streak?: number
  /** All card states after this event resolves */
  card_states?: CardSnapshot[]
}

/**
 * A single event within a combat.
 * Same as BattleEvent but without the `round` field since events are grouped by combat.
 *
 * @see {@link Combat} Parent combat containing events
 * @see {@link BattleEvent} Original event type with round field
 */
export interface CombatEvent {
  /** Type of event (attack, death, victory, etc.) */
  type: EventType
  /** Human-readable event description */
  message?: string
  /** Card ID of the attacker (attack events) */
  attacker_card_id?: number
  /** Card ID of the defender (attack events) */
  defender_card_id?: number
  /** User ID who owns the attacking card */
  attacker_team_owner_id?: number
  /** User ID who owns the defending card */
  defender_team_owner_id?: number
  /** Damage dealt in this event */
  damage?: number
  /** Defender's HP before damage */
  hp_before?: number
  /** Defender's HP after damage */
  hp_after?: number
  /** Whether this is a summary event */
  is_summary?: boolean
  /** Card that dealt the killing blow (death events) */
  killer_card_id?: number | null
  /** Total damage dealt by this card (summary) */
  total_damage_dealt?: number
  /** Total damage taken by this card (summary) */
  total_damage_taken?: number
  /** Rounds this duel lasted (summary) */
  rounds_in_duel?: number
  /** Consecutive kills by this card */
  kill_streak?: number
  /** All card states after this event resolves */
  card_states?: CardSnapshot[]
}

/**
 * Combat outcome indicating how a combat ended.
 */
export type CombatOutcome = 'card_a_died' | 'card_b_died' | 'both_died' | 'victory'

/**
 * A combat represents a duel between two cards until one or both die.
 * A battle consists of multiple combats as winning cards advance to fight next opponents.
 *
 * @example
 * // Render combat header showing dueling cards
 * const combat = combats[0];
 * console.log(`Combat ${combat.combat_number}: ${combat.outcome}`);
 *
 * @see {@link CombatEvent} Events within a combat
 * @see {@link BattleResult} Contains the combats array
 */
export interface Combat {
  /** 1-indexed position of this combat in the battle */
  combat_number: number
  /** Ordered sequence of events in this combat */
  events: CombatEvent[]
  /** How the combat ended */
  outcome: CombatOutcome
}

/**
 * A team's state during or after battle.
 *
 * Contains owner information and all cards in the team.
 *
 * @see {@link BattleResult} Contains team_a_final and team_b_final
 */
export interface BattleTeam {
  /** User ID of team owner */
  owner_id: number
  /** Display name of team owner */
  owner_name: string
  /** All cards in the team */
  cards: Card[]
}

/**
 * Complete battle results from the getBattle API endpoint.
 *
 * Contains everything needed to render a battle replay: both teams,
 * all events, scores, and winner information.
 *
 * @example
 * // Determine match outcome
 * if (battleResult.is_draw) {
 *   showDrawScreen();
 * } else if (battleResult.winner_id === currentUserId) {
 *   showVictoryScreen();
 * } else {
 *   showDefeatScreen();
 * }
 *
 * @see {@link BattleEvent} Individual battle events
 * @see {@link BattleTeam} Team structure
 */
export interface BattleResult {
  /** Match this battle belongs to */
  match_id: string
  /** User ID of winner (null if draw) */
  winner_id?: number | null
  /** Whether battle ended in a draw */
  is_draw: boolean
  /** Combats grouped by card duels (new primary structure) */
  combats: Combat[]
  /** Total number of combats */
  num_combats: number
  /** Ordered list of battle events for replay (legacy, used by animation hook) */
  events: BattleEvent[]
  /** Total number of combat rounds */
  num_rounds: number
  /** Total damage dealt by team A */
  team_a_damage: number
  /** Total damage dealt by team B */
  team_b_damage: number
  /** Team A final state */
  team_a_final: BattleTeam
  /** Team B final state */
  team_b_final: BattleTeam
  /** User ID of player A */
  player_a_id: number
  /** User ID of player B */
  player_b_id: number
  /** Display name of player A */
  player_a_name: string
  /** Display name of player B */
  player_b_name: string
}

// =============================================================================
// LEADERBOARD & STATS TYPES
// =============================================================================

/**
 * A single entry in the leaderboard rankings.
 *
 * Contains user identity and comprehensive stats for both ranked
 * and casual matches. Used in the Stats page leaderboard tab.
 *
 * @example
 * // Render rank badge
 * const badgeColor = entry.rank <= 3 ? 'gold' : 'default';
 *
 * // Calculate win rate
 * const totalGames = entry.ranked_wins + entry.ranked_losses + entry.ranked_draws;
 * const winRate = totalGames > 0 ? entry.ranked_wins / totalGames : 0;
 *
 * @see {@link LeaderboardResponse} Paginated response wrapper
 */
export interface LeaderboardEntry {
  /** Position in leaderboard (1 = top) */
  rank: number
  /** Telegram user ID */
  user_id: number
  /** Telegram chat ID */
  chat_id: number
  /** Display name */
  first_name: string
  /** Telegram username (optional) */
  username?: string
  /** Profile photo URL */
  photo_url?: string
  /** Ranked match wins */
  ranked_wins: number
  /** Ranked match losses */
  ranked_losses: number
  /** Ranked match draws */
  ranked_draws: number
  /** Total ranked tournaments entered */
  ranked_tournaments_played: number
  /** Ranked tournaments won */
  ranked_tournaments_won: number
  /** Current ranked win streak */
  ranked_current_streak: number
  /** Best ranked win streak ever */
  ranked_best_streak: number
  /** Casual match wins */
  regular_wins: number
  /** Casual match losses */
  regular_losses: number
  /** Casual match draws */
  regular_draws: number
  /** Total casual matches played */
  regular_matches_played: number
  /** Current casual win streak */
  regular_current_streak: number
  /** Best casual win streak ever */
  regular_best_streak: number
  /** Overall ranking score (Wilson Score 0-1 for regular, tournaments_won for ranked) */
  score: number
  /** Tier name (Lendário, Bichão, etc.) */
  tier?: string
  /** First match timestamp */
  first_match_at?: string
  /** Last match timestamp */
  last_match_at?: string
}

/**
 * Paginated leaderboard response from getLeaderboard endpoint.
 *
 * @see {@link LeaderboardEntry} Individual entry structure
 */
export interface LeaderboardResponse {
  /** Leaderboard entries for current page */
  entries: LeaderboardEntry[]
  /** Total number of entries */
  total: number
  /** Current page number (0-indexed) */
  page: number
  /** Entries per page */
  limit: number
  /** Whether there are more entries beyond current page */
  has_more: boolean
  /** Leaderboard type (ranked or regular/casual) */
  type: 'ranked' | 'regular'
}

/**
 * A single match in the user's history.
 *
 * Contains match result, opponent info, and team data.
 * Used in the Stats page history tab.
 *
 * @example
 * // Render result badge
 * const badgeClass = match.result === 'win' ? 'bg-green-500' :
 *                    match.result === 'loss' ? 'bg-red-500' : 'bg-gray-500';
 *
 * @see {@link HistoryResponse} Paginated response wrapper
 */
export interface MatchHistoryEntry {
  /** Match identifier */
  match_id: string
  /** Whether match was ranked or casual */
  match_type: MatchType
  /** Current user's profile photo URL */
  your_photo_url?: string
  /** Opponent information */
  opponent: {
    /** Opponent's user ID */
    user_id: number
    /** Opponent's display name */
    first_name: string
    /** Opponent's username (optional) */
    username?: string
    /** Opponent's profile photo URL */
    photo_url?: string
  }
  /** Match outcome for current user */
  result: 'win' | 'loss' | 'draw'
  /** Cards in user's team */
  your_team: Card[]
  /** Cards in opponent's team */
  opponent_team: Card[]
  /** ISO timestamp when match completed */
  completed_at: string
}

/**
 * Match history response from getHistory endpoint.
 *
 * @see {@link MatchHistoryEntry} Individual entry structure
 */
export interface HistoryResponse {
  /** Match history entries */
  matches: MatchHistoryEntry[]
  /** Total number of matches */
  total: number
  /** Whether more matches exist beyond current page */
  has_more: boolean
}

/**
 * Opponent information in H2H record.
 *
 * @see {@link H2HRecord} Parent record containing opponent
 */
export interface H2HOpponent {
  /** Opponent's user ID */
  user_id: number
  /** Opponent's display name */
  first_name: string
  /** Opponent's username (optional) */
  username?: string
}

/**
 * Head-to-head record against a specific opponent.
 *
 * Contains opponent info and win/loss statistics.
 *
 * @see {@link H2HResponse} Full response wrapper
 */
export interface H2HRecord {
  /** Opponent information */
  opponent: H2HOpponent
  /** Wins against this opponent */
  wins: number
  /** Losses against this opponent */
  losses: number
  /** Draws against this opponent */
  draws: number
  /** Total matches played */
  total_matches: number
  /** Win percentage (0-100) */
  win_rate: number
  /** ISO timestamp of last match */
  last_played?: string
}

/**
 * Head-to-head response from getH2H endpoint.
 *
 * Contains the win/loss record with nested opponent info and recent matches.
 *
 * @example
 * // Display H2H summary
 * const { record } = h2hData;
 * console.log(`vs ${record.opponent.first_name}`);
 * console.log(`${record.wins} W / ${record.draws} D / ${record.losses} L`);
 *
 * @see {@link H2HRecord} Record structure with opponent info
 */
export interface H2HResponse {
  /** Win/loss/draw record with opponent info */
  record: H2HRecord
  /** Recent matches against this opponent */
  recent_matches: MatchHistoryEntry[]
}

/**
 * Comprehensive user statistics.
 *
 * Contains all stats shown on the Profile tab, organized by
 * match type (ranked vs casual) and overall totals.
 *
 * @see {@link ProfileResponse} Full response with user info
 */
export interface ProfileStats {
  /** Ranked match wins */
  ranked_wins: number
  /** Ranked match losses */
  ranked_losses: number
  /** Ranked match draws */
  ranked_draws: number
  /** Total ranked matches */
  ranked_matches_played: number
  /** Ranked win percentage (0-100) */
  ranked_win_rate: number
  /** Current ranked win streak */
  ranked_current_streak: number
  /** Best ranked win streak */
  ranked_best_streak: number
  /** Ranked tournaments entered */
  ranked_tournaments_played: number
  /** Ranked tournaments won */
  ranked_tournaments_won: number
  /** Casual match wins */
  regular_wins: number
  /** Casual match losses */
  regular_losses: number
  /** Casual match draws */
  regular_draws: number
  /** Total casual matches */
  regular_matches_played: number
  /** Casual win percentage (0-100) */
  regular_win_rate: number
  /** Current casual win streak */
  regular_current_streak: number
  /** Best casual win streak */
  regular_best_streak: number
  /** Total matches (ranked + casual) */
  total_matches: number
  /** Total wins (ranked + casual) */
  total_wins: number
  /** Total damage dealt across all matches */
  total_damage_dealt: number
  /** Current tier name */
  tier?: string
  /** Current rank position */
  rank?: number
}

/**
 * User profile response from getProfile endpoint.
 *
 * Contains user identity and complete statistics.
 *
 * @example
 * // Display profile header
 * <h1>{profile.first_name}</h1>
 * <span>@{profile.username}</span>
 * <span>Tier: {profile.stats.tier}</span>
 *
 * @see {@link ProfileStats} Detailed statistics
 */
export interface ProfileResponse {
  /** Telegram user ID */
  user_id: number
  /** Display name */
  first_name: string
  /** Telegram username (optional) */
  username?: string
  /** Profile photo URL */
  photo_url?: string
  /** Complete user statistics (optional - may be undefined if user hasn't played any matches) */
  stats?: ProfileStats
}

// =============================================================================
// GAME CONSTANTS
// =============================================================================

/**
 * HP bar color thresholds for visual representation.
 *
 * Defines percentage thresholds for HP bar color transitions
 * and the corresponding CSS colors.
 *
 * @example
 * // Get HP bar color based on percentage
 * const percentage = (hp / maxHp) * 100;
 * const color = percentage > thresholds.high ? thresholds.colors.high :
 *               percentage >= thresholds.medium ? thresholds.colors.medium :
 *               thresholds.colors.low;
 */
export interface HpBarThresholds {
  /** HP percentage threshold for "high" health (default: 66) */
  high: number
  /** HP percentage threshold for "medium" health (default: 33) */
  medium: number
  /** Colors for each HP level */
  colors: {
    /** Color for high health (>66%) (default: '#22c55e' green) */
    high: string
    /** Color for medium health (33-66%) (default: '#eab308' yellow) */
    medium: string
    /** Color for low health (<33%) (default: '#ef4444' red) */
    low: string
  }
}

/**
 * Countdown timer urgency thresholds for visual representation.
 *
 * Defines time thresholds (in seconds) for timer color transitions
 * and the corresponding CSS colors.
 *
 * @example
 * // Get timer color based on remaining seconds
 * const color = remainingSeconds > thresholds.safe ? thresholds.colors.safe :
 *               remainingSeconds > thresholds.warning ? thresholds.colors.warning :
 *               thresholds.colors.urgent;
 */
export interface TimerThresholds {
  /** Time threshold in seconds for "safe" state (default: 120 = 2 minutes) */
  safe: number
  /** Time threshold in seconds for "warning" state (default: 30 seconds) */
  warning: number
  /** Colors for each timer urgency level */
  colors: {
    /** Color for safe state (>2 minutes) (default: '#22c55e' green) */
    safe: string
    /** Color for warning state (30s-2m) (default: '#eab308' yellow) */
    warning: string
    /** Color for urgent state (<30s) (default: '#ef4444' red) */
    urgent: string
  }
}

/**
 * Game economy and timing constants.
 *
 * Fetched once on app load and used throughout the UI for
 * validation, display, and game logic. These values are
 * configured on the backend and may change between deployments.
 *
 * @example
 * // Use constants for affordability check
 * const canBuy = coins >= constants.card_cost;
 * const canReroll = coins >= constants.reroll_cost;
 * const canUpgrade = coins >= constants.upgrade_cost;
 *
 * @example
 * // Use constants for display
 * <span>Cost: {constants.card_cost} coins</span>
 * <span>Upgrade: +{constants.upgrade_amount} stats</span>
 */
export interface GameConstants {
  /** Coins given at start of shop phase (default: 10) */
  starting_coins: number
  /** Cost to buy a card (default: 3) */
  card_cost: number
  /** Cost to reroll shop (default: 1) */
  reroll_cost: number
  /** Cost per upgrade (default: 1) */
  upgrade_cost: number
  /** Stats gained per upgrade (default: 3) */
  upgrade_amount: number
  /** Maximum team size (default: 3) */
  team_size: number
  /** Number of cards in shop (default: 6) */
  shop_size: number
  /** Seconds to join before auto-start */
  join_window_seconds: number
  /** Seconds for shop phase */
  shop_phase_seconds: number
  /** Minimum cards required to submit team */
  minimum_cards_required: number
  /** HP bar color thresholds */
  hp_bar_thresholds: HpBarThresholds
  /** Countdown timer urgency thresholds */
  timer_thresholds: TimerThresholds
}

// =============================================================================
// CARD IMAGE & COMPACT CARD TYPES
// =============================================================================

/**
 * Card image metadata from card-renderer API.
 *
 * Represents a generated card image stored in object storage.
 * Images are generated weekly from user activity metrics.
 *
 * @see {@link CardImageResponse} Version with presigned URL
 */
export interface CardImage {
  /** Unique image record ID */
  id: number
  /** Telegram user ID this card represents */
  user_id: number
  /** Chat where stats were collected */
  chat_id: number
  /** ISO date of week start for stats */
  week_start: string
  /** Object storage path */
  storage_path: string
  /** ISO timestamp when generated */
  generated_at: string
  /** User's first name (may be null) */
  first_name: string | null
  /** User's last name (may be null) */
  last_name: string | null
  /** User's username (may be null) */
  username: string | null
}

/**
 * Card image with presigned URL for direct access.
 *
 * The presigned URL allows temporary direct access to the image
 * without additional authentication. For compact cards, includes
 * position metadata for stat overlays.
 *
 * @extends CardImage
 * @see {@link PlaceholderPositions} Overlay positioning metadata
 */
export interface CardImageResponse extends CardImage {
  /** Presigned URL for image access */
  url: string
  /** Position metadata for compact card overlays */
  placeholder_positions?: PlaceholderPositions
}

/**
 * Position metadata for rendering a stat overlay on compact cards.
 *
 * Defines exact pixel coordinates, dimensions, and styling for
 * overlaying ATK/DEF/HP values on compact card images.
 *
 * @example
 * // Render stat at position
 * <div style={{
 *   position: 'absolute',
 *   left: position.x,
 *   top: position.y,
 *   width: position.width,
 *   fontSize: position.font_size,
 *   color: position.color
 * }}>
 *   {statValue}
 * </div>
 *
 * @see {@link CombatStatsPositions} Container for ATK/DEF/HP positions
 */
export interface StatPosition {
  /** X coordinate in pixels */
  x: number
  /** Y coordinate in pixels */
  y: number
  /** Width in pixels */
  width: number
  /** Height in pixels */
  height: number
  /** Anchor point (e.g., 'center') */
  anchor?: string
  /** Font size in pixels */
  font_size: number
  /** CSS font-weight value */
  font_weight?: string
  /** CSS color value */
  color: string
  /** CSS text-align value */
  text_align?: string
  /** Container X for grouped positioning */
  container_x?: number
  /** Container Y for grouped positioning */
  container_y?: number
  /** Container width */
  container_width?: number
  /** Container height */
  container_height?: number
  /** Development notes */
  note?: string
  /** CSS text-shadow value */
  text_shadow?: string
}

/**
 * Positions for combat stat overlays (ATK and HP).
 *
 * @see {@link StatPosition} Individual stat position
 */
export interface CombatStatsPositions {
  /** ATK stat position */
  atk: StatPosition
  /** HP stat position */
  hp: StatPosition
}

/**
 * Position and dimensions for HP bar container.
 *
 * The container is the background/frame of the HP bar.
 *
 * @see {@link HpBarPositions} Complete HP bar configuration
 */
export interface HpBarContainerPosition {
  /** X coordinate in pixels */
  x: number
  /** Y coordinate in pixels */
  y: number
  /** Width in pixels */
  width: number
  /** Height in pixels */
  height: number
  /** Border radius in pixels */
  border_radius: number
  /** Development notes */
  note?: string
}

/**
 * Position and styling for HP bar fill element.
 *
 * The fill represents current HP as a percentage of max_width.
 *
 * @see {@link HpBarPositions} Complete HP bar configuration
 */
export interface HpBarFillPosition {
  /** X coordinate in pixels */
  x: number
  /** Y coordinate in pixels */
  y: number
  /** Maximum width when HP is full */
  max_width: number
  /** Height in pixels */
  height: number
  /** Border radius in pixels */
  border_radius: number
  /** Fill color (CSS) */
  color: string
  /** Background/empty color (CSS) */
  background_color: string
  /** Development notes */
  note?: string
}

/**
 * Complete HP bar positioning with container and fill.
 *
 * @see {@link Placeholders} Parent structure containing HP bar
 */
export interface HpBarPositions {
  /** Background/frame position */
  container: HpBarContainerPosition
  /** Fill/progress position */
  fill: HpBarFillPosition
}

/**
 * All placeholder positions for compact card overlays.
 *
 * @see {@link PlaceholderPositions} Root metadata structure
 */
export interface Placeholders {
  /** ATK/HP stat positions */
  combat_stats: CombatStatsPositions
  /** HP bar container and fill positions (optional, may not be present) */
  hp_bar?: HpBarPositions
}

/**
 * Card dimensions metadata.
 *
 * Defines the size of the compact card image.
 */
export interface CardDimensions {
  /** Image width in specified units */
  width: number
  /** Image height in specified units */
  height: number
  /** Scale factor for rendering */
  scale: number
  /** CSS unit (e.g., 'px') */
  unit: string
  /** Development notes */
  note?: string
}

/**
 * Tier-specific style variations for stat overlays.
 *
 * Allows different tiers (Lendário, Bichão, etc.) to have
 * slightly different styling or positions.
 */
export interface TierVariation {
  /** Combat stat position overrides */
  combat_stats?: {
    /** ATK position overrides */
    atk?: Partial<StatPosition>
    /** HP position overrides */
    hp?: Partial<StatPosition>
  }
}

/**
 * Complete placeholder positions metadata for compact cards.
 *
 * This metadata is returned by the card-renderer API and enables
 * precise positioning of stat overlays on compact card images.
 * The CompactCard component uses this to render dynamic values.
 *
 * @example
 * // Render ATK overlay using metadata
 * const { combat_stats } = positions.placeholders;
 * <span style={{
 *   position: 'absolute',
 *   left: combat_stats.atk.x,
 *   top: combat_stats.atk.y,
 *   fontSize: combat_stats.atk.font_size,
 *   color: combat_stats.atk.color
 * }}>
 *   {card.atk}
 * </span>
 *
 * @see {@link CardImageResponse} Returns this in placeholder_positions
 * @see {@link EnhancedTeamCard} Cards that include this metadata
 */
export interface PlaceholderPositions {
  /** Metadata version for compatibility */
  version: string
  /** Card image dimensions */
  card_dimensions: CardDimensions
  /** All overlay positions */
  placeholders: Placeholders
  /** Optional tier-specific overrides */
  tier_variations?: Record<string, TierVariation>
  /** Developer usage notes */
  usage_notes?: Record<string, string>
}

// =============================================================================
// UI TYPES
// =============================================================================

/**
 * Main navigation tab identifiers.
 *
 * Controls which page is displayed in the main app view.
 *
 * @example
 * const [activeTab, setActiveTab] = useState<TabId>('lobby');
 */
export type TabId = 'lobby' | 'shop' | 'battle' | 'stats'

/**
 * Stats page sub-tab identifiers.
 *
 * Controls which section is displayed within the Stats page.
 * Note: H2H is now shown as a modal overlay instead of a separate tab.
 *
 * @example
 * const [subTab, setSubTab] = useState<StatsSubTab>('leaderboard');
 */
export type StatsSubTab = 'leaderboard' | 'history'

/**
 * Application lifecycle state.
 *
 * Controls what the main App component renders:
 * - `loading`: Show splash screen while authenticating
 * - `authenticated`: Show main app with tab navigation
 * - `error`: Show error screen with retry option
 *
 * @example
 * if (appState === 'loading') return <SplashScreen />;
 * if (appState === 'error') return <ErrorDisplay onRetry={retry} />;
 * return <MainApp />;
 */
export type AppState = 'loading' | 'authenticated' | 'error'

/**
 * Card upgrade types.
 *
 * - `atk`: Increase attack by 3
 * - `hp`: Increase max HP by 3
 *
 * @example
 * await apiClient.upgradeCard(matchId, slotIndex, 'atk');
 */
export type UpgradeType = 'atk' | 'hp'
