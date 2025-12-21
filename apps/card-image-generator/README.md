# Card Image Generator

A Python service that renders gamified user stats cards as PNG images using HTML/CSS templates and Playwright.

## Overview

This service generates static card images from user stats data stored in the `ml_user_cards` table. It uses:

- **Playwright** for headless Chromium rendering (HTML/CSS to PNG)
- **Jinja2** for HTML template rendering
- **FastAPI** for the internal REST API
- **MinIO/S3** for image storage

## Architecture

```
ml_user_cards (PostgreSQL)
        │
        ▼
┌───────────────────────────────────────────────────────┐
│           Card Image Generator Service                │
│                                                       │
│  ┌─────────────┐    ┌────────────────────────────┐   │
│  │   Queries   │───▶│  TemplateLoader            │   │
│  │ (ml_user_   │    │  - Transform stats to ctx  │   │
│  │  cards)     │    │  - Derive badges           │   │
│  └─────────────┘    │  - Render Jinja2 → HTML    │   │
│                     └────────────────────────────┘   │
│                                  │                   │
│                                  ▼                   │
│                     ┌────────────────────────────┐   │
│                     │  PlaywrightRenderer        │   │
│                     │  - HTML → PNG              │   │
│                     │  - 2x scale for retina     │   │
│                     └────────────────────────────┘   │
│                                  │                   │
│                                  ▼                   │
│  ┌─────────────┐    ┌────────────────────────────┐   │
│  │ Repository  │◀───│  CardStorageClient         │   │
│  │ (ml_user_   │    │  - Upload to MinIO/S3      │   │
│  │ card_images)│    │  - Presigned URLs          │   │
│  └─────────────┘    └────────────────────────────┘   │
└───────────────────────────────────────────────────────┘
```

## API Endpoints

All endpoints (except `/health`) require authentication via `Authorization: Bearer <key>` header.

### POST /api/v1/render

Trigger card image generation for a chat/week.

**Request:**
```json
{
  "chat_id": -1003280306634,
  "week_start": "2025-12-09",
  "user_ids": [123, 456],       // optional: specific users
  "theme": "gaming",            // optional (default: gaming)
  "force_regenerate": false     // optional: regenerate existing
}
```

**Response:**
```json
{
  "generated": 15,
  "skipped": 3,
  "failed": 0,
  "results": [
    {
      "user_id": 123,
      "status": "generated",
      "image_id": 42,
      "storage_path": "cards/-1003280306634/2025-12-09/123.png"
    }
  ]
}
```

### GET /api/v1/images

List card images for a chat/week.

**Query Parameters:**
- `chat_id` (required): Chat ID
- `week_start` (optional): Week start date (YYYY-MM-DD)
- `user_id` (optional): Filter by user
- `theme` (optional): Filter by theme

### GET /api/v1/image/{image_id}

Get presigned URL for a specific image.

**Query Parameters:**
- `expires` (optional): URL expiry in seconds (60-86400, default: 3600)

### GET /health

Health check (unauthenticated).

## Template System

Templates use Jinja2 with a layered approach (image assets + HTML/CSS).

### Directory Structure

```
templates/
└── themes/
    └── gaming/
        ├── card.html        # Main Jinja2 template
        └── assets/          # Static assets (optional)
            ├── background.png
            ├── frame.png
            └── badges/
                └── *.png
```

### Template Variables

