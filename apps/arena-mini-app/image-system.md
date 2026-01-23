# Arena Mini-App Image System

## Overview

The arena mini-app uses a centralized image system built on React Context. Images are loaded from an external URL (MinIO/S3) with support for preloading by category, URL caching with version busting, and type-safe access via hooks.

## Image Files

All images are `.png` format stored at `{VITE_IMAGE_BASE_URL}/{category}/{imageId}.png`.

**Configuration Files:**
- `src/types/images.ts` - Image type definitions and IMAGE_CONFIGS registry
- `src/utils/images.ts` - URL building and preloading utilities
- `src/hooks/useImages.ts` - React hook for image management

## Image Registry

### Pressing Buttons (Local Assets)

Loaded from `assets/images/pressing_buttons/` via direct imports in `src/assets/pressingButtons.ts`.

| Image ID | Dimensions | Description |
|----------|------------|-------------|
| `buttonLong_beige` | 190x49 | Long beige button (default state) |
| `buttonLong_beige_pressed` | 190x45 | Long beige button (pressed state) |
| `buttonLong_blue` | 190x49 | Long blue button (default state) |
| `buttonLong_blue_pressed` | 190x45 | Long blue button (pressed state) |
| `buttonLong_brown` | 190x49 | Long brown button (default state) |
| `buttonLong_brown_pressed` | 190x45 | Long brown button (pressed state) |
| `buttonLong_grey` | 190x49 | Long grey button (default state) |
| `buttonLong_grey_pressed` | 190x45 | Long grey button (pressed state) |
| `buttonSquare_beige` | 45x49 | Square beige button (default state) |
| `buttonSquare_beige_pressed` | 45x45 | Square beige button (pressed state) |
| `buttonSquare_blue` | 45x49 | Square blue button (default state) |
| `buttonSquare_blue_pressed` | 45x45 | Square blue button (pressed state) |
| `buttonSquare_brown` | 45x49 | Square brown button (default state) |
| `buttonSquare_brown_pressed` | 45x45 | Square brown button (pressed state) |
| `buttonSquare_grey` | 45x49 | Square grey button (default state) |
| `buttonSquare_grey_pressed` | 45x45 | Square grey button (pressed state) |

### HP Bars (Remote)

| Image ID | Category | Dimensions | Description |
|----------|----------|------------|-------------|
| `barHorizontal_green_left` | bars | 8x16 | Green HP bar left cap |
| `barHorizontal_green_mid` | bars | 1x16 | Green HP bar middle (repeating) |
| `barHorizontal_green_right` | bars | 8x16 | Green HP bar right cap |
| `barHorizontal_red_left` | bars | 8x16 | Red HP bar left cap |
| `barHorizontal_red_mid` | bars | 1x16 | Red HP bar middle (repeating) |
| `barHorizontal_red_right` | bars | 8x16 | Red HP bar right cap |

### Icons (Remote)

| Image ID | Category | Dimensions | Description |
|----------|----------|------------|-------------|
| `coin` | icons | 64x64 | Currency display icon |
| `star_yellow` | icons | 64x64 | Leaderboard rank badge |
| `checkmark` | icons | 64x64 | Success/validation indicator |
| `cross` | icons | 64x64 | Error/cancel indicator |
| `arrow_up` | icons | 64x64 | Upgrade button icon |
| `arrow_down` | icons | 64x64 | Downgrade indicator |
| `arrow_left` | icons | 64x64 | Left navigation |
| `arrow_right` | icons | 64x64 | Right navigation |

### Panels (Remote)

| Image ID | Category | Dimensions | Description |
|----------|----------|------------|-------------|
| `panel_brown` | panels | 100x100 | RPG panel outer frame (9-slice) |
| `panelInset_beige` | panels | 93x94 | RPG panel inner inset (9-slice) |
| `banner_classic_curtain` | panels | 254x179 | Modal header background |
| `banner_hanging` | panels | 190x171 | Leaderboard item background |
| `banner_modern` | panels | 190x45 | Modern banner style |
| `hexagon_grey` | panels | 100x114 | Hexagon decorative element |

### Effects (Remote)

| Image ID | Category | Dimensions | Description |
|----------|----------|------------|-------------|
| `flame_01` | effects | 512x512 | Attack animation frame 1 |
| `flame_02` | effects | 512x512 | Attack animation frame 2 |
| `magic_01` | effects | 512x512 | Damage animation frame 1 |
| `magic_02` | effects | 512x512 | Damage animation frame 2 |
| `spark_01` | effects | 512x512 | Spark animation frame 1 |
| `spark_02` | effects | 512x512 | Spark animation frame 2 |
| `explosion/explosion00` | effects | 256x256 | Death animation frame 0 |
| `explosion/explosion01` | effects | 256x256 | Death animation frame 1 |
| `explosion/explosion02` | effects | 256x256 | Death animation frame 2 |
| `explosion/explosion03` | effects | 256x256 | Death animation frame 3 |
| `explosion/explosion04` | effects | 256x256 | Death animation frame 4 |
| `explosion/explosion05` | effects | 256x256 | Death animation frame 5 |
| `explosion/explosion06` | effects | 256x256 | Death animation frame 6 |
| `explosion/explosion07` | effects | 256x256 | Death animation frame 7 |
| `explosion/explosion08` | effects | 256x256 | Death animation frame 8 |

