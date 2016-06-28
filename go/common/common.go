// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/influxdb_init"
	"go.skia.org/infra/go/metrics2"

	"github.com/BurntSushi/toml"
	"github.com/davecgh/go-spew/spew"
	"github.com/skia-dev/glog"
)

const (
	REPO_SKIA       = "https://skia.googlesource.com/skia.git"
	REPO_SKIA_INFRA = "https://skia.googlesource.com/buildbot.git"

	SAMPLE_PERIOD = time.Minute
)

// Init runs commonly-used initialization metrics.
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

// InitWithMetrics2 runs normal Init functions as well as tracking runtime metrics.
// It sets up metrics push into InfluxDB. The influx* arguments are ignored and read from metadata
// unless *skipMetadata is true.
func InitWithMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string, skipMetadata *bool) {
	Init()
	StartMetrics2(appName, influxHost, influxUser, influxPassword, influxDatabase, *skipMetadata)
}

// InitExternalWithMetrics2 runs normal Init functions as well as tracking runtime metrics.
// It sets up metrics push into InfluxDB. The influx* arguments are always used.
func InitExternalWithMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string) {
	Init()
	StartMetrics2(appName, influxHost, influxUser, influxPassword, influxDatabase, true)
}

// StartMetrics2 starts tracking runtime metrics and sets up metrics push into InfluxDB. The
// influx* arguments are ignored and read from metadata unless skipMetadata is true.
func StartMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string, skipMetadata bool) {
	influxClient, err := influxdb_init.NewClientFromParamsAndMetadata(*influxHost, *influxUser, *influxPassword, *influxDatabase, skipMetadata)
	if err != nil {
		glog.Fatal(err)
	}
	if err := metrics2.Init(appName, influxClient); err != nil {
		glog.Fatal(err)
	}

	// Start runtime metrics.
	metrics2.RuntimeMetrics()
}

// LogPanic, when deferred from main, logs any panics and flush the log. Defer this function before
//  any other defers.
func LogPanic() {
	if r := recover(); r != nil {
		glog.Fatal(r)
	}
	glog.Flush()
}

// DecodeTomlFile decodes a TOML file into the passed in struct and logs it to glog.  If there is
// an error, it panics.
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
