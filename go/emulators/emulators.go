// Package emulators contains functions to start and stop emulators, and utilities to work with the
// various *_EMULATOR_HOST environment variables.
//
// Unless otherwise specified, all functions in this package assume that there will be at most one
// instance of each emulator running at any given time.
package emulators

// This package uses "os/exec" as opposed to "go.skia.org/infra/go/exec" in order to avoid the
// following circular dependency:
//
//   //go/exec/exec_test.go -> //go/testutils/unittest/unittest.go -> //go/emulators/emulators.go

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"go.skia.org/infra/bazel/external/google_cloud_sdk"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/netutils"
	"go.skia.org/infra/go/skerr"
)

// Emulator represents a Google Cloud emulator, a test-only CockroachDB server, etc.
type Emulator string

const (
	// BigTable represents a Google Cloud BigTable emulator.
	BigTable = Emulator("BigTable")

	// CockroachDB represents a test-only CockroachDB instance.
	CockroachDB = Emulator("CockroachDB")

	// Datastore represents a Google Cloud Datastore emulator.
	Datastore = Emulator("Datastore")

	// Firestore represents a Google Cloud Firestore emulator.
	Firestore = Emulator("Firestore")

	// PubSub represents a Google Cloud PubSub emulator.
	PubSub = Emulator("PubSub")
)

// AllEmulators contains a list of all known emulators.
var AllEmulators = []Emulator{BigTable, CockroachDB, Datastore, Firestore, PubSub}

// emulatorInfo holds the information necessary to start an emulator and manage its lifecycle.
type emulatorInfo struct {
	cmd    string // Command and arguments to start the emulator on a developer workstation.
	envVar string // Name of the emulator's environment variable, e.g. "FOO_EMULATOR_HOST".
	port   int    // The emulator's TCP port when running locally. Ignored under RBE.
}

// cachedEmulatorInfos is populated by getCachedEmulatorInfo() on the first call. Do not use
// directly.
var cachedEmulatorInfos = map[Emulator]emulatorInfo{}

// getCachedEmulatorInfo builds and returns an emulatorInfo struct for the given emulator. The
// struct's contents will depend on whether we are running on Bazel and RBE or not. All structs are
// computed once and cached. Subsequent calls will return the same struct given the same input.
func getCachedEmulatorInfo(emulator Emulator) emulatorInfo {
	if len(cachedEmulatorInfos) == 0 {
		cachedEmulatorInfos = map[Emulator]emulatorInfo{
			BigTable:    makeEmulatorInfo(BigTable),
			CockroachDB: makeEmulatorInfo(CockroachDB),
			Datastore:   makeEmulatorInfo(Datastore),
			Firestore:   makeEmulatorInfo(Firestore),
			PubSub:      makeEmulatorInfo(PubSub),
		}
	}

	emulatorInfo, ok := cachedEmulatorInfos[emulator]
	if !ok {
		panic("Unknown emulator: " + emulator)
	}
	return emulatorInfo
}

// makeEmulatorInfo returns an emulatorInfo struct for the given emulator. Under RBE, the emulator
// port will be chosen by the OS to minimize chances of parallel tests from interfering with each
// other. This function returns a new struct every time it's called, so different calls under RBE
// RBE will return structs with different ports.
func makeEmulatorInfo(emulator Emulator) emulatorInfo {
	gcloud := "gcloud"
	if bazel.InBazelTest() {
		var err error
		gcloud, err = google_cloud_sdk.FindGcloud()
		if err != nil {
			panic(fmt.Sprintf("Could not find Bazel-downloaded gcloud command: %s", err))
		}
	}

	var info emulatorInfo
	switch emulator {
	case BigTable:
		info = emulatorInfo{
			cmd:    fmt.Sprintf("%s beta emulators bigtable start --host-port=localhost:%%d --project=test-project", gcloud),
			envVar: "BIGTABLE_EMULATOR_HOST",
			port:   8892,
		}
	case CockroachDB:
		info = emulatorInfo{
			cmd:    computeCockroachDBCmd(),
			envVar: "COCKROACHDB_EMULATOR_HOST",
			port:   8895,
		}
	case Datastore:
		info = emulatorInfo{
			cmd:    fmt.Sprintf("%s beta emulators datastore start --no-store-on-disk --host-port=localhost:%%d --project=test-project", gcloud),
			envVar: "DATASTORE_EMULATOR_HOST",
			port:   8891,
		}
	case Firestore:
		info = emulatorInfo{
			cmd:    fmt.Sprintf("%s beta emulators firestore start --host-port=localhost:%%d", gcloud),
			envVar: "FIRESTORE_EMULATOR_HOST",
			port:   8894,
		}
	case PubSub:
		info = emulatorInfo{
			cmd:    fmt.Sprintf("%s beta emulators pubsub start --host-port=localhost:%%d --project=test-project", gcloud),
			envVar: "PUBSUB_EMULATOR_HOST",
			port:   8893,
		}
	default:
		panic("Unknown emulator: " + emulator)
	}

	// Under Bazel and RBE, we choose an unused port to minimize the chances of parallel tests from
	// interfering with each other.
	if bazel.InBazelTestOnRBE() {
		info.port = netutils.FindUnusedTCPPort()
	}

	return info
}

