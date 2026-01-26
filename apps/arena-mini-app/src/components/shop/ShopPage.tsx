/**
 * ShopPage - Main shop view for arena matches
 *
 * Features:
 * - Displays shop cards available for purchase
 * - Polls for shop updates every 3 seconds
 * - Auto-navigates to Battle when match transitions to battle_phase
 * - Continues polling AFTER team submission to detect battle start
 */

import { useState, useCallback, useEffect, useRef } from 'react'

import { apiClient } from '../../api/client'
import { preloadBattleAssets } from '../../utils/assetPreloader'
import { useSoundContext } from '../../contexts'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, CountdownTimer, ErrorBanner, CompactCard } from '../common'
import { GameButton, CoinDisplay, CardSlot, RPGPanel } from '../ui'
import { usePolling, useErrorBanner, usePageBackground } from '../../hooks'
import TeamPhaseModal from './TeamPhaseModal'

import type {
  Match,
  EnhancedShopResponse,
  EnhancedShopCard,
  GameConstants,
} from '../../types'

// Polling interval (in ms)
const POLL_INTERVAL = 3000 // 3 seconds

// Reroll icon - circular refresh arrows
const RerollIcon = () => (
  <svg
    width="24"
    height="24"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M21 2v6h-6" />
    <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
    <path d="M3 22v-6h6" />
    <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
  </svg>
)

interface ShopPageProps {
  activeMatch: Match | null
  onNavigateToBattle: () => void
  onMatchChange: (match: Match | null) => void
  gameConstants: GameConstants | null
}

