/**
 * Rune Image Assets
 *
 * Local imports for rune placeholder images used in battle arena.
 * These are bundled with the app via Vite's asset handling.
 */

// Import all 10 rune variants
import rune001 from '../../assets/images/runes/runeBlack_rectangleOutline_001.png'
import rune002 from '../../assets/images/runes/runeBlack_rectangleOutline_002.png'
import rune003 from '../../assets/images/runes/runeBlack_rectangleOutline_003.png'
import rune004 from '../../assets/images/runes/runeBlack_rectangleOutline_004.png'
import rune005 from '../../assets/images/runes/runeBlack_rectangleOutline_005.png'
import rune006 from '../../assets/images/runes/runeBlack_rectangleOutline_006.png'
import rune007 from '../../assets/images/runes/runeBlack_rectangleOutline_007.png'
import rune008 from '../../assets/images/runes/runeBlack_rectangleOutline_008.png'
import rune009 from '../../assets/images/runes/runeBlack_rectangleOutline_009.png'
import rune010 from '../../assets/images/runes/runeBlack_rectangleOutline_010.png'

/** Array of all rune image URLs for random selection */
export const RUNE_IMAGES: string[] = [
  rune001,
  rune002,
  rune003,
  rune004,
  rune005,
  rune006,
  rune007,
  rune008,
  rune009,
  rune010,
]

/** Number of rune variants available */
export const RUNE_VARIANT_COUNT = RUNE_IMAGES.length

/**
 * Get a random rune image URL.
 * Uses Math.random() to select from available variants.
 */
export function getRandomRune(): string {
  const index = Math.floor(Math.random() * RUNE_VARIANT_COUNT)
  return RUNE_IMAGES[index]
}
