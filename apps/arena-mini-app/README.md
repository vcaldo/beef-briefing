# Arena Mini App

Telegram Mini App for turn-based card battle arena matches.

## Overview

The Arena Mini App is a competitive card battler where users build teams from weekly stats cards and battle other players. It features real-time match lobbies, a shop phase for team building with upgrades, animated battle replays, and comprehensive statistics tracking.

## Features

- **Match Lobby**: Create/join matches, view participants, auto-start countdowns
- **Shop Phase**: Buy cards, upgrade stats (+3 ATK/HP), reroll shop, build your team
- **Battle Replay**: Animated playback with HP bars, attack indicators, speed controls
- **Statistics**: Leaderboards (ranked/casual), player profiles, match history, head-to-head records
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Responsive Design**: Dark theme optimized for mobile Telegram clients

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
| `VITE_CARD_API_URL` | `` (same origin) | Card Renderer API base URL |
| `VITE_ENVIRONMENT` | `development` | Environment name |

## Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Styling**: Tailwind CSS with custom theme
- **Telegram SDK**: @telegram-apps/sdk-react
- **Fonts**: Outfit (display), Inter (body), JetBrains Mono (stats)

## API Integration

All API calls use JWT authentication obtained through Telegram init_data.

### Authentication Flow

1. Telegram provides `init_data` when launching Mini App
2. App exchanges `init_data` for JWT via `/api/v1/mini-app/auth`
3. Subsequent requests include JWT in `Authorization: Bearer <token>` header

### Endpoints Used

**Match Management**:
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mini-app/arena/matches` | List active matches |
| POST | `/api/v1/mini-app/arena/match` | Create new match |
| GET | `/api/v1/mini-app/arena/match/{id}` | Get match details |
| POST | `/api/v1/mini-app/arena/match/{id}/join` | Join match |
| POST | `/api/v1/mini-app/arena/match/{id}/leave` | Leave match |
| POST | `/api/v1/mini-app/arena/match/{id}/start` | Start match (creator) |

**Shop Phase**:
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mini-app/arena/match/{id}/shop` | Get shop state |
| POST | `/api/v1/mini-app/arena/match/{id}/buy` | Buy card |
| POST | `/api/v1/mini-app/arena/match/{id}/reroll` | Reroll shop |
| POST | `/api/v1/mini-app/arena/match/{id}/upgrade` | Upgrade card |
| POST | `/api/v1/mini-app/arena/match/{id}/order` | Set team order |
| POST | `/api/v1/mini-app/arena/match/{id}/team` | Submit team |

**Battle & Stats**:
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mini-app/arena/match/{id}/battle` | Get battle results |
| GET | `/api/v1/mini-app/arena/leaderboard` | Ranked/casual leaderboard |
| GET | `/api/v1/mini-app/arena/history` | Match history |
| GET | `/api/v1/mini-app/arena/h2h` | Head-to-head records |
| GET | `/api/v1/mini-app/arena/profile` | User profile stats |
| GET | `/api/v1/mini-app/arena/constants` | Game constants |

See [api-service README](../api-service/README.md) for full endpoint documentation.

## Game Economy

| Resource | Cost | Description |
|----------|------|-------------|
| Starting coins | 10 | Coins at match start |
| Card purchase | 3 | Buy a card from shop |
| Reroll | 1 | Refresh shop (before first buy only) |
| Upgrade | 1 | +3 ATK or +3 HP per upgrade |
| Team size | 3 | Cards required for battle |

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
│   ├── api/client.ts           # API client with JWT auth
│   ├── types/index.ts          # TypeScript interfaces
│   ├── styles/global.css       # Tailwind + custom CSS
│   ├── components/
│   │   ├── common/             # Shared components
│   │   │   ├── TabBar.tsx      # Bottom navigation
│   │   │   ├── CountdownTimer.tsx
│   │   │   ├── Card.tsx        # Regular card wrapper
│   │   │   ├── CompactCard.tsx # Battle card with HP bar
│   │   │   ├── LoadingSpinner.tsx
│   │   │   ├── ErrorDisplay.tsx
│   │   │   └── SplashScreen.tsx
│   │   ├── lobby/
│   │   │   └── LobbyPage.tsx   # Match list & creation
│   │   ├── shop/
│   │   │   └── ShopPage.tsx    # Card shop & team building
│   │   ├── battle/
│   │   │   └── BattlePage.tsx  # Battle replay & victory
│   │   └── stats/
│   │       └── StatsPage.tsx   # Leaderboards & profile
│   ├── App.tsx                 # Main app with tab routing
│   └── main.tsx                # Entry point (SDK init)
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.js
├── postcss.config.js
├── Dockerfile
├── nginx.conf
└── index.html
```

## Screen Flow

```
Lobby → Shop → Battle → Stats
  │       │       │       │
  │       │       │       └── Leaderboards, Profile, History, H2H
  │       │       └── Event playback, HP animations, Victory screen
  │       └── Buy cards, Upgrade stats, Submit team
  └── Create/Join matches, Wait for players, Auto-start
```

**Phase Transitions**:
- Lobby polls `/matches` (3s) or `/match/{id}` (2s)
- Auto-navigates to Shop when match status → `shop_phase`
- Shop polls `/shop` (3s), continues after team submission
- Auto-navigates to Battle when status → `battle_phase`

## Compact Card System

The app uses a dual card format:
- **Regular cards** (400x600): Used in shop display
- **Compact cards** (300x450): Used in team display and battle

Compact cards feature dynamic stat overlays positioned using `placeholder_positions` metadata from the card-renderer API. Stats (ATK, DEF, HP) are overlaid at exact pixel coordinates, and HP bars animate with color transitions:
- Green (>66% HP)
- Yellow (33-66% HP)
- Red (<33% HP)

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `arena.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

### Build Info

- **Bundle size**: ~83KB gzipped (target: <300KB)
- **Code splitting**: Lazy-loaded page components
- **Main bundle**: ~67KB gzipped
- **Page chunks**: 2-4KB gzipped each

## Troubleshooting

### React Error #310

This error indicates non-deterministic state initialization. The app prevents this by:
1. `main.tsx` awaits `initializeTelegramSDK()` before rendering
2. `CountdownTimer` initializes state to `0`, calculates time in `useEffect`

If you see this error, check for `Date.now()` or similar in state initializers.

### Match not starting

1. Verify 2+ players have joined
2. Check countdown timer hasn't expired
3. Ensure creator clicks Start or wait for auto-start

### Shop cards not loading

1. Verify API Service and Card Renderer are running
2. Check browser console for CORS or network errors
3. Ensure match is in `shop_phase` status

### Battle not showing

1. Verify all players submitted teams
2. Check match status is `battle_phase`
3. Refresh the tab or re-navigate from Shop

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
- Ensure chat_id is present (must open from group chat context)

### HP bar not animating

- Verify `placeholder_positions` metadata is included in card response
- Check CSS transitions are not being blocked
- Fallback HP bar displays if metadata is missing
