package services

import (
	"context"
	"errors"
	"testing"

	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// Test helpers

func newTestMLService(mockRepo *testutil.MockMLRepository) *MLService {
	return NewMLService(nil, nil, &MLServiceDeps{
		MLRepo: mockRepo,
	})
}

// Test fixtures

func newTestUnprocessedMessage(id, messageID, chatID int64, userID *int64, text string) repository.UnprocessedMessage {
	return repository.UnprocessedMessage{
		ID:        id,
		MessageID: messageID,
		ChatID:    chatID,
		UserID:    userID,
		Text:      text,
	}
}

// GetUnprocessedMessages Tests

// TestGetUnprocessedMessages_HappyPath verifies fetching unprocessed messages with default limit.
func TestGetUnprocessedMessages_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup test data
	userID1 := int64(1)
	userID2 := int64(2)
	messages := []repository.UnprocessedMessage{
		newTestUnprocessedMessage(1, 100, -100123, &userID1, "First message"),
		newTestUnprocessedMessage(2, 101, -100123, &userID2, "Second message"),
		newTestUnprocessedMessage(3, 102, -100123, &userID1, "Third message"),
	}
	mockRepo.AddUnprocessedMessages(messages)

	// Execute
	result, err := svc.GetUnprocessedMessages(ctx, 10)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.HasMore {
		t.Fatal("expected HasMore=false, got true")
	}

	// Verify first message
	if result.Messages[0].ID != 1 {
		t.Errorf("expected message ID 1, got %d", result.Messages[0].ID)
	}
	if result.Messages[0].MessageID != 100 {
		t.Errorf("expected messageID 100, got %d", result.Messages[0].MessageID)
	}
	if result.Messages[0].Text != "First message" {
		t.Errorf("expected text 'First message', got %s", result.Messages[0].Text)
	}
}

// TestGetUnprocessedMessages_EmptyResult verifies handling of no unprocessed messages.
func TestGetUnprocessedMessages_EmptyResult(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Execute (no messages added to mock)
	result, err := svc.GetUnprocessedMessages(ctx, 10)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(result.Messages))
	}
	if result.HasMore {
		t.Fatal("expected HasMore=false, got true")
	}
}

// TestGetUnprocessedMessages_Pagination verifies HasMore flag and limit handling.
func TestGetUnprocessedMessages_Pagination(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup test data - 5 messages
	userID := int64(1)
	messages := make([]repository.UnprocessedMessage, 5)
	for i := 0; i < 5; i++ {
		messages[i] = newTestUnprocessedMessage(int64(i+1), int64(100+i), -100123, &userID, "Message")
	}
	mockRepo.AddUnprocessedMessages(messages)

	// Execute with limit=3
	result, err := svc.GetUnprocessedMessages(ctx, 3)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages (limit), got %d", len(result.Messages))
	}
	if !result.HasMore {
		t.Fatal("expected HasMore=true, got false")
	}
}

// TestGetUnprocessedMessages_DefaultLimit verifies default limit=500 when limit=0.
func TestGetUnprocessedMessages_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Execute with limit=0 (should default to 500)
	_, err := svc.GetUnprocessedMessages(ctx, 0)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the repository was called with limit+1=501
	if mockRepo.LastGetUnprocessedLimit != 501 {
		t.Errorf("expected repository called with limit=501 (500+1), got %d", mockRepo.LastGetUnprocessedLimit)
	}
}

// TestGetUnprocessedMessages_MaxLimit verifies limit capped at 1000.
func TestGetUnprocessedMessages_MaxLimit(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Execute with limit=5000 (should cap at 1000)
	_, err := svc.GetUnprocessedMessages(ctx, 5000)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the repository was called with limit+1=1001
	if mockRepo.LastGetUnprocessedLimit != 1001 {
		t.Errorf("expected repository called with limit=1001 (1000+1), got %d", mockRepo.LastGetUnprocessedLimit)
	}
}

// TestGetUnprocessedMessages_RepositoryError verifies error propagation from repository.
func TestGetUnprocessedMessages_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup repository error
	expectedErr := errors.New("database connection failed")
	mockRepo.SetGetUnprocessedError(expectedErr)

	// Execute
	result, err := svc.GetUnprocessedMessages(ctx, 10)

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}

// SaveResults Tests

