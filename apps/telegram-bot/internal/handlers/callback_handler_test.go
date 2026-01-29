package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"
)

// TestSignGameURL_SignatureFormat tests that the signature generation
// produces the expected HMAC-SHA256 format
func TestSignGameURL_SignatureFormat(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800) // 2024-01-30 12:00:00 UTC

	sig := signGameURL(botToken, chatID, userID, matchID, ts)

	// Verify signature is valid hex
	decoded, err := hex.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature is not valid hex: %v", err)
	}

	// Verify signature length (SHA256 = 32 bytes = 64 hex chars)
	if len(sig) != 64 {
		t.Errorf("expected signature length 64, got %d", len(sig))
	}
	if len(decoded) != 32 {
		t.Errorf("expected decoded signature length 32, got %d", len(decoded))
	}
}

// TestSignGameURL_Deterministic tests that the same inputs produce
// the same signature (deterministic output)
func TestSignGameURL_Deterministic(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800)

	sig1 := signGameURL(botToken, chatID, userID, matchID, ts)
	sig2 := signGameURL(botToken, chatID, userID, matchID, ts)

	if sig1 != sig2 {
		t.Errorf("same inputs should produce same signature: got %s and %s", sig1, sig2)
	}
}

// TestSignGameURL_DifferentInputsProduceDifferentSignatures tests that
// changing any parameter produces a different signature
func TestSignGameURL_DifferentInputsProduceDifferentSignatures(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800)

	baseSig := signGameURL(botToken, chatID, userID, matchID, ts)

	tests := []struct {
		name     string
		botToken string
		chatID   int64
		userID   int64
		matchID  string
		ts       int64
	}{
		{"different bot token", "different-token", chatID, userID, matchID, ts},
		{"different chat_id", botToken, -1009999999999, userID, matchID, ts},
		{"different user_id", botToken, chatID, 111111111, matchID, ts},
		{"different match_id", botToken, chatID, userID, "different-match", ts},
		{"different timestamp", botToken, chatID, userID, matchID, 1706725200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sig := signGameURL(tc.botToken, tc.chatID, tc.userID, tc.matchID, tc.ts)
			if sig == baseSig {
				t.Errorf("expected different signature for %s, got same: %s", tc.name, sig)
			}
		})
	}
}

// TestSignGameURL_MatchesAPIFormat tests that the signature format matches
// what the API service expects (alphabetically sorted keys)
func TestSignGameURL_MatchesAPIFormat(t *testing.T) {
	botToken := "test-bot-token-123456"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800)

	// Manually compute expected signature using the same algorithm
	// as the API service (alphabetically sorted keys)
	dataString := fmt.Sprintf("chat_id=%d&match_id=%s&ts=%d&user_id=%d", chatID, matchID, ts, userID)

	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(dataString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	actualSig := signGameURL(botToken, chatID, userID, matchID, ts)

	if actualSig != expectedSig {
		t.Errorf("signature mismatch:\n  data: %s\n  expected: %s\n  actual: %s", dataString, expectedSig, actualSig)
	}
}

