# Card Renderer

Renders gamified user stats cards as PNG images using HTML/CSS templates and Playwright.

## Overview

The Card Renderer generates weekly stats cards from user data in the `ml_user_cards` table. It uses Jinja2 templates for HTML rendering and Playwright (headless Chromium) to convert HTML to high-quality PNG images. Generated images are stored in MinIO/S3 with presigned URLs for access.

## Features

- **Multi-Size Variants**: Automatically generates 3 responsive sizes (large, medium, small)
- **HTML/CSS Templates**: Customizable card designs with Jinja2 templating
- **Theme System**: 9+ built-in themes with JSON configuration
- **Retina Quality**: 2x scale rendering for crisp images on large variant
- **Badge System**: Automatically derived badges based on user stats
- **Presigned URLs**: Secure, time-limited image access for all size variants

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
| `APP_KEYS_DIR` | `/app/secrets/app_keys` | API keys directory |

**Note**: Card dimensions are now configured via `CARD_SIZES` in `config/__init__.py` (see Size Variants section below).

## API Reference

All endpoints (except `/health`) require `Authorization: Bearer <key>` header.

### POST `/api/v1/render`

Trigger card image generation.

```json
{
  "chat_id": -1003280306634,
  "week_start": "2025-01-06",
  "user_ids": [123, 456],       // optional: specific users
  "theme": "gaming",            // optional
  "force_regenerate": false     // optional: regenerate existing
}
```

### GET `/api/v1/images`

List card images for a chat/week.

**Parameters:** `chat_id`, `week_start`, `user_id`, `theme`

### GET `/api/v1/image/{id}`

Get presigned URLs for all size variants of a specific image.

**Parameters:** `expires` (60-86400 seconds, default: 3600)

**Response**:
```json
{
  "image_id": 12345,
  "sizes": {
    "large": {
      "url": "https://s3.../cards/-100123/2025-01-06/gaming/456_large.png?X-Amz...",
      "width": 800,
      "height": 1200
    },
    "medium": {
      "url": "https://s3.../cards/-100123/2025-01-06/gaming/456_medium.png?X-Amz...",
      "width": 400,
      "height": 600
    },
    "small": {
      "url": "https://s3.../cards/-100123/2025-01-06/gaming/456_small.png?X-Amz...",
      "width": 200,
      "height": 300
    }
  },
  "expires_in": 3600,
  "theme": "gaming",
  "week_start": "2025-01-06",
  "generated_at": "2025-01-13T10:00:00Z"
}
```

**Legacy Fields** (deprecated, use `sizes` object): `url`, `width`, `height` (pointing to large variant)

### GET `/health`

Health check (unauthenticated).

## Theme System

Themes consist of two files in `templates/themes/{name}/`:
- `theme.json` - Colors and typography configuration
- `card.html` - Jinja2 HTML/CSS template

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
cards/{chat_id}/{week_start}/{theme}/{user_id}_{size}.png
```

Each card generates 3 size variants:
- **Large**: `{user_id}_large.png` (800×1200px, rendered at 2x scale)
- **Medium**: `{user_id}_medium.png` (400×600px, rendered at 1x scale)
- **Small**: `{user_id}_small.png` (200×300px, rendered at 1x scale)

Example:
```
cards/-1003280306634/2025-01-06/gaming/123456789_large.png
cards/-1003280306634/2025-01-06/gaming/123456789_medium.png
cards/-1003280306634/2025-01-06/gaming/123456789_small.png
```

All 3 variants are stored in the database with separate rows (one per size).

## Size Variants

The Card Renderer automatically generates responsive image variants optimized for different use cases.

### Size Configuration

Sizes are defined in `config/__init__.py`:

```python
CARD_SIZES = {
    'large': {'width': 400, 'height': 600, 'scale': 2},    # Output: 800x1200px
    'medium': {'width': 400, 'height': 600, 'scale': 1},   # Output: 400x600px
    'small': {'width': 200, 'height': 300, 'scale': 1},    # Output: 200x300px
}
```

- **viewport width/height**: Chromium viewport dimensions
- **scale**: Device pixel ratio (DPR), affects output dimensions
- **output size**: viewport × scale = actual image dimensions

### Use Cases

| Size | Dimensions | Use Case |
|------|-----------|----------|
| Large | 800×1200px | Detail view, full-screen display, printing |
| Medium | 400×600px | Card list, gallery, previews |
| Small | 200×300px | Thumbnails, social sharing, mini-cards |

### Rendering Process

All 3 sizes are rendered in parallel using async Playwright contexts:
1. One context per size with its specific viewport dimensions
2. Each renders the HTML template independently (no downscaling)
3. Results are uploaded to MinIO with size suffixes
4. All 3 rows inserted into database in a single transaction

**Performance**: ~2.5-4s per card (vs ~2-3s previously, ~30-50% increase acceptable)

### Frontend Usage

```typescript
// Recommended: Use size-specific URLs
const { large, medium, small } = response.sizes;

// Thumbnail
<img src={small.url} width={small.width} height={small.height} alt="thumbnail" />

// Detail view
<img src={large.url} width={large.width} height={large.height} alt="full card" />

// Responsive
<picture>
  <source media="(min-width: 768px)" srcSet={large.url} />
  <source media="(min-width: 400px)" srcSet={medium.url} />
  <img src={small.url} alt="card" />
</picture>

// Legacy (still works, uses large)
<img src={response.url} width={response.width} height={response.height} />
```

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

### Missing size variants

If only some size variants are generated, check:

1. Verify all 3 files exist in MinIO:
```bash
# Via minio client or dashboard
# Should see:
# - {user_id}_large.png
# - {user_id}_medium.png
# - {user_id}_small.png
```

2. Verify 3 rows per user in database:
```sql
SELECT user_id, size, width, height, storage_path
FROM ml_user_card_images
WHERE chat_id = -1003280306634 AND week_start = '2025-01-06'
ORDER BY user_id, size;
-- Should show 3 rows per user_id
```

3. Check render logs for size-specific errors:
```bash
make logs-card-renderer
```

### Theme not found

- Check theme directory exists: `templates/themes/{name}/`
- Verify both `theme.json` and `card.html` are present

### Image quality issues

- **Large size blurry**: Verify scale=2 in CARD_SIZES['large']
- **Medium/Small size blurry**: Check that downsampling isn't being applied (should render native dimensions)
- **Fonts not loading**: Verify Google Fonts imports in theme CSS
- **3x render time slow**: Expected 30-50% increase; check system resources