```jinja2
{# User Info #}
{{ user.first_name }}
{{ user.last_name }}
{{ user.username }}
{{ user.initials }}
{{ user.photo_url }}

{# Week Info #}
{{ week_start }}        {# "2025-12-09" #}
{{ week_end }}          {# "2025-12-15" #}
{{ week_number }}       {# 50 #}
{{ period_display }}    {# "Dec 09 - Dec 15" #}
{{ rank }}              {# 1 (if provided) #}

{# Stats (list of StatContext) #}
{% for stat in stats %}
  {{ stat.key }}          {# "mood", "comedy", etc. #}
  {{ stat.label }}        {# "Mood", "Comedy" #}
  {{ stat.icon }}         {# emoji #}
  {{ stat.value }}        {# raw value #}
  {{ stat.percentage }}   {# 0-100 for progress bars #}
  {{ stat.display_value }}{# formatted: "72" or "45%" #}
  {% if stat.trend %}
    {{ stat.trend.direction }} {# "up", "down", "stable" #}
    {{ stat.trend.icon }}      {# arrow emoji #}
    {{ stat.trend.delta }}     {# "+5" or "-3" #}
  {% endif %}
{% endfor %}

{# Direct stat access #}
{{ mood.score }}
{{ comedy.score }}
{{ volatility.score }}
{{ toxicity.pct }}
{{ chronotype.type }}
{{ reactions_received }}

{# Activity summary #}
{{ activity.messages }}
{{ activity.active_days }}
{{ activity.avg_length }}

{# Derived Badges (max 4) #}
{% for badge in badges %}
  {{ badge.key }}    {# "night_owl", "comedian" #}
  {{ badge.name }}   {# "Night Owl" #}
  {{ badge.icon }}   {# emoji #}
  {{ badge.rarity }} {# "common", "rare", "epic", "legendary" #}
{% endfor %}
```

### Available Badges

Badges are derived from stats automatically:

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Night Owl | chronotype = "Coruja" | rare |
| Early Bird | chronotype = "Madrugador" | rare |
| Ray of Sunshine | mood >= 90 | legendary |
| Optimist | mood >= 75 | epic |
| Stand-Up King | comedy >= 70% | legendary |
| Class Clown | comedy >= 50% | epic |
| Chatterbox | messages >= 500 | legendary |
| Active Voice | messages >= 200 | epic |
| Regular | messages >= 100 | rare |
| Zen Master | toxicity < 1% | legendary |
| Peacekeeper | toxicity < 5% | epic |
| Beloved | reactions >= 100 | legendary |
| Popular | reactions >= 50 | epic |

## Configuration

Environment variables (all have defaults for development):

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | (empty) | PostgreSQL password |
| `DB_NAME` | beef_briefing | Database name |
| `MINIO_ENDPOINT` | localhost:9000 | MinIO/S3 endpoint |
| `MINIO_ACCESS_KEY` | minioadmin | Access key |
| `MINIO_SECRET_KEY` | minioadmin | Secret key |
| `MINIO_BUCKET` | telegram-media | Bucket name |
| `MINIO_USE_SSL` | false | Use SSL for MinIO |
| `CARD_GENERATOR_PORT` | 8051 | Service port |
| `TEMPLATES_DIR` | /app/templates | Templates directory |
| `DEFAULT_THEME` | gaming | Default theme name |
| `CARD_WIDTH` | 400 | Card width in pixels |
| `CARD_HEIGHT` | 600 | Card height in pixels |
| `CARD_SCALE` | 2 | Render scale (2 = retina) |
| `APP_KEYS_DIR` | /app/secrets/app_keys | API keys directory |
| `ENVIRONMENT` | development | Environment name |
| `LOG_LEVEL` | info | Logging level |

## Usage

### Via make targets

```bash
# Generate card data first
make ml-run-cards ML_ARGS="--week 2025-12-09 --timezone America/Sao_Paulo"

# Render card images
make ml-run-render ML_ARGS="--week 2025-12-09"

# Force re-render all images
make ml-run-render ML_ARGS="--week 2025-12-09 --force"

# Render with specific theme
make ml-run-render ML_ARGS="--week 2025-12-09 --theme gaming"
```

### Direct API call

```bash
# Set API key
API_KEY=$(cat infrastructure/secrets/apps/ml-processor/api_key)

# Render cards
curl -X POST http://localhost:8051/api/v1/render \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1003280306634,
    "week_start": "2025-12-09",
    "theme": "gaming"
  }'

# Get image URL
curl "http://localhost:8051/api/v1/image/42?expires=3600" \
  -H "Authorization: Bearer $API_KEY"
```

## Storage

Images are stored with the path pattern:
```
cards/{chat_id}/{week_start}/{user_id}.png
```

Example: `cards/-1003280306634/2025-12-09/123456789.png`

