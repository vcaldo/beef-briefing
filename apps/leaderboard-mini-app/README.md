# Leaderboard Mini App

A Telegram Mini App for viewing chat statistics and user leaderboards. Users can see activity trends, message counts, and rankings for their chat group.

## Features

- **Overview Stats**: Total messages, reactions, active users
- **Activity Timeline**: Daily message/reaction trends with charts
- **User Leaderboard**: Rankings by various metrics (messages, reactions sent/received, active days)
- **Period Filtering**: View stats for different time periods
- **Telegram Integration**: Seamless authentication via Telegram Mini App init data

## Architecture

### Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Charts**: Recharts
- **Telegram SDK**: @telegram-apps/sdk-react

### API Integration

All API calls go through the central api-service with JWT authentication.

**Authentication Flow**:
1. Telegram provides `init_data` when launching the Mini App
2. Mini App exchanges `init_data` for JWT token via `/api/v1/mini-app/auth`
3. All subsequent requests include JWT in `Authorization: Bearer <token>` header

## API Endpoints Used

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/mini-app/auth` | Exchange Telegram init_data for JWT |
| GET | `/api/v1/mini-app/stats` | Get chat overview statistics |
| GET | `/api/v1/mini-app/activity` | Get daily activity timeline |
| GET | `/api/v1/mini-app/leaderboard` | Get user rankings |

### Query Parameters

**Stats & Activity Endpoints**:
- `chat_id` (required): Chat to query
- `period` (optional): Time period - `7d`, `30d`, `90d`, `all` (default: `30d`)

**Leaderboard Endpoint**:
- `chat_id` (required): Chat to query
- `period` (optional): Time period (default: `30d`)
- `metric` (optional): Ranking metric (default: `message_count`)
  - `message_count` - Total messages sent
  - `reactions_sent` - Reactions given
  - `reactions_received` - Reactions received
  - `active_days` - Days with activity
- `page` (optional): Page number (default: 1)
- `limit` (optional): Results per page (default: 20, max: 100)

See [api-service README](../api-service/README.md) for full endpoint documentation.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Base URL for api-service | `` (same origin) |

### Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Lint code
npm run lint
```

## Project Structure

```
apps/leaderboard-mini-app/
├── src/
│   ├── api/
│   │   └── client.ts          # API client with JWT auth
│   ├── types/
│   │   └── index.ts           # TypeScript interfaces
│   ├── App.tsx                # Main application component
│   └── main.tsx               # Entry point
├── package.json
├── tsconfig.json
├── vite.config.ts
└── README.md
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `leaderboard.{domain}` (serves Mini App static files)
- **API**: Calls api-service at `api.{domain}` for data

### Docker Build

The app is built during the Docker image build process and served via nginx. See `Dockerfile` for build configuration.
