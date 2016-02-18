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
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/sharedb"
	"go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// flags
var (
	db_file        = flag.String("db_file", "", "The name of the BoltDB file that will store the traces.")
	port           = flag.String("port", ":9090", "The port to serve the gRPC endpoint on.")
	cpuprofile     = flag.String("cpuprofile", "", "Write cpu profile to file.")
	sharedbDir     = flag.String("sharedb_dir", "", "Directory used by shareDB. If empty shareDB service will not enabled.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func main() {
	common.InitWithMetrics2(filepath.Base(os.Args[0]), influxHost, influxUser, influxPassword, influxDatabase, local)
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

	// If a directory for sharedb was registered add a the sharedb service.
	if *sharedbDir != "" {
		sharedb.RegisterShareDBServer(s, sharedb.NewServer(*sharedbDir))
	}

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