// TestSignGameURL_EmptyMatchID tests that empty match_id is handled correctly
// (used when user has no active match - lobby-only access)
func TestSignGameURL_EmptyMatchID(t *testing.T) {
	botToken := "test-bot-token-123456"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "" // Empty match ID
	ts := int64(1706638800)

	sig := signGameURL(botToken, chatID, userID, matchID, ts)

	// Verify signature is still produced
	if sig == "" {
		t.Error("signature should not be empty for empty match_id")
	}

	// Verify it's valid hex
	if _, err := hex.DecodeString(sig); err != nil {
		t.Errorf("signature should be valid hex: %v", err)
	}

	// Verify it matches expected format with empty match_id
	dataString := fmt.Sprintf("chat_id=%d&match_id=%s&ts=%d&user_id=%d", chatID, matchID, ts, userID)
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(dataString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if sig != expectedSig {
		t.Errorf("signature mismatch for empty match_id:\n  expected: %s\n  actual: %s", expectedSig, sig)
	}
}

// TestGameURL_ContainsAllRequiredParams tests that the URL built from
// signGameURL contains all required parameters
func TestGameURL_ContainsAllRequiredParams(t *testing.T) {
	botToken := "test-bot-token-123456"
	arenaBaseURL := "https://arena.example.com"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800)

	sig := signGameURL(botToken, chatID, userID, matchID, ts)

	// Build URL the same way handleGameCallback does
	gameURL := fmt.Sprintf(
		"%s?chat_id=%d&user_id=%d&match_id=%s&ts=%d&sig=%s",
		arenaBaseURL,
		chatID,
		userID,
		matchID,
		ts,
		sig,
	)

	// Parse URL
	parsedURL, err := url.Parse(gameURL)
	if err != nil {
		t.Fatalf("failed to parse game URL: %v", err)
	}

	// Verify base URL
	if parsedURL.Scheme != "https" {
		t.Errorf("expected scheme 'https', got '%s'", parsedURL.Scheme)
	}
	if parsedURL.Host != "arena.example.com" {
		t.Errorf("expected host 'arena.example.com', got '%s'", parsedURL.Host)
	}

	// Verify all required query parameters
	query := parsedURL.Query()

	requiredParams := []struct {
		name     string
		expected string
	}{
		{"chat_id", "-1001234567890"},
		{"user_id", "987654321"},
		{"match_id", "match-abc-123"},
		{"ts", "1706638800"},
		{"sig", sig},
	}

	for _, param := range requiredParams {
		actual := query.Get(param.name)
		if actual == "" {
			t.Errorf("missing required parameter: %s", param.name)
		} else if actual != param.expected {
			t.Errorf("parameter %s: expected '%s', got '%s'", param.name, param.expected, actual)
		}
	}
}

// TestGameURL_EmptyMatchIDParam tests URL generation with empty match_id
func TestGameURL_EmptyMatchIDParam(t *testing.T) {
	botToken := "test-bot-token-123456"
	arenaBaseURL := "https://arena.example.com"
	chatID := int64(-1001234567890)
	userID := int64(987654321)
	matchID := "" // Empty - user has no active match
	ts := int64(1706638800)

	sig := signGameURL(botToken, chatID, userID, matchID, ts)

	gameURL := fmt.Sprintf(
		"%s?chat_id=%d&user_id=%d&match_id=%s&ts=%d&sig=%s",
		arenaBaseURL,
		chatID,
		userID,
		matchID,
		ts,
		sig,
	)

	parsedURL, err := url.Parse(gameURL)
	if err != nil {
		t.Fatalf("failed to parse game URL: %v", err)
	}

	query := parsedURL.Query()

	// match_id should be present but empty
	if !query.Has("match_id") {
		t.Error("match_id parameter should be present even when empty")
	}
	if query.Get("match_id") != "" {
		t.Errorf("expected empty match_id, got '%s'", query.Get("match_id"))
	}

	// Other params should still be present
	if query.Get("chat_id") == "" {
		t.Error("chat_id should be present")
	}
	if query.Get("user_id") == "" {
		t.Error("user_id should be present")
	}
	if query.Get("ts") == "" {
		t.Error("ts should be present")
	}
	if query.Get("sig") == "" {
		t.Error("sig should be present")
	}
}

// TestSignGameURL_NegativeChatID tests handling of negative chat IDs
// (Telegram uses negative IDs for groups/channels)
func TestSignGameURL_NegativeChatID(t *testing.T) {
	botToken := "test-bot-token-123456"
	chatID := int64(-1001234567890) // Negative (supergroup)
	userID := int64(987654321)
	matchID := "match-abc-123"
	ts := int64(1706638800)

	sig := signGameURL(botToken, chatID, userID, matchID, ts)

	// Verify signature can be produced for negative chat ID
	if sig == "" {
		t.Error("signature should be produced for negative chat_id")
	}

	// Verify it's different from positive chat ID
	sigPositive := signGameURL(botToken, 1001234567890, userID, matchID, ts)
	if sig == sigPositive {
		t.Error("negative and positive chat_id should produce different signatures")
	}
}