## Database

Uses `ml_user_card_images` table (created by migration 007):

```sql
CREATE TABLE ml_user_card_images (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES ml_user_cards(id),
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    week_start DATE NOT NULL,
    storage_path TEXT NOT NULL,
    file_hash VARCHAR(64) NOT NULL,
    file_size INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    theme VARCHAR(32) NOT NULL,
    template_version INTEGER NOT NULL,
    card_data_version INTEGER NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(card_id, theme)
);
```

## Development

### Local setup

```bash
# Start dependencies
make up-build

# Generate API key
make secrets-service-api APP=card-image-generator

# The service runs automatically via docker-compose
```

### Accessing images via api-service

The api-service exposes card images externally:

```bash
# Get presigned URL for a user's card image
curl "http://localhost:8080/api/v1/cards/123456/image?chat_id=-1003280306634" \
  -H "Authorization: Bearer $API_KEY"
```

Response:
```json
{
  "image_id": 42,
  "url": "https://...",
  "expires_in": 3600,
  "width": 800,
  "height": 1200,
  "theme": "gaming",
  "week_start": "2025-12-09",
  "generated_at": "2025-12-10T12:00:00Z"
}
```

## Creating Custom Themes

Themes consist of two files: a `theme.json` configuration file and a `card.html` Jinja2 template.

### Directory Structure

```
templates/themes/
├── gaming/              # Dark neon gaming theme (default)
│   ├── theme.json       # Color and typography config
│   └── card.html        # HTML/CSS template
├── clean/               # Light minimal theme
│   ├── theme.json
│   └── card.html
└── mytheme/             # Your custom theme
    ├── theme.json
    └── card.html
```

### Step-by-Step Guide

**1. Create theme directory:**

```bash
mkdir -p apps/card-image-generator/templates/themes/mytheme
```

**2. Create `theme.json`:**

The theme configuration defines colors and typography. All colors in CSS can reference these values via template variables.

```json
{
  "name": "mytheme",
  "colors": {
    "background_gradient": ["#1a1a2e", "#16213e", "#0f3460"],
    "primary_accent": "#00d9ff",
    "secondary_accent": "#e94560",
    "text_primary": "#ffffff",
    "text_secondary": "rgba(255, 255, 255, 0.6)",
    "border_color": "rgba(255, 255, 255, 0.1)",
    "stat_colors": {
      "vibe": "#fbbf24",
      "vibe_gradient": ["#f59e0b", "#fcd34d"],
      "activity": "#00d9ff",
      "activity_gradient": ["#00d9ff", "#67e8f9"],
      "presence": "#14b8a6",
      "presence_gradient": ["#14b8a6", "#5eead4"],
      "humor": "#a855f7",
      "humor_gradient": ["#a855f7", "#d8b4fe"],
      "toxicity": "#ef4444",
      "toxicity_gradient": ["#22c55e", "#ef4444"],
      "popularity": "#e94560",
      "popularity_gradient": ["#e94560", "#fda4af"]
    },
    "badge_rarity_colors": {
      "common": {
        "bg_start": "#374151",
        "bg_end": "#4b5563",
        "border": "#6b7280",
        "text": "#d1d5db"
      },
      "rare": {
        "bg_start": "#1e3a8a",
        "bg_end": "#2563eb",
        "border": "#3b82f6",
        "text": "#93c5fd"
      },
      "epic": {
        "bg_start": "#6b21a8",
        "bg_end": "#9333ea",
        "border": "#a855f7",
        "text": "#d8b4fe"
      },
      "legendary": {
        "bg_start": "#92400e",
        "bg_end": "#f59e0b",
        "border": "#fbbf24",
        "text": "#fef3c7",
        "glow": "rgba(251, 191, 36, 0.4)"
      }
    },
    "tier_colors": {
      "legendary": ["#fbbf24", "#f59e0b"],
      "elite": ["#00d9ff", "#0ea5e9"],
      "outstanding": ["#a855f7", "#7c3aed"],
      "regular": ["#14b8a6", "#0d9488"],
      "beginner": ["#9ca3af", "#6b7280"],
      "rookie": ["#f472b6", "#ec4899"]
    },
    "effects": {
      "avatar_glow": "rgba(233, 69, 96, 0.5)",
      "username_glow": "rgba(0, 217, 255, 0.5)",
      "border_gradient": ["#e94560", "#533483", "#00d9ff"],
      "decoration_pink": "rgba(233, 69, 96, 0.1)",
      "decoration_cyan": "rgba(0, 217, 255, 0.1)"
    },
    "trend_colors": {
      "up": "#22c55e",
      "down": "#ef4444",
      "stable": "rgba(255, 255, 255, 0.5)"
    }
  },
  "typography": {
    "header_font": "Orbitron",
    "body_font": "Rajdhani",
    "header_weights": [400, 700, 900],
    "body_weights": [400, 500, 700]
  }
}
```

