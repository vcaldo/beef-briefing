# Beef Arena Game API Documentation

> Complete guide to the Beef Arena card battle game mechanics and API for frontend implementation

## Table of Contents

- [Overview](#overview)
- [Game Mechanics](#game-mechanics)
  - [Match Types](#match-types)
  - [Match Lifecycle](#match-lifecycle)
  - [Shop Phase](#shop-phase)
  - [Battle Phase](#battle-phase)
- [API Endpoints](#api-endpoints)
  - [Authentication](#authentication)
  - [Match Management](#match-management)
  - [Shop Actions](#shop-actions)
  - [Battle & Results](#battle--results)
  - [Leaderboards & Stats](#leaderboards--stats)
  - [Ranked Tournaments](#ranked-tournaments)
- [Data Models](#data-models)
- [Game Constants](#game-constants)
- [Frontend Integration Guide](#frontend-integration-guide)
- [Code References](#code-references)

---

## Overview

Beef Arena is a competitive card battle game where players:
1. Join matches with other players
2. Build teams of 3 cards from a random shop (with optional rerolls before first purchase)
3. Battle automatically using turn-based combat
4. Earn rankings based on wins/losses

The game supports two modes:
- **Casual matches** (regular): Quick 1v1 matches anyone can create
- **Ranked tournaments**: Daily scheduled elimination tournaments

---

## Game Mechanics

### Match Types

| Type | Enum Value | Description | Lock Behavior |
|------|-----------|-------------|---------------|
| Casual | `regular` | Instant 1v1 matches | One active casual match per chat |
| Ranked | `ranked` | Daily tournaments with brackets | Separate lock from casual |

**Lock System**: Users cannot create a new casual match if they have an active casual match in `open`, `shop_phase`, or `battle_phase` status. Ranked matches use a separate lock system. *(Configured in [arena_service.go:314-328](../services/arena_service.go#L314-L328))*

### Match Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                         MATCH LIFECYCLE                          │
└─────────────────────────────────────────────────────────────────┘

 1. OPEN (5 min)          2. SHOP PHASE (3 min)       3. BATTLE         4. COMPLETED
┌──────────────┐        ┌──────────────────┐       ┌──────────┐       ┌───────────┐
│ Players join │   →    │ Build team of 3  │   →   │ Auto-sim │   →   │ Final     │
│ (min 2)      │        │ - Buy cards      │       │ battle   │       │ results   │
└──────────────┘        │ - Reroll shop    │       └──────────┘       └───────────┘
                        │ - Upgrade cards  │
                        │ - Set order      │
                        └──────────────────┘
```

#### Phase 1: Open (MatchStatusOpen)
- **Duration**: 5 minutes *(JoinWindowDuration in [arena_service.go:25](../services/arena_service.go#L25))*
- **Actions**:
  - Players join the match
  - Creator can start early if 2+ players
- **Transitions**:
  - Auto-starts when deadline expires (if 2+ players)
  - Cancels if only 1 player at deadline
  - Manual start by creator

#### Phase 2: Shop Phase (MatchStatusShopPhase)
- **Duration**: 3 minutes *(ShopPhaseDuration in [arena_service.go:24](../services/arena_service.go#L24))*
- **Features**:
  - After team submission, GetShop returns read-only state with `team_submitted: true` and `cards: null`
  - Frontend can continue polling to detect when battle starts
  - No errors returned for post-submission requests
- **Actions**:
  - **Reroll shop** (only before first card purchase) - Replace unpurchased cards with new ones
  - Buy cards from shop (4 cards available)
  - Upgrade purchased cards (ATK or HP)
  - Set team battle order
  - Submit team when ready
- **Transitions**:
  - All teams submitted → Battle starts immediately
  - Deadline expires → Auto-submit remaining teams

#### Phase 3: Battle Phase (MatchStatusBattlePhase)
- **Duration**: Instant (deterministic simulation)
- **Process**:
  - Backend simulates turn-based combat
  - Generates battle events/replay
  - Determines winner
- **Transitions**: Automatically moves to Completed

#### Phase 4: Completed/Cancelled
- **Status**: `completed` or `cancelled`
- **Data Available**:
  - Battle results
  - Winner/loser
  - Full event log
  - Updated leaderboard stats

---

### Shop Phase

#### Economy System

| Resource | Starting Amount | Description |
|----------|----------------|-------------|
| Coins | 10 | Currency for purchases |
| Shop Cards | 4 | Random cards from current week |
| Team Slots | 3 | Cards needed for battle |

*Constants defined in [shop/types.go:10-18](shop/types.go#L10-L18)*

#### Shop Actions & Costs

| Action | Cost | Constraint | Description |
|--------|------|------------|-------------|
| **Buy Card** | 3 coins | Team not full (< 3) | Purchase card from shop slot |
| **Reroll Shop** | 1 coin | No cards purchased yet | Replace unpurchased cards with new ones (only before first purchase) |
| **Upgrade ATK** | 1 coin | Card in team | +1 ATK per upgrade |
| **Upgrade HP** | 1 coin | Card in team | +3 HP per upgrade |

**Important**:
- **Reroll** is only available **before any card is purchased**. Once you buy your first card, the reroll button becomes permanently disabled for that match.
- **Upgrade** actions validate that you'll have enough coins remaining to complete your 3-card team.

*(Logic in [shop/types.go:48-82](shop/types.go#L48-L82))*

#### Shop Card Selection
- Cards are randomly selected from the current week's `ml_user_cards` for the chat
- Same card can appear multiple times in the shop
- Rerolls exclude already-purchased cards
- Each card shows:
  - User photo (from `user_profile_photos`)
  - Card image (from card renderer service)
  - Base stats: ATK, DEF, HP

*Card dealing logic in [shop/dealer.go:53-146](shop/dealer.go#L53-L146)*

#### Building Your Team

**Step-by-step**:
1. **Review initial shop cards**
   - You start with 4 random cards
2. **Optional: Reroll** (only before first purchase)
   - Costs 1 coin
   - Replaces unpurchased cards with new ones
   - **Once you buy your first card, reroll becomes permanently disabled**
3. **Buy 3 cards** from shop (required)
   - Each purchase costs 3 coins
   - Choose wisely based on stats
   - This is your only chance to reroll - decide before buying!
4. **Optional: Upgrade** purchased cards
   - Choose ATK for more damage
   - Choose HP for more survivability
5. **Set battle order** (positions 0, 1, 2)
   - Position 0 = front (battles first)
   - Position 2 = back (battles last)
6. **Submit team** to proceed to battle

---

### Battle Phase

#### Combat Rules (Super Auto Pets style)

The battle engine simulates deterministic turn-based combat:

```
┌─────────────────────────────────────────────────────────────────┐
│                      COMBAT SIMULATION                           │
└─────────────────────────────────────────────────────────────────┘

Round N:
  ┌─────────────┐                    ┌─────────────┐
  │   Team A    │                    │   Team B    │
  │  Front Card │ ←───────────────→  │ Front Card  │
  └─────────────┘    Simultaneous    └─────────────┘
                        Attacks

  1. Both front cards attack simultaneously
  2. Damage = attacker's ATK
  3. Defender takes full damage (DEF is cosmetic)
  4. Card dies if HP <= 0
  5. Next card advances to front
  6. Repeat until one/both teams empty
```

**Key Mechanics**:
- **Simultaneous attacks**: Both cards damage each other at the same time
- **No blocking/defense**: ATK value directly reduces opponent HP
- **DEF stat**: Display-only, doesn't affect combat *(Inherited from ML card stats)*
- **Sequential fighters**: Cards battle front-to-front, survivors advance
- **Max rounds**: 100 (prevents infinite loops) *(MaxRounds in [battle/engine.go:10](battle/engine.go#L10))*

#### Win Conditions

1. **Team elimination**: Opponent has no cards left with HP > 0
2. **Damage tiebreaker**: If both teams eliminated in same round, most total damage wins
3. **True draw**: Equal damage dealt by both teams

*Full combat logic in [battle/engine.go:88-403](battle/engine.go#L88-L403)*

#### Battle Events

The backend generates a detailed event log for replays:

| Event Type | Description | Data Included |
|------------|-------------|---------------|
| `attack` | Card attacks opponent | Attacker, defender, damage, HP before/after |
| `death` | Card dies | Card ID, killer card |
| `summary` | Duel summary when card dies | Rounds in duel, damage dealt/taken, kill streak |
| `advance` | Cards shift forward | Team states |
| `victory` | Match winner declared | Winner team |

Each event includes a `card_states` snapshot showing all cards' current HP, position, and alive status for animation purposes.

*Event types defined in [battle/types.go:6-15](battle/types.go#L6-L15)*

#### Kill Streaks
- Tracked per card during battle
- Increments when defeating opponent
- Resets when card dies
- Displayed in summary events (🔥 x2, 🔥 x3, etc.)

---

## API Endpoints

### Authentication

All Mini App endpoints require JWT authentication with chat context:

```http
Authorization: Bearer <jwt_token>
```

JWT claims must include:
- `user_id`: Telegram user ID
- `chat_id`: Telegram chat ID (for access control)

### Match Management

#### List Active Matches
```http
GET /api/v1/mini-app/arena/matches?chat_id=<chat_id>
```

**Response**:
```json
{
  "matches": [
    {
      "id": "01J9XYZ...",
      "chat_id": -1001234567890,
      "match_type": "regular",
      "status": "shop_phase",
      "created_at": "2024-01-15T10:00:00Z",
      "join_deadline": "2024-01-15T10:05:00Z",
      "shop_phase_deadline": "2024-01-15T10:08:00Z",
      "participants": [
        {
          "user_id": 123456,
          "first_name": "John",
          "status": "ready"
        }
      ]
    }
  ]
}
```

*Handler: [arena_handler.go:269-293](../handlers/arena_handler.go#L269-L293)*

#### Create Match
```http
POST /api/v1/mini-app/arena/match
Content-Type: application/json

{
  "chat_id": -1001234567890
}
```

**Response**: Match object (201 Created)

**Error Cases**:
- `400 Bad Request`: Not enough cards available (minimum 10 cards required)
- `400 Bad Request`: Active match already exists (see Lock System)

*Handler: [arena_handler.go:295-340](../handlers/arena_handler.go#L295-L340)*

#### Get Match Details
```http
GET /api/v1/mini-app/arena/match/<match_id>
```

**Response**: Full match object with participants, timestamps, and status

*Handler: [arena_handler.go:342-379](../handlers/arena_handler.go#L342-L379)*

#### Join Match
```http
POST /api/v1/mini-app/arena/match/<match_id>/join
```

**Requirements**:
- Match must be in `open` status
- User not already joined

**Response**: Updated match object

*Handler: [arena_handler.go:381-408](../handlers/arena_handler.go#L381-L408)*

#### Leave Match
```http
POST /api/v1/mini-app/arena/match/<match_id>/leave
```

**Requirements**:
- Match must be in `open` status (cannot leave after start)

**Response**: `200 OK`

*Handler: [arena_handler.go:410-437](../handlers/arena_handler.go#L410-L437)*

#### Start Match (Creator Only)
```http
POST /api/v1/mini-app/arena/match/<match_id>/start
```

**Requirements**:
- Only creator can start
- Match must be in `open` status
- Minimum 2 participants

**Response**: Updated match object in `shop_phase` status

*Handler: [arena_handler.go:439-466](../handlers/arena_handler.go#L439-L466)*

---

### Shop Actions

#### Get Shop State
```http
GET /api/v1/mini-app/arena/match/<match_id>/shop
```

**Response**:
```json
{
  "match_id": "01J9XYZ...",
  "status": "shop_phase",
  "coins": 10,
  "cards": [
    {
      "card_id": 789,
      "user_id": 111,
      "name": "Alice",
      "username": "alice",
      "photo_url": "https://...",
      "card_image_url": "https://...",
      "atk": 8,
      "def": 6,
      "hp": 24,
      "stats": {...},
      "is_purchased": false,
      "index": 0
    }
    // ... 3 more cards
  ],
  "team": [],
  "team_order": [0, 1, 2],
  "is_ready": false,
  "team_submitted": false,
  "deadline": "2024-01-15T10:08:00Z",
  "time_remaining_seconds": 180,
  "affordability": {
    "can_buy": true,
    "can_reroll": true,
    "can_upgrade": false,
    "can_submit": false
  }
}
```

**Behavior after Team Submission**:
When you submit your team (`team_submitted: true`), subsequent calls to GetShop return a **read-only state**:
- `cards` array is `null` (no shop interface)
- `team` shows your submitted team
- All affordability flags are `false` with reason "team already submitted"
- You can continue polling - no errors returned
- Useful for detecting when match transitions to `battle_phase`

**Phase Transitions**:
- If match transitions to `battle_phase` or `completed`, returns graceful read-only state
- Use `status` field to detect phase changes
- Safe to continue polling during transition

*Handler: [arena_handler.go:468-496](../handlers/arena_handler.go#L468-L496)*

#### Buy Card
```http
POST /api/v1/mini-app/arena/match/<match_id>/buy
Content-Type: application/json

{
  "card_index": 0
}
```

**Requirements**:
- Match in `shop_phase`
- Have 3+ coins
- Team not full (< 3 cards)
- Card at index not already purchased

**Response**: Updated shop state with card added to team

*Handler: [arena_handler.go:497-531](../handlers/arena_handler.go#L497-L531)*

#### Reroll Shop
```http
POST /api/v1/mini-app/arena/match/<match_id>/reroll
```

**Requirements**:
- Have 1+ coins
- **No cards purchased yet** (team must be empty)
- Once you buy your first card, reroll is permanently disabled for that match

**Response**: Updated shop state with new unpurchased cards

**Error**: Returns `400 Bad Request` if you've already purchased a card with message "cannot reroll after purchasing cards"

*Handler: [arena_handler.go:533-560](../handlers/arena_handler.go#L533-L560)*

#### Upgrade Card
```http
POST /api/v1/mini-app/arena/match/<match_id>/upgrade
Content-Type: application/json

{
  "team_slot": 0,
  "upgrade_type": "atk"
}
```

**Parameters**:
- `team_slot`: 0-2 (card index in your team)
- `upgrade_type`: `"atk"` or `"hp"`

**Effects**:
- ATK upgrade: +1 ATK
- HP upgrade: +3 HP and +3 MaxHP

**Requirements**:
- Have enough coins for: upgrade (2) + remaining team slots (2 each)

**Response**: Updated shop state with card stats modified

*Handler: [arena_handler.go:562-602](../handlers/arena_handler.go#L562-L602)*

#### Set Team Order
```http
POST /api/v1/mini-app/arena/match/<match_id>/order
Content-Type: application/json

{
  "order": [2, 0, 1]
}
```

**Parameters**:
- `order`: Array of 3 unique indices (0-2) in battle order
- First index = front card, last = back card

**Example**: `[2, 0, 1]` means:
- Team slot 2 battles first (front)
- Team slot 0 battles second (middle)
- Team slot 1 battles last (back)

**Response**: Updated shop state

*Handler: [arena_handler.go:604-638](../handlers/arena_handler.go#L604-L638)*

#### Submit Team
```http
POST /api/v1/mini-app/arena/match/<match_id>/team
```

**Requirements**:
- Team must have exactly 3 cards

**Response**: Updated shop state with `is_ready: true`

**Effect**:
- Marks you as ready
- When all participants ready → battle starts immediately
- Otherwise → battle starts at shop deadline

*Handler: [arena_handler.go:640-667](../handlers/arena_handler.go#L640-L667)*

---

### Battle & Results

#### Get Battle Results
```http
GET /api/v1/mini-app/arena/match/<match_id>/battle
```

**Available**: Only after match transitions to `battle_phase`

**Response**:
```json
{
  "winner_id": 123456,
  "is_draw": false,
  "events": [
    {
      "type": "attack",
      "round": 1,
      "attacker_card_id": 789,
      "defender_card_id": 790,
      "attacker_team_owner_id": 123456,
      "defender_team_owner_id": 654321,
      "damage": 8,
      "hp_before": 24,
      "hp_after": 16,
      "message": "🗡️ Alice attacks Bob (8 ATK) → [████████░░] 16 HP",
      "card_states": [
        {
          "card_id": 789,
          "user_id": 111,
          "name": "Alice",
          "hp": 18,
          "max_hp": 24,
          "atk": 8,
          "position": 0,
          "is_alive": true,
          "is_attacking": true,
          "is_defending": false
        }
        // ... all other cards
      ]
    },
    {
      "type": "death",
      "round": 3,
      "defender_card_id": 790,
      "message": "💀 Alice defeats Bob",
      "card_states": [...]
    },
    {
      "type": "summary",
      "round": 3,
      "is_summary": true,
      "killer_card_id": 789,
      "total_damage_dealt": 24,
      "total_damage_taken": 12,
      "rounds_in_duel": 3,
      "kill_streak": 1,
      "message": "⚔️ Alice defeats Bob | 3 rounds | 24 dealt / 12 taken | 12❤️",
      "card_states": [...]
    },
    {
      "type": "victory",
      "round": 8,
      "message": "🏆 Team Alice wins!",
      "card_states": [...]
    }
  ],
  "num_rounds": 8,
  "team_a_damage": 72,
  "team_b_damage": 54,
  "team_a_final": {
    "owner_id": 123456,
    "owner_name": "Alice",
    "cards": [...]
  },
  "team_b_final": {
    "owner_id": 654321,
    "owner_name": "Bob",
    "cards": [...]
  }
}
```

**Event Types**:
- `attack`: Card deals damage
- `damage`: (deprecated, same as attack)
- `death`: Card eliminated
- `summary`: Duel statistics when card dies
- `advance`: Cards shift positions
- `victory`: Match winner declared

**Card States**: Each event includes a snapshot of all cards for UI animation:
- Current HP/MaxHP
- Position (0=front, 1=mid, 2=back)
- Alive status
- Who's attacking/defending this turn

*Handler: [arena_handler.go:669-696](../handlers/arena_handler.go#L669-L696)*

---

### Leaderboards & Stats

#### Ranking System

The leaderboard uses different ranking algorithms depending on match type:

**Ranked Matches** (Tournaments):
- Simple ordering by `tournaments_won` (highest first)
- Tiebreakers: `wins DESC`, `losses ASC`
- `score` field = tournament wins count

**Casual Matches** (Regular):
- Uses **Wilson Score** for confidence-based ranking
- Naturally handles sample size - a player with 3/3 wins (100%) won't outrank someone with 50/60 wins (83%) because the confidence interval is wider with fewer games
- **Draws are excluded** from calculation (count as 0)
- `score` field = Wilson Score (0.0 - 1.0 range)
- Tiebreakers: `wins DESC`, `losses ASC`

**Wilson Score Formula** (95% confidence interval lower bound):
```
n = wins + losses
p = wins / n
z = 1.96 (95% confidence)

score = ((p + z²/2n) - z * sqrt((p(1-p) + z²/4n) / n)) / (1 + z²/n)
```

*Wilson Score SQL function: [migrations/sql/014_wilson_score.sql](../migrations/sql/014_wilson_score.sql)*

#### Get Leaderboard
```http
GET /api/v1/mini-app/arena/leaderboard?chat_id=<id>&type=ranked&limit=50&offset=0
```

**Parameters**:
- `type`: `"ranked"` or `"regular"` (default: `ranked`)
- `limit`: 1-100 (default: 50)
- `offset`: Pagination offset

**Response**:
```json
{
  "type": "ranked",
  "total": 45,
  "page": 0,
  "limit": 50,
  "has_more": false,
  "entries": [
    {
      "user_id": 123456,
      "chat_id": -1001234567890,
      "first_name": "Alice",
      "username": "alice",
      "photo_url": "https://...",
      "rank": 1,
      "score": 3.0,
      "ranked_wins": 15,
      "ranked_losses": 3,
      "ranked_draws": 0,
      "ranked_tournaments_played": 12,
      "ranked_tournaments_won": 3,
      "ranked_current_streak": 5,
      "ranked_best_streak": 8,
      "regular_wins": 42,
      "regular_losses": 18,
      "regular_draws": 2,
      "regular_matches_played": 62,
      "regular_current_streak": 3,
      "regular_best_streak": 12,
      "first_match_at": "2024-01-01T12:00:00Z",
      "last_match_at": "2024-01-15T10:30:00Z"
    }
    // ... more entries
  ]
}
```

**Ranking Fields**:
| Field | Description |
|-------|-------------|
| `rank` | 1-indexed position in leaderboard (calculated via `ROW_NUMBER()`) |
| `score` | Ranking score: Wilson Score (0-1) for regular, tournaments_won for ranked |
| `total` | Total number of entries in leaderboard |
| `page` | Current page number (0-indexed, calculated as `offset / limit`) |
| `has_more` | Whether there are more entries beyond current page |

*Handler: [match_handler.go:251-290](../handlers/match_handler.go#L251-L290)*
*Query: [match_repo.go:272-348](../repository/match_repo.go#L272-L348)*

#### Get Match History
```http
GET /api/v1/mini-app/arena/history?chat_id=<id>&limit=20&offset=0
```

**Response**:
```json
{
  "matches": [
    {
      "match_id": "01J9XYZ...",
      "match_type": "regular",
      "your_photo_url": "https://...",
      "opponent": {
        "user_id": 654321,
        "first_name": "Bob",
        "username": "bob",
        "photo_url": "https://..."
      },
      "result": "win",
      "your_team": [...],
      "opponent_team": [...],
      "completed_at": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 45,
  "has_more": true
}
```

**Result values**: `"win"`, `"loss"`, `"draw"`

*Handler: [arena_handler.go:735-812](../handlers/arena_handler.go#L735-L812)*

#### Get Head-to-Head Record
```http
GET /api/v1/mini-app/arena/h2h?chat_id=<id>&opponent_id=<user_id>
```

**Response**:
```json
{
  "record": {
    "opponent": {
      "user_id": 654321,
      "first_name": "Bob",
      "username": "bob"
    },
    "wins": 8,
    "losses": 3,
    "last_match_at": "2024-01-15T10:30:00Z"
  },
  "recent_matches": [
    {
      "match_id": "01J9XYZ...",
      "match_type": "regular",
      "result": "win",
      "your_team": [...],
      "opponent_team": [...],
      "completed_at": "2024-01-15T10:30:00Z"
    }
    // ... up to 10 recent matches
  ]
}
```

*Handler: [arena_handler.go:814-919](../handlers/arena_handler.go#L814-L919)*

#### Get User Profile
```http
GET /api/v1/mini-app/arena/profile?chat_id=<id>
```

**Response**:
```json
{
  "profile": {
    "user_id": 123456,
    "first_name": "Alice",
    "username": "alice",
    "ranked_wins": 15,
    "ranked_losses": 3,
    "ranked_tournaments_played": 12,
    "ranked_tournaments_won": 3,
    "ranked_current_streak": 5,
    "ranked_best_streak": 8,
    "ranked_rank": 1,
    "regular_wins": 42,
    "regular_losses": 18,
    "regular_matches_played": 60,
    "regular_current_streak": 3,
    "regular_best_streak": 12,
    "regular_rank": 5,
    "first_match_at": "2024-01-01T12:00:00Z",
    "last_match_at": "2024-01-15T10:30:00Z"
  },
  "recent_matches": [
    // ... last 5 matches
  ]
}
```

*Handler: [arena_handler.go:921-1044](../handlers/arena_handler.go#L921-L1044)*

---

### Ranked Tournaments

Ranked tournaments are daily scheduled events that run automatically via bot endpoints.

#### Tournament Lifecycle

```
1. ANNOUNCEMENT (12 hours before)
   ↓
2. REGISTRATION (open until tournament time)
   ↓
3. CLOSED (registration locked)
   ↓
4. IN_PROGRESS (bracket rounds)
   ↓
5. COMPLETED
```

#### For Frontend: Read-Only Access

Frontend users interact with tournaments through:
- Leaderboard with `type=ranked` filter
- Match history showing tournament matches
- Profile stats showing tournament wins

**Bot handles**:
- Announcing tournaments
- Closing registration
- Creating bracket matches
- Progressing rounds

*Tournament handlers: [arena_handler.go:1396-1780](../handlers/arena_handler.go#L1396-L1780) (Bot API Key auth only)*

---

### Game Constants

```http
GET /api/v1/mini-app/arena/constants
```

**Response**:
```json
{
  "costs": {
    "card": 3,
    "reroll": 1,
    "upgrade": 1
  },
  "sizes": {
    "shop": 4,
    "team": 3
  },
  "upgrades": {
    "atk_amount": 3,
    "hp_amount": 3
  },
  "timings": {
    "shop_phase_duration": 180,
    "join_window_duration": 300
  },
  "hp_bar_thresholds": {
    "high": 66,
    "medium": 33,
    "colors": {
      "high": "#22c55e",
      "medium": "#eab308",
      "low": "#ef4444"
    }
  },
  "timer_thresholds": {
    "safe": 120,
    "warning": 30,
    "colors": {
      "safe": "#22c55e",
      "warning": "#eab308",
      "urgent": "#ef4444"
    }
  }
}
```

*Handler: [arena_handler.go:291-340](../handlers/arena_handler.go#L291-L340)*

---

## Data Models

### Match
```typescript
interface Match {
  id: string;                    // ULID
  chat_id: number;               // Telegram chat ID
  match_type: "regular" | "ranked";
  format?: MatchFormat;          // For ranked tournaments
  status: "open" | "shop_phase" | "battle_phase" | "completed" | "cancelled";
  created_at: string;            // ISO 8601
  join_deadline?: string;
  shop_phase_started_at?: string;
  shop_phase_deadline?: string;
  battle_started_at?: string;
  completed_at?: string;
  tournament_date?: string;      // YYYY-MM-DD for ranked
  creator_user_id?: number;
  current_round: number;         // For tournaments
  winner_user_id?: number;
  participants: Participant[];
}
```

### Participant
```typescript
interface Participant {
  id: number;
  match_id: string;
  user_id: number;
  first_name: string;
  username?: string;
  status: "joined" | "ready" | "eliminated";
  joined_at: string;
  coins_remaining: number;
  shop_cards?: ShopCard[];
  team?: Card[];
  team_order: number[];          // [0, 1, 2] = battle order
  team_submitted_at?: string;
  placement?: number;            // 1 = winner, 2 = eliminated
}
```

### Card
```typescript
interface Card {
  card_id: number;               // ml_user_cards.id
  user_id: number;
  name: string;                  // first_name
  username?: string;
  photo_url?: string;            // User profile photo
  atk: number;
  def: number;                   // Cosmetic only
  hp: number;
  max_hp: number;
  atk_upgrades: number;          // Count of ATK upgrades applied
  hp_upgrades: number;           // Count of HP upgrades applied
  position: number;              // 0=front, 1=mid, 2=back
}
```

### ShopCard
```typescript
interface ShopCard {
  card_id: number;
  user_id: number;
  name: string;
  username?: string;
  photo_url?: string;            // For battle phase
  card_image_url?: string;       // For shop phase (from card-renderer)
  atk: number;
  def: number;
  hp: number;
  stats?: object;                // Full ML stats JSON
  is_purchased: boolean;
  index: number;                 // 0-3 (shop position)
}
```

### EnhancedShopResponse
```typescript
interface EnhancedShopResponse {
  match_id: string;
  status: string;                 // "open" | "shop_phase" | "battle_phase" | "completed"
  coins: number;
  cards: EnhancedShopCard[] | null;  // null after team submission or phase transition
  team: EnhancedTeamCard[];        // Length 0-3
  team_order: number[];           // [0, 1, 2]
  is_ready: boolean;
  team_submitted: boolean;        // true after calling submit endpoint
  deadline?: string;              // ISO 8601 deadline
  time_remaining_seconds: number; // Seconds until deadline
  affordability: ShopAffordability;
}

### ShopAffordability
```typescript
interface ShopAffordability {
  can_buy: boolean;
  can_reroll: boolean;
  can_upgrade: boolean;
  can_submit: boolean;
  buy_disabled_reason?: string;
  reroll_disabled_reason?: string;
  upgrade_disabled_reason?: string;
  submit_disabled_reason?: string;
}
```

### EnhancedShopCard (extends ShopCard)
```typescript
interface EnhancedShopCard extends ShopCard {
  // All ShopCard fields plus:
  can_afford: boolean;           // Whether you have 3+ coins to buy
  preview_cost: number;          // Cost of this specific action
}
```

### EnhancedTeamCard (extends Card)
```typescript
interface EnhancedTeamCard extends Card {
  // All Card fields plus:
  can_upgrade_atk: boolean;              // Whether ATK upgrade is affordable
  can_upgrade_hp: boolean;               // Whether HP upgrade is affordable
  upgrade_atk_disabled_reason?: string;  // Reason if ATK upgrade not available
  upgrade_hp_disabled_reason?: string;   // Reason if HP upgrade not available
  atk_if_upgraded: number;               // ATK value if upgraded once more
  hp_if_upgraded: number;                // HP value if upgraded once more
  max_hp_if_upgraded: number;            // MaxHP value if upgraded once more
}
```

### LeaderboardEntry
```typescript
interface LeaderboardEntry {
  user_id: number;
  chat_id: number;
  first_name: string;
  username?: string;
  photo_url?: string;

  // Ranking (calculated per query)
  rank: number;                      // 1-indexed position
  score: number;                     // Wilson Score (0-1) for regular, tournaments_won for ranked

  // Ranked match stats
  ranked_wins: number;
  ranked_losses: number;
  ranked_draws: number;
  ranked_tournaments_played: number;
  ranked_tournaments_won: number;
  ranked_current_streak: number;
  ranked_best_streak: number;

  // Casual match stats
  regular_wins: number;
  regular_losses: number;
  regular_draws: number;
  regular_matches_played: number;
  regular_current_streak: number;
  regular_best_streak: number;

  // Metadata
  head_to_head?: Record<string, any>;  // H2H records vs other users
  first_match_at?: string;             // ISO 8601
  last_match_at?: string;              // ISO 8601
}
```

### LeaderboardResponse
```typescript
interface LeaderboardResponse {
  type: "ranked" | "regular";
  entries: LeaderboardEntry[];
  total: number;                       // Total entries for pagination
  page: number;                        // Current page (offset / limit)
  limit: number;                       // Page size
  has_more: boolean;                   // Whether more entries exist
}
```

### BattleResult
```typescript
interface BattleResult {
  match_id: string;
  winner_id?: number;            // null if draw
  is_draw: boolean;
  events: BattleEvent[];
  num_rounds: number;
  team_a_damage: number;         // Total damage dealt by team A (absolute)
  team_b_damage: number;         // Total damage dealt by team B (absolute)
  damage_dealt: number;          // Player-relative: requesting user's damage
  damage_taken: number;          // Player-relative: damage received by user
  team_a_final: Team;
  team_b_final: Team;
  player_a_id: number;           // Player A's user ID
  player_b_id: number;           // Player B's user ID
  player_a_name: string;         // Player A's display name
  player_b_name: string;         // Player B's display name
}
```

**Player-Relative Damage**: The `damage_dealt` and `damage_taken` fields are calculated from the requesting user's perspective:
- If user is Player A: `damage_dealt = team_a_damage`, `damage_taken = team_b_damage`
- If user is Player B: `damage_dealt = team_b_damage`, `damage_taken = team_a_damage`

### BattleEvent
```typescript
interface BattleEvent {
  type: "attack" | "death" | "summary" | "advance" | "victory";
  round: number;
  message?: string;              // Human-readable description

  // Attack/Death event fields
  attacker_card_id?: number;
  defender_card_id?: number;
  attacker_team_owner_id?: number;
  defender_team_owner_id?: number;
  damage?: number;
  hp_before?: number;
  hp_after?: number;

  // Summary event fields
  is_summary?: boolean;
  killer_card_id?: number;
  total_damage_dealt?: number;
  total_damage_taken?: number;
  rounds_in_duel?: number;
  kill_streak?: number;

  // Card states for animation (all events)
  card_states?: CardSnapshot[];
}
```

### CardSnapshot
```typescript
interface CardSnapshot {
  card_id: number;
  user_id: number;
  name: string;
  hp: number;
  max_hp: number;
  atk: number;
  position: number;              // 0=front, 1=mid, 2=back
  is_alive: boolean;
  is_attacking: boolean;         // True if this card is attacking this turn
  is_defending: boolean;         // True if this card is defending this turn
}
```

---

## Game Constants

### Economy
- **Starting Coins**: 10
- **Card Cost**: 3 coins
- **Reroll Cost**: 1 coin
- **Upgrade Cost**: 1 coin

### Shop
- **Shop Size**: 4 cards
- **Team Size**: 3 cards (required)

### Upgrades
- **ATK Upgrade**: +1 ATK per upgrade
- **HP Upgrade**: +3 HP and +3 MaxHP per upgrade

### Timings
- **Join Window**: 5 minutes (300 seconds)
- **Shop Phase**: 3 minutes (180 seconds)
- **Battle**: Instant (deterministic)

### Combat
- **Max Rounds**: 100 (prevents infinite loops)
- **DEF Stat**: Display-only, doesn't affect damage
- **Simultaneous Combat**: Both front cards attack each turn

*All constants defined in:*
- Shop: [shop/types.go:10-18](shop/types.go#L10-L18)
- Timings: [arena_service.go:24-25](../services/arena_service.go#L24-L25)
- Battle: [battle/engine.go:10](battle/engine.go#L10)

---

## Frontend Integration Guide

### Recommended Flow

#### 1. Match Discovery
```typescript
// Check for active matches in this chat
const { matches } = await GET('/api/v1/mini-app/arena/matches?chat_id={chatId}');

if (matches.length === 0) {
  // No active match - show "Create Match" button
} else {
  const match = matches[0];
  // Route to appropriate phase UI based on match.status
}
```

#### 2. Creating/Joining Match
```typescript
// Create new match
try {
  const match = await POST('/api/v1/mini-app/arena/match', { chat_id: chatId });
  // Automatically joined as creator
} catch (error) {
  if (error.status === 400) {
    // Handle: "active match exists" or "not enough cards"
  }
}

// Join existing match
const match = await POST(`/api/v1/mini-app/arena/match/${matchId}/join`);
```

#### 3. Open Phase UI
```typescript
// Poll for updates while match.status === 'open'
setInterval(async () => {
  const match = await GET(`/api/v1/mini-app/arena/match/${matchId}`);

  if (match.status !== 'open') {
    // Transitioned to shop_phase
    navigateToShop();
  }

  updateParticipantList(match.participants);
  updateCountdown(match.join_deadline);
}, 2000); // Poll every 2 seconds
```

#### 4. Shop Phase UI
```typescript
// Load shop state
const shop = await GET(`/api/v1/mini-app/arena/match/${matchId}/shop`);

// Display 4 shop cards + purchased team (or read-only team if submitted)
if (shop.cards) {
  renderShop(shop.cards, shop.team);  // Interactive mode
} else {
  renderReadOnlyTeam(shop.team);      // Submitted state
}
updateCoins(shop.coins);

// Buy card
const updatedShop = await POST(`/api/v1/mini-app/arena/match/${matchId}/buy`, {
  card_index: 0
});

// Reroll shop (only available before first purchase)
if (shop.team.length === 0 && shop.coins >= 1) {
  const updatedShop = await POST(`/api/v1/mini-app/arena/match/${matchId}/reroll`);
}

// Upgrade card
const updatedShop = await POST(`/api/v1/mini-app/arena/match/${matchId}/upgrade`, {
  team_slot: 0,
  upgrade_type: "atk"
});

// Set battle order (drag & drop)
const updatedShop = await POST(`/api/v1/mini-app/arena/match/${matchId}/order`, {
  order: [2, 0, 1] // Rearranged indices
});

// Submit team (requires 3 cards)
if (shop.team.length === 3) {
  const updatedShop = await POST(`/api/v1/mini-app/arena/match/${matchId}/team`);
  // updatedShop.team_submitted === true
}

// Poll for battle start - CONTINUE POLLING EVEN AFTER TEAM SUBMISSION
// This allows the UI to respond immediately when battle starts
setInterval(async () => {
  const shop = await GET(`/api/v1/mini-app/arena/match/${matchId}/shop`);

  // Only stop polling when match exits shop phase entirely
  if (shop.status === 'battle_phase') {
    navigateToBattle();
    clearInterval();
  }
  // If team_submitted, render read-only state while waiting
  if (shop.team_submitted && !shop.cards) {
    renderReadOnlyTeam(shop.team);
  }
}, 3000); // Poll every 3s (reduced from 2s for efficiency)
```

**Important**: Continue polling GetShop even after team submission. The endpoint handles all phases gracefully:
- Returns `team_submitted: true` and `cards: null` while waiting
- No errors on phase transitions
- Safe for frontend to poll continuously during shop phase

#### 5. Battle Phase UI
```typescript
// Load battle results
const battle = await GET(`/api/v1/mini-app/arena/match/${matchId}/battle`);

// Animate events sequentially
for (const event of battle.events) {
  switch (event.type) {
    case 'attack':
      await animateAttack(
        event.attacker_card_id,
        event.defender_card_id,
        event.damage,
        event.card_states
      );
      break;

    case 'death':
      await animateDeath(event.defender_card_id, event.card_states);
      break;

    case 'summary':
      await showDuelSummary({
        killer: event.killer_card_id,
        rounds: event.rounds_in_duel,
        damageDealt: event.total_damage_dealt,
        damageTaken: event.total_damage_taken,
        streak: event.kill_streak
      });
      break;

    case 'advance':
      await animateAdvance(event.card_states);
      break;

    case 'victory':
      await showVictoryScreen(battle.winner_id, event.message);
      break;
  }

  await sleep(500); // Delay between events
}
```

### Validation Helpers

```typescript
// Check if can buy card
function canBuyCard(shop: ShopState, cardIndex: number): boolean {
  return shop.coins >= 3 &&
         shop.team.length < 3 &&
         !shop.cards[cardIndex].is_purchased;
}

// Check if can reroll
function canReroll(shop: ShopState): boolean {
  // Reroll only available before any card is purchased
  if (shop.team.length > 0) {
    return false;
  }
  return shop.coins >= 1;
}

// Check if can upgrade
function canUpgrade(shop: ShopState, teamSlot: number): boolean {
  const remainingCards = 3 - shop.team.length;
  const coinsNeeded = 1 + (remainingCards * 3);  // 1 for upgrade + 3 per remaining card
  return shop.coins >= coinsNeeded && teamSlot < shop.team.length;
}

// Check if can submit
function canSubmit(shop: ShopState): boolean {
  return shop.team.length === 3;
}
```

### Error Handling

All endpoints return errors in this format:
```json
{
  "error": "human-readable error message"
}
```

Common error scenarios:
- **401 Unauthorized**: Invalid/missing JWT token
- **403 Forbidden**: Access denied (wrong chat, not participant, not creator)
- **404 Not Found**: Match/resource not found
- **400 Bad Request**: Validation failed (see error message)

### Real-time Updates

Since this is a REST API, implement polling for live updates:

**During open phase**:
- Poll `/api/v1/mini-app/arena/match/{id}` every 2 seconds
- Watch for `status` changes and participant updates

**During shop phase**:
- Poll `/api/v1/mini-app/arena/match/{id}/shop` every 3 seconds
- Continue polling even after team submission (endpoint returns read-only state)
- Watch for `status` change to `battle_phase` to trigger battle view
- Endpoint gracefully handles all phase transitions without errors

**After team submission**:
- GetShop returns `team_submitted: true` and `cards: null`
- Affordability flags all false with reason "team already submitted"
- Continue polling to detect when other players finish and battle starts
- No need to poll match endpoint simultaneously

**During battle**:
- No polling needed - battle results are static

### Performance Tips

1. **Cache constants**: Call `/api/v1/mini-app/arena/constants` once on app load
2. **Lazy load photos**: Card images can be large, load progressively
3. **Debounce actions**: Prevent double-clicks on buy/upgrade buttons
4. **Optimize battle animations**: Use CSS transforms for smooth movement
5. **Preload card states**: Parse all event card_states upfront for faster animation

---

## Code References

### Core Game Logic
- **Battle Engine**: [battle/engine.go:88-403](battle/engine.go#L88-L403) - SAP-style combat simulation
- **Shop State**: [shop/types.go:22-224](shop/types.go#L22-L224) - Economy & team building logic
- **Card Dealer**: [shop/dealer.go:53-245](shop/dealer.go#L53-L245) - Random card selection

### API Layer
- **Match Handlers**: [arena_handler.go:269-466](../handlers/arena_handler.go#L269-L466) - Match lifecycle endpoints
- **Shop Handlers**: [arena_handler.go:468-667](../handlers/arena_handler.go#L468-L667) - Shop & team building
- **Battle Handlers**: [arena_handler.go:669-696](../handlers/arena_handler.go#L669-L696) - Battle results
- **Stats Handlers**: [arena_handler.go:698-1044](../handlers/arena_handler.go#L698-L1044) - Leaderboards & profiles

### Configuration
- **Economy Constants**: [shop/types.go:10-18](shop/types.go#L10-L18)
- **Timing Constants**: [arena_service.go:24-25](../services/arena_service.go#L24-L25)
- **Match Lock Logic**: [arena_service.go:189-201](../services/arena_service.go#L189-L201)
- **Combat Constants**: [battle/engine.go:10](battle/engine.go#L10)

### Data Types
- **Battle Types**: [battle/types.go](battle/types.go) - Cards, teams, events, results
- **Shop Types**: [shop/types.go](shop/types.go) - Shop state, upgrades
- **Repository Models**: [repository/game_repo.go:134-202](../repository/game_repo.go#L134-L202) - Match, Participant
- **Leaderboard Entry**: [repository/game_repo.go:249-277](../repository/game_repo.go#L249-L277) - Stats, rank, score

### Database Functions
- **Wilson Score**: [migrations/sql/014_wilson_score.sql](../migrations/sql/014_wilson_score.sql) - Confidence-based ranking for casual matches

---

## Support & Questions

For implementation questions or clarifications:
1. Check the code references above
2. Review the CLAUDE.md project overview at repo root
3. Test endpoints using the Swagger/Postman collection (if available)

**Game Design Philosophy**:
- Simple, fast-paced matches (< 10 minutes total)
- Deterministic combat (no RNG in battle)
- Strategic depth in team building (card selection, upgrades, positioning)
- Casual accessibility (low barrier to entry)
- Competitive ranked mode (daily tournaments)
