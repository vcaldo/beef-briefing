import { useState } from 'react'
import { TeamCard, EmptySlot } from '../common/Card'
import type { EnhancedTeamCard } from '../../types'

interface TeamDisplayProps {
  /** Team cards (0-3 cards) */
  team: EnhancedTeamCard[]
  /** Current team order */
  teamOrder: number[]
  /** Upgrade ATK callback */
  onUpgradeAtk?: (teamSlot: number) => void
  /** Upgrade HP callback */
  onUpgradeHp?: (teamSlot: number) => void
  /** Reorder team callback */
  onReorder?: (newOrder: number[]) => void
  /** Whether an upgrade is in progress */
  upgradeLoading?: boolean
  /** Whether reorder is in progress */
  orderLoading?: boolean
  /** Read-only mode (after submission) */
  readOnly?: boolean
}

/**
 * Team display with 3 card slots.
 * Shows cards in team order with upgrade buttons.
 * Supports drag-to-reorder functionality.
 */
export function TeamDisplay({
  team,
  teamOrder,
  onUpgradeAtk,
  onUpgradeHp,
  onReorder,
  upgradeLoading,
  orderLoading,
  readOnly = false,
}: TeamDisplayProps) {
  const [selectedSlot, setSelectedSlot] = useState<number | null>(null)

  // Get cards sorted by team order
  const getOrderedCards = () => {
    // Map team order indices to actual cards
    // teamOrder is [0, 1, 2] representing the battle order of team slots
    const orderedSlots = teamOrder.length > 0 ? teamOrder : [0, 1, 2]
    return orderedSlots.map((slotIndex) => ({
      slotIndex,
      card: team[slotIndex] || null,
    }))
  }

  const orderedCards = getOrderedCards()

  // Handle card click for reordering
  const handleCardClick = (slotIndex: number) => {
    if (readOnly || !onReorder) return

    if (selectedSlot === null) {
      // First selection
      setSelectedSlot(slotIndex)
    } else if (selectedSlot === slotIndex) {
      // Deselect
      setSelectedSlot(null)
    } else {
      // Swap positions
      const newOrder = [...(teamOrder.length > 0 ? teamOrder : [0, 1, 2])]
      const firstIdx = newOrder.indexOf(selectedSlot)
      const secondIdx = newOrder.indexOf(slotIndex)

      if (firstIdx !== -1 && secondIdx !== -1) {
        // Swap the positions
        newOrder[firstIdx] = slotIndex
        newOrder[secondIdx] = selectedSlot
        onReorder(newOrder)
      }

      setSelectedSlot(null)
    }
  }

  return (
    <div className={`team-display ${orderLoading ? 'loading' : ''}`}>
      {[0, 1, 2].map((displayPosition) => {
        const slotData = orderedCards[displayPosition]
        const card = slotData?.card
        const slotIndex = slotData?.slotIndex ?? displayPosition
        const isSelected = selectedSlot === slotIndex

        if (!card) {
          return (
            <EmptySlot
              key={displayPosition}
              position={displayPosition + 1}
              onClick={
                !readOnly
                  ? () => handleCardClick(slotIndex)
                  : undefined
              }
            />
          )
        }

        return (
          <TeamCard
            key={card.card_id}
            card={card}
            position={displayPosition + 1}
            onUpgradeAtk={
              !readOnly && onUpgradeAtk
                ? () => onUpgradeAtk(slotIndex)
                : undefined
            }
            onUpgradeHp={
              !readOnly && onUpgradeHp
                ? () => onUpgradeHp(slotIndex)
                : undefined
            }
            onClick={
              !readOnly && onReorder && team.length > 1
                ? () => handleCardClick(slotIndex)
                : undefined
            }
            isSelected={isSelected}
            readOnly={readOnly || upgradeLoading}
          />
        )
      })}

      {/* Reorder hint */}
      {!readOnly && team.length > 1 && (
        <div className="team-reorder-hint">
          {selectedSlot !== null
            ? 'Tap another card to swap positions'
            : 'Tap to swap battle order'
          }
        </div>
      )}
    </div>
  )
}
