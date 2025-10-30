package history

import (
	"context"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	pb "go.skia.org/infra/rag/proto/history/v1"
)

// ApiService provides a struct for the HistoryRag api implementation.
type ApiService struct {
	pb.UnimplementedHistoryRagApiServiceServer

	// Spanner database client.
	blameStore blamestore.BlameStore
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(dbClient *spanner.Client) *ApiService {
	return &ApiService{
		blameStore: blamestore.New(dbClient),
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
