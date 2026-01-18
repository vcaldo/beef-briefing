package battle

import (
	"encoding/json"
	"testing"
)

// =============================================================================
// ShopCard.ToCard() Tests
// =============================================================================

func TestShopCard_ToCard_CopiesAllFields(t *testing.T) {
	placeholderPositions := json.RawMessage(`{"version":"1.0","placeholders":{"atk":{"x":10,"y":20}}}`)

	shopCard := &ShopCard{
		CardID:               101,
		UserID:               202,
		Name:                 "TestUser",
		Username:             "testuser",
		PhotoURL:             "https://example.com/photo.jpg",
		CardImageURL:         "https://example.com/card.png",         // Regular card (shop display)
		CompactCardImageURL:  "https://example.com/card_compact.png", // Compact card (team display)
		PlaceholderPositions: placeholderPositions,
		ATK:                  15,
		DEF:                  10,
		HP:                   50,
		IsPurchased:          true,
		Index:                2,
	}

	card := shopCard.ToCard()

	// Verify all fields are copied correctly
	if card.CardID != shopCard.CardID {
		t.Errorf("expected CardID=%d, got %d", shopCard.CardID, card.CardID)
	}
	if card.UserID != shopCard.UserID {
		t.Errorf("expected UserID=%d, got %d", shopCard.UserID, card.UserID)
	}
	if card.Name != shopCard.Name {
		t.Errorf("expected Name=%q, got %q", shopCard.Name, card.Name)
	}
	if card.Username != shopCard.Username {
		t.Errorf("expected Username=%q, got %q", shopCard.Username, card.Username)
	}
	if card.PhotoURL != shopCard.PhotoURL {
		t.Errorf("expected PhotoURL=%q, got %q", shopCard.PhotoURL, card.PhotoURL)
	}
	// ToCard uses CompactCardImageURL for team display (not CardImageURL)
	if card.CardImageURL != shopCard.CompactCardImageURL {
		t.Errorf("expected CardImageURL=%q (from CompactCardImageURL), got %q", shopCard.CompactCardImageURL, card.CardImageURL)
	}
	if string(card.PlaceholderPositions) != string(shopCard.PlaceholderPositions) {
		t.Errorf("expected PlaceholderPositions=%s, got %s", shopCard.PlaceholderPositions, card.PlaceholderPositions)
	}
	if card.ATK != shopCard.ATK {
		t.Errorf("expected ATK=%d, got %d", shopCard.ATK, card.ATK)
	}
	if card.DEF != shopCard.DEF {
		t.Errorf("expected DEF=%d, got %d", shopCard.DEF, card.DEF)
	}
	if card.HP != shopCard.HP {
		t.Errorf("expected HP=%d, got %d", shopCard.HP, card.HP)
	}
	if card.MaxHP != shopCard.HP {
		t.Errorf("expected MaxHP=%d (from ShopCard.HP), got %d", shopCard.HP, card.MaxHP)
	}
}

func TestShopCard_ToCard_InitializesUpgradesToZero(t *testing.T) {
	shopCard := &ShopCard{
		CardID: 101,
		UserID: 202,
		Name:   "TestUser",
		ATK:    15,
		DEF:    10,
		HP:     50,
	}

	card := shopCard.ToCard()

	if card.ATKUpgrades != 0 {
		t.Errorf("expected ATKUpgrades=0, got %d", card.ATKUpgrades)
	}
	if card.HPUpgrades != 0 {
		t.Errorf("expected HPUpgrades=0, got %d", card.HPUpgrades)
	}
}

func TestShopCard_ToCard_HandlesEmptyOptionalFields(t *testing.T) {
	shopCard := &ShopCard{
		CardID: 101,
		UserID: 202,
		Name:   "TestUser",
		ATK:    15,
		DEF:    10,
		HP:     50,
		// Username, PhotoURL, CardImageURL, PlaceholderPositions all empty
	}

	card := shopCard.ToCard()

	if card.Username != "" {
		t.Errorf("expected empty Username, got %q", card.Username)
	}
	if card.PhotoURL != "" {
		t.Errorf("expected empty PhotoURL, got %q", card.PhotoURL)
	}
	if card.CardImageURL != "" {
		t.Errorf("expected empty CardImageURL, got %q", card.CardImageURL)
	}
	if card.PlaceholderPositions != nil {
		t.Errorf("expected nil PlaceholderPositions, got %s", card.PlaceholderPositions)
	}
}

