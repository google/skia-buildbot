// traceserver is a gRPC server for trace.service.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// flags
var (
	db_file        = flag.String("db_file", "", "The name of the BoltDB file that will store the traces.")
	port           = flag.String("port", ":9090", "The port to serve the gRPC endpoint on.")
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	cpuprofile     = flag.String("cpuprofile", "", "Write cpu profile to file.")
)

func main() {
	common.InitWithMetrics(filepath.Base(os.Args[0]), graphiteServer)
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

	go func() {
		if *cpuprofile != "" {
			f, err := os.Create(*cpuprofile)
			if err != nil {
				glog.Fatalf("Failed to open profiling file: %s", err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				glog.Fatalf("Failed to start profiling: %s", err)
			}
		}
		glog.Fatalf("Failure while serving: %s", s.Serve(lis))
	}()

	// Handle SIGINT and SIGTERM.
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
}
