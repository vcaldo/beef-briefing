# Arena Mini-App Implementation Plan

## Executive Summary

Build a complete Telegram Mini App for the Beef Arena card battle game with 4 screens: Match Lobby, Shop Phase, Battle Results, and Leaderboard & Stats. The backend API is already implemented with 18+ endpoints. We'll follow established patterns from leaderboard-mini-app and deck-mini-app, use Tailwind CSS for styling with a playful card-game aesthetic, and structure components to support future animations.

## User Requirements

- **Full MVP**: All 4 screens (Lobby, Shop, Battle, Stats)
- **Styling**: Tailwind CSS
- **Animations**: Structure for future animations, don't install library yet
- **Theme**: Playful/Card Game (colorful, Hearthstone/Clash Royale inspired)

## Architecture Overview

### Tech Stack
- React 18 + TypeScript
- Vite (build tool)
- Tailwind CSS (styling)
- Telegram Mini Apps SDK (@telegram-apps/sdk-react)
- Shared package: @beef-briefing/shared-mini-app (BaseApiClient, ErrorBoundary, etc.)

### Directory Structure
```
apps/arena-mini-app/
├── src/
│   ├── main.tsx                 # Entry point
│   ├── App.tsx                  # Main app with tabs
│   ├── api/
│   │   └── client.ts            # ArenaApiClient
│   ├── types/
│   │   └── index.ts             # All TypeScript interfaces
│   ├── components/
│   │   ├── common/              # Shared (TabBar, Card, CountdownTimer, etc.)
│   │   ├── lobby/               # Match discovery & joining
│   │   ├── shop/                # Team building (buy, upgrade, submit)
│   │   ├── battle/              # Battle replay & results
│   │   └── stats/               # Leaderboard, profile, history
│   └── styles/
│       └── global.css           # Tailwind + custom styles
├── Dockerfile
├── nginx.conf
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.js
└── postcss.config.js
```

## Game Flow

### Match Lifecycle
```
1. OPEN (5 min)
   - Players join
   - Creator can start early (2+ players)
   ↓
2. SHOP_PHASE (3 min)
   - Reroll shop (1 coin, only BEFORE first card purchase)
   - Buy 3 cards from shop (4-6 available depending on rerolls)
   - Upgrade cards (1 coin: +3 ATK or +3 HP per upgrade)
   - Set battle order
   - Submit team
   ↓
3. BATTLE_PHASE
   - Auto-simulated turn-based combat
   - Detailed event log for replay
   ↓
4. COMPLETED
   - View results & leaderboard
```

### Economy
- Starting coins: 10
- Card cost: 3
- Reroll cost: 1 (only before first card purchase)
- Upgrade cost: 1 (+3 ATK or +3 HP)

## API Integration

### API Client (`src/api/client.ts`)

Extend BaseApiClient with all arena endpoints:

**Match Management** (6 endpoints):
- `getMatches(chatId)` - List active matches
- `createMatch(chatId)` - Create new match
- `getMatch(matchId)` - Get match details
- `joinMatch(matchId)` - Join match
- `leaveMatch(matchId)` - Leave match
- `startMatch(matchId)` - Start match (creator only)

**Shop Phase** (6 endpoints):
- `getShop(matchId)` - Get shop state (poll every 3s)
- `buyCard(matchId, cardIndex)` - Buy card from shop
- `rerollShop(matchId)` - Reroll unpurchased cards (only before first purchase)
- `upgradeCard(matchId, teamSlot, upgradeType)` - Upgrade ATK or HP (+3 ATK or +3 HP)
- `setTeamOrder(matchId, order)` - Set battle order
- `submitTeam(matchId)` - Submit team

**Battle & Stats** (6 endpoints):
- `getBattle(matchId)` - Get battle results
- `getLeaderboard(chatId, type, limit, offset)` - Ranked/casual leaderboard
- `getHistory(chatId, limit, offset)` - Match history
- `getH2H(chatId, opponentId)` - Head-to-head records
- `getProfile(chatId)` - User profile stats
- `getConstants()` - Game constants

**Card Images** (for shop/team display):
- `getCardImage(userId, weekOffset?, cardType?)` - Get weekly stats card with presigned URL
- Cards are weekly generated images showing user stats (rendered by card-renderer)
- **Shop cards**: Use regular card format (400x600 pixels)
- **Team/battle cards**: Use compact card format (300x450 pixels) with placeholder positions for stat overlays
- Presigned URLs allow direct image access without additional auth
- Compact cards include `placeholder_positions` metadata for overlaying dynamic ATK/DEF/HP values and HP bar

