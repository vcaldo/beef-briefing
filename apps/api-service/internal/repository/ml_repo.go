package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MLRepository handles database operations for ML analytics.
type MLRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewMLRepository creates a new MLRepository.
func NewMLRepository(db *sql.DB, nrApp *newrelic.Application) *MLRepository {
	return &MLRepository{db: db, nrApp: nrApp}
}

// UnprocessedMessage represents a message that hasn't been processed by ML.
type UnprocessedMessage struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	UserID    *int64 `json:"user_id,omitempty"`
	Text      string `json:"text"`
}

// SentimentResult represents sentiment analysis results for a message.
type SentimentResult struct {
	MessageID     int64   `json:"message_id"`
	ChatID        int64   `json:"chat_id"`
	Label         string  `json:"label"`
	ScorePositive float32 `json:"score_positive"`
	ScoreNeutral  float32 `json:"score_neutral"`
	ScoreNegative float32 `json:"score_negative"`
	Confidence    float32 `json:"confidence"`
}

// ToxicityResult represents toxicity detection results for a message.
type ToxicityResult struct {
	MessageID int64   `json:"message_id"`
	ChatID    int64   `json:"chat_id"`
	IsToxic   bool    `json:"is_toxic"`
	Label     string  `json:"label"`
	Score     float32 `json:"score"`
}

// HumorResult represents humor detection results for a message.
type HumorResult struct {
	MessageID  int64   `json:"message_id"`
	ChatID     int64   `json:"chat_id"`
	IsHumorous bool    `json:"is_humorous"`
	HumorType  string  `json:"humor_type"`
	Score      float32 `json:"score"`
}

// QuestionResult represents question detection results for a message.
type QuestionResult struct {
	MessageID    int64   `json:"message_id"`
	ChatID       int64   `json:"chat_id"`
	IsQuestion   bool    `json:"is_question"`
	QuestionType string  `json:"question_type"`
	Score        float32 `json:"score"`
}

// NERResult represents a named entity extracted from a message.
type NERResult struct {
	MessageID  int64   `json:"message_id"`
	ChatID     int64   `json:"chat_id"`
	EntityType string  `json:"entity_type"`
	EntityText string  `json:"entity_text"`
	StartPos   *int    `json:"start_pos,omitempty"`
	EndPos     *int    `json:"end_pos,omitempty"`
	Confidence float32 `json:"confidence"`
}

// MessageTopicResult represents a message's topic assignment.
type MessageTopicResult struct {
	MessageID  int64   `json:"message_id"`
	ChatID     int64   `json:"chat_id"`
	TopicID    int     `json:"topic_id"`
	Similarity float32 `json:"similarity"`
}

// GetUnprocessedMessages fetches messages that haven't been processed by ML.
func (r *MLRepository) GetUnprocessedMessages(ctx context.Context, limit int) ([]UnprocessedMessage, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-get-unprocessed")
		defer segment.End()
	}

	query := `
		SELECT m.id, m.message_id, m.chat_id, m.user_id,
			   COALESCE(m.text, m.caption, '') as text
		FROM messages m
		LEFT JOIN ml_processing_state ps ON ps.message_id = m.id
		WHERE ps.message_id IS NULL
		  AND COALESCE(m.text, m.caption, '') != ''
		ORDER BY m.date ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed messages: %w", err)
	}
	defer rows.Close()

	var messages []UnprocessedMessage
	for rows.Next() {
		var msg UnprocessedMessage
		var userID sql.NullInt64

		if err := rows.Scan(&msg.ID, &msg.MessageID, &msg.ChatID, &userID, &msg.Text); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if userID.Valid {
			msg.UserID = &userID.Int64
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return messages, nil
}

// GetProcessingStats returns statistics about ML processing.
func (r *MLRepository) GetProcessingStats(ctx context.Context) (map[string]int64, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-get-stats")
		defer segment.End()
	}

	stats := make(map[string]int64)

	// Total messages with text
	var totalWithText int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE COALESCE(text, caption, '') != ''
	`).Scan(&totalWithText)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}
	stats["total_with_text"] = totalWithText

	// Processed messages
	var processed int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_processing_state`).Scan(&processed)
	if err != nil {
		return nil, fmt.Errorf("failed to count processed: %w", err)
	}
	stats["processed"] = processed

	// Unprocessed
	stats["unprocessed"] = totalWithText - processed

	// Sentiment counts
	var sentimentCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_sentiment`).Scan(&sentimentCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count sentiment: %w", err)
	}
	stats["sentiment_analyzed"] = sentimentCount

	// Toxicity counts
	var toxicityCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_toxicity`).Scan(&toxicityCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count toxicity: %w", err)
	}
	stats["toxicity_analyzed"] = toxicityCount

	// Toxic messages count
	var toxicCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_toxicity WHERE is_toxic = true`).Scan(&toxicCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count toxic: %w", err)
	}
	stats["toxic_messages"] = toxicCount

	// Humor counts (use 0 if table doesn't exist yet)
	var humorCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_humor`).Scan(&humorCount)
	if err == nil {
		stats["humor_analyzed"] = humorCount
	} else {
		stats["humor_analyzed"] = 0
	}

	// Question counts
	var questionCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_questions`).Scan(&questionCount)
	if err == nil {
		stats["questions_analyzed"] = questionCount
	} else {
		stats["questions_analyzed"] = 0
	}

	// NER counts (count unique messages with entities)
	var nerCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT message_id) FROM ml_ner`).Scan(&nerCount)
	if err == nil {
		stats["entities_extracted"] = nerCount
	} else {
		stats["entities_extracted"] = 0
	}

	// Topic assignment counts
	var topicCount int64
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ml_message_topics`).Scan(&topicCount)
	if err == nil {
		stats["topics_assigned"] = topicCount
	} else {
		stats["topics_assigned"] = 0
	}

	return stats, nil
}

