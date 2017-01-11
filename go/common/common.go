// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb_init"
	"go.skia.org/infra/go/metrics2"

	"github.com/BurntSushi/toml"
	"github.com/davecgh/go-spew/spew"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/sklog"
)

const (
	// Compute Engine project ID.
	PROJECT_ID = "google.com:skia-buildbots"

	REPO_SKIA               = "https://skia.googlesource.com/skia.git"
	REPO_SKIA_INFRA         = "https://skia.googlesource.com/buildbot.git"
	REPO_SKIA_INTERNAL_TEST = "https://skia.googlesource.com/internal_test.git"

	SAMPLE_PERIOD = time.Minute
)

var (
	PUBLIC_REPOS  = []string{REPO_SKIA, REPO_SKIA_INFRA}
	PRIVATE_REPOS = []string{REPO_SKIA_INTERNAL_TEST}
	ALL_REPOS     = append(PUBLIC_REPOS, PRIVATE_REPOS...)
)

// Need opts for Influx, Prometheus, CloudLogging.
//
// How to handle having CloudLogging, which is really
// two inits, put part in preinit().
//
// Order:
//  base - 0
//  cloudlogging - 1
//  influx - 2
//  prometheus - 3
//
// Then add InitViaOpts(opts...), which always prepends the baseInitOpt(),
// sorts, and then calls preinit() on each one, and then calls init() on each
// one.

type Opt interface {
	// order is the sort order that Opts are executed in.
	order() int
	preinit(appName string) error
	init(appName string) error
}

type OptSlice []Opt

