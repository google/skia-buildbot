package topicstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryTopicStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTopicStore()

	topic := &Topic{
		ID:    1,
		Title: "Test Topic",
		Chunks: []*TopicChunk{
			{
				ID:        101,
				Chunk:     "This is a test chunk",
				Embedding: []float32{1.0, 0.0, 0.0},
			},
		},
	}

	// Test Write and Read
	err := store.WriteTopic(ctx, topic)
	require.NoError(t, err)

	readTopic, err := store.ReadTopic(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, topic.Title, readTopic.Title)

	// Test Search
	// Query embedding exactly matches the chunk
	foundTopics, err := store.SearchTopics(ctx, []float32{1.0, 0.0, 0.0}, 1)
	require.NoError(t, err)
	assert.Len(t, foundTopics, 1)
	assert.Equal(t, int64(1), foundTopics[0].ID)
	assert.InDelta(t, 0.0, foundTopics[0].Distance, 1e-6)

	// Query embedding orthogonal to the chunk
	foundTopics, err = store.SearchTopics(ctx, []float32{0.0, 1.0, 0.0}, 1)
	require.NoError(t, err)
	assert.Len(t, foundTopics, 1)
	assert.InDelta(t, 1.0, foundTopics[0].Distance, 1e-6)
}

func TestInMemoryTopicStore_SearchMultiple(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTopicStore()

	err := store.WriteTopic(ctx, &Topic{
		ID:    1,
		Title: "Topic 1",
		Chunks: []*TopicChunk{
			{ID: 101, Chunk: "Chunk 1", Embedding: []float32{1.0, 0.0}},
		},
	})
	require.NoError(t, err)

	err = store.WriteTopic(ctx, &Topic{
		ID:    2,
		Title: "Topic 2",
		Chunks: []*TopicChunk{
			{ID: 201, Chunk: "Chunk 2", Embedding: []float32{0.0, 1.0}},
		},
	})
	require.NoError(t, err)

	// Search for something closer to Topic 1
	found, err := store.SearchTopics(ctx, []float32{0.8, 0.2}, 2)
	require.NoError(t, err)
	assert.Len(t, found, 2)
	assert.Equal(t, int64(1), found[0].ID)
	assert.Equal(t, int64(2), found[1].ID)
}
