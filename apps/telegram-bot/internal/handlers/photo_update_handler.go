package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"beef-briefing/apps/telegram-bot/internal"
	"beef-briefing/apps/telegram-bot/internal/client"
	"beef-briefing/pkg/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// PhotoUpdateHandler handles the /update_photos command
type PhotoUpdateHandler struct {
	apiClient  *client.APIClient
	httpClient *http.Client
	nrApp      *newrelic.Application
	config     *config.Config
}

// NewPhotoUpdateHandler creates a new PhotoUpdateHandler
func NewPhotoUpdateHandler(apiClient *client.APIClient, cfg *config.Config, nrApp *newrelic.Application) *PhotoUpdateHandler {
	return &PhotoUpdateHandler{
		apiClient: apiClient,
		httpClient: &http.Client{
			Timeout: internal.FileDownloadTimeout,
		},
		nrApp:  nrApp,
		config: cfg,
	}
}

// Handle processes the /update_photos command
func (h *PhotoUpdateHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	slog.Info("received /update_photos command", "chat_id", chatID, "user_id", userID)

	// Check if the user is an admin
	if !h.config.IsAdmin(userID) {
		slog.Warn("unauthorized /update_photos attempt", "user_id", userID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "You are not authorized to use this command.",
		})
		return
	}

	// Send initial status message
	statusMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Starting photo update... This may take a while.",
	})
	if err != nil {
		slog.Error("failed to send status message", "error", err)
	}

	// Fetch all user and chat IDs
	userIDs, err := h.apiClient.GetAllUsers(ctx)
	if err != nil {
		slog.Error("failed to get user IDs", "error", err)
		h.sendError(ctx, b, chatID, "Failed to get user list")
		return
	}

	chatIDs, err := h.apiClient.GetAllChats(ctx)
	if err != nil {
		slog.Error("failed to get chat IDs", "error", err)
		h.sendError(ctx, b, chatID, "Failed to get chat list")
		return
	}

	slog.Info("fetching photos", "users", len(userIDs), "chats", len(chatIDs))

	// Update status
	if statusMsg != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
			Text:      fmt.Sprintf("Fetching photos for %d users and %d chats...", len(userIDs), len(chatIDs)),
		})
	}

	var usersUpdated, chatsUpdated int
	var usersSkipped, chatsSkipped int

	// Process chat photos first (usually fewer chats than users)
	for _, cid := range chatIDs {
		success, err := h.fetchAndUploadChatPhoto(ctx, b, cid)
		if err != nil {
			slog.Warn("failed to fetch chat photo", "chat_id", cid, "error", err)
			chatsSkipped++
			continue
		}
		if success {
			chatsUpdated++
		} else {
			chatsSkipped++
		}
	}

	// Process user photos
	for _, uid := range userIDs {
		success, err := h.fetchAndUploadUserPhoto(ctx, b, uid)
		if err != nil {
			slog.Warn("failed to fetch user photo", "user_id", uid, "error", err)
			usersSkipped++
			continue
		}
		if success {
			usersUpdated++
		} else {
			usersSkipped++
		}
	}

	// Send completion message
	completionMsg := fmt.Sprintf(
		"Photo update complete!\n\nUsers: %d updated, %d skipped\nChats: %d updated, %d skipped",
		usersUpdated, usersSkipped, chatsUpdated, chatsSkipped,
	)

	if statusMsg != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
			Text:      completionMsg,
		})
	} else {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   completionMsg,
		})
	}

	slog.Info("photo update complete",
		"users_updated", usersUpdated,
		"users_skipped", usersSkipped,
		"chats_updated", chatsUpdated,
		"chats_skipped", chatsSkipped,
	)
}

