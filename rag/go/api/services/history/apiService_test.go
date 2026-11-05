// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

	mockTopicStore.On("ReadTopic", mock.Anything, topicID, mock.Anything).Return(mockTopic, nil)
	mockGenAI.On("GetSummary", mock.Anything, "test-summary-model", mock.MatchedBy(func(input string) bool {
		return assert.Contains(t, input, query) &&
			assert.Contains(t, input, mockTopic.Title) &&
			assert.Contains(t, input, mockTopic.Summary) &&
			assert.Contains(t, input, "content1")
	})).Return("Final LLM Summary", nil)

	req := &pb.GetSummaryRequest{
		Query: query,
		Topics: []*pb.GetSummaryRequest_TopicRequest{
			{
				TopicId:    topicID,
				Repository: "test-repo",
			},
		},
	}

	resp, err := service.GetSummary(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Final LLM Summary", resp.Summary)

	mockTopicStore.AssertExpectations(t)
	mockGenAI.AssertExpectations(t)
}

func TestApiService_GetTopics(t *testing.T) {
	mockGenAI := genai_mocks.NewGenAIClient(t)
	mockTopicStore := mocks.NewTopicStore(t)
	service := &ApiService{
		genAiClient:            mockGenAI,
		topicStore:             mockTopicStore,
		queryEmbeddingModel:    "test-emb-model",
		dimensionality:         768,
		getTopicsCounterMetric: metrics2.GetCounter("test"),
	}

	ctx := context.Background()
	query := "test query"
	repo := "test-repo"

	mockEmbedding := []float32{0.1, 0.2, 0.3}
	mockGenAI.On("GetEmbedding", mock.Anything, "test-emb-model", int32(768), query).Return(mockEmbedding, nil)

	mockFoundTopics := []*topicstore.FoundTopic{
		{
			ID:         123,
			Title:      "Test Topic",
			Summary:    "Summary",
			Repository: repo,
		},
	}
	mockTopicStore.On("SearchTopics", mock.Anything, mockEmbedding, defaultTopicCount, repo).Return(mockFoundTopics, nil)

	req := &pb.GetTopicsRequest{
		Query:      query,
		Repository: repo,
	}

	resp, err := service.GetTopics(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Topics, 1)
	assert.Equal(t, int64(123), resp.Topics[0].TopicId)
	assert.Equal(t, repo, resp.Topics[0].Repository)

	mockTopicStore.AssertExpectations(t)
	mockGenAI.AssertExpectations(t)
}

func TestApiService_GetTopicDetails(t *testing.T) {
	mockTopicStore := mocks.NewTopicStore(t)
	service := &ApiService{
		topicStore:                   mockTopicStore,
		getTopicDetailsCounterMetric: metrics2.GetCounter("test"),
	}

	ctx := context.Background()
	topicID := int64(123)
	repo := "test-repo"

	mockTopic := &topicstore.Topic{
		ID:          topicID,
		Title:       "Test Topic",
		Summary:     "Summary",
		CodeContext: "file.go\ncontent",
	}
	mockTopicStore.On("ReadTopic", mock.Anything, topicID, repo).Return(mockTopic, nil)

	req := &pb.GetTopicDetailsRequest{
		TopicIds:    []int64{topicID},
		Repository:  repo,
		IncludeCode: true,
	}

	resp, err := service.GetTopicDetails(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Topics, 1)
	assert.Equal(t, int64(123), resp.Topics[0].TopicId)
	assert.Len(t, resp.Topics[0].CodeChunks, 1)

	mockTopicStore.AssertExpectations(t)
}

func TestApiService_GetRepositories(t *testing.T) {
	mockTopicStore := mocks.NewTopicStore(t)
	repoPaths := map[string]string{
		"repo1": "path/to/repo1",
	}
	service := &ApiService{
		topicStore: mockTopicStore,
		repoPaths:  repoPaths,
	}

	ctx := context.Background()
	repos := []string{"repo1", "repo2"}
	mockTopicStore.On("GetRepositories", mock.Anything).Return(repos, nil)

	resp, err := service.GetRepositories(ctx, &pb.GetRepositoriesRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Repositories, 2)
	assert.Equal(t, "path/to/repo1", resp.Repositories[0])
	assert.Equal(t, "repo2", resp.Repositories[1])

	mockTopicStore.AssertExpectations(t)
}

func TestApiService_AdjustCodePaths(t *testing.T) {
	repoPaths := map[string]string{
		"repo1": "path/to/repo1",
	}
	service := &ApiService{
		repoPaths: repoPaths,
	}

	codeContext := "file1.go\ncontent1\n\nfile2.go\ncontent2"
	adjusted := service.adjustCodePaths(codeContext, "repo1")

	expected := "path/to/repo1/file1.go\ncontent1\n\npath/to/repo1/file2.go\ncontent2"
	assert.Equal(t, expected, adjusted)

	// Test with unknown repo
	adjusted = service.adjustCodePaths(codeContext, "unknown")
	assert.Equal(t, codeContext, adjusted)
}

func TestApiService_GetTopicDetails_RootRepo(t *testing.T) {
	mockTopicStore := mocks.NewTopicStore(t)
	repoPaths := map[string]string{
		"repo1": "path/to/repo1",
	}
	service := &ApiService{
		topicStore:                   mockTopicStore,
		getTopicDetailsCounterMetric: metrics2.GetCounter("test"),
		repoPaths:                    repoPaths,
	}

	ctx := context.Background()
	topicID := int64(123)

	mockTopic := &topicstore.Topic{
		ID:          topicID,
		Repository:  "repo1",
		Title:       "Test Topic",
		Summary:     "Summary",
		CodeContext: "file.go\ncontent",
	}
	// When requesting root, we look into the root (or specific repo if specified in ReadTopic)
	// Actually ReadTopic in apiService.go is called with req.Repository.
	mockTopicStore.On("ReadTopic", mock.Anything, topicID, "root").Return(mockTopic, nil)

	req := &pb.GetTopicDetailsRequest{
		TopicIds:         []int64{topicID},
		Repository:       "root",
		SearchRepository: "root",
		IncludeCode:      true,
	}

	resp, err := service.GetTopicDetails(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Topics, 1)
	assert.Equal(t, "path/to/repo1/file.go\ncontent", resp.Topics[0].CodeChunks[0])
}

func TestApiService_GetSummary_EmptyQuery(t *testing.T) {
	service := &ApiService{}
	ctx := context.Background()
	req := &pb.GetSummaryRequest{
		Query: "",
		Topics: []*pb.GetSummaryRequest_TopicRequest{
			{
				TopicId: 123,
			},
		},
	}
	_, err := service.GetSummary(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query cannot be empty")
}

func TestApiService_GetSummary_NoTopics(t *testing.T) {
	service := &ApiService{}
	ctx := context.Background()
	req := &pb.GetSummaryRequest{
		Query:  "test",
		Topics: []*pb.GetSummaryRequest_TopicRequest{},
	}
	_, err := service.GetSummary(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topics cannot be empty")
}
