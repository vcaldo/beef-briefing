# Ranked Matches Documentation

## Overview

The Arena Mini App supports two types of matches:

- **🏆 Ranked Matches**: Daily competitive tournaments with registration, bracket-based elimination, and tournament-focused rankings
- **⚔️ Casual Matches**: Instant quick-play matches with confidence-based rankings

Ranked matches are managed by the Telegram bot scheduler and follow a strict daily schedule with announcement, registration, and battle phases. Players compete for tournament wins, which serve as the primary ranking metric.

## Frontend Implementation

### Type Definitions

All types are defined in [src/types/index.ts](src/types/index.ts).

#### Match Type Enum (Line 68)
```typescript
export type MatchType = 'ranked' | 'regular';
```

#### Match Interface (Lines 197-232)
```typescript
export interface Match {
  id: string;
  chat_id: number;
  match_type: MatchType;  // Distinguishes ranked from casual
  format: MatchFormat;
  status: MatchStatus;
  tournament_date?: string;  // YYYY-MM-DD, populated only for ranked
  creator_user_id?: number;  // Only for casual matches
  // ... other fields
}
```

#### Leaderboard Types (Lines 654-721)

**LeaderboardEntry** - Individual player stats:
```typescript
export interface LeaderboardEntry {
  user_id: number;
  username?: string;
  first_name?: string;

  // Ranked-specific stats
  ranked_wins: number;
  ranked_losses: number;
  ranked_draws: number;
  ranked_tournaments_played: number;
  ranked_tournaments_won: number;
  ranked_current_streak: number;
  ranked_best_streak: number;

  // Regular (casual) stats
  regular_wins: number;
  regular_losses: number;
  regular_draws: number;
  regular_matches_played: number;
  regular_current_streak: number;
  regular_best_streak: number;

  // Other fields...
}
```

**LeaderboardResponse**:
```typescript
export interface LeaderboardResponse {
  entries: LeaderboardEntry[];
  total: number;
  type: 'ranked' | 'regular';  // Filter type
  chat_id?: number;
}
```

#### Profile Stats (Lines 844-887)
```typescript
export interface ProfileStats {
  ranked: {
    wins: number;
    losses: number;
    draws: number;
    tournaments_played: number;
    tournaments_won: number;
    current_streak: number;
    best_streak: number;
    win_rate: number;  // Calculated percentage
  };
  regular: {
    wins: number;
    losses: number;
    draws: number;
    matches_played: number;
    current_streak: number;
    best_streak: number;
    win_rate: number;
  };
  // ... other fields
}
```

### UI Components

#### Lobby Page ([src/components/lobby/LobbyPage.tsx](src/components/lobby/LobbyPage.tsx))

**Match Type Display** (Lines 425-430):
```typescript
// In match card title
{match.match_type === 'ranked' ? '🏆 Ranked' : '⚔️ Casual'}
```

Users see the match type prominently displayed in the lobby. There's no UI control to toggle between ranked and casual - the match type is determined by the backend based on tournament state.

#### Stats Page ([src/components/stats/StatsPage.tsx](src/components/stats/StatsPage.tsx))

**Leaderboard Toggle** (Lines 346-362):
```typescript
const [leaderboardType, setLeaderboardType] = useState<'ranked' | 'regular'>('regular');

// Toggle buttons
<button onClick={() => setLeaderboardType('regular')}>⚔️ Casual</button>
<button onClick={() => setLeaderboardType('ranked')}>🏆 Ranked</button>
```

**Leaderboard Rendering**:
- **Ranked**: Shows tournament wins as primary score
- **Casual**: Shows Wilson Score percentage (0-100%)
- Different stat columns based on type

**Match History** (Lines 667-668):
- Each match shows type indicator: `{match.match_type === 'ranked' ? '🏆' : '⚔️'}`

**Profile Stats Section** (Lines 525-560):
- Dedicated "🏆 Ranked" section showing:
  - Win/loss/draw record
  - Win rate percentage
  - Current and best streaks
  - Tournament wins / tournaments played

### API Integration

The API client ([src/api/client.ts](src/api/client.ts)) provides methods for interacting with ranked matches:

