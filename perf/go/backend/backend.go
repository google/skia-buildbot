package backend

import (
	"context"
	"net"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/grpcsp"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalygroup"
	ag_service "go.skia.org/infra/perf/go/anomalygroup/service"
	"go.skia.org/infra/perf/go/backend/shared"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/config/validate"
	"go.skia.org/infra/perf/go/culprit"
	"go.skia.org/infra/perf/go/culprit/notify"
	culprit_service "go.skia.org/infra/perf/go/culprit/service"
	"go.skia.org/infra/perf/go/subscription"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const appName = "backend"

// Backend provides a struct for the application.
type Backend struct {
	configFileName   string
	promPort         string
	grpcPort         string
	grpcServer       *grpc.Server
	serverAuthPolicy *grpcsp.ServerPolicy
	lisGRPC          net.Listener
	flags            *config.BackendFlags
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
func (b *Backend) initialize(anomalygroupStore anomalygroup.Store, culpritStore culprit.Store, subscriptionStore subscription.Store, notifier notify.CulpritNotifier) error {
	common.InitWithMust(
		appName,
		common.PrometheusOpt(&b.promPort),
	)

	var err error
	ctx := context.Background()

	// Load the config file.
	sklog.Infof("Loading configs from %s", b.configFileName)
	if err = validate.LoadAndValidate(b.configFileName); err != nil {
		sklog.Fatal(err)
	}

	sklog.Info("Creating anomalygroup store.")
	if anomalygroupStore == nil {
		anomalygroupStore, err = builders.NewAnomalyGroupStoreFromConfig(ctx, config.Config)
		if err != nil {
			sklog.Errorf("Error creating anomalgroup store. %s", err)
			return err
		}
	}
	sklog.Info("Creating culprit notifier.")
	if notifier == nil {
		notifier, err = notify.GetDefaultNotifier(ctx, config.Config, b.flags.CommitRangeURL)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	sklog.Info("Creating culprit store.")
	if culpritStore == nil {
		culpritStore, err = builders.NewCulpritStoreFromConfig(ctx, config.Config)
		if err != nil {
			sklog.Errorf("Error creating culprit store. %s", err)
			return err
		}
	}

	sklog.Info("Creating subscription store.")
	if subscriptionStore == nil {
		subscriptionStore, err = builders.NewSubscriptionStoreFromConfig(ctx, config.Config)
		if err != nil {
			sklog.Errorf("Error creating subscription store. %s", err)
			return err
		}
	}

	sklog.Info("Configuring grpc services.")
	// Add all the services that will be hosted here.
	services := []BackendService{
		NewPinpointService(nil, nil),
		ag_service.New(anomalygroupStore),
		culprit_service.New(anomalygroupStore, culpritStore, subscriptionStore, notifier),
	}
	err = b.configureServices(services)
	if err != nil {
		return err
	}
	opts := []grpc.ServerOption{grpc.UnaryInterceptor(b.serverAuthPolicy.UnaryInterceptor())}
	b.grpcServer = grpc.NewServer(opts...)
	sklog.Infof("Registering grpc reflection server.")
	reflection.Register(b.grpcServer)
	sklog.Info("Registering individual services.")
	b.registerServices(services)

	b.lisGRPC, _ = net.Listen("tcp4", b.grpcPort)

	sklog.Infof("Backend server listening at %v", b.lisGRPC.Addr())

	cleanup.AtExit(b.Cleanup)
	return nil
}

// configureServices configures all available services for Backend.
func (b *Backend) configureServices(services []BackendService) error {
	for _, service := range services {
		err := b.configureAuthorizationForService(service)
		if err != nil {
			return err
		}
	}

	return nil
}

// registerServices registers all available services with the grpc server.
func (b *Backend) registerServices(services []BackendService) {
	for _, service := range services {
		service.RegisterGrpc(b.grpcServer)
	}
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
func New(flags *config.BackendFlags,
	anomalygroupStore anomalygroup.Store,
	culpritStore culprit.Store,
	subscriptionStore subscription.Store,
	notifier notify.CulpritNotifier,
) (*Backend, error) {
	b := &Backend{
		configFileName:   flags.ConfigFilename,
		grpcPort:         flags.Port,
		promPort:         flags.PromPort,
		serverAuthPolicy: grpcsp.Server(),
		flags:            flags,
	}

	err := b.initialize(anomalygroupStore, culpritStore, subscriptionStore, notifier)
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