func TestShopCard_ToCard_CopiesPlaceholderPositions(t *testing.T) {
	// Test with complex JSON structure
	placeholderJSON := json.RawMessage(`{
		"version": "1.0",
		"card_dimensions": {"width": 300, "height": 450},
		"placeholders": {
			"atk": {"x": 10, "y": 20, "width": 50, "height": 30},
			"def": {"x": 70, "y": 20, "width": 50, "height": 30},
			"hp": {"x": 130, "y": 20, "width": 50, "height": 30}
		}
	}`)

	shopCard := &ShopCard{
		CardID:               101,
		UserID:               202,
		Name:                 "TestUser",
		PlaceholderPositions: placeholderJSON,
		ATK:                  15,
		DEF:                  10,
		HP:                   50,
	}

	card := shopCard.ToCard()

	if card.PlaceholderPositions == nil {
		t.Fatal("expected PlaceholderPositions to be copied, got nil")
	}

	// Verify the JSON is valid and identical
	if string(card.PlaceholderPositions) != string(placeholderJSON) {
		t.Errorf("PlaceholderPositions not copied correctly\nexpected: %s\ngot: %s",
			placeholderJSON, card.PlaceholderPositions)
	}

	// Verify it's valid JSON by unmarshaling
	var positions map[string]interface{}
	if err := json.Unmarshal(card.PlaceholderPositions, &positions); err != nil {
		t.Errorf("PlaceholderPositions is not valid JSON: %v", err)
	}

	// Verify structure
	if positions["version"] != "1.0" {
		t.Errorf("expected version=1.0 in positions, got %v", positions["version"])
	}
}

func TestShopCard_ToCard_CopiesCompactCardImageURL(t *testing.T) {
	regularURL := "https://storage.example.com/cards/neon_arcade/abc123.png"
	compactURL := "https://storage.example.com/cards/neon_arcade_compact/abc123.png"

	shopCard := &ShopCard{
		CardID:              101,
		UserID:              202,
		Name:                "TestUser",
		CardImageURL:        regularURL, // Regular card for shop display
		CompactCardImageURL: compactURL, // Compact card for team display
		ATK:                 15,
		DEF:                 10,
		HP:                  50,
	}

	card := shopCard.ToCard()

	// ToCard should use CompactCardImageURL for team display
	if card.CardImageURL != compactURL {
		t.Errorf("expected CardImageURL=%q (from CompactCardImageURL), got %q", compactURL, card.CardImageURL)
	}
}

func TestShopCard_ToCard_NilPlaceholderPositions(t *testing.T) {
	shopCard := &ShopCard{
		CardID:               101,
		UserID:               202,
		Name:                 "TestUser",
		PlaceholderPositions: nil, // Explicitly nil
		ATK:                  15,
		DEF:                  10,
		HP:                   50,
	}

	card := shopCard.ToCard()

	if card.PlaceholderPositions != nil {
		t.Errorf("expected nil PlaceholderPositions, got %s", card.PlaceholderPositions)
	}
}

func TestShopCard_ToCard_EmptyJSONPlaceholderPositions(t *testing.T) {
	shopCard := &ShopCard{
		CardID:               101,
		UserID:               202,
		Name:                 "TestUser",
		PlaceholderPositions: json.RawMessage(`{}`), // Empty JSON object
		ATK:                  15,
		DEF:                  10,
		HP:                   50,
	}

	card := shopCard.ToCard()

	if card.PlaceholderPositions == nil {
		t.Error("expected PlaceholderPositions to be copied (even if empty), got nil")
	}
	if string(card.PlaceholderPositions) != "{}" {
		t.Errorf("expected PlaceholderPositions='{}', got %s", card.PlaceholderPositions)
	}
}