```typescript
// Match creation - match type determined by backend
async createMatch(chatId?: number): Promise<Match>

// Leaderboard with type filter
async getLeaderboard(
  chatId?: number,
  type: 'ranked' | 'regular' = 'regular',
  limit: number = 50,
  offset: number = 0
): Promise<LeaderboardResponse>

// Other methods return match_type in response
async getMatches(): Promise<Match[]>
async getProfile(): Promise<ProfileStats>
async getMatchHistory(): Promise<MatchHistoryEntry[]>
```

## Backend Architecture

### Database Schema

#### Core Tables (Migration: `009_game_arena.sql`)

**`game_matches`** - All matches (ranked and casual)
```sql
CREATE TABLE game_matches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  chat_id BIGINT NOT NULL REFERENCES chats(id),
  match_type VARCHAR(20) NOT NULL CHECK (match_type IN ('ranked', 'regular')),
  format VARCHAR(20) NOT NULL CHECK (format IN ('1v1', 'arena')),
  status VARCHAR(20) NOT NULL,
  tournament_date DATE,  -- Populated only for ranked matches
  creator_user_id BIGINT,  -- Populated only for casual matches
  -- ... timestamps and other fields

  -- Constraints ensure data integrity
  CONSTRAINT ranked_has_tournament_date CHECK (
    match_type != 'ranked' OR tournament_date IS NOT NULL
  ),
  CONSTRAINT regular_has_creator CHECK (
    match_type != 'regular' OR creator_user_id IS NOT NULL
  )
);
```

**Indexes**:
- `idx_game_matches_tournament` on `(chat_id, tournament_date)` - Fast ranked match lookups
- `idx_game_matches_chat_status` on `(chat_id, status)` - Active match queries

**`game_ranked_tournaments`** - Daily tournament container
```sql
CREATE TABLE game_ranked_tournaments (
  id BIGSERIAL PRIMARY KEY,
  chat_id BIGINT NOT NULL REFERENCES chats(id),
  tournament_date DATE NOT NULL,
  status VARCHAR(20) NOT NULL,  -- scheduled, open, in_progress, completed, skipped
  announcement_message_id BIGINT,
  match_id UUID REFERENCES game_matches(id),
  winner_user_id BIGINT REFERENCES users(id),
  participant_count INT DEFAULT 0,
  bracket_state JSONB,  -- For arena format bracket tracking
  announced_at TIMESTAMP WITH TIME ZONE,
  registration_closed_at TIMESTAMP WITH TIME ZONE,
  completed_at TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

  UNIQUE(chat_id, tournament_date)  -- One tournament per chat per day
);
```

**Tournament Status Flow**:
```
scheduled → open → in_progress → completed
                                → skipped (< 2 participants)
```

**`game_tournament_participants`** - Registration tracking
```sql
CREATE TABLE game_tournament_participants (
  tournament_id BIGINT NOT NULL REFERENCES game_ranked_tournaments(id),
  user_id BIGINT NOT NULL REFERENCES users(id),
  joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  PRIMARY KEY (tournament_id, user_id)
);
```

**`game_leaderboard`** - Aggregated player statistics
```sql
CREATE TABLE game_leaderboard (
  user_id BIGINT NOT NULL,
  chat_id BIGINT NOT NULL,

  -- Ranked statistics
  ranked_wins INT DEFAULT 0,
  ranked_losses INT DEFAULT 0,
  ranked_draws INT DEFAULT 0,
  ranked_tournaments_played INT DEFAULT 0,
  ranked_tournaments_won INT DEFAULT 0,
  ranked_current_streak INT DEFAULT 0,
  ranked_best_streak INT DEFAULT 0,

  -- Regular (casual) statistics
  regular_wins INT DEFAULT 0,
  regular_losses INT DEFAULT 0,
  regular_draws INT DEFAULT 0,
  regular_matches_played INT DEFAULT 0,
  regular_current_streak INT DEFAULT 0,
  regular_best_streak INT DEFAULT 0,

  -- Head-to-head tracking (JSONB)
  head_to_head JSONB DEFAULT '{}',

  PRIMARY KEY (user_id, chat_id)
);
```

**`chats` table extension** - Per-group control
```sql
ALTER TABLE chats ADD COLUMN ranked_tournaments_enabled BOOLEAN DEFAULT FALSE;
```

**Default behavior**: Groups must **opt-in** to enable ranked tournaments.

### API Endpoints

All endpoints under `/api/v1/mini-app/arena/` require **JWT authentication** (Mini App users) or **API Key authentication** (Bot scheduler).

