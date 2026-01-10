# Beef Arena - Telegram Card Game

## Overview

A simplified Super Auto Pets-style battle game using existing weekly user stats cards. Group-based matchmaking only - you battle with cards of people from your Telegram group.

**Key Decisions:**
- Simple stats combat (ATK/DEF derived from weighted formula)
- Group-based matchmaking only
- Format auto-selected by participant count (2 = 1v1, 3+ = arena)
- Daily ranked tournament (00:01 open, 18:00 close, matches start immediately with 5min intervals)
- Regular casual matches on-demand (5min join window, creator can start early)
- All matches persisted (ranked vs regular distinguished)
- Real cards with usernames and photos
- 10 coins economy, cards cost 2 (4 coins for upgrades/re-rolls)
- Shared card pool (same card can be drafted by multiple players)
- Animated battle replay
- Runs as Telegram Game App (not Mini App)
- Battle engine in Go
- Full card in shop/selection, simplified card during battle

---

## Core Game Loop

```
Player joins match (from group)
    ↓
6 random cards dealt from group members' current week cards
    ↓
Shop Phase: 10 coins
    ├── Buy 3 cards (2 coins each) = 6 coins spent
    └── 4 coins remaining for: Re-rolls (1 each) + Upgrades (2 each)
    ↓
Arrange 3 cards in battle order [Front] [Mid] [Back]
    ↓
Auto-battle (sequential SAP-style) with animated replay
    ↓
Winner determined → Results shown
```

---

## Stat System

### Existing Stats (from ml_user_cards)

| Stat | Range | Description |
|------|-------|-------------|
| Aura | 0-100 | Positivity/sentiment |
| Activity | 0-100 | Message volume & engagement |
| Presence | 0-100 | Consistency/streak |
| Humor | 0-100 | Funny messages & reactions |
| Toxicity | 0-100 | Negative behavior % |
| Popularity | 0-100 | Reactions received |
| Overall | 0-100 | Composite tier score |

### New Combat Stats (ATK, DEF, HP) ✅ Implemented

> **Implementation**: `apps/ml-processor/src/cards/calculators.py:calculate_combat()` (lines 1334-1389)

Cards get combat stats derived from existing stats using weighted formula:

```python
# Calculate raw scores (0-100)
raw_atk = 0.40 * activity + 0.35 * toxicity + 0.25 * humor
raw_def = 0.40 * presence + 0.35 * aura + 0.25 * popularity

# Scale to game values (1-10)
ATK = max(1, round(raw_atk / 10))
DEF = max(1, round(raw_def / 10))

# HP derived from DEF for granular battles
HP = DEF * 3  # Range: 3-30
```

**Stat Thematic Mapping:**
- **ATK contributors**: Activity (volume), Toxicity (aggression), Humor (wit)
- **DEF contributors**: Presence (consistency), Aura (positivity), Popularity (support)

**Example Cards:**
| User | Activity | Toxicity | Humor | Presence | Aura | Pop | → ATK | → DEF | → HP |
|------|----------|----------|-------|----------|------|-----|-------|-------|------|
| João | 80 | 45 | 60 | 40 | 65 | 50 | 6 | 5 | 15 |
| Maria | 30 | 10 | 25 | 90 | 80 | 70 | 2 | 8 | 24 |
| Pedro | 95 | 70 | 40 | 20 | 30 | 25 | 7 | 3 | 9 |

---

## Battle Mechanics

### Sequential Combat (SAP-style)

1. Both teams line up: [Front] [Mid] [Back]
2. Front cards attack each other simultaneously
3. Damage dealt = ATK
4. Card dies when HP ≤ 0
5. If one front card dies, winner stays (with reduced HP)
6. Next card from losing team moves to front
7. Repeat until one team has no cards left

### Tie-breaker
- If both front cards die simultaneously, both are removed
- If both teams empty at same time: draw, or compare total damage dealt

### Example Battle
```
Player A: [João ATK:7 HP:15] [Maria ATK:5 HP:21] [Pedro ATK:8 HP:9]
Player B: [Ana ATK:6 HP:18] [Carlos ATK:9 HP:12] [Bia ATK:4 HP:24]

Round 1: João (7) vs Ana (6)
  João takes 6 dmg → HP: 9
  Ana takes 7 dmg → HP: 11

Round 2: João (7, HP:9) vs Ana (6, HP:11)
  João takes 6 dmg → HP: 3
  Ana takes 7 dmg → HP: 4

Round 3: João (7, HP:3) vs Ana (6, HP:4)
  João takes 6 dmg → HP: -3 (dies)
  Ana takes 7 dmg → HP: -3 (dies)
  Both die!

Round 4: Maria (5, HP:21) vs Carlos (9, HP:12)
  ...continues...
```

