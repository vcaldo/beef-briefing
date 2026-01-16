// Package testutil provides testing utilities for the api-service.
package testutil

import (
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
)

// SampleUser returns a valid Telegram User for testing.
// The returned user represents a typical Telegram user with reasonable defaults.
func SampleUser() models.User {
	return models.User{
		ID:           123456789,
		IsBot:        false,
		FirstName:    "Test",
		LastName:     "User",
		Username:     "testuser",
		LanguageCode: "en",
		IsPremium:    false,
	}
}

// SampleUserWithID returns a valid Telegram User with a custom ID.
func SampleUserWithID(id int64) models.User {
	return models.User{
		ID:           id,
		IsBot:        false,
		FirstName:    "Test",
		LastName:     "User",
		Username:     "testuser",
		LanguageCode: "en",
		IsPremium:    false,
	}
}

// SampleBotUser returns a valid Telegram bot User for testing.
func SampleBotUser() models.User {
	return models.User{
		ID:        987654321,
		IsBot:     true,
		FirstName: "TestBot",
		Username:  "test_bot",
	}
}

// SampleChat returns a valid Telegram Chat for testing.
// The returned chat represents a supergroup, which is the most common chat type
// for the application.
func SampleChat() models.Chat {
	return models.Chat{
		ID:       -1001234567890,
		Type:     "supergroup",
		Title:    "Test Group",
		Username: "testgroup",
		IsForum:  false,
	}
}

// SampleChatWithID returns a valid Telegram Chat with a custom ID.
func SampleChatWithID(id int64) models.Chat {
	return models.Chat{
		ID:       id,
		Type:     "supergroup",
		Title:    "Test Group",
		Username: "testgroup",
		IsForum:  false,
	}
}

// SamplePrivateChat returns a valid Telegram private chat for testing.
func SamplePrivateChat() models.Chat {
	return models.Chat{
		ID:        123456789,
		Type:      "private",
		FirstName: "Test",
		LastName:  "User",
		Username:  "testuser",
	}
}

// SampleMessage returns a valid Telegram Message for testing.
// The message includes a text message from the sample user in the sample chat.
func SampleMessage() models.Message {
	user := SampleUser()
	chat := SampleChat()
	return models.Message{
		MessageID: 42,
		From:      &user,
		Chat:      chat,
		Date:      time.Now().Unix(),
		Text:      "Hello, this is a test message!",
	}
}

// SampleMessageWithID returns a valid Telegram Message with a custom message ID.
func SampleMessageWithID(messageID int64) models.Message {
	user := SampleUser()
	chat := SampleChat()
	return models.Message{
		MessageID: messageID,
		From:      &user,
		Chat:      chat,
		Date:      time.Now().Unix(),
		Text:      "Hello, this is a test message!",
	}
}

// SampleMessageWithText returns a valid Telegram Message with custom text.
func SampleMessageWithText(text string) models.Message {
	user := SampleUser()
	chat := SampleChat()
	return models.Message{
		MessageID: 42,
		From:      &user,
		Chat:      chat,
		Date:      time.Now().Unix(),
		Text:      text,
	}
}

// SampleTelegramUpdate returns a valid Telegram Update for testing.
// The update contains a standard text message.
func SampleTelegramUpdate() models.Update {
	msg := SampleMessage()
	return models.Update{
		UpdateID: 100000001,
		Message:  &msg,
	}
}

// SampleTelegramUpdateWithID returns a valid Telegram Update with a custom update ID.
func SampleTelegramUpdateWithID(updateID int64) models.Update {
	msg := SampleMessage()
	return models.Update{
		UpdateID: updateID,
		Message:  &msg,
	}
}

// SampleEditedMessageUpdate returns a valid Telegram Update with an edited message.
func SampleEditedMessageUpdate() models.Update {
	msg := SampleMessage()
	editDate := time.Now().Unix()
	msg.EditDate = &editDate
	msg.Text = "This message has been edited!"
	return models.Update{
		UpdateID:      100000002,
		EditedMessage: &msg,
	}
}

// SampleReactionUpdate returns a valid Telegram Update with a message reaction.
func SampleReactionUpdate() models.Update {
	user := SampleUser()
	chat := SampleChat()
	return models.Update{
		UpdateID: 100000003,
		MessageReaction: &models.MessageReactionUpdated{
			Chat:      chat,
			MessageID: 42,
			User:      &user,
			Date:      time.Now().Unix(),
			OldReaction: []models.ReactionInfo{},
			NewReaction: []models.ReactionInfo{
				{
					Type:  "emoji",
					Emoji: "👍",
				},
			},
		},
	}
}

// SampleReactionCountUpdate returns a valid Telegram Update with reaction counts.
func SampleReactionCountUpdate() models.Update {
	chat := SampleChat()
	return models.Update{
		UpdateID: 100000004,
		MessageReactionCount: &models.MessageReactionCountUpdate{
			Chat:      chat,
			MessageID: 42,
			Date:      time.Now().Unix(),
			Reactions: []models.ReactionCount{
				{
					Type: models.ReactionInfo{
						Type:  "emoji",
						Emoji: "👍",
					},
					TotalCount: 5,
				},
			},
		},
	}
}

