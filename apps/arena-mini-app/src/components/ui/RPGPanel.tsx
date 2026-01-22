/**
 * RPGPanel - Reusable RPG-style panel component
 *
 * Uses 9-slice scaling with CSS border-image for wooden frame aesthetic.
 * Supports outer (brown) and inner (beige inset) panel styles.
 */

import { ReactNode } from 'react'
import { useImages } from '../../hooks'

export type RPGPanelVariant = 'outer' | 'inner'

export interface RPGPanelProps {
  variant: RPGPanelVariant
  children: ReactNode
  className?: string
}

export function RPGPanel({ variant, children, className = '' }: RPGPanelProps) {
  const { getUrl } = useImages()

  // Select image based on variant
  const panelUrl = variant === 'outer' ? getUrl('panels', 'panel_brown') : getUrl('panels', 'panelInset_beige')

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
