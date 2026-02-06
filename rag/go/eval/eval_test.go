package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	genai_mocks "go.skia.org/infra/rag/go/genai/mocks"
	"go.skia.org/infra/rag/go/topicstore"
)

func TestEvaluator_Run(t *testing.T) {
	ctx := context.Background()
	mockGenAI := &genai_mocks.GenAIClient{}
	store := topicstore.NewInMemoryTopicStore()

	// Setup store with some data
	topics := []*topicstore.Topic{
		{
			ID:    1,
			Title: "Data Retention Policy",
			Chunks: []*topicstore.TopicChunk{
				{ID: 101, Chunk: "Retention for 30 days", Embedding: []float32{1.0, 0.0}},
			},
		},
		{
			ID:    2,
			Title: "Security Policy",
			Chunks: []*topicstore.TopicChunk{
				{ID: 201, Chunk: "Security protocols", Embedding: []float32{0.0, 1.0}},
			},
		},
	}
	for _, topic := range topics {
		err := store.WriteTopic(ctx, topic)
		require.NoError(t, err)
	}

	evalSet := &EvaluationSet{
		TestCases: []TestCase{
			{
				Query:              "How long do we keep data?",
				ExpectedTopicNames: []string{"Data Retention Policy"},
			},
		},
	}

	// Mock embedding for the query to be close to topic 1
	mockGenAI.On("GetEmbedding", ctx, "test-model", int32(2), "How long do we keep data?").Return([]float32{0.9, 0.1}, nil)

	evaluator := NewEvaluator(mockGenAI, store, "test-model", 2)
	report, err := evaluator.Run(ctx, evalSet)

	require.NoError(t, err)
	assert.Equal(t, 1, report.TotalQueries)
	assert.Equal(t, 1.0, report.MeanRecallAt5)
	assert.Equal(t, 1.0, report.MeanMRR)
	assert.True(t, report.Results[0].Passed)
	assert.Equal(t, "Data Retention Policy", report.Results[0].FoundNames[0])
}