// SamplePhotoMessage returns a valid Telegram Message with a photo attachment.
func SamplePhotoMessage() models.Message {
	user := SampleUser()
	chat := SampleChat()
	fileSize := int64(12345)
	return models.Message{
		MessageID: 43,
		From:      &user,
		Chat:      chat,
		Date:      time.Now().Unix(),
		Caption:   "Check out this photo!",
		Photo: []models.PhotoSize{
			{
				FileID:       "AgACAgIAAxkBAAI-small",
				FileUniqueID: "AQADAgAT-small",
				Width:        90,
				Height:       90,
				FileSize:     &fileSize,
			},
			{
				FileID:       "AgACAgIAAxkBAAI-large",
				FileUniqueID: "AQADAgAT-large",
				Width:        800,
				Height:       600,
				FileSize:     &fileSize,
			},
		},
	}
}

// SampleMatch returns a valid arena Match for testing.
// The match is in "open" status, ready for participants to join.
func SampleMatch() repository.Match {
	creatorID := int64(123456789)
	joinDeadline := time.Now().Add(5 * time.Minute)
	format := repository.MatchFormat1v1
	return repository.Match{
		ID:           "test-match-001",
		ChatID:       -1001234567890,
		MatchType:    repository.MatchTypeRegular,
		Format:       &format,
		Status:       repository.MatchStatusOpen,
		CreatedAt:    time.Now(),
		JoinDeadline: &joinDeadline,
		CreatorUserID: &creatorID,
		CurrentRound: 0,
	}
}

// SampleMatchWithID returns a valid arena Match with a custom ID.
func SampleMatchWithID(id string) repository.Match {
	match := SampleMatch()
	match.ID = id
	return match
}

// SampleMatchWithStatus returns a valid arena Match with a custom status.
func SampleMatchWithStatus(status repository.MatchStatus) repository.Match {
	match := SampleMatch()
	match.Status = status
	return match
}

// SampleRankedMatch returns a valid ranked arena Match for testing.
func SampleRankedMatch() repository.Match {
	creatorID := int64(123456789)
	joinDeadline := time.Now().Add(5 * time.Minute)
	format := repository.MatchFormat1v1
	tournamentDate := time.Now().Format("2006-01-02")
	return repository.Match{
		ID:             "test-ranked-match-001",
		ChatID:         -1001234567890,
		MatchType:      repository.MatchTypeRanked,
		Format:         &format,
		Status:         repository.MatchStatusOpen,
		CreatedAt:      time.Now(),
		JoinDeadline:   &joinDeadline,
		TournamentDate: &tournamentDate,
		CreatorUserID:  &creatorID,
		CurrentRound:   0,
	}
}

// SampleShopPhaseMatch returns a valid arena Match in shop phase.
func SampleShopPhaseMatch() repository.Match {
	match := SampleMatch()
	match.Status = repository.MatchStatusShopPhase
	shopStart := time.Now()
	shopDeadline := time.Now().Add(2 * time.Minute)
	match.ShopPhaseStartedAt = &shopStart
	match.ShopPhaseDeadline = &shopDeadline
	return match
}

// SampleBattlePhaseMatch returns a valid arena Match in battle phase.
func SampleBattlePhaseMatch() repository.Match {
	match := SampleMatch()
	match.Status = repository.MatchStatusBattlePhase
	shopStart := time.Now().Add(-3 * time.Minute)
	battleStart := time.Now()
	match.ShopPhaseStartedAt = &shopStart
	match.BattleStartedAt = &battleStart
	match.CurrentRound = 1
	return match
}

// SampleCompletedMatch returns a valid completed arena Match.
func SampleCompletedMatch() repository.Match {
	match := SampleMatch()
	match.Status = repository.MatchStatusCompleted
	shopStart := time.Now().Add(-10 * time.Minute)
	battleStart := time.Now().Add(-5 * time.Minute)
	completedAt := time.Now()
	winnerID := int64(123456789)
	match.ShopPhaseStartedAt = &shopStart
	match.BattleStartedAt = &battleStart
	match.CompletedAt = &completedAt
	match.WinnerUserID = &winnerID
	match.CurrentRound = 3
	return match
}

// SampleParticipant returns a valid match Participant for testing.
func SampleParticipant(matchID string, userID int64) repository.Participant {
	return repository.Participant{
		ID:             1,
		MatchID:        matchID,
		UserID:         userID,
		Status:         repository.ParticipantStatusJoined,
		JoinedAt:       time.Now(),
		CoinsRemaining: 10,
		Wins:           0,
		Losses:         0,
		TotalDamageDealt: 0,
	}
}

// SampleParticipantWithTeam returns a participant who has submitted their team.
func SampleParticipantWithTeam(matchID string, userID int64) repository.Participant {
	p := SampleParticipant(matchID, userID)
	p.Status = repository.ParticipantStatusReady
	submittedAt := time.Now()
	p.TeamSubmittedAt = &submittedAt
	p.CoinsRemaining = 1
	return p
}