func (p OptSlice) Len() int           { return len(p) }
func (p OptSlice) Less(i, j int) bool { return p[i].order() < p[j].order() }
func (p OptSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// === base ===
type baseInitOpt struct{}

// Init runs commonly-used initialization metrics.
func (b *baseInitOpt) preinit(appName string) error {
	flag.Parse()
	defer sklog.Flush()
	return nil
}

func (b *baseInitOpt) init(appName string) error {
	flag.VisitAll(func(f *flag.Flag) {
		sklog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// See skbug.com/4386 for details on why the below section exists.
	sklog.Info("Initializing logging for log level INFO.")
	sklog.Warning("Initializing logging for log level WARNING.")
	sklog.Error("Initializing logging for log level ERROR.")

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	return nil
}

func (b *baseInitOpt) order() int {
	return 0
}

// == cloudlogging ==

type cloudLoggingInitOpt struct{}

func CloudLoggingOpt() Opt {
	return &cloudLoggingInitOpt{}
}

func (b *cloudLoggingInitOpt) preinit(appName string) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Could not get hostname: %s", err)
	}
	return sklog.PreInitCloudLogging(hostname, appName)
}

func (b *cloudLoggingInitOpt) init(appName string) error {
	transport := &http.Transport{
		Dial: httputils.FastDialTimeout,
	}
	c, err := auth.NewJWTServiceAccountClient("", "", transport, sklog.CLOUD_LOGGING_WRITE_SCOPE)
	if err != nil {
		return fmt.Errorf("Problem getting authenticated client: %s", err)
	}
	metricsCallback := func(severity string) {
		metrics2.GetCounter("num_log_lines", map[string]string{"level": severity, "log_source": appName}).Inc(1)
	}
	return sklog.PostInitCloudLogging(c, metricsCallback)
}

func (b *cloudLoggingInitOpt) order() int {
	return 1
}

// == influx ==

type influxInitOpt struct {
	influxHost     *string
	influxUser     *string
	influxPassword *string
	influxDatabase *string
	skipMetadata   *bool
}

func InfluxOpt(influxHost, influxUser, influxPassword, influxDatabase *string, skipMetadata *bool) Opt {
	return &influxInitOpt{
		influxHost:     influxHost,
		influxUser:     influxUser,
		influxPassword: influxPassword,
		influxDatabase: influxDatabase,
		skipMetadata:   skipMetadata,
	}
}

func (b *influxInitOpt) preinit(appName string) error {
	return nil
}

func (b *influxInitOpt) init(appName string) error {
	influxClient, err := influxdb_init.NewClientFromParamsAndMetadata(*b.influxHost, *b.influxUser, *b.influxPassword, *b.influxDatabase, *b.skipMetadata)
	if err != nil {
		return fmt.Errorf("Failed to create influx client: %s", err)
	}
	if err := metrics2.Init(appName, influxClient); err != nil {
		return fmt.Errorf("Failed to init metrics2 for influx: %s", err)
	}
	metrics2.RuntimeMetrics()
	return nil
}

func (b *influxInitOpt) order() int {
	return 2
}

// == prom ==

type promInitOpt struct {
	port *string
}

func PrometheusOpt(port *string) Opt {
	return &promInitOpt{
		port: port,
	}
}

func (b *promInitOpt) preinit(appName string) error {
	return nil
}

func (b *promInitOpt) init(appName string) error {
	return metrics2.InitPromMaybeInflux(*b.port)
}

func (b *promInitOpt) order() int {
	return 3
}

func InitWith(appName string, opts ...Opt) error {
	sort.Sort(OptSlice(opts))
	for _, o := range opts {
		if err := o.preinit(appName); err != nil {
			return err
		}
	}
	for _, o := range opts {
		if err := o.init(appName); err != nil {
			return err
		}
	}
	return nil
}

// Init runs commonly-used initialization metrics.
func Init() {
	flag.Parse()
	defer sklog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		sklog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// See skbug.com/4386 for details on why the below section exists.
	sklog.Info("Initializing logging for log level INFO.")
	sklog.Warning("Initializing logging for log level WARNING.")
	sklog.Error("Initializing logging for log level ERROR.")

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

// InitWithCloudLogging runs normal Init functions, and tracks runtime metrics. It uses Cloud Logging
// if skipMetadata is false. The influx* arguments are ignored and read from metadata
// unless *skipMetadata is true.
// InitWithCloudLogging should be called before the program creates any go routines such that all
// subsequent logs are properly sent to the Cloud.
func InitWithCloudLogging(appName string, influxHost, influxUser, influxPassword, influxDatabase *string, skipMetadata *bool) {
	Init()
	StartMetrics2(appName, influxHost, influxUser, influxPassword, influxDatabase, *skipMetadata)
	// disable cloud logging when run locally.
	if !*skipMetadata {
		StartCloudLogging(appName)
	}
}

// StartMetrics2 starts tracking runtime metrics and sets up metrics push into InfluxDB. The
// influx* arguments are ignored and read from metadata unless skipMetadata is true.
func StartMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string, skipMetadata bool) {
	influxClient, err := influxdb_init.NewClientFromParamsAndMetadata(*influxHost, *influxUser, *influxPassword, *influxDatabase, skipMetadata)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := metrics2.Init(appName, influxClient); err != nil {
		sklog.Fatal(err)
	}

	// Start runtime metrics.
	metrics2.RuntimeMetrics()
}

// StartCloudLogging initializes cloud logging. It is assumed to be running in GCE where the
// project metadata has the sklog.CLOUD_LOGGING_WRITE_SCOPE set. It exits fatally if anything
// goes wrong. InitWithCloudLogging should be called before the program creates any go routines
// such that all subsequent logs are properly sent to the Cloud.
func StartCloudLogging(logName string) {
	transport := &http.Transport{
		Dial: httputils.FastDialTimeout,
	}
	c, err := auth.NewJWTServiceAccountClient("", "", transport, sklog.CLOUD_LOGGING_WRITE_SCOPE)
	if err != nil {
		sklog.Fatalf("Problem getting authenticated client: %s", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Could not get hostname: %s", err)
	}

	StartCloudLoggingWithClient(c, hostname, logName)
}

// StartCloudLoggingWithClient initializes cloud logging with the passed in params.
// It is recommended clients only call this if they need to specially configure the params,
// otherwise use StartCloudLogging or, better, InitWithCloudLogging.
// StartCloudLoggingWithClient should be called before the program creates any go routines
// such that all subsequent logs are properly sent to the Cloud.
func StartCloudLoggingWithClient(authClient *http.Client, logGrouping, defaultReport string) {
	// Initialize all severity counters to 0, otherwise uncommon logs (like Error), won't
	// be in InfluxDB at all.
	initSeverities := []string{sklog.INFO, sklog.WARNING, sklog.ERROR}
	for _, severity := range initSeverities {
		metrics2.GetCounter("num_log_lines", map[string]string{"level": severity, "log_source": defaultReport}).Reset()
	}

	metricsCallback := func(severity string) {
		metrics2.GetCounter("num_log_lines", map[string]string{"level": severity, "log_source": defaultReport}).Inc(1)
	}
	if err := sklog.InitCloudLogging(authClient, logGrouping, defaultReport, metricsCallback); err != nil {
		sklog.Fatal(err)
	}
}

// LogPanic, when deferred from main, logs any panics and flush the log to local disk using glog.
// Defer this function before any other defers.
func LogPanic() {
	if r := recover(); r != nil {
		glog.Fatal(r)
	}
	glog.Flush()
}

// DecodeTomlFile decodes a TOML file into the passed in struct and logs it to sklog.  If there is
// an error, it panics.
func DecodeTomlFile(filename string, configuration interface{}) {
	if _, err := toml.DecodeFile(filename, configuration); err != nil {
		sklog.Fatalf("Failed to decode config file %s: %s", filename, err)
	}

	conf_str := spew.Sdump(configuration)
	sklog.Infof("Read TOML configuration from %s: %s", filename, conf_str)
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