---

## Match Types

### Match Format Auto-Selection

The format (1v1 vs Arena) is **automatically determined by participant count**:

| Participants | Format | Structure |
|--------------|--------|-----------|
| 2 | 1v1 | Single match |
| 3-4 | Mini Arena | Round-robin or single elimination |
| 5-8 | Arena | Single elimination bracket |
| 9+ | Arena | Double elimination or Swiss |

---

## Daily Ranked Tournament

One ranked tournament per group per day, automatically scheduled.

### Schedule (Group Timezone)

```
00:01 - Tournament opens, announcement sent to group
        ↓
        Join period (users can join anytime)
        ↓
18:00 - Registration closes
        ↓
18:00 - First round begins immediately (all participants build teams)
        ↓
18:05 - Round 1 results, next round starts
        ↓
18:10 - Round 2 results, next round starts
        ↓
...continues every 5 minutes until tournament complete...
        ↓
Final results announced, rankings updated
```

### Ranked Tournament Rules
- **Timezone**: Uses group's configured timezone (from `chats` table) or fallback (UTC-3)
- **Announcement**: Bot posts tournament open message at 00:01
- **Join period**: 00:01 to 18:00 (18 hours)
- **Round interval**: 5 minutes between rounds
- **Minimum players**: 2 (becomes 1v1)
- **Minimum cards**: 10 cards required in group to play
- **Persistence**: All results stored for leaderboard
- **No participants**: If nobody joins by 18:00, tournament is silently skipped (no announcement)

### Tournament Flow
```
1. User joins via /ranked or Game App before 18:00
2. At 18:00, all participants notified to build teams
3. Shop phase: 3 minutes to draft and arrange (or until all users are done)
   - DM sent to users if possible to notify them
4. Battles simulated, results shown
5. Winners advance, losers eliminated (or Swiss pairing)
6. Repeat until champion crowned
```

---

## Regular Matches (Casual)

On-demand matches that can be started anytime.

### Starting a Match
- `/match` - Create a match lobby
- Players join within time window (5 minutes)
- Creator has a **Start Match** button to begin early
- Format auto-selected based on final participant count

### Regular Match Flow
```
/match
    ↓
Join phase (5 minutes OR creator clicks Start Match)
    ↓
Registration closes, format determined by count
    ↓
All players build teams simultaneously (3 minutes or until all done)
    ↓
Battles resolve
    ↓
Results announced
```

### Key Differences: Ranked vs Regular

| Aspect | Ranked | Regular |
|--------|--------|---------|
| Frequency | Once daily | On-demand |
| Join window | 00:01 - 18:00 | 5 minutes (or creator starts early) |
| Announcement | Automatic at 00:01 | Manual via /match |
| Persistence | Stored for leaderboard | Stored but flagged casual |
| Leaderboard points | Yes (ranked leaderboard) | Yes (unranked leaderboard) |

---

## Match Persistence

All matches are stored in the database for history and leaderboards.

### Match Record Schema
```
matches:
  - id: UUID
  - chat_id: INT
  - match_type: ENUM('ranked', 'regular')
  - format: ENUM('1v1', 'arena')
  - status: ENUM('open', 'in_progress', 'completed')
  - created_at: TIMESTAMP
  - started_at: TIMESTAMP
  - completed_at: TIMESTAMP
  - winner_user_id: INT (for 1v1) or NULL
  - tournament_date: DATE (for ranked, NULL for regular)
  - creator_user_id: INT (for regular matches)

match_participants:
  - match_id: UUID
  - user_id: INT
  - placement: INT (1 = winner, 2 = runner-up, etc.)
  - team: JSONB (cards used, upgrades applied)
  - joined_at: TIMESTAMP

match_rounds:
  - match_id: UUID
  - round_number: INT
  - player_a_id: INT
  - player_b_id: INT
  - winner_id: INT
  - battle_log: JSONB
```

### Leaderboard Data

**Ranked Leaderboard:**
- Ranked wins/losses per user per group
- Win streaks
- Tournament placements
- Head-to-head records

**Unranked Leaderboard:**
- Regular match wins/losses per user per group
- Casual win streaks
- Separate from ranked standings

---

## Shop Phase

