// Card types
export interface GameCard {
  card_id: number;
  user_id: number;
  name: string;
  username?: string;
  photo_url?: string;
  atk: number;
  def: number;
  hp: number;
  max_hp: number;
  atk_upgrades: number;
  hp_upgrades: number;
  position?: number;
}

export interface ShopCard extends GameCard {
  is_purchased: boolean;
  index: number;
  stats?: Record<string, unknown>;
}

// Match types
export type MatchType = 'ranked' | 'regular';
export type MatchFormat = '1v1' | 'arena';
export type MatchStatus = 'open' | 'shop_phase' | 'battle_phase' | 'completed' | 'cancelled';
export type ParticipantStatus = 'joined' | 'ready' | 'eliminated' | 'winner';

export interface Participant {
  id: number;
  match_id: string;
  user_id: number;
  status: ParticipantStatus;
  joined_at: string;
  coins_remaining: number;
  first_name: string;
  username?: string;
}

export interface Match {
  id: string;
  chat_id: number;
  match_type: MatchType;
  format?: MatchFormat;
  status: MatchStatus;
  created_at: string;
  join_deadline?: string;
  shop_phase_started_at?: string;
  shop_phase_deadline?: string;
  battle_started_at?: string;
  completed_at?: string;
  tournament_date?: string;
  creator_user_id?: number;
  current_round: number;
  winner_user_id?: number;
  participants: Participant[];
  card_count: number;
}

// Shop types
export interface ShopState {
  match_id: string;
  status: string;
  coins: number;
  cards: ShopCard[];
  team: GameCard[];
  team_order: number[];
  is_ready: boolean;
  deadline?: string;
  time_remaining_seconds: number;
}

// Battle types
export type EventType = 'attack' | 'damage' | 'death' | 'advance' | 'victory';

export interface BattleEvent {
  type: EventType;
  round: number;
  message?: string;
  attacker_card_id?: number;
  defender_card_id?: number;
  attacker_team_owner_id?: number;
  defender_team_owner_id?: number;
  damage?: number;
  hp_before?: number;
  hp_after?: number;
}

export interface MatchRound {
  id: number;
  match_id: string;
  round_number: number;
  player_a_id: number;
  player_b_id: number;
  player_a_team: GameCard[];
  player_b_team: GameCard[];
  winner_id?: number;
  is_draw: boolean;
  battle_log: BattleEvent[];
  player_a_damage: number;
  player_b_damage: number;
  total_rounds: number;
  created_at: string;
}

export interface BattleResult {
  match_id: string;
  status: string;
  rounds: MatchRound[];
  winner_id?: number;
  is_complete: boolean;
}

// Leaderboard types
export interface LeaderboardEntry {
  user_id: number;
  chat_id: number;
  ranked_wins: number;
  ranked_losses: number;
  ranked_tournaments_played: number;
  ranked_tournaments_won: number;
  ranked_current_streak: number;
  ranked_best_streak: number;
  regular_wins: number;
  regular_losses: number;
  regular_matches_played: number;
  regular_current_streak: number;
  regular_best_streak: number;
  first_name: string;
  username?: string;
}

// Auth types
export interface AuthResponse {
  token: string;
  user_id: number;
  chat_id?: number;
  chat_title?: string;
  first_name: string;
  username?: string;
  is_admin: boolean;
  chat_timezone?: string;
}

// Match history types
export interface MatchHistoryEntry {
  match_id: string;
  match_type: MatchType;
  opponent: {
    user_id: number;
    first_name: string;
    username?: string;
  };
  result: 'win' | 'loss' | 'draw';
  your_team: GameCard[];
  opponent_team: GameCard[];
  completed_at: string;
}

// Head-to-head types
export interface H2HRecord {
  opponent: {
    user_id: number;
    first_name: string;
    username?: string;
  };
  wins: number;
  losses: number;
  last_match_at?: string;
}

// App state
export type AppPage = 'lobby' | 'shop' | 'battle' | 'results' | 'leaderboard' | 'history' | 'h2h';
export type AppState = 'loading' | 'authenticated' | 'error';
