// package main is the main entry point for the cabe server executable.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/go-chi/chi/v5"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.opencensus.io/plugin/ocgrpc"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/grpcsp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tracing"
	"go.skia.org/infra/go/tracing/loggingtracer"

	"go.skia.org/infra/cabe/go/analysisserver"
	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/grpclogging"
	"go.skia.org/infra/perf/go/perfresults"
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
	port            string
	grpcPort        string
	promPort        string
	disableGRPCSP   bool
	disableGRPCLog  bool
	traceSampleRate float64
	logTracing      bool

	lisGRPC net.Listener
	lisHTTP net.Listener

	authPolicy     *grpcsp.ServerPolicy
	grpcLogger     *grpclogging.GRPCLogger
	swarmingClient swarmingv2.SwarmingV2Client
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
	fs.BoolVar(&a.disableGRPCLog, "disable_grpclog", false, "disable structured logging for grpc client and server calls")
	fs.Float64Var(&a.traceSampleRate, "trace_sample", 0.0, "sampling rate for trace collection")
	fs.BoolVar(&a.logTracing, "log_tracing", false, "send tracing information to logs (DO NOT use in production!)")

	return fs
}

func (a *App) casResultReader(ctx context.Context, instance, digest string) (map[string]perfresults.PerfResults, error) {
	rbeClient, ok := a.rbeClients[instance]
	if !ok {
		return nil, fmt.Errorf("no RBE client for instance %s", instance)
	}

	return backends.FetchBenchmarkJSON(ctx, rbeClient, digest)
}

func (a *App) swarmingTaskReader(ctx context.Context, pinpointJobID string) ([]*apipb.TaskRequestMetadataResponse, error) {
	tasksResp, err := swarmingv2.ListTaskRequestMetadataHelper(ctx, a.swarmingClient, &apipb.TasksWithPerfRequest{
		Start: timestamppb.New(time.Now().AddDate(0, 0, -56)),
		State: apipb.StateQuery_QUERY_ALL,
		Tags:  []string{"pinpoint_job_id:" + pinpointJobID},
	})
	if err != nil {
		sklog.Fatalf("list task results: %v", err)
		return nil, err
	}
	return tasksResp, nil
}

// Init creates listeners for required service ports and prepares the App for serving.
func (a *App) Init(ctx context.Context) error {
	if a.swarmingClient == nil {
		return fmt.Errorf("missing swarming service client")
	}
	if a.rbeClients == nil {
		return fmt.Errorf("missing rbe service clients")
	}
	if a.authPolicy == nil && !a.disableGRPCSP {
		return fmt.Errorf("missing required grpc authorization policy")
	}
	var err error

	// Just testing the http healthz check to make sure envoy can
	// connect to these processes at all. If we end up needing
	// both the http server and the grpc server in order to satisfy envoy
	// health checks AND serve grpc requests, we can separate the http and
	// grpc port flags in k8s configs.
	sklog.Infof("registering http healthz handler")
	topLevelRouter := chi.NewRouter()
	h := httputils.HealthzAndHTTPS(topLevelRouter)
	httpServeMux := http.NewServeMux()
	httpServeMux.Handle("/", h)
	a.lisHTTP, err = net.Listen("tcp", a.port)
	if err != nil {
		return err
	}
	// If the port was specified as ":0" and the OS picked a port for us,
	// set the app's port to the actual port it's listening on.
	a.port = a.lisHTTP.Addr().String()
	a.httpServer = &http.Server{
		Addr:    a.port,
		Handler: httpServeMux,
	}

	opts := []grpc.ServerOption{}

	interceptors := []grpc.UnaryServerInterceptor{}
	if a.grpcLogger != nil {
		interceptors = append(interceptors, a.grpcLogger.ServerUnaryLoggingInterceptor)
	}

	if !a.disableGRPCSP {
		interceptors = append(interceptors, a.authPolicy.UnaryInterceptor())
	}

	opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	if !a.disableGRPCSP {
		opts = append(opts, grpc.UnaryInterceptor(a.authPolicy.UnaryInterceptor()))
	}
	// Add the Opencensus grpc stats/server handler in order to attach traces to the
	// contexts for incoming gRPC requests.
	opts = append(opts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))

	a.grpcServer = grpc.NewServer(opts...)

	sklog.Infof("registering grpc health server")
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.grpcServer, healthServer)

	sklog.Infof("registering grpc reflection server")
	reflection.Register(a.grpcServer)

	sklog.Infof("registering cabe grpc server")
	analysisServer := analysisserver.New(a.casResultReader, a.swarmingTaskReader)
	cpb.RegisterAnalysisServer(a.grpcServer, analysisServer)
	a.lisGRPC, err = net.Listen("tcp", a.grpcPort)
	if err != nil {
		sklog.Fatalf("failed to listen: %v", err)
	}
	// If the port was specified as ":0" and the OS picked a port for us,
	// set the app's grpc port to the actual port it's listening on.
	a.grpcPort = a.lisGRPC.Addr().String()
	sklog.Infof("server listening at %v", a.lisGRPC.Addr())
	return nil
}

