package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// setupUpdateTestDB creates a new database connection for testing
func setupUpdateTestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnvOrDefaultUpdate("TEST_DB_HOST", "localhost")
	port := getEnvOrDefaultUpdate("TEST_DB_PORT", "5432")
	user := getEnvOrDefaultUpdate("TEST_DB_USER", "postgres")
	password := getEnvOrDefaultUpdate("TEST_DB_PASSWORD", "Vb96vpmAEZVdSpruALAgDmZrHFRDXATg")
	dbname := getEnvOrDefaultUpdate("TEST_DB_NAME", "beef_briefing")
	sslmode := getEnvOrDefaultUpdate("TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	return db
}

// getEnvOrDefaultUpdate retrieves an environment variable or returns a default value
func getEnvOrDefaultUpdate(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// teardownUpdateTestDB closes the test database connection
func teardownUpdateTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db != nil {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	}
}

// cleanupUpdateTables truncates the updates table for test isolation
func cleanupUpdateTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE updates CASCADE"); err != nil {
		t.Fatalf("failed to truncate updates table: %v", err)
	}
}

// TestInsertUpdateMessage tests inserting a message update type
func TestInsertUpdateMessage(t *testing.T) {
	db := setupUpdateTestDB(t)
	defer teardownUpdateTestDB(t, db)
	cleanupUpdateTables(t, db)

	repo := NewUpdateRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Sample message update data
	updateID := int64(12345)
	updateType := "message"
	rawData := map[string]interface{}{
		"update_id": updateID,
		"message": map[string]interface{}{
			"message_id": 1,
			"chat": map[string]interface{}{
				"id":   -1001234567890,
				"type": "supergroup",
			},
			"from": map[string]interface{}{
				"id":         123456789,
				"first_name": "John",
			},
			"date": 1733611200,
			"text": "Hello, world!",
		},
	}

	// Insert the update
	err = repo.InsertUpdate(ctx, tx, updateID, updateType, rawData)
	if err != nil {
		t.Fatalf("failed to insert update: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify the update was inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM updates WHERE update_id = $1", updateID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query updates: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 update, got %d", count)
	}

	// Verify the data was stored correctly
	var storedType string
	var storedData []byte
	err = db.QueryRowContext(ctx, "SELECT update_type, raw_data FROM updates WHERE update_id = $1", updateID).Scan(&storedType, &storedData)
	if err != nil {
		t.Fatalf("failed to query update data: %v", err)
	}

	if storedType != updateType {
		t.Errorf("expected update_type %q, got %q", updateType, storedType)
	}

	// Verify JSON can be unmarshaled
	var decodedData map[string]interface{}
	if err := json.Unmarshal(storedData, &decodedData); err != nil {
		t.Fatalf("failed to unmarshal stored JSON: %v", err)
	}

	// Verify message text exists in the stored data
	message, ok := decodedData["message"].(map[string]interface{})
	if !ok {
		t.Fatal("stored data does not contain 'message' field")
	}

	text, ok := message["text"].(string)
	if !ok || text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got %q", text)
	}
}

