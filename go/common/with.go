package common

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

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
	flag.Parse()
	glog.Info("base preinit")
	return nil
}

func (b *baseInitOpt) init(appName string) error {
	glog.Info("base init")
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

// cloudLoggingInitOpt implements Opt for cloud logging.
type cloudLoggingInitOpt struct {
	serviceAccountPath *string
}

// CloudLoggingOpt creates an Opt to initialize cloud logging when passed to InitWith().
//
// Uses metadata to configure the auth.
func CloudLoggingOpt() Opt {
	return &cloudLoggingInitOpt{}
}

// CloudLoggingJWTOpt creates an Opt to initialize cloud logging when passed to InitWith().
//
// Instead of using metadata, this uses a local service account file.
func CloudLoggingJWTOpt(serviceAccountPath *string) Opt {
	return &cloudLoggingInitOpt{
		serviceAccountPath: serviceAccountPath,
	}
}

func (o *cloudLoggingInitOpt) preinit(appName string) error {
	glog.Info("cloudlogging preinit")
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Could not get hostname: %s", err)
	}
	return sklog.PreInitCloudLogging(hostname, appName)
}

func (o *cloudLoggingInitOpt) init(appName string) error {
	glog.Info("cloudlogging init")
	transport := &http.Transport{
		Dial: httputils.FastDialTimeout,
	}
	path := ""
	if o.serviceAccountPath != nil {
		path = *(o.serviceAccountPath)
	}
	c, err := auth.NewJWTServiceAccountClient("", path, transport, sklog.CLOUD_LOGGING_WRITE_SCOPE)
	if err != nil {
		return fmt.Errorf("Problem getting authenticated client: %s", err)
	}
	metricLookup := map[string]metrics2.Counter{}
	for _, sev := range sklog.AllSeverities {
		metricLookup[sev] = metrics2.GetCounter("num_log_lines", map[string]string{"level": sev, "log_source": appName})
	}
	metricsCallback := func(severity string) {
		metricLookup[severity].Inc(1)
	}
	return sklog.PostInitCloudLogging(c, metricsCallback)
}

func (o *cloudLoggingInitOpt) order() int {
	return 1
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
	glog.Info("prom preinit")
	return metrics2.InitPromMaybeInflux(*o.port)
}

func (o *promInitOpt) init(appName string) error {
	glog.Info("prom init")

	// App uptime.
	_ = metrics2.NewLiveness("uptime", nil)
	return nil
}

func (o *promInitOpt) order() int {
	return 3
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
			return fmt.Errorf("Only one of each type of Opt can be used.")
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
