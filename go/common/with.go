package common

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"

	"cloud.google.com/go/logging"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/cloudlogging"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"golang.org/x/oauth2/google"
)

// FlagSet is the flag.FlagSet we use to parse command line flags.
// It defaults to flag.CommandLine, but can be over-ridden with
// FlagSetOpt.
var FlagSet *flag.FlagSet = flag.CommandLine

// Opt represents the initialization parameters for a single init service, where
// services are Prometheus, etc.
//
// Initializing flags, metrics, and logging, with two options for metrics, and
// another option for logging is complicated by the fact that some
// initializations are order dependent, and each app may want a different
// subset of options. The solution is to encapsulate each optional piece,
// prom, etc, into its own Opt, and then initialize each Opt in the
// right order.
//
// Not only are the Opts order dependent but initialization needs to be broken
// into two phases, preinit() and init().
//
// The desired order for all Opts is:
//  0 - base
//  1 - cloudlogging
//  3 - prometheus
//  4 - slog
//
// Construct the Opts that are desired and pass them to common.InitWith(), i.e.:
//
//	common.InitWith(
//		"skiaperf",
//		common.PrometheusOpt(promPort),
//		common.CloudLoggingOpt(),
//	)
//
type Opt interface {
	// order is the sort order that Opts are executed in.
	order() int
	preinit(appName string) error
	init(appName string) error
}

// optSlice is a utility type for sorting Opts by order().
type optSlice []Opt

func (p optSlice) Len() int           { return len(p) }
func (p optSlice) Less(i, j int) bool { return p[i].order() < p[j].order() }
func (p optSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// baseInitOpt is an Opt that is always constructed internally, added to any
// Opts passed into InitWith() and always runs first.
//
// Implements Opt.
type baseInitOpt struct{}

func (b *baseInitOpt) preinit(appName string) error {
	return FlagSet.Parse(os.Args[1:])
}

func (b *baseInitOpt) init(appName string) error {
	// Log all flags and their values.
	FlagSet.VisitAll(func(f *flag.Flag) {
		sklog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// Log all environment variables.
	for _, env := range os.Environ() {
		sklog.Infof("Env: %s", env)
	}

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Enable signal handling for the cleanup package.
	cleanup.Enable()

	// Record UID and GID.
	sklog.Infof("Running as %d:%d", os.Getuid(), os.Getgid())

	return nil
}

func (b *baseInitOpt) order() int {
	return 0
}

// cloudLoggingInitOpt implements Opt for cloud logging.
type cloudLoggingInitOpt struct {
	projectID string
	local     *bool
}

// CloudLogging creates an Opt to initialize cloud logging when passed to InitWith().
//
// Uses the instance service account for auth.
// No cloud logging is done if local is true.
func CloudLogging(local *bool, projectID string) Opt {
	return &cloudLoggingInitOpt{
		projectID: projectID,
		local:     local,
	}
}

func (o *cloudLoggingInitOpt) preinit(appName string) error {
	ctx := context.Background()
	if *o.local {
		return nil
	}
	ts, err := google.DefaultTokenSource(ctx, logging.WriteScope)
	if err != nil {
		return fmt.Errorf("problem getting authenticated token source: %s", err)
	}
	// Try to grab a token right away to confirm auth is set up correctly.
	_, err = ts.Token()
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	l, err := cloudlogging.New(ctx, o.projectID, appName, ts, map[string]string{"hostname": hostname})
	if err != nil {
		return err
	}
	sklogimpl.SetLogger(l)
	return nil
}

func (o *cloudLoggingInitOpt) init(appName string) error {
	return nil
}

func (o *cloudLoggingInitOpt) order() int {
	return 1
}

// metricsLoggingInitOpt implements Opt for logging with metrics.
type metricsLoggingInitOpt struct {
}

// MetricsLoggingOpt creates an Opt to initialize logging and record metrics when passed to InitWith().
//
func MetricsLoggingOpt() Opt {
	return &metricsLoggingInitOpt{}
}

func (o *metricsLoggingInitOpt) preinit(appName string) error {
	return nil
}

func (o *metricsLoggingInitOpt) init(appName string) error {
	severities := sklogimpl.AllSeverities()
	metricLookup := make([]metrics2.Counter, len(severities))
	for _, sev := range severities {
		metricLookup[sev] = metrics2.GetCounter("num_log_lines", map[string]string{"level": sev.String()})
	}
	metricsCallback := func(severity sklogimpl.Severity) {
		metricLookup[severity].Inc(1)
	}
	sklogimpl.SetMetricsCallback(metricsCallback)
	return nil
}

func (o *metricsLoggingInitOpt) order() int {
	return 2
}

// promInitOpt implments Opt for Prometheus.
type promInitOpt struct {
	port *string
}

// PrometheusOpt creates an Opt to initialize Prometheus metrics when passed to InitWith().
func PrometheusOpt(port *string) Opt {
	return &promInitOpt{
		port: port,
	}
}

func (o *promInitOpt) preinit(appName string) error {
	metrics2.InitPrometheus(*o.port)
	return nil
}

var defaultServeMux http.ServeMux

func (o *promInitOpt) init(appName string) error {
	// App uptime.
	_ = metrics2.NewLiveness("uptime", nil)

	// Prometheus client loads "expvar" which automatically registers
	// "/debug/vars" in the default http handler, which exposes potentially
	// secret information. Since we can't register a new handler for
	// "/debug/vars" because that causes the http lib to panic, we need to
	// replace http.DefaultServeMux with a fresh handler, and since we do this
	// after init()'s have been called, we won't get the /debug/vars handler.
	// http://b/241539244
	http.DefaultServeMux = &defaultServeMux
	http.HandleFunc("/debug/vars", http.NotFound)
	return nil
}

func (o *promInitOpt) order() int {
	return 3
}

// flagSetInitOpt implments Opt for flag.FlagSet.
type flagSetInitOpt struct {
	flagSet *flag.FlagSet
}

// FlagSetOpt allows changing the flag.FlagSet used to parse command line arguments.
//
// If this Opt is not used then flag.CommandLine is used as the default.
func FlagSetOpt(flagSet *flag.FlagSet) Opt {
	return &flagSetInitOpt{
		flagSet: flagSet,
	}
}

func (o *flagSetInitOpt) preinit(appName string) error {
	FlagSet = o.flagSet
	return nil
}

func (o *flagSetInitOpt) init(appName string) error {
	return nil
}

func (o *flagSetInitOpt) order() int {
	return -1
}

// InitWith takes Opt's and initializes each service, where services are Prometheus, etc.
func InitWith(appName string, opts ...Opt) error {

	// Add baseInitOpt.
	opts = append(opts, &baseInitOpt{})

	// Sort by order().
	sort.Sort(optSlice(opts))

	// Check for duplicate Opts.
	for i := 0; i < len(opts)-1; i++ {
		if opts[i].order() == opts[i+1].order() {
			return fmt.Errorf("only one of each type of Opt can be used")
		}
	}

	// Run all preinit's.
	for _, o := range opts {
		if err := o.preinit(appName); err != nil {
			return err
		}
	}

	// Run all init's.
	for _, o := range opts {
		if err := o.init(appName); err != nil {
			return err
		}
	}
	sklog.Flush()
	return nil
}

// InitWithMust calls InitWith and fails fatally if an error is encountered.
func InitWithMust(appName string, opts ...Opt) {
	if err := InitWith(appName, opts...); err != nil {
		sklog.Fatalf("Failed to initialize: %s", err)
	}
}
