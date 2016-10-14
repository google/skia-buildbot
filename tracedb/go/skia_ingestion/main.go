package main

// skia_ingestion is the server process that runs an arbitary number of
// ingesters and stores them in traceDB backends.

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/geventbus"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
	_ "go.skia.org/infra/golden/go/goldingestion"
	_ "go.skia.org/infra/golden/go/pdfingestion"
	_ "go.skia.org/infra/perf/go/perfingestion"
	storage "google.golang.org/api/storage/v1"
)

// Command line flags.
var (
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	configFilename     = flag.String("config_filename", "default.toml", "Configuration file in TOML format.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
	nsqdAddress        = flag.String("nsqd", "", "Address and port of nsqd instance.")
	influxHost         = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser         = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword     = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase     = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	memProfile         = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
)

func main() {
	defer common.LogPanic()
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMetrics2(appName, influxHost, influxUser, influxPassword, influxDatabase, local)

	// If no nsqd servers is defines, we simply don't have gloabl events.
	var globalEventBus geventbus.GlobalEventBus = nil
	var err error
	if *nsqdAddress != "" {
		globalEventBus, err = geventbus.NewNSQEventBus(*nsqdAddress)
		if err != nil {
			glog.Fatalf("Unable to connect to NSQ server at address %s: %s", *nsqdAddress, err)
		}
	}
	evt := eventbus.New(globalEventBus)

	// Initialize oauth client and start the ingesters.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, storage.CloudPlatformScope)
	if err != nil {
		glog.Fatalf("Failed to auth: %s", err)
	}

	// Start the ingesters.
	config, err := sharedconfig.ConfigFromTomlFile(*configFilename)
	if err != nil {
		glog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}

	ingesters, err := ingestion.IngestersFromConfig(config, client, evt)
	if err != nil {
		glog.Fatalf("Unable to instantiate ingesters: %s", err)
	}
	for _, oneIngester := range ingesters {
		oneIngester.Start()
	}

	// Enable the memory profiler if memProfile was set.
	if *memProfile > 0 {
		writeProfileFn := func() {
			glog.Infof("\nWriting Memory Profile")
			f, err := ioutil.TempFile("./", "memory-profile")
			if err != nil {
				glog.Fatalf("Unable to create memory profile file: %s", err)
			}
			if err := pprof.WriteHeapProfile(f); err != nil {
				glog.Fatalf("Unable to write memory profile file: %v", err)
			}
			util.Close(f)
			glog.Infof("Memory profile written to %s", f.Name())

			os.Exit(0)
		}

		// Write the profile after the given time or whenever we get a SIGINT signal.
		time.AfterFunc(*memProfile, writeProfileFn)
		ch := make(chan os.Signal)
		signal.Notify(ch, os.Interrupt)
		go func() {
			<-ch
			writeProfileFn()
		}()
	}

	// Run the ingesters forever.
	select {}
}
