package services

import (
	"context"
	"time"

	"beef-briefing/apps/api-service/internal/game/shop"
)

// Shop delegation methods - ArenaService delegates to ShopService

// GetShop retrieves the enhanced shop state for a player.
func (s *ArenaService) GetShop(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	return s.shopService.GetShop(ctx, matchID, userID)
}

// BuyCard purchases a card from the shop.
func (s *ArenaService) BuyCard(ctx context.Context, matchID string, userID int64, cardIndex int) (*EnhancedShopResponse, error) {
	return s.shopService.BuyCard(ctx, matchID, userID, cardIndex)
}

// Reroll replaces unpurchased cards with new ones.
func (s *ArenaService) Reroll(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	return s.shopService.Reroll(ctx, matchID, userID)
}

// UpgradeCard applies an upgrade to a team card.
func (s *ArenaService) UpgradeCard(ctx context.Context, matchID string, userID int64, teamSlot int, upgradeType shop.UpgradeType) (*EnhancedShopResponse, error) {
	return s.shopService.UpgradeCard(ctx, matchID, userID, teamSlot, upgradeType)
}

// SetTeamOrder sets the battle order for a player's team.
func (s *ArenaService) SetTeamOrder(ctx context.Context, matchID string, userID int64, order []int) (*EnhancedShopResponse, error) {
	return s.shopService.SetTeamOrder(ctx, matchID, userID, order)
}

// SubmitTeam submits the team for battle and triggers battle start check.
func (s *ArenaService) SubmitTeam(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	// Call shop service to submit the team
	if err := s.shopService.SubmitTeam(ctx, matchID, userID); err != nil {
		return nil, err
	}

	// Check if all participants are ready and start battle if so
	// Create detached context for background battle check with timeout protection
	asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		defer cancel()
		s.checkAndStartBattle(asyncCtx, matchID)
	}()

	// Return updated shop state
	return s.shopService.GetShop(ctx, matchID, userID)
}
