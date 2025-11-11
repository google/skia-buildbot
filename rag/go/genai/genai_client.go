package genai

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"google.golang.org/genai"
)

// GenAIClient defines an interface for defining a genAI client.
type GenAIClient interface {
	// GetEmbedding returns the embedding vector for the given input.
	GetEmbedding(ctx context.Context, model string, dimensionality int32, input string) ([]float32, error)
}

// GeminiClient implements GenAIClient, defines a struct to access Gemini api.
type GeminiClient struct {
	// Gemini api client.
	genAiClient *genai.Client
}

// NewGeminiClient returns a new instance of the GeminiClient.
func NewGeminiClient(ctx context.Context, project, location string) (*GeminiClient, error) {
	genAiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		sklog.Errorf("Error creating new gemini client: %v", err)
		return nil, err
	}

	return &GeminiClient{
		genAiClient: genAiClient,
	}, nil
}

// NewLocalGeminiClient returns a new instance of the GeminiClient using an api key.
//
// This is not intended for production purposes.
func NewLocalGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	genAiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		sklog.Errorf("Error creating new gemini client: %v", err)
		return nil, err
	}

	return &GeminiClient{
		genAiClient: genAiClient,
	}, nil
}

// GetEmbedding returns the embedding for the input using the supplied model.
func (c *GeminiClient) GetEmbedding(ctx context.Context, model string, dimensionality int32, input string) ([]float32, error) {
	embeddingInput := genai.Content{
		Parts: []*genai.Part{
			genai.NewPartFromText(input),
		},
	}
	resp, err := c.genAiClient.Models.EmbedContent(
		ctx,
		model,
		[]*genai.Content{&embeddingInput},
		&genai.EmbedContentConfig{
			OutputDimensionality: &dimensionality,
		},
	)
	if err != nil {
		sklog.Errorf("Error getting embedding: %v", err)
		return nil, err
	}
	return resp.Embeddings[0].Values, nil
}
