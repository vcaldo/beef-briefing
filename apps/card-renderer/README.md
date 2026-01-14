# Card Renderer

Renders gamified user stats cards as PNG images using HTML/CSS templates and Playwright.

## Overview

The Card Renderer generates weekly stats cards from user data in the `ml_user_cards` table. It uses Jinja2 templates for HTML rendering and Playwright (headless Chromium) to convert HTML to high-quality PNG images. Generated images are stored in MinIO/S3 with presigned URLs for access.

## Features

- **HTML/CSS Templates**: Customizable card designs with Jinja2 templating
- **Theme System**: 9+ built-in themes with JSON configuration
- **Retina Quality**: 2x scale rendering for crisp images
- **Badge System**: Automatically derived badges based on user stats
- **Presigned URLs**: Secure, time-limited image access

## Quick Start

```bash
# Start with Docker (recommended)
make up-build

# Generate cards for a week
make ml-run-render ML_ARGS="--week 2025-01-06 --theme gaming"

# Or via API
API_KEY=$(cat infrastructure/secrets/apps/ml-processor/api_key)
curl -X POST http://localhost:8051/api/v1/render \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"chat_id": -1003280306634, "week_start": "2025-01-06"}'
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CARD_GENERATOR_PORT` | `8051` | Service port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `` | Database password |
| `DB_NAME` | `beef_briefing` | Database name |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO endpoint |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO access key |
| `MINIO_SECRET_KEY` | `minioadmin` | MinIO secret key |
| `MINIO_BUCKET` | `telegram-media` | Storage bucket |
| `TEMPLATES_DIR` | `/app/templates` | Templates directory |
| `DEFAULT_THEME` | `gaming` | Default theme name |
| `CARD_WIDTH` | `400` | Card width (pixels) |
| `CARD_HEIGHT` | `600` | Card height (pixels) |
| `CARD_SCALE` | `2` | Render scale (2 = retina) |
| `COMPACT_CARD_WIDTH` | `300` | Compact card width (pixels) |
| `COMPACT_CARD_HEIGHT` | `450` | Compact card height (pixels) |
| `APP_KEYS_DIR` | `/app/secrets/app_keys` | API keys directory |

## API Reference

All endpoints (except `/health`) require `Authorization: Bearer <key>` header.

### POST `/api/v1/render`

Trigger card image generation.

```json
{
  "chat_id": -1003280306634,
  "week_start": "2025-01-06",
  "user_ids": [123, 456],           // optional: specific users
  "theme": "gaming",                // optional: uses DEFAULT_CARD_THEME if not specified
  "card_type": "regular",           // optional: "regular" or "compact" (default: "regular")
  "force_regenerate": false         // optional: regenerate existing
}
```

**Card Types:**
- `regular`: Full-size cards (400x600 pixels, 75% of screen)
- `compact`: Smaller cards (300x450 pixels, designed for compact gallery views)
  - Compact cards are stored with `{theme}_compact` suffix (e.g., `gaming_compact`)
  - Include placeholder boxes for combat stats and HP bar that React apps can overlay values into

### GET `/api/v1/images`

List card images for a chat/week.

**Parameters:** `chat_id`, `week_start`, `user_id`, `theme`

### GET `/api/v1/image/{id}`

Get presigned URL for a specific image.

**Parameters:** `expires` (60-86400 seconds, default: 3600)

### GET `/health`

Health check (unauthenticated).

## Theme System

Themes consist of the following files in `templates/themes/{name}/`:
- `theme.json` - Colors and typography configuration
- `card.html` - Jinja2 HTML/CSS template for regular cards (400x600)
- `compact_card.html` - (optional) Jinja2 HTML/CSS template for compact cards (300x450)

### Compact Card Templates