## Where Images Are Used

### GameButton.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `buttonLong_brown` | Primary variant, long shape | 87-127 |
| `buttonLong_brown_pressed` | Primary variant pressed state | 87-127 |
| `buttonLong_blue` | Secondary variant, long shape | 87-127 |
| `buttonLong_blue_pressed` | Secondary variant pressed state | 87-127 |
| `buttonLong_beige` | Neutral variant, long shape | 87-127 |
| `buttonLong_beige_pressed` | Neutral variant pressed state | 87-127 |
| `buttonLong_grey` | Danger variant, long shape | 87-127 |
| `buttonLong_grey_pressed` | Danger variant pressed state | 87-127 |
| `buttonSquare_*` | Same variants, square shape | 87-127 |

**Variant to Color Mapping:**
- Primary → brown
- Secondary → blue
- Neutral → beige
- Danger → grey

### HPBar.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `barHorizontal_green_left` | HP > 33%, left cap | 135-145 |
| `barHorizontal_green_mid` | HP > 33%, fill (repeat-x) | 153 |
| `barHorizontal_green_right` | HP > 33%, right cap | 163-174 |
| `barHorizontal_red_left` | HP <= 33%, left cap | 135-145 |
| `barHorizontal_red_mid` | HP <= 33%, fill (repeat-x) | 153 |
| `barHorizontal_red_right` | HP <= 33%, right cap | 163-174 |

### CoinDisplay.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `coin` | Always displayed with coin amount | 81-88 |

**Appears in:**
- ShopPage header (line 296)
- Buy card button text (line 424)
- TeamPhaseModal header (line 248)

### TeamPhaseModal.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `banner_classic_curtain` | Modal background | 232-237 |
| `arrow_up` | ATK upgrade button icon | 321 |
| `arrow_up` | HP upgrade button icon | 338 |

### StatsPage.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `banner_hanging` | Leaderboard item background | 370 |
| `star_yellow` | Top 3 rank badge | 377-383 |

### RPGPanel.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `panel_brown` | Outer variant border-image | 29 |
| `panelInset_beige` | Inner variant border-image | 29 |

### BattleEffect.tsx

| Image | Trigger | Line |
|-------|---------|------|
| `flame_01`, `flame_02` | Attack effect (3 loops, 100ms/frame) | 199 |
| `magic_01`, `magic_02` | Damage effect (2 loops, 80ms/frame) | 199 |
| `spark_01`, `spark_02` | Spark effect (4 loops, 60ms/frame) | 199 |
| `explosion00-08` | Death effect (no loop, 70ms/frame) | 199 |

**Effect Configurations (lines 67-101):**
| Effect Type | Frames | Frame Duration | Loops |
|-------------|--------|----------------|-------|
| `attack` | flame_01, flame_02 | 100ms | 3 |
| `damage` | magic_01, magic_02 | 80ms | 2 |
| `death` | explosion00-08 | 70ms | 1 |
| `spark` | spark_01, spark_02 | 60ms | 4 |

## Preloading Strategy

Images are preloaded by category using the `useImages` hook:

| Category | When Preloaded | Images |
|----------|----------------|--------|
| `buttons` | Local imports (bundled) | All pressing button variants |
| `bars` | BattlePage mount | HP bar segments (green/red) |
| `icons` | App mount | coin, star_yellow, arrows, checkmark, cross |
| `panels` | Component mount | panel_brown, panelInset_beige, banners |
| `effects` | BattlePage mount | flame, magic, spark, explosion frames |

**Preloading Functions:**
- `preloadImage(url)` - Single image preload
- `preloadImages(urls)` - Parallel batch preload
- `preloadImagesByCategory(category)` - All images in category
- `preloadCategories(categories)` - Multiple categories in parallel

## CSS Animations

### HP Bar Critical State
When HP drops below 33%, the bar pulses red:
```css
@keyframes hp-bar-critical-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}
```

### Game Button Press
Buttons swap background image on press (mousedown/touchstart) and restore on release.

### Battle Effects
Frame-based animation using requestAnimationFrame with configurable frame duration and loop count.

## Unused Images

The following images are defined but not currently used in components:
- `banner_modern` - Available panel but not referenced
- `hexagon_grey` - Available panel but not referenced
- `arrow_down` - Defined icon but not used
- `arrow_left` - Defined icon but not used
- `arrow_right` - Defined icon but not used
- `checkmark` - Defined icon but not used
- `cross` - Defined icon but not used
