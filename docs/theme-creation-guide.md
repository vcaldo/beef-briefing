# Card Theme Creation Guide

This document describes the theme system for the card image generator and provides instructions for creating new themes. Use this as a reference when designing new card themes.

## Overview

The card image generator renders gamified user stats as PNG images (400x600px, rendered at 2x for retina). Each card displays:

- User profile (avatar, name, username, rank)
- Overall "Beef Meter" score with tier label
- Six stat categories with progress bars and trends
- Achievement badges (up to 4, with rarity tiers)
- Weekly activity summary

Themes control all visual aspects: colors, typography, gradients, and effects.

---

## Existing Themes

### Gaming Theme (Default)

**Aesthetic**: Dark cyberpunk/neon gaming aesthetic

- **Background**: Deep navy gradient (`#1a1a2e` → `#16213e` → `#0f3460`)
- **Primary Accent**: Cyan (`#00d9ff`)
- **Secondary Accent**: Hot pink (`#e94560`)
- **Typography**: Orbitron (headers) + Rajdhani (body) - geometric, futuristic
- **Effects**: Neon glows on avatar and username, gradient border with pink/purple/cyan
- **Character**: High contrast, vibrant neon highlights against dark backgrounds

```json
{
  "name": "gaming",
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

### Clean Theme

**Aesthetic**: Light, minimal, professional

- **Background**: White/light gray gradient (`#ffffff` → `#f8fafc` → `#f1f5f9`)
- **Primary Accent**: Blue (`#3b82f6`)
- **Secondary Accent**: Purple (`#8b5cf6`)
- **Typography**: Inter (both headers and body) - clean, readable
- **Effects**: Subtle glows, softer badge colors
- **Character**: Professional, accessible, less intense than gaming theme

```json
{
  "name": "clean",
  "colors": {
    "background_gradient": ["#ffffff", "#f8fafc", "#f1f5f9"],
    "primary_accent": "#3b82f6",
    "secondary_accent": "#8b5cf6",
    "text_primary": "#1e293b",
    "text_secondary": "rgba(30, 41, 59, 0.6)"
  },
  "typography": {
    "header_font": "Inter",
    "body_font": "Inter"
  }
}
```

---

## Theme Architecture

Each theme consists of two files in `templates/themes/{theme_name}/`:

```
templates/themes/
├── gaming/
│   ├── theme.json    # Color and typography configuration
│   └── card.html     # Jinja2 HTML/CSS template
└── mytheme/
    ├── theme.json
    └── card.html
```

### theme.json

Defines all colors and typography. The card.html template reads these values and applies them as CSS variables.

### card.html

Jinja2 template that generates HTML with embedded CSS. Uses theme values via template variables like `{{ theme.colors.primary }}`. The same HTML structure is typically reused across themes - only colors change.

---

## Complete theme.json Schema

```json
{
  "name": "theme_name",

  "colors": {
    "background_gradient": ["#start", "#mid", "#end"],
    "primary_accent": "#hex",
    "secondary_accent": "#hex",
    "text_primary": "#hex",
    "text_secondary": "rgba(r, g, b, a)",
    "border_color": "rgba(r, g, b, a)",

    "stat_colors": {
      "vibe": "#hex",
      "vibe_gradient": ["#start", "#end"],
      "activity": "#hex",
      "activity_gradient": ["#start", "#end"],
      "presence": "#hex",
      "presence_gradient": ["#start", "#end"],
      "humor": "#hex",
      "humor_gradient": ["#start", "#end"],
      "toxicity": "#hex",
      "toxicity_gradient": ["#start", "#end"],
      "popularity": "#hex",
      "popularity_gradient": ["#start", "#end"]
    },

    "badge_rarity_colors": {
      "common": {
        "bg_start": "#hex",
        "bg_end": "#hex",
        "border": "#hex",
        "text": "#hex"
      },
      "rare": {
        "bg_start": "#hex",
        "bg_end": "#hex",
        "border": "#hex",
        "text": "#hex"
      },
      "epic": {
        "bg_start": "#hex",
        "bg_end": "#hex",
        "border": "#hex",
        "text": "#hex"
      },
      "legendary": {
        "bg_start": "#hex",
        "bg_end": "#hex",
        "border": "#hex",
        "text": "#hex",
        "glow": "rgba(r, g, b, a)"
      }
    },

    "tier_colors": {
      "legendary": ["#start", "#end"],
      "elite": ["#start", "#end"],
      "outstanding": ["#start", "#end"],
      "regular": ["#start", "#end"],
      "beginner": ["#start", "#end"],
      "rookie": ["#start", "#end"]
    },

    "effects": {
      "avatar_glow": "rgba(r, g, b, a)",
      "username_glow": "rgba(r, g, b, a)",
      "border_gradient": ["#color1", "#color2", "#color3"],
      "decoration_pink": "rgba(r, g, b, a)",
      "decoration_cyan": "rgba(r, g, b, a)"
    },

    "trend_colors": {
      "up": "#hex",
      "down": "#hex",
      "stable": "rgba(r, g, b, a)"
    }
  },

  "typography": {
    "header_font": "Google Font Name",
    "body_font": "Google Font Name",
    "header_weights": [400, 700, 900],
    "body_weights": [400, 500, 700]
  }
}
```

