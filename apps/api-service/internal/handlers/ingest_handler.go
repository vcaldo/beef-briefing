package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// IngestHandler handles HTTP requests for the ingest endpoint.
// It is responsible only for HTTP concerns: parsing requests, extracting files,
// and writing responses. Business logic is delegated to IngestService.
type IngestHandler struct {
	ingestService *services.IngestService
	config        *config.Config
}

// NewIngestHandler creates a new IngestHandler with the given dependencies.
func NewIngestHandler(ingestService *services.IngestService, cfg *config.Config) *IngestHandler {
	return &IngestHandler{
		ingestService: ingestService,
		config:        cfg,
	}
}

// HandleIngest processes incoming multipart ingestion requests.
func (h *IngestHandler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	updateJSON := r.FormValue("update")
	if updateJSON == "" {
		slog.Error("missing update field in request")
		httputil.RespondError(w, "missing update field", http.StatusBadRequest)
		return
	}

	var update models.Update
	if err := json.Unmarshal([]byte(updateJSON), &update); err != nil {
		slog.Error("failed to decode update", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid update JSON", http.StatusBadRequest)
		return
	}

	// Add custom attributes for debugging and filtering
	if txn != nil {
		txn.AddAttribute("update_id", update.UpdateID)
		if update.Message != nil {
			txn.AddAttribute("chat_id", update.Message.Chat.ID)
			txn.AddAttribute("update_type", "message")
		} else if update.EditedMessage != nil {
			txn.AddAttribute("chat_id", update.EditedMessage.Chat.ID)
			txn.AddAttribute("update_type", "edited_message")
		} else if update.MessageReaction != nil {
			txn.AddAttribute("chat_id", update.MessageReaction.Chat.ID)
			txn.AddAttribute("update_type", "message_reaction")
		} else if update.MessageReactionCount != nil {
			txn.AddAttribute("chat_id", update.MessageReactionCount.Chat.ID)
			txn.AddAttribute("update_type", "message_reaction_count")
		} else if update.MyChatMember != nil {
			txn.AddAttribute("chat_id", update.MyChatMember.Chat.ID)
			txn.AddAttribute("update_type", "my_chat_member")
		}
	}

	slog.Info("received update", "update_id", update.UpdateID)

	files := h.extractFiles(r)
	if txn != nil {
		txn.AddAttribute("files_count", len(files))
	}

	if err := h.ingestService.ProcessUpdate(ctx, &update, files); err != nil {
		slog.Error("failed to process update",
			"update_id", update.UpdateID,
			"error", err,
		)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondOK(w)
}

// extractFiles reads all uploaded files from the multipart form.
func (h *IngestHandler) extractFiles(r *http.Request) map[string][]byte {
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