### TypeScript Types (`src/types/index.ts`)

Key interfaces:
- `Match` - Match data with participants, status, deadlines
- `Participant` - User in match with coins, team, status
- `EnhancedShopResponse` - Shop state with cards, team, affordability
- `EnhancedShopCard` - Shop card with can_afford flag (includes card_image_url for regular cards)
- `EnhancedTeamCard` - Team card with upgrade info (includes card_image_url for compact cards + placeholder_positions)
- `BattleResult` - Complete battle with events, winner
- `BattleEvent` - Single event with card states for animation
- `CardSnapshot` - Card state at specific moment (includes card_image_url for compact cards)
- `LeaderboardEntry`, `MatchHistoryEntry`, `UserProfile`
- `CardImageResponse` - Response from card API with presigned URL and metadata
- `PlaceholderPositions` - Metadata for overlaying stats on compact cards (combat stats and HP bar positions)

## Screen Implementations

### 1. Match Lobby (`components/lobby/`)

**LobbyPage.tsx** (Main):
- Poll `/matches` every 2-3s when no active match
- Display active matches OR "Create Match" button
- Auto-navigate to Shop when match starts

**Components**:
- `MatchList` - List of active matches
- `MatchCard` - Match details with join/leave/start buttons
- `CreateMatchButton` - Prominent CTA
- `ParticipantsList` - Avatar list with ready indicators

**Polling Strategy**:
```typescript
// Poll matches endpoint
useEffect(() => {
  const interval = setInterval(async () => {
    const { matches } = await apiClient.getMatches(chatId)
    setMatches(matches)

    // If user is in a match, navigate to correct screen
    if (myMatch && myMatch.status !== 'open') {
      navigateToCorrectScreen(myMatch.status)
    }
  }, 2000)

  return () => clearInterval(interval)
}, [chatId])
```

### 2. Shop Phase (`components/shop/`)

**ShopPage.tsx** (Main):
- Poll `/shop` every 3s
- Continue polling AFTER team submission (to detect battle start)
- Handle all shop actions (buy, reroll, upgrade, order, submit)
- **Shop Card Display**: Use regular card format (400x600) for shop cards
- **Team Card Display**: Use compact card format (300x450) with stat overlays for team slots
- **Reroll Constraint**: Disable reroll button after first card purchase (permanent)

**Layout** (Mobile-optimized):
```
┌─────────────────────────┐
│ 💰 10   ⏱ 2:45         │ ← Fixed header
├─────────────────────────┤
│ [Team 1] [Team 2] [3]   │ ← Team display (sticky)
├─────────────────────────┤
│ ┌──────┐ ┌──────┐       │
│ │Shop 1│ │Shop 2│       │ ← Scrollable shop
│ └──────┘ └──────┘       │   (2x2 grid)
│ ┌──────┐ ┌──────┐       │
│ │Shop 3│ │Shop 4│       │
│ └──────┘ └──────┘       │
├─────────────────────────┤
│ [Submit Team] 🔄        │ ← Fixed footer
└─────────────────────────┘
```

**Components**:
- `ShopCards` - 2x2 grid of 4-6 shop cards
- `ShopCard` - Single card with buy button (displays regular 400x600 card image)
- `TeamDisplay` - Horizontal row of 3 team slots
- `TeamCard` - Compact card (300x450) with stat overlays using placeholder_positions
  - Overlays ATK/DEF/HP values at precise coordinates
  - Shows HP bar (background + animated fill)
  - Displays before/after stats during upgrade preview
  - Upgrade buttons (+ATK, +HP) show stat changes
- `CoinDisplay` - Large coin icon with count
- `UpgradeButtons` - ATK/HP upgrade UI with preview
- `SubmitButton` - Large CTA when team complete
- `RerollButton` - Only enabled before first card purchase

**Card Image Integration**:
- **Shop cards**: Display regular card images (400x600) from card-renderer API
- **Team cards**: Display compact card images (300x450) from card-renderer API with position overlays
- Images are fetched via the Cards API endpoints with presigned URLs
- Cards show gamified user stats (rendered by card-renderer service)
- Fallback to placeholder if card image unavailable