func computeCockroachDBCmd() string {
	// Read the CockroachDB storage directory from an environment variable, or create a temp dir.
	cockroachDbStoreDir := os.Getenv("COCKROACHDB_EMULATOR_STORE_DIR")
	if cockroachDbStoreDir == "" {
		var err error
		cockroachDbStoreDir, err = ioutil.TempDir("", "crdb-emulator-*")
		if err != nil {
			panic("Error while creating temporary directory: " + skerr.Wrap(err).Error())
		}
	}

	cmd := fmt.Sprintf("cockroach start-single-node --insecure --listen-addr=localhost:%%d --store=%s", cockroachDbStoreDir)

	// Under RBE, we want the web UI to be served on a random TCP port. This minimizes the chance of
	// parallel tests from interfering with each other.
	if bazel.InBazelTestOnRBE() {
		cmd += " --http-addr=localhost:0"
	} else {
		// The default port for Cockroach's web UI 8080, but that is the same port at which we serve
		// demo pages during development.
		cmd += " --http-addr=localhost:9090"
	}

	return cmd
}

// GetEmulatorHostEnvVar returns the contents of the *_EMULATOR_HOST environment variable
// corresponding to the given emulator, or the empty string if the environment variable is unset.
func GetEmulatorHostEnvVar(emulator Emulator) string {
	return os.Getenv(getCachedEmulatorInfo(emulator).envVar)
}

// SetEmulatorHostEnvVar sets the *_EMULATOR_HOST environment variable for the given emulator to
// point to an emulator instance started via StartEmulatorIfNotRunning.
//
// It's OK to call this function before calling StartEmulatorIfNotRunning because both functions
// look up the emulator information (e.g. TCP port) from a package-private, global dictionary.
func SetEmulatorHostEnvVar(emulator Emulator) error {
	return setEmulatorHostEnvVarFromEmulatorInfo(getCachedEmulatorInfo(emulator))
}

func setEmulatorHostEnvVarFromEmulatorInfo(info emulatorInfo) error {
	return skerr.Wrap(os.Setenv(info.envVar, fmt.Sprintf("localhost:%d", info.port)))
}

