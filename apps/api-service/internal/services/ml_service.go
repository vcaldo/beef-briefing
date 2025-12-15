package services

import (
	"context"
	"database/sql"
	"log/slog"

	"beef-briefing/apps/api-service/internal/repository"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// MLService handles ML analytics business logic.
type MLService struct {
	mlRepo *repository.MLRepository
	nrApp  *newrelic.Application
}

// NewMLService creates a new MLService.
func NewMLService(db *sql.DB, nrApp *newrelic.Application) *MLService {
	return &MLService{
		mlRepo: repository.NewMLRepository(db, nrApp),
		nrApp:  nrApp,
	}
}

// MLMessage represents a message for ML processing.
type MLMessage struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	UserID    *int64 `json:"user_id,omitempty"`
	Text      string `json:"text"`
}

// GetMessagesResponse is the response for GetUnprocessedMessages.
type GetMessagesResponse struct {
	Messages []MLMessage `json:"messages"`
	HasMore  bool        `json:"has_more"`
}

// MLResultRequest represents a single message's ML analysis results.
type MLResultRequest struct {
	MessageID int64            `json:"message_id"`
	ChatID    int64            `json:"chat_id"`
	Sentiment *SentimentInput  `json:"sentiment,omitempty"`
	Toxicity  *ToxicityInput   `json:"toxicity,omitempty"`
}

// SentimentInput represents sentiment analysis input.
type SentimentInput struct {
	Label  string         `json:"label"`
	Scores SentimentScore `json:"scores"`
}

// SentimentScore represents sentiment scores.
type SentimentScore struct {
	Positive float32 `json:"positive"`
	Neutral  float32 `json:"neutral"`
	Negative float32 `json:"negative"`
}

// ToxicityInput represents toxicity detection input.
type ToxicityInput struct {
	IsToxic bool    `json:"is_toxic"`
	Label   string  `json:"label"`
	Score   float32 `json:"score"`
}

// SaveResultsRequest is the request for saving ML results.
type SaveResultsRequest struct {
	Results          []MLResultRequest `json:"results"`
	ProcessorVersion string            `json:"processor_version"`
}

// ProcessingStats contains ML processing statistics.
type ProcessingStats struct {
	TotalWithText    int64 `json:"total_with_text"`
	Processed        int64 `json:"processed"`
	Unprocessed      int64 `json:"unprocessed"`
	SentimentCount   int64 `json:"sentiment_analyzed"`
	ToxicityCount    int64 `json:"toxicity_analyzed"`
	ToxicMessages    int64 `json:"toxic_messages"`
}

// GetUnprocessedMessages fetches messages that haven't been processed by ML.
func (s *MLService) GetUnprocessedMessages(ctx context.Context, limit int) (*GetMessagesResponse, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}

	// Fetch one more than requested to check if there are more
	messages, err := s.mlRepo.GetUnprocessedMessages(ctx, limit+1)
	if err != nil {
		slog.Error("failed to get unprocessed messages", "error", err)
		return nil, err
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	result := make([]MLMessage, len(messages))
	for i, msg := range messages {
		result[i] = MLMessage{
			ID:        msg.ID,
			MessageID: msg.MessageID,
			ChatID:    msg.ChatID,
			UserID:    msg.UserID,
			Text:      msg.Text,
		}
	}

	return &GetMessagesResponse{
		Messages: result,
		HasMore:  hasMore,
	}, nil
}

// SaveResults saves ML analysis results.
func (s *MLService) SaveResults(ctx context.Context, req *SaveResultsRequest) error {
	if len(req.Results) == 0 {
		return nil
	}

	// Collect sentiment results
	var sentimentResults []repository.SentimentResult
	for _, r := range req.Results {
		if r.Sentiment != nil {
			confidence := r.Sentiment.Scores.Positive
			if r.Sentiment.Scores.Neutral > confidence {
				confidence = r.Sentiment.Scores.Neutral
			}
			if r.Sentiment.Scores.Negative > confidence {
				confidence = r.Sentiment.Scores.Negative
			}

			sentimentResults = append(sentimentResults, repository.SentimentResult{
				MessageID:     r.MessageID,
				ChatID:        r.ChatID,
				Label:         r.Sentiment.Label,
				ScorePositive: r.Sentiment.Scores.Positive,
				ScoreNeutral:  r.Sentiment.Scores.Neutral,
				ScoreNegative: r.Sentiment.Scores.Negative,
				Confidence:    confidence,
			})
		}
	}

	// Collect toxicity results
	var toxicityResults []repository.ToxicityResult
	for _, r := range req.Results {
		if r.Toxicity != nil {
			toxicityResults = append(toxicityResults, repository.ToxicityResult{
				MessageID: r.MessageID,
				ChatID:    r.ChatID,
				IsToxic:   r.Toxicity.IsToxic,
				Label:     r.Toxicity.Label,
				Score:     r.Toxicity.Score,
			})
		}
	}

	// Save sentiment results
	if len(sentimentResults) > 0 {
		if err := s.mlRepo.SaveSentimentResults(ctx, sentimentResults); err != nil {
			slog.Error("failed to save sentiment results", "count", len(sentimentResults), "error", err)
			return err
		}
		slog.Info("saved sentiment results", "count", len(sentimentResults))
	}

	// Save toxicity results
	if len(toxicityResults) > 0 {
		if err := s.mlRepo.SaveToxicityResults(ctx, toxicityResults); err != nil {
			slog.Error("failed to save toxicity results", "count", len(toxicityResults), "error", err)
			return err
		}
		slog.Info("saved toxicity results", "count", len(toxicityResults))
	}

	// Mark messages as processed
	messageIDs := make([]int64, len(req.Results))
	chatIDs := make([]int64, len(req.Results))
	for i, r := range req.Results {
		messageIDs[i] = r.MessageID
		chatIDs[i] = r.ChatID
	}

	if err := s.mlRepo.MarkMessagesProcessed(ctx, messageIDs, chatIDs, req.ProcessorVersion); err != nil {
		slog.Error("failed to mark messages as processed", "count", len(messageIDs), "error", err)
		return err
	}

	slog.Info("marked messages as processed", "count", len(messageIDs), "version", req.ProcessorVersion)

	return nil
}

// GetProcessingStats returns ML processing statistics.
func (s *MLService) GetProcessingStats(ctx context.Context) (*ProcessingStats, error) {
	stats, err := s.mlRepo.GetProcessingStats(ctx)
	if err != nil {
		slog.Error("failed to get processing stats", "error", err)
		return nil, err
	}

	return &ProcessingStats{
		TotalWithText:  stats["total_with_text"],
		Processed:      stats["processed"],
		Unprocessed:    stats["unprocessed"],
		SentimentCount: stats["sentiment_analyzed"],
		ToxicityCount:  stats["toxicity_analyzed"],
		ToxicMessages:  stats["toxic_messages"],
	}, nil
}