// fetchAndUploadUserPhoto fetches user profile photos and uploads them to API
func (h *PhotoUpdateHandler) fetchAndUploadUserPhoto(ctx context.Context, b *bot.Bot, userID int64) (bool, error) {
	// Get user profile photos
	photos, err := b.GetUserProfilePhotos(ctx, &bot.GetUserProfilePhotosParams{
		UserID: userID,
		Limit:  1, // Get only the most recent photo
	})
	if err != nil {
		return false, fmt.Errorf("failed to get user profile photos: %w", err)
	}

	if photos.TotalCount == 0 || len(photos.Photos) == 0 {
		slog.Debug("user has no profile photo", "user_id", userID)
		return false, nil
	}

	// Get all sizes of the first (most recent) photo
	photoSizes := photos.Photos[0]
	if len(photoSizes) == 0 {
		return false, nil
	}

	// Download all sizes
	var photoMetas []client.ProfilePhotoMeta
	files := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ps := range photoSizes {
		wg.Add(1)
		go func(photoSize models.PhotoSize) {
			defer wg.Done()

			data, err := h.downloadFile(ctx, b, photoSize.FileID)
			if err != nil {
				slog.Warn("failed to download user photo", "user_id", userID, "file_id", photoSize.FileID, "error", err)
				return
			}

			mu.Lock()
			files[photoSize.FileID] = data
			photoMetas = append(photoMetas, client.ProfilePhotoMeta{
				FileID:       photoSize.FileID,
				FileUniqueID: photoSize.FileUniqueID,
				Width:        photoSize.Width,
				Height:       photoSize.Height,
				FileSize:     int64PtrFromInt(photoSize.FileSize),
			})
			mu.Unlock()
		}(ps)
	}

	wg.Wait()

	if len(files) == 0 {
		return false, fmt.Errorf("failed to download any photos")
	}

	// Upload to API
	if err := h.apiClient.SendUserProfilePhotos(ctx, userID, photoMetas, files); err != nil {
		return false, fmt.Errorf("failed to upload user photos: %w", err)
	}

	slog.Debug("uploaded user profile photos", "user_id", userID, "count", len(files))
	return true, nil
}

// fetchAndUploadChatPhoto fetches chat profile photo and uploads to API
func (h *PhotoUpdateHandler) fetchAndUploadChatPhoto(ctx context.Context, b *bot.Bot, chatID int64) (bool, error) {
	// Get chat info (includes photo)
	chat, err := b.GetChat(ctx, &bot.GetChatParams{
		ChatID: chatID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get chat: %w", err)
	}

	if chat.Photo == nil {
		slog.Debug("chat has no photo", "chat_id", chatID)
		return false, nil
	}

	// Download both sizes
	var photoMetas []client.ProfilePhotoMeta
	files := make(map[string][]byte)

	// Small photo (160x160)
	if chat.Photo.SmallFileID != "" {
		data, err := h.downloadFile(ctx, b, chat.Photo.SmallFileID)
		if err != nil {
			slog.Warn("failed to download small chat photo", "chat_id", chatID, "error", err)
		} else {
			files[chat.Photo.SmallFileID] = data
			photoMetas = append(photoMetas, client.ProfilePhotoMeta{
				FileID:       chat.Photo.SmallFileID,
				FileUniqueID: chat.Photo.SmallFileUniqueID,
				Width:        160,
				Height:       160,
				PhotoSize:    "small",
			})
		}
	}

	// Big photo (640x640)
	if chat.Photo.BigFileID != "" {
		data, err := h.downloadFile(ctx, b, chat.Photo.BigFileID)
		if err != nil {
			slog.Warn("failed to download big chat photo", "chat_id", chatID, "error", err)
		} else {
			files[chat.Photo.BigFileID] = data
			photoMetas = append(photoMetas, client.ProfilePhotoMeta{
				FileID:       chat.Photo.BigFileID,
				FileUniqueID: chat.Photo.BigFileUniqueID,
				Width:        640,
				Height:       640,
				PhotoSize:    "big",
			})
		}
	}

	if len(files) == 0 {
		return false, fmt.Errorf("failed to download any chat photos")
	}

	// Upload to API
	if err := h.apiClient.SendChatProfilePhotos(ctx, chatID, photoMetas, files); err != nil {
		return false, fmt.Errorf("failed to upload chat photos: %w", err)
	}

	slog.Debug("uploaded chat profile photos", "chat_id", chatID, "count", len(files))
	return true, nil
}

// downloadFile downloads a file from Telegram
func (h *PhotoUpdateHandler) downloadFile(ctx context.Context, b *bot.Bot, fileID string) ([]byte, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileURL := b.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	txn := newrelic.FromContext(ctx)
	var externalSegment *newrelic.ExternalSegment
	if txn != nil {
		parsedURL, _ := url.Parse(fileURL)
		host := "api.telegram.org"
		if parsedURL != nil {
			host = parsedURL.Host
		}
		externalSegment = &newrelic.ExternalSegment{
			StartTime: txn.StartSegmentNow(),
			URL:       fileURL,
			Host:      host,
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := h.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Check file size
	if resp.ContentLength > internal.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes", resp.ContentLength)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

func (h *PhotoUpdateHandler) sendError(ctx context.Context, b *bot.Bot, chatID int64, msg string) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Error: " + msg,
	})
}

func int64PtrFromInt(i int) *int64 {
	if i == 0 {
		return nil
	}
	v := int64(i)
	return &v
}
