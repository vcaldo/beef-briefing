import { useState, useEffect, useCallback } from 'react';
import { Reorder } from 'framer-motion';
import { apiClient } from '../api/client';
import { addPageAction, noticeError } from '../newrelic';
import { useAudio } from '../hooks/useAudio';
import type { Match, ShopState, ShopCard, GameCard } from '../types';
import './ShopPage.css';

interface ShopPageProps {
  match: Match;
  userId: number;
  onBack: () => void;
  onBattleStart: () => void;
}

export function ShopPage({ match, userId: _userId, onBack, onBattleStart }: ShopPageProps) {
  const [shopState, setShopState] = useState<ShopState | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRemaining, setTimeRemaining] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const { play } = useAudio();

  // Fetch shop state
  const fetchShop = useCallback(async () => {
    try {
      const state = await apiClient.getShop(match.id);
      setShopState(state);
      setTimeRemaining(state.time_remaining_seconds);

      // If match moved to battle phase, trigger callback
      if (state.status === 'battle_phase' || state.status === 'completed') {
        onBattleStart();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load shop');
    } finally {
      setLoading(false);
    }
  }, [match.id, onBattleStart]);

  useEffect(() => {
    fetchShop();
    const interval = setInterval(fetchShop, 3000); // Poll every 3s
    return () => clearInterval(interval);
  }, [fetchShop]);

  // Countdown timer
  useEffect(() => {
    if (timeRemaining <= 0) return;
    const timer = setInterval(() => {
      setTimeRemaining(prev => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, [timeRemaining]);

  // Buy card
  const handleBuyCard = async (cardIndex: number) => {
    setError(null);
    try {
      const state = await apiClient.buyCard(match.id, cardIndex);
      setShopState(state);
      addPageAction('card_purchased', {
        match_id: match.id,
        card_index: cardIndex,
        coins_remaining: state.coins,
        team_size: state.team.length,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to buy card');
      if (err instanceof Error) {
        noticeError(err, { context: 'buy_card', match_id: match.id, card_index: cardIndex });
      }
    }
  };

  // Reroll
  const handleReroll = async () => {
    setError(null);
    try {
      const state = await apiClient.reroll(match.id);
      setShopState(state);
      addPageAction('shop_rerolled', {
        match_id: match.id,
        coins_remaining: state.coins,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reroll');
      if (err instanceof Error) {
        noticeError(err, { context: 'reroll', match_id: match.id });
      }
    }
  };

  // Upgrade card
  const handleUpgrade = async (teamSlot: number, upgradeType: 'atk' | 'hp') => {
    setError(null);
    try {
      const state = await apiClient.upgradeCard(match.id, teamSlot, upgradeType);
      setShopState(state);
      addPageAction('card_upgraded', {
        match_id: match.id,
        team_slot: teamSlot,
        upgrade_type: upgradeType,
        coins_remaining: state.coins,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to upgrade');
      if (err instanceof Error) {
        noticeError(err, { context: 'upgrade', match_id: match.id, team_slot: teamSlot, upgrade_type: upgradeType });
      }
    }
  };

  // Submit team
  const handleSubmit = async () => {
    setError(null);
    try {
      const state = await apiClient.submitTeam(match.id);
      setShopState(state);
      addPageAction('team_submitted', {
        match_id: match.id,
        team_size: state.team.length,
        coins_remaining: state.coins,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit team');
      if (err instanceof Error) {
        noticeError(err, { context: 'submit_team', match_id: match.id });
      }
    }
  };

  // Handle drag start - haptic feedback
  const handleDragStart = () => {
    setIsDragging(true);
    if (navigator.vibrate) {
      navigator.vibrate(50);
    }
  };

  // Handle reorder with optimistic updates
  const handleReorder = async (newOrder: GameCard[]) => {
    if (!shopState) return;

    // Optimistic update
    setShopState({ ...shopState, team: newOrder });
    play('place');
    setIsDragging(false);

    // Persist to API
    try {
      // Convert new order to indices based on original team positions
      const indices = newOrder.map(card =>
        shopState.team.findIndex(originalCard => originalCard.card_id === card.card_id)
      );
      const state = await apiClient.setTeamOrder(match.id, indices);
      setShopState(state);
      addPageAction('team_reordered', {
        match_id: match.id,
      });
    } catch (err) {
      // Revert on error
      setShopState(shopState);
      setError(err instanceof Error ? err.message : 'Failed to reorder team');
      if (err instanceof Error) {
        noticeError(err, { context: 'reorder_team', match_id: match.id });
      }
    }
  };

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

  const remainingCardsNeeded = 3 - shopState.team.length;
  const coinsNeededForCards = remainingCardsNeeded * 2; // CardCost = 2
  const canBuy = shopState.coins >= 2 && shopState.team.length < 3;
  const canReroll = shopState.coins >= (1 + coinsNeededForCards); // RerollCost = 1
  const canUpgrade = shopState.coins >= (2 + coinsNeededForCards); // UpgradeCost = 2
  const canSubmit = shopState.team.length === 3 && !shopState.is_ready;

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
                Reroll (1)
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
            <Reorder.Group
              axis="x"
              values={shopState.team}
              onReorder={handleReorder}
              className={`team-slots ${isDragging ? 'dragging-active' : ''}`}
            >
              {shopState.team.map((card, index) => {
                const slotLabel = index === 0 ? 'Front' : index === 1 ? 'Mid' : 'Back';
                return (
                  <div key={card.card_id} className="team-slot-wrapper">
                    <div className="slot-label">{slotLabel}</div>
                    <TeamCard
                      card={card}
                      canUpgrade={canUpgrade}
                      onUpgradeAtk={() => handleUpgrade(index, 'atk')}
                      onUpgradeHp={() => handleUpgrade(index, 'hp')}
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
      <div className="card-photo">
        {card.photo_url ? (
          <img src={card.photo_url} alt={card.name} />
        ) : (
          <div className="no-photo">{card.name[0]}</div>
        )}
      </div>
      <div className="card-info">
        <div className="card-name">{card.name}</div>
        <div className="card-stats">
          <span className="stat-badge stat-atk">ATK {card.atk}</span>
          <span className="stat-badge stat-def">DEF {card.def}</span>
          <span className="stat-badge stat-hp">HP {card.hp}</span>
        </div>
      </div>
      {!card.is_purchased && (
        <button
          className="btn btn-primary buy-btn"
          onClick={onBuy}
          disabled={!canBuy}
        >
          Buy (2)
        </button>
      )}
      {card.is_purchased && (
        <div className="purchased-badge">Purchased</div>
      )}
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
      <div className="card-photo">
        {card.photo_url ? (
          <img src={card.photo_url} alt={card.name} draggable={false} />
        ) : (
          <div className="no-photo">{card.name[0]}</div>
        )}
      </div>
      <div className="card-info">
        <div className="card-name">{card.name}</div>
        <div className="card-stats">
          <span className="stat-badge stat-atk">
            ATK {card.atk}
            {card.atk_upgrades > 0 && <span className="upgrade-count">+{card.atk_upgrades}</span>}
          </span>
          <span className="stat-badge stat-hp">
            HP {card.hp}
            {card.hp_upgrades > 0 && <span className="upgrade-count">+{card.hp_upgrades * 3}</span>}
          </span>
        </div>
      </div>
      <div className="upgrade-buttons">
        <button
          className="btn btn-secondary upgrade-btn"
          onClick={(e) => {
            e.stopPropagation();
            onUpgradeAtk();
          }}
          disabled={!canUpgrade}
          title="Upgrade ATK (+1)"
        >
          +ATK
        </button>
        <button
          className="btn btn-secondary upgrade-btn"
          onClick={(e) => {
            e.stopPropagation();
            onUpgradeHp();
          }}
          disabled={!canUpgrade}
          title="Upgrade HP (+3)"
        >
          +HP
        </button>
      </div>
    </Reorder.Item>
  );
}
