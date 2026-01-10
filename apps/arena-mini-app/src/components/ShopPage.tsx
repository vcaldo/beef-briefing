import { useState, useEffect, useCallback } from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
  DragEndEvent,
  DragStartEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  horizontalListSortingStrategy,
  useSortable,
  arrayMove,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { apiClient } from '../api/client';
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

  // Drag-and-drop sensors with touch support
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
    useSensor(TouchSensor, {
      activationConstraint: { delay: 150, tolerance: 5 },
    }),
    useSensor(KeyboardSensor)
  );

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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to buy card');
    }
  };

  // Reroll
  const handleReroll = async () => {
    setError(null);
    try {
      const state = await apiClient.reroll(match.id);
      setShopState(state);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reroll');
    }
  };

  // Upgrade card
  const handleUpgrade = async (teamSlot: number, upgradeType: 'atk' | 'hp') => {
    setError(null);
    try {
      const state = await apiClient.upgradeCard(match.id, teamSlot, upgradeType);
      setShopState(state);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to upgrade');
    }
  };

  // Submit team
  const handleSubmit = async () => {
    setError(null);
    try {
      const state = await apiClient.submitTeam(match.id);
      setShopState(state);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit team');
    }
  };

  // Drag handlers
  const handleDragStart = (_event: DragStartEvent) => {
    setIsDragging(true);
    // Haptic feedback on mobile
    if (navigator.vibrate) {
      navigator.vibrate(50);
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    setIsDragging(false);
    const { active, over } = event;

    if (!over || !shopState || active.id === over.id) return;

    const oldIndex = shopState.team.findIndex(c => c.card_id === active.id);
    const newIndex = shopState.team.findIndex(c => c.card_id === over.id);

    if (oldIndex === -1 || newIndex === -1) return;

    // Optimistic update
    const newTeam = arrayMove(shopState.team, oldIndex, newIndex);
    setShopState({ ...shopState, team: newTeam });
    play('place');

    // Persist to API
    try {
      const newOrder = newTeam.map(c => c.card_id);
      const state = await apiClient.setTeamOrder(match.id, newOrder);
      setShopState(state);
    } catch (err) {
      // Revert on error
      setShopState(shopState);
      setError(err instanceof Error ? err.message : 'Failed to reorder team');
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

  const canBuy = shopState.coins >= 2 && shopState.team.length < 3;
  const canReroll = shopState.coins >= 1;
  const canUpgrade = shopState.coins >= 2;
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
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragStart={handleDragStart}
              onDragEnd={handleDragEnd}
            >
              <SortableContext
                items={shopState.team.map(c => c.card_id)}
                strategy={horizontalListSortingStrategy}
              >
                <div className={`team-slots ${isDragging ? 'dragging-active' : ''}`}>
                  {[0, 1, 2].map(slot => {
                    const card = shopState.team[slot];
                    const slotLabel = slot === 0 ? 'Front' : slot === 1 ? 'Mid' : 'Back';
                    return (
                      <div key={slot} className="team-slot">
                        <div className="slot-label">{slotLabel}</div>
                        {card ? (
                          <SortableTeamCard
                            card={card}
                            canUpgrade={canUpgrade}
                            onUpgradeAtk={() => handleUpgrade(slot, 'atk')}
                            onUpgradeHp={() => handleUpgrade(slot, 'hp')}
                          />
                        ) : (
                          <div className="empty-slot">
                            <span>Empty</span>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </SortableContext>
            </DndContext>
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

// Sortable Team Card Component (with drag-and-drop)
interface SortableTeamCardProps {
  card: GameCard;
  canUpgrade: boolean;
  onUpgradeAtk: () => void;
  onUpgradeHp: () => void;
}

function SortableTeamCard({ card, canUpgrade, onUpgradeAtk, onUpgradeHp }: SortableTeamCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: card.card_id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
    zIndex: isDragging ? 100 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`team-card ${isDragging ? 'dragging' : ''}`}
      {...attributes}
    >
      {/* Drag Handle */}
      <div className="drag-handle" {...listeners}>
        <span className="drag-icon">&#x2630;</span>
      </div>

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
          onClick={onUpgradeAtk}
          disabled={!canUpgrade}
          title="Upgrade ATK (+1)"
        >
          +ATK
        </button>
        <button
          className="btn btn-secondary upgrade-btn"
          onClick={onUpgradeHp}
          disabled={!canUpgrade}
          title="Upgrade HP (+3)"
        >
          +HP
        </button>
      </div>
    </div>
  );
}
