/**
 * RPGPanel - Reusable RPG-style panel component
 *
 * Uses 9-slice scaling with CSS border-image for wooden frame aesthetic.
 * Supports multiple panel styles:
 * - outer: Brown wooden frame for main containers
 * - inner: Beige inset for content areas
 * - inner-blue: Blue inset for stats emphasis
 * - inner-light: Light beige for subtle backgrounds
 * - inner-brown: Brown inset for darker accents
 */

import { ReactNode } from 'react'
import { useImages } from '../../hooks'
import type { PanelImageId } from '../../types/images'

export type RPGPanelVariant = 'outer' | 'inner' | 'inner-blue' | 'inner-light' | 'inner-brown'

export interface RPGPanelProps {
  variant: RPGPanelVariant
  children: ReactNode
  className?: string
}

// Map variant to panel image ID
const variantToPanelId: Record<RPGPanelVariant, PanelImageId> = {
  'outer': 'panel_brown',
  'inner': 'panelInset_beige',
  'inner-blue': 'panelInset_blue',
  'inner-light': 'panelInset_beigeLight',
  'inner-brown': 'panelInset_brown',
}

export function RPGPanel({ variant, children, className = '' }: RPGPanelProps) {
  const { getUrl } = useImages()

  // Select image based on variant
  const panelUrl = getUrl('panels', variantToPanelId[variant])

  return (
    <div
      className={`rpg-panel rpg-panel-${variant} ${className}`}
      style={{
        borderImageSource: `url(${panelUrl})`,
      }}
    >
      {children}
    </div>
  )
}
