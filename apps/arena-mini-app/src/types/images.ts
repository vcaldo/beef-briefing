/**
 * Image types for arena game assets
 *
 * Defines TypeScript types for image categories, filenames, and configuration.
 * Used by image utilities and hooks for type-safe image loading.
 */

// Image categories matching asset directory structure
export type ImageCategory = 'buttons' | 'panels' | 'bars' | 'icons' | 'effects'

// Button image IDs
export type ButtonImageId = 'primary' | 'secondary' | 'neutral' | 'danger'

// Panel image IDs
export type PanelImageId = 'hexagon_grey' | 'banner_modern' | 'banner_classic_curtain' | 'banner_hanging' | 'panel_brown' | 'panelInset_beige'

// HP Bar image IDs (left/mid/right segments for green/red variants)
export type BarImageId =
  | 'barHorizontal_green_left'
  | 'barHorizontal_green_mid'
  | 'barHorizontal_green_right'
  | 'barHorizontal_red_left'
  | 'barHorizontal_red_mid'
  | 'barHorizontal_red_right'

// Icon image IDs
export type IconImageId =
  | 'coin'
  | 'star_yellow'
  | 'checkmark'
  | 'cross'
  | 'arrow_up'
  | 'arrow_down'
  | 'arrow_left'
  | 'arrow_right'

// Effect image IDs (including explosion frames)
export type EffectImageId =
  | 'flame_01'
  | 'flame_02'
  | 'magic_01'
  | 'magic_02'
  | 'spark_01'
  | 'spark_02'
  | 'explosion/explosion00'
  | 'explosion/explosion01'
  | 'explosion/explosion02'
  | 'explosion/explosion03'
  | 'explosion/explosion04'
  | 'explosion/explosion05'
  | 'explosion/explosion06'
  | 'explosion/explosion07'
  | 'explosion/explosion08'

// Union of all image IDs
export type ImageId = ButtonImageId | PanelImageId | BarImageId | IconImageId | EffectImageId

// Image configuration interface
export interface ImageConfig {
  /** Image category (determines subdirectory) */
  category: ImageCategory
  /** Image filename (without extension, relative to category directory) */
  filename: string
  /** Optional width in pixels (for preloading hints) */
  width?: number
  /** Optional height in pixels (for preloading hints) */
  height?: number
}

// Map image IDs to their configurations
export const IMAGE_CONFIGS: Record<ImageId, ImageConfig> = {
  // Buttons (190x49 pixels)
  primary: { category: 'buttons', filename: 'primary', width: 190, height: 49 },
  secondary: { category: 'buttons', filename: 'secondary', width: 190, height: 49 },
  neutral: { category: 'buttons', filename: 'neutral', width: 190, height: 49 },
  danger: { category: 'buttons', filename: 'danger', width: 190, height: 49 },

  // Panels (various sizes)
  hexagon_grey: { category: 'panels', filename: 'hexagon_grey', width: 100, height: 114 },
  banner_modern: { category: 'panels', filename: 'banner_modern', width: 190, height: 45 },
  banner_classic_curtain: { category: 'panels', filename: 'banner_classic_curtain', width: 254, height: 179 },
  banner_hanging: { category: 'panels', filename: 'banner_hanging', width: 190, height: 171 },
  panel_brown: { category: 'panels', filename: 'panel_brown', width: 100, height: 100 },
  panelInset_beige: { category: 'panels', filename: 'panelInset_beige', width: 93, height: 94 },

  // HP Bars (green segments)
  barHorizontal_green_left: { category: 'bars', filename: 'barHorizontal_green_left', width: 8, height: 16 },
  barHorizontal_green_mid: { category: 'bars', filename: 'barHorizontal_green_mid', width: 1, height: 16 },
  barHorizontal_green_right: { category: 'bars', filename: 'barHorizontal_green_right', width: 8, height: 16 },

  // HP Bars (red segments)
  barHorizontal_red_left: { category: 'bars', filename: 'barHorizontal_red_left', width: 8, height: 16 },
  barHorizontal_red_mid: { category: 'bars', filename: 'barHorizontal_red_mid', width: 1, height: 16 },
  barHorizontal_red_right: { category: 'bars', filename: 'barHorizontal_red_right', width: 8, height: 16 },

  // Icons (64x64 pixels)
  coin: { category: 'icons', filename: 'coin', width: 64, height: 64 },
  star_yellow: { category: 'icons', filename: 'star_yellow', width: 64, height: 64 },
  checkmark: { category: 'icons', filename: 'checkmark', width: 64, height: 64 },
  cross: { category: 'icons', filename: 'cross', width: 64, height: 64 },
  arrow_up: { category: 'icons', filename: 'arrow_up', width: 64, height: 64 },
  arrow_down: { category: 'icons', filename: 'arrow_down', width: 64, height: 64 },
  arrow_left: { category: 'icons', filename: 'arrow_left', width: 64, height: 64 },
  arrow_right: { category: 'icons', filename: 'arrow_right', width: 64, height: 64 },

  // Effects (flame and magic - 512x512 pixels)
  flame_01: { category: 'effects', filename: 'flame_01', width: 512, height: 512 },
  flame_02: { category: 'effects', filename: 'flame_02', width: 512, height: 512 },
  magic_01: { category: 'effects', filename: 'magic_01', width: 512, height: 512 },
  magic_02: { category: 'effects', filename: 'magic_02', width: 512, height: 512 },
  spark_01: { category: 'effects', filename: 'spark_01', width: 512, height: 512 },
  spark_02: { category: 'effects', filename: 'spark_02', width: 512, height: 512 },

  // Effects (explosion frames - 512x512 pixels)
  'explosion/explosion00': { category: 'effects', filename: 'explosion/explosion00', width: 512, height: 512 },
  'explosion/explosion01': { category: 'effects', filename: 'explosion/explosion01', width: 512, height: 512 },
  'explosion/explosion02': { category: 'effects', filename: 'explosion/explosion02', width: 512, height: 512 },
  'explosion/explosion03': { category: 'effects', filename: 'explosion/explosion03', width: 512, height: 512 },
  'explosion/explosion04': { category: 'effects', filename: 'explosion/explosion04', width: 512, height: 512 },
  'explosion/explosion05': { category: 'effects', filename: 'explosion/explosion05', width: 512, height: 512 },
  'explosion/explosion06': { category: 'effects', filename: 'explosion/explosion06', width: 512, height: 512 },
  'explosion/explosion07': { category: 'effects', filename: 'explosion/explosion07', width: 512, height: 512 },
  'explosion/explosion08': { category: 'effects', filename: 'explosion/explosion08', width: 512, height: 512 },
}