#### Tournament Management (Bot API Key Only)
| Method | Endpoint | Purpose | Auth |
|--------|----------|---------|------|
| GET | `/arena/tournament/today?chat_id=X&date=YYYY-MM-DD` | Get today's tournament | Bot API Key |
| GET | `/arena/tournament/{id}` | Get tournament by ID | Bot API Key |
| GET | `/arena/tournaments/pending-announcements` | Tournaments needing 00:01 announcement | Bot API Key |
| POST | `/arena/tournament/announce` | Create and announce tournament | Bot API Key |
| POST | `/arena/tournament/join` | Add user to tournament | Bot API Key |
| POST | `/arena/tournament/leave` | Remove user from tournament | Bot API Key |
| GET | `/arena/tournaments/pending-close` | Tournaments at 18:00 close time | Bot API Key |
| POST | `/arena/tournament/{id}/close` | Close registration, start match | Bot API Key |
| GET | `/arena/tournaments/pending-rounds` | Tournaments with pending rounds | Bot API Key |

#### Mini App Endpoints (JWT Authentication)
| Method | Endpoint | Purpose | Returns match_type |
|--------|----------|---------|-------------------|
| POST | `/arena/match` | Create match (casual only) | Yes |
| GET | `/arena/matches` | List active matches | Yes |
| GET | `/arena/match/{id}` | Get match details | Yes |
| GET | `/arena/leaderboard?type=ranked` | Get ranked or casual leaderboard | Response includes type |
| GET | `/arena/profile` | Get user profile with ranked stats | Yes |
| GET | `/arena/history?chat_id=X` | Match history with types | Yes |

**Note**: Frontend users cannot create ranked matches via `/arena/match`. Ranked matches are created exclusively by the bot scheduler when closing tournament registration.

### Tournament Lifecycle

#### Phase 1: Announcement (00:01-00:10 local time)
```
Bot Scheduler:
1. Calls GET /arena/tournaments/pending-announcements
2. For each tournament:
   a. Sends Telegram announcement message
   b. Calls POST /arena/tournament/announce
   c. Tournament status: scheduled → open
```

**Database Function**: `get_tournaments_needing_announcement(current_time)`
- Filters tournaments where local time is 00:01-00:10
- Only includes chats with `ranked_tournaments_enabled = true`
- Uses timezone from `chats.timezone` (default: America/Sao_Paulo)

#### Phase 2: Registration (00:01-18:00 local time)
```
Users interact via Telegram bot /ranked command:
- /ranked join → POST /arena/tournament/join
- /ranked leave → POST /arena/tournament/leave
- /ranked status → Shows current tournament state
```

**State**: Tournament status = `open`

#### Phase 3: Registration Close (18:00-18:05 local time)
```
Bot Scheduler:
1. Calls GET /arena/tournaments/pending-close
2. For each tournament:
   a. Retrieves registered participants
   b. If < 2 participants → SkipTournament (status='skipped')
   c. If >= 2 participants → CloseAndStartTournament:
      - Determine format: 2 players = '1v1', 3+ = 'arena'
      - Create ranked match with tournament_date
      - Add all participants to match
      - Deal shop cards (6 unique per player)
      - Start shop phase (3-minute deadline)
      - Link tournament → match, status='in_progress'
```

**Database Function**: `get_tournaments_needing_close(current_time)`
- Returns tournaments with registration closing time
- Includes participant count for format determination

#### Phase 4: Match Execution (Shop → Battle)
```
Shop Phase:
- Players submit 3-card teams with upgrades
- Polling continues until all teams submitted or deadline

Battle Phase:
- 1v1 format: Single battle, direct winner
- Arena format: Bracket elimination rounds
  - Round 1: Pair all players, winners advance
  - Round 2+: Continue until 1 winner remains
```

**State**: Tournament status = `in_progress`

#### Phase 5: Completion
```
When match completes:
1. Winner determined
2. Leaderboard updated (ranked_wins++, ranked_tournaments_won++ for winner)
3. Tournament marked completed
4. Participants can create new casual matches
```

**State**: Tournament status = `completed`

### Scheduler Integration

**Component**: `TournamentScheduler` in telegram-bot service
**Poll Interval**: 1 minute
**Processing Order**:
1. Process announcements (00:01-00:10)
2. Process registration closes (18:00-18:05)
3. Process pending rounds (ongoing arena brackets)

