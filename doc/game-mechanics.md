# Beef Arena Game Mechanics

A comprehensive guide to understanding how the Beef Arena card battle system works.

## Table of Contents

1. [Game Overview](#game-overview)
2. [Match Types and Flow](#match-types-and-flow)
3. [Card System](#card-system)
4. [Shop Phase](#shop-phase)
5. [Battle Mechanics](#battle-mechanics)
6. [Battle Resolution](#battle-resolution)
7. [Leaderboard and Stats](#leaderboard-and-stats)

---

## Game Overview

Beef Arena is a **team-based card battle system** where players build teams of 3 cards and compete against other players in simultaneous combat. The game combines strategic team building during a shopping phase with deterministic turn-based combat resolution.

### Key Features

- **Cards from User Data**: Battle cards are derived from users in your Telegram group, with stats based on their weekly ML analysis
- **Strategic Deck Building**: Spend coins to purchase cards and apply upgrades during the shop phase
- **Simultaneous Combat**: Both teams attack at the same time each round
- **Deterministic Battles**: Battle outcomes are reproducible and can be replayed
- **Multi-mode Play**: Casual matches anytime, ranked tournaments daily at scheduled times
- **Multi-group Support**: Players maintain separate stats in each group they participate in

---

## Match Types and Flow

### Match Types

#### Casual (Regular) Matches
- **Creation**: Any player can create a match anytime
- **Participants**: 2+ players
- **Join Window**: 5 minutes to join after creation
- **Format**: 1v1 (2 players) or Arena (3+ players)
- **Leaderboard**: Tracked separately under "regular" stats

#### Ranked Tournaments
- **Frequency**: Daily at scheduled time (typically 18:00)
- **Registration Window**: 00:01 - 18:00 (user configurable per group)
- **Participants**: All users who registered on that day
- **Format**: 1v1 (2 players) or Arena (3+ players) depending on registration count
- **Leaderboard**: Tracked separately under "ranked" stats
- **Tournament Stats**: Tracks tournaments played and won separately

### Match Flow

Every match follows the same three-phase progression:

#### Phase 1: Join Phase
```
Duration: 5 minutes for casual, 18 hours for ranked
Action: Players join the match
Output: Match roster established
```

#### Phase 2: Shop Phase
```
Duration: 3 minutes
Players can:
  - Browse 6 random cards from the group's player pool
  - Purchase cards (costs coins)
  - Upgrade purchased cards
  - Reroll available cards
  - Arrange team order for battle
Output: 3-card team submitted by each player
```

#### Phase 3: Battle Phase
```
Duration: ~seconds (depends on match type and round count)
Action: Simultaneous combat between teams
Output: Winner(s) and statistics recorded
```

---

## Card System

### Card Statistics

Each card has two categories of stats:

#### Combat Stats (used in battle)
- **ATK (Attack)**: Damage dealt per round when this card is in front
- **HP (Health Points)**: Health remaining (card dies when HP ≤ 0)
- **DEF (Defense)**: Display only—not used in combat calculations

#### Base Stats
- Cards start with base stats derived from the player's weekly ML analytics
- ATK and HP can be upgraded during shop phase
- DEF remains static and is cosmetic (useful for display/theming)

### Card Identity

Cards are represented by the player whose stats they derive from:
- **Name**: Player's first name
- **Username**: Player's optional username
- **Photo**: Player's profile photo
- **Card ID**: Unique identifier linking to the original player
- **Position**: Where the card sits in the team (0=front, 1=mid, 2=back)

### Example Stats

```
Base Card: "João"
  ATK: 8
  HP: 20
  DEF: 6

After Upgrades:
  ATK: 10 (1 ATK upgrade)
  HP: 26 (1 HP upgrade)
  DEF: 6 (unchanged)
```

---

## Shop Phase

The shop phase is where strategy begins. Players have **10 starting coins** and 3 minutes to build their team.

### Shop Mechanics

#### Card Pool
- **Count**: 6 cards available simultaneously
- **Source**: Random selection from group members' ML cards for current week
- **Cost to Purchase**: 2 coins per card
- **Team Size**: Exactly 3 cards required to submit

#### Available Actions

##### Buy Card (Cost: 2 coins)
```
Preconditions:
  - Have at least 2 coins
  - Card must be unpurchased
  - Team must have <3 cards

Effect:
  - Card is added to your team
  - Coins decrease by 2
  - Card is marked as purchased in shop
```

##### Upgrade Card (Cost: 2 coins per upgrade)
```
Types:
  1. ATK Upgrade: +1 ATK, unlimited upgrades per card
  2. HP Upgrade: +3 HP, unlimited upgrades per card

Preconditions:
  - Have at least 2 coins
  - Card must be in your team
  - Cannot cause team to go incomplete

Effect:
  - Card stat increases
  - Coins decrease by 2
  - Upgrade count tracked for display
```

##### Reroll (Cost: 1 coin)
```
Preconditions:
  - Have enough coins to: reroll (1) + complete team
  - Example: If you have 2 cards, need 2 (for 1 card) + 1 (reroll) = 3 coins

Effect:
  - All unpurchased cards replaced with new random cards
  - Coins decrease by 1
  - Purchased cards stay in shop
  - Reroll counter incremented (for tracking)
```

##### Set Team Order (Free)
```
Action: Arrange 3 purchased cards into battle order
Order Positions:
  1. Index 0: Front position (attacks first, takes damage first)
  2. Index 1: Mid position (enters battle when front dies)
  3. Index 2: Back position (final card)

Effect:
  - Determines which card leads battle
  - Can be changed anytime before submitting
  - Critical for strategy (position affects survival chance)
```

##### Submit Team (Cost: 0 coins)
```
Preconditions:
  - Exactly 3 cards in team
  - Team order valid

Effect:
  - Team locked in for battle
  - Player marked as ready
  - Cannot be changed (cannot replace cards or reorder)
  - Once all players ready, battle starts immediately

Edge Case:
  - If shop phase timer expires before all players submit,
    system auto-purchases 3 highest ATK cards available
```

### Coin Economy

| Action | Cost | Repeatable | Notes |
|--------|------|-----------|-------|
| Buy Card | 2 | Yes | Limited by cards available in shop |
| ATK Upgrade | 2 | Yes | Max coins limit: 10 + upgrades |
| HP Upgrade | 2 | Yes | Max coins limit: 10 + upgrades |
| Reroll | 1 | Yes | Must maintain team completability |
| Submit | 0 | Once | Locks team for battle |

### Economic Constraints

**Financial Safety Net**
- System ensures you never get stranded unable to complete your team
- Minimum coins required = `(remaining cards needed) × 2`
- Prevents "coin trap" scenarios

**Optimal Play Examples**

```
Scenario 1: Perfect Shopping
  Starting: 10 coins, 0 team
  Buy card 1: 8 coins, 1 card
  Buy card 2: 6 coins, 2 cards
  ATK upgrade card 1: 4 coins, 2 cards
  Buy card 3: 2 coins, 3 cards
  Ready to submit

Scenario 2: Heavy Upgrades
  Starting: 10 coins, 0 team
  Buy card 1: 8 coins, 1 card
  ATK upgrade card 1: 6 coins, 1 card
  ATK upgrade card 1: 4 coins, 1 card (now +2 ATK)
  Buy card 2: 2 coins, 2 cards
  Cannot reroll or buy card 3 with remaining coins

  Problem: 2 cards left, need 2 coins for last card
  Solution: Would need 4 coins minimum, but auto-submit will select
           the two cards + highest ATK unpurchased
```

---

## Battle Mechanics

Once both players submit their teams, the **simultaneous turn-based combat** system takes over.

### Battle Structure

Battles proceed in **rounds** until one or both teams are eliminated.

#### Each Round

1. **Front Card Identification**
   - The first living card from each team is identified
   - If all cards are dead (HP ≤ 0), team is eliminated

2. **Simultaneous Attacks**
   - Both front cards attack each other **at the same time**
   - Damage dealt = Attacker's current ATK stat
   - Damage received → reduces HP

3. **Elimination Check**
   - Cards with HP ≤ 0 are marked as dead
   - Next living card moves to front position
   - DEF stat is never consulted (cosmetic only)

4. **Round End**
   - Damage is recorded (for tiebreaking)
   - Round counter increments
   - If either team is empty, battle ends
   - Otherwise, repeat next round

### Combat Example

```
Team A: [Alice (ATK:8, HP:20), Bob (ATK:6, HP:18), Carol (ATK:7, HP:15)]
Team B: [David (ATK:9, HP:16), Eve (ATK:5, HP:22), Frank (ATK:8, HP:14)]

Round 1:
  Front A: Alice (ATK:8)
  Front B: David (ATK:9)

  Simultaneous Attack:
    Alice attacks David: 9 damage
      David: 16 → 7 HP
    David attacks Alice: 8 damage
      Alice: 20 → 12 HP

  Status: Both alive, continue

Round 2:
  Front A: Alice (ATK:8, HP:12)
  Front B: David (ATK:9, HP:7)

  Simultaneous Attack:
    Alice attacks David: 8 damage
      David: 7 → -1 HP (DEAD)
    David attacks Alice: 9 damage
      Alice: 12 → 3 HP

  Status: David eliminated, Eve advances

Round 3:
  Front A: Alice (ATK:8, HP:3)
  Front B: Eve (ATK:5, HP:22)

  Simultaneous Attack:
    Alice attacks Eve: 8 damage
      Eve: 22 → 14 HP
    Eve attacks Alice: 5 damage
      Alice: 3 → -2 HP (DEAD)

  Status: Alice eliminated, Bob advances

Round 4:
  Front A: Bob (ATK:6, HP:18)
  Front B: Eve (ATK:5, HP:14)

  ... battle continues until one team is empty
```

### Key Mechanics

#### Simultaneous Combat
- Both attacks resolve at the same time
- Neither player has "first strike" advantage
- Order doesn't matter; both take damage in same round

#### Card Advancement
- When a card dies, the next card automatically advances
- No gaps: cards in positions 1 and 2 move up
- New front card inherits full HP

#### Damage Accumulation
- Total damage dealt by each team is tracked
- Used for tiebreaking in case of simultaneous elimination
- Matters even if the match is decided by one team surviving

#### Battle Limits
- **Maximum Rounds**: 100
- Prevents infinite loops in edge cases
- In practice, most battles finish in 10-20 rounds

---

## Battle Resolution

### Victory Conditions

A battle ends when one of three scenarios occurs:

#### Scenario 1: One Team Survives
```
Team A: 0 cards alive
Team B: 1+ cards alive

Result: Team B Wins
  Winner: The surviving player
  Loser: The player whose team was eliminated
  Draws: 0
```

#### Scenario 2: Simultaneous Elimination
```
Team A: 0 cards alive (last card killed this round)
Team B: 0 cards alive (last card killed this round)

Result: Total Damage Comparison
  If Team A dealt more damage: Team A Wins
  If Team B dealt more damage: Team B Wins
  If equal damage: DRAW
```

#### Scenario 3: Max Rounds Reached
```
Round Count: 100
Both teams: Still have living cards

Result: Based on Remaining Cards or Total Damage
  More cards remaining: That team wins
  If equal: Compare total damage dealt
  If equal: DRAW
```

### Draw Conditions

A **Draw** occurs when:
1. Both teams are eliminated in the same round AND dealt equal total damage
2. Battle reaches 100 rounds with equal cards remaining AND equal damage
3. Any other tie scenario (rare edge cases)

**Draw Mechanics**:
- Both players' stats are updated as if they didn't win or lose
- Match count increments for both players
- Draw count increments for both players
- Win streak is **NOT** reset (design choice: draws are neutral)
- Head-to-head record shows "draw" outcome
- Tournament standings: varies by format

---

## Tiebreaker System

When multiple players tie, the system uses a **three-tier tiebreaker hierarchy**:

### Tier 1: Most Wins (Primary)
```
Ranking:
  1st: 5 wins
  2nd: 3 wins
  3rd: 1 win
```

### Tier 2: Total Damage Dealt (Secondary)
```
If wins tied (e.g., both 3 wins):
  1st: 250 total damage dealt
  2nd: 200 total damage dealt
```

### Tier 3: User ID (Deterministic)
```
If wins AND damage both tied:
  1st: User ID 123
  2nd: User ID 456

Ensures reproducible, non-arbitrary results
```

### Application

**1v1 Matches**: Tiebreaker not needed (one winner, one loser)

**Arena Matches** (3+ players): Uses round-robin format
- Each player fights each other player once
- Results ranked by wins, then damage, then user ID
- Example: 3-player arena
  ```
  Matches: A vs B, A vs C, B vs C

  If results:
    A: 2 wins, 150 damage
    B: 1 win, 140 damage
    C: 1 win, 160 damage

  Ranking:
    1st: A (2 wins)
    2nd: C (1 win, more damage)
    3rd: B (1 win, less damage)
  ```

---

## Leaderboard and Stats

### Stat Categories

Players maintain separate stats for **Ranked** and **Regular (Casual)** matches.

#### Matches Played
```
Definition: Total number of matches completed
Incremented: After every match (win, loss, or draw)
Resets: Never (cumulative throughout season)
```

#### Wins / Losses
```
Wins: Matches where this player's team survived
Losses: Matches where this player's team was eliminated
Draws: Matches ending in draw

Formula: Matches Played = Wins + Losses + Draws
```

#### Current Streak
```
Type: Current consecutive win streak

Rules:
  - Win: +1 to streak
  - Loss: Reset to 0
  - Draw: NO CHANGE (neutral event)

Example:
  W, W, W, D, W, W, L, W
  Current Streak: 1 (reset by loss)
```

#### Best Streak
```
Definition: Highest consecutive win count ever achieved
Incremented: When current streak exceeds best streak
Never Decreases: Only increases or stays same
```

#### Tournaments Won (Ranked Only)
```
Definition: Daily ranked tournaments this player won
Incremented: Only when player wins an entire tournament
Not Incremented: For beating individual opponents in a tournament
  (only if player wins all matches and becomes tournament champion)
```

### Multi-Group Support

**Key Feature**: Same user has **completely separate stats in each group**

```
Example:
  User: João (ID: 123)

  Group A Stats:
    Matches: 10
    Wins: 7
    Losses: 2
    Draws: 1

  Group B Stats:
    Matches: 5
    Wins: 2
    Losses: 3
    Draws: 0

  Total Across Groups: 15 matches

When João plays in Group A, only Group A stats change
When João plays in Group B, only Group B stats change
```

### Head-to-Head Records

**Personal Matchup History**

Each player maintains a detailed record against every opponent they've faced:

```
Format: { opponent_id: { wins: #, losses: #, draws: # } }

Example:
  João's head-to-head vs Alice:
    Wins: 3
    Losses: 1
    Draws: 1

  Total matches vs Alice: 5
  Win rate: 60%
```

### Leaderboard Ranking

**Primary Sort**: Wins (descending)
**Secondary Sort**: Losses (ascending)

```
Rank | Player | Wins | Losses | Draws | Matches |
-----|--------|------|--------|-------|---------|
1    | Alice  | 15   | 3      | 2     | 20      |
2    | Bob    | 12   | 5      | 1     | 18      |
3    | Carol  | 12   | 6      | 0     | 18      |
```

Note: Carol has fewer losses than Bob, so ranks higher despite same wins

---

## Ranked vs Casual Differences

Both use identical battle mechanics. The main differences are organizational:

| Aspect | Ranked | Casual |
|--------|--------|--------|
| **Creation** | Automatic daily | Player-initiated |
| **Registration** | 00:01 - 18:00 | N/A (instant join) |
| **Schedule** | Fixed (18:00) | On-demand (5-min window) |
| **Join Window** | 18 hours | 5 minutes |
| **Stat Prefix** | `ranked_*` | `regular_*` |
| **Tournament Tracking** | Yes | No |
| **Streak Tracking** | Yes | Yes |
| **Per-Group Config** | Opt-in (configurable) | Always enabled |
| **Group Requirement** | Must be enabled by admin | N/A |

### Battle Mechanics: Identical

- Same shop phase duration (3 minutes)
- Same 10 starting coins
- Same card costs and upgrades
- Same tiebreaker system
- Same simultaneous combat rules
- Same draw conditions

### Why Separate Stats?

- Players can focus on either competitive (ranked) or casual (regular) play
- Different skill levels can self-select
- Tournaments award tournaments-won separately
- Groups can disable ranked if they prefer casual only
- Personal best-streak tracking per mode

---

## Arena Format (3+ Players)

When 3+ players register for a match, **round-robin tournament** is used:

### Round-Robin System

Each player fights **every other player once**:

```
Example: 3 players (Alice, Bob, Carol)
Matches:
  1. Alice vs Bob
  2. Alice vs Carol
  3. Bob vs Carol

Tournament Winner: Player with most wins
```

```
Example: 4 players (A, B, C, D)
Matches:
  1. A vs B
  2. A vs C
  3. A vs D
  4. B vs C
  5. B vs D
  6. C vs D

Total: 6 matches
Max wins available: 3 (if undefeated)
```

### Calculation

For `n` players: `n × (n-1) / 2` total matches

- 2 players: 1 match
- 3 players: 3 matches
- 4 players: 6 matches
- 5 players: 10 matches
- 8 players: 28 matches

### Winner Determination

Players ranked by:
1. **Wins** (most wins first)
2. **Total Damage** (if wins tied, most damage)
3. **User ID** (if both tied, lower ID)

```
Results after round-robin:
  Alice: 2 wins, 280 damage
  Bob: 2 wins, 250 damage
  Carol: 0 wins, 200 damage

Final Ranking:
  1st: Alice (2 wins, more damage)
  2nd: Bob (2 wins, less damage)
  3rd: Carol (0 wins)
```

### Tournament Win Award

- Only the **final tournament winner** increments `tournaments_won`
- Players who lose in round-robin don't get tournament credit
- Useful for distinguishing "beat a player" vs "won entire tournament"

---

## Game Philosophy

### Design Principles

1. **Simultaneous, Fair Combat**: No first-strike advantage; both sides attack same round
2. **Deterministic**: Battles are reproducible; identical teams always produce identical results
3. **Strategic Depth**: Team building during shop phase is the main strategic element
4. **Resource Management**: Limited coins create meaningful choices
5. **Transparent Outcomes**: Clear stats, draws, and streaks let players understand performance
6. **Multi-Group Isolation**: Players have independent progressions in different groups
7. **Ranked/Casual Separation**: Different competitive levels without mixing stats

### Win Conditions

- Elimination: Survive while opponent doesn't
- Damage: More damage when both eliminated simultaneously
- Tiebreaker: Deterministic system for arena ties

### Fairness Mechanics

- Both players have identical shop phase duration (3 min)
- Both players have identical starting coins (10)
- Both players deal damage simultaneously (no attack order)
- Both players see same card pool sourcing method (random from group)
- Draws are possible and properly credited (not counted as loss)

---

## Summary

Beef Arena combines **strategic team building** with **deterministic simultaneous combat**. Success requires understanding:

1. **Card Economy**: Maximize your 10 coins for the strongest team
2. **Positioning**: Order matters—consider which card leads battle
3. **Combat Math**: ATK is damage; simultaneous attacks mean position affects survival
4. **Tiebreakers**: Wins first, then damage, then user ID
5. **Multi-group Support**: Separate stats in each group
6. **Streak Preservation**: Draws don't break win streaks
7. **Arena Format**: Round-robin against all opponents

Happy battling! 🎮