// TestInsertUpdateReaction tests inserting a reaction update type
func TestInsertUpdateReaction(t *testing.T) {
	db := setupUpdateTestDB(t)
	defer teardownUpdateTestDB(t, db)
	cleanupUpdateTables(t, db)

	repo := NewUpdateRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Sample reaction update data
	updateID := int64(67890)
	updateType := "message_reaction"
	rawData := map[string]interface{}{
		"update_id": updateID,
		"message_reaction": map[string]interface{}{
			"chat": map[string]interface{}{
				"id":   -1001234567890,
				"type": "supergroup",
			},
			"message_id": 123,
			"user": map[string]interface{}{
				"id":         123456789,
				"first_name": "John",
			},
			"date": 1733611200,
			"old_reaction": []interface{}{
				map[string]interface{}{"type": "emoji", "emoji": "👍"},
			},
			"new_reaction": []interface{}{
				map[string]interface{}{"type": "emoji", "emoji": "❤️"},
			},
		},
	}

	// Insert the update
	err = repo.InsertUpdate(ctx, tx, updateID, updateType, rawData)
	if err != nil {
		t.Fatalf("failed to insert update: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify the update was inserted
	var storedType string
	var storedData []byte
	err = db.QueryRowContext(ctx, "SELECT update_type, raw_data FROM updates WHERE update_id = $1", updateID).Scan(&storedType, &storedData)
	if err != nil {
		t.Fatalf("failed to query update: %v", err)
	}

	if storedType != updateType {
		t.Errorf("expected update_type %q, got %q", updateType, storedType)
	}

	// Verify JSON structure
	var decodedData map[string]interface{}
	if err := json.Unmarshal(storedData, &decodedData); err != nil {
		t.Fatalf("failed to unmarshal stored JSON: %v", err)
	}

	messageReaction, ok := decodedData["message_reaction"].(map[string]interface{})
	if !ok {
		t.Fatal("stored data does not contain 'message_reaction' field")
	}

	// Verify reaction data exists
	if messageReaction["message_id"] == nil {
		t.Error("message_reaction should contain message_id")
	}
}

// TestInsertUpdateOtherTypes tests inserting various other update types
func TestInsertUpdateOtherTypes(t *testing.T) {
	db := setupUpdateTestDB(t)
	defer teardownUpdateTestDB(t, db)
	cleanupUpdateTables(t, db)

	repo := NewUpdateRepository(db, nil)
	ctx := context.Background()

	tests := []struct {
		name       string
		updateID   int64
		updateType string
		rawData    map[string]interface{}
	}{
		{
			name:       "edited_message",
			updateID:   100001,
			updateType: "edited_message",
			rawData: map[string]interface{}{
				"update_id": 100001,
				"edited_message": map[string]interface{}{
					"message_id": 1,
					"chat": map[string]interface{}{
						"id":   -1001234567890,
						"type": "supergroup",
					},
					"edit_date": 1733611300,
					"text":      "Edited text",
				},
			},
		},
		{
			name:       "channel_post",
			updateID:   100002,
			updateType: "channel_post",
			rawData: map[string]interface{}{
				"update_id": 100002,
				"channel_post": map[string]interface{}{
					"message_id": 2,
					"chat": map[string]interface{}{
						"id":   -1001234567890,
						"type": "channel",
					},
					"date": 1733611200,
					"text": "Channel announcement",
				},
			},
		},
		{
			name:       "message_reaction_count",
			updateID:   100003,
			updateType: "message_reaction_count",
			rawData: map[string]interface{}{
				"update_id": 100003,
				"message_reaction_count": map[string]interface{}{
					"chat": map[string]interface{}{
						"id":   -1001234567890,
						"type": "supergroup",
					},
					"message_id": 123,
					"date":       1733611200,
					"reactions": []interface{}{
						map[string]interface{}{
							"type":        "emoji",
							"emoji":       "👍",
							"total_count": 5,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("failed to begin transaction: %v", err)
			}
			defer tx.Rollback()

			// Insert the update
			err = repo.InsertUpdate(ctx, tx, tt.updateID, tt.updateType, tt.rawData)
			if err != nil {
				t.Fatalf("failed to insert update: %v", err)
			}

			// Commit the transaction
			if err := tx.Commit(); err != nil {
				t.Fatalf("failed to commit transaction: %v", err)
			}

			// Verify the update was inserted with correct type
			var storedType string
			err = db.QueryRowContext(ctx, "SELECT update_type FROM updates WHERE update_id = $1", tt.updateID).Scan(&storedType)
			if err != nil {
				t.Fatalf("failed to query update: %v", err)
			}

			if storedType != tt.updateType {
				t.Errorf("expected update_type %q, got %q", tt.updateType, storedType)
			}
		})
	}

	// Verify all updates were inserted
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM updates").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query update count: %v", err)
	}

	if count != len(tests) {
		t.Errorf("expected %d updates, got %d", len(tests), count)
	}
}

// TestInsertUpdateJSONSerialization tests that complex JSON data is correctly serialized
func TestInsertUpdateJSONSerialization(t *testing.T) {
	db := setupUpdateTestDB(t)
	defer teardownUpdateTestDB(t, db)
	cleanupUpdateTables(t, db)

	repo := NewUpdateRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Complex nested JSON structure
	updateID := int64(999999)
	updateType := "message"
	rawData := map[string]interface{}{
		"update_id": updateID,
		"message": map[string]interface{}{
			"message_id": 1,
			"chat": map[string]interface{}{
				"id":   -1001234567890,
				"type": "supergroup",
			},
			"from": map[string]interface{}{
				"id":         123456789,
				"first_name": "John",
				"last_name":  "Doe",
				"username":   "johndoe",
				"is_bot":     false,
			},
			"date": 1733611200,
			"text": "Message with entities",
			"entities": []interface{}{
				map[string]interface{}{
					"type":   "mention",
					"offset": 0,
					"length": 8,
				},
				map[string]interface{}{
					"type":   "url",
					"offset": 20,
					"length": 15,
					"url":    "https://example.com",
				},
			},
			"reply_markup": map[string]interface{}{
				"inline_keyboard": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"text":          "Button 1",
							"callback_data": "action_1",
						},
					},
				},
			},
		},
	}

	// Insert the update
	err = repo.InsertUpdate(ctx, tx, updateID, updateType, rawData)
	if err != nil {
		t.Fatalf("failed to insert update: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Retrieve and verify the data
	var storedData []byte
	err = db.QueryRowContext(ctx, "SELECT raw_data FROM updates WHERE update_id = $1", updateID).Scan(&storedData)
	if err != nil {
		t.Fatalf("failed to query update: %v", err)
	}

	// Unmarshal and verify structure
	var decodedData map[string]interface{}
	if err := json.Unmarshal(storedData, &decodedData); err != nil {
		t.Fatalf("failed to unmarshal stored JSON: %v", err)
	}

	message, ok := decodedData["message"].(map[string]interface{})
	if !ok {
		t.Fatal("stored data does not contain 'message' field")
	}

	// Verify nested entities array
	entities, ok := message["entities"].([]interface{})
	if !ok {
		t.Fatal("message does not contain 'entities' array")
	}

	if len(entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(entities))
	}

	// Verify first entity
	firstEntity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("first entity is not a map")
	}

	entityType, ok := firstEntity["type"].(string)
	if !ok || entityType != "mention" {
		t.Errorf("expected first entity type 'mention', got %q", entityType)
	}

	// Verify nested reply_markup
	replyMarkup, ok := message["reply_markup"].(map[string]interface{})
	if !ok {
		t.Fatal("message does not contain 'reply_markup' field")
	}

	inlineKeyboard, ok := replyMarkup["inline_keyboard"].([]interface{})
	if !ok || len(inlineKeyboard) == 0 {
		t.Fatal("reply_markup does not contain valid 'inline_keyboard'")
	}
}

