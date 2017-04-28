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

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sharedb"
	"go.skia.org/infra/go/sklog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// flags
var (
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file.")
	db_file    = flag.String("db_file", "", "The name of the BoltDB file that will store the traces.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":9090", "The port to serve the gRPC endpoint on.")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	sharedbDir = flag.String("sharedb_dir", "", "Directory used by shareDB. If empty shareDB service will not enabled.")
	noCloudLog = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
)

func main() {
	// Parse the options. So we can configure logging.
	flag.Parse()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}

	// Should we disable cloud logging.
	if !(*noCloudLog) {
		logOpts = append(logOpts, common.CloudLoggingOpt())
	}
	common.InitWithMust(filepath.Base(os.Args[0]), logOpts...)

	ts, err := traceservice.NewTraceServiceServer(*db_file)
	if err != nil {
		sklog.Fatalf("Failed to initialize the tracestore server: %s", err)
	}

	lis, err := net.Listen("tcp", *port)
	if err != nil {
		sklog.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.MaxMsgSize(tracedb.MAX_MESSAGE_SIZE))
	traceservice.RegisterTraceServiceServer(s, ts)

	// If a directory for sharedb was registered add a the sharedb service.
	if *sharedbDir != "" {
		sharedb.RegisterShareDBServer(s, sharedb.NewServer(*sharedbDir))
	}

	go func() {
		if *cpuprofile != "" {
			f, err := os.Create(*cpuprofile)
			if err != nil {
				sklog.Fatalf("Failed to open profiling file: %s", err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				sklog.Fatalf("Failed to start profiling: %s", err)
			}
		}
		sklog.Fatalf("Failure while serving: %s", s.Serve(lis))
	}()

	// Handle SIGINT and SIGTERM.
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
}
