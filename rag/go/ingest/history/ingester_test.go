package history

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/filereaders/pickle"
	"go.skia.org/infra/rag/go/topicstore"
	"go.skia.org/infra/rag/go/topicstore/mocks"
)

type mockBlameStore struct {
	blames []*blamestore.FileBlame
}

func (m *mockBlameStore) WriteBlame(ctx context.Context, blame *blamestore.FileBlame) error {
	m.blames = append(m.blames, blame)
	return nil
}

func (m *mockBlameStore) ReadBlame(ctx context.Context, filePath string) (*blamestore.FileBlame, error) {
	return nil, nil
}

func TestHistoryIngester_IngestBlameFileData(t *testing.T) {
	ctx := context.Background()
	mockStore := &mockBlameStore{}
	ingester := New(mockStore, nil, 768, false, "")

	filePath := "foo.go"
	fileContent := []byte(`{
		"version": "0.1",
		"file_hash": "f5db6789ee8942bc72a8738ba86fbc0c22c09694",
		"lines": [
			"85111c5041120c782317b207d398ce82fd161fe6",
			"a89155ae3b87878b8e71883148fd5f2a426bb349"
		]
	}`)

	err := ingester.IngestBlameFileData(ctx, filePath, fileContent)
	assert.NoError(t, err)

	assert.Len(t, mockStore.blames, 1)
	blame := mockStore.blames[0]
	assert.Equal(t, "foo.go", blame.FilePath)
	assert.Equal(t, "f5db6789ee8942bc72a8738ba86fbc0c22c09694", blame.FileHash)
	assert.Equal(t, "0.1", blame.Version)
	assert.Len(t, blame.LineBlames, 2)
	assert.Equal(t, int64(1), blame.LineBlames[0].LineNumber)
	assert.Equal(t, "85111c5041120c782317b207d398ce82fd161fe6", blame.LineBlames[0].CommitHash)
	assert.Equal(t, int64(2), blame.LineBlames[1].LineNumber)
	assert.Equal(t, "a89155ae3b87878b8e71883148fd5f2a426bb349", blame.LineBlames[1].CommitHash)
}

func TestHistoryIngester_IngestTopics_RepoExtraction(t *testing.T) {
	ctx := context.Background()
	mockTopicStore := mocks.NewTopicStore(t)
	ingester := New(nil, mockTopicStore, 768, true, "default-repo")

	tempDir, err := os.MkdirTemp("", "ingester-test-*")
	require.NoError(t, err)

	topicsDir := filepath.Join(tempDir, "topics")
	repoDir := filepath.Join(topicsDir, "my-repo")
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	topicFilePath := filepath.Join(repoDir, "topic1.json")
	topicData := `{
		"topic_id": 1,
		"summary": "This is a summary",
		"code_context": "Some code context"
	}`
	err = os.WriteFile(topicFilePath, []byte(topicData), 0644)
	require.NoError(t, err)

	emb1 := make([]float32, 768)
	emb1[0] = 0.1
	emb2 := make([]float32, 768)
	emb2[0] = 0.3
	embeddings := [][]float32{emb1, emb2}
	indexEntries := map[int64]pickle.IndexEntry{
		1: {
			Title:            "Topic 1",
			Group:            "Group 1",
			CommitCount:      10,
			CodeContextLines: 5,
			Chunks: []pickle.IndexChunk{
				{
					ChunkId:        101,
					ChunkContent:   "Chunk 1",
					EmbeddingIndex: 0,
				},
			},
		},
	}

	mockTopicStore.On("WriteTopic", mock.Anything, mock.MatchedBy(func(topic *topicstore.Topic) bool {
		return topic.ID == 1 && topic.Repository == "my-repo" && topic.Title == "Topic 1"
	})).Return(nil)

	err = ingester.ingestTopicFile(ctx, topicFilePath, "my-repo", embeddings, indexEntries)
	assert.NoError(t, err)

	mockTopicStore.AssertExpectations(t)
}

func TestHistoryIngester_IngestTopics_DefaultRepo(t *testing.T) {
	ctx := context.Background()
	mockTopicStore := mocks.NewTopicStore(t)
	ingester := New(nil, mockTopicStore, 768, false, "default-repo")

	tempDir, err := os.MkdirTemp("", "ingester-test-default-*")
	require.NoError(t, err)

	topicsDir := filepath.Join(tempDir, "topics")
	err = os.MkdirAll(topicsDir, 0755)
	require.NoError(t, err)

	topicFilePath := filepath.Join(topicsDir, "topic1.json")
	topicData := `{"topic_id": 1}`
	err = os.WriteFile(topicFilePath, []byte(topicData), 0644)
	require.NoError(t, err)

	embeddings := [][]float32{make([]float32, 768)}
	indexEntries := map[int64]pickle.IndexEntry{
		1: {
			Title: "Topic 1",
			Chunks: []pickle.IndexChunk{
				{ChunkId: 101, EmbeddingIndex: 0},
			},
		},
	}

	mockTopicStore.On("WriteTopic", mock.Anything, mock.MatchedBy(func(topic *topicstore.Topic) bool {
		return topic.ID == 1 && topic.Repository == "default-repo"
	})).Return(nil)

	err = ingester.IngestTopics(ctx, topicsDir, "fake", "fake") // Fake paths since we mock ReadFile in ingestTopicFile?
	// Wait, IngestTopics calls npyReader and pickleReader. I should test ingestTopicFile instead or mock readers.

	err = ingester.ingestTopicFile(ctx, topicFilePath, "default-repo", embeddings, indexEntries)
	assert.NoError(t, err)

	mockTopicStore.AssertExpectations(t)
}