**Compact Card Overlay System**:
- Compact cards include `placeholder_positions` metadata from card-renderer API
- Position data provides exact coordinates for overlaying dynamic values:
  - `combat_stats.atk` - ATK value position (x, y, width, height, font size, color)
  - `combat_stats.def` - DEF value position
  - `combat_stats.hp` - HP value position
  - `hp_bar.container` - HP bar background position
  - `hp_bar.fill` - HP bar fill position (animates based on current_hp / max_hp)
- All positions are in logical pixels (300x450 card dimensions)
- React overlays absolutely positioned divs on top of static card image
- HP bar animates smoothly with CSS transitions during battle events

**Key Features**:
- Disable actions during API calls (loading state)
- Show affordability tooltips (can't afford, team full, etc.)
- Read-only state after submission with "Waiting for others..." message
- Countdown timer with urgency colors (green → yellow → red)

### 3. Battle Results (`components/battle/`)

**BattlePage.tsx** (Main):
- Fetch battle results once on mount
- Control event playback (play/pause, speed, skip to end)
- Animate events sequentially
- Update card states (ATK, HP, alive status) after each event

**BattleArena.tsx**:
- Visual representation of 2 teams
- 3 compact cards (300x450) per team in battle order
- Update card states based on current event
- Cards use compact format with stat overlays and animated HP bars

**Components**:
- `BattleCard` - Compact card with stat overlays using placeholder_positions
  - Displays current ATK/DEF/HP using position metadata
  - Animated HP bar that updates after each attack event
  - HP bar width = (current_hp / max_hp) * 100%
  - HP bar color changes based on HP percentage (green → yellow → red)
  - Grayscale + opacity when card dies (alive/dead state)
  - Position indicator showing battle order
- `EventLog` - Scrollable event list
- `EventMessage` - Single event display
- `VictoryScreen` - Overlay with winner announcement

**HP Bar Animation**:
- After each attack event, HP bar smoothly animates to new width
- CSS transition: `width 0.3s ease`
- Color transitions: green (>66%), yellow (33-66%), red (<33%)
- Bar empties when card dies (HP = 0)

**Animation Preparation**:
```tsx
// BattleCard with data attributes for future animations
<div
  className="battle-card"
  data-card-id={card.card_id}
  data-is-attacking={card.is_attacking}
  data-is-defending={card.is_defending}
  data-is-alive={card.is_alive}
>
  {/* Card content */}
</div>
```

CSS classes:
- `.battle-card[data-is-attacking="true"]` - Ring + pulse
- `.battle-card[data-is-defending="true"]` - Ring
- `.battle-card[data-is-alive="false"]` - Grayscale + opacity

### 4. Leaderboard & Stats (`components/stats/`)

**StatsPage.tsx** (Main):
- 4 sub-tabs: Leaderboard, Profile, History, H2H
- No automatic polling (manual refresh only)
- Cache data per tab

**Components**:
- `LeaderboardTab` - Ranked/casual toggle, pagination
- `LeaderboardEntry` - Rank, avatar, name, stats
- `ProfileTab` - Current user's stats (ranked + casual)
- `ProfileStats` - Stat cards with icons
- `HistoryTab` - Paginated match history
- `MatchHistoryCard` - Match result card (W/L/D)
- `H2HTab` - Search opponent, show H2H record

## Compact Card Overlay Implementation

### Overview
Compact cards (300x450) are used for team display and battle views. They provide placeholder position metadata for overlaying dynamic stats.

### API Integration

**Fetching Compact Cards**:
```typescript
// Get compact card image with position metadata
const response = await fetch(
  `/api/v1/images?chat_id=${chatId}&week_start=${weekStart}&theme=neon_arcade_compact&include_positions=true`,
  { headers: { 'Authorization': `Bearer ${apiKey}` } }
);

const data = await response.json();
const card = data.images[0];

// Access position metadata
const { placeholders, card_dimensions } = card.placeholder_positions;
const { combat_stats, hp_bar } = placeholders;
```

### React Component Structure

**CompactCard.tsx** (Reusable component):
```tsx
interface CompactCardProps {
  imageUrl: string;
  positions: PlaceholderPositions;
  currentStats: {
    atk: number;
    def: number;
    hp: number;
    maxHp: number;
  };
  isAlive?: boolean;
  showHpBar?: boolean;
}

function CompactCard({ imageUrl, positions, currentStats, isAlive = true, showHpBar = true }: CompactCardProps) {
  const { combat_stats, hp_bar } = positions.placeholders;
  const hpPercentage = (currentStats.hp / currentStats.maxHp) * 100;

  // HP bar color based on percentage
  const getHpBarColor = (percent: number) => {
    if (percent > 66) return '#22c55e'; // green
    if (percent > 33) return '#eab308'; // yellow
    return '#ef4444'; // red
  };

  return (
    <div style={{ position: 'relative', width: 300, height: 450, opacity: isAlive ? 1 : 0.5 }}>
      {/* Base card image */}
      <img
        src={imageUrl}
        alt="Card"
        style={{
          width: '100%',
          height: '100%',
          filter: isAlive ? 'none' : 'grayscale(100%)'
        }}
      />

      {/* ATK overlay */}
      <div style={{
        position: 'absolute',
        left: combat_stats.atk.x,
        top: combat_stats.atk.y,
        width: combat_stats.atk.width,
        fontSize: combat_stats.atk.font_size,
        fontWeight: combat_stats.atk.font_weight,
        color: combat_stats.atk.color,
        textAlign: 'center',
        transform: 'translateX(-50%)'
      }}>
        {currentStats.atk}
      </div>

      {/* DEF overlay */}
      <div style={{
        position: 'absolute',
        left: combat_stats.def.x,
        top: combat_stats.def.y,
        width: combat_stats.def.width,
        fontSize: combat_stats.def.font_size,
        fontWeight: combat_stats.def.font_weight,
        color: combat_stats.def.color,
        textAlign: 'center',
        transform: 'translateX(-50%)'
      }}>
        {currentStats.def}
      </div>

      {/* HP overlay */}
      <div style={{
        position: 'absolute',
        left: combat_stats.hp.x,
        top: combat_stats.hp.y,
        width: combat_stats.hp.width,
        fontSize: combat_stats.hp.font_size,
        fontWeight: combat_stats.hp.font_weight,
        color: combat_stats.hp.color,
        textAlign: 'center',
        transform: 'translateX(-50%)'
      }}>
        {currentStats.hp}
      </div>

      {/* HP bar (if enabled) */}
      {showHpBar && (
        <div style={{
          position: 'absolute',
          left: hp_bar.container.x,
          top: hp_bar.container.y,
          width: hp_bar.container.width,
          height: hp_bar.container.height,
          backgroundColor: hp_bar.fill.background_color,
          borderRadius: hp_bar.container.border_radius,
          overflow: 'hidden'
        }}>
          <div style={{
            height: '100%',
            width: `${hpPercentage}%`,
            backgroundColor: getHpBarColor(hpPercentage),
            borderRadius: hp_bar.fill.border_radius,
            transition: 'width 0.3s ease, background-color 0.3s ease'
          }} />
        </div>
      )}
    </div>
  );
}
```

### Usage Scenarios

**1. Team Display (Shop Phase)**:
- Show compact cards for purchased team members
- Display current stats (may be upgraded from base stats)
- Show HP bar (static, always at 100%)
- Update stats when upgrade is applied

**2. Upgrade Preview**:
- Show before/after stats when hovering over upgrade button
- Highlight changed stat value (ATK or HP)
- Animate stat value change

**3. Battle Display**:
- Show compact cards for both teams
- Update stats after each battle event
- Animate HP bar as damage is dealt
- Grayscale + fade out when card dies
- Show position indicator (front/back)

**4. Team Reorder**:
- Show compact cards in draggable list
- Display current stats
- Show HP bar (static)
- Reorder by dragging (future enhancement: drag-and-drop library)

### Position Data Caching
- Cache placeholder_positions for each theme to avoid redundant API calls
- Position data is static for a given theme
- Only need to fetch once per session per theme

## Theme System

### Tailwind Configuration

**Color Palette** (Playful/Card Game):
```js
colors: {
  primary: { 500: '#f97316' },      // Orange (energy, action)
  secondary: { 500: '#3b82f6' },    // Blue (cool, strategic)
  success: { 500: '#22c55e' },      // Green (victory)
  danger: { 500: '#ef4444' },       // Red (attacks, low HP)
  gold: { 500: '#eab308' },         // Gold (coins)
  arena: {
    dark: '#1a1625',                // Background
    purple: '#7c3aed',              // Accent
    pink: '#ec4899',                // Accent
    cyan: '#06b6d4',                // Accent
  }
}
```

**Typography**:
- Display: Outfit (modern, geometric, friendly)
- Body: Inter (legible, neutral)
- Mono: JetBrains Mono (stats, timers)

**Custom Animations**:
```js
animation: {
  'bounce-slow': 'bounce 3s infinite',
  'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
  'shake': 'shake 0.5s cubic-bezier(.36,.07,.19,.97) both',
}
```

### Global CSS Classes

Defined in `src/styles/global.css`:
- `.splash-screen` - Loading screen
- `.tab-bar` - Bottom navigation
- `.card` - Card container with shadow
- `.btn-primary`, `.btn-secondary`, etc. - Button styles
- `.hp-bar` - HP bar with color transitions
- `.coin-display` - Coin icon + count
- `.countdown-timer.urgent` - Red pulsing timer

## Polling Strategy

| Screen | Endpoint | Interval | Stop Condition |
|--------|----------|----------|----------------|
| Lobby (no match) | `/matches` | 3s | Match created/joined |
| Lobby (in match) | `/match/{id}` | 2s | Status !== 'open' |
| Shop | `/match/{id}/shop` | 3s | Status === 'battle_phase' |
| Battle | - | - | No polling |
| Stats | - | - | Manual refresh only |

**Phase Transition Detection**:
- Open → Shop: Poll `/match/{id}`, check `status === 'shop_phase'`
- Shop → Battle: Poll `/match/{id}/shop`, check `status === 'battle_phase'`
- **Important**: Continue polling shop even after team submission (GetShop returns read-only state)

## State Management

- **Local state**: All component state managed with useState/useEffect
- **No global state library**: Simple enough without Redux/Zustand
- **Caching**: Prefetch data during splash screen (like leaderboard-mini-app)
- **Polling**: setInterval for real-time updates, clearInterval on unmount

## Future Animation Support

### Component Structure
- Use data attributes for animation hooks (`data-is-attacking`, `data-card-id`)
- CSS transitions for simple animations (HP bar, button clicks)
- Structure ready for Framer Motion or GSAP later

### Recommended Animation Points
**Shop**: Card purchase (fly to team), reroll (flip), upgrade (glow), coin change (bounce)
**Battle**: Attack (lunge), damage (shake), death (fade), advance (slide), victory (confetti)
**Lobby**: Match creation (slide in), join (avatar pop), countdown (pulse)

### Future Integration
```tsx
// Current (no animation library)
<BattleCard card={card} data-is-attacking={isAttacking} />

// Future (after installing framer-motion)
import { motion } from 'framer-motion'

<motion.div
  animate={isAttacking ? "attack" : "idle"}
  variants={attackVariants}
>
  <BattleCard card={card} />
</motion.div>
```

## Main App Component (`src/App.tsx`)

**Structure** (following leaderboard pattern):
```typescript
function App() {
  const [appState, setAppState] = useState<'loading' | 'authenticated' | 'error'>('loading')
  const [activeTab, setActiveTab] = useState<TabId>('lobby')
  const [userId, setUserId] = useState<number>(0)
  const [chatId, setChatId] = useState<number>(0)
  const [firstName, setFirstName] = useState<string>('')
  const [splashMinTimeElapsed, setSplashMinTimeElapsed] = useState(false)

  const launchParams = useLaunchParams()

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        const auth = await apiClient.authenticate(launchParams.initDataRaw)
        setUserId(auth.user_id)
        setChatId(auth.chat_id)
        setFirstName(auth.first_name)
        setAppState('authenticated')

        // Prefetch game constants
        apiClient.getConstants()
      } catch (err) {
        setError(err.message)
        setAppState('error')
      }
    }
    authenticate()
  }, [launchParams?.initDataRaw])

  // Minimum splash time (2s)
  useEffect(() => {
    const timer = setTimeout(() => setSplashMinTimeElapsed(true), 2000)
    return () => clearTimeout(timer)
  }, [])

  // Show splash screen
  if (appState === 'loading' || !splashMinTimeElapsed) {
    return <SplashScreen />
  }

  // Main app
  return (
    <div className="app">
      <ErrorBoundary key={activeTab}>
        {renderPage()}
      </ErrorBoundary>
      <TabBar activeTab={activeTab} onTabChange={setActiveTab} />
    </div>
  )
}
```

**Tab Navigation**:
- 4 tabs: Lobby (🎮), Shop (🛒), Battle (⚔️), Stats (📊)
- Auto-navigate based on match status
- ErrorBoundary per tab (key={activeTab} resets on tab change)

## Deployment

### Docker Setup

**Dockerfile** (multi-stage):
```dockerfile
# Build stage
FROM node:22-alpine AS builder
RUN npm install -g pnpm
WORKDIR /app
COPY package*.json pnpm-workspace.yaml ./
COPY apps/arena-mini-app/ ./apps/arena-mini-app/
COPY apps/shared-mini-app/ ./apps/shared-mini-app/
RUN pnpm install --frozen-lockfile
WORKDIR /app/apps/arena-mini-app
ARG VITE_API_URL
ARG VITE_NEW_RELIC_*
ARG VITE_ENVIRONMENT
RUN pnpm run build

# Production stage
FROM nginx:alpine
COPY apps/arena-mini-app/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/apps/arena-mini-app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### docker-compose.prod.yml Entry

```yaml
arena-mini-app:
  build:
    context: ..
    dockerfile: apps/arena-mini-app/Dockerfile
    args:
      VITE_API_URL: https://api.${DOMAIN_NAME}
      VITE_NEW_RELIC_BROWSER_ACCOUNT_ID: ${NEW_RELIC_ACCOUNT_ID:-}
      VITE_NEW_RELIC_BROWSER_APP_ID: ${NEW_RELIC_BROWSER_ARENA_APP_ID:-}
      VITE_NEW_RELIC_BROWSER_LICENSE_KEY: ${NEW_RELIC_BROWSER_LICENSE_KEY:-}
      VITE_ENVIRONMENT: production
  image: beef-briefing/arena-mini-app:${IMAGE_TAG}
  container_name: beef-arena-mini-app-prod
  networks:
    - beef-prod-network
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.arena-mini-app.rule=Host(`arena.${DOMAIN_NAME}`)"
    - "traefik.http.routers.arena-mini-app.entrypoints=websecure"
    - "traefik.http.routers.arena-mini-app.tls.certresolver=letsencrypt"
    - "traefik.http.services.arena-mini-app.loadbalancer.server.port=80"
  restart: unless-stopped
```

### Infrastructure Changes

**API CORS Update** (in api-service):
```yaml
CORS_ORIGINS: https://leaderboard.${DOMAIN_NAME},https://deck.${DOMAIN_NAME},https://arena.${DOMAIN_NAME}
```

**DNS Configuration**:
- Add A record: `arena.yourdomain.com` → Server IP
- Traefik auto-provisions SSL via Let's Encrypt

## Implementation Sequence

### Phase 1: Foundation (Day 1-2)
1. ✅ Create project structure
2. ✅ Set up configuration files (package.json, tsconfig, vite, tailwind, postcss)
3. ✅ Implement API client (`src/api/client.ts`)
4. ✅ Define all TypeScript types (`src/types/index.ts`)
5. ✅ Create global styles (`src/styles/global.css`)
6. ✅ Implement common components (TabBar, Card, CountdownTimer, LoadingSpinner, ErrorDisplay)
7. ✅ Set up main app component (`src/App.tsx`)
8. ✅ Test Dockerfile build

### Phase 2: Lobby Screen (Day 3-4)
9. ✅ Implement LobbyPage
10. ✅ Implement MatchList, MatchCard, CreateMatchButton, ParticipantsList
11. ✅ Test match creation, joining, leaving
12. ✅ Test countdown timer and auto-start
13. ✅ Test polling and phase transition to shop

### Phase 3: Shop Screen (Day 5-7)
14. ✅ Implement ShopPage with polling
15. ✅ Implement ShopCards, ShopCard (regular 400x600 card images)
16. ✅ Implement CompactCard component (300x450 with position overlays)
17. ✅ Implement TeamDisplay, TeamCard (compact cards with stat overlays and HP bar)
18. ✅ Implement CoinDisplay, UpgradeButtons, SubmitButton, RerollButton
19. ✅ Test buy, reroll (before first purchase only), upgrade, order, submit flows
20. ✅ Test reroll disabled after first card purchase
21. ✅ Test stat overlay positioning on compact cards
22. ✅ Test upgrade preview showing before/after stats
23. ✅ Test read-only state after submission
24. ✅ Test countdown and auto-transition to battle
25. ✅ Test card image loading and fallback states

### Phase 4: Battle Screen (Day 8-9)
26. ✅ Implement BattlePage
27. ✅ Implement BattleArena with compact cards for both teams
28. ✅ Implement BattleCard using CompactCard component
29. ✅ Implement HP bar animation after each attack event
30. ✅ Implement HP bar color transitions (green → yellow → red)
31. ✅ Implement EventLog, EventMessage
32. ✅ Implement VictoryScreen
33. ✅ Test event playback (sequential display with stat updates)
34. ✅ Test HP bar animations during battle
35. ✅ Test all event types (attack, death, summary, victory)

### Phase 5: Stats Screen (Day 10-11)
36. ✅ Implement StatsPage with sub-tabs
37. ✅ Implement LeaderboardTab, LeaderboardEntry
38. ✅ Implement ProfileTab, ProfileStats
39. ✅ Implement HistoryTab, MatchHistoryCard
40. ✅ Implement H2HTab
41. ✅ Test pagination, filters, data loading

### Phase 6: Integration & Polish (Day 12-14)
42. ✅ End-to-end testing (full match flow)
43. ✅ Error handling and edge cases
44. ✅ Performance optimization (bundle size, lazy loading)
45. ✅ UI polish and theme refinement
46. ✅ Test compact card overlay system across all themes
47. ✅ Accessibility improvements
48. ✅ Documentation and README

## Critical Files to Create

### Core (7 files)
1. `package.json` - Dependencies and scripts
2. `tsconfig.json` - TypeScript config
3. `vite.config.ts` - Vite config
4. `tailwind.config.js` - Tailwind theme
5. `postcss.config.js` - PostCSS plugins
6. `Dockerfile` - Production build
7. `nginx.conf` - Nginx config

### Entry Points (3 files)
8. `index.html` - HTML entry
9. `src/main.tsx` - React entry, Telegram SDK init
10. `src/App.tsx` - Main app component

### API & Types (3 files)
11. `src/api/client.ts` - ArenaApiClient (18+ endpoints)
12. `src/types/index.ts` - All TypeScript interfaces
13. `src/vite-env.d.ts` - Vite type declarations

### Styles (1 file)
14. `src/styles/global.css` - Tailwind + custom styles

### Common Components (7 files)
15. `src/components/common/index.ts` - Export barrel
16. `src/components/common/TabBar.tsx` - Main navigation
17. `src/components/common/CountdownTimer.tsx` - Countdown timer
18. `src/components/common/Card.tsx` - Reusable card component (regular cards)
19. `src/components/common/CompactCard.tsx` - Compact card with stat overlays and HP bar
20. `src/components/common/LoadingSpinner.tsx` - Loading state
21. `src/components/common/ErrorDisplay.tsx` - Error state

### Lobby Components (6 files)
22-27. LobbyPage, MatchList, MatchCard, CreateMatchButton, ParticipantsList, index.ts

### Shop Components (10 files)
28-37. ShopPage, ShopCards, ShopCard, TeamDisplay, TeamCard, CoinDisplay, UpgradeButtons, SubmitButton, RerollButton, index.ts

### Battle Components (7 files)
38-44. BattlePage, BattleArena, BattleCard (uses CompactCard), EventLog, EventMessage, VictoryScreen, index.ts

### Stats Components (9 files)
45-53. StatsPage, LeaderboardTab, LeaderboardEntry, ProfileTab, ProfileStats, HistoryTab, MatchHistoryCard, H2HTab, index.ts

### Documentation (1 file)
54. `README.md` - Project documentation

**Total: 54 files**

## Testing Strategy

### Manual Testing Checklist
- [ ] Create match (casual)
- [ ] Join match
- [ ] Leave match (before start)
- [ ] Start match early
- [ ] Auto-start at deadline
- [ ] Buy cards from shop (4-6 available)
- [ ] Reroll shop (only before first purchase)
- [ ] Verify reroll disabled after first card purchase
- [ ] Upgrade ATK and HP (verify +3 ATK, +3 HP)
- [ ] Verify upgrade preview shows before/after stats on compact cards
- [ ] Verify stat overlays display correctly on compact cards (ATK/DEF/HP at precise positions)
- [ ] Reorder team
- [ ] Submit team
- [ ] View read-only state after submission
- [ ] Battle playback (all event types)
- [ ] Verify HP bar animates smoothly during battle
- [ ] Verify HP bar color changes (green → yellow → red)
- [ ] Verify cards grayscale when dead
- [ ] Verify stat overlays update during battle
- [ ] Test compact cards across different themes (gaming, clean, neon_arcade)
- [ ] View leaderboard (ranked/casual)
- [ ] View profile stats
- [ ] View match history
- [ ] View H2H records
- [ ] Test on iOS, Android, desktop

## Potential Challenges & Mitigations

### Challenge 1: Small Touch Targets
**Mitigation**: Minimum 44x44px touch targets, generous padding, visual feedback

### Challenge 2: Countdown Sync Drift
**Mitigation**: Calculate from server deadline, refresh every 10-15s, use Date.now()

### Challenge 3: Shop Race Conditions
**Mitigation**: Disable actions during API call, optimistic UI, debounce clicks

### Challenge 4: Battle Event Performance
**Mitigation**: Virtualize event log, CSS transforms, speed controls, skip button

### Challenge 5: Long History Lists
**Mitigation**: Pagination (20-50 per page), infinite scroll, cache loaded pages

## Success Criteria

- ✅ All 4 screens implemented and functional
- ✅ Full match flow works end-to-end (lobby → shop → battle → stats)
- ✅ Polling transitions between phases automatically
- ✅ Shop economy enforced correctly (affordability checks, reroll only before first purchase)
- ✅ Compact card overlay system working correctly (stat overlays at precise positions)
- ✅ HP bar animations working smoothly during battle
- ✅ Upgrade preview shows before/after stats on compact cards
- ✅ Battle replay shows all events sequentially with stat updates
- ✅ Leaderboard, profile, and history data display correctly
- ✅ Mobile-optimized UI with playful card-game aesthetic
- ✅ Structured for future animations (data attributes, CSS classes)
- ✅ Deployed to production at arena.${DOMAIN_NAME}
- ✅ Bundle size < 300KB gzipped

## Future Enhancements (Post-MVP)

### Phase 2
- Install Framer Motion and add battle animations
- Implement drag-and-drop for team ordering
- View past battle replays from history
- Push notifications via Telegram

### Phase 3
- Tournament bracket visualization
- Card collection viewer
- Spectator mode (watch live battles)
- Achievements and badges
- Seasonal leaderboards

---

## Verification Plan

After implementation:

1. **End-to-End Test**: Create match → join → shop phase → battle → view stats
2. **Edge Cases**: Test match cancellation, user leaving, timeout scenarios
3. **Network Errors**: Test offline mode, slow network, API errors
4. **Cross-Device**: Test on iOS, Android, desktop Telegram
5. **Performance**: Check bundle size, API response times, polling overhead
6. **Accessibility**: Test keyboard navigation, screen reader compatibility

---

## Notes

- Backend API is **production-ready** (18+ endpoints already implemented)
- Shared package provides BaseApiClient with JWT auth logic
- Follow leaderboard-mini-app patterns for consistency
- Avoid generic AI aesthetics (see CLAUDE.md frontend_aesthetics section)
- Keep bundle small (defer animation library to future phase)
- Mobile-first responsive design
- Traefik handles SSL automatically

### Card Display Strategy
- **Shop cards**: Use regular card format (400x600) from card-renderer API - each card shows a real user with their weekly stats
- **Team/battle cards**: Use compact card format (300x450) with position overlays for dynamic stats (ATK/DEF/HP) and animated HP bar
- **Compact card overlay system**: Card-renderer API provides `placeholder_positions` metadata with exact coordinates for overlaying stat values
- **HP bar**: Animated progress bar that updates during battle events, with color transitions (green → yellow → red) based on HP percentage

### Recent Changes (Commits 839406c & 38e221f)
- **Compact Card Renderer Feature**: Added 300x450 compact cards with placeholder position metadata for stat overlays
- **Re-roll Mechanics Change**: Reroll now only available BEFORE first card purchase (permanent lockout after buying first card)
- **Shop Balance**: 4-6 cards available (depending on rerolls), costs 3 coins per card, 1 coin per reroll, 1 coin per upgrade
- **Upgrade Values**: +3 ATK or +3 HP per upgrade (changed from previous values)
