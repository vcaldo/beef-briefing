// Package apperror provides centralized error definitions for the api-service.
// All sentinel errors should be defined here to maintain consistency across
// handlers and services, and to enable proper error wrapping with %w.
package apperror

import "errors"

// =====================================================
// ARENA SERVICE ERRORS
// =====================================================

// Match lifecycle errors
var (
	ErrMatchNotFound     = errors.New("match not found")
	ErrMatchNotOpen      = errors.New("match is not open for joining")
	ErrAlreadyJoined     = errors.New("already joined this match")
	ErrNotParticipant    = errors.New("not a participant in this match")
	ErrNotCreator        = errors.New("only the match creator can perform this action")
	ErrActiveMatchExists = errors.New("an active match already exists; please wait for it to complete before creating a new one")
)

// Card and deck errors
var (
	ErrNotEnoughCards = errors.New("not enough cards in group (minimum 10 required)")
)

// Shop phase errors
var (
	ErrMatchNotInShopPhase  = errors.New("match is not in shop phase")
	ErrShopPhaseExpired     = errors.New("shop phase has expired")
	ErrTeamAlreadySubmitted = errors.New("team already submitted")
	ErrInvalidCardIndex     = errors.New("invalid card index")
	ErrNotEnoughCoins       = errors.New("not enough coins")
	ErrTeamFull             = errors.New("team is full (max 3 cards)")
	ErrCardAlreadyPurchased = errors.New("card already purchased")
)

// =====================================================
// TOURNAMENT ERRORS
// =====================================================

var (
	ErrTournamentNotFound           = errors.New("tournament not found")
	ErrTournamentNotOpen            = errors.New("tournament is not open for registration")
	ErrTournamentRegistrationClosed = errors.New("tournament registration has closed")
	ErrAlreadyRegistered            = errors.New("already registered for this tournament")
	ErrNotRegistered                = errors.New("not registered for this tournament")
	ErrNoParticipants               = errors.New("no participants registered")
)

// =====================================================
// MINI APP AUTHENTICATION ERRORS
// =====================================================

var (
	ErrMissingHash     = errors.New("missing hash in init data")
	ErrInvalidHash     = errors.New("invalid hash")
	ErrExpiredInitData = errors.New("init data expired")
	ErrInvalidUserData = errors.New("invalid user data format")
	ErrMissingUserID   = errors.New("missing user ID in init data")
)

// =====================================================
// GAMES API AUTHENTICATION ERRORS
// =====================================================

var (
	ErrMissingGameSignature   = errors.New("missing signature")
	ErrInvalidGameSignature   = errors.New("invalid game signature")
	ErrExpiredGameTimestamp   = errors.New("game timestamp expired")
	ErrMissingGameChatID      = errors.New("missing chat_id")
	ErrMissingGameUserID      = errors.New("missing user_id")
	ErrMissingGameMatchID     = errors.New("missing match_id")
	ErrMissingGameTimestamp   = errors.New("missing timestamp")
	ErrInvalidGameTimestamp   = errors.New("invalid timestamp format")
	ErrFutureGameTimestamp    = errors.New("timestamp is in the future")
)

// =====================================================
// CARD SERVICE ERRORS
// =====================================================

var (
	ErrCardNotFound      = errors.New("card not found")
	ErrCardImageNotFound = errors.New("card image not found")
)

// =====================================================
// PROFILE PHOTO SERVICE ERRORS
// =====================================================

var (
	ErrPhotoNotFound = errors.New("photo not found")
)

// =====================================================
// COMMON SERVICE ERRORS
// =====================================================

var (
	ErrNotEnoughParticipants      = errors.New("need at least 2 participants to start")
	ErrInvalidTeamOrder           = errors.New("order length must match team size")
	ErrStorageClientNotConfigured = errors.New("storage client not configured")
)
