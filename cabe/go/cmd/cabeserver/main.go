// package main is the main entry point for the cabe server executable.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/swarming"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/grpcsp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/cabe/go/analysisserver"
	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
)

const (
	appName   = "cabe"
	drainTime = time.Second * 5
)

func init() {
	// Workaround for "ERROR: logging before flag.Parse" messages that show
	// up due to some transitive dependency on glog (we don't use it directly).
	// See: https://github.com/kubernetes/kubernetes/issues/17162
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	_ = fs.Parse([]string{})
	flag.CommandLine = fs
}

// App is the cabe server application.
type App struct {
	port          string
	grpcPort      string
	promPort      string
	disableGRPCSP bool
	authPolicy    *grpcsp.ServerPolicy

	swarmingClient swarming.ApiClient
	rbeClients     map[string]*rbeclient.Client

	httpServer *http.Server
	grpcServer *grpc.Server
}

// FlagSet constructs a flag.FlagSet for the App.
func (a *App) FlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	fs.StringVar(&a.port, "port", ":8002", "HTTP service address (e.g., ':8002')")
	fs.StringVar(&a.promPort, "prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.StringVar(&a.grpcPort, "grpc_port", ":50051", "gRPC service port (e.g., ':50051')")
	fs.BoolVar(&a.disableGRPCSP, "disable_grpcsp", false, "disable authorization checks for incoming grpc calls")

	return fs
}

// Start creates server instances and listens for connections on their ports.
// It does not return unless there is an error during the startup process, in which case it
// returns the error, or if a call to [Cleanup()] causes a graceful shutdown, in which
// case it returns either nil if the graceful shutdown succeeds, or an error if it does not.
func (a *App) Start(ctx context.Context) error {
	if a.swarmingClient == nil {
		return fmt.Errorf("missing swarming service client")
	}
	if a.rbeClients == nil {
		return fmt.Errorf("missing rbe service clients")
	}
	if a.authPolicy == nil && !a.disableGRPCSP {
		return fmt.Errorf("missing required grpc authorization policy")
	}

	go func() {
		// Just testing the http healthz check to make sure envoy can
		// connect to these processes at all. If we end up needing
		// both the http server and the grpc server in order to satisfy envoy
		// health checks AND serve grpc requests, we can separate the http and
		// grpc port flags in k8s configs.
		sklog.Infof("registering http healthz handler")
		topLevelRouter := mux.NewRouter()
		h := httputils.HealthzAndHTTPS(topLevelRouter)
		httpServeMux := http.NewServeMux()
		httpServeMux.Handle("/", h)
		lis, err := net.Listen("tcp", a.port)
		if err != nil {
			sklog.Fatal(err)
		}
		// If the port was specified as ":0" and the OS picked a port for us,
		// set the app's port to the actual port it's listening on.
		a.port = lis.Addr().String()
		a.httpServer = &http.Server{
			Addr:    a.port,
			Handler: httpServeMux,
		}
		if err := a.httpServer.Serve(lis); err != nil && err != http.ErrServerClosed {
			sklog.Fatal(err)
		}
	}()

	opts := []grpc.ServerOption{}
	if !a.disableGRPCSP {
		opts = append(opts, grpc.UnaryInterceptor(a.authPolicy.UnaryInterceptor()))
	}
	a.grpcServer = grpc.NewServer(opts...)

	sklog.Infof("registering grpc health server")
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.grpcServer, healthServer)

	sklog.Infof("registering grpc reflection server")
	reflection.Register(a.grpcServer)

	sklog.Infof("registering cabe grpc server")
	analysisServer := analysisserver.New(a.rbeClients, a.swarmingClient)

	lis, err := net.Listen("tcp", a.grpcPort)
	if err != nil {
		sklog.Fatalf("failed to listen: %v", err)
	}
	// If the port was specified as ":0" and the OS picked a port for us,
	// set the app's grpc port to the actual port it's listening on.
	a.grpcPort = lis.Addr().String()
	cpb.RegisterAnalysisServer(a.grpcServer, analysisServer)

	sklog.Infof("server listening at %v", lis.Addr())
	if err := a.grpcServer.Serve(lis); err != nil {
		sklog.Fatalf("failed to serve grpc: %v", err)
	}

	return nil
}

// DialBackends establishes rpc channel connections to backend
// services required by App.
func (a *App) DialBackends(ctx context.Context) error {
	sklog.Infof("dialing RBE-CAS backends")
	rbeClients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Fatalf("dialing RBE-CAS backends: %v", err)
		return err
	}
	sklog.Infof("successfully dialed %d RBE-CAS instances", len(rbeClients))
	a.rbeClients = rbeClients

	sklog.Infof("dialing Swarming")
	swarmingClient, err := backends.DialSwarming(ctx)
	if err != nil {
		sklog.Fatalf("dialing swarming: %v", err)
		return err
	}
	sklog.Infof("successfully dialed swarming")
	a.swarmingClient = swarmingClient
	return nil
}

// ConfigureAuthorization configures a role-based authorization policy for the grpc server and
// the services it serves.
func (a *App) ConfigureAuthorization() error {
	a.authPolicy = grpcsp.Server()

	healthPolicy, err := a.authPolicy.Service(grpc_health_v1.Health_ServiceDesc)
	if err != nil {
		sklog.Errorf("creating auth policy for service: %v", err)
		return err
	}
	if err := healthPolicy.AuthorizeUnauthenticated(); err != nil {
		sklog.Errorf("configuring roles for service: %v", err)
		return err
	}

	analysisPolicy, err := a.authPolicy.Service(cpb.Analysis_ServiceDesc)
	if err != nil {
		sklog.Errorf("creating auth policy for service: %v", err)
		return err
	}
	if err := analysisPolicy.AuthorizeRoles(roles.Roles{roles.Admin}); err != nil {
		sklog.Errorf("configuring roles for service: %v", err)
		return err
	}
	if err := analysisPolicy.AuthorizeMethodForRoles("GetAnalysis", roles.Roles{roles.Viewer}); err != nil {
		sklog.Errorf("configuring roles for method: %v", err)
		return err
	}

	return nil
}

// Cleanup gracefully shuts down any running servers and closes
// any open backend connections.
func (a *App) Cleanup() {
	sklog.Info("Shutdown server gracefully.")
	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}

	if err := a.httpServer.Shutdown(context.Background()); err != nil {
		sklog.Errorf("shutting down http server: %v", err)
	}

	// Now shut down client connections to backends that have clean shutdown methods.
	for instance, rbeClient := range a.rbeClients {
		if err := rbeClient.Close(); err != nil {
			sklog.Errorf("closing RBE client connection for instance %q: %v", instance, err)
		}
	}

	// The [swarming.ApiClient] interface does not offer a clean shutdown method.
}

func main() {
	a := &App{}

	common.InitWithMust(
		appName,
		common.PrometheusOpt(&a.promPort),
		common.FlagSetOpt(a.FlagSet()),
	)

	if err := a.ConfigureAuthorization(); err != nil {
		sklog.Fatalf("configuring authorization policy: %v", err)
	}

	ctx := context.Background()

	if err := a.DialBackends(ctx); err != nil {
		sklog.Fatalf("dialing backends: %v", err)
	}

	cleanup.AtExit(a.Cleanup)
	sklog.Fatal(a.Start(ctx))
}