// TestSaveResults_SentimentSingle verifies saving single sentiment result.
func TestSaveResults_SentimentSingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup request
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label: "positive",
					Scores: SentimentScore{
						Positive: 0.85,
						Neutral:  0.10,
						Negative: 0.05,
					},
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify sentiment results were saved
	if len(mockRepo.SentimentResults) != 1 {
		t.Fatalf("expected 1 sentiment result, got %d", len(mockRepo.SentimentResults))
	}

	result := mockRepo.SentimentResults[0]
	if result.MessageID != 100 {
		t.Errorf("expected messageID 100, got %d", result.MessageID)
	}
	if result.Label != "positive" {
		t.Errorf("expected label 'positive', got %s", result.Label)
	}
	if result.ScorePositive != 0.85 {
		t.Errorf("expected ScorePositive 0.85, got %f", result.ScorePositive)
	}
	// Confidence should be max of all scores (0.85)
	if result.Confidence != 0.85 {
		t.Errorf("expected Confidence 0.85, got %f", result.Confidence)
	}

	// Verify messages were marked as processed
	if len(mockRepo.ProcessedMessageIDs) != 1 {
		t.Fatalf("expected 1 processed message, got %d", len(mockRepo.ProcessedMessageIDs))
	}
	if mockRepo.ProcessedMessageIDs[0] != 100 {
		t.Errorf("expected processed messageID 100, got %d", mockRepo.ProcessedMessageIDs[0])
	}
	if mockRepo.ProcessedVersion != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %s", mockRepo.ProcessedVersion)
	}
}

// TestSaveResults_SentimentBatch verifies saving multiple sentiment results.
func TestSaveResults_SentimentBatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup request with 3 sentiment results
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label:  "positive",
					Scores: SentimentScore{Positive: 0.85, Neutral: 0.10, Negative: 0.05},
				},
			},
			{
				MessageID: 101,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label:  "neutral",
					Scores: SentimentScore{Positive: 0.20, Neutral: 0.60, Negative: 0.20},
				},
			},
			{
				MessageID: 102,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label:  "negative",
					Scores: SentimentScore{Positive: 0.05, Neutral: 0.15, Negative: 0.80},
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify 3 sentiment results were saved
	if len(mockRepo.SentimentResults) != 3 {
		t.Fatalf("expected 3 sentiment results, got %d", len(mockRepo.SentimentResults))
	}

	// Verify confidence calculation (should be max of scores)
	if mockRepo.SentimentResults[0].Confidence != 0.85 {
		t.Errorf("expected Confidence 0.85 for positive, got %f", mockRepo.SentimentResults[0].Confidence)
	}
	if mockRepo.SentimentResults[1].Confidence != 0.60 {
		t.Errorf("expected Confidence 0.60 for neutral, got %f", mockRepo.SentimentResults[1].Confidence)
	}
	if mockRepo.SentimentResults[2].Confidence != 0.80 {
		t.Errorf("expected Confidence 0.80 for negative, got %f", mockRepo.SentimentResults[2].Confidence)
	}
}

// TestSaveResults_ToxicitySingle verifies saving single toxicity result.
func TestSaveResults_ToxicitySingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup request
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Toxicity: &ToxicityInput{
					IsToxic: true,
					Label:   "toxic",
					Score:   0.92,
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify toxicity results were saved
	if len(mockRepo.ToxicityResults) != 1 {
		t.Fatalf("expected 1 toxicity result, got %d", len(mockRepo.ToxicityResults))
	}

	result := mockRepo.ToxicityResults[0]
	if !result.IsToxic {
		t.Error("expected IsToxic=true, got false")
	}
	if result.Label != "toxic" {
		t.Errorf("expected label 'toxic', got %s", result.Label)
	}
	if result.Score != 0.92 {
		t.Errorf("expected Score 0.92, got %f", result.Score)
	}
}

// TestSaveResults_HumorSingle verifies saving single humor result with NULL humor_type.
func TestSaveResults_HumorSingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup request
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Humor: &HumorInput{
					IsHumorous: true,
					HumorType:  "sarcasm",
					Score:      0.78,
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify humor results were saved
	if len(mockRepo.HumorResults) != 1 {
		t.Fatalf("expected 1 humor result, got %d", len(mockRepo.HumorResults))
	}

	result := mockRepo.HumorResults[0]
	if !result.IsHumorous {
		t.Error("expected IsHumorous=true, got false")
	}
	if result.HumorType != "sarcasm" {
		t.Errorf("expected HumorType 'sarcasm', got %s", result.HumorType)
	}
	if result.Score != 0.78 {
		t.Errorf("expected Score 0.78, got %f", result.Score)
	}
}

// TestSaveResults_QuestionSingle verifies saving single question result.
func TestSaveResults_QuestionSingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup request
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Question: &QuestionInput{
					IsQuestion:   true,
					QuestionType: "what",
					Score:        0.95,
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify question results were saved
	if len(mockRepo.QuestionResults) != 1 {
		t.Fatalf("expected 1 question result, got %d", len(mockRepo.QuestionResults))
	}

	result := mockRepo.QuestionResults[0]
	if !result.IsQuestion {
		t.Error("expected IsQuestion=true, got false")
	}
	if result.QuestionType != "what" {
		t.Errorf("expected QuestionType 'what', got %s", result.QuestionType)
	}
	if result.Score != 0.95 {
		t.Errorf("expected Score 0.95, got %f", result.Score)
	}
}

