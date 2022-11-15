// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"os"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/sklog"
)

const (
	// Compute Engine project ID.
	// TODO(dogben): This probably should be passed as a command-line flag wherever it's used.
	PROJECT_ID = "google.com:skia-buildbots"

	REPO_ANGLE                         = "https://chromium.googlesource.com/angle/angle.git"
	REPO_CHROMIUM                      = "https://chromium.googlesource.com/chromium/src.git"
	REPO_DEPOT_TOOLS                   = "https://chromium.googlesource.com/chromium/tools/depot_tools.git"
	REPO_ESKIA                         = "https://skia.googlesource.com/eskia.git"
	REPO_K8S_CONFIG                    = "https://skia.googlesource.com/k8s-config.git"
	REPO_LOTTIE_CI                     = "https://skia.googlesource.com/lottie-ci.git"
	REPO_PDFIUM                        = "https://pdfium.googlesource.com/pdfium.git"
	REPO_SKCMS                         = "https://skia.googlesource.com/skcms.git"
	REPO_SKIA                          = "https://skia.googlesource.com/skia.git"
	REPO_SKIABOT_TEST                  = "https://skia.googlesource.com/skiabot-test.git"
	REPO_SKIA_AUTOROLL_INTERNAL_CONFIG = "https://skia.googlesource.com/skia-autoroll-internal-config.git"
	REPO_SKIA_INFRA                    = "https://skia.googlesource.com/buildbot.git"
	REPO_SKIA_INTERNAL                 = "https://skia.googlesource.com/skia_internal.git"
	REPO_SKIA_INTERNAL_TEST            = "https://skia.googlesource.com/internal_test.git"
	REPO_WEBRTC                        = "https://webrtc.googlesource.com/src.git"

	SAMPLE_PERIOD = time.Minute
)

var (
	// PROJECT_REPO_MAPPING is a mapping of project names to repo URLs. It
	// is filled in during init().
	PROJECT_REPO_MAPPING = map[string]string{}

	// REPO_PROJECT_MAPPING is a mapping of repo URLs to project names.
	REPO_PROJECT_MAPPING = map[string]string{
		REPO_ESKIA:                         "eskia",
		REPO_K8S_CONFIG:                    "k8s-config",
		REPO_LOTTIE_CI:                     "lottie-ci",
		REPO_SKCMS:                         "skcms",
		REPO_SKIA:                          "skia",
		REPO_SKIABOT_TEST:                  "skiabot-test",
		REPO_SKIA_AUTOROLL_INTERNAL_CONFIG: "skia-autoroll-internal-config",
		REPO_SKIA_INFRA:                    "skiabuildbot",
		REPO_SKIA_INTERNAL:                 "skia-internal",
		REPO_SKIA_INTERNAL_TEST:            "skia-internal-test",
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

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Enable signal handling for the cleanup package.
	cleanup.Enable()

	// Record UID and GID.
	sklog.Infof("Running as %d:%d", os.Getuid(), os.Getgid())
}

// Any programs which use a variant of common.Init should do `defer common.Defer()` in main.
func Defer() {
	if r := recover(); r != nil {
		// sklog.Fatal doesn't actually panic (glog does os.Exit(255)),
		// so we don't need to worry about double-printing those here.
		sklog.Fatal(r)
	}
	cleanup.Cleanup()
	sklog.Flush()
}

// multiString implements flag.Value, allowing it to be used as
// var slice multiString
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
type multiString struct {
	values *[]string
	set    bool
}

// newMultiString is a helper for creating a new MultiString.
func newMultiString(target *[]string, defaults []string) *multiString {
	if defaults != nil {
		*target = append([]string{}, defaults...)
	}
	return &multiString{
		values: target,
	}
}

// FSNewMultiStringFlag returns a []string flag, loaded with the given
// defaults, usage string and name.
func FSNewMultiStringFlag(fs *flag.FlagSet, name string, defaults []string, usage string) *[]string {
	var values []string
	m := newMultiString(&values, defaults)
	fs.Var(m, name, usage)
	return &values
}

// FSMultiStringFlagVar defines a MultiString flag with the specified name,
// defaults, and usage string. The argument target points to a []string
// variable in which to store the values of the flag.
func FSMultiStringFlagVar(fs *flag.FlagSet, target *[]string, name string, defaults []string, usage string) {
	m := newMultiString(target, defaults)
	fs.Var(m, name, usage)
}

// NewMultiStringFlag returns a []string flag, loaded with the given
// defaults, usage string and name.
func NewMultiStringFlag(name string, defaults []string, usage string) *[]string {
	return FSNewMultiStringFlag(flag.CommandLine, name, defaults, usage)
}

// MultiStringFlagVar defines a MultiString flag with the specified name,
// defaults, and usage string. The argument target points to a []string
// variable in which to store the values of the flag.
func MultiStringFlagVar(target *[]string, name string, defaults []string, usage string) {
	FSMultiStringFlagVar(flag.CommandLine, target, name, defaults, usage)
}

// String() returns the current values of multiString, as a comma separated list
func (m *multiString) String() string {
	if m == nil || m.values == nil || *m.values == nil {
		return ""
	}
	return strings.Join(*m.values, ",")
}

// From the flag docs: "Set is called once, in command line order, for each flag present.""
func (m *multiString) Set(value string) error {
	if !m.set {
		*m.values = []string{}
		m.set = true
	}
	*m.values = append(*m.values, strings.Split(value, ",")...)
	return nil
}
