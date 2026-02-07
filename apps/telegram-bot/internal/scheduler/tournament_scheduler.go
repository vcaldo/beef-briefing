package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	// TournamentPollInterval is how often to check for tournament actions
	TournamentPollInterval = 1 * time.Minute

	// DefaultTimezone is the fallback timezone (UTC-3 São Paulo)
	DefaultTimezone = "America/Sao_Paulo"
)

// TournamentScheduler handles background tournament processing
type TournamentScheduler struct {
	apiClient     *client.APIClient
	bot           *bot.Bot
	nrApp         *newrelic.Application
	rankedEnabled bool
	betaGroups    map[int64]bool
}

// NewTournamentScheduler creates a new tournament scheduler
func NewTournamentScheduler(apiClient *client.APIClient, b *bot.Bot, nrApp *newrelic.Application, rankedEnabled bool, betaGroups map[int64]bool) *TournamentScheduler {
	return &TournamentScheduler{
		apiClient:     apiClient,
		bot:           b,
		nrApp:         nrApp,
		rankedEnabled: rankedEnabled,
		betaGroups:    betaGroups,
	}
}

// Start begins the scheduler loop
func (s *TournamentScheduler) Start(ctx context.Context) {
	slog.Info("starting tournament scheduler", "poll_interval", TournamentPollInterval)

	ticker := time.NewTicker(TournamentPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("tournament scheduler stopped")
			return
		case <-ticker.C:
			s.processTournaments(ctx)
		}
	}
}

// processTournaments checks for and processes tournament actions
func (s *TournamentScheduler) processTournaments(ctx context.Context) {
	// Check if ranked tournaments are globally enabled
	if !s.rankedEnabled {
		slog.Debug("ranked tournaments globally disabled, skipping processing")
		return
	}

	// Start New Relic transaction if available
	var txn *newrelic.Transaction
	if s.nrApp != nil {
		txn = s.nrApp.StartTransaction("scheduler:process-tournaments")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
	}

	// Process in order of priority:
	// 1. Announcements (00:01 local time)
	// 2. Registration close (18:00 local time)
	// 3. Pending rounds (5-min intervals)

	s.processAnnouncements(ctx, txn)
	s.processRegistrationClose(ctx, txn)
	s.processPendingRounds(ctx, txn)
}

// processAnnouncements handles tournaments needing 00:01 announcement
func (s *TournamentScheduler) processAnnouncements(ctx context.Context, txn *newrelic.Transaction) {
	tournaments, err := s.apiClient.GetPendingAnnouncements(ctx)
	if err != nil {
		slog.Error("failed to get pending announcements", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	if len(tournaments) == 0 {
		return
	}

	slog.Info("found tournaments needing announcement", "count", len(tournaments))
	if txn != nil {
		txn.AddAttribute("announcement_count", len(tournaments))
	}

	for _, t := range tournaments {
		if !s.betaGroups[t.ChatID] {
			slog.Debug("skipping tournament announcement for non-beta group", "chat_id", t.ChatID)
			continue
		}
		s.sendTournamentAnnouncement(ctx, t, txn)
	}
}

// sendTournamentAnnouncement sends the tournament open announcement
func (s *TournamentScheduler) sendTournamentAnnouncement(ctx context.Context, t client.TournamentInfo, txn *newrelic.Transaction) {
	slog.Info("announcing tournament", "chat_id", t.ChatID, "date", t.TournamentDate)

	// Load timezone for display
	loc, err := time.LoadLocation(t.Timezone)
	if err != nil {
		loc, _ = time.LoadLocation(DefaultTimezone)
	}

	// Calculate close time in local timezone
	// First convert current time to local timezone, then extract date components
	now := time.Now().In(loc)
	closeTime := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, loc)

	text := fmt.Sprintf(
		"🏆 *Ranked Tournament Open!*\n\n"+
			"Today's arena battle is now accepting participants.\n\n"+
			"⏰ Registration closes at %s\n"+
			"🎮 Format: Auto-selected based on participants\n"+
			"🏅 Results count towards ranked leaderboard\n\n"+
			"Use /ranked to join!",
		closeTime.Format("15:04"),
	)

	// Create inline keyboard with join button
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "Join Tournament",
					CallbackData: fmt.Sprintf("ranked_join:%d", t.TournamentID),
				},
			},
		},
	}

	msg, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      t.ChatID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to send tournament announcement", "chat_id", t.ChatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	// Register announcement with API
	_, err = s.apiClient.AnnounceTournament(ctx, t.ChatID, t.TournamentDate, int64(msg.ID))
	if err != nil {
		slog.Error("failed to register tournament announcement", "chat_id", t.ChatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
	}

	slog.Info("tournament announced", "chat_id", t.ChatID, "message_id", msg.ID)
}

