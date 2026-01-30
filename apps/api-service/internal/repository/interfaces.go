// Package repository defines database operations for the API service.
// This file contains interface definitions for all repositories, enabling
// dependency injection and testability through mocking.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

// DBTX is an interface that accepts both *sql.DB and *sql.Tx,
// allowing repositories to work with either direct database connections
// or within transactions.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// UpdateRepositoryInterface defines methods for update operations.
type UpdateRepositoryInterface interface {
	InsertUpdate(ctx context.Context, tx *sql.Tx, updateID int64, updateType string, rawData interface{}) error
}

// MessageRepositoryInterface defines methods for message operations.
type MessageRepositoryInterface interface {
	InsertMessage(ctx context.Context, tx *sql.Tx, msg *models.Message) (int64, error)
	InsertMessageEdit(ctx context.Context, tx *sql.Tx, messageID int64, editDate time.Time, previousText, newText, previousCaption, newCaption string) error
	InsertEntity(ctx context.Context, tx *sql.Tx, messageID int64, entity *models.MessageEntity) error
	GetMessageByID(ctx context.Context, tx *sql.Tx, chatID, messageID int64) (*models.DBMessage, error)
}

// ChatRepositoryInterface defines methods for chat operations.
type ChatRepositoryInterface interface {
	UpsertChat(ctx context.Context, tx *sql.Tx, chat *models.Chat) error
	LinkMigratedChat(ctx context.Context, tx *sql.Tx, newChatID, oldChatID int64) error
}

// UserRepositoryInterface defines methods for user operations.
type UserRepositoryInterface interface {
	UpsertUser(ctx context.Context, tx *sql.Tx, user *models.User) error
}

// ReactionRepositoryInterface defines methods for reaction operations.
type ReactionRepositoryInterface interface {
	InsertMessageReaction(ctx context.Context, tx *sql.Tx, reaction *models.MessageReactionUpdated) error
	UpsertReactionCount(ctx context.Context, tx *sql.Tx, countUpdate *models.MessageReactionCountUpdate) error
}

// MediaRepositoryInterface defines methods for media operations.
type MediaRepositoryInterface interface {
	GetObjectKeyByHash(ctx context.Context, tx *sql.Tx, fileHash string) (string, error)
	InsertMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) error
	InsertMediaFileReturningID(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) (int64, error)
	InsertPhoto(ctx context.Context, tx *sql.Tx, messageID int64, photo *models.PhotoSize, objectKey, fileHash string) error
	InsertLocation(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) error
	InsertLocationReturningID(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) (int64, error)
	InsertSticker(ctx context.Context, tx *sql.Tx, messageID, mediaFileID int64, sticker *models.Sticker) error
	InsertGame(ctx context.Context, tx *sql.Tx, messageID int64, game *models.Game) (int64, error)
	InsertGamePhoto(ctx context.Context, tx *sql.Tx, gameID int64, photo *models.PhotoSize, objectKey, fileHash string) error
	InsertPoll(ctx context.Context, tx *sql.Tx, messageID int64, poll *models.Poll) (int64, error)
	InsertPollOption(ctx context.Context, tx *sql.Tx, pollID int64, index int, option *models.PollOption) error
	InsertContact(ctx context.Context, tx *sql.Tx, messageID int64, contact *models.Contact) error
	InsertVenue(ctx context.Context, tx *sql.Tx, messageID, locationID int64, venue *models.Venue) error
	InsertDice(ctx context.Context, tx *sql.Tx, messageID int64, dice *models.Dice) error
}

