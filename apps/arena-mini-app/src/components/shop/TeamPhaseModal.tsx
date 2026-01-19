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

import { useSoundContext } from '../../contexts'
import type {
  Match,
  EnhancedShopResponse,
  EnhancedTeamCard,
  GameConstants,
  UpgradeType,
} from '../../types'
import { CountdownTimer, LoadingSpinner } from '../common'
import { CompactCard } from '../common/CompactCard'
import { apiClient } from '../../api/client'
import { usePolling } from '../../hooks'

const POLL_INTERVAL = 3000 // 3 seconds

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
  onNavigateToBattle,
  onMatchChange,
}: TeamPhaseModalProps) {
  // Sound context
  const { play } = useSoundContext()

  // Derived state
  const coins = shopData?.coins ?? 0
  const isSubmitted = shopData?.is_ready ?? false
  const teamCards = shopData?.team ?? []
  const teamOrder = shopData?.team_order ?? []
  const upgradeCost = gameConstants?.upgrade_cost ?? 1
  const teamSize = gameConstants?.team_size ?? 3
  const isTeamComplete = teamCards.length >= teamSize
  const canSubmit = isTeamComplete && !isSubmitted

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

  /**
   * Poll shop data to detect battle phase transition.
   *
   * CRITICAL: Polling continues AFTER team submission!
   *
   * The team phase modal needs to detect when both players have submitted
   * their teams and the match transitions to battle_phase. This requires
   * continuous polling even after the user has submitted.
   *
   * Polling flow:
   * 1. User organizes team, upgrades cards, and submits
   * 2. After submission, user sees "Waiting for opponent..." message
   * 3. Polling continues at 3s intervals
   * 4. When API returns status='battle_phase' or 'completed', call onNavigateToBattle()
   */
  const handleShopSuccess = useCallback((data: EnhancedShopResponse) => {
    // Update shop data for UI
    onShopDataChange(data)

    // Check for phase transition to battle (or completed - battle executes instantly)
    if (data.status === 'battle_phase' || data.status === 'completed') {
      onNavigateToBattle()
      return
    }

    // Check if match was cancelled
    if (data.status === 'cancelled') {
      onMatchChange(null)
    }
  }, [onShopDataChange, onNavigateToBattle, onMatchChange])

  const handleShopError = useCallback((err: Error) => {
    console.error('Failed to poll shop data:', err)
    // Don't show error UI during polling - just log it
  }, [])

  // Poll shop data to detect phase transitions
  usePolling({
    fetchFn: useCallback(() => apiClient.getShop(activeMatch.id), [activeMatch.id]),
    interval: POLL_INTERVAL,
    enabled: !!activeMatch,
    onSuccess: handleShopSuccess,
    onError: handleShopError,
  })

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
      play('team_place')
    } catch (err) {
      console.error('Failed to reorder team:', err)
      // Revert to server state on error
      setLocalTeamOrder(teamCards)
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, isSubmitted, teamCards, onShopDataChange, play])

  // Upgrade card handler
  const handleUpgrade = useCallback(
    async (teamSlot: number, upgradeType: UpgradeType) => {
      if (!activeMatch || actionLoading || isSubmitted) return

      setActionLoading(`upgrade-${teamSlot}-${upgradeType}`)
      try {
        const data = await apiClient.upgradeCard(activeMatch.id, teamSlot, upgradeType)
        onShopDataChange(data)
        play('team_upgrade')
      } catch (err) {
        console.error('Failed to upgrade card:', err)
        // Optionally show error to user via toast/banner
      } finally {
        setActionLoading(null)
      }
    },
    [activeMatch, actionLoading, isSubmitted, onShopDataChange, play]
  )

  // Submit team handler
  const handleSubmitTeam = useCallback(async () => {
    if (!activeMatch || actionLoading || isSubmitted) return

    setActionLoading('submit')
    try {
      const data = await apiClient.submitTeam(activeMatch.id)
      onShopDataChange(data)
      play('button_click')
    } catch (err) {
      console.error('Failed to submit team:', err)
      // Optionally show error to user via toast/banner
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, actionLoading, isSubmitted, onShopDataChange, play])

  // Coin display class based on affordability
  const getCoinClass = () => {
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
                      <CompactCard
                        imageUrl={card.card_image_url || ''}
                        positions={card.placeholder_positions}
                        currentStats={{
                          atk: card.atk,
                          def: card.def,
                          hp: card.hp,
                          maxHp: card.max_hp,
                        }}
                        hpBarThresholds={gameConstants?.hp_bar_thresholds}
                        showHpBar={true}
                        cardName={card.name}
                        cardId={card.card_id}
                        className="team-phase-compact-card"
                      />
                    </div>

                    {/* Upgrade buttons - only show if not submitted */}
                    {!isSubmitted && (
                      <div className="team-phase-card-upgrades">
                        <button
                          className="upgrade-btn upgrade-atk"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleUpgrade(card.position, 'atk')
                          }}
                          disabled={coins < upgradeCost || actionLoading !== null}
                          title={`+3 ATK (${upgradeCost} coin)`}
                        >
                          {actionLoading === `upgrade-${card.position}-atk` ? (
                            <LoadingSpinner size="sm" inline />
                          ) : (
                            <>⚔️ +3</>
                          )}
                        </button>
                        <button
                          className="upgrade-btn upgrade-hp"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleUpgrade(card.position, 'hp')
                          }}
                          disabled={coins < upgradeCost || actionLoading !== null}
                          title={`+3 HP (${upgradeCost} coin)`}
                        >
                          {actionLoading === `upgrade-${card.position}-hp` ? (
                            <LoadingSpinner size="sm" inline />
                          ) : (
                            <>❤️ +3</>
                          )}
                        </button>
                      </div>
                    )}
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

        {/* Footer with Submit Team button or waiting state */}
        <footer className="team-phase-footer">
          {!isSubmitted ? (
            <button
              className="btn-primary btn-lg submit-btn"
              onClick={handleSubmitTeam}
              disabled={!canSubmit || actionLoading !== null}
            >
              {actionLoading === 'submit' ? (
                <LoadingSpinner size="sm" inline />
              ) : isTeamComplete ? (
                'Submit Team'
              ) : (
                `Need ${teamSize - teamCards.length} more card${teamSize - teamCards.length > 1 ? 's' : ''}`
              )}
            </button>
          ) : (
            <div className="submit-status">
              <span className="submit-status-icon">✓</span>
              <span className="submit-status-text">Waiting for opponent...</span>
            </div>
          )}
        </footer>
      </div>
    </div>
  )
}

export default TeamPhaseModal