// ServeGRPC does not return unless there is an error during the startup process, in which case it
// returns the error, or if a call to [Cleanup()] causes a graceful shutdown, in which
// case it returns either nil if the graceful shutdown succeeds, or an error if it does not.
func (a *App) ServeGRPC(ctx context.Context) error {
	if err := a.grpcServer.Serve(a.lisGRPC); err != nil {
		sklog.Errorf("failed to serve grpc: %v", err)
		return err
	}

	return nil
}

// ServeHTTP does not return unless there is an error in [http.Server#Serve], in which case
// it returns the error, or if a call to [Cleanup()] causes a graceful shutdown, in which
// case it returns either nil if the graceful shutdown succeeds, or an error if it does not.
func (a *App) ServeHTTP() error {
	if err := a.httpServer.Serve(a.lisHTTP); err != nil && err != http.ErrServerClosed {
		sklog.Errorf("faled to serve http: %v", err)
		return err
	}
	return nil
}

// DialBackends establishes rpc channel connections to backend
// services required by App.
func (a *App) DialBackends(ctx context.Context) error {
	sklog.Infof("dialing RBE-CAS backends")
	opts := []grpc.DialOption{}
	if a.grpcLogger != nil {
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(a.grpcLogger.ClientUnaryLoggingInterceptor),
			grpc.WithChainStreamInterceptor(a.grpcLogger.ClientStreamLoggingInterceptor))
	}

	rbeClients, err := backends.DialRBECAS(ctx, opts...)
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

	// The [swarmingv2.SwarmingV2Client] interface does not offer a clean shutdown method.
}

func main() {
	a := &App{}

	common.InitWithMust(
		appName,
		common.PrometheusOpt(&a.promPort),
		common.FlagSetOpt(a.FlagSet()),
	)

	traceAttrs := map[string]interface{}{
		"podName":       os.Getenv("POD_NAME"),
		"containerName": os.Getenv("CONTAINER_NAME"),
		"service.name":  "cabeserver",
	}
	if a.logTracing {
		loggingtracer.Initialize()
	}
	if err := tracing.Initialize(a.traceSampleRate, "skia-infra-public", traceAttrs); err != nil {
		sklog.Fatalf("Could not initialize tracing: %s", err)
	}
	if err := a.ConfigureAuthorization(); err != nil {
		sklog.Fatalf("configuring authorization policy: %v", err)
	}

	if !a.disableGRPCLog {
		projectID := ""
		if metadata.OnGCE() {
			var err error
			projectID, err = metadata.ProjectID()
			if err != nil {
				sklog.Fatal(err)
			}
		}
		a.grpcLogger = grpclogging.New(projectID, os.Stdout)
	}
	ctx := context.Background()

	if err := a.DialBackends(ctx); err != nil {
		sklog.Fatalf("dialing backends: %v", err)
	}

	cleanup.AtExit(a.Cleanup)
	if err := a.Init(ctx); err != nil {
		sklog.Fatal(err)
	}
	ch := make(chan interface{})
	go func() {
		if err := a.ServeGRPC(ctx); err != nil {
			sklog.Fatal(err)
		}
		ch <- nil
	}()
	go func() {
		if err := a.ServeHTTP(); err != nil {
			sklog.Fatal(err)
		}
		ch <- nil
	}()
	<-ch
	<-ch
}