// ProfilePhotoRepositoryInterface defines methods for profile photo operations.
type ProfilePhotoRepositoryInterface interface {
	ReplaceUserPhotos(ctx context.Context, tx *sql.Tx, userID int64, photos []models.DBUserProfilePhoto) error
	ReplaceChatPhotos(ctx context.Context, tx *sql.Tx, chatID int64, photos []models.DBChatProfilePhoto) error
	GetUserPhotos(ctx context.Context, userID int64) ([]models.DBUserProfilePhoto, error)
	GetChatPhotos(ctx context.Context, chatID int64) ([]models.DBChatProfilePhoto, error)
	GetAllUserIDs(ctx context.Context) ([]int64, error)
	GetAllChatIDs(ctx context.Context) ([]int64, error)
	GetUserPhotoBySize(ctx context.Context, userID int64, size string) (*models.DBUserProfilePhoto, error)
	GetChatPhotoBySize(ctx context.Context, chatID int64, size string) (*models.DBChatProfilePhoto, error)
}

// CardRepositoryInterface defines methods for user card operations.
type CardRepositoryInterface interface {
	GetUserCard(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*UserCard, error)
	GetUserInfo(ctx context.Context, userID int64) (*CardUser, error)
	GetChatCards(ctx context.Context, q ChatCardsQuery) ([]CardWithUser, int, error)
	GetUserHistory(ctx context.Context, userID int64, chatID int64, limit int) ([]UserCard, error)
	GetAvailableWeeks(ctx context.Context, chatID int64) ([]WeekInfo, error)
	GetCardImage(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string) (*CardImage, error)
	GetGalleryWeeks(ctx context.Context, chatID int64) ([]string, error)
	GetGalleryImages(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) ([]GalleryImage, error)
	GetGalleryImageByID(ctx context.Context, imageID int64) (*GalleryImage, error)
}

// MLRepositoryInterface defines methods for ML analytics operations.
// Methods that perform batch writes accept an optional DBTX parameter for transaction management:
// - If dbtx is nil/omitted, a new transaction is created and committed internally
// - If dbtx is provided, the caller is responsible for transaction management
type MLRepositoryInterface interface {
	GetUnprocessedMessages(ctx context.Context, limit int) ([]UnprocessedMessage, error)
	GetProcessingStats(ctx context.Context) (map[string]int64, error)
	SaveSentimentResults(ctx context.Context, results []SentimentResult, dbtx ...DBTX) error
	SaveToxicityResults(ctx context.Context, results []ToxicityResult, dbtx ...DBTX) error
	SaveHumorResults(ctx context.Context, results []HumorResult, dbtx ...DBTX) error
	SaveQuestionResults(ctx context.Context, results []QuestionResult, dbtx ...DBTX) error
	SaveNERResults(ctx context.Context, results []NERResult, dbtx ...DBTX) error
	SaveTopics(ctx context.Context, chatID int64, topics map[int][]string, dbtx ...DBTX) error
	SaveMessageTopics(ctx context.Context, results []MessageTopicResult, dbtx ...DBTX) error
	MarkMessagesProcessed(ctx context.Context, messageIDs []int64, chatIDs []int64, version string, dbtx ...DBTX) error
}

