package comment

import (
	"context"
	"os"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"

	"go.skia.org/infra/comment_rag/go/commentstore"
	"go.skia.org/infra/comment_rag/go/spanner"
	pb "go.skia.org/infra/comment_rag/proto/comment/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/genai"
)

const (
	geminiApiKeyEnvVar   = "GEMINI_API_KEY"
	geminiProjectEnvVar  = "GEMINI_PROJECT"
	geminiLocationEnvVar = "GEMINI_LOCATION"
	defaultCommentsLimit = 10
)

// ApiService provides a struct for the Comment RAG api implementation.
type ApiService struct {
	pb.UnimplementedCommentRagApiServiceServer

	// Store instance.
	commentStore commentstore.CommentStore

	// GenAI Client instance.
	genAiClient genai.GenAIClient

	// Embedding model to use for query.
	queryEmbeddingModel string

	// Output dimensionality for query embedding.
	dimensionality int32

	// Metric to count SearchComments calls.
	searchCommentsCounterMetric metrics2.Counter
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(ctx context.Context, commentStore commentstore.CommentStore, queryEmbeddingModel string, dimensionality int32) *ApiService {
	var genAiClient *genai.GeminiClient
	var err error
	// Get the api key from the env.
	apiKey := os.Getenv(geminiApiKeyEnvVar)

	if apiKey != "" {
		sklog.Infof("Gemini api key specified in the environment, creating a local client.")
		genAiClient, err = genai.NewLocalGeminiClient(ctx, apiKey)
	} else {
		projectId := os.Getenv(geminiProjectEnvVar)
		location := os.Getenv(geminiLocationEnvVar)
		if projectId == "" || location == "" {
			sklog.Fatalf("%s and %s environment variables need to be set.", geminiProjectEnvVar, geminiLocationEnvVar)
		}
		sklog.Infof("Creating a new Gemini client for project %s and location %s", projectId, location)
		genAiClient, err = genai.NewGeminiClient(ctx, projectId, location)
	}

	if err != nil {
		sklog.Errorf("Error creating new gemini client: %v", err)
		return nil
	}

	return &ApiService{
		commentStore:                commentStore,
		genAiClient:                 genAiClient,
		queryEmbeddingModel:         queryEmbeddingModel,
		dimensionality:              dimensionality,
		searchCommentsCounterMetric: metrics2.GetCounter("commentrag_searchComments_count"),
	}
}

// RegisterGrpc registers the grpc service with the server instance.
func (service *ApiService) RegisterGrpc(server *grpc.Server) {
	pb.RegisterCommentRagApiServiceServer(server, service)
}

// RegisterHttp registers the service with the http handler.
func (service *ApiService) RegisterHttp(ctx context.Context, mux *runtime.ServeMux) error {
	return pb.RegisterCommentRagApiServiceHandlerServer(ctx, mux, service)
}

// GetServiceDescriptor returns the service descriptor.
func (service *ApiService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.CommentRagApiService_ServiceDesc
}

// SearchComments implements the SearchComments endpoint.
func (service *ApiService) SearchComments(ctx context.Context, req *pb.SearchCommentsRequest) (*pb.SearchCommentsResponse, error) {
	if req == nil {
		return nil, skerr.Fmt("request cannot be nil.")
	}

	var categories []string
	seen := make(map[string]bool)
	for _, cat := range req.GetCategories() {
		trimmed := strings.TrimSpace(cat)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToUpper(trimmed)
		if !spanner.IsValidCategory(normalized) {
			return nil, skerr.Fmt("invalid category: %q. Supported categories are: %v", cat, spanner.ValidCategories)
		}
		if !seen[normalized] {
			seen[normalized] = true
			categories = append(categories, normalized)
		}
	}

	if len(categories) == 0 {
		return nil, skerr.Fmt("at least one category must be specified.")
	}

	project := strings.TrimSpace(req.GetProject())
	if project == "" {
		return nil, skerr.Fmt("project cannot be empty.")
	}

	repo := strings.TrimSpace(req.GetRepo())
	if repo == "" {
		return nil, skerr.Fmt("repo cannot be empty.")
	}

	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, skerr.Fmt("query cannot be empty.")
	}
	sklog.Infof("Received SearchComments request with query: %s", query)
	service.searchCommentsCounterMetric.Inc(1)

	ctx, span := trace.StartSpan(ctx, "commentrag.service.SearchComments")
	defer span.End()

	// Get the embedding vector for the input query.
	queryEmbedding, err := service.genAiClient.GetEmbedding(ctx, service.queryEmbeddingModel, service.dimensionality, query)
	if err != nil {
		sklog.Errorf("Error getting embedding for query %s: %v", query, err)
		return nil, err
	}

	sklog.Infof("Embedding for query %q has length %d", query, len(queryEmbedding))

	// Search the relevant comments in Spanner.
	limit := defaultCommentsLimit
	if req.GetMaxComments() > 0 {
		limit = int(req.GetMaxComments())
	}

	foundCases, err := service.commentStore.SearchComments(ctx, queryEmbedding, limit, project, repo, categories)
	if err != nil {
		sklog.Errorf("Error searching for comments: %v", err)
		return nil, err
	}

	// Generate the response.
	resp := &pb.SearchCommentsResponse{}
	for _, c := range foundCases {
		resp.Comments = append(resp.Comments, &pb.SearchCommentsResponse_CommentRecord{
			Id:             c.ID,
			ChangeId:       c.ChangeID,
			Project:        c.Project,
			Category:       c.Category,
			Repo:           c.Repo,
			FilePath:       c.FilePath,
			CommentText:    c.CommentText,
			CodeSnippet:    c.CodeSnippet,
			ClSubject:      c.CLSubject,
			ClDescription:  c.CLDescription,
			Analysis:       c.Analysis,
			CosineDistance: float32(c.Distance),
		})
	}

	sklog.Infof("Returning %d matching comment records", len(resp.Comments))
	return resp, nil
}

// ListValidCategories lists all valid review categories supported by the comment_rag service.
func (service *ApiService) ListValidCategories(ctx context.Context, req *pb.ListValidCategoriesRequest) (*pb.ListValidCategoriesResponse, error) {
	sklog.Info("Received ListValidCategories request")
	return &pb.ListValidCategoriesResponse{
		Categories: spanner.ValidCategories,
	}, nil
}
