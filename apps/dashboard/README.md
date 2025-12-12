# Beef Dashboard

Analytics dashboard for Telegram group data with interactive visualizations.

## Overview

A Python-based dashboard built with Dash (Plotly) that provides:
- Interactive data visualizations (heatmaps, time series, distributions)
- User leaderboards and statistics
- Telegram OAuth authentication
- Group membership verification
- Responsive design with dark theme

## Technology Stack

- **Framework**: Dash (Plotly) with Flask
- **Database**: PostgreSQL (read-only access)
- **Charts**: Plotly.js
- **Authentication**: Telegram Login Widget
- **Styling**: Custom CSS with CSS variables

## Quick Start

### Development

1. Generate secrets:
```bash
make generate-dashboard-secrets
```

2. Configure environment (add to `.env.dev`):
```bash
DASHBOARD_PORT=8050
ALLOWED_CHAT_IDS=-1001234567890  # Your Telegram group ID
SESSION_LIFETIME_DAYS=7
```

3. Start services:
```bash
make up-build
```

4. Access dashboard at: http://localhost:8050/beef-dashboard/

### Production

1. Generate secrets:
```bash
make generate-dashboard-secrets
```

2. Configure `.env.prod` with:
```bash
DASHBOARD_PORT=8050
ALLOWED_CHAT_IDS=-1001234567890
SESSION_LIFETIME_DAYS=7
```

3. Deploy:
```bash
make deploy
```

4. Access at: `https://yourdomain.com/beef-dashboard/`

## Authentication Flow

1. User visits `/beef-dashboard/`
2. If not authenticated, redirected to login page
3. User clicks Telegram Login Widget
4. Telegram confirms identity
5. Dashboard validates auth hash (HMAC-SHA256)
6. Dashboard verifies group membership via Bot API
7. Session created (7-day lifetime)
8. User redirected to dashboard

## Features

### Overview Cards
- Total messages in period
- Active users count
- Total reactions
- Media files shared

### Visualizations
- **Message Timeline**: Line chart with messages and active users over time
- **Activity Heatmap**: Hour-by-day activity patterns
- **Top Reactions**: Bar chart of most used reactions
- **Media Distribution**: Pie chart of media types (photo, video, etc.)
- **User Leaderboard**: Top contributors with message counts, active days, reactions

### Period Selection
- Today
- Last 7 days (default)
- Last 30 days
- Last 90 days (quarter)
- Custom date range

## Project Structure

```
apps/dashboard/
├── Dockerfile              # Multi-stage Python build
├── requirements.txt        # Python dependencies
├── README.md              # This file
├── src/
│   ├── __init__.py
│   ├── main.py            # Entry point (gunicorn compatible)
│   ├── config.py          # Configuration from environment
│   ├── app.py             # Dash application factory
│   ├── auth/
│   │   ├── telegram_oauth.py   # Telegram Login Widget validation
│   │   ├── membership.py       # Group membership verification
│   │   └── session.py          # PostgreSQL session management
│   ├── database/
│   │   ├── connection.py       # SQLAlchemy connection
│   │   └── queries.py          # Analytics queries
│   ├── components/
│   │   └── layout.py           # Dashboard layout components
│   ├── callbacks/
│   │   └── dashboard_callbacks.py  # Dash interactivity
│   └── assets/
│       ├── styles.css          # Main stylesheet
│       └── custom.js           # Animation helpers
└── tests/
    └── __init__.py
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DASHBOARD_PORT` | HTTP port | `8050` |
| `DASHBOARD_DOMAIN` | Domain for OAuth callback | `localhost` |
| `TELEGRAM_BOT_TOKEN` | Bot token for API calls | Required |
| `ALLOWED_CHAT_IDS` | Comma-separated chat IDs | Required |
| `SESSION_LIFETIME_DAYS` | Session duration | `7` |
| `FLASK_SECRET_KEY_FILE` | Path to Flask secret | Required in prod |
| `DB_HOST`, `DB_USER`, etc. | Database connection | From main config |

## Database

The dashboard requires:
1. Read access to main tables (`messages`, `users`, `message_reactions`, etc.)
2. Write access to `dashboard_sessions` table

Materialized views (optional, for performance):
- `mv_daily_message_stats`
- `mv_user_statistics`
- `mv_hourly_activity`
- `mv_media_distribution`
- `mv_reaction_distribution`

Run migration: `apps/postgres/migrations/002_dashboard.sql`

Refresh views:
```sql
SELECT refresh_dashboard_views();
```

## Makefile Targets

```bash
make build-dashboard        # Build Docker image
make logs-dashboard         # Tail service logs
make shell-dashboard        # Shell into container
make generate-dashboard-secrets  # Generate Flask secret
```

## UX Design

- **Theme**: Dark analytics with cyan/coral accents
- **Typography**: JetBrains Mono (headings), Space Grotesk (body)
- **Animations**: Staggered fade-in, smooth transitions
- **Responsive**: Mobile-first, breakpoints at 768px and 1024px

## Monitoring

New Relic APM is supported. Set environment variables:
```bash
NEW_RELIC_APP_NAME=beef-briefing
NEW_RELIC_LICENSE_KEY=your_key
```

## Troubleshooting

### Login not working
- Check `TELEGRAM_BOT_TOKEN` is set
- Verify bot username in Telegram Login Widget config
- Check `DASHBOARD_DOMAIN` matches your domain

### No data showing
- Verify `ALLOWED_CHAT_IDS` contains correct chat IDs
- Check database connection
- Ensure chat has messages in selected date range

### Session expires immediately
- Check `FLASK_SECRET_KEY_FILE` exists and is readable
- Verify `SESSION_LIFETIME_DAYS` is set correctly
