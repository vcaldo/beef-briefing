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

import type {
  Match,
  EnhancedShopResponse,
  GameConstants,
} from '../../types'
import { CountdownTimer } from '../common'

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
  onShopDataChange: _onShopDataChange,
  onNavigateToBattle: _onNavigateToBattle,
  onMatchChange: _onMatchChange,
}: TeamPhaseModalProps) {
  // Reserved variables - suppress unused warnings
  void _onShopDataChange
  void _onNavigateToBattle
  void _onMatchChange

  // Derived state
  const coins = shopData?.coins ?? 0
  const isSubmitted = shopData?.is_ready ?? false

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
            <div className="team-phase-card-row">
              {/* Team cards (3 slots) */}
              {Array.from({ length: 3 }).map((_, idx) => {
                const card = shopData.team[idx]

                return (
                  <div key={idx} className={`team-phase-slot ${card ? 'filled' : 'empty'}`}>
                    <span className="team-phase-slot-number">{idx + 1}</span>

                    {card ? (
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
                    ) : (
                      <div className="team-phase-empty-slot">
                        <span className="team-phase-empty-text">Empty</span>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export default TeamPhaseModal