**3. Create `card.html`:**

Copy an existing theme's `card.html` as a starting point, or create from scratch. The template receives theme colors as CSS variables:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <style>
        @import url('{{ theme.typography.google_fonts_import }}');

        :root {
            /* Background */
            --bg-gradient-start: {{ theme.colors.bg_start }};
            --bg-gradient-mid: {{ theme.colors.bg_mid }};
            --bg-gradient-end: {{ theme.colors.bg_end }};

            /* Primary colors */
            --color-primary: {{ theme.colors.primary }};
            --color-secondary: {{ theme.colors.secondary }};

            /* Text */
            --text-primary: {{ theme.colors.text_primary }};
            --text-secondary: {{ theme.colors.text_secondary }};

            /* Typography */
            --font-header: '{{ theme.typography.header_font }}', sans-serif;
            --font-body: '{{ theme.typography.body_font }}', sans-serif;

            /* ... see existing themes for full variable list */
        }

        .card {
            background: linear-gradient(135deg, var(--bg-gradient-start), var(--bg-gradient-mid), var(--bg-gradient-end));
            font-family: var(--font-body);
            color: var(--text-primary);
        }
    </style>
</head>
<body>
    <!-- Use template variables for user data, stats, badges -->
    <div class="card">
        <h1>{{ user.first_name }}</h1>
        <!-- ... -->
    </div>
</body>
</html>
```

**4. Use the theme:**

```bash
# Via make target
make ml-run-render ML_ARGS="--week 2025-12-09 --theme mytheme"

# Via API
curl -X POST http://localhost:8051/api/v1/render \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"chat_id": -1003280306634, "week_start": "2025-12-09", "theme": "mytheme"}'
```

### Theme Configuration Reference

| Section | Description |
|---------|-------------|
| `colors.background_gradient` | 3-color gradient array [start, mid, end] |
| `colors.primary_accent` | Main accent color (buttons, highlights) |
| `colors.secondary_accent` | Secondary accent (decorations) |
| `colors.stat_colors` | Colors for each of the 6 stats (vibe, activity, presence, humor, toxicity, popularity) |
| `colors.badge_rarity_colors` | Badge styling by rarity (common, rare, epic, legendary) |
| `colors.tier_colors` | Gradient pairs for overall score tiers |
| `colors.effects` | Glow effects, border gradients, decorations |
| `colors.trend_colors` | Colors for trend indicators (up/down/stable) |
| `typography.header_font` | Google Font for headers |
| `typography.body_font` | Google Font for body text |
| `typography.*_weights` | Font weights to load from Google Fonts |

### Available Themes

| Theme | Description |
|-------|-------------|
| `gaming` | Dark neon theme with Orbitron/Rajdhani fonts, cyan/pink accents |
| `clean` | Light minimal theme with Inter font, blue/purple accents |

## Gaming Theme

The default gaming theme features:

- **Dimensions**: 400x600px (rendered at 800x1200 for retina)
- **Colors**: Dark gradient backgrounds (#1a1a2e → #0f3460), neon accents (cyan #00d9ff, pink #e94560)
- **Typography**: Orbitron for headers, Rajdhani for body
- **Elements**: Progress bars for stats, badge grid with rarity glow effects
- **User Avatar**: Profile photo with initials fallback
