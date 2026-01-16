package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type ReactionRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

func NewReactionRepository(db *sql.DB, nrApp *newrelic.Application) *ReactionRepository {
	return &ReactionRepository{db: db, nrApp: nrApp}
}

// InsertMessageReaction inserts individual user reactions
func (r *ReactionRepository) InsertMessageReaction(ctx context.Context, tx *sql.Tx, reaction *models.MessageReactionUpdated) error {
	defer nrutil.StartSegment(ctx, "db:insert-message-reaction")()

	// Insert reactions from new_reaction array
	for _, newReact := range reaction.NewReaction {
		var userID sql.NullInt64
		if reaction.User != nil {
			userID = sql.NullInt64{Int64: reaction.User.ID, Valid: true}
		}

		var actorChatID sql.NullInt64
		if reaction.ActorChat != nil {
			actorChatID = sql.NullInt64{Int64: reaction.ActorChat.ID, Valid: true}
		}

		reactionType := newReact.Type
		emojiValue := NullString(newReact.Emoji)
		customEmojiID := NullString(newReact.CustomEmojiID)

		query := `
			INSERT INTO message_reactions (
				chat_id, message_id, user_id, actor_chat_id,
				reaction_type, emoji_value, custom_emoji_id, date, is_removed
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (chat_id, message_id, user_id, reaction_type, emoji_value, custom_emoji_id)
			DO UPDATE SET
				is_removed = EXCLUDED.is_removed,
				date = EXCLUDED.date
		`

		_, err := tx.ExecContext(ctx, query,
			reaction.Chat.ID,
			reaction.MessageID,
			userID,
			actorChatID,
			reactionType,
			emojiValue,
			customEmojiID,
			time.Unix(reaction.Date, 0).UTC(),
			false,
		)
		if err != nil {
			return fmt.Errorf("failed to insert message reaction: %w", err)
		}
	}

	// Mark old reactions as removed if they're not in new_reaction
	for _, oldReact := range reaction.OldReaction {
		// Check if this reaction is still in new_reaction
		found := false
		for _, newReact := range reaction.NewReaction {
			if oldReact.Type == newReact.Type &&
				oldReact.Emoji == newReact.Emoji &&
				oldReact.CustomEmojiID == newReact.CustomEmojiID {
				found = true
				break
			}
		}

		if !found {
			var userID sql.NullInt64
			if reaction.User != nil {
				userID = sql.NullInt64{Int64: reaction.User.ID, Valid: true}
			}

			query := `
				UPDATE message_reactions
				SET is_removed = true, date = $1
				WHERE chat_id = $2 AND message_id = $3 AND user_id = $4
					AND reaction_type = $5 AND emoji_value = $6 AND custom_emoji_id = $7
			`

			_, err := tx.ExecContext(ctx, query,
				time.Unix(reaction.Date, 0).UTC(),
				reaction.Chat.ID,
				reaction.MessageID,
				userID,
				oldReact.Type,
				NullString(oldReact.Emoji),
				NullString(oldReact.CustomEmojiID),
			)
			if err != nil {
				return fmt.Errorf("failed to mark reaction as removed: %w", err)
			}
		}
	}

	return nil
}

// UpsertReactionCount upserts aggregate reaction counts
func (r *ReactionRepository) UpsertReactionCount(ctx context.Context, tx *sql.Tx, countUpdate *models.MessageReactionCountUpdate) error {
	for _, reactionCount := range countUpdate.Reactions {
		query := `
			INSERT INTO reaction_counts (
				chat_id, message_id, reaction_type, emoji_value, custom_emoji_id, total_count, date
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (chat_id, message_id, reaction_type, emoji_value, custom_emoji_id)
			DO UPDATE SET
				total_count = EXCLUDED.total_count,
				date = EXCLUDED.date,
				updated_at = NOW()
		`

		_, err := tx.ExecContext(ctx, query,
			countUpdate.Chat.ID,
			countUpdate.MessageID,
			reactionCount.Type.Type,
			NullString(reactionCount.Type.Emoji),
			NullString(reactionCount.Type.CustomEmojiID),
			reactionCount.TotalCount,
			time.Unix(countUpdate.Date, 0).UTC(),
		)
		if err != nil {
			return fmt.Errorf("failed to upsert reaction count: %w", err)
		}
	}

	return nil
}
