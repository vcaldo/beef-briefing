# Arena Mini App

Telegram Mini App for competitive card-battle gameplay.

## Overview

The Arena Mini App is a turn-based card battle game where players create or join matches, build teams by purchasing and upgrading cards during a shop phase, then watch AI-simulated battles unfold. Players can track their performance through leaderboards and head-to-head records.

## Features

- **Lobby**: Create matches, join existing ones, real-time polling with auto-navigation
- **Shop Phase**: Purchase cards, reroll shop (before first buy), upgrade ATK/HP stats, arrange team order
- **Battle Replay**: Watch battles with playback controls, adjustable speeds, event log
- **Stats Tracking**: Leaderboards (ranked/casual), personal profile, match history, head-to-head records
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Accessibility**: ARIA labels, keyboard navigation, reduced motion support

## Quick Start

```bash
# Install dependencies
cd apps/arena-mini-app
pnpm install

# Start development server
pnpm run dev

# Build for production
pnpm run build
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `` (same origin) | API Service base URL |

## Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite with code splitting
- **Styling**: Tailwind CSS + custom CSS
- **Fonts**: Outfit (headings), Nunito (body), JetBrains Mono (stats)
- **Telegram SDK**: @telegram-apps/sdk-react
- **Monitoring**: New Relic Browser Agent

## Game Flow

```
┌─────────┐     ┌────────────┐     ┌────────────┐     ┌─────────┐
│  Lobby  │ ──► │ Shop Phase │ ──► │   Battle   │ ──► │  Stats  │
└─────────┘     └────────────┘     └────────────┘     └─────────┘
  Create/         Buy cards         Watch replay       View
  Join match      Upgrade stats     Event playback     rankings
                  Submit team       Victory screen
```

### Lobby
- Browse and join open matches
- Create new matches
- Auto-navigation when match starts
- 2.5s polling with exponential backoff

### Shop Phase
- Purchase cards from randomized shop (costs coins)
- Reroll shop for new cards (only before first purchase)
- Upgrade cards: +3 ATK or +3 HP per upgrade
- Arrange team order (tap to swap)
- Submit team when ready
- Countdown timer shows remaining time

### Battle
- Automatic playback of battle events
- Three speeds: slow (2s), normal (1s), fast (0.5s)
- Play/pause, restart, skip to end controls
- HP bars animate with color transitions (green > yellow > red)
- Event log shows attacks, deaths, and victory

### Stats
- **Leaderboard**: Ranked/casual toggle, top 50 players
- **Profile**: Personal stats split by ranked/casual
- **History**: Match history with pagination
- **H2H**: Search opponent by ID for head-to-head record

## API Integration

All API calls use JWT authentication obtained through Telegram init_data.

### Authentication Flow

1. Telegram provides `init_data` when launching Mini App
2. App exchanges `init_data` for JWT via `/api/v1/mini-app/auth`
3. Subsequent requests include JWT in `Authorization: Bearer <token>` header

### Endpoints Used

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/mini-app/auth` | Exchange init_data for JWT |
| GET | `/api/v1/mini-app/arena/matches` | List active matches |
| POST | `/api/v1/mini-app/arena/match` | Create new match |
| GET | `/api/v1/mini-app/arena/match/{id}` | Get match details |
| POST | `/api/v1/mini-app/arena/match/{id}/join` | Join match |
| POST | `/api/v1/mini-app/arena/match/{id}/leave` | Leave match |
| POST | `/api/v1/mini-app/arena/match/{id}/start` | Start match early |
| GET | `/api/v1/mini-app/arena/match/{id}/shop` | Get shop state |
| POST | `/api/v1/mini-app/arena/match/{id}/buy` | Purchase card |
| POST | `/api/v1/mini-app/arena/match/{id}/reroll` | Reroll shop |
| POST | `/api/v1/mini-app/arena/match/{id}/upgrade` | Upgrade card |
| POST | `/api/v1/mini-app/arena/match/{id}/order` | Set team order |
| POST | `/api/v1/mini-app/arena/match/{id}/team` | Submit team |
| GET | `/api/v1/mini-app/arena/match/{id}/battle` | Get battle results |
| POST | `/api/v1/mini-app/arena/match/{id}/share` | Share results |
| GET | `/api/v1/mini-app/arena/leaderboard` | Get rankings |
| GET | `/api/v1/mini-app/arena/profile` | Get user stats |
| GET | `/api/v1/mini-app/arena/history` | Get match history |
| GET | `/api/v1/mini-app/arena/h2h` | Get head-to-head record |
| GET | `/api/v1/mini-app/arena/constants` | Get game config |