// SaveSentimentResults saves sentiment analysis results in batch.
func (r *MLRepository) SaveSentimentResults(ctx context.Context, results []SentimentResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-sentiment")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_sentiment (message_id, chat_id, label, score_positive, score_neutral, score_negative, confidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (message_id) DO UPDATE SET
			label = EXCLUDED.label,
			score_positive = EXCLUDED.score_positive,
			score_neutral = EXCLUDED.score_neutral,
			score_negative = EXCLUDED.score_negative,
			confidence = EXCLUDED.confidence,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.Label, r.ScorePositive, r.ScoreNeutral, r.ScoreNegative, r.Confidence)
		if err != nil {
			return fmt.Errorf("failed to insert sentiment for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveToxicityResults saves toxicity detection results in batch.
func (r *MLRepository) SaveToxicityResults(ctx context.Context, results []ToxicityResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-toxicity")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_toxicity (message_id, chat_id, is_toxic, label, score)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id) DO UPDATE SET
			is_toxic = EXCLUDED.is_toxic,
			label = EXCLUDED.label,
			score = EXCLUDED.score,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.IsToxic, r.Label, r.Score)
		if err != nil {
			return fmt.Errorf("failed to insert toxicity for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MarkMessagesProcessed marks messages as processed by ML.
func (r *MLRepository) MarkMessagesProcessed(ctx context.Context, messageIDs []int64, chatIDs []int64, version string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-mark-processed")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_processing_state (message_id, chat_id, processor_version)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id) DO UPDATE SET
			processor_version = EXCLUDED.processor_version,
			processed_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, msgID := range messageIDs {
		chatID := chatIDs[i]
		_, err := stmt.ExecContext(ctx, msgID, chatID, version)
		if err != nil {
			return fmt.Errorf("failed to mark message %d as processed: %w", msgID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveTopics saves or updates topic clusters for a chat.
func (r *MLRepository) SaveTopics(ctx context.Context, chatID int64, topics map[int][]string) error {
	if len(topics) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-topics")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_topics (chat_id, topic_id, keywords)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, topic_id) DO UPDATE SET
			keywords = EXCLUDED.keywords,
			updated_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for topicID, keywords := range topics {
		_, err := stmt.ExecContext(ctx, chatID, topicID, pq.Array(keywords))
		if err != nil {
			return fmt.Errorf("failed to save topic %d: %w", topicID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveHumorResults saves humor detection results in batch.
func (r *MLRepository) SaveHumorResults(ctx context.Context, results []HumorResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-humor")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_humor (message_id, chat_id, is_humorous, humor_type, score)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id) DO UPDATE SET
			is_humorous = EXCLUDED.is_humorous,
			humor_type = EXCLUDED.humor_type,
			score = EXCLUDED.score,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		humorType := sql.NullString{String: r.HumorType, Valid: r.HumorType != ""}
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.IsHumorous, humorType, r.Score)
		if err != nil {
			return fmt.Errorf("failed to insert humor for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveQuestionResults saves question detection results in batch.
func (r *MLRepository) SaveQuestionResults(ctx context.Context, results []QuestionResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-questions")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_questions (message_id, chat_id, is_question, question_type, score)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id) DO UPDATE SET
			is_question = EXCLUDED.is_question,
			question_type = EXCLUDED.question_type,
			score = EXCLUDED.score,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		questionType := sql.NullString{String: r.QuestionType, Valid: r.QuestionType != ""}
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.IsQuestion, questionType, r.Score)
		if err != nil {
			return fmt.Errorf("failed to insert question for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveNERResults saves named entity recognition results in batch.
func (r *MLRepository) SaveNERResults(ctx context.Context, results []NERResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-ner")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_ner (message_id, chat_id, entity_type, entity_text, start_pos, end_pos, confidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (message_id, entity_type, entity_text, COALESCE(start_pos, -1)) DO UPDATE SET
			confidence = EXCLUDED.confidence,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		var startPos, endPos sql.NullInt32
		if r.StartPos != nil {
			startPos = sql.NullInt32{Int32: int32(*r.StartPos), Valid: true}
		}
		if r.EndPos != nil {
			endPos = sql.NullInt32{Int32: int32(*r.EndPos), Valid: true}
		}
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.EntityType, r.EntityText, startPos, endPos, r.Confidence)
		if err != nil {
			return fmt.Errorf("failed to insert NER for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveMessageTopics saves message-to-topic assignments in batch.
func (r *MLRepository) SaveMessageTopics(ctx context.Context, results []MessageTopicResult) error {
	if len(results) == 0 {
		return nil
	}

	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:ml-save-message-topics")
		defer segment.End()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ml_message_topics (message_id, chat_id, topic_id, similarity)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (message_id) DO UPDATE SET
			topic_id = EXCLUDED.topic_id,
			similarity = EXCLUDED.similarity,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		_, err := stmt.ExecContext(ctx, r.MessageID, r.ChatID, r.TopicID, r.Similarity)
		if err != nil {
			return fmt.Errorf("failed to insert topic for message %d: %w", r.MessageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
