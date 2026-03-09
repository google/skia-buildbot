package history

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	maxPromptLength        = 450000
)

// ApiService provides a struct for the HistoryRag api implementation.
type ApiService struct {
	pb.UnimplementedHistoryRagApiServiceServer

	// TopicStore instance.
	topicStore topicstore.TopicStore

	// GenAI Client instance.
	genAiClient genai.GenAIClient

	// Embedding model to use for query.
	queryEmbeddingModel string

	// Model to use for summary.
	summaryModel string

	// Output dimensionality for query embedding.
	dimensionality int32

	// Metric to count GetTopics calls.
	getTopicsCounterMetric metrics2.Counter
	// Metric to count GetTopicDetails calls.
	getTopicDetailsCounterMetric metrics2.Counter
	// Metric to count GetSummary calls.
	getSummaryCounterMetric metrics2.Counter

	// Mapping of repository names to their relative paths from the root.
	repoPaths map[string]string

	// Mapping of repository relative paths back to their names.
	repoNames map[string]string
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(ctx context.Context, dbClient *spanner.Client, queryEmbeddingModel, summaryModel string, dimensionality int32, repoPaths map[string]string) *ApiService {
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

	repoNames := make(map[string]string)
	for name, path := range repoPaths {
		repoNames[path] = name
	}

	topicStore := topicstore.NewRepositoryTopicStore(dbClient)
	return &ApiService{
		topicStore:          topicStore,
		genAiClient:         genAiClient,
		queryEmbeddingModel: queryEmbeddingModel,
		summaryModel:        summaryModel,
		dimensionality:      dimensionality,
		repoPaths:           repoPaths,
		repoNames:           repoNames,

		// Initialize the metric objects.
		getTopicsCounterMetric:       metrics2.GetCounter("historyrag_getTopics_count"),
		getTopicDetailsCounterMetric: metrics2.GetCounter("historyrag_getTopicDetails_count"),
		getSummaryCounterMetric:      metrics2.GetCounter("historyrag_getSummary_count"),
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

// GetTopics implements the GetTopics endpoint.
func (service *ApiService) GetTopics(ctx context.Context, req *pb.GetTopicsRequest) (*pb.GetTopicsResponse, error) {
	query := req.GetQuery()
	if query == "" {
		return nil, skerr.Fmt("query cannot be empty.")
	}
	sklog.Infof("Received GetTopics request with query: %s", query)
	service.getTopicsCounterMetric.Inc(1)
	ctx, span := trace.StartSpan(ctx, "historyrag.service.GetTopics")
	defer span.End()

	// Get the embedding vector for the input query.
	queryEmbedding, err := service.genAiClient.GetEmbedding(ctx, service.queryEmbeddingModel, service.dimensionality, query)
	if err != nil {
		sklog.Errorf("Error getting embedding for query %s: %v", query, err)
		return nil, err
	}

	sklog.Infof("Embedding for query %q has length %d", query, len(queryEmbedding))
	if len(queryEmbedding) > 0 {
		var sumSq float32
		for _, v := range queryEmbedding {
			sumSq += v * v
		}
		sklog.Infof("Embedding magnitude squared: %f", sumSq)
	}

	// Search the relevant topics for the given query embedding.
	topicCount := defaultTopicCount
	if req.GetTopicCount() > 0 {
		topicCount = int(req.GetTopicCount())
	}
	topics, err := service.topicStore.SearchTopics(ctx, queryEmbedding, topicCount, service.getRepoName(req.GetRepository()))
	if err != nil {
		sklog.Errorf("Error searching for topics: %v", err)
		return nil, err
	}
	if len(topics) == 0 {
		sklog.Warningf("SearchTopics returned 0 topics for query %q", query)
	}

	// Generate the response.
	resp := &pb.GetTopicsResponse{}
	for _, topic := range topics {
		respTopic := &pb.GetTopicsResponse_Topic{
			TopicId:        int64(topic.ID),
			TopicName:      topic.Title,
			CosineDistance: float32(topic.Distance),
			Summary:        topic.Summary,
			Repository:     topic.Repository,
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
	sklog.Infof("Returning %d topics", len(resp.Topics))
	return resp, nil
}

// GetRepositories implements the GetRepositories endpoint.
func (service *ApiService) GetRepositories(ctx context.Context, req *pb.GetRepositoriesRequest) (*pb.GetRepositoriesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "historyrag.service.GetRepositories")
	defer span.End()

	repositories, err := service.topicStore.GetRepositories(ctx)
	if err != nil {
		sklog.Errorf("Error getting repositories: %v", err)
		return nil, err
	}

	resp := &pb.GetRepositoriesResponse{}
	for _, repo := range repositories {
		path := repo
		if p, ok := service.repoPaths[repo]; ok {
			path = p
		}
		resp.Repositories = append(resp.Repositories, path)
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

	service.getTopicDetailsCounterMetric.Inc(1)
	ctx, span := trace.StartSpan(ctx, "historyrag.service.GetTopicDetails")
	defer span.End()

	resp := &pb.GetTopicDetailsResponse{}

	// Process all the topics one by one.
	// TODO(ashwinpv): We can potentially do this in one db call.
	searchRepo := service.getRepoName(req.SearchRepository)
	repo := service.getRepoName(req.Repository)

	for _, topicId := range topicIds {
		if topicId < 0 {
			return nil, skerr.Fmt("topicIds cannot be negative.")
		}

		// Read the topic data from the db.
		topic, err := service.topicStore.ReadTopic(ctx, topicId, repo)
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

			// If the search repository is "root", then we update the file paths in the code chunks
			// to reflect the path from the root.
			codeContext := topic.CodeContext
			if searchRepo == "root" || searchRepo == "" {
				codeContext = service.adjustCodePaths(topic.CodeContext, topic.Repository)
			}

			allTopicCode := strings.Split(codeContext, "\n\n")
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

// adjustCodePaths prepends the repository relative path to the file paths in the code context.
func (service *ApiService) adjustCodePaths(codeContext, repoName string) string {
	repoPath, ok := service.repoPaths[repoName]
	if !ok || repoPath == "" {
		return codeContext
	}

	var sb strings.Builder
	allTopicCode := strings.Split(codeContext, "\n\n")
	for i, code := range allTopicCode {
		lines := strings.Split(code, "\n")
		if len(lines) > 0 {
			lines[0] = filepath.Join(repoPath, strings.TrimSpace(lines[0]))
		}
		sb.WriteString(strings.Join(lines, "\n"))
		if i < len(allTopicCode)-1 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// getRepoName returns the internal repository name for a given path or name.
func (service *ApiService) getRepoName(pathOrName string) string {
	if name, ok := service.repoNames[pathOrName]; ok {
		return name
	}
	return pathOrName
}

// GetSummary implements the GetSummary endpoint.
func (service *ApiService) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.GetSummaryResponse, error) {
	query := req.GetQuery()
	if query == "" {
		return nil, skerr.Fmt("query cannot be empty.")
	}
	topicsReq := req.GetTopics()
	if len(topicsReq) == 0 {
		return nil, skerr.Fmt("topics cannot be empty.")
	}

	service.getSummaryCounterMetric.Inc(1)
	ctx, span := trace.StartSpan(ctx, "historyrag.service.GetSummary")
	defer span.End()

	// Construct the prompt for the LLM.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Based on the following search results for the query \"%s\", please provide a concise and helpful summary.\n\n", query))

	searchRepo := service.getRepoName(req.SearchRepository)

	for _, tReq := range topicsReq {
		topicId := tReq.GetTopicId()
		repo := service.getRepoName(tReq.GetRepository())
		topic, err := service.topicStore.ReadTopic(ctx, topicId, repo)
		if err != nil {
			sklog.Errorf("Error reading topic %d in repo %s: %v", topicId, repo, err)
			return nil, err
		}

		sb.WriteString(fmt.Sprintf("Topic: %s\n", topic.Title))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", topic.Summary))
		if topic.CodeContext != "" {
			sb.WriteString("Code Chunks:\n")
			// We split the code context and add it to the prompt.
			// To keep the prompt size reasonable, we can potentially truncate here as well.

			// If the search repository is "root", then we update the file paths in the code chunks
			// to reflect the path from the root.
			codeContext := topic.CodeContext
			if searchRepo == "root" || searchRepo == "" {
				codeContext = service.adjustCodePaths(topic.CodeContext, topic.Repository)
			}

			allTopicCode := strings.Split(codeContext, "\n\n")
			currentLength := sb.Len()
			for _, code := range allTopicCode {
				if currentLength+len(code) > maxPromptLength {
					break
				}
				sb.WriteString(fmt.Sprintf("%s\n", code))
				currentLength += len(code)
			}
		}
		sb.WriteString("\n")
	}

	summary, err := service.genAiClient.GetSummary(ctx, service.summaryModel, sb.String())
	if err != nil {
		sklog.Errorf("Error getting summary from Gemini: %v", err)
		return nil, err
	}

	return &pb.GetSummaryResponse{
		Summary: summary,
	}, nil
}
