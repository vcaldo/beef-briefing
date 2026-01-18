// Package services contains the business logic layer for the api-service.
// This file contains interface definitions for all services, enabling
// dependency injection and testability through mocking.
package services

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
)

// =============================================================================
// ARENA SERVICE INTERFACE
// =============================================================================

// ArenaServiceInterface defines all public methods for the arena game service.
// The arena service handles match lifecycle, shop phase, battles, tournaments,
// and leaderboards.
type ArenaServiceInterface interface {
	// Match Lifecycle Methods
	CreateMatch(ctx context.Context, chatID int64, creatorUserID int64) (*MatchResponse, error)
	GetMatch(ctx context.Context, matchID string) (*MatchResponse, error)
	GetActiveMatches(ctx context.Context, chatID int64) ([]*MatchResponse, error)
	JoinMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error)
	LeaveMatch(ctx context.Context, matchID string, userID int64) error
	StartMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error)

	// Shop Phase Methods
	GetShop(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error)
	BuyCard(ctx context.Context, matchID string, userID int64, cardIndex int) (*EnhancedShopResponse, error)
	Reroll(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error)
	UpgradeCard(ctx context.Context, matchID string, userID int64, teamSlot int, upgradeType shop.UpgradeType) (*EnhancedShopResponse, error)
	SetTeamOrder(ctx context.Context, matchID string, userID int64, order []int) (*EnhancedShopResponse, error)
	SubmitTeam(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error)

	// Battle Methods
	StartBattle(ctx context.Context, matchID string) (*BattleResponse, error)
	GetBattle(ctx context.Context, matchID string, userID int64) (*BattleResponse, error)

	// Automation Methods (for scheduler)
	GetPendingMatches(ctx context.Context) ([]*PendingMatch, error)
	AutoStartMatch(ctx context.Context, matchID string) (*AutoStartResult, error)
	ForceSubmitTeams(ctx context.Context, matchID string) (*ForceSubmitResult, error)

	// Leaderboard Methods
	GetLeaderboard(ctx context.Context, chatID int64, matchType string, limit, offset int) ([]*repository.LeaderboardEntry, error)

	// Tournament Methods
	GetOrCreateTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error)
	GetTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error)
	GetTournamentByID(ctx context.Context, tournamentID int64) (*TournamentResponse, error)
	SetTournamentAnnounced(ctx context.Context, tournamentID int64, messageID int64) error
	JoinTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error)
	LeaveTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error)
	GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error)
	GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error)
	CloseAndStartTournament(ctx context.Context, tournamentID int64) (*TournamentStartResult, error)
	GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error)
	CompleteTournament(ctx context.Context, tournamentID int64, winnerUserID *int64) error
	GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error)

	// Profile and History Methods
	GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error)
	GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error)
	GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error)
	GetProfile(ctx context.Context, chatID, userID int64) (*ArenaProfileResponse, error)

	// Utility Methods
	GetPhotoPresignedURL(ctx context.Context, objectKey string) (string, error)
}

// =============================================================================
// INGEST SERVICE INTERFACE
// =============================================================================

// IngestServiceInterface defines methods for processing Telegram updates.
// The ingest service handles incoming Telegram updates, processing messages,
// reactions, and media files.
type IngestServiceInterface interface {
	// ProcessUpdate handles a Telegram update within a database transaction.
	// It processes messages, reactions, chat member updates, and associated media.
	ProcessUpdate(ctx context.Context, update *models.Update, files map[string][]byte) error
}

// =============================================================================
// MINI APP SERVICE INTERFACE
// =============================================================================

// MiniAppServiceInterface defines methods for Mini App authentication and analytics.
// The mini app service handles JWT authentication, overview stats, user rankings,
// reactions, replies, profiles, heatmaps, and media statistics.
type MiniAppServiceInterface interface {
	// Authentication Methods
	ValidateInitData(initData string, maxAgeSeconds int64) (*ValidatedInitData, error)
	Authenticate(ctx context.Context, initData string) (*AuthResponse, error)

	// Overview Stats Methods
	GetOverviewStats(ctx context.Context, chatID int64, period string, tz *time.Location) (*repository.OverviewStats, error)
	GetDailyActivity(ctx context.Context, chatID int64, period string, tz *time.Location) ([]repository.DailyActivity, error)

	// User Rankings Methods
	GetUserRankings(ctx context.Context, chatID int64, metric, period string, page, limit int, tz *time.Location) ([]UserRankingWithPhoto, int, error)

	// Reactions Methods
	GetReactionsOverview(ctx context.Context, chatID int64, period string, limit int, tz *time.Location) (*ReactionsOverviewResponse, error)

	// Replies Methods
	GetRepliesOverview(ctx context.Context, chatID int64, period string, limit int, tz *time.Location) (*RepliesOverviewResponse, error)

	// User Profile Methods
	GetUserProfile(ctx context.Context, chatID, userID int64, period string, tz *time.Location) (*ProfileResponse, error)

	// Heatmap Methods
	GetHeatmapData(ctx context.Context, chatID int64, userID *int64, period string, tz *time.Location) (*HeatmapResponse, error)

	// Admin and User Info Methods
	IsAdmin(userID int64) bool
	GetChatUsers(ctx context.Context, chatID int64) ([]ChatUserWithPhoto, error)
	GetUserInfo(ctx context.Context, userID int64) (*ChatUserWithPhoto, error)

	// Media Methods
	GetMediaOverview(ctx context.Context, chatID int64, period string, limit int, tz *time.Location) (*MediaOverviewResponse, error)

	// Timezone Methods
	GetChatTimezone(ctx context.Context, chatID int64) (*ChatTimezoneResponse, error)
	SetChatTimezone(ctx context.Context, chatID int64, timezone string) error
}