Compact cards are smaller versions designed for gallery views and React apps that overlay values dynamically. Each compact template includes:
- Circular avatar with tier-styled glow
- User name and rank badge (#N)
- Tier bar with label and gradient
- **Placeholder boxes** for combat stats (ATK, DEF, HP) showing "--"
- **Placeholder container** for HP progress bar (0% fill)

The PNG images are **static structures only** - React apps use absolute positioning to overlay actual values on top of the placeholder positions.

### Available Themes

| Theme | Description |
|-------|-------------|
| `gaming` | Dark neon cyberpunk with cyan/pink accents |
| `clean` | Light minimal professional design |
| `sticker` | Telegram-native sticker style |
| `mythic` | Dark fantasy RPG with golden accents |
| `meme` | Chaotic rainbow with oversized emojis |
| `vaporwave` | Retro 80s aesthetic |
| `blueprint` | Technical monospace design |
| `noir_luxury` | Matte black with gold foil |
| `neon_arcade` | Bright arcade game style |

### Creating Custom Themes

1. Create theme directory:
```bash
mkdir -p apps/card-renderer/templates/themes/mytheme
```

2. Create `theme.json`:
```json
{
  "name": "mytheme",
  "colors": {
    "background_gradient": ["#1a1a2e", "#16213e", "#0f3460"],
    "primary_accent": "#00d9ff",
    "secondary_accent": "#e94560",
    "text_primary": "#ffffff",
    "text_secondary": "rgba(255, 255, 255, 0.6)"
  },
  "typography": {
    "header_font": "Orbitron",
    "body_font": "Rajdhani"
  }
}
```

3. Create `card.html` (copy from existing theme and customize).

4. Use the theme:
```bash
make ml-run-render ML_ARGS="--week 2025-01-06 --theme mytheme"
```

### Template Variables

```jinja2
{# User Info #}
{{ user.first_name }}
{{ user.username }}
{{ user.photo_url }}

{# Week Info #}
{{ week_start }}        {# "2025-01-06" #}
{{ week_end }}          {# "2025-01-12" #}
{{ period_display }}    {# "Jan 06 - Jan 12" #}

{# Stats (list of StatContext) #}
{% for stat in stats %}
  {{ stat.key }}          {# "aura", "activity", etc. #}
  {{ stat.label }}        {# "Aura", "Activity" #}
  {{ stat.value }}        {# raw value #}
  {{ stat.percentage }}   {# 0-100 for progress bars #}
{% endfor %}

{# Combat Stats (RPG-style) #}
{% if combat %}
  {{ combat.atk }}        {# 1-10 attack value #}
  {{ combat.def_ }}       {# 1-10 defense value #}
  {{ combat.hp }}         {# 3-30 health points #}
{% endif %}

{# Overall Score & Tier #}
{{ overall.score }}       {# 0-100 overall score #}
{{ overall.label }}       {# "Lendario", "Bichao", etc. #}
{{ overall.tier_class }}  {# "tier-1" through "tier-6" #}

{# Badges (max 4) #}
{% for badge in badges %}
  {{ badge.name }}   {# "Night Owl" #}
  {{ badge.icon }}   {# emoji #}
  {{ badge.rarity }} {# "common", "rare", "epic", "legendary" #}
{% endfor %}
```

### Badge System

Badges are automatically derived from stats:

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Night Owl | chronotype = "Coruja" | rare |
| Ray of Sunshine | mood >= 90 | legendary |
| Stand-Up King | comedy >= 70% | legendary |
| Chatterbox | messages >= 500 | legendary |
| Zen Master | toxicity < 1% | legendary |
| Beloved | reactions >= 100 | legendary |

## Storage

Images are stored with the path pattern:
```
cards/{chat_id}/{week_start}/{user_id}.png
```

Example: `cards/-1003280306634/2025-01-06/123456789.png`

## Architecture

```
apps/card-renderer/
├── src/
│   ├── api/routes.py           # FastAPI endpoints
│   ├── services/
│   │   ├── template_loader.py  # Jinja2 rendering
│   │   └── playwright_renderer.py  # HTML→PNG
│   ├── storage/client.py       # MinIO client
│   └── repository/             # Database access
├── templates/themes/           # Theme files
├── main.py                     # Entry point
└── Dockerfile
```

## Troubleshooting

### Cards not generating

1. Ensure `ml_user_cards` has data for the week:
```sql
SELECT * FROM ml_user_cards WHERE week_start = '2025-01-06';
```

2. Run card generation first:
```bash
make ml-run-cards ML_ARGS="--week 2025-01-06 --timezone America/Sao_Paulo"
```

### Theme not found

- Check theme directory exists: `templates/themes/{name}/`
- Verify both `theme.json` and `card.html` are present

### Image quality issues

- Check `CARD_SCALE` is set to 2 for retina
- Verify fonts are loading (check Google Fonts imports in theme)