// TestInsertUpdateDuplicateHandling tests ON CONFLICT DO NOTHING behavior
func TestInsertUpdateDuplicateHandling(t *testing.T) {
	db := setupUpdateTestDB(t)
	defer teardownUpdateTestDB(t, db)
	cleanupUpdateTables(t, db)

	repo := NewUpdateRepository(db, nil)
	ctx := context.Background()

	updateID := int64(555555)
	updateType := "message"
	rawData1 := map[string]interface{}{
		"update_id": updateID,
		"message": map[string]interface{}{
			"text": "First insert",
		},
	}

	rawData2 := map[string]interface{}{
		"update_id": updateID,
		"message": map[string]interface{}{
			"text": "Second insert (should be ignored)",
		},
	}

	// First insert
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin first transaction: %v", err)
	}
	defer tx1.Rollback()

	err = repo.InsertUpdate(ctx, tx1, updateID, updateType, rawData1)
	if err != nil {
		t.Fatalf("failed to insert first update: %v", err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("failed to commit first transaction: %v", err)
	}

	// Second insert (should be ignored due to ON CONFLICT)
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin second transaction: %v", err)
	}
	defer tx2.Rollback()

	err = repo.InsertUpdate(ctx, tx2, updateID, updateType, rawData2)
	if err != nil {
		t.Fatalf("second insert should not fail: %v", err)
	}

	if err := tx2.Commit(); err != nil {
		t.Fatalf("failed to commit second transaction: %v", err)
	}

	// Verify only one update exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM updates WHERE update_id = $1", updateID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query update count: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 update after duplicate insert, got %d", count)
	}

	// Verify the original data was kept (not updated)
	var storedData []byte
	err = db.QueryRowContext(ctx, "SELECT raw_data FROM updates WHERE update_id = $1", updateID).Scan(&storedData)
	if err != nil {
		t.Fatalf("failed to query update data: %v", err)
	}

	var decodedData map[string]interface{}
	if err := json.Unmarshal(storedData, &decodedData); err != nil {
		t.Fatalf("failed to unmarshal stored JSON: %v", err)
	}

	message, ok := decodedData["message"].(map[string]interface{})
	if !ok {
		t.Fatal("stored data does not contain 'message' field")
	}

	text, ok := message["text"].(string)
	if !ok || text != "First insert" {
		t.Errorf("expected original text 'First insert', got %q (conflict handling failed)", text)
	}
}
