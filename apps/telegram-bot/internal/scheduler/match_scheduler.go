package scheduler

import (
	"context"
	"fmt"
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

	// Send notification to group
	if result.Action == "started" {
		s.sendMatchStartedNotification(ctx, result.ChatID, result.MatchID, result.Participants)
	} else if result.Action == "cancelled" {
		// Silent cancel - no notification
		slog.Info("match cancelled silently", "match_id", result.MatchID, "reason", result.Reason)
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

	// Send notification about battle starting
	if result.BattleStarted {
		s.sendBattleStartedNotification(ctx, result.ChatID, result.MatchID, result.ForcedUsers)
	}
}

// sendMatchStartedNotification sends a notification when a match auto-starts
func (s *MatchScheduler) sendMatchStartedNotification(ctx context.Context, chatID int64, matchID string, participantCount int) {
	text := fmt.Sprintf(
		"⚔️ *Arena Match Auto-Started!*\n\n"+
			"The join window has closed.\n"+
			"👥 %d participants are now in the shop phase.\n\n"+
			"⏰ You have 3 minutes to build your team!",
		participantCount,
	)

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	})
	if err != nil {
		slog.Error("failed to send match started notification", "chat_id", chatID, "error", err)
	}

	// Send DM notifications to participants
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
		go s.sendShopPhaseDM(ctx, p.UserID, matchID, p.Name)
	}
}

// sendShopPhaseDM sends a DM to a single participant
func (s *MatchScheduler) sendShopPhaseDM(ctx context.Context, userID int64, matchID string, userName string) {
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

// sendBattleStartedNotification sends a notification when battle phase begins
func (s *MatchScheduler) sendBattleStartedNotification(ctx context.Context, chatID int64, matchID string, forcedUsers []int64) {
	text := "⚔️ *Battle Phase Started!*\n\n"

	if len(forcedUsers) > 0 {
		text += fmt.Sprintf(
			"⏰ Time's up! %d player(s) had their teams auto-assigned.\n\n",
			len(forcedUsers),
		)
	}

	text += "The battles are now being simulated. Results coming soon!"

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	})
	if err != nil {
		slog.Error("failed to send battle started notification", "chat_id", chatID, "error", err)
	}
}
