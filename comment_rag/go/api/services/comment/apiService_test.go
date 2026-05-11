package comment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/comment_rag/go/commentstore"
	"go.skia.org/infra/comment_rag/go/spanner"
	pb "go.skia.org/infra/comment_rag/proto/comment/v1"
	"go.skia.org/infra/go/metrics2"
)

// MockCommentStore mocks the commentstore.CommentStore interface.
type MockCommentStore struct {
	mock.Mock
}

func (m *MockCommentStore) SearchComments(ctx context.Context, queryEmbedding []float32, maxComments int, project, repo string, categories []string) ([]*commentstore.FoundCommentRecord, error) {
	args := m.Called(ctx, queryEmbedding, maxComments, project, repo, categories)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commentstore.FoundCommentRecord), args.Error(1)
}

func (m *MockCommentStore) WriteCommentRecord(ctx context.Context, c *commentstore.CommentRecord) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

// MockGenAIClient mocks the genai.GenAIClient interface.
type MockGenAIClient struct {
	mock.Mock
}

func (m *MockGenAIClient) GetEmbedding(ctx context.Context, model string, dimensionality int32, content string) ([]float32, error) {
	args := m.Called(ctx, model, dimensionality, content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockGenAIClient) GetSummary(ctx context.Context, model string, input string) (string, error) {
	args := m.Called(ctx, model, input)
	return args.String(0), args.Error(1)
}

func TestApiService_ListValidCategories(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockCommentStore{}
	service := &ApiService{
		commentStore: mockStore,
	}

	resp, err := service.ListValidCategories(ctx, &pb.ListValidCategoriesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.GetCategories(), 1)
	assert.Equal(t, "IPC_SECURITY", resp.GetCategories()[0])
}

func TestApiService_SearchComments_Validation(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockCommentStore{}
	mockGenAi := &MockGenAIClient{}

	service := &ApiService{
		commentStore:                mockStore,
		genAiClient:                 mockGenAi,
		queryEmbeddingModel:         "text-embedding-005",
		dimensionality:              768,
		searchCommentsCounterMetric: metrics2.GetCounter("test_search_comments_count"),
	}

	// Case 0: Request cannot be nil
	_, err := service.SearchComments(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")

	// Case 1: Query cannot be empty
	_, err = service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      "",
		Categories: []string{spanner.CategoryIpcSecurity},
		Project:    "chromium",
		Repo:       "chromium/src",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query cannot be empty")

	// Case 1.5: At least one category must be specified
	_, err = service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query: "some-concept",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one category must be specified")

	// Case 2: Category must be supported
	_, err = service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      "some-concept",
		Categories: []string{"STYLE"}, // Unsupported category
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid category: \"STYLE\"")
	assert.Contains(t, err.Error(), "Supported categories are: [IPC_SECURITY]")

	// Case 2.1: Project cannot be empty
	_, err = service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      "some-concept",
		Categories: []string{spanner.CategoryIpcSecurity},
		Repo:       "chromium/src",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project cannot be empty")

	// Case 2.2: Repo cannot be empty
	_, err = service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      "some-concept",
		Categories: []string{spanner.CategoryIpcSecurity},
		Project:    "chromium",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo cannot be empty")

	// Case 3: Valid category (IPC_SECURITY) passes validation and queries DB
	mockEmb := []float32{1.0, 0.0}
	mockGenAi.On("GetEmbedding", mock.Anything, "text-embedding-005", int32(768), "mojo handle").Return(mockEmb, nil).Once()
	mockStore.On("SearchComments", mock.Anything, mockEmb, 10, "chromium", "chromium/src", []string{spanner.CategoryIpcSecurity}).Return([]*commentstore.FoundCommentRecord{
		{
			CommentRecord: commentstore.CommentRecord{
				ID:          "123",
				ChangeID:    12345,
				Project:     "chromium",
				Repo:        "chromium/src",
				Category:    "IPC_SECURITY",
				CommentText: "Mojo thread",
			},
			Distance: 0.0,
		},
	}, nil).Once()

	resp, err := service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      " mojo handle",
		Categories: []string{spanner.CategoryIpcSecurity},
		Project:    "chromium",
		Repo:       "chromium/src",
	})
	require.NoError(t, err)
	assert.Len(t, resp.GetComments(), 1)
	assert.Equal(t, "123", resp.GetComments()[0].Id)
	assert.Equal(t, int64(12345), resp.GetComments()[0].ChangeId)

	mockStore.AssertExpectations(t)
	mockGenAi.AssertExpectations(t)
}

func TestApiService_SearchComments_CategoryNormalization(t *testing.T) {
	ctx := context.Background()
	mockStore := &MockCommentStore{}
	mockGenAi := &MockGenAIClient{}

	service := &ApiService{
		commentStore:                mockStore,
		genAiClient:                 mockGenAi,
		queryEmbeddingModel:         "text-embedding-005",
		dimensionality:              768,
		searchCommentsCounterMetric: metrics2.GetCounter("test_search_normalization_count"),
	}

	mockEmb := []float32{1.0, 0.0}
	// Expectation: GetEmbedding is called, and SearchComments is called with the normalized "IPC_SECURITY"!
	mockGenAi.On("GetEmbedding", mock.Anything, "text-embedding-005", int32(768), "mojo").Return(mockEmb, nil).Once()
	mockStore.On("SearchComments", mock.Anything, mockEmb, 10, "chromium", "chromium/src", []string{spanner.CategoryIpcSecurity}).Return([]*commentstore.FoundCommentRecord{
		{
			CommentRecord: commentstore.CommentRecord{
				ID:       "123",
				Category: "IPC_SECURITY",
				Project:  "chromium",
				Repo:     "chromium/src",
			},
		},
	}, nil).Once()

	_, err := service.SearchComments(ctx, &pb.SearchCommentsRequest{
		Query:      "mojo",
		Categories: []string{"ipc_security ", "IPC_SECURITY", "  ipc_security  "},
		Project:    "chromium",
		Repo:       "chromium/src",
	})
	require.NoError(t, err)

	mockStore.AssertExpectations(t)
	mockGenAi.AssertExpectations(t)
}
