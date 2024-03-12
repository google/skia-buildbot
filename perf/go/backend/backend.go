package backend

import (
	"net"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/grpcsp"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const appName = "backend"

// Backend provides a struct for the application.
type Backend struct {
	promPort         string
	grpcPort         string
	grpcServer       *grpc.Server
	serverAuthPolicy *grpcsp.ServerPolicy
	lisGRPC          net.Listener
}

// BackendService provides an interface for a service to be hosted on Backend application.
type BackendService interface {
	// GetAuthorizationPolicy returns the authorization policy for the service.
	GetAuthorizationPolicy() shared.AuthorizationPolicy

	// RegisterGrpc registers the grpc service with the server instance.
	RegisterGrpc(server *grpc.Server)

	// GetServiceDescriptor returns the service descriptor for the service.
	GetServiceDescriptor() grpc.ServiceDesc
}

// initialize initializes the Backend application.
func (b *Backend) initialize() error {
	common.InitWithMust(
		appName,
		common.PrometheusOpt(&b.promPort),
	)

	sklog.Infof("Registering grpc reflection server.")
	reflection.Register(b.grpcServer)

	sklog.Info("Registering grpc services.")

	// Add all the services that will be hosted here.
	services := []BackendService{}
	err := b.registerServices(services)
	if err != nil {
		return err
	}
	b.lisGRPC, _ = net.Listen("tcp", b.grpcPort)

	sklog.Infof("Backend server listening at %v", b.lisGRPC.Addr())

	cleanup.AtExit(b.Cleanup)
	return nil
}

// registerServices registers all available services for Backend.
func (b *Backend) registerServices(services []BackendService) error {
	for _, service := range services {
		service.RegisterGrpc(b.grpcServer)
		err := b.configureAuthorizationForService(service)
		if err != nil {
			return err
		}
	}

	return nil
}

// configureAuthorizationForService configures authorization rules for the given BackendService.
func (b *Backend) configureAuthorizationForService(service BackendService) error {
	servicePolicy, err := b.serverAuthPolicy.Service(service.GetServiceDescriptor())
	if err != nil {
		sklog.Errorf("Error creating auth policy for service: %v", err)
		return err
	}
	authPolicy := service.GetAuthorizationPolicy()
	if authPolicy.AllowUnauthenticated {
		if err := servicePolicy.AuthorizeUnauthenticated(); err != nil {
			sklog.Errorf("Error configuring unauthenticated access for service: %v", err)
			return err
		}
	} else {
		if err := servicePolicy.AuthorizeRoles(authPolicy.AuthorizedRoles); err != nil {
			sklog.Errorf("Error configuring roles for service: %v", err)
			return err
		}
		if authPolicy.MethodAuthorizedRoles != nil {
			for method, authorizedRoles := range authPolicy.MethodAuthorizedRoles {
				if err := servicePolicy.AuthorizeMethodForRoles(method, authorizedRoles); err != nil {
					sklog.Errorf("Error configuring roles for method %s: %v", method, err)
					return err
				}
			}
		}
	}

	return nil
}

// ServeGRPC does not return unless there is an error during the startup process, in which case it
// returns the error, or if a call to [Cleanup()] causes a graceful shutdown, in which
// case it returns either nil if the graceful shutdown succeeds, or an error if it does not.
func (b *Backend) ServeGRPC() error {
	if err := b.grpcServer.Serve(b.lisGRPC); err != nil {
		sklog.Errorf("failed to serve grpc: %v", err)
		return err
	}

	return nil
}

// New creates a new instance of Backend application.
func New(flags *config.BackendFlags) (*Backend, error) {
	opts := []grpc.ServerOption{}
	b := &Backend{
		grpcServer:       grpc.NewServer(opts...),
		grpcPort:         flags.Port,
		promPort:         flags.PromPort,
		serverAuthPolicy: grpcsp.Server(),
	}

	err := b.initialize()
	return b, err
}

// Cleanup performs a graceful shutdown of the grpc server.
func (b *Backend) Cleanup() {
	sklog.Info("Shutdown server gracefully.")
	if b.grpcServer != nil {
		b.grpcServer.GracefulStop()
	}
}

// Serve intiates the listener to serve traffic.
func (b *Backend) Serve() {
	if err := b.ServeGRPC(); err != nil {
		sklog.Fatal(err)
	}
}
