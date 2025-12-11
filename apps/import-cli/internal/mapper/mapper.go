package mapper

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"beef-briefing/apps/import-cli/internal/models"
)

// BotChecker provides bot status lookup for users
type BotChecker interface {
	IsBot(userID int64) bool
}

// noOpBotChecker always returns false for IsBot
type noOpBotChecker struct{}

func (n *noOpBotChecker) IsBot(userID int64) bool {
	return false
}

// Mapper converts export messages to API format
type Mapper struct {
	chatID     int64
	chatType   string
	chatName   string
	botChecker BotChecker
}

// New creates a new Mapper instance
func New(chatID int64, chatType, chatName string) *Mapper {
	return &Mapper{
		chatID:     chatID,
		chatType:   chatType,
		chatName:   chatName,
		botChecker: &noOpBotChecker{},
	}
}

// SetBotChecker sets the bot checker for user lookups
func (m *Mapper) SetBotChecker(checker BotChecker) {
	if checker != nil {
		m.botChecker = checker
	}
}

// UpdateResult contains the converted update and metadata
type UpdateResult struct {
	Update   *models.Update
	UserID   int64
	IsBot    bool
	UserName string
}

// ToUpdate converts an ExportMessage to an API Update
// Returns UpdateResult with bot status for filtering decisions
func (m *Mapper) ToUpdate(msg *models.ExportMessage) (*UpdateResult, error) {
	// Skip service messages for now
	if msg.IsServiceMessage() {
		return nil, nil
	}

	// Parse unix timestamp
	timestamp, err := strconv.ParseInt(msg.DateUnixtime, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp: %w", err)
	}

	// Parse user ID from from_id (format: "user123456")
	userID, userName := m.parseUser(msg.From, msg.FromID)

	// Check if user is a bot
	isBot := m.botChecker.IsBot(userID)

	// Build the message
	apiMsg := &models.Message{
		MessageID: msg.ID,
		Date:      timestamp,
		Chat: models.Chat{
			ID:    m.chatID,
			Type:  m.mapChatType(m.chatType),
			Title: m.chatName,
		},
		From: &models.User{
			ID:        userID,
			IsBot:     isBot,
			FirstName: userName,
		},
		Text: msg.GetText(),
	}

	// Handle edit date
	if msg.EditedUnixtime != "" {
		editTimestamp, err := strconv.ParseInt(msg.EditedUnixtime, 10, 64)
		if err == nil {
			apiMsg.EditDate = &editTimestamp
		}
	}

	// Handle reply
	if msg.ReplyToMsgID != nil {
		apiMsg.ReplyToMessage = &models.Message{
			MessageID: *msg.ReplyToMsgID,
			Chat: models.Chat{
				ID: m.chatID,
			},
		}
	}

	// Handle text entities
	apiMsg.Entities = m.mapEntities(msg.TextEntities)

	// Handle media
	if msg.Photo != "" {
		apiMsg.Photo = m.mapPhoto(msg)
		// Move text to caption for photo messages
		if apiMsg.Text != "" {
			apiMsg.Caption = apiMsg.Text
			apiMsg.Text = ""
		}
	} else if msg.File != "" {
		m.mapFileMedia(apiMsg, msg)
		// Move text to caption for media messages
		if apiMsg.Text != "" {
			apiMsg.Caption = apiMsg.Text
			apiMsg.Text = ""
		}
	}

	update := &models.Update{
		UpdateID: msg.ID,
		Message:  apiMsg,
	}

	return &UpdateResult{
		Update:   update,
		UserID:   userID,
		IsBot:    isBot,
		UserName: userName,
	}, nil
}

// parseUser extracts user ID and name from export format
func (m *Mapper) parseUser(name, fromID string) (int64, string) {
	// Default to using name as display, or fallback to ID
	displayName := name
	if displayName == "" {
		displayName = fromID
	}

	// Parse user ID from "user123456" or "channel123456" format
	var userID int64
	if strings.HasPrefix(fromID, "user") {
		idStr := strings.TrimPrefix(fromID, "user")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			userID = id
		}
	} else if strings.HasPrefix(fromID, "channel") {
		idStr := strings.TrimPrefix(fromID, "channel")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			userID = id
		}
	}

	// If we couldn't parse an ID but have a fromID string, generate a hash-based ID
	if userID == 0 && fromID != "" {
		userID = m.generateUserID(fromID)
	}

	return userID, displayName
}