// UnsetAllEmulatorHostEnvVars unsets the *_EMULATOR_HOST environment variables for all known
// emulators.
func UnsetAllEmulatorHostEnvVars() error {
	for _, emulator := range AllEmulators {
		if err := os.Setenv(getCachedEmulatorInfo(emulator).envVar, ""); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// GetEmulatorHostEnvVarName returns the name of the *_EMULATOR_HOST environment variable
// corresponding to the given emulator.
func GetEmulatorHostEnvVarName(emulator Emulator) string {
	return getCachedEmulatorInfo(emulator).envVar
}

// runningEmulators keeps track of which emulators have been started
var runningEmulators = map[Emulator]bool{}

// IsRunning returns true is the given emulator was started, or false if it hasn't been started or
// if it's been stopped.
func IsRunning(emulator Emulator) bool {
	return runningEmulators[emulator]
}

// StartEmulatorIfNotRunning starts an emulator if it's not already running. Returns true if it
// started the emulator, or false if the emulator was already running.
func StartEmulatorIfNotRunning(emulator Emulator) (bool, error) {
	if IsRunning(emulator) {
		return false, nil
	}
	if err := startEmulator(getCachedEmulatorInfo(emulator)); err != nil {
		return false, skerr.Wrap(err)
	}
	runningEmulators[emulator] = true
	return true, nil
}

// StartAdHocEmulatorInstanceAndSetEmulatorHostEnvVarBazelRBEOnly starts a new instance of the given
// emulator, regardless of whether a previous instance was already started, and sets the
// corresponding *_EMULATOR_HOST environment variable to point to the newly started instance. Any
// emulator instances started via this function will be ignored by StartEmulatorIfNotRunning.
//
// This only works under RBE because under RBE, emulators are assigned an unused TCP port chosen by
// the operating system, which makes it possible to run multiple instances of the same emulator in
// parallel (e.g. one instance started via this function, and another one started via
// StartEmulatorIfNotRunning). This function will panic if called outside of RBE.
func StartAdHocEmulatorInstanceAndSetEmulatorHostEnvVarBazelRBEOnly(emulator Emulator) error {
	if !bazel.InBazelTestOnRBE() {
		panic("This function cannot be called outside of RBE.")
	}
	info := makeEmulatorInfo(emulator)
	if err := startEmulator(info); err != nil {
		return skerr.Wrap(err)
	}
	if err := setEmulatorHostEnvVarFromEmulatorInfo(info); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// startEmulator starts an emulator using the command in the given struct.
func startEmulator(emulatorInfo emulatorInfo) error {
	programAndArgsStr := fmt.Sprintf(emulatorInfo.cmd, emulatorInfo.port)
	programAndArgs := strings.Split(programAndArgsStr, " ")
	cmd := exec.Command(programAndArgs[0], programAndArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if bazel.InBazelTestOnRBE() {
		// Force emulator child processes to die as soon as the parent process (e.g. the Go test runner)
		// dies. If we don't do this, the emulators will continue running indefinitely after the parent
		// process dies, eventually timing out.
		//
		// Note that this is only possible under Linux. The below function call will panic under
		// non-Linux operating systems. Running emulator tests under RBE on non-Linux OSes is therefore
		// not supported. This is OK because our RBE instance is currently Linux-only. See the comments
		// in the function body for alternative approaches if we ever decide to run emulator tests under
		// RBE on other operating systems.
		cmd.SysProcAttr = makeSysProcAttrWithPdeathsigSIGKILL()
	}

	// Start the emulator.
	fmt.Printf("Starting emulator: %s\n", programAndArgsStr)
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}

	// Log the emulator's exit status.
	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("Emulator %s finished with error: %v\n", programAndArgsStr, err)
			return
		}
		fmt.Printf("Emulator %s finished with exit status: %d\n", programAndArgsStr, cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus())
	}()

	return nil
}

// StartAllEmulators starts all known emulators.
func StartAllEmulators() error {
	for _, emulator := range AllEmulators {
		if _, err := StartEmulatorIfNotRunning(emulator); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

var emulatorProcsToKill = []*regexp.Regexp{
	regexp.MustCompile("[g]cloud\\.py"),
	regexp.MustCompile("[c]loud_datastore_emulator"),
	regexp.MustCompile("[C]loudDatastore.jar"),
	regexp.MustCompile("[c]btemulator"),
	regexp.MustCompile("[c]loud-pubsub-emulator"),
	regexp.MustCompile("[c]loud-firestore-emulator"),
	regexp.MustCompile("[c]ockroach"),
}

// StopAllEmulators gracefully terminates all known emulators.
func StopAllEmulators() error {
	signal := "SIGTERM"
	if bazel.InBazelTestOnRBE() {
		// Under Bazel and RBE, we don't need graceful termination because the RBE containers are
		// ephemeral. Killing the emulators with SIGKILL is faster and simpler.
		signal = "SIGKILL"
	}
	return stopAllEmulators(signal)
}

// ForceStopAllEmulators immediately terminates all known emulators with SIGKILL.
func ForceStopAllEmulators() error {
	return stopAllEmulators("SIGKILL")
}

func stopAllEmulators(signal string) error {
	// List all processes.
	psCmd := exec.Command("ps", "aux")
	var psOut bytes.Buffer
	psCmd.Stdout = &psOut
	if err := psCmd.Run(); err != nil {
		return skerr.Wrap(err)
	}

	// Parse the output of the previous command.
	lines := strings.Split(psOut.String(), "\n")
	procs := make(map[string]string, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		procs[line] = fields[1]
	}

	// Kill each matching process.
	for _, re := range emulatorProcsToKill {
		for desc, id := range procs {
			if re.MatchString(desc) {
				if err := exec.Command("kill", "-s", signal, id).Run(); err != nil {
					return skerr.Wrap(err)
				}
				delete(procs, desc)
			}
		}
	}

	// After this function returns, IsRunning(emulator) should return false for all known emulators.
	for _, emulator := range AllEmulators {
		runningEmulators[emulator] = false
	}

	return nil
}