---

## Color System Reference

### Background Gradient
3-color linear gradient (135deg) for card background.
- `background_gradient[0]` - Top-left start color
- `background_gradient[1]` - Middle color (50% position)
- `background_gradient[2]` - Bottom-right end color

### Primary & Secondary Accents
- `primary_accent` - Main highlight color (rank numbers, message count, stat highlights)
- `secondary_accent` - Supporting accent (avatar gradient, decorations)

### Text Colors
- `text_primary` - Main text color (usernames, values)
- `text_secondary` - Subdued text (labels, handles, secondary info)
- `border_color` - Card borders and dividers

### Stat Colors (6 categories)
Each stat has a solid color and a gradient pair for progress bars:

| Stat | Purpose | Icon |
|------|---------|------|
| `vibe` | Mood/sentiment score | Emoji varies |
| `activity` | Message frequency | Lightning bolt |
| `presence` | Days active | Calendar |
| `humor` | Comedy percentage | Laugh emoji |
| `toxicity` | Toxicity percentage | Warning |
| `popularity` | Reactions received | Heart |

### Badge Rarity Colors (4 tiers)
Badges have gradient backgrounds with matching borders:

| Rarity | Character |
|--------|-----------|
| `common` | Gray/muted, basic achievements |
| `rare` | Blue tints, notable achievements |
| `epic` | Purple tints, impressive achievements |
| `legendary` | Gold/amber with glow effect, exceptional achievements |

### Tier Colors (6 levels)
Used for the "Beef Meter" overall score bar:

| Tier | Score Range | Character |
|------|-------------|-----------|
| `legendary` | 90-100 | Gold, prestigious |
| `elite` | 75-89 | Cyan/blue, excellent |
| `outstanding` | 60-74 | Purple, impressive |
| `regular` | 40-59 | Teal/green, solid |
| `beginner` | 20-39 | Gray, developing |
| `rookie` | 0-19 | Pink, starting out |

### Effect Colors
- `avatar_glow` - Box shadow around avatar circle
- `username_glow` - Text shadow on username
- `border_gradient` - 3-color gradient for card border
- `decoration_pink/cyan` - Radial gradient decorations in card background

### Trend Colors
- `up` - Positive change indicator (typically green)
- `down` - Negative change indicator (typically red)
- `stable` - No significant change (muted/gray)

---

## Typography System

Fonts are loaded from Google Fonts automatically. Specify:

- `header_font` - Used for: username, rank number, stat values, beef meter value
- `body_font` - Used for: labels, handles, badges, footer text
- `header_weights` - Font weights to load for headers (e.g., [400, 700, 900])
- `body_weights` - Font weights to load for body (e.g., [400, 500, 700])

The system generates a Google Fonts import URL automatically.

### Recommended Font Pairings

| Style | Header | Body |
|-------|--------|------|
| Gaming/Tech | Orbitron, Exo 2, Audiowide | Rajdhani, Exo 2, Share Tech |
| Clean/Modern | Inter, Poppins, Outfit | Inter, Open Sans, Nunito |
| Retro | Press Start 2P, VT323 | Share Tech Mono, IBM Plex Mono |
| Elegant | Playfair Display, Cormorant | Lato, Source Sans Pro |
| Bold | Bebas Neue, Oswald | Barlow, Roboto |

---

## Card Layout Structure

```
┌─────────────────────────────────────┐
│ ┌──────┐                    ┌─────┐ │
│ │Avatar│ Name               │Rank │ │
│ │ 64px │ @handle            │ #1  │ │
│ └──────┘                    └─────┘ │
├─────────────────────────────────────┤
│ [Beef Emoji] Beef Meter  72 • Elite │
│ ████████████████░░░░░░░░░░░░░░░░░░░ │
├─────────────────────────────────────┤
│ ┌───────────────┐ ┌───────────────┐ │
│ │ Stat 1        │ │ Stat 2        │ │
│ │ Value  +5%    │ │ Value  -2%    │ │
│ │ ████████░░░░░ │ │ ██████░░░░░░░ │ │
│ └───────────────┘ └───────────────┘ │
│ ┌───────────────┐ ┌───────────────┐ │
│ │ Stat 3        │ │ Stat 4        │ │
│ └───────────────┘ └───────────────┘ │
│ ┌───────────────┐ ┌───────────────┐ │
│ │ Stat 5        │ │ Stat 6        │ │
│ └───────────────┘ └───────────────┘ │
├─────────────────────────────────────┤
│ [Badge] [Badge] [Badge] [Badge]     │
├─────────────────────────────────────┤
│ 247 messages    Week 50 · Dec 09-15 │
└─────────────────────────────────────┘
```

---

## Template Variables Available

### User Data
```jinja2
{{ user.first_name }}      {# "John" #}
{{ user.last_name }}       {# "Doe" #}
{{ user.username }}        {# "johndoe" #}
{{ user.initials }}        {# "JD" #}
{{ user.photo_url }}       {# URL or empty #}
```

