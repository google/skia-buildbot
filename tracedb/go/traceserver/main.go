// traceserver is a gRPC server for trace.service.
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
	"google.golang.org/grpc"
)

// flags
var (
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file.")
	db_file    = flag.String("db_file", "", "The name of the BoltDB file that will store the traces.")
	httpPort   = flag.String("http_port", ":9091", "The http port where ready-ness endpoints are served.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":9090", "The port to serve the gRPC endpoint on.")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
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
	s := grpc.NewServer(grpc.MaxSendMsgSize(tracedb.MAX_MESSAGE_SIZE), grpc.MaxRecvMsgSize(tracedb.MAX_MESSAGE_SIZE))
	traceservice.RegisterTraceServiceServer(s, ts)

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
	if *cpuprofile != "" {
		cleanup.AtExit(func() {
			pprof.StopCPUProfile()
		})
	}

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("ready"))
		util.LogErr(err)
	})
	log.Fatal(http.ListenAndServe(*httpPort, nil))
}
