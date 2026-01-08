# Deck Mini App

Telegram Mini App for browsing weekly user stats card galleries.

## Overview

The Deck Mini App provides a gallery interface for viewing generated stats cards within Telegram. Users can browse cards organized by week, view individual cards in full size, and share them with friends.

## Features

- **Card Gallery**: Browse cards organized by week
- **Full-Size Viewing**: Tap cards to view in detail
- **Week Navigation**: Switch between different weeks
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Responsive Design**: Works on mobile and desktop Telegram clients

## Quick Start

```bash
# Install dependencies
cd apps/deck-mini-app
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
| GET | `/api/v1/mini-app/gallery/weeks` | List weeks with cards |
| GET | `/api/v1/mini-app/gallery/images` | Get cards for a week |
| GET | `/api/v1/mini-app/gallery/image/{id}` | Get presigned URL |

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
apps/deck-mini-app/
├── src/
│   ├── api/client.ts     # API client with JWT auth
│   ├── App.tsx           # Main application
│   └── main.tsx          # Entry point
├── package.json
├── tsconfig.json
├── vite.config.ts
└── Dockerfile
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `deck.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

## Troubleshooting

### Cards not loading

1. Verify API Service is running
2. Check browser console for errors
3. Ensure JWT token is valid (refresh by reopening Mini App)

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