export function ShopPage({
  activeMatch,
  onNavigateToBattle,
  onMatchChange,
  gameConstants,
}: ShopPageProps) {
  // Shop state
  const [shopData, setShopData] = useState<EnhancedShopResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<string | null>(null) // 'buy' | 'reroll'
  const [isTeamPhase, setIsTeamPhase] = useState(false) // Team phase modal state

  // Error banner with auto-dismiss
  const { error, showError, clearError } = useErrorBanner()

  // Sound effects
  const { play, playSequence, preloadCategory } = useSoundContext()
  const soundsPreloadedRef = useRef(false)
  // Track if initial shop cards have been loaded (for card_draw sound)
  const initialCardsLoadedRef = useRef(false)

  // Apply splash background image
  usePageBackground({ backgroundId: 'splash' })

  // Preload shop and team sounds on mount
  useEffect(() => {
    if (soundsPreloadedRef.current) return
    soundsPreloadedRef.current = true

    preloadCategory('shop').catch((err) => {
      console.warn('Failed to preload shop sounds:', err)
    })
    preloadCategory('team').catch((err) => {
      console.warn('Failed to preload team sounds:', err)
    })
  }, [preloadCategory])

  // Preload battle assets after shop UI is ready (while user is selecting cards)
  const battleAssetsPreloadedRef = useRef(false)
  useEffect(() => {
    if (loading || !shopData || battleAssetsPreloadedRef.current) return
    battleAssetsPreloadedRef.current = true

    // Preload battle images (effects, HP bars) for smooth animations
    preloadBattleAssets()
    // Preload battle sounds
    preloadCategory('battle').catch((err) => {
      console.warn('Failed to preload battle sounds:', err)
    })
  }, [loading, shopData, preloadCategory])

  // Derived state
  const coins = shopData?.coins ?? 0
  const canReroll = shopData?.can_reroll ?? false
  const isSubmitted = shopData?.is_ready ?? false
  const teamCards = shopData?.team ?? []
  const shopCards = shopData?.cards ?? []
  const teamSize = gameConstants?.team_size ?? 3
  const cardCost = gameConstants?.card_cost ?? 3
  const rerollCost = gameConstants?.reroll_cost ?? 1


  /**
   * Handle successful shop data fetch.
   * Detects phase transitions and triggers navigation accordingly.
   */
  const handleShopSuccess = useCallback(
    (data: EnhancedShopResponse) => {
      setShopData(data)

      // Check for phase transition to battle (or completed - battle executes instantly)
      if (data.status === 'battle_phase' || data.status === 'completed') {
        addPageAction('match_phase_transition', {
          match_id: activeMatch?.id,
          from_status: 'shop_phase',
          to_status: data.status,
        })
        onNavigateToBattle()
        return
      }

      // Check if match was cancelled
      if (data.status === 'cancelled') {
        addPageAction('match_ended_in_shop', {
          match_id: activeMatch?.id,
          status: data.status,
        })
        onMatchChange(null)
        return
      }

      // First successful load
      if (loading) {
        setLoading(false)
        addPageAction('shop_loaded', {
          match_id: activeMatch?.id,
          coins: data.coins,
          shop_cards: data.cards.length,
          team_cards: data.team.length,
          can_reroll: data.can_reroll,
        })
        // Play card_draw sound when shop cards first appear
        if (!initialCardsLoadedRef.current && data.cards.length > 0) {
          initialCardsLoadedRef.current = true
          play('arena_card_draw')
        }
      }
    },
    [activeMatch?.id, onNavigateToBattle, onMatchChange, loading, play]
  )

  /**
   * Handle shop fetch errors.
   */
  const handleShopError = useCallback(
    (err: Error) => {
      console.error('Failed to fetch shop:', err)
      showError(err, 'Failed to load shop')
      setLoading(false)
      noticeError(err, { context: 'shop_fetch', match_id: activeMatch?.id })
    },
    [activeMatch?.id, showError]
  )

  /**
   * Shop polling for phase transition detection.
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
  usePolling({
    fetchFn: useCallback(
      () => apiClient.getShop(activeMatch?.id ?? ''),
      [activeMatch?.id]
    ),
    interval: POLL_INTERVAL,
    enabled: !!activeMatch,
    onSuccess: handleShopSuccess,
    onError: handleShopError,
  })

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
        playSequence(['arena_coin_spend', 'arena_success'])
      } catch (err) {
        console.error('Failed to buy card:', err)
        showError(err, 'Failed to buy card')
        play('arena_error')
        if (err instanceof Error) {
          noticeError(err, { context: 'buy_card', match_id: activeMatch.id })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [activeMatch, actionLoading, showError, play]
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
      playSequence(['arena_card_shuffle', 'arena_card_draw'])
    } catch (err) {
      console.error('Failed to reroll shop:', err)
      showError(err, 'Failed to reroll shop')
      play('arena_error')
      if (err instanceof Error) {
        noticeError(err, { context: 'reroll_shop', match_id: activeMatch.id })
      }
    } finally {
      setActionLoading(null)
    }
  }, [activeMatch, actionLoading, canReroll, showError, play])

  // No active match - should not be on this page
  if (!activeMatch) {
    return (
      <div className="shop-page page-bg page-bg--splash">
        <div className="shop-error">
          <p>No active match. Return to the lobby to join or create a match.</p>
        </div>
      </div>
    )
  }

  // Loading state
  if (loading) {
    return (
      <div className="shop-page page-bg page-bg--splash">
        <LoadingSpinner message="Loading shop..." />
      </div>
    )
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
    <div className="shop-page rpg-shop-page page-bg page-bg--splash">
      <RPGPanel variant="outer" className="rpg-shop-outer">
        {/* Header with button, timer, and coins */}
        <RPGPanel variant="inner" className="rpg-shop-header">
          <div className="rpg-shop-header-row">
            <div className="rpg-shop-header-left">
              {teamCards.length >= teamSize && !isSubmitted && (
                <GameButton
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    playSequence(['arena_button_click', 'arena_success'])
                    setIsTeamPhase(true)
                  }}
                  className="done-btn"
                >
                  Done
                </GameButton>
              )}
              {isSubmitted && (
                <span className="rpg-shop-submitted-badge">Team Submitted</span>
              )}
            </div>
            <div className="rpg-shop-header-center">
              {shopData?.deadline && (
                <CountdownTimer
                  deadline={shopData.deadline}
                  onExpire={() => {
                    addPageAction('shop_phase_expired', { match_id: activeMatch.id })
                  }}
                  timerThresholds={gameConstants?.timer_thresholds}
                />
              )}
            </div>
            <div className="rpg-shop-header-right">
              <CoinDisplay amount={coins} size="lg" animated />
            </div>
          </div>
        </RPGPanel>

        {/* Error banner */}
        {error && (
          <ErrorBanner
            message={error}
            onDismiss={clearError}
            className="shop-error-banner"
          />
        )}

        {/* Waiting message when submitted */}
        {isSubmitted && (
          <RPGPanel variant="inner-blue" className="rpg-shop-waiting">
            <div className="rpg-shop-waiting-icon">⏳</div>
            <p className="rpg-shop-waiting-text">Waiting for opponent to submit their team...</p>
          </RPGPanel>
        )}

        {/* Shop cards grid - only show if not submitted */}
        {!isSubmitted && (
          <RPGPanel variant="inner" className="rpg-shop-cards-panel">
            {/* Reroll row with hint text and button */}
            <div className="rpg-shop-reroll-row">
              <span className="rpg-shop-reroll-hint">
                Replaces all cards. Only before first buy.
              </span>
              <GameButton
                variant="primary"
                shape="square"
                size="sm"
                onClick={handleReroll}
                disabled={!canReroll || coins < rerollCost || actionLoading !== null}
                title={`Reroll (${rerollCost} coin)`}
                className="reroll-btn"
              >
                {actionLoading === 'reroll' ? (
                  <LoadingSpinner size="sm" inline />
                ) : (
                  <RerollIcon />
                )}
              </GameButton>
            </div>
            <div className="rpg-shop-grid">
              {shopCards.map((card: EnhancedShopCard) => {
                const isPurchased = card.is_purchased
                const canAfford = card.can_afford
                const teamFull = teamCards.length >= teamSize

                // Determine CardSlot variant based on state
                const slotVariant = isPurchased
                  ? 'default'
                  : canAfford && !teamFull
                    ? 'hover'
                    : 'default'

                return (
                  <CardSlot
                    key={card.index}
                    variant={slotVariant}
                    className={`shop-card-slot ${isPurchased ? 'shop-card-purchased' : ''} ${
                      !canAfford && !isPurchased ? 'shop-card-disabled' : ''
                    }`}
                  >
                    <div className="shop-card">
                      {/* Card image with compact card or fallback */}
                      <div className="shop-card-compact">
                        {card.compact_card_image_url ? (
                          <CompactCard
                            imageUrl={card.compact_card_image_url}
                            positions={card.placeholder_positions}
                            currentStats={{
                              atk: card.atk,
                              hp: card.hp,
                              maxHp: card.hp,
                            }}
                            cardName={card.name}
                            cardId={card.card_id}
                            className="shop-compact-card"
                          />
                        ) : card.card_image_url ? (
                          <div className="shop-card-image">
                            <img
                              src={card.card_image_url}
                              alt={card.name}
                              loading="lazy"
                            />
                            <div className="shop-card-stats">
                              <span className="stat atk" title="Attack">⚔️ {card.atk}</span>
                              <span className="stat hp" title="Health">❤️ {card.hp}</span>
                            </div>
                          </div>
                        ) : (
                          <div className="shop-card-fallback">
                            <div className="shop-card-name">{card.name}</div>
                            {card.username && (
                              <div className="shop-card-username">@{card.username}</div>
                            )}
                          </div>
                        )}
                      </div>

                      {/* Buy button or status */}
                      <div className="shop-card-footer">
                        {isPurchased ? (
                          <span className="shop-card-purchased-badge">Purchased</span>
                        ) : (
                          <GameButton
                            variant="primary"
                            size="sm"
                            onClick={() => handleBuyCard(card.index)}
                            disabled={!canAfford || teamFull || actionLoading !== null}
                            className="shop-card-buy"
                          >
                            {actionLoading === 'buy' ? (
                              <LoadingSpinner size="sm" inline />
                            ) : (
                              <>
                                Buy <CoinDisplay amount={cardCost} size="sm" />
                              </>
                            )}
                          </GameButton>
                        )}
                      </div>
                    </div>
                  </CardSlot>
                )
              })}
            </div>

            {shopCards.filter((c) => !c.is_purchased).length === 0 && (
              <div className="rpg-shop-empty">
                <p>All cards purchased! Fill your team with {teamSize} cards.</p>
              </div>
            )}
          </RPGPanel>
        )}

      </RPGPanel>
    </div>
  )
}

export default ShopPage
