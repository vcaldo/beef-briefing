import { useState, useEffect, useCallback, useRef } from 'react'
import { apiClient } from '../../api/client'
import { LoadingSpinner } from '../common/LoadingSpinner'
import { ErrorDisplay } from '../common/ErrorDisplay'
import { CountdownTimer } from '../common/CountdownTimer'
import { ShopCards } from './ShopCards'
import { TeamDisplay } from './TeamDisplay'
import { CoinDisplay } from './CoinDisplay'
import { RerollButton } from './RerollButton'
import { SubmitButton } from './SubmitButton'
import type { ShopResponse, GamePhase } from '../../types'

/** Polling interval in milliseconds (3 seconds as per plan) */
const POLL_INTERVAL = 3000

interface ShopPageProps {
  /** Match ID to load shop for */
  matchId: string | null
  /** Current user's Telegram ID */
  currentUserId?: number
  /** Callback to navigate to a different tab */
  onTabChange?: (tab: GamePhase) => void
  /** Callback when match transitions to battle */
  onBattleStart?: (matchId: string) => void
}

/**
 * Shop page for building and upgrading teams.
 * Handles card purchasing, rerolling, upgrading, and team submission.
 * Polls for shop state updates and auto-navigates to battle when phase changes.
 */
export function ShopPage({
  matchId,
  currentUserId: _currentUserId,
  onTabChange,
  onBattleStart,
}: ShopPageProps) {
  const [shop, setShop] = useState<ShopResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  // Track previous status for phase transition detection
  const prevStatusRef = useRef<string | null>(null)

  /**
   * Fetch shop state from the API
   */
  const fetchShop = useCallback(async (showLoading = false) => {
    if (!matchId) return

    try {
      if (showLoading) setLoading(true)
      const data = await apiClient.getShop(matchId)
      setShop(data)
      setError(null)

      // Check for phase transition to battle
      if (prevStatusRef.current === 'shop_phase' && data.status === 'battle_phase') {
        onBattleStart?.(matchId)
        onTabChange?.('battle')
      }

      prevStatusRef.current = data.status
    } catch (err) {
      console.error('Failed to fetch shop:', err)
      setError(err instanceof Error ? err.message : 'Failed to load shop')
    } finally {
      if (showLoading) setLoading(false)
    }
  }, [matchId, onBattleStart, onTabChange])

  /**
   * Buy a card from the shop
   */
  const handleBuyCard = async (cardIndex: number) => {
    if (!matchId) return

    try {
      setActionLoading(`buy-${cardIndex}`)
      const updatedShop = await apiClient.buyCard(matchId, cardIndex)
      setShop(updatedShop)
    } catch (err) {
      console.error('Failed to buy card:', err)
      setError(err instanceof Error ? err.message : 'Failed to buy card')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Reroll shop cards (only available before first purchase)
   */
  const handleReroll = async () => {
    if (!matchId) return

    try {
      setActionLoading('reroll')
      const updatedShop = await apiClient.rerollCards(matchId)
      setShop(updatedShop)
    } catch (err) {
      console.error('Failed to reroll:', err)
      setError(err instanceof Error ? err.message : 'Failed to reroll cards')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Upgrade a card in the team
   */
  const handleUpgrade = async (teamSlot: number, upgradeType: 'atk' | 'hp') => {
    if (!matchId) return

    try {
      setActionLoading(`upgrade-${teamSlot}-${upgradeType}`)
      const updatedShop = await apiClient.upgradeCard(matchId, teamSlot, upgradeType)
      setShop(updatedShop)
    } catch (err) {
      console.error('Failed to upgrade card:', err)
      setError(err instanceof Error ? err.message : 'Failed to upgrade card')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Set team order
   */
  const handleSetOrder = async (order: number[]) => {
    if (!matchId) return

    try {
      setActionLoading('order')
      const updatedShop = await apiClient.setTeamOrder(matchId, order)
      setShop(updatedShop)
    } catch (err) {
      console.error('Failed to set order:', err)
      setError(err instanceof Error ? err.message : 'Failed to set team order')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Submit team for battle
   */
  const handleSubmit = async () => {
    if (!matchId) return

    try {
      setActionLoading('submit')
      const updatedShop = await apiClient.submitTeam(matchId)
      setShop(updatedShop)
    } catch (err) {
      console.error('Failed to submit team:', err)
      setError(err instanceof Error ? err.message : 'Failed to submit team')
    } finally {
      setActionLoading(null)
    }
  }

  // Initial fetch
  useEffect(() => {
    if (matchId) {
      fetchShop(true)
    }
  }, [matchId, fetchShop])

  // Set up polling
  useEffect(() => {
    if (!matchId) return

    const interval = setInterval(() => {
      fetchShop(false)
    }, POLL_INTERVAL)

    return () => clearInterval(interval)
  }, [matchId, fetchShop])

  // No match selected
  if (!matchId) {
    return (
      <div className="shop-page">
        <div className="empty-state">
          <div className="empty-state-icon">🛒</div>
          <h3 className="empty-state-title">No Match Selected</h3>
          <p className="empty-state-text">
            Join a match from the Lobby to start building your team.
          </p>
          <button
            className="btn btn-primary mt-4"
            onClick={() => onTabChange?.('lobby')}
          >
            Go to Lobby
          </button>
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

  // Error state
  if (error && !shop) {
    return (
      <div className="shop-page">
        <ErrorDisplay
          title="Failed to Load Shop"
          message={error}
          onRetry={() => fetchShop(true)}
        />
      </div>
    )
  }

  // No shop data
  if (!shop) {
    return (
      <div className="shop-page">
        <div className="empty-state">
          <div className="empty-state-icon">🛒</div>
          <h3 className="empty-state-title">Shop Not Available</h3>
          <p className="empty-state-text">
            The shop is not available for this match.
          </p>
        </div>
      </div>
    )
  }

  // Team submitted - waiting state
  if (shop.team_submitted) {
    return (
      <div className="shop-page">
        {/* Header with timer */}
        <div className="shop-header">
          <div className="shop-header-info">
            <CoinDisplay coins={shop.coins} />
          </div>
          <CountdownTimer
            timeRemainingSeconds={shop.time_remaining_seconds}
            compact
          />
        </div>

        {/* Team display (read-only) */}
        <div className="section-header">
          <h2 className="section-title">Your Team</h2>
        </div>
        <TeamDisplay
          team={shop.team}
          teamOrder={shop.team_order}
          readOnly
        />

        {/* Waiting state */}
        <div className="waiting-state">
          <div className="waiting-spinner" />
          <p className="waiting-text">Waiting for others...</p>
          <p className="text-sm text-[var(--tg-theme-hint-color)] mt-2">
            Your team has been submitted. The battle will start when all players are ready.
          </p>
        </div>
      </div>
    )
  }

  // Check if any cards have been purchased (for reroll button)
  const hasPurchasedCard = shop.cards.some(card => card.is_purchased)

  return (
    <div className="shop-page">
      {/* Error banner (non-blocking) */}
      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button onClick={() => setError(null)}>✕</button>
        </div>
      )}

      {/* Header with coins and timer */}
      <div className="shop-header">
        <div className="shop-header-info">
          <CoinDisplay coins={shop.coins} />
        </div>
        <CountdownTimer
          timeRemainingSeconds={shop.time_remaining_seconds}
          compact
        />
      </div>

      {/* Team display with upgrade buttons */}
      <div className="section-header">
        <h2 className="section-title">Your Team</h2>
        <span className="text-sm text-[var(--tg-theme-hint-color)]">
          {shop.team.length}/3 cards
        </span>
      </div>
      <TeamDisplay
        team={shop.team}
        teamOrder={shop.team_order}
        onUpgradeAtk={(slot) => handleUpgrade(slot, 'atk')}
        onUpgradeHp={(slot) => handleUpgrade(slot, 'hp')}
        onReorder={handleSetOrder}
        upgradeLoading={actionLoading?.startsWith('upgrade')}
        orderLoading={actionLoading === 'order'}
      />

      {/* Shop cards */}
      <div className="section-header">
        <h2 className="section-title">Shop</h2>
        <RerollButton
          canReroll={shop.affordability.can_reroll && !hasPurchasedCard}
          disabledReason={
            hasPurchasedCard
              ? 'Cannot reroll after buying'
              : shop.affordability.reroll_disabled_reason
          }
          isLoading={actionLoading === 'reroll'}
          onReroll={handleReroll}
        />
      </div>
      <ShopCards
        cards={shop.cards}
        onBuy={handleBuyCard}
        buyingCardIndex={
          actionLoading?.startsWith('buy-')
            ? parseInt(actionLoading.split('-')[1])
            : undefined
        }
      />

      {/* Submit button */}
      <div className="shop-actions">
        <SubmitButton
          canSubmit={shop.affordability.can_submit}
          disabledReason={shop.affordability.submit_disabled_reason}
          isLoading={actionLoading === 'submit'}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  )
}