// processRegistrationClose handles tournaments at 18:00 close time
func (s *TournamentScheduler) processRegistrationClose(ctx context.Context, txn *newrelic.Transaction) {
	tournaments, err := s.apiClient.GetPendingCloseTournaments(ctx)
	if err != nil {
		slog.Error("failed to get pending close tournaments", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	if len(tournaments) == 0 {
		return
	}

	slog.Info("found tournaments needing close", "count", len(tournaments))
	if txn != nil {
		txn.AddAttribute("close_count", len(tournaments))
	}

	for _, t := range tournaments {
		if !s.betaGroups[t.ChatID] {
			slog.Debug("skipping tournament close for non-beta group", "chat_id", t.ChatID)
			continue
		}
		s.closeTournament(ctx, t, txn)
	}
}

// closeTournament closes registration and starts a tournament
func (s *TournamentScheduler) closeTournament(ctx context.Context, t client.TournamentInfo, txn *newrelic.Transaction) {
	slog.Info("closing tournament", "tournament_id", t.TournamentID, "chat_id", t.ChatID, "participants", t.ParticipantCount)

	result, err := s.apiClient.CloseTournament(ctx, t.TournamentID)
	if err != nil {
		slog.Error("failed to close tournament", "tournament_id", t.TournamentID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	if result.Skipped {
		// Tournament skipped - don't send any message (silent skip)
		slog.Info("tournament skipped", "tournament_id", t.TournamentID, "reason", result.Reason)
		return
	}

	// Send notification that tournament is starting
	text := fmt.Sprintf(
		"⚔️ *Tournament Starting!*\n\n"+
			"Registration is now closed.\n"+
			"👥 %d participants registered\n\n"+
			"⏰ Build your team - 3 minutes!\n\n"+
			"Format: %s",
		result.ParticipantCount,
		formatMatchFormat(result.Format),
	)

	_, err = s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    t.ChatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		slog.Error("failed to send tournament start notification", "chat_id", t.ChatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
	}

	// Send DM notifications to participants
	if result.Match != nil && result.Match.ID != "" {
		s.sendTournamentShopPhaseDMs(ctx, result.Match.ID)
	}

	slog.Info("tournament started", "tournament_id", t.TournamentID, "participants", result.ParticipantCount, "format", result.Format)
}

// sendTournamentShopPhaseDMs sends DM notifications to tournament participants
func (s *TournamentScheduler) sendTournamentShopPhaseDMs(ctx context.Context, matchID string) {
	// Get match details to get participant user IDs
	match, err := s.apiClient.GetArenaMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get tournament match for DM notifications", "match_id", matchID, "error", err)
		return
	}

	for _, p := range match.Participants {
		go s.sendTournamentDM(ctx, p.UserID, matchID)
	}
}

// sendTournamentDM sends a DM to a single tournament participant
func (s *TournamentScheduler) sendTournamentDM(ctx context.Context, userID int64, matchID string) {
	text := "🏆 *Ranked Tournament Starting!*\n\n" +
		"Build your team now - you have 3 minutes.\n\n" +
		"Open the Arena to pick your cards!"

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})

	if err != nil {
		// This is expected if the user hasn't started a chat with the bot
		slog.Debug("failed to send tournament shop phase DM", "user_id", userID, "error", err)
	} else {
		slog.Info("sent tournament shop phase DM", "user_id", userID, "match_id", matchID)
	}
}

// processPendingRounds handles tournaments with pending battle rounds
func (s *TournamentScheduler) processPendingRounds(ctx context.Context, txn *newrelic.Transaction) {
	tournaments, err := s.apiClient.GetPendingRoundsTournaments(ctx)
	if err != nil {
		slog.Error("failed to get pending rounds tournaments", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	if len(tournaments) == 0 {
		return
	}

	slog.Debug("found tournaments with pending rounds", "count", len(tournaments))
	if txn != nil {
		txn.AddAttribute("pending_rounds_count", len(tournaments))
	}

	// TODO: Implement round execution logic
	// This will require:
	// 1. Check if enough time has passed since last round (5 min)
	// 2. Execute next round via API
	// 3. Send round results notification
	// 4. Check if tournament is complete
	// 5. Send tournament complete notification
}

// formatMatchFormat returns a human-readable format string
func formatMatchFormat(format string) string {
	switch format {
	case "1v1":
		return "1v1 Battle"
	case "arena":
		return "Arena (Multi-player)"
	default:
		return format
	}
}
