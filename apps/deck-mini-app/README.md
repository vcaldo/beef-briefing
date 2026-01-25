# Deck Mini App

Telegram Mini App for browsing weekly user stats card galleries.

## Overview

The Deck Mini App provides a gallery interface for viewing generated stats cards within Telegram. Users can browse cards organized by week, view individual cards in full size with reveal animations, and share them with friends.

## Features

- **Card Gallery**: Browse cards organized by week with masonry layout
- **Card Reveal**: Tap-to-reveal animation when viewing new cards
- **Week Navigation**: Switch between different weeks with dropdown selector
- **Full-Size Viewing**: Tap cards to view in detail
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Responsive Design**: Adapts to Telegram theme (light/dark mode)

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
- **Styling**: CSS with Telegram theme variables
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
│   ├── api/client.ts           # API client with JWT auth
│   ├── types/index.ts          # TypeScript interfaces
│   ├── styles/global.css       # CSS with Telegram theme variables
│   ├── components/
│   │   ├── CardGallery.tsx     # Main gallery grid
│   │   ├── CardImage.tsx       # Individual card display
│   │   ├── CardReveal.tsx      # Reveal animation overlay
│   │   ├── WeekSelector.tsx    # Week dropdown
│   │   └── InfoModal.tsx       # Info/help modal
│   ├── App.tsx                 # Main app component
│   └── main.tsx                # Entry point (SDK init)
├── package.json
├── tsconfig.json
├── vite.config.ts
├── Dockerfile
└── index.html
```

## Screen Flow

```
Week Selection → Card Gallery → Card Detail
      │              │              │
      │              │              └── Full-size card view with actions
      │              └── Tap card to reveal/view, masonry grid
      └── Dropdown to switch weeks
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `deck.{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

### Build Info

- **Bundle size**: ~68KB gzipped (target: <300KB)
- **Single bundle**: No code splitting (small app)

## Troubleshooting

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
- Ensure chat_id is present (must open from group chat context)

### Cards not loading

1. Verify API Service is running
2. Check browser console for errors
3. Ensure JWT token is valid (refresh by reopening Mini App)
4. Verify the chat has generated cards for the selected week

### Week selector empty

1. Check `/api/v1/mini-app/gallery/weeks` returns data
2. Ensure cards have been generated for this chat
3. Verify chat_id in request matches the current chat
