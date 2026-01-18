/**
 * ShopPage - Main shop view for arena matches
 *
 * Features:
 * - Displays shop cards available for purchase
 * - Polls for shop updates every 3 seconds
 * - Auto-navigates to Battle when match transitions to battle_phase
 * - Continues polling AFTER team submission to detect battle start
 */

import { useState, useEffect, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, CountdownTimer } from '../common'
import TeamPhaseModal from './TeamPhaseModal'

import type {
  Match,
  EnhancedShopResponse,
  EnhancedShopCard,
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
  const [actionLoading, setActionLoading] = useState<string | null>(null) // 'buy' | 'reroll'
  const [isTeamPhase, setIsTeamPhase] = useState(false) // Team phase modal state

  // Refs for cleanup and preventing stale closures
  const pollIntervalRef = useRef<number | null>(null)
  const isMountedRef = useRef(true)

  // Derived state
  const coins = shopData?.coins ?? 0
  const canReroll = shopData?.can_reroll ?? false
  const isSubmitted = shopData?.is_ready ?? false
  const teamCards = shopData?.team ?? []
  const shopCards = shopData?.cards ?? []
  const teamSize = gameConstants?.team_size ?? 3
  const cardCost = gameConstants?.card_cost ?? 3
  const rerollCost = gameConstants?.reroll_cost ?? 1


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

    </div>
  )
}

export default ShopPage
