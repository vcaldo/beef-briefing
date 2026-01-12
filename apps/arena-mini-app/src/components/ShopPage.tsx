import { useState, useEffect, useCallback } from 'react';
import { Reorder } from 'framer-motion';
import { apiClient } from '../api/client';
import { addPageAction } from '@beef-briefing/shared-mini-app/monitoring';
import { useAudio } from '../hooks/useAudio';
import { usePolling } from '../hooks/usePolling';
import { useApiCall } from '../hooks/useApiCall';
import { POLLING_INTERVALS, SHOP_COSTS, TEAM } from '../config/constants';
import type { Match, ShopState, ShopCard, GameCard } from '../types';
import { ShopCardImage } from './ShopCardImage';
import './ShopPage.css';

/**
 * Applies team order to get cards in battle position order.
 * team_order contains indices into the team array.
 * Example: team_order [2, 0, 1] means position 0 gets team[2], position 1 gets team[0], etc.
 */
function applyTeamOrder(team: GameCard[], teamOrder: number[]): GameCard[] {
  if (teamOrder.length !== team.length) {
    // Fallback to original team if lengths don't match
    if (import.meta.env.DEV) {
      console.warn('Team order length mismatch', { teamLen: team.length, orderLen: teamOrder.length });
    }
    return team;
  }
  return teamOrder.map(index => team[index]);
}

interface ShopPageProps {
  match: Match;
  userId: number;
  onBack: () => void;
  onBattleStart: () => void;
}

