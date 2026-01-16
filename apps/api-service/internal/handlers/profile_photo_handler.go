package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// ProfilePhotoHandler handles HTTP requests for profile photo endpoints.
type ProfilePhotoHandler struct {
	profilePhotoService *services.ProfilePhotoService
	config              *config.Config
}

// PhotoURLResponse is the JSON response for photo URL endpoints.
type PhotoURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

// NewProfilePhotoHandler creates a new ProfilePhotoHandler.
func NewProfilePhotoHandler(profilePhotoService *services.ProfilePhotoService, cfg *config.Config) *ProfilePhotoHandler {
	return &ProfilePhotoHandler{
		profilePhotoService: profilePhotoService,
		config:              cfg,
	}
}

// HandleUserPhotos processes user profile photo uploads.
// POST /api/v1/profile-photos/user
// Multipart form with:
// - metadata: JSON with UserProfilePhotosRequest
// - files: binary photo data keyed by file_id
func (h *ProfilePhotoHandler) HandleUserPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:photos:user-upload")
	}

	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	metadataJSON := r.FormValue("metadata")
	if metadataJSON == "" {
		slog.Error("missing metadata field in request")
		httputil.RespondError(w, "missing metadata field", http.StatusBadRequest)
		return
	}

	var req models.UserProfilePhotosRequest
	if err := jsonutil.Unmarshal([]byte(metadataJSON), &req); err != nil {
		slog.Error("failed to decode metadata", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid metadata JSON", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("user_id", req.UserID)
		txn.AddAttribute("photos_count", len(req.Photos))
	}

	slog.Info("received user profile photos", "user_id", req.UserID, "photos_count", len(req.Photos))

	files := h.extractFiles(r)
	if txn != nil {
		txn.AddAttribute("files_count", len(files))
	}

	if err := h.profilePhotoService.ProcessUserPhotos(ctx, req.UserID, req.Photos, files); err != nil {
		slog.Error("failed to process user photos", "user_id", req.UserID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondOK(w)
}

// HandleChatPhotos processes chat profile photo uploads.
// POST /api/v1/profile-photos/chat
// Multipart form with:
// - metadata: JSON with ChatProfilePhotosRequest
// - files: binary photo data keyed by file_id
func (h *ProfilePhotoHandler) HandleChatPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:photos:chat-upload")
	}

	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	metadataJSON := r.FormValue("metadata")
	if metadataJSON == "" {
		slog.Error("missing metadata field in request")
		httputil.RespondError(w, "missing metadata field", http.StatusBadRequest)
		return
	}

	var req models.ChatProfilePhotosRequest
	if err := jsonutil.Unmarshal([]byte(metadataJSON), &req); err != nil {
		slog.Error("failed to decode metadata", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid metadata JSON", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 {
		httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("photos_count", len(req.Photos))
	}

	slog.Info("received chat profile photos", "chat_id", req.ChatID, "photos_count", len(req.Photos))

	files := h.extractFiles(r)
	if txn != nil {
		txn.AddAttribute("files_count", len(files))
	}

	if err := h.profilePhotoService.ProcessChatPhotos(ctx, req.ChatID, req.Photos, files); err != nil {
		slog.Error("failed to process chat photos", "chat_id", req.ChatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondOK(w)
}

// HandleGetUsers returns all user IDs from the database.
// GET /api/v1/users
func (h *ProfilePhotoHandler) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:users:list")
	}

	userIDs, err := h.profilePhotoService.GetAllUserIDs(ctx)
	if err != nil {
		slog.Error("failed to get user IDs", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("users_count", len(userIDs))
	}

	httputil.RespondJSON(w, map[string][]int64{"user_ids": userIDs}, http.StatusOK)
}

// HandleGetChats returns all chat IDs from the database.
// GET /api/v1/chats
func (h *ProfilePhotoHandler) HandleGetChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:chats:list")
	}

	chatIDs, err := h.profilePhotoService.GetAllChatIDs(ctx)
	if err != nil {
		slog.Error("failed to get chat IDs", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("chats_count", len(chatIDs))
	}

	httputil.RespondJSON(w, map[string][]int64{"chat_ids": chatIDs}, http.StatusOK)
}

// extractFiles reads all uploaded files from the multipart form.
func (h *ProfilePhotoHandler) extractFiles(r *http.Request) map[string][]byte {
	files := make(map[string][]byte)

	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return files
	}

	for fileID, fileHeaders := range r.MultipartForm.File {
		if len(fileHeaders) == 0 {
			continue
		}

		file, err := fileHeaders[0].Open()
		if err != nil {
			slog.Warn("failed to open uploaded file", "file_id", fileID, "error", err)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			slog.Warn("failed to read uploaded file", "file_id", fileID, "error", err)
			continue
		}

		files[fileID] = data
		slog.Debug("extracted file from request", "file_id", fileID, "size", len(data))
	}

	return files
}