See [api-service README](../api-service/README.md) for full endpoint documentation.

## Development

```bash
# Install dependencies
pnpm install

# Start development server (hot reload)
pnpm run dev

# Build for production
pnpm run build

# Preview production build
pnpm run preview

# Lint code
pnpm run lint

# Type check
npx tsc --noEmit
```

## Architecture

```
apps/arena-mini-app/
├── src/
│   ├── api/
│   │   └── client.ts           # API client (18+ methods)
│   ├── types/
│   │   └── index.ts            # TypeScript interfaces (30+ types)
│   ├── components/
│   │   ├── common/             # Shared components
│   │   │   ├── TabBar.tsx      # Bottom navigation
│   │   │   ├── Card.tsx        # Card display variants
│   │   │   ├── CountdownTimer.tsx
│   │   │   ├── LoadingSpinner.tsx
│   │   │   └── ErrorDisplay.tsx
│   │   ├── lobby/              # Lobby screen
│   │   │   ├── LobbyPage.tsx
│   │   │   ├── MatchList.tsx
│   │   │   ├── MatchCard.tsx
│   │   │   ├── CreateMatchButton.tsx
│   │   │   └── ParticipantsList.tsx
│   │   ├── shop/               # Shop screen
│   │   │   ├── ShopPage.tsx
│   │   │   ├── ShopCards.tsx
│   │   │   ├── TeamDisplay.tsx
│   │   │   ├── CoinDisplay.tsx
│   │   │   ├── RerollButton.tsx
│   │   │   └── SubmitButton.tsx
│   │   ├── battle/             # Battle screen
│   │   │   ├── BattlePage.tsx
│   │   │   ├── BattleArena.tsx
│   │   │   ├── BattleCard.tsx
│   │   │   ├── EventLog.tsx
│   │   │   ├── EventMessage.tsx
│   │   │   └── VictoryScreen.tsx
│   │   └── stats/              # Stats screen
│   │       └── StatsPage.tsx   # 4 sub-tabs inline
│   ├── styles/
│   │   └── global.css          # Tailwind + custom CSS
│   ├── App.tsx                 # Main app with routing
│   └── main.tsx                # Entry point
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.js
├── postcss.config.js
├── Dockerfile
└── nginx.conf
```

## Bundle Size

The app is optimized for fast loading:

| Chunk | Size (gzip) |
|-------|-------------|
| vendor-react | ~45 KB |
| vendor-telegram | ~13 KB |
| index (main) | ~7 KB |
| LobbyPage | ~2 KB |
| ShopPage | ~4 KB |
| BattlePage | ~4 KB |
| StatsPage | ~3 KB |
| CSS | ~9 KB |
| **Total** | **~87 KB** |

Pages are lazy-loaded for optimal initial load time.

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `arena.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

```bash
# Build Docker image
docker build -f apps/arena-mini-app/Dockerfile -t arena-mini-app .

# Run container
docker run -p 8080:80 arena-mini-app
```

## Troubleshooting

### Match not loading

1. Verify API Service is running
2. Check browser console for errors
3. Ensure user is authenticated (JWT valid)

### Shop actions failing

- Check coin balance (displayed in header)
- Reroll only works before first card purchase
- Upgrade requires sufficient coins and valid card

### Battle not playing

- Verify match is in `battle_phase` status
- Check API response contains battle events
- Try refreshing the page

### Stats not updating

- Stats refresh when tab becomes active
- Pull-to-refresh or switch tabs to force reload
- Check API connectivity

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
