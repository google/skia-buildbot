package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

	"cloud.google.com/go/spanner"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/api/services/history"
	"go.skia.org/infra/rag/go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const defaultOutputDimensionality = 768

// Service defines an interface for a service hosted by the HistoryRag server.
type Service interface {
	// RegisterGrpc registers the grpc service with the server instance.
	RegisterGrpc(server *grpc.Server)

	// RegisterHttp registers the http service with the server instance.
	RegisterHttp(ctx context.Context, mux *runtime.ServeMux) error

	// GetServiceDescriptor returns the service descriptor for the service.
	GetServiceDescriptor() grpc.ServiceDesc
}

// ApiServerFlags defines the commandline flags to start the api server.
type ApiServerFlags struct {
	ConfigFilename string
	GrpcPort       string
	HttpPort       string
	PromPort       string
	Services       cli.StringSlice
	Local          bool
	ResourcesDir   string
}

// AsCliFlags returns a slice of cli.Flag.
func (flags *ApiServerFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "./configs/demo.json",
			Usage:       "The name of the config file to use.",
		},
		&cli.StringSliceFlag{
			Name:        "services",
			Value:       cli.NewStringSlice("history"),
			Usage:       "This list of RAG services to host on the api.",
			Destination: &flags.Services,
		},
		&cli.StringFlag{
			Destination: &flags.GrpcPort,
			Name:        "grpc_port",
			Value:       ":8000",
			Usage:       "The port number to use for grpc server.",
		},
		&cli.StringFlag{
			Destination: &flags.HttpPort,
			Name:        "http_port",
			Value:       ":8002",
			Usage:       "The port number to use for http server.",
		},
		&cli.StringFlag{
			Destination: &flags.PromPort,
			Name:        "prom_port",
			Value:       ":20000",
			Usage:       "Metrics service address (e.g., ':10110')",
		},
		&cli.BoolFlag{
			Destination: &flags.Local,
			Name:        "local",
			Value:       false,
		},
		&cli.StringFlag{
			Destination: &flags.ResourcesDir,
			Name:        "resources_dir",
			Value:       "./dist",
			Usage:       "The directory to serve static files from.",
		},
	}
}

// apiServer defines a struct for creating the server.
type apiServer struct {
	// Spanner database client.
	dbClient            *spanner.Client
	queryEmbeddingModel string
	dimensionality      int32

	// Grpc server objects
	grpcServer *grpc.Server
	lisGRPC    net.Listener
	grpcPort   string

	// HTTP server objects
	httpHandler   http.Handler
	httpPort      string
	resourcesDir  string
	instanceName  string
	headerIconUrl string
}

// NewApiServer returns a new instance of the api server based on the provided flags.
func NewApiServer(flags *ApiServerFlags) (*apiServer, error) {
	ctx := context.Background()
	// Read the configuration.
	config, err := config.NewApiServerConfigFromFile(flags.ConfigFilename)
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

	dimensionality := int32(config.OutputDimensionality)
	if dimensionality == 0 {
		dimensionality = defaultOutputDimensionality
	}

	server := &apiServer{
		dbClient:            spannerClient,
		queryEmbeddingModel: config.QueryEmbeddingModel,
		dimensionality:      dimensionality,
		grpcPort:            flags.GrpcPort,
		httpPort:            flags.HttpPort,
		resourcesDir:        flags.ResourcesDir,
		instanceName:        config.InstanceName,
		headerIconUrl:       config.HeaderIconUrl,
	}
	err = server.initialize(ctx, flags)
	if err != nil {
		return nil, err
	}

	return server, nil
}

// initialize performs the init steps for the apiServer object.
func (server *apiServer) initialize(ctx context.Context, flags *ApiServerFlags) error {
	// Initialize metrics/
	metrics2.InitPrometheus(flags.PromPort)

	// Define the list of services to be hosted based on the "services" flag.
	serviceList := []Service{}
	var serviceMap = map[string]Service{
		"history": history.NewApiService(ctx, server.dbClient, server.queryEmbeddingModel, server.dimensionality),
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
	gwmux := runtime.NewServeMux()

	sklog.Info("Registering individual services.")
	server.registerServices(ctx, serviceList, gwmux)

	rootMux := http.NewServeMux()
	rootMux.Handle("/historyrag/", gwmux)

	server.registerUIHandlers(rootMux)
	server.httpHandler = rootMux

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
func (server *apiServer) registerServices(ctx context.Context, serviceList []Service, gwmux *runtime.ServeMux) {
	for _, service := range serviceList {
		service.RegisterGrpc(server.grpcServer)
		err := service.RegisterHttp(ctx, gwmux)
		if err != nil {
			sklog.Fatalf("Error registering http handler for service %v", err)
		}
	}
}

// registerUIHandlers registers the handler required to serve the UI pages.
func (server *apiServer) registerUIHandlers(serverMux *http.ServeMux) {
	// Add the handler to serve static content.
	serverMux.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir(server.resourcesDir))))

	// Add the handler for the home page.
	serverMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, filepath.Join(server.resourcesDir, "index.html"))
	})

	// Add the handler for retrieving config data.
	serverMux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := struct {
			InstanceName  string `json:"instance_name"`
			HeaderIconUrl string `json:"header_icon_url"`
		}{
			InstanceName:  server.instanceName,
			HeaderIconUrl: server.headerIconUrl,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			httputils.ReportError(w, err, "Failed to encode config", http.StatusInternalServerError)
		}
	})
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
		Handler: httputils.HealthzAndHTTPS(server.httpHandler),
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