**Stopping Condition**: If `RANKED_TOURNAMENTS_ENABLED=false`, scheduler exits early without processing.

## Enable/Disable Configuration

Ranked tournaments use a **dual control system**: both global AND per-group settings must be enabled.

### Global Control (Environment Variable)

**Variable**: `RANKED_TOURNAMENTS_ENABLED`
**Default**: `true`
**Location**: `infrastructure/.env.dev` or `infrastructure/.env.prod`

```bash
# Enable globally (default)
RANKED_TOURNAMENTS_ENABLED=true

# Disable globally (kills all tournaments across all groups)
RANKED_TOURNAMENTS_ENABLED=false
```

**Effect**: If false, the bot scheduler's `processTournaments()` returns early without checking any tournaments. No announcements, no registration closes, no matches created.

### Per-Group Control (Database Flag)

**Column**: `chats.ranked_tournaments_enabled`
**Default**: `FALSE` (opt-in model)
**Type**: `BOOLEAN`

Groups are **disabled by default** and must explicitly enable ranked tournaments.

#### Using Makefile Commands

**Development**:
```bash
# Enable ranked for specific group
make ranked-enable CHAT_ID=-1002345678901

# Disable ranked for specific group
make ranked-disable CHAT_ID=-1002345678901

# Check status of all groups
make ranked-status

# Check status of specific group
make ranked-status-chat CHAT_ID=-1002345678901

# Enable all groups (requires confirmation)
make ranked-enable-all

# Disable all groups (requires confirmation)
make ranked-disable-all
```

**Production** (requires `make pg-tunnel` in another terminal):
```bash
# Enable for specific group
make ranked-enable-prod CHAT_ID=-1002345678901

# Disable for specific group
make ranked-disable-prod CHAT_ID=-1002345678901

# Check status of all groups
make ranked-status-prod

# Check status of specific group
make ranked-status-chat-prod CHAT_ID=-1002345678901

# Enable all groups (requires confirmation)
make ranked-enable-all-prod

# Disable all groups (requires confirmation)
make ranked-disable-all-prod
```

#### Using SQL Directly

```sql
-- Enable for specific group
UPDATE chats SET ranked_tournaments_enabled = true WHERE id = -1002345678901;

-- Disable for specific group
UPDATE chats SET ranked_tournaments_enabled = false WHERE id = -1002345678901;

-- Find chat ID by name
SELECT id, title, ranked_tournaments_enabled FROM chats WHERE title ILIKE '%group name%';

-- Check all groups
SELECT id, title, ranked_tournaments_enabled
FROM chats
WHERE type IN ('group', 'supergroup');
```

### Control Flow

```
Tournament runs ONLY if:
├─ RANKED_TOURNAMENTS_ENABLED = true (global env var)
└─ chats.ranked_tournaments_enabled = true (per-group DB flag)

If either is false:
├─ Scheduler skips tournament processing
└─ /ranked command shows error message
```

**User-facing error** (when per-group disabled):
```
⚠️ Ranked tournaments are disabled for this group.
```

## Tournament Workflow

### Daily Schedule

All times are in the **group's local timezone** (configured in `chats.timezone`, default: `America/Sao_Paulo`).

| Time | Phase | Action |
|------|-------|--------|
| **00:01** | Announcement | Bot sends tournament announcement, opens registration |
| **00:01-18:00** | Registration | Users join/leave via `/ranked` command |
| **18:00** | Close | Bot closes registration, creates match, deals shop cards |
| **18:00-18:03** | Shop Phase | 3-minute timer for team building |
| **18:03+** | Battle Phase | Bracket execution (instant or multi-round) |
| **18:05+** | Completion | Winner announced, leaderboard updated |

### Format Determination

Format is determined automatically based on participant count when registration closes:

| Participants | Format | Battle Type |
|--------------|--------|-------------|
| 0-1 | None | Tournament skipped |
| 2 | `1v1` | Single head-to-head battle |
| 3+ | `arena` | Multi-round bracket elimination |

### Timezone Handling

Each chat can have a custom timezone:
```sql
SELECT id, title, timezone FROM chats WHERE id = -1002345678901;
```

**Default timezone**: `America/Sao_Paulo` (UTC-3)