// generateUserID creates a deterministic user ID from a string
func (m *Mapper) generateUserID(input string) int64 {
	hash := sha256.Sum256([]byte(input))
	// Use first 8 bytes to create an int64
	var id int64
	for i := 0; i < 8; i++ {
		id = (id << 8) | int64(hash[i])
	}
	// Ensure positive
	if id < 0 {
		id = -id
	}
	return id
}

// mapChatType converts export chat type to API chat type
func (m *Mapper) mapChatType(exportType string) string {
	switch exportType {
	case "private_supergroup":
		return "supergroup"
	case "public_supergroup":
		return "supergroup"
	case "private_group":
		return "group"
	case "public_group":
		return "group"
	case "private_channel":
		return "channel"
	case "public_channel":
		return "channel"
	case "personal_chat":
		return "private"
	default:
		return "supergroup"
	}
}

// mapEntities converts export entities to API entities
func (m *Mapper) mapEntities(entities []models.ExportEntity) []models.MessageEntity {
	if len(entities) == 0 {
		return nil
	}

	var result []models.MessageEntity
	offset := 0

	for _, e := range entities {
		entityType := m.mapEntityType(e.Type)
		if entityType == "" {
			offset += len(e.Text)
			continue
		}

		entity := models.MessageEntity{
			Type:   entityType,
			Offset: offset,
			Length: len(e.Text),
		}

		// Handle mention_name with user_id
		if e.Type == "mention_name" && e.UserID != nil {
			entity.User = &models.User{
				ID:        *e.UserID,
				FirstName: e.Text,
			}
		}

		// Handle links
		if e.Type == "link" {
			entity.URL = e.Text
		}

		result = append(result, entity)
		offset += len(e.Text)
	}

	return result
}

// mapEntityType converts export entity type to API entity type
func (m *Mapper) mapEntityType(exportType string) string {
	switch exportType {
	case "plain":
		return "" // Skip plain text
	case "bold":
		return "bold"
	case "italic":
		return "italic"
	case "underline":
		return "underline"
	case "strikethrough":
		return "strikethrough"
	case "code":
		return "code"
	case "pre":
		return "pre"
	case "link":
		return "url"
	case "mention":
		return "mention"
	case "mention_name":
		return "text_mention"
	case "hashtag":
		return "hashtag"
	case "cashtag":
		return "cashtag"
	case "bot_command":
		return "bot_command"
	case "email":
		return "email"
	case "phone_number":
		return "phone_number"
	case "blockquote":
		return "blockquote"
	default:
		return ""
	}
}

// mapPhoto creates PhotoSize array from export message
func (m *Mapper) mapPhoto(msg *models.ExportMessage) []models.PhotoSize {
	fileID := m.generateFileID(msg.Photo)
	fileSize := msg.PhotoFileSize

	return []models.PhotoSize{
		{
			FileID:       fileID,
			FileUniqueID: fileID,
			Width:        msg.Width,
			Height:       msg.Height,
			FileSize:     &fileSize,
		},
	}
}

// mapFileMedia handles non-photo media types
func (m *Mapper) mapFileMedia(apiMsg *models.Message, msg *models.ExportMessage) {
	fileID := m.generateFileID(msg.File)
	fileSize := msg.FileSize

	switch msg.MediaType {
	case "animation":
		apiMsg.Animation = &models.Animation{
			FileID:       fileID,
			FileUniqueID: fileID,
			Width:        msg.Width,
			Height:       msg.Height,
			Duration:     msg.DurationSeconds,
			FileName:     msg.FileName,
			MimeType:     msg.MimeType,
			FileSize:     &fileSize,
		}
	case "video_file":
		apiMsg.Video = &models.Video{
			FileID:       fileID,
			FileUniqueID: fileID,
			Width:        msg.Width,
			Height:       msg.Height,
			Duration:     msg.DurationSeconds,
			FileName:     msg.FileName,
			MimeType:     msg.MimeType,
			FileSize:     &fileSize,
		}
	case "voice_message":
		apiMsg.Voice = &models.Voice{
			FileID:       fileID,
			FileUniqueID: fileID,
			Duration:     msg.DurationSeconds,
			MimeType:     msg.MimeType,
			FileSize:     &fileSize,
		}
	case "video_message":
		apiMsg.VideoNote = &models.VideoNote{
			FileID:       fileID,
			FileUniqueID: fileID,
			Length:       msg.Width, // Video notes are square
			Duration:     msg.DurationSeconds,
			FileSize:     &fileSize,
		}
	case "audio_file":
		apiMsg.Audio = &models.Audio{
			FileID:       fileID,
			FileUniqueID: fileID,
			Duration:     msg.DurationSeconds,
			FileName:     msg.FileName,
			MimeType:     msg.MimeType,
			FileSize:     &fileSize,
		}
	default:
		// Default to document
		apiMsg.Document = &models.Document{
			FileID:       fileID,
			FileUniqueID: fileID,
			FileName:     msg.FileName,
			MimeType:     msg.MimeType,
			FileSize:     &fileSize,
		}
	}
}

