package config

import "testing"

func TestParseBetaGroups_Empty(t *testing.T) {
	cfg := &Config{BetaGroups: ""}
	result := cfg.ParseBetaGroups()
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseBetaGroups_Single(t *testing.T) {
	cfg := &Config{BetaGroups: "-1001234567890"}
	result := cfg.ParseBetaGroups()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if !result[-1001234567890] {
		t.Error("expected -1001234567890 to be in map")
	}
}

func TestParseBetaGroups_Multiple(t *testing.T) {
	cfg := &Config{BetaGroups: "-1001234567890, -1009876543210, -1005555555555"}
	result := cfg.ParseBetaGroups()
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	for _, id := range []int64{-1001234567890, -1009876543210, -1005555555555} {
		if !result[id] {
			t.Errorf("expected %d to be in map", id)
		}
	}
}

func TestParseBetaGroups_SkipsInvalid(t *testing.T) {
	cfg := &Config{BetaGroups: "-1001234567890,invalid,-1009876543210"}
	result := cfg.ParseBetaGroups()
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestIsBetaGroup_Empty(t *testing.T) {
	cfg := &Config{BetaGroups: ""}
	if cfg.IsBetaGroup(-1001234567890) {
		t.Error("expected false for empty beta groups")
	}
}

func TestIsBetaGroup_Match(t *testing.T) {
	cfg := &Config{BetaGroups: "-1001234567890,-1009876543210"}
	if !cfg.IsBetaGroup(-1001234567890) {
		t.Error("expected true for listed group")
	}
	if !cfg.IsBetaGroup(-1009876543210) {
		t.Error("expected true for listed group")
	}
}

func TestIsBetaGroup_NoMatch(t *testing.T) {
	cfg := &Config{BetaGroups: "-1001234567890,-1009876543210"}
	if cfg.IsBetaGroup(-1005555555555) {
		t.Error("expected false for unlisted group")
	}
}

func TestIsAdmin_Empty(t *testing.T) {
	cfg := &Config{AdminUserIDs: ""}
	if cfg.IsAdmin(123) {
		t.Error("expected false for empty admin list")
	}
}

func TestIsAdmin_Match(t *testing.T) {
	cfg := &Config{AdminUserIDs: "123,456"}
	if !cfg.IsAdmin(123) {
		t.Error("expected true for listed admin")
	}
}
