package history

import (
	"context"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/genai"
	"go.skia.org/infra/rag/go/topicstore"
	pb "go.skia.org/infra/rag/proto/history/v1"
)

const (
	geminiApiKeyEnvVar   = "GEMINI_API_KEY"
	geminiProjectEnvVar  = "GEMINI_PROJECT"
	geminiLocationEnvVar = "GEMINI_LOCATION"
)

// ApiService provides a struct for the HistoryRag api implementation.
type ApiService struct {
	pb.UnimplementedHistoryRagApiServiceServer

	// BlameStore instance.
	blameStore blamestore.BlameStore

	// TopicStore instance.
	topicStore topicstore.TopicStore

	// GenAI Client instance.
	genAiClient genai.GenAIClient

	// Embedding model to use for query.
	queryEmbeddingModel string
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(ctx context.Context, dbClient *spanner.Client, queryEmbeddingModel string) *ApiService {
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
		blameStore:          blamestore.New(dbClient),
		topicStore:          topicstore.New(dbClient),
		genAiClient:         genAiClient,
		queryEmbeddingModel: queryEmbeddingModel,
	}
}

// RegisterGrpc registers the grpc service with the server instance.
func (service *ApiService) RegisterGrpc(server *grpc.Server) {
	pb.RegisterHistoryRagApiServiceServer(server, service)
}

// RegisterHttp registers the service with the http handler.
func (service *ApiService) RegisterHttp(ctx context.Context, mux *runtime.ServeMux) error {
	return pb.RegisterHistoryRagApiServiceHandlerServer(ctx, mux, service)
}

// GetServiceDescriptor returns the service descriptor.
func (service *ApiService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.HistoryRagApiService_ServiceDesc
}

// GetBlames implements the GetBlames api endpoint.
func (service *ApiService) GetBlames(ctx context.Context, req *pb.GetBlamesRequest) (*pb.GetBlamesResponse, error) {
	if req.GetFilePath() == "" {
		return nil, skerr.Fmt("filePath cannot be empty.")
	}
	fileBlames, err := service.blameStore.ReadBlame(ctx, req.GetFilePath())
	if err != nil {
		sklog.Errorf("Error retrieving blame data for file %s: %v", req.GetFilePath(), err)
		return nil, err
	}

	// Populate the response.
	resp := &pb.GetBlamesResponse{
		FilePath: fileBlames.FilePath,
		FileHash: fileBlames.FileHash,
		Version:  fileBlames.Version,
	}
	for _, lb := range fileBlames.LineBlames {
		resp.LineBlames = append(resp.LineBlames, &pb.GetBlamesResponse_LineBlame{
			LineNumber: lb.LineNumber,
			CommitHash: lb.CommitHash,
		})
	}
	return resp, nil
}

// GetTopics implements the GetTopics endpoint.
func (service *ApiService) GetTopics(ctx context.Context, req *pb.GetTopicsRequest) (*pb.GetTopicsResponse, error) {
	query := req.GetQuery()
	if query == "" {
		return nil, skerr.Fmt("query cannot be empty.")
	}

	// Get the embedding vector for the input query.
	queryEmbedding, err := service.genAiClient.GetEmbedding(ctx, service.queryEmbeddingModel, query)
	if err != nil {
		sklog.Errorf("Error getting embedding for query %s: %v", query, err)
		return nil, err
	}

	// Search the relevant topics for the given query embedding.
	topics, err := service.topicStore.SearchTopics(ctx, queryEmbedding)
	if err != nil {
		sklog.Errorf("Error searching for topics: %v", err)
		return nil, err
	}

	// Generate the response.
	resp := &pb.GetTopicsResponse{}
	for _, topic := range topics {
		respTopic := &pb.GetTopicsResponse_Topic{
			TopicId:    int64(topic.ID),
			TopicName:  topic.Title,
			Similarity: float32(topic.Distance),
		}
		for _, chunk := range topic.Chunks {
			respTopic.MatchingChunks = append(respTopic.MatchingChunks, &pb.GetTopicsResponse_Topic_Chunk{
				ChunkId:      int64(chunk.ID),
				ChunkContent: chunk.Chunk,
				ChunkIndex:   int32(chunk.ChunkIndex),
			})
		}
		resp.Topics = append(resp.Topics, respTopic)
	}
	return resp, nil
}
