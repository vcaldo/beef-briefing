# Beef Briefing Documentation

Welcome to the Beef Briefing documentation. This index organizes all project documentation by category for easy navigation.

## Quick Navigation

| I want to... | Go to |
|--------------|-------|
| Get started with the project | [Quick Start](#quick-start) |
| Understand the architecture | [Technical Documentation](../apps/README.md) |
| Deploy to production | [Infrastructure Guide](../infrastructure/README.md) |
| Run the ML pipeline | [ML Quickstart](ML_QUICKSTART.md) |
| Create custom card themes | [Theme Creation Guide](theme-creation-guide.md) |
| Understand the scoring system | [How Scoring Works](HOW_SCORING_WORKS.md) |
| Work with Claude AI | [CLAUDE.md](../CLAUDE.md) |
| Write tests | [Testing Guidelines](../TESTING_GUIDELINES.md) |

---

## Getting Started

| Document | Description |
|----------|-------------|
| [Project README](../README.md) | Overview, features, and quick start |
| [Technical Documentation](../apps/README.md) | Architecture, services, and data flow |
| [Infrastructure](../infrastructure/README.md) | Docker Compose, deployment, and secrets |

---

## Services Documentation

### Backend Services (Go)

| Service | Description |
|---------|-------------|
| [api-service](../apps/api-service/README.md) | Central REST API for data ingestion, Mini App endpoints, and Arena game |
| [telegram-bot](../apps/telegram-bot/README.md) | Real-time Telegram message listener with concurrent media downloads |
| [import-cli](../apps/import-cli/README.md) | CLI tool for importing Telegram Desktop exports |

### Backend Services (Python)

| Service | Description |
|---------|-------------|
| [card-renderer](../apps/card-renderer/README.md) | Gamified stats card image generator with theme system |
| [ml-processor](../apps/ml-processor/README.md) | ML analytics pipeline (sentiment, humor, toxicity) |
| [ml-dashboard](../apps/ml-dashboard/README.md) | Analytics dashboard with FastAPI backend |

### Telegram Mini Apps (React/TypeScript)

| Service | Description |
|---------|-------------|
| [arena-mini-app](../apps/arena-mini-app/README.md) | Turn-based card battle arena game |
| [deck-mini-app](../apps/deck-mini-app/README.md) | Card gallery for browsing weekly stats cards |
| [leaderboard-mini-app](../apps/leaderboard-mini-app/README.md) | Stats leaderboard and user profiles |

---

## Feature Documentation

### Game Systems

| Document | Description |
|----------|-------------|
| [Arena Game API](../apps/api-service/internal/game/README.md) | Complete Arena game endpoint documentation |
| [Ranked Tournaments](../apps/arena-mini-app/ranked.md) | Tournament system, scheduling, and configuration |
| [How Scoring Works](HOW_SCORING_WORKS.md) | Stats calculation (Aura, Activity, Humor, etc.) |

### Card System

| Document | Description |
|----------|-------------|
| [Theme Creation Guide](theme-creation-guide.md) | Create custom card themes with JSON and HTML/CSS |
| [Card Generation](../apps/ml-processor/CARD_GENERATION.md) | Detailed formulas and calculations for card stats |
| [Image System](../apps/arena-mini-app/image-system.md) | Arena mini-app image/asset system |
| [Sound System](../apps/arena-mini-app/sound-system.md) | Arena mini-app audio effects |

### ML Pipeline

| Document | Description |
|----------|-------------|
| [ML Quickstart](ML_QUICKSTART.md) | Step-by-step guide to run the ML pipeline |
| [ML Architecture](../apps/ml-processor/ARCHITECTURE.md) | Technical deep-dive into analyzers and data flow |
| [Cost Estimation](../apps/ml-processor/COST_ESTIMATION.md) | OpenAI API cost analysis |

---

## Infrastructure Documentation

| Document | Description |
|----------|-------------|
| [Infrastructure Overview](../infrastructure/README.md) | Docker Compose services, network architecture, commands |
| [Terraform Guide](../infrastructure/terraform/README.md) | Linode provisioning, DNS, SSL certificates |
| [Secrets Management](../infrastructure/secrets/README.md) | API keys, authentication, file permissions |
| [Cleanup Scripts](../infrastructure/scripts/CLEANUP_README.md) | Game arena data cleanup for migrations |

### Environment Configuration

| File | Description |
|------|-------------|
| [.env.dev.example](../infrastructure/.env.dev.example) | Development environment variables template |
| [.env.prod.example](../infrastructure/.env.prod.example) | Production environment variables template |

---

## Development Guides

| Document | Description |
|----------|-------------|
| [Testing Guidelines](../TESTING_GUIDELINES.md) | Testing standards, patterns, and best practices |
| [Code Style Guide](../agents.md) | Go and Python code style guidelines |
| [CLAUDE.md](../CLAUDE.md) | AI assistant guidance and project reference |

---

## API Testing

| Document | Description |
|----------|-------------|
| [Card Renderer Position API](../apps/card-renderer/TEST_POSITION_API.md) | Test placeholder positions for compact cards |

---

## Documentation Templates

Templates for creating new documentation following project standards:

| Template | Use For |
|----------|---------|
| [Service README Template](TEMPLATE_SERVICE_README.md) | Backend services (Go, Python) |
| [Mini App README Template](TEMPLATE_MINI_APP_README.md) | Telegram Mini Apps (React/TypeScript) |
| [Infrastructure README Template](TEMPLATE_INFRASTRUCTURE_README.md) | Infrastructure components |

---

## Quick Start

```bash
# 1. Clone and configure
git clone <repo>
cd beef-briefing
cp infrastructure/.env.dev.example infrastructure/.env.dev
# Edit .env.dev with your TELEGRAM_BOT_TOKEN

# 2. Generate secrets
make secrets-service-api APP=telegram-bot

# 3. Start all services
make up-build

# 4. View logs
make logs
```

For detailed setup instructions, see [Technical Documentation](../apps/README.md).

---

## Maintenance Guidelines

When adding new documentation:

1. **Choose the right location:**
   - Service-specific docs → `apps/{service}/` directory
   - Cross-cutting guides → `docs/` directory
   - Infrastructure docs → `infrastructure/` directory

2. **Use templates:** Copy the appropriate template from `docs/TEMPLATE_*.md`

3. **Update this index:** Add a link to any new documentation file

4. **Link from related docs:** Add cross-references from existing documentation

5. **Verify links:** Ensure all relative paths are correct

---

## Document Map

```
beef-briefing/
├── README.md                    # Project overview
├── CLAUDE.md                    # AI assistant guidance
├── TESTING_GUIDELINES.md        # Testing standards
├── agents.md                    # Code style guidelines
├── docs/
│   ├── README.md                # This index
│   ├── HOW_SCORING_WORKS.md     # Scoring system
│   ├── ML_QUICKSTART.md         # ML pipeline guide
│   ├── theme-creation-guide.md  # Card themes
│   └── TEMPLATE_*.md            # Documentation templates
├── apps/
│   ├── README.md                # Architecture overview
│   ├── api-service/
│   │   ├── README.md            # API service docs
│   │   └── internal/game/README.md  # Arena game API
│   ├── telegram-bot/README.md
│   ├── card-renderer/
│   │   ├── README.md
│   │   └── TEST_POSITION_API.md
│   ├── ml-processor/
│   │   ├── README.md
│   │   ├── ARCHITECTURE.md
│   │   ├── CARD_GENERATION.md
│   │   └── COST_ESTIMATION.md
│   ├── ml-dashboard/README.md
│   ├── arena-mini-app/
│   │   ├── README.md
│   │   ├── ranked.md
│   │   ├── image-system.md
│   │   └── sound-system.md
│   ├── deck-mini-app/README.md
│   ├── leaderboard-mini-app/README.md
│   └── import-cli/README.md
└── infrastructure/
    ├── README.md                # Infrastructure overview
    ├── terraform/README.md      # Terraform guide
    ├── secrets/README.md        # Secrets management
    └── scripts/CLEANUP_README.md
```