// TestSaveResults_NERSingle verifies saving single NER result.
func TestSaveResults_NERSingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	startPos := 0
	endPos := 7
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Entities: []NEREntity{
					{
						EntityType: "PERSON",
						EntityText: "John Doe",
						StartPos:   &startPos,
						EndPos:     &endPos,
						Confidence: 0.92,
					},
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify NER results were saved
	if len(mockRepo.NERResults) != 1 {
		t.Fatalf("expected 1 NER result, got %d", len(mockRepo.NERResults))
	}

	result := mockRepo.NERResults[0]
	if result.EntityType != "PERSON" {
		t.Errorf("expected EntityType 'PERSON', got %s", result.EntityType)
	}
	if result.EntityText != "John Doe" {
		t.Errorf("expected EntityText 'John Doe', got %s", result.EntityText)
	}
	if *result.StartPos != 0 {
		t.Errorf("expected StartPos 0, got %d", *result.StartPos)
	}
	if *result.EndPos != 7 {
		t.Errorf("expected EndPos 7, got %d", *result.EndPos)
	}
}

// TestSaveResults_NERBatch verifies saving multiple NER results from same message.
func TestSaveResults_NERBatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	pos1Start, pos1End := 0, 7
	pos2Start, pos2End := 15, 22
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Entities: []NEREntity{
					{
						EntityType: "PERSON",
						EntityText: "John Doe",
						StartPos:   &pos1Start,
						EndPos:     &pos1End,
						Confidence: 0.92,
					},
					{
						EntityType: "LOCATION",
						EntityText: "New York",
						StartPos:   &pos2Start,
						EndPos:     &pos2End,
						Confidence: 0.88,
					},
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify 2 NER results were saved
	if len(mockRepo.NERResults) != 2 {
		t.Fatalf("expected 2 NER results, got %d", len(mockRepo.NERResults))
	}
}

// TestSaveResults_TopicSingle verifies saving single topic assignment.
func TestSaveResults_TopicSingle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Topic: &TopicInput{
					TopicID:    5,
					Similarity: 0.87,
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify topic results were saved
	if len(mockRepo.MessageTopics) != 1 {
		t.Fatalf("expected 1 topic result, got %d", len(mockRepo.MessageTopics))
	}

	result := mockRepo.MessageTopics[0]
	if result.TopicID != 5 {
		t.Errorf("expected TopicID 5, got %d", result.TopicID)
	}
	if result.Similarity != 0.87 {
		t.Errorf("expected Similarity 0.87, got %f", result.Similarity)
	}
}

// TestSaveResults_MixedAnalyzers verifies saving results from multiple analyzers for same message.
func TestSaveResults_MixedAnalyzers(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	startPos, endPos := 0, 7
	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label:  "positive",
					Scores: SentimentScore{Positive: 0.85, Neutral: 0.10, Negative: 0.05},
				},
				Toxicity: &ToxicityInput{
					IsToxic: false,
					Label:   "safe",
					Score:   0.05,
				},
				Humor: &HumorInput{
					IsHumorous: true,
					HumorType:  "wordplay",
					Score:      0.72,
				},
				Question: &QuestionInput{
					IsQuestion:   false,
					QuestionType: "",
					Score:        0.15,
				},
				Entities: []NEREntity{
					{
						EntityType: "PERSON",
						EntityText: "Alice",
						StartPos:   &startPos,
						EndPos:     &endPos,
						Confidence: 0.90,
					},
				},
				Topic: &TopicInput{
					TopicID:    3,
					Similarity: 0.82,
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify all analyzer results were saved
	if len(mockRepo.SentimentResults) != 1 {
		t.Errorf("expected 1 sentiment result, got %d", len(mockRepo.SentimentResults))
	}
	if len(mockRepo.ToxicityResults) != 1 {
		t.Errorf("expected 1 toxicity result, got %d", len(mockRepo.ToxicityResults))
	}
	if len(mockRepo.HumorResults) != 1 {
		t.Errorf("expected 1 humor result, got %d", len(mockRepo.HumorResults))
	}
	if len(mockRepo.QuestionResults) != 1 {
		t.Errorf("expected 1 question result, got %d", len(mockRepo.QuestionResults))
	}
	if len(mockRepo.NERResults) != 1 {
		t.Errorf("expected 1 NER result, got %d", len(mockRepo.NERResults))
	}
	if len(mockRepo.MessageTopics) != 1 {
		t.Errorf("expected 1 topic result, got %d", len(mockRepo.MessageTopics))
	}

	// Verify message marked as processed
	if len(mockRepo.ProcessedMessageIDs) != 1 {
		t.Fatalf("expected 1 processed message, got %d", len(mockRepo.ProcessedMessageIDs))
	}
}

