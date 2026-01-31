package scheduler

import (
	"context"
	"log/slog"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	// PollInterval is how often to check for pending matches
	PollInterval = 30 * time.Second
)

// MatchScheduler handles background match processing
type MatchScheduler struct {
	apiClient *client.APIClient
	bot       *bot.Bot
	nrApp     *newrelic.Application
}

// NewMatchScheduler creates a new match scheduler
func NewMatchScheduler(apiClient *client.APIClient, b *bot.Bot, nrApp *newrelic.Application) *MatchScheduler {
	return &MatchScheduler{
		apiClient: apiClient,
		bot:       b,
		nrApp:     nrApp,
	}
}

// Start begins the scheduler loop
func (s *MatchScheduler) Start(ctx context.Context) {
	slog.Info("starting match scheduler", "poll_interval", PollInterval)

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("match scheduler stopped")
			return
		case <-ticker.C:
			s.processPendingMatches(ctx)
		}
	}
}

// processPendingMatches checks for and processes matches with expired deadlines
func (s *MatchScheduler) processPendingMatches(ctx context.Context) {
	// Start New Relic transaction if available
	var txn *newrelic.Transaction
	if s.nrApp != nil {
		txn = s.nrApp.StartTransaction("scheduler:process-pending-matches")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
	}

	// Get pending matches from API
	pendingMatches, err := s.apiClient.GetPendingMatches(ctx)
	if err != nil {
		slog.Error("failed to get pending matches", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	if len(pendingMatches) == 0 {
		return
	}

	slog.Debug("found pending matches", "count", len(pendingMatches))
	if txn != nil {
		txn.AddAttribute("pending_count", len(pendingMatches))
	}

	for _, match := range pendingMatches {
		switch match.Action {
		case "auto_start":
			s.handleAutoStart(ctx, match, txn)
		case "force_submit":
			s.handleForceSubmit(ctx, match, txn)
		default:
			slog.Warn("unknown pending action", "action", match.Action, "match_id", match.ID)
		}
	}
}

// handleAutoStart processes a match with expired join deadline
func (s *MatchScheduler) handleAutoStart(ctx context.Context, match client.PendingMatch, txn *newrelic.Transaction) {
	slog.Info("auto-starting match", "match_id", match.ID, "chat_id", match.ChatID, "participants", match.ParticipantCount)

	result, err := s.apiClient.AutoStartMatch(ctx, match.ID)
	if err != nil {
		slog.Error("failed to auto-start match", "match_id", match.ID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Info("auto-start result", "match_id", result.MatchID, "action", result.Action, "reason", result.Reason)

	// Send DM notifications to participants
	if result.Action == "started" {
		s.sendMatchStartedNotification(ctx, result.MatchID)
	} else if result.Action == "cancelled" {
		slog.Info("match cancelled", "match_id", result.MatchID, "reason", result.Reason)
		s.handleMatchCancelled(ctx, result)
	}
}

// handleForceSubmit processes a match with expired shop phase deadline
func (s *MatchScheduler) handleForceSubmit(ctx context.Context, match client.PendingMatch, txn *newrelic.Transaction) {
	slog.Info("force-submitting teams", "match_id", match.ID, "chat_id", match.ChatID)

	result, err := s.apiClient.ForceSubmitTeams(ctx, match.ID)
	if err != nil {
		slog.Error("failed to force-submit teams", "match_id", match.ID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Info("force-submit result",
		"match_id", result.MatchID,
		"forced_users", len(result.ForcedUsers),
		"battle_started", result.BattleStarted,
	)

	// Send DM notifications to forced users
	if result.BattleStarted {
		s.sendBattleStartedNotification(ctx, result.MatchID, result.ForcedUsers)
	}
}

// sendMatchStartedNotification sends DM notifications when a match auto-starts
func (s *MatchScheduler) sendMatchStartedNotification(ctx context.Context, matchID string) {
	// Send DM notifications to participants - no group message
	s.sendShopPhaseDMs(ctx, matchID)
}

// sendShopPhaseDMs sends DM notifications to all participants when shop phase starts
func (s *MatchScheduler) sendShopPhaseDMs(ctx context.Context, matchID string) {
	// Get match details to get participant user IDs
	match, err := s.apiClient.GetArenaMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match for DM notifications", "match_id", matchID, "error", err)
		return
	}

	for _, p := range match.Participants {
		go s.sendShopPhaseDM(ctx, p.UserID, matchID)
	}
}

// sendShopPhaseDM sends a DM to a single participant
func (s *MatchScheduler) sendShopPhaseDM(ctx context.Context, userID int64, matchID string) {
	text := "🎮 *Your match is starting!*\n\n" +
		"Build your team now - you have 3 minutes.\n\n" +
		"Open the Arena to pick your cards!"

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      text,
		ParseMode: "Markdown",
	})

	if err != nil {
		// This is expected if the user hasn't started a chat with the bot
		slog.Debug("failed to send shop phase DM", "user_id", userID, "error", err)
	} else {
		slog.Info("sent shop phase DM", "user_id", userID, "match_id", matchID)
	}
}

// sendBattleStartedNotification sends DM notifications to forced users when battle phase begins
func (s *MatchScheduler) sendBattleStartedNotification(ctx context.Context, matchID string, forcedUsers []int64) {
	if len(forcedUsers) == 0 {
		slog.Debug("no forced users, skipping battle started DMs", "match_id", matchID)
		return
	}
	s.sendBattleAutoAssignedDMs(ctx, matchID, forcedUsers)
}

// sendBattleAutoAssignedDMs sends DM notifications to all forced users
func (s *MatchScheduler) sendBattleAutoAssignedDMs(ctx context.Context, matchID string, forcedUsers []int64) {
	for _, userID := range forcedUsers {
		go s.sendBattleAutoAssignedDM(ctx, userID, matchID)
	}
}

// sendBattleAutoAssignedDM sends a DM to a single forced user
func (s *MatchScheduler) sendBattleAutoAssignedDM(ctx context.Context, userID int64, matchID string) {
	text := "⏰ *Time's up!*\n\n" +
		"Your team was auto-assigned because you didn't submit in time.\n\n" +
		"The battle is now being simulated. Results coming soon!"

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      text,
		ParseMode: "Markdown",
	})

	if err != nil {
		slog.Debug("failed to send battle auto-assigned DM", "user_id", userID, "error", err)
	} else {
		slog.Info("sent battle auto-assigned DM", "user_id", userID, "match_id", matchID)
	}
}

// handleMatchCancelled handles a cancelled match: deletes the group message and sends DM to creator
func (s *MatchScheduler) handleMatchCancelled(ctx context.Context, result *client.AutoStartResult) {
	// Delete the match message from the group
	if result.TelegramMessageID != nil && *result.TelegramMessageID != 0 {
		_, err := s.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    result.ChatID,
			MessageID: int(*result.TelegramMessageID),
		})
		if err != nil {
			slog.Debug("failed to delete match message", "chat_id", result.ChatID, "message_id", *result.TelegramMessageID, "error", err)
		} else {
			slog.Info("deleted match message", "chat_id", result.ChatID, "message_id", *result.TelegramMessageID, "match_id", result.MatchID)
		}
	}

	// Send DM to the match creator
	if result.CreatorUserID == nil {
		slog.Debug("no creator user ID, skipping match cancelled DM", "match_id", result.MatchID)
		return
	}

	text := "😕 *Match Cancelled*\n\n" +
		"Your arena match was cancelled because not enough players joined.\n\n" +
		"Try starting another match when more people are around!"

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    *result.CreatorUserID,
		Text:      text,
		ParseMode: "Markdown",
	})

	if err != nil {
		slog.Debug("failed to send match cancelled DM", "user_id", *result.CreatorUserID, "error", err)
	} else {
		slog.Info("sent match cancelled DM", "user_id", *result.CreatorUserID, "match_id", result.MatchID)
	}
}