The scheduler uses PostgreSQL database functions to calculate local time:
- `get_tournaments_needing_announcement(current_time)` - Checks if local time is 00:01-00:10
- `get_tournaments_needing_close(current_time)` - Checks if local time is 18:00-18:05

This allows different groups in different timezones to have tournaments at their local times.

## Ranking Systems

### Ranked Leaderboard

**Primary Metric**: Tournament wins (`ranked_tournaments_won`)

**SQL Ordering**:
```sql
ORDER BY
  ranked_tournaments_won DESC,  -- Primary: tournament wins
  ranked_wins DESC,              -- Tiebreaker: total ranked wins
  ranked_losses ASC              -- Tiebreaker: fewest losses
```

**Score Display**: Integer count of tournament wins

**Focus**: Rewards consistency in winning daily tournaments rather than just individual match wins.

### Casual Leaderboard

**Primary Metric**: Wilson Score confidence interval

**SQL Ordering**:
```sql
ORDER BY
  wilson_score_lower_bound(regular_wins, regular_losses) DESC,
  regular_wins DESC,
  regular_losses ASC
```

**Wilson Score Formula** (from `014_wilson_score.sql`):
```sql
CREATE FUNCTION wilson_score_lower_bound(wins INT, losses INT) RETURNS FLOAT AS $$
DECLARE
  n INT;
  p FLOAT;
  z FLOAT := 1.96;  -- 95% confidence
BEGIN
  n := wins + losses;
  IF n = 0 THEN RETURN 0; END IF;

  p := wins::FLOAT / n;
  RETURN ((p + z*z/(2*n)) - z * sqrt((p*(1-p) + z*z/(4*n))/n)) / (1 + z*z/n);
END;
$$ LANGUAGE plpgsql IMMUTABLE;
```

**Score Display**: Percentage (0-100%) representing win confidence

**Advantages**:
- Penalizes players with few games (low confidence)
- Rewards high win percentage with sufficient sample size
- Statistical rigor for fair casual rankings
- Draws excluded from calculation

## Key Differences: Ranked vs Casual

| Aspect | Ranked 🏆 | Casual ⚔️ |
|--------|-----------|-----------|
| **Creation** | Bot creates daily tournaments | Anyone creates instant matches |
| **Schedule** | Fixed daily (00:01 announcement, 18:00 start) | Anytime, instant |
| **Registration** | 12h+ registration period | Instant join during open phase |
| **Format** | 1v1 or arena (based on participant count) | 1v1 or arena (based on participants) |
| **Lock System** | Separate from casual | One active casual match per chat |
| **Leaderboard Algorithm** | Tournament wins count | Wilson Score confidence (0-1) |
| **Score Display** | Integer (tournaments won) | Percentage (0-100% confidence) |
| **Primary Stat** | `ranked_tournaments_won` | Wilson Score lower bound |
| **Stats Tracked** | Wins, losses, draws, tournaments played/won, streaks | Wins, losses, draws, matches played, streaks |
| **UI Indicator** | 🏆 Ranked | ⚔️ Casual |
| **Database Fields** | `match_type='ranked'`, `tournament_date` NOT NULL | `match_type='regular'`, `creator_user_id` NOT NULL |
| **Timezone-aware** | Yes (local time for announcements) | No |
| **Enable/Disable** | Global env var + per-group DB flag | Always available |

## Match Creation Flow

### Creating Casual Matches

**Frontend**:
```typescript
const match = await apiClient.createMatch(chatId);
// Returns match with match_type='regular'
```

**Backend**:
1. Check if active casual match exists for chat → Error if exists
2. Create match with `match_type='regular'`, `creator_user_id=current_user`
3. Add creator as participant
4. Status = `open` (waiting for opponent)

### Creating Ranked Matches

**Frontend**: No creation endpoint available (read-only access)

**Backend** (Bot Scheduler only):
1. Tournament registration closes at 18:00
2. Bot calls `POST /arena/tournament/{id}/close`
3. Backend:
   - Retrieves all registered participants
   - If < 2: Skip tournament
   - If >= 2: Create match with `match_type='ranked'`, `tournament_date=today`
   - Add all participants
   - Deal shop cards
   - Start shop phase
   - Link tournament → match

## Troubleshooting

### Tournaments Not Running

**Check 1: Global setting**
```bash
# In .env.prod or .env.dev
grep RANKED_TOURNAMENTS_ENABLED infrastructure/.env.prod
```
Should be `true`. If missing, defaults to `true`.

