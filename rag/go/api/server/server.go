package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/api/services/history"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Service defines an interface for a service hosted by the HistoryRag server.
type Service interface {
	// RegisterGrpc registers the grpc service with the server instance.
	RegisterGrpc(server *grpc.Server)

	// RegisterHttp registers the http service with the server instance.
	RegisterHttp(ctx context.Context, mux *runtime.ServeMux) error

	// GetServiceDescriptor returns the service descriptor for the service.
	GetServiceDescriptor() grpc.ServiceDesc
}

// apiServer defines a struct for creating the server.
type apiServer struct {
	// Spanner database client.
	dbClient *spanner.Client

	// Grpc server objects
	grpcServer *grpc.Server
	lisGRPC    net.Listener
	grpcPort   string

	// HTTP server objects
	httpHandler *runtime.ServeMux
	httpPort    string
}

// NewApiServer returns a new instance of the api server based on the provided flags.
func NewApiServer(flags *ApiServerFlags) (*apiServer, error) {
	ctx := context.Background()
	// Read the configuration.
	config, err := NewApiServerConfigFromFile(flags.ConfigFilename)
	if err != nil {
		sklog.Errorf("Error reading config file %s: %v", flags.ConfigFilename, err)
		return nil, err
	}

	// Generate the database identifier string and create the spanner client.
	databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", config.SpannerConfig.ProjectID, config.SpannerConfig.InstanceID, config.SpannerConfig.DatabaseID)
	spannerClient, err := spanner.NewClient(ctx, databaseName)
	if err != nil {
		return nil, err
	}

	server := &apiServer{
		dbClient: spannerClient,
		grpcPort: flags.GrpcPort,
		httpPort: flags.HttpPort,
	}
	err = server.initialize(ctx, flags)
	if err != nil {
		return nil, err
	}

	return server, nil
}

// initialize performs the init steps for the apiServer object.
func (server *apiServer) initialize(ctx context.Context, flags *ApiServerFlags) error {
	// Define the list of services to be hosted based on the "services" flag.
	serviceList := []Service{}
	var serviceMap = map[string]Service{
		"history": history.NewApiService(server.dbClient),
	}
	for _, serviceName := range flags.Services.Value() {
		service, ok := serviceMap[serviceName]
		if !ok {
			sklog.Fatalf("Invalid service name: %s", &serviceName)
		}
		serviceList = append(serviceList, service)
		sklog.Infof("Added service: %s", serviceName)
	}

	// Create the GRPC server.
	opts := []grpc.ServerOption{}
	server.grpcServer = grpc.NewServer(opts...)

	sklog.Infof("Registering grpc reflection server.")
	reflection.Register(server.grpcServer)

	// Create the HTTP server.
	server.httpHandler = runtime.NewServeMux()

	sklog.Info("Registering individual services.")
	server.registerServices(ctx, serviceList)

	// Set up the TCP listener for the GRPC server.
	var err error
	server.lisGRPC, err = net.Listen("tcp4", server.grpcPort)
	if err != nil {
		sklog.Errorf("failed to listen: %v", err)
		return err
	}

	cleanup.AtExit(server.cleanup)

	return nil

}

// registerServices registers all the hosted services with the server instances.
func (server *apiServer) registerServices(ctx context.Context, serviceList []Service) {
	for _, service := range serviceList {
		service.RegisterGrpc(server.grpcServer)
		err := service.RegisterHttp(ctx, server.httpHandler)
		if err != nil {
			sklog.Fatalf("Error registering http handler for service %v", err)
		}
	}
}

// server sets up the server instances to start listening for incoming requests.
func (server *apiServer) serve() error {

	// The GRPC server listens on a separate thread.
	go func() {
		sklog.Infof("Listening GRPC at %s", server.lisGRPC.Addr())
		if err := server.grpcServer.Serve(server.lisGRPC); err != nil {
			sklog.Fatalf("failed to serve grpc: %v", err)
		}
	}()

	// The http server listens on the main thread.
	httpServer := &http.Server{
		Addr:    server.httpPort,
		Handler: server.httpHandler,
	}
	sklog.Infof("Listening HTTP at %s", server.httpPort)
	if err := httpServer.ListenAndServe(); err != nil {
		sklog.Fatalf("failed to serve grpc:")
	}

	return nil
}

// Cleanup performs a graceful shutdown of the grpc server.
func (server *apiServer) cleanup() {
	sklog.Info("Shutdown server gracefully.")
	if server.grpcServer != nil {
		server.grpcServer.GracefulStop()
	}

	if server.dbClient != nil {
		server.dbClient.Close()
	}
}