### Week Info
```jinja2
{{ week_start }}           {# "2025-12-09" #}
{{ week_end }}             {# "2025-12-15" #}
{{ week_number }}          {# 50 #}
{{ period_display }}       {# "Dec 09 - Dec 15" #}
{{ rank }}                 {# 1-N or null #}
```

### Overall Score
```jinja2
{{ overall.score }}        {# 0-100 float #}
{{ overall.label }}        {# "Elite", "Legendary", etc. #}
```

### Stats Array
```jinja2
{% for stat in stats %}
  {{ stat.key }}           {# "vibe", "activity", etc. #}
  {{ stat.label }}         {# "Vibe", "Activity" #}
  {{ stat.icon }}          {# emoji #}
  {{ stat.value }}         {# raw numeric value #}
  {{ stat.percentage }}    {# 0-100 for progress bars #}
  {{ stat.display_value }} {# formatted: "72" or "45%" #}
  {{ stat.category_rank_medal }} {# medal emoji if top 3 #}
  {% if stat.trend %}
    {{ stat.trend.direction }}  {# "up", "down", "stable" #}
    {{ stat.trend.icon }}       {# arrow emoji #}
    {{ stat.trend.pct_change }} {# "+5%" or "-3%" #}
  {% endif %}
{% endfor %}
```

### Badges Array
```jinja2
{% for badge in badges %}
  {{ badge.key }}      {# "night_owl", "comedian" #}
  {{ badge.name }}     {# "Night Owl" #}
  {{ badge.icon }}     {# emoji #}
  {{ badge.rarity }}   {# "common", "rare", "epic", "legendary" #}
{% endfor %}
```

### Activity Summary
```jinja2
{{ activity.messages }}     {# total message count #}
{{ activity.active_days }}  {# days with messages #}
{{ activity.avg_length }}   {# average message length #}
```

### Theme Data
```jinja2
{{ theme.colors.bg_start }}
{{ theme.colors.bg_mid }}
{{ theme.colors.bg_end }}
{{ theme.colors.primary }}
{{ theme.colors.secondary }}
{{ theme.colors.text_primary }}
{{ theme.colors.text_secondary }}
{{ theme.colors.border }}
{{ theme.colors.stat.vibe }}
{{ theme.colors.stat.vibe_gradient[0] }}
{{ theme.colors.badge.legendary.glow }}
{{ theme.colors.tier.elite[0] }}
{{ theme.colors.effects.avatar_glow }}
{{ theme.colors.trend_colors.up }}
{{ theme.typography.header_font }}
{{ theme.typography.body_font }}
{{ theme.typography.google_fonts_import }}
```

---

## Creating a New Theme

### Step 1: Create Theme Directory
```bash
mkdir -p apps/card-image-generator/templates/themes/mytheme
```

### Step 2: Create theme.json
Start from an existing theme and modify colors/typography to match your aesthetic vision.

### Step 3: Copy card.html
Copy from gaming or clean theme as a starting point:
```bash
cp apps/card-image-generator/templates/themes/gaming/card.html \
   apps/card-image-generator/templates/themes/mytheme/card.html
```

The card.html template uses CSS variables that are populated from theme.json. You typically don't need to modify the HTML structure - just the theme.json colors will change the appearance.

### Step 4: Test the Theme
```bash
make ml-run-render ML_ARGS="--week 2025-12-09 --theme mytheme"
```

---

## Design Guidelines

### Color Contrast
- Ensure `text_primary` has sufficient contrast against `background_gradient`
- Light themes: dark text (`#1e293b` or similar)
- Dark themes: white or light text (`#ffffff`)

### Stat Color Distinctiveness
All six stat colors should be visually distinct from each other when viewed side by side.

### Badge Hierarchy
Rarity colors should form a clear hierarchy:
- Common: Neutral/gray (least attention)
- Rare: Cool color (blue family)
- Epic: Purple/violet (impressive)
- Legendary: Warm color + glow (most prestigious)

### Typography Harmony
Header and body fonts should complement each other. Consider:
- Similar x-height
- Compatible style (both geometric, both humanist, etc.)
- Adequate weight range for hierarchy

### Effect Restraint
Glows and decorations should enhance, not overwhelm. Use lower opacity values (0.1-0.5) for decorative effects.

---

## Theme Ideas for Inspiration

- **Synthwave**: Purple/pink gradients, neon effects, retro-futuristic fonts
- **Nature**: Earth tones, green accents, organic feel
- **Minimalist**: Near-white background, single accent color, lots of whitespace
- **Dark Mode Pro**: True black background, muted colors, accessibility focused
- **Pastel**: Soft colors, light backgrounds, friendly aesthetic
- **Corporate**: Blue-gray palette, professional fonts, understated
- **Retro Gaming**: Pixel-inspired, limited color palette, 8-bit fonts
- **Ocean**: Blue gradients, teal accents, flowing aesthetic
- **Sunset**: Orange/pink gradients, warm accents
- **Monochrome**: Single hue with varying shades
