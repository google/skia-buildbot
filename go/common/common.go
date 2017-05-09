// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/sklog"
)

const (
	// Compute Engine project ID.
	PROJECT_ID = "google.com:skia-buildbots"

	REPO_CHROMIUM           = "https://chromium.googlesource.com/chromium/src.git"
	REPO_DEPOT_TOOLS        = "https://chromium.googlesource.com/chromium/tools/depot_tools.git"
	REPO_SKIA               = "https://skia.googlesource.com/skia.git"
	REPO_SKIA_INFRA         = "https://skia.googlesource.com/buildbot.git"
	REPO_SKIA_INTERNAL      = "https://skia.googlesource.com/skia_internal.git"
	REPO_SKIA_INTERNAL_TEST = "https://skia.googlesource.com/internal_test.git"

	SAMPLE_PERIOD = time.Minute
)

var (
	PUBLIC_REPOS  = []string{REPO_SKIA, REPO_SKIA_INFRA}
	PRIVATE_REPOS = []string{REPO_SKIA_INTERNAL, REPO_SKIA_INTERNAL_TEST}
	ALL_REPOS     = append(PUBLIC_REPOS, PRIVATE_REPOS...)

	// PROJECT_REPO_MAPPING is a mapping of project names to repo URLs. It
	// is filled in during init().
	PROJECT_REPO_MAPPING = map[string]string{}

	// REPO_PROJECT_MAPPING is a mapping of repo URLs to project names.
	REPO_PROJECT_MAPPING = map[string]string{
		REPO_SKIA:               "skia",
		REPO_SKIA_INFRA:         "skiabuildbot",
		REPO_SKIA_INTERNAL:      "skia-internal",
		REPO_SKIA_INTERNAL_TEST: "skia-internal-test",
	}
)

// init runs setup for the common package.
func init() {
	// Fill in PROJECT_REPO_MAPPING.
	for k, v := range REPO_PROJECT_MAPPING {
		PROJECT_REPO_MAPPING[v] = k
	}
	// buildbot.git is sometimes referred to as "buildbot" instead of
	// "skiabuildbot". Add the alias to the mapping.
	PROJECT_REPO_MAPPING["buildbot"] = REPO_SKIA_INFRA

	// internal_test.git is sometimes referred to as "internal_test" instead
	// of "skia_internal_test". Add the alias to the mapping.
	PROJECT_REPO_MAPPING["internal_test"] = REPO_SKIA_INTERNAL_TEST

	// skia_internal.git is sometimes referred to as "skia_internal" instead
	// of "skia-internal". Add the alias to the mapping.
	PROJECT_REPO_MAPPING["skia_internal"] = REPO_SKIA_INTERNAL
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

	startCloudLoggingWithClient(c, hostname, logName)
}

// startCloudLoggingWithClient initializes cloud logging with the passed in params.
// It is recommended clients only call this if they need to specially configure the params,
// otherwise use StartCloudLogging or, better, InitWithCloudLogging.
// startCloudLoggingWithClient should be called before the program creates any go routines
// such that all subsequent logs are properly sent to the Cloud.
func startCloudLoggingWithClient(authClient *http.Client, logGrouping, defaultReport string) {
	// Initialize all severity counters to 0, otherwise uncommon logs (like Error), won't
	// be in metrics at all.
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
