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

	"go.skia.org/infra/go/influxdb_init"
	"go.skia.org/infra/go/metrics2"

	"github.com/BurntSushi/toml"
	graphite "github.com/cyberdelia/go-metrics-graphite"
	"github.com/davecgh/go-spew/spew"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
)

const (
	REPO_SKIA       = "https://skia.googlesource.com/skia.git"
	REPO_SKIA_INFRA = "https://skia.googlesource.com/buildbot.git"

	SAMPLE_PERIOD = time.Minute
)

// Runs commonly-used initialization metrics.
func Init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// See skbug.com/4386 for details on why the below section exists.
	glog.Info("Initializing logserver for log level INFO.")
	glog.Warning("Initializing logserver for log level WARNING.")
	glog.Error("Initializing logserver for log level ERROR.")

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// Runs normal Init functions as well as tracking runtime metrics.
// Sets up Graphite push for go-metrics' DefaultRegistry. Users of
// both InitWithMetrics and metrics.DefaultRegistry will not need to
// run graphite.Graphite(metrics.DefaultRegistry, ...) separately.
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
		glog.Warningf("No metrics server specified.")
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
	go graphite.Graphite(metrics.DefaultRegistry, SAMPLE_PERIOD, appPrefix, addr)

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

// Runs normal Init functions as well as tracking runtime metrics.
// Sets up metrics push into InfluxDB.
func InitWithMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string, local *bool) {
	Init()
	influxClient, err := influxdb_init.NewClientFromParamsAndMetadata(*influxHost, *influxUser, *influxPassword, *influxDatabase, *local)
	if err != nil {
		glog.Fatal(err)
	}
	if err := metrics2.Init(appName, influxClient); err != nil {
		glog.Fatal(err)
	}

	// Start runtime metrics.
	metrics2.RuntimeMetrics()
}

// Defer from main() to log any panics and flush the log. Defer this function before any other
// defers.
func LogPanic() {
	if r := recover(); r != nil {
		glog.Fatal(r)
	}
	glog.Flush()
}

func DecodeTomlFile(filename string, configuration interface{}) {
	if _, err := toml.DecodeFile(filename, configuration); err != nil {
		glog.Fatalf("Failed to decode config file %s: %s", filename, err)
	}

	conf_str := spew.Sdump(configuration)
	glog.Infof("Read TOML configuration from %s: %s", filename, conf_str)
}

// MultiString implements flag.Value, allowing it to be used as
// var slice common.MultiString
// func init() {
// 	flag.Var(&slice, "someArg", "list of frobulators")
// }
//
// And then a client can pass in multiple values like
// my_executable --someArg foo --someArg bar
// or
// my_executable --someArg foo,bar,baz
// or any combination of
// my_executable --someArg alpha --someArg beta,gamma --someArg delta
type MultiString []string

// NewMultiStringFlag returns a MultiString flag, loaded with the given
// preloadedValues, usage string and name.
// NOTE: because of how MultiString functions, the values passed in are
// not the traditional "default" values, because they will not be replaced
// by the flags, only appended to.
func NewMultiStringFlag(name string, preloadedValues []string, usage string) *MultiString {
	m := MultiString(preloadedValues)
	flag.Var(&m, name, usage)
	return &m
}

// String() returns the current value of MultiString, as a comma seperated list
func (m *MultiString) String() string {
	return strings.Join(*m, ",")
}

// From the flag docs: "Set is called once, in command line order, for each flag present.""
func (m *MultiString) Set(value string) error {
	for _, s := range strings.Split(value, ",") {
		*m = append(*m, s)
	}
	return nil
}
