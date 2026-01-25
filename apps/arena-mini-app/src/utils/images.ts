/**
 * Image utilities for arena game assets
 *
 * Features:
 * - Type-safe URL building for all image categories
 * - Cache-busting with version parameter
 * - Single image preloading with Promise
 * - Batch preloading by category
 *
 * Usage:
 * ```tsx
 * import { getImageUrl, preloadImage, preloadImagesByCategory } from '../utils/images'
 *
 * // Get a single image URL
 * const coinUrl = getImageUrl('icons', 'coin')
 *
 * // Preload a single image
 * await preloadImage(coinUrl)
 *
 * // Preload all images in a category
 * await preloadImagesByCategory('icons')
 * ```
 */

import { ImageCategory, ImageId, IMAGE_CONFIGS, BackgroundImageId } from '../types/images'

/**
 * Get the base URL for images from environment variable.
 * Falls back to empty string if not configured.
 */
function getBaseUrl(): string {
  return import.meta.env.VITE_IMAGE_BASE_URL || ''
}

/**
 * Get the image version for cache busting.
 * Falls back to '1' if not configured.
 */
function getImageVersion(): string {
  return import.meta.env.VITE_IMAGE_VERSION || '1'
}

/**
 * Build a complete URL for an image asset.
 *
 * @param category - Image category (buttons, panels, bars, icons, effects, bg)
 * @param filename - Image filename without extension
 * @param extension - File extension (default: 'png')
 * @returns Full URL with cache-busting version parameter
 *
 * @example
 * getImageUrl('icons', 'coin')
 * // => "http://localhost:9000/telegram-media/images/arena/icons/coin.png?v=1"
 *
 * getImageUrl('effects', 'explosion/explosion00')
 * // => "http://localhost:9000/telegram-media/images/arena/effects/explosion/explosion00.png?v=1"
 *
 * getImageUrl('bg', 'arena', 'webp')
 * // => "http://localhost:9000/telegram-media/images/arena/bg/arena.webp?v=1"
 */
export function getImageUrl(
  category: ImageCategory,
  filename: string,
  extension: 'png' | 'webp' = 'png'
): string {
  const baseUrl = getBaseUrl()
  const version = getImageVersion()
  return `${baseUrl}/${category}/${filename}.${extension}?v=${version}`
}

/**
 * Get URL for an image by its ID using the IMAGE_CONFIGS mapping.
 *
 * @param imageId - Image ID from the ImageId type
 * @returns Full URL with cache-busting version parameter
 *
 * @example
 * getImageUrlById('coin')
 * // => "http://localhost:9000/telegram-media/images/arena/icons/coin.png?v=1"
 *
 * getImageUrlById('arena')
 * // => "http://localhost:9000/telegram-media/images/arena/bg/arena.webp?v=1"
 */
export function getImageUrlById(imageId: ImageId): string {
  const config = IMAGE_CONFIGS[imageId]
  if (!config) {
    console.error(`[images] Unknown imageId: ${imageId}`)
    return ''
  }
  return getImageUrl(config.category, config.filename, config.extension ?? 'png')
}

/**
 * Get URL for a background image by its ID.
 *
 * @param backgroundId - Background image ID
 * @returns Full URL with cache-busting version parameter
 *
 * @example
 * getBackgroundUrl('arena')
 * // => "http://localhost:9000/telegram-media/images/arena/bg/arena.webp?v=1"
 */
export function getBackgroundUrl(backgroundId: BackgroundImageId): string {
  return getImageUrlById(backgroundId)
}

/**
 * Preload a single image and return a Promise that resolves when loaded.
 *
 * @param url - Full URL of the image to preload
 * @returns Promise that resolves when image is loaded, rejects on error
 *
 * @example
 * const url = getImageUrl('icons', 'coin')
 * await preloadImage(url)
 */
export function preloadImage(url: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const img = new Image()

    img.onload = () => {
      resolve()
    }

    img.onerror = () => {
      reject(new Error(`Failed to load image: ${url}`))
    }

    img.src = url
  })
}

/**
 * Preload multiple images in parallel.
 *
 * @param urls - Array of image URLs to preload
 * @returns Promise that resolves when all images are loaded
 *
 * @example
 * const urls = ['coin', 'star_yellow'].map(id => getImageUrlById(id as ImageId))
 * await preloadImages(urls)
 */
export async function preloadImages(urls: string[]): Promise<void> {
  await Promise.all(urls.map(preloadImage))
}

/**
 * Get all image IDs for a specific category.
 *
 * @param category - Image category to get IDs for
 * @returns Array of ImageId values belonging to the category
 */
export function getImageIdsByCategory(category: ImageCategory): ImageId[] {
  return (Object.entries(IMAGE_CONFIGS) as [ImageId, { category: ImageCategory }][])
    .filter(([, config]) => config.category === category)
    .map(([id]) => id)
}

/**
 * Preload all images in a specific category.
 *
 * @param category - Image category to preload (buttons, panels, bars, icons, effects)
 * @returns Promise that resolves when all images in the category are loaded
 *
 * @example
 * // Preload all icons before shop page renders
 * await preloadImagesByCategory('icons')
 *
 * // Preload all effects before battle page renders
 * await preloadImagesByCategory('effects')
 */
export async function preloadImagesByCategory(category: ImageCategory): Promise<void> {
  const imageIds = getImageIdsByCategory(category)
  const urls = imageIds.map(getImageUrlById)
  await preloadImages(urls)
}

/**
 * Preload multiple categories in parallel.
 *
 * @param categories - Array of categories to preload
 * @returns Promise that resolves when all categories are loaded
 *
 * @example
 * // Preload battle-related assets
 * await preloadCategories(['bars', 'effects'])
 */
export async function preloadCategories(categories: ImageCategory[]): Promise<void> {
  await Promise.all(categories.map(preloadImagesByCategory))
}
