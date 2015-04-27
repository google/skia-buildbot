// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/davecgh/go-spew/spew"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
)

const SAMPLE_PERIOD = time.Minute

// Runs commonly-used initialization metrics.
func Init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// Runs normal Init functions as well as tracking runtime metrics.
// Sets up Graphite push for go-metrics' DefaultRegistry. Users of
// both InitWithMetrics and metrics.DefaultRegistry will not need to
// run metrics.Graphite(metrics.DefaultRegistry, ...) separately.
func InitWithMetrics(appName string, graphiteServer *string) {
	Init()

	startMetrics(appName, *graphiteServer)
}

// Get the graphite server from a callback function; useful when the graphite
// server isn't known ahead of time (e.g., when reading from a config file)
func InitWithMetricsCB(appName string, getGraphiteServer func() string) {
	Init()

	// Note(stephana): getGraphiteServer relies on Init() being called first.
	startMetrics(appName, getGraphiteServer())
}

// TODO(stephana): Refactor startMetrics to return an error instead of
// terminating the app.

func startMetrics(appName, graphiteServer string) {
	if graphiteServer == "" {
		glog.Warningf("No metrics server speicifed.")
		return
	}

	addr, err := net.ResolveTCPAddr("tcp", graphiteServer)
	if err != nil {
		glog.Fatalf("Unable to resolve metrics server address: %s", err)
	}

	// Get the hostname and create the app-prefix.
	hostName, err := os.Hostname()
	if err != nil {
		glog.Fatalf("Unable to retrieve hostname: %s", err)
	}
	appPrefix := fmt.Sprintf("%s.%s", appName, strings.Replace(hostName, ".", "-", -1))

	// Runtime metrics.
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, SAMPLE_PERIOD)
	go metrics.Graphite(metrics.DefaultRegistry, SAMPLE_PERIOD, appPrefix, addr)

	// Uptime.
	uptimeGuage := metrics.GetOrRegisterGaugeFloat64("uptime", metrics.DefaultRegistry)
	go func() {
		startTime := time.Now()
		uptimeGuage.Update(0)
		for _ = range time.Tick(SAMPLE_PERIOD) {
			uptimeGuage.Update(time.Since(startTime).Seconds())
		}
	}()
}

func DecodeTomlFile(filename string, configuration interface{}) {
	if _, err := toml.DecodeFile(filename, configuration); err != nil {
		glog.Fatalf("Failed to decode config file %s: %s", filename, err)
	}

	conf_str := spew.Sdump(configuration)
	glog.Infof("Read TOML configuration from %s: %s", filename, conf_str)
}
