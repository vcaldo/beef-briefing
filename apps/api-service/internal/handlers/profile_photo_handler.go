package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// ProfilePhotoHandler handles HTTP requests for profile photo endpoints.
type ProfilePhotoHandler struct {
	profilePhotoService *services.ProfilePhotoService
	config              *config.Config
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

	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	metadataJSON := r.FormValue("metadata")
	if metadataJSON == "" {
		slog.Error("missing metadata field in request")
		http.Error(w, "missing metadata field", http.StatusBadRequest)
		return
	}

	var req models.UserProfilePhotosRequest
	if err := json.Unmarshal([]byte(metadataJSON), &req); err != nil {
		slog.Error("failed to decode metadata", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid metadata JSON", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleChatPhotos processes chat profile photo uploads.
// POST /api/v1/profile-photos/chat
// Multipart form with:
// - metadata: JSON with ChatProfilePhotosRequest
// - files: binary photo data keyed by file_id
func (h *ProfilePhotoHandler) HandleChatPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	metadataJSON := r.FormValue("metadata")
	if metadataJSON == "" {
		slog.Error("missing metadata field in request")
		http.Error(w, "missing metadata field", http.StatusBadRequest)
		return
	}

	var req models.ChatProfilePhotosRequest
	if err := json.Unmarshal([]byte(metadataJSON), &req); err != nil {
		slog.Error("failed to decode metadata", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid metadata JSON", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 {
		http.Error(w, "chat_id is required", http.StatusBadRequest)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleGetUsers returns all user IDs from the database.
// GET /api/v1/users
func (h *ProfilePhotoHandler) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	userIDs, err := h.profilePhotoService.GetAllUserIDs(ctx)
	if err != nil {
		slog.Error("failed to get user IDs", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("users_count", len(userIDs))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]int64{"user_ids": userIDs})
}

// HandleGetChats returns all chat IDs from the database.
// GET /api/v1/chats
func (h *ProfilePhotoHandler) HandleGetChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatIDs, err := h.profilePhotoService.GetAllChatIDs(ctx)
	if err != nil {
		slog.Error("failed to get chat IDs", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("chats_count", len(chatIDs))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]int64{"chat_ids": chatIDs})
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
