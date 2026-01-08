# Leaderboard Mini App

Telegram Mini App for viewing chat statistics and user leaderboards.

## Overview

The Leaderboard Mini App displays activity statistics and user rankings for Telegram groups. Users can view overview stats, activity trends over time, and compare rankings across different metrics like message count, reactions, and active days.

## Features

- **Overview Stats**: Total messages, reactions, active users
- **Activity Timeline**: Daily message/reaction trends with charts
- **User Rankings**: Leaderboards by various metrics
- **Period Filtering**: View stats for 7d, 30d, 90d, or all time
- **Multiple Metrics**: Rank by messages, reactions sent/received, active days
- **Telegram Integration**: Seamless authentication via Mini App init data

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
- **Charts**: Recharts
- **Telegram SDK**: @telegram-apps/sdk-react

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

### Query Parameters

**Stats & Activity**:
- `chat_id` (required)
- `period`: `7d`, `30d`, `90d`, `all` (default: `30d`)

**Leaderboard**:
- `chat_id` (required)
- `period`: Time period
- `metric`: `message_count`, `reactions_sent`, `reactions_received`, `active_days`
- `page`, `limit`: Pagination

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
│   ├── api/client.ts     # API client with JWT auth
│   ├── types/index.ts    # TypeScript interfaces
│   ├── App.tsx           # Main application
│   └── main.tsx          # Entry point
├── package.json
├── tsconfig.json
├── vite.config.ts
└── Dockerfile
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `leaderboard.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

## Troubleshooting

### Stats not loading

1. Verify API Service is running
2. Check browser console for errors
3. Ensure the chat has message data

### Charts not rendering

- Check Recharts is installed: `npm ls recharts`
- Verify activity data is being returned from API

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
