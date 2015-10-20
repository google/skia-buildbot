// traceserver is a gRPC server for trace.service.
package main

import (
	"flag"
	"net"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/trace/service"
	"google.golang.org/grpc"
)

// flags
var (
	db_file        = flag.String("db_file", "", "The name of the BoltDB file that will store the traces.")
	port           = flag.String("port", ":9090", "The port to serve the gRPC endpoint on.")
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
)

func main() {
	common.InitWithMetrics("traceserver", graphiteServer)
	ts, err := traceservice.NewTraceServiceServer(*db_file)
	if err != nil {
		glog.Fatalf("Failed to initialize the tracestore server: %s", err)
	}

	lis, err := net.Listen("tcp", *port)
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(s, ts)
	glog.Fatalf("Failure while serving: %s", s.Serve(lis))
}