// GameRepositoryInterface defines methods for arena game operations.
// Methods are grouped by domain: match operations, participant operations,
// tournament operations, and leaderboard/stats operations.
type GameRepositoryInterface interface {
	// Match operations
	CreateMatch(ctx context.Context, chatID int64, matchType MatchType, creatorUserID *int64, tournamentDate *string) (*Match, error)
	GetMatch(ctx context.Context, matchID string) (*Match, error)
	GetActiveMatches(ctx context.Context, chatID int64) ([]*Match, error)
	GetUserActiveMatch(ctx context.Context, chatID, userID int64) (*Match, error)
	GetChatOpenMatch(ctx context.Context, chatID int64) (*Match, error)
	GetMatchesByStatus(ctx context.Context, status MatchStatus) ([]*Match, error)
	UpdateMatchStatus(ctx context.Context, matchID string, status MatchStatus) error
	UpdateMatchFormat(ctx context.Context, matchID string, format MatchFormat) error
	StartShopPhase(ctx context.Context, matchID string, format MatchFormat, deadline time.Time) error
	StartBattlePhase(ctx context.Context, matchID string) error
	CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error
	CancelMatch(ctx context.Context, matchID string) error

	// Participant operations
	AddParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error)
	GetParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error)
	GetMatchParticipants(ctx context.Context, matchID string) ([]*ParticipantWithUser, error)
	RemoveParticipant(ctx context.Context, matchID string, userID int64) error
	UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error
	SubmitTeam(ctx context.Context, matchID string, userID int64) error
	SetParticipantRerolled(ctx context.Context, matchID string, userID int64) error
	GetParticipantCount(ctx context.Context, matchID string) (int, error)
	GetReadyParticipantCount(ctx context.Context, matchID string) (int, error)

	// Round operations
	CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*MatchRound, error)
	GetMatchRounds(ctx context.Context, matchID string) ([]*MatchRound, error)

	// Tournament operations
	GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error)
	GetTournamentByID(ctx context.Context, id int64) (*RankedTournament, error)
	GetTodayTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error)
	GetTournamentsByStatus(ctx context.Context, status TournamentStatus) ([]*RankedTournament, error)
	GetTournamentsWithPendingRounds(ctx context.Context) ([]*RankedTournament, error)
	UpdateTournamentStatus(ctx context.Context, id int64, status TournamentStatus) error
	SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error
	CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error
	CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error
	SkipTournament(ctx context.Context, id int64) error
	UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error

	// Tournament participant operations
	AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error
	RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error
	GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*TournamentParticipant, error)
	IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error)

	// Tournament scheduler operations
	GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error)
	GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error)
	GetChatsWithTimezone(ctx context.Context) ([]*ChatTimezone, error)

	// Leaderboard and stats operations
	GetLeaderboard(ctx context.Context, chatID int64, matchType MatchType, limit, offset int) ([]*LeaderboardEntry, int, error)
	UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType MatchType, isWin bool, opponentID *int64, isTournamentWin bool, isDraw bool) error
	GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*MatchHistoryEntry, int, error)
	GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*H2HRecord, error)
	GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*MatchHistoryEntry, error)
	GetUserProfile(ctx context.Context, chatID, userID int64) (*UserProfile, error)
}

// MiniAppRepositoryInterface defines methods for Mini App analytics operations.
type MiniAppRepositoryInterface interface {
	// Overview and activity
	GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*OverviewStats, error)
	GetDailyActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error)
	GetUserDailyActivity(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error)

	// User rankings
	GetUserRankings(ctx context.Context, chatID int64, metric string, limit, offset int, startDate, endDate *time.Time, tzName string) ([]UserRanking, error)
	GetUserRankingsTotal(ctx context.Context, chatID int64, startDate, endDate *time.Time) (int, error)

	// Reactions
	GetTopReactions(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]TopReaction, error)
	GetTopReactionGivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error)
	GetTopReactionReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error)

	// User profile and interactions
	GetUserProfileStats(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tzName string) (*ProfileStats, error)
	GetTopReactorsToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error)
	GetTopReactedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error)
	GetTopRepliersToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error)
	GetTopRepliedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error)

	// Reply rankings
	GetTopReplySenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error)
	GetTopReplyReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error)

	// Heatmaps
	GetGroupHeatmap(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error)
	GetUserHeatmap(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error)

	// Chat and user info
	GetUserPhotoObjectKey(ctx context.Context, userID int64) (*string, error)
	GetChatTitle(ctx context.Context, chatID int64) (*string, error)
	GetChatUsers(ctx context.Context, chatID int64) ([]ChatUser, error)
	GetUserInfo(ctx context.Context, userID int64) (*ChatUser, error)

	// Media stats
	GetMediaOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*MediaOverviewStats, error)
	GetMediaTypeDistribution(ctx context.Context, chatID int64, startDate, endDate *time.Time) ([]MediaTypeDistribution, error)
	GetMediaActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]MediaActivity, error)
	GetTopMediaSenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]MediaUser, error)

	// Settings
	GetChatTimezone(ctx context.Context, chatID int64) (*string, error)
	SetChatTimezone(ctx context.Context, chatID int64, timezone string) error
}
