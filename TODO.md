# Arena Mini-App Implementation Tasks

This file tracks implementation tasks for the Arena Mini-App Telegram Mini App, extracted from [arena-mini-app-plan.md](arena-mini-app-plan.md). Tasks are organized by priority phase.

Ralph will work through these tasks in priority order. See [scripts/RALPH.md](scripts/RALPH.md) for detailed instructions on running automated task execution.

---

## High Priority

### Phase 1: Foundation (Day 1-2)

- [x] Create project structure (`apps/arena-mini-app/`)
- [x] Set up configuration files (package.json, tsconfig, vite, tailwind, postcss)
- [x] Implement API client (`src/api/client.ts`) with 18+ Arena endpoints
- [x] Define all TypeScript types (`src/types/index.ts`)
- [x] Create global styles (`src/styles/global.css`) with Tailwind + custom CSS
- [x] Implement common components (TabBar, Card, CountdownTimer, LoadingSpinner, ErrorDisplay)
- [x] Set up main app component (`src/App.tsx`) with tab navigation and authentication
- [x] Test Dockerfile build (multi-stage build with Nginx)

---

## Medium Priority

### Phase 2: Lobby Screen (Day 3-4)

- [x] Implement LobbyPage with match polling and auto-navigation
- [x] Implement MatchList, MatchCard, CreateMatchButton, ParticipantsList components
- [x] Test match creation, joining, and leaving flows
- [ ] Test countdown timer and auto-start mechanics
- [ ] Test polling and phase transition to shop when match starts

### Phase 3: Shop Screen (Day 5-7)

- [x] Implement ShopPage with shop polling and action handling
- [x] Implement ShopCards and ShopCard components (display regular 400x600 card images)
- [x] Implement CompactCard component (300x450 with position overlays and stat rendering)
- [x] Implement TeamDisplay and TeamCard components (compact cards with stat overlays and HP bar)
- [x] Implement CoinDisplay, UpgradeButtons, SubmitButton, RerollButton components
- [ ] Test buy, reroll (before first purchase only), upgrade, order, and submit flows
- [ ] Test reroll button disabled after first card purchase (permanent lockout)
- [ ] Test stat overlay positioning on compact cards using placeholder_positions
- [ ] Test upgrade preview showing before/after stats on compact cards
- [ ] Test read-only state after team submission ("Waiting for others...")
- [ ] Test countdown timer and auto-transition to battle phase
- [ ] Test card image loading and fallback states

---

## Low Priority

### Phase 4: Battle Screen (Day 8-9)

- [x] Implement BattlePage with event playback controls and battle results display
- [x] Implement BattleArena with compact cards for both teams (3 cards each)
- [x] Implement BattleCard component using CompactCard with live stat updates
- [x] Implement HP bar animation after each attack event (smooth CSS transitions)
- [x] Implement HP bar color transitions (green >66% → yellow 33-66% → red <33%)
- [x] Implement EventLog and EventMessage components for battle replay
- [x] Implement VictoryScreen overlay with winner announcement
- [ ] Test event playback with sequential display and stat updates
- [ ] Test HP bar animations and color changes during battle
- [ ] Test all event types (attack, death, summary, victory)

### Phase 5: Stats Screen (Day 10-11)

- [ ] Implement StatsPage with 4 sub-tabs (Leaderboard, Profile, History, H2H)
- [ ] Implement LeaderboardTab and LeaderboardEntry components (ranked/casual toggle)
- [ ] Implement ProfileTab and ProfileStats components
- [ ] Implement HistoryTab and MatchHistoryCard components
- [ ] Implement H2HTab with opponent search and head-to-head records
- [ ] Test pagination, filters, and data loading across all stats tabs

### Phase 6: Integration & Polish (Day 12-14)

- [ ] End-to-end testing (full match flow: lobby → shop → battle → stats)
- [ ] Error handling and edge cases (API errors, network failures, timeouts)
- [ ] Performance optimization (bundle size <300KB gzipped, lazy loading)
- [ ] UI polish and theme refinement (playful card-game aesthetic)
- [ ] Test compact card overlay system across all themes (gaming, clean, neon_arcade)
- [ ] Accessibility improvements (touch targets, keyboard nav, screen readers)
- [ ] Documentation and README

---

## Completed

- [x] Create project structure (`apps/arena-mini-app/`)
- [x] Set up configuration files (package.json, tsconfig, vite, Dockerfile, nginx.conf)
- [x] Set up main app component (`src/App.tsx`) with tab navigation and authentication
- [x] Implement API client (`src/api/client.ts`) with 18+ Arena endpoints
- [x] Define all TypeScript types (`src/types/index.ts`)
- [x] Create global styles (`src/styles/global.css`) with Tailwind + custom CSS
- [x] Implement LobbyPage with match polling and auto-navigation
- [x] Implement MatchList, MatchCard, CreateMatchButton, ParticipantsList components
- [x] Implement BattlePage with event playback controls and battle results display
- [x] Implement BattleArena, BattleCard, EventLog, EventMessage, VictoryScreen components

---

## Notes for Ralph

### Context
This is the Arena Mini-App implementation for the Beef Briefing Telegram bot system. The backend API (18+ endpoints) is already production-ready. The app displays a turn-based card battle game with 4 main screens.

### Architecture
- **Framework**: React 18 + TypeScript
- **Build**: Vite
- **Styling**: Tailwind CSS with custom animations
- **SDK**: Telegram Mini Apps SDK (@telegram-apps/sdk-react)
- **Shared**: Use patterns from [leaderboard-mini-app](apps/leaderboard-mini-app/) and [deck-mini-app](apps/deck-mini-app/)

### Key Implementation Details
- **Polling**: Use setInterval for real-time updates (Lobby: 2-3s, Shop: 3s)
- **Compact Cards**: 300x450 format with stat overlays using placeholder_positions metadata
- **Economy**: 10 starting coins, 3 coins/card, 1 coin/reroll, 1 coin/upgrade
- **Reroll**: Only available BEFORE first card purchase (permanent lockout after)
- **Upgrades**: +3 ATK or +3 HP per upgrade
- **HP Bar**: Smooth CSS transitions with color based on % health

### Important Notes
- Follow patterns from existing mini-apps for consistency
- Avoid generic AI aesthetics (see CLAUDE.md frontend_aesthetics section)
- Keep bundle small (defer animation library to Phase 2)
- Mobile-first responsive design
- Structure components for future animations (data attributes, CSS classes)

### Files Reference
- Full plan: [arena-mini-app-plan.md](arena-mini-app-plan.md)
- Project docs: [CLAUDE.md](CLAUDE.md)
- Ralph guide: [scripts/RALPH.md](scripts/RALPH.md)
- Shared package: [apps/shared-mini-app/](apps/shared-mini-app/)

### Running Ralph
Once this TODO.md is ready:
```bash
./scripts/ralph.sh 10          # Run 10 iterations
./scripts/ralph.sh 20 TODO.md  # Custom iteration count
```

Monitor progress:
```bash
tail -f progress.txt
git log --oneline -10
```
