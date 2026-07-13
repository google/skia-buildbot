package backend

import (
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/perf/go/backend/shared"
	pinpoint_service "go.skia.org/infra/pinpoint/go/service"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	tpr_client "go.skia.org/infra/temporal/go/client"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

// pinpointService implements backend.BackendService, provides a wrapper struct
// for the pinpoint service implementation.
type pinpointService struct {
	pb.PinpointServer
}

// NewPinpointService returns a new instance of the pinpoint service.
func NewPinpointService(t tpr_client.TemporalProvider, l *rate.Limiter, hostPort string, namespace string, taskQueue string, devMode bool) *pinpointService {
	return &pinpointService{
		PinpointServer: pinpoint_service.New(t, l, hostPort, namespace, taskQueue, devMode),
	}
}

// GetAuthorizationPolicy returns the authorization policy for the service.
func (service *pinpointService) GetAuthorizationPolicy() shared.AuthorizationPolicy {
	return shared.AuthorizationPolicy{
		AllowUnauthenticated: false,
		AuthorizedRoles: []roles.Role{
			roles.Editor,
			roles.Bisecter,
			roles.Admin,
		},
	}
}

// RegisterGrpc registers the grpc service with the server instance.
func (service *pinpointService) RegisterGrpc(grpcServer *grpc.Server) {
	pb.RegisterPinpointServer(grpcServer, service.PinpointServer)
}

// GetServiceDescriptor returns the service descriptor for the service.
func (service *pinpointService) GetServiceDescriptor() grpc.ServiceDesc {
	return pb.Pinpoint_ServiceDesc
}
