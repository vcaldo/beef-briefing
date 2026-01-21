/**
 * useImages - Image loading hook for arena game assets
 *
 * Features:
 * - Type-safe URL building for all image categories
 * - Image preloading by category (buttons, panels, bars, icons, effects)
 * - Tracking of preload completion with isReady state
 * - Wraps utility functions from utils/images.ts for React usage
 *
 * Usage:
 * ```tsx
 * const { getUrl, preloadCategory, isReady } = useImages()
 *
 * // Preload images for current phase
 * useEffect(() => {
 *   preloadCategory('icons').then(() => console.log('Icons loaded'))
 * }, [preloadCategory])
 *
 * // Get URL for an image
 * const coinUrl = getUrl('icons', 'coin')
 * ```
 */

import { useState, useCallback, useRef, useEffect } from 'react'
import { ImageCategory, ImageId } from '../types/images'
import {
  getImageUrl,
  getImageUrlById,
  preloadImage,
  preloadImagesByCategory as preloadCategoryUtil,
  preloadCategories as preloadCategoriesUtil,
} from '../utils/images'

export interface UseImagesReturn {
  /** Get URL for an image by category and filename */
  getUrl: (category: ImageCategory, filename: string) => string
  /** Get URL for an image by its ID */
  getUrlById: (imageId: ImageId) => string
  /** Preload a single image by URL */
  preload: (url: string) => Promise<void>
  /** Preload all images in a category */
  preloadCategory: (category: ImageCategory) => Promise<void>
  /** Preload multiple categories in parallel */
  preloadCategories: (categories: ImageCategory[]) => Promise<void>
  /** Whether all requested preloads have completed */
  isReady: boolean
  /** Set of categories that have been successfully preloaded */
  loadedCategories: Set<ImageCategory>
}

/**
 * Hook for loading and preloading game images.
 *
 * Provides type-safe URL building and preloading with completion tracking.
 *
 * Usage:
 * ```tsx
 * const { getUrl, preloadCategory, isReady, loadedCategories } = useImages()
 *
 * // Preload on mount
 * useEffect(() => {
 *   preloadCategory('icons')
 * }, [preloadCategory])
 *
 * // Check if ready
 * if (!isReady) return <LoadingSpinner />
 *
 * // Use images
 * <img src={getUrl('icons', 'coin')} alt="Coin" />
 * ```
 */
export function useImages(): UseImagesReturn {
  // Track loaded categories
  const [loadedCategories, setLoadedCategories] = useState<Set<ImageCategory>>(
    () => new Set()
  )
  // Track pending preload count
  const pendingCountRef = useRef(0)
  // Track if any preload has been requested
  const hasRequestedPreloadRef = useRef(false)
  // isReady = no pending preloads AND at least one preload was requested
  const [isReady, setIsReady] = useState(false)

  // Update isReady when pending count changes
  useEffect(() => {
    if (hasRequestedPreloadRef.current && pendingCountRef.current === 0) {
      setIsReady(true)
    } else {
      setIsReady(false)
    }
  }, [loadedCategories]) // Re-check when loaded categories change

  // Get URL for an image by category and filename
  const getUrl = useCallback(
    (category: ImageCategory, filename: string): string => {
      return getImageUrl(category, filename)
    },
    []
  )

  // Get URL for an image by its ID
  const getUrlById = useCallback((imageId: ImageId): string => {
    return getImageUrlById(imageId)
  }, [])

  // Preload a single image by URL
  const preload = useCallback(async (url: string): Promise<void> => {
    hasRequestedPreloadRef.current = true
    pendingCountRef.current++
    setIsReady(false)

    try {
      await preloadImage(url)
    } finally {
      pendingCountRef.current--
      if (pendingCountRef.current === 0) {
        setIsReady(true)
      }
    }
  }, [])

  // Preload all images in a category
  const preloadCategory = useCallback(
    async (category: ImageCategory): Promise<void> => {
      // Skip if already loaded
      if (loadedCategories.has(category)) {
        return
      }

      hasRequestedPreloadRef.current = true
      pendingCountRef.current++
      setIsReady(false)

      try {
        await preloadCategoryUtil(category)
        setLoadedCategories((prev) => {
          const next = new Set(prev)
          next.add(category)
          return next
        })
      } finally {
        pendingCountRef.current--
        if (pendingCountRef.current === 0) {
          setIsReady(true)
        }
      }
    },
    [loadedCategories]
  )

  // Preload multiple categories in parallel
  const preloadCategories = useCallback(
    async (categories: ImageCategory[]): Promise<void> => {
      // Filter out already loaded categories
      const toLoad = categories.filter((c) => !loadedCategories.has(c))
      if (toLoad.length === 0) {
        return
      }

      hasRequestedPreloadRef.current = true
      pendingCountRef.current++
      setIsReady(false)

      try {
        await preloadCategoriesUtil(toLoad)
        setLoadedCategories((prev) => {
          const next = new Set(prev)
          toLoad.forEach((c) => next.add(c))
          return next
        })
      } finally {
        pendingCountRef.current--
        if (pendingCountRef.current === 0) {
          setIsReady(true)
        }
      }
    },
    [loadedCategories]
  )

  return {
    getUrl,
    getUrlById,
    preload,
    preloadCategory,
    preloadCategories,
    isReady,
    loadedCategories,
  }
}

export default useImages
