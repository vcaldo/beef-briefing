package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// MLHandler handles HTTP requests for ML analytics endpoints.
type MLHandler struct {
	mlService *services.MLService
	config    *config.Config
}

// NewMLHandler creates a new MLHandler.
func NewMLHandler(mlService *services.MLService, cfg *config.Config) *MLHandler {
	return &MLHandler{
		mlService: mlService,
		config:    cfg,
	}
}

// HandleGetMessages returns unprocessed messages for ML analysis.
// GET /api/v1/ml/messages?limit=500
func (h *MLHandler) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:ml:messages")
	}

	// Parse limit from query params
	limitStr := r.URL.Query().Get("limit")
	limit := 500
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if txn != nil {
		txn.AddAttribute("limit", limit)
	}

	response, err := h.mlService.GetUnprocessedMessages(ctx, limit)
	if err != nil {
		slog.Error("failed to get unprocessed messages", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("messages_count", len(response.Messages))
		txn.AddAttribute("has_more", response.HasMore)
	}

	slog.Debug("returning unprocessed messages", "count", len(response.Messages), "has_more", response.HasMore)

	httputil.RespondJSON(w, response, http.StatusOK)
}

// HandlePostResults saves ML analysis results.
// POST /api/v1/ml/results
func (h *MLHandler) HandlePostResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:ml:results")
	}

	var req services.SaveResultsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request body", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Results) == 0 {
		httputil.RespondError(w, "results array is empty", http.StatusBadRequest)
		return
	}

	if req.ProcessorVersion == "" {
		req.ProcessorVersion = "unknown"
	}

	if txn != nil {
		txn.AddAttribute("results_count", len(req.Results))
		txn.AddAttribute("processor_version", req.ProcessorVersion)
	}

	slog.Info("received ML results", "count", len(req.Results), "version", req.ProcessorVersion)

	if err := h.mlService.SaveResults(ctx, &req); err != nil {
		slog.Error("failed to save ML results", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"status": "ok",
		"saved":  len(req.Results),
	}, http.StatusOK)
}

// HandleGetStatus returns ML processing statistics.
// GET /api/v1/ml/status
func (h *MLHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:ml:status")
	}

	stats, err := h.mlService.GetProcessingStats(ctx)
	if err != nil {
		slog.Error("failed to get processing stats", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("total_with_text", stats.TotalWithText)
		txn.AddAttribute("processed", stats.Processed)
		txn.AddAttribute("unprocessed", stats.Unprocessed)
	}

	httputil.RespondJSON(w, stats, http.StatusOK)
}
