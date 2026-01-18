/**
 * ShopPage - Main shop view for arena matches
 *
 * Features:
 * - Displays shop cards available for purchase
 * - Shows team display with upgrade options
 * - Polls for shop updates every 3 seconds
 * - Auto-navigates to Battle when match transitions to battle_phase
 * - Continues polling AFTER team submission to detect battle start
 */

import { useState, useEffect, useCallback, useRef } from 'react'
import { Reorder } from 'framer-motion'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, CountdownTimer, CompactCard } from '../common'
import { TeamPhaseModal } from './TeamPhaseModal'

import type {
  Match,
  EnhancedShopResponse,
  EnhancedShopCard,
  EnhancedTeamCard,
  UpgradeType,
  GameConstants,
} from '../../types'

// Polling interval (in ms)
const POLL_INTERVAL = 3000 // 3 seconds

interface ShopPageProps {
  chatId: number // Reserved for future use (e.g., card image fetching by chat)
  userId: number // Reserved for future use (e.g., highlight user's cards)
  activeMatch: Match | null
  onNavigateToBattle: () => void
  onMatchChange: (match: Match | null) => void
  gameConstants: GameConstants | null
}

export function ShopPage({
  chatId: _chatId, // Reserved for future use
  userId: _userId, // Reserved for future use
  activeMatch,
  onNavigateToBattle,
  onMatchChange,
  gameConstants,
}: ShopPageProps) {
  // Reserved variables - suppress unused warnings
  void _chatId
  void _userId
  // Shop state
  const [shopData, setShopData] = useState<EnhancedShopResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null) // 'buy' | 'reroll' | 'upgrade' | 'submit'
  const [isTeamPhase, setIsTeamPhase] = useState(false) // Team phase modal state

  // Refs for cleanup and preventing stale closures
  const pollIntervalRef = useRef<number | null>(null)
  const isMountedRef = useRef(true)
  const isDraggingRef = useRef(false)

  // Local state for smooth drag-and-drop (decoupled from server state)
  const [localTeamOrder, setLocalTeamOrder] = useState<EnhancedTeamCard[]>([])
  // Ref to track current order (avoids stale closure in handleDragEnd)
  const localTeamOrderRef = useRef<EnhancedTeamCard[]>([])

  // Derived state
  const coins = shopData?.coins ?? 0
  const canReroll = shopData?.can_reroll ?? false
  const isSubmitted = shopData?.is_ready ?? false
  const teamCards = shopData?.team ?? []
  const teamOrder = shopData?.team_order ?? []
  const shopCards = shopData?.cards ?? []
  const teamSize = gameConstants?.team_size ?? 3
  const cardCost = gameConstants?.card_cost ?? 3
  const rerollCost = gameConstants?.reroll_cost ?? 1
  const upgradeCost = gameConstants?.upgrade_cost ?? 1

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
      addPageAction('team_reordered', {
        match_id: activeMatch.id,
        old_order: oldCardIds,
        new_order: newCardIds,
      })
      setShopData(data)
    } catch (err) {
      console.error('Failed to reorder team:', err)
      // Revert to server state on error
      setLocalTeamOrder(teamCards)
      setError(err instanceof Error ? err.message : 'Failed to reorder team')
      if (err instanceof Error) {
        noticeError(err, { context: 'reorder_team', match_id: activeMatch.id })
      }
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, teamCards, isSubmitted])

  // Fetch shop data from API
  const fetchShop = useCallback(async () => {
    if (!isMountedRef.current || !activeMatch) return

    try {
      const data = await apiClient.getShop(activeMatch.id)

      if (!isMountedRef.current) return

      setShopData(data)
      setError(null)

      // Check for phase transition to battle (or completed - battle executes instantly)
      if (data.status === 'battle_phase' || data.status === 'completed') {
        addPageAction('match_phase_transition', {
          match_id: activeMatch.id,
          from_status: 'shop_phase',
          to_status: data.status,
        })
        onNavigateToBattle()
        return
      }

      // Check if match was cancelled
      if (data.status === 'cancelled') {
        addPageAction('match_ended_in_shop', {
          match_id: activeMatch.id,
          status: data.status,
        })
        onMatchChange(null)
        return
      }

      // First successful load
      if (loading) {
        setLoading(false)
        addPageAction('shop_loaded', {
          match_id: activeMatch.id,
          coins: data.coins,
          shop_cards: data.cards.length,
          team_cards: data.team.length,
          can_reroll: data.can_reroll,
        })
      }
    } catch (err) {
      if (!isMountedRef.current) return

      console.error('Failed to fetch shop:', err)
      setError(err instanceof Error ? err.message : 'Failed to load shop')
      setLoading(false)

      if (err instanceof Error) {
        noticeError(err, { context: 'shop_fetch', match_id: activeMatch?.id })
      }
    }
  }, [activeMatch, onNavigateToBattle, onMatchChange, loading])

  /**
   * Setup shop polling for phase transition detection.
   *
   * CRITICAL: Polling continues AFTER team submission!
   *
   * Unlike typical patterns where polling stops after user action,
   * we must keep polling to detect when the opponent submits their
   * team and the match transitions to battle_phase.
   *
   * Polling flow:
   * 1. User buys cards, upgrades, and submits team
   * 2. After submission, user sees "Waiting for opponent..." message
   * 3. Polling continues at 3s intervals
   * 4. When API returns status='battle_phase', navigate to Battle tab
   */
  useEffect(() => {
    isMountedRef.current = true

    // Fetch shop data immediately
    fetchShop()

    // Poll every 3 seconds to detect battle phase transition
    // This interval runs continuously regardless of submission status
    pollIntervalRef.current = window.setInterval(fetchShop, POLL_INTERVAL)

    // Cleanup on unmount
    return () => {
      isMountedRef.current = false
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current)
        pollIntervalRef.current = null
      }
    }
  }, [fetchShop])

  /**
   * Buy a card from the shop and add it to the team.
   *
   * Game mechanics:
   * - Each card costs cardCost (default: 3 coins)
   * - Card is added to first empty team slot
   * - IMPORTANT: First purchase permanently disables reroll (canReroll → false)
   * - Team can hold up to teamSize cards (default: 3)
   *
   * The cardIndex identifies which shop slot to purchase from (0-indexed).
   */
  const handleBuyCard = useCallback(
    async (cardIndex: number) => {
      if (!activeMatch || actionLoading) return

      setActionLoading('buy')
      try {
        const data = await apiClient.buyCard(activeMatch.id, cardIndex)
        addPageAction('card_bought', {
          match_id: activeMatch.id,
          card_index: cardIndex,
          coins_remaining: data.coins,
        })
        // After buying, data.can_reroll will be false (server enforces this)
        setShopData(data)
      } catch (err) {
        console.error('Failed to buy card:', err)
        setError(err instanceof Error ? err.message : 'Failed to buy card')
        if (err instanceof Error) {
          noticeError(err, { context: 'buy_card', match_id: activeMatch.id })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [activeMatch, actionLoading]
  )

  /**
   * Reroll the shop to get new cards.
   *
   * CRITICAL GAME MECHANIC:
   * - Reroll costs 1 coin
   * - Can ONLY reroll BEFORE buying any card
   * - After first card purchase, reroll is permanently disabled (canReroll=false)
   * - This is enforced both in UI (button disabled) and backend (API returns error)
   *
   * The canReroll flag comes from the API response and reflects server-side state.
   */
  const handleReroll = useCallback(async () => {
    // Guard: prevent action if already loading or reroll not allowed
    if (!activeMatch || actionLoading || !canReroll) return

    setActionLoading('reroll')
    try {
      const data = await apiClient.rerollShop(activeMatch.id)
      addPageAction('shop_rerolled', {
        match_id: activeMatch.id,
        coins_remaining: data.coins,
      })
      // Note: After reroll, canReroll remains true until a card is bought
      setShopData(data)
    } catch (err) {
      console.error('Failed to reroll shop:', err)
      setError(err instanceof Error ? err.message : 'Failed to reroll shop')
      if (err instanceof Error) {
        noticeError(err, { context: 'reroll_shop', match_id: activeMatch.id })
      }
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, actionLoading, canReroll])

  // Upgrade card handler
  const handleUpgrade = useCallback(
    async (teamSlot: number, upgradeType: UpgradeType) => {
      if (!activeMatch || actionLoading) return

      setActionLoading(`upgrade-${teamSlot}-${upgradeType}`)
      try {
        const data = await apiClient.upgradeCard(activeMatch.id, teamSlot, upgradeType)
        addPageAction('card_upgraded', {
          match_id: activeMatch.id,
          team_slot: teamSlot,
          upgrade_type: upgradeType,
          coins_remaining: data.coins,
        })
        setShopData(data)
      } catch (err) {
        console.error('Failed to upgrade card:', err)
        setError(err instanceof Error ? err.message : 'Failed to upgrade card')
        if (err instanceof Error) {
          noticeError(err, { context: 'upgrade_card', match_id: activeMatch.id })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [activeMatch, actionLoading]
  )

  // Submit team handler
  const handleSubmitTeam = useCallback(async () => {
    if (!activeMatch || actionLoading || isSubmitted) return

    setActionLoading('submit')
    try {
      const data = await apiClient.submitTeam(activeMatch.id)
      addPageAction('team_submitted', {
        match_id: activeMatch.id,
        team_size: data.team.length,
      })
      setShopData(data)
    } catch (err) {
      console.error('Failed to submit team:', err)
      setError(err instanceof Error ? err.message : 'Failed to submit team')
      if (err instanceof Error) {
        noticeError(err, { context: 'submit_team', match_id: activeMatch.id })
      }
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, actionLoading, isSubmitted])

  // Clear error after a timeout
  useEffect(() => {
    if (error) {
      const timeout = setTimeout(() => setError(null), 5000)
      return () => clearTimeout(timeout)
    }
  }, [error])

  // No active match - should not be on this page
  if (!activeMatch) {
    return (
      <div className="shop-page">
        <div className="shop-error">
          <p>No active match. Return to the lobby to join or create a match.</p>
        </div>
      </div>
    )
  }

  // Loading state
  if (loading) {
    return (
      <div className="shop-page">
        <LoadingSpinner message="Loading shop..." />
      </div>
    )
  }

  // Check if team is complete
  const isTeamComplete = teamCards.length >= teamSize
  const canSubmit = isTeamComplete && !isSubmitted

  // Coin display class based on affordability
  const getCoinClass = () => {
    if (coins >= cardCost) return 'coin-display can-afford'
    if (coins > 0) return 'coin-display limited'
    return 'coin-display empty'
  }

  // Render TeamPhaseModal when in team phase
  if (isTeamPhase && shopData && activeMatch) {
    return (
      <TeamPhaseModal
        shopData={shopData}
        activeMatch={activeMatch}
        gameConstants={gameConstants}
        onShopDataChange={setShopData}
        onNavigateToBattle={onNavigateToBattle}
        onMatchChange={onMatchChange}
      />
    )
  }

  return (
    <div className="shop-page">
      {/* Header with coins and timer */}
      <header className="shop-header">
        <div className="shop-header-left">
          <h1 className="shop-title">Build Your Team</h1>
          {isSubmitted && (
            <span className="shop-submitted-badge">Team Submitted</span>
          )}
        </div>
        <div className="shop-header-right">
          <div className={getCoinClass()}>
            <span className="coin-icon">🪙</span>
            <span className="coin-amount">{coins}</span>
          </div>
          {shopData?.deadline && (
            <div className="shop-timer">
              <CountdownTimer
                deadline={shopData.deadline}
                onExpire={() => {
                  addPageAction('shop_phase_expired', { match_id: activeMatch.id })
                }}
                timerThresholds={gameConstants?.timer_thresholds}
              />
            </div>
          )}
        </div>
      </header>

      {/* Error banner */}
      {error && (
        <div className="shop-error-banner" role="alert">
          <span className="shop-error-icon">⚠️</span>
          <span className="shop-error-text">{error}</span>
          <button className="shop-error-dismiss" onClick={() => setError(null)} aria-label="Dismiss error">
            ×
          </button>
        </div>
      )}

      {/* Waiting message when submitted */}
      {isSubmitted && (
        <div className="shop-waiting">
          <div className="shop-waiting-icon">⏳</div>
          <p className="shop-waiting-text">Waiting for opponent to submit their team...</p>
        </div>
      )}

      {/* Team display - sticky at top with drag-and-drop reordering */}
      <section className="shop-team-section">
        <h2 className="shop-section-title">Your Team ({teamCards.length}/{teamSize})</h2>
        <div className="team-display">
          {/* Draggable team cards */}
          {localTeamOrder.length > 0 && (
            <Reorder.Group
              axis="x"
              values={localTeamOrder}
              onReorder={handleVisualReorder}
              className="team-reorder-group"
              as="div"
            >
              {localTeamOrder.map((card: EnhancedTeamCard, idx: number) => (
                <Reorder.Item
                  key={card.card_id}
                  value={card}
                  className="team-slot filled"
                  drag={!isSubmitted}
                  onDragStart={() => { isDraggingRef.current = true }}
                  onDragEnd={handleDragEnd}
                  whileDrag={{ scale: 1.05, opacity: 0.8 }}
                  transition={{ type: 'spring', stiffness: 300, damping: 25 }}
                  as="div"
                >
                  <span className="team-slot-number">{idx + 1}</span>
                  {card.card_image_url ? (
                    <CompactCard
                      imageUrl={card.card_image_url}
                      positions={card.placeholder_positions}
                      currentStats={{
                        atk: card.atk,
                        def: card.def,
                        hp: card.hp,
                        maxHp: card.max_hp,
                      }}
                      hpBarThresholds={gameConstants?.hp_bar_thresholds}
                      isAlive={true}
                      showHpBar={true}
                      cardName={card.name}
                      cardId={card.card_id}
                    />
                  ) : (
                    <div className="team-card-simple">
                      <div className="team-card-name">{card.name}</div>
                      <div className="team-card-stats">
                        <span className="stat atk">⚔️ {card.atk}</span>
                        <span className="stat def">🛡️ {card.def}</span>
                        <span className="stat hp">❤️ {card.hp}</span>
                      </div>
                    </div>
                  )}

                  {/* Upgrade buttons - only show if not submitted */}
                  {!isSubmitted && (
                    <div className="team-card-upgrades">
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
          )}

          {/* Empty slots (not draggable) */}
          {Array.from({ length: teamSize - teamCards.length }).map((_, idx) => (
            <div key={`empty-${idx}`} className="team-slot empty">
              <span className="team-slot-number">{teamCards.length + idx + 1}</span>
              <span className="team-slot-empty-text">Empty</span>
            </div>
          ))}
        </div>
      </section>

      {/* Done Shopping button - appears when team is complete */}
      {!isSubmitted && teamCards.length >= teamSize && (
        <div className="done-shopping-section">
          <button
            className="btn-primary btn-lg done-shopping-btn"
            onClick={() => setIsTeamPhase(true)}
          >
            Done Shopping - Organize Team
          </button>
          <p className="done-shopping-hint">
            Ready to organize your team? Click above to arrange positions and make final upgrades.
          </p>
        </div>
      )}

      {/* Shop cards grid - only show if not submitted */}
      {!isSubmitted && (
        <section className="shop-cards-section">
          <div className="shop-section-header">
            <h2 className="shop-section-title">Available Cards</h2>
            {/* Reroll button */}
            <button
              className="btn-secondary reroll-btn"
              onClick={handleReroll}
              disabled={!canReroll || coins < rerollCost || actionLoading !== null}
              title={canReroll ? `Reroll (${rerollCost} coin)` : 'Cannot reroll after buying'}
            >
              {actionLoading === 'reroll' ? (
                <LoadingSpinner size="sm" inline />
              ) : (
                <>🔄 Reroll ({rerollCost})</>
              )}
            </button>
          </div>

          <div className="shop-grid">
            {shopCards.map((card: EnhancedShopCard) => {
              const isPurchased = card.is_purchased
              const canAfford = card.can_afford
              const teamFull = teamCards.length >= teamSize

              return (
                <div
                  key={card.index}
                  className={`shop-card ${isPurchased ? 'shop-card-purchased' : ''} ${
                    !canAfford && !isPurchased ? 'shop-card-disabled' : ''
                  }`}
                >
                  {/* Card image or fallback */}
                  {card.card_image_url ? (
                    <div className="shop-card-image">
                      <img
                        src={card.card_image_url}
                        alt={card.name}
                        loading="lazy"
                      />
                    </div>
                  ) : (
                    <div className="shop-card-fallback">
                      <div className="shop-card-name">{card.name}</div>
                      {card.username && (
                        <div className="shop-card-username">@{card.username}</div>
                      )}
                    </div>
                  )}

                  {/* Card stats */}
                  <div className="shop-card-stats">
                    <span className="stat atk" title="Attack">⚔️ {card.atk}</span>
                    <span className="stat def" title="Defense">🛡️ {card.def}</span>
                    <span className="stat hp" title="Health">❤️ {card.hp}</span>
                  </div>

                  {/* Buy button or status */}
                  <div className="shop-card-footer">
                    {isPurchased ? (
                      <span className="shop-card-purchased-badge">Purchased</span>
                    ) : (
                      <button
                        className="btn-primary shop-card-buy"
                        onClick={() => handleBuyCard(card.index)}
                        disabled={!canAfford || teamFull || actionLoading !== null}
                      >
                        {actionLoading === 'buy' ? (
                          <LoadingSpinner size="sm" inline />
                        ) : (
                          <>
                            Buy <span className="cost">🪙 {cardCost}</span>
                          </>
                        )}
                      </button>
                    )}
                  </div>

                </div>
              )
            })}
          </div>

          {shopCards.filter((c) => !c.is_purchased).length === 0 && (
            <div className="shop-empty">
              <p>All cards purchased! Fill your team with {teamSize} cards.</p>
            </div>
          )}
        </section>
      )}

      {/* Submit button - fixed at bottom */}
      <footer className="shop-footer">
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
            <span className="submit-status-text">Team submitted - waiting for battle</span>
          </div>
        )}
      </footer>
    </div>
  )
}

export default ShopPage