// generateFileID creates a synthetic file ID from a file path
func (m *Mapper) generateFileID(filePath string) string {
	hash := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("import_%x", hash[:16])
}

// ToReactionUpdates converts export reactions to API reaction updates
// Returns both individual user reactions and aggregate count updates
func (m *Mapper) ToReactionUpdates(msg *models.ExportMessage) ([]*models.Update, *models.Update) {
	if len(msg.Reactions) == 0 {
		return nil, nil
	}

	// Parse message timestamp for reactions without dates
	msgTimestamp, _ := strconv.ParseInt(msg.DateUnixtime, 10, 64)

	chat := models.Chat{
		ID:    m.chatID,
		Type:  m.mapChatType(m.chatType),
		Title: m.chatName,
	}

	var userReactions []*models.Update
	var reactionCounts []models.ReactionCount

	for _, reaction := range msg.Reactions {
		// Build reaction count
		reactionCounts = append(reactionCounts, models.ReactionCount{
			Type: models.ReactionInfo{
				Type:  "emoji",
				Emoji: reaction.Emoji,
			},
			TotalCount: reaction.Count,
		})

		// Build individual user reactions from Recent list
		for _, user := range reaction.Recent {
			userID, userName := m.parseUser(user.From, user.FromID)
			isBot := m.botChecker.IsBot(userID)

			// Parse reaction date if available
			reactionDate := msgTimestamp
			if user.Date != "" {
				if parsed, err := strconv.ParseInt(user.Date, 10, 64); err == nil {
					reactionDate = parsed
				}
			}

			userReaction := &models.Update{
				UpdateID: m.generateReactionUpdateID(msg.ID, userID, reaction.Emoji),
				MessageReaction: &models.MessageReactionUpdated{
					Chat:      chat,
					MessageID: msg.ID,
					User: &models.User{
						ID:        userID,
						IsBot:     isBot,
						FirstName: userName,
					},
					Date:        reactionDate,
					OldReaction: []models.ReactionInfo{},
					NewReaction: []models.ReactionInfo{
						{
							Type:  "emoji",
							Emoji: reaction.Emoji,
						},
					},
				},
			}
			userReactions = append(userReactions, userReaction)
		}
	}

	// Build reaction count update
	var countUpdate *models.Update
	if len(reactionCounts) > 0 {
		countUpdate = &models.Update{
			UpdateID: m.generateReactionCountUpdateID(msg.ID),
			MessageReactionCount: &models.MessageReactionCountUpdate{
				Chat:      chat,
				MessageID: msg.ID,
				Date:      msgTimestamp,
				Reactions: reactionCounts,
			},
		}
	}

	return userReactions, countUpdate
}

// generateReactionUpdateID creates a unique update ID for a user reaction
func (m *Mapper) generateReactionUpdateID(msgID, userID int64, emoji string) int64 {
	// Create a deterministic ID based on message, user, and emoji
	input := fmt.Sprintf("reaction:%d:%d:%s", msgID, userID, emoji)
	hash := sha256.Sum256([]byte(input))
	var id int64
	for i := 0; i < 8; i++ {
		id = (id << 8) | int64(hash[i])
	}
	if id < 0 {
		id = -id
	}
	return id
}

// generateReactionCountUpdateID creates a unique update ID for reaction counts
func (m *Mapper) generateReactionCountUpdateID(msgID int64) int64 {
	input := fmt.Sprintf("reaction_count:%d", msgID)
	hash := sha256.Sum256([]byte(input))
	var id int64
	for i := 0; i < 8; i++ {
		id = (id << 8) | int64(hash[i])
	}
	if id < 0 {
		id = -id
	}
	return id
}
