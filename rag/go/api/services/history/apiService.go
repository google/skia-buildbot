package history

import (
	"context"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/sklog"
	pb "go.skia.org/infra/rag/proto/history/v1"
)

// ApiService provides a struct for the HistoryRag api implementation.
type ApiService struct {
	pb.UnimplementedHistoryRagApiServiceServer

	// Spanner database client.
	dbClient *spanner.Client
}

// NewApiService returns a new instance of the ApiService struct.
func NewApiService(dbClient *spanner.Client) *ApiService {
	return &ApiService{
		dbClient: dbClient,
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
//
// TODO(ashwinpv): Implement the api.
func (service *ApiService) GetBlames(ctx context.Context, req *pb.GetBlamesRequest) (*pb.GetBlamesResponse, error) {
	sklog.Infof("Received GetBlames request")
	resp := &pb.GetBlamesResponse{}
	return resp, nil
}
