package topicstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/rag/go/sqltest"
)

func TestRepositoryTopicStore(t *testing.T) {
	ctx := context.Background()
	spannerClient, err := sqltest.NewSpannerDBForTests(t, "repo_topicstore")
	require.NoError(t, err)

	store := NewRepositoryTopicStore(spannerClient)

	emb1 := make([]float32, 768)
	emb1[0] = 1.0
	topic1 := &Topic{
		ID:         1,
		Repository: "repo-a",
		Title:      "Topic 1",
		Chunks: []*TopicChunk{
			{
				ID:        101,
				Chunk:     "Chunk 1",
				Embedding: emb1,
			},
		},
	}

	emb2 := make([]float32, 768)
	emb2[1] = 1.0
	topic2 := &Topic{
		ID:         1, // Same ID, different repo
		Repository: "repo-b",
		Title:      "Topic 2",
		Chunks: []*TopicChunk{
			{
				ID:        201,
				Chunk:     "Chunk 2",
				Embedding: emb2,
			},
		},
	}

	// Test Write
	err = store.WriteTopic(ctx, topic1)
	require.NoError(t, err)

	err = store.WriteTopic(ctx, topic2)
	require.NoError(t, err)

	// Test ReadTopic (ambiguous by ID, returns one)
	readTopic, err := store.ReadTopic(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), readTopic.ID)
	assert.Contains(t, []string{"repo-a", "repo-b"}, readTopic.Repository)

	// Test SearchTopics
	// Searching with embedding closer to topic1
	searchEmb := make([]float32, 768)
	searchEmb[0] = 1.0
	found, err := store.SearchTopics(ctx, searchEmb, 10)
	require.NoError(t, err)
	assert.Len(t, found, 2)
	assert.Equal(t, "Topic 1", found[0].Title)
	assert.Equal(t, "repo-a", found[0].Repository)
	assert.Equal(t, "Topic 2", found[1].Title)
	assert.Equal(t, "repo-b", found[1].Repository)
}
