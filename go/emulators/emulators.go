// Package emulators contains functions to start and stop emulators, and utilities to work with the
// various *_EMULATOR_HOST environment variables.
package emulators

// This package uses "os/exec" as opposed to "go.skia.org/infra/go/exec" in order to avoid the
// following circular dependency:
//
//   //go/exec/exec_test.go -> //go/testutils/unittest/unittest.go -> //go/emulators/emulators.go

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"go.skia.org/infra/bazel/go/bazel"
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

// cachedEmulatorInfos is populated by getEmulatorInfo() on the first call. Do not use directly.
var cachedEmulatorInfos = map[Emulator]*emulatorInfo{}

// getEmulatorInfo builds and returns an emulatorInfo struct for the given emulator. The struct's
// contents will depend on whether we are running on Bazel and RBE or not. All structs are computed
// once and cached. Subsequent calls will return the same struct given the same input.
func getEmulatorInfo(emulator Emulator) emulatorInfo {
	if len(cachedEmulatorInfos) == 0 {
		cachedEmulatorInfos = map[Emulator]*emulatorInfo{
			BigTable: {
				cmd:    "gcloud beta emulators bigtable start --host-port=localhost:%d --project=test-project",
				envVar: "BIGTABLE_EMULATOR_HOST",
				port:   8892,
			},
			CockroachDB: {
				cmd:    computeCockroachDBCmd(),
				envVar: "COCKROACHDB_EMULATOR_HOST",
				port:   8895,
			},
			Datastore: {
				cmd:    "gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:%d --project=test-project",
				envVar: "DATASTORE_EMULATOR_HOST",
				port:   8891,
			},
			Firestore: {
				cmd:    "gcloud beta emulators firestore start --host-port=localhost:%d",
				envVar: "FIRESTORE_EMULATOR_HOST",
				port:   8894,
			},
			PubSub: {
				cmd:    "gcloud beta emulators pubsub start --host-port=localhost:%d --project=test-project",
				envVar: "PUBSUB_EMULATOR_HOST",
				port:   8893,
			},
		}

		// Under Bazel and RBE, we choose an unused port to minimize the chances of parallel tests from
		// interfering with each other.
		if bazel.InRBE() {
			for _, emulatorInfo := range cachedEmulatorInfos {
				emulatorInfo.port = findUnusedTCPPort()
			}
		}
	}

	emulatorInfo, ok := cachedEmulatorInfos[emulator]
	if !ok {
		panic("Unknown emulator: " + emulator)
	}
	return *emulatorInfo
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
	if bazel.InRBE() {
		cmd += " --http-addr=localhost:0"
	}

	return cmd
}

// findUnusedTCPPort finds an unused TCP port by opening a TCP port on an unused port chosen by the
// operating system, recovering the port number and immediately closing the socket.
func findUnusedTCPPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(skerr.Wrap(err))
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		panic(skerr.Wrap(err))
	}
	return port
}

// GetEmulatorHostEnvVar returns the contents of the *_EMULATOR_HOST environment variable corresponding to
// the given emulator, or the empty string if the environment variable is unset.
func GetEmulatorHostEnvVar(emulator Emulator) string {
	return os.Getenv(getEmulatorInfo(emulator).envVar)
}

// SetEmulatorHostEnvVar sets the *_EMULATOR_HOST environment variable for the given emulator.
func SetEmulatorHostEnvVar(emulator Emulator) error {
	emulatorInfo := getEmulatorInfo(emulator)
	return skerr.Wrap(os.Setenv(emulatorInfo.envVar, fmt.Sprintf("localhost:%d", emulatorInfo.port)))
}

// UnsetAllEmulatorHostEnvVars unsets the *_EMULATOR_HOST environment variables for all known
// emulators.
func UnsetAllEmulatorHostEnvVars() error {
	for _, emulator := range AllEmulators {
		if err := os.Setenv(getEmulatorInfo(emulator).envVar, ""); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// GetEmulatorHostEnvVarName returns the name of the *_EMULATOR_HOST environment variable
// corresponding to the given emulator.
func GetEmulatorHostEnvVarName(emulator Emulator) string {
	return getEmulatorInfo(emulator).envVar
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

	emulatorInfo := getEmulatorInfo(emulator)
	programAndArgs := strings.Split(fmt.Sprintf(emulatorInfo.cmd, emulatorInfo.port), " ")
	cmd := exec.Command(programAndArgs[0], programAndArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if bazel.InRBE() {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			// Under Bazel and RBE, emulators are launched by each individual go_test Bazel target. The
			// below setting kills the emulator processes (and any other child processes) as soon as the
			// parent process (i.e. the test runner) dies.
			//
			// If we don't do this, the emulators will continue running indefinitely, and Bazel will
			// eventually time out while waiting for these child processes to die.
			//
			// This setting is Linux-only, but that's OK because our RBE instance consists of Linux
			// machines exclusively. Alternative approaches include adding a TestMain function to our
			// emulator tests that launches the emulators before running the test cases and kills them
			// afterwards, or leveraging the test_on_env Bazel macro to run an environment binary
			// alongside the tests which controls the emulators' lifecycle. Any of these approaches would
			// work on non-Linux OSes as well.
			Pdeathsig: syscall.SIGKILL,
		}
	}

	if err := cmd.Start(); err != nil {
		return false, skerr.Wrap(err)
	}

	runningEmulators[emulator] = true
	return true, nil
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

// StopAllEmulators stops all known emulators.
func StopAllEmulators() error {
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

	signal := "SIGTERM"
	if bazel.InRBE() {
		// Under Bazel and RBE, we don't need graceful termination because the RBE containers are
		// ephemeral. Killing the emulators with SIGKILL is faster and simpler.
		signal = "SIGKILL"
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