// TestSaveResults_EmptyResults verifies handling of empty results array.
func TestSaveResults_EmptyResults(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results:          []MLResultRequest{},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify (should succeed without calling repository)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify no results were saved
	if len(mockRepo.SentimentResults) != 0 {
		t.Errorf("expected 0 sentiment results, got %d", len(mockRepo.SentimentResults))
	}
	if len(mockRepo.ProcessedMessageIDs) != 0 {
		t.Errorf("expected 0 processed messages, got %d", len(mockRepo.ProcessedMessageIDs))
	}
}

// TestSaveResults_RepositoryError verifies error propagation from repository.
func TestSaveResults_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup repository error for sentiment
	expectedErr := errors.New("database write failed")
	mockRepo.SetSaveSentimentError(expectedErr)

	req := &SaveResultsRequest{
		ProcessorVersion: "v1.0.0",
		Results: []MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -100123,
				Sentiment: &SentimentInput{
					Label:  "positive",
					Scores: SentimentScore{Positive: 0.85, Neutral: 0.10, Negative: 0.05},
				},
			},
		},
	}

	// Execute
	err := svc.SaveResults(ctx, req)

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// GetProcessingStats Tests

// TestGetProcessingStats_Success verifies fetching processing stats.
func TestGetProcessingStats_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup mock stats
	stats := map[string]int64{
		"total_with_text":    1000,
		"processed":          750,
		"unprocessed":        250,
		"sentiment_analyzed": 700,
		"toxicity_analyzed":  680,
		"toxic_messages":     45,
		"humor_analyzed":     650,
		"questions_analyzed": 120,
		"entities_extracted": 320,
		"topics_assigned":    600,
	}
	mockRepo.SetProcessingStats(stats)

	// Execute
	result, err := svc.GetProcessingStats(ctx)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.TotalWithText != 1000 {
		t.Errorf("expected TotalWithText 1000, got %d", result.TotalWithText)
	}
	if result.Processed != 750 {
		t.Errorf("expected Processed 750, got %d", result.Processed)
	}
	if result.Unprocessed != 250 {
		t.Errorf("expected Unprocessed 250, got %d", result.Unprocessed)
	}
	if result.SentimentCount != 700 {
		t.Errorf("expected SentimentCount 700, got %d", result.SentimentCount)
	}
	if result.ToxicityCount != 680 {
		t.Errorf("expected ToxicityCount 680, got %d", result.ToxicityCount)
	}
	if result.ToxicMessages != 45 {
		t.Errorf("expected ToxicMessages 45, got %d", result.ToxicMessages)
	}
	if result.HumorCount != 650 {
		t.Errorf("expected HumorCount 650, got %d", result.HumorCount)
	}
	if result.QuestionCount != 120 {
		t.Errorf("expected QuestionCount 120, got %d", result.QuestionCount)
	}
	if result.NERCount != 320 {
		t.Errorf("expected NERCount 320, got %d", result.NERCount)
	}
	if result.TopicCount != 600 {
		t.Errorf("expected TopicCount 600, got %d", result.TopicCount)
	}
}

// TestGetProcessingStats_EmptyStats verifies handling of empty stats (all zeros).
func TestGetProcessingStats_EmptyStats(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup empty stats
	stats := map[string]int64{
		"total_with_text":    0,
		"processed":          0,
		"unprocessed":        0,
		"sentiment_analyzed": 0,
		"toxicity_analyzed":  0,
		"toxic_messages":     0,
		"humor_analyzed":     0,
		"questions_analyzed": 0,
		"entities_extracted": 0,
		"topics_assigned":    0,
	}
	mockRepo.SetProcessingStats(stats)

	// Execute
	result, err := svc.GetProcessingStats(ctx)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// All values should be zero
	if result.TotalWithText != 0 || result.Processed != 0 || result.Unprocessed != 0 {
		t.Error("expected all stats to be zero")
	}
}

// TestGetProcessingStats_RepositoryError verifies error propagation from repository.
func TestGetProcessingStats_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMLRepository()
	svc := newTestMLService(mockRepo)

	// Setup repository error
	expectedErr := errors.New("database query failed")
	mockRepo.SetGetStatsError(expectedErr)

	// Execute
	result, err := svc.GetProcessingStats(ctx)

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}
