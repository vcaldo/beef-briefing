# [App Name] Mini App

Telegram Mini App for [one-line purpose description].

## Overview

[2-3 sentences describing the Mini App's purpose, key functionality, and user value.]

## Features

- **[Feature Name]**: [Brief description]
- **[Feature Name]**: [Brief description]
- **[Feature Name]**: [Brief description]
- **Telegram Integration**: Seamless authentication via Mini App init data
- **Responsive Design**: Dark theme optimized for mobile Telegram clients

## Quick Start

```bash
# Install dependencies
cd apps/[app-name]
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
| `VITE_ENVIRONMENT` | `development` | Environment name |

## Tech Stack

- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Styling**: Tailwind CSS with custom theme
- **Telegram SDK**: @telegram-apps/sdk-react
- **Fonts**: [List primary fonts used]

## API Integration

All API calls use JWT authentication obtained through Telegram init_data.

### Authentication Flow

1. Telegram provides `init_data` when launching Mini App
2. App exchanges `init_data` for JWT via `/api/v1/mini-app/auth`
3. Subsequent requests include JWT in `Authorization: Bearer <token>` header

### Endpoints Used

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mini-app/[endpoint]` | [Description] |
| POST | `/api/v1/mini-app/[endpoint]` | [Description] |

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
apps/[app-name]/
├── src/
│   ├── api/client.ts           # API client with JWT auth
│   ├── types/index.ts          # TypeScript interfaces
│   ├── styles/global.css       # Tailwind + custom CSS
│   ├── components/
│   │   ├── common/             # Shared components
│   │   └── [feature]/          # Feature-specific components
│   ├── App.tsx                 # Main app component
│   └── main.tsx                # Entry point (SDK init)
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.js
├── Dockerfile
└── index.html
```

## Screen Flow

_Include ASCII diagram showing navigation between screens._

```
Screen A → Screen B → Screen C
    │          │          │
    │          │          └── [What this screen shows]
    │          └── [What this screen shows]
    └── [What this screen shows]
```

## Deployment

The Mini App is deployed as a static site behind Traefik:
- **Route**: `[subdomain].{domain}`
- **API**: Calls `api.{domain}` for data

The Docker build process compiles the React app and serves it via nginx.

### Build Info

- **Bundle size**: ~[XX]KB gzipped (target: <300KB)
- **Code splitting**: [Strategy used]

## Troubleshooting

### Authentication fails

- Check `VITE_API_URL` points to correct API
- Verify the Mini App is launched from Telegram (not standalone browser)
- Check API Service logs for init_data validation errors
- Ensure chat_id is present (must open from group chat context)

### [Component] not loading

1. [Diagnostic step]
2. [Diagnostic step]
3. [Solution]

---

## Template Usage Guide

_Delete this section when using the template._

### Required Sections for Mini Apps

1. **Title + One-liner**: Clear app name and purpose
2. **Overview**: 2-3 sentences on user value
3. **Features**: Include Telegram Integration and Responsive Design
4. **Quick Start**: pnpm-based commands
5. **Configuration**: VITE_ prefixed environment variables
6. **Tech Stack**: Framework, build tool, styling, SDK, fonts
7. **API Integration**: Auth flow + endpoints table
8. **Development**: Common pnpm commands
9. **Architecture**: src/ structure with components breakdown
10. **Screen Flow**: User navigation diagram
11. **Deployment**: Route info and bundle size
12. **Troubleshooting**: Auth issues first, then feature-specific

### Telegram Mini App Specifics

Always document:
- How authentication works with init_data
- Which chat context is required (group vs private)
- Polling intervals for real-time updates
- Any SDK initialization requirements

### Common Troubleshooting Items

Include these for every Mini App:
- Authentication failures (init_data issues)
- CORS errors
- Missing chat_id context
- Network/connectivity issues
