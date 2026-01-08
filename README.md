# Beef Briefing

A Telegram group analytics and gamification platform that transforms chat activity into weekly stats cards, leaderboards, and insights.

## What is Beef Briefing?

Beef Briefing captures and analyzes Telegram group conversations to create a fun, gamified experience for group members. Every week, users receive personalized stats cards showing their communication style, mood, humor level, and social influence within the group.

The platform uses ML models to analyze message sentiment, detect humor patterns, and measure engagement. Results are presented through:

- **Weekly Stats Cards**: Gamified cards with scores for Aura (mood), Activity, Presence, Humor, Toxicity, and Popularity
- **Leaderboard Mini App**: Rankings and activity trends accessible directly in Telegram
- **Card Gallery**: Browse and share generated stats cards

## Features

- **Real-time Message Capture**: Automatically ingests messages, reactions, and media from Telegram groups
- **ML-Powered Analytics**: Sentiment analysis, humor detection, toxicity classification, and entity recognition
- **Gamified Stats Cards**: Beautiful themed cards with customizable designs (gaming, clean, mythic, vaporwave, and more)
- **Telegram Mini Apps**: Native Telegram experience for viewing leaderboards and card galleries
- **Historical Import**: Import existing chat history from Telegram Desktop exports
- **Privacy-Focused**: Self-hosted solution - your data stays on your infrastructure

## Screenshots

<!-- TODO: Add screenshots of stats cards and Mini Apps -->

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend Services | Go 1.25 |
| ML Processing | Python 3.14, PyTorch, OpenAI |
| Frontend | React, TypeScript, Vite |
| Database | PostgreSQL 17 with PostGIS |
| Storage | MinIO / S3 |
| Card Rendering | Playwright, HTML/CSS |
| Infrastructure | Docker, Traefik, Terraform |

## Documentation

| Document | Description |
|----------|-------------|
| [Technical Documentation](apps/README.md) | Architecture, services, and data flow |
| [Infrastructure](infrastructure/README.md) | Docker Compose, deployment, and secrets |
| [Terraform](infrastructure/terraform/README.md) | Linode provisioning |

## Quick Start

```bash
# 1. Configure environment
cp infrastructure/.env.dev.example infrastructure/.env.dev
# Edit .env.dev: set TELEGRAM_BOT_TOKEN

# 2. Generate secrets
make secrets-service-api APP=telegram-bot

# 3. Start services
make up-build

# 4. View logs
make logs
```

See [Technical Documentation](apps/README.md) for detailed setup instructions.

## License

MIT
