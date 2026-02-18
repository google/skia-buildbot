package sources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	genai_mocks "go.skia.org/infra/rag/go/genai/mocks"
)

func TestPubSubSource_RunEvaluation_NoGenAi(t *testing.T) {
	source := &PubSubSource{
		genAiClient: nil,
		evalSetPath: "any",
	}
	err := source.runEvaluation(context.Background(), "a", "b", "c")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "genAiClient is nil")
}

func TestPubSubSource_RunEvaluation_InvalidEvalSetPath(t *testing.T) {
	source := &PubSubSource{
		genAiClient:    &genai_mocks.GenAIClient{},
		evalSetPath:    "non-existent.json",
		dimensionality: 768,
	}
	// IngestTopics will fail if files don't exist.
	err := source.runEvaluation(context.Background(), "non-existent-dir", "non-existent-npy", "non-existent-pickle")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ingest topics")
}

func TestNewPubSubSource(t *testing.T) {
	ctx := context.Background()
	source, err := NewPubSubSource(ctx, nil, nil, nil, "eval_set.json", "model", 768, false, "default-repo")
	// This might fail if google.DefaultTokenSource fails in non-GCP environment.
	if err != nil {
		t.Skip("Skipping NewPubSubSource test as it requires GCP credentials")
	}
	assert.NotNil(t, source)
	assert.Equal(t, "eval_set.json", source.evalSetPath)
}