// =============================================================================
// CARD SERVICE INTERFACE
// =============================================================================

// CardServiceInterface defines methods for user card operations.
// The card service handles user cards, card history, available weeks,
// card images, and gallery operations.
type CardServiceInterface interface {
	// Card Retrieval Methods
	GetUserCard(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error)
	GetChatCards(ctx context.Context, req GetChatCardsRequest) (*GetChatCardsResponse, error)

	// History Methods
	GetUserHistory(ctx context.Context, userID int64, chatID int64, limit int) (*UserHistoryResponse, error)
	GetAvailableWeeks(ctx context.Context, chatID int64) (*AvailableWeeksResponse, error)

	// Card Image Methods
	GetCardImageURL(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*CardImageURLResponse, error)
	GetCardImageURLString(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error)
	GetPlaceholderPositions(theme string) json.RawMessage

	// Gallery Methods
	GetGalleryWeeks(ctx context.Context, chatID int64) (*GalleryWeeksResponse, error)
	GetGalleryImages(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) (*GalleryImagesResponse, error)
	GetGalleryImageURL(ctx context.Context, imageID int64, expirySeconds int) (*GalleryImageURLResponse, error)
}

// =============================================================================
// ML SERVICE INTERFACE
// =============================================================================

// MLServiceInterface defines methods for ML analytics operations.
// The ML service handles unprocessed messages retrieval, ML results storage,
// and processing statistics.
type MLServiceInterface interface {
	// GetUnprocessedMessages retrieves messages that haven't been processed by ML analyzers.
	GetUnprocessedMessages(ctx context.Context, limit int) (*GetMessagesResponse, error)

	// SaveResults saves ML analysis results for messages.
	SaveResults(ctx context.Context, req *SaveResultsRequest) error

	// GetProcessingStats retrieves statistics about ML processing progress.
	GetProcessingStats(ctx context.Context) (*ProcessingStats, error)
}

// =============================================================================
// SUPPORTING INTERFACES
// =============================================================================

// MinIOClientInterface defines the interface for MinIO storage operations.
// This allows the storage client to be mocked in tests.
type MinIOClientInterface interface {
	GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
	GetPresignedURLSeconds(ctx context.Context, objectKey string, expirySeconds int) (string, error)
	UploadMedia(ctx context.Context, fileID string, data []byte, mimeType, mediaType string) (objectKey, fileHash string, err error)
	GetObject(ctx context.Context, objectKey string) (reader io.ReadCloser, size int64, contentType string, err error)
}

// BattleCard represents a card in battle context (re-export from battle package for interface clarity).
type BattleCard = battle.Card

// ShopCard represents a card in the shop (re-export from battle package for interface clarity).
type ShopCard = battle.ShopCard

// =============================================================================
// PROFILE PHOTO SERVICE INTERFACE
// =============================================================================

// ProfilePhotoServiceInterface defines methods for profile photo operations.
// The profile photo service handles uploading, retrieving, and managing
// user and chat profile photos.
type ProfilePhotoServiceInterface interface {
	// ProcessUserPhotos stores profile photos for a user, replacing any existing photos.
	ProcessUserPhotos(ctx context.Context, userID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error

	// ProcessChatPhotos stores profile photos for a chat, replacing any existing photos.
	ProcessChatPhotos(ctx context.Context, chatID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error

	// GetAllUserIDs returns all user IDs from the database.
	GetAllUserIDs(ctx context.Context) ([]int64, error)

	// GetAllChatIDs returns all chat IDs from the database.
	GetAllChatIDs(ctx context.Context) ([]int64, error)

	// GetUserPhoto retrieves a user's profile photo by size.
	// Size can be "small", "medium", or "large" (default).
	// Returns a pre-signed URL for the photo with 1-hour expiry.
	GetUserPhoto(ctx context.Context, userID int64, size string) (string, error)

	// GetChatPhoto retrieves a chat's profile photo by size.
	// Size can be "small", "medium", or "large" (default).
	// Returns a pre-signed URL for the photo with 1-hour expiry.
	GetChatPhoto(ctx context.Context, chatID int64, size string) (string, error)
}
