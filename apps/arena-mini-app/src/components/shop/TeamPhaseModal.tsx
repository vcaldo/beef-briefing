/**
 * TeamPhaseModal - Full-screen modal for team management
 *
 * Features:
 * - Full-screen overlay during team phase
 * - Drag-and-drop card reordering
 * - Card upgrade buttons (ATK/HP)
 * - Team submission and waiting state
 * - Automatic navigation when battle starts
 */

import { useState, useEffect, useCallback, useRef } from 'react'
import { Reorder } from 'framer-motion'

import type {
  Match,
  EnhancedShopResponse,
  EnhancedTeamCard,
  GameConstants,
} from '../../types'
import { CountdownTimer } from '../common'
import { apiClient } from '../../api/client'

interface TeamPhaseModalProps {
  shopData: EnhancedShopResponse
  activeMatch: Match
  gameConstants: GameConstants | null
  onShopDataChange: (data: EnhancedShopResponse) => void
  onNavigateToBattle: () => void
  onMatchChange: (match: Match | null) => void
}

export function TeamPhaseModal({
  shopData,
  activeMatch,
  gameConstants,
  onShopDataChange,
  onNavigateToBattle: _onNavigateToBattle,
  onMatchChange: _onMatchChange,
}: TeamPhaseModalProps) {
  // Reserved variables - suppress unused warnings
  void _onNavigateToBattle
  void _onMatchChange

  // Derived state
  const coins = shopData?.coins ?? 0
  const isSubmitted = shopData?.is_ready ?? false
  const teamCards = shopData?.team ?? []
  const teamOrder = shopData?.team_order ?? []

  // Local state for smooth drag-and-drop (decoupled from server state)
  const [localTeamOrder, setLocalTeamOrder] = useState<EnhancedTeamCard[]>([])
  // Ref to track current order (avoids stale closure in handleDragEnd)
  const localTeamOrderRef = useRef<EnhancedTeamCard[]>([])
  // Ref to track if user is actively dragging (prevents server updates from interrupting)
  const isDraggingRef = useRef(false)
  // Loading state for API calls
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  // Sync local team order with server state (when not dragging)
  // Apply team_order to display cards in battle order
  useEffect(() => {
    if (!isDraggingRef.current && teamCards.length > 0) {
      if (teamOrder.length === teamCards.length) {
        // Reorder cards according to team_order (battle order)
        // Filter out undefined values in case of stale/invalid indices
        const orderedTeam = teamOrder
          .map((posIdx: number) => teamCards[posIdx])
          .filter((card: EnhancedTeamCard | undefined): card is EnhancedTeamCard => card !== undefined)
        setLocalTeamOrder(orderedTeam.length > 0 ? orderedTeam : teamCards)
      } else {
        setLocalTeamOrder(teamCards)
      }
    }
  }, [teamCards, teamOrder])

  // Keep ref in sync with state (for handleDragEnd to avoid stale closure)
  useEffect(() => {
    localTeamOrderRef.current = localTeamOrder
  }, [localTeamOrder])

  // Instant visual update during drag (no API call)
  const handleVisualReorder = useCallback((newOrder: EnhancedTeamCard[]) => {
    setLocalTeamOrder(newOrder)
  }, [])

  // API call only when drag ends
  const handleDragEnd = useCallback(async () => {
    isDraggingRef.current = false

    if (!activeMatch || isSubmitted) return

    // Read current order from ref (avoids stale closure)
    const currentOrder = localTeamOrderRef.current

    // Compare with server order by card IDs
    const newCardIds = currentOrder.map((c: EnhancedTeamCard) => c.card_id)
    const oldCardIds = teamCards.map((c: EnhancedTeamCard) => c.card_id)

    // Skip if order hasn't changed
    if (JSON.stringify(newCardIds) === JSON.stringify(oldCardIds)) return

    setActionLoading('reorder')
    try {
      // Send the original positions in new order
      // e.g., if user dragged card from pos 2 to pos 0: [card2, card0, card1] → [2, 0, 1]
      const newPositions = currentOrder.map((c: EnhancedTeamCard) => c.position)
      const data = await apiClient.setTeamOrder(activeMatch.id, newPositions)
      onShopDataChange(data)
    } catch (err) {
      console.error('Failed to reorder team:', err)
      // Revert to server state on error
      setLocalTeamOrder(teamCards)
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, isSubmitted, teamCards, onShopDataChange])

  // Coin display class based on affordability
  const getCoinClass = () => {
    const upgradeCost = gameConstants?.upgrade_cost ?? 1
    if (coins >= upgradeCost * 2) return 'coin-display can-afford'
    if (coins > 0) return 'coin-display limited'
    return 'coin-display empty'
  }

  return (
    <div className="team-phase-backdrop">
      <div className="team-phase-modal">
        {/* Header with title, coins, and timer */}
        <header className="team-phase-header">
          <div className="team-phase-header-left">
            <h1 className="team-phase-title">Organize Your Team</h1>
            {isSubmitted && (
              <span className="team-phase-submitted-badge">Team Submitted</span>
            )}
          </div>
          <div className="team-phase-header-right">
            <div className={getCoinClass()}>
              <span className="coin-icon">🪙</span>
              <span className="coin-amount">{coins}</span>
            </div>
            {shopData?.deadline && (
              <div className="team-phase-timer">
                <CountdownTimer
                  deadline={shopData.deadline}
                  onExpire={() => {
                    // Timer expired - will be handled by polling
                  }}
                  timerThresholds={gameConstants?.timer_thresholds}
                />
              </div>
            )}
          </div>
        </header>

        {/* Main content - card row layout */}
        <div className="team-phase-content">
          <section className="team-phase-cards-section">
            <h2 className="team-phase-section-title">Battle Formation</h2>

            {/* Draggable team cards */}
            {localTeamOrder.length > 0 ? (
              <Reorder.Group
                axis="x"
                values={localTeamOrder}
                onReorder={handleVisualReorder}
                className="team-phase-card-row"
                as="div"
              >
                {localTeamOrder.map((card: EnhancedTeamCard, idx: number) => (
                  <Reorder.Item
                    key={card.card_id}
                    value={card}
                    className={`team-phase-slot filled ${actionLoading === 'reorder' ? 'reordering' : ''}`}
                    drag={!isSubmitted}
                    onDragStart={() => { isDraggingRef.current = true }}
                    onDragEnd={handleDragEnd}
                    whileDrag={{ scale: 1.05, opacity: 0.8 }}
                    transition={{ type: 'spring', stiffness: 300, damping: 25 }}
                    as="div"
                  >
                    <span className="team-phase-slot-number">{idx + 1}</span>

                    <div className="team-phase-card-container">
                      {card.card_image_url ? (
                        <img
                          src={card.card_image_url}
                          alt={card.name}
                          className="team-phase-card-image"
                          loading="lazy"
                        />
                      ) : (
                        <div className="team-phase-card-fallback">
                          <div className="team-phase-card-name">{card.name}</div>
                          <div className="team-phase-card-stats">
                            <span className="stat atk">⚔️ {card.atk}</span>
                            <span className="stat hp">❤️ {card.hp}</span>
                          </div>
                        </div>
                      )}
                    </div>
                  </Reorder.Item>
                ))}
              </Reorder.Group>
            ) : (
              <div className="team-phase-card-row">
                {/* Empty slots when no team */}
                {Array.from({ length: 3 }).map((_, idx) => (
                  <div key={idx} className="team-phase-slot empty">
                    <span className="team-phase-slot-number">{idx + 1}</span>
                    <div className="team-phase-empty-slot">
                      <span className="team-phase-empty-text">Empty</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}

export default TeamPhaseModal