export function ShopPage({ match, userId: _userId, onBack, onBattleStart }: ShopPageProps) {
  const [error, setError] = useState<string | null>(null);
  const [timeRemaining, setTimeRemaining] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const { play } = useAudio();

  // Poll shop state
  const { data: shopState, loading } = usePolling(
    async () => {
      const state = await apiClient.getShop(match.id);
      setTimeRemaining(state.time_remaining_seconds);

      // Trigger battle start if phase changed
      if (state.status === 'battle_phase' || state.status === 'completed') {
        onBattleStart();
      }

      return state;
    },
    POLLING_INTERVALS.SHOP,
  );

  // Countdown timer
  useEffect(() => {
    if (timeRemaining <= 0) return;
    const timer = setInterval(() => {
      setTimeRemaining(prev => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, [timeRemaining]);

  // Buy card
  const buyCardApi = useApiCall<ShopState>({
    context: 'buy_card',
    onError: (err) => setError(err),
  });

  const handleBuyCard = useCallback(
    async (cardIndex: number) => {
      const state = await buyCardApi.execute(() => apiClient.buyCard(match.id, cardIndex));
      if (state) {
        addPageAction('card_purchased', {
          match_id: match.id,
          card_index: cardIndex,
          coins_remaining: state.coins,
          team_size: state.team.length,
        });
      }
    },
    [buyCardApi, match.id],
  );

  // Reroll
  const rerollApi = useApiCall<ShopState>({
    context: 'reroll',
    onError: (err) => setError(err),
  });

  const handleReroll = useCallback(async () => {
    const state = await rerollApi.execute(() => apiClient.reroll(match.id));
    if (state) {
      addPageAction('shop_rerolled', {
        match_id: match.id,
        coins_remaining: state.coins,
      });
    }
  }, [rerollApi, match.id]);

  // Upgrade card
  const upgradeApi = useApiCall<ShopState>({
    context: 'upgrade',
    onError: (err) => setError(err),
  });

  const handleUpgrade = useCallback(
    async (teamSlot: number, upgradeType: 'atk' | 'hp') => {
      const state = await upgradeApi.execute(() =>
        apiClient.upgradeCard(match.id, teamSlot, upgradeType),
      );
      if (state) {
        addPageAction('card_upgraded', {
          match_id: match.id,
          team_slot: teamSlot,
          upgrade_type: upgradeType,
          coins_remaining: state.coins,
        });
      }
    },
    [upgradeApi, match.id],
  );

  // Submit team
  const submitApi = useApiCall<ShopState>({
    context: 'submit_team',
    onError: (err) => setError(err),
  });

  const handleSubmit = useCallback(async () => {
    const state = await submitApi.execute(() => apiClient.submitTeam(match.id));
    if (state) {
      addPageAction('team_submitted', {
        match_id: match.id,
        team_size: state.team.length,
        coins_remaining: state.coins,
      });
    }
  }, [submitApi, match.id]);

  // Handle drag start - haptic feedback
  const handleDragStart = () => {
    setIsDragging(true);
    if (navigator.vibrate) {
      navigator.vibrate(50);
    }
  };

  // Reorder team
  const reorderApi = useApiCall<ShopState>({
    context: 'reorder_team',
    onError: (err) => setError(err),
  });

  // Handle reorder - persist to API
  const handleReorder = useCallback(
    async (newOrder: GameCard[]) => {
      if (!shopState) return;

      // Calculate new team_order based on where each card in newOrder came from in the original team
      const newTeamOrder = newOrder.map(card =>
        shopState.team.findIndex(originalCard => originalCard.card_id === card.card_id),
      );

      play('place');
      setIsDragging(false);

      // Persist to API
      const state = await reorderApi.execute(() =>
        apiClient.setTeamOrder(match.id, newTeamOrder),
      );
      if (state) {
        addPageAction('team_reordered', {
          match_id: match.id,
        });
      }
    },
    [shopState, reorderApi, match.id, play],
  );

  // Format time
  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  if (loading) {
    return (
      <div className="shop-page">
        <div className="shop-loading">
          <div className="spinner" />
        </div>
      </div>
    );
  }

  if (!shopState) {
    return (
      <div className="shop-page">
        <div className="shop-error">Failed to load shop</div>
        <button className="btn btn-secondary" onClick={onBack}>Back</button>
      </div>
    );
  }

  const remainingCardsNeeded = TEAM.MAX_SIZE - shopState.team.length;
  const coinsNeededForCards = remainingCardsNeeded * SHOP_COSTS.CARD;
  const canBuy = shopState.coins >= SHOP_COSTS.CARD && shopState.team.length < TEAM.MAX_SIZE;
  const canReroll = shopState.coins >= (SHOP_COSTS.REROLL + coinsNeededForCards);
  const canUpgrade = shopState.coins >= (SHOP_COSTS.UPGRADE + coinsNeededForCards);
  const canSubmit = shopState.team.length === TEAM.MAX_SIZE && !shopState.is_ready;

  return (
    <div className="shop-page">
      {/* Header */}
      <header className="shop-header">
        <button className="btn btn-secondary back-btn" onClick={onBack}>
          Back
        </button>
        <div className={`timer ${timeRemaining < 30 ? 'warning' : ''}`}>
          {formatTime(timeRemaining)}
        </div>
        <div className="coins">
          {shopState.coins}
        </div>
      </header>

      {error && (
        <div className="shop-error-banner">{error}</div>
      )}

      {shopState.is_ready ? (
        <div className="shop-waiting">
          <h2>Team Submitted!</h2>
          <p>Waiting for other players...</p>
          <div className="spinner" />
        </div>
      ) : (
        <>
          {/* Available Cards */}
          <section className="shop-section">
            <div className="section-header">
              <h2>Shop</h2>
              <button
                className="btn btn-secondary reroll-btn"
                onClick={handleReroll}
                disabled={!canReroll}
              >
                Reroll ({SHOP_COSTS.REROLL})
              </button>
            </div>
            <div className="shop-cards">
              {shopState.cards.map((card, index) => (
                <ShopCardComponent
                  key={card.card_id}
                  card={card}
                  onBuy={() => handleBuyCard(index)}
                  canBuy={canBuy && !card.is_purchased}
                />
              ))}
            </div>
          </section>

          {/* Team */}
          <section className="shop-section">
            <div className="section-header">
              <h2>Your Team ({shopState.team.length}/3)</h2>
              {shopState.team.length > 1 && (
                <span className="drag-hint">Drag to reorder</span>
              )}
            </div>

            {/* Apply team order for display */}
            {(() => {
              const orderedTeam = applyTeamOrder(shopState.team, shopState.team_order);

              return (
                <Reorder.Group
                  axis="x"
                  values={orderedTeam}
                  onReorder={handleReorder}
                  className={`team-slots ${isDragging ? 'dragging-active' : ''}`}
                >
                  {orderedTeam.map((card, visualIndex) => {
                    const slotLabel = visualIndex === 0 ? 'Front' : visualIndex === 1 ? 'Mid' : 'Back';
                    // Translate visual position to team array index for upgrade handlers
                    const teamArrayIndex = shopState.team_order[visualIndex];

                    return (
                      <div key={card.card_id} className="team-slot-wrapper">
                        <div className="slot-label">{slotLabel}</div>
                        <TeamCard
                          card={card}
                          canUpgrade={canUpgrade}
                          onUpgradeAtk={() => handleUpgrade(teamArrayIndex, 'atk')}
                          onUpgradeHp={() => handleUpgrade(teamArrayIndex, 'hp')}
                          onDragStart={handleDragStart}
                        />
                      </div>
                    );
                  })}

                  {/* Empty slots for remaining positions */}
                  {[...Array(3 - shopState.team.length)].map((_, i) => {
                    const slotIndex = shopState.team.length + i;
                    const slotLabel = slotIndex === 0 ? 'Front' : slotIndex === 1 ? 'Mid' : 'Back';
                    return (
                      <div key={`empty-${slotIndex}`} className="team-slot-wrapper">
                        <div className="slot-label">{slotLabel}</div>
                        <div className="empty-slot">
                          <span>Empty</span>
                        </div>
                      </div>
                    );
                  })}
                </Reorder.Group>
              );
            })()}
          </section>

          {/* Submit */}
          <section className="shop-actions">
            <button
              className="btn btn-success submit-btn"
              onClick={handleSubmit}
              disabled={!canSubmit}
            >
              {canSubmit ? 'Ready for Battle!' : `Need ${3 - shopState.team.length} more card(s)`}
            </button>
          </section>
        </>
      )}
    </div>
  );
}

// Shop Card Component
interface ShopCardComponentProps {
  card: ShopCard;
  onBuy: () => void;
  canBuy: boolean;
}

function ShopCardComponent({ card, onBuy, canBuy }: ShopCardComponentProps) {
  return (
    <div className={`shop-card ${card.is_purchased ? 'purchased' : ''}`}>
      {/* Full card image with 2:3 aspect ratio */}
      <ShopCardImage
        imageUrl={card.card_image_url}
        name={card.name}
        fallbackPhotoUrl={card.photo_url}
      />

      {/* Info below card */}
      <div className="shop-card-footer">
        <div className="shop-card-name">{card.name}</div>
        <div className="shop-card-stats-row">
          <span className="stat-badge stat-atk">ATK {card.atk}</span>
          <span className="stat-badge stat-def">DEF {card.def}</span>
          <span className="stat-badge stat-hp">HP {card.hp}</span>
        </div>

        {!card.is_purchased ? (
          <button
            className="btn btn-primary buy-btn"
            onClick={onBuy}
            disabled={!canBuy}
          >
            Buy ({SHOP_COSTS.CARD})
          </button>
        ) : (
          <div className="purchased-badge">Purchased</div>
        )}
      </div>
    </div>
  );
}

// Team Card Component (with framer-motion drag)
interface TeamCardProps {
  card: GameCard;
  canUpgrade: boolean;
  onUpgradeAtk: () => void;
  onUpgradeHp: () => void;
  onDragStart: () => void;
}

function TeamCard({
  card,
  canUpgrade,
  onUpgradeAtk,
  onUpgradeHp,
  onDragStart
}: TeamCardProps) {
  // Prefer card image over avatar photo
  const imageUrl = card.card_image_url || card.photo_url;

  return (
    <Reorder.Item
      value={card}
      onDragStart={onDragStart}
      className="team-card"
      dragListener={true}
      whileDrag={{
        scale: 1.05,
        boxShadow: "0 8px 24px rgba(0, 0, 0, 0.4)",
        borderColor: "var(--accent-gold)",
        zIndex: 100
      }}
      transition={{
        type: "spring",
        damping: 25,
        stiffness: 200
      }}
    >
      <div className="team-avatar">
        {imageUrl ? (
          <img src={imageUrl} alt={card.name} draggable={false} />
        ) : (
          <div className="team-avatar-fallback">{card.name[0]}</div>
        )}
      </div>
      <div className="team-card-info">
        <div className="team-card-name">{card.name}</div>
        <div className="team-card-stats-column">
          <div className="stat-badge-with-upgrade stat-atk">
            <span className="stat-content">
              ATK {card.atk}
              {card.atk_upgrades > 0 && <span className="upgrade-count">+{card.atk_upgrades}</span>}
            </span>
            {canUpgrade && (
              <button
                className="stat-upgrade-btn"
                onClick={(e) => {
                  e.stopPropagation();
                  e.preventDefault();
                  onUpgradeAtk();
                }}
                title="Upgrade ATK (+1)"
                aria-label={`Upgrade ${card.name} ATK by 1 for ${SHOP_COSTS.UPGRADE} coins`}
              >
                +
              </button>
            )}
          </div>
          <div className="stat-badge-with-upgrade stat-hp">
            <span className="stat-content">
              HP {card.hp}
              {card.hp_upgrades > 0 && <span className="upgrade-count">+{card.hp_upgrades * 3}</span>}
            </span>
            {canUpgrade && (
              <button
                className="stat-upgrade-btn"
                onClick={(e) => {
                  e.stopPropagation();
                  e.preventDefault();
                  onUpgradeHp();
                }}
                title="Upgrade HP (+3)"
                aria-label={`Upgrade ${card.name} HP by 3 for ${SHOP_COSTS.UPGRADE} coins`}
              >
                +
              </button>
            )}
          </div>
        </div>
      </div>
    </Reorder.Item>
  );
}
