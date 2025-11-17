package history

import (
	"context"
	"encoding/json"
	"os"
	"strings"

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
	geminiApiKeyEnvVar     = "GEMINI_API_KEY"
	geminiProjectEnvVar    = "GEMINI_PROJECT"
	geminiLocationEnvVar   = "GEMINI_LOCATION"
	defaultTopicCount      = 20
	maxTopicResponseLength = 450000
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

	// Output dimensionality for query embedding.
	dimensionality int32
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(ctx context.Context, dbClient *spanner.Client, queryEmbeddingModel string, dimensionality int32) *ApiService {
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
		dimensionality:      dimensionality,
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
	queryEmbedding, err := service.genAiClient.GetEmbedding(ctx, service.queryEmbeddingModel, service.dimensionality, query)
	if err != nil {
		sklog.Errorf("Error getting embedding for query %s: %v", query, err)
		return nil, err
	}

	// Search the relevant topics for the given query embedding.
	topicCount := defaultTopicCount
	if req.GetTopicCount() > 0 {
		topicCount = int(req.GetTopicCount())
	}
	topics, err := service.topicStore.SearchTopics(ctx, queryEmbedding, topicCount)
	if err != nil {
		sklog.Errorf("Error searching for topics: %v", err)
		return nil, err
	}

	// Generate the response.
	resp := &pb.GetTopicsResponse{}
	for _, topic := range topics {
		respTopic := &pb.GetTopicsResponse_Topic{
			TopicId:        int64(topic.ID),
			TopicName:      topic.Title,
			CosineDistance: float32(topic.Distance),
			Summary:        topic.Summary,
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

// GetTopicDetails implements the GetTopicDetails endpoint.
func (service *ApiService) GetTopicDetails(ctx context.Context, req *pb.GetTopicDetailsRequest) (*pb.GetTopicDetailsResponse, error) {
	// Get all the topic ids from the request.
	topicIds := req.GetTopicIds()
	if len(topicIds) == 0 {
		return nil, skerr.Fmt("topicIds cannot be empty.")
	}

	resp := &pb.GetTopicDetailsResponse{}

	// Process all the topics one by one.
	// TODO(ashwinpv): We can potentially do this in one db call.
	for _, topicId := range topicIds {
		if topicId < 0 {
			return nil, skerr.Fmt("topicIds cannot be negative.")
		}

		// Read the topic data from the db.
		topic, err := service.topicStore.ReadTopic(ctx, topicId)
		if err != nil {
			sklog.Errorf("Error reading topic %d: %v", topicId, err)
			return nil, err
		}

		respTopic := &pb.GetTopicDetailsResponse_Topic{
			TopicId:   topic.ID,
			TopicName: topic.Title,
			Summary:   topic.Summary,
		}

		// To ensure that the size of the response is kept within a reasonable limit,
		// we keep a track of the length and check whether we exceed the max limit.
		jsonResponse, err := json.Marshal(respTopic)
		if err != nil {
			sklog.Errorf("Error marshalling response: %v", err)
			return nil, err
		}
		currentTopicResponseLength := len(jsonResponse)
		// Process the code context if code or tests are to be included in the response.
		if topic.CodeContext != "" && (req.IncludeCode || req.IncludeTests) {
			testChunks := []string{}
			isTestFile := func(fileName string) bool {
				return strings.Contains(strings.ToLower(fileName), "test")
			}
			allTopicCode := strings.Split(topic.CodeContext, "\n\n")
			for _, code := range allTopicCode {
				// Check if this is a test file.
				fileName := strings.TrimSpace(strings.Split(code, "\n")[0])

				// Prioritize code files if specified and collect their chunks first.
				if req.IncludeCode && !isTestFile(fileName) {
					if currentTopicResponseLength+len(code) > maxTopicResponseLength {
						break
					}
					respTopic.CodeChunks = append(respTopic.CodeChunks, code)
					currentTopicResponseLength += len(code)
				}
				// Keep a track of the test files to be added later if IncludeTests=true.
				if req.IncludeTests && isTestFile(fileName) {
					testChunks = append(testChunks, code)
				}
			}

			// If there are test chunks to be added, let's add those now.
			if len(testChunks) > 0 {
				for _, testChunk := range testChunks {
					if currentTopicResponseLength+len(testChunk) > maxTopicResponseLength {
						break
					}
					respTopic.CodeChunks = append(respTopic.CodeChunks, testChunk)
					currentTopicResponseLength += len(testChunk)
				}
			}
		}

		resp.Topics = append(resp.Topics, respTopic)
	}
	return resp, nil
}
