# Leaderboard Mini App

Telegram Mini App for viewing chat statistics and user leaderboards.

## Overview

The Leaderboard Mini App displays activity statistics and user rankings for Telegram groups. Users can view overview stats, activity trends over time, and compare rankings across different metrics like message count, reactions, and active days.

## Features

- **Overview Stats**: Total messages, reactions, active users with trend indicators
- **Activity Timeline**: Daily message/reaction trends with interactive charts
- **User Rankings**: Leaderboards by various metrics with pagination
- **Media Statistics**: Breakdown by type (photos, videos, GIFs, voice, docs, stickers) with pie chart, timeline, and top senders
- **Interactions**: Reply and reaction network analysis between users
- **Period Filtering**: View stats for 7d, 30d, 90d, or all time
- **User Profiles**: Individual user statistics and activity heatmaps
- **Admin Panel**: Group management for administrators
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Responsive Design**: Adapts to Telegram theme (light/dark mode)

## Quick Start

```bash
# Install dependencies
cd apps/leaderboard-mini-app
npm install

# Start development server
npm run dev

# Build for production
npm run build
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `` (same origin) | API Service base URL |

## Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Styling**: CSS with Telegram theme variables
- **Charts**: Recharts (area, pie, bar charts)
- **Telegram SDK**: @telegram-apps/sdk-react
- **Fonts**: System font stack (Apple, Segoe UI, Roboto)

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
| GET | `/api/v1/mini-app/stats` | Get chat overview |
| GET | `/api/v1/mini-app/activity` | Get daily activity |
| GET | `/api/v1/mini-app/leaderboard` | Get user rankings |
| GET | `/api/v1/mini-app/media-overview` | Get media statistics |

### Query Parameters

**Stats & Activity**:
- `chat_id` (required)
- `period`: `7d`, `30d`, `90d`, `all` (default: `30d`)

**Leaderboard**:
- `chat_id` (required)
- `period`: Time period
- `metric`: `message_count`, `reactions_sent`, `reactions_received`, `active_days`
- `page`, `limit`: Pagination

**Media Overview**:
- `chat_id` (required)
- `period`: `7d`, `30d`, `90d`, `all` (default: `30d`)
- `limit`: Top senders count (default: `10`, max: `50`)

See [api-service README](../api-service/README.md) for full endpoint documentation.

## Development

```bash
# Install dependencies
npm install

# Start development server (hot reload)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Lint code
npm run lint

# Type check
npx tsc --noEmit
```

## Architecture

```
apps/leaderboard-mini-app/
├── src/
│   ├── api/client.ts              # API client with JWT auth
│   ├── types/index.ts             # TypeScript interfaces
│   ├── styles/global.css          # CSS with Telegram theme variables
│   ├── components/
│   │   ├── common/
│   │   │   ├── TabBar.tsx         # Bottom navigation
│   │   │   ├── TrendIndicator.tsx # Up/down trend arrows
│   │   │   └── HeatmapGrid.tsx    # Activity heatmap
│   │   ├── home/
│   │   │   └── HomePage.tsx       # Overview dashboard
│   │   ├── leaderboard/
│   │   │   └── LeaderboardPage.tsx
│   │   ├── media/
│   │   │   └── MediaPage.tsx      # Media statistics
│   │   ├── interactions/
│   │   │   └── InteractionsPage.tsx
│   │   ├── card/
│   │   │   └── CardPage.tsx       # User card view
│   │   ├── profile/
│   │   │   └── ProfilePage.tsx    # User profile
│   │   ├── admin/
│   │   │   └── AdminPage.tsx      # Admin controls
│   │   ├── OverviewStats.tsx      # Stats summary cards
│   │   ├── ActivityChart.tsx      # Timeline chart
│   │   ├── LeaderboardTable.tsx   # Rankings table
│   │   └── PeriodSelector.tsx     # Time period filter
│   ├── App.tsx                    # Main app with tab routing
│   └── main.tsx                   # Entry point (SDK init)
├── package.json
├── tsconfig.json
├── vite.config.ts
├── Dockerfile
└── index.html
```

## Screen Flow

```
Home → Leaderboard → Media → Interactions → Profile
  │        │           │          │           │
  │        │           │          │           └── User stats, heatmap, cards
  │        │           │          └── Reply/reaction networks
  │        │           └── Media breakdown, top senders
  │        └── Rankings by metric, pagination
  └── Overview stats, activity chart, quick rankings

Tab navigation allows direct access to any section.
Period selector (7d/30d/90d/all) persists across tabs.
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `leaderboard.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

### Build Info

- **Bundle size**: ~182KB gzipped (target: <300KB)
- **Single bundle**: Includes Recharts (~100KB contribution)

## Troubleshooting

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
- Ensure chat_id is present (must open from group chat context)

### Stats not loading

1. Verify API Service is running
2. Check browser console for errors
3. Ensure the chat has message data
4. Verify chat_id in request matches the current chat

### Charts not rendering

- Check Recharts is installed: `npm ls recharts`
- Verify activity data is being returned from API
- Check for JavaScript errors in browser console

### Period selector not working

1. Verify the period parameter is being passed to API
2. Check that data exists for the selected period
3. Try switching to "all" period to verify data exists
