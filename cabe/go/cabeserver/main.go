// package main is the main entry point for the cabe server executable.
package main

import (
	"flag"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"

	cpb "go.skia.org/infra/cabe/go/proto"
)

const (
	appName = "cabe"
)

var (
	host     = flag.String("host", "localhost", "HTTP service host")
	port     = flag.Int("port", 50051, "gRPC service port (e.g., '50051')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	flag.Parse()

	// Setup flags.
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	s := grpc.NewServer()

	sklog.Infof("registering grpc health server")
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	sklog.Infof("registering grpc reflection server")
	reflection.Register(s)

	sklog.Infof("registering cabe grpc server")
	cabeServer := NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		sklog.Fatalf("failed to listen: %v", err)
	}
	cpb.RegisterAnalysisServer(s, cabeServer)

	sklog.Infof("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		sklog.Fatalf("failed to serve: %v", err)
	}
}