### Resources
- **Starting coins**: 10
- **Cards shown**: 6 (random from group's current week cards)
- **Card pool**: Shared (same card can appear for multiple players)
- **Time limit**: 3 minutes OR when all users have submitted their teams
- **User notification**: DM sent to users if possible when shop phase starts

### Actions

| Action | Cost | Effect |
|--------|------|--------|
| Buy card | 2 | Add card to your team (max 3) |
| Re-roll | 1 | Replace unbought cards with new random selection |
| Upgrade ATK | 2 | +1 ATK to one of your cards |
| Upgrade HP | 2 | +3 HP to one of your cards |

### Economy Breakdown
```
Starting: 10 coins

Mandatory: Buy 3 cards = 6 coins
Remaining: 4 coins

Example strategies:
- 2 ATK upgrades (4 coins) → Glass cannon
- 2 HP upgrades (4 coins) → Tank build
- 1 ATK + 1 HP upgrade (4 coins) → Balanced
- 1 upgrade + 2 re-rolls (4 coins) → Better cards
- 4 re-rolls (4 coins) → Maximum card selection
```

---

## Technical Implementation

### Where It Runs
- **Telegram Game App**: Rich UI, card visuals, animations
- Bot commands for quick access: `/battle`, `/arena`

### Data Flow
```
1. User opens game / joins match
2. Backend fetches group's current week cards from ml_user_cards
3. Random 6 dealt to player
4. Player builds team (stored in match state)
5. Battle simulated on backend (Go battle engine)
6. Results displayed with animated replay
```

### State Management
- Match state: Redis or in-memory (ephemeral)
- No persistent game data needed (no progression)
- Only need: active matches, player selections

### New Components Needed

1. **Game API endpoints** (add to api-service or new service)
   - `POST /game/match/create` - Start match
   - `POST /game/match/join/{id}` - Join match
   - `POST /game/match/{id}/start` - Creator starts match early
   - `GET /game/cards/{chat_id}` - Get available cards
   - `POST /game/match/{id}/team` - Submit team
   - `GET /game/match/{id}/result` - Get battle result

2. **Battle Engine** (Go)
   - Simulate combat
   - Generate battle log
   - Determine winner

3. **Game App** (Telegram Game App - React)
   - Card shop UI (full cards)
   - Team arrangement
   - Battle visualization (simplified cards)
   - Results screen

---

## Card Visuals for Game

### Full Card (Shop/Selection)
- Reuse existing card renderer
- Already generates beautiful card images
- Shows all stats, photo, username, tier
- Used in shop phase for card selection

### Simplified Card (Battle)
- Smaller card format for battle UI
- Shows: Photo, Name, ATK, DEF, HP
- Compact for battle animations
- New template needed

---

## Design Decisions

| Question | Decision |
|----------|----------|
| Stat formula | Weighted: ATK = 40% Activity + 35% Toxicity + 25% Humor |
| Economy | 10 coins, cards cost 2 each |
| Card pool | Shared (same card available to all players) |
| Battle display | Animated turn-by-turn replay |
| Upgrades | Fixed values (+1 ATK, +3 HP) |
| Match format | Auto-selected by participant count |
| Ranked schedule | Daily: open 00:01, close 18:00, rounds every 5min |
| Match persistence | All matches stored, ranked vs regular distinguished |
| Timezone | Group timezone from DB, fallback UTC-3 |
| Platform | Telegram Game App |
| Battle engine | Go |
| Card display | Full card in shop, simplified in battle |
| Re-roll scope | Only unbought cards |
| Badges | Ignored for now |
| Shop timer | 3 minutes or when all users done (DM notification) |
| Minimum cards | 10 cards required in group |
| Late joiners (ranked) | Don't join - registration closes at 18:00 |
| No participants | Silently skip tournament |
| Regular match join | 5 minutes window, creator can start early |
| Leaderboards | Both ranked and unranked leaderboards |

---

## Implementation Phases

### Phase 1: Core Game
1. ~~Add ATK/DEF calculation to ml-processor card generation~~ ✅ Done
2. Create database schema for matches, participants, rounds
3. Create game API endpoints in api-service
4. Build battle simulation engine (Go)
5. Create basic Game App with shop UI

### Phase 2: Match System
1. Implement match persistence (ranked + regular)
2. Auto-format selection based on participant count
3. Match lobby with join/leave functionality
4. Regular match flow (`/match` command)
5. Creator "Start Match" button for early start

### Phase 3: Ranked Tournament
1. Daily tournament scheduler (cron job or background worker)
2. Timezone-aware scheduling per group
3. Tournament announcement at 00:01
4. Registration close + round execution at 18:00
5. 5-minute round intervals until completion
6. Silent skip when no participants

### Phase 4: Battle Experience
1. Build animated battle replay system
2. Add team arrangement drag-and-drop
3. Sound effects and visual feedback
4. Simplified card template for battle view

### Phase 5: Leaderboards & History
1. Ranked leaderboard per group
2. Unranked leaderboard per group
3. Match history viewer
4. Head-to-head records
5. Share battle results to group
6. DM notifications for shop phase
