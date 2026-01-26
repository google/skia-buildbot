package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/metrics2"
	genai_mocks "go.skia.org/infra/rag/go/genai/mocks"
	"go.skia.org/infra/rag/go/topicstore"
	"go.skia.org/infra/rag/go/topicstore/mocks"
	pb "go.skia.org/infra/rag/proto/history/v1"
)

func TestApiService_GetSummary(t *testing.T) {
	mockGenAI := genai_mocks.NewGenAIClient(t)
	mockTopicStore := mocks.NewTopicStore(t)
	service := &ApiService{
		genAiClient:             mockGenAI,
		topicStore:              mockTopicStore,
		summaryModel:            "test-summary-model",
		getSummaryCounterMetric: metrics2.GetCounter("test"), // We don't need metrics for this test.
	}

	ctx := context.Background()
	topicID := int64(123)
	query := "test query"

	mockTopic := &topicstore.Topic{
		ID:      topicID,
		Title:   "Test Topic",
		Summary: "Test Topic Summary",
		CodeContext: `File1.go
content1

File2.go
content2`,
	}

	mockTopicStore.On("ReadTopic", mock.Anything, topicID).Return(mockTopic, nil)
	mockGenAI.On("GetSummary", mock.Anything, "test-summary-model", mock.MatchedBy(func(input string) bool {
		return assert.Contains(t, input, query) &&
			assert.Contains(t, input, mockTopic.Title) &&
			assert.Contains(t, input, mockTopic.Summary) &&
			assert.Contains(t, input, "content1")
	})).Return("Final LLM Summary", nil)

	req := &pb.GetSummaryRequest{
		Query:    query,
		TopicIds: []int64{topicID},
	}

	resp, err := service.GetSummary(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Final LLM Summary", resp.Summary)

	mockTopicStore.AssertExpectations(t)
	mockGenAI.AssertExpectations(t)
}

func TestApiService_GetSummary_EmptyQuery(t *testing.T) {
	service := &ApiService{}
	ctx := context.Background()
	req := &pb.GetSummaryRequest{
		Query:    "",
		TopicIds: []int64{123},
	}
	_, err := service.GetSummary(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query cannot be empty")
}

func TestApiService_GetSummary_NoTopics(t *testing.T) {
	service := &ApiService{}
	ctx := context.Background()
	req := &pb.GetSummaryRequest{
		Query:    "test",
		TopicIds: []int64{},
	}
	_, err := service.GetSummary(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topicIds cannot be empty")
}
