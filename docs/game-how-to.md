# Beef Arena Game - Complete Guide

A strategic card battle game built into Telegram that pits members of your group against each other using their real conversation statistics as in-game card stats.

**Table of Contents:**
- [Getting Started](#getting-started)
- [Creating a Telegram Mini Game](#creating-a-telegram-mini-game)
- [Game Mechanics](#game-mechanics)
- [Match Types](#match-types)
- [Card System](#card-system)
- [Shop Phase](#shop-phase)
- [Battle System](#battle-system)
- [Leaderboards](#leaderboards)
- [API Reference](#api-reference)
- [Architecture](#architecture)

---

## Getting Started

### Quick Start

**In your Telegram group:**

```
/match              → Create a casual arena match
/ranked             → Join the daily ranked tournament
```

**Click the game button when it appears** to open the Beef Arena mini app.

### Prerequisites

- **At least 2 players** in the group (max 8 for ideal gameplay)
- **10+ cards** in the group (cards are generated from weekly message stats)
- **Modern browser** on mobile (iOS Safari, Chrome, etc.)

---

## Creating a Telegram Mini Game

### What is a Telegram Mini Game?

A Telegram Mini Game (formerly Web App Game) is a lightweight web application embedded in Telegram that users can launch from bots. Unlike regular Telegram Games, mini games offer:
- Full web application capabilities
- Direct API access to your backend
- Rich interactive UI
- Real-time multiplayer support

### Steps to Register Your Game with BotFather

#### 1. Create a Bot (if you don't have one)

```
Message: @BotFather
Command: /newbot

Follow prompts to create bot
Result: Bot token (e.g., 123456789:ABCdefGHIjklmnoPQRstuvWXYZ)
```

#### 2. Register the Mini Game

```
Message: @BotFather
Command: /newgame

Steps:
1. Select your bot
2. Provide game details:
   - Short Name: "arena" (alphanumeric, no spaces)
   - Title: "Beef Arena"
   - Description: "Battle with player cards in strategic PvP matches"
   - Promo Photo: Upload game artwork (640×360px recommended)
   - Game URL: https://arena.barra-pesada.online
```

#### 3. Verify Registration

```
Message: @BotFather
Command: /mygames

You should see your game listed with all details
```

#### 4. Update Game Settings (if needed)

```
Message: @BotFather
Command: /editgame

Select your game to:
- Change title/description
- Update promo photo
- Change game URL
- Edit short name (⚠️ permanent, cannot be changed later)
```

### Game Registration Considerations

| Setting | Value | Notes |
|---------|-------|-------|
| **Bot Token** | From BotFather | Keep secret, never commit to repo |
| **Short Name** | "arena" | Permanent identifier, cannot change later |
| **Game URL** | `https://arena.barra-pesada.online` | Main entry point for game |
| **Title** | "Beef Arena" | Visible in Telegram |
| **Description** | Battle description | 256 chars max |
| **Promo Photo** | Game screenshot | 640×360px+ (square 1:1 or 16:9) |

### Environment Variables Required

For the bot to work, set these in your `.env`:

```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN=your_token_here
TELEGRAM_BOT_USERNAME=your_bot_username  # Without @

# Game URL (where the mini app is hosted)
GAME_URL=https://arena.barra-pesada.online

# API endpoints
API_BASE_URL=https://api.yourdomain.com
```

---

## Game Mechanics

### Core Concept

Players battle with **cards** representing group members. Each card's stats are derived from that person's Telegram activity:

```
Card Creator's Weekly Stats:
  - Activity: How much they message
  - Humor: How funny their messages are
  - Toxicity: How negative they are
  ↓
  Converted to Combat Stats:
  - ATK (Attack): Based on activity, toxicity, humor
  - DEF (Defense): Based on consistency, positivity, popularity
  - HP (Health): Derived from DEF
```

### The Game Loop

```
1. CREATE MATCH
   └─→ /match in group → Bot creates match lobby

2. JOIN PHASE (5 minutes)
   └─→ Users click "Play" button → Mini app opens
   └─→ Users join match from Lobby page

3. SHOP PHASE (3 minutes)
   └─→ Each player gets 10 coins
   └─→ 6 random group cards shown
   └─→ Players buy/upgrade cards for their team
   └─→ Teams arranged in battle order

4. BATTLE PHASE
   └─→ Cards fight sequentially (Auto Battlers style)
   └─→ Turn-by-turn animations show damage

5. RESULTS
   └─→ Winner crowned
   └─→ Leaderboards updated
   └─→ Results can be shared to group
```

### Stat Derivation

**Raw Stats (from user's weekly stats, 0-100 range):**
```
Activity    → How many messages sent
Humor       → Funny message reactions
Toxicity    → Negative behavior percentage
Presence    → Consistency/streak
Aura        → Overall positivity
Popularity  → Reactions from others
```

**Combat Stats (derived, 1-10 range):**
```
ATK = 40% Activity + 35% Toxicity + 25% Humor
DEF = 40% Presence + 35% Aura + 25% Popularity
HP = DEF × 3  (range 3-30)
```

**Example:**
```
User: João
Activity: 80  Humor: 60  Toxicity: 45
→ ATK = (80×0.4) + (60×0.25) + (45×0.35) = 64.75 → 6 ATK

Presence: 40  Aura: 65  Popularity: 50
→ DEF = (40×0.4) + (65×0.35) + (50×0.25) = 57.75 → 5 DEF
→ HP = 5 × 3 = 15
```

---

## Match Types

### Casual Matches (`/match`)

**Start a quick match anytime:**

| Aspect | Detail |
|--------|--------|
| **Command** | `/match` in group |
| **Join Window** | 5 minutes (or creator clicks "Start") |
| **Format** | Auto-selected: 1v1 (2 players) or Arena (3-8 players) |
| **Persistence** | Stored in database, doesn't affect ranking |
| **Stats** | Casual leaderboard |
| **Min Players** | 2 |
| **Max Players** | 8 |

**Match Flow:**
```
1. /match typed in group
2. Bot sends game widget with "Play" button
3. Users click "Play" → Mini app opens
4. Users join from Lobby page
5. Creator clicks "Start Match" OR 5 minutes pass
6. Shop phase begins
7. Battle simulates
8. Results announced
```

### Ranked Tournament (`/ranked`)

**Daily competitive tournament:**

| Aspect | Detail |
|--------|--------|
| **Command** | `/ranked` in group (or join via game) |
| **Schedule** | Daily: Opens 00:01, closes 18:00, runs 5-min rounds |
| **Join Window** | 00:01 to 18:00 (entire day) |
| **Format** | Auto-selected based on participants |
| **Persistence** | Ranked leaderboard (persistent scores) |
| **Round Time** | 5 minutes between rounds |
| **Shop Time** | 3 minutes per round to build team |

**Tournament Schedule Example:**
```
00:01   Tournament opens, announcement in group
        ↓ (Users join anytime during day)
18:00   Registration closes, Round 1 starts
18:03   Shop phase: Users arrange teams (3 min)
18:05   Round 1 results, Round 2 starts
18:08   Shop phase
18:10   Round 2 results, next round...
...continues every 5 minutes...
19:30   Final round complete, winner announced
```

### Format Selection

Format is **automatically chosen** based on participant count:

| Players | Format | Structure |
|---------|--------|-----------|
| 2 | 1v1 | Single match |
| 3-4 | Mini Arena | Round-robin |
| 5-8 | Arena | Single elimination |
| 9+ | Arena | Double elimination (if supported) |

---

## Card System

### What is a Card?

A **card** represents one group member in the game. Each card shows:

```
┌─────────────────────────┐
│   João's Card           │
│  [Profile Photo]        │
│  Activity: 80 (ATK: 6)  │
│  Humor: 60              │
│  Toxicity: 45           │
│  Presence: 40 (DEF: 5)  │
│  Aura: 65               │
│  Popularity: 50         │
│  [Tier: Elite]          │
└─────────────────────────┘
```

### Card Pool

- **Source**: Generated from weekly message statistics
- **Availability**: 6 random cards shown in each shop phase
- **Pool Size**: All group members with stats
- **Shared Pool**: Same card can appear for multiple players in shop
- **Refresh**: Changes each match (new random selection)

### Card Tiers

Based on user's overall score:

```
Score  Tier
81+    Lendário (Legendary)
77-80  Bichão (Elite)
72-76  CLT (Outstanding)
55-71  Coadjuvante (Regular)
32-54  Fióti (Beginner)
10-31  Random (Rookie)
```

### Card Stats Summary

| Stat | Range | What It Represents |
|------|-------|-------------------|
| **ATK** | 1-10 | How much damage card deals |
| **DEF** | 1-10 | How much damage card resists |
| **HP** | 3-30 | Total health (DEF × 3) |

---

## Shop Phase

The most strategic part of the game. You have **3 minutes** (or until all players submit) to build your team.

### Resources

```
Starting: 10 Coins
Objective: Buy 3 cards + upgrades

Cost Breakdown:
- Card: 2 coins
- Upgrade ATK: 2 coins (+1 attack)
- Upgrade HP: 2 coins (+3 health)
- Re-roll: 1 coin (refresh unbought cards)
```

### Strategy Example

**Glass Cannon Build (High Damage):**
```
10 coins
├─ Buy 3 cards: 6 coins
├─ Upgrade ATK card 1: 2 coins
└─ Remaining: 2 coins (1 re-roll OR save)
→ Result: Very high damage, low defense
```

**Tank Build (Survivability):**
```
10 coins
├─ Buy 3 cards: 6 coins
├─ Upgrade HP card 1: 2 coins
├─ Upgrade HP card 2: 2 coins
└─ Remaining: 0 coins
→ Result: Extra durable cards
```

**Balanced Build:**
```
10 coins
├─ Buy 3 cards: 6 coins
├─ Upgrade ATK card 1: 2 coins
├─ Upgrade HP card 2: 2 coins
└─ Remaining: 0 coins
→ Result: Moderate offense & defense
```

**Reroll Strategy (Increase Card Quality):**
```
10 coins
├─ Buy 3 cards: 6 coins
├─ Re-roll: 1 coin (get new card selection)
├─ Re-roll: 1 coin (try again)
├─ Re-roll: 1 coin (last attempt)
└─ Remaining: 1 coin
→ Result: Potentially better card selection
```

### Shop Actions

1. **Buy Card** (2 coins)
   - Add card to your team
   - Max 3 cards total
   - Cards fill battle positions: [Front] [Mid] [Back]

2. **Upgrade ATK** (2 coins)
   - Select one of your 3 cards
   - Increase its ATK by +1
   - Max 10 ATK per card

3. **Upgrade HP** (2 coins)
   - Select one of your 3 cards
   - Increase its HP by +3
   - Max 30 HP per card

4. **Re-roll** (1 coin)
   - Replace all unbought cards with new selection
   - Bought cards stay the same
   - Get fresh random cards from pool

### Team Arrangement

After buying cards, you **arrange them in battle order:**

```
[Front]  ← Attacks first, takes damage first
[Mid]    ← Attacks when front dies
[Back]   ← Reserves, attacks last
```

**Strategy Tips:**
- **Front**: Defensive/balanced card (absorbs damage)
- **Mid**: All-rounder (balanced stats)
- **Back**: Offensive card (high ATK)

**OR reverse it** for pure offense:
- **Front**: Weak sacrificial card
- **Mid**: Strong attacker
- **Back**: Strongest attacker

---

## Battle System

### How Battles Work

Cards fight **sequentially**, like Auto Battlers (Super Auto Pets):

```
Round 1:
  João (ATK:7 HP:15) vs Pedro (ATK:6 HP:18)
  ├─ João takes 6 damage → HP: 9
  └─ Pedro takes 7 damage → HP: 11

Round 2:
  João (ATK:7 HP:9) vs Pedro (ATK:6 HP:11)
  ├─ João takes 6 damage → HP: 3
  └─ Pedro takes 7 damage → HP: 4

Round 3:
  João (ATK:7 HP:3) vs Pedro (ATK:6 HP:4)
  ├─ João takes 6 damage → HP: -3 (DIES)
  └─ Pedro takes 7 damage → HP: -3 (DIES)
  Both die! Next card advances...

Round 4:
  Maria (ATK:5 HP:21) vs Ana (ATK:9 HP:12)
  ...continues...
```

### Battle Rules

1. **Damage = Attacker's ATK**
2. **Simultaneous Attacks** (both cards deal damage)
3. **Card Dies** when HP ≤ 0
4. **Winner Advances** with remaining HP
5. **Both Die** → Both removed, next cards fight
6. **Battle Ends** when one team has no cards left

### Battle Visualization

The mini app shows:

- **Card Images**: Full card art for each combatant
- **Health Bars**: Visual representation of HP
- **Damage Numbers**: Floating text showing damage dealt
- **Round Counter**: Current round number
- **Animations**: Cards taking damage, cards falling

---

## Leaderboards

### Casual Leaderboard

Tracks **regular match** performance (not ranked):

- **Unranked Wins**: Total casual match wins
- **Win Rate**: Wins / Total matches
- **Streaks**: Current win streak
- **Match History**: All casual matches

### Ranked Leaderboard

Tracks **tournament** performance:

- **Ranked Wins**: Tournament match wins only
- **Placements**: Final positions in tournaments
- **Points**: Cumulative ranking points
- **Head-to-Head**: Record vs each opponent
- **Tournament Wins**: Number of tournaments won

### Head-to-Head (H2H)

View detailed matchup history with another player:

- Total matches played
- Win/loss record
- Recent matches
- Upcoming matchups

### Accessing Leaderboards

**In the game mini app:**

1. Click "Leaderboard" tab at bottom
2. View top players by wins
3. Click player name for H2H history
4. Check "History" tab for past matches

---

## API Reference

### Game API Endpoints

All endpoints require JWT authentication (from Telegram Mini App SDK).

#### Matches

**Create Match**
```
POST /api/v1/mini-app/arena/match
Request: { }
Response: { id, chat_id, status, participants, ... }
```

**Get Active Matches**
```
GET /api/v1/mini-app/arena/matches
Response: { matches: [...] }
```

**Join Match**
```
POST /api/v1/mini-app/arena/match/{id}/join
Request: { }
Response: { id, participants: [...], ... }
```

**Leave Match**
```
POST /api/v1/mini-app/arena/match/{id}/leave
Request: { }
Response: { id, participants: [...], ... }
```

**Start Match**
```
POST /api/v1/mini-app/arena/match/{id}/start
Request: { }
Response: { id, status: "shop_phase", ... }
```

#### Shop

**Get Shop**
```
GET /api/v1/mini-app/arena/match/{id}/shop
Response: {
  cards: [
    { id, user_id, name, atk, def, hp, ... },
    ...
  ],
  coins: 10,
  time_remaining: 180
}
```

**Submit Team**
```
POST /api/v1/mini-app/arena/match/{id}/team
Request: {
  cards: [
    { card_id, upgrades: { atk: 1, hp: 0 }, position: 0 },
    ...
  ]
}
Response: { match_id, status: "battle_phase", ... }
```

#### Battle Results

**Get Match Result**
```
GET /api/v1/mini-app/arena/match/{id}/result
Response: {
  winner_id,
  placements: [
    { user_id, placement: 1, ... },
    ...
  ],
  battle_log: { ... }
}
```

#### Leaderboard

**Get Leaderboard**
```
GET /api/v1/mini-app/arena/leaderboard?type=ranked|casual
Response: {
  leaderboard: [
    { user_id, name, wins, losses, ... },
    ...
  ]
}
```

**Get H2H Record**
```
GET /api/v1/mini-app/arena/h2h/{opponent_id}
Response: {
  opponent: { ... },
  record: { wins, losses },
  recent_matches: [...]
}
```

#### Authentication

**Authenticate**
```
POST /api/v1/mini-app/auth
Request: { init_data_raw: "..." }  # From Telegram SDK
Response: {
  user_id,
  username,
  first_name,
  chat_id,
  token: "jwt..."
}
```

### Bot API Endpoints

Telegram bot commands (IP-restricted):

```
POST /api/v1/arena/match          # Create match
POST /api/v1/arena/match/{id}/join
POST /api/v1/arena/match/{id}/leave
POST /api/v1/arena/match/{id}/start
POST /api/v1/arena/ranked
```

---

## Architecture

### Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Frontend** | React + TypeScript | Mini app UI |
| **Styling** | CSS3 + Framer Motion | Animations & layout |
| **Backend** | Go | API server & battle engine |
| **Database** | PostgreSQL | Match storage & history |
| **Messaging** | Telegram Bot API | Game invitations |
| **State** | Redis | Active match state |

### Project Structure

```
beef-briefing/
├── apps/
│   ├── telegram-bot/           # Telegram bot (Go)
│   │   ├── handlers/
│   │   │   ├── match_handler.go     # /match command
│   │   │   ├── callback_handler.go  # Join/Leave/Start buttons
│   │   │   └── ...
│   │   └── client/
│   │       └── api_client.go        # API calls
│   │
│   ├── api-service/            # REST API (Go)
│   │   ├── handlers/
│   │   │   ├── arena.go             # Arena endpoints
│   │   │   └── ...
│   │   ├── services/
│   │   │   ├── arena_service.go
│   │   │   ├── battle_engine.go
│   │   │   └── ...
│   │   └── migrations/
│   │       └── arena_schema.sql
│   │
│   └── arena-mini-app/         # React mini app
│       ├── src/
│       │   ├── components/
│       │   │   ├── LobbyPage.tsx     # Match list & joining
│       │   │   ├── ShopPage.tsx      # Team building
│       │   │   ├── BattlePage.tsx    # Battle view
│       │   │   ├── LeaderboardPage.tsx
│       │   │   ├── HistoryPage.tsx
│       │   │   └── H2HPage.tsx
│       │   ├── api/
│       │   │   └── client.ts         # API client
│       │   └── App.tsx               # Root component
│       └── ...
│
├── docs/
│   ├── GDD-beef-arena.md       # Game design document
│   └── game-how-to.md          # This file
│
└── infrastructure/
    ├── docker-compose.*.yml    # Docker setup
    └── terraform/              # Infrastructure as code
```

### Data Flow

```
1. User sends /match
   ↓
2. Telegram Bot creates match via API
   ↓
3. Bot sends game widget with "Play" button
   ↓
4. User clicks "Play"
   ↓
5. Telegram opens Mini App with query params
   ↓
6. Mini App authenticates via JWT
   ↓
7. Mini App fetches match details
   ↓
8. User joins/starts/shops/battles
   ↓
9. Results persisted to database
   ↓
10. Leaderboards updated
```

### Deployment

**Development:**
```bash
docker-compose -f infrastructure/docker-compose.dev.yml up
# Arena mini app: http://localhost:3000
# API: http://localhost:8080
# Bot connects to production API
```

**Production:**
```bash
docker-compose -f infrastructure/docker-compose.prod.yml up
# Nginx serves arena-mini-app at https://arena.barra-pesada.online
# Traefik routes to API at https://api.barra-pesada.online
# Telegram bot connects via webhook
```

---

## Troubleshooting

### Common Issues

**"Not enough cards" error**
- Your group needs at least 10 cards
- Cards are generated from weekly message stats
- Wait for weekly stats refresh or message more

**Game won't open**
- Check game is registered with BotFather (`/mygames`)
- Verify game URL points to correct domain
- Check internet connection

**Can't join match**
- Ensure you're in the group chat
- Check if match already started (only open phase allows joins)
- Verify you have a card available

**Leaderboard not updating**
- Results update immediately after battle
- Historical data takes a few seconds to process
- Refresh the page if needed

### Debug Commands

**Check game registration:**
```
Message @BotFather: /mygames
```

**Test mini app in browser:**
```
https://arena.barra-pesada.online?startapp=test
```

**View match details (API):**
```
curl -H "Authorization: Bearer $TOKEN" \
  https://api.barra-pesada.online/api/v1/mini-app/arena/matches
```

---

## Advanced Topics

### Customizing Game Behavior

All game mechanics are configurable via environment variables:

```bash
# Card costs
CARD_COST=2          # Coins per card
UPGRADE_COST=2       # Coins per upgrade
REROLL_COST=1        # Coins per re-roll

# Match settings
MATCH_JOIN_TIMEOUT=300        # Seconds (5 min)
MATCH_SHOP_TIMEOUT=180        # Seconds (3 min)
MATCH_MIN_PLAYERS=2
MATCH_MAX_PLAYERS=8

# Ranked tournament
RANKED_START_TIME=00:01       # HH:MM format
RANKED_CLOSE_TIME=18:00
RANKED_ROUND_INTERVAL=300     # Seconds (5 min)

# Stat derivation
ATK_ACTIVITY_WEIGHT=0.40
ATK_TOXICITY_WEIGHT=0.35
ATK_HUMOR_WEIGHT=0.25
DEF_PRESENCE_WEIGHT=0.40
DEF_AURA_WEIGHT=0.35
DEF_POPULARITY_WEIGHT=0.25
```

### Creating a Variant

To create your own version of the game:

1. **Fork the repository**
2. **Register new bot with BotFather**
3. **Create new mini app deployment**
4. **Update environment variables**
5. **Deploy to your server**

```bash
# Clone and setup
git clone https://github.com/yourusername/beef-briefing.git
cd beef-briefing

# Build
make go-build
make docker-build

# Deploy
make deploy
```

### Monitoring

**View logs:**
```bash
make logs-bot         # Telegram bot logs
make logs-api         # API server logs
make logs            # All services
```

**Check health:**
```bash
curl https://api.barra-pesada.online/health
curl https://arena.barra-pesada.online/health
```

---

## Contributing

Want to improve the game?

- **Bug fixes**: Submit PR with issue description
- **Features**: Propose via GitHub issues first
- **Balance changes**: Update mechanics in GDD + code
- **UI improvements**: Please test on mobile first

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

---

## References

- [Telegram Bot API](https://core.telegram.org/bots/api)
- [Telegram Mini Apps](https://core.telegram.org/bots/webapps)
- [Super Auto Pets Wiki](https://superautopets.fandom.com/)
- [React Documentation](https://react.dev)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)

---

## Appendix

### Glossary

- **Card**: In-game representation of a group member
- **ATK**: Attack power (damage dealt per round)
- **DEF**: Defense power (damage resistance)
- **HP**: Health points (total durability)
- **Shop Phase**: 3-minute period to buy/upgrade cards
- **Battle Phase**: Automated card fights
- **Match**: One complete game session
- **Tournament**: Daily ranked competition
- **Leaderboard**: Ranking of players by wins
- **Tier**: Badge level (Legendary, Elite, etc.)

### Keyboard Shortcuts

**In Mini App:**
- `←` Back button
- `↑↓` Scroll
- Tap/Click cards to select
- Drag to arrange cards

---

**Last Updated**: 2026-01-11
**Game Version**: 0.1.0
**API Version**: 1.0.0

For support, open an issue on GitHub or contact the development team.