// validSizes contains the allowed size parameter values.
var validSizes = map[string]bool{
	"small":  true,
	"medium": true,
	"large":  true,
}

// HandleGetUserPhoto serves a user's profile photo.
// GET /api/v1/users/{id}/photo?size=small|medium|large
func (h *ProfilePhotoHandler) HandleGetUserPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:photos:user")
	}

	// Extract user ID from path
	vars := mux.Vars(r)
	idStr := vars["id"]
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("invalid user ID", "id", idStr, "error", err)
		httputil.RespondError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	// Get size parameter, default to "large"
	size := r.URL.Query().Get("size")
	if size == "" {
		size = "large"
	}

	// Validate size parameter
	if !validSizes[size] {
		slog.Warn("invalid size parameter", "size", size)
		httputil.RespondError(w, "invalid size parameter, must be small, medium, or large", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("user_id", userID)
		txn.AddAttribute("size", size)
	}

	// Get photo URL from service
	url, err := h.profilePhotoService.GetUserPhoto(ctx, userID, size)
	if err != nil {
		if errors.Is(err, apperror.ErrPhotoNotFound) {
			slog.Debug("user photo not found", "user_id", userID, "size", size)
			httputil.RespondError(w, "photo not found", http.StatusNotFound)
			return
		}

		slog.Error("failed to get user photo", "user_id", userID, "size", size, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, PhotoURLResponse{
		URL:       url,
		ExpiresIn: 3600,
	}, http.StatusOK)

	slog.Debug("served user photo URL", "user_id", userID, "size", size)
}

// HandleGetChatPhoto serves a chat's profile photo.
// GET /api/v1/chats/{id}/photo?size=small|medium|large
func (h *ProfilePhotoHandler) HandleGetChatPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:photos:chat")
	}

	// Extract chat ID from path
	vars := mux.Vars(r)
	idStr := vars["id"]
	chatID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("invalid chat ID", "id", idStr, "error", err)
		httputil.RespondError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Get size parameter, default to "large"
	size := r.URL.Query().Get("size")
	if size == "" {
		size = "large"
	}

	// Validate size parameter
	if !validSizes[size] {
		slog.Warn("invalid size parameter", "size", size)
		httputil.RespondError(w, "invalid size parameter, must be small, medium, or large", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("size", size)
	}

	// Get photo URL from service
	url, err := h.profilePhotoService.GetChatPhoto(ctx, chatID, size)
	if err != nil {
		if errors.Is(err, apperror.ErrPhotoNotFound) {
			slog.Debug("chat photo not found", "chat_id", chatID, "size", size)
			httputil.RespondError(w, "photo not found", http.StatusNotFound)
			return
		}

		slog.Error("failed to get chat photo", "chat_id", chatID, "size", size, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, PhotoURLResponse{
		URL:       url,
		ExpiresIn: 3600,
	}, http.StatusOK)

	slog.Debug("served chat photo URL", "chat_id", chatID, "size", size)
}