**Check 2: Per-group setting**
```bash
# Development
make ranked-status

# Production
make ranked-status-prod
```

Look for the specific chat_id in the output. If `ranked_tournaments_enabled = false`, the group is disabled.

**Check 3: Scheduler logs**
```bash
# Check if scheduler is running
make logs-bot | grep -i tournament

# Look for:
# - "Tournament scheduler initialized"
# - "Processing tournaments..."
# - "Announced tournament for chat_id=X"
```

If no logs appear, check:
1. Is the bot running? `docker ps | grep telegram-bot`
2. Is `RANKED_TOURNAMENTS_ENABLED` set to false?

### Tournaments Getting Skipped

**Symptom**: Tournament announced but never starts at 18:00

**Check participant count**:
```sql
SELECT t.id, t.chat_id, t.tournament_date, t.status, t.participant_count
FROM game_ranked_tournaments t
WHERE t.tournament_date = CURRENT_DATE
AND t.status = 'open';
```

If `participant_count < 2`, tournament will be skipped when registration closes.

**Check registration data**:
```sql
SELECT tp.tournament_id, tp.user_id, u.first_name, tp.joined_at
FROM game_tournament_participants tp
JOIN users u ON u.id = tp.user_id
WHERE tp.tournament_id = X;
```

### Leaderboard Not Showing Ranked Stats

**Check 1: Match type filter**
```typescript
// In API call
const response = await apiClient.getLeaderboard(chatId, 'ranked');
```

Ensure `type` parameter is `'ranked'`, not `'regular'`.

**Check 2: Database stats**
```sql
SELECT user_id, ranked_wins, ranked_losses, ranked_tournaments_won
FROM game_leaderboard
WHERE chat_id = -1002345678901
ORDER BY ranked_tournaments_won DESC
LIMIT 10;
```

If all values are 0, no ranked matches have been played yet.

**Check 3: Group enabled**
```sql
SELECT ranked_tournaments_enabled FROM chats WHERE id = -1002345678901;
```

### Match Shows Wrong Type

**Symptom**: Match appears as casual when it should be ranked (or vice versa)

**Investigate match**:
```sql
SELECT id, chat_id, match_type, tournament_date, creator_user_id, status
FROM game_matches
WHERE id = 'match-uuid-here';
```

**Expected values**:
- Ranked: `match_type='ranked'`, `tournament_date IS NOT NULL`, `creator_user_id IS NULL`
- Casual: `match_type='regular'`, `tournament_date IS NULL`, `creator_user_id IS NOT NULL`

If values don't match expectations, there may be a bug in match creation logic.

### Timezone Issues

**Symptom**: Announcements or closes happening at wrong times

**Check chat timezone**:
```sql
SELECT id, title, timezone FROM chats WHERE id = -1002345678901;
```

**Update timezone**:
```sql
UPDATE chats SET timezone = 'America/New_York' WHERE id = -1002345678901;
```

**Supported timezones**: Any IANA timezone name (e.g., `America/Sao_Paulo`, `Europe/London`, `Asia/Tokyo`)

## Related Documentation

- [Arena Mini App README](README.md) - General arena game documentation
- [Backend Game System](../../api-service/internal/game/README.md) - Detailed backend implementation
- [CLAUDE.md](../../CLAUDE.md#ranked-tournaments-configuration) - Configuration reference

## Code References

**Frontend**:
- [src/types/index.ts](src/types/index.ts) - Type definitions for Match, LeaderboardEntry, ProfileStats
- [src/components/lobby/LobbyPage.tsx](src/components/lobby/LobbyPage.tsx#L425-L430) - Match type display
- [src/components/stats/StatsPage.tsx](src/components/stats/StatsPage.tsx#L346-L362) - Leaderboard toggle
- [src/api/client.ts](src/api/client.ts) - API integration

**Backend**:
- `apps/api-service/internal/migrations/sql/009_game_arena.sql` - Database schema
- `apps/api-service/internal/handlers/arena_handler.go` - HTTP handlers
- `apps/api-service/internal/services/tournament_service.go` - Tournament business logic
- `apps/api-service/internal/repository/game_repository.go` - Database queries
- `apps/telegram-bot/internal/tournament_scheduler.go` - Scheduler implementation
