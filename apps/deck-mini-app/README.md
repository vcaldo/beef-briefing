# Deck Mini App

A Telegram Mini App for viewing user card galleries. Users can browse weekly generated stat cards for their chat group.

## Features

- **Card Gallery**: Browse card images organized by week
- **Telegram Integration**: Seamless authentication via Telegram Mini App init data
- **Responsive Design**: Works on mobile and desktop Telegram clients

## Architecture

### Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
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
| GET | `/api/v1/mini-app/gallery/weeks` | List weeks with available cards |
| GET | `/api/v1/mini-app/gallery/images` | Get card images for a week |
| GET | `/api/v1/mini-app/gallery/image/{id}` | Get presigned URL for card image |

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
apps/deck-mini-app/
├── src/
│   ├── api/
│   │   └── client.ts          # API client with JWT auth
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
